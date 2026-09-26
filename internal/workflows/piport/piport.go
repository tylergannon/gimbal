// Package piport is the temporary, one-job Pi port delivery workflow.
// It reads a local JSON file of dependency waves, ports each wave in parallel
// using isolated worktrees, and integrates tested candidates serially.
// Workers translate the assigned upstream code and its relevant unit tests.
// Failed candidates retain their worktrees. This command does not push or merge.
//
// Supply --assignments-file and --scratch-dir as absolute local paths.
// Each wave contains modules with id, handoff, paths and check (an argv array).
// IDs name the fixed package branches below; handoffs are local Markdown files.
// Example: gimbal run pi-port --assignments-file /local/pi/waves.json
// --scratch-dir /local/pi/workers --pi-port-coding pi/diffusion/deepseek-4.1-flash
// --pi-port-review claude-sonnet-4-6 --pi-port-scope claude-sonnet-4-6.
// Run with a matching built server that has pi on PATH and DIFFUSION_API_KEY set.
package piport

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tylergannon/gimbal"
)

//go:generate go tool polytype --validate
//go:generate go run github.com/tylergannon/gimbal/internal/generate/gimbalgen -entry Port -name pi-port

const (
	roleCoding gimbal.WorkflowRole = "pi-port-coding"
	roleReview gimbal.WorkflowRole = "pi-port-review"
	roleScope  gimbal.WorkflowRole = "pi-port-scope"
)

type Params struct {
	// AssignmentsFile names the local JSON dependency waves and module handoffs.
	AssignmentsFile string
	// ScratchDir is an existing absolute directory outside the repository for retained worker worktrees.
	ScratchDir string
}

type module struct {
	ID      string   `json:"id"`
	Handoff string   `json:"handoff"`
	Paths   []string `json:"paths"`
	Check   []string `json:"check"`
	workdir string
	passed  bool
}

type wave struct {
	Modules []*module `json:"modules"`
}

// Assessment reports whether the integrated wave fulfills its supplied assignments.
type Assessment struct {
	Complete bool     `json:"complete"`
	Findings []string `json:"findings"`
}

