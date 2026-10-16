package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type gitFixture struct {
	repo, remote, source string
	p                    preparer
	t                    *testing.T
}

func (f gitFixture) git(args ...string) string {
	f.t.Helper()
	value, err := f.p.git(args...)
	if err != nil {
		f.t.Fatal(err)
	}
	return value
}
func (f gitFixture) write(name, text string) {
	f.t.Helper()
	if err := os.WriteFile(filepath.Join(f.repo, name), []byte(text), 0600); err != nil {
		f.t.Fatal(err)
	}
}
func (f gitFixture) commit(subject string) string {
	f.t.Helper()
	f.write("source.txt", subject)
	f.git("add", ".")
	f.git("commit", "-qm", subject)
	f.git("push", "origin", "main")
	return f.git("rev-parse", "HEAD")
}
func fixture(t *testing.T) gitFixture {
	t.Helper()
	root := t.TempDir()
	f := gitFixture{t: t, repo: filepath.Join(root, "repo"), remote: filepath.Join(root, "remote.git")}
	for _, args := range [][]string{{"init", "--bare", f.remote}, {"clone", f.remote, f.repo}} {
		if raw, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("%v: %v: %s", args, err, raw)
		}
	}
	f.p = newPreparer(f.repo)
	f.git("config", "user.name", "Release test")
	f.git("config", "user.email", "release@example.invalid")
	f.git("checkout", "-b", "main")
	f.write("CHANGELOG.md", "# Changelog\n\n## [Unreleased]\n\n## [0.13.3] - 2026-10-12\n\nOld release.\n\n[Unreleased]: https://github.com/tylergannon/gimbal/compare/v0.13.3...main\n")
	f.git("add", ".")
	f.git("commit", "-qm", "Initial release")
	f.git("tag", "v0.13.3")
	f.git("push", "origin", "main", "--tags")
	text, err := os.ReadFile(filepath.Join(f.repo, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	f.write("CHANGELOG.md", strings.Replace(string(text), "## [Unreleased]\n", "## [Unreleased]\n\n### Added\n\n- Native workspace.\n", 1))
	f.source = f.commit("fix: workspace release")
	return f
}
func prepare(t *testing.T, f gitFixture, source string) (string, string) {
	t.Helper()
	tag, sha, err := f.p.prepare(source, "origin")
	if err != nil {
		t.Fatal(err)
	}
	return tag, sha
}
func TestPatchChangelogAtomicPushAndSourceIdentity(t *testing.T) {
	f := fixture(t)
	tag, sha := prepare(t, f, f.source)
	if tag != "v0.13.4" {
		t.Fatal(tag)
	}
	if got := strings.Fields(f.git("ls-remote", "origin", "refs/heads/main"))[0]; got != sha {
		t.Fatal(got)
	}
	if f.git("diff", "--name-only", f.source, sha) != "CHANGELOG.md" {
		t.Fatal("non-metadata changed")
	}
	if !strings.Contains(f.git("show", "-s", "--format=%B", sha), "Release-Source: "+f.source) {
		t.Fatal("source marker missing")
	}
	raw, err := os.ReadFile(filepath.Join(f.repo, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if strings.Count(text, "- Native workspace.") != 1 || !strings.Contains(text, "## [Unreleased]\n\n## [0.13.4] - ") || !strings.Contains(text, "[0.13.4]: https://github.com/tylergannon/gimbal/compare/v0.13.3...v0.13.4") {
		t.Fatal(text)
	}
}
func TestRerunReusesAndMetadataDoesNotBump(t *testing.T) {
	f := fixture(t)
	tag, sha := prepare(t, f, f.source)
	againTag, againSHA := prepare(t, f, f.source)
	metadataTag, metadataSHA := prepare(t, f, sha)
	if tag != againTag || sha != againSHA || tag != metadataTag || sha != metadataSHA || strings.Contains(f.git("tag", "--list"), "v0.13.5") {
		t.Fatal("rerun changed reservation")
	}
}
func TestOldSourceCannotDowngradeNewRelease(t *testing.T) {
	f := fixture(t)
	prepare(t, f, f.source)
	f.git("checkout", "-B", "main", "origin/main")
	newer := f.commit("fix: next source")
	tag, _ := prepare(t, f, newer)
	if tag != "v0.13.5" {
		t.Fatal(tag)
	}
	tag, sha := prepare(t, f, f.source)
	if tag != "" || sha != "" {
		t.Fatal("old source republished")
	}
}
func TestNewerMainSkipsStaleSource(t *testing.T) {
	f := fixture(t)
	newer := f.commit("Newer workspace")
	tag, sha := prepare(t, f, f.source)
	if tag != "" || sha != "" {
		t.Fatal("stale reserved")
	}
	if !strings.HasPrefix(f.git("ls-remote", "origin", "refs/heads/main"), newer) || strings.Contains(f.git("tag", "--list"), "v0.13.4") {
		t.Fatal("stale source changed remote")
	}
}
func TestMainRaceRejectsAtomicPushWithoutTag(t *testing.T) {
	f := fixture(t)
	other := filepath.Join(t.TempDir(), "other")
	if raw, err := exec.Command("git", "clone", "--branch", "main", f.remote, other).CombinedOutput(); err != nil {
		t.Fatalf("clone: %v %s", err, raw)
	}
	racer := newPreparer(other)
	for _, args := range [][]string{{"config", "user.name", "Race"}, {"config", "user.email", "race@example.invalid"}} {
		if _, err := racer.git(args...); err != nil {
			t.Fatal(err)
		}
	}
	original := f.p.git
	f.p.git = func(args ...string) (string, error) {
		if len(args) > 1 && args[0] == "push" && args[1] == "--atomic" {
			if err := os.WriteFile(filepath.Join(other, "newer.txt"), []byte("newer main"), 0600); err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{{"add", "."}, {"commit", "-qm", "Race advance"}, {"push", "origin", "main"}} {
				if _, err := racer.git(args...); err != nil {
					t.Fatal(err)
				}
			}
		}
		return original(args...)
	}
	tag, sha := prepare(t, f, f.source)
	if tag != "" || sha != "" || f.git("ls-remote", "origin", "refs/tags/v0.13.4") != "" || f.git("show", "origin/main:newer.txt") != "newer main" {
		t.Fatal("atomic race published stale tag")
	}
}
func TestEmptyUnreleasedUsesCommitSubjects(t *testing.T) {
	f := fixture(t)
	raw, err := os.ReadFile(filepath.Join(f.repo, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	f.write("CHANGELOG.md", strings.Replace(string(raw), "\n### Added\n\n- Native workspace.\n", "", 1))
	source := f.commit("fix: new icon")
	prepare(t, f, source)
	raw, err = os.ReadFile(filepath.Join(f.repo, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "- fix: new icon") || !strings.Contains(string(raw), "- fix: workspace release") {
		t.Fatal(string(raw))
	}
}

func TestFeatureWinsAcrossCoalescedCommits(t *testing.T) {
	f := fixture(t)
	f.commit("feat(ui): show run transcripts")
	source := f.commit("fix: repair transcript scrolling")
	tag, sha := prepare(t, f, source)
	if tag != "v0.14.0" {
		t.Fatal(tag)
	}
	if again, same := prepare(t, f, source); again != tag || same != sha {
		t.Fatal("minor release retry changed reservation")
	}
}

func TestNoReleaseCommitsLeaveRepositoryUntouched(t *testing.T) {
	f := fixture(t)
	f.git("reset", "--hard", "v0.13.3")
	f.git("push", "--force", "origin", "main")
	source := f.commit("chore: maintain release tooling")
	tag, sha := prepare(t, f, source)
	if tag != "" || sha != "" || f.git("tag", "--list") != "v0.13.3" || f.git("rev-parse", "HEAD") != source {
		t.Fatal("non-release commit reserved a version")
	}
}

func TestBreakingDeclarationCannotReserveRelease(t *testing.T) {
	for _, message := range []string{"feat!: replace the API", "fix(api)!: replace the API", "feat: replace the API\n\nBREAKING CHANGE: old API removed", "chore: remove old API\n\nBREAKING-CHANGE: old API removed"} {
		t.Run(message, func(t *testing.T) {
			f := fixture(t)
			source := f.commit(message)
			tag, sha, err := f.p.prepare(source, "origin")
			if err == nil || !strings.Contains(err.Error(), "major version increments are forbidden") {
				t.Fatalf("%q %q %v", tag, sha, err)
			}
			if f.git("tag", "--list") != "v0.13.3" || f.git("rev-parse", "HEAD") != source || f.git("status", "--porcelain") != "" {
				t.Fatal("rejected declaration mutated release state")
			}
		})
	}
}
