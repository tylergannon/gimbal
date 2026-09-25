package codex

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/tylergannon/gimbal"
)

// projector turns the ordered app-server stream into OpenCode session events.
// responseID arrives only at rawResponse/completed, so a response window keeps
// one provisional native message key and binds that ID without renaming it.
type projector struct {
	mu        sync.Mutex
	emit      func(gimbal.AgentEvent) error
	sessionID string
	turnID    string
	model     string

	response      int
	messageID     string
	responseID    string
	stepOpen      bool
	streamed      bool
	compacting    bool
	partOrdinal   int
	parts         map[string]*contentPart
	partQueues    map[string][]*contentPart
	tools         map[string]*toolState
	nestedTools   map[string]*nestedToolState
	responseCalls map[string]bool
	pendingTools  int
	usage         normalizedUsage
}

type toolState struct {
	name   string
	output strings.Builder
	done   bool
}

// Native items can overlap or repeat. Keep their identity and buffer later
// parts while the transcript's current part streams. This serializes display
// events without imposing that ordering on the provider or mixing their text.
type contentPart struct {
	ordinal       int
	text          strings.Builder
	done, visible bool
	native        any
}

// A sink failure (for example, writing the run record) is operational, not
// an unexpected provider display shape. Preserve it through projection.
type eventSinkError struct{ error }

func (e *eventSinkError) Unwrap() error { return e.error }

type nestedToolState struct {
	messageID string
	events    []map[string]any
}

type normalizedUsage struct {
	input, output, reasoning, cacheRead, cacheWrite float64
}

func newProjector(sessionID, turnID, model string, emit func(gimbal.AgentEvent) error) *projector {
	return &projector{
		emit: emit, sessionID: sessionID, turnID: turnID, model: model,
		tools: make(map[string]*toolState), nestedTools: make(map[string]*nestedToolState), responseCalls: make(map[string]bool),
		parts: make(map[string]*contentPart), partQueues: make(map[string][]*contentPart),
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
	if err := p.emit(event); err != nil {
		return &eventSinkError{err}
	}
	return nil
}

func (p *projector) ensureStep(nativeMessageID string, native any) error {
	if p.stepOpen {
		return nil
	}
	p.response++
	p.messageID = nativeMessageID
	if p.messageID == "" {
		p.messageID = fmt.Sprintf("%s/response.%d", p.turnID, p.response)
	}
	p.responseID = ""
	p.stepOpen, p.streamed = true, false
	p.partOrdinal = 0
	p.usage = normalizedUsage{}
	if ref, ok := native.(map[string]any); ok {
		ref["messageID"] = p.messageID
	}
	return p.event("session.step.started", map[string]any{
		"assistantMessageID": p.messageID,
		"agent":              "codex",
		"model":              map[string]any{"providerID": "openai", "id": p.model},
	}, native)
}

func (p *projector) nativeRef(params json.RawMessage) map[string]any {
	ref := map[string]any{"provider": "codex", "sessionID": p.sessionID, "turnID": p.turnID}
	var value map[string]any
	_ = json.Unmarshal(params, &value)
	itemID := stringField(value, "itemId")
	if item := objectValueOrNil(value["item"]); itemID == "" && item != nil {
		itemID = stringField(item, "id")
	}
	if itemID != "" {
		ref["itemID"] = itemID
	}
	if p.messageID != "" {
		ref["messageID"] = p.messageID
	}
	if p.responseID != "" {
		ref["responseID"] = p.responseID
	}
	return ref
}

func (p *projector) itemStarted(params json.RawMessage) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	item, ok := decodeItem(params)
	if !ok {
		return errors.New("codex: item/started has no item identity")
	}
	if item.kind == "contextCompaction" {
		p.compacting = true
		return nil
	}
	if (item.kind == "reasoning" && p.parts["reasoning\x00"+item.id] != nil) ||
		(item.kind == "agentMessage" && p.parts["text\x00"+item.id] != nil) {
		return nil // a delta or earlier start already introduced this item
	}
	if p.streamed && (!isTool(item.kind) || p.responseCallsSettled()) {
		if err := p.endStep(nil); err != nil {
			return err
		}
	}
	if err := p.ensureStep(item.id, p.nativeRef(params)); err != nil {
		return err
	}
	switch {
	case item.kind == "agentMessage":
		_, err := p.contentPart("text", item.id, p.nativeRef(params))
		return err
	case item.kind == "reasoning":
		_, err := p.contentPart("reasoning", item.id, p.nativeRef(params))
		return err
	case isTool(item.kind):
		return p.startTool(item, params)
	}
	return nil
}

