package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coder/websocket"
)

func TestRawCapturePrecedesParsingAndRouting(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GIMBLE_CODEX_DEBUG_DIR", dir)
	want := []string{` {"method":"unknown/event","params":{"threadId":"unregistered"}} `, `{"method":"another/event"}`, `not JSON`}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ws, err := websocket.Accept(w, req, nil)
		if err != nil {
			return
		}
		defer func() { _ = ws.CloseNow() }()
		for _, frame := range want {
			if err := ws.Write(req.Context(), websocket.MessageText, []byte(frame)); err != nil {
				return
			}
		}
	}))
	defer server.Close()
	ws, _, err := websocket.Dial(context.Background(), strings.Replace(server.URL, "http:", "ws:", 1), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ws.CloseNow() }()
	c := &connection{ws: ws, debug: openRawRecorder(), threads: make(map[string]chan rpcMessage), readDone: make(chan struct{})}
	c.read()
	if c.readErr == nil {
		t.Fatal("expected malformed input to fail parsing")
	}
	paths, err := filepath.Glob(filepath.Join(dir, "codex-*.jsonl"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("capture files: %v, %v", paths, err)
	}
	file, err := os.Open(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("capture permissions: %v", info.Mode())
	}
	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		var row struct {
			Seq  int
			Time string
			Data string
		}
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		if count >= len(want) || row.Seq != count+1 || row.Time == "" || row.Data != want[count] {
			t.Fatalf("row %d: %+v", count, row)
		}
		count++
	}
	if scanner.Err() != nil || count != len(want) {
		t.Fatalf("read %d frames: %v", count, scanner.Err())
	}
}

func TestRawCaptureDisabledAndWriteFailureDoesNotAbort(t *testing.T) {
	t.Setenv("GIMBLE_CODEX_DEBUG_DIR", "")
	if recorder := openRawRecorder(); recorder != nil {
		t.Fatal("capture enabled by default")
	}
	t.Setenv("GIMBLE_CODEX_DEBUG_DIR", t.TempDir())
	recorder := openRawRecorder()
	if recorder == nil {
		t.Fatal("capture did not open")
	}
	_ = recorder.file.Close()
	recorder.record([]byte(`{"method":"event"}`))
	if recorder.file != nil {
		t.Fatal("failed capture was not disabled")
	}
	recorder.record([]byte("later frame"))
}
