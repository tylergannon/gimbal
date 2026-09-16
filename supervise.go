package gimble

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

//go:generate go tool polytype --validate

// AgentOption is an argument to Generate. The same options are a
// supervisor's own where WithSupervisor attaches it, so a supervisor can
// have an interval and supervisors of its own.
type AgentOption func(*options)

type options struct {
	supervisors []supervisor
	every       time.Duration
}

type supervisor struct {
	session     *Session
	instruction string
	opts        []AgentOption
}

// WithSupervisor attaches a supervisor to a turn: a session of its own and
// an instruction saying what to watch for. While the turn runs, the
// supervisor looks at what the worker did since its last look, and each
// objection it raises is steered into the turn. When the worker finishes,
// Generate cancels and joins its supervisors before returning the worker's
// result and error. opts are the supervisor's: WithInterval for how often it
// looks, and WithSupervisor for supervisors of its looks.
func WithSupervisor(session *Session, instruction string, opts ...AgentOption) AgentOption {
	return func(o *options) {
		o.supervisors = append(o.supervisors, supervisor{session: session, instruction: instruction, opts: opts})
	}
}

// WithInterval is how often a supervisor looks, given among its options to
// WithSupervisor. The default is three minutes.
func WithInterval(every time.Duration) AgentOption {
	return func(o *options) { o.every = every }
}

