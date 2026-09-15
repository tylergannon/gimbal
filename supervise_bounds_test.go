package gimble

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestTranscriptBoundsRetentionWithFastAndSlowReaders(t *testing.T) {
	const retained = supervisorRetentionBytes
	transcript := newTranscript(2, retained)
	var peakAfterSaturation int
	var entriesAfterSaturation int
	for i := range 10_000 {
		_ = transcript.append(supervisorTestEvent("session.tool.success", map[string]any{
			"id": fmt.Sprintf("call-%05d", i), "content": strings.Repeat("x", 80),
		}))
		if i%10 == 0 {
			batch := transcript.since(0, 4<<10)
			if batch.missingThrough != 0 {
				t.Fatalf("fast reader unexpectedly fell behind at event %d: gap=%d..%d", i, batch.missingFirst, batch.missingThrough)
			}
			if got := transcriptBatchBytes(batch); got > 4<<10 {
				t.Fatalf("fast reader batch = %d bytes, want at most %d", got, 4<<10)
			}
		}
		transcript.mu.Lock()
		got, peak := transcript.bytes, transcript.peak
		transcript.mu.Unlock()
		if got > retained || peak > retained {
			t.Fatalf("retained=%d peak=%d, want both at most %d", got, peak, retained)
		}
		if i == 2_999 {
			peakAfterSaturation = peak
			transcript.mu.Lock()
			entriesAfterSaturation = len(transcript.entries)
			transcript.mu.Unlock()
		}
	}
	transcript.mu.Lock()
	finalPeak, finalEntries := transcript.peak, len(transcript.entries)
	transcript.mu.Unlock()
	if finalPeak != peakAfterSaturation || finalEntries != entriesAfterSaturation {
		t.Fatalf("retention grew after the buffer saturated: peak=%d->%d entries=%d->%d", peakAfterSaturation, finalPeak, entriesAfterSaturation, finalEntries)
	}
	t.Logf("events=10000 retained_limit=%d saturated_peak=%d final_peak=%d saturated_entries=%d final_entries=%d", retained, peakAfterSaturation, finalPeak, entriesAfterSaturation, finalEntries)

	slow := transcript.since(1, 256)
	if slow.missingFirst != 1 || slow.missingThrough == 0 {
		t.Fatalf("slow reader gap = %d..%d, want an explicit gap from item 1", slow.missingFirst, slow.missingThrough)
	}
	if got := transcriptBatchBytes(slow); got > 256 {
		t.Fatalf("slow reader batch = %d bytes, want at most 256", got)
	}
	if joined := strings.Join(slow.lines, ""); !strings.Contains(joined, "call_id=call-09999") {
		t.Fatalf("slow reader did not receive the newest tool item: %q", joined)
	}

	_ = transcript.since(0, 4<<10)
	transcript.mu.Lock()
	defer transcript.mu.Unlock()
	if transcript.bytes != 0 || len(transcript.entries) != 0 {
		t.Fatalf("consumed buffer retained %d bytes in %d entries", transcript.bytes, len(transcript.entries))
	}
}

