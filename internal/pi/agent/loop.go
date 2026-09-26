package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"sync"

	"github.com/tylergannon/gimbal/internal/pi/model"
	"github.com/tylergannon/gimbal/internal/pi/wire"
)

// This file ports packages/agent/src/agent-loop.ts. The loop works on
// AgentMessage throughout and converts to provider messages only at the stream
// boundary. Context cancellation replaces pi's AbortSignal, and the provider
// stream function is injected explicitly.

// StreamFunction is the provider stream contract the loop calls. It is
// model.StreamFunction; the alias keeps call sites concise.
type StreamFunction = model.StreamFunction

type emitPanic struct{ err error }

func mustEmit(emit model.EventSink, event model.AgentEvent) {
	if err := emit(event); err != nil {
		panic(emitPanic{err: err})
	}
}

func aborted(ctx context.Context) bool {
	return ctx != nil && ctx.Err() != nil
}

func nowMillis() int64 { return timeNowMillis() }

// AgentLoop starts an agent loop with new prompt messages and returns a stream
// of AgentEvents whose final result is the new messages the run produced.
func AgentLoop(ctx context.Context, prompts []model.AgentMessage, agentCtx model.AgentContext, config model.AgentLoopConfig, streamFn StreamFunction) *wire.EventStream[model.AgentEvent, []model.AgentMessage] {
	stream := newAgentStream()
	go func() {
		defer func() { _ = recover() }()
		messages, err := RunAgentLoop(ctx, prompts, agentCtx, config, func(event model.AgentEvent) error {
			stream.Push(event)
			return nil
		}, streamFn)
		if err != nil {
			stream.End()
			return
		}
		stream.End(messages)
	}()
	return stream
}

// AgentLoopContinue continues from the current context without adding a new
// message. The last message must convert to a user or tool-result message.
func AgentLoopContinue(ctx context.Context, agentCtx model.AgentContext, config model.AgentLoopConfig, streamFn StreamFunction) (*wire.EventStream[model.AgentEvent, []model.AgentMessage], error) {
	if len(agentCtx.Messages) == 0 {
		return nil, errors.New("Cannot continue: no messages in context") //nolint:staticcheck // pi's exact error message
	}
	if agentCtx.Messages[len(agentCtx.Messages)-1].MessageRole() == model.RoleAssistant {
		return nil, errors.New("Cannot continue from message role: assistant") //nolint:staticcheck // pi's exact error message
	}
	stream := newAgentStream()
	go func() {
		defer func() { _ = recover() }()
		messages, err := RunAgentLoopContinue(ctx, agentCtx, config, func(event model.AgentEvent) error {
			stream.Push(event)
			return nil
		}, streamFn)
		if err != nil {
			stream.End()
			return
		}
		stream.End(messages)
	}()
	return stream, nil
}

func newAgentStream() *wire.EventStream[model.AgentEvent, []model.AgentMessage] {
	return wire.NewEventStream(
		func(event model.AgentEvent) bool { return event.Type == model.EvAgentEnd },
		func(event model.AgentEvent) []model.AgentMessage {
			if event.Type == model.EvAgentEnd {
				return event.Messages
			}
			return nil
		},
	)
}

// RunAgentLoop adds prompts to the context and runs the loop to completion,
// returning the messages produced during the run.
func RunAgentLoop(ctx context.Context, prompts []model.AgentMessage, agentCtx model.AgentContext, config model.AgentLoopConfig, emit model.EventSink, streamFn StreamFunction) (messages []model.AgentMessage, err error) {
	defer func() {
		if r := recover(); r != nil {
			if ep, ok := r.(emitPanic); ok {
				err = ep.err
				return
			}
			panic(r)
		}
	}()

	initialMessages := declareToolChanges(agentCtx, prompts)
	newMessages := make([]model.AgentMessage, len(initialMessages))
	copy(newMessages, initialMessages)
	current := agentCtx
	current.Messages = append(append([]model.AgentMessage(nil), agentCtx.Messages...), initialMessages...)

	mustEmit(emit, model.AgentEvent{Type: model.EvAgentStart})
	mustEmit(emit, model.AgentEvent{Type: model.EvTurnStart})
	for _, message := range initialMessages {
		mustEmit(emit, model.AgentEvent{Type: model.EvMessageStart, Message: message})
		mustEmit(emit, model.AgentEvent{Type: model.EvMessageEnd, Message: message})
	}

	if err := runLoop(ctx, &current, &newMessages, config, emit, streamFn); err != nil {
		return newMessages, err
	}
	return newMessages, nil
}