func (p *projector) contentPart(kind, id string, native any) (*contentPart, error) {
	if id == "" && len(p.partQueues[kind]) > 0 {
		return p.partQueues[kind][0], nil
	}
	key := kind + "\x00" + id
	if part := p.parts[key]; part != nil {
		return part, nil
	}
	part := &contentPart{ordinal: p.partOrdinal, native: native}
	p.partOrdinal++
	p.parts[key] = part
	p.partQueues[kind] = append(p.partQueues[kind], part)
	return part, p.flushParts(kind)
}

func (p *projector) flushParts(kind string) error {
	for len(p.partQueues[kind]) > 0 {
		part := p.partQueues[kind][0]
		data := map[string]any{"assistantMessageID": p.messageID, "ordinal": part.ordinal}
		if !part.visible {
			if err := p.event("session."+kind+".started", data, part.native); err != nil {
				return err
			}
			part.visible = true
			if !part.done && part.text.Len() > 0 {
				if err := p.event("session."+kind+".delta", map[string]any{"assistantMessageID": p.messageID, "ordinal": part.ordinal, "delta": part.text.String()}, part.native); err != nil {
					return err
				}
			}
		}
		if !part.done {
			return nil
		}
		data["text"] = part.text.String()
		if err := p.event("session."+kind+".ended", data, part.native); err != nil {
			return err
		}
		p.partQueues[kind] = p.partQueues[kind][1:]
	}
	return nil
}

func (p *projector) startTool(item nativeItem, params json.RawMessage) error {
	if _, exists := p.tools[item.id]; exists {
		return fmt.Errorf("codex: tool %s opened twice", item.id)
	}
	state := &toolState{name: toolName(item)}
	p.tools[item.id] = state
	if item.kind == "collabAgentToolCall" {
		p.nestedTools[item.id] = &nestedToolState{messageID: p.messageID}
	}
	p.pendingTools++
	ref := p.nativeRef(params)
	if err := p.event("session.tool.input.started", map[string]any{"assistantMessageID": p.messageID, "id": item.id, "name": state.name}, ref); err != nil {
		return err
	}
	input := toolArgs(item)
	inputRaw, _ := json.Marshal(input)
	if err := p.event("session.tool.input.ended", map[string]any{"assistantMessageID": p.messageID, "id": item.id, "text": string(inputRaw)}, ref); err != nil {
		return err
	}
	return p.event("session.tool.called", map[string]any{"assistantMessageID": p.messageID, "id": item.id, "input": objectValue(input), "executed": true}, ref)
}

func (p *projector) textDelta(params json.RawMessage) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.contentDelta("text", params)
}

func (p *projector) reasoningDelta(params json.RawMessage) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.contentDelta("reasoning", params)
}

func (p *projector) contentDelta(kind string, params json.RawMessage) error {
	id := paramItemID(params)
	if part := p.parts[kind+"\x00"+id]; part != nil && part.done {
		return nil
	}
	if err := p.ensureStep(id, p.nativeRef(params)); err != nil {
		return err
	}
	part, err := p.contentPart(kind, id, p.nativeRef(params))
	if err != nil {
		return err
	}
	delta := deltaText(params)
	part.text.WriteString(delta)
	if !part.visible {
		return nil
	}
	return p.event("session."+kind+".delta", map[string]any{"assistantMessageID": p.messageID, "ordinal": part.ordinal, "delta": delta}, p.nativeRef(params))
}

