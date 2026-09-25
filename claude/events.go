package claude

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/tylergannon/gimbal"
)

// projector consumes Claude Code's raw JSON before the SDK parser. Stream
// events are authoritative for part boundaries; materialized assistant UUIDs
// are audit identity only, while nested message.id owns the step.
type projector struct {
	mu        sync.Mutex
	emit      func(gimbal.AgentEvent) error
	sessionID string
	model     string

	messageID     string
	modelID       string
	stepOpen      bool
	streamed      bool
	stopReason    string
	blocks        map[int]*blockState
	textOpen      bool
	reasoningOpen bool
	pendingTools  map[string]bool
	toolMessages  map[string]string
	nested        map[string][]map[string]any
	usage         claudeUsage
	report        map[string]gimbal.Usage
	failureSeen   bool
}

type blockState struct {
	kind    string
	ordinal int
	id      string
	name    string
	text    strings.Builder
}

type claudeUsage struct {
	input, outputTotal, reasoning, cacheRead, cacheWrite float64
}

func newProjector(sessionID, model string, emit func(gimbal.AgentEvent) error) *projector {
	return &projector{
		emit: emit, sessionID: sessionID, model: model,
		blocks: make(map[int]*blockState), pendingTools: make(map[string]bool), toolMessages: make(map[string]string), nested: make(map[string][]map[string]any),
	}
}

func (p *projector) event(eventType string, data map[string]any, native any) error {
	data["sessionID"] = p.sessionID
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	event := gimbal.AgentEvent{Type: eventType, Data: raw}
	if native != nil {
		event.NativeRef, err = json.Marshal(native)
		if err != nil {
			return err
		}
	}
	return p.emit(event)
}

// observe never lets a projection error stop Claude's turn. The raw message
// remains available in a diagnostic when the transcript cannot represent it.
func (p *projector) observe(raw json.RawMessage) {
	if err := p.raw(raw); err != nil {
		_ = p.event("session.synthetic", map[string]any{
			"text":        "Claude event projection gap: " + err.Error(),
			"description": "Observation gap",
			"metadata":    map[string]any{"raw": string(raw)},
		}, map[string]any{"provider": "claude", "sessionID": p.sessionID})
	}
}

func (p *projector) raw(raw json.RawMessage) error {
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("claude: decode raw message: %w", err)
	}
	if parent := stringValue(envelope["parent_tool_use_id"]); parent != "" && p.ownsNestedParent(parent) {
		return p.nestedEvent(parent, envelope)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	switch envelope["type"] {
	case "stream_event":
		event, _ := envelope["event"].(map[string]any)
		if event == nil {
			return errors.New("claude: stream_event has no event object")
		}
		return p.stream(event, envelope)
	case "assistant":
		return p.materializedAssistant(envelope)
	case "user":
		return p.toolResults(envelope)
	case "tool_progress":
		return p.toolProgress(envelope)
	case "result":
		return p.result(envelope)
	case "system":
		return p.system(envelope)
	}
	return nil
}

func (p *projector) ownsNestedParent(parent string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.toolMessages[parent] != ""
}

// nestedEvent retains Claude Code's lossless child message under the Task tool
// that owns it. It deliberately does not feed the child's step, text, or usage
// through the parent projector.
func (p *projector) nestedEvent(parent string, envelope map[string]any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	messageID := p.toolMessages[parent]
	if messageID == "" {
		return fmt.Errorf("claude: nested transcript for unknown parent tool_use_id %s", parent)
	}
	p.nested[parent] = append(p.nested[parent], envelope)
	ref := p.nativeRef(envelope, parent)
	ref["messageID"] = messageID
	return p.event("session.tool.progress", map[string]any{
		"assistantMessageID": messageID,
		"id":                 parent,
		"metadata":           map[string]any{"mode": "append", "transcript": []any{envelope}},
	}, ref)
}

func (p *projector) system(envelope map[string]any) error {
	switch stringValue(envelope["subtype"]) {
	case "api_retry":
		errorValue := map[string]any{"type": envelope["error"], "message": "Claude API request failed and will be retried"}
		if envelope["error_status"] != nil {
			errorValue["status"] = envelope["error_status"]
		}
		if p.stepOpen {
			return p.event("session.retry.scheduled", map[string]any{
				"assistantMessageID": p.messageID,
				"attempt":            envelope["attempt"],
				"delayMS":            envelope["retry_delay_ms"],
				"error":              errorValue,
			}, p.nativeRef(envelope, ""))
		}
		return p.event("session.synthetic", map[string]any{
			"text":        "Claude API request failed and will be retried",
			"description": "Harness retry",
			"metadata":    envelope,
		}, p.nativeRef(envelope, ""))
	case "permission_denied":
		id := stringValue(envelope["tool_use_id"])
		if id == "" {
			id = stringValue(envelope["uuid"])
		}
		if err := p.permissionAsked(id, stringValue(envelope["tool_name"]), envelope, p.nativeRef(envelope, id)); err != nil {
			return err
		}
		return p.permissionReplied(id, "denied", p.nativeRef(envelope, id))
	}
	return nil
}

