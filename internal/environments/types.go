// Package environments owns desktop machine configuration and project-directory
// placement. Execution and native resource ownership stay in backend/taskdaemon.
package environments

import (
	"encoding/json"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
)

const (
	ConfigVersion         = 1
	Local                 = "local"
	KindSSH               = "ssh"
	KindAddress           = "address"
	KindRoute             = "route"
	StateConnecting       = "connecting"
	StateStarting         = "starting"
	StateConnected        = "connected"
	StateUnreachable      = "unreachable"
	StateError            = "error"
	StateNotStarted       = "not-started"
	HarnessReady          = "ready"
	HarnessSignInRequired = "sign-in-required"
	HarnessMissing        = "missing"
	HarnessUnknown        = "unknown"
)

// Config is the single editable source for Machines and project directories.
// Local is implicit and cannot be replaced by a machine entry.
type Config struct {
	Version      int           `json:"version"`
	Machines     []Machine     `json:"machines"`
	Environments []Environment `json:"environments"`
}

// Machine selects exactly one transport. Route is the current backend.Route,
// including opaque Configuration/Backends; it is not the removed start API.
// Extra retains unknown entry properties for UI edits of existing entries.
type Machine struct {
	ID         string                     `json:"id"`
	Name       string                     `json:"name"`
	Kind       string                     `json:"kind"`
	SSH        *SSH                       `json:"ssh,omitempty"`
	Address    string                     `json:"address,omitempty"`
	Route      *backend.Route             `json:"route,omitempty"`
	Executable string                     `json:"executable,omitempty"`
	Extra      map[string]json.RawMessage `json:"-"`
}

type SSH struct {
	Host          string `json:"host"`
	Port          int    `json:"port,omitempty"`
	DaemonPort    int    `json:"daemon_port,omitempty"`
	WorkspaceRoot string `json:"workspace_root,omitempty"`
}

// Environment.Directory is interpreted on Machine, never on the desktop when
// Machine is remote. Project is the stable local host project ID.
type Environment struct {
	ID        string                     `json:"id"`
	Project   string                     `json:"project"`
	Machine   string                     `json:"machine"`
	Directory string                     `json:"directory"`
	Extra     map[string]json.RawMessage `json:"-"`
}

// Issue.Index is zero-based within machines/environments; -1 means the document.
type Issue struct {
	Entry   string `json:"entry"`
	Index   int    `json:"index"`
	Line    int    `json:"line,omitempty"`
	Message string `json:"message"`
}

type Harness struct {
	ID      string   `json:"id"`
	Default string   `json:"default"`
	Models  []string `json:"models"`
	State   string   `json:"state"`
	Detail  string   `json:"detail"`
}

type DirectoryEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// Directory distinguishes filesystem/Git facts from the cwd actually used by
// a task daemon. FixedWorkdir must never silently override a chosen checkout.
type Directory struct {
	Path               string           `json:"path"`
	Exists             bool             `json:"exists"`
	Empty              bool             `json:"empty"`
	Git                bool             `json:"git"`
	Unborn             bool             `json:"unborn"`
	Branch             string           `json:"branch"`
	Base               string           `json:"base"`
	Bases              []string         `json:"bases"`
	Origin             string           `json:"origin"`
	ExecutionDirectory string           `json:"execution_directory"`
	FixedWorkdir       string           `json:"fixed_workdir"`
	Entries            []DirectoryEntry `json:"entries"`
}

type Capabilities struct {
	Steer    bool `json:"steer"`
	Fork     bool `json:"fork"`
	Worktree bool `json:"worktree"`
	Browse   bool `json:"browse"`
	Clone    bool `json:"clone"`
}

// SessionUsage is supplied by the host, not discovered by scanning OS processes.
// Pending includes owned work with an unconfirmed outcome or cleanup.
type SessionUsage struct {
	Project      string `json:"project"`
	Conversation string `json:"conversation"`
	Title        string `json:"title"`
	Machine      string `json:"machine"`
	Environment  string `json:"environment"`
	Directory    string `json:"directory"`
	Status       string `json:"status"`
	Active       bool   `json:"active"`
	Pending      bool   `json:"pending"`
}

// Since and Observed are Unix milliseconds. Empty Build is unknown, not a match.
type MachineState struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Kind         string       `json:"kind"`
	Detail       string       `json:"detail"`
	State        string       `json:"state"`
	Since        int64        `json:"since"`
	LatencyMS    int64        `json:"latency_ms"`
	Build        string       `json:"build"`
	Harnesses    []Harness    `json:"harnesses"`
	Capabilities Capabilities `json:"capabilities"`
	Issues       []Issue      `json:"issues"`
}

type EnvironmentRow struct {
	Environment  Environment  `json:"environment"`
	Directory    Directory    `json:"directory"`
	Capabilities Capabilities `json:"capabilities"`
	Observed     int64        `json:"observed"`
	LastUsed     int64        `json:"last_used"`
	LiveSessions int          `json:"live_sessions"`
	Issues       []Issue      `json:"issues"`
}

type Snapshot struct {
	Project      string           `json:"project"`
	ConfigPath   string           `json:"config_path"`
	Revision     string           `json:"revision"`
	Machines     []MachineState   `json:"machines"`
	Environments []EnvironmentRow `json:"environments"`
	Sessions     []SessionUsage   `json:"sessions"`
	Issues       []Issue          `json:"issues"`
}

// Target is a durable snapshot, independent of the current configuration file.
// Acquire reconstructs SSH forwarding from Machine.SSH, never from a saved
// transient localhost port. Route.Environment is placement, not a display name.
type Target struct {
	Environment  Environment   `json:"environment"`
	Machine      Machine       `json:"machine"`
	Directory    string        `json:"directory"`
	Route        backend.Route `json:"route"`
	Capabilities Capabilities  `json:"capabilities"`
}

