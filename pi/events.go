package pi

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/tylergannon/gimbal"
)

type turn struct {
	mu      sync.Mutex
	session string
	model   string
	emit    func(gimbal.AgentEvent) error
	settled chan struct{}
	final   *assistantMessage
	usage   map[string]gimbal.Usage
	err     error
	closed  bool
}

type assistantMessage struct {
	Role       string         `json:"role"`
	Content    []contentBlock `json:"content"`
	StopReason string         `json:"stopReason"`
	Error      string         `json:"errorMessage"`
	Provider   string         `json:"provider"`
	Model      string         `json:"model"`
	Usage      piUsage        `json:"usage"`
}

type contentBlock struct {
	Type string          `json:"type"`
	Text string          `json:"text"`
	ID   string          `json:"id"`
	Name string          `json:"name"`
	Args json.RawMessage `json:"arguments"`
}

type piUsage struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite"`
	Cost       struct {
		Total float64 `json:"total"`
	} `json:"cost"`
}

func newTurn(sessionID, model string, emit func(gimbal.AgentEvent) error) *turn {
	return &turn{session: sessionID, model: model, emit: emit, settled: make(chan struct{}), usage: make(map[string]gimbal.Usage)}
}

func (t *turn) record(record wireRecord) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return
	}
	switch record.Type {
	case "message_update":
		var update struct {
			Type      string `json:"type"`
			Delta     string `json:"delta"`
			ToolID    string `json:"id"`
			ToolName  string `json:"toolName"`
			ContentID int    `json:"contentIndex"`
		}
		_ = json.Unmarshal(record.Event, &update)
		switch update.Type {
		case "text_delta":
			t.emitEvent("session.text.delta", map[string]any{"sessionID": t.session, "ordinal": update.ContentID, "delta": update.Delta})
		case "toolcall_start":
			t.emitEvent("session.tool.input.started", map[string]any{"sessionID": t.session, "id": update.ToolID, "name": update.ToolName})
		}
	case "tool_execution_start":
		input, _ := json.Marshal(record.Args)
		t.emitEvent("session.tool.input.ended", map[string]any{"sessionID": t.session, "id": record.ToolID, "text": string(input)})
		t.emitEvent("session.tool.called", map[string]any{"sessionID": t.session, "id": record.ToolID, "name": record.ToolName, "input": record.Args, "executed": true})
	case "tool_execution_end":
		if record.IsError {
			t.emitEvent("session.tool.failed", map[string]any{"sessionID": t.session, "id": record.ToolID, "name": record.ToolName, "error": record.Result})
		} else {
			t.emitEvent("session.tool.success", map[string]any{"sessionID": t.session, "id": record.ToolID, "name": record.ToolName, "result": record.Result})
		}
	case "message_end":
		var message assistantMessage
		if json.Unmarshal(record.Message, &message) != nil || message.Role != "assistant" {
			return
		}
		copy := message
		t.final = &copy
		model := message.Model
		if model == "" {
			model = t.model
		}
		usage := gimbal.Usage{Cost: message.Usage.Cost.Total}
		usage.Tokens.Input = message.Usage.Input
		usage.Tokens.Output = message.Usage.Output
		usage.Tokens.Cache.Read = message.Usage.CacheRead
		usage.Tokens.Cache.Write = message.Usage.CacheWrite
		t.usage[model] = addUsage(t.usage[model], usage)
	case "agent_settled":
		t.closed = true
		close(t.settled)
	case "agent_end":
		var data struct {
			Messages []assistantMessage `json:"messages"`
		}
		_ = json.Unmarshal(record.Message, &data)
		for i := range data.Messages {
			if data.Messages[i].Role == "assistant" && data.Messages[i].Error != "" {
				t.err = fmt.Errorf("pi: provider error: %s", data.Messages[i].Error)
			}
		}
	}
}

func (t *turn) emitEvent(kind string, data any) {
	if t.emit == nil {
		return
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return
	}
	ref, err := json.Marshal(map[string]string{"provider": "pi", "sessionID": t.session})
	if err != nil {
		return
	}
	_ = t.emit(gimbal.AgentEvent{Type: kind, Data: payload, NativeRef: ref})
}

func addUsage(a, b gimbal.Usage) gimbal.Usage {
	a.Cost += b.Cost
	a.Tokens.Input += b.Tokens.Input
	a.Tokens.Output += b.Tokens.Output
	a.Tokens.Reasoning += b.Tokens.Reasoning
	a.Tokens.Cache.Read += b.Tokens.Cache.Read
	a.Tokens.Cache.Write += b.Tokens.Cache.Write
	return a
}
