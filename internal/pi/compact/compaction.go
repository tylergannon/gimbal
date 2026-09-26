package compact

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"time"

	"github.com/tylergannon/gimbal/internal/pi/history"
	"github.com/tylergannon/gimbal/internal/pi/model"
	"github.com/tylergannon/gimbal/internal/pi/wire"
)

// Context compaction for long sessions, ported from
// packages/coding-agent/src/core/compaction/compaction.ts (upstream d6af72e1).
// The pure functions live here; the session manager handles I/O and reloads the
// session after it commits a CompactionResult.

// CompactionDetails is the data tracked in CompactionEntry.Details.
type CompactionDetails struct {
	ReadFiles     []string `json:"readFiles"`
	ModifiedFiles []string `json:"modifiedFiles"`
}

// CompactionResult is compact()'s proposal; the session adds uuid/parent when
// saving.
type CompactionResult struct {
	Summary              string
	FirstKeptEntryID     string
	TokensBefore         int
	EstimatedTokensAfter *int
	Usage                *model.Usage
	Details              CompactionDetails
}

// CompactionSettings configures automatic compaction.
type CompactionSettings struct {
	Enabled          bool
	ReserveTokens    int
	KeepRecentTokens int
}

// DefaultCompactionSettings mirrors pi's defaults.
var DefaultCompactionSettings = CompactionSettings{
	Enabled:          true,
	ReserveTokens:    16384,
	KeepRecentTokens: 20000,
}

// CalculateContextTokens returns usage.TotalTokens when reported, else the sum
// of its parts.
func CalculateContextTokens(usage model.Usage) int {
	return wire.CalculateContextTokens(usage)
}

func getAssistantUsage(message model.AgentMessage) (model.Usage, bool) {
	assistant, ok := assistantMessageOf(message)
	if !ok {
		return model.Usage{}, false
	}
	if assistant.StopReason == model.StopAborted || assistant.StopReason == model.StopError {
		return model.Usage{}, false
	}
	if CalculateContextTokens(assistant.Usage) <= 0 {
		return model.Usage{}, false
	}
	return assistant.Usage, true
}

// GetLastAssistantUsage finds the last valid assistant message usage in session
// entries.
func GetLastAssistantUsage(entries []model.SessionEntry) (model.Usage, bool) {
	for _, entrie := range slices.Backward(entries) {
		messageEntry, ok := entrie.(*model.SessionMessageEntry)
		if !ok {
			continue
		}
		if usage, ok := getAssistantUsage(messageEntry.Message); ok {
			return usage, true
		}
	}
	return model.Usage{}, false
}

func getLastAssistantUsageInfo(messages []model.AgentMessage) (model.Usage, int, bool) {
	for i, message := range slices.Backward(messages) {
		if usage, ok := getAssistantUsage(message); ok {
			return usage, i, true
		}
	}
	return model.Usage{}, -1, false
}

// EstimateContextTokens estimates context tokens from messages, anchoring on the
// last assistant usage when one is available and estimating the trailing
// messages otherwise.
func EstimateContextTokens(messages []model.AgentMessage) wire.ContextUsageEstimate {
	usage, index, ok := getLastAssistantUsageInfo(messages)
	if !ok {
		estimated := 0
		for _, message := range messages {
			estimated += wire.EstimateMessageTokens(message)
		}
		return wire.ContextUsageEstimate{Tokens: estimated, TrailingTokens: estimated}
	}

	usageTokens := CalculateContextTokens(usage)
	trailingTokens := 0
	for i := index + 1; i < len(messages); i++ {
		trailingTokens += wire.EstimateMessageTokens(messages[i])
	}
	return wire.ContextUsageEstimate{
		Tokens:         usageTokens + trailingTokens,
		UsageTokens:    usageTokens,
		TrailingTokens: trailingTokens,
		LastUsageIndex: &index,
	}
}