// RunAgentLoopContinue continues from an existing context, returning only the
// messages produced after the existing transcript.
func RunAgentLoopContinue(ctx context.Context, agentCtx model.AgentContext, config model.AgentLoopConfig, emit model.EventSink, streamFn StreamFunction) (messages []model.AgentMessage, err error) {
	defer func() {
		if r := recover(); r != nil {
			if ep, ok := r.(emitPanic); ok {
				err = ep.err
				return
			}
			panic(r)
		}
	}()

	newMessages := []model.AgentMessage{}
	current := agentCtx

	mustEmit(emit, model.AgentEvent{Type: model.EvAgentStart})
	mustEmit(emit, model.AgentEvent{Type: model.EvTurnStart})

	if err := runLoop(ctx, &current, &newMessages, config, emit, streamFn); err != nil {
		return newMessages, err
	}
	return newMessages, nil
}

func runLoop(ctx context.Context, current *model.AgentContext, newMessages *[]model.AgentMessage, config model.AgentLoopConfig, emit model.EventSink, streamFn StreamFunction) error {
	var lastCompletedTurn *model.AgentTurnContext
	explicitContinuation := false
	var pending []model.AgentMessage
	if config.GetSteeringMessages != nil {
		pending = config.GetSteeringMessages()
	}

	for {
		hasMoreToolCalls := true

		for hasMoreToolCalls || len(pending) > 0 {
			var prepared []model.AgentMessage
			if lastCompletedTurn != nil {
				if config.PrepareNextTurn != nil {
					if update := config.PrepareNextTurn(*lastCompletedTurn); update != nil {
						prepared = applyTurnUpdate(update, current, &config)
					}
				}
				if len(pending) == 0 && config.GetSteeringMessages != nil {
					pending = config.GetSteeringMessages()
				}
				mustEmit(emit, model.AgentEvent{Type: model.EvTurnStart})
			}

			for _, message := range declareToolChanges(*current, slices.Concat(prepared, pending)) {
				mustEmit(emit, model.AgentEvent{Type: model.EvMessageStart, Message: message})
				mustEmit(emit, model.AgentEvent{Type: model.EvMessageEnd, Message: message})
				current.Messages = append(current.Messages, message)
				*newMessages = append(*newMessages, message)
			}

			if config.PrepareRequest != nil {
				level := config.Reasoning
				if level == "" {
					level = model.ThinkingOff
				}
				if update := config.PrepareRequest(ctx, model.PrepareRequestContext{Context: current, Model: config.Model, ThinkingLevel: level}); update != nil {
					applyRequestUpdate(update, current, &config)
				}
			}

			message, err := streamAssistantResponse(ctx, current, config, emit, streamFn)
			if err != nil {
				return err
			}
			*newMessages = append(*newMessages, message)

			if message.StopReason == model.StopError || message.StopReason == model.StopAborted {
				lastCompletedTurn = &model.AgentTurnContext{
					Message:     message,
					ToolResults: []model.ToolResultMessage{},
					Context:     current,
					NewMessages: *newMessages,
				}
				if config.FinishTurn != nil {
					config.FinishTurn(ctx, *lastCompletedTurn)
				}
				mustEmit(emit, model.AgentEvent{Type: model.EvTurnEnd, Message: message, ToolResults: []model.ToolResultMessage{}})
				mustEmit(emit, model.AgentEvent{Type: model.EvAgentEnd, Messages: *newMessages})
				return nil
			}

			toolCalls := filterToolCalls(message)
			toolResults := []model.ToolResultMessage{}
			hasMoreToolCalls = false
			if len(toolCalls) > 0 {
				var batch executedBatch
				if message.StopReason == model.StopLength {
					batch = failToolCallsFromTruncatedMessage(toolCalls, emit)
				} else {
					batch = executeToolCalls(ctx, current, message, config, emit)
				}
				toolResults = append(toolResults, batch.messages...)
				hasMoreToolCalls = !batch.terminate
				for _, result := range toolResults {
					current.Messages = append(current.Messages, result)
					*newMessages = append(*newMessages, result)
				}
			}

			lastCompletedTurn = &model.AgentTurnContext{
				Message:     message,
				ToolResults: toolResults,
				Context:     current,
				NewMessages: *newMessages,
			}
			decision := model.AgentTurnDecision("")
			if config.FinishTurn != nil {
				decision = config.FinishTurn(ctx, *lastCompletedTurn)
			}
			mustEmit(emit, model.AgentEvent{Type: model.EvTurnEnd, Message: message, ToolResults: toolResults})

			if decision == model.TurnEnd {
				mustEmit(emit, model.AgentEvent{Type: model.EvAgentEnd, Messages: *newMessages})
				return nil
			}

			explicitContinuation = decision == model.TurnContinue
			if config.GetSteeringMessages != nil {
				pending = config.GetSteeringMessages()
			} else {
				pending = nil
			}
			if hasMoreToolCalls || len(pending) > 0 {
				explicitContinuation = false
			}
		}

		var followUps []model.AgentMessage
		if config.GetFollowUpMessages != nil {
			followUps = config.GetFollowUpMessages()
		}
		if len(followUps) > 0 {
			explicitContinuation = false
			pending = followUps
			continue
		}
		if explicitContinuation {
			explicitContinuation = false
			continue
		}
		break
	}

	mustEmit(emit, model.AgentEvent{Type: model.EvAgentEnd, Messages: *newMessages})
	return nil
}

