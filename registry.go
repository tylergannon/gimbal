package gimbal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"plugin"
	"slices"
	"strings"
	"sync"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/workflow"
)

// Invocation is a prepared ordinary Go workflow. Preparation starts no work.
type Invocation func(context.Context, Env) error

// WorkflowField describes one supported input and its visible presentation.
type WorkflowField struct {
	Name        string `json:"name"`
	Flag        string `json:"flag"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
	Optional    bool   `json:"optional"`
	Nullable    bool   `json:"nullable"`
}

// WorkflowDescription is the serializable catalog used by clients.
type WorkflowDescription struct {
	Name        string                  `json:"name"`
	Summary     string                  `json:"summary"`
	Help        string                  `json:"help"`
	InputSchema json.RawMessage         `json:"input_schema"`
	Fields      []WorkflowField         `json:"fields"`
	Roles       map[WorkflowRole]string `json:"roles"`
	SourcePage  json.RawMessage         `json:"source_page"`
}

// WorkflowDefinition combines a catalog entry with its plugin-owned code.
type WorkflowDefinition struct {
	WorkflowDescription
	Graph   workflow.Graph
	Prepare func(json.RawMessage) (Invocation, error)
}

// Registry admits workflow definitions before a host accepts work.
// Its zero value is usable; a failed insertion leaves the catalog unchanged.
type Registry struct {
	mu        sync.RWMutex
	workflows map[string]WorkflowDefinition
	backends  map[string]BackendDefinition
	source    string
}

var DefaultRegistry = &Registry{}

func (r *Registry) RegisterWorkflow(d WorkflowDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	fail := func(err error) error {
		if r.source != "" {
			return fmt.Errorf("plugin %s: %w", r.source, err)
		}
		return err
	}
	if strings.TrimSpace(d.Name) == "" || d.Prepare == nil || d.Graph.Name != d.Name {
		return fail(fmt.Errorf("malformed workflow definition %q", d.Name))
	}
	if _, ok := r.workflows[d.Name]; ok {
		return fail(fmt.Errorf("duplicate workflow %q", d.Name))
	}
	if _, err := compileInputSchema(d.InputSchema); err != nil {
		return fail(fmt.Errorf("workflow %s input schema: %w", d.Name, err))
	}
	seen := map[string]bool{}
	flags := map[string]bool{}
	for _, name := range []string{"project", "work-dir", "instance-dir", "input", "follow", "help", "conversation", "workflow-endpoint", "workflow-project", "workflow-route", "task-endpoint", "task-route"} {
		flags[name] = true
	}
	for _, role := range d.Graph.Roles() {
		if flags[role] {
			return fail(fmt.Errorf("workflow %s role flag %q is reserved", d.Name, role))
		}
		flags[role] = true
	}
	for _, f := range d.Fields {
		if f.Name == "" || f.Flag == "" || flags[f.Flag] || seen[f.Name] || (f.Kind != "string" && f.Kind != "int" && f.Kind != "bool") {
			return fail(fmt.Errorf("workflow %s malformed field %q", d.Name, f.Name))
		}
		seen[f.Name] = true
		flags[f.Flag] = true
	}
	roles := map[WorkflowRole]bool{}
	for _, role := range d.Graph.Roles() {
		roles[WorkflowRole(role)] = true
	}
	for role := range d.Roles {
		if !roles[role] {
			return fail(fmt.Errorf("workflow %s default names unknown role %q", d.Name, role))
		}
	}
	for _, role := range d.Graph.Roles() {
		if strings.TrimSpace(d.Roles[WorkflowRole(role)]) == "" {
			return fail(fmt.Errorf("workflow %s missing default for role %s", d.Name, role))
		}
	}
	if r.workflows == nil {
		r.workflows = map[string]WorkflowDefinition{}
	}
	r.workflows[d.Name] = d
	return nil
}

func (r *Registry) Workflow(name string) (WorkflowDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.workflows[name]
	return d, ok
}
func (r *Registry) Catalog() []WorkflowDescription {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]WorkflowDescription, 0, len(r.workflows))
	for _, d := range r.workflows {
		result = append(result, d.WorkflowDescription)
	}
	slices.SortFunc(result, func(a, b WorkflowDescription) int { return strings.Compare(a.Name, b.Name) })
	return result
}

// LoadPlugins loads sorted native plugin paths. A failure aborts startup.
// Plugins export Register(*gimbal.Registry) error and must match the host build.
func (r *Registry) LoadPlugins(dirs []string) error {
	var files []string
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("plugins %s: %w", dir, err)
		}
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".so" {
				file, err := filepath.Abs(filepath.Join(dir, entry.Name()))
				if err != nil {
					return err
				}
				files = append(files, file)
			}
		}
	}
	slices.Sort(files)
	for _, file := range files {
		p, err := plugin.Open(file)
		if err != nil {
			return fmt.Errorf("plugin %s: %w", file, err)
		}
		symbol, err := p.Lookup("Register")
		if err != nil {
			return fmt.Errorf("plugin %s: %w", file, err)
		}
		register, ok := symbol.(func(*Registry) error)
		if !ok {
			return fmt.Errorf("plugin %s: Register must be func(*gimbal.Registry) error", file)
		}
		r.source = file
		err = register(r)
		r.source = ""
		if err != nil {
			return fmt.Errorf("plugin %s: %w", file, err)
		}
	}
	return nil
}

func compileInputSchema(raw json.RawMessage) (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	if err = c.AddResource("input.json", doc); err != nil {
		return nil, err
	}
	return c.Compile("input.json")
}

// InputError names the exact catalog field whose submitted value is invalid.
type InputError struct {
	Field   string
	Message string
}

func (e *InputError) Error() string {
	if e.Field == "" {
		return e.Message
	}
	return e.Field + ": " + e.Message
}

// ValidateWorkflowInput validates raw schema constraints before concrete decode,
// then preserves the established nonblank-string/positive-integer semantics.
func ValidateWorkflowInput(d WorkflowDescription, raw json.RawMessage) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return &InputError{Message: "input must be a JSON object"}
	}
	schema, err := compileInputSchema(d.InputSchema)
	if err != nil {
		return err
	}
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	if err = schema.Validate(value); err != nil {
		field := ""
		if validation, ok := errors.AsType[*jsonschema.ValidationError](err); ok {
			var locate func(*jsonschema.ValidationError) string
			locate = func(v *jsonschema.ValidationError) string {
				switch k := v.ErrorKind.(type) {
				case *kind.Required:
					if len(k.Missing) > 0 {
						return k.Missing[0]
					}
				case *kind.AdditionalProperties:
					if len(k.Properties) > 0 {
						return k.Properties[0]
					}
				}
				if len(v.InstanceLocation) > 0 {
					return v.InstanceLocation[0]
				}
				for _, cause := range v.Causes {
					if field := locate(cause); field != "" {
						return field
					}
				}
				return ""
			}
			field = locate(validation)
		}
		return &InputError{Field: field, Message: err.Error()}
	}
	declared := map[string]WorkflowField{}
	for _, f := range d.Fields {
		declared[f.Name] = f
		value, present := fields[f.Name]
		if !present {
			if !f.Optional {
				return &InputError{Field: f.Name, Message: "required"}
			}
			continue
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			if f.Nullable {
				continue
			}
			return &InputError{Field: f.Name, Message: "null is not allowed"}
		}
		switch f.Kind {
		case "string":
			var s string
			if json.Unmarshal(value, &s) != nil {
				return &InputError{Field: f.Name, Message: "give a string"}
			}
			if !f.Optional && strings.TrimSpace(s) == "" {
				return &InputError{Field: f.Name, Message: "give a nonblank string"}
			}
		case "int":
			var n int
			if json.Unmarshal(value, &n) != nil {
				return &InputError{Field: f.Name, Message: "give an integer"}
			}
			if !f.Optional && n <= 0 {
				return &InputError{Field: f.Name, Message: "give a positive integer"}
			}
		case "bool":
			var b bool
			if json.Unmarshal(value, &b) != nil {
				return &InputError{Field: f.Name, Message: "give true or false"}
			}
		}
	}
	for name := range fields {
		if _, ok := declared[name]; !ok {
			return &InputError{Field: name, Message: "unknown field"}
		}
	}

	return nil
}

// BackendDefinition supplies a validated configuration and independent channels.
type BackendDefinition struct {
	Name                  string
	ConfigurationSchema   json.RawMessage
	ValidateConfiguration func(json.RawMessage) error
	Connect               func(context.Context, json.RawMessage) (backend.Connection, error)
}

func (r *Registry) RegisterBackend(d BackendDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	prefix := func(err error) error {
		if r.source != "" {
			return fmt.Errorf("plugin %s: %w", r.source, err)
		}
		return err
	}
	if strings.TrimSpace(d.Name) == "" || d.Connect == nil || d.ValidateConfiguration == nil {
		return prefix(fmt.Errorf("malformed backend definition %q", d.Name))
	}
	if _, ok := r.backends[d.Name]; ok {
		return prefix(fmt.Errorf("duplicate backend %q", d.Name))
	}
	if _, err := compileInputSchema(d.ConfigurationSchema); err != nil {
		return prefix(fmt.Errorf("backend %s configuration schema: %w", d.Name, err))
	}
	if r.backends == nil {
		r.backends = map[string]BackendDefinition{}
	}
	r.backends[d.Name] = d
	return nil
}
func (r *Registry) Backend(name string) (BackendDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.backends[name]
	return d, ok
}
func (r *Registry) Connect(ctx context.Context, route backend.Route) (backend.Connection, error) {
	d, ok := r.Backend(route.Name)
	if !ok {
		return backend.Connection{}, fmt.Errorf("unknown backend %q", route.Name)
	}
	raw := route.Configuration
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	schema, err := compileInputSchema(d.ConfigurationSchema)
	if err != nil {
		return backend.Connection{}, err
	}
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return backend.Connection{}, err
	}
	if err := schema.Validate(value); err != nil {
		return backend.Connection{}, fmt.Errorf("backend %s configuration: %w", route.Name, err)
	}
	if err := d.ValidateConfiguration(raw); err != nil {
		return backend.Connection{}, err
	}
	return d.Connect(ctx, raw)
}
