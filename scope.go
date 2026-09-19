package gimble

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// closeTimeout bounds one adapter's Close call, on a context independent of
// the run's own ctx so a cancelled run still releases everything it holds.
const closeTimeout = 10 * time.Second

// Output is what polytype generates, and what Generate and SetJSON require.
type Output interface {
	Schema() json.RawMessage
	ValidateJSON([]byte) error
}

type scopeKey struct{}
type taskKey struct{}

// scope is one instance of a named span of the workflow. Nothing is in the
// ctx but a pointer to it.
type scope struct {
	run    *run
	parent *scope
	key    string                  // names with ordinals from the root, as in lap.3/bakeoff.1/attempt.2; "" for the root
	ctx    context.Context         // canonical lifetime context for work owned by this scope
	cancel context.CancelCauseFunc // ends the scope's ctx; run.cancelScope reaches it by key

	loop bool // a PromiseLoop's own scope, which takes messages for its planner

	mu          sync.Mutex
	ordinals    map[string]int // the last ordinal given to each child scope and session name
	keys        []string       // in the order they were set
	values      map[string]*scopeValue
	sessions    []*Session
	services    []*ownedService
	serviceErr  error
	ended       bool
	dispatching bool     // the loop's body is running, so its planner can still be reached
	messages    []string // messages waiting for the planner's next decision
}

// scopeValue is the immutable semantic value Set captured. Its representation
// may move from raw JSON to a run artifact when either the value or the visible
// aggregate exceeds a prompt budget.
type scopeValue struct {
	owner    *scope
	raw      []byte
	artifact *artifactDescriptor
}

// queueMessage holds message for the planner of a loop that is still
// dispatching, and reports whether it will be read. It is held rather than
// delivered: a planner is not always in a turn, so a message to a loop
// waits for its next planning turn instead of being dropped.
func (s *scope) queueMessage(message string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.dispatching || s.ended {
		return false
	}
	s.messages = append(s.messages, message)
	return true
}

// takeMessages empties the queue for the planner's next prompt.
func (s *scope) takeMessages() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	messages := s.messages
	s.messages = nil
	return messages
}

// endDispatch closes the loop to further messages and returns any that
// never reached its planner.
func (s *scope) endDispatch() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dispatching = false
	messages := s.messages
	s.messages = nil
	return messages
}

func current(ctx context.Context) (*scope, error) {
	if s, _ := ctx.Value(scopeKey{}).(*scope); s != nil {
		return s, nil
	}
	return nil, errors.New("gimble: no scope in the ctx; it must come from gimble.Run")
}

// next names the next child of s called name. The caller holds s.mu.
func (s *scope) next(name string) string {
	if s.ordinals == nil {
		s.ordinals = make(map[string]int)
	}
	s.ordinals[name]++
	return path.Join(s.key, fmt.Sprintf("%s.%d", name, s.ordinals[name]))
}

func (s *scope) child(name string) *scope {
	s.mu.Lock()
	defer s.mu.Unlock()
	return &scope{run: s.run, parent: s, key: s.next(name)}
}

// adopt makes session belong to s, which closes it when s ends.
func (s *scope) adopt(session *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session.id = s.next(session.name)
	session.closed = s.ended
	s.sessions = append(s.sessions, session)
}

// adoptService makes service belong to s, which stops it when s ends.
func (s *scope) adoptService(service *ownedService) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return fmt.Errorf("gimble: service %q: scope %q ended", service.name, s.key)
	}
	s.services = append(s.services, service)
	return nil
}

// failService records the first required service that disappeared and
// cancels the scope so work which follows its ctx starts unwinding.
func (s *scope) failService(err error) {
	if err == nil {
		return
	}
	s.mu.Lock()
	if s.serviceErr != nil {
		s.mu.Unlock()
		return
	}
	s.serviceErr = err
	cancel := s.cancel
	s.mu.Unlock()
	cancel(err)
}

