// Run with go run ./ephemeral/attest/issue-113. One Claude worker on the
// cheap model takes a turn long enough to steer: it writes six small files
// one at a time, with a sleep between each. A second goroutine watches the
// workdir and, once the second file exists, steers the turn to change what
// the remaining files are called. Session.Steer's landed result is printed,
// the run log under logs/ carries the Steer record, and the workdir listing
// and the worker's final message show that the change took effect.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/claude"
	"github.com/tylergannon/gimble/web"
	"golang.org/x/sync/errgroup"
)

const model = "claude-haiku-4-5-20251001"

const task = `You are working in the current directory. Do these steps in order, one at a time, with a separate Write tool call for each file:
1. Write planet1.txt containing one sentence about Mercury.
2. Write planet2.txt containing one sentence about Venus.
3. Write planet3.txt containing one sentence about Earth.
4. Write planet4.txt containing one sentence about Mars.
5. Write planet5.txt containing one sentence about Jupiter.
6. Write planet6.txt containing one sentence about Saturn.
After each file, run the shell command "sleep 3" before the next step, so every step is its own tool call.
When you are done, reply with the list of files you created, in order, one per line, and nothing else.`

const steer = `Change of plan from your supervisor: stop writing planet files. Every file you have not yet written must instead be named moonN.txt, keeping the same number N, and contain one sentence about a moon of that planet. Leave the files you already wrote alone. Your final reply must still list every file you created, in order, one per line.`

func main() {
	port := flag.Int("port", 8113, "loopback TCP port for the web application")
	flag.Parse()

	base, err := filepath.Abs("ephemeral/attest/issue-113")
	if err != nil {
		log.Fatal(err)
	}
	logs := filepath.Join(base, "logs")
	if err := os.RemoveAll(logs); err != nil {
		log.Fatal(err)
	}
	workspace := filepath.Join(base, "workspace")
	if err := os.RemoveAll(workspace); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	runtime, err := web.NewRuntime(ctx, logs, web.WithPort(*port))
	if err != nil {
		log.Fatal(err)
	}
	go announce(ctx, logs, *port)
	fmt.Printf("Model: claude=%s\n", model)

	start := time.Now()
	runErr := runtime.Run(ctx, "issue-113", func(ctx context.Context) error {
		worker := gimble.NewSession(ctx, "worker", claude.New(), model, workspace)
		group, groupCtx := errgroup.WithContext(ctx)
		turnDone := make(chan struct{})
		var reply gimble.Text
		group.Go(func() error {
			defer close(turnDone)
			var err error
			reply, err = worker.Generate[gimble.Text](groupCtx, task)
			return err
		})
		group.Go(func() error {
			// Steer once the worker is two files in: it is mid-turn, with
			// four files still to write, so the change has room to show.
			for {
				if _, err := os.Stat(filepath.Join(workspace, "planet2.txt")); err == nil {
					break
				}
				select {
				case <-turnDone:
					return errors.New("the turn ended before planet2.txt appeared, so there was nothing to steer")
				case <-groupCtx.Done():
					return groupCtx.Err()
				case <-time.After(200 * time.Millisecond):
				}
			}
			landed, err := worker.Steer(groupCtx, steer)
			fmt.Printf("Steer at %s: landed=%v err=%v\n", time.Since(start).Round(time.Millisecond), landed, err)
			if err != nil {
				return err
			}
			if !landed {
				return errors.New("the steer did not land")
			}
			return nil
		})
		if err := group.Wait(); err != nil {
			return err
		}
		fmt.Printf("[worker reply]\n%s\n", reply)
		return nil
	})
	fmt.Printf("Run finished in %s: err=%v\n", time.Since(start).Round(time.Millisecond), runErr)

	entries, err := os.ReadDir(workspace)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Workspace after the run:")
	for _, entry := range entries {
		fmt.Printf("  %s\n", entry.Name())
	}
	if runErr != nil {
		os.Exit(1)
	}
}

// announce prints the run id and its page URL as soon as the run directory
// appears, so the page can be opened while the run is still going.
func announce(ctx context.Context, logs string, port int) {
	for ctx.Err() == nil {
		entries, err := os.ReadDir(filepath.Join(logs, "runs"))
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					fmt.Printf("Run: %s\nPage: http://127.0.0.1:%d/runs/%s\n", entry.Name(), port, entry.Name())
					return
				}
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
}
