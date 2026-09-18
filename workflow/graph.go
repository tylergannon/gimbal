package workflow

//go:generate go tool polytype --typescript ../web/src/lib/workflow

// Graph is the shape of one workflow as its source writes it: the scopes,
// the sessions declared in them, the agent calls those sessions make, the
// commands, and the context each scope writes, in order. It describes a
// workflow before it runs.
//
// It is not a model of the program. Ordinary Go calculation and error
// handling are absent. While a run is going the runtime records what
// actually happened, and a viewer shows the two together, so nothing the
// runtime already records belongs here: no adapter, model, or reasoning
// effort, which are a run's bindings; no results, timings, or costs.
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
// which is how the runtime names it too. Name is the role for NewSession and
// the fork's name for Fork. From is the name of the session this one forks,
// empty for NewSession. What it runs on is a run's binding and is not here.
type Session struct {
	Source
	Name string `json:"name"`
	From string `json:"from"`
}

// AgentCall is one Session.Generate. Session is the name of the session that
// speaks, resolved outward to the nearest body declaring it, so two calls on
// one name are two steps on one conversation. Role is the role that session
// runs as: its own name for a NewSession, its ancestor's for a fork, so the
// coder's role is "researcher".
type AgentCall struct {
	Source
	Session string `json:"session"`
	Role    string `json:"role"`
	// Prompt is the constant the source passes (#230). The scope's context
	// the run appends to it is the run's record.
	Prompt string `json:"prompt"`
	// Supervisors watch this call. They are unordered and are not steps in
	// the body. Their looks and steers are the run's record.
	Supervisors []Supervisor `json:"supervisors"`
}

// Interview is one gimble.Interview. Session is the conversation conducting
// it; its questions, answers, and internal turns are runtime facts.
type Interview struct {
	Source
	Name    string `json:"name"`
	Session string `json:"session"`
}

// Supervisor is one gimble.WithSupervisor on a call: a session watching the
// work and the constant it was told to watch for. Its own supervisors watch
// its look turns. How often it looks is a knob the run records, not shape.
type Supervisor struct {
	Source
	Session     string       `json:"session"`
	Role        string       `json:"role"`
	Instruction string       `json:"instruction"`
	Supervisors []Supervisor `json:"supervisors"`
}

// Command is one gimble.RunCommand. Name is the constant name the call
// gives it; the command line and its outcome are the run's record.
type Command struct {
	Source
	Name string `json:"name"`
}

// Set is one gimble.Set or SetJSON: a key written into the containing scope.
// The order of these in a body is the order an agent reads them back. The
// value written is the run's, not the source's.
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

// PromiseLoop is planner-directed dispatch. Planner names the planner session;
// Supervisors watch each planning decision. Body runs under a child scope named
// "task", where the runtime writes key "task". How many tasks there will be is
// the planner's decision.
type PromiseLoop struct {
	Source
	Name        string       `json:"name"`
	Planner     string       `json:"planner"`
	Supervisors []Supervisor `json:"supervisors"`
	Body        []Operation  `json:"body"`
}

// Iterate ranges over a collection supplied by ordinary Go. Each item runs in
// a fresh child scope named Name. The collection's values and length are
// runtime data, so the graph records only the scoped body.
type Iterate struct {
	Source
	Name string      `json:"name"`
	Body []Operation `json:"body"`
}

// Repeat is a Go for or range statement whose body contains an operation,
// such as the sprint's rounds. Cond is the loop header as written. It creates
// no Gimble scope; how many times it runs is the run's record.
type Repeat struct {
	Source
	Cond string      `json:"cond"`
	Body []Operation `json:"body"`
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

// Condition is an if/else chain or a switch, recorded when one of its
// branches contains an operation or exits. A branch guarding nothing but
// error handling is not shape and does not appear. Branch order is source
// order and is not an execution order. A Condition creates no Gimble scope.
type Condition struct {
	Source
	Branches []Branch `json:"branches"`
}

// Branch is one alternative. Case is the condition or case as written; an
// empty Case is the else or default branch. Exits is set when the branch
// ends in break, continue, or return, so the operations after the Condition
// are what happens when no exiting branch is taken: the sprint commits only
// when the task passed, and stops its rounds when the validator has no
// findings.
type Branch struct {
	Source
	Case  string      `json:"case"`
	Exits bool        `json:"exits"`
	Body  []Operation `json:"body"`
}

// Diagnostic is one thing the extractor could not read.
type Diagnostic struct {
	Source  Source `json:"source"`
	Message string `json:"message"`
}

func (Session) operation()     {}
func (AgentCall) operation()   {}
func (Interview) operation()   {}
func (Command) operation()     {}
func (Set) operation()         {}
func (Scope) operation()       {}
func (PromiseLoop) operation() {}
func (Iterate) operation()     {}
func (Repeat) operation()      {}
func (Group) operation()       {}
func (Condition) operation()   {}
