package environments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
)

type Service struct {
	mu          sync.Mutex
	sshMu       sync.Mutex
	operationMu sync.Mutex
	ctx         context.Context
	cancel      context.CancelFunc
	path        string
	registry    *gimbal.Registry
	options     Options
	config      Config
	document    ConfigDocument
	projects    map[string]string
	states      map[string]MachineState
	facts       map[string]EnvironmentRow
	recents     map[string]int64
	tunnels     map[string]*tunnel
	connections map[string]backend.Connection
	closed      bool
}

func New(ctx context.Context, o Options) (*Service, error) {
	if o.InstanceDir == "" {
		return nil, fmt.Errorf("environment instance directory required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("environment context required")
	}
	path := o.ConfigFile
	if path == "" {
		path = os.Getenv("GIMBAL_ENVIRONMENTS_FILE")
	}
	if path == "" {
		if o.InstanceDir == "" {
			return nil, fmt.Errorf("environment instance directory required")
		}
		path = filepath.Join(o.InstanceDir, "environments.json")
	}
	path, e := filepath.Abs(path)
	if e != nil {
		return nil, e
	}
	if o.Registry == nil {
		o.Registry = gimbal.DefaultRegistry
	}
	ctx, cancel := context.WithCancel(ctx)
	s := &Service{ctx: ctx, cancel: cancel, path: path, registry: o.Registry, options: o, projects: map[string]string{}, states: map[string]MachineState{}, facts: map[string]EnvironmentRow{}, recents: map[string]int64{}, tunnels: map[string]*tunnel{}, connections: map[string]backend.Connection{}}
	if b, e := os.ReadFile(filepath.Join(o.InstanceDir, "environment-recents.json")); e == nil {
		_ = json.Unmarshal(b, &s.recents)
	}
	if s.recents == nil {
		s.recents = map[string]int64{}
	}
	if e = s.refreshLocked(); e != nil {
		cancel()
		return nil, e
	}
	go s.watch()
	return s, nil
}
func (s *Service) watch() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			_ = s.refreshLocked()
			machines := slices.Clone(s.config.Machines)
			s.mu.Unlock()
			s.retryProbeCleanup()
			for _, m := range machines {
				if m.Kind == KindRoute {
					continue
				}
				ctx, c := context.WithTimeout(s.ctx, 3*time.Second)
				_ = s.probe(ctx, m, false)
				c()
			}
		}
	}
}
func (s *Service) RegisterProject(id, directory string) error {
	p, e := canonical(directory)
	if e != nil {
		return e
	}
	if id == "" {
		return fmt.Errorf("project ID required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.projects[id] = p
	return nil
}
func (s *Service) Config(ctx context.Context) (ConfigDocument, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.refreshLocked()
	return s.document, e
}
func (s *Service) SaveConfig(ctx context.Context, in ConfigEdit) (ConfigDocument, error) {
	var raw map[string]json.RawMessage
	if e := json.Unmarshal([]byte(in.Text), &raw); e != nil || raw == nil {
		return ConfigDocument{}, fmt.Errorf("configuration must be a valid JSON object")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.writeLocked(in.Revision, in.Text); e != nil {
		return ConfigDocument{}, e
	}
	return s.document, nil
}
func (s *Service) SaveMachine(ctx context.Context, in MachineEdit) (MachineState, error) {
	if e := s.validateMachine(in.Machine); e != nil {
		return MachineState{}, e
	}
	if !in.ConnectAnyway {
		if e := s.probe(ctx, in.Machine, true); e != nil {
			return s.machineState(in.Machine), e
		}
	}
	s.mu.Lock()
	e := s.editLocked(in.Revision, "machines", in.Machine.ID, in.Machine, false)
	s.mu.Unlock()
	if e != nil {
		return MachineState{}, e
	}
	return s.machineState(in.Machine), nil
}
func (s *Service) RemoveMachine(ctx context.Context, in MachineRemoval) error {
	if in.ID == Local {
		return fmt.Errorf("This Mac is built in")
	}
	if s.options.Sessions != nil {
		for _, u := range s.options.Sessions() {
			if u.Machine == in.ID && (u.Active || u.Pending) {
				return fmt.Errorf("machine is used by %s (%s); stop it and confirm cleanup before removing", u.Title, u.Conversation)
			}
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.editLocked(in.Revision, "machines", in.ID, nil, true); e != nil {
		return e
	}
	delete(s.states, in.ID)
	return nil
}
func (s *Service) RemoveEnvironment(ctx context.Context, in EnvironmentRemoval) error {
	if in.ID == Local {
		return fmt.Errorf("the admitted local directory is built in")
	}
	if s.options.Sessions != nil {
		for _, u := range s.options.Sessions() {
			if u.Environment == in.ID && (u.Active || u.Pending) {
				return fmt.Errorf("environment is used by %s (%s)", u.Title, u.Conversation)
			}
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.editLocked(in.Revision, "environments", in.ID, nil, true)
}
func (s *Service) SaveEnvironment(ctx context.Context, in EnvironmentEdit) (EnvironmentRow, error) {
	v := in.Environment
	if v.ID == "" || v.ID == Local || v.Project == "" || v.Directory == "" {
		return EnvironmentRow{}, fmt.Errorf("environment needs ID, project and directory")
	}
	s.mu.Lock()
	_ = s.refreshLocked()
	_, projectOK := s.projects[v.Project]
	m, ok := s.machineLocked(v.Machine)
	s.mu.Unlock()
	if !projectOK || !ok {
		return EnvironmentRow{}, fmt.Errorf("unknown project or machine")
	}
	target := Target{Environment: v, Machine: m, Directory: v.Directory}
	d, e := s.Inspect(ctx, target, v.Directory)
	if e != nil {
		return EnvironmentRow{}, e
	}
	if !d.Exists {
		return EnvironmentRow{}, fmt.Errorf("directory does not exist; clone explicitly before adding")
	}
	v.Directory = d.Path
	row := EnvironmentRow{Environment: v, Directory: d, Observed: time.Now().UnixMilli(), Capabilities: capabilities(m, d)}
	s.mu.Lock()
	e = s.editLocked(in.Revision, "environments", v.ID, v, false)
	if e == nil {
		s.facts[v.Project+"\x00"+v.ID] = row
	}
	s.mu.Unlock()
	return row, e
}
func (s *Service) machineLocked(id string) (Machine, bool) {
	if id == Local {
		return Machine{ID: Local, Name: "This Mac", Kind: "local", Executable: s.options.Executable}, true
	}
	for _, m := range s.config.Machines {
		if m.ID == id {
			return m, true
		}
	}
	return Machine{}, false
}
func (s *Service) machineState(m Machine) MachineState {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := s.states[m.ID]; ok {
		return v
	}
	state := StateUnreachable
	detail := "Not yet connected"
	if m.Kind == KindRoute {
		state = StateNotStarted
		detail = "Starts when needed"
	}
	return MachineState{ID: m.ID, Name: m.Name, Kind: m.Kind, State: state, Detail: detail, Since: time.Now().UnixMilli(), Harnesses: []Harness{}, Issues: []Issue{}}
}
func capabilities(m Machine, d Directory) Capabilities {
	return Capabilities{Steer: m.Kind != KindRoute || m.Route != nil && m.Route.Name == "http", Fork: m.Kind != KindRoute || m.Route != nil && m.Route.Name == "http", Worktree: d.Git && !d.Unborn && d.FixedWorkdir == "", Browse: true, Clone: d.FixedWorkdir == ""}
}
func (s *Service) Snapshot(ctx context.Context, project string) (Snapshot, error) {
	s.mu.Lock()
	e := s.refreshLocked()
	cfg := s.config
	doc := s.document
	recents := map[string]int64{}
	for k, v := range s.recents {
		recents[k] = v
	}
	projects := map[string]string{}
	for k, v := range s.projects {
		projects[k] = v
	}
	facts := map[string]EnvironmentRow{}
	for k, v := range s.facts {
		facts[k] = v
		facts[k] = withRecent(v, s.recents[k])
	}
	s.mu.Unlock()
	if e != nil {
		return Snapshot{}, e
	}
	out := Snapshot{Project: project, ConfigPath: s.path, Revision: doc.Revision, Issues: doc.Issues, Machines: []MachineState{}, Environments: []EnvironmentRow{}, Sessions: []SessionUsage{}}
	out.Issues = append(out.Issues, s.pendingIssues()...)
	build, hs := localFacts()
	out.Machines = append(out.Machines, MachineState{ID: Local, Name: "This Mac", Kind: "local", State: StateConnected, Build: build, Harnesses: hs, Capabilities: Capabilities{Steer: true, Fork: true, Browse: true, Clone: true}, Issues: []Issue{}})
	for _, m := range cfg.Machines {
		out.Machines = append(out.Machines, s.machineState(m))
	}
	if s.options.Sessions != nil {
		out.Sessions = s.options.Sessions()
	}
	for id, path := range projects {
		if project != "" && id != project {
			continue
		}
		d, e := inspectLocal(ctx, path, false)
		row := EnvironmentRow{LastUsed: recents[id+"\x00"+Local], Environment: Environment{ID: Local, Project: id, Machine: Local, Directory: path}, Directory: d, Observed: time.Now().UnixMilli(), Capabilities: capabilities(Machine{}, d), Issues: []Issue{}}
		if e != nil {
			row.Issues = append(row.Issues, Issue{Entry: Local, Index: -1, Message: e.Error()})
		}
		out.Environments = append(out.Environments, row)
	}
	for _, v := range cfg.Environments {
		if project != "" && v.Project != project {
			continue
		}
		row, ok := facts[v.Project+"\x00"+v.ID]
		if !ok || row.Environment.Directory != v.Directory || row.Environment.Machine != v.Machine {
			row = EnvironmentRow{Environment: v, Directory: Directory{Path: v.Directory, Bases: []string{}, Entries: []DirectoryEntry{}}, Issues: []Issue{}}
		}
		row.LastUsed = recents[v.Project+"\x00"+v.ID]
		out.Environments = append(out.Environments, row)
	}
	for i := range out.Environments {
		for _, u := range out.Sessions {
			if u.Project == out.Environments[i].Environment.Project && u.Environment == out.Environments[i].Environment.ID && (u.Active || u.Pending) {
				out.Environments[i].LiveSessions++
			}
		}
	}
	return out, nil
}
func (s *Service) Resolve(ctx context.Context, project, id string) (Target, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.refreshLocked(); e != nil {
		return Target{}, e
	}
	if id == "" || id == Local {
		path, ok := s.projects[project]
		if !ok {
			return Target{}, fmt.Errorf("unknown project")
		}
		m, _ := s.machineLocked(Local)
		return Target{Environment: Environment{ID: Local, Project: project, Machine: Local, Directory: path}, Machine: m, Directory: path, Capabilities: Capabilities{Steer: true, Fork: true, Browse: true, Clone: true}}, nil
	}
	for _, v := range s.config.Environments {
		if v.ID != id || v.Project != project {
			continue
		}
		m, ok := s.machineLocked(v.Machine)
		if !ok {
			return Target{}, fmt.Errorf("environment's machine is missing")
		}
		s.recents[project+"\x00"+id] = time.Now().UnixMilli()
		_ = writeReceipt(filepath.Join(s.options.InstanceDir, "environment-recents.json"), s.recents)
		route, e := routeFor(m)
		if e != nil {
			return Target{}, e
		}
		return Target{Environment: v, Machine: m, Directory: v.Directory, Route: route, Capabilities: capabilities(m, Directory{})}, nil
	}
	return Target{}, fmt.Errorf("environment is not available for this project")
}
func addressURL(address string) (string, error) {
	u, e := url.Parse(address)
	if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("enter an HTTP(S) daemon address without credentials, query or fragment")
	}
	return strings.TrimRight(address, "/"), nil
}
func routeFor(m Machine) (backend.Route, error) {
	if m.Kind == KindRoute {
		if m.Route == nil {
			return backend.Route{}, fmt.Errorf("missing route")
		}
		return *m.Route, nil
	}
	if m.Kind == KindAddress {
		endpoint, e := addressURL(m.Address)
		if e != nil {
			return backend.Route{}, e
		}
		raw, _ := json.Marshal(map[string]string{"endpoint": endpoint})
		return backend.Route{Name: "http", Configuration: raw, Label: m.Name}, nil
	}
	return backend.Route{}, nil
}
func (s *Service) Acquire(ctx context.Context, t Target, owner backend.Owner) (Binding, error) {
	return s.binding(ctx, t, owner, true)
}
func (s *Service) binding(ctx context.Context, t Target, owner backend.Owner, acquire bool) (Binding, error) {
	if acquire && t.Machine.ID != "" && t.Machine.ID != Local {
		s.mu.Lock()
		st := s.states[t.Machine.ID]
		if st.State != StateConnected {
			st.ID = t.Machine.ID
			st.Name = t.Machine.Name
			st.Kind = t.Machine.Kind
			st.State = StateConnecting
			st.Detail = "Connecting"
			if t.Machine.Route != nil && t.Machine.Route.Environment != "" {
				st.State = StateStarting
				st.Detail = "Starting environment"
			}
			st.Since = time.Now().UnixMilli()
			s.states[t.Machine.ID] = st
		}
		s.mu.Unlock()
	}
	if t.Machine.ID == Local || t.Machine.ID == "" {
		return Binding{Target: t, Owner: owner}, nil
	}
	route := t.Route
	if t.Machine.Kind == KindSSH {
		endpoint, e := s.sshEndpoint(ctx, t.Machine, acquire)
		if e != nil {
			return Binding{}, e
		}
		raw, _ := json.Marshal(map[string]string{"endpoint": endpoint})
		route = backend.Route{Name: "http", Configuration: raw, Label: t.Machine.Name}
	} else if route.Name == "" {
		var e error
		route, e = routeFor(t.Machine)
		if e != nil {
			return Binding{}, e
		}
	}
	keyRaw, _ := json.Marshal(route)
	key := string(keyRaw)
	s.mu.Lock()
	if c, ok := s.connections[key]; ok {
		s.mu.Unlock()
		t.Route = route
		b := Binding{Target: t, Owner: owner, Connection: c}
		if acquire && c.Orchestrator != nil {
			_, e := c.Orchestrator.AcquireEnvironment(ctx, owner, route.Environment)
			return b, e
		}
		return b, nil
	}
	s.mu.Unlock()
	c, e := s.registry.Connect(s.ctx, route)
	if e != nil {
		return Binding{}, e
	}
	s.mu.Lock()
	if existing, ok := s.connections[key]; ok {
		s.mu.Unlock()
		_ = c.Close()
		c = existing
	} else {
		s.connections[key] = c
		s.mu.Unlock()
	}
	t.Route = route
	b := Binding{Target: t, Owner: owner, Connection: c}
	if acquire && c.Orchestrator != nil {
		_, e = c.Orchestrator.AcquireEnvironment(ctx, owner, route.Environment)
	}
	return b, e
}
func (s *Service) Release(ctx context.Context, b Binding) error {
	if b.Connection.Orchestrator != nil {
		return b.Connection.Orchestrator.Release(ctx, backend.Scope{Run: b.Owner.Run, Scope: b.Owner.Scope})
	}
	return nil
}
func (s *Service) Reconnect(ctx context.Context, id string) (MachineState, error) {
	s.mu.Lock()
	_ = s.refreshLocked()
	m, ok := s.machineLocked(id)
	s.mu.Unlock()
	if !ok {
		return MachineState{}, fmt.Errorf("unknown machine")
	}
	e := s.probe(ctx, m, true)
	return s.machineState(m), e
}
func (s *Service) Close() error {
	s.cancel()
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	tunnels := s.tunnels
	connections := s.connections
	s.mu.Unlock()
	var es []error
	for _, t := range tunnels {
		es = append(es, t.close())
	}
	for _, c := range connections {
		es = append(es, c.Close())
	}
	return errors.Join(es...)
}

func withRecent(row EnvironmentRow, ms int64) EnvironmentRow { row.LastUsed = ms; return row }
