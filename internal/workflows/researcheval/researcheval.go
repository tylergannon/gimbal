// Package researcheval compares economical research/indexing models using a
// fixed local-source benchmark, independent assessment, and bounded index queries.
//
// An Opus planner selects candidate IDs in a PromiseLoop. Code runs the real
// research-document workflow, reports quality, audit repairs, usage and cost,
// and freezes the cheapest quality-eligible candidate before a held-out case.
// The planner cannot change the measure or see holdout feedback before selection.
// Success means both development and holdout passed; an ended loop is not proof.
// The small fictional suite measures source-grounded synthesis and navigation,
// not open-web discovery or a statistically established model success rate.
//
// Run in a hosted project, with --work-dir equal to --project. ResearchInstance
// must select the same instance as the parent (omit it for the normal default).
// Child research runs are hosted normally and cancelled/joined on parent timeout.
// The report in OutputDir names isolated trial directories and raw assessments.
// Existing output directories are refused so another evaluation cannot be reused
// accidentally. Costs are catalog proxies, not subscription invoices; unknown
// usage is never treated as free. Reader/assessor/planner overhead is separate.
//
// CandidatesFile optionally names a JSON array of {id,research_model,index_model}.
// It is an allowlist; defaults allow independent research/index combinations
// of Gemini Flash, OpenAI Luna and Terra, and Claude Haiku and Sonnet through
// their native providers. The planner samples controlled role swaps within the
// round budget; it does not exhaust all combinations. Other research roles use their production defaults; FixedModel can override
// them. Role flags independently pin the evaluation planner,
// reader and assessor. An unavailable provider is a recorded failed trial.
//
// Example:
//
//	gimbal run research-eval --project /path/project --work-dir /path/project \
//	  --output-dir /tmp/research-eval-1 --max-trials 10
package researcheval

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/live"
	"github.com/tylergannon/gimbal/internal/observation"
	"github.com/tylergannon/polytype"
)

//go:generate go tool polytype --validate
//go:generate go run github.com/tylergannon/gimbal/internal/generate/gimbalgen -entry ResearchEval -name research-eval

type Params struct {
	// OutputDir is a new directory for report.json; it names isolated trial outputs.
	OutputDir string
	// CandidatesFile optionally replaces the bundled model-combination allowlist.
	CandidatesFile polytype.Optional[string]
	// SuiteDir optionally names a directory with suite.json and original source files.
	SuiteDir polytype.Optional[string]
	// MaxTrials bounds development rounds across all cases; default ten. Selection needs two repeats of every development case.
	MaxTrials polytype.Optional[int]
	// TrialMinutes bounds each research trial; default fifteen minutes.
	TrialMinutes polytype.Optional[int]
	// FixedModel optionally overrides the production defaults for planning, authoring, review and supervision.
	FixedModel polytype.Optional[string]
	// ResearchInstance selects the same instance state directory as the parent invocation.
	ResearchInstance polytype.Optional[string]
}

type candidate struct {
	ID            string `json:"id"`
	ResearchModel string `json:"research_model"`
	IndexModel    string `json:"index_model"`
}

type trialResult struct {
	Candidate           candidate         `json:"candidate"`
	Case                string            `json:"case"`
	Split               string            `json:"split"`
	RunID               string            `json:"run_id"`
	Directory           string            `json:"directory"`
	OriginalDirectory   string            `json:"original_directory"`
	Seconds             float64           `json:"seconds"`
	CostUSD             float64           `json:"cost_usd"`
	CostKnown           bool              `json:"cost_known"`
	Roles               []roleMeasurement `json:"roles"`
	Audit               auditSummary      `json:"audit"`
	InitialAuditClean   bool              `json:"initial_audit_clean"`
	OneRepairAuditClean bool              `json:"one_repair_audit_clean"`
	AuditRepairPasses   int               `json:"audit_repair_passes"`
	JevInputTokens      int64             `json:"jev_input_tokens"`
	Quality             score             `json:"quality"`
	Error               string            `json:"error,omitempty"`
}

type auditSummary struct {
	Complete         bool `json:"complete"`
	CoverageComplete bool `json:"coverage_complete"`
	AuthoringAllowed bool `json:"authoring_allowed"`
	Metrics          struct {
		Claims                  int   `json:"claims"`
		SourceFindings          int   `json:"source_findings"`
		PairFindings            int   `json:"pair_findings"`
		ExtractionFindings      int   `json:"extraction_findings"`
		InputTokens             int64 `json:"input_tokens"`
		RequestCount            int   `json:"request_count"`
		WholeIndexO200kTokens   int   `json:"whole_index_o200k_tokens"`
		WholeIndexUnder24kProxy bool  `json:"whole_index_under_24k_proxy"`
		WholeIndexJevTested     bool  `json:"whole_index_jev_tested"`
		RepairPass              int   `json:"repair_pass"`
	} `json:"metrics"`
}