func applyRequestUpdate(update *model.AgentRequestUpdate, current *model.AgentContext, config *model.AgentLoopConfig) {
	if update == nil {
		return
	}
	if update.Context != nil {
		*current = *update.Context
	}
	if update.Model != nil {
		config.Model = update.Model
	}
	if update.ThinkingLevel != nil {
		if *update.ThinkingLevel == model.ThinkingOff {
			config.Reasoning = ""
		} else {
			config.Reasoning = *update.ThinkingLevel
		}
	}
}

func applyTurnUpdate(update *model.AgentLoopTurnUpdate, current *model.AgentContext, config *model.AgentLoopConfig) []model.AgentMessage {
	if update == nil {
		return nil
	}
	applyRequestUpdate(&model.AgentRequestUpdate{
		Context:       update.Context,
		Model:         update.Model,
		ThinkingLevel: update.ThinkingLevel,
	}, current, config)
	return update.Messages
}

func filterToolCalls(message *model.AssistantMessage) []model.ToolCall {
	var out []model.ToolCall
	for _, content := range message.Content {
		if toolCall, ok := content.(model.ToolCall); ok {
			out = append(out, toolCall)
		}
	}
	return out
}

func streamAssistantResponse(ctx context.Context, current *model.AgentContext, config model.AgentLoopConfig, emit model.EventSink, streamFn StreamFunction) (*model.AssistantMessage, error) {
	messages := current.Messages
	if config.TransformContext != nil {
		messages = config.TransformContext(ctx, messages)
	}

	var llmMessages []model.Message
	if config.ConvertToLlm != nil {
		llmMessages = config.ConvertToLlm(messages)
	} else {
		llmMessages = defaultConvertToLlm(messages)
	}
	llmCtx := model.NormalizeContext(model.Context{Messages: llmMessages})

	if streamFn == nil {
		return nil, errors.New("agent: no stream function configured")
	}

	options := config.SimpleStreamOptions
	options.Reasoning = config.Reasoning
	apiKey := config.APIKey
	if config.GetApiKey != nil {
		if key := config.GetApiKey(config.Model.Provider); key != "" {
			apiKey = key
		}
	}
	options.APIKey = apiKey

	response, err := streamFn(ctx, config.Model, llmCtx, &options)
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, errors.New("agent: stream function returned no stream")
	}

	var partial *model.AssistantMessage
	addedPartial := false

	for {
		event, recvErr := response.Recv()
		if recvErr != nil {
			if errors.Is(recvErr, io.EOF) {
				break
			}
			_ = response.Close()
			return nil, recvErr
		}
		switch event.Type {
		case model.EventStart:
			partial = event.Partial
			if partial == nil {
				continue
			}
			current.Messages = append(current.Messages, partial)
			addedPartial = true
			mustEmit(emit, model.AgentEvent{Type: model.EvMessageStart, Message: model.CloneMessage(partial)})

		case model.EventTextStart, model.EventTextDelta, model.EventTextEnd,
			model.EventThinkingStart, model.EventThinkingDelta, model.EventThinkingEnd,
			model.EventToolCallStart, model.EventToolCallDelta, model.EventToolCallEnd:
			if partial != nil {
				partial = event.Partial
				current.Messages[len(current.Messages)-1] = partial
				eventCopy := event
				mustEmit(emit, model.AgentEvent{
					Type:                  model.EvMessageUpdate,
					Message:               model.CloneMessage(partial),
					AssistantMessageEvent: &eventCopy,
				})
			}

		case model.EventDone, model.EventError:
			final := finalMessageFromEvent(event)
			if final == nil {
				_ = response.Close()
				return nil, errors.New("agent: terminal stream event carried no message")
			}
			if addedPartial {
				current.Messages[len(current.Messages)-1] = final
			} else {
				current.Messages = append(current.Messages, final)
				mustEmit(emit, model.AgentEvent{Type: model.EvMessageStart, Message: model.CloneMessage(final)})
			}
			mustEmit(emit, model.AgentEvent{Type: model.EvMessageEnd, Message: final})
			_ = response.Close()
			return final, nil
		}
	}

	final := partial
	if final == nil {
		_ = response.Close()
		return nil, errors.New("agent: stream ended without a message")
	}
	if addedPartial {
		current.Messages[len(current.Messages)-1] = final
	} else {
		current.Messages = append(current.Messages, final)
		mustEmit(emit, model.AgentEvent{Type: model.EvMessageStart, Message: model.CloneMessage(final)})
	}
	mustEmit(emit, model.AgentEvent{Type: model.EvMessageEnd, Message: final})
	_ = response.Close()
	return final, nil
}

