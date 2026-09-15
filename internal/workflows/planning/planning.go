// Package planning turns a request into a local implementation plan.
package planning

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/agy"
	"github.com/tylergannon/gimble/claude"
	"github.com/tylergannon/gimble/codex"
)

// Input is the complete context for a planning run. OutputDir is allocated by
// the caller and is expected to be unique for this run.
type Input struct {
	Repo          string
	Goal          string
	Acceptance    string
	Constraints   string
	ContextFiles  []string
	Checks        []string
	Model         string
	ReviewModel   string
	PlanningModel string
	OutputDir     string
	DryRun        bool
}

type synthesis struct {
	// One essential clarification, or empty when a plan can be written from the supplied information.
	Question string `json:"question"`
	// Complete Markdown plan preserving the user's goal, acceptance criteria, constraints, and concrete validation.
	Plan string `json:"plan"`
}

const (
	defaultModel         = "gpt-5.6-luna"
	defaultReviewModel   = "haiku"
	defaultPlanningModel = "gemini-3.8-flash-low"
	keyDraftResult       = "draft result"
	keyCritiqueResult    = "critique result"
	keyClarification     = "clarification answer"
)

// Plan creates three independent drafts, three independent cross-critiques,
// and a final Markdown plan. It returns the final plan's absolute path.
func Plan(ctx context.Context, in Input, reader *bufio.Reader, out io.Writer) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := validate(&in); err != nil {
		return "", err
	}
	repo, err := filepath.Abs(in.Repo)
	if err != nil {
		return "", err
	}
	outDir, err := filepath.Abs(in.OutputDir)
	if err != nil {
		return "", err
	}
	in.Repo, in.OutputDir = repo, outDir
	if err := os.MkdirAll(in.OutputDir, 0o755); err != nil {
		return "", fmt.Errorf("planning output: %w", err)
	}
	if in.DryRun {
		return dryRun(in, out)
	}

	brief := promptBrief(in)
	gimble.Set(ctx, "goal", in.Goal)
	gimble.Set(ctx, "acceptance", in.Acceptance)
	gimble.Set(ctx, "constraints", in.Constraints)
	gimble.Set(ctx, "context files", in.ContextFiles)
	gimble.Set(ctx, "checks", in.Checks)
	ls := lanes(in)
	if err := plan(ctx, in, brief, reader, out, ls); err != nil {
		return "", err
	}
	return filepath.Abs(filepath.Join(in.OutputDir, "plan.md"))
}

func plan(ctx context.Context, in Input, brief string, reader *bufio.Reader, out io.Writer, ls []lane) error {
	if len(ls) != 3 {
		return errors.New("planning: need three agent lanes")
	}
	if err := runDrafts(ctx, in, brief, ls); err != nil {
		return err
	}
	if err := runCritiques(ctx, in, brief, ls); err != nil {
		return err
	}
	_, err := synthesize(ctx, in, brief, reader, out, ls[2])
	return err
}

func validate(in *Input) error {
	for name, value := range map[string]string{"repo": in.Repo, "goal": in.Goal, "output dir": in.OutputDir} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("planning: %s is required", name)
		}
	}
	repo, err := filepath.Abs(in.Repo)
	if err != nil {
		return err
	}
	if info, err := os.Stat(repo); err != nil || !info.IsDir() {
		return fmt.Errorf("planning: repo must be a directory")
	}
	in.Repo = repo
	for i, path := range in.ContextFiles {
		absolute := path
		if !filepath.IsAbs(path) {
			absolute = filepath.Join(repo, path)
		}
		info, err := os.Stat(absolute)
		if err != nil {
			return fmt.Errorf("planning context %s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("planning context %s is not a regular file", path)
		}
		in.ContextFiles[i] = absolute
	}
	return nil
}