// EstimateProjectedContextTokens estimates projected context without trusting
// usage captured before a later edit or compaction.
func EstimateProjectedContextTokens(projection model.SessionProjection, branchEntries []model.SessionEntry) wire.ContextUsageEstimate {
	estimate := EstimateContextTokens(projection.Messages)
	if estimate.LastUsageIndex != nil {
		projectedMessageIndex := 0
		usageEntryID := ""
		for _, entry := range projection.Entries {
			nextMessageIndex := projectedMessageIndex + len(entry.Messages)
			if *estimate.LastUsageIndex < nextMessageIndex {
				usageEntryID = entry.SourceEntry.Base().ID
				break
			}
			projectedMessageIndex = nextMessageIndex
		}

		usageEntryIndex := -1
		if usageEntryID != "" {
			for i, entry := range branchEntries {
				if entry.Base().ID == usageEntryID {
					usageEntryIndex = i
					break
				}
			}
		}
		latestInvalidatingEntryIndex := -1
		for i, branchEntrie := range slices.Backward(branchEntries) {
			entryType := branchEntrie.EntryType()
			if entryType == "context_edit" || entryType == "compaction" {
				latestInvalidatingEntryIndex = i
				break
			}
		}
		if usageEntryIndex > latestInvalidatingEntryIndex {
			return estimate
		}
	}

	tokens := 0
	if currentSystem, ok := model.GetCurrentSystemMessage(projection.Messages); ok {
		tokens = wire.EstimateMessageTokens(currentSystem)
	}
	for _, message := range projection.Messages {
		if message.MessageRole() != model.RoleSystem {
			tokens += wire.EstimateMessageTokens(message)
		}
	}
	return wire.ContextUsageEstimate{Tokens: tokens, TrailingTokens: tokens}
}

// ShouldCompact reports whether compaction should trigger.
func ShouldCompact(contextTokens, contextWindow int, settings CompactionSettings) bool {
	if !settings.Enabled {
		return false
	}
	return contextTokens > contextWindow-settings.ReserveTokens
}

// CutPointResult is the chosen cut point for a compaction.
type CutPointResult struct {
	// FirstKeptEntryIndex is the index of the first entry to keep.
	FirstKeptEntryIndex int
	// TurnStartIndex is the index of the user message starting the turn being
	// split, or -1 when the cut is not a split.
	TurnStartIndex int
	// IsSplitTurn reports whether the cut lands mid-turn.
	IsSplitTurn bool
}

func isCutPointMessage(message model.AgentMessage) bool {
	switch message.MessageRole() {
	case model.RoleUser, model.RoleAssistant, model.RoleBashExecution, model.RoleCustom,
		model.RoleBranchSummary, model.RoleCompactionSummary:
		return true
	}
	return false
}

func isTurnStartMessage(message model.AgentMessage) bool {
	switch message.MessageRole() {
	case model.RoleUser, model.RoleBashExecution, model.RoleCustom,
		model.RoleBranchSummary, model.RoleCompactionSummary:
		return true
	}
	return false
}

func isTurnStartEntry(entry model.SessionEntry) bool {
	if entry.EntryType() == "compaction" {
		return false
	}
	return slices.ContainsFunc(history.SessionEntryToContextMessages(entry), isTurnStartMessage)
}

// findValidCutPoints returns the indices of context-visible user-like or
// assistant messages. Tool results are never cut points: they must follow their
// tool call.
func findValidCutPoints(entries []model.SessionEntry, startIndex, endIndex int) []int {
	var cutPoints []int
	for i := startIndex; i < endIndex; i++ {
		entry := entries[i]
		if entry.EntryType() == "compaction" {
			continue
		}
		if slices.ContainsFunc(history.SessionEntryToContextMessages(entry), isCutPointMessage) {
			cutPoints = append(cutPoints, i)
		}
	}
	return cutPoints
}

// FindTurnStartIndex finds the context-visible user-role message that starts the
// turn containing entryIndex, or -1 when none precedes the index.
func FindTurnStartIndex(entries []model.SessionEntry, entryIndex, startIndex int) int {
	for i := entryIndex; i >= startIndex; i-- {
		if isTurnStartEntry(entries[i]) {
			return i
		}
	}
	return -1
}

func entryTokens(entry model.SessionEntry) int {
	tokens := 0
	for _, message := range history.SessionEntryToContextMessages(entry) {
		tokens += wire.EstimateMessageTokens(message)
	}
	return tokens
}

