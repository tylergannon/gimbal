// Package plan is the plan workflow, the Gimble translation of the
// df-sprint-plan skill. A planner orients and writes the sprint's intent;
// Claude, Codex, and Gemini each draft a sprint plan from it, then each
// critiques the other two; the planner writes questions for the person
// planning the sprint, the run waits for their answers, and the planner
// merges the drafts into the sprint document.
package plan

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/tylergannon/gimble"
)

//go:generate go run github.com/tylergannon/gimble/internal/generate/gimblegen -entry Plan -name plan

// Input starts the plan workflow.
type Input struct {
	// The sprint to plan: NNN of the docs/sprints/SPRINT-NNN.md it writes.
	Sprint int
	// What the sprint should be about, in a sentence or a paragraph.
	Seed string
	// Absolute path of the working directory.
	WorkDir string
}

// The roles Plan names, and the model each runs on unless the run's flag
// says otherwise: the planner and the Claude lane on Claude, the Codex and
// Gemini lanes on theirs, all on the cheap tier.
var roles = map[string]string{
	"planner": "claude-haiku-4-5-20251001",
	"claude":  "claude-haiku-4-5-20251001",
	"codex":   "gpt-5.6-luna",
	"gemini":  "gemini-3.8-flash-low",
}

// Plan writes docs/sprints/SPRINT-NNN.md for in.Sprint. It names four roles,
// which the run binds: planner, claude, codex, and gemini.
func Plan(ctx context.Context, in Input) error {
	drafts := filepath.Join(in.WorkDir, "docs", "sprints", "drafts")
	if err := os.MkdirAll(drafts, 0o755); err != nil {
		return err
	}
	file := func(name string) string {
		return filepath.Join(drafts, fmt.Sprintf("SPRINT-%03d-%s.md", in.Sprint, name))
	}
	claudeDraft, codexDraft, geminiDraft := file("CLAUDE-DRAFT"), file("CODEX-DRAFT"), file("GEMINI-DRAFT")
	gimble.Set(ctx, "seed", in.Seed)
	gimble.Set(ctx, "intent document", file("INTENT"))
	gimble.Set(ctx, "drafts directory", drafts)

	planner := gimble.NewSession(ctx, "planner", in.WorkDir)
	if _, err := planner.Generate[gimble.Text](ctx, intentPrompt); err != nil {
		return err
	}

	lanes := gimble.Group(ctx, "drafts")
	lanes.Go("claude", func(ctx context.Context) error {
		gimble.Set(ctx, "your draft", claudeDraft)
		claude := gimble.NewSession(ctx, "claude", in.WorkDir)
		_, err := claude.Generate[gimble.Text](ctx, draftPrompt)
		return err
	})
	lanes.Go("codex", func(ctx context.Context) error {
		gimble.Set(ctx, "your draft", codexDraft)
		codex := gimble.NewSession(ctx, "codex", in.WorkDir)
		_, err := codex.Generate[gimble.Text](ctx, draftPrompt)
		return err
	})
	lanes.Go("gemini", func(ctx context.Context) error {
		gimble.Set(ctx, "your draft", geminiDraft)
		gemini := gimble.NewSession(ctx, "gemini", in.WorkDir)
		_, err := gemini.Generate[gimble.Text](ctx, draftPrompt)
		return err
	})
	if err := lanes.Wait(); err != nil {
		return err
	}

	critiques := gimble.Group(ctx, "critiques")
	critiques.Go("claude", func(ctx context.Context) error {
		gimble.Set(ctx, "drafts to review", []string{codexDraft, geminiDraft})
		gimble.Set(ctx, "your critique", file("CLAUDE-CRITIQUE"))
		claude := gimble.NewSession(ctx, "claude", in.WorkDir)
		_, err := claude.Generate[gimble.Text](ctx, critiquePrompt)
		return err
	})
	critiques.Go("codex", func(ctx context.Context) error {
		gimble.Set(ctx, "drafts to review", []string{claudeDraft, geminiDraft})
		gimble.Set(ctx, "your critique", file("CODEX-CRITIQUE"))
		codex := gimble.NewSession(ctx, "codex", in.WorkDir)
		_, err := codex.Generate[gimble.Text](ctx, critiquePrompt)
		return err
	})
	critiques.Go("gemini", func(ctx context.Context) error {
		gimble.Set(ctx, "drafts to review", []string{claudeDraft, codexDraft})
		gimble.Set(ctx, "your critique", file("GEMINI-CRITIQUE"))
		gemini := gimble.NewSession(ctx, "gemini", in.WorkDir)
		_, err := gemini.Generate[gimble.Text](ctx, critiquePrompt)
		return err
	})
	if err := critiques.Wait(); err != nil {
		return err
	}

	// The interview: the planner asks, the person answers in a file, and the
	// run waits for it.
	questions, answers := file("QUESTIONS"), file("ANSWERS")
	gimble.Set(ctx, "questions file", questions)
	gimble.Set(ctx, "answers file", answers)
	if _, err := planner.Generate[gimble.Text](ctx, questionsPrompt); err != nil {
		return err
	}
	log.Printf("plan: answer the questions in %s by writing %s; the run waits for it", questions, answers)
	if _, _, _, err := gimble.RunCommand(ctx, "ask", in.WorkDir, "sh", "-c", `until [ -s "$1" ]; do sleep 5; done`, "sh", answers); err != nil {
		return err
	}

	gimble.Set(ctx, "merge notes file", file("MERGE-NOTES"))
	gimble.Set(ctx, "sprint document", filepath.Join(in.WorkDir, "docs", "sprints", fmt.Sprintf("SPRINT-%03d.md", in.Sprint)))
	summary, err := planner.Generate[gimble.Text](ctx, mergePrompt)
	if err != nil {
		return err
	}
	log.Printf("plan: sprint %d:\n%s", in.Sprint, summary)
	return nil
}