// Port translates dependency waves in parallel and integrates their tested modules.
func Port(ctx context.Context, env gimbal.Env, params Params) error {
	waves, err := load(params)
	if err != nil {
		return err
	}
	exit, status, stderr, err := gimbal.RunCommand(ctx, "initial-status", env.WorkDir, "git", "status", "--porcelain", "--", ".", ":(exclude).gimbal")
	if err != nil {
		return err
	}
	if exit != 0 || strings.TrimSpace(status) != "" {
		return fmt.Errorf("integration worktree must be clean: %s %s", status, stderr)
	}
	scratch, err := os.MkdirTemp(params.ScratchDir, "pi-port-")
	if err != nil {
		return err
	}
	gimbal.Set(ctx, "assignments file", params.AssignmentsFile)
	gimbal.Set(ctx, "worker worktrees", scratch)
	for waveCtx, batch := range gimbal.Iterate(ctx, "wave", waves) {
		exit, out, stderr, err := gimbal.RunCommand(waveCtx, "baseline", env.WorkDir, "git", "rev-parse", "HEAD")
		if err != nil {
			return err
		}
		if exit != 0 {
			return fmt.Errorf("baseline: %s", stderr)
		}
		baseline := strings.TrimSpace(out)
		tasks := make(map[string]*module)
		for prepCtx, task := range gimbal.Iterate(waveCtx, "prepare", batch.Modules) {
			task.workdir = filepath.Join(scratch, task.ID)
			branch := "codex/" + filepath.Base(scratch) + "/" + task.ID
			exit, _, stderr, err := gimbal.RunCommand(prepCtx, "worktree", env.WorkDir, "git", "worktree", "add", "-b", branch, task.workdir, baseline)
			if err != nil {
				return err
			}
			if exit != 0 {
				return fmt.Errorf("worktree %s: %s", task.ID, stderr)
			}
			tasks[task.ID] = task
		}
		group := gimbal.Group(waveCtx, "modules")
		group.Go("model", func(child context.Context) error {
			task := tasks["model"]
			if task == nil {
				return nil
			}
			gimbal.Set(child, "assignment file", task.Handoff)
			gimbal.Set(child, "allowed paths", strings.Join(task.Paths, "\n"))
			worker := gimbal.NewSession(child, roleCoding, task.workdir)
			coach := gimbal.NewSession(child, roleScope, task.workdir)
			feedback := ""
			for attemptCtx, _ := range gimbal.Iterate(child, "attempt", []int{1, 2, 3}) {
				gimbal.Set(attemptCtx, "previous check", feedback)
				if _, err := worker.Generate[gimbal.Text](attemptCtx, portPrompt, gimbal.WithSupervisor(coach, scopePrompt)); err != nil {
					return err
				}
				exit, out, stderr, err := gimbal.RunCommand(attemptCtx, "package-test", task.workdir, task.Check[0], task.Check[1:]...)
				if err != nil {
					return err
				}
				feedback = fmt.Sprintf("exit %d\n%s\n%s", exit, out, stderr)
				gimbal.Set(attemptCtx, "package test", feedback)
				if exit == 0 {
					task.passed = true
					break
				}
			}
			return child.Err()
		})
		group.Go("wire", func(child context.Context) error {
			task := tasks["wire"]
			if task == nil {
				return nil
			}
			gimbal.Set(child, "assignment file", task.Handoff)
			gimbal.Set(child, "allowed paths", strings.Join(task.Paths, "\n"))
			worker := gimbal.NewSession(child, roleCoding, task.workdir)
			coach := gimbal.NewSession(child, roleScope, task.workdir)
			feedback := ""
			for attemptCtx, _ := range gimbal.Iterate(child, "attempt", []int{1, 2, 3}) {
				gimbal.Set(attemptCtx, "previous check", feedback)
				if _, err := worker.Generate[gimbal.Text](attemptCtx, portPrompt, gimbal.WithSupervisor(coach, scopePrompt)); err != nil {
					return err
				}
				exit, out, stderr, err := gimbal.RunCommand(attemptCtx, "package-test", task.workdir, task.Check[0], task.Check[1:]...)
				if err != nil {
					return err
				}
				feedback = fmt.Sprintf("exit %d\n%s\n%s", exit, out, stderr)
				gimbal.Set(attemptCtx, "package test", feedback)
				if exit == 0 {
					task.passed = true
					break
				}
			}
			return child.Err()
		})
		group.Go("openai", func(child context.Context) error {
			task := tasks["openai"]
			if task == nil {
				return nil
			}
			gimbal.Set(child, "assignment file", task.Handoff)
			gimbal.Set(child, "allowed paths", strings.Join(task.Paths, "\n"))
			worker := gimbal.NewSession(child, roleCoding, task.workdir)
			coach := gimbal.NewSession(child, roleScope, task.workdir)
			feedback := ""
			for attemptCtx, _ := range gimbal.Iterate(child, "attempt", []int{1, 2, 3}) {
				gimbal.Set(attemptCtx, "previous check", feedback)
				if _, err := worker.Generate[gimbal.Text](attemptCtx, portPrompt, gimbal.WithSupervisor(coach, scopePrompt)); err != nil {
					return err
				}
				exit, out, stderr, err := gimbal.RunCommand(attemptCtx, "package-test", task.workdir, task.Check[0], task.Check[1:]...)
				if err != nil {
					return err
				}
				feedback = fmt.Sprintf("exit %d\n%s\n%s", exit, out, stderr)
				gimbal.Set(attemptCtx, "package test", feedback)
				if exit == 0 {
					task.passed = true
					break
				}
			}
			return child.Err()
		})
		group.Go("config", func(child context.Context) error {
			task := tasks["config"]
			if task == nil {
				return nil
			}
			gimbal.Set(child, "assignment file", task.Handoff)
			gimbal.Set(child, "allowed paths", strings.Join(task.Paths, "\n"))
			worker := gimbal.NewSession(child, roleCoding, task.workdir)
			coach := gimbal.NewSession(child, roleScope, task.workdir)
			feedback := ""
			for attemptCtx, _ := range gimbal.Iterate(child, "attempt", []int{1, 2, 3}) {
				gimbal.Set(attemptCtx, "previous check", feedback)
				if _, err := worker.Generate[gimbal.Text](attemptCtx, portPrompt, gimbal.WithSupervisor(coach, scopePrompt)); err != nil {
					return err
				}
				exit, out, stderr, err := gimbal.RunCommand(attemptCtx, "package-test", task.workdir, task.Check[0], task.Check[1:]...)
				if err != nil {
					return err
				}
				feedback = fmt.Sprintf("exit %d\n%s\n%s", exit, out, stderr)
				gimbal.Set(attemptCtx, "package test", feedback)
				if exit == 0 {
					task.passed = true
					break
				}
			}
			return child.Err()
		})
		group.Go("files", func(child context.Context) error {
			task := tasks["files"]
			if task == nil {
				return nil
			}
			gimbal.Set(child, "assignment file", task.Handoff)
			gimbal.Set(child, "allowed paths", strings.Join(task.Paths, "\n"))
			worker := gimbal.NewSession(child, roleCoding, task.workdir)
			coach := gimbal.NewSession(child, roleScope, task.workdir)
			feedback := ""
			for attemptCtx, _ := range gimbal.Iterate(child, "attempt", []int{1, 2, 3}) {
				gimbal.Set(attemptCtx, "previous check", feedback)
				if _, err := worker.Generate[gimbal.Text](attemptCtx, portPrompt, gimbal.WithSupervisor(coach, scopePrompt)); err != nil {
					return err
				}
				exit, out, stderr, err := gimbal.RunCommand(attemptCtx, "package-test", task.workdir, task.Check[0], task.Check[1:]...)
				if err != nil {
					return err
				}
				feedback = fmt.Sprintf("exit %d\n%s\n%s", exit, out, stderr)
				gimbal.Set(attemptCtx, "package test", feedback)
				if exit == 0 {
					task.passed = true
					break
				}
			}
			return child.Err()
		})
		group.Go("readtools", func(child context.Context) error {
			task := tasks["readtools"]
			if task == nil {
				return nil
			}
			gimbal.Set(child, "assignment file", task.Handoff)
			gimbal.Set(child, "allowed paths", strings.Join(task.Paths, "\n"))
			worker := gimbal.NewSession(child, roleCoding, task.workdir)
			coach := gimbal.NewSession(child, roleScope, task.workdir)
			feedback := ""
			for attemptCtx, _ := range gimbal.Iterate(child, "attempt", []int{1, 2, 3}) {
				gimbal.Set(attemptCtx, "previous check", feedback)
				if _, err := worker.Generate[gimbal.Text](attemptCtx, portPrompt, gimbal.WithSupervisor(coach, scopePrompt)); err != nil {
					return err
				}
				exit, out, stderr, err := gimbal.RunCommand(attemptCtx, "package-test", task.workdir, task.Check[0], task.Check[1:]...)
				if err != nil {
					return err
				}
				feedback = fmt.Sprintf("exit %d\n%s\n%s", exit, out, stderr)
				gimbal.Set(attemptCtx, "package test", feedback)
				if exit == 0 {
					task.passed = true
					break
				}
			}
			return child.Err()
		})
		group.Go("edittools", func(child context.Context) error {
			task := tasks["edittools"]
			if task == nil {
				return nil
			}
			gimbal.Set(child, "assignment file", task.Handoff)
			gimbal.Set(child, "allowed paths", strings.Join(task.Paths, "\n"))
			worker := gimbal.NewSession(child, roleCoding, task.workdir)
			coach := gimbal.NewSession(child, roleScope, task.workdir)
			feedback := ""
			for attemptCtx, _ := range gimbal.Iterate(child, "attempt", []int{1, 2, 3}) {
				gimbal.Set(attemptCtx, "previous check", feedback)
				if _, err := worker.Generate[gimbal.Text](attemptCtx, portPrompt, gimbal.WithSupervisor(coach, scopePrompt)); err != nil {
					return err
				}
				exit, out, stderr, err := gimbal.RunCommand(attemptCtx, "package-test", task.workdir, task.Check[0], task.Check[1:]...)
				if err != nil {
					return err
				}
				feedback = fmt.Sprintf("exit %d\n%s\n%s", exit, out, stderr)
				gimbal.Set(attemptCtx, "package test", feedback)
				if exit == 0 {
					task.passed = true
					break
				}
			}
			return child.Err()
		})
		group.Go("shell", func(child context.Context) error {
			task := tasks["shell"]
			if task == nil {
				return nil
			}
			gimbal.Set(child, "assignment file", task.Handoff)
			gimbal.Set(child, "allowed paths", strings.Join(task.Paths, "\n"))
			worker := gimbal.NewSession(child, roleCoding, task.workdir)
			coach := gimbal.NewSession(child, roleScope, task.workdir)
			feedback := ""
			for attemptCtx, _ := range gimbal.Iterate(child, "attempt", []int{1, 2, 3}) {
				gimbal.Set(attemptCtx, "previous check", feedback)
				if _, err := worker.Generate[gimbal.Text](attemptCtx, portPrompt, gimbal.WithSupervisor(coach, scopePrompt)); err != nil {
					return err
				}
				exit, out, stderr, err := gimbal.RunCommand(attemptCtx, "package-test", task.workdir, task.Check[0], task.Check[1:]...)
				if err != nil {
					return err
				}
				feedback = fmt.Sprintf("exit %d\n%s\n%s", exit, out, stderr)
				gimbal.Set(attemptCtx, "package test", feedback)
				if exit == 0 {
					task.passed = true
					break
				}
			}
			return child.Err()
		})
		group.Go("resources", func(child context.Context) error {
			task := tasks["resources"]
			if task == nil {
				return nil
			}
			gimbal.Set(child, "assignment file", task.Handoff)
			gimbal.Set(child, "allowed paths", strings.Join(task.Paths, "\n"))
			worker := gimbal.NewSession(child, roleCoding, task.workdir)
			coach := gimbal.NewSession(child, roleScope, task.workdir)
			feedback := ""
			for attemptCtx, _ := range gimbal.Iterate(child, "attempt", []int{1, 2, 3}) {
				gimbal.Set(attemptCtx, "previous check", feedback)
				if _, err := worker.Generate[gimbal.Text](attemptCtx, portPrompt, gimbal.WithSupervisor(coach, scopePrompt)); err != nil {
					return err
				}
				exit, out, stderr, err := gimbal.RunCommand(attemptCtx, "package-test", task.workdir, task.Check[0], task.Check[1:]...)
				if err != nil {
					return err
				}
				feedback = fmt.Sprintf("exit %d\n%s\n%s", exit, out, stderr)
				gimbal.Set(attemptCtx, "package test", feedback)
				if exit == 0 {
					task.passed = true
					break
				}
			}
			return child.Err()
		})
		group.Go("history", func(child context.Context) error {
			task := tasks["history"]
			if task == nil {
				return nil
			}
			gimbal.Set(child, "assignment file", task.Handoff)
			gimbal.Set(child, "allowed paths", strings.Join(task.Paths, "\n"))
			worker := gimbal.NewSession(child, roleCoding, task.workdir)
			coach := gimbal.NewSession(child, roleScope, task.workdir)
			feedback := ""
			for attemptCtx, _ := range gimbal.Iterate(child, "attempt", []int{1, 2, 3}) {
				gimbal.Set(attemptCtx, "previous check", feedback)
				if _, err := worker.Generate[gimbal.Text](attemptCtx, portPrompt, gimbal.WithSupervisor(coach, scopePrompt)); err != nil {
					return err
				}
				exit, out, stderr, err := gimbal.RunCommand(attemptCtx, "package-test", task.workdir, task.Check[0], task.Check[1:]...)
				if err != nil {
					return err
				}
				feedback = fmt.Sprintf("exit %d\n%s\n%s", exit, out, stderr)
				gimbal.Set(attemptCtx, "package test", feedback)
				if exit == 0 {
					task.passed = true
					break
				}
			}
			return child.Err()
		})
		group.Go("compact", func(child context.Context) error {
			task := tasks["compact"]
			if task == nil {
				return nil
			}
			gimbal.Set(child, "assignment file", task.Handoff)
			gimbal.Set(child, "allowed paths", strings.Join(task.Paths, "\n"))
			worker := gimbal.NewSession(child, roleCoding, task.workdir)
			coach := gimbal.NewSession(child, roleScope, task.workdir)
			feedback := ""
			for attemptCtx, _ := range gimbal.Iterate(child, "attempt", []int{1, 2, 3}) {
				gimbal.Set(attemptCtx, "previous check", feedback)
				if _, err := worker.Generate[gimbal.Text](attemptCtx, portPrompt, gimbal.WithSupervisor(coach, scopePrompt)); err != nil {
					return err
				}
				exit, out, stderr, err := gimbal.RunCommand(attemptCtx, "package-test", task.workdir, task.Check[0], task.Check[1:]...)
				if err != nil {
					return err
				}
				feedback = fmt.Sprintf("exit %d\n%s\n%s", exit, out, stderr)
				gimbal.Set(attemptCtx, "package test", feedback)
				if exit == 0 {
					task.passed = true
					break
				}
			}
			return child.Err()
		})
		group.Go("agent", func(child context.Context) error {
			task := tasks["agent"]
			if task == nil {
				return nil
			}
			gimbal.Set(child, "assignment file", task.Handoff)
			gimbal.Set(child, "allowed paths", strings.Join(task.Paths, "\n"))
			worker := gimbal.NewSession(child, roleCoding, task.workdir)
			coach := gimbal.NewSession(child, roleScope, task.workdir)
			feedback := ""
			for attemptCtx, _ := range gimbal.Iterate(child, "attempt", []int{1, 2, 3}) {
				gimbal.Set(attemptCtx, "previous check", feedback)
				if _, err := worker.Generate[gimbal.Text](attemptCtx, portPrompt, gimbal.WithSupervisor(coach, scopePrompt)); err != nil {
					return err
				}
				exit, out, stderr, err := gimbal.RunCommand(attemptCtx, "package-test", task.workdir, task.Check[0], task.Check[1:]...)
				if err != nil {
					return err
				}
				feedback = fmt.Sprintf("exit %d\n%s\n%s", exit, out, stderr)
				gimbal.Set(attemptCtx, "package test", feedback)
				if exit == 0 {
					task.passed = true
					break
				}
			}
			return child.Err()
		})
		group.Go("session", func(child context.Context) error {
			task := tasks["session"]
			if task == nil {
				return nil
			}
			gimbal.Set(child, "assignment file", task.Handoff)
			gimbal.Set(child, "allowed paths", strings.Join(task.Paths, "\n"))
			worker := gimbal.NewSession(child, roleCoding, task.workdir)
			coach := gimbal.NewSession(child, roleScope, task.workdir)
			feedback := ""
			for attemptCtx, _ := range gimbal.Iterate(child, "attempt", []int{1, 2, 3}) {
				gimbal.Set(attemptCtx, "previous check", feedback)
				if _, err := worker.Generate[gimbal.Text](attemptCtx, portPrompt, gimbal.WithSupervisor(coach, scopePrompt)); err != nil {
					return err
				}
				exit, out, stderr, err := gimbal.RunCommand(attemptCtx, "package-test", task.workdir, task.Check[0], task.Check[1:]...)
				if err != nil {
					return err
				}
				feedback = fmt.Sprintf("exit %d\n%s\n%s", exit, out, stderr)
				gimbal.Set(attemptCtx, "package test", feedback)
				if exit == 0 {
					task.passed = true
					break
				}
			}
			return child.Err()
		})
		group.Go("adapter", func(child context.Context) error {
			task := tasks["adapter"]
			if task == nil {
				return nil
			}
			gimbal.Set(child, "assignment file", task.Handoff)
			gimbal.Set(child, "allowed paths", strings.Join(task.Paths, "\n"))
			worker := gimbal.NewSession(child, roleCoding, task.workdir)
			coach := gimbal.NewSession(child, roleScope, task.workdir)
			feedback := ""
			for attemptCtx, _ := range gimbal.Iterate(child, "attempt", []int{1, 2, 3}) {
				gimbal.Set(attemptCtx, "previous check", feedback)
				if _, err := worker.Generate[gimbal.Text](attemptCtx, portPrompt, gimbal.WithSupervisor(coach, scopePrompt)); err != nil {
					return err
				}
				exit, out, stderr, err := gimbal.RunCommand(attemptCtx, "package-test", task.workdir, task.Check[0], task.Check[1:]...)
				if err != nil {
					return err
				}
				feedback = fmt.Sprintf("exit %d\n%s\n%s", exit, out, stderr)
				gimbal.Set(attemptCtx, "package test", feedback)
				if exit == 0 {
					task.passed = true
					break
				}
			}
			return child.Err()
		})
		group.Go("integration", func(child context.Context) error {
			task := tasks["integration"]
			if task == nil {
				return nil
			}
			gimbal.Set(child, "assignment file", task.Handoff)
			gimbal.Set(child, "allowed paths", strings.Join(task.Paths, "\n"))
			worker := gimbal.NewSession(child, roleCoding, task.workdir)
			coach := gimbal.NewSession(child, roleScope, task.workdir)
			feedback := ""
			for attemptCtx, _ := range gimbal.Iterate(child, "attempt", []int{1, 2, 3}) {
				gimbal.Set(attemptCtx, "previous check", feedback)
				if _, err := worker.Generate[gimbal.Text](attemptCtx, portPrompt, gimbal.WithSupervisor(coach, scopePrompt)); err != nil {
					return err
				}
				exit, out, stderr, err := gimbal.RunCommand(attemptCtx, "package-test", task.workdir, task.Check[0], task.Check[1:]...)
				if err != nil {
					return err
				}
				feedback = fmt.Sprintf("exit %d\n%s\n%s", exit, out, stderr)
				gimbal.Set(attemptCtx, "package test", feedback)
				if exit == 0 {
					task.passed = true
					break
				}
			}
			return child.Err()
		})
		if err := group.Wait(); err != nil {
			return err
		}
		if err := waveCtx.Err(); err != nil {
			return err
		}
		for _, task := range batch.Modules {
			if !task.passed {
				return fmt.Errorf("module %s did not pass; worktree retained at %s", task.ID, task.workdir)
			}
		}
		for integrateCtx, task := range gimbal.Iterate(waveCtx, "integrate", batch.Modules) {
			exit, head, stderr, err := gimbal.RunCommand(integrateCtx, "candidate-head", task.workdir, "git", "rev-parse", "HEAD")
			if err != nil {
				return err
			}
			if exit != 0 || strings.TrimSpace(head) != baseline {
				return fmt.Errorf("module %s changed its Git baseline: %s", task.ID, stderr)
			}
			exit, changed, stderr, err := gimbal.RunCommand(integrateCtx, "changed-paths", task.workdir, "git", "diff", "--name-only", "--no-renames", "-z", "HEAD")
			if err != nil {
				return err
			}
			if exit != 0 {
				return fmt.Errorf("changed paths: %s", stderr)
			}
			exit, added, stderr, err := gimbal.RunCommand(integrateCtx, "new-paths", task.workdir, "git", "ls-files", "--others", "--exclude-standard", "-z")
			if err != nil {
				return err
			}
			if exit != 0 {
				return fmt.Errorf("new paths: %s", stderr)
			}
			if err := owned(changed+added, task.Paths); err != nil {
				return fmt.Errorf("module %s: %w", task.ID, err)
			}
			exit, _, stderr, err = gimbal.RunCommand(integrateCtx, "stage", task.workdir, "git", append([]string{"add", "--"}, strings.Split(strings.TrimSuffix(changed+added, "\x00"), "\x00")...)...)
			if err != nil {
				return err
			}
			if exit != 0 {
				return fmt.Errorf("stage: %s", stderr)
			}
			exit, _, stderr, err = gimbal.RunCommand(integrateCtx, "commit", task.workdir, "git", "commit", "-m", "Port Pi module: "+task.ID)
			if err != nil {
				return err
			}
			if exit != 0 {
				return fmt.Errorf("commit: %s", stderr)
			}
			exit, commit, stderr, err := gimbal.RunCommand(integrateCtx, "candidate-commit", task.workdir, "git", "rev-parse", "HEAD")
			if err != nil {
				return err
			}
			if exit != 0 {
				return fmt.Errorf("candidate commit: %s", stderr)
			}
			exit, _, stderr, err = gimbal.RunCommand(integrateCtx, "cherry-pick", env.WorkDir, "git", "cherry-pick", strings.TrimSpace(commit))
			if err != nil {
				return err
			}
			if exit != 0 {
				return fmt.Errorf("integrating %s failed; resolve retained state: %s", task.ID, stderr)
			}
			exit, out, stderr, err := gimbal.RunCommand(integrateCtx, "integrated-test", env.WorkDir, "go", "test", "./internal/pi/...")
			gimbal.Set(integrateCtx, "integrated test", fmt.Sprintf("exit %d\n%s\n%s", exit, out, stderr))
			if err != nil {
				return err
			}
			if exit != 0 {
				return fmt.Errorf("integrated tests failed after %s", task.ID)
			}
		}
		handoffs := make([]string, 0, len(batch.Modules))
		for _, task := range batch.Modules {
			handoffs = append(handoffs, task.Handoff)
		}
		gimbal.Set(waveCtx, "wave assignments", strings.Join(handoffs, "\n"))
		reviewer := gimbal.NewSession(waveCtx, roleReview, env.WorkDir)
		assessment, err := reviewer.Generate[Assessment](waveCtx, reviewPrompt)
		if err != nil {
			return err
		}
		gimbal.SetJSON(waveCtx, "assessment", assessment)
		if !assessment.Complete {
			return fmt.Errorf("wave requires repair: %s", strings.Join(assessment.Findings, "; "))
		}
	}
	return ctx.Err()
}