func (p *projector) toolOutputDelta(params json.RawMessage) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	var value struct {
		ItemID string `json:"itemId"`
		Delta  string `json:"delta"`
	}
	if err := json.Unmarshal(params, &value); err != nil || value.ItemID == "" {
		return errors.New("codex: tool output delta has no itemId")
	}
	// A delta can arrive for a tool this step no longer holds: the step was
	// ended by a new item while the command still ran, or the command had
	// already completed. The completed item carries the whole output, so
	// the delta is dropped rather than ending the turn.
	state := p.tools[value.ItemID]
	if state == nil || state.done {
		return nil
	}
	state.output.WriteString(value.Delta)
	return p.event("session.tool.progress", map[string]any{
		"assistantMessageID": p.messageID, "id": value.ItemID,
		"metadata": map[string]any{"output": state.output.String(), "mode": "replace"},
	}, p.nativeRef(params))
}

// itemCompleted emits an authoritative completed item and returns final prose.
func (p *projector) itemCompleted(params json.RawMessage) (string, bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	item, ok := decodeItem(params)
	if !ok {
		return "", false, errors.New("codex: item/completed has no item identity")
	}
	if item.kind == "contextCompaction" {
		p.compacting = false
		return "", false, nil
	}
	for _, kind := range []string{"text", "reasoning"} {
		if part := p.parts[kind+"\x00"+item.id]; part != nil && part.done {
			return "", false, nil
		}
	}
	if !p.stepOpen {
		if err := p.ensureStep(item.id, p.nativeRef(params)); err != nil {
			return "", false, err
		}
	}
	ref := p.nativeRef(params)
	switch {
	case item.kind == "agentMessage":
		text, _ := item.value["text"].(string)
		err := p.completePart("text", item.id, text, ref)
		return text, true, err
	case item.kind == "reasoning":
		text := joined(item.value["summary"])
		if text == "" {
			text = joined(item.value["content"])
		}
		if text == "" && p.parts["reasoning\x00"+item.id] == nil {
			return "", false, nil
		}
		if err := p.completePart("reasoning", item.id, text, ref); err != nil {
			return "", false, err
		}
	case isTool(item.kind):
		state := p.tools[item.id]
		if state == nil {
			if err := p.startTool(item, params); err != nil {
				return "", false, err
			}
			state = p.tools[item.id]
		}
		if state.done {
			return "", false, fmt.Errorf("codex: tool %s completed twice", item.id)
		}
		state.done = true
		p.pendingTools--
		output := toolOutput(item)
		nested := p.nestedTools[item.id]
		if failed, message := toolFailure(item); failed {
			data := map[string]any{"assistantMessageID": p.messageID, "id": item.id, "error": map[string]any{"type": item.kind, "message": message}, "executed": true}
			if nested != nil && len(nested.events) > 0 {
				data["content"] = []any{map[string]any{"type": "transcript", "events": nested.events}}
			}
			if err := p.event("session.tool.failed", data, ref); err != nil {
				return "", false, err
			}
		} else {
			content := []any{map[string]any{"type": "text", "text": outputText(output)}}
			if nested != nil && len(nested.events) > 0 {
				content = append(content, map[string]any{"type": "transcript", "events": nested.events})
			}
			if err := p.event("session.tool.success", map[string]any{"assistantMessageID": p.messageID, "id": item.id, "content": content, "executed": true}, ref); err != nil {
				return "", false, err
			}
		}
		if p.streamed && p.pendingTools == 0 && p.responseCallsSettled() {
			if err := p.endStep(params); err != nil {
				return "", false, err
			}
		}
	}
	return "", false, nil
}

func (p *projector) completePart(kind, id, text string, native any) error {
	part, err := p.contentPart(kind, id, native)
	if err != nil || part.done {
		return err
	}
	part.text.Reset()
	part.text.WriteString(text)
	part.done = true
	if err := p.flushParts(kind); err != nil {
		return err
	}
	if p.streamed && p.pendingTools == 0 && p.responseCallsSettled() {
		return p.endStep(nil)
	}
	return nil
}

