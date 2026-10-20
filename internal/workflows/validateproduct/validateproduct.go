// Package validateproduct runs practical user testing: up to three independent
// workloads in parallel, one screenshot review, then one synthesis/issue-triage
// turn. It is a focus group, not an exhaustive feature checklist or source review.
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
// commands as ordinary users, including invoking Gimbal to delegate work on B.
//
// The three tester slots are explicit; unused slots do nothing. The caller assigns
// workloads; no planner invents work or retries failures. Each tester saves ordered,
// captioned screenshots and reports task outcome. One follow-up on the same session
// asks for its three favorite and least favorite aspects of UX and UI separately,
// appended to user-report.md. Elapsed time covers the task, excluding this debrief.
// Video records the browser for optional human review; agents do not analyze it.
// After recording stops, ffmpeg makes a 2.5x H.264 MP4 capped at 1280x720 for
// browser playback and upload with gimbal upload-artifact.
// Gemini Flash opens screenshots to check readability and claims, not to repeat the workload.
// The final agent reads all reports, deduplicates findings against existing GitHub
// issues, uploads supporting screenshots, and opens actionable issues in the
// required issue_repo (owner/repository). Product defects are findings, not
// workflow execution errors; failed agent turns remain execution errors.
//
// Prerequisites: authenticated harnesses, playwright-cli and its installed browser,
// ffmpeg with libx264,
// authenticated gh, and a configured gimbal upload-artifact destination.
// playwright_cli can override the driver's executable path. All paths resolve
// from the suite file. Output is a unique
// user-testing-* directory containing reports.json,
// per-tester user-report.md, screenshots, raw video.webm and processed video.mp4,
// visual-review.md and
// findings.md. Elapsed time is measured by the workflow. The final command/run
// status includes cleanup errors; files alone do not certify run completion.
// Browser cleanup follows explicit completion and error paths.
//
// Roles: product-operation defaults to Claude Opus 5.5, product-visual-review to
// Gemini Flash, and product-triage to GPT-6 Astra. Each has its model override flag.
//
// Example:
//
//	gimbal run validate-product --suite-file /abs/user-testing.yaml --instance-dir /abs/instance --project /abs/project
package validateproduct

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tylergannon/gimbal"
)

//go:generate go run github.com/tylergannon/gimbal/internal/generate/gimbalgen -entry ValidateProduct -name validate-product

type Params struct {
	// SuiteFile names the JSON/YAML product, local workload assignments, and issue repository.
	SuiteFile string
}

type workloadReport struct {
	Name           string  `json:"name"`
	Assignment     string  `json:"assignment"`
	Report         string  `json:"report"`
	Video          string  `json:"video"`
	ElapsedSeconds float64 `json:"elapsed_seconds"`
	Error          string  `json:"error,omitempty"`
}

