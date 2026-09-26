package session

import (
	"context"
	"errors"
	"time"

	"github.com/tylergannon/gimbal/internal/pi/compact"
	"github.com/tylergannon/gimbal/internal/pi/history"
	"github.com/tylergannon/gimbal/internal/pi/model"
	"github.com/tylergannon/gimbal/internal/pi/wire"
)

// This file ports the compaction and retry portions of agent-session.ts:
// _checkCompaction, _runAutoCompaction and compact.

func (s *Session) compactionSettings() (compact.CompactionSettings, bool) {
	active := s.Model()
	modelID := ""
	if active != nil {
		modelID = active.ID
	}
	value, err := s.settings.GetCompactionSettings(modelID)
	if err != nil {
		return compact.CompactionSettings{}, false
	}
	return compact.CompactionSettings{
		Enabled:          value.Enabled,
		ReserveTokens:    value.ReserveTokens,
		KeepRecentTokens: value.KeepRecentTokens,
	}, true
}

// checkCompaction is the automatic dispatch after a run or before a prompt. It
// reports whether the post-run loop should continue the agent.
func (s *Session) checkCompaction(
	ctx context.Context,
	message *model.AssistantMessage,
	entryID string,
	toolResults []model.ToolResultMessage,
	toolResultIDs []string,
	skipAbortedCheck bool,
) (bool, error) {
	settings, ok := s.compactionSettings()
	if !ok || !settings.Enabled {
		return false, nil
	}
	if skipAbortedCheck && message.StopReason == model.StopAborted {
		return false, nil
	}
	active := s.Model()
	contextWindow := 0
	if active != nil {
		contextWindow = active.ContextWindow
	}
	sameModel := active != nil && message.Provider == active.Provider && message.Model == active.ID

	branch := s.sessionManager.GetBranch()
	compactionEntry := history.GetLatestCompactionEntry(branch)
	if compactionEntry != nil && message.Timestamp <= timestampMillis(compactionEntry.Timestamp) {
		return false, nil
	}

	projection := s.sessionManager.BuildSessionProjection()
	assistantIsProjected := entryID == "" || projectionHasAssistant(projection, entryID)
	explicitOverflow := message.StopReason == model.StopError && wire.IsContextOverflow(message, 0)
	contextOverflow := sameModel &&
		((explicitOverflow) || (assistantIsProjected && wire.IsContextOverflow(message, contextWindow)))
	maxTokens := 0
	if active != nil {
		maxTokens = active.MaxTokens
	}
	recoverableLength := sameModel && assistantIsProjected && wire.IsRecoverableLength(message, maxTokens)
	if contextOverflow || recoverableLength {
		willRetry := message.StopReason != model.StopStop
		if !willRetry {
			return s.runAutoCompaction(ctx, CompactionOverflow, false)
		}
		s.mu.Lock()
		alreadyAttempted := s.overflowAttempted
		s.mu.Unlock()
		if alreadyAttempted {
			errorMessage := "Context overflow recovery failed after one compact-and-retry attempt. Try reducing context or switching to a larger-context model."
			if !contextOverflow {
				errorMessage = "Truncated response recovery failed after one compact-and-retry attempt."
			}
			s.emitOrPanic(Event{
				Type: EventCompactionEnd, Reason: CompactionOverflow,
				Aborted: false, WillRetry: false, ErrorMessage: errorMessage,
			})
			return false, nil
		}
		s.mu.Lock()
		s.overflowAttempted = true
		s.mu.Unlock()
		s.omitRecoveryAttempt(message, entryID, toolResults, toolResultIDs)
		return s.runAutoCompaction(ctx, CompactionOverflow, willRetry)
	}

	var contextTokens int
	hasContextEdits := false
	for _, entry := range projection.Entries {
		if entry.SourceEntry != nil && entry.SourceEntry.EntryType() == "context_edit" {
			hasContextEdits = true
			break
		}
	}
	directContextTokens := compact.CalculateContextTokens(message.Usage)
	if hasContextEdits {
		contextTokens = compact.EstimateProjectedContextTokens(projection, branch).Tokens
	} else if message.StopReason == model.StopError || directContextTokens == 0 {
		messages := s.agent.State().Messages
		estimate := compact.EstimateContextTokens(messages)
		if estimate.LastUsageIndex != nil && *estimate.LastUsageIndex >= 0 && *estimate.LastUsageIndex < len(messages) {
			usageMessage := messages[*estimate.LastUsageIndex]
			if compactionEntry != nil {
				if usageAssistant, ok := assistantOf(usageMessage); ok && usageAssistant.Timestamp <= timestampMillis(compactionEntry.Timestamp) {
					return false, nil
				}
			}
		}
		contextTokens = estimate.Tokens
	} else {
		contextTokens = directContextTokens
	}
	if compact.ShouldCompact(contextTokens, contextWindow, settings) {
		return s.runAutoCompaction(ctx, CompactionThreshold, false)
	}
	return false, nil
}