type Checkout struct {
	SharedCheckout bool   `json:"shared_checkout"`
	Branch         string `json:"branch"`
	Base           string `json:"base"`
}

type Prepared struct {
	Directory  string `json:"directory"`
	Branch     string `json:"branch"`
	BaseCommit string `json:"base_commit"`
	Target     Target `json:"target"`
}

// Binding is a transient connection to an immutable target under one owner.
// Connections are not persisted or sent to the web. Release is resource release
// after confirmed native scope cleanup; closing a handle alone is not cleanup.
type Binding struct {
	Target     Target
	Owner      backend.Owner
	Connection backend.Connection
}

// ConfigDocument.Revision is a hash of the exact current file bytes; empty means
// missing. Valid describes JSON syntax, not whether every entry is usable.
// Malformed hand edits stay visible while Snapshot retains last-known-good rows.
type ConfigDocument struct {
	Path     string  `json:"path"`
	Revision string  `json:"revision"`
	Text     string  `json:"text"`
	Valid    bool    `json:"valid"`
	Issues   []Issue `json:"issues"`
}

type ConfigEdit struct {
	Revision string `json:"revision"`
	Text     string `json:"text"`
}

type MachineEdit struct {
	Revision      string  `json:"revision"`
	Machine       Machine `json:"machine"`
	ConnectAnyway bool    `json:"connect_anyway"`
}

type MachineRemoval struct {
	Revision string `json:"revision"`
	ID       string `json:"id"`
}

type EnvironmentEdit struct {
	Revision    string      `json:"revision"`
	Environment Environment `json:"environment"`
}

type EnvironmentRemoval struct {
	Revision string `json:"revision"`
	ID       string `json:"id"`
}

// Options is supplied once by the host. Registry defaults to DefaultRegistry;
// Executable is this desktop's own Gimbal binary for local operations. ConfigFile
// overrides GIMBAL_ENVIRONMENTS_FILE, then <InstanceDir>/environments.json.
// Sessions must return a copy without holding host locks across service calls.
type Options struct {
	InstanceDir string
	ConfigFile  string
	Executable  string
	Registry    *gimbal.Registry
	Sessions    func() []SessionUsage
}

// Service implementation belongs in service.go. Its agreed concrete API is:
//
//	New(context.Context, Options) (*Service, error)
//	(*Service).RegisterProject(id, directory string) error
//	(*Service).Snapshot(context.Context, project string) (Snapshot, error)
//	(*Service).Config(context.Context) (ConfigDocument, error)
//	(*Service).SaveConfig(context.Context, ConfigEdit) (ConfigDocument, error)
//	(*Service).SaveMachine(context.Context, MachineEdit) (MachineState, error)
//	(*Service).RemoveMachine(context.Context, MachineRemoval) error
//	(*Service).SaveEnvironment(context.Context, EnvironmentEdit) (EnvironmentRow, error)
//	(*Service).RemoveEnvironment(context.Context, EnvironmentRemoval) error
//	(*Service).Resolve(context.Context, project, environment string) (Target, error)
//	(*Service).Acquire(context.Context, Target, backend.Owner) (Binding, error)
//	(*Service).Release(context.Context, Binding) error
//	(*Service).Inspect(context.Context, Target, path string) (Directory, error)
//	(*Service).ListDirectories(context.Context, Target, path string) (Directory, error)
//	(*Service).Prepare(context.Context, Target, Checkout, conversationID string) (Prepared, error)
//	(*Service).Clone(context.Context, Target, origin, path, operationID string) (Directory, error)
//	(*Service).Reconnect(context.Context, machine string) (MachineState, error)
//	(*Service).Close() error
//
// RegisterProject supplies the canonical admitted local path without writing a
// config entry. Snapshot("") includes all projects for Machines; a project ID
// filters environments. Local is always the implicit machine/environment ID for
// the selected project. Sessions and LiveSessions are composed from Options.
// Sessions, not the config layer, remain authoritative for native lifecycle.
//
// Resolve consults current configuration and verifies project membership. Acquire
// instead resolves the supplied saved Target without requiring its entry to
// still exist, reconnects SSH as needed, and returns its backend connection. It
// must not acquire an orchestrated resource merely to connect. Admission through
// effects.Submit acquires that resource using the same Owner. A probe uses a
// distinct owner, cleans its native scope, then releases it; active conversation
// ownership is never borrowed for setup. Release calls only the bound route's
// orchestrator release after the caller confirms native cleanup. Failure keeps
// the caller's persisted pending ownership, and Close never releases active
// conversation resources or stops a remote daemon.
//
// All writes compare the submitted Revision to freshly read file bytes. Conflicts
// return an actionable error without changing the file. SaveConfig preserves the
// user's text when syntactically valid; per-entry errors do not reject valid
// siblings. Structured entry edits preserve unknown raw properties. Malformed
// documents cannot be rewritten by an entry edit. RemoveMachine refuses Active
// or Pending Sessions with named identities, and never implicitly cancels them.
//
// Inspect/ListDirectories are reads, although a stopped route may require a
// bounded probe owner. Prepare/Clone persist their supplied operation identity
// before a remote mutation and reconcile it after a lost reply; they never
// repeat a mutation as fresh work. Prepare uses the environment's directory,
// returns the actual execution path/branch and refuses fixed-workdir mismatch
// before harness admission. Inspection never treats executable existence as
// authenticated harness readiness. No operation returns provider credentials.
//
