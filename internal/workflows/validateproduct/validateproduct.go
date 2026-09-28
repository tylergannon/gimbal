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
// The final agent synthesizes local findings. When issue_repo (owner/repository)
// is supplied, it also deduplicates against existing GitHub issues, uploads
// supporting screenshots, and opens actionable issues. Product defects are findings, not
// workflow execution errors; failed agent turns remain execution errors.
//
// Prerequisites in the execution environment: authenticated harnesses, zsh,
// playwright-cli with its configured browser, and ffmpeg with libx264. Publishing
// with issue_repo also needs authenticated gh and a gimbal upload-artifact
// destination. Docker/Temporal requires all three roles to use Codex models.
// Each video encode has a two-minute budget; finalized WebM remains on failure.
// timeout defaults to 1h. All paths resolve from the suite file. Output is a unique
// user-testing-* directory containing reports.json,
// per-tester user-report.md, screenshots, raw video.webm and processed video.mp4,
// visual-review.md and
// findings.md. Elapsed time is measured by the workflow. The final command/run
// status includes cleanup errors; files alone do not certify run completion.
// Bounded browser cleanup runs outside cancellation; hard kills cannot guarantee it.
//
// Roles: product-operation defaults to Claude Opus 5.5, product-visual-review to
// Gemini Flash, and product-triage to GPT-6 Astra. Each has its model override flag.
//
// Example:
//
//	gimbal run validate-product --suite-file /abs/user-testing.yaml --instance-dir /abs/instance --project /abs/project --follow
//
// For Docker/Temporal, prepare the environment tools and mounted worker binary,
// start the instance with --execution-config, then add --product-operation
// gpt-5.6-luna --product-visual-review gpt-5.6-luna --product-triage gpt-5.6-luna.
package validateproduct

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tylergannon/gimbal"
)

//go:generate go run github.com/tylergannon/gimbal/internal/generate/gimbalgen -entry ValidateProduct -name validate-product

