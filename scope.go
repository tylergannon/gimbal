package gimble

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
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
	cancel context.CancelCauseFunc // ends the scope's ctx; run.cancelScope reaches it by key

	loop bool // a PromiseLoop's own scope, which takes messages for its planner

	mu          sync.Mutex
	ordinals    map[string]int // the last ordinal given to each child scope and session name
	keys        []string       // in the order they were set
	values      map[string][]byte
	sessions    []*Session
	ended       bool
	dispatching bool     // the loop's body is running, so its planner can still be reached
	messages    []string // messages waiting for the planner's next decision
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

// do runs body as the scope: it begins when body is called and ends when
// body returns.
func (s *scope) do(ctx context.Context, body func(context.Context) error) error {
	ctx, s.cancel = context.WithCancelCause(context.WithValue(ctx, scopeKey{}, s))
	s.run.addScope(s)
	e := ScopeBegan{Name: path.Base(s.key), Loop: s.loop}
	if task, ok := ctx.Value(taskKey{}).(Task); ok {
		e.Task = optionalTask(task)
	}
	s.run.event(s.key, "", "", e)
	defer s.end()
	err := body(ctx)
	s.run.event(s.key, "", "", ScopeEnded{Error: errString(err)})
	return err
}

// end closes the scope's sessions, then cancels its ctx. Each session is
// marked closed before its adapter is asked to release it, so no Generate
// can enter while cleanup runs. A session with no native id had nothing
// allocated by its adapter and gets no Close call, only its SessionClosed
// event. Close runs on a timeout ctx independent of the run's own, so a
// cancelled run still releases every native session. A Close failure never
// changes the scope's own result (recorded on SessionClosed and folded into
// this run's aggregate close error instead); Run joins that aggregate into
// its returned error once the root scope has ended. The scope leaves the
// run's table first, so a kill by key cannot reach a scope that is ending.
func (s *scope) end() {
	s.run.removeScope(s)
	s.mu.Lock()
	s.ended = true
	sessions := s.sessions
	s.mu.Unlock()
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
}

// Scope runs body in a child scope named name, and returns its error. The
// scope ends when body returns: each session it created is closed through
// its adapter's Close, then its ctx is cancelled. A Close failure is
// recorded, not returned here: it never enters this function's error and
// instead surfaces from the run's own aggregate cleanup error.
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
		s.values = make(map[string][]byte)
	}
	s.values[key] = raw
	s.keys = append(s.keys, key)
	s.run.event(s.key, "", "", ValueSet{Key: key, Value: JSONText(raw)})
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
	for s, _ := ctx.Value(scopeKey{}).(*scope); s != nil; s = s.parent {
		s.mu.Lock()
		for _, key := range slices.Backward(s.keys) {
			if _, shown := data.By[key]; shown {
				continue
			}
			raw := s.values[key]
			value := ScopeValue{Key: key, Text: render(raw)}
			if err := json.Unmarshal(raw, &value.Value); err != nil {
				value.Value = value.Text
			}
			data.By[key] = value
			data.Values = append(data.Values, value) // innermost first, reversed below
		}
		s.mu.Unlock()
	}
	slices.Reverse(data.Values)
	return data
}

// scopeText renders every value visible from the ctx's scope for a prompt:
// outermost scope first, and for each key the value of the nearest scope
// that set it. Generate appends this to a turn's prompt itself; a workflow
// no longer calls it.
func scopeText(ctx context.Context) string {
	data := scopeData(ctx)
	sections := make([]string, 0, len(data.Values))
	for _, value := range data.Values {
		sections = append(sections, "## "+value.Key+"\n\n"+value.Text)
	}
	return strings.Join(sections, "\n\n")
}

// localText renders only the values written in this scope. PromiseLoop uses it to
// carry a completed task's record forward without promoting those values into
// the parent scope.
func (s *scope) localText() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	sections := make([]string, 0, len(s.keys))
	for _, key := range s.keys {
		sections = append(sections, "## "+key+"\n\n"+render(s.values[key]))
	}
	return strings.Join(sections, "\n\n")
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
