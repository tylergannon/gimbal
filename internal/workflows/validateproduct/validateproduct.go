// Package validateproduct runs practical user testing: up to three independent
// workloads in parallel, screenshot-grounding feedback, then one synthesis/issue-
// triage turn. It is a focus group, not an exhaustive feature checklist or source review.
// Testers never inspect the implementation source of the product under test (A).
// If A does work on a second project (B), B's source is permitted but the tester
// should normally rely on A to do that work.
//
// Supply a JSON/YAML suite with product, guides (local user-documentation files),
// output_dir, and one to three workloads. Each workload has name, assignment_file
// (local task/issue text and allowed actions), an existing isolated workdir, url,
// and optional foreground start and ready shell commands. Prepare/build the desired
// product version before invocation. Existing targets are not stopped. A start
// command requires ready, polled for at most 30 seconds. For a CLI-only product,
// start a loopback terminal such as GoTTY and supply its URL. Testers may use shell
// commands as ordinary users, including invoking Gimble to delegate work on B.
//
// The three tester slots are explicit; unused slots do nothing. The caller assigns
// workloads; no planner invents work or retries failures. Each tester saves ordered,
// captioned screenshots and reports task outcome. A follow-up on the tester session
// asks for its three favorite and least favorite aspects of UX and UI separately and
// appends that debrief to user-report.md. Gemini Flash then opens every cited screenshot
// and checks its visible claims. One mismatch gets one correction turn on the tester's
// existing session and one recheck; a remaining mismatch is preserved for triage. Elapsed
// time covers the task, excluding evidence correction and this debrief. Video records
// the browser for optional human review; agents do not analyze it.
// The final agent reads all reports, deduplicates findings against existing GitHub
// issues, and opens actionable issues in issue_repo (owner/repository). Omit
// issue_repo to produce a report without publishing issues. Product defects are
// findings, not workflow execution errors; failed agent turns remain execution errors.
//
// Prerequisites: authenticated harnesses, playwright-cli and its installed browser,
// and authenticated gh when publishing issues. playwright_cli can override the
// driver's executable path. timeout defaults to 1h. All paths resolve from the
// suite file. Output is a unique user-testing-* directory containing reports.json,
// per-tester user-report.md, visual-review.md, screenshots and video.webm, plus
// findings.md. Elapsed time is measured by the workflow. The final command/run
// status includes cleanup errors; files alone do not certify run completion.
// Bounded browser cleanup runs outside cancellation; hard kills cannot guarantee it.
//
// Roles: product-operation defaults to Claude Opus 5, product-visual-review to
// Gemini Flash, and product-triage to GPT-6 Astra. Each has its model override flag.
//
// Example:
//
//	gimble run validate-product --suite-file /abs/user-testing.yaml --no-web
package validateproduct

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tylergannon/gimble"
)

//go:generate go tool polytype --validate
//go:generate go run github.com/tylergannon/gimble/internal/generate/gimblegen -entry ValidateProduct -name validate-product -mermaid ../../../docs-site/src/lib/generated/workflows/validate-product.mmd

type Params struct {
	// SuiteFile names the JSON/YAML product, local workload assignments, and issue repository.
	SuiteFile string
}

type workloadReport struct {
	Name           string  `json:"name"`
	Assignment     string  `json:"assignment"`
	Report         string  `json:"report"`
	VisualReview   string  `json:"visual_review"`
	Video          string  `json:"video"`
	ElapsedSeconds float64 `json:"elapsed_seconds"`
	Error          string  `json:"error,omitempty"`
}

// VisualVerdict says whether Gemini could inspect the cited screenshots and
// whether their visible contents support the report's visual claims.
type VisualVerdict struct {
	// ReviewCompleted is false when Gemini could not open all cited screenshots.
	ReviewCompleted bool `json:"review_completed"`
	// Supported is true only when every inspected screenshot supports its caption, reference, and visible claim.
	Supported bool `json:"supported"`
	// OpenedImages lists the exact paths Gemini opened with its image-viewing tool.
	OpenedImages []string `json:"opened_images"`
	// Feedback identifies each mismatch or evidence limitation precisely enough for correction or triage.
	Feedback string `json:"feedback"`
}