func (p *projector) permissionAsked(id, permission string, metadata map[string]any, ref map[string]any) error {
	return p.event("permission.asked", map[string]any{"id": id, "permission": permission, "metadata": metadata}, ref)
}

func (p *projector) permissionReplied(id, reply string, ref map[string]any) error {
	return p.event("permission.replied", map[string]any{"requestID": id, "reply": reply}, ref)
}

func (p *projector) stream(event, envelope map[string]any) error {
	ref := p.nativeRef(envelope, "")
	switch event["type"] {
	case "message_start":
		if p.stepOpen {
			return errors.New("claude: message_start before the previous step ended")
		}
		message, _ := event["message"].(map[string]any)
		p.messageID = stringValue(message["id"])
		if p.messageID == "" {
			return errors.New("claude: message_start has no nested message.id")
		}
		p.modelID = stringValue(message["model"])
		if p.modelID == "" {
			p.modelID = p.model
		}
		p.stepOpen, p.streamed = true, false
		p.stopReason = ""
		p.blocks = make(map[int]*blockState)
		p.pendingTools = make(map[string]bool)
		p.usage = claudeUsage{}
		p.usage.merge(object(message["usage"]))
		ref["messageID"] = p.messageID
		return p.event("session.step.started", map[string]any{
			"assistantMessageID": p.messageID, "agent": "claude",
			"model": map[string]any{"providerID": "anthropic", "id": p.modelID},
		}, ref)
	case "content_block_start":
		return p.blockStart(event, ref)
	case "content_block_delta":
		if delta := object(event["delta"]); stringValue(delta["type"]) == "signature_delta" {
			return nil
		}
		return p.blockDelta(event, ref)
	case "content_block_stop":
		return p.blockStop(event, ref)
	case "message_delta":
		delta := object(event["delta"])
		if stop := stringValue(delta["stop_reason"]); stop != "" {
			p.stopReason = stop
		}
		if usage := object(event["usage"]); usage != nil {
			p.usage.merge(usage)
		}
		return nil
	case "message_stop":
		if !p.stepOpen {
			return errors.New("claude: message_stop without message_start")
		}
		if p.textOpen || p.reasoningOpen {
			return errors.New("claude: message_stop with an open text or reasoning block")
		}
		p.streamed = true
		if err := p.event("session.step.streamed", map[string]any{"assistantMessageID": p.messageID}, ref); err != nil {
			return err
		}
		if len(p.pendingTools) == 0 {
			return p.endStep(ref)
		}
	}
	return nil
}

func (p *projector) blockStart(event map[string]any, ref map[string]any) error {
	if !p.stepOpen {
		return errors.New("claude: content block opened without a message")
	}
	index := int(numberValue(event["index"]))
	if _, exists := p.blocks[index]; exists {
		return fmt.Errorf("claude: content block %d opened twice", index)
	}
	block := object(event["content_block"])
	state := &blockState{kind: stringValue(block["type"]), ordinal: index, id: stringValue(block["id"]), name: stringValue(block["name"])}
	p.blocks[index] = state
	switch state.kind {
	case "text":
		if p.textOpen {
			return errors.New("claude: a second text block opened before the first ended")
		}
		p.textOpen = true
		return p.event("session.text.started", map[string]any{"assistantMessageID": p.messageID, "ordinal": index}, ref)
	case "thinking":
		if p.reasoningOpen {
			return errors.New("claude: a second reasoning block opened before the first ended")
		}
		p.reasoningOpen = true
		return p.event("session.reasoning.started", map[string]any{"assistantMessageID": p.messageID, "ordinal": index}, ref)
	case "tool_use":
		if state.id == "" || state.name == "" {
			return errors.New("claude: tool_use block has no id or name")
		}
		if p.pendingTools[state.id] {
			return fmt.Errorf("claude: tool_use_id %s opened twice", state.id)
		}
		p.toolMessages[state.id] = p.messageID
		ref["itemID"] = state.id
		if input, ok := block["input"]; ok {
			raw, _ := json.Marshal(input)
			if string(raw) != "{}" && string(raw) != "null" {
				state.text.Write(raw)
			}
		}
		return p.event("session.tool.input.started", map[string]any{"assistantMessageID": p.messageID, "id": state.id, "name": state.name}, ref)
	default:
		return fmt.Errorf("claude: unsupported content block type %q", state.kind)
	}
}

