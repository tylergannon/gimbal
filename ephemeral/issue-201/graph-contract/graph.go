package workflow

// Graph is the root description of one workflow: identity, source and build
// facts, the declared sessions, the ordered root body, and the supervision
// attached to calls executing directly in the run's root scope. A Graph with
// Diagnostics is partial: it stays inspectable but fails the generation gate
// and must not start a run.
type Graph struct {
	// Name is the constant workflow name given to the generator.
	Name string `json:"name"`
	// Package is the import path of the package holding the entry function.
	Package string `json:"package"`
	// Entry is the entry function's name, such as "Sprint".
	Entry string `json:"entry"`
	// Input is the Go type name of the entry function's input parameter.
	Input string `json:"input"`
	// Source anchors the entry function.
	Source Source `json:"source"`
	// Generator is the generator's module version, for example "v0.3.0".
	Generator string `json:"generator"`
	// Sessions describes the named conversations used by the workflow.
	Sessions []Session `json:"sessions"`
	// Body is the entry function's ordered body.
	Body Sequence `json:"body"`
	// Supervision attaches to calls whose execution scope is the run root.
	Supervision []Supervision `json:"supervision"`
	// Diagnostics records unresolved relevant structure. Empty means complete.
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// Source anchors a static site in the generating source tree.
type Source struct {
	// File is the path relative to the module root, with forward slashes.
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

// Site is the identity every static element carries. ID is deterministic for
// unchanged source and distinct from any runtime scope, session, or turn
// instance ID. Same-named sites in different scopes have different IDs, and a
// helper expanded from two callers yields two sites.
type Site struct {
	ID     string `json:"id"`
	Source Source `json:"source"`
}

// Expression is a Go expression retained as text rather than evaluated.
// Prompts, command arguments, conditions, and values stay dynamic; the graph
// records what was written, and runtime observations supply actual values.
// Constant is true when the extractor resolved the expression to a
// compile-time constant, in which case Text is that constant's value.
type Expression struct {
	Text     string `json:"text"`
	Constant bool   `json:"constant"`
}

// Session describes a named conversation using a logical role. The run resolves
// that role to an adapter, model, and reasoning effort; those are not build-time
// properties of this description. Fork provenance is outside this model.
type Session struct {
	Site
	Name string `json:"name"`
	// Role identifies the configured role, such as "coder" or "reviewer".
	Role string `json:"role"`
	// OwnerScope identifies the scope that owns the conversation's lifetime.
	// It does not place calls or supervision attachments: their containing bodies
	// do that. An empty value means the run root. In this draft it references
	// a static scope site; runtime instance identity is not yet represented.
	OwnerScope string `json:"owner_scope"`
}

// Operation is the sealed union of everything an ordered body can contain.
// Variants are exactly the same-package structs that declare operation().
type Operation interface {
	operation()
}

// Subgraph is the sealed subset of Operation whose variants contain ordered
// bodies: Sequence, Condition, Loop, Scope, and Group.
type Subgraph interface {
	Operation
	subgraph()
}

// AgentCall describes one use of a conversation, inheriting its logical role.
// Two calls on the same session are two steps referencing one conversation.
// This draft still carries source-only Prompt and Output metadata; it is not yet
// the shared template/observed-instance representation.
type AgentCall struct {
	Site
	// Session is the static ID of the Session declaration.
	Session string `json:"session"`
	// Prompt is the prompt argument as written.
	Prompt Expression `json:"prompt"`
	// Output is the Go type name of Generate's type argument.
	Output string `json:"output"`
}

// Command is one call to the approved Gimble command execution function,
// whose name and signature are still to be selected. Arguments are the call's
// arguments in order, as written. Constructing or running a subprocess by any
// other means is a lint violation, not a graph element.
type Command struct {
	Site
	Arguments []Expression `json:"arguments"`
}

// Set is a primitive context write at its source position.
type Set struct {
	Site
	// Key is the resolved constant key; a nonconstant key is a diagnostic.
	Key   string     `json:"key"`
	Value Expression `json:"value"`
}

// SetJSON is a structured context write at its source position.
type SetJSON struct {
	Site
	Key   string     `json:"key"`
	Value Expression `json:"value"`
	// Type is the Go type name of the written value.
	Type string `json:"type"`
}

// Exit is a return, break, or continue that changes which relevant work can
// follow. Ordinary returns at the end of a body are not recorded.
type Exit struct {
	Site
	Statement ExitKind `json:"statement"`
}

// ExitKind names the Go statement an Exit stands for.
type ExitKind string

const (
	Return   ExitKind = "return"
	Break    ExitKind = "break"
	Continue ExitKind = "continue"
)

func (ExitKind) enum() {}

// Sequence is an ordered body with its own site. The root body is a Sequence
// anchored at the entry function; a bounded helper expansion is a Sequence
// anchored at the helper's call site so two callers of one helper stay
// distinct.
type Sequence struct {
	Site
	Body []Operation `json:"body"`
}

// Condition is a set of alternative branches from an if/else chain or a
// switch. Branch order is source order, not an execution sequence. A
// Condition creates no Gimble scope; branches write to the enclosing scope.
type Condition struct {
	Site
	Branches []Branch `json:"branches"`
}

// Branch is one alternative. Case is the condition or case expression as
// written; an empty Text is the else/default branch.
type Branch struct {
	Site
	Case Expression  `json:"case"`
	Body []Operation `json:"body"`
}

// Loop is a repeated body. For gimble.Loop, Name is the constant loop name,
// Planner references the planner session, Goal is the goal argument, and Body
// is the per-task template executed in the task scope Tasks establishes. For
// an ordinary Go loop, Name and Planner are empty, Condition is the loop
// condition as written, and no Gimble scope is created.
type Loop struct {
	Site
	Name      string      `json:"name"`
	Planner   string      `json:"planner"`
	Goal      Expression  `json:"goal"`
	Condition Expression  `json:"condition"`
	Body      []Operation `json:"body"`
	// Supervision attaches to calls executing in the task scope.
	Supervision []Supervision `json:"supervision"`
}

// Scope is a gimble.Scope call: a named body with its own context and
// session-ownership boundary.
type Scope struct {
	Site
	Name        string        `json:"name"`
	Body        []Operation   `json:"body"`
	Supervision []Supervision `json:"supervision"`
}

// Group is a gimble.Group call. Children run concurrently; their order is
// source order and implies no dependency between completions.
type Group struct {
	Site
	Name     string       `json:"name"`
	Children []GroupChild `json:"children"`
}

// GroupChild is one Group.Go call: a named body executing in its own scope.
type GroupChild struct {
	Site
	Name        string        `json:"name"`
	Body        []Operation   `json:"body"`
	Supervision []Supervision `json:"supervision"`
}

// Supervision is the unordered hierarchy attached beside one watched call.
type Supervision struct {
	// Target is the watched AgentCall's static ID.
	Target string `json:"target"`
	// Supervisors watch the target; their order carries no meaning.
	Supervisors []Supervisor `json:"supervisors"`
}

// Supervisor is one WithSupervisor attachment. Session references a Session
// declaration, which may be owned by an ancestor scope; the attachment and
// its look turns are local to the watched call. Nested Supervisors watch this
// supervisor's look turns.
type Supervisor struct {
	Site
	Session     string       `json:"session"`
	Instruction Expression   `json:"instruction"`
	Interval    Expression   `json:"interval"`
	Supervisors []Supervisor `json:"supervisors"`
}

// Diagnostic records relevant structure the extractor could not resolve.
type Diagnostic struct {
	Source  Source `json:"source"`
	Message string `json:"message"`
}

func (AgentCall) operation() {}
func (Command) operation()   {}
func (Set) operation()       {}
func (SetJSON) operation()   {}
func (Exit) operation()      {}
func (Sequence) operation()  {}
func (Condition) operation() {}
func (Loop) operation()      {}
func (Scope) operation()     {}
func (Group) operation()     {}

func (Sequence) subgraph()  {}
func (Condition) subgraph() {}
func (Loop) subgraph()      {}
func (Scope) subgraph()     {}
func (Group) subgraph()     {}