// ValidateProduct runs user workloads, checks their screenshots, and triages findings.
func ValidateProduct(ctx context.Context, env gimble.Env, params Params) (resultErr error) {
	suite, timeout, err := readSuite(absolute(env.WorkDir, params.SuiteFile))
	if err != nil {
		return err
	}
	driver, err := exec.LookPath(suite.PlaywrightCLI)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(suite.OutputDir, 0755); err != nil {
		return err
	}
	output, err := os.MkdirTemp(suite.OutputDir, "user-testing-")
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	reports := make([]workloadReport, len(suite.Workloads))
	dirs, names := make([]string, len(reports)), make([]string, len(reports))
	var turns [3]error
	var opened, recording [3]bool
	reportsFile := filepath.Join(output, "reports.json")
	defer func() { resultErr = errors.Join(resultErr, writeJSON(reportsFile, reports)) }()
	// Explicit teardown is needed because playwright-cli launches a browser daemon.
	closeBrowsers := func() error {
		var failures []error
		for i := range reports {
			if !opened[i] {
				continue
			}
			opened[i] = false
			for _, action := range []string{"video-stop", "close"} {
				if action == "video-stop" && !recording[i] {
					continue
				}
				cleanup, stop := context.WithTimeout(context.Background(), 20*time.Second)
				cmd := exec.CommandContext(cleanup, driver, "-s="+names[i], action)
				cmd.Dir = dirs[i]
				cmd.WaitDelay = 2 * time.Second
				log, err := cmd.CombinedOutput()
				stop()
				if err != nil {
					failures = append(failures, fmt.Errorf("%s %s: %w: %s", reports[i].Name, action, err, log))
				}
				failures = append(failures, os.WriteFile(filepath.Join(dirs[i], action+".log"), log, 0644))
			}
		}
		return errors.Join(failures...)
	}
	defer func() { resultErr = errors.Join(resultErr, closeBrowsers()) }()
	for i, w := range suite.Workloads {
		dirs[i] = filepath.Join(output, fmt.Sprintf("tester%d", i+1))
		names[i] = filepath.Base(output) + fmt.Sprintf("-%d", i+1)
		reports[i] = workloadReport{Name: w.Name, Assignment: w.AssignmentFile, Report: filepath.Join(dirs[i], "user-report.md"), VisualReview: filepath.Join(dirs[i], "visual-review.md"), Video: filepath.Join(dirs[i], "video.webm"), Error: "not run"}
	}
	if err := writeJSON(reportsFile, reports); err != nil {
		return err
	}
	for i, w := range suite.Workloads {
		if err := os.MkdirAll(dirs[i], 0755); err != nil {
			return err
		}
		if w.Start != "" {
			if err := gimble.Service(ctx, "product", w.Workdir, w.Start); err != nil {
				return err
			}
		}
		if w.Ready != "" {
			ready, stop := context.WithTimeout(ctx, 30*time.Second)
			code, _, stderr, err := gimble.RunCommand(ready, "readiness", w.Workdir, "zsh", "-c", "until ( "+w.Ready+"\n); do sleep 0.25; done")
			stop()
			if err != nil || code != 0 {
				return fmt.Errorf("%s readiness: %w", w.Name, errors.Join(err, fmt.Errorf("exit %d: %s", code, stderr)))
			}
		}
		opened[i], recording[i] = true, true
		browser := shellQuote(driver) + " -s=" + shellQuote(names[i])
		code, _, stderr, err := gimble.RunCommand(ctx, "record-browser", dirs[i], "zsh", "-c", browser+" open about:blank && "+browser+" video-start "+shellQuote(reports[i].Video)+" --cursor && "+browser+" goto "+shellQuote(w.URL))
		if err != nil || code != 0 {
			return fmt.Errorf("%s browser: %w", w.Name, errors.Join(err, fmt.Errorf("exit %d: %s", code, stderr)))
		}
	}
	gimble.Set(ctx, "product under test", suite.Product)
	gimble.Set(ctx, "product user documentation", suite.Guides)
	users := gimble.Group(ctx, "user-testing")
	users.Go("tester1", func(ctx context.Context) error {
		gimble.Set(ctx, "assignment file", suite.Workloads[0].AssignmentFile)
		gimble.Set(ctx, "screenshots directory", dirs[0])
		gimble.Set(ctx, "browser command", shellQuote(driver)+" -s="+shellQuote(names[0]))
		gimble.Set(ctx, "current user report", reports[0].Report)
		gimble.Set(ctx, "screenshot review", reports[0].VisualReview)
		tester := gimble.NewSession(ctx, "product-operation", suite.Workloads[0].Workdir)
		start := time.Now()
		text, taskErr := tester.Generate[gimble.Text](ctx, userPrompt)
		reports[0].ElapsedSeconds = time.Since(start).Seconds()
		reportErr := os.WriteFile(reports[0].Report, []byte(text), 0644)
		var debriefErr error
		if taskErr == nil && reportErr == nil {
			feedback, err := tester.Generate[gimble.Text](ctx, experiencePrompt)
			debriefErr = err
			if err == nil {
				text += "\n\n" + feedback
				debriefErr = os.WriteFile(reports[0].Report, []byte(text), 0644)
			}
		}
		var reviews []VisualVerdict
		var visualErr error
		if taskErr == nil && reportErr == nil {
			visual := gimble.NewSession(ctx, "product-visual-review", dirs[0])
			for round := range 2 {
				verdict, err := visual.Generate[VisualVerdict](ctx, visualPrompt)
				if err != nil {
					visualErr = err
					break
				}
				reviews = append(reviews, verdict)
				if !verdict.ReviewCompleted {
					visualErr = fmt.Errorf("screenshot review incomplete: %s", verdict.Feedback)
					break
				}
				if verdict.Supported || round == 1 {
					break
				}
				if err := writeVisualReview(reports[0].VisualReview, reviews, nil); err != nil {
					visualErr = err
					break
				}
				revised, err := tester.Generate[gimble.Text](ctx, correctionPrompt)
				if err != nil {
					visualErr = err
					break
				}
				if err := os.WriteFile(reports[0].Report, []byte(revised), 0644); err != nil {
					visualErr = err
					break
				}
			}
		}
		visualErr = errors.Join(visualErr, writeVisualReview(reports[0].VisualReview, reviews, errors.Join(taskErr, reportErr, visualErr)))
		turns[0] = errors.Join(taskErr, reportErr, visualErr, debriefErr)
		reports[0].Error = errorText(turns[0])
		return nil
	})
	users.Go("tester2", func(ctx context.Context) error {
		if len(suite.Workloads) < 2 {
			return nil
		}
		gimble.Set(ctx, "assignment file", suite.Workloads[1].AssignmentFile)
		gimble.Set(ctx, "screenshots directory", dirs[1])
		gimble.Set(ctx, "browser command", shellQuote(driver)+" -s="+shellQuote(names[1]))
		gimble.Set(ctx, "current user report", reports[1].Report)
		gimble.Set(ctx, "screenshot review", reports[1].VisualReview)
		tester := gimble.NewSession(ctx, "product-operation", suite.Workloads[1].Workdir)
		start := time.Now()
		text, taskErr := tester.Generate[gimble.Text](ctx, userPrompt)
		reports[1].ElapsedSeconds = time.Since(start).Seconds()
		reportErr := os.WriteFile(reports[1].Report, []byte(text), 0644)
		var debriefErr error
		if taskErr == nil && reportErr == nil {
			feedback, err := tester.Generate[gimble.Text](ctx, experiencePrompt)
			debriefErr = err
			if err == nil {
				text += "\n\n" + feedback
				debriefErr = os.WriteFile(reports[1].Report, []byte(text), 0644)
			}
		}
		var reviews []VisualVerdict
		var visualErr error
		if taskErr == nil && reportErr == nil {
			visual := gimble.NewSession(ctx, "product-visual-review", dirs[1])
			for round := range 2 {
				verdict, err := visual.Generate[VisualVerdict](ctx, visualPrompt)
				if err != nil {
					visualErr = err
					break
				}
				reviews = append(reviews, verdict)
				if !verdict.ReviewCompleted {
					visualErr = fmt.Errorf("screenshot review incomplete: %s", verdict.Feedback)
					break
				}
				if verdict.Supported || round == 1 {
					break
				}
				if err := writeVisualReview(reports[1].VisualReview, reviews, nil); err != nil {
					visualErr = err
					break
				}
				revised, err := tester.Generate[gimble.Text](ctx, correctionPrompt)
				if err != nil {
					visualErr = err
					break
				}
				if err := os.WriteFile(reports[1].Report, []byte(revised), 0644); err != nil {
					visualErr = err
					break
				}
			}
		}
		visualErr = errors.Join(visualErr, writeVisualReview(reports[1].VisualReview, reviews, errors.Join(taskErr, reportErr, visualErr)))
		turns[1] = errors.Join(taskErr, reportErr, visualErr, debriefErr)
		reports[1].Error = errorText(turns[1])
		return nil
	})
	users.Go("tester3", func(ctx context.Context) error {
		if len(suite.Workloads) < 3 {
			return nil
		}
		gimble.Set(ctx, "assignment file", suite.Workloads[2].AssignmentFile)
		gimble.Set(ctx, "screenshots directory", dirs[2])
		gimble.Set(ctx, "browser command", shellQuote(driver)+" -s="+shellQuote(names[2]))
		gimble.Set(ctx, "current user report", reports[2].Report)
		gimble.Set(ctx, "screenshot review", reports[2].VisualReview)
		tester := gimble.NewSession(ctx, "product-operation", suite.Workloads[2].Workdir)
		start := time.Now()
		text, taskErr := tester.Generate[gimble.Text](ctx, userPrompt)
		reports[2].ElapsedSeconds = time.Since(start).Seconds()
		reportErr := os.WriteFile(reports[2].Report, []byte(text), 0644)
		var debriefErr error
		if taskErr == nil && reportErr == nil {
			feedback, err := tester.Generate[gimble.Text](ctx, experiencePrompt)
			debriefErr = err
			if err == nil {
				text += "\n\n" + feedback
				debriefErr = os.WriteFile(reports[2].Report, []byte(text), 0644)
			}
		}
		var reviews []VisualVerdict
		var visualErr error
		if taskErr == nil && reportErr == nil {
			visual := gimble.NewSession(ctx, "product-visual-review", dirs[2])
			for round := range 2 {
				verdict, err := visual.Generate[VisualVerdict](ctx, visualPrompt)
				if err != nil {
					visualErr = err
					break
				}
				reviews = append(reviews, verdict)
				if !verdict.ReviewCompleted {
					visualErr = fmt.Errorf("screenshot review incomplete: %s", verdict.Feedback)
					break
				}
				if verdict.Supported || round == 1 {
					break
				}
				if err := writeVisualReview(reports[2].VisualReview, reviews, nil); err != nil {
					visualErr = err
					break
				}
				revised, err := tester.Generate[gimble.Text](ctx, correctionPrompt)
				if err != nil {
					visualErr = err
					break
				}
				if err := os.WriteFile(reports[2].Report, []byte(revised), 0644); err != nil {
					visualErr = err
					break
				}
			}
		}
		visualErr = errors.Join(visualErr, writeVisualReview(reports[2].VisualReview, reviews, errors.Join(taskErr, reportErr, visualErr)))
		turns[2] = errors.Join(taskErr, reportErr, visualErr, debriefErr)
		reports[2].Error = errorText(turns[2])
		return nil
	})
	groupErr := users.Wait()
	recordingErr := closeBrowsers()
	if err := writeJSON(reportsFile, reports); err != nil {
		return errors.Join(groupErr, recordingErr, err)
	}
	if ctx.Err() != nil {
		return errors.Join(ctx.Err(), groupErr, recordingErr, errors.Join(turns[:]...))
	}
	gimble.Set(ctx, "workload reports", reportsFile)
	gimble.Set(ctx, "execution errors", errorText(errors.Join(groupErr, recordingErr, errors.Join(turns[:]...))))
	gimble.Set(ctx, "issue repository", suite.IssueRepo)
	triage := gimble.NewSession(ctx, "product-triage", output)
	findings, triageErr := triage.Generate[gimble.Text](ctx, triagePrompt)
	triageErr = errors.Join(triageErr, os.WriteFile(filepath.Join(output, "findings.md"), []byte(findings), 0644))
	return errors.Join(groupErr, recordingErr, errors.Join(turns[:]...), triageErr)
}

func writeVisualReview(path string, reviews []VisualVerdict, reviewErr error) error {
	var body strings.Builder
	body.WriteString("# Screenshot review\n\n")
	for i, review := range reviews {
		fmt.Fprintf(&body, "## Round %d\n\nReview completed: %t\n\nClaims supported: %t\n\nOpened images:\n", i+1, review.ReviewCompleted, review.Supported)
		for _, image := range review.OpenedImages {
			fmt.Fprintf(&body, "- %s\n", image)
		}
		fmt.Fprintf(&body, "\n%s\n\n", review.Feedback)
	}
	if len(reviews) == 0 {
		body.WriteString("No screenshot review completed.\n")
	}
	if reviewErr != nil {
		fmt.Fprintf(&body, "\nReview error: %s\n", reviewErr)
	}
	return os.WriteFile(path, []byte(body.String()), 0644)
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path+".tmp", data, 0644); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}
func errorText(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}
