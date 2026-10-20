package environments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/tylergannon/gimbal/internal/modelalias"
)

const FixedWorkdirEnv = "GIMBAL_DESKTOP_FIXED_WORKDIR"

type HelperRequest struct {
	Operation   string   `json:"operation"`
	Path        string   `json:"path"`
	Origin      string   `json:"origin,omitempty"`
	OperationID string   `json:"operation_id,omitempty"`
	Checkout    Checkout `json:"checkout"`
}
type HelperResult struct {
	Directory Directory `json:"directory"`
	Prepared  Prepared  `json:"prepared"`
	Build     string    `json:"build"`
	Harnesses []Harness `json:"harnesses"`
}

func buildRevision() string {
	build := "unknown"
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, v := range info.Settings {
			if v.Key == "vcs.revision" {
				build = v.Value
			}
		}
	}
	return build
}

// inProcessHarnesses run inside Gimbal itself, so no executable of that name
// is expected on the machine. Pi's adapter is native Go.
var inProcessHarnesses = []string{"pi"}

func localFacts() (string, []Harness) {
	return localFactsFor(exec.LookPath)
}

func localFactsFor(lookPath func(string) (string, error)) (string, []Harness) {
	build := buildRevision()
	models := modelalias.Newest()
	hs := []Harness{}
	for _, id := range []string{"claude", "codex", "agy", "opencode", "pi"} {
		h := Harness{ID: id, Models: models[id], State: HarnessUnknown, Detail: "Sign-in has not been verified on this machine"}
		defaults := map[string]string{"claude": "haiku", "codex": "gpt-6-luna", "agy": "gemini-3.8-flash-low", "opencode": "opencode/gpt-5-nano", "pi": "anthropic/claude-haiku-4-5"}
		h.Default = defaults[id]
		if resolved, e := modelalias.Resolve(modelalias.Selection{Name: h.Default}); e == nil {
			h.Default = resolved.Model
		}
		if !slices.Contains(h.Models, h.Default) {
			h.Models = append([]string{h.Default}, h.Models...)
		}
		if slices.Contains(inProcessHarnesses, id) {
			h.Detail = "Built into Gimbal; provider sign-in has not been verified on this machine"
		} else if _, e := lookPath(id); e != nil {
			h.State = HarnessMissing
			h.Detail = "Not installed on this machine"
		}
		hs = append(hs, h)
	}
	return build, hs
}
func canonical(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, e := os.UserHomeDir()
		if e != nil {
			return "", e
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~"))
	}
	if path == "" {
		var e error
		path, e = os.Getwd()
		if e != nil {
			return "", e
		}
	}
	p, e := filepath.Abs(path)
	if e != nil {
		return "", e
	}
	if real, e := filepath.EvalSymlinks(p); e == nil {
		return real, nil
	}
	parent, e := filepath.EvalSymlinks(filepath.Dir(p))
	if e == nil {
		return filepath.Join(parent, filepath.Base(p)), nil
	}
	return p, nil
}
func git(ctx context.Context, dir string, args ...string) (string, error) {
	b, e := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if e != nil {
		return "", fmt.Errorf("git: %w: %s", e, strings.TrimSpace(string(b)))
	}
	return strings.TrimSpace(string(b)), nil
}
func inspectLocal(ctx context.Context, path string, list bool) (Directory, error) {
	p, e := canonical(path)
	if e != nil {
		return Directory{}, e
	}
	d := Directory{Path: p, ExecutionDirectory: p, Bases: []string{}, Entries: []DirectoryEntry{}}
	if fixed := os.Getenv(FixedWorkdirEnv); fixed != "" {
		d.FixedWorkdir, _ = canonical(fixed)
		d.ExecutionDirectory = d.FixedWorkdir
	}
	info, e := os.Stat(p)
	if errors.Is(e, os.ErrNotExist) {
		return d, nil
	}
	if e != nil {
		return d, e
	}
	if !info.IsDir() {
		return d, fmt.Errorf("%s is not a directory", p)
	}
	d.Exists = true
	entries, e := os.ReadDir(p)
	if e != nil {
		return d, e
	}
	d.Empty = len(entries) == 0
	if list {
		for _, entry := range entries {
			child := filepath.Join(p, entry.Name())
			isDir := entry.IsDir()
			if entry.Type()&os.ModeSymlink != 0 {
				f, e := os.Stat(child)
				isDir = e == nil && f.IsDir()
			}
			if isDir {
				d.Entries = append(d.Entries, DirectoryEntry{Name: entry.Name(), Path: child})
			}
		}
	}
	if _, e := os.Stat(filepath.Join(p, ".git")); e != nil {
		return d, nil
	}
	repo, e := git(ctx, p, "rev-parse", "--show-toplevel")
	if e != nil {
		return d, e
	}
	realRepo, _ := canonical(repo)
	if realRepo != p {
		return d, nil
	}
	d.Git = true
	d.Origin, _ = git(ctx, p, "remote", "get-url", "origin")
	d.Branch, _ = git(ctx, p, "symbolic-ref", "--quiet", "--short", "HEAD")
	if _, e = git(ctx, p, "rev-parse", "--verify", "--quiet", "HEAD"); e != nil {
		d.Unborn = true
		return d, nil
	}
	if d.Branch == "" {
		d.Branch, _ = git(ctx, p, "rev-parse", "HEAD")
	}
	refs, e := git(ctx, p, "for-each-ref", "--format=%(refname:short) %(symref)", "refs/heads", "refs/remotes")
	if e != nil {
		return d, e
	}
	for line := range strings.SplitSeq(refs, "\n") {
		if fields := strings.Fields(line); len(fields) == 1 {
			d.Bases = append(d.Bases, fields[0])
		}
	}
	if !slices.Contains(d.Bases, d.Branch) {
		d.Bases = append(d.Bases, d.Branch)
	}
	d.Base = d.Branch
	if known, e := git(ctx, p, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"); e == nil && slices.Contains(d.Bases, known) {
		d.Base = known
		if local, ok := strings.CutPrefix(known, "origin/"); ok && slices.Contains(d.Bases, local) {
			d.Base = local
		}
	}
	return d, nil
}
func RunHelper(ctx context.Context, r HelperRequest) (HelperResult, error) {
	build, hs := localFacts()
	out := HelperResult{Build: build, Harnesses: hs}
	var e error
	switch r.Operation {
	case "inspect", "list":
		out.Directory, e = inspectLocal(ctx, r.Path, r.Operation == "list")
	case "prepare":
		out.Prepared, e = prepareLocal(ctx, r.Path, r.Checkout, r.OperationID)
		if e == nil {
			out.Directory, e = inspectLocal(ctx, out.Prepared.Directory, false)
		}
	case "clone":
		out.Directory, e = cloneLocal(ctx, r.Path, r.Origin, r.OperationID)
	default:
		e = fmt.Errorf("unknown desktop-environment operation %q", r.Operation)
	}
	return out, e
}
func prepareLocal(ctx context.Context, path string, c Checkout, id string) (Prepared, error) {
	d, e := inspectLocal(ctx, path, false)
	if e != nil {
		return Prepared{}, e
	}
	if !d.Exists {
		return Prepared{}, fmt.Errorf("directory %s does not exist", d.Path)
	}
	if d.FixedWorkdir != "" && d.ExecutionDirectory != d.Path {
		return Prepared{}, fmt.Errorf("daemon fixes execution to %s; selected directory %s is unavailable", d.FixedWorkdir, d.Path)
	}
	p := Prepared{Directory: d.Path, Branch: d.Branch}
	if c.SharedCheckout || !d.Git || d.Unborn {
		if !c.SharedCheckout && (c.Branch != "" || c.Base != "") {
			return p, fmt.Errorf("directory has no committed Git history")
		}
		return p, nil
	}
	if d.FixedWorkdir != "" {
		return p, fmt.Errorf("daemon has a fixed working directory; worktree isolation is unavailable")
	}
	if !validOperationID(id) {
		return p, fmt.Errorf("valid conversation identity required")
	}
	base := c.Base
	if base == "" {
		base = d.Base
	}
	commit, e := git(ctx, d.Path, "rev-parse", "--verify", "--end-of-options", base+"^{commit}")
	if e != nil {
		return p, fmt.Errorf("base %q is not an available commit", base)
	}
	branch := strings.TrimSpace(c.Branch)
	if branch == "" {
		branch = "gimbal/conversation-" + id[:min(12, len(id))]
	}
	if strings.HasPrefix(branch, "-") || branch == "HEAD" {
		return p, fmt.Errorf("invalid branch")
	}
	if _, e = git(ctx, d.Path, "check-ref-format", "refs/heads/"+branch); e != nil {
		return p, e
	}
	worktree := filepath.Join(d.Path, ".gimbal", "conversation-worktrees", id)
	receipt := filepath.Join(d.Path, ".gimbal", "environment-operations", id+".json")
	want := Prepared{Directory: worktree, Branch: branch, BaseCommit: commit}
	if b, e := os.ReadFile(receipt); e == nil {
		var saved Prepared
		if json.Unmarshal(b, &saved) != nil || saved.Directory != want.Directory || saved.Branch != want.Branch || saved.BaseCommit != want.BaseCommit {
			return p, fmt.Errorf("operation identity already belongs to a different checkout")
		}
		if actual, e := git(ctx, worktree, "symbolic-ref", "--quiet", "--short", "HEAD"); e == nil {
			if actual != branch {
				return p, fmt.Errorf("owned worktree branch changed")
			}
			return want, nil
		}
	}
	if _, e := os.Stat(worktree); e == nil {
		actual, e := git(ctx, worktree, "symbolic-ref", "--quiet", "--short", "HEAD")
		head, herr := git(ctx, worktree, "rev-parse", "HEAD")
		if e == nil && herr == nil && actual == branch && head == commit {
			if e = writeReceipt(receipt, want); e != nil {
				return p, e
			}
			return want, nil
		}
		return p, fmt.Errorf("existing operation worktree does not match branch/base")
	}
	if _, e = git(ctx, d.Path, "show-ref", "--verify", "--quiet", "refs/heads/"+branch); e == nil {
		return p, fmt.Errorf("branch %q already exists; choose a new branch", branch)
	}
	if e = os.MkdirAll(filepath.Dir(worktree), 0700); e != nil {
		return p, e
	}
	if e = writeReceipt(receipt, want); e != nil {
		return p, e
	}
	if _, e = git(ctx, d.Path, "worktree", "add", "-b", branch, worktree, commit); e != nil {
		return p, e
	}
	return want, nil
}
func validOperationID(id string) bool {
	if id == "" || id == "." || id == ".." {
		return false
	}
	for _, c := range id {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_':
		default:
			return false
		}
	}
	return true
}
func writeReceipt(path string, v any) error {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".operation-*")
	if e != nil {
		return e
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if _, e = f.Write(b); e == nil {
		e = f.Sync()
	}
	e = errors.Join(e, f.Close())
	if e != nil {
		return e
	}
	return os.Rename(f.Name(), path)
}
func cloneLocal(ctx context.Context, path, origin, id string) (Directory, error) {
	if !validOperationID(id) || origin == "" || strings.HasPrefix(origin, "-") {
		return Directory{}, fmt.Errorf("origin and operation identity required")
	}
	p, e := canonical(path)
	if e != nil {
		return Directory{}, e
	}
	receipt := filepath.Join(filepath.Dir(p), ".gimbal-clone-"+id+".json")
	type cloneRecord struct {
		Path, Origin string
		Complete     bool
	}
	want := cloneRecord{Path: p, Origin: origin}
	if b, e := os.ReadFile(receipt); e == nil {
		var old cloneRecord
		if json.Unmarshal(b, &old) != nil || old.Path != p || old.Origin != origin {
			return Directory{}, fmt.Errorf("clone identity already belongs to another operation")
		}
		d, e := inspectLocal(ctx, p, false)
		if e != nil {
			return d, e
		}
		if d.Git && d.Origin == origin {
			return d, nil
		}
		return d, fmt.Errorf("prior clone outcome is unconfirmed; inspect destination before starting another clone")
	}
	d, e := inspectLocal(ctx, p, false)
	if e != nil {
		return d, e
	}
	if d.Exists && !d.Empty {
		return d, fmt.Errorf("clone destination must be empty")
	}
	if e = writeReceipt(receipt, want); e != nil {
		return d, e
	}
	b, e := exec.CommandContext(ctx, "git", "clone", "--", origin, p).CombinedOutput()
	if e != nil {
		return d, fmt.Errorf("clone outcome requires inspection: %w: %s", e, strings.TrimSpace(string(b)))
	}
	want.Complete = true
	if e = writeReceipt(receipt, want); e != nil {
		return d, e
	}
	return inspectLocal(ctx, p, false)
}
