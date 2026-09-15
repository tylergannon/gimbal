package workflow

// Graph is the shape of one workflow as its source writes it: the scopes,
// the sessions declared in them, the agent calls those sessions make, the
// commands, and the context each scope writes, in order. It describes a
// workflow before it runs.
//
// It is not a model of the program. Ordinary Go calculation, error handling,
// and early exits are absent. While a run is going the runtime records what
// actually happened, and a viewer shows the two together, so nothing the
// runtime already records belongs here: no adapter, model, or reasoning
// effort, which are parameters of a run; no results, timings, or costs.
//
// A node carries no identifier. The runtime names every scope, session, and
// turn in one namespace of name.ordinal segments, so a node is addressed by
// its path of names from the root, which is a runtime key with the ordinal
// stripped from each segment. This graph's coder sits at
// round/sprint/task/coder and a run's at round.2/sprint.1/task.3/coder.1.
type Graph struct {
	// Name is the workflow's name, which is also the run's name.
	Name string `json:"name"`
	// Source anchors the entry function.
	Source Source `json:"source"`
	// Body is the entry function's operations, in source order.
	Body []Operation `json:"body"`
	// Diagnostics records what the extractor could not read. A graph with
	// diagnostics has honest holes; it is never completed by a guess.
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// Source is where an operation is written, so a reader can go look at it.
type Source struct {
	// File is the path relative to the module root, with forward slashes.
	File string `json:"file"`
	Line int    `json:"line"`
}

// Operation is the sealed union of what an ordered body contains. Its
// variants are exactly the same-package structs that declare operation().
type Operation interface {
	operation()
}

// Session is one gimble.NewSession or Session.Fork, standing where the source
// creates it: the body containing it is the scope that owns the conversation,
// which is how the runtime names it too. From is the name of the session this
// one forks, empty for NewSession. Which harness and model it runs on are a
// run's parameters and are not here.
type Session struct {
	Source
	Name string `json:"name"`
	From string `json:"from"`
}

// AgentCall is one Session.Generate. Session is the name of the session that
// speaks, resolved outward to the nearest body declaring it, so two calls on
// one name are two steps on one conversation.
type AgentCall struct {
	Source
	Session string `json:"session"`
	// Prompt is the prompt the source composes, with its constants filled in
	// and every part known only at run time left as a {{ hole }} naming the
	// expression behind it. Prompts will later be authored as text/template;
	// until then this is that text by hand, and nothing parses it.
	Prompt string `json:"prompt"`
	// Output is Generate's type argument, which the run records per turn.
	Output string `json:"output"`
	// Supervisors watch this call. They are unordered and are not steps in
	// the body. Their looks and steers are the run's record.
	Supervisors []Supervisor `json:"supervisors"`
}

// Supervisor is one gimble.WithSupervisor on a call: a session watching the
// work and what it was told to watch for. Its own supervisors watch its look
// turns. How often it looks is a knob the run records, not shape.
type Supervisor struct {
	Source
	Session     string       `json:"session"`
	Instruction string       `json:"instruction"`
	Supervisors []Supervisor `json:"supervisors"`
}

// Command is one process the workflow runs. Text is the command as written,
// with runtime parts left as {{ holes }}. The runtime records no commands, so
// this is the one operation with no counterpart in a run.
type Command struct {
	Source
	Text string `json:"text"`
}

// Set is one gimble.Set or SetJSON: a key written into the containing scope.
// The order of these in a body is the order an agent reads them back through
// ScopeText. The value written is the run's, not the source's.
type Set struct {
	Source
	Key string `json:"key"`
}

// Scope is a gimble.Scope call: a named body that owns the sessions declared
// in it and holds the values set in it.
type Scope struct {
	Source
	Name string      `json:"name"`
	Body []Operation `json:"body"`
}

// Loop is planner-directed dispatch: gimble.Loop and the range over its Tasks.
// Planner is the planner session's name and Goal its standing instruction.
// Body is the per-task body, which the runtime places in a scope named "task"
// beneath the loop's own and where it writes the key "task". How many tasks
// there will be is the planner's decision and is not knowable here.
type Loop struct {
	Source
	Name    string      `json:"name"`
	Planner string      `json:"planner"`
	Goal    string      `json:"goal"`
	Body    []Operation `json:"body"`
}

// Group is a gimble.Group call. Its children run concurrently; their order is
// source order and implies nothing about the order they finish in.
type Group struct {
	Source
	Name     string       `json:"name"`
	Children []GroupChild `json:"children"`
}

// GroupChild is one Group.Go call: a named body in its own scope.
type GroupChild struct {
	Source
	Name string      `json:"name"`
	Body []Operation `json:"body"`
}

// Condition is an if/else chain or a switch, recorded only when one of its
// branches contains an operation; a branch guarding nothing but error
// handling is not shape and does not appear. Branch order is source order and
// is not an execution order. A Condition creates no Gimble scope.
type Condition struct {
	Source
	Branches []Branch `json:"branches"`
}

// Branch is one alternative. Case is the condition or case as written; an
// empty Case is the else or default branch.
type Branch struct {
	Source
	Case string      `json:"case"`
	Body []Operation `json:"body"`
}

// Diagnostic is one thing the extractor could not read.
type Diagnostic struct {
	Source  Source `json:"source"`
	Message string `json:"message"`
}

func (Session) operation()   {}
func (AgentCall) operation() {}
func (Command) operation()   {}
func (Set) operation()       {}
func (Scope) operation()     {}
func (Loop) operation()      {}
func (Group) operation()     {}
func (Condition) operation() {}