func finalMessageFromEvent(event model.AssistantMessageEvent) *model.AssistantMessage {
	if event.Type == model.EventDone {
		return event.Message
	}
	if event.Type == model.EventError {
		return event.Error
	}
	return nil
}

func declareToolChanges(agentCtx model.AgentContext, pending []model.AgentMessage) []model.AgentMessage {
	systemIndex := -1
	var system model.SystemMessage
	for i, p := range slices.Backward(pending) {
		if message, ok := systemMessageOf(p); ok {
			systemIndex, system = i, message
			break
		}
	}

	baseline := pending
	if systemIndex >= 0 {
		baseline = slices.Clone(pending)
		baseline[systemIndex] = withToolChanges(system, model.ToolStateChanges{})
	}

	executable := make([]model.Tool, len(agentCtx.Tools))
	for i, tool := range agentCtx.Tools {
		executable[i] = model.ToToolDeclaration(tool.AsTool())
	}
	changes := model.GetToolStateChanges(
		model.GetCurrentTools(slices.Concat(agentCtx.Messages, baseline)),
		executable,
	)
	unchanged := len(changes.ToolsAdded) == 0 && len(changes.ToolsRemoved) == 0

	if systemIndex >= 0 {
		if unchanged && len(system.ToolsAdded) == 0 && len(system.ToolsRemoved) == 0 {
			return pending
		}
		baseline[systemIndex] = withToolChanges(system, changes)
		return baseline
	}
	if unchanged {
		return pending
	}
	update := withToolChanges(model.NewSystemText("", nowMillis()), changes)
	index := slices.IndexFunc(pending, func(message model.AgentMessage) bool {
		return message.MessageRole() != model.RoleSystem
	})
	if index == -1 {
		index = len(pending)
	}
	return slices.Concat(pending[:index], []model.AgentMessage{update}, pending[index:])
}