func apply(opts []AgentOption) options {
	o := options{every: 3 * time.Minute}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// review is a supervisor's answer to one look at the work.
type review struct {
	// Each objection is one thing the agent under review must change or stop doing, written as an instruction to that agent. Leave the list empty when you have no objection.
	Objections []string `json:"objections"`
}

const (
	supervisorRetentionBytes   = 256 << 10
	supervisorLookBytes        = 64 << 10
	supervisorEventTextBytes   = 2000
	supervisorInstructionBytes = 8 << 10
	supervisorTaskBytes        = 16 << 10
	supervisorPathBytes        = 2 << 10
	supervisorGapReserveBytes  = 3 << 10
)

// supervise is Generate with supervisors attached.
func supervise[T Output](ctx context.Context, s *Session, prompt string, supervisors []supervisor) (T, error) {
	t := newTranscript(len(supervisors), supervisorRetentionBytes)
	history := filepath.Join(runDir(ctx), "sessions", s.id+".jsonl")
	for _, sup := range supervisors {
		s.mu.Lock()
		turn := fmt.Sprintf("%s/turn.%d", s.id, s.turns+1)
		s.mu.Unlock()
		if scope, err := current(ctx); err == nil {
			o := apply(sup.opts)
			scope.run.event(scope.key, s.id, turn, SuperviseAttached{Reviewer: sup.session.id, Worker: turn, Instruction: sup.instruction, Interval: o.every})
		}
	}

	// Each supervisor, on its own clock: look at what is new, steer on
	// objection, stop when the turn ends. A look is a turn with the
	// supervisor's own options, so it can be supervised in turn.
	lookCtx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	defer func() {
		cancel()
		wg.Wait()
	}()
	for reader, sup := range supervisors {
		wg.Go(func() {
			tick := time.NewTicker(apply(sup.opts).every)
			defer tick.Stop()
			first := true
			for {
				select {
				case <-lookCtx.Done():
					return
				case <-tick.C:
				}
				look := t.look(reader, sup, prompt, first, history)
				if look == "" {
					continue
				}
				first = false
				review, err := dispatch[review](withSteerSource(lookCtx, sup.session.id), sup.session, look, sup.opts)
				if err != nil {
					if lookCtx.Err() != nil {
						return
					}
					logf("%s: a look at %s failed: %v", sup.session.id, s.id, err)
					continue
				}
				if len(review.Objections) > 0 {
					_, _ = s.Steer(withSteerSource(lookCtx, sup.session.id), "Your supervisor objects:\n\n- "+strings.Join(review.Objections, "\n- "))
				}
			}
		})
	}
	var out T
	return generate[T](ctx, s, prompt, t.append, fmt.Sprintf("%T", out))
}

// lookIntro carries the fixed instructions, plus bounded copies of the
// supervisor's instruction and the worker's task on the first look.
func lookIntro(sup supervisor, prompt string, first bool) string {
	var b strings.Builder
	if first {
		fmt.Fprintf(&b, "You are supervising another agent in %s while it works. What you watch for:\n\n%s\n\n",
			clipText(sup.session.workdir, supervisorPathBytes), clipText(sup.instruction, supervisorInstructionBytes))
		fmt.Fprintf(&b, "The agent was asked:\n\n%s\n\n", clipText(prompt, supervisorTaskBytes))
		b.WriteString("Every few minutes you are shown what it did since your last look; tool output is shortened, so read the files yourself when you need more, and change none. Object only when what it does goes against what you watch for; each objection is sent to the agent at once, as an instruction. When you have no objection, answer with an empty list.\n\n")
	}
	return b.String()
}

// clipText returns an owned string no larger than limit. Owning the clipped
// bytes matters here: retaining a slice of a large tool result would retain
// the large result's backing storage too.
func clipText(text string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if len(text) <= limit {
		return strings.Clone(text)
	}
	keep := limit
	for {
		marker := fmt.Sprintf(" [... %d bytes omitted]", len(text)-keep)
		if len(marker) >= limit {
			return strings.Clone(marker[:limit])
		}
		newKeep := limit - len(marker)
		if newKeep == keep {
			prefix := strings.ToValidUTF8(text[:keep], "")
			return strings.Clone(prefix) + marker
		}
		keep = newKeep
	}
}

func clipContent(text string, limit int) (string, int) {
	if limit <= 0 {
		return "", len(text)
	}
	if len(text) <= limit {
		return strings.Clone(text), 0
	}
	prefix := strings.Clone(strings.ToValidUTF8(text[:limit], ""))
	return prefix, len(text) - len(prefix)
}

func transcriptIdentity(value string) string {
	const visibleBytes = 160
	if len(value) <= visibleBytes {
		return strings.Clone(value)
	}
	digest := sha256.Sum256([]byte(value))
	return clipText(value, visibleBytes) + fmt.Sprintf(" sha256=%x", digest)
}

type transcriptPart struct {
	prefix      string
	text        string
	coalesceKey string
}

// transcriptPartFor keeps only the activity supervisors already inspect. It
// makes message and tool identity explicit so incremental fragments cannot be
// mistaken for a different item.
func transcriptPartFor(e AgentEvent) (transcriptPart, bool) {
	identity := func(value string) string { return transcriptIdentity(value) }
	var data struct {
		AssistantMessageID string          `json:"assistantMessageID"`
		ID                 string          `json:"id"`
		Delta              string          `json:"delta"`
		Text               string          `json:"text"`
		Input              json.RawMessage `json:"input"`
		Content            json.RawMessage `json:"content"`
		Metadata           json.RawMessage `json:"metadata"`
		Error              json.RawMessage `json:"error"`
		Item               struct {
			Type    string `json:"type"`
			Payload struct {
				Text string `json:"text"`
			} `json:"payload"`
		} `json:"item"`
	}
	if json.Unmarshal(e.Data, &data) != nil {
		return transcriptPart{}, false
	}
	switch e.Type {
	case "session.inbox.enqueued":
		if data.Item.Type != "user" {
			return transcriptPart{}, false
		}
		return transcriptPart{prefix: "[message to the agent] ", text: data.Item.Payload.Text}, true
	case "session.text.ended":
		return transcriptPart{prefix: fmt.Sprintf("[agent] message_id=%s ", identity(data.AssistantMessageID)), text: data.Text}, true
	case "session.text.delta":
		id := identity(data.AssistantMessageID)
		return transcriptPart{prefix: fmt.Sprintf("[agent fragment] message_id=%s ", id), text: data.Delta, coalesceKey: "assistant:" + id}, true
	case "session.tool.called":
		return transcriptPart{prefix: fmt.Sprintf("[tool call] call_id=%s ", identity(data.ID)), text: string(data.Input)}, true
	case "session.tool.input.delta":
		id := identity(data.ID)
		return transcriptPart{prefix: fmt.Sprintf("[tool input fragment] call_id=%s ", id), text: data.Delta, coalesceKey: "tool-input:" + id}, true
	case "session.tool.success":
		return transcriptPart{prefix: fmt.Sprintf("[tool result] call_id=%s ", identity(data.ID)), text: string(data.Content)}, true
	case "session.tool.failed":
		return transcriptPart{prefix: fmt.Sprintf("[tool failed] call_id=%s ", identity(data.ID)), text: string(data.Error)}, true
	case "session.tool.progress":
		id := identity(data.ID)
		return transcriptPart{prefix: fmt.Sprintf("[tool result fragment] call_id=%s ", id), text: string(data.Metadata), coalesceKey: "tool-result:" + id}, true
	default:
		return transcriptPart{}, false
	}
}

type transcriptEntry struct {
	first, last uint64
	prefix      string
	text        string
	omitted     int
	coalesceKey string
}

func newTranscriptEntry(seq uint64, part transcriptPart) transcriptEntry {
	text, omitted := clipContent(part.text, supervisorEventTextBytes)
	return transcriptEntry{
		first:       seq,
		last:        seq,
		prefix:      strings.Clone(part.prefix),
		text:        text,
		omitted:     omitted,
		coalesceKey: strings.Clone(part.coalesceKey),
	}
}

func (e *transcriptEntry) append(text string) {
	if e.omitted > 0 {
		e.omitted += len(text)
		return
	}
	remaining := supervisorEventTextBytes - len(e.text)
	if remaining <= 0 {
		e.omitted += len(text)
		return
	}
	clipped, omitted := clipContent(text, remaining)
	e.text += clipped
	e.omitted += omitted
}

func (e transcriptEntry) render() string {
	if e.omitted == 0 {
		return e.prefix + e.text + "\n"
	}
	return e.prefix + e.text + fmt.Sprintf(" [... %d bytes omitted]\n", e.omitted)
}

func (e transcriptEntry) bytes() int {
	n := len(e.prefix) + len(e.text)
	if e.omitted == 0 {
		return n + 1 // newline
	}
	return n + len(" [...  bytes omitted]\n") + decimalDigits(e.omitted)
}

func decimalDigits(n int) int {
	digits := 1
	for n >= 10 {
		n /= 10
		digits++
	}
	return digits
}

type transcriptBatch struct {
	lines          []string
	missingFirst   uint64
	missingThrough uint64
}

// transcript is a byte-bounded recent view of one worker turn. Readers are
// the active supervisors; their cursors let consumed entries be released.
type transcript struct {
	mu       sync.Mutex
	entries  []transcriptEntry
	readers  []uint64
	next     uint64
	bytes    int
	maxBytes int
	peak     int
}

func newTranscript(readers, maxBytes int) *transcript {
	t := &transcript{readers: make([]uint64, readers), next: 1, maxBytes: maxBytes}
	for i := range t.readers {
		t.readers[i] = 1
	}
	return t
}

func (t *transcript) append(e AgentEvent) error {
	part, ok := transcriptPartFor(e)
	if !ok {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	seq := t.next
	t.next++
	if n := len(t.entries); n > 0 && part.coalesceKey != "" && t.entries[n-1].coalesceKey == part.coalesceKey && !t.consumed(t.entries[n-1]) {
		old := t.entries[n-1].bytes()
		t.entries[n-1].append(part.text)
		t.entries[n-1].last = seq
		t.bytes += t.entries[n-1].bytes() - old
	} else {
		entry := newTranscriptEntry(seq, part)
		t.entries = append(t.entries, entry)
		t.bytes += entry.bytes()
	}
	t.trimToLimit()
	if t.bytes > t.peak {
		t.peak = t.bytes
	}
	return nil
}

func (t *transcript) consumed(entry transcriptEntry) bool {
	for _, cursor := range t.readers {
		if cursor > entry.first {
			return true
		}
	}
	return false
}

func (t *transcript) trimToLimit() {
	for t.bytes > t.maxBytes && len(t.entries) > 0 {
		t.dropFirst()
	}
}

func (t *transcript) dropFirst() {
	entry := t.entries[0]
	t.bytes -= entry.bytes()
	clear(t.entries[:1])
	t.entries = t.entries[1:]
	if len(t.entries) == 0 {
		t.entries = nil
	}
}

func (t *transcript) trimConsumed() {
	if len(t.readers) == 0 {
		return
	}
	through := t.readers[0]
	for _, cursor := range t.readers[1:] {
		if cursor < through {
			through = cursor
		}
	}
	for len(t.entries) > 0 && t.entries[0].last < through {
		t.dropFirst()
	}
}

// since advances one supervisor to the current end and returns only the newest
// entries that fit. Any retained-history or prompt-budget loss is one explicit
// missing interval before those entries.
func (t *transcript) since(reader, maxBytes int) transcriptBatch {
	t.mu.Lock()
	defer t.mu.Unlock()
	if reader < 0 || reader >= len(t.readers) {
		panic("gimble: invalid supervisor transcript reader")
	}
	cursor := t.readers[reader]
	first := len(t.entries)
	used := 0
	for i, entry := range slices.Backward(t.entries) {

		if entry.last < cursor {
			break
		}
		if size := entry.bytes(); used+size > maxBytes {
			break
		} else {
			used += size
			first = i
		}
	}
	batch := transcriptBatch{lines: make([]string, 0, len(t.entries)-first)}
	selected := t.next
	if first < len(t.entries) {
		selected = t.entries[first].first
		for _, entry := range t.entries[first:] {
			if entry.last >= cursor {
				batch.lines = append(batch.lines, entry.render())
			}
		}
	}
	if selected > cursor {
		batch.missingFirst = cursor
		batch.missingThrough = selected - 1
	}
	t.readers[reader] = t.next
	t.trimConsumed()
	return batch
}

// look asks a supervisor for objections to its unread bounded view. An empty
// result means no renderable worker activity arrived since its last look.
func (t *transcript) look(reader int, sup supervisor, prompt string, first bool, history string) string {
	intro := lookIntro(sup, prompt, first)
	const heading = "What the agent did since your last look:\n\n"
	eventBudget := max(supervisorLookBytes-len(intro)-len(heading)-supervisorGapReserveBytes, 0)
	batch := t.since(reader, eventBudget)
	if len(batch.lines) == 0 && batch.missingThrough == 0 {
		return ""
	}
	var b strings.Builder
	b.Grow(supervisorLookBytes)
	b.WriteString(intro)
	b.WriteString(heading)
	if batch.missingThrough != 0 {
		fmt.Fprintf(&b, "[gap] Supervisor activity items %d through %d are missing from this look. The recent view below is complete only after that gap. Inspect the durable worker transcript at %s if the missing interval matters.\n\n",
			batch.missingFirst, batch.missingThrough, clipText(history, supervisorPathBytes))
	}
	for _, line := range batch.lines {
		b.WriteString(line)
	}
	if b.Len() > supervisorLookBytes {
		panic("gimble: supervisor look exceeded its byte budget")
	}
	return b.String()
}