// FindCutPoint finds the cut point in session entries that keeps approximately
// keepRecentTokens, considering entries in [startIndex, endIndex).
func FindCutPoint(entries []model.SessionEntry, startIndex, endIndex, keepRecentTokens int) CutPointResult {
	cutPoints := findValidCutPoints(entries, startIndex, endIndex)
	if len(cutPoints) == 0 {
		return CutPointResult{FirstKeptEntryIndex: startIndex, TurnStartIndex: -1}
	}

	accumulatedTokens := 0
	cutIndex := cutPoints[0]
	for i := endIndex - 1; i >= startIndex; i-- {
		messageTokens := entryTokens(entries[i])
		if messageTokens == 0 {
			continue
		}
		accumulatedTokens += messageTokens
		if accumulatedTokens >= keepRecentTokens {
			cutIndex = cutPoints[len(cutPoints)-1]
			for _, candidate := range cutPoints {
				if candidate >= i {
					cutIndex = candidate
					break
				}
			}
			break
		}
	}

	for cutIndex > startIndex {
		previous := entries[cutIndex-1]
		if previous.EntryType() == "compaction" || len(history.SessionEntryToContextMessages(previous)) > 0 {
			break
		}
		cutIndex--
	}

	startsTurn := isTurnStartEntry(entries[cutIndex])
	turnStartIndex := -1
	if !startsTurn {
		turnStartIndex = FindTurnStartIndex(entries, cutIndex, startIndex)
	}
	return CutPointResult{
		FirstKeptEntryIndex: cutIndex,
		TurnStartIndex:      turnStartIndex,
		IsSplitTurn:         !startsTurn && turnStartIndex != -1,
	}
}

// Summarization prompts, byte-for-byte from compaction.ts.
const summarizationPrompt = `The messages above are a conversation to summarize. Create a structured context checkpoint summary that another LLM will use to continue the work.

Use this EXACT format:

## Goal
[What is the user trying to accomplish? Can be multiple items if the session covers different tasks.]

## Constraints & Preferences
- [Any constraints, preferences, or requirements mentioned by user]
- [Or "(none)" if none were mentioned]

## Progress
### Done
- [x] [Completed tasks/changes]

### In Progress
- [ ] [Current work]

### Blocked
- [Issues preventing progress, if any]

## Key Decisions
- **[Decision]**: [Brief rationale]

## Next Steps
1. [Ordered list of what should happen next]

## Critical Context
- [Any data, examples, or references needed to continue]
- [Or "(none)" if not applicable]

Keep each section concise. Preserve exact file paths, function names, and error messages.`

const updateSummarizationInstructions = `Update the existing structured summary with new information. RULES:
- PRESERVE all existing information from the previous summary
- ADD new progress, decisions, and context from the new messages
- UPDATE the Progress section: move items from "In Progress" to "Done" when completed
- UPDATE "Next Steps" based on what was accomplished
- PRESERVE exact file paths, function names, and error messages
- If something is no longer relevant, you may remove it

Use this EXACT format:

## Goal
[Preserve existing goals, add new ones if the task expanded]

## Constraints & Preferences
- [Preserve existing, add new ones discovered]

## Progress
### Done
- [x] [Include previously done items AND newly completed items]

### In Progress
- [ ] [Current work - update based on progress]

### Blocked
- [Current blockers - remove if resolved]

## Key Decisions
- **[Decision]**: [Brief rationale] (preserve all previous, add new)

## Next Steps
1. [Update based on current state]

## Critical Context
- [Preserve important context, add new if needed]

Keep each section concise. Preserve exact file paths, function names, and error messages.`

const updateSummarizationPrompt = `The messages above are NEW conversation messages to incorporate into the existing summary provided in <previous-summary> tags.

` + updateSummarizationInstructions

const turnPrefixSummarizationPrompt = `The messages above are earlier context from an ongoing conversation. Later messages are stored separately and do not need to be reconstructed.

Create a concise checkpoint of the user's request and the progress shown above. This checkpoint will be placed before the later messages so the conversation can continue with the necessary context.

## Original Request
[What did the user ask for?]

## Progress So Far
- [Key decisions and work completed in these messages]

## Context Needed to Continue
- [Information from these messages needed to understand the later work]

Only summarize information explicitly present above. Do not infer or recreate later messages.`

