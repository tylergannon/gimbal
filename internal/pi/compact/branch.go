package compact

import (
	"context"
	"slices"
	"time"

	"github.com/tylergannon/gimbal/internal/pi/history"
	"github.com/tylergannon/gimbal/internal/pi/model"
	"github.com/tylergannon/gimbal/internal/pi/wire"
)

// Branch summarization for tree navigation, ported from
// packages/coding-agent/src/core/compaction/branch-summarization.ts (upstream
// d6af72e1).

// BranchSummaryResult is the outcome of a branch summarization.
type BranchSummaryResult struct {
	Summary       string
	Usage         *model.Usage
	ReadFiles     []string
	ModifiedFiles []string
	Aborted       bool
	Error         string
}

// BranchSummaryDetails is the data tracked in BranchSummaryEntry.Details.
type BranchSummaryDetails struct {
	ReadFiles     []string `json:"readFiles"`
	ModifiedFiles []string `json:"modifiedFiles"`
}

// BranchPreparation is the messages and file operations selected for a branch
// summary.
type BranchPreparation struct {
	// Messages are extracted for summarization, in chronological order.
	Messages []model.AgentMessage
	// FileOps are the file operations extracted from tool calls.
	FileOps *FileOperations
	// TotalTokens is the total estimated tokens in Messages.
	TotalTokens int
}

// CollectEntriesResult is the entries to summarize and their common ancestor.
type CollectEntriesResult struct {
	// Entries are the entries to summarize, in chronological order.
	Entries []model.SessionEntry
	// CommonAncestorID is the common ancestor between old and new position.
	CommonAncestorID *string
}

// BranchSession is the read-only session view branch collection needs.
type BranchSession interface {
	GetBranch(fromID ...string) []model.SessionEntry
	GetEntry(id string) model.SessionEntry
}

// GenerateBranchSummaryOptions configures a branch summary.
type GenerateBranchSummaryOptions struct {
	SummaryOptions
	// CustomInstructions are optional instructions for summarization.
	CustomInstructions string
	// ReplaceInstructions makes CustomInstructions replace the default prompt
	// instead of being appended.
	ReplaceInstructions bool
	// ReserveTokens is reserved when selecting branch history.
	ReserveTokens int
}

// CollectEntriesForBranchSummary collects the entries to summarize when
// navigating from oldLeafID back to the common ancestor with targetID.
func CollectEntriesForBranchSummary(session BranchSession, oldLeafID *string, targetID string) CollectEntriesResult {
	if oldLeafID == nil {
		return CollectEntriesResult{}
	}

	oldPath := map[string]struct{}{}
	for _, entry := range session.GetBranch(*oldLeafID) {
		oldPath[entry.Base().ID] = struct{}{}
	}
	targetPath := session.GetBranch(targetID)

	var commonAncestorID *string
	for _, t := range slices.Backward(targetPath) {
		if _, ok := oldPath[t.Base().ID]; ok {
			id := t.Base().ID
			commonAncestorID = &id
			break
		}
	}

	var entries []model.SessionEntry
	current := oldLeafID
	for current != nil && (commonAncestorID == nil || *current != *commonAncestorID) {
		entry := session.GetEntry(*current)
		if entry == nil {
			break
		}
		entries = append(entries, entry)
		current = entry.Base().ParentID
	}
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}

	return CollectEntriesResult{Entries: entries, CommonAncestorID: commonAncestorID}
}

// getMessageFromEntry extracts a context-visible message from a session entry.
// Tool results are skipped: their context is in the assistant's tool call.
func getMessageFromEntry(entry model.SessionEntry) (model.AgentMessage, bool) {
	messages := history.SessionEntryToContextMessages(entry)
	for _, message := range slices.Backward(messages) {
		if message.MessageRole() == model.RoleToolResult {
			continue
		}
		return message, true
	}
	return nil, false
}

// PrepareBranchEntries selects the most recent messages within tokenBudget.
// A tokenBudget of 0 means no limit. File operations are collected from every
// entry, even those that do not fit, so cumulative tracking survives budget
// truncation.
func PrepareBranchEntries(entries []model.SessionEntry, tokenBudget int) BranchPreparation {
	var messages []model.AgentMessage
	fileOps := NewFileOperations()
	totalTokens := 0

	for _, entry := range entries {
		branchSummary, ok := entry.(*model.BranchSummaryEntry)
		if !ok || branchSummary.FromHook {
			continue
		}
		readFiles, modifiedFiles := branchSummaryDetailsLists(branchSummary.Details)
		for _, file := range readFiles {
			fileOps.Read[file] = struct{}{}
		}
		for _, file := range modifiedFiles {
			fileOps.Edited[file] = struct{}{}
		}
	}

	for _, entry := range slices.Backward(entries) {

		message, ok := getMessageFromEntry(entry)
		if !ok {
			continue
		}
		ExtractFileOpsFromMessage(message, fileOps)

		tokens := wire.EstimateMessageTokens(message)
		if tokenBudget > 0 && totalTokens+tokens > tokenBudget {
			entryType := entry.EntryType()
			if entryType == "compaction" || entryType == "branch_summary" {
				if float64(totalTokens) < float64(tokenBudget)*0.9 {
					messages = append([]model.AgentMessage{message}, messages...)
					totalTokens += tokens
				}
			}
			break
		}
		messages = append([]model.AgentMessage{message}, messages...)
		totalTokens += tokens
	}

	return BranchPreparation{Messages: messages, FileOps: fileOps, TotalTokens: totalTokens}
}

