package host

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tylergannon/polytype"
)

// WindowState is where the desktop window last was: its project, the view it
// showed, and the tabs it had open. The instance keeps it so the window
// reopens there, checked against the projects this instance actually owns.
type WindowState struct {
	Project string                  `json:"project"`
	Path    string                  `json:"path"`
	Tabs    []WindowTab             `json:"tabs"`
	Sidebar polytype.Nullable[bool] `json:"sidebar"`
	Seen    []SeenReply             `json:"seen"`
}

type SeenReply struct {
	Path    string `json:"path"`
	Request string `json:"request"`
}

type WindowTab struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Kind  string `json:"kind"`
}

// Window returns the remembered window when its project is still admitted,
// otherwise the first project in the catalog's order, otherwise no project.
func (o *Owner) Window() WindowState {
	o.mu.RLock()
	defer o.mu.RUnlock()
	var saved WindowState
	if o.dir != "" {
		if raw, err := os.ReadFile(filepath.Join(o.dir, "window.json")); err == nil {
			_ = json.Unmarshal(raw, &saved)
		}
	}
	for _, p := range o.projects {
		if p.id == saved.Project {
			return saved.within()
		}
	}
	if len(o.order) == 0 {
		return WindowState{Tabs: []WindowTab{}}
	}
	return WindowState{Project: o.projects[o.order[0]].id}.within()
}

// RememberWindow records the window's place for an admitted project.
func (o *Owner) RememberWindow(w WindowState) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	admitted := false
	for _, p := range o.projects {
		admitted = admitted || p.id == w.Project
	}
	if !admitted {
		return errors.New("gimbal: project is not admitted")
	}
	if o.dir == "" {
		return nil
	}
	var previous WindowState
	if raw, err := os.ReadFile(filepath.Join(o.dir, "window.json")); err == nil {
		_ = json.Unmarshal(raw, &previous)
	}
	seen := map[string]SeenReply{}
	if !w.Sidebar.Present {
		w.Sidebar = previous.Sidebar
	}
	entries := append([]SeenReply{}, previous.Seen...)
	for _, entry := range w.Seen {
		if strings.HasPrefix(entry.Path, "/projects/"+w.Project+"/conversations/") {
			entries = append(entries, entry)
		}
	}
	for _, entry := range entries {
		for _, p := range o.projects {
			if strings.HasPrefix(entry.Path, "/projects/"+p.id+"/conversations/") {
				seen[entry.Path] = entry
				break
			}
		}
	}
	w.Seen = nil
	for _, entry := range seen {
		w.Seen = append(w.Seen, entry)
	}
	slices.SortFunc(w.Seen, func(a, b SeenReply) int { return strings.Compare(a.Path, b.Path) })
	raw, err := json.Marshal(w.within())
	if err != nil {
		return err
	}
	temporary := filepath.Join(o.dir, "window.json.tmp")
	if err := os.WriteFile(temporary, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, filepath.Join(o.dir, "window.json"))
}

// within drops any path or tab outside the window's own project.
func (w WindowState) within() WindowState {
	root := "/projects/" + w.Project
	inside := func(path string) bool { return strings.HasPrefix(path, root+"/") || path == root }
	out := WindowState{Project: w.Project, Path: root + "/recordings", Tabs: []WindowTab{}, Sidebar: w.Sidebar}
	if w.Seen != nil {
		out.Seen = []SeenReply{}
		for _, seen := range w.Seen {
			parts := strings.Split(strings.Trim(seen.Path, "/"), "/")
			if len(parts) == 4 && parts[0] == "projects" && parts[1] != "" && parts[2] == "conversations" && parts[3] != "" {
				out.Seen = append(out.Seen, seen)
			}
		}
	}
	if inside(w.Path) {
		out.Path = w.Path
	}
	for _, tab := range w.Tabs {
		if inside(tab.ID) && !slices.ContainsFunc(out.Tabs, func(t WindowTab) bool { return t.ID == tab.ID }) {
			out.Tabs = append(out.Tabs, tab)
		}
	}
	return out
}
