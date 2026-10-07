package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"
	"time"
)

const reconnectAttempts = 3

type nativeTurn struct {
	ID     string            `json:"id"`
	Status string            `json:"status"`
	Items  []json.RawMessage `json:"items"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type threadSnapshot struct {
	ID     string `json:"id"`
	Status struct {
		Type string `json:"type"`
	} `json:"status"`
	Turns []nativeTurn `json:"turns"`
}

func readThread(ctx context.Context, conn *connection, id, itemsView string) (threadSnapshot, error) {
	callCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	raw, err := conn.call(callCtx, "thread/read", map[string]any{"threadId": id, "includeTurns": false})
	if err != nil {
		return threadSnapshot{}, err
	}
	var response struct {
		Thread threadSnapshot `json:"thread"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return threadSnapshot{}, fmt.Errorf("codex: decode recovery history: %w", err)
	}
	if response.Thread.ID != id {
		return threadSnapshot{}, fmt.Errorf("codex: recovery read returned thread %q, want %q", response.Thread.ID, id)
	}
	// Page native turns explicitly. The live paginated daemon does not
	// implement full-history thread/read, and a new thread has no page yet.
	cursor := ""
	for {
		params := map[string]any{"threadId": id, "limit": 100, "itemsView": itemsView}
		if cursor != "" {
			params["cursor"] = cursor
		}
		raw, err := conn.call(callCtx, "thread/turns/list", params)
		if err != nil {
			if rpc, ok := errors.AsType[*rpcError](err); ok && rpc.Code == -32600 && strings.Contains(rpc.Message, "not materialized yet") {
				return response.Thread, nil
			}
			return threadSnapshot{}, err
		}
		var page struct {
			Data       []nativeTurn `json:"data"`
			NextCursor string       `json:"nextCursor"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return threadSnapshot{}, err
		}
		response.Thread.Turns = append(response.Thread.Turns, page.Data...)
		if page.NextCursor == "" {
			break
		}
		if page.NextCursor == cursor {
			return threadSnapshot{}, errors.New("codex: native turn history cursor did not advance")
		}
		cursor = page.NextCursor
	}
	return response.Thread, nil
}

// recoverTurn maintains the original blocking call. It never interprets a lost
// response as permission to repeat a tool effect or run a concurrent worker.
// Automatic attempts are bounded; only an operator can initiate another batch.
func (a *adapter) recoverTurn(ctx context.Context, s *session, id, prompt string, params map[string]any, before threadSnapshot, active *activeTurn, cause error) (string, error) {
	original := cause
	confirmed := false
	for {
		batchCtx, batchCancel := context.WithTimeout(ctx, time.Minute)
		for attempt := 1; attempt <= reconnectAttempts && batchCtx.Err() == nil; attempt++ {
			if err := active.notice("recovering", attempt, cause); err != nil {
				batchCancel()
				return "", err
			}
			if attempt > 1 {
				select {
				case <-time.After(time.Duration(attempt-1) * a.recoveryDelay):
				case <-batchCtx.Done():
				}
			}
			if batchCtx.Err() != nil {
				break
			}
			attemptCtx, cancel := context.WithTimeout(batchCtx, 10*time.Second)
			conn, err := a.conn(attemptCtx)
			var snapshot threadSnapshot
			if err == nil {
				snapshot, err = readThread(attemptCtx, conn, id, "full")
			}
			cancel()
			if ctx.Err() != nil {
				batchCancel()
				return "", ctx.Err()
			}
			if err != nil {
				cause = err
				continue
			}
			active.mu.Lock()
			active.conn = conn
			turn := active.turnID
			active.mu.Unlock()
			if turn == "" {
				// A start may have reached the server despite its lost acknowledgement.
				// Match a single new native turn to the exact prompt, not just "latest".
				candidates := newTurns(before, snapshot, turnPrompt(params))
				if len(candidates) == 1 {
					turn = candidates[0].ID
					active.mu.Lock()
					active.turnID = turn
					active.mu.Unlock()
					active.emit.mu.Lock()
					active.emit.turnID = turn
					active.emit.mu.Unlock()
				}
			}
			current, found := findTurn(snapshot, turn)
			switch {
			case found && current.Status == "completed":
				answer := turnAnswer(current)
				if strings.TrimSpace(answer) == "" {
					cause = errors.New("codex: recovered completed turn has no final answer")
					break
				}
				if err := active.notice("recovered", attempt, nil); err != nil {
					batchCancel()
					return "", err
				}
				batchCancel()
				return answer, nil
			case found && current.Status == "failed":
				batchCancel()
				failure := "recovered native turn failed"
				if current.Error != nil && current.Error.Message != "" {
					failure = current.Error.Message
				}
				return "", fmt.Errorf("codex: %s", failure)
			case found && current.Status == "inProgress" && snapshot.Status.Type == "active":
				if err := active.notice("recovered", attempt, nil); err != nil {
					batchCancel()
					return "", err
				}
				// History can contain the final item while completion is still pending.
				// Seed the reader from authoritative items without replaying transcript
				// events, tools or usage that were already observed.
				seed, final := nativeAnswer(current)
				answer, readErr := readTurnFrom(ctx, conn, conn.registerThread(id), id, turn, active.emit, seed, final)
				if !isTransportError(readErr) {
					batchCancel()
					return answer, readErr
				}
				cause = readErr
			case confirmed && snapshot.Status.Type == "idle" && ((!found && turn == "") || (found && current.Status != "completed" && current.Status != "failed")):
				// The operator has inspected partial work and confirmed continuation is
				// safe. Start a follow-up on the SAME thread, retaining the original task.
				next := make(map[string]any, len(params))
				maps.Copy(next, params)
				next["input"] = input("The connection was lost during this task. Inspect the existing edits and durable tool results first. Continue from the work already done; do not repeat completed operations. Finish the original task:\n\n" + prompt)
				startCtx, startCancel := context.WithTimeout(ctx, requestTimeout)
				raw, startErr := conn.call(startCtx, "turn/start", next)
				startCancel()
				confirmed = false
				// Capture the pre-start history for another ambiguous acknowledgement.
				before = snapshot
				params = next
				active.mu.Lock()
				active.turnID = ""
				active.mu.Unlock()
				if startErr == nil {
					turn, startErr = turnID(raw)
					active.mu.Lock()
					active.turnID = turn
					active.mu.Unlock()
					active.emit = newProjector(id, turn, s.model, active.emit.emit)
					if startErr == nil {
						batchCancel()
						answer, nextErr := readTurn(ctx, conn, conn.registerThread(id), id, turn, active.emit)
						if !isTransportError(nextErr) {
							return answer, nextErr
						}
						cause = nextErr
						return a.recoverTurn(ctx, s, id, prompt, params, before, active, cause)
					}
				}
				if !isTransportError(startErr) && !errors.Is(startErr, context.DeadlineExceeded) {
					batchCancel()
					return "", startErr
				}
				cause = startErr
			default:
				cause = fmt.Errorf("codex: cannot safely continue thread %s turn %s (thread %s, turn %s); inspect partial work and confirm all prior operations have ceased before resuming", id, turn, snapshot.Status.Type, current.Status)
				// Native status can lag persisted turn history. Re-read within the
				// existing bounded budget; reconciliation does not repeat work.
			}
		}
		batchCancel()
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		active.mu.Lock()
		active.paused = true
		active.mu.Unlock()
		explanation := fmt.Errorf("%w; recovery paused after at most %d attempts: %v. The same run and task are retained. Send resume to this session with gimbal steer RUN_ID --session SESSION_ID resume to retry reconciliation, or cancel the run", original, reconnectAttempts, cause)
		if err := active.notice("paused", reconnectAttempts, explanation); err != nil {
			return "", err
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-active.resume:
			active.mu.Lock()
			active.paused = false
			active.mu.Unlock()
			confirmed = true
		}
	}
}

func (active *activeTurn) notice(state string, attempt int, err error) error {
	message := "Codex connection " + state
	if err != nil {
		message += ": " + err.Error()
	}
	active.emit.mu.Lock()
	defer active.emit.mu.Unlock()
	return active.emit.event("session.connection", map[string]any{
		"assistantMessageID": active.noticeID, "attempt": attempt, "maxAttempts": reconnectAttempts,
		"state": state, "message": message,
	}, map[string]any{"provider": "codex", "sessionID": active.emit.sessionID, "turnID": active.emit.turnID, "messageID": active.noticeID})
}

func findTurn(snapshot threadSnapshot, id string) (nativeTurn, bool) {
	for _, turn := range snapshot.Turns {
		if id != "" && turn.ID == id {
			return turn, true
		}
	}
	return nativeTurn{}, false
}

func newTurns(before, after threadSnapshot, prompt string) []nativeTurn {
	known := make(map[string]bool, len(before.Turns))
	for _, turn := range before.Turns {
		known[turn.ID] = true
	}
	var found []nativeTurn
	for _, turn := range after.Turns {
		if known[turn.ID] {
			continue
		}
		for _, raw := range turn.Items {
			var item struct {
				Type    string `json:"type"`
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			}
			if json.Unmarshal(raw, &item) == nil && item.Type == "userMessage" {
				for _, content := range item.Content {
					if content.Text == prompt {
						found = append(found, turn)
						break
					}
				}
			}
		}
	}
	return found
}

func turnAnswer(turn nativeTurn) string {
	answer, _ := nativeAnswer(turn)
	return answer
}

func nativeAnswer(turn nativeTurn) (answer string, final bool) {
	for _, raw := range turn.Items {
		var item struct {
			Type  string `json:"type"`
			Text  string `json:"text"`
			Phase string `json:"phase"`
		}
		if json.Unmarshal(raw, &item) != nil || item.Type != "agentMessage" {
			continue
		}
		if item.Phase == "final_answer" || !final {
			answer = item.Text
			final = item.Phase == "final_answer"
		}
	}
	return answer, final
}

func turnPrompt(params map[string]any) string {
	items, _ := params["input"].([]map[string]any)
	if len(items) == 1 {
		text, _ := items[0]["text"].(string)
		return text
	}
	return ""
}