func projectionHasAssistant(projection model.SessionProjection, entryID string) bool {
	for _, entry := range projection.Entries {
		if entry.SourceEntry == nil || entry.SourceEntry.Base().ID != entryID {
			continue
		}
		for _, message := range entry.Messages {
			if message.MessageRole() == model.RoleAssistant {
				return true
			}
		}
	}
	return false
}

func timestampMillis(timestamp string) int64 {
	parsed, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil {
		return 0
	}
	return parsed.UnixMilli()
}

func (s *Session) summaryOptions() (compact.SummaryOptions, bool) {
	active := s.Model()
	if active == nil {
		return compact.SummaryOptions{}, false
	}
	value := s.settings.GetRetrySettings()
	policy := retryPolicy(value)
	options := compact.SummaryOptions{
		Model:         active,
		StreamFn:      s.streamFn,
		APIKey:        s.apiKey,
		Headers:       s.headers,
		Env:           s.env,
		ThinkingLevel: s.ThinkingLevel(),
		Retry:         &policy,
		SessionID:     s.SessionID(),
		Callbacks: &wire.RetryCallbacks{
			OnRetryScheduled: func(attempt, maxAttempts, delayMs int, errorMessage string) {
				s.emitOrPanic(Event{
					Type: EventAutoRetryStart, Attempt: attempt, MaxAttempts: maxAttempts,
					DelayMs: delayMs, ErrorMessage: errorMessage,
				})
			},
			OnRetryFinished: func(success bool, attempt int, finalError string) {
				s.emitOrPanic(Event{Type: EventAutoRetryEnd, Success: success, Attempt: attempt, FinalError: finalError})
			},
		},
	}
	return options, true
}

// runAutoCompaction executes threshold or overflow compaction.
func (s *Session) runAutoCompaction(ctx context.Context, reason CompactionReason, willRetry bool) (bool, error) {
	active := s.Model()
	if active == nil {
		return false, nil
	}
	settings, ok := s.compactionSettings()
	if !ok {
		return false, nil
	}
	preparation, ok := compact.PrepareCompaction(s.sessionManager.GetBranch(), settings)
	if !ok {
		return false, nil
	}
	options, ok := s.summaryOptions()
	if !ok {
		return false, nil
	}

	compactionCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.autoCompactionCancel = cancel
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.autoCompactionCancel = nil
		s.notifyIdleLocked()
		s.mu.Unlock()
		cancel()
	}()

	s.emitOrPanic(Event{Type: EventCompactionStart, Reason: reason})

	result, err := compact.Compact(compactionCtx, preparation, options, "")
	if err != nil {
		aborted := errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
		var errorMessage string
		if !aborted {
			if reason == CompactionOverflow {
				errorMessage = "Context overflow recovery failed: " + err.Error()
			} else {
				errorMessage = "Auto-compaction failed: " + err.Error()
			}
		}
		s.emitOrPanic(Event{
			Type: EventCompactionEnd, Reason: reason, Aborted: aborted,
			WillRetry: false, ErrorMessage: errorMessage,
		})
		return false, nil
	}

	firstKept := result.FirstKeptEntryID
	usage := result.Usage
	if _, err := s.sessionManager.AppendCompaction(result.Summary, &firstKept, result.TokensBefore, result.Details, false, usage); err != nil {
		s.emitOrPanic(Event{
			Type: EventCompactionEnd, Reason: reason, Aborted: false,
			WillRetry: false, ErrorMessage: "Auto-compaction failed: " + err.Error(),
		})
		return false, nil
	}
	s.refreshContext()
	estimated := 0
	for _, message := range s.sessionManager.BuildSessionProjection().Messages {
		estimated += wire.EstimateMessageTokens(message)
	}
	result.EstimatedTokensAfter = &estimated
	s.emitOrPanic(Event{Type: EventCompactionEnd, Reason: reason, Result: &result, Aborted: false, WillRetry: willRetry})

	if willRetry {
		return true, nil
	}
	return s.agent.HasQueuedMessages(), nil
}