// GetSummarizationFailure returns an error message when a summarization
// response cannot safely be persisted, or "" when it can. A length stop contains
// partial text and must not become a session checkpoint.
func GetSummarizationFailure(response *model.AssistantMessage, label string) string {
	if response.StopReason == model.StopError {
		message := response.ErrorMessage
		if message == "" {
			message = "Unknown error"
		}
		return fmt.Sprintf("%s failed: %s", label, message)
	}
	if response.StopReason == model.StopLength {
		return fmt.Sprintf("%s failed: generation hit the token cap and the summary is incomplete", label)
	}
	return ""
}

// SummaryOptions carries the model and injected provider call used to generate a
// summary. A nil StreamFn is an error.
type SummaryOptions struct {
	Model         *model.Model
	StreamFn      model.StreamFunction
	APIKey        string
	Headers       model.ProviderHeaders
	Env           model.ProviderEnv
	ThinkingLevel model.ThinkingLevel
	Retry         *wire.RetryPolicy
	Callbacks     *wire.RetryCallbacks
	SessionID     string
}

func (o SummaryOptions) streamOptions(maxTokens int) model.SimpleStreamOptions {
	options := model.SimpleStreamOptions{
		APIKey:    o.APIKey,
		Headers:   o.Headers,
		Env:       o.Env,
		MaxTokens: &maxTokens,
		SessionID: o.SessionID,
	}
	if o.Model != nil && o.Model.Reasoning && o.ThinkingLevel != "" && o.ThinkingLevel != model.ThinkingOff {
		options.Reasoning = o.ThinkingLevel
	}
	return options
}

// completeSummarization is the shared choke point for every summarization call,
// wrapping the single LLM call in wire.RetryAssistantCall so transient stream
// drops honor the configured retry policy.
func completeSummarization(ctx context.Context, options SummaryOptions, transcript model.TranscriptContext, requestOptions model.SimpleStreamOptions) (*model.AssistantMessage, error) {
	if options.StreamFn == nil {
		return nil, errors.New("compact: no stream function configured for summarization")
	}
	requestOptions.CacheRetention = model.CacheNone
	if requestOptions.SessionID == "" {
		requestOptions.SessionID = model.UUIDv7()
	}

	var streamErr error
	produce := func() *model.AssistantMessage {
		streamErr = nil
		stream, err := options.StreamFn(ctx, options.Model, transcript, &requestOptions)
		if err != nil {
			streamErr = err
			return &model.AssistantMessage{StopReason: model.StopError, ErrorMessage: err.Error()}
		}
		message, err := drainAssistantMessage(stream)
		if err != nil {
			streamErr = err
			return &model.AssistantMessage{StopReason: model.StopError, ErrorMessage: err.Error()}
		}
		return message
	}

	response := wire.RetryAssistantCall(ctx, produce, options.Retry, options.Callbacks)
	if streamErr != nil {
		return nil, streamErr
	}
	return response, nil
}

func drainAssistantMessage(stream model.AssistantMessageEventChannel) (*model.AssistantMessage, error) {
	var partial *model.AssistantMessage
	for {
		event, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch event.Type {
		case model.EventDone:
			if event.Message != nil {
				return event.Message, nil
			}
		case model.EventError:
			if event.Error != nil {
				return event.Error, nil
			}
		default:
			if event.Partial != nil {
				partial = event.Partial
			}
		}
	}
	if partial != nil {
		return partial, nil
	}
	return nil, errors.New("compact: summarization stream ended without a message")
}

func buildSummarizationContext(promptText string) model.TranscriptContext {
	return model.NormalizeContext(model.Context{
		SystemPrompt: SummarizationSystemPrompt,
		Messages: []model.Message{
			model.NewUserText(promptText, time.Now().UnixMilli()),
		},
	})
}

// SummaryResult is a generated summary and the usage that produced it.
type SummaryResult struct {
	Text  string
	Usage model.Usage
}

