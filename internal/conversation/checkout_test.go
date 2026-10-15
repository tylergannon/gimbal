package conversation

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal"
)

func TestSessionCheckoutChoiceUsesSelectedBaseAndPreservesOriginal(t *testing.T) {
	repo := newRepository(t)
	initial := git(t, repo, "rev-parse", "HEAD")
	git(t, repo, "branch", "release-base")
	git(t, repo, "update-ref", "refs/remotes/origin/release-base", initial)
	git(t, repo, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/release-base")
	git(t, repo, "checkout", "-qb", "active-work")
	git(t, repo, "commit", "--allow-empty", "-qm", "unrelated checkout work")
	original := git(t, repo, "rev-parse", "HEAD")
	adapter := &chatAdapter{sessions: make(map[string][]string)}
	state := filepath.Join(repo, ".gimbal")
	manager, err := New(t.Context(), state, func(string) (gimbal.HarnessAdapter, error) { return adapter, nil }, "/bin/gimbal", "/instance", repo, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	setup, err := manager.Setup(t.Context())
	if err != nil || setup.Base != "release-base" || setup.Branch != "active-work" {
		t.Fatalf("setup=%+v err=%v", setup, err)
	}
	item, err := manager.Start(t.Context(), NewConversation{Provider: "claude", Branch: "feature/selected", Base: "release-base"}, "Remember cobalt.")
	if err != nil {
		t.Fatal(err)
	}
	waitRequest(t, manager, item.ID, item.Requests[0].ID, "completed")
	if item.Branch != "feature/selected" || git(t, item.Worktree, "rev-parse", "HEAD") != initial {
		t.Fatalf("wrong checkout: %+v", item)
	}
	if git(t, repo, "rev-parse", "HEAD") != original || git(t, repo, "branch", "--show-current") != "active-work" {
		t.Fatal("original checkout changed")
	}
	shared, err := manager.Start(t.Context(), NewConversation{Provider: "claude", SharedCheckout: true, Branch: "ignored", Base: "missing"}, "Shared checkout.")
	if err != nil {
		t.Fatal(err)
	}
	waitRequest(t, manager, shared.ID, shared.Requests[0].ID, "completed")
	if len(adapter.workdirs) != 2 || adapter.workdirs[0] != item.Worktree || adapter.workdirs[1] != repo {
		t.Fatalf("harness directories = %v", adapter.workdirs)
	}
	if shared.Worktree != repo || shared.Branch != "active-work" {
		t.Fatalf("shared=%+v", shared)
	}
	if len(strings.Split(git(t, repo, "worktree", "list", "--porcelain"), "worktree ")) != 3 {
		t.Fatal("shared session made a worktree")
	}
	manager.Close()
	restarted, err := New(t.Context(), state, func(string) (gimbal.HarnessAdapter, error) { t.Fatal("restart started a harness"); return nil, nil }, "/bin/gimbal", "/instance", repo, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	for _, want := range []Conversation{item, shared} {
		got, ok := restarted.Get(want.ID)
		if !ok || got.Worktree != want.Worktree || got.Branch != want.Branch {
			t.Fatalf("lost checkout: %+v", got)
		}
	}
}

func TestInvalidSessionCheckoutDoesNotAdmitOrStartHarness(t *testing.T) {
	repo := newRepository(t)
	git(t, repo, "branch", "taken")
	manager, err := New(t.Context(), filepath.Join(repo, ".gimbal"), func(string) (gimbal.HarnessAdapter, error) {
		t.Fatal("invalid checkout contacted a harness")
		return nil, nil
	}, "/bin/gimbal", "/instance", repo, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	for _, in := range []NewConversation{
		{Provider: "claude", Branch: "taken"},
		{Provider: "claude", Branch: "bad branch"},
		{Provider: "claude", Branch: "@{-1}"},
		{Provider: "claude", Branch: "-invalid"},
		{Provider: "claude", Branch: "HEAD"},
		{Provider: "claude", Branch: "feature/valid", Base: "missing"},
		{Provider: "claude", Branch: "feature/valid", Base: "--help"},
	} {
		if _, err := manager.Start(t.Context(), in, "Do the work."); err == nil {
			t.Errorf("accepted %+v", in)
		}
	}
	if len(manager.List()) != 0 {
		t.Fatal("invalid checkout admitted a session")
	}
	if _, err := os.Stat(filepath.Join(repo, ".gimbal", "conversation-worktrees")); !os.IsNotExist(err) {
		t.Fatalf("invalid checkout created worktrees: %v", err)
	}
}

func TestSessionBaseDefaultsStayLocalAndHandleDetachedAndStaleRefs(t *testing.T) {
	repo := newRepository(t)
	current := git(t, repo, "branch", "--show-current")
	_, _, base, err := checkoutBases(t.Context(), repo)
	if err != nil || base != current {
		t.Fatalf("fallback=%q %v", base, err)
	}
	git(t, repo, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/missing")
	_, _, base, err = checkoutBases(t.Context(), repo)
	if err != nil || base != current {
		t.Fatalf("stale default=%q %v", base, err)
	}
	git(t, repo, "checkout", "--detach", "-q")
	detached := git(t, repo, "rev-parse", "HEAD")
	branch, _, base, err := checkoutBases(context.Background(), repo)
	if err != nil || branch != detached || base != detached {
		t.Fatalf("detached=%q/%q %v", branch, base, err)
	}
}

func TestSessionBranchComesFromMessageWithSafeFallback(t *testing.T) {
	for title, want := range map[string]string{"Fix the TAB strip!": "gimbal/fix-the-tab-strip", "日本語": "", " ../etc/lock ": "gimbal/etc-lock", "": ""} {
		if got := branchFrom(title); got != want {
			t.Errorf("branchFrom(%q)=%q want %q", title, got, want)
		}
	}
	repo := newRepository(t)
	adapter := &chatAdapter{sessions: make(map[string][]string)}
	m, err := New(t.Context(), filepath.Join(repo, ".gimbal"), func(string) (gimbal.HarnessAdapter, error) { return adapter, nil }, "/bin/gimbal", "/instance", repo, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	item, err := m.Start(t.Context(), NewConversation{Provider: "claude"}, "日本語")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(item.Branch, "gimbal/conversation-") {
		t.Fatalf("fallback=%q", item.Branch)
	}
}

func TestSessionDoesNotSilentlyShareAfterGitHistoryDisappears(t *testing.T) {
	directory := t.TempDir()
	m, err := New(t.Context(), filepath.Join(directory, ".gimbal"), func(string) (gimbal.HarnessAdapter, error) {
		t.Fatal("invalid checkout started a harness")
		return nil, nil
	}, "/bin/gimbal", "/instance", directory, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if _, err := m.Start(t.Context(), NewConversation{Provider: "claude", Base: "main"}, "Keep isolation"); err == nil {
		t.Fatal("silently fell back to shared checkout")
	}
}
