// Run with go run ./ephemeral/attest/issue117 -port 8117. The turns use the
// production adapters and web handler: Luna and Haiku each run one native
// child agent, Luna probes request_user_input under Gimble's fixed
// noninteractive policy, and invalid model names exercise terminal errors.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/claude"
	"github.com/tylergannon/gimble/codex"
	"github.com/tylergannon/gimble/web"
)

func main() {
	port := flag.Int("port", 8117, "loopback TCP port")
	flag.Parse()
	repo, err := filepath.Abs(".")
	if err != nil {
		log.Fatal(err)
	}
	project := filepath.Join(repo, "ephemeral", "attest", "issue117", ".gimble")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	runtime, err := web.NewRuntime(ctx, project, web.WithPort(*port))
	if err != nil {
		log.Fatal(err)
	}
	err = runtime.Run(ctx, "issue117-native-events", func(ctx context.Context) error {
		group := gimble.Group(ctx, "native")
		group.Go("codex-child", func(ctx context.Context) error {
			session := gimble.NewSession(ctx, "codex", codex.New(), "gpt-5.6-luna", repo)
			out, err := session.Generate[gimble.Text](ctx, "Use spawn_agent exactly once. Ask that child to reply with exactly CODEX_CHILD_NATIVE_117 and use no tools. Wait for it to finish, then reply with exactly CODEX_PARENT_NATIVE_117.")
			gimble.Set(ctx, "result", string(out))
			if err != nil {
				gimble.Set(ctx, "error", err.Error())
			}
			return nil
		})
		group.Go("codex-request", func(ctx context.Context) error {
			session := gimble.NewSession(ctx, "codex", codex.New(), "gpt-5.6-luna", repo)
			out, err := session.Generate[gimble.Text](ctx, "Call request_user_input once with one yes-or-no question, then stop.")
			gimble.Set(ctx, "result", string(out))
			if err != nil {
				gimble.Set(ctx, "error", err.Error())
			}
			return nil
		})
		group.Go("claude-child", func(ctx context.Context) error {
			session := gimble.NewSession(ctx, "claude", claude.New(), "claude-haiku-4-5-20251001", repo)
			out, err := session.Generate[gimble.Text](ctx, "Use the Agent tool exactly once with run_in_background true and a general-purpose subagent. Ask the child to reply with exactly CLAUDE_CHILD_NATIVE_117 and use no tools. Immediately after spawning it, use Bash to run `sleep 3` so the child completion notification arrives while this turn is open. Then reply with exactly CLAUDE_PARENT_NATIVE_117.")
			gimble.Set(ctx, "result", string(out))
			if err != nil {
				gimble.Set(ctx, "error", err.Error())
			}
			return nil
		})
		group.Go("codex-error", func(ctx context.Context) error {
			session := gimble.NewSession(ctx, "codex", codex.New(), "not-a-real-model-issue117", repo)
			_, err := session.Generate[gimble.Text](ctx, "Reply with exactly CODEX_UNEXPECTED_SUCCESS_117.")
			if err != nil {
				gimble.Set(ctx, "expected-error", err.Error())
			}
			return nil
		})
		group.Go("claude-error", func(ctx context.Context) error {
			session := gimble.NewSession(ctx, "claude", claude.New(), "not-a-real-model-issue117", repo)
			_, err := session.Generate[gimble.Text](ctx, "Reply with exactly CLAUDE_UNEXPECTED_SUCCESS_117.")
			if err != nil {
				gimble.Set(ctx, "expected-error", err.Error())
			}
			return nil
		})
		return group.Wait()
	})
	fmt.Println("Run ended:", err)
	fmt.Printf("Open http://127.0.0.1:%d/\n", *port)
	time.Sleep(5 * time.Minute)
}