// GenerateSummary generates a summary of the conversation, merging into
// previousSummary when it is non-empty.
func GenerateSummary(ctx context.Context, messages []model.AgentMessage, options SummaryOptions, reserveTokens int, customInstructions, previousSummary string) (string, error) {
	result, err := GenerateSummaryWithUsage(ctx, messages, options, reserveTokens, customInstructions, previousSummary)
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

// GenerateSummaryWithUsage generates or updates a conversation summary and
// returns its provider usage.
func GenerateSummaryWithUsage(ctx context.Context, messages []model.AgentMessage, options SummaryOptions, reserveTokens int, customInstructions, previousSummary string) (SummaryResult, error) {
	maxTokens := int(math.Floor(0.8 * float64(reserveTokens)))
	if options.Model != nil && options.Model.MaxTokens > 0 && options.Model.MaxTokens < maxTokens {
		maxTokens = options.Model.MaxTokens
	}

	basePrompt := summarizationPrompt
	if previousSummary != "" {
		basePrompt = updateSummarizationPrompt
	}
	if customInstructions != "" {
		basePrompt = basePrompt + "\n\nAdditional focus: " + customInstructions
	}

	llmMessages := model.ConvertToLlm(messages)
	conversationText := SerializeConversation(llmMessages)

	promptText := "<conversation>\n" + conversationText + "\n</conversation>\n\n"
	if previousSummary != "" {
		promptText += "<previous-summary>\n" + previousSummary + "\n</previous-summary>\n\n"
	}
	promptText += basePrompt

	response, err := completeSummarization(ctx, options, buildSummarizationContext(promptText), options.streamOptions(maxTokens))
	if err != nil {
		return SummaryResult{}, err
	}
	if failure := GetSummarizationFailure(response, "Summarization"); failure != "" {
		return SummaryResult{}, errors.New(failure)
	}
	if hasToolCall(response.Content) {
		return SummaryResult{}, errors.New("summarization attempted to call a tool")
	}

	return SummaryResult{Text: model.ContentText(response.Content), Usage: response.Usage}, nil
}

func hasToolCall(content model.ContentList) bool {
	for _, block := range content {
		if _, ok := block.(model.ToolCall); ok {
			return true
		}
	}
	return false
}

// CompactionPreparation is the pre-calculated input to Compact.
type CompactionPreparation struct {
	// FirstKeptEntryID is the id of the first entry to keep.
	FirstKeptEntryID string
	// MessagesToSummarize are summarized and discarded.
	MessagesToSummarize []model.AgentMessage
	// TurnPrefixMessages become the turn prefix summary when splitting.
	TurnPrefixMessages []model.AgentMessage
	// IsSplitTurn reports whether the cut lands mid-turn.
	IsSplitTurn  bool
	TokensBefore int
	// PreviousSummary is the summary of the previous compaction, when there is
	// one.
	PreviousSummary *string
	// FileOps are the file operations extracted from MessagesToSummarize.
	FileOps *FileOperations
	// Settings are the compaction settings.
	Settings CompactionSettings
}

func isProjectedTurnStart(entry model.ProjectedSessionEntry) bool {
	if entry.SourceEntry.EntryType() == "compaction" {
		return false
	}
	return slices.ContainsFunc(entry.Messages, isTurnStartMessage)
}

func findProjectedTurnStartIndex(entries []model.ProjectedSessionEntry, entryIndex, startIndex int) int {
	for i := entryIndex; i >= startIndex; i-- {
		if isProjectedTurnStart(entries[i]) {
			return i
		}
	}
	return -1
}

func isIntrinsicallyVisible(entry model.ProjectedSessionEntry) bool {
	return entry.SourceEntry.EntryType() != "context_edit" && len(history.SessionEntryToContextMessages(entry.SourceEntry)) > 0
}

func isOmittedProjected(entry model.ProjectedSessionEntry) bool {
	return isIntrinsicallyVisible(entry) && len(entry.Messages) == 0
}

func findProjectedCutPoint(entries []model.ProjectedSessionEntry, startIndex, endIndex, keepRecentTokens int) CutPointResult {
	var cutPoints []int
	for i := startIndex; i < endIndex; i++ {
		entry := entries[i]
		if entry.SourceEntry.EntryType() == "compaction" {
			continue
		}
		if slices.ContainsFunc(entry.Messages, isCutPointMessage) {
			cutPoints = append(cutPoints, i)
		}
	}
	if len(cutPoints) == 0 {
		return CutPointResult{FirstKeptEntryIndex: startIndex, TurnStartIndex: -1}
	}

	accumulatedTokens := 0
	exceededBudget := false
	cutIndex := cutPoints[0]
	for i := endIndex - 1; i >= startIndex; i-- {
		messageTokens := 0
		for _, message := range entries[i].Messages {
			messageTokens += wire.EstimateMessageTokens(message)
		}
		if messageTokens == 0 {
			continue
		}
		accumulatedTokens += messageTokens
		if accumulatedTokens >= keepRecentTokens {
			exceededBudget = true
			cutIndex = cutPoints[len(cutPoints)-1]
			for _, candidate := range cutPoints {
				if candidate >= i {
					cutIndex = candidate
					break
				}
			}
			break
		}
	}

	// A recovery attempt and its omission edits are context-invisible after the
	// last visible input. Advance only for a closed suffix containing an omitted
	// assistant attempt; arbitrary metadata must not move the cut past unsent
	// input.
	suffix := entries[cutIndex+1 : endIndex]
	omittedSuffixIDs := map[string]struct{}{}
	for _, entry := range suffix {
		if isOmittedProjected(entry) {
			omittedSuffixIDs[entry.SourceEntry.Base().ID] = struct{}{}
		}
	}
	hasExternalReplacement := false
	for _, entry := range suffix {
		if entry.SourceEntry.EntryType() != "context_edit" {
			continue
		}
		edit := entry.SourceEntry.(*model.ContextEditEntry)
		if edit.Replacement == nil {
			continue
		}
		if _, omitted := omittedSuffixIDs[edit.TargetID]; !omitted {
			hasExternalReplacement = true
			break
		}
	}
	hasOmittedAssistant := false
	for _, entry := range suffix {
		messageEntry, ok := entry.SourceEntry.(*model.SessionMessageEntry)
		if ok && messageEntry.Message.MessageRole() == model.RoleAssistant && isOmittedProjected(entry) {
			hasOmittedAssistant = true
			break
		}
	}
	allClosedOrOmitted := true
	for _, entry := range suffix {
		if entry.SourceEntry.EntryType() == "compaction" {
			allClosedOrOmitted = false
			break
		}
		if isIntrinsicallyVisible(entry) && !isOmittedProjected(entry) {
			allClosedOrOmitted = false
			break
		}
	}
	isRecoveryOmissionSuffix := exceededBudget && !hasExternalReplacement && hasOmittedAssistant && allClosedOrOmitted
	if isRecoveryOmissionSuffix {
		cutIndex++
	}

	for cutIndex > startIndex {
		previous := entries[cutIndex-1]
		if previous.SourceEntry.EntryType() == "compaction" || len(previous.Messages) > 0 {
			break
		}
		cutIndex--
	}

	startsTurn := isProjectedTurnStart(entries[cutIndex])
	turnStartIndex := -1
	if !startsTurn {
		turnStartIndex = findProjectedTurnStartIndex(entries, cutIndex, startIndex)
	}
	return CutPointResult{
		FirstKeptEntryIndex: cutIndex,
		TurnStartIndex:      turnStartIndex,
		IsSplitTurn:         !startsTurn && turnStartIndex != -1,
	}
}

func getMessagesFromProjectedEntryForCompaction(entry model.ProjectedSessionEntry) []model.AgentMessage {
	if entry.SourceEntry.EntryType() == "compaction" {
		return nil
	}
	out := make([]model.AgentMessage, 0, len(entry.Messages))
	for _, message := range entry.Messages {
		if message.MessageRole() != model.RoleSystem {
			out = append(out, message)
		}
	}
	return out
}

func flatMapProjectedMessages(entries []model.ProjectedSessionEntry) []model.AgentMessage {
	out := make([]model.AgentMessage, 0)
	for _, entry := range entries {
		out = append(out, getMessagesFromProjectedEntryForCompaction(entry)...)
	}
	return out
}

// extractFileOperations merges the previous compaction's details with the tool
// calls in messages.
func extractFileOperations(messages []model.AgentMessage, entries []model.SessionEntry, prevCompactionIndex int) *FileOperations {
	fileOps := NewFileOperations()

	if prevCompactionIndex >= 0 && prevCompactionIndex < len(entries) {
		if previous, ok := entries[prevCompactionIndex].(*model.CompactionEntry); ok && !previous.FromHook {
			readFiles, modifiedFiles := compactionDetailsLists(previous.Details)
			for _, file := range readFiles {
				fileOps.Read[file] = struct{}{}
			}
			for _, file := range modifiedFiles {
				fileOps.Edited[file] = struct{}{}
			}
		}
	}

	for _, message := range messages {
		ExtractFileOpsFromMessage(message, fileOps)
	}
	return fileOps
}

func compactionDetailsLists(details any) (readFiles, modifiedFiles []string) {
	switch typed := details.(type) {
	case CompactionDetails:
		return typed.ReadFiles, typed.ModifiedFiles
	case *CompactionDetails:
		if typed != nil {
			return typed.ReadFiles, typed.ModifiedFiles
		}
	case map[string]any:
		return stringList(typed["readFiles"]), stringList(typed["modifiedFiles"])
	}
	return nil, nil
}

func stringList(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				out = append(out, text)
			}
		}
		return out
	}
	return nil
}