// do runs body as the scope: it begins when body is called and ends when
// body returns.
func (s *scope) do(ctx context.Context, body func(context.Context) error) (err error) {
	ctx, s.cancel = context.WithCancelCause(context.WithValue(ctx, scopeKey{}, s))
	s.ctx = ctx
	s.run.addScope(s)
	e := ScopeBegan{Name: path.Base(s.key), Loop: s.loop}
	if task, ok := ctx.Value(taskKey{}).(Task); ok {
		e.Task = optionalTask(task)
	}
	s.run.event(s.key, "", "", e)
	defer func() {
		if recovered := recover(); recovered != nil {
			_ = s.end()
			panic(recovered)
		}
		err = s.finish(err)
	}()
	return body(ctx)
}

// finish stops the scope's services, closes its sessions, cancels its ctx,
// records its result, and returns its body's result joined with any required
// service failure or incomplete service cleanup.
func (s *scope) finish(bodyErr error) error {
	serviceCleanupErr := s.end()
	s.mu.Lock()
	serviceErr := s.serviceErr
	s.mu.Unlock()
	if serviceErr != nil && (bodyErr == nil || errors.Is(bodyErr, context.Canceled)) {
		bodyErr = serviceErr
	} else {
		bodyErr = errors.Join(bodyErr, serviceErr)
	}
	err := errors.Join(bodyErr, serviceCleanupErr)
	s.run.event(s.key, "", "", ScopeEnded{Error: errString(err)})
	return err
}

// end stops the scope's services, closes its sessions, then cancels its ctx.
// Each service is marked as intentionally stopping before any signal is sent,
// so a concurrent exit is classified by whichever lifecycle transition won.
// Service cleanup failures are returned because the scope owns that process.
// Each session is
// marked closed before its adapter is asked to release it, so no Generate
// can enter while cleanup runs. A session with no native id had nothing
// allocated by its adapter and gets no Close call, only its SessionClosed
// event. Close runs on a timeout ctx independent of the run's own, so a
// cancelled run still releases every native session. A Close failure never
// changes the scope's own result (recorded on SessionClosed and folded into
// this run's aggregate close error instead); Run joins that aggregate into
// its returned error once the root scope has ended. The scope leaves the
// run's table first, so a kill by key cannot reach a scope that is ending.
func (s *scope) end() error {
	s.run.removeScope(s)
	s.mu.Lock()
	s.ended = true
	services := append([]*ownedService(nil), s.services...)
	for _, service := range services {
		service.beginStop()
	}
	sessions := s.sessions
	s.mu.Unlock()
	var serviceErrs []error
	for _, service := range services {
		if err := service.stop(); err != nil {
			serviceErrs = append(serviceErrs, err)
		}
	}
	for _, session := range sessions {
		session.mu.Lock()
		session.closed = true
		native := session.native
		session.mu.Unlock()
		var closeErr error
		if native != "" {
			closeCtx, cancel := context.WithTimeout(context.Background(), closeTimeout)
			closeErr = session.adapter.Close(closeCtx, native)
			cancel()
			if closeErr != nil {
				s.run.recordCloseFailure(session.id, closeErr)
			}
		}
		s.run.event(s.key, session.id, "", SessionClosed{Error: errString(closeErr)})
	}
	s.cancel(nil)
	return errors.Join(serviceErrs...)
}

// Scope runs body in a child scope named name, and returns its error. The
// scope ends when body returns: each service it started is stopped, each
// session it created is closed through its adapter's Close, then its ctx is
// cancelled. A service failure enters the scope's error. A session Close
// failure is recorded, not returned here: it instead surfaces from the run's
// own aggregate cleanup error.
func Scope(ctx context.Context, name string, body func(ctx context.Context) error) error {
	parent, err := current(ctx)
	if err != nil {
		return err
	}
	return parent.child(name).do(ctx, body)
}

// Set stores a scalar in the ctx's scope. A key is set once per scope
// instance; revision is shadowing, in a child scope. Misuse is a
// programming error and panics with a message naming the key and the
// scope: a key already set in this scope, a scope that has ended, or a
// ctx with no scope. The run records the panic before it escapes.
func Set[V ~string | ~int | ~bool | ~[]string](ctx context.Context, key string, value V) {
	store(ctx, key, encode(key, value))
}

