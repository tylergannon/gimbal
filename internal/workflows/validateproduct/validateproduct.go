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
// browser, ffmpeg for video decoding, and GoTTY plus zsh for CLI features. The
// optional tools object overrides playwright_cli, terminal_server (GoTTY), and
// video_decoder executable paths. Nothing is installed automatically. CLI commands
// run in a real loopback terminal; agents retain output and exit status separately
// because its canvas text is not available in accessibility snapshots.
//
// Each feature gets a fresh recorded browser session and independent operator and
// validator sessions. The workflow finalizes video, closes the browser, and decodes
// the video before validation. Missing evidence or prerequisites are blocked, not
// passes. Failed expectations remain failures; there are no automatic test retries.
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

// Observation records what the operator actually observed, not a verdict.
type Observation struct {
	Summary       string   `json:"summary"`
	EvidenceFiles []string `json:"evidence_files"`
	BlockedReason string   `json:"blocked_reason"`
}

// Verdict is the independent validator's evidence-based result.
type Verdict struct {
	// Status is pass, fail, or blocked. Missing evidence is blocked.
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
}

type report struct {
	Product  string          `json:"product"`
	Revision string          `json:"revision,omitempty"`
	Error    string          `json:"workflow_error,omitempty"`
	Features []featureResult `json:"features"`
}

// ValidateProduct operates and independently validates each declared feature with video.
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
	decoder, err := exec.LookPath(suite.Tools.VideoDecoder)
	if err != nil {
		return fmt.Errorf("video decoder prerequisite: %w", err)
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
			reportDigest, err := fileDigest(reportPath)
			if err != nil {
				return err
			}
			item := &result.Features[index]
			index++
			dir := filepath.Join(output, strconv.Itoa(index))
			if err := os.Mkdir(dir, 0o755); err != nil {
				return err
			}
			item.Video = filepath.Join(dir, "video.webm")
			session := "validation-" + filepath.Base(output) + "-" + strconv.Itoa(index)
			var observation Observation
			err = gimble.Scope(featureCtx, "exercise", func(ctx context.Context) (exerciseErr error) {
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
				observation, err = operator.Generate[Observation](ctx, operatePrompt)
				return err
			})
			if err != nil {
				item.Reason = err.Error()
			} else if err := unchanged(reportPath, reportDigest); err != nil {
				item.Reason = "operator modified the workflow report: " + err.Error()
			} else if observation.BlockedReason != "" {
				item.Reason = observation.BlockedReason
			} else if len(observation.EvidenceFiles) == 0 {
				item.Reason = "operator supplied no evidence"
			} else {
				code, _, stderr, decodeErr := gimble.RunCommand(featureCtx, "decode-video", dir, decoder, "-v", "error", "-xerror", "-i", item.Video, "-f", "image2", "-update", "1", "-y", filepath.Join(dir, "last-frame.png"))
				if decodeErr != nil {
					item.Reason = decodeErr.Error()
				} else if code != 0 {
					item.Reason = fmt.Sprintf("video decoding failed: %s", stderr)
				} else if info, err := os.Stat(filepath.Join(dir, "last-frame.png")); err != nil || info.Size() == 0 {
					item.Reason = "video decoder produced no frame"
				} else {
					originals, err := snapshotEvidence(dir, append(append([]string{}, observation.EvidenceFiles...), item.Video))
					if err != nil {
						item.Reason = "invalid operator evidence: " + err.Error()
					} else {
						// Close the validator before checking that original evidence stayed unchanged.
						var verdict Verdict
						err := gimble.Scope(featureCtx, "validation", func(ctx context.Context) error {
							gimble.SetJSON(ctx, "feature", feature)
							gimble.SetJSON(ctx, "operator observation", observation)
							gimble.Set(ctx, "evidence directory", dir)
							gimble.Set(ctx, "video", item.Video)
							gimble.Set(ctx, "video decoder", decoder)
							validator := gimble.NewSession(ctx, "product-validation", dir)
							var err error
							verdict, err = validator.Generate[Verdict](ctx, validatePrompt)
							return err
						})
						evidence, evidenceErr := verdictEvidence(dir, originals, verdict.EvidenceFiles)
						if err != nil {
							item.Reason = err.Error()
						} else if evidenceErr != nil {
							item.Reason = "invalid validator evidence: " + evidenceErr.Error()
						} else if verdict.Status != "pass" && verdict.Status != "fail" && verdict.Status != "blocked" {
							item.Reason = "validator returned an invalid status"
						} else if strings.TrimSpace(verdict.Reason) == "" || len(evidence) == 0 {
							item.Reason = "validator supplied no explanation or evidence"
						} else {
							item.Status, item.Reason, item.Evidence = verdict.Status, verdict.Reason, evidence
						}
					}
				}
			}
			if err := unchanged(reportPath, reportDigest); err != nil {
				item.Status, item.Reason = "blocked", "agent modified the workflow report: "+err.Error()
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
		if item.Status != "pass" {
			return fmt.Errorf("validation incomplete: feature %s is %s", item.ID, item.Status)
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

const operatePrompt = `Exercise the given feature against the running target. Perform its setup and interactions, preserving the expected outcome unchanged. Use the supplied browser command and its existing session for all browser or terminal interactions: recording is already active and the workflow owns video-stop and close. Do not create another browser session or stop this one. Do not repair the product or change its source.
For CLI features, type commands into the browser terminal. Its text is drawn on canvas: retain command output and exit status in files in the evidence directory, using tee and the shell's pipeline status where appropriate, and take screenshots of the visible terminal. Do not substitute direct shell execution for the recorded CLI interaction. Browser features likewise need snapshots or screenshots showing the relevant actual state.
Save original observations in the evidence directory and return their absolute paths with a concise account of what happened. Report missing prerequisites or inability to exercise the feature in BlockedReason. A failed expectation is an observation for the independent validator, not a reason to silently retry until it passes. Do not modify the workflow report or fabricate evidence.`

const validatePrompt = `Independently decide whether the recorded interaction demonstrates the feature's expected behavior. Inspect the actual evidence files and sample relevant frames of the finalized video using the supplied decoder; do not accept the operator's summary as proof. The recording must show the same interaction as the supporting outputs and screenshots. Verify command exit statuses where relevant, distinguish an expected product error from an infrastructure error, and preserve unmet expectations.
Return pass only when the original observations establish the whole expected outcome. Return fail for a demonstrated product mismatch, or blocked for insufficient evidence or missing prerequisites. Explain the observed-versus-expected result and cite the actual evidence files you inspected. Return the verdict only in your structured response; the workflow owns report writing. Do not write or modify any report, original evidence, product files, or expected outcome. You may create sampled video frames in the evidence directory for inspection, but cite only the original operator evidence files and the supplied video. All citations must belong to this feature. Do not operate the target or repair the product.`