// PrepareCompaction extracts the messages to summarize and the cut point for the
// given session path. It returns false when there is nothing to summarize.
func PrepareCompaction(pathEntries []model.SessionEntry, settings CompactionSettings) (CompactionPreparation, bool) {
	if len(pathEntries) > 0 && pathEntries[len(pathEntries)-1].EntryType() == "compaction" {
		return CompactionPreparation{}, false
	}

	projection := history.BuildSessionProjection(pathEntries, nil, nil)
	projectedEntries := projection.Entries
	sourceEntries := make([]model.SessionEntry, len(projectedEntries))
	for i, entry := range projectedEntries {
		sourceEntries[i] = entry.SourceEntry
	}

	prevCompactionIndex := -1
	for i, entry := range projectedEntries {
		if entry.SourceEntry.EntryType() == "compaction" && len(entry.Messages) > 0 {
			prevCompactionIndex = i
			break
		}
	}

	var previousSummary *string
	boundaryStart := 0
	if prevCompactionIndex >= 0 {
		if previous, ok := projectedEntries[prevCompactionIndex].SourceEntry.(*model.CompactionEntry); ok {
			summary := previous.Summary
			previousSummary = &summary
		}
		boundaryStart = prevCompactionIndex + 1
	}
	boundaryEnd := len(projectedEntries)
	tokensBefore := EstimateProjectedContextTokens(projection, pathEntries).Tokens
	cutPoint := findProjectedCutPoint(projectedEntries, boundaryStart, boundaryEnd, settings.KeepRecentTokens)

	if cutPoint.FirstKeptEntryIndex < 0 || cutPoint.FirstKeptEntryIndex >= len(projectedEntries) {
		return CompactionPreparation{}, false
	}
	firstKeptEntry := projectedEntries[cutPoint.FirstKeptEntryIndex].SourceEntry
	if firstKeptEntry == nil || firstKeptEntry.Base().ID == "" {
		return CompactionPreparation{}, false
	}

	historyEnd := cutPoint.FirstKeptEntryIndex
	if cutPoint.IsSplitTurn {
		historyEnd = cutPoint.TurnStartIndex
	}
	messagesToSummarize := flatMapProjectedMessages(projectedEntries[boundaryStart:historyEnd])
	turnPrefixMessages := []model.AgentMessage{}
	if cutPoint.IsSplitTurn {
		turnPrefixMessages = flatMapProjectedMessages(projectedEntries[cutPoint.TurnStartIndex:cutPoint.FirstKeptEntryIndex])
	}
	if len(messagesToSummarize) == 0 && len(turnPrefixMessages) == 0 {
		return CompactionPreparation{}, false
	}

	fileOps := extractFileOperations(messagesToSummarize, sourceEntries, prevCompactionIndex)
	if cutPoint.IsSplitTurn {
		for _, message := range turnPrefixMessages {
			ExtractFileOpsFromMessage(message, fileOps)
		}
	}

	return CompactionPreparation{
		FirstKeptEntryID:    firstKeptEntry.Base().ID,
		MessagesToSummarize: messagesToSummarize,
		TurnPrefixMessages:  turnPrefixMessages,
		IsSplitTurn:         cutPoint.IsSplitTurn,
		TokensBefore:        tokensBefore,
		PreviousSummary:     previousSummary,
		FileOps:             fileOps,
		Settings:            settings,
	}, true
}

