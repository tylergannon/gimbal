// Package researchdocument collects source information into a local semantic
// index and compresses it into a factual document within a token budget.
//
// The planner returns exactly five coherent groups of adjacent or related
// topics, one for each explicitly named parallel researcher. Each topic must
// preserve at least the requested number of useful local sources, distinguish
// originals and their provenance, and address each question or mark it
// unresolved. Topic indexes and a compact combined index route reader questions
// to precise source passages and verbatim excerpts. The index is a means
// to finding evidence for the document, not a second report.
//
// Collection, indexing, and document writing select, organize, summarize, and
// compress what sources state. They do not add opinions, recommendations,
// deductions, proposed designs, or judgments about which approach is better.
// Source attribution, qualifications, and conflicting accounts remain visible.
// The caller's goal sets the subject and scope, not permission to add analysis.
//
// The author uses the combined semantic index as its entry point to the corpus.
// An editorial pass checks consequential claims against original evidence,
// along with the document goal and token budget. Material omissions trigger
// targeted research and index repair before another revision. The workflow succeeds when the document is within budget
// and the editor reports only nitpicks, or returns an error after the editorial
// round limit.
//
// Before authoring, an independent reader checks the index summaries against
// original passages and walks its routes. Concrete defects go back to the
// curator for bounded repair. The checks are focused source comparisons and retrieval walks.
//
// Model defaults deliberately put broad collection on Gemini Flash, combined
// index curation on Claude Sonnet 5.5, and factual compression on Gemini Pro.
// Research planning and parallel research and indexing use
// Gemini 3.8 Flash at medium effort.
// Document authoring and independent editorial review use Gemini 3.1 Pro at
// high effort. The displayed role flags can still replace an individual pin.
//
// Example:
//
//	gimbal run research-document \
//	  --goal "Explain passkeys to security-conscious product managers" \
//	  --research-dir ./passkeys-research \
//	  --output ./passkeys.md \
//	  --token-budget 4000
package researchdocument

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/polytype"
)

//go:generate go tool polytype --validate
//go:generate go run github.com/tylergannon/gimbal/internal/generate/gimbalgen -entry ResearchDocument -name research-document

const (
	roleResearchPlanning   gimbal.WorkflowRole = "research-planning"
	roleResearchIndexing   gimbal.WorkflowRole = "research-indexing"
	roleIndexCuration      gimbal.WorkflowRole = "index-curation"
	roleDocumentAuthoring  gimbal.WorkflowRole = "document-authoring"
	roleEditorialReview    gimbal.WorkflowRole = "editorial-review"
	defaultMinSources                          = 3
	defaultEditorialRounds                     = 3
)

