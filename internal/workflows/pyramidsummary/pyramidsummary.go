// Package pyramidsummary compresses one validated research-backed document
// into a pyramid that repeatedly halves its token budget until the next level
// would be under 100 tokens. The largest budget defaults to 3200.
//
// The largest document and its semantic index already exist when this workflow
// starts. Six fixed author slots independently write up to six of the smallest
// derived levels in parallel. When a larger starting budget creates additional
// upper levels, a bounded Promise Loop writes those after the fixed fan-out.
// Every author retains the original goal and index as aids for judging which
// knowledge matters, but research is over.
//
// One editor then reads every level together. It checks factual fidelity,
// legibility, useful progressive compression, and whether important ideas
// survive longer than secondary detail. Derived levels with material issues
// receive one bounded repair wave followed by one final whole-pyramid review.
// A material defect in the supplied largest document or after the repair wave
// ends the workflow honestly.
//
// Model defaults use Gemini 3.1 Pro at high effort for document authoring and
// editorial review, Gemini 3.8 Flash at medium effort for document
// supervision, and Luna for pyramid planning. Because authoring runs in
// parallel across up to six slots, callers should inspect these displayed pins
// before launching a large pyramid and override them deliberately when needed.
//
// Example:
//
//	gimble run pyramid-summary \
//	  --goal "Explain passkeys to security-conscious product managers" \
//	  --semantic-index ./passkeys-research/INDEX.md \
//	  --largest-document ./passkeys.md \
//	  --output-dir ./passkeys-pyramid
package pyramidsummary

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/polytype"
)

//go:generate go tool polytype --validate
//go:generate go run github.com/tylergannon/gimble/internal/generate/gimblegen -entry PyramidSummary -name pyramid-summary

const (
	roleDocumentAuthoring   gimble.WorkflowRole = "document-authoring"
	roleEditorialReview     gimble.WorkflowRole = "editorial-review"
	roleDocumentSupervision gimble.WorkflowRole = "document-supervision"
	rolePyramidPlanning     gimble.WorkflowRole = "pyramid-planning"
	defaultLargestBudget                        = 3200
	minimumLevelBudget                          = 100
)

// Params identify the accepted largest document, its knowledge, and the output directory.
type Params struct {
	// Goal describes the audience, subject, and understanding every level must preserve.
	Goal string
	// SemanticIndex is the existing index used to judge importance and factual fidelity.
	SemanticIndex string
	// LargestDocument is the already-validated document within the configured largest token budget.
	LargestDocument string
	// OutputDir receives one Markdown file per derived token budget.
	OutputDir string
	// LargestTokenBudget overrides the default starting budget of 3200 tokens.
	LargestTokenBudget polytype.Optional[int]
}

type levelSpec struct {
	Level  int
	Budget int
	Path   string
}

// LevelVerdict lists the material problems assigned to one pyramid level.
type LevelVerdict struct {
	Level          int      `json:"level"`
	MaterialIssues []string `json:"material_issues"`
}

// PyramidVerdict is the editor's judgment of the complete pyramid.
type PyramidVerdict struct {
	OnlyNitpicks bool           `json:"only_nitpicks"`
	Levels       []LevelVerdict `json:"levels"`
}