func withToolChanges(message model.SystemMessage, changes model.ToolStateChanges) model.SystemMessage {
	out := message
	out.ToolsAdded = changes.ToolsAdded
	out.ToolsRemoved = changes.ToolsRemoved
	return out
}

func systemMessageOf(message model.AgentMessage) (model.SystemMessage, bool) {
	switch m := message.(type) {
	case model.SystemMessage:
		return m, true
	case *model.SystemMessage:
		if m != nil {
			return *m, true
		}
	}
	return model.SystemMessage{}, false
}

func defaultConvertToLlm(messages []model.AgentMessage) []model.Message {
	var out []model.Message
	for _, message := range messages {
		switch message.MessageRole() {
		case model.RoleSystem, model.RoleUser, model.RoleAssistant, model.RoleToolResult:
			out = append(out, message)
		}
	}
	return out
}

type executedBatch struct {
	messages  []model.ToolResultMessage
	terminate bool
}

func findTool(tools []model.AgentTool, name string) (model.AgentTool, bool) {
	for _, tool := range tools {
		if tool.Name == name {
			return tool, true
		}
	}
	return model.AgentTool{}, false
}

func failToolCallsFromTruncatedMessage(toolCalls []model.ToolCall, emit model.EventSink) executedBatch {
	messages := make([]model.ToolResultMessage, 0, len(toolCalls))
	for _, toolCall := range toolCalls {
		mustEmit(emit, model.AgentEvent{Type: model.EvToolExecutionStart, ToolCallID: toolCall.ID, ToolName: toolCall.Name, Args: toolCall.Arguments})
		finalized := finalizedOutcome{
			toolCall: toolCall,
			result: errorToolResult(fmt.Sprintf(
				`Tool call "%s" was not executed: the response hit the output token limit, so its arguments may be truncated. Re-issue the tool call with complete arguments.`,
				toolCall.Name,
			)),
			isError: true,
		}
		emitToolExecutionEnd(finalized, emit)
		toolResult := createToolResultMessage(finalized)
		emitToolResultMessage(toolResult, emit)
		messages = append(messages, toolResult)
	}
	return executedBatch{messages: messages}
}

func executeToolCalls(ctx context.Context, current *model.AgentContext, message *model.AssistantMessage, config model.AgentLoopConfig, emit model.EventSink) executedBatch {
	toolCalls := filterToolCalls(message)
	hasSequential := false
	for _, toolCall := range toolCalls {
		if tool, ok := findTool(current.Tools, toolCall.Name); ok && tool.ExecutionMode == model.ToolSequential {
			hasSequential = true
			break
		}
	}
	if config.ToolExecution == model.ToolSequential || hasSequential {
		return executeToolCallsSequential(ctx, current, message, toolCalls, config, emit)
	}
	return executeToolCallsParallel(ctx, current, message, toolCalls, config, emit)
}

type finalizedOutcome struct {
	toolCall model.ToolCall
	result   model.AgentToolResult
	isError  bool
}

