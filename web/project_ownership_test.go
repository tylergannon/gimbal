package web

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectOwnershipAcrossProcesses(t *testing.T) {
	if project := os.Getenv("GIMBLE_TEST_OWNED_PROJECT"); project != "" {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		instance, err := NewInstance(ctx, os.Getenv("GIMBLE_TEST_INSTANCE_DIR"), WithNoWeb())
		if err != nil {
			t.Fatal(err)
		}
		p, err := instance.AdmitProject(project)
		if err != nil {
			t.Fatal(err)
		}
		if err := p.Run(ctx, "ownership-history", nil, func(context.Context) error { return nil }); err != nil {
			t.Fatal(err)
		}
		entries, err := os.ReadDir(filepath.Join(p.dir, "runs"))
		if err != nil || len(entries) != 1 {
			t.Fatalf("owned run history: %v, %v", entries, err)
		}
		fmt.Println(entries[0].Name())
		_, _ = io.Copy(io.Discard, os.Stdin)
		cancel()
		<-instance.done
		return
	}

	base := t.TempDir()
	projectA := filepath.Join(base, "a")
	projectB := filepath.Join(base, "b")
	for _, project := range []string{projectA, projectB} {
		if err := os.Mkdir(project, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	owner := exec.Command(os.Args[0], "-test.run=^TestProjectOwnershipAcrossProcesses$")
	owner.Env = append(os.Environ(), "GIMBLE_TEST_OWNED_PROJECT="+projectA, "GIMBLE_TEST_INSTANCE_DIR="+filepath.Join(base, "owner"))
	stdout, err := owner.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := owner.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	owner.Stderr = &stderr
	if err := owner.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = stdin.Close()
		if owner.ProcessState == nil {
			_ = owner.Process.Kill()
			_ = owner.Wait()
		}
	})
	runID, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatalf("owner did not start: %v; stderr: %s", err, stderr.String())
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		t.Fatal("owner created no history")
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	other, err := NewInstance(ctx, filepath.Join(base, "other"), WithPort(0))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.AdmitProject(projectB); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(base, "alias-a")
	if err := os.Symlink(projectA, alias); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{projectA, alias, filepath.Join(projectB, "..", "a")} {
		if _, err := other.AdmitProject(path); err == nil || !strings.Contains(err.Error(), "already owned by another instance") {
			t.Fatalf("admit owned project %s: %v", path, err)
		}
	}
	response, err := http.Get("http://" + other.address + "/")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("independent instance stopped: %d", response.StatusCode)
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	if err := owner.Wait(); err != nil {
		t.Fatalf("owner shutdown: %v; stderr: %s", err, stderr.String())
	}
	reopened, err := other.AdmitProject(alias)
	if err != nil {
		t.Fatalf("reopen after owner shutdown: %v", err)
	}
	response, err = http.Get("http://" + other.address + "/projects/" + reopened.id + "/runs/" + runID)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("reopened run history status: %d", response.StatusCode)
	}
}
