package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/claude"
	"github.com/tylergannon/gimble/codex"
	"github.com/tylergannon/gimble/web"
)

func main() {
	port := flag.Int("port", 8080, "loopback TCP port for the web application")
	flag.Parse()

	repo, err := filepath.Abs(".")
	if err != nil {
		log.Fatal(err)
	}

	candidateDirs := make([]string, 2)
	for i := range candidateDirs {
		dir, err := os.MkdirTemp("", fmt.Sprintf("gimble-bakeoff-%d-", i+1))
		if err != nil {
			log.Fatal(err)
		}
		candidateDirs[i] = dir
		out, err := exec.Command("git", "-C", repo, "worktree", "add", "--detach", dir).CombinedOutput()
		if err != nil {
			log.Fatalf("git worktree add %s: %v: %s", dir, err, out)
		}
	}
	defer func() {
		for _, dir := range candidateDirs {
			_ = exec.Command("git", "-C", repo, "worktree", "remove", "--force", dir).Run()
			_ = os.RemoveAll(dir)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	runtime, err := web.NewRuntime(ctx, filepath.Join(repo, ".gimble"), web.WithPort(*port))
	if err != nil {
		log.Fatal(err)
	}

	err = runtime.Run(ctx, "skill-bakeoff", func(ctx context.Context) error {
		goal := "Read AGENTS.md and the current Gimble repository. Make one small, in-scope correctness improvement that is supported by a focused test. Do not change anything outside the candidate worktree, do not commit, and answer only with what you changed."
		adapter := codex.New()
		group := gimble.Group(ctx, "codex-candidates")
		for i, dir := range candidateDirs {
			candidate, workdir := i+1, dir
			group.Go("candidate", func(ctx context.Context) error {
				coder := gimble.NewSession(ctx, fmt.Sprintf("codex-%d", candidate), adapter, "gpt-5.6-luna", workdir)
				result, err := coder.Generate[gimble.Text](ctx, goal+"\n\nWorktree: "+workdir)
				if err != nil {
					return err
				}
				gimble.Set(ctx, "candidate result", string(result))
				return nil
			})
		}
		if err := group.Wait(); err != nil {
			return err
		}

		out, err := exec.CommandContext(ctx, "git", "-C", repo, "status", "--porcelain").CombinedOutput()
		if err != nil {
			return fmt.Errorf("git status: %w: %s", err, out)
		}
		if len(out) != 0 {
			return fmt.Errorf("a candidate changed the repository: %s", out)
		}

		judge := gimble.NewSession(ctx, "claude-judge", claude.New(), "claude-haiku-4-5-20251001", repo)
		verdict, err := judge.Generate[gimble.Text](ctx, fmt.Sprintf("Inspect the repository at %s and the two candidate worktrees at %s and %s. Read the actual diffs and focused tests in both worktrees, and run whatever read-only checks are needed. Judge which candidate is the smaller correct improvement that satisfies this task: %s. Do not trust or request the Codex sessions' summaries, and do not modify any files. Answer with the winning candidate number and the concrete evidence for your decision.", repo, candidateDirs[0], candidateDirs[1], goal))
		if err != nil {
			return err
		}
		fmt.Println(verdict)
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
}