func (p *projector) blockDelta(event map[string]any, ref map[string]any) error {
	index := int(numberValue(event["index"]))
	state := p.blocks[index]
	if state == nil {
		return fmt.Errorf("claude: delta for unopened content block %d", index)
	}
	if state.id != "" {
		ref["itemID"] = state.id
	}
	delta := object(event["delta"])
	switch stringValue(delta["type"]) {
	case "text_delta":
		if state.kind != "text" || !p.textOpen {
			return errors.New("claude: text delta outside an open text block")
		}
		text := stringValue(delta["text"])
		state.text.WriteString(text)
		return p.event("session.text.delta", map[string]any{"assistantMessageID": p.messageID, "ordinal": state.ordinal, "delta": text}, ref)
	case "thinking_delta":
		if state.kind != "thinking" || !p.reasoningOpen {
			return errors.New("claude: thinking delta outside an open reasoning block")
		}
		text := stringValue(delta["thinking"])
		state.text.WriteString(text)
		return p.event("session.reasoning.delta", map[string]any{"assistantMessageID": p.messageID, "ordinal": state.ordinal, "delta": text}, ref)
	case "input_json_delta":
		if state.kind != "tool_use" {
			return errors.New("claude: input JSON delta outside a tool_use block")
		}
		text := stringValue(delta["partial_json"])
		state.text.WriteString(text)
		return p.event("session.tool.input.delta", map[string]any{"assistantMessageID": p.messageID, "id": state.id, "delta": text}, ref)
	default:
		return fmt.Errorf("claude: unsupported content delta type %q", stringValue(delta["type"]))
	}
}

func (p *projector) blockStop(event map[string]any, ref map[string]any) error {
	index := int(numberValue(event["index"]))
	state := p.blocks[index]
	if state == nil {
		return fmt.Errorf("claude: stop for unopened content block %d", index)
	}
	if state.id != "" {
		ref["itemID"] = state.id
	}
	delete(p.blocks, index)
	switch state.kind {
	case "text":
		if !p.textOpen {
			return errors.New("claude: text block stopped twice")
		}
		p.textOpen = false
		return p.event("session.text.ended", map[string]any{"assistantMessageID": p.messageID, "ordinal": state.ordinal, "text": state.text.String()}, ref)
	case "thinking":
		if !p.reasoningOpen {
			return errors.New("claude: reasoning block stopped twice")
		}
		p.reasoningOpen = false
		return p.event("session.reasoning.ended", map[string]any{"assistantMessageID": p.messageID, "ordinal": state.ordinal, "text": state.text.String()}, ref)
	case "tool_use":
		inputText := state.text.String()
		if inputText == "" {
			inputText = "{}"
		}
		// A model sometimes streams input that is not a JSON object. Claude
		// Code does not end the turn on it: it records the raw text under
		// __unparsedToolInput, answers the call with an input validation
		// error, and lets the model try again. The turn goes on here too,
		// with the input recorded the same way.
		var input map[string]any
		if err := json.Unmarshal([]byte(inputText), &input); err != nil {
			input = map[string]any{"__unparsedToolInput": map[string]any{"raw": inputText, "len": len(inputText)}}
		}
		if err := p.event("session.tool.input.ended", map[string]any{"assistantMessageID": p.messageID, "id": state.id, "text": inputText}, ref); err != nil {
			return err
		}
		p.pendingTools[state.id] = true
		return p.event("session.tool.called", map[string]any{"assistantMessageID": p.messageID, "id": state.id, "input": input, "executed": true}, ref)
	}
	return nil
}

func (p *projector) materializedAssistant(envelope map[string]any) error {
	message := object(envelope["message"])
	nestedID := stringValue(message["id"])
	if nestedID == "" {
		return errors.New("claude: materialized assistant message has no nested message.id")
	}
	if p.stepOpen && nestedID != p.messageID {
		return fmt.Errorf("claude: materialized message.id %s does not match stream %s", nestedID, p.messageID)
	}
	if code := stringValue(envelope["error"]); code != "" {
		// Claude Code's own explanation of the failure is the message's
		// text. It is the only statement of why the turn failed, so it
		// belongs on the record beside the code.
		text := "Claude assistant error: " + code
		errorValue := map[string]any{"type": code}
		if blocks, ok := message["content"].([]any); ok {
			if explanation := oneLine(contentText(blocks)); explanation != "" {
				text += ": " + explanation
				errorValue["harnessMessage"] = explanation
			}
		}
		errorValue["message"] = text
		if requestID := stringValue(envelope["request_id"]); requestID != "" {
			errorValue["requestID"] = requestID
		}
		return p.harnessFailure(errorValue, envelope)
	}
	return nil
}