// PyramidSummary writes and validates every derived compression of a largest document.
func PyramidSummary(ctx context.Context, env gimble.Env, params Params) error {
	goal := strings.TrimSpace(params.Goal)
	if goal == "" {
		return fmt.Errorf("goal must not be blank")
	}
	largestBudget := defaultLargestBudget
	if params.LargestTokenBudget.Present {
		largestBudget = params.LargestTokenBudget.Value
	}
	if largestBudget < minimumLevelBudget {
		return fmt.Errorf("largest-token-budget must be at least %d", minimumLevelBudget)
	}
	semanticIndex, err := absoluteFrom(env.WorkDir, params.SemanticIndex)
	if err != nil {
		return fmt.Errorf("semantic-index: %w", err)
	}
	largestDocument, err := absoluteFrom(env.WorkDir, params.LargestDocument)
	if err != nil {
		return fmt.Errorf("largest-document: %w", err)
	}
	outputDir, err := absoluteFrom(env.WorkDir, params.OutputDir)
	if err != nil {
		return fmt.Errorf("output-dir: %w", err)
	}
	if err := requireNonemptyFile(semanticIndex); err != nil {
		return fmt.Errorf("semantic index: %w", err)
	}
	if err := requireNonemptyFile(largestDocument); err != nil {
		return fmt.Errorf("largest document: %w", err)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	tokenCounter, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve token counter: %w", err)
	}

	budgets := tokenBudgets(largestBudget)
	levels := make([]levelSpec, len(budgets))
	documents := make([]string, len(budgets))
	budgetLabels := make([]string, len(budgets))
	for i, budget := range budgets {
		path := filepath.Join(outputDir, fmt.Sprintf("%d.md", budget))
		if i == 0 {
			path = largestDocument
		}
		levels[i] = levelSpec{Level: i + 1, Budget: budget, Path: path}
		documents[i] = path
		budgetLabels[i] = fmt.Sprintf("level %d: %d", i+1, budget)
	}
	gimble.Set(ctx, "document goal", goal)
	gimble.Set(ctx, "semantic index path", semanticIndex)
	gimble.Set(ctx, "largest document path", largestDocument)
	gimble.Set(ctx, "largest token budget", largestBudget)
	gimble.Set(ctx, "pyramid document paths", documents)
	gimble.Set(ctx, "pyramid token budgets", budgetLabels)
	gimble.Set(ctx, "token counter executable", tokenCounter)

	largestCount, err := countPyramid(ctx, env.WorkDir, tokenCounter, documents[:1])
	if err != nil {
		return err
	}
	if largestCount[0] > largestBudget {
		return fmt.Errorf("largest document has %d tokens, over its %d-token budget", largestCount[0], largestBudget)
	}

	bottomStart := max(1, len(levels)-6)
	parallelLevels := levels[bottomStart:]
	var slots [6]levelSpec
	copy(slots[:], parallelLevels)
	extraTopLevels := levels[1:bottomStart]

	compressions := gimble.Group(ctx, "compressions")
	compressions.Go("summary1", func(ctx context.Context) error {
		if slots[0].Level == 0 {
			return nil
		}
		gimble.Set(ctx, "pyramid level", slots[0].Level)
		gimble.Set(ctx, "target document path", slots[0].Path)
		gimble.Set(ctx, "target token budget", slots[0].Budget)
		author := gimble.NewSession(ctx, roleDocumentAuthoring, env.WorkDir)
		coach := gimble.NewSession(ctx, roleDocumentSupervision, env.WorkDir)
		if _, err := author.Generate[gimble.Text](ctx, writeLevelPrompt,
			gimble.WithSupervisor(coach, compressionCoachPrompt)); err != nil {
			return err
		}
		return requireNonemptyFile(slots[0].Path)
	})
	compressions.Go("summary2", func(ctx context.Context) error {
		if slots[1].Level == 0 {
			return nil
		}
		gimble.Set(ctx, "pyramid level", slots[1].Level)
		gimble.Set(ctx, "target document path", slots[1].Path)
		gimble.Set(ctx, "target token budget", slots[1].Budget)
		author := gimble.NewSession(ctx, roleDocumentAuthoring, env.WorkDir)
		coach := gimble.NewSession(ctx, roleDocumentSupervision, env.WorkDir)
		if _, err := author.Generate[gimble.Text](ctx, writeLevelPrompt,
			gimble.WithSupervisor(coach, compressionCoachPrompt)); err != nil {
			return err
		}
		return requireNonemptyFile(slots[1].Path)
	})
	compressions.Go("summary3", func(ctx context.Context) error {
		if slots[2].Level == 0 {
			return nil
		}
		gimble.Set(ctx, "pyramid level", slots[2].Level)
		gimble.Set(ctx, "target document path", slots[2].Path)
		gimble.Set(ctx, "target token budget", slots[2].Budget)
		author := gimble.NewSession(ctx, roleDocumentAuthoring, env.WorkDir)
		coach := gimble.NewSession(ctx, roleDocumentSupervision, env.WorkDir)
		if _, err := author.Generate[gimble.Text](ctx, writeLevelPrompt,
			gimble.WithSupervisor(coach, compressionCoachPrompt)); err != nil {
			return err
		}
		return requireNonemptyFile(slots[2].Path)
	})
	compressions.Go("summary4", func(ctx context.Context) error {
		if slots[3].Level == 0 {
			return nil
		}
		gimble.Set(ctx, "pyramid level", slots[3].Level)
		gimble.Set(ctx, "target document path", slots[3].Path)
		gimble.Set(ctx, "target token budget", slots[3].Budget)
		author := gimble.NewSession(ctx, roleDocumentAuthoring, env.WorkDir)
		coach := gimble.NewSession(ctx, roleDocumentSupervision, env.WorkDir)
		if _, err := author.Generate[gimble.Text](ctx, writeLevelPrompt,
			gimble.WithSupervisor(coach, compressionCoachPrompt)); err != nil {
			return err
		}
		return requireNonemptyFile(slots[3].Path)
	})
	compressions.Go("summary5", func(ctx context.Context) error {
		if slots[4].Level == 0 {
			return nil
		}
		gimble.Set(ctx, "pyramid level", slots[4].Level)
		gimble.Set(ctx, "target document path", slots[4].Path)
		gimble.Set(ctx, "target token budget", slots[4].Budget)
		author := gimble.NewSession(ctx, roleDocumentAuthoring, env.WorkDir)
		coach := gimble.NewSession(ctx, roleDocumentSupervision, env.WorkDir)
		if _, err := author.Generate[gimble.Text](ctx, writeLevelPrompt,
			gimble.WithSupervisor(coach, compressionCoachPrompt)); err != nil {
			return err
		}
		return requireNonemptyFile(slots[4].Path)
	})
	compressions.Go("summary6", func(ctx context.Context) error {
		if slots[5].Level == 0 {
			return nil
		}
		gimble.Set(ctx, "pyramid level", slots[5].Level)
		gimble.Set(ctx, "target document path", slots[5].Path)
		gimble.Set(ctx, "target token budget", slots[5].Budget)
		author := gimble.NewSession(ctx, roleDocumentAuthoring, env.WorkDir)
		coach := gimble.NewSession(ctx, roleDocumentSupervision, env.WorkDir)
		if _, err := author.Generate[gimble.Text](ctx, writeLevelPrompt,
			gimble.WithSupervisor(coach, compressionCoachPrompt)); err != nil {
			return err
		}
		return requireNonemptyFile(slots[5].Path)
	})
	if err := compressions.Wait(); err != nil {
		return err
	}

	if len(extraTopLevels) > 0 {
		gimble.Set(ctx, "extra top level assignments", levelAssignments(extraTopLevels, nil))
		planner := gimble.NewSession(ctx, rolePyramidPlanning, env.WorkDir)
		worker := gimble.NewSession(ctx, roleDocumentAuthoring, env.WorkDir)
		plannerCoach := gimble.NewSession(ctx, roleDocumentSupervision, env.WorkDir)
		workerCoach := gimble.NewSession(ctx, roleDocumentSupervision, env.WorkDir)
		loop := gimble.PromiseLoop(ctx, "extra-top-levels", extraTopGoal, planner,
			gimble.WithSupervisor(plannerCoach, topPlannerCoachPrompt))
		tasksRun := 0
		for taskCtx := range loop.Tasks {
			if tasksRun >= len(extraTopLevels) {
				break
			}
			tasksRun++
			if _, err := worker.Generate[gimble.Text](taskCtx, writeTopLevelPrompt,
				gimble.WithSupervisor(workerCoach, compressionCoachPrompt)); err != nil {
				return err
			}
		}
		if err := loop.Err(); err != nil {
			return err
		}
		for _, level := range extraTopLevels {
			if err := requireNonemptyFile(level.Path); err != nil {
				return fmt.Errorf("extra top level %d: %w", level.Level, err)
			}
		}
	}

	counts, err := countPyramid(ctx, env.WorkDir, tokenCounter, documents)
	if err != nil {
		return err
	}
	for i, count := range counts {
		if count > budgets[i] {
			return fmt.Errorf("level %d has %d tokens, over its %d-token budget", i+1, count, budgets[i])
		}
	}
	gimble.Set(ctx, "measured token counts", measuredCounts(counts, budgets))

	editor := gimble.NewSession(ctx, roleEditorialReview, env.WorkDir)
	verdict, err := editor.Generate[PyramidVerdict](ctx, reviewPyramidPrompt)
	if err != nil {
		return err
	}
	gimble.SetJSON(ctx, "pyramid verdict", verdict)
	if verdict.OnlyNitpicks && len(verdict.Levels) == 0 {
		return nil
	}
	if issues := issuesForLevel(verdict, 1); len(issues) > 0 {
		return fmt.Errorf("largest document needs repair before pyramid compression: %s", strings.Join(issues, "; "))
	}

	repairIssues := make([][]string, len(levels))
	for level := 2; level <= len(levels); level++ {
		repairIssues[level-1] = issuesForLevel(verdict, level)
	}
	if noIssues(repairIssues[1:]) {
		return fmt.Errorf("editor rejected the pyramid without assigning a material issue to a level")
	}

	repair := gimble.Group(ctx, "repair")
	repair.Go("summary1", func(ctx context.Context) error {
		if slots[0].Level == 0 || len(repairIssues[slots[0].Level-1]) == 0 {
			return nil
		}
		gimble.Set(ctx, "pyramid level", slots[0].Level)
		gimble.Set(ctx, "target document path", slots[0].Path)
		gimble.Set(ctx, "target token budget", slots[0].Budget)
		gimble.Set(ctx, "material issues", repairIssues[slots[0].Level-1])
		author := gimble.NewSession(ctx, roleDocumentAuthoring, env.WorkDir)
		coach := gimble.NewSession(ctx, roleDocumentSupervision, env.WorkDir)
		_, err := author.Generate[gimble.Text](ctx, reviseLevelPrompt,
			gimble.WithSupervisor(coach, compressionCoachPrompt))
		return err
	})
	repair.Go("summary2", func(ctx context.Context) error {
		if slots[1].Level == 0 || len(repairIssues[slots[1].Level-1]) == 0 {
			return nil
		}
		gimble.Set(ctx, "pyramid level", slots[1].Level)
		gimble.Set(ctx, "target document path", slots[1].Path)
		gimble.Set(ctx, "target token budget", slots[1].Budget)
		gimble.Set(ctx, "material issues", repairIssues[slots[1].Level-1])
		author := gimble.NewSession(ctx, roleDocumentAuthoring, env.WorkDir)
		coach := gimble.NewSession(ctx, roleDocumentSupervision, env.WorkDir)
		_, err := author.Generate[gimble.Text](ctx, reviseLevelPrompt,
			gimble.WithSupervisor(coach, compressionCoachPrompt))
		return err
	})
	repair.Go("summary3", func(ctx context.Context) error {
		if slots[2].Level == 0 || len(repairIssues[slots[2].Level-1]) == 0 {
			return nil
		}
		gimble.Set(ctx, "pyramid level", slots[2].Level)
		gimble.Set(ctx, "target document path", slots[2].Path)
		gimble.Set(ctx, "target token budget", slots[2].Budget)
		gimble.Set(ctx, "material issues", repairIssues[slots[2].Level-1])
		author := gimble.NewSession(ctx, roleDocumentAuthoring, env.WorkDir)
		coach := gimble.NewSession(ctx, roleDocumentSupervision, env.WorkDir)
		_, err := author.Generate[gimble.Text](ctx, reviseLevelPrompt,
			gimble.WithSupervisor(coach, compressionCoachPrompt))
		return err
	})
	repair.Go("summary4", func(ctx context.Context) error {
		if slots[3].Level == 0 || len(repairIssues[slots[3].Level-1]) == 0 {
			return nil
		}
		gimble.Set(ctx, "pyramid level", slots[3].Level)
		gimble.Set(ctx, "target document path", slots[3].Path)
		gimble.Set(ctx, "target token budget", slots[3].Budget)
		gimble.Set(ctx, "material issues", repairIssues[slots[3].Level-1])
		author := gimble.NewSession(ctx, roleDocumentAuthoring, env.WorkDir)
		coach := gimble.NewSession(ctx, roleDocumentSupervision, env.WorkDir)
		_, err := author.Generate[gimble.Text](ctx, reviseLevelPrompt,
			gimble.WithSupervisor(coach, compressionCoachPrompt))
		return err
	})
	repair.Go("summary5", func(ctx context.Context) error {
		if slots[4].Level == 0 || len(repairIssues[slots[4].Level-1]) == 0 {
			return nil
		}
		gimble.Set(ctx, "pyramid level", slots[4].Level)
		gimble.Set(ctx, "target document path", slots[4].Path)
		gimble.Set(ctx, "target token budget", slots[4].Budget)
		gimble.Set(ctx, "material issues", repairIssues[slots[4].Level-1])
		author := gimble.NewSession(ctx, roleDocumentAuthoring, env.WorkDir)
		coach := gimble.NewSession(ctx, roleDocumentSupervision, env.WorkDir)
		_, err := author.Generate[gimble.Text](ctx, reviseLevelPrompt,
			gimble.WithSupervisor(coach, compressionCoachPrompt))
		return err
	})
	repair.Go("summary6", func(ctx context.Context) error {
		if slots[5].Level == 0 || len(repairIssues[slots[5].Level-1]) == 0 {
			return nil
		}
		gimble.Set(ctx, "pyramid level", slots[5].Level)
		gimble.Set(ctx, "target document path", slots[5].Path)
		gimble.Set(ctx, "target token budget", slots[5].Budget)
		gimble.Set(ctx, "material issues", repairIssues[slots[5].Level-1])
		author := gimble.NewSession(ctx, roleDocumentAuthoring, env.WorkDir)
		coach := gimble.NewSession(ctx, roleDocumentSupervision, env.WorkDir)
		_, err := author.Generate[gimble.Text](ctx, reviseLevelPrompt,
			gimble.WithSupervisor(coach, compressionCoachPrompt))
		return err
	})
	if err := repair.Wait(); err != nil {
		return err
	}

	var extraTopRepairs []levelSpec
	for _, level := range extraTopLevels {
		if len(repairIssues[level.Level-1]) > 0 {
			extraTopRepairs = append(extraTopRepairs, level)
		}
	}
	if len(extraTopRepairs) > 0 {
		gimble.Set(ctx, "extra top level repairs", levelAssignments(extraTopRepairs, repairIssues))
		planner := gimble.NewSession(ctx, rolePyramidPlanning, env.WorkDir)
		worker := gimble.NewSession(ctx, roleDocumentAuthoring, env.WorkDir)
		plannerCoach := gimble.NewSession(ctx, roleDocumentSupervision, env.WorkDir)
		workerCoach := gimble.NewSession(ctx, roleDocumentSupervision, env.WorkDir)
		loop := gimble.PromiseLoop(ctx, "repair-extra-top-levels", repairTopGoal, planner,
			gimble.WithSupervisor(plannerCoach, topPlannerCoachPrompt))
		tasksRun := 0
		for taskCtx := range loop.Tasks {
			if tasksRun >= len(extraTopRepairs) {
				break
			}
			tasksRun++
			if _, err := worker.Generate[gimble.Text](taskCtx, reviseTopLevelPrompt,
				gimble.WithSupervisor(workerCoach, compressionCoachPrompt)); err != nil {
				return err
			}
		}
		if err := loop.Err(); err != nil {
			return err
		}
	}

	return gimble.Scope(ctx, "final-validation", func(ctx context.Context) error {
		counts, err := countPyramid(ctx, env.WorkDir, tokenCounter, documents)
		if err != nil {
			return err
		}
		for i, count := range counts {
			if count > budgets[i] {
				return fmt.Errorf("level %d has %d tokens after repair, over its %d-token budget", i+1, count, budgets[i])
			}
		}
		gimble.Set(ctx, "measured token counts", measuredCounts(counts, budgets))
		finalVerdict, err := editor.Generate[PyramidVerdict](ctx, reviewPyramidPrompt)
		if err != nil {
			return err
		}
		gimble.SetJSON(ctx, "final pyramid verdict", finalVerdict)
		if finalVerdict.OnlyNitpicks && len(finalVerdict.Levels) == 0 {
			return nil
		}
		return fmt.Errorf("pyramid still has material issues after its one repair wave")
	})
}

