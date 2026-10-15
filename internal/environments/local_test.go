package environments

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
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
	for _, h := range hs {
		if h.State == HarnessReady {
			t.Fatal("installation claimed authentication")
		}
		if h.ID == "claude" && h.Default != "claude-haiku-4-5" {
			t.Fatalf("Claude default became expensive: %s", h.Default)
		}
		if !slices.Contains(h.Models, h.Default) {
			t.Fatal("default missing from models")
		}
	}
}
