package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"

	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/internal/modelalias"
	"github.com/tylergannon/gimbal/runstore"
)

// TestRunTurnAfterRedialResumesThread proves finding 1's fix live: killing
// the adapter's current WebSocket out from under it must not orphan a turn
// on a session that already exists. It talks to the machine's real shared
// `codex app-server` daemon (never restarting it) using the cheap model
// luna, so it only runs when explicitly requested:
//
//	GIMBAL_LIVE=1 go test ./codex -run TestRunTurnAfterRedialResumesThread -v
//
// Before the fix in adapter.conn/resumeThreads, the second turn below hangs
// until the bounded context expires: turn/start succeeds on the redialed
// connection, but that connection was never subscribed to the thread, so
// its notifications go nowhere and readTurn never sees turn/completed.
// After the fix, the redial resumes every known thread before handing the
// connection back, and the second turn returns text well within the bound.
func TestRunTurnAfterRedialResumesThread(t *testing.T) {
	if os.Getenv("GIMBAL_LIVE") != "1" {
		t.Skip("set GIMBAL_LIVE=1 to run against the live codex app-server daemon")
	}
	ad, ok := New().(*adapter)
	if !ok {
		t.Fatalf("codex.New() did not return *adapter")
	}

	dir := os.Getenv("GIMBAL_LIVE_PROJECT")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ctx = gimbal.Project(ctx, dir)

	err := gimbal.Run(ctx, string(runstore.NewRunID()), "daemon-live", map[gimbal.WorkflowRole]gimbal.ModelBinding{"live": {Adapter: ad, Model: liveLuna(t)}}, func(ctx context.Context) error {
		session := gimbal.NewSession(ctx, "live", dir)

		first, err := session.Generate[gimbal.Text](ctx, "Reply with exactly one word: one.")
		if err != nil {
			return err
		}
		if strings.TrimSpace(string(first)) == "" {
			t.Fatal("first turn returned no text")
		}
		conn := ad.current()
		if conn == nil {
			t.Fatal("adapter has no connection after the first turn")
		}

		// An ordinary second turn reuses the connection the first turn
		// dialed: this is the one-connection contract, observed directly
		// rather than inferred from process counts.
		second, err := session.Generate[gimbal.Text](ctx, "Reply with exactly one word: two.")
		if err != nil {
			return err
		}
		if strings.TrimSpace(string(second)) == "" {
			t.Fatal("second turn returned no text")
		}
		if ad.current() != conn {
			t.Fatal("an ordinary second turn opened a new connection instead of reusing the first")
		}

		// Kill that connection, the same way a network blip or the
		// daemon-side idle timeout would. The next call must redial and,
		// per the fix, resume this thread before running a turn on it.
		_ = conn.ws.CloseNow()
		<-conn.readDone // wait for the reader to notice, so the next call redials deterministically

		third, err := session.Generate[gimbal.Text](ctx, "Use exactly one shell tool to run `printf CODEX_RECONNECT_TOOL_MARKER`, then answer exactly CODEX_RECONNECT_FINAL_MARKER.")
		if err != nil {
			return err
		}
		if strings.TrimSpace(string(third)) == "" {
			t.Fatal("third turn (after redial) returned no text")
		}
		if ad.current() == conn {
			t.Fatal("the turn after the kill ran on the dead connection")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	assertReconnectTranscript(t, dir)
}

// assertReconnectTranscript checks the retained run record used by the browser
// proof. The turn after the socket-only reconnect must still carry the daemon's
// exact raw response IDs, with the tool and final answer in distinct
// token-bearing response rows.
func assertReconnectTranscript(t *testing.T, project string) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(project, "runs", "*", "sessions", "live.1.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 {
		t.Fatalf("live transcript paths = %v, want one", paths)
	}
	file, err := os.Open(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()

	type eventData struct {
		AssistantMessageID string        `json:"assistantMessageID"`
		Text               string        `json:"text"`
		Content            []any         `json:"content"`
		Tokens             gimbal.Tokens `json:"tokens"`
	}
	type nativeRef struct {
		ResponseID string `json:"responseID"`
	}
	rows := make(map[string]struct {
		responseID string
		tokens     gimbal.Tokens
	})
	var toolMessageID, finalMessageID string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var record gimbal.AgentRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(record.Turn, "/turn.3") {
			continue
		}
		var data eventData
		if err := json.Unmarshal(record.Event.Data, &data); err != nil {
			t.Fatal(err)
		}
		switch record.Event.Type {
		case "session.step.streamed":
			var ref nativeRef
			if err := json.Unmarshal(record.NativeRef, &ref); err != nil {
				t.Fatal(err)
			}
			if ref.ResponseID == "" {
				t.Fatal("reconnected turn streamed a step without a native response ID")
			}
			row := rows[data.AssistantMessageID]
			row.responseID = ref.ResponseID
			rows[data.AssistantMessageID] = row
		case "session.step.ended":
			row := rows[data.AssistantMessageID]
			row.tokens = data.Tokens
			rows[data.AssistantMessageID] = row
		case "session.tool.success":
			encoded, _ := json.Marshal(data.Content)
			if strings.Contains(string(encoded), "CODEX_RECONNECT_TOOL_MARKER") {
				toolMessageID = data.AssistantMessageID
			}
		case "session.text.ended":
			if data.Text == "CODEX_RECONNECT_FINAL_MARKER" {
				finalMessageID = data.AssistantMessageID
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if toolMessageID == "" || finalMessageID == "" {
		t.Fatalf("reconnected turn markers: tool row %q, final row %q", toolMessageID, finalMessageID)
	}
	if toolMessageID == finalMessageID {
		t.Fatalf("reconnected tool and final answer share response row %q", toolMessageID)
	}
	for label, messageID := range map[string]string{"tool": toolMessageID, "final": finalMessageID} {
		row := rows[messageID]
		if row.responseID == "" {
			t.Errorf("reconnected %s row %q has no native response ID", label, messageID)
		}
		if row.tokens.Input+row.tokens.Output+row.tokens.Reasoning+row.tokens.Cache.Read+row.tokens.Cache.Write == 0 {
			t.Errorf("reconnected %s row %q has no model-call tokens", label, messageID)
		}
		t.Logf("reconnected %s row %s: response %s, tokens %+v", label, messageID, row.responseID, row.tokens)
	}
}

// current returns the adapter's shared connection without dialing.
func (a *adapter) current() *connection {
	a.connMu.Lock()
	defer a.connMu.Unlock()
	return a.sharedConn
}

// TestCloseArchivesThreadsWithoutTouchingTheDaemon proves commit 2's
// contract live: HarnessAdapter.Close on the Codex adapter deregisters a
// session's routing and requests archive, without stopping the shared daemon.
// Loaded-thread observations alone do not prove descendant/FD cleanup. It creates a session,
// runs one short turn, forks it without ever running a turn on the fork
// (proving Close also releases a session whose only allocation was
// thread/fork, not thread/start), and lets gimbal.Run's root scope end.
// It talks to the machine's real shared `codex app-server` daemon, using
// the cheap model luna, so it only runs when explicitly requested:
//
//	GIMBAL_LIVE=1 go test ./codex -run TestCloseArchivesThreadsWithoutTouchingTheDaemon -v
func TestCloseArchivesThreadsWithoutTouchingTheDaemon(t *testing.T) {
	if os.Getenv("GIMBAL_LIVE") != "1" {
		t.Skip("set GIMBAL_LIVE=1 to run against the live codex app-server daemon")
	}

	beforePID, err := managedDaemonPID()
	if err != nil {
		t.Fatal(err)
	}

	ad, ok := New().(*adapter)
	if !ok {
		t.Fatalf("codex.New() did not return *adapter")
	}
	var mu sync.Mutex
	archived := map[string]bool{}
	ad.onArchive = func(threadID string) {
		mu.Lock()
		defer mu.Unlock()
		archived[threadID] = true
	}

	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ctx = gimbal.Project(ctx, dir)

	err = gimbal.Run(ctx, string(runstore.NewRunID()), "daemon-close-live", map[gimbal.WorkflowRole]gimbal.ModelBinding{"live": {Adapter: ad, Model: liveLuna(t)}}, func(ctx context.Context) error {
		session := gimbal.NewSession(ctx, "live", dir)
		if _, err := session.Generate[gimbal.Text](ctx, "Reply with exactly one word: proof."); err != nil {
			return err
		}
		// thread/fork subscribes the fork's thread immediately; forking
		// without ever running a turn on it still leaves something for
		// Close to release when the scope ends.
		if _, err := session.Fork(effects.WithExecutionIdentity(ctx, backend.NewExecutionIdentity("session")), "forked"); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	ad.mu.Lock()
	remainingSessions := len(ad.sessions)
	ad.mu.Unlock()
	if remainingSessions != 0 {
		t.Fatalf("adapter session map after Run = %d entries, want 0", remainingSessions)
	}

	if ad.current() != nil {
		t.Fatal("idle adapter retained its connection after the run")
	}

	mu.Lock()
	archivedIDs := make([]string, 0, len(archived))
	for id := range archived {
		archivedIDs = append(archivedIDs, id)
	}
	mu.Unlock()
	if len(archivedIDs) != 2 {
		t.Fatalf("archived threads = %v, want 2 (the session and its fork)", archivedIDs)
	}

	// The adapter released its client. Use a separately owned, read-only probe
	// of the existing daemon; this never starts one and makes no model turn.
	conn, err := connect(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.finishRead(errors.New("live probe ended"))
	callCtx, cancelCall := context.WithTimeout(context.Background(), requestTimeout)
	loadedRaw, err := conn.call(callCtx, "thread/loaded/list", map[string]any{})
	cancelCall()
	if err != nil {
		t.Fatal(err)
	}
	loadedIDs := loadedThreadIDs(loadedRaw)
	t.Logf("daemon thread/loaded/list after Close: %d threads", len(loadedIDs))
	for _, id := range archivedIDs {
		if slices.Contains(loadedIDs, id) {
			t.Errorf("archived thread %s is still loaded in the daemon", id)
		} else {
			t.Logf("archived thread %s: unloaded from the daemon", id)
		}
	}

	afterPID, err := managedDaemonPID()
	if err != nil {
		t.Fatal(err)
	}
	if beforePID != afterPID {
		t.Fatalf("daemon pid changed from %s to %s: Close must never restart the shared daemon", beforePID, afterPID)
	}
	t.Logf("daemon pid before=%s after=%s (unchanged)", beforePID, afterPID)
}

// TestCloseArchivesThroughARedialedConnection covers the intersection the
// two tests above leave open: the adapter's socket dies after a turn, and
// the scope then ends with no further turn. Close must not take the dead
// socket as permission to skip the archive; it redials (the daemon is still
// up) and archives through the new connection, so the thread is unloaded
// from the daemon exactly as on the healthy path.
//
//	GIMBAL_LIVE=1 go test ./codex -run TestCloseArchivesThroughARedialedConnection -v
func TestCloseArchivesThroughARedialedConnection(t *testing.T) {
	if os.Getenv("GIMBAL_LIVE") != "1" {
		t.Skip("set GIMBAL_LIVE=1 to run against the live codex app-server daemon")
	}
	beforePID, err := managedDaemonPID()
	if err != nil {
		t.Fatal(err)
	}
	ad := New().(*adapter)
	var mu sync.Mutex
	var archived []string
	ad.onArchive = func(threadID string) {
		mu.Lock()
		defer mu.Unlock()
		archived = append(archived, threadID)
	}

	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ctx = gimbal.Project(ctx, dir)

	var dead *connection
	err = gimbal.Run(ctx, string(runstore.NewRunID()), "daemon-close-redial-live", map[gimbal.WorkflowRole]gimbal.ModelBinding{"live": {Adapter: ad, Model: liveLuna(t)}}, func(ctx context.Context) error {
		session := gimbal.NewSession(ctx, "live", dir)
		if _, err := session.Generate[gimbal.Text](ctx, "Reply with exactly one word: proof."); err != nil {
			return err
		}
		dead = ad.current()
		_ = dead.ws.CloseNow()
		<-dead.readDone
		return nil // the scope ends here; Close runs on a dead socket
	})
	if err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	archivedIDs := slices.Clone(archived)
	mu.Unlock()
	if len(archivedIDs) != 1 {
		t.Fatalf("archived threads = %v, want exactly the one session", archivedIDs)
	}
	if ad.current() != nil {
		t.Fatal("idle adapter retained its redialed connection")
	}
	conn, err := connect(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.finishRead(errors.New("live probe ended"))
	callCtx, cancelCall := context.WithTimeout(context.Background(), requestTimeout)
	loadedRaw, err := conn.call(callCtx, "thread/loaded/list", map[string]any{})
	cancelCall()
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(loadedThreadIDs(loadedRaw), archivedIDs[0]) {
		t.Fatalf("thread %s is still loaded in the daemon after Close on a dead socket", archivedIDs[0])
	}
	t.Logf("thread %s archived through the redialed connection and unloaded from the daemon", archivedIDs[0])
	afterPID, err := managedDaemonPID()
	if err != nil {
		t.Fatal(err)
	}
	if beforePID != afterPID {
		t.Fatalf("daemon pid changed from %s to %s", beforePID, afterPID)
	}
	t.Logf("daemon pid before=%s after=%s (unchanged)", beforePID, afterPID)
}

// TestDetachedConversationResumesTheSameNativeThread is the live proof for
// human-facing dashboard conversations: one adapter detaches without archive,
// a fresh adapter resumes the exact thread id, and Codex retains context that
// Gimbal did not replay. The test archives its own thread after proving resume.
//
//	GIMBAL_LIVE=1 go test ./codex -run TestDetachedConversationResumesTheSameNativeThread -v
func TestDetachedConversationResumesTheSameNativeThread(t *testing.T) {
	if os.Getenv("GIMBAL_LIVE") != "1" {
		t.Skip("set GIMBAL_LIVE=1 to run against the live codex app-server daemon")
	}
	workdir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	first := New().(*adapter)
	thread, err := first.CreateSession(effects.WithExecutionIdentity(ctx, backend.NewExecutionIdentity("session")), liveLuna(t), "low", workdir)
	if err != nil {
		t.Fatal(err)
	}
	token := "cobalt-7319"
	if _, err := first.RunTurn(effects.WithExecutionIdentity(ctx, backend.NewExecutionIdentity("turn")), thread, "Remember the token "+token+". Reply only READY.", nil, func(gimbal.AgentEvent) error { return nil }); err != nil {
		t.Fatal(err)
	}
	beforeSocket, running, err := daemonStatus(ctx)
	if err != nil || !running {
		t.Fatalf("daemon after first turn: running=%t err=%v", running, err)
	}
	if err := first.DetachSession(ctx, thread); err != nil {
		t.Fatal(err)
	}

	second := New().(*adapter)
	if err := second.ResumeSession(ctx, thread, liveLuna(t), "low", workdir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := second.Close(effects.WithExecutionIdentity(context.Background(), backend.NewExecutionIdentity("close")), thread); err != nil {
			t.Errorf("archive resumed test thread: %v", err)
		}
	}()
	result, err := second.RunTurn(effects.WithExecutionIdentity(ctx, backend.NewExecutionIdentity("turn")), thread, "What token did I ask you to remember? Reply with only the token.", nil, func(gimbal.AgentEvent) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	var answer string
	if err := json.Unmarshal(result.Output, &answer); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(answer) != token {
		t.Fatalf("resumed thread answered %q, want %q", answer, token)
	}
	afterSocket, running, err := daemonStatus(ctx)
	if err != nil || !running {
		t.Fatalf("daemon after resume: running=%t err=%v", running, err)
	}
	if beforeSocket != afterSocket {
		t.Fatalf("daemon socket changed from %s to %s", beforeSocket, afterSocket)
	}
	t.Logf("thread %s detached and resumed with native context; daemon socket stayed %s", thread, beforeSocket)
}

// TestForkOfAnArchivedParentFailsWithoutAGhost: Close archives threads, and
// the daemon refuses thread/fork and thread/resume on an archived thread.
// The adapter must not unarchive for the caller: on this daemon version
// every unarchive, over the API or the CLI, leaves a ghost thread loaded
// that nothing can archive or delete. So a fork of an archived parent is a
// clear error after a read-only thread/read, with no fork, resume, or
// unarchive call, and the run leaves no new thread loaded. The parent here
// is archived out from under the adapter, as Codex Desktop or an earlier
// Close would.
//
//	GIMBAL_LIVE=1 go test ./codex -run TestForkOfAnArchivedParentFailsWithoutAGhost -v
func TestForkOfAnArchivedParentFailsWithoutAGhost(t *testing.T) {
	if os.Getenv("GIMBAL_LIVE") != "1" {
		t.Skip("set GIMBAL_LIVE=1 to run against the live codex app-server daemon")
	}
	ad := New().(*adapter)
	var mu sync.Mutex
	var archived []string
	ad.onArchive = func(threadID string) {
		mu.Lock()
		defer mu.Unlock()
		archived = append(archived, threadID)
	}
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ctx = gimbal.Project(ctx, dir)

	// Every thread loaded in the daemon before the run. Afterwards the set
	// must not have grown: a thread/fork issued against a still-archived
	// parent leaves a ghost thread loaded that nothing can archive or delete,
	// and that is exactly what reading the state first must prevent.
	probe, err := connect(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = probe.ws.CloseNow() }()
	loadedBefore := loadedSet(t, probe)

	var parent string
	err = gimbal.Run(ctx, string(runstore.NewRunID()), "daemon-fork-archived-live", map[gimbal.WorkflowRole]gimbal.ModelBinding{"live": {Adapter: ad, Model: liveLuna(t)}}, func(ctx context.Context) error {
		session := gimbal.NewSession(ctx, "live", dir)
		if _, err := session.Generate[gimbal.Text](ctx, "Reply with exactly one word: parent."); err != nil {
			return err
		}
		ad.mu.Lock()
		for id := range ad.sessions {
			parent = id
		}
		ad.mu.Unlock()
		conn := ad.current()
		callCtx, cancelCall := context.WithTimeout(ctx, requestTimeout)
		_, err := conn.call(callCtx, "thread/archive", map[string]any{"threadId": parent})
		cancelCall()
		if err != nil {
			return fmt.Errorf("archive parent out of band: %w", err)
		}
		_, err = session.Fork(effects.WithExecutionIdentity(ctx, backend.NewExecutionIdentity("session")), "forked")
		if !errors.Is(err, errThreadArchived) {
			return fmt.Errorf("fork of an archived parent returned %v, want errThreadArchived", err)
		}
		t.Logf("fork refused: %v", err)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Close still ran for the parent: its archive answers "no rollout found"
	// for an already-archived thread, which Close treats as done.
	mu.Lock()
	n := len(archived)
	mu.Unlock()
	if n != 1 {
		t.Fatalf("Close archived %d threads, want 1 (the parent)", n)
	}
	for id := range loadedSet(t, probe) {
		if !loadedBefore[id] {
			t.Errorf("the run left a new thread %s loaded in the daemon", id)
		}
	}
	t.Logf("parent %s stayed archived, the fork was refused after thread/read with no fork or unarchive call, and no new thread was left loaded", parent)
}

// loadedSet is the daemon's thread/loaded/list as a set.
func loadedSet(t *testing.T, conn *connection) map[string]bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	raw, err := conn.call(ctx, "thread/loaded/list", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	set := map[string]bool{}
	for _, id := range loadedThreadIDs(raw) {
		set[id] = true
	}
	return set
}

// loadedThreadIDs decodes thread/loaded/list's result, whose entries have
// appeared as a bare id, {"id":...}, or {"thread":{"id":...}}.
func loadedThreadIDs(raw json.RawMessage) []string {
	var response struct {
		Data []json.RawMessage `json:"data"`
	}
	if json.Unmarshal(raw, &response) != nil {
		return nil
	}
	var ids []string
	for _, entry := range response.Data {
		if id, ok := decodeThreadEntry(entry); ok {
			ids = append(ids, id)
		}
	}
	return ids
}

func decodeThreadEntry(entry json.RawMessage) (string, bool) {
	var bare string
	if json.Unmarshal(entry, &bare) == nil && bare != "" {
		return bare, true
	}
	var withID struct {
		ID     string `json:"id"`
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	if json.Unmarshal(entry, &withID) != nil {
		return "", false
	}
	if withID.ID != "" {
		return withID.ID, true
	}
	if withID.Thread.ID != "" {
		return withID.Thread.ID, true
	}
	return "", false
}

// managedDaemonPID identifies the process listening on the selected daemon socket.
func managedDaemonPID() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	socket, running, err := daemonStatus(ctx)
	if err != nil || !running {
		return "", fmt.Errorf("selected daemon status: running=%t: %w", running, err)
	}
	resolved, err := filepath.EvalSymlinks(socket)
	if err != nil {
		return "", fmt.Errorf("resolve daemon socket %s: %w", socket, err)
	}
	out, err := exec.CommandContext(ctx, "lsof", "-t", "--", resolved).Output()
	if err != nil {
		return "", fmt.Errorf("identify listener for %s: %w", socket, err)
	}
	pids := strings.Fields(string(out))
	if len(pids) != 1 {
		return "", fmt.Errorf("socket %s has %d owners: %v", socket, len(pids), pids)
	}
	return pids[0], nil
}

func liveLuna(t *testing.T) string {
	t.Helper()
	resolved, err := modelalias.Resolve(modelalias.Selection{Name: "luna"})
	if err != nil || resolved.Harness != "codex" {
		t.Fatalf("resolve luna: %+v, %v", resolved, err)
	}
	t.Logf("luna resolves to %s", resolved.Model)
	return resolved.Model
}

// Socket loss during an active tool uses the shared daemon without stopping or
// restarting it. The file effect precedes loss and must occur exactly once.
func TestActiveTurnReconnectPreservesToolEffectLive(t *testing.T) {
	if os.Getenv("GIMBAL_LIVE") != "1" {
		t.Skip("set GIMBAL_LIVE=1; uses luna")
	}
	ad := New().(*adapter)
	dir := t.TempDir()
	effect := filepath.Join(dir, "effect.txt")
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	dropped := make(chan error, 1)
	go func() {
		for {
			raw, err := os.ReadFile(effect)
			if err == nil && len(raw) > 0 {
				conn := ad.current()
				if conn == nil {
					dropped <- errors.New("no active connection")
					return
				}
				dropped <- conn.ws.CloseNow()
				return
			}
			select {
			case <-ctx.Done():
				dropped <- ctx.Err()
				return
			case <-time.After(20 * time.Millisecond):
			}
		}
	}()
	validations := 0
	err := gimbal.Run(gimbal.Project(ctx, dir), string(runstore.NewRunID()), "active-reconnect-live", map[gimbal.WorkflowRole]gimbal.ModelBinding{"coding": {Adapter: ad, Model: liveLuna(t)}, "qa": {Adapter: ad, Model: liveLuna(t)}}, func(ctx context.Context) error {
		for taskCtx, stage := range gimbal.Iterate(ctx, "outcome", []string{"first", "second", "third"}) {
			gimbal.Set(taskCtx, "stage", stage)
			gimbal.Set(taskCtx, "effect file", effect)
			worker := gimbal.NewSession(taskCtx, "coding", dir)
			report, err := worker.Generate[gimbal.Text](taskCtx, "For first and third stages, reply with the stage name and no tools. For the second stage, run one shell command that appends exactly one line EFFECT to the supplied effect file, then sleeps 8 seconds before exiting. Do not run that command again. After it finishes, reply exactly SECOND_COMPLETE.")
			if err != nil {
				return err
			}
			if stage == "second" && strings.TrimSpace(string(report)) != "SECOND_COMPLETE" {
				return fmt.Errorf("second answer=%q", report)
			}
			qa := gimbal.NewSession(taskCtx, "qa", dir)
			answer, err := qa.Generate[gimbal.Text](taskCtx, "Independently check the supplied stage. For the second stage, use a shell tool to read the effect file and confirm it contains exactly one line EFFECT. For other stages no tool is necessary. Reply exactly ACCEPTED if the stage is valid, otherwise FAILED.")
			if err != nil {
				return err
			}
			if strings.TrimSpace(string(answer)) != "ACCEPTED" {
				return fmt.Errorf("validation=%q", answer)
			}
			validations++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := <-dropped; err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(effect)
	if err != nil || string(raw) != "EFFECT\n" {
		t.Fatalf("effect=%q,%v", raw, err)
	}
	if validations != 3 {
		t.Fatalf("validations=%d", validations)
	}
	t.Log("luna: active tool socket loss recovered; effect occurred once; each of three outcomes independently validated once")
}

// TestSharedSessionsReleaseLive checks one connection survives one session's release.
func TestSharedSessionsReleaseLive(t *testing.T) {
	if os.Getenv("GIMBAL_LIVE") != "1" {
		t.Skip("set GIMBAL_LIVE=1")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	ad := New().(*adapter)
	workdir, model := t.TempDir(), liveLuna(t)
	ids := make([]string, 2)
	for i := range ids {
		var err error
		ids[i], err = ad.CreateSession(effects.WithExecutionIdentity(ctx, backend.NewExecutionIdentity("session")), model, "low", workdir)
		if err != nil {
			t.Fatal(err)
		}
	}
	defer func() {
		for _, id := range ids {
			if id != "" {
				_ = ad.Close(effects.WithExecutionIdentity(context.Background(), backend.NewExecutionIdentity("close")), id)
			}
		}
	}()
	conn := ad.current()
	errs := make(chan error, 2)
	for _, id := range ids {
		go func(id string) {
			_, err := ad.RunTurn(effects.WithExecutionIdentity(ctx, backend.NewExecutionIdentity("turn")), id, "Reply briefly without tools.", nil, func(gimbal.AgentEvent) error { return nil })
			errs <- err
		}(id)
	}
	for range ids {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	if ad.current() != conn {
		t.Fatal("sessions did not share the connection")
	}
	if err := ad.Close(effects.WithExecutionIdentity(ctx, backend.NewExecutionIdentity("close")), ids[0]); err != nil {
		t.Fatal(err)
	}
	ids[0] = ""
	if ad.current() != conn {
		t.Fatal("release interrupted remaining session")
	}
	if _, err := ad.RunTurn(effects.WithExecutionIdentity(ctx, backend.NewExecutionIdentity("turn")), ids[1], "Reply briefly without tools.", nil, func(gimbal.AgentEvent) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := ad.Close(effects.WithExecutionIdentity(ctx, backend.NewExecutionIdentity("close")), ids[1]); err != nil {
		t.Fatal(err)
	}
	ids[1] = ""
	if ad.current() != nil {
		t.Fatal("last release retained connection")
	}
	select {
	case <-conn.readerDone:
	case <-ctx.Done():
		t.Fatalf("connection reader did not stop: %v", ctx.Err())
	}
	conn.threadsMu.Lock()
	routes := len(conn.threads)
	conn.threadsMu.Unlock()
	if routes != 0 {
		t.Fatalf("retained %d routes", routes)
	}
}

// TestTerminalConnectionFailureLive disconnects only this adapter during a real tool turn.
func TestTerminalConnectionFailureLive(t *testing.T) {
	if os.Getenv("GIMBAL_LIVE") != "1" {
		t.Skip("set GIMBAL_LIVE=1")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	ad := New().(*adapter)
	ad.recoveryDelay = 10 * time.Millisecond
	dir := t.TempDir()
	id, err := ad.CreateSession(effects.WithExecutionIdentity(ctx, backend.NewExecutionIdentity("session")), liveLuna(t), "low", dir)
	if err != nil {
		t.Fatal(err)
	}
	// The induced release failure cannot archive; clean up only this test thread
	// using a separately owned connection after the assertions.
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		probe, err := connect(cleanupCtx, false)
		if err != nil {
			t.Errorf("connect for test thread cleanup: %v", err)
			return
		}
		defer probe.finishRead(errors.New("test cleanup ended"))
		if _, err := probe.call(cleanupCtx, "thread/archive", map[string]any{"threadId": id}); err != nil {
			t.Errorf("archive test thread: %v", err)
		}
	}()
	effect := filepath.Join(dir, "effect")
	conn := ad.current()
	ad.dial = func(context.Context, bool) (*connection, error) { return nil, errors.New("test-owned redial rejected") }
	done := make(chan error, 1)
	go func() {
		_, err := ad.RunTurn(effects.WithExecutionIdentity(ctx, backend.NewExecutionIdentity("turn")), id, "Run one shell command that appends one line EFFECT to the file named effect in the working directory and then sleeps 8 seconds. Do not repeat it.", nil, func(gimbal.AgentEvent) error { return nil })
		done <- err
	}()
	for {
		if raw, err := os.ReadFile(effect); err == nil && len(raw) > 0 {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("turn ended before tool effect: %v", err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}
	if err := conn.ws.CloseNow(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err == nil {
		t.Fatal("terminal connection loss reported success")
	} else {
		t.Logf("turn failure: %v", err)
	}
	if err := ad.Close(effects.WithExecutionIdentity(ctx, backend.NewExecutionIdentity("close")), id); err == nil {
		t.Fatal("release unexpectedly succeeded after rejecting redial")
	} else {
		t.Logf("release failure: %v", err)
	}
	if ad.current() != nil {
		t.Fatal("failed release retained connection")
	}
	select {
	case <-conn.readerDone:
	case <-ctx.Done():
		t.Fatalf("connection reader did not stop: %v", ctx.Err())
	}
	ad.mu.Lock()
	sessions := len(ad.sessions)
	ad.mu.Unlock()
	conn.threadsMu.Lock()
	routes := len(conn.threads)
	conn.threadsMu.Unlock()
	if sessions != 0 || routes != 0 {
		t.Fatalf("retained sessions=%d routes=%d", sessions, routes)
	}
	raw, err := os.ReadFile(effect)
	if err != nil || string(raw) != "EFFECT\n" {
		t.Fatalf("effect=%q: %v", raw, err)
	}
}

// TestSeparateClientsSurviveReleaseLive keeps one disposable client usable while another releases.
func TestSeparateClientsSurviveReleaseLive(t *testing.T) {
	if os.Getenv("GIMBAL_LIVE") != "1" {
		t.Skip("set GIMBAL_LIVE=1")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	model, dir := liveLuna(t), t.TempDir()
	clients := []*adapter{New().(*adapter), New().(*adapter)}
	ids := make([]string, len(clients))
	defer func() {
		for i, id := range ids {
			if id != "" {
				cleanup, stop := context.WithTimeout(context.Background(), requestTimeout)
				if err := clients[i].Close(effects.WithExecutionIdentity(cleanup, backend.NewExecutionIdentity("close")), id); err != nil {
					t.Errorf("cleanup client %d: %v", i, err)
				}
				stop()
			}
		}
	}()
	for i, ad := range clients {
		var err error
		ids[i], err = ad.CreateSession(effects.WithExecutionIdentity(ctx, backend.NewExecutionIdentity("session")), model, "low", dir)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ad.RunTurn(effects.WithExecutionIdentity(ctx, backend.NewExecutionIdentity("turn")), ids[i], "Reply briefly without tools.", nil, func(gimbal.AgentEvent) error { return nil }); err != nil {
			t.Fatal(err)
		}
	}
	other := clients[1].current()
	if clients[0].current() == other {
		t.Fatal("adapters unexpectedly share a connection")
	}
	if err := clients[0].Close(effects.WithExecutionIdentity(ctx, backend.NewExecutionIdentity("close")), ids[0]); err != nil {
		t.Fatal(err)
	}
	ids[0] = ""
	if clients[1].current() != other {
		t.Fatal("other client's connection changed during release")
	}
	if _, err := clients[1].RunTurn(effects.WithExecutionIdentity(ctx, backend.NewExecutionIdentity("turn")), ids[1], "Reply briefly without tools.", nil, func(gimbal.AgentEvent) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := clients[1].Close(effects.WithExecutionIdentity(ctx, backend.NewExecutionIdentity("close")), ids[1]); err != nil {
		t.Fatal(err)
	}
	ids[1] = ""
	if clients[0].current() != nil || clients[1].current() != nil {
		t.Fatal("clients retained their connections")
	}
}