// ValidateProduct runs user workloads, checks their screenshots, and triages findings.
func ValidateProduct(ctx context.Context, env gimbal.Env, params Params) (resultErr error) {
	suite, err := readSuite(ctx, absolute(env.WorkDir, params.SuiteFile))
	if err != nil {
		return err
	}
	code, paths, stderr, err := gimbal.RunCommand(ctx, "prepare-output", env.WorkDir, "sh", "-c", `set -eu
 command -v "$1"
 command -v ffmpeg
 mkdir -p -- "$2"
 mktemp -d "$2/user-testing-XXXXXXXX"`, "prepare-output", suite.PlaywrightCLI, suite.OutputDir)
	if err != nil || code != 0 {
		return fmt.Errorf("prepare output: exit %d: %s: %w", code, stderr, err)
	}
	resolved := strings.Split(strings.TrimSpace(paths), "\n")
	if len(resolved) != 3 {
		return fmt.Errorf("invalid executable/output paths")
	}
	driver, ffmpeg, output := resolved[0], resolved[1], resolved[2]
	reports := make([]workloadReport, len(suite.Workloads))
	dirs, names := make([]string, len(reports)), make([]string, len(reports))
	var turns [3]error
	var opened, recording [3]bool
	reportsFile := filepath.Join(output, "reports.json")
	resultErr = gimbal.Scope(ctx, "testing", func(ctx context.Context) error {
		for i, w := range suite.Workloads {
			dirs[i] = filepath.Join(output, fmt.Sprintf("tester%d", i+1))
			names[i] = filepath.Base(output) + fmt.Sprintf("-%d", i+1)
			reports[i] = workloadReport{Name: w.Name, Assignment: w.AssignmentFile, Report: filepath.Join(dirs[i], "user-report.md"), Video: filepath.Join(dirs[i], "video.mp4"), Error: "not run"}
		}
		if err := writeJSON(ctx, reportsFile, reports); err != nil {
			return err
		}
		// Prepare the browsers
		// Start each product and open a recorded browser for its assigned workload.
		for i, w := range suite.Workloads {
			if err := makeDirectory(ctx, dirs[i], 0755); err != nil {
				return err
			}
			// Start command?
			// Start the product only when this workload provides a command.
			// When true: Command provided
			if w.Start != "" {
				// Start the product
				if err := gimbal.Service(ctx, "product", w.Workdir, readyServiceCommand(w.Start, w.Ready)); err != nil {
					return err
				}
			}
			// Readiness check?
			// Wait for the product when this workload provides a readiness check.
			// When true: Check provided
			if w.Ready != "" && w.Start == "" {
				// Wait until ready
				code, _, stderr, err := gimbal.RunCommand(ctx, "readiness", w.Workdir, "zsh", "-c", "until ( "+w.Ready+"\n); do sleep 0.25; done")
				// Readiness check failed?
				// When true: Check failed
				if err != nil || code != 0 {
					return fmt.Errorf("%s readiness: %w", w.Name, errors.Join(err, fmt.Errorf("exit %d: %s", code, stderr)))
				}
			}
			opened[i], recording[i] = true, true
			browser := shellQuote(driver) + " -s=" + shellQuote(names[i])
			// Open and record the browser
			code, _, stderr, err := gimbal.RunCommand(ctx, "record-browser", dirs[i], "zsh", "-c", browser+" open about:blank && "+browser+" video-start "+shellQuote(filepath.Join(dirs[i], "video.webm"))+" --cursor && "+browser+" goto "+shellQuote(w.URL))
			// Browser setup failed?
			// When true: Setup failed
			if err != nil || code != 0 {
				return fmt.Errorf("%s browser: %w", w.Name, errors.Join(err, fmt.Errorf("exit %d: %s", code, stderr)))
			}
		}
		// Product brief
		// Give every tester the same description of the product under review.
		gimbal.Set(ctx, "product under test", suite.Product)
		gimbal.Set(ctx, "product user documentation", suite.Guides)
		// Test in parallel
		// Each tester follows its own assignment in a separate browser.
		users := gimbal.Group(ctx, "user-testing")
		// Tester 1
		// Follow workload 1 and report the experience of using the product.
		users.Go("tester1", func(ctx context.Context) error {
			gimbal.Set(ctx, "assignment file", suite.Workloads[0].AssignmentFile)
			gimbal.Set(ctx, "screenshots directory", dirs[0])
			gimbal.Set(ctx, "browser command", shellQuote(driver)+" -s="+shellQuote(names[0]))
			tester := gimbal.NewSession(ctx, "product-operation", suite.Workloads[0].Workdir)
			start, err := clock(ctx)
			if err != nil {
				return err
			}
			// Try the assigned task
			// Use the product in the browser and record what happens.
			text, err := tester.Generate[gimbal.Text](ctx, userPrompt)
			finished, clockErr := clock(ctx)
			if clockErr != nil {
				return clockErr
			}
			reports[0].ElapsedSeconds = finished - start
			// If the tester responded
			// Collect feedback when the task turn returns without an error.
			if err == nil {
				// Describe the experience
				// Explain what was clear, confusing, or difficult during the task.
				feedback, feedbackErr := tester.Generate[gimbal.Text](ctx, experiencePrompt)
				text += "\n\n" + feedback
				err = feedbackErr
			}
			turns[0] = errors.Join(err, writeFile(ctx, reports[0].Report, []byte(text), 0644))
			reports[0].Error = errorText(turns[0])
			return nil
		})
		// Tester 2
		// Follow workload 2 and report the experience of using the product.
		users.Go("tester2", func(ctx context.Context) error {
			if len(suite.Workloads) < 2 {
				return nil
			}
			gimbal.Set(ctx, "assignment file", suite.Workloads[1].AssignmentFile)
			gimbal.Set(ctx, "screenshots directory", dirs[1])
			gimbal.Set(ctx, "browser command", shellQuote(driver)+" -s="+shellQuote(names[1]))
			tester := gimbal.NewSession(ctx, "product-operation", suite.Workloads[1].Workdir)
			start, err := clock(ctx)
			if err != nil {
				return err
			}
			// Try the assigned task
			// Use the product in the browser and record what happens.
			text, err := tester.Generate[gimbal.Text](ctx, userPrompt)
			finished, clockErr := clock(ctx)
			if clockErr != nil {
				return clockErr
			}
			reports[1].ElapsedSeconds = finished - start
			// If the tester responded
			// Collect feedback when the task turn returns without an error.
			if err == nil {
				// Describe the experience
				// Explain what was clear, confusing, or difficult during the task.
				feedback, feedbackErr := tester.Generate[gimbal.Text](ctx, experiencePrompt)
				text += "\n\n" + feedback
				err = feedbackErr
			}
			turns[1] = errors.Join(err, writeFile(ctx, reports[1].Report, []byte(text), 0644))
			reports[1].Error = errorText(turns[1])
			return nil
		})
		// Tester 3
		// Follow workload 3 and report the experience of using the product.
		users.Go("tester3", func(ctx context.Context) error {
			if len(suite.Workloads) < 3 {
				return nil
			}
			gimbal.Set(ctx, "assignment file", suite.Workloads[2].AssignmentFile)
			gimbal.Set(ctx, "screenshots directory", dirs[2])
			gimbal.Set(ctx, "browser command", shellQuote(driver)+" -s="+shellQuote(names[2]))
			tester := gimbal.NewSession(ctx, "product-operation", suite.Workloads[2].Workdir)
			start, err := clock(ctx)
			if err != nil {
				return err
			}
			// Try the assigned task
			// Use the product in the browser and record what happens.
			text, err := tester.Generate[gimbal.Text](ctx, userPrompt)
			finished, clockErr := clock(ctx)
			if clockErr != nil {
				return clockErr
			}
			reports[2].ElapsedSeconds = finished - start
			// If the tester responded
			// Collect feedback when the task turn returns without an error.
			if err == nil {
				// Describe the experience
				// Explain what was clear, confusing, or difficult during the task.
				feedback, feedbackErr := tester.Generate[gimbal.Text](ctx, experiencePrompt)
				text += "\n\n" + feedback
				err = feedbackErr
			}
			turns[2] = errors.Join(err, writeFile(ctx, reports[2].Report, []byte(text), 0644))
			reports[2].Error = errorText(turns[2])
			return nil
		})
		groupErr := users.Wait()
		recordingErr := closeBrowsers(ctx, driver, dirs, names, reports, &opened, &recording)
		var videoErr error
		for i := range reports {
			code, stdout, stderr, err := gimbal.RunCommand(ctx, "encode-video", dirs[i], ffmpeg, "-y", "-i", filepath.Join(dirs[i], "video.webm"),
				"-vf", "setpts=PTS/2.5,fps=25,scale=w='min(1280,iw)':h='min(720,ih)':force_original_aspect_ratio=decrease:force_divisible_by=2",
				"-an", "-c:v", "libx264", "-preset", "medium", "-crf", "23", "-pix_fmt", "yuv420p", "-movflags", "+faststart", reports[i].Video)
			log := stdout + stderr
			if code != 0 && err == nil {
				err = fmt.Errorf("exit %d", code)
			}
			if err != nil {
				videoErr = errors.Join(videoErr, fmt.Errorf("%s video conversion: %w: %s", reports[i].Name, err, log))
			}
		}
		if err := writeJSON(ctx, reportsFile, reports); err != nil {
			return errors.Join(groupErr, recordingErr, videoErr, err)
		}

		gimbal.Set(ctx, "workload reports", reportsFile)
		gimbal.Set(ctx, "execution errors", errorText(errors.Join(groupErr, recordingErr, videoErr, errors.Join(turns[:]...))))
		visual := gimbal.NewSession(ctx, "product-visual-review", output)
		// Review the screenshots
		// Inspect the captured screens for visual and usability problems.
		visualText, visualErr := visual.Generate[gimbal.Text](ctx, visualPrompt)
		visualFile := filepath.Join(output, "visual-review.md")
		visualErr = errors.Join(visualErr, writeFile(ctx, visualFile, []byte(visualText), 0644))
		gimbal.Set(ctx, "screenshot review", visualFile)
		gimbal.Set(ctx, "screenshot review error", errorText(visualErr))
		gimbal.Set(ctx, "issue repository", suite.IssueRepo)
		triage := gimbal.NewSession(ctx, "product-triage", output)
		// Triage the findings
		// Combine tester reports and visual review into actionable findings.
		findings, triageErr := triage.Generate[gimbal.Text](ctx, triagePrompt)
		triageErr = errors.Join(triageErr, writeFile(ctx, filepath.Join(output, "findings.md"), []byte(findings), 0644))
		return errors.Join(groupErr, recordingErr, videoErr, errors.Join(turns[:]...), visualErr, triageErr)
	})
	return errors.Join(resultErr, closeBrowsers(ctx, driver, dirs, names, reports, &opened, &recording), writeJSON(ctx, reportsFile, reports))
}