// Compact generates the summaries for a preparation and returns a proposed
// CompactionResult for the session to commit.
func Compact(ctx context.Context, preparation CompactionPreparation, options SummaryOptions, customInstructions string) (CompactionResult, error) {
	var summary string
	var summaryUsage model.Usage

	if preparation.IsSplitTurn && len(preparation.TurnPrefixMessages) > 0 {
		historyText := "No prior history."
		if preparation.PreviousSummary != nil {
			historyText = *preparation.PreviousSummary
		}
		var historyUsage *model.Usage
		if len(preparation.MessagesToSummarize) > 0 {
			previousSummary := ""
			if preparation.PreviousSummary != nil {
				previousSummary = *preparation.PreviousSummary
			}
			result, err := GenerateSummaryWithUsage(ctx, preparation.MessagesToSummarize, options, preparation.Settings.ReserveTokens, customInstructions, previousSummary)
			if err != nil {
				return CompactionResult{}, err
			}
			historyText = result.Text
			usage := result.Usage
			historyUsage = &usage
		}
		turnPrefixResult, err := generateTurnPrefixSummary(ctx, preparation.TurnPrefixMessages, options, preparation.Settings.ReserveTokens)
		if err != nil {
			return CompactionResult{}, err
		}
		summary = historyText + "\n\n---\n\n**Turn Context (split turn):**\n\n" + turnPrefixResult.Text
		if historyUsage != nil {
			summaryUsage = historyUsage.Add(turnPrefixResult.Usage)
		} else {
			summaryUsage = turnPrefixResult.Usage
		}
	} else {
		previousSummary := ""
		if preparation.PreviousSummary != nil {
			previousSummary = *preparation.PreviousSummary
		}
		result, err := GenerateSummaryWithUsage(ctx, preparation.MessagesToSummarize, options, preparation.Settings.ReserveTokens, customInstructions, previousSummary)
		if err != nil {
			return CompactionResult{}, err
		}
		summary = result.Text
		summaryUsage = result.Usage
	}

	readFiles, modifiedFiles := ComputeFileLists(preparation.FileOps)
	summary += FormatFileOperations(readFiles, modifiedFiles)

	if preparation.FirstKeptEntryID == "" {
		return CompactionResult{}, errors.New("first kept entry has no UUID - session may need migration")
	}

	return CompactionResult{
		Summary:          summary,
		FirstKeptEntryID: preparation.FirstKeptEntryID,
		TokensBefore:     preparation.TokensBefore,
		Usage:            &summaryUsage,
		Details:          CompactionDetails{ReadFiles: readFiles, ModifiedFiles: modifiedFiles},
	}, nil
}

func generateTurnPrefixSummary(ctx context.Context, messages []model.AgentMessage, options SummaryOptions, reserveTokens int) (SummaryResult, error) {
	maxTokens := int(math.Floor(0.5 * float64(reserveTokens)))
	if options.Model != nil && options.Model.MaxTokens > 0 && options.Model.MaxTokens < maxTokens {
		maxTokens = options.Model.MaxTokens
	}
	llmMessages := model.ConvertToLlm(messages)
	conversationText := SerializeConversation(llmMessages)
	promptText := "# Conversation\n" + conversationText + "\n\n# Instructions\n" + turnPrefixSummarizationPrompt

	response, err := completeSummarization(ctx, options, buildSummarizationContext(promptText), options.streamOptions(maxTokens))
	if err != nil {
		return SummaryResult{}, err
	}
	if failure := GetSummarizationFailure(response, "Turn prefix summarization"); failure != "" {
		return SummaryResult{}, errors.New(failure)
	}
	if hasToolCall(response.Content) {
		return SummaryResult{}, errors.New("turn prefix summarization attempted to call a tool")
	}
	return SummaryResult{Text: model.ContentText(response.Content), Usage: response.Usage}, nil
}