func countPyramid(ctx context.Context, workDir, executable string, documents []string) ([]int, error) {
	counts := make([]int, len(documents))
	for i, document := range documents {
		exit, stdout, stderr, err := gimble.RunCommand(ctx, "count-tokens", workDir, executable, "count-tokens", document)
		if err != nil {
			return nil, err
		}
		if exit != 0 {
			return nil, fmt.Errorf("count tokens in %s: %s", document, strings.TrimSpace(stderr))
		}
		count, err := strconv.Atoi(strings.TrimSpace(stdout))
		if err != nil {
			return nil, fmt.Errorf("parse token count for %s: %w", document, err)
		}
		counts[i] = count
	}
	return counts, nil
}

func tokenBudgets(largest int) []int {
	var budgets []int
	for budget := largest; ; budget /= 2 {
		budgets = append(budgets, budget)
		if budget/2 < minimumLevelBudget {
			return budgets
		}
	}
}

func measuredCounts(counts, budgets []int) []string {
	measured := make([]string, len(counts))
	for i, count := range counts {
		measured[i] = fmt.Sprintf("level %d: %d/%d", i+1, count, budgets[i])
	}
	return measured
}

func levelAssignments(levels []levelSpec, issues [][]string) []string {
	assignments := make([]string, len(levels))
	for i, level := range levels {
		assignment := fmt.Sprintf("level %d: budget %d; output %s", level.Level, level.Budget, level.Path)
		if issues != nil && len(issues[level.Level-1]) > 0 {
			assignment += "; material issues: " + strings.Join(issues[level.Level-1], " | ")
		}
		assignments[i] = assignment
	}
	return assignments
}

