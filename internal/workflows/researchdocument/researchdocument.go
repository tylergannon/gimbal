// Package researchdocument researches a subject into a local semantic index,
// writes a size-bounded document, and revises it until an independent editor
// finds only nitpicks.
//
// The planner returns exactly five coherent groups of adjacent or related
// topics, one for each explicitly named parallel researcher. Each topic must
// leave at least the requested number of useful local source files, address
// every assigned question or mark it unresolved, and write an INDEX.md that
// cites those sources and links to longer clips when compression would lose
// important detail. The index is a means to the document, not a completeness
// project.
//
// The author uses the combined semantic index as its entry point to the corpus.
// An editorial pass checks the document against the goal, index, and token
// budget. Material omissions trigger targeted research and index repair before
// another revision. The workflow succeeds when the document is within budget
// and the editor reports only nitpicks, or returns an error after the editorial
// round limit.
//
// Model defaults deliberately put broad collection on Gemini Flash and report
// synthesis on Gemini Pro. Research planning, parallel research and indexing,
// index curation, and supervision use Gemini 3.8 Flash at medium effort.
// Document authoring and independent editorial review use Gemini 3.1 Pro at
// high effort. The displayed role flags can still replace an individual pin.
//
// Example:
//
//	gimble run research-document \
//	  --goal "Explain passkeys to security-conscious product managers" \
//	  --research-dir ./passkeys-research \
//	  --output ./passkeys.md \
//	  --token-budget 4000
package researchdocument

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/polytype"
)

//go:generate go tool polytype --validate
//go:generate go run github.com/tylergannon/gimble/internal/generate/gimblegen -entry ResearchDocument -name research-document

const (
	roleResearchPlanning    gimble.WorkflowRole = "research-planning"
	roleResearchIndexing    gimble.WorkflowRole = "research-indexing"
	roleIndexCuration       gimble.WorkflowRole = "index-curation"
	roleDocumentAuthoring   gimble.WorkflowRole = "document-authoring"
	roleEditorialReview     gimble.WorkflowRole = "editorial-review"
	roleDocumentSupervision gimble.WorkflowRole = "document-supervision"
	defaultMinSources                           = 3
	defaultEditorialRounds                      = 3
)

// Params are the document goal, local outputs, and finite work limits.
type Params struct {
	// Goal describes the audience, subject, and understanding the document must produce.
	Goal string
	// ResearchDir receives downloaded sources, clips, topic indexes, and the combined INDEX.md.
	ResearchDir string
	// Output is the document file the author creates or revises.
	Output string
	// TokenBudget is the maximum o200k_base token count accepted for the document.
	TokenBudget int
	// MinSourcesPerTopic overrides the default minimum of three useful local source files per planned topic.
	MinSourcesPerTopic polytype.Optional[int]
	// MaxEditorialRounds overrides the default limit of three editorial passes.
	MaxEditorialRounds polytype.Optional[int]
}

// Topic is one bounded area of research and the questions it must address.
type Topic struct {
	Name      string   `json:"name"`
	Questions []string `json:"questions"`
}

// TopicGroup is one coherent assignment for a parallel researcher.
type TopicGroup struct {
	Name   string  `json:"name"`
	Topics []Topic `json:"topics"`
}

// ResearchPlan is exactly five adjacent or related groups of topics.
type ResearchPlan struct {
	Groups []TopicGroup `json:"groups"`
}

// ResearchResult summarizes the local evidence a researcher actually left.
type ResearchResult struct {
	Summary             string   `json:"summary"`
	SourceFiles         []string `json:"source_files"`
	QuestionsAddressed  []string `json:"questions_addressed"`
	UnresolvedQuestions []string `json:"unresolved_questions"`
}

// EditorialVerdict distinguishes material document failures from optional polish.
type EditorialVerdict struct {
	OnlyNitpicks   bool     `json:"only_nitpicks"`
	MaterialIssues []string `json:"material_issues"`
	MissingTopics  []string `json:"missing_topics"`
}