// nestedEvent attaches one normalized event from a native child thread to the
// collab tool that spawned it. The child projector is separate, so its steps,
// text, tools, and usage can never mutate the parent's projector state.
func (p *projector) nestedEvent(parentTool string, event gimbal.AgentEvent) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	state := p.nestedTools[parentTool]
	if state == nil {
		return fmt.Errorf("codex: nested transcript for unknown parent tool %s", parentTool)
	}
	entry := map[string]any{"type": event.Type, "data": json.RawMessage(event.Data)}
	if len(event.NativeRef) > 0 {
		entry["nativeRef"] = json.RawMessage(event.NativeRef)
	}
	state.events = append(state.events, entry)
	return p.event("session.tool.progress", map[string]any{
		"assistantMessageID": state.messageID,
		"id":                 parentTool,
		"metadata":           map[string]any{"mode": "append", "transcript": []any{entry}},
	}, map[string]any{"provider": "codex", "sessionID": p.sessionID, "turnID": p.turnID, "itemID": parentTool})
}

func codexChildThreads(params json.RawMessage) (string, []string) {
	item, ok := decodeItem(params)
	if !ok || item.kind != "collabAgentToolCall" {
		return "", nil
	}
	raw, _ := item.value["receiverThreadIds"].([]any)
	children := make([]string, 0, len(raw))
	for _, value := range raw {
		if id, _ := value.(string); id != "" {
			children = append(children, id)
		}
	}
	return item.id, children
}

func (p *projector) rawResponseItemCompleted(params json.RawMessage) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.compacting {
		return nil
	}
	id, output := rawResponseToolIdentity(params)
	if id == "" {
		return nil
	}
	if output {
		if _, ok := p.responseCalls[id]; ok {
			p.responseCalls[id] = true
		}
	} else {
		p.responseCalls[id] = false
	}
	if p.streamed && p.pendingTools == 0 && p.responseCallsSettled() {
		return p.endStep(params)
	}
	return nil
}

func (p *projector) rawResponseCompleted(params json.RawMessage) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.compacting {
		return nil
	}
	if p.streamed {
		return errors.New("codex: response completed twice before its step ended")
	}
	var value map[string]any
	if err := json.Unmarshal(params, &value); err != nil {
		return fmt.Errorf("codex: decode raw response completion: %w", err)
	}
	responseID := stringField(value, "responseId", "response_id", "id")
	if responseID == "" {
		return errors.New("codex: rawResponse/completed has no responseId")
	}
	if err := p.ensureStep(responseID, p.nativeRef(params)); err != nil {
		return err
	}
	p.responseID = responseID
	if usage, ok := value["usage"].(map[string]any); ok {
		p.usage = normalizeUsage(usage)
	} else {
		p.usage = normalizedUsage{}
	}
	p.streamed = true
	if err := p.event("session.step.streamed", map[string]any{"assistantMessageID": p.messageID}, p.nativeRef(params)); err != nil {
		return err
	}
	if p.pendingTools == 0 && p.responseCallsSettled() {
		return p.endStep(params)
	}
	return nil
}

func (p *projector) responseCallsSettled() bool {
	for _, settled := range p.responseCalls {
		if !settled {
			return false
		}
	}
	return true
}

// tokenUsageUpdated fills the open step from the turn's most recent model
// call. Only tokenUsage.last is read: tokenUsage.total is cumulative per
// app-server process, so it is never one step's figure. On a turn that
// produced no rawResponse/completed this is the only usage there is.
func (p *projector) tokenUsageUpdated(params json.RawMessage) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.stepOpen || p.usage != (normalizedUsage{}) {
		return nil
	}
	var value struct {
		TokenUsage struct {
			Last map[string]any `json:"last"`
		} `json:"tokenUsage"`
	}
	if err := json.Unmarshal(params, &value); err != nil {
		return fmt.Errorf("codex: decode thread/tokenUsage/updated: %w", err)
	}
	if value.TokenUsage.Last != nil {
		p.usage = normalizeUsage(value.TokenUsage.Last)
	}
	return nil
}

func (p *projector) turnCompleted(params json.RawMessage) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stepOpen {
		// Turn completion is authoritative even when a partial item never
		// received its own completion (for example, an abandoned retry).
		for _, kind := range []string{"text", "reasoning"} {
			for _, part := range p.partQueues[kind] {
				part.done = true
			}
			if err := p.flushParts(kind); err != nil {
				return err
			}
		}
		if p.pendingTools != 0 {
			return fmt.Errorf("codex: turn completed with %d unsettled tools", p.pendingTools)
		}
		return p.endStep(params)
	}
	return nil
}