func shouldTerminateBatch(calls []finalizedOutcome) bool {
	if len(calls) == 0 {
		return false
	}
	for _, call := range calls {
		if !call.result.Terminate {
			return false
		}
	}
	return true
}

func executeToolCallsSequential(ctx context.Context, current *model.AgentContext, message *model.AssistantMessage, toolCalls []model.ToolCall, config model.AgentLoopConfig, emit model.EventSink) executedBatch {
	var finalized []finalizedOutcome
	var messages []model.ToolResultMessage

	for _, toolCall := range toolCalls {
		mustEmit(emit, model.AgentEvent{Type: model.EvToolExecutionStart, ToolCallID: toolCall.ID, ToolName: toolCall.Name, Args: toolCall.Arguments})

		preparation := prepareToolCall(ctx, current, message, toolCall, config)
		var outcome finalizedOutcome
		if preparation.immediate != nil {
			outcome = finalizedOutcome{toolCall: toolCall, result: preparation.immediate.result, isError: preparation.immediate.isError}
		} else {
			executed := executePreparedToolCall(ctx, *preparation.prepared, emit)
			outcome = finalizeExecutedToolCall(ctx, current, message, *preparation.prepared, executed, config)
		}

		emitToolExecutionEnd(outcome, emit)
		toolResult := createToolResultMessage(outcome)
		emitToolResultMessage(toolResult, emit)
		finalized = append(finalized, outcome)
		messages = append(messages, toolResult)

		if aborted(ctx) {
			break
		}
	}

	return executedBatch{messages: messages, terminate: shouldTerminateBatch(finalized)}
}

func executeToolCallsParallel(ctx context.Context, current *model.AgentContext, message *model.AssistantMessage, toolCalls []model.ToolCall, config model.AgentLoopConfig, emit model.EventSink) executedBatch {
	var serialMu sync.Mutex
	safeEmit := func(event model.AgentEvent) error {
		serialMu.Lock()
		defer serialMu.Unlock()
		return emit(event)
	}

	type slot struct {
		immediate *finalizedOutcome
		toolCall  model.ToolCall
		thunk     func() finalizedOutcome
	}
	slots := make([]slot, 0, len(toolCalls))

	for _, toolCall := range toolCalls {
		mustEmit(safeEmit, model.AgentEvent{Type: model.EvToolExecutionStart, ToolCallID: toolCall.ID, ToolName: toolCall.Name, Args: toolCall.Arguments})

		preparation := prepareToolCall(ctx, current, message, toolCall, config)
		if preparation.immediate != nil {
			outcome := finalizedOutcome{toolCall: toolCall, result: preparation.immediate.result, isError: preparation.immediate.isError}
			emitToolExecutionEnd(outcome, safeEmit)
			slots = append(slots, slot{immediate: &outcome})
			if aborted(ctx) {
				break
			}
			continue
		}
		prepared := *preparation.prepared
		slots = append(slots, slot{toolCall: prepared.toolCall, thunk: func() finalizedOutcome {
			executed := executePreparedToolCall(ctx, prepared, safeEmit)
			var outcome finalizedOutcome
			var emitErr error
			func() {
				serialMu.Lock()
				defer serialMu.Unlock()
				outcome = finalizeExecutedToolCall(ctx, current, message, prepared, executed, config)
				emitErr = emit(model.AgentEvent{
					Type:       model.EvToolExecutionEnd,
					ToolCallID: outcome.toolCall.ID,
					ToolName:   outcome.toolCall.Name,
					Result:     outcome.result,
					IsError:    outcome.isError,
				})
			}()
			if emitErr != nil {
				panic(emitPanic{err: emitErr})
			}
			return outcome
		}})
		if aborted(ctx) {
			break
		}
	}

	ordered := make([]finalizedOutcome, len(slots))
	var wg sync.WaitGroup
	var panicOnce sync.Once
	var panicVal any
	batchAborted := aborted(ctx)
	for i, s := range slots {
		if s.immediate != nil {
			ordered[i] = *s.immediate
			continue
		}
		if batchAborted {
			outcome := finalizedOutcome{toolCall: s.toolCall, result: errorToolResult("Operation aborted"), isError: true}
			func() {
				defer func() {
					if r := recover(); r != nil {
						panicOnce.Do(func() { panicVal = r })
					}
				}()
				emitToolExecutionEnd(outcome, safeEmit)
			}()
			ordered[i] = outcome
			continue
		}
		wg.Add(1)
		go func(i int, thunk func() finalizedOutcome) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicOnce.Do(func() { panicVal = r })
				}
			}()
			ordered[i] = thunk()
		}(i, s.thunk)
	}
	wg.Wait()
	if panicVal != nil {
		panic(panicVal)
	}

	var messages []model.ToolResultMessage
	for _, outcome := range ordered {
		toolResult := createToolResultMessage(outcome)
		emitToolResultMessage(toolResult, emit)
		messages = append(messages, toolResult)
	}
	return executedBatch{messages: messages, terminate: shouldTerminateBatch(ordered)}
}