// ResearchDocument produces a research-backed document within a token budget.
func ResearchDocument(ctx context.Context, env gimble.Env, params Params) error {
	goal := strings.TrimSpace(params.Goal)
	if goal == "" {
		return fmt.Errorf("goal must not be blank")
	}
	if params.TokenBudget < 1 {
		return fmt.Errorf("token-budget must be at least 1")
	}
	minSources := defaultMinSources
	if params.MinSourcesPerTopic.Present {
		minSources = params.MinSourcesPerTopic.Value
	}
	if minSources < 1 {
		return fmt.Errorf("min-sources-per-topic must be at least 1")
	}
	maxRounds := defaultEditorialRounds
	if params.MaxEditorialRounds.Present {
		maxRounds = params.MaxEditorialRounds.Value
	}
	if maxRounds < 1 {
		return fmt.Errorf("max-editorial-rounds must be at least 1")
	}

	researchDir, err := absoluteFrom(env.WorkDir, params.ResearchDir)
	if err != nil {
		return fmt.Errorf("research-dir: %w", err)
	}
	documentPath, err := absoluteFrom(env.WorkDir, params.Output)
	if err != nil {
		return fmt.Errorf("output: %w", err)
	}
	indexPath := filepath.Join(researchDir, "INDEX.md")
	tokenCounter, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve token counter: %w", err)
	}
	if err := os.MkdirAll(researchDir, 0o755); err != nil {
		return fmt.Errorf("create research directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(documentPath), 0o755); err != nil {
		return fmt.Errorf("create document directory: %w", err)
	}

	gimble.Set(ctx, "document goal", goal)
	gimble.Set(ctx, "document path", documentPath)
	gimble.Set(ctx, "semantic index path", indexPath)
	gimble.Set(ctx, "research directory", researchDir)
	gimble.Set(ctx, "document token budget", params.TokenBudget)
	gimble.Set(ctx, "token counter executable", tokenCounter)
	gimble.Set(ctx, "minimum sources per topic", minSources)
	gimble.Set(ctx, "maximum editorial rounds", maxRounds)

	planner := gimble.NewSession(ctx, roleResearchPlanning, env.WorkDir)
	plan, err := planner.Generate[ResearchPlan](ctx, planTopicsPrompt)
	if err != nil {
		return err
	}
	if len(plan.Groups) != 5 {
		return fmt.Errorf("research planner returned %d topic groups, need exactly 5", len(plan.Groups))
	}
	gimble.SetJSON(ctx, "research plan", plan)

	groupDirs := make([][]string, len(plan.Groups))
	var topicIndexes []string
	topicNumber := 0
	for groupNumber, group := range plan.Groups {
		if strings.TrimSpace(group.Name) == "" || len(group.Topics) == 0 {
			return fmt.Errorf("research group %d needs a name and at least one topic", groupNumber+1)
		}
		for _, topic := range group.Topics {
			topicNumber++
			if strings.TrimSpace(topic.Name) == "" || len(topic.Questions) == 0 {
				return fmt.Errorf("research topic %d needs a name and at least one question", topicNumber)
			}
			topicDir := filepath.Join(researchDir, fmt.Sprintf("topic-%03d", topicNumber))
			if err := os.MkdirAll(filepath.Join(topicDir, "sources"), 0o755); err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Join(topicDir, "clips"), 0o755); err != nil {
				return err
			}
			groupDirs[groupNumber] = append(groupDirs[groupNumber], topicDir)
			topicIndexes = append(topicIndexes, filepath.Join(topicDir, "INDEX.md"))
		}
	}

	research := gimble.Group(ctx, "research")
	research.Go("agent1", func(ctx context.Context) error {
		gimble.SetJSON(ctx, "assigned topic group", plan.Groups[0])
		gimble.Set(ctx, "topic directories", groupDirs[0])
		gimble.Set(ctx, "minimum sources per assigned topic", minSources)
		researcher := gimble.NewSession(ctx, roleResearchIndexing, env.WorkDir)
		coach := gimble.NewSession(ctx, roleDocumentSupervision, env.WorkDir)
		result, err := researcher.Generate[ResearchResult](ctx, researchTopicsPrompt,
			gimble.WithSupervisor(coach, researchCoachPrompt))
		if err != nil {
			return err
		}
		gimble.SetJSON(ctx, "research result", result)
		for _, dir := range groupDirs[0] {
			if err := verifyResearchFloor(dir, minSources); err != nil {
				return err
			}
		}
		return nil
	})
	research.Go("agent2", func(ctx context.Context) error {
		gimble.SetJSON(ctx, "assigned topic group", plan.Groups[1])
		gimble.Set(ctx, "topic directories", groupDirs[1])
		gimble.Set(ctx, "minimum sources per assigned topic", minSources)
		researcher := gimble.NewSession(ctx, roleResearchIndexing, env.WorkDir)
		coach := gimble.NewSession(ctx, roleDocumentSupervision, env.WorkDir)
		result, err := researcher.Generate[ResearchResult](ctx, researchTopicsPrompt,
			gimble.WithSupervisor(coach, researchCoachPrompt))
		if err != nil {
			return err
		}
		gimble.SetJSON(ctx, "research result", result)
		for _, dir := range groupDirs[1] {
			if err := verifyResearchFloor(dir, minSources); err != nil {
				return err
			}
		}
		return nil
	})
	research.Go("agent3", func(ctx context.Context) error {
		gimble.SetJSON(ctx, "assigned topic group", plan.Groups[2])
		gimble.Set(ctx, "topic directories", groupDirs[2])
		gimble.Set(ctx, "minimum sources per assigned topic", minSources)
		researcher := gimble.NewSession(ctx, roleResearchIndexing, env.WorkDir)
		coach := gimble.NewSession(ctx, roleDocumentSupervision, env.WorkDir)
		result, err := researcher.Generate[ResearchResult](ctx, researchTopicsPrompt,
			gimble.WithSupervisor(coach, researchCoachPrompt))
		if err != nil {
			return err
		}
		gimble.SetJSON(ctx, "research result", result)
		for _, dir := range groupDirs[2] {
			if err := verifyResearchFloor(dir, minSources); err != nil {
				return err
			}
		}
		return nil
	})
	research.Go("agent4", func(ctx context.Context) error {
		gimble.SetJSON(ctx, "assigned topic group", plan.Groups[3])
		gimble.Set(ctx, "topic directories", groupDirs[3])
		gimble.Set(ctx, "minimum sources per assigned topic", minSources)
		researcher := gimble.NewSession(ctx, roleResearchIndexing, env.WorkDir)
		coach := gimble.NewSession(ctx, roleDocumentSupervision, env.WorkDir)
		result, err := researcher.Generate[ResearchResult](ctx, researchTopicsPrompt,
			gimble.WithSupervisor(coach, researchCoachPrompt))
		if err != nil {
			return err
		}
		gimble.SetJSON(ctx, "research result", result)
		for _, dir := range groupDirs[3] {
			if err := verifyResearchFloor(dir, minSources); err != nil {
				return err
			}
		}
		return nil
	})
	research.Go("agent5", func(ctx context.Context) error {
		gimble.SetJSON(ctx, "assigned topic group", plan.Groups[4])
		gimble.Set(ctx, "topic directories", groupDirs[4])
		gimble.Set(ctx, "minimum sources per assigned topic", minSources)
		researcher := gimble.NewSession(ctx, roleResearchIndexing, env.WorkDir)
		coach := gimble.NewSession(ctx, roleDocumentSupervision, env.WorkDir)
		result, err := researcher.Generate[ResearchResult](ctx, researchTopicsPrompt,
			gimble.WithSupervisor(coach, researchCoachPrompt))
		if err != nil {
			return err
		}
		gimble.SetJSON(ctx, "research result", result)
		for _, dir := range groupDirs[4] {
			if err := verifyResearchFloor(dir, minSources); err != nil {
				return err
			}
		}
		return nil
	})
	if err := research.Wait(); err != nil {
		return err
	}
	gimble.Set(ctx, "topic index paths", topicIndexes)

	curator := gimble.NewSession(ctx, roleIndexCuration, env.WorkDir)
	indexCoach := gimble.NewSession(ctx, roleDocumentSupervision, env.WorkDir)
	if _, err := curator.Generate[gimble.Text](ctx, buildIndexPrompt,
		gimble.WithSupervisor(indexCoach, researchCoachPrompt)); err != nil {
		return err
	}
	if err := requireNonemptyFile(indexPath); err != nil {
		return fmt.Errorf("semantic index: %w", err)
	}

	author := gimble.NewSession(ctx, roleDocumentAuthoring, env.WorkDir)
	compressionCoach := gimble.NewSession(ctx, roleDocumentSupervision, env.WorkDir)
	if _, err := author.Generate[gimble.Text](ctx, writeDocumentPrompt,
		gimble.WithSupervisor(compressionCoach, compressionCoachPrompt)); err != nil {
		return err
	}
	if err := requireNonemptyFile(documentPath); err != nil {
		return fmt.Errorf("document: %w", err)
	}

	editor := gimble.NewSession(ctx, roleEditorialReview, env.WorkDir)
	for round := 1; round <= maxRounds; round++ {
		accepted := false
		err := gimble.Scope(ctx, "editorial-round", func(ctx context.Context) error {
			gimble.Set(ctx, "editorial round", round)
			exit, stdout, stderr, err := gimble.RunCommand(ctx, "count-tokens", env.WorkDir, tokenCounter, "count-tokens", documentPath)
			if err != nil {
				return err
			}
			if exit != 0 {
				return fmt.Errorf("count document tokens: %s", strings.TrimSpace(stderr))
			}
			tokens, err := strconv.Atoi(strings.TrimSpace(stdout))
			if err != nil {
				return fmt.Errorf("parse document token count %q: %w", strings.TrimSpace(stdout), err)
			}
			gimble.Set(ctx, "document token count", tokens)

			verdict, err := editor.Generate[EditorialVerdict](ctx, editorialPrompt)
			if err != nil {
				return err
			}
			gimble.SetJSON(ctx, "editorial verdict", verdict)
			if tokens <= params.TokenBudget && verdict.OnlyNitpicks && len(verdict.MaterialIssues) == 0 && len(verdict.MissingTopics) == 0 {
				accepted = true
				return nil
			}

			if len(verdict.MissingTopics) > 0 {
				gapDir := filepath.Join(researchDir, fmt.Sprintf("editorial-gap-%02d", round))
				if err := os.MkdirAll(filepath.Join(gapDir, "sources"), 0o755); err != nil {
					return err
				}
				if err := os.MkdirAll(filepath.Join(gapDir, "clips"), 0o755); err != nil {
					return err
				}
				gapIndex := filepath.Join(gapDir, "INDEX.md")
				requiredSources := minSources * len(verdict.MissingTopics)
				gimble.Set(ctx, "missing topics", verdict.MissingTopics)
				gimble.Set(ctx, "gap research directory", gapDir)
				gimble.Set(ctx, "gap research index path", gapIndex)
				gimble.Set(ctx, "minimum gap source files", requiredSources)

				gapResearcher := gimble.NewSession(ctx, roleResearchIndexing, env.WorkDir)
				gapCoach := gimble.NewSession(ctx, roleDocumentSupervision, env.WorkDir)
				result, err := gapResearcher.Generate[ResearchResult](ctx, researchGapsPrompt,
					gimble.WithSupervisor(gapCoach, researchCoachPrompt))
				if err != nil {
					return err
				}
				gimble.SetJSON(ctx, "gap research result", result)
				if err := verifyResearchFloor(gapDir, requiredSources); err != nil {
					return err
				}
				if _, err := curator.Generate[gimble.Text](ctx, updateIndexPrompt,
					gimble.WithSupervisor(indexCoach, researchCoachPrompt)); err != nil {
					return err
				}
			}

			if _, err := author.Generate[gimble.Text](ctx, reviseDocumentPrompt,
				gimble.WithSupervisor(compressionCoach, compressionCoachPrompt)); err != nil {
				return err
			}
			return requireNonemptyFile(documentPath)
		})
		if err != nil {
			return err
		}
		if accepted {
			return nil
		}
	}

	return fmt.Errorf("document still has material editorial defects after %d rounds", maxRounds)
}