func (p *projector) toolResults(envelope map[string]any) error {
	message := object(envelope["message"])
	content, _ := message["content"].([]any)
	mapped := false
	for _, rawBlock := range content {
		block := object(rawBlock)
		if stringValue(block["type"]) != "tool_result" {
			continue
		}
		mapped = true
		id := stringValue(block["tool_use_id"])
		if id == "" {
			return errors.New("claude: tool_result has no tool_use_id")
		}
		if !p.pendingTools[id] {
			if err := p.event("session.synthetic", map[string]any{
				"text":        "Claude tool result has no observed tool call: " + id,
				"description": "Observation gap",
				"metadata":    envelope,
			}, p.nativeRef(envelope, id)); err != nil {
				return err
			}
			continue
		}
		delete(p.pendingTools, id)
		ref := p.nativeRef(envelope, id)
		isError, _ := block["is_error"].(bool)
		text := contentText(block["content"])
		content := []any{map[string]any{"type": "text", "text": text}}
		if transcript := p.nested[id]; len(transcript) > 0 {
			content = append(content, map[string]any{"type": "transcript", "events": transcript})
		}
		if isError {
			if err := p.event("session.tool.failed", map[string]any{"assistantMessageID": p.messageID, "id": id, "error": map[string]any{"type": "tool_result", "message": text}, "content": content, "executed": true}, ref); err != nil {
				return err
			}
		} else {
			if err := p.event("session.tool.success", map[string]any{"assistantMessageID": p.messageID, "id": id, "content": content, "executed": true}, ref); err != nil {
				return err
			}
		}
	}
	if !mapped {
		return nil
	}
	if p.stepOpen && p.streamed && len(p.pendingTools) == 0 {
		return p.endStep(p.nativeRef(envelope, ""))
	}
	return nil
}

func (p *projector) toolProgress(envelope map[string]any) error {
	id := stringValue(envelope["tool_use_id"])
	if id == "" || !p.pendingTools[id] {
		return fmt.Errorf("claude: progress for unknown tool_use_id %s", id)
	}
	metadata := map[string]any{"toolName": envelope["tool_name"], "elapsedSeconds": envelope["elapsed_time_seconds"]}
	if envelope["task_id"] != nil {
		metadata["taskID"] = envelope["task_id"]
	}
	return p.event("session.tool.progress", map[string]any{"assistantMessageID": p.messageID, "id": id, "metadata": metadata}, p.nativeRef(envelope, id))
}

func (p *projector) endStep(ref map[string]any) error {
	if !p.stepOpen {
		return nil
	}
	ref["messageID"] = p.messageID
	// Claude Code states no per-step cost; the turn report carries the cost.
	data := map[string]any{
		"assistantMessageID": p.messageID, "finish": claudeFinish(p.stopReason), "cost": 0,
		"tokens": p.usage.tokens(),
	}
	if p.stopReason != "" {
		data["rawFinish"] = p.stopReason
	}
	err := p.event("session.step.ended", data, ref)
	if err != nil {
		return err
	}
	p.stepOpen, p.streamed = false, false
	p.messageID, p.stopReason = "", ""
	return nil
}

func (p *projector) harnessFailure(errorValue map[string]any, envelope map[string]any) error {
	if p.failureSeen {
		return nil
	}
	p.failureSeen = true
	ref := p.nativeRef(envelope, "")
	if !p.stepOpen {
		return p.event("session.synthetic", map[string]any{
			"text":        errorValue["message"],
			"description": "Harness error",
			"metadata":    errorValue,
		}, ref)
	}
	err := p.event("session.step.failed", map[string]any{
		"assistantMessageID": p.messageID,
		"finish":             "error",
		"error":              errorValue,
	}, ref)
	p.stepOpen, p.streamed = false, false
	p.textOpen, p.reasoningOpen = false, false
	p.messageID = ""
	p.blocks = make(map[int]*blockState)
	p.pendingTools = make(map[string]bool)
	return err
}

func (p *projector) nativeRef(envelope map[string]any, itemID string) map[string]any {
	ref := map[string]any{"provider": "claude", "sessionID": p.sessionID}
	if p.messageID != "" {
		ref["messageID"] = p.messageID
	}
	if itemID != "" {
		ref["itemID"] = itemID
	}
	if parent := stringValue(envelope["parent_tool_use_id"]); parent != "" {
		ref["parentToolUseID"] = parent
	}
	return ref
}

