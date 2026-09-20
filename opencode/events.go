package opencode

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/tylergannon/gimble"
)

type projector struct {
	mu        sync.Mutex
	sessionID string
	provider  string
	model     string
	emit      func(gimble.AgentEvent) error
	steps     map[string]*projectedStep
	parts     map[string]*projectedPart
}

type projectedStep struct {
	started bool
	ended   bool
}

type projectedPart struct {
	messageID string
	kind      string
	name      string
	text      string
	started   bool
	called    bool
	ended     bool
}

func newProjector(sessionID, provider, model string, emit func(gimble.AgentEvent) error) *projector {
	if emit == nil {
		emit = func(gimble.AgentEvent) error { return nil }
	}
	return &projector{
		sessionID: sessionID, provider: provider, model: model, emit: emit,
		steps: make(map[string]*projectedStep), parts: make(map[string]*projectedPart),
	}
}

func (p *projector) event(eventType, eventID string, payload json.RawMessage) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	var event struct {
		Properties json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("opencode: decode %s: %w", eventType, err)
	}
	switch eventType {
	case "message.updated":
		return p.messageUpdated(eventID, event.Properties)
	case "message.part.updated":
		return p.partUpdated(eventID, event.Properties)
	case "message.part.delta":
		return p.partDelta(eventID, event.Properties)
	default:
		return nil
	}
}

func (p *projector) messageUpdated(eventID string, properties json.RawMessage) error {
	var update struct {
		Info json.RawMessage `json:"info"`
	}
	if err := json.Unmarshal(properties, &update); err != nil {
		return err
	}
	var info struct {
		ID   string `json:"id"`
		Role string `json:"role"`
	}
	if err := json.Unmarshal(update.Info, &info); err != nil || info.Role != "assistant" {
		return err
	}
	return p.ensureStep(info.ID, eventID)
}

func (p *projector) partUpdated(eventID string, properties json.RawMessage) error {
	var update struct {
		Part json.RawMessage `json:"part"`
	}
	if err := json.Unmarshal(properties, &update); err != nil {
		return err
	}
	return p.projectPart(eventID, update.Part)
}

func (p *projector) partDelta(eventID string, properties json.RawMessage) error {
	var delta struct {
		SessionID string `json:"sessionID"`
		MessageID string `json:"messageID"`
		PartID    string `json:"partID"`
		Field     string `json:"field"`
		Delta     string `json:"delta"`
	}
	if err := json.Unmarshal(properties, &delta); err != nil {
		return err
	}
	if delta.Field != "text" || delta.MessageID == "" || delta.PartID == "" {
		return nil
	}
	if err := p.ensureStep(delta.MessageID, eventID); err != nil {
		return err
	}
	part := p.part(delta.PartID, delta.MessageID, "text")
	if err := p.startContent(part, eventID); err != nil {
		return err
	}
	part.text += delta.Delta
	return p.emitEvent("session."+part.kind+".delta", map[string]any{
		"assistantMessageID": delta.MessageID, "ordinal": 0, "delta": delta.Delta,
	}, p.ref(delta.MessageID, delta.PartID, eventID))
}

func (p *projector) projectPart(eventID string, raw json.RawMessage) error {
	var part struct {
		ID        string          `json:"id"`
		MessageID string          `json:"messageID"`
		Type      string          `json:"type"`
		Text      string          `json:"text"`
		Time      map[string]any  `json:"time"`
		Tool      string          `json:"tool"`
		State     json.RawMessage `json:"state"`
		Cost      float64         `json:"cost"`
		Tokens    map[string]any  `json:"tokens"`
		Reason    string          `json:"reason"`
	}
	if err := json.Unmarshal(raw, &part); err != nil {
		return err
	}
	if part.MessageID == "" {
		return nil
	}
	switch part.Type {
	case "step-start":
		return p.ensureStep(part.MessageID, eventID)
	case "text", "reasoning":
		if err := p.ensureStep(part.MessageID, eventID); err != nil {
			return err
		}
		state := p.part(part.ID, part.MessageID, part.Type)
		if err := p.startContent(state, eventID); err != nil {
			return err
		}
		if after, ok := strings.CutPrefix(part.Text, state.text); ok {
			delta := after
			if delta != "" {
				state.text += delta
				if err := p.emitEvent("session."+part.Type+".delta", map[string]any{
					"assistantMessageID": part.MessageID, "ordinal": 0, "delta": delta,
				}, p.ref(part.MessageID, part.ID, eventID)); err != nil {
					return err
				}
			}
		}
		if part.Time != nil && part.Time["end"] != nil {
			return p.endContent(state, eventID)
		}
		return nil
	case "tool":
		if err := p.ensureStep(part.MessageID, eventID); err != nil {
			return err
		}
		return p.projectTool(eventID, part.ID, part.MessageID, part.Tool, part.State)
	case "step-finish":
		if err := p.ensureStep(part.MessageID, eventID); err != nil {
			return err
		}
		return p.endStep(part.MessageID, part.Cost, part.Tokens, part.Reason, eventID, nil)
	default:
		return nil
	}
}