// SetJSON stores a polytype-generated value in the ctx's scope, once per
// key like Set, and panics on misuse like Set.
func SetJSON[V Output](ctx context.Context, key string, value V) {
	store(ctx, key, encode(key, value))
}

func encode(key string, value any) []byte {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("gimble: set %q: %v", key, err))
	}
	return raw
}

func store(ctx context.Context, key string, raw []byte) {
	s, _ := ctx.Value(scopeKey{}).(*scope)
	if s == nil {
		panic(fmt.Sprintf("gimble: set %q: no scope in the ctx; it must come from gimble.Run", key))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		panic(fmt.Sprintf("gimble: set %q in scope %q after it ended", key, s.key))
	}
	if _, ok := s.values[key]; ok {
		panic(fmt.Sprintf("gimble: %q is already set in scope %q", key, s.key))
	}
	if s.values == nil {
		s.values = make(map[string]*scopeValue)
	}
	value := &scopeValue{owner: s, raw: append([]byte(nil), raw...)}
	if tokenCount("## "+key+"\n\n"+render(raw)) > contextEntryTokenLimit {
		if err := s.spillValueLocked(key, value); err != nil {
			panic(fmt.Sprintf("gimble: set %q: write artifact: %v", key, err))
		}
	}
	s.values[key] = value
	s.keys = append(s.keys, key)
	s.run.event(s.key, "", "", valueEvent(key, value))
}

func (s *scope) spillValueLocked(key string, value *scopeValue) error {
	if value.artifact != nil {
		return nil
	}
	var text string
	format, extension := "json", ".json"
	var scalar string
	data := value.raw
	if json.Unmarshal(value.raw, &scalar) == nil {
		format, extension = "text", ".txt"
		text, data = scalar, []byte(scalar)
	} else {
		text = render(value.raw)
	}
	relative := filepath.ToSlash(filepath.Join("values", encodedPath(s.key), encodedComponent(key)+extension))
	desc, err := s.run.writeArtifact(relative, data)
	if err != nil {
		return err
	}
	desc.format, desc.preview = format, preview(text)
	value.raw, value.artifact = nil, &desc
	return nil
}

func (s *scope) ensureArtifact(key string, value *scopeValue) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if value.artifact != nil {
		return nil
	}
	if err := s.spillValueLocked(key, value); err != nil {
		return err
	}
	// This is a representation update for the same immutable value. Reducers
	// replace the inline row, so finished tables and a log rebuild agree.
	s.run.event(s.key, "", "", valueEvent(key, value))
	return nil
}

// ScopeValue is one value visible from a scope, as a WithScopeTemplate
// template sees it.
type ScopeValue struct {
	// Key is what the value was set under.
	Key string
	// Value is the value itself, decoded from the JSON it was stored as: a
	// string for Set, and for SetJSON the fields of the object.
	Value any
	// Text is the value as the default rendering shows it: a string as its
	// text, anything else as indented JSON.
	Text string
}

// ScopeData is the argument of a WithScopeTemplate template: the scoped
// data Generate would otherwise render its own way.
type ScopeData struct {
	// Values is every value visible from the ctx's scope, outermost scope
	// first, and for each key the value of the nearest scope that set it.
	Values []ScopeValue
	// By is the same values by key, for a template that names the ones it
	// wants: {{.By.goal.Text}}, or index .By "definition of done" for a key
	// that is not an identifier.
	By map[string]ScopeValue
}

// scopeData collects every value visible from the ctx's scope, outermost
// scope first, and for each key the value of the nearest scope that set it.
func scopeData(ctx context.Context) ScopeData {
	data := ScopeData{By: map[string]ScopeValue{}}
	for _, visible := range visibleValues(ctx) {
		raw, err := valueBytes(visible.value)
		if err != nil {
			panic(fmt.Sprintf("gimble: read scope value %q: %v", visible.key, err))
		}
		value := ScopeValue{Key: visible.key, Text: render(raw)}
		if err := json.Unmarshal(raw, &value.Value); err != nil {
			value.Value = value.Text
		}
		data.By[visible.key] = value
		data.Values = append(data.Values, value)
	}
	return data
}