const intentPrompt = `You are planning a sprint from the seed below. Orient first: read AGENTS.md, CLAUDE.md, or equivalent; the three most recent documents in docs/sprints/; docs/chapters/ledger.yaml and any chapter the seed names or a recent sprint links, if chapters exist; docs/SEMANTIC-INDEX.md if it exists; and the code the seed touches. Then write the intent document named below with these sections: Seed, Context, Pyramid Index, Semantic Index, Chapter Context, Recent Sprint Context, Relevant Codebase Areas, Constraints, Success Criteria, Open Questions. Answer with a short orientation summary.`

const draftPrompt = `Read the intent document named below, this project's structure in AGENTS.md or equivalent, and its planning style in any existing docs/sprints/SPRINT-*.md. If the intent selects a chapter, read its chapter document and keep the plan aligned to it without treating the chapter as authoritative over sprint status. If the intent names a semantic index, follow its entrypoint to prior art before proposing an approach. Then write a comprehensive sprint plan to your draft file named below, with these sections: Overview, Use Cases, Architecture, Implementation Plan (phased, with files and tasks), Files Summary, Definition of Done, Risks and Mitigations, Dependencies, Open Questions.`

const critiquePrompt = `You are reviewing two competing sprint plan drafts. Read the intent document named below for context, then both drafts named below. Write your critique to the critique file named below. For each draft, evaluate architectural soundness, completeness, phasing and ordering, risk coverage, feasibility, and definition of done. Note the strongest ideas worth keeping from each, and its weaknesses and gaps.`

const questionsPrompt = `Read the three drafts and the three critiques in the drafts directory named below. Write the questions file named below for the person planning this sprint: first a short summary of the key differences across the drafts, where the critiques agree, and the unresolved tensions; then two to four targeted questions covering which direction resonates, whether to expand or narrow the scope, which aspects are critical against nice to have, and technical preferences where the drafts diverge. Answer with the questions.`

const mergePrompt = `The person's answers are in the answers file named below. Merge the three drafts into the final sprint document named below: identify consensus across the critiques and ideas that appear in more than one draft, compare the architecture, phasing, risks, definition of done, and the novel ideas unique to one draft, and take the answers as decisions. Write your synthesis to the merge notes file named below first, then the sprint document, in the planning style of the existing sprint documents, with a concise Pyramid Index. Answer with what you chose and why.`
