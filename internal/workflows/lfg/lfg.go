// Package lfg runs one supervised coding-agent turn against a repository.
package lfg

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/claude"
	"github.com/tylergannon/gimble/codex"
)

const (
	goalKey         = "goal"
	acceptanceKey   = "acceptance"
	constraintsKey  = "constraints"
	contextFilesKey = "context files"
	checksKey       = "checks"
	workerResultKey = "worker result"
	commandKey      = "command"
	outputKey       = "output"
	exitCodeKey     = "exitcode"
)

// Input describes one supervised coding turn.
type Input struct {
	Repo                      string   `json:"repo"`
	Goal                      string   `json:"goal"`
	Acceptance                string   `json:"acceptance"`
	Constraints               string   `json:"constraints"`
	Model                     string   `json:"model"`
	ReviewModel               string   `json:"review_model"`
	ContextFiles              []string `json:"context_files"`
	Checks                    []string `json:"checks"`
	SupervisorIntervalSeconds int      `json:"supervisor_interval_seconds"`
	DryRun                    bool     `json:"dry_run"`
}

const supervisorInstruction = "Watch the worker's actual changes and steer it when it over-engineers, leaves the requested scope, or ignores a stated check. Treat checks as evidence to run and inspect, not as the gate by themselves. Keep the worker focused on the user's goal."

const workerInstruction = "Inspect the repository's local rules before editing. Implement the user's goal and acceptance requirements in the repository. Verify the actual result with the stated checks where possible. Leave changes uncommitted and do not perform remote actions."

// LFG runs one supervised worker turn. The caller must already be inside a
// Gimble Run. It returns the worker's final response.
func LFG(ctx context.Context, in Input, out io.Writer) (string, error) {
	if in.DryRun {
		return dryRun(in, out)
	}
	return run(ctx, in, codex.New(), claude.New())
}

func run(ctx context.Context, in Input, worker, reviewer gimble.HarnessAdapter) (string, error) {
	if err := normalize(&in); err != nil {
		return "", err
	}
	if err := setInput(ctx, in); err != nil {
		return "", err
	}
	workerSession := gimble.NewSession(ctx, "worker", worker, in.Model, in.Repo)
	supervisor := gimble.NewSession(ctx, "supervisor", reviewer, in.ReviewModel, in.Repo)
	interval := 30 * time.Second
	if in.SupervisorIntervalSeconds > 0 {
		interval = time.Duration(in.SupervisorIntervalSeconds) * time.Second
	}
	prompt := workerInstruction + "\n\nRepository (absolute path): " + in.Repo + "\n\n" + gimble.ScopeText(ctx)
	result, workerErr := workerSession.Generate[gimble.Text](ctx, prompt,
		gimble.WithSupervisor(supervisor, supervisorInstruction, gimble.WithInterval(interval)),
	)
	gimble.Set(ctx, workerResultKey, string(result))
	checkErr := runChecks(ctx, in)
	if workerErr != nil && checkErr != nil {
		return string(result), errors.Join(workerErr, checkErr)
	}
	if workerErr != nil {
		return string(result), workerErr
	}
	if checkErr != nil {
		return string(result), checkErr
	}
	return string(result), nil
}

func normalize(in *Input) error {
	if strings.TrimSpace(in.Repo) == "" {
		return errors.New("lfg: repo is required")
	}
	if strings.TrimSpace(in.Goal) == "" {
		return errors.New("lfg: goal is required")
	}
	if in.SupervisorIntervalSeconds < 0 {
		return errors.New("lfg: supervisor interval cannot be negative")
	}
	repo, err := filepath.Abs(in.Repo)
	if err != nil {
		return fmt.Errorf("lfg: repo: %w", err)
	}
	info, err := os.Stat(repo)
	if err != nil {
		return fmt.Errorf("lfg: repo: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("lfg: repo %s is not a directory", repo)
	}
	in.Repo = repo
	if strings.TrimSpace(in.Model) == "" {
		in.Model = "gpt-5.6-luna"
	}
	if strings.TrimSpace(in.ReviewModel) == "" {
		in.ReviewModel = "haiku"
	}
	for i, path := range in.ContextFiles {
		if !filepath.IsAbs(path) {
			path = filepath.Join(repo, path)
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("lfg: context file %d: %w", i+1, err)
		}
		info, err := os.Stat(abs)
		if err != nil {
			return fmt.Errorf("lfg: context file %s: %w", abs, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("lfg: context file %s is not a regular file", abs)
		}
		in.ContextFiles[i] = abs
	}
	return nil
}

func setInput(ctx context.Context, in Input) error {
	gimble.Set(ctx, goalKey, in.Goal)
	gimble.Set(ctx, acceptanceKey, in.Acceptance)
	gimble.Set(ctx, constraintsKey, in.Constraints)
	text, err := contextFilesText(in.ContextFiles)
	if err != nil {
		return err
	}
	gimble.Set(ctx, contextFilesKey, text)
	gimble.Set(ctx, checksKey, strings.Join(in.Checks, "\n"))
	return nil
}

func contextFilesText(paths []string) (string, error) {
	var b strings.Builder
	for i, path := range paths {
		contents, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("lfg: context file %s: %w", path, err)
		}
		fmt.Fprintf(&b, "[%d] %s\n%s\n", i+1, path, contents)
	}
	return b.String(), nil
}

func runChecks(ctx context.Context, in Input) error {
	var failures []error
	for _, check := range in.Checks {
		if err := ctx.Err(); err != nil {
			return err
		}
		command := strings.TrimSpace(check)
		if command == "" {
			continue
		}
		var code int
		var output string
		err := gimble.Scope(ctx, "check", func(ctx context.Context) error {
			var executeErr error
			code, output, executeErr = execute(ctx, in.Repo, command)
			gimble.Set(ctx, commandKey, command)
			gimble.Set(ctx, outputKey, output)
			gimble.Set(ctx, exitCodeKey, code)
			if executeErr != nil {
				return executeErr
			}
			if code != 0 {
				return fmt.Errorf("check exited with code %d", code)
			}
			return nil
		})
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("lfg: check %q: %w", command, err))
		}
	}
	return errors.Join(failures...)
}

func execute(ctx context.Context, repo, command string) (int, string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = repo
	output, err := cmd.CombinedOutput()
	if err == nil {
		return 0, string(output), nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode(), string(output), nil
	}
	return -1, string(output), err
}

func dryRun(in Input, out io.Writer) (string, error) {
	if out == nil {
		return "", errors.New("lfg: dry-run output is nil")
	}
	if err := normalize(&in); err != nil {
		return "", err
	}
	contextText, err := contextFilesText(in.ContextFiles)
	if err != nil {
		return "", err
	}
	prompt := workerInstruction + "\n\n## repository\n" + in.Repo + "\n\n## goal\n" + in.Goal + "\n\n## acceptance\n" + in.Acceptance + "\n\n## constraints\n" + in.Constraints + "\n\n## context files\n" + contextText + "\n\n## checks\n" + strings.Join(in.Checks, "\n")
	_, err = fmt.Fprintf(out, "--- worker prompt (dry run; no model or command was called) ---\n%s\n\n--- supervisor instruction (dry run) ---\n%s\n", prompt, supervisorInstruction)
	return "<example answer (no model was called)>", err
}