func branchSummaryDetailsLists(details any) (readFiles, modifiedFiles []string) {
	switch typed := details.(type) {
	case BranchSummaryDetails:
		return typed.ReadFiles, typed.ModifiedFiles
	case *BranchSummaryDetails:
		if typed != nil {
			return typed.ReadFiles, typed.ModifiedFiles
		}
	case map[string]any:
		return stringList(typed["readFiles"]), stringList(typed["modifiedFiles"])
	}
	return nil, nil
}

// Branch summary prompts, byte-for-byte from branch-summarization.ts.
const branchSummaryPreamble = `The user explored a different conversation branch before returning here.
Summary of that exploration:

`

const branchSummaryPrompt = `Create a structured summary of this conversation branch for context when returning later.

Use this EXACT format:

## Goal
[What was the user trying to accomplish in this branch?]

## Constraints & Preferences
- [Any constraints, preferences, or requirements mentioned]
- [Or "(none)" if none were mentioned]

## Progress
### Done
- [x] [Completed tasks/changes]

### In Progress
- [ ] [Work that was started but not finished]

### Blocked
- [Issues preventing progress, if any]

## Key Decisions
- **[Decision]**: [Brief rationale]

## Next Steps
1. [What should happen next to continue this work]

Keep each section concise. Preserve exact file paths, function names, and error messages.`

// GenerateBranchSummary summarizes the abandoned branch entries.
func GenerateBranchSummary(ctx context.Context, entries []model.SessionEntry, options GenerateBranchSummaryOptions) BranchSummaryResult {
	reserveTokens := options.ReserveTokens
	if reserveTokens == 0 {
		reserveTokens = 16384
	}

	contextWindow := 128000
	if options.Model != nil && options.Model.ContextWindow != 0 {
		contextWindow = options.Model.ContextWindow
	}
	tokenBudget := contextWindow - reserveTokens

	preparation := PrepareBranchEntries(entries, tokenBudget)
	if len(preparation.Messages) == 0 {
		return BranchSummaryResult{Summary: "No content to summarize"}
	}

	llmMessages := model.ConvertToLlm(preparation.Messages)
	conversationText := SerializeConversation(llmMessages)

	instructions := branchSummaryPrompt
	if options.ReplaceInstructions && options.CustomInstructions != "" {
		instructions = options.CustomInstructions
	} else if options.CustomInstructions != "" {
		instructions = branchSummaryPrompt + "\n\nAdditional focus: " + options.CustomInstructions
	}
	promptText := "<conversation>\n" + conversationText + "\n</conversation>\n\n" + instructions

	summarizationMessages := []model.Message{
		model.NewUserText(promptText, time.Now().UnixMilli()),
	}

	maxTokens := 4096
	if options.Model != nil && options.Model.MaxTokens > 0 && options.Model.MaxTokens < maxTokens {
		maxTokens = options.Model.MaxTokens
	}

	transcript := model.NormalizeContext(model.Context{SystemPrompt: SummarizationSystemPrompt, Messages: summarizationMessages})
	streamOptions := model.SimpleStreamOptions{
		APIKey:    options.APIKey,
		Headers:   options.Headers,
		Env:       options.Env,
		MaxTokens: &maxTokens,
	}

	response, err := completeSummarization(ctx, options.SummaryOptions, transcript, streamOptions)
	if err != nil {
		return BranchSummaryResult{Error: err.Error()}
	}
	if response.StopReason == model.StopAborted {
		return BranchSummaryResult{Aborted: true}
	}
	if failure := GetSummarizationFailure(response, "Branch summarization"); failure != "" {
		return BranchSummaryResult{Error: failure}
	}
	if hasToolCall(response.Content) {
		return BranchSummaryResult{Error: "Branch summarization attempted to call a tool"}
	}

	summary := branchSummaryPreamble + model.ContentText(response.Content)
	readFiles, modifiedFiles := ComputeFileLists(preparation.FileOps)
	summary += FormatFileOperations(readFiles, modifiedFiles)
	if summary == "" {
		summary = "No summary generated"
	}

	return BranchSummaryResult{
		Summary:       summary,
		Usage:         &response.Usage,
		ReadFiles:     readFiles,
		ModifiedFiles: modifiedFiles,
	}
}