type Params struct {
	// SuiteFile names the JSON/YAML product, local workload assignments, and optional issue repository.
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
	suite, timeout, err := readSuite(absolute(env.WorkDir, params.SuiteFile))
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
	dirs := make([]string, len(reports))
	var turns [3]error
	reportsFile := filepath.Join(output, "reports.json")
	defer func() { resultErr = errors.Join(resultErr, writeJSON(reportsFile, reports)) }()
	for i, w := range suite.Workloads {
		dirs[i] = filepath.Join(output, fmt.Sprintf("tester%d", i+1))
		reports[i] = workloadReport{Name: w.Name, Assignment: w.AssignmentFile, Report: filepath.Join(dirs[i], "user-report.md"), Video: filepath.Join(dirs[i], "video.mp4"), Error: "not run"}
	}
	if err := writeJSON(reportsFile, reports); err != nil {
		return err
	}
	for i, w := range suite.Workloads {
		if err := os.MkdirAll(dirs[i], 0755); err != nil {
			return err
		}
		if w.Start != "" {
			if err := gimbal.Service(ctx, "product", w.Workdir, w.Start); err != nil {
				return err
			}
		}
		if w.Ready != "" {
			ready, stop := context.WithTimeout(ctx, 30*time.Second)
			code, _, stderr, err := gimbal.RunCommand(ready, "readiness", w.Workdir, "zsh", "-c", "until ( "+w.Ready+"\n); do sleep 0.25; done")
			stop()
			if err != nil || code != 0 {
				return fmt.Errorf("%s readiness: %w", w.Name, errors.Join(err, fmt.Errorf("exit %d: %s", code, stderr)))
			}
		}

	}
	inputs := append([]string{"-c", `for f; do test -r "$f" || { echo "not readable in the environment: $f" >&2; exit 1; }; done`, "zsh"}, suite.Guides...)
	for _, w := range suite.Workloads {
		inputs = append(inputs, w.AssignmentFile)
	}
	code, _, stderr, err := gimbal.RunCommand(ctx, "check-inputs", output, "zsh", inputs...)
	if err != nil || code != 0 {
		return fmt.Errorf("check evaluator inputs: %w", errors.Join(err, fmt.Errorf("exit %d: %s", code, stderr)))
	}
	gimbal.Set(ctx, "product under test", suite.Product)
	gimbal.Set(ctx, "product user documentation", suite.Guides)
	users := gimbal.Group(ctx, "user-testing")
	users.Go("tester1", func(ctx context.Context) error {
		gimbal.Set(ctx, "assignment file", suite.Workloads[0].AssignmentFile)
		gimbal.Set(ctx, "screenshots directory", dirs[0])
		gimbal.Set(ctx, "product URL", suite.Workloads[0].URL)
		var taskErr error
		scopeErr := gimbal.Scope(ctx, "browser-session", func(ctx context.Context) error {
			browser, err := gimbal.NewBrowser(ctx, "browser", dirs[0], filepath.Join(dirs[0], "video.webm"))
			if err != nil {
				return err
			}
			tester := gimbal.NewSession(ctx, "product-operation", suite.Workloads[0].Workdir)
			start := time.Now()
			text, err := tester.Generate[gimbal.Text](ctx, userPrompt, gimbal.WithBrowser(browser))
			reports[0].ElapsedSeconds = time.Since(start).Seconds()
			if err == nil {
				feedback, feedbackErr := tester.Generate[gimbal.Text](ctx, experiencePrompt)
				text += "\n\n" + feedback
				err = feedbackErr
			}
			taskErr = errors.Join(err, os.WriteFile(reports[0].Report, []byte(text), 0644))
			return nil // retain task failure separately from browser startup/cleanup
		})
		var encodeErr error
		if scopeErr == nil {
			encodeCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
			encodeErr = videoError(gimbal.RunCommand(encodeCtx, "encode-video", dirs[0], "ffmpeg", ffmpegArgs(dirs[0], reports[0].Video)...))
			stop()
		}
		turns[0] = errors.Join(taskErr, scopeErr, encodeErr)
		reports[0].Error = errorText(turns[0])
		return nil
	})
	users.Go("tester2", func(ctx context.Context) error {
		if len(suite.Workloads) < 2 {
			return nil
		}
		gimbal.Set(ctx, "assignment file", suite.Workloads[1].AssignmentFile)
		gimbal.Set(ctx, "screenshots directory", dirs[1])
		gimbal.Set(ctx, "product URL", suite.Workloads[1].URL)
		var taskErr error
		scopeErr := gimbal.Scope(ctx, "browser-session", func(ctx context.Context) error {
			browser, err := gimbal.NewBrowser(ctx, "browser", dirs[1], filepath.Join(dirs[1], "video.webm"))
			if err != nil {
				return err
			}
			tester := gimbal.NewSession(ctx, "product-operation", suite.Workloads[1].Workdir)
			start := time.Now()
			text, err := tester.Generate[gimbal.Text](ctx, userPrompt, gimbal.WithBrowser(browser))
			reports[1].ElapsedSeconds = time.Since(start).Seconds()
			if err == nil {
				feedback, feedbackErr := tester.Generate[gimbal.Text](ctx, experiencePrompt)
				text += "\n\n" + feedback
				err = feedbackErr
			}
			taskErr = errors.Join(err, os.WriteFile(reports[1].Report, []byte(text), 0644))
			return nil // retain task failure separately from browser startup/cleanup
		})
		var encodeErr error
		if scopeErr == nil {
			encodeCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
			encodeErr = videoError(gimbal.RunCommand(encodeCtx, "encode-video", dirs[1], "ffmpeg", ffmpegArgs(dirs[1], reports[1].Video)...))
			stop()
		}
		turns[1] = errors.Join(taskErr, scopeErr, encodeErr)
		reports[1].Error = errorText(turns[1])
		return nil
	})
	users.Go("tester3", func(ctx context.Context) error {
		if len(suite.Workloads) < 3 {
			return nil
		}
		gimbal.Set(ctx, "assignment file", suite.Workloads[2].AssignmentFile)
		gimbal.Set(ctx, "screenshots directory", dirs[2])
		gimbal.Set(ctx, "product URL", suite.Workloads[2].URL)
		var taskErr error
		scopeErr := gimbal.Scope(ctx, "browser-session", func(ctx context.Context) error {
			browser, err := gimbal.NewBrowser(ctx, "browser", dirs[2], filepath.Join(dirs[2], "video.webm"))
			if err != nil {
				return err
			}
			tester := gimbal.NewSession(ctx, "product-operation", suite.Workloads[2].Workdir)
			start := time.Now()
			text, err := tester.Generate[gimbal.Text](ctx, userPrompt, gimbal.WithBrowser(browser))
			reports[2].ElapsedSeconds = time.Since(start).Seconds()
			if err == nil {
				feedback, feedbackErr := tester.Generate[gimbal.Text](ctx, experiencePrompt)
				text += "\n\n" + feedback
				err = feedbackErr
			}
			taskErr = errors.Join(err, os.WriteFile(reports[2].Report, []byte(text), 0644))
			return nil // retain task failure separately from browser startup/cleanup
		})
		var encodeErr error
		if scopeErr == nil {
			encodeCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
			encodeErr = videoError(gimbal.RunCommand(encodeCtx, "encode-video", dirs[2], "ffmpeg", ffmpegArgs(dirs[2], reports[2].Video)...))
			stop()
		}
		turns[2] = errors.Join(taskErr, scopeErr, encodeErr)
		reports[2].Error = errorText(turns[2])
		return nil
	})
	groupErr := users.Wait()
	if err := writeJSON(reportsFile, reports); err != nil {
		return errors.Join(groupErr, errors.Join(turns[:]...), err)
	}
	if ctx.Err() != nil {
		return errors.Join(ctx.Err(), groupErr, errors.Join(turns[:]...))
	}
	gimbal.Set(ctx, "workload reports", reportsFile)
	gimbal.Set(ctx, "execution errors", errorText(errors.Join(groupErr, errors.Join(turns[:]...))))
	visual := gimbal.NewSession(ctx, "product-visual-review", output)
	visualText, visualErr := visual.Generate[gimbal.Text](ctx, visualPrompt)
	visualFile := filepath.Join(output, "visual-review.md")
	visualErr = errors.Join(visualErr, os.WriteFile(visualFile, []byte(visualText), 0644))
	gimbal.Set(ctx, "screenshot review", visualFile)
	gimbal.Set(ctx, "screenshot review error", errorText(visualErr))
	triage := gimbal.NewSession(ctx, "product-triage", output)
	var findings gimbal.Text
	var triageErr error
	if suite.IssueRepo == "" {
		findings, triageErr = triage.Generate[gimbal.Text](ctx, reportPrompt)
	} else {
		gimbal.Set(ctx, "issue repository", suite.IssueRepo)
		findings, triageErr = triage.Generate[gimbal.Text](ctx, triagePrompt)
	}
	triageErr = errors.Join(triageErr, os.WriteFile(filepath.Join(output, "findings.md"), []byte(findings), 0644))
	return errors.Join(groupErr, errors.Join(turns[:]...), visualErr, triageErr)
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

func ffmpegArgs(dir, output string) []string {
	return []string{"-y", "-i", filepath.Join(dir, "video.webm"),
		"-vf", "setpts=PTS/2.5,fps=25,scale=w='min(1280,iw)':h='min(720,ih)':force_original_aspect_ratio=decrease:force_divisible_by=2",
		"-an", "-c:v", "libx264", "-preset", "medium", "-crf", "23", "-pix_fmt", "yuv420p", "-movflags", "+faststart", output}
}

func videoError(code int, _, stderr string, err error) error {
	if err != nil || code != 0 {
		return fmt.Errorf("video conversion: %w", errors.Join(err, fmt.Errorf("exit %d: %s", code, stderr)))
	}
	return nil
}