// Params are the document goal, local outputs, and finite work limits.
type Params struct {
	// Goal describes the subject, scope, and audience for factual research; it does not authorize recommendations or designs.
	Goal string
	// ResearchDir receives downloaded sources, clips, topic indexes, and the combined INDEX.md.
	ResearchDir string
	// SourceDir optionally supplies a fixed local corpus; researchers index these originals without web discovery.
	SourceDir polytype.Optional[string]
	// Output is the document file the author creates or revises.
	Output string
	// TokenBudget is the maximum o200k_base token count accepted for the document.
	TokenBudget int
	// MinSourcesPerTopic overrides the default minimum of three useful local source files per planned topic.
	MinSourcesPerTopic polytype.Optional[int]
	// MaxEditorialRounds bounds index and document review passes separately; default three.
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

// ResearchDocument produces a factual research document within a token budget.
func ResearchDocument(ctx context.Context, env gimbal.Env, params Params) error {
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
	if documentPath == researchDir || strings.HasPrefix(documentPath, researchDir+string(filepath.Separator)) {
		return fmt.Errorf("output must be outside research-dir so the document is not treated as research input")
	}
	indexPath := filepath.Join(researchDir, "INDEX.md")
	var sourceDir, sharedSources string
	if params.SourceDir.Present {
		sourceDir, err = absoluteFrom(env.WorkDir, params.SourceDir.Value)
		if err != nil {
			return fmt.Errorf("source-dir: %w", err)
		}
		if sourceDir == researchDir || strings.HasPrefix(sourceDir, researchDir+string(filepath.Separator)) {
			return fmt.Errorf("source-dir must be outside research-dir")
		}
	}
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

	gimbal.Set(ctx, "document goal", goal)
	gimbal.Set(ctx, "document path", documentPath)
	gimbal.Set(ctx, "semantic index path", indexPath)
	gimbal.Set(ctx, "research directory", researchDir)
	sourceMode := "web"
	if sourceDir != "" {
		sourceMode = "fixed"
		sharedSources = filepath.Join(researchDir, "sources")
		if err := copyOriginals(sourceDir, sharedSources); err != nil {
			return err
		}
	}
	gimbal.Set(ctx, "source mode", sourceMode)
	gimbal.Set(ctx, "fixed originals directory", sharedSources)
	gimbal.Set(ctx, "document token budget", params.TokenBudget)
	gimbal.Set(ctx, "token counter executable", tokenCounter)
	gimbal.Set(ctx, "minimum sources per topic", minSources)
	gimbal.Set(ctx, "maximum editorial rounds", maxRounds)

	planner := gimbal.NewSession(ctx, roleResearchPlanning, env.WorkDir)
	plan, err := planner.Generate[ResearchPlan](ctx, planTopicsPrompt, gimbal.WithScopeTemplate(planContext))
	if err != nil {
		return err
	}
	if len(plan.Groups) != 5 {
		return fmt.Errorf("research planner returned %d topic groups, need exactly 5", len(plan.Groups))
	}
	gimbal.SetJSON(ctx, "research plan", plan)

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

	research := gimbal.Group(ctx, "research")
	research.Go("agent1", func(ctx context.Context) error {
		gimbal.SetJSON(ctx, "assigned topic group", plan.Groups[0])
		gimbal.Set(ctx, "topic directories", groupDirs[0])
		gimbal.Set(ctx, "minimum sources per assigned topic", minSources)
		researcher := gimbal.NewSession(ctx, roleResearchIndexing, researchDir)
		result, err := researcher.Generate[ResearchResult](ctx, researchTopicsPrompt,
			gimbal.WithScopeTemplate(researchContext))
		if err != nil {
			return err
		}
		gimbal.SetJSON(ctx, "research result", result)
		for _, dir := range groupDirs[0] {
			if err := verifyResearchFloor(dir, sharedSources, minSources); err != nil {
				return err
			}
		}
		return nil
	})
	research.Go("agent2", func(ctx context.Context) error {
		gimbal.SetJSON(ctx, "assigned topic group", plan.Groups[1])
		gimbal.Set(ctx, "topic directories", groupDirs[1])
		gimbal.Set(ctx, "minimum sources per assigned topic", minSources)
		researcher := gimbal.NewSession(ctx, roleResearchIndexing, researchDir)
		result, err := researcher.Generate[ResearchResult](ctx, researchTopicsPrompt,
			gimbal.WithScopeTemplate(researchContext))
		if err != nil {
			return err
		}
		gimbal.SetJSON(ctx, "research result", result)
		for _, dir := range groupDirs[1] {
			if err := verifyResearchFloor(dir, sharedSources, minSources); err != nil {
				return err
			}
		}
		return nil
	})
	research.Go("agent3", func(ctx context.Context) error {
		gimbal.SetJSON(ctx, "assigned topic group", plan.Groups[2])
		gimbal.Set(ctx, "topic directories", groupDirs[2])
		gimbal.Set(ctx, "minimum sources per assigned topic", minSources)
		researcher := gimbal.NewSession(ctx, roleResearchIndexing, researchDir)
		result, err := researcher.Generate[ResearchResult](ctx, researchTopicsPrompt,
			gimbal.WithScopeTemplate(researchContext))
		if err != nil {
			return err
		}
		gimbal.SetJSON(ctx, "research result", result)
		for _, dir := range groupDirs[2] {
			if err := verifyResearchFloor(dir, sharedSources, minSources); err != nil {
				return err
			}
		}
		return nil
	})
	research.Go("agent4", func(ctx context.Context) error {
		gimbal.SetJSON(ctx, "assigned topic group", plan.Groups[3])
		gimbal.Set(ctx, "topic directories", groupDirs[3])
		gimbal.Set(ctx, "minimum sources per assigned topic", minSources)
		researcher := gimbal.NewSession(ctx, roleResearchIndexing, researchDir)
		result, err := researcher.Generate[ResearchResult](ctx, researchTopicsPrompt,
			gimbal.WithScopeTemplate(researchContext))
		if err != nil {
			return err
		}
		gimbal.SetJSON(ctx, "research result", result)
		for _, dir := range groupDirs[3] {
			if err := verifyResearchFloor(dir, sharedSources, minSources); err != nil {
				return err
			}
		}
		return nil
	})
	research.Go("agent5", func(ctx context.Context) error {
		gimbal.SetJSON(ctx, "assigned topic group", plan.Groups[4])
		gimbal.Set(ctx, "topic directories", groupDirs[4])
		gimbal.Set(ctx, "minimum sources per assigned topic", minSources)
		researcher := gimbal.NewSession(ctx, roleResearchIndexing, researchDir)
		result, err := researcher.Generate[ResearchResult](ctx, researchTopicsPrompt,
			gimbal.WithScopeTemplate(researchContext))
		if err != nil {
			return err
		}
		gimbal.SetJSON(ctx, "research result", result)
		for _, dir := range groupDirs[4] {
			if err := verifyResearchFloor(dir, sharedSources, minSources); err != nil {
				return err
			}
		}
		return nil
	})
	if err := research.Wait(); err != nil {
		return err
	}
	gimbal.Set(ctx, "topic index paths", topicIndexes)

	curator := gimbal.NewSession(ctx, roleIndexCuration, researchDir)
	if _, err := curator.Generate[gimbal.Text](ctx, buildIndexPrompt,
		gimbal.WithScopeTemplate(indexContext)); err != nil {
		return err
	}
	if err := requireNonemptyFile(indexPath); err != nil {
		return fmt.Errorf("semantic index: %w", err)
	}
	editor := gimbal.NewSession(ctx, roleEditorialReview, researchDir)
	indexAccepted := false
	for round := 1; round <= maxRounds; round++ {
		err := gimbal.Scope(ctx, "index-review", func(ctx context.Context) error {
			structureError := ""
			if err := verifyIndex(researchDir, topicIndexes); err != nil {
				structureError = err.Error()
			}
			gimbal.Set(ctx, "index structure problems", structureError)
			verdict, err := editor.Generate[EditorialVerdict](ctx, reviewIndexPrompt, gimbal.WithScopeTemplate(indexReviewContext))
			if err != nil {
				return err
			}
			gimbal.SetJSON(ctx, "index verdict", verdict)
			if structureError == "" && verdict.OnlyNitpicks && len(verdict.MaterialIssues) == 0 && len(verdict.MissingTopics) == 0 {
				indexAccepted = true
				return nil
			}
			if round == maxRounds {
				return fmt.Errorf("index still has material defects after %d reviews: %s; %v; %v", maxRounds, structureError, verdict.MaterialIssues, verdict.MissingTopics)
			}
			_, err = curator.Generate[gimbal.Text](ctx, repairIndexPrompt, gimbal.WithScopeTemplate(indexReviewContext))
			return err
		})
		if err != nil {
			return err
		}
		if indexAccepted {
			break
		}
	}

	author := gimbal.NewSession(ctx, roleDocumentAuthoring, researchDir)
	if _, err := author.Generate[gimbal.Text](ctx, writeDocumentPrompt,
		gimbal.WithScopeTemplate(documentContext)); err != nil {
		return err
	}
	if err := requireNonemptyFile(documentPath); err != nil {
		return fmt.Errorf("document: %w", err)
	}

	for round := 1; round <= maxRounds; round++ {
		accepted := false
		err := gimbal.Scope(ctx, "editorial-round", func(ctx context.Context) error {
			gimbal.Set(ctx, "editorial round", round)
			exit, stdout, stderr, err := gimbal.RunCommand(ctx, "count-tokens", env.WorkDir, tokenCounter, "count-tokens", documentPath)
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
			gimbal.Set(ctx, "document token count", tokens)

			verdict, err := editor.Generate[EditorialVerdict](ctx, editorialPrompt, gimbal.WithScopeTemplate(documentContext))
			if err != nil {
				return err
			}
			gimbal.SetJSON(ctx, "editorial verdict", verdict)
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
				gimbal.Set(ctx, "missing topics", verdict.MissingTopics)
				gimbal.Set(ctx, "gap research directory", gapDir)
				gimbal.Set(ctx, "gap research index path", gapIndex)
				gimbal.Set(ctx, "minimum gap source files", requiredSources)

				gapResearcher := gimbal.NewSession(ctx, roleResearchIndexing, researchDir)
				result, err := gapResearcher.Generate[ResearchResult](ctx, researchGapsPrompt,
					gimbal.WithScopeTemplate(gapResearchContext))
				if err != nil {
					return err
				}
				gimbal.SetJSON(ctx, "gap research result", result)
				if err := verifyResearchFloor(gapDir, sharedSources, requiredSources); err != nil {
					return err
				}
				if _, err := curator.Generate[gimbal.Text](ctx, updateIndexPrompt); err != nil {
					return err
				}
				for review := 1; review <= maxRounds; review++ {
					gapAccepted := false
					err := gimbal.Scope(ctx, "gap-index-review", func(ctx context.Context) error {
						structureError := ""
						if err := verifyIndex(researchDir, append(topicIndexes, gapIndex)); err != nil {
							structureError = err.Error()
						}
						gimbal.Set(ctx, "index structure problems", structureError)
						verdict, err := editor.Generate[EditorialVerdict](ctx, reviewIndexPrompt, gimbal.WithScopeTemplate(indexReviewContext))
						if err != nil {
							return err
						}
						gimbal.SetJSON(ctx, "index verdict", verdict)
						if structureError == "" && verdict.OnlyNitpicks && len(verdict.MaterialIssues) == 0 && len(verdict.MissingTopics) == 0 {
							gapAccepted = true
							return nil
						}
						if review == maxRounds {
							return fmt.Errorf("gap index still has material defects: %s; %v; %v", structureError, verdict.MaterialIssues, verdict.MissingTopics)
						}
						_, err = curator.Generate[gimbal.Text](ctx, repairIndexPrompt, gimbal.WithScopeTemplate(indexReviewContext))
						return err
					})
					if err != nil {
						return err
					}
					if gapAccepted {
						break
					}
				}

			}

			if _, err := author.Generate[gimbal.Text](ctx, reviseDocumentPrompt,
				gimbal.WithScopeTemplate(documentContext)); err != nil {
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

func verifyResearchFloor(dir, sharedSources string, minimum int) error {
	sources := 0
	sourceRoot := filepath.Join(dir, "sources")
	if sharedSources != "" {
		sourceRoot = sharedSources
	}
	err := filepath.WalkDir(sourceRoot, func(_ string, entry fs.DirEntry, err error) error {
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

func copyOriginals(sourceDir, target string) error {
	root, err := filepath.Abs(sourceDir)
	if err != nil {
		return err
	}
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("fixed source corpus contains symlink %s", path)
		}
		dest := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		out, err := os.Create(dest)
		if err != nil {
			_ = in.Close()
			return err
		}
		_, copyErr := io.Copy(out, in)
		inErr := in.Close()
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if inErr != nil {
			return inErr
		}
		return closeErr
	})
}

const factualResearchContract = `The research artifacts contain factual source information only. The source cache holds downloaded originals or mechanically extracted source text, one identifiable URL or document per file, with precise provenance. Never put model-authored reconstructions or combined summaries in the source cache; summaries belong in index leaves. Select, organize, summarize, and compress what sources state, preserving attribution, scope, qualifications, and uncertainty. Every substantive statement must faithfully summarize identifiable source material. Grouping and links help readers understand and navigate it. Do not add opinions, recommendations, deductions, proposed designs, or judgments about which approach is better, even if the goal asks for them. Report conflicting source accounts with their citations without settling or reconciling them.`

const planContext = `## document goal
{{(index .By "document goal").Text}}

## source mode
{{(index .By "source mode").Text}}

## minimum sources per topic
{{(index .By "minimum sources per topic").Text}}`

const researchContext = `## document goal
{{(index .By "document goal").Text}}

## source mode
{{(index .By "source mode").Text}}

## fixed originals directory
{{(index .By "fixed originals directory").Text}}

## assigned topic group
{{(index .By "assigned topic group").Text}}

## topic directories
{{(index .By "topic directories").Text}}

## minimum sources per assigned topic
{{(index .By "minimum sources per assigned topic").Text}}`

const gapResearchContext = `## document goal
{{(index .By "document goal").Text}}

## source mode
{{(index .By "source mode").Text}}

## fixed originals directory
{{(index .By "fixed originals directory").Text}}

## missing topics
{{(index .By "missing topics").Text}}

## gap research directory
{{(index .By "gap research directory").Text}}

## gap research index path
{{(index .By "gap research index path").Text}}

## minimum gap source files
{{(index .By "minimum gap source files").Text}}`

const indexContext = `## document goal
{{(index .By "document goal").Text}}

## semantic index path
{{(index .By "semantic index path").Text}}

## topic index paths
{{(index .By "topic index paths").Text}}

## research directory
{{(index .By "research directory").Text}}`

const planTopicsPrompt = factualResearchContract + `

Plan factual research for the supplied subject and scope in exactly five coherent groups, one per parallel researcher, with at least one topic per group. Give each topic specific, neutral questions about source-documented capabilities, prior art, or techniques. Do not presuppose that particular capabilities, integrations, or implementations exist. Keep assignments distinct and proportional to the requested document. Preserve the caller's subject and settled requirements; do not plan evaluations of which approach is best, engineering proposals, or recommendations.`

const researchTopicsPrompt = factualResearchContract + `

Collect original evidence for every assigned topic. The topic directories align with the topics in the same order. Read and write only your assigned topic directories and original source material; generated research reports and Gimbal run records are outside your assignment. In fixed source mode, use only the shared fixed originals directory and reference its files without copying them into topics; do not browse or add sources. In web mode, download at least the required number of useful originals into each topic's sources directory. Preserve original source text, origin, version or retrieval date, and precise source locations.

Write a compact INDEX.md for each assigned topic: neutral routes and short faithful source summaries with Markdown links to original source passages using #L10-L20 line spans. Link local originals directly, not clips or other summaries as factual evidence. Keep it to a few hundred words, with the information needed to understand the sources and navigate them. Clips contain verbatim excerpts with provenance and exact source locations, not authored explanations or designs; create them only when they help retrieval. Preserve source qualifications and conflicting accounts. Leave unanswered questions open. Check local citations, then return the files and questions addressed. Your assignment ends with these topic indexes; the curator writes the root INDEX.md.`

const researchGapsPrompt = factualResearchContract + `

Collect original evidence for the missing factual topics in the gap research sources directory, meeting the required minimum per topic. In fixed source mode, use only the shared fixed originals directory and reference its files without copying them; do not browse or add sources. Otherwise collect needed original sources with origin, version or retrieval date, and precise source locations. Write a compact INDEX.md at the exact gap research index path with neutral routes, faithful source summaries, and citations. Any clips must be verbatim excerpts with provenance. Preserve qualifications, conflicting accounts, and open questions. Check citations and return what you actually researched.`

const buildIndexPrompt = factualResearchContract + `

Build a compact semantic routing tree at the exact semantic index path using the topic indexes and local originals. Organize routes by reader questions and source-documented topics, not researcher assignments. Each route briefly explains when to follow it and links to a topic index or precise original passage. Keep detailed source summaries in the leaves instead of repeating them in the root. Keep the root at most 500 words.

Explain the corpus scope, source locations, and citation conventions. Preserve working source summaries and verbatim clips; correct any authored opinions, deductions, advice, or designs by replacing them with faithful source information or removing them. Preserve original source files. Link to conflicting source accounts and unanswered questions without inventing resolutions or claims about absent documentation. Check links and walk representative routes from entrypoint to original evidence. Report checks in your response, not index prose. Stop when a reader can find the relevant information through a small number of clear choices.`

const updateIndexPrompt = factualResearchContract + `

Integrate gap research into the existing semantic routes. Add or repair links to original evidence, update open questions and links to conflicting source accounts, and preserve working routes. Keep the entrypoint compact and originals unchanged. Correct affected summaries to preserve source meaning and remove authored interpretation. Any clips remain verbatim source excerpts. Check affected routes from entrypoint to original evidence.`

const writeDocumentPrompt = factualResearchContract + `

Write a factual research document at the exact document path. Start retrieval at the semantic index and follow its routes to original passages. The document organizes and compresses source information about the requested subject; it is not an analysis or design proposal. Use the index for navigation, not as a substitute for original evidence. Preserve attribution, source qualifications, versions, experimental conditions, and conflicting accounts. State open questions when evidence is missing. Do not independently expand the research or redesign the index; identify material factual coverage gaps for the editor.

Prioritize the requested subject and audience while keeping citations usable. Run the supplied token counter executable with "count-tokens" and the document path. Compress repetition and secondary detail until the measured count is within budget without changing source meaning or certainty.`

const editorialPrompt = factualResearchContract + `

Check the document for faithful factual compression of the requested subject within the token budget. Locate original evidence through the semantic index. Trace substantive statements to their source passages and check attribution, scope, qualifications, uncertainty, and conflicting accounts. Agreement with an index summary alone is not verification. Model-authored opinions, recommendations, deductions, designs, and reconciliation of source conflicts are defects, including when separately labeled or requested by the goal.

Set OnlyNitpicks only when no added interpretation, unsupported or misleading statement, missing requested factual coverage, broken citation, or loss of meaning through compression warrants revision. Put defects repairable from existing evidence in MaterialIssues. Put requested factual topics requiring additional evidence in MissingTopics. An explicitly open question or faithfully preserved source disagreement is not itself a defect. Do not demand preferred approaches, new conclusions, optional polish, or broader research.`

const reviseDocumentPrompt = factualResearchContract + `

Revise the factual document using the editorial verdict, measured token count, and semantic index. Follow affected routes to original evidence. Repair source fidelity and citation defects; remove authored interpretation, advice, deductions, and designs rather than relabeling them. Preserve requested factual coverage, source qualifications, conflicting accounts, and open questions. Remove repetition and secondary detail before weakening necessary qualifications. Run the supplied token counter executable with "count-tokens" and the document path until the measured count is within budget.`

const indexReviewContext = `## research goal
{{(index .By "document goal").Text}}

## research directory
{{(index .By "research directory").Text}}

## entrypoint
{{(index .By "semantic index path").Text}}

## structural defects
{{(index .By "index structure problems").Text}}

{{with index .By "index verdict"}}## previous index verdict
{{.Text}}{{end}}`

const documentContext = `## research goal
{{(index .By "document goal").Text}}

## research directory
{{(index .By "research directory").Text}}

## entrypoint
{{(index .By "semantic index path").Text}}

## output document
{{(index .By "document path").Text}}

## token budget
{{(index .By "document token budget").Text}}

## token counter executable
{{(index .By "token counter executable").Text}}

{{with index .By "document token count"}}## measured document tokens
{{.Text}}{{end}}

{{with index .By "editorial verdict"}}## editorial verdict
{{.Text}}{{end}}`

const reviewIndexPrompt = factualResearchContract + `

Independently validate the semantic index. Choose three to five reader questions covering the main routes, including a qualification-sensitive fact or conflicting measurements when present. Follow each route to original passages and surrounding sections; compare the relevant summaries with that evidence. Check that attribution, versions, conditions, uncertainty, and conflicting source measurements survive compression. Clips, if present, must be verbatim. A citation to another summary does not establish a fact.

Use at most five file reads per retrieval route. Check that the sampled routes reach useful evidence, routing prose stays compact, and citations name precise local passages. This is a focused retrieval and source-fidelity check, not an exhaustive review of the corpus. Do not edit files. Set OnlyNitpicks only when the source checks and retrieval walks succeed and the supplied structural defects are empty. MaterialIssues must name concrete files, assertions, source spans, or broken routes to repair. MissingTopics is only for required factual coverage absent from the index despite available evidence; an open research question is allowed. Do not request analysis, preferred designs, extra research, or optional polish.`

const repairIndexPrompt = factualResearchContract + `

Repair the supplied index verdict and structural defects using existing original sources. Correct misleading summaries and citation targets, restore necessary qualifications and visible source disagreements, and fix broken or long routes. Keep the root at most 500 words; put detailed factual summaries in the leaves. Preserve original sources. Do not add evidence-free absence claims or fill open questions by inference. Return the affected paths and concrete repairs; the independent reader will check again.`

func verifyIndex(researchDir string, topicIndexes []string) error {
	root := filepath.Join(researchDir, "INDEX.md")
	if err := requireNonemptyFile(root); err != nil {
		return err
	}
	b, err := os.ReadFile(root)
	if err != nil {
		return err
	}
	if words := len(strings.Fields(string(b))); words > 500 {
		return fmt.Errorf("root INDEX.md has %d words; keep it at most 500 by moving details to leaves", words)
	}
	for _, path := range topicIndexes {
		if err := requireNonemptyFile(path); err != nil {
			return err
		}
	}
	return nil
}