func absoluteFrom(workDir, name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("path must not be blank")
	}
	if !filepath.IsAbs(name) {
		name = filepath.Join(workDir, name)
	}
	return filepath.Abs(name)
}

func verifyResearchFloor(dir string, minimum int) error {
	sources := 0
	err := filepath.WalkDir(filepath.Join(dir, "sources"), func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			sources++
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("inspect research sources: %w", err)
	}
	if sources < minimum {
		return fmt.Errorf("research floor not met in %s: found %d source files, need at least %d", dir, sources, minimum)
	}
	if err := requireNonemptyFile(filepath.Join(dir, "INDEX.md")); err != nil {
		return fmt.Errorf("research floor not met in %s: %w", dir, err)
	}
	return nil
}

func requireNonemptyFile(name string) error {
	info, err := os.Stat(name)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return fmt.Errorf("%s is not a nonempty regular file", name)
	}
	return nil
}

const planTopicsPrompt = `Break the research needed for the document into exactly five groups of adjacent or related topics, one group for each parallel researcher. Every group must contain at least one topic. For each topic, give the specific questions its researcher must answer. Cover what the audience needs for the requested understanding; do not plan globally comprehensive research.`

const researchTopicsPrompt = `Research every assigned topic for the final document. The topic directories align with the assigned topics in the same order. For each topic, download at least the required number of useful sources into that topic directory's sources subdirectory. Address every assigned question or explicitly mark it unresolved. Write INDEX.md in each topic directory with a concise goal-relevant synthesis, citations to the local source files, and links to longer clips in its clips subdirectory when summarizing would lose important detail. Return the files and questions you actually addressed. Stop once this definition of done is met for every assigned topic.`

