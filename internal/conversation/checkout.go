package conversation

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

type checkout struct{ repo, branch, commit string }

// prepareCheckout validates all choices before creating files or a harness.
func (m *Manager) prepareCheckout(ctx context.Context, in NewConversation) (checkout, error) {
	repo, err := conversationRepository(ctx, filepath.Dir(m.projectDir))
	if err != nil {
		return checkout{}, fmt.Errorf("conversation: find repository: %w", err)
	}
	if repo == "" {
		if !in.SharedCheckout && (in.Branch != "" || in.Base != "") {
			return checkout{}, fmt.Errorf("conversation: this directory has no committed Git history; refresh session setup before starting")
		}
		return checkout{}, nil
	}
	current, _, base, err := checkoutBases(ctx, repo)
	if err != nil {
		return checkout{}, err
	}
	if in.SharedCheckout {
		return checkout{repo: repo, branch: current}, nil
	}
	branch := strings.TrimSpace(in.Branch)
	if branch == "" {
		branch = branchFrom(in.Title)
	}
	if branch != "" {
		if _, err := gitOutput(ctx, repo, "check-ref-format", "refs/heads/"+branch); err != nil || strings.HasPrefix(branch, "-") || branch == "HEAD" {
			return checkout{}, fmt.Errorf("conversation: invalid branch %q", branch)
		}
		if _, err := gitOutput(ctx, repo, "show-ref", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
			return checkout{}, fmt.Errorf("conversation: branch %q already exists; choose a new branch", branch)
		}
	}
	if in.Base != "" {
		base = in.Base
	}
	commit, err := gitOutput(ctx, repo, "rev-parse", "--verify", "--end-of-options", base+"^{commit}")
	if err != nil {
		return checkout{}, fmt.Errorf("conversation: base %q is not an available commit; choose a local branch", base)
	}
	return checkout{repo: repo, branch: branch, commit: commit}, nil
}

// checkoutBases reads only refs already present. A known remote default is used
// only when it still resolves; otherwise the current branch/commit is explicit.
func checkoutBases(ctx context.Context, repo string) (current string, bases []string, base string, err error) {
	current, err = gitOutput(ctx, repo, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		current, err = gitOutput(ctx, repo, "rev-parse", "HEAD")
	}
	if err != nil {
		return "", nil, "", fmt.Errorf("conversation: read current checkout: %w", err)
	}
	refs, err := gitOutput(ctx, repo, "for-each-ref", "--format=%(refname:short) %(symref)", "refs/heads", "refs/remotes")
	if err != nil {
		return "", nil, "", fmt.Errorf("conversation: list base branches: %w", err)
	}
	bases = []string{}
	for line := range strings.SplitSeq(refs, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 1 {
			continue
		} // Ignore symbolic remote HEAD aliases.
		bases = append(bases, fields[0])
	}
	if !slices.Contains(bases, current) {
		bases = append(bases, current)
	}
	base = current
	if known, readErr := gitOutput(ctx, repo, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"); readErr == nil && slices.Contains(bases, known) {
		base = known
		if local, ok := strings.CutPrefix(known, "origin/"); ok && slices.Contains(bases, local) {
			base = local
		}
	}
	return current, bases, base, nil
}

func branchFrom(title string) string {
	var slug strings.Builder
	separator := false
	for _, c := range strings.ToLower(title) {
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' {
			if separator && slug.Len() > 0 {
				slug.WriteByte('-')
			}
			slug.WriteRune(c)
			separator = false
		} else {
			separator = true
		}
		if slug.Len() >= 48 {
			break
		}
	}
	if slug.Len() == 0 {
		return ""
	}
	return "gimbal/" + slug.String()
}