// result records the latest report from Claude Code. Across automatic
// continuations modelUsage is cumulative for the native process, so the latest
// report already includes the waiting generations that preceded it.
func (p *projector) result(envelope map[string]any) error {
	report := make(map[string]gimbal.Usage)
	for model, raw := range object(envelope["modelUsage"]) {
		entry := object(raw)
		if entry == nil {
			continue
		}
		report[model] = gimbal.Usage{
			Cost: numberValue(entry["costUSD"]),
			Tokens: claudeTokens(numberValue(entry["inputTokens"]), numberValue(entry["outputTokens"]),
				numberValue(entry["thinkingTokens"]), numberValue(entry["cacheReadInputTokens"]),
				numberValue(entry["cacheCreationInputTokens"])),
		}
	}
	if len(report) == 0 {
		usage, cost := object(envelope["usage"]), numberValue(envelope["total_cost_usd"])
		if usage != nil || envelope["total_cost_usd"] != nil {
			var u claudeUsage
			u.merge(usage)
			report[p.model] = gimbal.Usage{Cost: cost, Tokens: u.tokens()}
		}
	}
	if len(report) > 0 {
		p.report = report
	}
	if failed, _ := envelope["is_error"].(bool); failed || (stringValue(envelope["subtype"]) != "" && stringValue(envelope["subtype"]) != "success") {
		message := contentText(envelope["errors"])
		if message == "null" || message == "[]" || message == "" {
			message = "Claude turn failed: " + stringValue(envelope["subtype"])
		}
		return p.harnessFailure(map[string]any{"type": stringValue(envelope["subtype"]), "message": message}, envelope)
	}
	return nil
}

// turnUsage is the harness's own report for the turn, or nil when the turn
// ended without one.
func (p *projector) turnUsage() map[string]gimbal.Usage {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.report
}

// claudeTokens is the five-field mapping, for one Claude accounting record in
// either spelling. Anthropic's input tokens are already the non-cached count,
// with cache read and cache creation reported separately, so input passes
// through unchanged; thinking tokens are a subset of the output tokens, so
// the visible output is the output less the thinking.
func claudeTokens(input, output, reasoning, cacheRead, cacheWrite float64) gimbal.Tokens {
	var tokens gimbal.Tokens
	tokens.Input = input
	tokens.Output = max(0, output-reasoning)
	tokens.Reasoning = reasoning
	tokens.Cache.Read = cacheRead
	tokens.Cache.Write = cacheWrite
	return tokens
}

func (u claudeUsage) tokens() gimbal.Tokens {
	return claudeTokens(u.input, u.outputTotal, u.reasoning, u.cacheRead, u.cacheWrite)
}

// merge folds one stream usage object in: message_start states the prompt
// side and message_delta the final output. A field the message omits keeps
// what an earlier message stated, and a field neither states is zero.
func (u *claudeUsage) merge(value map[string]any) {
	if value == nil {
		return
	}
	if n, ok := number(value["input_tokens"]); ok {
		u.input = n
	}
	if n, ok := number(value["output_tokens"]); ok {
		u.outputTotal = n
	}
	if n, ok := number(value["cache_read_input_tokens"]); ok {
		u.cacheRead = n
	}
	if n, ok := number(value["cache_creation_input_tokens"]); ok {
		u.cacheWrite = n
	}
	if details := object(value["output_tokens_details"]); details != nil {
		if n, ok := number(details["thinking_tokens"]); ok {
			u.reasoning = n
		}
	}
}

func claudeFinish(raw string) string {
	switch raw {
	case "end_turn", "stop_sequence":
		return "stop"
	case "tool_use":
		return "tool-calls"
	case "max_tokens":
		return "length"
	case "refusal", "content_filter":
		return "content-filter"
	case "error":
		return "error"
	default:
		return "unknown"
	}
}

func contentText(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	if blocks, ok := value.([]any); ok {
		var parts []string
		for _, raw := range blocks {
			block := object(raw)
			if text, _ := block["text"].(string); text != "" {
				parts = append(parts, text)
			} else if encoded, err := json.Marshal(raw); err == nil {
				parts = append(parts, string(encoded))
			}
		}
		return strings.Join(parts, "\n")
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(raw)
}

// oneLine collapses runs of whitespace, so a message Claude Code wrote
// across several lines reads as one line in an error and in a log.
func oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func object(value any) map[string]any {
	object, _ := value.(map[string]any)
	return object
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func numberValue(value any) float64 {
	number, _ := value.(float64)
	return number
}

func number(value any) (float64, bool) {
	n, ok := value.(float64)
	return n, ok
}