func promptBrief(in Input) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Goal:\n%s\n\nAcceptance:\n%s\n\nConstraints:\n%s\n\nRepository: %s\n", in.Goal, in.Acceptance, in.Constraints, in.Repo)
	if len(in.ContextFiles) > 0 {
		b.WriteString("Read these local context files:\n")
		for _, path := range in.ContextFiles {
			b.WriteString("- ")
			b.WriteString(path)
			b.WriteByte('\n')
		}
	}
	if len(in.Checks) > 0 {
		b.WriteString("Use these exact validation commands when describing proof:\n")
		for _, check := range in.Checks {
			b.WriteString("- ")
			b.WriteString(check)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

type lane struct {
	name, model string
	adapter     gimble.HarnessAdapter
}

func lanes(in Input) []lane {
	model, review, planning := in.Model, in.ReviewModel, in.PlanningModel
	if model == "" {
		model = defaultModel
	}
	if review == "" {
		review = defaultReviewModel
	}
	if planning == "" {
		planning = defaultPlanningModel
	}
	return []lane{
		{name: "codex", model: model, adapter: codex.New()},
		{name: "claude", model: review, adapter: claude.New()},
		{name: "gemini", model: planning, adapter: agy.New()},
	}
}

func runDrafts(ctx context.Context, in Input, brief string, ls []lane) error {
	g := gimble.Group(ctx, "drafts")
	g.Go("codex", func(ctx context.Context) error { return draft(ctx, in, brief, ls[0]) })
	g.Go("claude", func(ctx context.Context) error { return draft(ctx, in, brief, ls[1]) })
	g.Go("gemini", func(ctx context.Context) error { return draft(ctx, in, brief, ls[2]) })
	return g.Wait()
}

func draft(ctx context.Context, in Input, brief string, l lane) error {
	s := gimble.NewSession(ctx, "draft", l.adapter, l.model, in.Repo)
	text, err := s.Generate[gimble.Text](ctx, "Read the repository instructions and relevant code, then draft a concise implementation plan with ordered tasks and concrete validation. Preserve the request and supplied validation commands exactly; leave optional improvements out. Change no files and run no mutating commands.\n\n"+brief+"\n\n"+gimble.ScopeText(ctx))
	if err != nil {
		return err
	}
	gimble.Set(ctx, keyDraftResult, string(text))
	return writeText(in.OutputDir, "draft-"+l.name+".md", string(text))
}

func runCritiques(ctx context.Context, in Input, brief string, ls []lane) error {
	draftCodex, err := os.ReadFile(filepath.Join(in.OutputDir, "draft-codex.md"))
	if err != nil {
		return err
	}
	draftClaude, err := os.ReadFile(filepath.Join(in.OutputDir, "draft-claude.md"))
	if err != nil {
		return err
	}
	draftGemini, err := os.ReadFile(filepath.Join(in.OutputDir, "draft-gemini.md"))
	if err != nil {
		return err
	}
	g := gimble.Group(ctx, "critiques")
	g.Go("codex", func(ctx context.Context) error {
		return critique(ctx, in, brief, ls[0], string(draftClaude), string(draftGemini))
	})
	g.Go("claude", func(ctx context.Context) error {
		return critique(ctx, in, brief, ls[1], string(draftCodex), string(draftGemini))
	})
	g.Go("gemini", func(ctx context.Context) error {
		return critique(ctx, in, brief, ls[2], string(draftCodex), string(draftClaude))
	})
	return g.Wait()
}

func critique(ctx context.Context, in Input, brief string, l lane, first, second string) error {
	s := gimble.NewSession(ctx, "critique", l.adapter, l.model, in.Repo)
	gimble.Set(ctx, "other plans", "OTHER PLAN A:\n"+first+"\n\nOTHER PLAN B:\n"+second)
	prompt := "Critique exactly these other two plans for scope, ordering, feasibility, and proof. Preserve and assess the supplied validation commands exactly. Ground findings in the repository and original request. Change no files and run no mutating commands.\n\n" + brief + "\n\n" + gimble.ScopeText(ctx)
	text, err := s.Generate[gimble.Text](ctx, prompt)
	if err != nil {
		return err
	}
	gimble.Set(ctx, keyCritiqueResult, string(text))
	return writeText(in.OutputDir, "critique-"+l.name+".md", string(text))
}

func synthesize(ctx context.Context, in Input, brief string, reader *bufio.Reader, out io.Writer, l lane) (string, error) {
	all, err := readNamed(in.OutputDir, "draft-codex.md", "draft-claude.md", "draft-gemini.md", "critique-codex.md", "critique-claude.md", "critique-gemini.md")
	if err != nil {
		return "", err
	}
	s := gimble.NewSession(ctx, "synthesizer", l.adapter, l.model, in.Repo)
	gimble.Set(ctx, "drafts and critiques", all)
	prompt := "Synthesize the final Markdown implementation plan. Preserve the original goal, acceptance, constraints, local context paths, and supplied validation commands exactly. Resolve critiques on their merits without expanding the task. Include ordered tasks and how to demonstrate the requested result. Change no files and run no mutating commands. Put a focused clarification in question when essential; otherwise leave question empty. Put the complete Markdown plan in plan when no question is needed.\n\n" + brief + "\n\n" + gimble.ScopeText(ctx)
	if reader == nil {
		prompt += "\nThe caller requested non-interactive planning. Leave question empty, state necessary assumptions in the plan, and do not invent additional requirements."
	}
	text, err := s.Generate[synthesis](ctx, prompt)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(string(text.Plan)) == "" && strings.TrimSpace(string(text.Question)) == "" {
		return "", errors.New("planning: synthesizer returned neither a question nor a plan")
	}
	final := string(text.Plan)
	if strings.TrimSpace(string(text.Question)) != "" {
		if reader == nil {
			return "", errors.New("planning: synthesizer asked a question but no input reader was provided")
		}
		question := string(text.Question)
		if _, err := fmt.Fprintln(out, question); err != nil {
			return "", err
		}
		answer, err := planningAnswer(ctx, reader)
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		if strings.TrimSpace(answer) == "" {
			return "", errors.New("planning: empty clarification answer")
		}
		gimble.Set(ctx, keyClarification, answer)
		if err := os.WriteFile(filepath.Join(in.OutputDir, "answers.md"), []byte("# Clarification answers\n\nQuestion:\n"+question+"\n\nAnswer:\n"+answer), 0o644); err != nil {
			return "", err
		}
		text, err = s.Generate[synthesis](ctx, "Use this exact clarification answer and now write only the final Markdown plan in Plan, with Question empty:\n"+answer)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(string(text.Question)) != "" || strings.TrimSpace(string(text.Plan)) == "" {
			return "", errors.New("planning: synthesizer did not return a final plan after clarification")
		}
		final = string(text.Plan)
	}
	path := filepath.Join(in.OutputDir, "plan.md")
	if err := writeText(in.OutputDir, "plan.md", final); err != nil {
		return "", err
	}
	absolute, err := filepath.Abs(path)
	return absolute, err
}

func planningAnswer(ctx context.Context, reader *bufio.Reader) (string, error) {
	// Reading is synchronous. The caller owns closing a blocked reader when it
	// needs cancellation to unblock it; this helper does not leak a goroutine.
	if err := ctx.Err(); err != nil {
		return "", err
	}
	text, err := reader.ReadString('\n')
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", ctxErr
	}
	return text, err
}

func readNamed(dir string, names ...string) (string, error) {
	var b strings.Builder
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "\n## %s\n\n%s\n", name, data)
	}
	return b.String(), nil
}

func writeText(dir, name, text string) error {
	return os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644)
}

func dryRun(in Input, out io.Writer) (string, error) {
	for _, name := range []string{"codex", "claude", "gemini"} {
		if _, err := fmt.Fprintf(out, "=== draft %s (no model was called) ===\n%s\n", name, promptBrief(in)); err != nil {
			return "", err
		}
		if err := writeText(in.OutputDir, "draft-"+name+".md", "# Example draft\n\n(example answer; no model was called)\n"); err != nil {
			return "", err
		}
	}
	for _, name := range []string{"codex", "claude", "gemini"} {
		if _, err := fmt.Fprintf(out, "=== critique %s (no model was called) ===\n", name); err != nil {
			return "", err
		}
		if err := writeText(in.OutputDir, "critique-"+name+".md", "# Example critique\n\n(example answer; no model was called)\n"); err != nil {
			return "", err
		}
	}
	if err := writeText(in.OutputDir, "plan.md", "# Example plan\n\n(example plan; no model was called)\n"); err != nil {
		return "", err
	}
	return filepath.Abs(filepath.Join(in.OutputDir, "plan.md"))
}