const researchGapsPrompt = `Research the missing topics identified by the editor. Download at least the required number of useful sources into the gap research sources directory, using the same minimum per missing topic. Write INDEX.md at the exact gap research index path with a concise goal-relevant synthesis, local citations, and links to indispensable longer clips. Return what you actually researched. Stop when the gaps can support the document.`

const researchCoachPrompt = `The goal is the best possible document within its token budget, not a perfect corpus or index. Ensure the worker meets the stated research floor, preserves useful evidence locally, and makes the index sufficient for writing. Object to work beyond what would materially improve the document.`

const compressionCoachPrompt = `The goal is the most important understanding the token budget can hold. Ensure the author actively measures the document with the supplied token counter and edits it under budget. Object when low-value specifics, examples, qualifications, or secondary claims displace the central explanation.`

const buildIndexPrompt = `Build the semantic index at the exact semantic index path from the topic indexes. Organize it for the document goal: preserve the important knowledge, cite local evidence, and link to longer clips when compression would lose something the author may need. The index should be the author's sufficient entry point into the corpus. Stop when it supports writing the document; do not perfect it for its own sake.`

const updateIndexPrompt = `Update the semantic index with the gap research identified in this editorial round. Preserve its usefulness as the author's single entry point, add the important missing knowledge and local citations, and stop when the document can be repaired.`

const writeDocumentPrompt = `Write the requested document at the exact document path. Use the semantic index as the only entry point to the research; follow its links to longer local clips only when needed. Maximize the audience's understanding of the most important knowledge within the token budget. Prefer strong lossy compression over accumulating facts. Do not browse the corpus independently or spend work improving the index. Before finishing, repeatedly run the token counter executable with "count-tokens" and the document path, editing the document until the reported count is at or below the budget.`

const editorialPrompt = `Independently read the document and semantic index at their exact paths. Assess whether the document achieves its goal within the token budget. OnlyNitpicks is true only when nothing important is absent, misleading, badly prioritized, or wasteful enough to justify another revision. Put authoring or compression defects in MaterialIssues. Put a topic in MissingTopics only when the semantic index lacks the knowledge needed to repair the document.`

const reviseDocumentPrompt = `Revise the document at its exact path using the editorial verdict, measured token count, and current semantic index. Fix every material issue. Protect the most important understanding; remove lower-value specifics, examples, qualifications, and secondary claims before weakening the central explanation. Before finishing, repeatedly run the token counter executable with "count-tokens" and the document path, editing until the reported count is at or below the budget.`
