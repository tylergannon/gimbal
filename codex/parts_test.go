package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
)

func TestReadTurnProjectionMismatchDoesNotLoseNativeAnswer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn := &connection{readDone: make(chan struct{})}
	ch := make(chan rpcMessage, 6)
	for _, value := range []struct{ method, body string }{
		{"item/started", `"item":{"id":"unfinished","type":"reasoning"}`},
		{"rawResponse/completed", `"responseId":"first"`},
		// A later response supersedes an item with no completion. Even if
		// the display projection cannot account for that cut, work continues.
		{"rawResponse/completed", `"responseId":"second"`},
		{"item/completed", `"item":{"id":"answer","type":"agentMessage","text":"actual result"}`},
		{"turn/completed", `"turn":{"id":"turn","status":"completed"}`},
	} {
		ch <- rpcMessage{Method: value.method, Params: json.RawMessage(`{"threadId":"thread","turnId":"turn",` + value.body + `}`)}
	}
	p := newProjector("thread", "turn", "model", func(gimbal.AgentEvent) error { return nil })
	answer, err := readTurn(ctx, conn, ch, "thread", "turn", p)
	if err != nil || answer != "actual result" {
		t.Fatalf("answer=%q, error=%v", answer, err)
	}
}

func TestReadTurnStillReportsProviderAndSinkFailures(t *testing.T) {
	for _, sink := range []bool{false, true} {
		t.Run(fmt.Sprint("sink=", sink), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			boom := errors.New("recording failed")
			p := newProjector("thread", "turn", "model", func(gimbal.AgentEvent) error {
				if sink {
					return boom
				}
				return nil
			})
			ch := make(chan rpcMessage, 1)
			ch <- rpcMessage{Method: "error", Params: json.RawMessage(`{"threadId":"thread","turnId":"turn","error":{"message":"provider failed"},"willRetry":false}`)}
			_, err := readTurn(ctx, &connection{readDone: make(chan struct{})}, ch, "thread", "turn", p)
			if sink && !errors.Is(err, boom) {
				t.Fatalf("sink error lost: %v", err)
			}
			if !sink && (err == nil || err.Error() != "codex: provider failed") {
				t.Fatalf("provider error lost: %v", err)
			}
		})
	}
}

func TestReadTurnLateCompletionCannotReplaceFinalAnswer(t *testing.T) {
	for _, phase := range []string{"", "final_answer"} {
		t.Run(phase, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			ch := make(chan rpcMessage, 8)
			for _, value := range []struct{ method, body string }{
				{"item/started", `"item":{"id":"old","type":"agentMessage"}`},
				{"item/started", `"item":{"id":"new","type":"agentMessage"}`},
				{"item/completed", fmt.Sprintf(`"item":{"id":"new","type":"agentMessage","text":"actual final","phase":%q}`, phase)},
				{"item/completed", `"item":{"id":"old","type":"agentMessage","text":"late old text"}`},
				{"turn/completed", `"turn":{"id":"turn","status":"completed"}`},
			} {
				ch <- rpcMessage{Method: value.method, Params: json.RawMessage(`{"threadId":"thread","turnId":"turn",` + value.body + `}`)}
			}
			p := newProjector("thread", "turn", "model", func(gimbal.AgentEvent) error { return nil })
			answer, err := readTurn(ctx, &connection{readDone: make(chan struct{})}, ch, "thread", "turn", p)
			if err != nil || answer != "actual final" {
				t.Fatalf("answer=%q, error=%v", answer, err)
			}
		})
	}
}

func TestContentItemsTolerateOverlapDuplicatesAndLateStarts(t *testing.T) {
	for _, kind := range []string{"text", "reasoning"} {
		t.Run(kind, func(t *testing.T) {
			var events []gimbal.AgentEvent
			p := newProjector("thread", "turn", "model", func(e gimbal.AgentEvent) error { events = append(events, e); return nil })
			nativeKind := "reasoning"
			delta := p.reasoningDelta
			if kind == "text" {
				nativeKind, delta = "agentMessage", p.textDelta
			}
			start := func(id string) {
				mustProject(t, p.itemStarted(json.RawMessage(fmt.Sprintf(`{"item":{"id":%q,"type":%q}}`, id, nativeKind))))
			}
			complete := func(id, text string) {
				_, _, err := p.itemCompleted(json.RawMessage(fmt.Sprintf(`{"item":{"id":%q,"type":%q,"text":%q,"summary":[%q]}}`, id, nativeKind, text, text)))
				mustProject(t, err)
			}
			// A delta can introduce an item before its explicit start arrives.
			mustProject(t, delta(json.RawMessage(`{"itemId":"a","delta":"partial a"}`)))
			start("a")
			start("a")
			start("b")
			mustProject(t, delta(json.RawMessage(`{"itemId":"b","delta":"partial b"}`)))
			// Completion order differs from start order. Keep both identities.
			complete("b", "complete b")
			mustProject(t, p.rawResponseCompleted(json.RawMessage(`{"responseId":"response"}`)))
			complete("a", "complete a")
			complete("a", "duplicate must not replace a")
			start("b")
			mustProject(t, delta(json.RawMessage(`{"itemId":"b","delta":"late delta"}`)))
			mustProject(t, p.turnCompleted(json.RawMessage(`{"turn":{"status":"completed"}}`)))
			var texts, ids []string
			for _, event := range events {
				if event.Type != "session."+kind+".ended" {
					continue
				}
				var data, ref map[string]any
				decodeData(t, event, &data)
				if err := json.Unmarshal(event.NativeRef, &ref); err != nil {
					t.Fatal(err)
				}
				texts = append(texts, data["text"].(string))
				ids = append(ids, ref["itemID"].(string))
			}
			if !slices.Equal(texts, []string{"complete a", "complete b"}) || !slices.Equal(ids, []string{"a", "b"}) {
				t.Fatalf("completed parts: %v, ids: %v", texts, ids)
			}
			if countType(events, "session."+kind+".started") != 2 || countType(events, "session.step.started") != 1 || countType(events, "session.step.ended") != 1 {
				t.Fatalf("duplicate or unclosed display events: %v", types(events))
			}
		})
	}
}

func TestTurnCompletionRetainsUnfinishedReasoning(t *testing.T) {
	var events []gimbal.AgentEvent
	p := newProjector("thread", "turn", "model", func(e gimbal.AgentEvent) error { events = append(events, e); return nil })
	mustProject(t, p.reasoningDelta(json.RawMessage(`{"itemId":"abandoned","delta":"retained partial"}`)))
	mustProject(t, p.itemStarted(json.RawMessage(`{"item":{"id":"replacement","type":"reasoning"}}`)))
	_, _, err := p.itemCompleted(json.RawMessage(`{"item":{"id":"replacement","type":"reasoning","summary":["replacement complete"]}}`))
	mustProject(t, err)
	mustProject(t, p.turnCompleted(json.RawMessage(`{"turn":{"status":"completed"}}`)))
	if countType(events, "session.reasoning.ended") != 2 || countType(events, "session.step.ended") != 1 {
		t.Fatalf("unfinished part lost or turn failed: %v", types(events))
	}
	var data map[string]any
	decodeData(t, firstType(t, events, "session.reasoning.ended"), &data)
	if data["text"] != "retained partial" {
		t.Fatalf("partial content lost: %v", data)
	}
}