func writeJSON(ctx context.Context, path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(ctx, path, append(data, '\n'), 0644)
}
func writeFile(ctx context.Context, path string, data []byte, _ uint32) error {
	code, _, stderr, err := gimbal.RunCommand(ctx, "write-file", "/", "sh", "-c", `printf '%s' "$2" > "$1.tmp" && mv -- "$1.tmp" "$1"`, "write-file", path, string(data))
	if err != nil || code != 0 {
		return fmt.Errorf("write %s: exit %d: %s: %w", path, code, stderr, err)
	}
	return nil
}
func makeDirectory(ctx context.Context, path string, _ uint32) error {
	code, _, stderr, err := gimbal.RunCommand(ctx, "create-directory", "/", "mkdir", "-p", "--", path)
	if err != nil || code != 0 {
		return fmt.Errorf("create %s: exit %d: %s: %w", path, code, stderr, err)
	}
	return nil
}
func clock(ctx context.Context) (float64, error) {
	code, stdout, stderr, err := gimbal.RunCommand(ctx, "read-clock", "/", "node", "-e", "console.log(Number(process.hrtime.bigint())/1e9)")
	if err != nil || code != 0 {
		return 0, fmt.Errorf("clock: exit %d: %s: %w", code, stderr, err)
	}
	return strconv.ParseFloat(strings.TrimSpace(stdout), 64)
}