// Compact manually compacts the session context. It aborts the active run
// first and never retries the interrupted turn.
func (s *Session) Compact(ctx context.Context, customInstructions string) (*compact.CompactionResult, error) {
	s.Abort()
	if err := s.WaitForIdle(ctx); err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.manualCompaction++
	compactionCtx, cancel := context.WithCancel(ctx)
	s.manualCompactionCx = cancel
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.manualCompactionCx = nil
		s.manualCompaction--
		s.notifyIdleLocked()
		s.mu.Unlock()
		cancel()
	}()

	s.emitOrPanic(Event{Type: EventCompactionStart, Reason: CompactionManual})

	active := s.Model()
	if active == nil {
		err := errors.New("No model selected") //nolint:staticcheck // pi's exact error message
		s.emitOrPanic(Event{Type: EventCompactionEnd, Reason: CompactionManual, ErrorMessage: err.Error()})
		return nil, err
	}
	settings, ok := s.compactionSettings()
	if !ok {
		err := errors.New("compaction settings unavailable")
		s.emitOrPanic(Event{Type: EventCompactionEnd, Reason: CompactionManual, ErrorMessage: err.Error()})
		return nil, err
	}
	pathEntries := s.sessionManager.GetBranch()
	preparation, ok := compact.PrepareCompaction(pathEntries, settings)
	if !ok {
		var err error
		if last := pathEntries[len(pathEntries)-1]; last != nil && last.EntryType() == "compaction" {
			err = errors.New("Already compacted") //nolint:staticcheck // pi's exact error message
		} else {
			err = errors.New("Nothing to compact (session too small)") //nolint:staticcheck // pi's exact error message
		}
		s.emitOrPanic(Event{Type: EventCompactionEnd, Reason: CompactionManual, ErrorMessage: "Compaction failed: " + err.Error()})
		return nil, err
	}

	options, ok := s.summaryOptions()
	if !ok {
		err := errors.New("No model selected") //nolint:staticcheck // pi's exact error message
		s.emitOrPanic(Event{Type: EventCompactionEnd, Reason: CompactionManual, ErrorMessage: err.Error()})
		return nil, err
	}
	result, err := compact.Compact(compactionCtx, preparation, options, customInstructions)
	if err != nil {
		aborted := errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
		var errorMessage string
		if !aborted {
			errorMessage = "Compaction failed: " + err.Error()
		}
		s.emitOrPanic(Event{Type: EventCompactionEnd, Reason: CompactionManual, Aborted: aborted, ErrorMessage: errorMessage})
		return nil, err
	}
	firstKept := result.FirstKeptEntryID
	if _, err := s.sessionManager.AppendCompaction(result.Summary, &firstKept, result.TokensBefore, result.Details, false, result.Usage); err != nil {
		s.emitOrPanic(Event{Type: EventCompactionEnd, Reason: CompactionManual, ErrorMessage: "Compaction failed: " + err.Error()})
		return nil, err
	}
	s.refreshContext()
	estimated := 0
	for _, message := range s.sessionManager.BuildSessionProjection().Messages {
		estimated += wire.EstimateMessageTokens(message)
	}
	result.EstimatedTokensAfter = &estimated
	s.emitOrPanic(Event{Type: EventCompactionEnd, Reason: CompactionManual, Result: &result, Aborted: false, WillRetry: false})
	return &result, nil
}

// AutoCompactionEnabled reports whether auto-compaction is enabled.
func (s *Session) AutoCompactionEnabled() bool { return s.settings.GetCompactionEnabled() }

// SetAutoCompactionEnabled toggles auto-compaction.
func (s *Session) SetAutoCompactionEnabled(enabled bool) { s.settings.SetCompactionEnabled(enabled) }

// AutoRetryEnabled reports whether auto-retry is enabled.
func (s *Session) AutoRetryEnabled() bool { return s.settings.GetRetryEnabled() }

// SetAutoRetryEnabled toggles auto-retry.
func (s *Session) SetAutoRetryEnabled(enabled bool) { s.settings.SetRetryEnabled(enabled) }
