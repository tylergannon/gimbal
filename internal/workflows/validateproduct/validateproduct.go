// Package validateproduct exercises a product's declared CLI and browser features
// and records each interaction as video. Product-specific commands and expected
// outcomes come from a JSON or YAML suite file; no scheduler or product repair is
// performed. Use a disposable target project, separate from this observing run.
//
// The suite contains product {name, workdir, prepare?, start?, ready?, browser_url?,
// cli?, revision?}, output_dir, timeout (default 15m), and features with unique id,
// surface (browser or cli), optional setup, exercise, and expected instructions.
// Relative paths resolve from the suite file. prepare/start/ready are zsh commands;
// ready is polled for at most 30 seconds and is required with start. An existing
// target can omit start and is never stopped by this workflow. A started target
// must remain in the foreground and keep descendants in its process group.
//
// Prerequisites are authenticated agent harnesses, playwright-cli with an installed
// browser, and GoTTY plus zsh for CLI features. The optional tools object overrides
// playwright_cli and terminal_server (GoTTY) executable paths. Nothing is installed
// automatically. CLI commands run in a real loopback terminal.
//
// One agent exercises each feature, takes screenshots at useful moments, and
// reports pass/fail/blocked from what it observes. CLI output and exit statuses
// supplement screenshots. Video is recorded for optional human review, not analyzed
// by another agent. There are no automatic test retries or evidence fingerprints.
// Reports and recordings go into a unique validation-* directory under output_dir.
// report.json includes every declared feature, including those never reached.
// The report contains feature evidence, not an overall completion claim. Overall
// success is the command's zero exit status (or the final Gimble run status), which
// requires every feature to pass and owned-resource cleanup to succeed. Consumers
// must check that final outcome: agent cleanup can fail after this report is written.
// Cancellation uses bounded cleanup outside the cancelled context. SIGKILL or a
// machine crash cannot guarantee finalized video, process cleanup, or a final report.
//
// Example:
//
//	gimble run validate-product --suite-file /abs/project/validation.yaml --no-web
package validateproduct

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tylergannon/gimble"
)

//go:generate go tool polytype --validate
//go:generate go run github.com/tylergannon/gimble/internal/generate/gimblegen -entry ValidateProduct -name validate-product

// Params locates the caller-owned product and feature definitions.
type Params struct {
	// SuiteFile is the JSON/YAML product target, feature list, and artifact configuration.
	SuiteFile string
}

// Verdict is the exercising agent's result, with screenshots and command output.
type Verdict struct {
	// Status is pass, fail, or blocked.
	Status        string   `json:"status"`
	Reason        string   `json:"reason"`
	EvidenceFiles []string `json:"evidence_files"`
}

type featureResult struct {
	ID       string   `json:"id"`
	Status   string   `json:"status"`
	Reason   string   `json:"reason"`
	Video    string   `json:"video,omitempty"`
	Evidence []string `json:"evidence,omitempty"`
	Error    string   `json:"error,omitempty"` // Recording, cleanup, or attachment problem; preserves the product finding.
}

type report struct {
	Product  string          `json:"product"`
	Revision string          `json:"revision,omitempty"`
	Error    string          `json:"workflow_error,omitempty"`
	Features []featureResult `json:"features"`
}