// scopeText renders every value visible from the ctx's scope for a prompt:
// outermost scope first, and for each key the value of the nearest scope
// that set it. Generate appends this to a turn's prompt itself; a workflow
// no longer calls it.
func scopeText(ctx context.Context) string {
	visible := visibleValues(ctx)
	if len(visible) == 0 {
		return ""
	}
	sections := renderVisible(visible, -1)
	text := strings.Join(sections, "\n\n")
	if tokenCount(text) <= contextTokenLimit {
		return text
	}
	for _, item := range visible {
		if err := item.owner.ensureArtifact(item.key, item.value); err != nil {
			panic(fmt.Sprintf("gimble: render scope value %q: write artifact: %v", item.key, err))
		}
	}
	low, high := 0, artifactPreviewBytes
	best := renderVisible(visible, 0)
	for low <= high {
		middle := low + (high-low)/2
		candidate := renderVisible(visible, middle)
		if tokenCount(strings.Join(candidate, "\n\n")) <= contextTokenLimit {
			best = candidate
			low = middle + 1
		} else {
			high = middle - 1
		}
	}
	text = strings.Join(best, "\n\n")
	if tokenCount(text) <= contextTokenLimit {
		return text
	}
	var index strings.Builder
	for _, item := range visible {
		fmt.Fprintf(&index, "## %s\n\nComplete value: %s\n\n", item.key, artifactAbsolute(item.owner.run, *item.value.artifact))
	}
	desc, err := visible[0].owner.run.writeContentArtifact("scope-index", strings.TrimSpace(index.String()))
	if err != nil {
		panic(fmt.Sprintf("gimble: write scope index: %v", err))
	}
	return fitExcerpt(index.String(), artifactAbsolute(visible[0].owner.run, desc), contextTokenLimit)
}

// localText renders only the values written in this scope. PromiseLoop uses it to
// carry a completed task's record forward without promoting those values into
// the parent scope.
func (s *scope) localText() string {
	ctx := context.WithValue(context.Background(), scopeKey{}, s)
	visible := visibleValues(ctx)
	local := visible[:0]
	for _, item := range visible {
		if item.owner == s {
			local = append(local, item)
		}
	}
	return strings.Join(renderVisible(local, -1), "\n\n")
}

type visibleValue struct {
	owner *scope
	key   string
	value *scopeValue
}

func visibleValues(ctx context.Context) []visibleValue {
	seen := map[string]bool{}
	var values []visibleValue
	for s, _ := ctx.Value(scopeKey{}).(*scope); s != nil; s = s.parent {
		s.mu.Lock()
		for _, key := range slices.Backward(s.keys) {
			if !seen[key] {
				seen[key] = true
				values = append(values, visibleValue{owner: s, key: key, value: s.values[key]})
			}
		}
		s.mu.Unlock()
	}
	slices.Reverse(values)
	return values
}

func renderVisible(values []visibleValue, bodyBytes int) []string {
	sections := make([]string, 0, len(values))
	for _, item := range values {
		item.owner.mu.Lock()
		raw := append([]byte(nil), item.value.raw...)
		var artifact *artifactDescriptor
		if item.value.artifact != nil {
			copy := *item.value.artifact
			artifact = &copy
		}
		item.owner.mu.Unlock()
		var text string
		if artifact == nil {
			text = render(raw)
		} else {
			desc := *artifact
			path := artifactAbsolute(item.owner.run, desc)
			prefix := ""
			if desc.format == "json" {
				prefix = "JSON excerpt (the complete JSON is in the referenced file):\n\n"
			}
			maxBytes := artifactPreviewBytes
			if bodyBytes >= 0 {
				maxBytes = bodyBytes
			}
			allowed := contextEntryTokenLimit - tokenCount("## "+item.key+"\n\n"+prefix)
			text = prefix + fitExcerptBytes(desc.preview, path, allowed, maxBytes)
		}
		sections = append(sections, "## "+item.key+"\n\n"+text)
	}
	return sections
}

// render shows a JSON string as its text and anything else as indented JSON.
func render(raw []byte) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var b bytes.Buffer
	if json.Indent(&b, raw, "", "  ") != nil {
		return string(raw)
	}
	return b.String()
}