func (p *projector) projectTool(eventID, partID, messageID, name string, raw json.RawMessage) error {
	var state struct {
		Status string         `json:"status"`
		Input  map[string]any `json:"input"`
		Output string         `json:"output"`
		Error  string         `json:"error"`
		Title  string         `json:"title"`
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return err
	}
	part := p.part(partID, messageID, "tool")
	if name != "" {
		part.name = name
	}
	ref := p.ref(messageID, partID, eventID)
	if !part.started {
		part.started = true
		if err := p.emitEvent("session.tool.input.started", map[string]any{
			"assistantMessageID": messageID, "id": partID, "name": part.name,
		}, ref); err != nil {
			return err
		}
	}
	if state.Status == "pending" {
		return nil
	}
	if !part.called {
		part.called = true
		input, _ := json.Marshal(state.Input)
		if err := p.emitEvent("session.tool.input.ended", map[string]any{
			"assistantMessageID": messageID, "id": partID, "text": string(input),
		}, ref); err != nil {
			return err
		}
		if err := p.emitEvent("session.tool.called", map[string]any{
			"assistantMessageID": messageID, "id": partID, "input": state.Input, "executed": true,
		}, ref); err != nil {
			return err
		}
	}
	if part.ended {
		return nil
	}
	switch state.Status {
	case "completed":
		part.ended = true
		return p.emitEvent("session.tool.success", map[string]any{
			"assistantMessageID": messageID, "id": partID, "executed": true,
			"content": []any{map[string]any{"type": "text", "text": state.Output}},
		}, ref)
	case "error":
		part.ended = true
		return p.emitEvent("session.tool.failed", map[string]any{
			"assistantMessageID": messageID, "id": partID, "executed": true,
			"error": map[string]any{"type": "tool", "message": state.Error},
		}, ref)
	default:
		return nil
	}
}

func (p *projector) finish(response PromptResponse) {
	p.mu.Lock()
	defer p.mu.Unlock()
	infoRaw, _ := json.Marshal(response.Info)
	_ = p.messageUpdated("post-response", mustProperties("info", infoRaw))
	for _, part := range response.Parts {
		raw, err := json.Marshal(part)
		if err == nil {
			_ = p.projectPart("post-response", raw)
		}
	}
	for _, part := range p.parts {
		if part.messageID == response.Info.Id && (part.kind == "text" || part.kind == "reasoning") {
			_ = p.endContent(part, "post-response")
		}
	}
	step := p.step(response.Info.Id)
	if step.ended {
		return
	}
	tokens := map[string]any{
		"input": response.Info.Tokens.Input, "output": response.Info.Tokens.Output,
		"reasoning": response.Info.Tokens.Reasoning,
		"cache": map[string]any{
			"read": response.Info.Tokens.Cache.Read, "write": response.Info.Tokens.Cache.Write,
		},
	}
	if response.Info.Error != nil {
		raw, _ := json.Marshal(response.Info.Error)
		_ = p.failStep(response.Info.Id, float64(response.Info.Cost), tokens, string(raw), "post-response")
		return
	}
	finish := "stop"
	if response.Info.Finish != nil {
		finish = *response.Info.Finish
	}
	_ = p.endStep(response.Info.Id, float64(response.Info.Cost), tokens, finish, "post-response", nil)
}

