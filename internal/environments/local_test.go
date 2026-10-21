package environments

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/internal/modelalias"
)

func repository(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	ctx := context.Background()
	for _, args := range [][]string{{"init", "-b", "main"}, {"config", "user.email", "test@example.invalid"}, {"config", "user.name", "Test"}, {"commit", "--allow-empty", "-m", "initial"}} {
		if _, e := git(ctx, dir, args...); e != nil {
			t.Fatal(e)
		}
	}
	p, e := canonical(dir)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func TestDirectoryWorktreeChoiceAndPreparationIdentity(t *testing.T) {
	ctx := context.Background()
	dir := repository(t)
	if e := os.Mkdir(filepath.Join(dir, "nested"), 0700); e != nil {
		t.Fatal(e)
	}
	nested, e := inspectLocal(ctx, filepath.Join(dir, "nested"), false)
	if e != nil || nested.Git {
		t.Fatalf("nested folder falsely became admitted repo: %+v %v", nested, e)
	}
	d, e := inspectLocal(ctx, dir, true)
	if e != nil || !d.Git || d.Branch != "main" || !slices.Contains(d.Bases, "main") {
		t.Fatalf("repository facts: %+v %v", d, e)
	}
	shared, e := prepareLocal(ctx, dir, Checkout{SharedCheckout: true}, "shared")
	if e != nil || shared.Directory != dir {
		t.Fatalf("shared checkout: %+v %v", shared, e)
	}
	choice := Checkout{Branch: "feature/one", Base: "main"}
	p, e := prepareLocal(ctx, dir, choice, "identity123")
	if e != nil {
		t.Fatal(e)
	}
	again, e := prepareLocal(ctx, dir, choice, "identity123")
	if e != nil || again.Directory != p.Directory || again.BaseCommit != p.BaseCommit {
		t.Fatalf("retry created another checkout: %+v %v", again, e)
	}
	if _, e = prepareLocal(ctx, dir, Checkout{Branch: "feature/two", Base: "main"}, "identity123"); e == nil {
		t.Fatal("same operation admitted another branch")
	}
}
func TestFixedWorkdirAndUnbornAreHonest(t *testing.T) {
	ctx := context.Background()
	dir := repository(t)
	t.Setenv(FixedWorkdirEnv, dir)
	d, e := inspectLocal(ctx, dir, false)
	if e != nil || d.FixedWorkdir != dir || d.ExecutionDirectory != dir {
		t.Fatal(d, e)
	}
	if _, e = prepareLocal(ctx, dir, Checkout{}, "id123"); e == nil {
		t.Fatal("isolation accepted on fixed-workdir daemon")
	}
	other := t.TempDir()
	if _, e = prepareLocal(ctx, other, Checkout{SharedCheckout: true}, "id123"); e == nil {
		t.Fatal("wrong directory accepted on fixed-workdir daemon")
	}
	t.Setenv(FixedWorkdirEnv, "")
	unborn := t.TempDir()
	if _, e = git(ctx, unborn, "init"); e != nil {
		t.Fatal(e)
	}
	d, e = inspectLocal(ctx, unborn, false)
	if e != nil || !d.Git || !d.Unborn {
		t.Fatal(d, e)
	}
}
func TestExplicitCloneReconcilesAndRejectsNewMutation(t *testing.T) {
	ctx := context.Background()
	origin := repository(t)
	dest := filepath.Join(t.TempDir(), "checkout")
	d, e := cloneLocal(ctx, dest, origin, "clone123")
	if e != nil || !d.Git {
		t.Fatal(d, e)
	}
	if _, e = cloneLocal(ctx, dest, origin, "clone123"); e != nil {
		t.Fatal("same identity did not reconcile", e)
	}
	if _, e = cloneLocal(ctx, dest, t.TempDir(), "clone123"); e == nil {
		t.Fatal("identity reused for another origin")
	}
	if _, e = cloneLocal(ctx, dest, origin, "different"); e == nil {
		t.Fatal("nonempty clone accepted")
	}
}
func TestHarnessMetadataUsesCheapDefaultsAndNeverInventsAuthentication(t *testing.T) {
	_, hs := localFacts()
	claudeDefault, err := modelalias.Resolve(modelalias.Selection{Name: "haiku"})
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range hs {
		if h.State == HarnessReady {
			t.Fatal("installation claimed authentication")
		}
		if h.ID == "claude" && h.Default != claudeDefault.Model {
			t.Fatalf("Claude default = %q, want resolved haiku alias %q", h.Default, claudeDefault.Model)
		}
		if !slices.Contains(h.Models, h.Default) {
			t.Fatal("default missing from models")
		}
	}
}

func TestInProcessPiIsNotMissingWithoutAnExecutable(t *testing.T) {
	absent := func(string) (string, error) { return "", exec.ErrNotFound }
	_, hs := localFactsFor(absent)
	states := map[string]Harness{}
	for _, h := range hs {
		states[h.ID] = h
	}
	for _, id := range []string{"claude", "codex", "agy", "opencode"} {
		if states[id].State != HarnessMissing {
			t.Fatalf("%s without an executable = %q, want missing", id, states[id].State)
		}
	}
	pi, ok := states["pi"]
	if !ok {
		t.Fatal("pi harness not reported")
	}
	if pi.State != HarnessUnknown {
		t.Fatalf("pi state = %q, want unknown: in-process adapter needs no executable and its sign-in is unverified", pi.State)
	}
	if strings.Contains(pi.Detail, "Not installed") {
		t.Fatalf("pi detail claims missing executable: %q", pi.Detail)
	}
}

func TestMissingExternalHarnessIsReportedAndPiOnlyWarnsAboutSignIn(t *testing.T) {
	present := func(id string) (string, error) {
		if id == "claude" {
			return "/bin/claude", nil
		}
		return "", exec.ErrNotFound
	}
	_, hs := localFactsFor(present)
	for _, h := range hs {
		switch h.ID {
		case "claude", "pi":
			if h.State != HarnessUnknown {
				t.Fatalf("%s = %q, want unknown", h.ID, h.State)
			}
		default:
			if h.State != HarnessMissing {
				t.Fatalf("%s = %q, want missing", h.ID, h.State)
			}
		}
	}
}

func TestInspectionWithNoExecutablesOnPathStillOffersPi(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	out, e := RunHelper(context.Background(), HelperRequest{Operation: "inspect", Path: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	states := map[string]string{}
	for _, h := range out.Harnesses {
		states[h.ID] = h.State
	}
	if states["pi"] != HarnessUnknown {
		t.Fatalf("pi = %q with an empty PATH, want unknown", states["pi"])
	}
	for _, id := range []string{"claude", "codex", "agy", "opencode"} {
		if states[id] != HarnessMissing {
			t.Fatalf("%s = %q with an empty PATH, want missing", id, states[id])
		}
	}
}
