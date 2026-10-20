package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/workflows/implementation"

	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/runstore"
)

// This isolated daemon speaks the real WebSocket JSON-RPC transport. A tool
// effect is written before the socket fails; native history survives redial.
type recoveryDaemon struct {
	mu                                                              sync.Mutex
	threads                                                         map[string]*fakeThread
	sockets                                                         []*websocket.Conn
	mode                                                            string
	fullReads                                                       int
	coding, validations, planning, starts, effects, dials, archives int
	outage                                                          bool
	effectFile                                                      string
	injected                                                        bool
	ignoreInterrupt, completeStalled                                bool
	interrupts                                                      int
}
type fakeThread struct {
	model  string
	status string
	turns  []nativeTurn
	finish bool
}

func newRecoveryDaemon(t *testing.T, mode string) (*adapter, *recoveryDaemon) {
	t.Helper()
	d := &recoveryDaemon{threads: make(map[string]*fakeThread), mode: mode, effectFile: filepath.Join(t.TempDir(), "effects")}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = ws.CloseNow() }()
		d.mu.Lock()
		d.sockets = append(d.sockets, ws)
		d.mu.Unlock()
		for {
			_, raw, err := ws.Read(r.Context())
			if err != nil {
				return
			}
			var message rpcMessage
			if json.Unmarshal(raw, &message) != nil {
				return
			}
			if message.Method == "initialized" {
				continue
			}
			var p map[string]any
			_ = json.Unmarshal(message.Params, &p)
			id, _ := p["threadId"].(string)
			var result any = map[string]any{}
			disconnect, finish := false, false
			d.mu.Lock()
			thread := d.threads[id]
			if thread != nil && thread.status == "broken" && (message.Method == "thread/read" || message.Method == "thread/resume") {
				d.mu.Unlock()
				_ = fakeSend(ws, map[string]any{"id": message.ID, "error": map[string]any{"code": -32600, "message": "native rollout unavailable"}})
				continue
			}
			switch message.Method {
			case "initialize":
			case "thread/start":
				id = fmt.Sprintf("thread-%d", len(d.threads)+1)
				model, _ := p["model"].(string)
				d.threads[id] = &fakeThread{model: model, status: "idle"}
				result = map[string]any{"thread": map[string]any{"id": id}}
			case "thread/read":
				result = map[string]any{"thread": map[string]any{"id": id, "path": "/sessions/owned.jsonl", "status": map[string]string{"type": thread.status}, "turns": []nativeTurn{}}}
			case "thread/turns/list":
				turns := append([]nativeTurn(nil), thread.turns...)
				if p["itemsView"] == "notLoaded" {
					for i := range turns {
						turns[i].Items = nil
					}
				}
				result = map[string]any{"data": turns, "nextCursor": nil}
				if p["itemsView"] == "full" {
					d.fullReads++
					if thread.finish {
						finish = true
						thread.finish = false
					}
				}
			case "thread/resume":
				result = map[string]any{"thread": map[string]any{"id": id, "status": map[string]string{"type": thread.status}, "turns": thread.turns}}
			case "turn/start":
				d.starts++
				var in []struct {
					Text string `json:"text"`
				}
				b, _ := json.Marshal(p["input"])
				_ = json.Unmarshal(b, &in)
				prompt := in[0].Text
				turn := nativeTurn{ID: fmt.Sprintf("turn-%d", d.starts), Status: "inProgress", Items: []json.RawMessage{json.RawMessage(mustEncode(map[string]any{"type": "userMessage", "content": []map[string]string{{"text": prompt}}}))}}
				thread.turns = append(thread.turns, turn)
				thread.status = "active"
				result = map[string]any{"turn": map[string]string{"id": turn.ID}}
				if thread.model == "coding" {
					d.coding++
					if !strings.Contains(prompt, "Inspect the existing edits") {
						d.effects++
						file, err := os.OpenFile(d.effectFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
						if err != nil {
							panic(err)
						}
						_, _ = fmt.Fprintln(file, d.effects)
						_ = file.Close()
					}
					if !d.injected && d.coding == 2 {
						d.injected = true
						disconnect = true
						switch d.mode {
						case "survive":
							thread.finish = true
						case "status-lag":
							thread.status = "idle"
							thread.finish = true
						case "restart", "restart-stale":
							thread.turns[len(thread.turns)-1].Status = "interrupted"
							thread.status = "idle"
							if d.mode == "restart-stale" {
								thread.turns[len(thread.turns)-1].Status = "inProgress"
							}
						case "outage", "cancel":
							d.outage = true
						default:
							thread.turns[len(thread.turns)-1].Status = "completed"
							thread.turns[len(thread.turns)-1].Items = append(thread.turns[len(thread.turns)-1].Items, json.RawMessage(`{"id":"answer","type":"agentMessage","phase":"final_answer","text":"implemented"}`))
							thread.status = "idle"
						}
					} else {
						finish = true
					}
				} else if (thread.model != "stalled" && thread.model != "chat") || d.completeStalled {
					finish = true
				}
			case "thread/archive":
				d.archives++
				thread.status = "notLoaded"
			case "turn/interrupt":
				d.interrupts++
				if !d.ignoreInterrupt {
					thread.status = "idle"
					if len(thread.turns) > 0 {
						thread.turns[len(thread.turns)-1].Status = "interrupted"
					}
				}
			}
			mode := d.mode
			d.mu.Unlock()
			if !disconnect || mode != "ack" {
				if err := fakeSend(ws, map[string]any{"id": message.ID, "result": result}); err != nil {
					return
				}
			}
			if disconnect {
				if mode != "ack" {
					_ = fakeSend(ws, map[string]any{"method": "item/started", "params": map[string]any{"threadId": id, "turnId": turnIDFromResult(result), "item": map[string]any{"id": "effect", "type": "commandExecution", "command": "append effect", "status": "inProgress"}}})
				}
				_ = ws.CloseNow()
				return
			}
			if finish {
				d.finish(ws, id)
			}
		}
	}))
	t.Cleanup(func() {
		d.mu.Lock()
		sockets := append([]*websocket.Conn(nil), d.sockets...)
		d.mu.Unlock()
		for _, ws := range sockets {
			_ = ws.CloseNow()
		}
		server.Close()
	})
	ad := New().(*adapter)
	ad.recoveryDelay = 0
	ad.dial = func(ctx context.Context, _ bool) (*connection, error) {
		d.mu.Lock()
		d.dials++
		outage := d.outage
		d.mu.Unlock()
		if outage {
			return nil, errors.New("isolated daemon unavailable")
		}
		ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
		if err != nil {
			return nil, err
		}
		conn := &connection{ws: ws, pending: make(map[int64]chan rpcMessage), threads: make(map[string]chan rpcMessage), readDone: make(chan struct{})}
		go conn.read()
		_, err = conn.call(ctx, "initialize", map[string]any{})
		if err != nil {
			_ = ws.CloseNow()
			return nil, err
		}
		return conn, nil
	}
	return ad, d
}
func mustEncode(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}
func fakeSend(ws *websocket.Conn, v any) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return ws.Write(ctx, websocket.MessageText, []byte(mustEncode(v)))
}
func turnIDFromResult(v any) string { return v.(map[string]any)["turn"].(map[string]string)["id"] }
func (d *recoveryDaemon) finish(ws *websocket.Conn, id string) {
	d.mu.Lock()
	thread := d.threads[id]
	turn := &thread.turns[len(thread.turns)-1]
	answer := "implemented"
	switch thread.model {
	case "planning":
		d.planning++
		answer = `{"tasks":[{"name":"implement","description":"complete this outcome","definition_of_done":"working","validation":{"command":"","query":"observe"}}],"next":0}`
	case "chat":
		answer = mustEncode(map[string]any{"decision": map[string]string{"type": "chat", "message": "reply from " + turn.ID}})
	case "qa":
		d.validations++
		answer = `{"validation_passed":true,"observed":"inspected existing effect","substantial_gaps":[],"small_gaps":[]}`
	}
	item := json.RawMessage(mustEncode(map[string]any{"id": "answer-" + turn.ID, "type": "agentMessage", "phase": "final_answer", "text": answer}))
	turn.Items = append(turn.Items, item)
	turn.Status = "completed"
	thread.status = "idle"
	tid := turn.ID
	d.mu.Unlock()
	_ = fakeSend(ws, map[string]any{"method": "item/completed", "params": map[string]any{"threadId": id, "turnId": tid, "item": item}})
	_ = fakeSend(ws, map[string]any{"method": "turn/completed", "params": map[string]any{"threadId": id, "turn": map[string]string{"id": tid, "status": "completed"}}})
}