type immediateOutcome struct {
	result  model.AgentToolResult
	isError bool
}

type preparedToolCall struct {
	toolCall model.ToolCall
	tool     model.AgentTool
	args     map[string]any
}

type prepareResult struct {
	immediate *immediateOutcome
	prepared  *preparedToolCall
}

func prepareToolCall(ctx context.Context, current *model.AgentContext, message *model.AssistantMessage, toolCall model.ToolCall, config model.AgentLoopConfig) (result prepareResult) {
	tool, ok := findTool(current.Tools, toolCall.Name)
	if !ok {
		return prepareResult{immediate: &immediateOutcome{result: errorToolResult("Tool " + toolCall.Name + " not found"), isError: true}}
	}

	defer func() {
		if r := recover(); r != nil {
			if ep, ok := r.(emitPanic); ok {
				panic(ep)
			}
			result = prepareResult{immediate: &immediateOutcome{result: errorToolResult(panicMessage(r)), isError: true}}
		}
	}()

	prepared := toolCall
	if tool.PrepareArguments != nil {
		if newArgs := tool.PrepareArguments(toolCall.Arguments); newArgs != nil {
			prepared = model.ToolCall{ID: toolCall.ID, Name: toolCall.Name, Arguments: newArgs, ThoughtSignature: toolCall.ThoughtSignature, Namespace: toolCall.Namespace}
		}
	}

	validated, err := ValidateToolArguments(tool.AsTool(), prepared)
	if err != nil {
		return prepareResult{immediate: &immediateOutcome{result: errorToolResult(err.Error()), isError: true}}
	}

	if config.BeforeToolCall != nil {
		before := config.BeforeToolCall(ctx, model.BeforeToolCallContext{
			AssistantMessage: message,
			ToolCall:         toolCall,
			Args:             validated,
			Context:          current,
		})
		if aborted(ctx) {
			return prepareResult{immediate: &immediateOutcome{result: errorToolResult("Operation aborted"), isError: true}}
		}
		if before != nil && before.Block {
			reason := before.Reason
			if reason == "" {
				reason = "Tool execution was blocked"
			}
			toolResult := errorToolResult(reason)
			toolResult.Terminate = before.Terminate
			return prepareResult{immediate: &immediateOutcome{result: toolResult, isError: true}}
		}
	}
	if aborted(ctx) {
		return prepareResult{immediate: &immediateOutcome{result: errorToolResult("Operation aborted"), isError: true}}
	}
	return prepareResult{prepared: &preparedToolCall{toolCall: toolCall, tool: tool, args: validated}}
}

