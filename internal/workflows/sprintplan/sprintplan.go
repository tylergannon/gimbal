// Package sprintplan turns an accepted value proposition into a proposed plan
// supported by a local, indexed working set. Supply an intent file naming the
// stakeholder, desired outcome, boundaries, and observable success. Include
// paths to useful local references or existing indexes in that file. Relative
// reference paths are interpreted from the project directory.
//
// Project and prior-art research run together, followed by index curation,
// three independent drafts, cross-critiques, and synthesis. Researchers collect
// information without recommending a solution. Project sources stay in place;
// external material is collected locally when useful. Research follows the
// intent's source restrictions, with no fixed source count or local/web mode.
//
// A new sprint directory receives intent.md, working-set/INDEX.md and its
// sources, draft/ and critique/ documents, and plan.md. Existing sprint
// directories are refused. Success means these planning artifacts exist, not
// that the stakeholder outcome has been implemented or validated. Unresolved
// stakeholder choices remain explicit in the plan for human review.
//
// The three planning lanes default to Claude, Codex, and Gemini; role flags can
// override them. Planning reads project sources without changing them. Only
// research and synthesis maintain the working set; parallel planners return
// discoveries in their own documents.
//
// Example:
//
//	gimbal run sprint-plan --intent ./intent.md --sprint-dir ./ephemeral/sprints/export
package sprintplan

import (
	"context"
	"path/filepath"

	"github.com/tylergannon/gimbal"
)

//go:generate go run github.com/tylergannon/gimbal/internal/generate/gimbalgen -entry SprintPlan -name sprint-plan

// Params identify the accepted intent and the new sprint's local working files.
type Params struct {
	// Intent is a local file describing who benefits, the outcome, boundaries, success evidence, and useful reference paths.
	Intent string
	// SprintDir is a new directory for the intent, indexed working set, drafts, critiques, and proposed plan.
	SprintDir string
}

