package codex

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"time"
)

// GIMBLE_CODEX_DEBUG_DIR enables a private JSONL file per connection. Each
// received WebSocket frame is recorded before decoding or routing, including
// unknown and malformed messages. Data is a string so invalid JSON survives.
// These files can contain sensitive prompts and tool output; keep them local.
type rawRecorder struct {
	file *os.File
	seq  uint64
}

func openRawRecorder() *rawRecorder {
	dir := os.Getenv("GIMBLE_CODEX_DEBUG_DIR")
	if dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		log.Printf("gimble: Codex debug capture unavailable: %v", err)
		return nil
	}
	file, err := os.CreateTemp(dir, "codex-*.jsonl")
	if err != nil {
		log.Printf("gimble: Codex debug capture unavailable: %v", err)
		return nil
	}
	path, _ := filepath.Abs(file.Name())
	log.Printf("gimble: raw Codex events: %s (sensitive; do not commit)", path)
	return &rawRecorder{file: file}
}

// Only the connection reader calls record, preserving receive order.
func (r *rawRecorder) record(data []byte) {
	if r == nil || r.file == nil {
		return
	}
	r.seq++
	err := json.NewEncoder(r.file).Encode(struct {
		Seq  uint64    `json:"seq"`
		Time time.Time `json:"time"`
		Data string    `json:"data"`
	}{r.seq, time.Now().UTC(), string(data)})
	if err != nil {
		log.Printf("gimble: Codex debug capture stopped: %v", err)
		r.close()
	}
}

func (r *rawRecorder) close() {
	if r != nil && r.file != nil {
		_ = r.file.Close()
		r.file = nil
	}
}