func TestTranscriptCoalescesUnreadDeltasAndPreservesIdentity(t *testing.T) {
	transcript := newTranscript(1, 64<<10)
	_ = transcript.append(supervisorTestEvent("session.text.delta", map[string]any{"assistantMessageID": "message-1", "delta": "hel"}))
	_ = transcript.append(supervisorTestEvent("session.text.delta", map[string]any{"assistantMessageID": "message-1", "delta": "lo"}))
	_ = transcript.append(supervisorTestEvent("session.tool.input.delta", map[string]any{"id": "call-1", "delta": `{"command":"go`}))
	_ = transcript.append(supervisorTestEvent("session.tool.input.delta", map[string]any{"id": "call-1", "delta": ` test"}`}))

	batch := transcript.since(0, 64<<10)
	if len(batch.lines) != 2 {
		t.Fatalf("coalesced lines = %d, want 2: %q", len(batch.lines), batch.lines)
	}
	joined := strings.Join(batch.lines, "")
	for _, want := range []string{
		"[agent fragment] message_id=message-1 hello",
		`[tool input fragment] call_id=call-1 {"command":"go test"}`,
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("coalesced view lacks %q: %q", want, joined)
		}
	}

	// Once a reader has consumed a fragment, later fragments are a new entry;
	// the next look must not repeat the already consumed text.
	_ = transcript.append(supervisorTestEvent("session.text.delta", map[string]any{"assistantMessageID": "message-1", "delta": "!"}))
	next := strings.Join(transcript.since(0, 64<<10).lines, "")
	if !strings.Contains(next, "message_id=message-1 !") || strings.Contains(next, "hello") {
		t.Fatalf("next incremental view = %q", next)
	}

	shared := strings.Repeat("x", 500)
	_ = transcript.append(supervisorTestEvent("session.text.delta", map[string]any{"assistantMessageID": shared + "a", "delta": "first"}))
	_ = transcript.append(supervisorTestEvent("session.text.delta", map[string]any{"assistantMessageID": shared + "b", "delta": "second"}))
	distinct := transcript.since(0, 64<<10).lines
	if len(distinct) != 2 || distinct[0] == distinct[1] {
		t.Fatalf("long distinct message identities were merged: %q", distinct)
	}
}

func TestSupervisorLookHasIndependentByteBoundAndExplicitGap(t *testing.T) {
	transcript := newTranscript(1, 1<<20)
	for i := range 200 {
		_ = transcript.append(supervisorTestEvent("session.tool.success", map[string]any{
			"id": fmt.Sprintf("call-%03d", i), "content": strings.Repeat("result", 500),
		}))
	}
	sup := supervisor{
		session:     &Session{workdir: strings.Repeat("w", 10<<10)},
		instruction: strings.Repeat("watch carefully ", 10<<10),
	}
	look := transcript.look(0, sup, strings.Repeat("do the work ", 10<<10), true, "/tmp/project/runs/run/sessions/worker.1.jsonl")
	if len(look) > supervisorLookBytes {
		t.Fatalf("look = %d bytes, want at most %d", len(look), supervisorLookBytes)
	}
	t.Logf("look_limit=%d observed_look=%d", supervisorLookBytes, len(look))
	for _, want := range []string{
		"bytes omitted",
		"[gap] Supervisor activity items 1 through ",
		"durable worker transcript at /tmp/project/runs/run/sessions/worker.1.jsonl",
		"call_id=call-199",
	} {
		if !strings.Contains(look, want) {
			t.Errorf("bounded look lacks %q", want)
		}
	}
}

func TestTranscriptOwnsAndBoundsOversizedEventText(t *testing.T) {
	transcript := newTranscript(1, 4<<10)
	const size = 1 << 20
	_ = transcript.append(supervisorTestEvent("session.text.ended", map[string]any{"assistantMessageID": "large", "text": strings.Repeat("z", size)}))

	transcript.mu.Lock()
	if len(transcript.entries) != 1 {
		got := len(transcript.entries)
		transcript.mu.Unlock()
		t.Fatalf("entries = %d, want 1", got)
	}
	entry := transcript.entries[0]
	transcript.mu.Unlock()
	if len(entry.text) > supervisorEventTextBytes {
		t.Fatalf("stored event text = %d bytes, want at most %d", len(entry.text), supervisorEventTextBytes)
	}
	if entry.omitted != size-len(entry.text) {
		t.Fatalf("omitted = %d, want %d", entry.omitted, size-len(entry.text))
	}
	if entry.bytes() != len(entry.render()) {
		t.Fatalf("accounted bytes = %d, rendered bytes = %d", entry.bytes(), len(entry.render()))
	}
	line := strings.Join(transcript.since(0, 4<<10).lines, "")
	if !strings.Contains(line, fmt.Sprintf("[... %d bytes omitted]", size-supervisorEventTextBytes)) {
		t.Fatalf("oversized event was not explicitly shortened: %q", line[len(line)-80:])
	}
}

func transcriptBatchBytes(batch transcriptBatch) int {
	var total int
	for _, line := range batch.lines {
		total += len(line)
	}
	return total
}

func supervisorTestEvent(eventType string, data any) AgentEvent {
	raw, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	return AgentEvent{Type: eventType, Data: raw}
}