// SprintPlan prepares an indexed working set and a reference-backed implementation plan.
func SprintPlan(ctx context.Context, env gimbal.Env, params Params) error {
	dir, err := prepare(env.WorkDir, params)
	if err != nil {
		return err
	}
	gimbal.Set(ctx, "intent file", filepath.Join(dir, "intent.md"))
	gimbal.Set(ctx, "working set", filepath.Join(dir, "working-set"))
	gimbal.Set(ctx, "project directory", env.WorkDir)
	gimbal.Set(ctx, "planning rules", "Read the intent first. Keep its stakeholder outcome authoritative. Read project instructions; leave project sources unchanged. Resolve relative references from the project directory; use absolute local citations and original evidence. Preserve uncertainty and surface missing stakeholder decisions rather than inventing them.")

	gimbal.Set(ctx, "document output", "Write the requested Markdown document at its supplied path. Return a short summary; the workflow checks the file and does not save your response over it.")

	research := gimbal.Group(ctx, "research")
	research.Go("project", func(ctx context.Context) error {
		gimbal.Set(ctx, "document path", filepath.Join(dir, "working-set", "project", "INDEX.md"))
		reader := gimbal.NewSession(ctx, "research-indexing", env.WorkDir)
		_, err := reader.Generate[gimbal.Text](ctx, `Locate project code, tests, documentation, and existing knowledge that might help realize the intent. Omit your own opinions and solution judgments. Write a compact index at the document path, routing questions to precise local source references for later decisions. Include adjacent material when potentially useful. Reference existing files in place; preserve qualifications, disagreements, and open questions.`)
		return checkArtifact(filepath.Join(dir, "working-set", "project", "INDEX.md"), err)
	})
	research.Go("prior-art", func(ctx context.Context) error {
		gimbal.Set(ctx, "collection directory", filepath.Join(dir, "working-set", "prior-art"))
		gimbal.Set(ctx, "document path", filepath.Join(dir, "working-set", "prior-art", "INDEX.md"))
		reader := gimbal.NewSession(ctx, "research-indexing", env.WorkDir)
		_, err := reader.Generate[gimbal.Text](ctx, `Locate prior art, examples, and reference material that might help realize the intent, including adjacent approaches. Follow supplied local references and source restrictions; discover additional sources when useful. Save collected originals or faithful excerpts with provenance in the collection directory. Omit your own opinions and solution judgments. Write a compact index at the document path with precise local references, qualifications, and unresolved questions for later planning.`)
		return checkArtifact(filepath.Join(dir, "working-set", "prior-art", "INDEX.md"), err)
	})
	if err := research.Wait(); err != nil {
		return err
	}
	gimbal.Set(ctx, "root index path", filepath.Join(dir, "working-set", "INDEX.md"))
	curator := gimbal.NewSession(ctx, "index-curation", env.WorkDir)
	_, err = curator.Generate[gimbal.Text](ctx, `Read both research indexes and check their cited local sources. Write a compact index at the root index path organized around likely planning and implementation questions, linking to those indexes and original material. Explain where to look without recommending a solution. Preserve uncertainty and disagreements; a little extra relevant material is useful. Verify representative routes reach useful evidence.`)
	if err := checkArtifact(filepath.Join(dir, "working-set", "INDEX.md"), err); err != nil {
		return err
	}

	for ctx, phase := range gimbal.Iterate(ctx, "planning", []string{"draft", "critique"}) {
		gimbal.Set(ctx, "phase", phase)
		gimbal.Set(ctx, "draft directory", filepath.Join(dir, "draft"))
		const instruction = `Read the intent and retrieve relevant evidence through INDEX.md in the supplied working set. In draft phase, independently propose ordered implementation work with useful source references, reuse opportunities, dependencies, and observable acceptance; do not read other drafts. In critique phase, read the other two lanes' drafts and assess them against the intent and original evidence for missed value, unnecessary invention, and weak acceptance. Write your document at the document path, including discoveries and unresolved decisions. Leave the working set and other lanes' documents unchanged.`
		plans := gimbal.Group(ctx, "plans")
		plans.Go("claude", func(ctx context.Context) error {
			gimbal.Set(ctx, "lane", "claude")
			gimbal.Set(ctx, "document path", filepath.Join(dir, phase, "claude.md"))
			planner := gimbal.NewSession(ctx, "sprint-plan-claude", env.WorkDir)
			_, err := planner.Generate[gimbal.Text](ctx, instruction)
			return checkArtifact(filepath.Join(dir, phase, "claude.md"), err)
		})
		plans.Go("codex", func(ctx context.Context) error {
			gimbal.Set(ctx, "lane", "codex")
			gimbal.Set(ctx, "document path", filepath.Join(dir, phase, "codex.md"))
			planner := gimbal.NewSession(ctx, "sprint-plan-codex", env.WorkDir)
			_, err := planner.Generate[gimbal.Text](ctx, instruction)
			return checkArtifact(filepath.Join(dir, phase, "codex.md"), err)
		})
		plans.Go("gemini", func(ctx context.Context) error {
			gimbal.Set(ctx, "lane", "gemini")
			gimbal.Set(ctx, "document path", filepath.Join(dir, phase, "gemini.md"))
			planner := gimbal.NewSession(ctx, "sprint-plan-gemini", env.WorkDir)
			_, err := planner.Generate[gimbal.Text](ctx, instruction)
			return checkArtifact(filepath.Join(dir, phase, "gemini.md"), err)
		})
		if err := plans.Wait(); err != nil {
			return err
		}
	}
	gimbal.Set(ctx, "sprint directory", dir)
	gimbal.Set(ctx, "plan path", filepath.Join(dir, "plan.md"))
	synthesizer := gimbal.NewSession(ctx, gimbal.RoleSprintPlanning, env.WorkDir)
	_, err = synthesizer.Generate[gimbal.Text](ctx, `Read the intent, working set, all drafts, and all critiques. Synthesize the simplest sound route to the stakeholder outcome. Write a proposed plan at the plan path with ordered work, dependencies, relevant local references and reuse opportunities, observable acceptance, consequential choices, and unresolved decisions. Incorporate source-supported discoveries and corrections into the working set, keeping recommendations in the plan. Make the handoff usable by a fresh implementation agent; do not implement or claim completion.`)
	return checkArtifact(filepath.Join(dir, "plan.md"), err)
}