func executePreparedToolCall(ctx context.Context, prepared preparedToolCall, emit model.EventSink) immediateOutcome {
	var updateMu sync.Mutex
	var updateEmitErr error
	acceptingUpdates := true
	onUpdate := func(partial model.AgentToolResult) {
		updateMu.Lock()
		accepting := acceptingUpdates
		updateMu.Unlock()
		if !accepting {
			return
		}
		err := emit(model.AgentEvent{
			Type:          model.EvToolExecutionUpdate,
			ToolCallID:    prepared.toolCall.ID,
			ToolName:      prepared.toolCall.Name,
			Args:          prepared.toolCall.Arguments,
			PartialResult: partial,
		})
		if err != nil {
			updateMu.Lock()
			if updateEmitErr == nil {
				updateEmitErr = err
			}
			updateMu.Unlock()
		}
	}

	outcome := func() (out immediateOutcome) {
		defer func() {
			if r := recover(); r != nil {
				if ep, ok := r.(emitPanic); ok {
					panic(ep)
				}
				out = immediateOutcome{result: errorToolResult(panicMessage(r)), isError: true}
			}
		}()
		result, err := prepared.tool.Execute(ctx, prepared.toolCall.ID, prepared.args, onUpdate)
		if err != nil {
			return immediateOutcome{result: errorToolResult(err.Error()), isError: true}
		}
		return immediateOutcome{result: result, isError: false}
	}()

	updateMu.Lock()
	acceptingUpdates = false
	emitErr := updateEmitErr
	updateMu.Unlock()
	if emitErr != nil {
		panic(emitPanic{err: emitErr})
	}
	return outcome
}

func finalizeExecutedToolCall(ctx context.Context, current *model.AgentContext, message *model.AssistantMessage, prepared preparedToolCall, executed immediateOutcome, config model.AgentLoopConfig) finalizedOutcome {
	result := executed.result
	isError := executed.isError

	if config.AfterToolCall != nil {
		func() {
			defer func() {
				if r := recover(); r != nil {
					if ep, ok := r.(emitPanic); ok {
						panic(ep)
					}
					result = errorToolResult(panicMessage(r))
					isError = true
				}
			}()
			after := config.AfterToolCall(ctx, model.AfterToolCallContext{
				AssistantMessage: message,
				ToolCall:         prepared.toolCall,
				Args:             prepared.args,
				Result:           result,
				IsError:          isError,
				Context:          current,
			})
			if after != nil {
				if after.HasContent {
					result.Content = after.Content
				}
				if after.HasDetails {
					result.Details = after.Details
				}
				if after.Usage != nil {
					result.Usage = after.Usage
				}
				if after.Terminate != nil {
					result.Terminate = *after.Terminate
				}
				if after.IsError != nil {
					isError = *after.IsError
				}
			}
		}()
	}

	return finalizedOutcome{toolCall: prepared.toolCall, result: result, isError: isError}
}

func errorToolResult(message string) model.AgentToolResult {
	return model.AgentToolResult{
		Content: model.ContentList{model.TextContent{Text: message}},
		Details: map[string]any{},
	}
}

func emitToolExecutionEnd(outcome finalizedOutcome, emit model.EventSink) {
	mustEmit(emit, model.AgentEvent{
		Type:       model.EvToolExecutionEnd,
		ToolCallID: outcome.toolCall.ID,
		ToolName:   outcome.toolCall.Name,
		Result:     outcome.result,
		IsError:    outcome.isError,
	})
}

func createToolResultMessage(outcome finalizedOutcome) model.ToolResultMessage {
	return model.ToolResultMessage{
		ToolCallID: outcome.toolCall.ID,
		ToolName:   outcome.toolCall.Name,
		Content:    outcome.result.Content,
		Details:    outcome.result.Details,
		Usage:      outcome.result.Usage,
		IsError:    outcome.isError,
		Timestamp:  nowMillis(),
	}
}

func emitToolResultMessage(toolResult model.ToolResultMessage, emit model.EventSink) {
	mustEmit(emit, model.AgentEvent{Type: model.EvMessageStart, Message: toolResult})
	mustEmit(emit, model.AgentEvent{Type: model.EvMessageEnd, Message: toolResult})
}