// harnessError projects the app-server's native error notification. A
// retryable notification keeps the current step open because the app-server,
// not Gimbal, owns the retry. A terminal notification closes that step as a
// failure before RunTurn returns the same native error to Generate.
func (p *projector) harnessError(params json.RawMessage) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	var value struct {
		Error     map[string]any `json:"error"`
		WillRetry bool           `json:"willRetry"`
	}
	if err := json.Unmarshal(params, &value); err != nil {
		return false, fmt.Errorf("codex: decode error notification: %w", err)
	}
	ref := p.nativeRef(params)
	if err := p.ensureStep("", ref); err != nil {
		return value.WillRetry, err
	}
	errorValue := objectValue(value.Error)
	if value.WillRetry {
		return true, p.event("session.retry.scheduled", map[string]any{
			"assistantMessageID": p.messageID,
			"error":              errorValue,
		}, ref)
	}
	err := p.event("session.step.failed", map[string]any{
		"assistantMessageID": p.messageID,
		"finish":             "error",
		"error":              errorValue,
	}, ref)
	p.resetStep()
	return false, err
}

func (p *projector) approvalRequested(method string, params json.RawMessage) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	var value map[string]any
	if err := json.Unmarshal(params, &value); err != nil {
		return "", fmt.Errorf("codex: decode %s: %w", method, err)
	}
	id := stringField(value, "approvalId", "itemId")
	if id == "" {
		id = fmt.Sprintf("%s/request.%d", p.turnID, p.response+1)
	}
	ref := p.nativeRef(params)
	return id, p.event("permission.asked", map[string]any{
		"id":         id,
		"permission": method,
		"metadata":   value,
	}, ref)
}

func (p *projector) approvalReplied(id, decision, method string, params json.RawMessage) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	ref := p.nativeRef(params)
	return p.event("permission.replied", map[string]any{
		"requestID": id,
		"reply":     decision,
	}, ref)
}

func (p *projector) endStep(params json.RawMessage) error {
	if !p.stepOpen {
		return nil
	}
	if len(p.partQueues["text"]) > 0 || len(p.partQueues["reasoning"]) > 0 {
		return nil
	}
	ref := p.nativeRef(params)
	err := p.event("session.step.ended", map[string]any{
		"assistantMessageID": p.messageID, "finish": "unknown", "cost": 0,
		"tokens": map[string]any{
			"input": p.usage.input, "output": p.usage.output, "reasoning": p.usage.reasoning,
			"cache": map[string]any{"read": p.usage.cacheRead, "write": p.usage.cacheWrite},
		},
	}, ref)
	if err != nil {
		return err
	}
	p.resetStep()
	return nil
}

func (p *projector) resetStep() {
	p.stepOpen, p.streamed = false, false
	p.partQueues = make(map[string][]*contentPart)
	p.messageID, p.responseID = "", ""
	p.tools = make(map[string]*toolState)
	p.responseCalls = make(map[string]bool)
	p.pendingTools = 0
}

type nativeItem struct {
	id    string
	kind  string
	value map[string]any
}

func decodeItem(params json.RawMessage) (nativeItem, bool) {
	var envelope struct {
		Item map[string]any `json:"item"`
	}
	if json.Unmarshal(params, &envelope) != nil || envelope.Item == nil {
		return nativeItem{}, false
	}
	id, _ := envelope.Item["id"].(string)
	kind, _ := envelope.Item["type"].(string)
	return nativeItem{id: id, kind: kind, value: envelope.Item}, id != "" && kind != ""
}

func rawResponseToolIdentity(params json.RawMessage) (string, bool) {
	var envelope struct {
		Item map[string]any `json:"item"`
	}
	if json.Unmarshal(params, &envelope) != nil || envelope.Item == nil {
		return "", false
	}
	switch stringField(envelope.Item, "type") {
	case "local_shell_call", "function_call", "tool_search_call", "custom_tool_call":
		return stringField(envelope.Item, "call_id", "callId", "id"), false
	case "function_call_output", "tool_search_output", "custom_tool_call_output":
		return stringField(envelope.Item, "call_id", "callId"), true
	}
	return "", false
}

func deltaText(params json.RawMessage) string {
	var value map[string]any
	_ = json.Unmarshal(params, &value)
	return stringField(value, "delta", "text")
}