func TestImplementRecoversActiveCodexTurnWithoutReplayingOutcomes(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "test-key")
	for _, mode := range []string{"survive", "status-lag", "completed", "ack", "restart", "restart-stale", "outage"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			ad, d := newRecoveryDaemon(t, mode)
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			dir := t.TempDir()
			path := filepath.Join(dir, "outcomes.json")
			if err := os.WriteFile(path, []byte(`["first","second","third"]`), 0600); err != nil {
				t.Fatal(err)
			}
			result := make(chan error, 1)
			go func() {
				result <- gimbal.Run(gimbal.Project(ctx, dir), string(runstore.NewRunID()), "implement-recovery", map[gimbal.WorkflowRole]gimbal.ModelBinding{
					gimbal.RoleSprintPlanning: {Adapter: ad, Model: "planning"}, "coding": {Adapter: ad, Model: "coding"}, gimbal.RoleQAOrchestration: {Adapter: ad, Model: "qa"}, gimbal.RoleArchitecturalCritique: {Adapter: ad, Model: "coach"},
				}, func(ctx context.Context) error {
					return implementation.Implement(ctx, gimbal.Env{WorkDir: dir}, implementation.Params{OutcomesFile: path, MaxTasksPerOutcome: 1})
				})
			}()
			if strings.HasPrefix(mode, "restart") || mode == "outage" {
				id := waitPaused(t, ctx, ad)
				d.mu.Lock()
				d.outage = false
				if mode == "outage" {
					thread := d.threads[id]
					thread.turns[len(thread.turns)-1].Status = "completed"
					thread.turns[len(thread.turns)-1].Items = append(thread.turns[len(thread.turns)-1].Items, json.RawMessage(`{"type":"agentMessage","text":"implemented","phase":"final_answer"}`))
					thread.status = "idle"
				}
				d.mu.Unlock()
				landed, err := ad.Steer(ctx, id, "resume")
				if err != nil || !landed {
					t.Fatalf("resume=%v,%v", landed, err)
				}
			}
			if err := <-result; err != nil {
				t.Fatal(err)
			}
			d.mu.Lock()
			coding, qa, plans, effects, fullReads := d.coding, d.validations, d.planning, d.effects, d.fullReads
			d.mu.Unlock()
			if mode == "status-lag" && fullReads < 2 {
				t.Fatalf("lagging state was not re-read: %d", fullReads)
			}
			wantCoding := 3
			if strings.HasPrefix(mode, "restart") {
				wantCoding = 4
			}
			if coding != wantCoding || qa != 3 || plans != 3 || effects != 3 {
				t.Fatalf("coding=%d QA=%d planning=%d effects=%d", coding, qa, plans, effects)
			}
			bytes, err := os.ReadFile(d.effectFile)
			if err != nil || string(bytes) != "1\n2\n3\n" {
				t.Fatalf("effects=%q,%v", bytes, err)
			}
		})
	}
}