func issuesForLevel(verdict PyramidVerdict, level int) []string {
	var issues []string
	for _, result := range verdict.Levels {
		if result.Level == level {
			issues = append(issues, result.MaterialIssues...)
		}
	}
	return issues
}

func noIssues(levels [][]string) bool {
	for _, issues := range levels {
		if len(issues) > 0 {
			return false
		}
	}
	return true
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

const writeLevelPrompt = `Write this pyramid level at the exact target document path. Compress the largest document rather than restarting research. Use the original goal and semantic index only to judge importance, preserve factual fidelity, and resolve what should survive at this level. The result must be accurate, standalone, and legible prose rather than notes or fragments. Do not pad it. Before finishing, repeatedly run the token counter executable with "count-tokens" and the target path, editing until the document is at or below its target budget.`

const reviseLevelPrompt = `Revise this pyramid level at its exact target path to fix every assigned material issue. Judge importance against the original goal, largest document, and semantic index. Preserve legible standalone prose and the right knowledge for this level. Before finishing, repeatedly run the token counter executable with "count-tokens" and the target path, editing until the document is at or below its target budget.`

const extraTopGoal = `Write every extra top-level assignment exactly once. Each task must copy one assignment's level, budget, and output path verbatim into its description and definition of done. Do not invent, combine, repeat, or omit assignments. End dispatch after every listed output exists.`

const repairTopGoal = `Repair every listed extra top-level assignment exactly once. Each task must copy one assignment's level, budget, output path, and material issues verbatim into its description and definition of done. Do not invent, combine, repeat, or omit assignments. End dispatch after every listed repair is complete.`

const writeTopLevelPrompt = `Perform the current extra top-level task exactly as assigned. Write its output path within its stated budget by compressing the largest document, using the original goal and semantic index only to judge importance and factual fidelity. Produce accurate, standalone, legible prose. Before finishing, repeatedly run the token counter executable with "count-tokens" and the assigned output path until it is within budget.`

const reviseTopLevelPrompt = `Perform the current extra top-level repair exactly as assigned. Fix every listed material issue using the original goal, largest document, and semantic index as judgment aids. Preserve accurate, standalone, legible prose. Before finishing, repeatedly run the token counter executable with "count-tokens" and the assigned output path until it is within budget.`

const compressionCoachPrompt = `The goal is the clearest standalone understanding this pyramid level can hold. Ensure the author measures with the supplied token counter, removes secondary detail before central knowledge, and writes legible prose rather than compressed fragments. Object to renewed research or polishing beyond the assigned document.`

const topPlannerCoachPrompt = `Dispatch only the exact extra top-level assignments listed in context, once each. Object to invented work, repeated levels, changed budgets or paths, or continued dispatch after the list is exhausted.`

const reviewPyramidPrompt = `Read every pyramid document, the original goal, and the semantic index at their exact paths. Assess the complete pyramid together. Every level must be accurate, independently legible, within its measured budget, and meaningfully more compressed than the preceding level. Important knowledge should survive longer than secondary detail, and no level may contradict another. Report only material problems, not optional polish. Assign every material problem to each level that must change. Return OnlyNitpicks true with an empty Levels list only when the entire pyramid needs no further substantive work.`