func (p *projector) ensureStep(messageID, eventID string) error {
	if messageID == "" {
		return nil
	}
	step := p.step(messageID)
	if step.started {
		return nil
	}
	step.started = true
	return p.emitEvent("session.step.started", map[string]any{
		"assistantMessageID": messageID,
		"agent":              "opencode",
		"model":              map[string]any{"providerID": p.provider, "id": p.model},
	}, p.ref(messageID, "", eventID))
}

func (p *projector) startContent(part *projectedPart, eventID string) error {
	if part.started {
		return nil
	}
	part.started = true
	return p.emitEvent("session."+part.kind+".started", map[string]any{
		"assistantMessageID": part.messageID, "ordinal": 0,
	}, p.ref(part.messageID, partID(part), eventID))
}

func (p *projector) endContent(part *projectedPart, eventID string) error {
	if part.ended || !part.started {
		return nil
	}
	part.ended = true
	return p.emitEvent("session."+part.kind+".ended", map[string]any{
		"assistantMessageID": part.messageID, "ordinal": 0, "text": part.text,
	}, p.ref(part.messageID, partID(part), eventID))
}

func (p *projector) endStep(messageID string, cost float64, tokens map[string]any, finish, eventID string, native any) error {
	step := p.step(messageID)
	if step.ended {
		return nil
	}
	for _, part := range p.parts {
		if part.messageID == messageID && (part.kind == "text" || part.kind == "reasoning") {
			if err := p.endContent(part, eventID); err != nil {
				return err
			}
		}
	}
	var ref any = p.ref(messageID, "", eventID)
	if native != nil {
		ref = native
	}
	if err := p.emitEvent("session.step.streamed", map[string]any{"assistantMessageID": messageID}, ref); err != nil {
		return err
	}
	if tokens == nil {
		tokens = emptyTokens()
	}
	if err := p.emitEvent("session.step.ended", map[string]any{
		"assistantMessageID": messageID, "finish": finish, "cost": cost, "tokens": tokens,
	}, ref); err != nil {
		return err
	}
	step.ended = true
	return nil
}

func (p *projector) failStep(messageID string, cost float64, tokens map[string]any, message, eventID string) error {
	step := p.step(messageID)
	if step.ended {
		return nil
	}
	if tokens == nil {
		tokens = emptyTokens()
	}
	if err := p.emitEvent("session.step.failed", map[string]any{
		"assistantMessageID": messageID, "cost": cost, "tokens": tokens,
		"error": map[string]any{"type": "opencode", "message": message},
	}, p.ref(messageID, "", eventID)); err != nil {
		return err
	}
	step.ended = true
	return nil
}

func (p *projector) emitEvent(eventType string, data map[string]any, native any) error {
	data["sessionID"] = p.sessionID
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	ref, err := json.Marshal(native)
	if err != nil {
		return err
	}
	return p.emit(gimble.AgentEvent{Type: eventType, Data: raw, NativeRef: ref})
}

func (p *projector) ref(messageID, itemID, eventID string) map[string]any {
	ref := map[string]any{"provider": "opencode", "sessionID": p.sessionID}
	if messageID != "" {
		ref["messageID"] = messageID
	}
	if itemID != "" {
		ref["itemID"] = itemID
	}
	return ref
}

func (p *projector) step(messageID string) *projectedStep {
	step := p.steps[messageID]
	if step == nil {
		step = &projectedStep{}
		p.steps[messageID] = step
	}
	return step
}

func (p *projector) part(id, messageID, kind string) *projectedPart {
	part := p.parts[id]
	if part == nil {
		part = &projectedPart{messageID: messageID, kind: kind, name: id}
		p.parts[id] = part
	}
	return part
}

func partID(part *projectedPart) string {
	if part.kind == "tool" {
		return part.name
	}
	return ""
}

func emptyTokens() map[string]any {
	return map[string]any{
		"input": 0, "output": 0, "reasoning": 0,
		"cache": map[string]any{"read": 0, "write": 0},
	}
}

func mustProperties(key string, value json.RawMessage) json.RawMessage {
	raw, _ := json.Marshal(map[string]json.RawMessage{key: value})
	return raw
}