func waitPaused(t *testing.T, ctx context.Context, ad *adapter) string {
	t.Helper()
	for {
		ad.mu.Lock()
		sessions := make(map[string]*session, len(ad.sessions))
		maps.Copy(sessions, ad.sessions)
		ad.mu.Unlock()
		for id, s := range sessions {
			if active := s.getActive(); active != nil {
				active.mu.Lock()
				paused := active.paused
				active.mu.Unlock()
				if paused {
					return id
				}
			}
		}
		select {
		case <-ctx.Done():
			t.Fatal("recovery did not pause")
			return ""
		case <-time.After(time.Millisecond):
		}
	}
}

func TestPersistentOutageStopsRetryingAndCancellationRetainsOwnership(t *testing.T) {
	ad, d := newRecoveryDaemon(t, "cancel")
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	id, err := ad.CreateSession(effects.WithExecutionIdentity(ctx, backend.NewExecutionIdentity("session")), "coding", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ad.RunTurn(effects.WithExecutionIdentity(ctx, backend.NewExecutionIdentity("turn")), id, "first", nil, func(gimbal.AgentEvent) error { return nil }); err != nil {
		t.Fatal(err)
	}
	turnCtx, stop := context.WithCancel(ctx)
	result := make(chan error, 1)
	go func() {
		_, err := ad.RunTurn(effects.WithExecutionIdentity(turnCtx, backend.NewExecutionIdentity("turn")), id, "second", nil, func(gimbal.AgentEvent) error { return nil })
		result <- err
	}()
	_ = waitPaused(t, ctx, ad)
	d.mu.Lock()
	dials := d.dials
	d.mu.Unlock()
	time.Sleep(30 * time.Millisecond)
	d.mu.Lock()
	after := d.dials
	d.mu.Unlock()
	if after != dials || dials != 1+reconnectAttempts {
		t.Fatalf("dial count=%d -> %d", dials, after)
	}
	stop()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel=%v", err)
	} else {
		if _, ok := errors.AsType[*backend.DeliveryUncertain](err); !ok {
			t.Fatalf("unconfirmed stop lost uncertainty: %v", err)
		}
	}
	if err := ad.Close(effects.WithExecutionIdentity(ctx, backend.NewExecutionIdentity("close")), id); err == nil {
		t.Fatal("unavailable cleanup reported success")
	}
	ad.mu.Lock()
	_, retained := ad.sessions[id]
	ad.mu.Unlock()
	if !retained {
		t.Fatal("lost owned session")
	}
	d.mu.Lock()
	d.outage = false
	coding := d.coding
	d.mu.Unlock()
	if err := ad.Close(effects.WithExecutionIdentity(ctx, backend.NewExecutionIdentity("close")), id); err != nil {
		t.Fatal(err)
	}
	if err := ad.Close(effects.WithExecutionIdentity(ctx, backend.NewExecutionIdentity("close")), id); err != nil {
		t.Fatal(err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.archives != 1 || d.coding != coding {
		t.Fatalf("archives=%d coding=%d", d.archives, d.coding)
	}
}

func TestNormalTurnSnapshotOmitsPriorToolOutput(t *testing.T) {
	ad, _ := newRecoveryDaemon(t, "survive")
	id, err := ad.CreateSession(effects.WithExecutionIdentity(t.Context(), backend.NewExecutionIdentity("session")), "coding", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ad.RunTurn(effects.WithExecutionIdentity(t.Context(), backend.NewExecutionIdentity("turn")), id, "first", nil, func(gimbal.AgentEvent) error { return nil }); err != nil {
		t.Fatal(err)
	}
	before, err := readThread(t.Context(), ad.current(), id, "notLoaded")
	if err != nil {
		t.Fatal(err)
	}
	if len(before.Turns) != 1 || before.Turns[0].ID == "" || len(before.Turns[0].Items) != 0 {
		t.Fatalf("pre-start ids = %+v", before)
	}
}

func TestClosingUnresumableSessionDoesNotBlockHealthyRedial(t *testing.T) {
	ad, d := newRecoveryDaemon(t, "survive")
	bad, err := ad.CreateSession(effects.WithExecutionIdentity(t.Context(), backend.NewExecutionIdentity("session")), "coding", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	good, err := ad.CreateSession(effects.WithExecutionIdentity(t.Context(), backend.NewExecutionIdentity("session")), "coding", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if err := ad.Close(effects.WithExecutionIdentity(cancelled, backend.NewExecutionIdentity("close")), bad); err == nil {
		t.Fatal("expected retained cleanup failure")
	}
	d.mu.Lock()
	d.threads[bad].status = "broken"
	d.mu.Unlock()
	dead := ad.current()
	_ = dead.ws.CloseNow()
	<-dead.readDone
	if _, err := ad.RunTurn(effects.WithExecutionIdentity(t.Context(), backend.NewExecutionIdentity("turn")), good, "first", nil, func(gimbal.AgentEvent) error { return nil }); err != nil {
		t.Fatalf("healthy turn = %v", err)
	}
	if err := ad.Close(effects.WithExecutionIdentity(t.Context(), backend.NewExecutionIdentity("close")), bad); err != nil {
		t.Fatalf("cleanup retry = %v", err)
	}
	ad.mu.Lock()
	_, retained := ad.sessions[bad]
	ad.mu.Unlock()
	if retained {
		t.Fatal("successful archive retained failed thread")
	}
}

func TestRecoveredFinalAnswerIsNotReplacedByCommentary(t *testing.T) {
	conn := &connection{readDone: make(chan struct{})}
	ch := make(chan rpcMessage, 2)
	ch <- rpcMessage{Method: "item/completed", Params: json.RawMessage(mustEncode(map[string]any{"threadId": "thread-1", "turnId": "turn-1", "item": map[string]any{"id": "commentary", "type": "agentMessage", "phase": "commentary", "text": "late progress"}}))}
	ch <- rpcMessage{Method: "turn/completed", Params: json.RawMessage(mustEncode(map[string]any{"threadId": "thread-1", "turn": map[string]any{"id": "turn-1", "status": "completed"}}))}
	seed, final := nativeAnswer(nativeTurn{Items: []json.RawMessage{json.RawMessage(`{"type":"agentMessage","phase":"final_answer","text":"native final"}`)}})
	answer, err := readTurnFrom(t.Context(), conn, ch, "thread-1", "turn-1", newProjector("thread-1", "turn-1", "coding", func(gimbal.AgentEvent) error { return nil }), seed, final)
	if err != nil || answer != "native final" {
		t.Fatalf("answer = %q, %v", answer, err)
	}
}

// Codex may record an interrupted turn without delivering its terminal
// notification. The thread history is then the confirmation, and Stop must not
// leave the conversation unresolved.
func TestStopConfirmedByNativeHistoryWithoutTerminalNotification(t *testing.T) {
	shortenControlTimeout(t)
	ad, d := newRecoveryDaemon(t, "default")
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	id, err := ad.CreateSession(effects.WithExecutionIdentity(ctx, backend.NewExecutionIdentity("session")), "stalled", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	turnCtx, stop := context.WithCancel(ctx)
	result := make(chan error, 1)
	go func() {
		_, err := ad.RunTurn(effects.WithExecutionIdentity(turnCtx, backend.NewExecutionIdentity("turn")), id, "sleep", nil, func(gimbal.AgentEvent) error { return nil })
		result <- err
	}()
	for {
		d.mu.Lock()
		started := d.starts
		d.mu.Unlock()
		if started == 1 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	stop()
	err = <-result
	if _, uncertain := errors.AsType[*backend.DeliveryUncertain](err); uncertain || !errors.Is(err, context.Canceled) {
		t.Fatalf("confirmed stop reported %v", err)
	}
}

func shortenControlTimeout(t *testing.T) {
	t.Helper()
	old := controlTimeout
	oldPoll := busyPoll
	controlTimeout, busyPoll = 200*time.Millisecond, 10*time.Millisecond
	t.Cleanup(func() { controlTimeout, busyPoll = old, oldPoll })
}

// stopStalledTurn starts a turn the daemon acknowledges but never advances and
// stops it, returning the adapter's report.
func stopStalledTurn(t *testing.T, ctx context.Context, ad *adapter, d *recoveryDaemon, id string, prompt string, want int) error {
	t.Helper()
	turnCtx, stop := context.WithCancel(ctx)
	defer stop()
	result := make(chan error, 1)
	go func() {
		_, err := ad.RunTurn(effects.WithExecutionIdentity(turnCtx, backend.NewExecutionIdentity("turn-"+prompt)), id, prompt, nil, func(gimbal.AgentEvent) error { return nil })
		result <- err
	}()
	for {
		d.mu.Lock()
		started := d.starts
		d.mu.Unlock()
		if started >= want && acknowledged(ad, id) {
			break
		}
		select {
		case err := <-result:
			return err
		case <-time.After(time.Millisecond):
		}
	}
	stop()
	return <-result
}

func (d *recoveryDaemon) count(read func() int) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return read()
}

func eventually(t *testing.T, ready func() bool) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); !ready(); time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("condition never held")
		}
	}
}

// Stop after turn/start was acknowledged but before any event is uncertain only
// about cessation: delivery is certain, and a next prompt waits for the earlier
// native turn instead of overlapping it, failing, or replaying it.
func TestUnconfirmedStopAfterAcknowledgementDoesNotOverlapNextTurn(t *testing.T) {
	shortenControlTimeout(t)
	ad, d := newRecoveryDaemon(t, "default")
	d.ignoreInterrupt = true
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	id, err := ad.CreateSession(effects.WithExecutionIdentity(ctx, backend.NewExecutionIdentity("session")), "stalled", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	err = stopStalledTurn(t, ctx, ad, d, id, "first", 1)
	uncertain, ok := errors.AsType[*backend.DeliveryUncertain](err)
	if !ok || !uncertain.Accepted || uncertain.RequestID != "turn-1" || !errors.Is(err, context.Canceled) {
		t.Fatalf("unconfirmed stop reported %v", err)
	}
	starts := func() int { return d.starts }
	interrupts := func() int { return d.interrupts }
	base := d.count(interrupts)

	// (a) The earlier turn is still in progress: the next prompt waits, nothing
	// is started, and the call has not failed.
	var events []string
	var eventsMu sync.Mutex
	result := make(chan error, 1)
	var output json.RawMessage
	go func() {
		out, err := ad.RunTurn(effects.WithExecutionIdentity(ctx, backend.NewExecutionIdentity("turn-second")), id, "second", nil, func(event gimbal.AgentEvent) error {
			eventsMu.Lock()
			defer eventsMu.Unlock()
			events = append(events, string(mustJSON(event)))
			return nil
		})
		output = out.Output
		result <- err
	}()
	eventually(t, func() bool { return d.count(interrupts) > base })
	time.Sleep(50 * time.Millisecond)
	select {
	case err := <-result:
		t.Fatalf("waiting prompt returned %v", err)
	default:
	}
	if d.count(starts) != 1 {
		t.Fatalf("turn/start while earlier turn runs: %d", d.count(starts))
	}

	// (d) Late terminal output of the earlier turn reaches the connection while
	// the new prompt is still waiting.
	d.mu.Lock()
	ws := d.sockets[len(d.sockets)-1]
	d.mu.Unlock()
	late := json.RawMessage(mustEncode(map[string]any{"id": "late", "type": "agentMessage", "phase": "final_answer", "text": "late-old"}))
	_ = fakeSend(ws, map[string]any{"method": "item/completed", "params": map[string]any{"threadId": id, "turnId": "turn-1", "item": late}})
	_ = fakeSend(ws, map[string]any{"method": "turn/completed", "params": map[string]any{"threadId": id, "turn": map[string]string{"id": "turn-1", "status": "completed"}}})

	// (b) The earlier turn settles, then exactly one next prompt starts.
	d.mu.Lock()
	d.threads[id].turns[0].Status = "interrupted"
	d.threads[id].status = "idle"
	d.completeStalled = true
	d.mu.Unlock()
	if err := <-result; err != nil {
		t.Fatalf("next prompt: %v", err)
	}
	if d.count(starts) != 2 {
		t.Fatalf("starts %d", d.count(starts))
	}
	eventsMu.Lock()
	seen := strings.Join(events, "\n")
	eventsMu.Unlock()
	if string(output) != `"implemented"` || strings.Contains(seen, "late-old") {
		t.Fatalf("new prompt got output %s events %s", output, seen)
	}
}

// (c) Stop or Finish while the next prompt waits abandons it: Codex never saw
// it, so the report is plain cancellation and nothing is started.
func TestStopWhileNextPromptWaitsSubmitsNothing(t *testing.T) {
	shortenControlTimeout(t)
	ad, d := newRecoveryDaemon(t, "default")
	d.ignoreInterrupt = true
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	id, err := ad.CreateSession(effects.WithExecutionIdentity(ctx, backend.NewExecutionIdentity("session")), "stalled", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_ = stopStalledTurn(t, ctx, ad, d, id, "first", 1)
	waitCtx, abandon := context.WithCancel(ctx)
	result := make(chan error, 1)
	go func() {
		_, err := ad.RunTurn(effects.WithExecutionIdentity(waitCtx, backend.NewExecutionIdentity("turn-second")), id, "second", nil, func(gimbal.AgentEvent) error { return nil })
		result <- err
	}()
	eventually(t, func() bool { return d.count(func() int { return d.interrupts }) >= 2 })
	abandon()
	err = <-result
	_, uncertain := errors.AsType[*backend.DeliveryUncertain](err)
	if !errors.Is(err, context.Canceled) || uncertain || d.count(func() int { return d.starts }) != 1 {
		t.Fatalf("abandoned prompt: err=%v starts=%d", err, d.starts)
	}
}

// A native turn that already completed is authoritative even when cancellation
// arrives while its terminal notification is being handled.
func TestNativeCompletionRacingStopIsReturned(t *testing.T) {
	ad, _ := newRecoveryDaemon(t, "default")
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	id, err := ad.CreateSession(effects.WithExecutionIdentity(ctx, backend.NewExecutionIdentity("session")), "plain", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	turnCtx, stop := context.WithCancel(ctx)
	defer stop()
	out, err := ad.RunTurn(effects.WithExecutionIdentity(turnCtx, backend.NewExecutionIdentity("turn")), id, "go", nil, func(event gimbal.AgentEvent) error {
		if event.Type == "session.step.ended" {
			stop()
		}
		return nil
	})
	if turnCtx.Err() == nil {
		t.Fatal("fixture never cancelled during the terminal notification")
	}
	if err != nil || string(out.Output) != `"implemented"` {
		t.Fatalf("racing completion: out=%s err=%v", out.Output, err)
	}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// acknowledged reports whether the adapter has seen turn/start's answer.
func acknowledged(ad *adapter, id string) bool {
	s, err := ad.session(id)
	if err != nil {
		return false
	}
	active := s.getActive()
	if active == nil {
		return false
	}
	active.mu.Lock()
	defer active.mu.Unlock()
	return active.turnID != ""
}
