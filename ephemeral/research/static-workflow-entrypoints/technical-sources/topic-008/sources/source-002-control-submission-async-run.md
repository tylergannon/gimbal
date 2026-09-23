# Source: web/control.go - Asynchronous Run Decoupling and Project Control Isolation

- **Origin**: `/Users/tyler/.codex/worktrees/d798/gimble/web/control.go`
- **Commit**: `40dc82947eed99202fd9cb1dd377b6a3c2abbccc`
- **Retrieval Date**: 2026-09-23
- **Scope**: Submission goroutine spawning, `conversationRunStartedKey` synchronization, detached execution, and project-scoped run listing (`controlRuns`).

---

### Run Goroutine Decoupling and ID Synchronization (`web/control.go:119-158`)

```go
	started := make(chan string, 1)
	ctx := context.WithValue(p.ctx, conversationRunStartedKey{}, func(id string) { started <- id })
	done := make(chan error, 1)
	go func() {
		done <- p.Run(ctx, request.Name, models, func(ctx context.Context) error {
			return entry(ctx, gimble.Env{WorkDir: request.WorkDir}, request.Params)
		})
	}()
	var id string
	finished := false
	select {
	case id = <-started:
	case err := <-done:
		finished = true
		select {
		case id = <-started:
		default:
		}
		if id == "" {
			http.Error(w, fmt.Sprintf("run could not start: %v", err), http.StatusInternalServerError)
			return
		}
	case <-p.ctx.Done():
		http.Error(w, "instance stopped", http.StatusServiceUnavailable)
		return
	}
	if request.Conversation != "" {
		if err := p.conversations.Associate(request.Conversation, id, request.Name, request.WorkDir); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if finished {
			p.conversations.RefreshRun(id)
		} else {
			go func() { <-done; p.conversations.RefreshRun(id) }()
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(Admission{ID: id})
```

### Project-Scoped Run Filtering (`web/control.go:335-361`)

```go
func (r *Runtime) controlRuns() ([]observation.RunRow, error) {
	entries, err := os.ReadDir(filepath.Join(r.dir, "runs"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []observation.RunRow{}, nil
		}
		return nil, err
	}
	rows := make([]observation.RunRow, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := r.runs.InProgress(entry.Name()); err != nil {
			continue
		}
		snapshot, err := r.registry.Snapshot(entry.Name())
		if err != nil {
			continue
		}
		rows = append(rows, snapshot.Run)
	}
	slices.SortFunc(rows, func(a, b observation.RunRow) int {
		return strings.Compare(a.ID, b.ID)
	})
	return rows, nil
}
```