func isTool(kind string) bool {
	switch kind {
	case "commandExecution", "fileChange", "mcpToolCall", "dynamicToolCall", "collabAgentToolCall", "subAgentActivity", "webSearch", "imageView", "sleep", "imageGeneration":
		return true
	}
	return false
}

func toolName(item nativeItem) string {
	switch item.kind {
	case "mcpToolCall":
		return fmt.Sprintf("%v/%v", item.value["server"], item.value["tool"])
	case "dynamicToolCall":
		if namespace, _ := item.value["namespace"].(string); namespace != "" {
			return namespace + "/" + fmt.Sprint(item.value["tool"])
		}
		return fmt.Sprint(item.value["tool"])
	}
	return item.kind
}

func toolArgs(item nativeItem) any {
	switch item.kind {
	case "commandExecution":
		return map[string]any{"command": item.value["command"], "cwd": item.value["cwd"]}
	case "fileChange":
		return map[string]any{"changes": item.value["changes"]}
	case "mcpToolCall", "dynamicToolCall":
		return item.value["arguments"]
	case "webSearch":
		return map[string]any{"query": item.value["query"], "action": item.value["action"]}
	}
	return item.value
}

func toolOutput(item nativeItem) any {
	switch item.kind {
	case "commandExecution":
		return item.value["aggregatedOutput"]
	case "fileChange":
		return map[string]any{"status": item.value["status"], "changes": item.value["changes"]}
	case "mcpToolCall":
		if item.value["error"] != nil {
			return item.value["error"]
		}
		return item.value["result"]
	case "dynamicToolCall":
		return map[string]any{"success": item.value["success"], "contentItems": item.value["contentItems"]}
	case "webSearch":
		return item.value["results"]
	}
	return item.value
}

func toolFailure(item nativeItem) (bool, string) {
	if item.value["error"] != nil {
		return true, outputText(item.value["error"])
	}
	if success, ok := item.value["success"].(bool); ok && !success {
		return true, outputText(toolOutput(item))
	}
	status := strings.ToLower(fmt.Sprint(item.value["status"]))
	if status == "failed" || status == "error" || status == "declined" {
		return true, outputText(toolOutput(item))
	}
	return false, ""
}

func outputText(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(raw)
}

func objectValue(value any) map[string]any {
	if object, ok := value.(map[string]any); ok && object != nil {
		return object
	}
	return map[string]any{"value": value}
}

func objectValueOrNil(value any) map[string]any {
	object, _ := value.(map[string]any)
	return object
}

func paramItemID(params json.RawMessage) string {
	var value map[string]any
	_ = json.Unmarshal(params, &value)
	return stringField(value, "itemId")
}

func joined(value any) string {
	values, _ := value.([]any)
	var parts []string
	for _, v := range values {
		if text, ok := v.(string); ok && strings.TrimSpace(text) != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n\n")
}

func stringField(value map[string]any, keys ...string) string {
	for _, key := range keys {
		if text, _ := value[key].(string); text != "" {
			return text
		}
	}
	return ""
}

// normalizeUsage maps Codex's counters onto the five fields every adapter
// reports. The names are camelCase, the only spelling the app-server emits.
// cachedInputTokens and cacheWriteInputTokens are both members of the
// Responses API's input_tokens_details, so both are subsets of inputTokens
// (codex-rs/codex-api/src/sse/responses.rs, tag rust-v0.153.4, whose fixture
// has cached 40 + cache write 60 = input 100); reasoningOutputTokens is a
// subset of outputTokens. Each is subtracted, as OpenCode does. A counter the
// notification omits is zero.
func normalizeUsage(value map[string]any) normalizedUsage {
	get := func(key string) float64 {
		number, _ := value[key].(float64)
		return number
	}
	cached, cacheWrite, reasoning := get("cachedInputTokens"), get("cacheWriteInputTokens"), get("reasoningOutputTokens")
	return normalizedUsage{
		input:      max(0, get("inputTokens")-cached-cacheWrite),
		output:     max(0, get("outputTokens")-reasoning),
		reasoning:  reasoning,
		cacheRead:  cached,
		cacheWrite: cacheWrite,
	}
}