const portPrompt = "Read the local assignment file named in context. Port its TypeScript modules to the assigned Go package, using the upstream source and relevant unit tests. Run and fix ordinary package tests; preserve behavior without redesign or unrelated features. You own only the listed paths in this isolated worktree. Do not edit other packages, shared contracts, this workflow or the handoff, and do not commit or push. If a shared contract blocks you, report it instead of inventing a competing type. Report what changed and the tests you ran."
const scopePrompt = "Steer only against expanding this assignment, redesigning the upstream, or building elaborate test machinery. Preserve the requested behavior and relevant upstream unit tests. Do not edit files."
const reviewPrompt = "Read the wave assignments at their local paths, inspect the integrated Go code against the assigned TypeScript and tests, and run relevant checks. Make no edits. Report only concrete missing behavior or incorrect translations that block these assignments. Ordinary relevant unit tests are sufficient here; do not demand new frameworks, exhaustive test matrices, or work assigned to later waves. Complete is true when these assignments are implemented with no substantial gap."

func load(params Params) ([]wave, error) {
	if !filepath.IsAbs(params.AssignmentsFile) || !filepath.IsAbs(params.ScratchDir) {
		return nil, fmt.Errorf("assignments-file and scratch-dir must be absolute")
	}
	raw, err := os.ReadFile(params.AssignmentsFile)
	if err != nil {
		return nil, err
	}
	var waves []wave
	if err := json.Unmarshal(raw, &waves); err != nil {
		return nil, err
	}
	if len(waves) == 0 {
		return nil, fmt.Errorf("no waves")
	}
	seen := map[string]bool{}
	for _, batch := range waves {
		if len(batch.Modules) == 0 {
			return nil, fmt.Errorf("empty wave")
		}
		ownedInWave := map[string]string{}
		for _, task := range batch.Modules {
			if task == nil || seen[task.ID] {
				return nil, fmt.Errorf("nil or duplicate module")
			}
			switch task.ID {
			case "model", "wire", "openai", "config", "files", "readtools", "edittools", "shell", "resources", "history", "compact", "agent", "session", "adapter", "integration":
			default:
				return nil, fmt.Errorf("unknown module %q", task.ID)
			}
			seen[task.ID] = true
			if !filepath.IsAbs(task.Handoff) {
				return nil, fmt.Errorf("%s handoff must be absolute", task.ID)
			}
			if _, err := os.Stat(task.Handoff); err != nil {
				return nil, err
			}
			if len(task.Check) == 0 || task.Check[0] == "" || len(task.Paths) == 0 {
				return nil, fmt.Errorf("%s needs check argv and owned paths", task.ID)
			}
			for _, path := range task.Paths {
				for previous, owner := range ownedInWave {
					if path == previous || strings.HasPrefix(path, previous+"/") || strings.HasPrefix(previous, path+"/") {
						return nil, fmt.Errorf("%s and %s have overlapping paths", task.ID, owner)
					}
				}
				ownedInWave[path] = task.ID
				if path == "." || path == ".git" || strings.HasPrefix(path, ".git/") || filepath.IsAbs(path) || filepath.Clean(path) != path || path == ".." || strings.HasPrefix(path, "../") {
					return nil, fmt.Errorf("invalid owned path %q", path)
				}
			}
		}
	}
	return waves, nil
}

func owned(paths string, allowed []string) error {
	if paths == "" {
		return fmt.Errorf("no implementation changes")
	}
	for path := range strings.SplitSeq(paths, "\x00") {
		if path == "" {
			continue
		}
		found := false
		for _, root := range allowed {
			if path == root || strings.HasPrefix(path, root+"/") {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("changed unowned path %s", path)
		}
	}
	return nil
}
