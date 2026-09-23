# Source: Project Ownership, Path Aliasing, and Endpoint Isolation Tests

- **Origin**: `/Users/tyler/.codex/worktrees/d798/gimble/web/project_ownership_test.go`, `/Users/tyler/.codex/worktrees/d798/gimble/web/control_ownership_test.go`, `/Users/tyler/.codex/worktrees/d798/gimble/web/instance_test.go`
- **Commit**: `40dc82947eed99202fd9cb1dd377b6a3c2abbccc`
- **Retrieval Date**: 2026-09-23
- **Scope**: Behavioral tests proving canonical path resolution (symlinks, `..`), inter-process lock contention, run lifetime unwinding before lock release, and isolation of project-scoped observation and control endpoints.

---

### Symlink Path Aliasing and Cross-Process Locking (`web/project_ownership_test.go:83-127`)

```go
	alias := filepath.Join(base, "alias-a")
	if err := os.Symlink(projectA, alias); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{projectA, alias, filepath.Join(projectB, "..", "a")} {
		if _, err := other.AdmitProject(path); err == nil || !strings.Contains(err.Error(), "already owned by another instance") {
			t.Fatalf("admit owned project %s: %v", path, err)
		}
	}
```

### In-Process Path Aliasing Reusing Runtime (`web/instance_test.go:54-65`)

```go
	if again, err := i.AdmitProject(projectA); err != nil || again != a {
		t.Fatalf("readmit project: %p, %v", again, err)
	}
	alias := filepath.Join(base, "alias-a")
	if err := os.Symlink(projectA, alias); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{alias, filepath.Join(base, "b", "..", "a"), projectA} {
		if again, err := i.AdmitProject(path); err != nil || again != a {
			t.Fatalf("alias %s admitted separate project: %p, %v", path, again, err)
		}
	}
```

### Run Lifetime and Lock Release Coordination (`web/project_ownership_test.go:148-186`)

```go
	go func() {
		runDone <- p.Run(context.Background(), "slow-shutdown", nil, func(runCtx context.Context) error {
			close(started)
			<-runCtx.Done()
			close(unwinding)
			<-release
			return runCtx.Err()
		})
	}()
	<-started
	cancel()
	<-unwinding
	waitDone := make(chan struct{})
	go func() {
		owner.Wait()
		close(waitDone)
	}()
	select {
	case <-waitDone:
		t.Fatal("owner ended while its run was still unwinding")
	case <-time.After(50 * time.Millisecond):
	}
	otherCtx, otherCancel := context.WithCancel(t.Context())
	defer otherCancel()
	if _, err := NewInstance(otherCtx, filepath.Join(base, "other"), []string{project}, WithNoWeb()); err == nil || !strings.Contains(err.Error(), "already owned by another instance") {
		t.Fatalf("second owner admitted during shutdown: %v", err)
	}
	close(release)
```

### Multi-Project Run Isolation under Single PID (`web/instance_test.go:167-180`, `web/control_ownership_test.go:46-60`)

```go
	for _, test := range []struct {
		path, contains, absent string
		status                 int
	}{
		{"/projects/" + a.id, aID, bID, http.StatusOK},
		{"/projects/" + b.id, bID, aID, http.StatusOK},
		{"/projects/" + a.id + "/runs/" + aID, aID, bID, http.StatusOK},
		{"/projects/" + b.id + "/runs/" + aID, "", "", http.StatusNotFound},
		{"/projects/" + a.id + "/api/runs/" + aID, aID, bID, http.StatusOK},
		{"/projects/" + b.id + "/api/runs/" + aID, "", "", http.StatusNotFound},
		{"/projects/" + a.id + "/conversations/a", "Selected conversation a", "Selected conversation b", http.StatusOK},
		{"/projects/" + b.id + "/conversations/b", "Selected conversation b", "Selected conversation a", http.StatusOK},
		{"/projects/" + b.id + "/api/runs/" + aID + "/events", "", "", http.StatusNotFound},
	} {
		response, err := http.Get("http://" + i.address + test.path)
...
```

```go
	firstRuns, err := first.controlRuns()
	if err != nil {
		t.Fatal(err)
	}
	secondRuns, err := second.controlRuns()
	if err != nil {
		t.Fatal(err)
	}
	if len(firstRuns) != 1 || len(secondRuns) != 1 {
		t.Fatalf("each project must advertise only its own active run: first=%d second=%d", len(firstRuns), len(secondRuns))
	}
	if firstRuns[0].ID == secondRuns[0].ID {
		t.Fatal("different projects advertised the same run")
	}
```