type report struct {
	Suite               string             `json:"suite"`
	FixedModel          string             `json:"fixed_model"`
	Trials              []trialResult      `json:"trials"`
	Candidates          []candidateSummary `json:"candidate_summaries"`
	Selected            string             `json:"selected"`
	Fulfilled           bool               `json:"fulfilled"`
	CalibrationPassed   bool               `json:"calibration_passed"`
	Calibration         calibrationCounts  `json:"calibration"`
	EvaluationCostUSD   float64            `json:"evaluation_cost_usd"`
	EvaluationCostKnown bool               `json:"evaluation_cost_known"`
	PriceBasis          string             `json:"price_basis"`
	Limitation          string             `json:"limitation"`
}

// ResearchEval measures research candidates and checks a frozen choice on holdout.
func ResearchEval(ctx context.Context, env gimbal.Env, params Params) error {
	registry, controls := observation.FromContext(ctx), live.RunsFrom(ctx)
	if registry == nil || controls == nil {
		return fmt.Errorf("research-eval requires a hosted run")
	}
	maxTrials, minutes, fixed := 10, 15, ""
	if params.MaxTrials.Present {
		maxTrials = params.MaxTrials.Value
	}
	if params.TrialMinutes.Present {
		minutes = params.TrialMinutes.Value
	}
	if params.FixedModel.Present {
		fixed = params.FixedModel.Value
	}
	if maxTrials < 1 || minutes < 1 || (params.FixedModel.Present && strings.TrimSpace(fixed) == "") {
		return fmt.Errorf("trial limits and fixed model must be nonempty/positive")
	}
	output, err := absoluteFrom(env.WorkDir, params.OutputDir)
	if err != nil {
		return err
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		return fmt.Errorf("output-dir must not already exist")
	}
	if err := os.MkdirAll(output, 0o755); err != nil {
		return err
	}
	suiteDir, err := os.MkdirTemp("", "gimbal-eval-private-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(suiteDir) }()
	privateDir := suiteDir
	if params.SuiteDir.Present {
		suiteDir, err = absoluteFrom(env.WorkDir, params.SuiteDir.Value)
		if err != nil {
			return err
		}
	} else if err := materializeSuite(suiteDir); err != nil {
		return err
	}
	s, err := loadSuite(suiteDir)
	if err != nil {
		return err
	}
	if !params.SuiteDir.Present {
		// Keep the answer key in memory until the research being assessed has ended.
		if err := os.Remove(filepath.Join(suiteDir, "suite.json")); err != nil {
			return err
		}
	}
	candidates, err := loadCandidates(env.WorkDir, params.CandidatesFile)
	if err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	// The child's --project must be this very hosted project. Checking a live
	// registry record under the requested root establishes that before submission.
	if err := verifyProject(env.WorkDir, registry); err != nil {
		return err
	}
	result := report{Suite: s.Name, FixedModel: fixed, Trials: []trialResult{}, PriceBasis: priceBasis, Limitation: "Small controlled-source pilot; model judgments and catalog-price proxies, not open-web quality or a population success rate. First-pass audit cleanliness is distinct from independent correctness. Price covers recorded agent and claim-audit usage; automatic supervisor Jev checks are not metered here."}
	if err := writeJSON(filepath.Join(output, "report.json"), result); err != nil {
		return err
	}
	gimbal.Set(ctx, "candidate allowlist", jsonText(candidates))
	gimbal.Set(ctx, "maximum development trials", maxTrials)
	gimbal.Set(ctx, "evaluation rules", plannerRules)
	gimbal.Set(ctx, "evaluation report", filepath.Join(output, "report.json"))
	parentID, err := reportRun(env.WorkDir, registry, filepath.Join(output, "report.json"))
	if err != nil {
		return err
	}

	// Preserve measured overhead even when calibration or cancellation ends the run.
	defer func() {
		if snapshot, err := registry.Snapshot(parentID); err == nil {
			result.EvaluationCostUSD, result.EvaluationCostKnown = researchCost(snapshot)
		}
		_ = writeJSON(filepath.Join(output, "report.json"), result)
	}()

	// Sanity-check the semantic assessor on labeled good and corrupted outputs
	// before spending on candidates. This detects obvious judge failures; it is
	// not a statistical estimate of judge accuracy on arbitrary research.
	calibrationIndex := 0
	for calibrationCtx, corrupted := range gimbal.Iterate(ctx, "calibrate-assessor", []bool{false, true, false, true, false, true}) {
		calibrationIndex++
		dir := filepath.Join(privateDir, fmt.Sprintf("assessor-check-%d", calibrationIndex))
		sources := filepath.Join(dir, "sources")
		if err := copySources(sources, map[string]string{"manual.md": "For version 3, the request limit is 12 KiB. Version 2 allowed 32 KiB.\n"}); err != nil {
			return err
		}
		c := researchCase{Facts: []fact{{ID: "current-limit", Statement: "For version 3, the request limit is 12 KiB.", Evidence: []evidence{{Source: "manual.md", Quote: "For version 3, the request limit is 12 KiB."}}}}}
		gold, document, index := filepath.Join(dir, "gold.json"), filepath.Join(dir, "document.md"), filepath.Join(dir, "index")
		if err := writeJSON(gold, c); err != nil {
			return err
		}
		if err := os.WriteFile(document, []byte("For version 3, the request limit is 12 KiB. See sources/manual.md.\n"), 0644); err != nil {
			return err
		}
		assertion := "For version 3, the request limit is 12 KiB."
		if corrupted {
			assertion = "For version 3, the request limit is 32 KiB."
		}
		if err := copySources(index, map[string]string{"INDEX.md": assertion + " See [manual](sources/manual.md).\n", "sources/manual.md": "For version 3, the request limit is 12 KiB. Version 2 allowed 32 KiB.\n"}); err != nil {
			return err
		}
		gimbal.Set(calibrationCtx, "assessment gold", gold)
		gimbal.Set(calibrationCtx, "original source directory", sources)
		gimbal.Set(calibrationCtx, "trial document", document)
		gimbal.Set(calibrationCtx, "trial corpus", index)
		assessor := gimbal.NewSession(calibrationCtx, "research-eval-assessment", env.WorkDir)
		quality, err := assessor.Generate[Quality](calibrationCtx, qualityPrompt, gimbal.WithScopeTemplate(assessmentContext))
		if err != nil {
			return err
		}
		if err := writeJSON(filepath.Join(output, fmt.Sprintf("calibration-%02d.json", calibrationIndex)), quality); err != nil {
			return err
		}
		if len(quality.Facts) != 1 || quality.Facts[0].ID != "current-limit" || !quality.Facts[0].DocumentCovered || (!corrupted && !quality.Facts[0].IndexCovered) || quality.DocumentAssertions < 1 || quality.IndexAssertions < 1 || len(quality.UnsupportedDocumentClaims) != 0 || (len(quality.UnsupportedIndexClaims) > 0) != corrupted {
			if corrupted {
				result.Calibration.FalseNegatives++
			} else {
				result.Calibration.FalsePositives++
			}
		}
		if corrupted {
			result.Calibration.BadSamples++
		} else {
			result.Calibration.GoodSamples++
		}
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	if result.Calibration.FalseNegatives != 0 || result.Calibration.FalsePositives != 0 {
		return fmt.Errorf("assessor calibration failed: %+v; optimization not started", result.Calibration)
	}
	result.CalibrationPassed = true
	if err := writeJSON(filepath.Join(output, "report.json"), result); err != nil {
		return err
	}
	planner := gimbal.NewSession(ctx, "research-eval-planning", env.WorkDir)
	loop := gimbal.PromiseLoop(ctx, "optimize-research", "Find an economical research/indexing combination that passes the fixed quality measure with few audit repairs. Select only supplied candidate IDs; finish by dispatching select for the frozen holdout check.", planner)
	trialCount, heldout := 0, false
	for taskCtx, task := range loop.Tasks {
		phase := "development"
		choice, ok := candidateByID(candidates, strings.TrimSpace(task.Name))
		if task.Name == "select" || trialCount >= maxTrials {
			phase = "holdout"
			choice, ok = bestCandidate(candidates, result.Trials, s)
			if !ok {
				break
			}
			result.Selected = choice.ID
		} else if !ok {
			return fmt.Errorf("planner selected unknown candidate %q", task.Name)
		}
		trialCount++
		var cases []researchCase
		for _, c := range s.Cases {
			if c.Split == phase {
				cases = append(cases, c)
			}
		}
		for caseCtx, c := range gimbal.Iterate(taskCtx, "case", cases) {
			started := time.Now()
			dir, err := os.MkdirTemp("", "gimbal-research-trial-")
			if err != nil {
				return err
			}
			dir, err = filepath.EvalSymlinks(dir)
			if err != nil {
				return err
			}
			corpus, document := filepath.Join(dir, "research"), filepath.Join(dir, "document.md")
			originals, err := sourceTexts(suiteDir, c)
			if err != nil {
				return err
			}
			sourceDir := filepath.Join(dir, "source-input")
			err = copySources(sourceDir, originals)
			if err != nil {
				return err
			}
			row := trialResult{Candidate: choice, Case: c.ID, Split: phase, Directory: dir}
			args := []string{"run", "research-document", "--project", env.WorkDir, "--work-dir", dir, "--goal", c.Goal, "--source-dir", sourceDir, "--research-dir", corpus, "--output", document, "--token-budget", "1800", "--min-sources-per-topic", "1", "--max-editorial-rounds", "2", "--research-indexing", choice.ResearchModel, "--index-curation", choice.IndexModel}
			if fixed != "" {
				args = append(args, "--research-planning", fixed, "--document-authoring", fixed, "--editorial-review", fixed, "--document-supervision", fixed)
			}
			if params.ResearchInstance.Present {
				args = append(args, "--instance-dir", params.ResearchInstance.Value)
			}
			// Submission is brief and must deliver the accepted ID even if the
			// caller cancels meanwhile; the parent then cancels and joins that ID.
			submitCtx, endSubmit := context.WithTimeout(context.WithoutCancel(caseCtx), time.Minute)
			exit, stdout, stderr, launchErr := gimbal.RunCommand(submitCtx, "start-research", env.WorkDir, executable, args...)
			endSubmit()
			if launchErr != nil || exit != 0 {
				row.Error = fmt.Sprintf("research submission failed (acceptance may be unknown): exit=%d error=%v %s", exit, launchErr, stderr)
			} else {
				row.RunID = strings.TrimSpace(stdout)
				trialCtx, endTrial := context.WithTimeout(caseCtx, time.Duration(minutes)*time.Minute)
				snapshot, runErr := waitResearch(trialCtx, registry, controls, row.RunID)
				endTrial()
				if runErr != nil {
					row.Error = runErr.Error()
				}
				row.CostUSD, row.CostKnown = researchCost(snapshot)
				row.Roles = roleMeasurements(snapshot)
			}
			if raw, readErr := os.ReadFile(filepath.Join(corpus, ".semantic-index", "completion.json")); readErr == nil {
				if err := json.Unmarshal(raw, &row.Audit); err != nil {
					row.Error = "invalid audit summary: " + err.Error()
				}
			} else {
				row.CostKnown = false
			}
			row.InitialAuditClean, row.OneRepairAuditClean, row.AuditRepairPasses, row.JevInputTokens = auditHistory(corpus)
			if !row.Audit.Complete {
				// Completed passes alone are archived; retain current partial
				// usage but do not call an interrupted request's price known.
				row.JevInputTokens += row.Audit.Metrics.InputTokens
				row.CostKnown = false
			}
			row.CostUSD += float64(row.JevInputTokens) * 0.042 / 1_000_000
			if row.Error == "" {
				goldPath := filepath.Join(privateDir, fmt.Sprintf("assessment-gold-%03d-%s.json", trialCount, c.ID))
				if err := writeJSON(goldPath, c); err != nil {
					return err
				}
				gimbal.Set(caseCtx, "assessment gold", goldPath)
				gimbal.Set(caseCtx, "original source directory", sourceDir)
				gimbal.Set(caseCtx, "trial document", document)
				gimbal.Set(caseCtx, "trial corpus", corpus)
				assessor := gimbal.NewSession(caseCtx, "research-eval-assessment", env.WorkDir)
				quality, assessErr := assessor.Generate[Quality](caseCtx, qualityPrompt, gimbal.WithScopeTemplate(assessmentContext))
				if err := os.Remove(goldPath); err != nil {
					return err
				}
				if assessErr != nil {
					row.Error = assessErr.Error()
				} else {
					if err := writeJSON(filepath.Join(dir, "quality.json"), quality); err != nil {
						return err
					}
					var queryScores []queryScore
					for queryCtx, q := range gimbal.Iterate(caseCtx, "query", c.Queries) {
						before, err := registry.Snapshot(parentID)
						if err != nil {
							return err
						}
						queryResult := queryScore{ID: q.ID, ExpectedEvidence: len(q.Evidence)}
						read := map[string]string{}
						entry, err := os.ReadFile(filepath.Join(corpus, "INDEX.md"))
						if err != nil {
							return err
						}
						read["INDEX.md"] = string(entry)
						queryResult.Reads, queryResult.Bytes = 1, len(entry)
						if len(entry) > 18000 {
							queryResult.Error = "initial index exceeds byte budget"
						}
						reader := gimbal.NewSession(queryCtx, "research-eval-reading", dir)
						var answer ReaderStep
						for stepCtx, _ := range gimbal.Iterate(queryCtx, "navigate", []int{0, 1, 2, 3}) {
							if queryResult.Error != "" {
								break
							}
							gimbal.Set(stepCtx, "question", q.Question)
							gimbal.Set(stepCtx, "read passages", jsonText(read))
							gimbal.Set(stepCtx, "remaining file reads", 6-queryResult.Reads)
							step, err := reader.Generate[ReaderStep](stepCtx, readerPrompt, gimbal.WithScopeTemplate(readerContext))
							if err != nil {
								queryResult.Error = err.Error()
								break
							}
							if len(step.Paths) == 0 {
								answer = step
								break
							}
							if len(step.Paths) > 2 {
								queryResult.Error = "more than two paths requested in one turn"
								break
							}
							for _, name := range step.Paths {
								path, err := readerPath(corpus, name)
								if err != nil {
									queryResult.Error = "invalid path"
									break
								}
								key, _ := filepath.Rel(corpus, path)
								key = filepath.ToSlash(key)
								if _, exists := read[key]; exists {
									continue
								}
								if queryResult.Reads >= 6 {
									queryResult.Error = "read budget exceeded"
									break
								}
								resolved, err := filepath.EvalSymlinks(path)
								if err != nil || !inside(corpus, resolved) {
									queryResult.Error = "path unavailable or outside corpus"
									break
								}
								body, err := os.ReadFile(path)
								if err != nil || queryResult.Bytes+len(body) > 18000 {
									queryResult.Error = "file unavailable or byte budget exceeded"
									break
								}
								read[key] = string(body)
								queryResult.Reads++
								queryResult.Bytes += len(body)
							}
							if queryResult.Error != "" {
								break
							}
						}
						normalizeCitations(corpus, &answer)
						queryResult.Answer = answer.Answer
						after, err := registry.Snapshot(parentID)
						if err != nil {
							return err
						}
						if err := readerTelemetry(before, after); err != nil {
							queryResult.Error = err.Error()
						}
						found, exact := exactAnswer(q, answer, read, originals)
						queryResult.FoundEvidence = found
						gimbal.Set(queryCtx, "query gold", jsonText(q))
						gimbal.Set(queryCtx, "reader answer", jsonText(answer))
						gimbal.Set(queryCtx, "retrieved passages", jsonText(read))
						judge := gimbal.NewSession(queryCtx, "research-eval-assessment", env.WorkDir)
						judgment, err := judge.Generate[AnswerGrade](queryCtx, answerPrompt, gimbal.WithScopeTemplate(answerContext))
						if err != nil {
							queryResult.Error = err.Error()
						}
						queryResult.Success = exact && queryResult.Error == "" && judgment.Correct && judgment.Grounded
						queryScores = append(queryScores, queryResult)
						if err := writeJSON(filepath.Join(dir, "query-"+q.ID+".json"), struct {
							Answer ReaderStep
							Grade  AnswerGrade
							Score  queryScore
						}{answer, judgment, queryResult}); err != nil {
							return err
						}
						gimbal.Set(queryCtx, "query score", jsonText(queryResult))
					}
					row.Quality, err = scoreQuality(c, quality, queryScores)
					if err != nil {
						row.Error = err.Error()
					}
				}
			}
			row.Seconds = time.Since(started).Seconds()
			row.Quality.Passed = row.Quality.Passed && row.Error == "" && row.Audit.Complete && row.Audit.CoverageComplete && row.Audit.AuthoringAllowed
			// Move finished evidence out of the next worker's temporary
			// neighborhood. Preserve bytes; native logs retain original paths.
			archive := filepath.Join(output, "trials", fmt.Sprintf("trial-%03d-%s", trialCount, c.ID))
			if err := os.MkdirAll(filepath.Dir(archive), 0755); err != nil {
				return err
			}
			if err := os.Rename(dir, archive); err != nil {
				return fmt.Errorf("archive trial before starting another: %w", err)
			}
			row.OriginalDirectory, row.Directory = dir, archive
			result.Trials = append(result.Trials, row)
			result.Candidates = summarizeCandidates(candidates, result.Trials)
			if err := writeJSON(filepath.Join(output, "report.json"), result); err != nil {
				return err
			}
			gimbal.Set(caseCtx, "trial measurement", jsonText(row))
			if err := caseCtx.Err(); err != nil {
				return err
			}
		}
		gimbal.Set(taskCtx, "development measurements", jsonText(result.Trials))
		if phase == "holdout" {
			heldout = true
			break
		}
	}
	if err := loop.Err(); err != nil {
		return err
	}
	if snapshot, err := registry.Snapshot(parentID); err == nil {
		result.EvaluationCostUSD, result.EvaluationCostKnown = researchCost(snapshot)
	}
	result.Fulfilled = heldout && result.Selected != ""
	for _, trial := range result.Trials {
		if trial.Split == "holdout" {
			result.Fulfilled = result.Fulfilled && trial.Quality.Passed
		}
	}
	if err := writeJSON(filepath.Join(output, "report.json"), result); err != nil {
		return err
	}
	if !result.Fulfilled {
		return fmt.Errorf("research evaluation promise unfulfilled; see %s", filepath.Join(output, "report.json"))
	}
	return nil
}

const plannerRules = `Use task.Name exactly equal to an allowed candidate ID, or select. Include each name only once in the current backlog; repeat a candidate by selecting that same name in a later plan, not by adding duplicate names. Research_model controls source collection and topic indexing; index_model controls combined-index curation, claim extraction and repair. Start with a cheap baseline, then make controlled comparisons changing one role while holding the other fixed. Include Terra and Sonnet when testing whether stronger indexing pays for itself. Use per-role usage, summed turn time, audit repairs and independent quality to identify where additional model capability helps; tokens and time measure workload, not cognitive difficulty by themselves. Explore the three native provider families within the budget and reserve rounds to repeat promising combinations. Do not exhaust the Cartesian product. A candidate needs at least two complete repetitions of every development case to be eligible. Each candidate dispatch runs all development cases. Failed trials remain in its denominator. Use reported counts and rates, not a one-off success, to choose repeats. Actual quality and price measurements determine eligibility; never treat a provider failure as proof of bad reasoning. End exploration by dispatching select, not an empty backlog: code freezes the cheapest eligible candidate and performs the withheld evaluation. The final holdout is never a tuning target. Do not use tools, edit any files, change thresholds or answers, or claim success without the code's holdout result. First-pass audit cleanliness, one-repair cleanliness and final independent correctness are separate measures.`
const qualityPrompt = `Read the assessment gold, original sources, trial document, and semantic index including its topic indexes and clips. Independently assess every required gold fact in both document and index: preserve units, versions, conditions, attribution and unresolved source disagreements. Read the original evidence, not the operational audit's conclusions. Count factual assertions in the document and generated index prose/clips, excluding copied original sources from the index assertion count, and list unsupported or contradicted assertions, missing consequential qualifications, and hidden source disagreements. A fact is index-covered only when a reader can find it or its precise evidence through INDEX.md links. Return exactly one grade for every gold fact ID with reasons. Do not edit files. Do not treat a Jev pass as independent proof.`
const readerPrompt = `Answer the question using only the supplied read passages. Do not call tools or access files yourself. Begin at INDEX.md and request up to two paths per turn in Paths to navigate links into original sources. Paths may be relative to the corpus or absolute links inside it. The workflow supplies those files subject to six total reads and 18000 bytes. When ready, return no Paths, your answer and exact quotes from original files already read, with their relative paths. Summaries are navigation aids, not original citations. Preserve version, unit and time scope. Report both sides of an unresolved disagreement. If the supplied corpus does not establish the answer, abstain without invented facts or citations. You have at most four turns.`
const answerPrompt = `Independently compare the reader answer with the query gold and retrieved original passages. Correct requires every part of the question, correct scope and relationships, and explicit unresolved disagreement where appropriate; merely containing the expected numbers is insufficient. For an unanswerable query, justified abstention is correct. Grounded means every factual part follows from the cited original evidence; an appropriate abstention needs no citation. Do not edit files or use the operational audit as proof.`
const assessmentContext = `{{range .Values}}{{if or (eq .Key "assessment gold") (eq .Key "original source directory") (eq .Key "trial document") (eq .Key "trial corpus")}}{{.Key}}: {{.Value}}
{{end}}{{end}}`
const readerContext = `{{range .Values}}{{if or (eq .Key "question") (eq .Key "read passages") (eq .Key "remaining file reads")}}{{.Key}}: {{.Value}}
{{end}}{{end}}`
const answerContext = `{{range .Values}}{{if or (eq .Key "query gold") (eq .Key "reader answer") (eq .Key "retrieved passages")}}{{.Key}}: {{.Value}}
{{end}}{{end}}`

func absoluteFrom(root, name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("empty path")
	}
	if !filepath.IsAbs(name) {
		name = filepath.Join(root, name)
	}
	return filepath.Abs(name)
}
func jsonText(v any) string { b, _ := json.Marshal(v); return string(b) }
func inside(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
func materializeSuite(dir string) error {
	return fs.WalkDir(bundledSuite, "suite", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel("suite", path)
		target := filepath.Join(dir, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := bundledSuite.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
}
func loadCandidates(root string, file polytype.Optional[string]) ([]candidate, error) {
	models := []struct{ id, model string }{
		{"flash", "gemini-3.8-flash-medium"}, {"luna", "gpt-5.6-luna"},
		{"terra", "gpt-5.6-terra"}, {"haiku", "claude-haiku-4-5"}, {"sonnet", "claude-sonnet-5-5"},
	}
	var list []candidate
	for _, research := range models {
		for _, index := range models {
			id := research.id
			if research.id != index.id {
				id += "-" + index.id
			}
			list = append(list, candidate{id, research.model, index.model})
		}
	}
	if file.Present {
		path, err := absoluteFrom(root, file.Value)
		if err != nil {
			return nil, err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(b, &list); err != nil {
			return nil, err
		}
	}
	seen := map[string]bool{}
	for _, c := range list {
		if c.ID == "" || c.ID == "select" || seen[c.ID] || c.ResearchModel == "" || c.IndexModel == "" {
			return nil, fmt.Errorf("invalid candidate %q", c.ID)
		}
		seen[c.ID] = true
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("no candidates")
	}
	return list, nil
}
func candidateByID(list []candidate, id string) (candidate, bool) {
	for _, c := range list {
		if c.ID == id {
			return c, true
		}
	}
	return candidate{}, false
}
func bestCandidate(list []candidate, trials []trialResult, s suite) (candidate, bool) {
	var best candidate
	bestCost := 0.0
	bestRepairs := 0.0
	found := false
	for _, c := range list {
		seen := map[string]int{}
		cost, repairs, count := 0.0, 0.0, 0
		eligible := true
		for _, t := range trials {
			if t.Split != "development" || t.Candidate.ID != c.ID {
				continue
			}
			count++
			seen[t.Case]++
			eligible = eligible && t.Quality.Passed && t.CostKnown && t.Error == ""
			cost += t.CostUSD
			repairs += float64(t.AuditRepairPasses)
		}
		for _, rc := range s.Cases {
			if rc.Split == "development" && seen[rc.ID] < 2 {
				eligible = false
			}
		}
		if !eligible || count == 0 {
			continue
		}
		cost /= float64(count)
		repairs /= float64(count)
		if repairs > 1 {
			continue
		}
		if !found || cost < bestCost || (cost == bestCost && repairs < bestRepairs) {
			best, bestCost, bestRepairs, found = c, cost, repairs, true
		}
	}
	return best, found
}
func verifyProject(dir string, registry *observation.Registry) error {
	entries, err := os.ReadDir(filepath.Join(dir, ".gimbal", "runs"))
	if err != nil {
		return fmt.Errorf("work-dir must be the owning hosted project: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		snap, err := registry.Snapshot(entry.Name())
		if err == nil && snap.Run.Status == observation.StatusRunning {
			return nil
		}
	}
	return fmt.Errorf("work-dir is not an observed running project; use equal --project and --work-dir")
}
func waitResearch(ctx context.Context, registry *observation.Registry, controls *live.Runs, id string) (observation.RunSnapshot, error) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	cancelled := false
	for {
		snapshot, err := registry.Snapshot(id)
		if err != nil {
			return snapshot, fmt.Errorf("accepted research %s is not in the parent's project: %w", id, err)
		}
		if snapshot.Run.Status != observation.StatusRunning {
			if cancelled {
				return snapshot, context.Cause(ctx)
			}
			if snapshot.Run.Status != observation.StatusCompleted {
				return snapshot, fmt.Errorf("research %s ended %s: %s", id, snapshot.Run.Status, snapshot.Run.Error)
			}
			return snapshot, nil
		}
		if ctx.Err() != nil && !cancelled {
			controller, err := controls.InProgress(id)
			if err == nil {
				if err := controller.CancelScope("", context.Cause(ctx)); err != nil {
					return snapshot, err
				}
			}
			cancelled = true
		}
		<-ticker.C
	}
}
func researchCost(snapshot observation.RunSnapshot) (float64, bool) {
	var cost float64
	known := len(snapshot.Turns) > 0
	for id := range snapshot.Turns {
		if len(snapshot.TurnUsage[id]) == 0 {
			known = false
		}
		for model, usage := range snapshot.TurnUsage[id] {
			part, priced := observation.TotalCost(observation.Total{ByModel: map[string]observation.Usage{model: usage}})
			cost += part
			known = known && priced
		}
	}
	return cost, known
}

const priceBasis = "Catalog prices or harness-stated cost. These are usage-price proxies, not subscription invoices. Unpriced models or absent usage remain unknown."

func auditHistory(corpus string) (bool, bool, int, int64) {
	paths, _ := filepath.Glob(filepath.Join(corpus, ".semantic-index", "history", "*-pass-*.json"))
	first, one := false, false
	var tokens int64
	phaseRepairs := map[string]int{}
	for _, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var a auditSummary
		if json.Unmarshal(b, &a) != nil {
			continue
		}
		tokens += a.Metrics.InputTokens
		phase, _, _ := strings.Cut(filepath.Base(path), "-pass-")
		phaseRepairs[phase] = max(phaseRepairs[phase], a.Metrics.RepairPass)
		if !strings.HasPrefix(filepath.Base(path), "initial-pass-") {
			continue
		}
		clean := a.Complete && a.CoverageComplete && a.AuthoringAllowed && a.Metrics.SourceFindings == 0 && a.Metrics.PairFindings == 0 && a.Metrics.ExtractionFindings == 0
		if a.Metrics.RepairPass == 0 {
			first = clean
		}
		if a.Metrics.RepairPass <= 1 {
			one = one || clean
		}
	}
	repairs := 0
	for _, passes := range phaseRepairs {
		repairs += passes
	}
	return first, one, repairs, tokens
}

func reportRun(dir string, registry *observation.Registry, reportPath string) (string, error) {
	entries, err := os.ReadDir(filepath.Join(dir, ".gimbal", "runs"))
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		snap, err := registry.Snapshot(entry.Name())
		if err != nil {
			continue
		}
		var value string
		_ = json.Unmarshal(snap.Scopes[""].Values["evaluation report"].Value, &value)
		if value == reportPath {
			return snap.Run.ID, nil
		}
	}
	return "", fmt.Errorf("current evaluation record unavailable")
}

func readerTelemetry(before, after observation.RunSnapshot) error {
	observed := false
	for id, turn := range after.Turns {
		if _, old := before.Turns[id]; old {
			continue
		}
		if after.Sessions[turn.Session].Name != "research-eval-reading" {
			continue
		}
		transcript, ok := after.Transcripts[id]
		if !ok || len(transcript.Snapshot.State.Message) == 0 {
			return fmt.Errorf("reader tool telemetry unavailable")
		}
		observed = true
		data, _ := json.Marshal(transcript.Snapshot)
		var tree any
		_ = json.Unmarshal(data, &tree)
		if containsTool(tree) {
			return fmt.Errorf("reader used tools outside the measured navigation protocol")
		}
	}
	if !observed {
		return fmt.Errorf("no reader turn observed")
	}
	return nil
}
func containsTool(v any) bool {
	switch x := v.(type) {
	case map[string]any:
		if x["type"] == "tool" || x["type"] == "tool-invocation" {
			// Claude emits the structured return value as this native tool.
			// It completes the turn without reading anything outside the protocol.
			return x["name"] != "StructuredOutput"
		}
		for _, child := range x {
			if containsTool(child) {
				return true
			}
		}
	case []any:
		if slices.ContainsFunc(x, containsTool) {
			return true
		}
	}
	return false
}

func copySources(dir string, originals map[string]string) error {
	for name, body := range originals {
		path, err := within(dir, name)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			return err
		}
	}
	return nil
}

type candidateSummary struct {
	ID                    string  `json:"id"`
	Trials                int     `json:"trials"`
	Passed                int     `json:"quality_passes"`
	InitialAuditClean     int     `json:"initial_audit_clean"`
	AtMostOneRepair       int     `json:"quality_passes_with_at_most_one_repair"`
	PassRate              float64 `json:"quality_pass_rate"`
	InitialAuditCleanRate float64 `json:"initial_audit_clean_rate"`
	AtMostOneRepairRate   float64 `json:"quality_pass_with_at_most_one_repair_rate"`
	MeanUSD               float64 `json:"mean_usd"`
	CostKnown             bool    `json:"cost_known"`
}

func summarizeCandidates(candidates []candidate, trials []trialResult) []candidateSummary {
	var result []candidateSummary
	for _, c := range candidates {
		row := candidateSummary{ID: c.ID, CostKnown: true}
		for _, t := range trials {
			if t.Candidate.ID != c.ID || t.Split != "development" {
				continue
			}
			row.Trials++
			if t.Quality.Passed {
				row.Passed++
				if t.AuditRepairPasses <= 1 {
					row.AtMostOneRepair++
				}
			}
			if t.InitialAuditClean {
				row.InitialAuditClean++
			}
			row.MeanUSD += t.CostUSD
			row.CostKnown = row.CostKnown && t.CostKnown
		}
		if row.Trials > 0 {
			n := float64(row.Trials)
			row.PassRate, row.InitialAuditCleanRate, row.AtMostOneRepairRate, row.MeanUSD = float64(row.Passed)/n, float64(row.InitialAuditClean)/n, float64(row.AtMostOneRepair)/n, row.MeanUSD/n
		} else {
			row.CostKnown = false
		}
		result = append(result, row)
	}
	return result
}

func readerPath(corpus, name string) (string, error) {
	name, _, _ = strings.Cut(name, "#")
	if filepath.IsAbs(name) {
		if !inside(corpus, name) {
			return "", fmt.Errorf("path outside corpus")
		}
		return filepath.Clean(name), nil
	}
	return within(corpus, name)
}

func normalizeCitations(corpus string, answer *ReaderStep) {
	for i := range answer.Citations {
		path, err := readerPath(corpus, answer.Citations[i].Path)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(corpus, path)
		if err == nil {
			answer.Citations[i].Path = filepath.ToSlash(rel)
		}
	}
}

type calibrationCounts struct {
	GoodSamples    int `json:"known_good_samples"`
	FalsePositives int `json:"false_positives"`
	BadSamples     int `json:"known_bad_samples"`
	FalseNegatives int `json:"false_negatives"`
}