func errorText(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

// Adapt an operator-supplied program and readiness check to the service contract.
// The foreground wrapper owns its child and publishes readiness only after the
// actual check succeeds; Gimbal still owns the whole process group.
func readyServiceCommand(start, ready string) string {
	return "zsh -c " + shellQuote(start) + ` &
child=$!
trap 'kill -INT "$child" 2>/dev/null; wait "$child"; exit' INT TERM
for attempt in {1..120}; do
 kill -0 "$child" 2>/dev/null || { wait "$child"; exit 1; }
 if ( ` + ready + `
); then
  : > "$ENV_FILE.tmp" && mv "$ENV_FILE.tmp" "$ENV_FILE" || exit 1
  wait "$child"
  exit $?
 fi
 sleep 0.25
done
kill -INT "$child" 2>/dev/null
wait "$child"
exit 1
`
}

func closeBrowsers(ctx context.Context, driver string, dirs, names []string, reports []workloadReport, opened, recording *[3]bool) error {
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
			code, stdout, stderr, err := gimbal.RunCommand(ctx, "close-browser", dirs[i], driver, "-s="+names[i], action)
			if err != nil || code != 0 {
				failures = append(failures, fmt.Errorf("%s %s: exit %d: %s: %s: %w", reports[i].Name, action, code, stdout, stderr, err))
			}
			failures = append(failures, writeFile(ctx, filepath.Join(dirs[i], action+".log"), []byte(stdout+stderr), 0644))
		}
	}
	return errors.Join(failures...)
}
