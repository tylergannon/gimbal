package environments

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func decodeExtra(data []byte, dest any, known ...string) (map[string]json.RawMessage, error) {
	if err := json.Unmarshal(data, dest); err != nil {
		return nil, err
	}
	var extra map[string]json.RawMessage
	if err := json.Unmarshal(data, &extra); err != nil {
		return nil, err
	}
	for _, k := range known {
		delete(extra, k)
	}
	return extra, nil
}
func encodeExtra(value any, extra map[string]json.RawMessage) ([]byte, error) {
	b, e := json.Marshal(value)
	if e != nil {
		return nil, e
	}
	var fields map[string]json.RawMessage
	if e = json.Unmarshal(b, &fields); e != nil {
		return nil, e
	}
	for k, v := range extra {
		if _, ok := fields[k]; !ok {
			fields[k] = v
		}
	}
	return json.Marshal(fields)
}
func (m *Machine) UnmarshalJSON(b []byte) error {
	type plain Machine
	var p plain
	extra, e := decodeExtra(b, &p, "id", "name", "kind", "ssh", "address", "route", "executable")
	*m = Machine(p)
	m.Extra = extra
	return e
}
func (m Machine) MarshalJSON() ([]byte, error) {
	type plain Machine
	return encodeExtra(plain(m), m.Extra)
}
func (v *Environment) UnmarshalJSON(b []byte) error {
	type plain Environment
	var p plain
	extra, e := decodeExtra(b, &p, "id", "project", "machine", "directory")
	*v = Environment(p)
	v.Extra = extra
	return e
}
func (v Environment) MarshalJSON() ([]byte, error) {
	type plain Environment
	return encodeExtra(plain(v), v.Extra)
}
func revision(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func readDocument(path string) (ConfigDocument, error) {
	b, e := os.ReadFile(path)
	if errors.Is(e, os.ErrNotExist) {
		return ConfigDocument{Path: path, Valid: true, Issues: []Issue{}}, nil
	}
	if e != nil {
		return ConfigDocument{}, e
	}
	d := ConfigDocument{Path: path, Text: string(b), Revision: revision(b), Valid: true, Issues: []Issue{}}
	var object map[string]json.RawMessage
	if e = json.Unmarshal(b, &object); e != nil || object == nil {
		d.Valid = false
		line := 1
		var syntax *json.SyntaxError
		if errors.As(e, &syntax) {
			line += bytes.Count(b[:min(int(syntax.Offset), len(b))], []byte("\n"))
		}
		msg := "configuration must be a JSON object"
		if e != nil {
			msg = e.Error()
		}
		d.Issues = append(d.Issues, Issue{Entry: "document", Index: -1, Line: line, Message: msg})
	}
	return d, nil
}
func (s *Service) refreshLocked() error {
	d, e := readDocument(s.path)
	if e != nil {
		return e
	}
	s.document = d
	if !d.Valid {
		return nil
	}
	cfg := Config{Version: 1, Machines: []Machine{}, Environments: []Environment{}}
	if d.Text == "" {
		s.config = cfg
		return nil
	}
	var raw map[string]json.RawMessage
	_ = json.Unmarshal([]byte(d.Text), &raw)
	if v := raw["version"]; v != nil {
		if e = json.Unmarshal(v, &cfg.Version); e != nil || cfg.Version != 1 {
			s.document.Issues = append(s.document.Issues, Issue{Entry: "version", Index: -1, Message: "unsupported configuration version; expected 1"})
			return nil
		}
	}
	var machines []json.RawMessage
	if b := raw["machines"]; b != nil {
		if e = json.Unmarshal(b, &machines); e != nil {
			s.document.Issues = append(s.document.Issues, Issue{Entry: "machines", Index: -1, Message: "machines must be an array"})
		}
	}
	seen := map[string]bool{Local: true}
	for i, b := range machines {
		var m Machine
		e = json.Unmarshal(b, &m)
		if e == nil {
			e = s.validateMachine(m)
		}
		if e == nil && seen[m.ID] {
			e = fmt.Errorf("duplicate machine ID %q", m.ID)
		}
		if e != nil {
			s.document.Issues = append(s.document.Issues, Issue{Entry: "machines", Index: i, Message: e.Error()})
			continue
		}
		seen[m.ID] = true
		cfg.Machines = append(cfg.Machines, m)
		if m.Kind == KindRoute && m.Route != nil {
			_, native := s.registry.Backend(m.Route.Name)
			_, messaging := s.registry.Messaging(m.Route.Name)
			if !native && !messaging {
				s.document.Issues = append(s.document.Issues, Issue{Entry: "machines", Index: i, Message: fmt.Sprintf("machine %s: install backend plugin %q built with this Gimbal source/toolchain in %s and restart Gimbal", m.ID, m.Route.Name, filepath.Join(s.options.InstanceDir, "plugins"))})
				s.states[m.ID] = MachineState{ID: m.ID, Name: m.Name, Kind: m.Kind, State: StateError, Detail: "Install compatible backend plugin " + m.Route.Name + " in " + filepath.Join(s.options.InstanceDir, "plugins") + " and restart Gimbal", Harnesses: []Harness{}, Issues: []Issue{}}
			}
		}
	}
	var envs []json.RawMessage
	if b := raw["environments"]; b != nil {
		if e = json.Unmarshal(b, &envs); e != nil {
			s.document.Issues = append(s.document.Issues, Issue{Entry: "environments", Index: -1, Message: "environments must be an array"})
		}
	}
	ids := map[string]bool{Local: true}
	for i, b := range envs {
		var v Environment
		e = json.Unmarshal(b, &v)
		if e == nil {
			if v.ID == "" || v.Project == "" || v.Directory == "" || !seen[v.Machine] {
				e = fmt.Errorf("environment needs a unique ID, project, directory and valid machine")
			}
			if ids[v.ID] {
				e = fmt.Errorf("duplicate or reserved environment ID %q", v.ID)
			}
		}
		if e != nil {
			s.document.Issues = append(s.document.Issues, Issue{Entry: "environments", Index: i, Message: e.Error()})
			continue
		}
		ids[v.ID] = true
		cfg.Environments = append(cfg.Environments, v)
	}
	s.config = cfg
	return nil
}
func (s *Service) validateMachine(m Machine) error {
	if m.ID == "" || m.ID == Local || strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("machine needs a nonreserved ID and name")
	}
	n := 0
	if m.SSH != nil {
		n++
	}
	if m.Address != "" {
		n++
	}
	if m.Route != nil {
		n++
	}
	if n != 1 {
		return fmt.Errorf("machine %q needs exactly one of ssh, address or route", m.ID)
	}
	switch m.Kind {
	case KindSSH:
		if m.SSH == nil || strings.TrimSpace(m.SSH.Host) == "" {
			return fmt.Errorf("SSH machine needs a host")
		}
		if strings.HasPrefix(m.SSH.Host, "-") || strings.ContainsAny(m.SSH.Host, "\r\n\x00") {
			return fmt.Errorf("invalid SSH host")
		}
		if m.SSH.Port < 0 || m.SSH.Port > 65535 || m.SSH.DaemonPort < 0 || m.SSH.DaemonPort > 65535 {
			return fmt.Errorf("invalid SSH port")
		}
	case KindAddress:
		if _, e := addressURL(m.Address); e != nil {
			return e
		}
	case KindRoute:
		if m.Route == nil {
			return fmt.Errorf("route machine needs a route")
		}
		if d, ok := s.registry.Backend(m.Route.Name); ok {
			if e := d.ValidateConfiguration(m.Route.Configuration); e != nil {
				return e
			}
		} else if d, ok := s.registry.Messaging(m.Route.Name); ok {
			if e := d.ValidateConfiguration(m.Route.Configuration); e != nil {
				return e
			}
		}
	default:
		return fmt.Errorf("unknown machine kind %q", m.Kind)
	}
	return nil
}
func (s *Service) writeLocked(expected, text string) error {
	d, e := readDocument(s.path)
	if e != nil {
		return e
	}
	if d.Revision != expected {
		return fmt.Errorf("environments.json changed; reload before saving")
	}
	if e = os.MkdirAll(filepath.Dir(s.path), 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(s.path), ".environments-*")
	if e != nil {
		return e
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if e = f.Chmod(0600); e == nil {
		_, e = f.WriteString(text)
	}
	if e == nil {
		e = f.Sync()
	}
	e = errors.Join(e, f.Close())
	if e != nil {
		return e
	}
	if e = os.Rename(f.Name(), s.path); e != nil {
		return e
	}
	return s.refreshLocked()
}
func (s *Service) editLocked(expected, section, id string, value any, remove bool) error {
	if e := s.refreshLocked(); e != nil {
		return e
	}
	if !s.document.Valid {
		return fmt.Errorf("repair malformed environments.json before editing entries")
	}
	if s.document.Revision != expected {
		return fmt.Errorf("environments.json changed; reload before saving")
	}
	raw := map[string]json.RawMessage{}
	if s.document.Text != "" {
		_ = json.Unmarshal([]byte(s.document.Text), &raw)
	}
	if raw["version"] == nil {
		raw["version"] = json.RawMessage("1")
	}
	var entries []json.RawMessage
	if raw[section] != nil {
		if e := json.Unmarshal(raw[section], &entries); e != nil {
			return fmt.Errorf("repair %s before editing entries", section)
		}
	}
	found := false
	out := []json.RawMessage{}
	for _, entry := range entries {
		var key struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(entry, &key)
		if key.ID != id {
			out = append(out, entry)
			continue
		}
		found = true
		if remove {
			continue
		}
		fresh, e := json.Marshal(value)
		if e != nil {
			return e
		}
		var oldFields, newFields map[string]json.RawMessage
		_ = json.Unmarshal(entry, &oldFields)
		_ = json.Unmarshal(fresh, &newFields)
		for k, v := range newFields {
			oldFields[k] = v
		}
		if section == "machines" {
			for _, k := range []string{"ssh", "address", "route", "executable"} {
				if _, ok := newFields[k]; !ok {
					delete(oldFields, k)
				}
			}
		}
		fresh, _ = json.Marshal(oldFields)
		out = append(out, fresh)
	}
	if !found && !remove {
		fresh, e := json.Marshal(value)
		if e != nil {
			return e
		}
		out = append(out, fresh)
	}
	raw[section], _ = json.Marshal(out)
	if raw["machines"] == nil {
		raw["machines"] = json.RawMessage("[]")
	}
	if raw["environments"] == nil {
		raw["environments"] = json.RawMessage("[]")
	}
	b, e := json.MarshalIndent(raw, "", "  ")
	if e != nil {
		return e
	}
	return s.writeLocked(expected, string(b)+"\n")
}