// ValidateProduct exercises declared features and saves screenshots, results, and video for human review.
func ValidateProduct(ctx context.Context, env gimble.Env, params Params) (resultErr error) {
	suite, timeout, err := readSuite(absolute(env.WorkDir, params.SuiteFile))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(suite.OutputDir, 0o755); err != nil {
		return err
	}
	output, err := os.MkdirTemp(suite.OutputDir, "validation-")
	if err != nil {
		return err
	}
	reportPath := filepath.Join(output, "report.json")
	result := report{Product: suite.Product.Name, Revision: suite.Product.Revision}
	for _, feature := range suite.Features {
		result.Features = append(result.Features, featureResult{ID: feature.ID, Status: "blocked", Reason: "not exercised"})
	}
	defer func() {
		if resultErr != nil {
			result.Error = resultErr.Error()
		}
		resultErr = errors.Join(resultErr, writeReport(reportPath, result))
	}()
	if err := writeReport(reportPath, result); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	driver, err := exec.LookPath(suite.Tools.PlaywrightCLI)
	if err != nil {
		return fmt.Errorf("browser driver prerequisite: %w", err)
	}

	err = gimble.Scope(ctx, "product", func(ctx context.Context) error {
		if suite.Product.Prepare != "" {
			code, _, stderr, err := gimble.RunCommand(ctx, "prepare", suite.Product.Workdir, "zsh", "-c", suite.Product.Prepare)
			if err != nil {
				return err
			}
			if code != 0 {
				return fmt.Errorf("prepare exited %d: %s", code, stderr)
			}
		}
		if suite.Product.Start != "" {
			if err := gimble.Service(ctx, "target", suite.Product.Workdir, suite.Product.Start); err != nil {
				return err
			}
		}
		if suite.Product.Ready != "" {
			readyCtx, stop := context.WithTimeout(ctx, 30*time.Second)
			defer stop()
			for {
				code, _, _, err := gimble.RunCommand(readyCtx, "readiness", suite.Product.Workdir, "zsh", "-c", suite.Product.Ready)
				if err != nil {
					return fmt.Errorf("readiness: %w", err)
				}
				if code == 0 {
					break
				}
				select {
				case <-readyCtx.Done():
					return fmt.Errorf("product readiness: %w", readyCtx.Err())
				case <-time.After(250 * time.Millisecond):
				}
			}
		}
		index := 0
		for featureCtx, feature := range gimble.Iterate(ctx, "feature", suite.Features) {
			item := &result.Features[index]
			index++
			dir := filepath.Join(output, strconv.Itoa(index))
			if err := os.Mkdir(dir, 0o755); err != nil {
				return err
			}
			item.Video = filepath.Join(dir, "video.webm")
			session := "validation-" + filepath.Base(output) + "-" + strconv.Itoa(index)
			var verdict Verdict
			err := gimble.Scope(featureCtx, "exercise", func(ctx context.Context) (exerciseErr error) {
				target := suite.Product.BrowserURL
				if feature.Surface == "cli" {
					terminal, err := exec.LookPath(suite.Tools.TerminalServer)
					if err != nil {
						return fmt.Errorf("terminal prerequisite: %w", err)
					}
					executable, err := exec.LookPath(suite.Product.CLI)
					if err != nil {
						return fmt.Errorf("CLI prerequisite: %w", err)
					}
					listener, err := net.Listen("tcp", "127.0.0.1:0")
					if err != nil {
						return err
					}
					port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
					if err := listener.Close(); err != nil {
						return err
					}
					command := "exec " + shellQuote(terminal) + " --config /dev/null --enable-webgl=false --permit-write --address 127.0.0.1 --port " + port + " --close-signal 15 --close-timeout 3 zsh -f"
					if err := gimble.Service(ctx, "terminal", suite.Product.Workdir, command); err != nil {
						return err
					}
					target = "http://127.0.0.1:" + port
					readyCtx, stop := context.WithTimeout(ctx, 30*time.Second)
					defer stop()
					for {
						req, err := http.NewRequestWithContext(readyCtx, http.MethodGet, target, nil)
						if err != nil {
							return err
						}
						response, err := http.DefaultClient.Do(req)
						ready := err == nil && response.StatusCode == http.StatusOK
						if response != nil {
							_ = response.Body.Close()
						}
						if ready {
							break
						}
						select {
						case <-readyCtx.Done():
							return fmt.Errorf("terminal readiness: %w", readyCtx.Err())
						case <-time.After(100 * time.Millisecond):
						}
					}
					gimble.Set(ctx, "CLI executable", executable)
				}
				// Playwright daemonizes. Explicit cleanup is independent of the scope's ctx.
				recording := false
				defer func() {
					for _, action := range []string{"video-stop", "close"} {
						if action == "video-stop" && !recording {
							continue
						}
						cleanupCtx, stop := context.WithTimeout(context.Background(), 20*time.Second)
						cmd := exec.CommandContext(cleanupCtx, driver, "-s="+session, action)
						cmd.Dir = dir
						cmd.WaitDelay = 2 * time.Second
						output, err := cmd.CombinedOutput()
						stop()
						if err != nil {
							exerciseErr = errors.Join(exerciseErr, fmt.Errorf("browser %s: %w: %s", action, err, output))
						}
						if err := os.WriteFile(filepath.Join(dir, action+".log"), output, 0o644); err != nil {
							exerciseErr = errors.Join(exerciseErr, err)
						}
					}
				}()
				code, _, stderr, err := gimble.RunCommand(ctx, "open-browser", dir, driver, "-s="+session, "open", "about:blank")
				if err != nil {
					return err
				}
				if code != 0 {
					return fmt.Errorf("open browser exited %d: %s", code, stderr)
				}
				recording = true // Attempt stop even if recording startup is interrupted.
				code, _, stderr, err = gimble.RunCommand(ctx, "start-video", dir, driver, "-s="+session, "video-start", item.Video, "--cursor")
				if err != nil {
					return err
				}
				if code != 0 {
					return fmt.Errorf("start video exited %d: %s", code, stderr)
				}
				code, _, stderr, err = gimble.RunCommand(ctx, "open-target", dir, driver, "-s="+session, "goto", target)
				if err != nil {
					return err
				}
				if code != 0 {
					return fmt.Errorf("open target exited %d: %s", code, stderr)
				}
				gimble.SetJSON(ctx, "feature", feature)
				gimble.Set(ctx, "target working directory", suite.Product.Workdir)
				gimble.Set(ctx, "browser command", shellQuote(driver)+" -s="+shellQuote(session))
				gimble.Set(ctx, "evidence directory", dir)
				operator := gimble.NewSession(ctx, "product-operation", dir)
				verdict, err = operator.Generate[Verdict](ctx, operatePrompt)
				return err
			})
			if (verdict.Status == "pass" || verdict.Status == "fail" || verdict.Status == "blocked") && strings.TrimSpace(verdict.Reason) != "" {
				item.Status, item.Reason = verdict.Status, verdict.Reason
			} else if err == nil {
				err = errors.New("agent returned no valid feature result")
			}
			for _, path := range verdict.EvidenceFiles {
				resolved, pathErr := evidencePath(dir, path)
				if pathErr == nil {
					info, statErr := os.Stat(resolved)
					if statErr != nil {
						pathErr = statErr
					} else if !info.Mode().IsRegular() {
						pathErr = fmt.Errorf("evidence is not a file: %s", path)
					}
				}
				if pathErr != nil {
					err = errors.Join(err, pathErr)
				} else {
					item.Evidence = append(item.Evidence, resolved)
				}
			}
			if item.Status == "pass" && len(item.Evidence) == 0 {
				item.Status = "blocked"
				err = errors.Join(err, errors.New("no screenshots or command output attached"))
			}
			if info, videoErr := os.Stat(item.Video); videoErr != nil || !info.Mode().IsRegular() || info.Size() == 0 {
				err = errors.Join(err, errors.New("recording is missing or empty"))
			}
			if err != nil {
				item.Error = err.Error()
			}
			if err := writeReport(reportPath, result); err != nil {
				return err
			}
		}
		return ctx.Err()
	})
	if err != nil {
		return err
	}
	for _, item := range result.Features {
		if item.Status != "pass" || item.Error != "" {
			return fmt.Errorf("validation incomplete: feature %s is %s (error: %s)", item.ID, item.Status, item.Error)
		}
	}
	return nil
}

func writeReport(path string, result report) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path+".tmp", data, 0o644); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}

const operatePrompt = `Exercise the given feature against the running product and compare what you observe with its expected behavior. Perform its setup, use the supplied browser command and existing session for all interactions, and take screenshots at useful moments, especially the result or any failure. The workflow records video for optional human review and handles video-stop and close; your assessment should use the live product, screenshots, and command output.
For CLI features, type commands into the browser terminal and save their output and exit statuses alongside terminal screenshots in the evidence directory. Empty output can be meaningful. Do not substitute direct shell execution for the recorded CLI interaction.
Return pass when the expected behavior is observed, fail when the product behaves incorrectly, or blocked when you cannot perform the check. Explain what happened and attach the absolute paths of relevant screenshots and output files from this feature's evidence directory. Return your result only in the structured response; the workflow writes the report. Do not repair the product, change expectations, fabricate evidence, or retry until a failure disappears.`
