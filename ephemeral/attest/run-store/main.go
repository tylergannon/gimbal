// Run with go run ./ephemeral/attest/run-store. The proof run for the run
// store (docs/sprints/RUN-STORE.md, Phase 3): one session made at the root
// and used again inside a child scope (the placement case), then a Group of
// two concurrent attempts. Cheap tier only. The runtime serves the live page
// on -port and is held open after the run so the page, the six table files,
// and the logs can be read side by side.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/agy"
	"github.com/tylergannon/gimble/claude"
	"github.com/tylergannon/gimble/codex"
	"github.com/tylergannon/gimble/web"
)

const (
	sharedFirst  = "Define a token budget in one sentence. Answer from your own knowledge and use no tools."
	sharedSecond = "Name one useful budget measurement in one sentence. Use no tools."
	attemptOne   = "Explain token caching in about 100 words. Use no tools."
	attemptTwo   = "Give one example of token caching paying off, in about 100 words. Use no tools."
	geminiPrompt = "Explain token caching in about 150 words. Use no tools."
)

func main() {
	port := flag.Int("port", 8098, "loopback TCP port for the web application")
	hold := flag.Duration("hold", 15*time.Minute, "how long to hold the server open after the run")
	second := flag.String("second", "claude", "harness for the first compare attempt: claude or codex")
	pause := flag.Duration("pause", 0, "wait this long between the placement case and the compare group, to open the page in a browser")
	flag.Parse()

	base, err := filepath.Abs("ephemeral/attest/run-store")
	if err != nil {
		log.Fatal(err)
	}
	project := filepath.Join(base, ".gimble")
	workspace := filepath.Join(base, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	runtime, err := web.NewRuntime(ctx, project, web.WithPort(*port))
	if err != nil {
		log.Fatal(err)
	}
	go announce(ctx, project, *port)

	var writerAdapter gimble.HarnessAdapter = claude.New()
	writerModel := "claude-haiku-4-5-20251001"
	if *second == "codex" {
		writerAdapter, writerModel = codex.New(), "gpt-5.6-luna"
	}
	fmt.Printf("Models: shared=gpt-5.6-luna writer=%s writer(gemini)=gemini-3.8-flash-low\n", writerModel)

	start := time.Now()
	runErr := runtime.Run(ctx, "run-store-proof", func(ctx context.Context) error {
		// Created at the root, used again in a child scope: turn 1 is charged
		// to the root, turn 2 to research.1.
		shared := gimble.NewSession(ctx, "shared", codex.New(), "gpt-5.6-luna", workspace)
		first, err := shared.Generate[gimble.Text](ctx, sharedFirst)
		if err != nil {
			return fmt.Errorf("shared turn 1: %w", err)
		}
		fmt.Printf("[shared turn 1] %s\n", first)
		if err := gimble.Scope(ctx, "research", func(ctx context.Context) error {
			second, err := shared.Generate[gimble.Text](ctx, sharedSecond)
			if err != nil {
				return fmt.Errorf("shared turn 2: %w", err)
			}
			fmt.Printf("[shared turn 2] %s\n", second)
			return nil
		}); err != nil {
			return err
		}
		if *pause > 0 {
			fmt.Printf("Pausing %s before compare; open the page now.\n", *pause)
			select {
			case <-time.After(*pause):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		// A Group of two concurrent attempts.
		group := gimble.Group(ctx, "compare")
		group.Go("attempt", func(ctx context.Context) error {
			s := gimble.NewSession(ctx, "writer", writerAdapter, writerModel, workspace)
			one, err := s.Generate[gimble.Text](ctx, attemptOne)
			if err != nil {
				return fmt.Errorf("attempt 1 turn 1: %w", err)
			}
			fmt.Printf("[attempt.1 turn 1] %s\n", one)
			two, err := s.Generate[gimble.Text](ctx, attemptTwo)
			if err != nil {
				return fmt.Errorf("attempt 1 turn 2: %w", err)
			}
			fmt.Printf("[attempt.1 turn 2] %s\n", two)
			return nil
		})
		group.Go("attempt", func(ctx context.Context) error {
			s := gimble.NewSession(ctx, "writer", agy.New(), "gemini-3.8-flash-low", workspace)
			out, err := s.Generate[gimble.Text](ctx, geminiPrompt)
			if err != nil {
				return fmt.Errorf("attempt 2 turn 1: %w", err)
			}
			fmt.Printf("[attempt.2 turn 1] %s\n", out)
			return nil
		})
		return group.Wait()
	})
	fmt.Printf("Run finished in %s: err=%v\n", time.Since(start).Round(time.Millisecond), runErr)

	fmt.Printf("Holding the server for %s; press Enter to stop early.\n", *hold)
	done := make(chan struct{})
	go func() {
		var line [1]byte
		for {
			n, err := os.Stdin.Read(line[:])
			if err != nil {
				return // no console: hold for the full duration
			}
			if n > 0 && line[0] == '\n' {
				close(done)
				return
			}
		}
	}()
	select {
	case <-done:
	case <-time.After(*hold):
	case <-ctx.Done():
	}
	if runErr != nil {
		os.Exit(1)
	}
}

// announce prints the newest run id and its page URL as soon as a run
// directory appears after start, so the page can be opened mid-run.
func announce(ctx context.Context, project string, port int) {
	started := time.Now()
	for ctx.Err() == nil {
		entries, err := os.ReadDir(filepath.Join(project, "runs"))
		if err == nil {
			for _, entry := range entries {
				info, err := entry.Info()
				if err == nil && entry.IsDir() && info.ModTime().After(started.Add(-time.Second)) {
					fmt.Printf("Run: %s\nPage: http://127.0.0.1:%d/runs/%s\n", entry.Name(), port, entry.Name())
					return
				}
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
}
