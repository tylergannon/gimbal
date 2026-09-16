package gimble_test

// The workflow shapes, one compiling Example each, named in
// .agents/skills/gimble-workflows/SKILL.md. Each runs on a scripted fake
// harness so its output is fixed; a real workflow passes codex.New(),
// claude.New(), or agy.New() where these pass a *script, and its prompts
// name absolute paths in the repository it works on. Example (one turn),
// ExampleGroup, and ExampleLoop are in example_test.go.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/web"
)

// script is the fake harness of the shape examples: answer is called with
// each turn's prompt and schema and returns the turn's output, prose for
// an empty schema and JSON otherwise.
type script struct {
	answer func(ctx context.Context, prompt string, schema json.RawMessage) (string, error)
	mu     sync.Mutex
	made   int
}

// says is a script whose every prose turn answers text.
func says(text string) *script {
	return &script{answer: func(context.Context, string, json.RawMessage) (string, error) { return text, nil }}
}

func (s *script) CreateSession(context.Context, string, string, string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.made++
	return fmt.Sprintf("native-%d", s.made), nil
}

func (s *script) RunTurn(ctx context.Context, _ string, prompt string, schema json.RawMessage, _ func(gimble.AgentEvent) error) (gimble.TurnResult, error) {
	out, err := s.answer(ctx, prompt, schema)
	if err != nil {
		return gimble.TurnResult{}, err
	}
	if len(schema) == 0 {
		raw, err := json.Marshal(out)
		return gimble.TurnResult{Output: raw}, err
	}
	return gimble.TurnResult{Output: json.RawMessage(out)}, nil
}

func (*script) Steer(context.Context, string, string) (bool, error) { return false, nil }
func (*script) Fork(_ context.Context, session string) (string, error) {
	return session + "-fork", nil
}
func (*script) Close(context.Context, string) error { return nil }

// verdict is a judge's answer. Its field comments are the descriptions
// the judge reads in the schema. A real workflow generates Schema and
// ValidateJSON with polytype (`//go:generate go tool polytype --validate`,
// as internal/workflows/sprint does); here they are written out so the
// type can live in a test file.
type verdict struct {
	// Winner is the number of the better candidate: 1 or 2.
	Winner int `json:"winner"`
	// Why is what decided it, in one sentence.
	Why string `json:"why"`
}

func (verdict) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{
"winner":{"type":"integer","description":"The number of the better candidate: 1 or 2."},
"why":{"type":"string","description":"What decided it, in one sentence."}
},"required":["winner","why"],"additionalProperties":false}`)
}

func (verdict) ValidateJSON(raw []byte) error { return required(raw, "winner", "why") }

// critique is a critic's answer: the defects it found, none when it found
// nothing wrong.
type critique struct {
	// Defects is each thing the draft gets wrong or leaves out, one entry each.
	Defects []string `json:"defects"`
}

func (critique) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{
"defects":{"type":"array","items":{"type":"string"},"description":"Each thing the draft gets wrong or leaves out, one entry each."}
},"required":["defects"],"additionalProperties":false}`)
}

func (critique) ValidateJSON(raw []byte) error { return required(raw, "defects") }

// required stands in for polytype's generated validation: raw is an object
// with every named field present.
func required(raw []byte, fields ...string) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return err
	}
	for _, field := range fields {
		if _, ok := object[field]; !ok {
			return fmt.Errorf("%q is required", field)
		}
	}
	return nil
}

// repoDir is the absolute path of the repository the examples' sessions
// work in. Every prompt that names a place names it absolutely.
func repoDir() string {
	dir, err := filepath.Abs(".")
	if err != nil {
		panic(err)
	}
	return dir
}

// Example_bakeOff is fork and bake-off: one session reads the code once,
// two forks of it each propose an approach at the same time in a Group,
// and a judge on a different harness picks one. The judge reads the
// proposals themselves, never the proposers' opinion of them.
func Example_bakeOff() {
	ctx, closeProject := exampleContext()
	defer closeProject()
	repo := repoDir()
	codex := says("Cache the parsed config on the loader and return it from Load.")
	claude := says(`{"winner":2,"why":"It changes one file and keeps Load's signature."}`)

	err := gimble.Run(ctx, "bakeoff", map[string]gimble.ModelBinding{"judge": {Adapter: claude, Model: "claude-haiku-4-5-20251001"}, "researcher": {Adapter: codex, Model: "gpt-5.6-luna"}}, func(ctx context.Context) error {
		researcher := gimble.NewSession(ctx, "researcher", repo)
		if _, err := researcher.Generate[gimble.Text](ctx, "Read the config loader in "+repo+" and everything that calls it. Change nothing. Answer with where the file is read and by whom."); err != nil {
			return err
		}

		proposals := make([]gimble.Text, 2)
		group := gimble.Group(ctx, "candidates")
		for i := range proposals {
			group.Go("candidate", func(ctx context.Context) error {
				coder, err := researcher.Fork(ctx, "coder")
				if err != nil {
					return err
				}
				proposals[i], err = coder.Generate[gimble.Text](ctx, "Propose one way to make the loader read its config file once per process. Change nothing. Answer with the change in plain English, naming each file it touches.")
				return err
			})
		}
		if err := group.Wait(); err != nil {
			return err
		}

		judge := gimble.NewSession(ctx, "judge", repo)
		v, err := judge.Generate[verdict](ctx, fmt.Sprintf("Two proposals to make the config loader in %s read its file once per process. Read the code they name and pick the smaller change that is correct.\n\n1. %s\n\n2. %s", repo, proposals[0], proposals[1]))
		if err != nil {
			return err
		}
		fmt.Println("winner:", v.Winner, v.Why)
		return nil
	})
	fmt.Println(err)

	// Output:
	// winner: 2 It changes one file and keeps Load's signature.
	// <nil>
}

// Example_critiqueRound is a critique round: a writer drafts, a critic on
// another harness reads the draft against the code and lists defects, and
// the writer answers each one, taking the fix or rejecting it with a
// reason. A finding is a claim to investigate, not an order. The round
// ends when the critic finds nothing, or after two rounds.
func Example_critiqueRound() {
	ctx, closeProject := exampleContext()
	defer closeProject()
	repo := repoDir()
	note := filepath.Join(repo, "config", "NOTES.md")
	codex := says("Done: " + note + " names the three cases.")
	looks := 0
	claude := &script{answer: func(context.Context, string, json.RawMessage) (string, error) {
		if looks++; looks == 1 {
			return `{"defects":["The empty-file case is missing."]}`, nil
		}
		return `{"defects":[]}`, nil
	}}

	err := gimble.Run(ctx, "critique", map[string]gimble.ModelBinding{"critic": {Adapter: claude, Model: "claude-haiku-4-5-20251001"}, "writer": {Adapter: codex, Model: "gpt-5.6-luna"}}, func(ctx context.Context) error {
		writer := gimble.NewSession(ctx, "writer", repo)
		critic := gimble.NewSession(ctx, "critic", repo)
		if _, err := writer.Generate[gimble.Text](ctx, "Write "+note+": how the config loader handles a missing, an empty, and a malformed config file, from the code."); err != nil {
			return err
		}
		for round := 1; round <= 2; round++ {
			var defects []string
			err := gimble.Scope(ctx, "round", func(ctx context.Context) error {
				c, err := critic.Generate[critique](ctx, "Read "+note+" against the config loader's code. List each case it describes wrongly or leaves out. Change nothing.")
				if err != nil {
					return err
				}
				defects = c.Defects
				if len(defects) == 0 {
					return nil
				}
				gimble.Set(ctx, "defects", defects)
				_, err = writer.Generate[gimble.Text](ctx, "A reviewer read "+note+" and found the defects below. Fix each one that is real. For each you reject, say why in one line.\n\n"+gimble.ScopeText(ctx))
				return err
			})
			if err != nil {
				return err
			}
			fmt.Printf("round %d: %d defects\n", round, len(defects))
			if len(defects) == 0 {
				break
			}
		}
		return nil
	})
	fmt.Println(err)

	// Output:
	// round 1: 1 defects
	// round 2: 0 defects
	// <nil>
}

// Example_supervisedWorker is a supervised worker: a supervisor is a
// session and one instruction, attached to the turn it watches. At its
// interval it looks at what the worker did since its last look and steers
// the worker with any objection. It never gates the result: the worker's
// own answer comes back from Generate.
func Example_supervisedWorker() {
	ctx, closeProject := exampleContext()
	defer closeProject()
	repo := repoDir()
	codex := says("Done: Parse in config/parse.go, one test in config/parse_test.go, uncommitted.")
	claude := says(`{"objections":[]}`)

	err := gimble.Run(ctx, "supervised", map[string]gimble.ModelBinding{"coder": {Adapter: codex, Model: "gpt-5.6-luna"}, "taste": {Adapter: claude, Model: "claude-haiku-4-5-20251001"}}, func(ctx context.Context) error {
		coder := gimble.NewSession(ctx, "coder", repo)
		taste := gimble.NewSession(ctx, "taste", repo)
		result, err := coder.Generate[gimble.Text](ctx,
			"Add a Parse function to the config loader in "+repo+", with one test that shows it working. Leave the work uncommitted. Answer with what changed.",
			gimble.WithSupervisor(taste, "Don't let it build what the task does not ask for, or break a rule in AGENTS.md. Object to nothing else.", gimble.WithInterval(2*time.Minute)),
		)
		if err != nil {
			return err
		}
		fmt.Println(result)
		return nil
	})
	fmt.Println(err)

	// Output:
	// Done: Parse in config/parse.go, one test in config/parse_test.go, uncommitted.
	// <nil>
}

// Example_loopWithPlanner is a Loop with a planner: the planner, forked
// from the session that read the code, keeps the backlog; each task runs
// in its own scope with a coder forked from the same reading; what the
// body records with Set is what the planner sees before its next
// decision. The planner ends dispatch by choosing no task.
func Example_loopWithPlanner() {
	ctx, closeProject := exampleContext()
	defer closeProject()
	repo := repoDir()
	plans := 0
	codex := &script{answer: func(_ context.Context, _ string, schema json.RawMessage) (string, error) {
		if len(schema) == 0 {
			return "Done: Load caches the parsed config; config/load_test.go shows the second Load returning it.", nil
		}
		if plans++; plans == 1 {
			return `{"tasks":[{"name":"Read the config once","description":"Cache the parsed config on the loader so Load reads the file once per process.","definition_of_done":"A second Load returns the cached value, and a test shows it.","validation":{"command":"go test ./config/...","query":""}}],"next":0}`, nil
		}
		return `{"tasks":[],"next":null}`, nil
	}}

	err := gimble.Run(ctx, "loop", map[string]gimble.ModelBinding{"researcher": {Adapter: codex, Model: "gpt-5.6-luna"}}, func(ctx context.Context) error {
		researcher := gimble.NewSession(ctx, "researcher", repo)
		if _, err := researcher.Generate[gimble.Text](ctx, "Read the config loader in "+repo+" and its tests. Change nothing. Answer with what exists."); err != nil {
			return err
		}
		planner, err := researcher.Fork(ctx, "planner")
		if err != nil {
			return err
		}
		loop := gimble.Loop(ctx, "work", "The config loader in "+repo+" reads its file once per process, and a test shows it.", planner)
		for ctx, task := range loop.Tasks {
			fmt.Println("task:", task.Name)
			coder, err := researcher.Fork(ctx, "coder")
			if err != nil {
				return err
			}
			result, err := coder.Generate[gimble.Text](ctx, "Do the task in the scoped context below. Leave the work uncommitted. Answer with what changed and what you saw working.\n\n"+gimble.ScopeText(ctx))
			if err != nil {
				return err
			}
			gimble.Set(ctx, "result", string(result))
		}
		return loop.Err()
	})
	fmt.Println(err)

	// Output:
	// task: Read the config once
	// <nil>
}

// Example_worktreePerCandidate is a worktree per candidate: each coder
// edits its own git worktree, so two candidates never share a working
// tree. The prompt names the worktree by absolute path, and the workflow
// checks that nothing landed in the repository itself before it reads the
// result: an agent told a bare filename writes to the repository root.
// The worktree is removed on a ctx without cancel, so it goes even when
// the group has cancelled the candidate. It is not run here because it
// needs git.
func Example_worktreePerCandidate() {
	ctx, closeProject := exampleContext()
	defer closeProject()
	repo := repoDir()
	codex := says("Done: the loader reads its file once; the test is in config/load_test.go.")

	err := gimble.Run(ctx, "worktrees", map[string]gimble.ModelBinding{"coder": {Adapter: codex, Model: "gpt-5.6-luna"}}, func(ctx context.Context) error {
		group := gimble.Group(ctx, "candidates")
		for i := range 2 {
			group.Go("candidate", func(ctx context.Context) error {
				dir, err := os.MkdirTemp("", "candidate-")
				if err != nil {
					return err
				}
				defer func() { _ = os.RemoveAll(dir) }()
				if code, _, stderr, err := gimble.RunCommand(ctx, "add-worktree", repo, "git", "worktree", "add", "--detach", dir); err != nil {
					return err
				} else if code != 0 {
					return fmt.Errorf("git worktree add exited %d: %s", code, stderr)
				}
				defer func() {
					_, _, _, _ = gimble.RunCommand(context.WithoutCancel(ctx), "remove-worktree", repo, "git", "worktree", "remove", "--force", dir)
				}()

				coder := gimble.NewSession(ctx, "coder", dir)
				result, err := coder.Generate[gimble.Text](ctx, "In "+dir+", make the config loader read its file once per process, with a test that shows it. Write nowhere outside "+dir+". Leave the work uncommitted. Answer with what changed.")
				if err != nil {
					return err
				}
				if code, status, _, err := gimble.RunCommand(ctx, "status", repo, "git", "status", "--porcelain"); err != nil {
					return err
				} else if code != 0 || status != "" {
					return fmt.Errorf("candidate %d wrote outside its worktree (git status exited %d):\n%s", i+1, code, status)
				}
				gimble.Set(ctx, "result", string(result))
				return nil
			})
		}
		return group.Wait()
	})
	fmt.Println(err)
}

// Example_validationCommand is a validation command: the check is a
// command the workflow runs, chosen before the coder starts and never the
// coder's to edit. Its exit code is the verdict, and its output is what the
// coder is shown on the next try. It is not run here because the check
// runs the Go toolchain.
func Example_validationCommand() {
	ctx, closeProject := exampleContext()
	defer closeProject()
	repo := repoDir()
	codex := says("Done: Load caches the parsed config.")
	check := "go test ./config/..."

	err := gimble.Run(ctx, "validated", map[string]gimble.ModelBinding{"coder": {Adapter: codex, Model: "gpt-5.6-luna"}}, func(ctx context.Context) error {
		coder := gimble.NewSession(ctx, "coder", repo)
		prompt := "Make the config loader in " + repo + " read its file once per process, with a test that shows it. Leave the work uncommitted."
		for try := 1; try <= 3; try++ {
			if _, err := coder.Generate[gimble.Text](ctx, prompt); err != nil {
				return err
			}
			code, stdout, stderr, err := gimble.RunCommand(ctx, "check", repo, "sh", "-c", check)
			if err != nil {
				return err
			}
			if code == 0 {
				gimble.Set(ctx, "validated by", check)
				return nil
			}
			prompt = fmt.Sprintf("`%s` in %s exited %d:\n\n%s%s\nMake it pass. The check itself stays as it is.", check, repo, code, stdout, stderr)
		}
		return fmt.Errorf("%s still fails after three tries", check)
	})
	fmt.Println(err)
}

// runID is the id of the one run under project: the id the page shows,
// which an operator holds instead of a pointer.
func runID(project string) string {
	entries, err := os.ReadDir(filepath.Join(project, "runs"))
	if err != nil || len(entries) != 1 {
		panic(fmt.Sprintf("runs under %s: %v, %v", project, entries, err))
	}
	return entries[0].Name()
}

// Example_killedTurn is a turn killed by an operator and the loop that
// recovers: while a task's coder is mid-turn, the operator watching the
// page kills that turn by its id through the web runtime. The coder's
// Generate returns an error whose cause is the Killed; the session, the
// task's scope, and the Loop all keep running, so the body decides what to
// do next: here it tells the same session what was wrong and asks it to
// finish. Runtime.KillScope ends a whole scope the same way; then the
// scope's sessions are closed, its Group siblings run on, and a Loop
// records the task failed and shows the planner why.
func Example_killedTurn() {
	project, err := os.MkdirTemp("", "gimble-example-")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(project) }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runtime, err := web.NewRuntime(ctx, project, web.WithNoWeb())
	if err != nil {
		panic(err)
	}
	repo := repoDir()
	coding := make(chan struct{})
	turns, plans := 0, 0
	codex := &script{answer: func(ctx context.Context, _ string, schema json.RawMessage) (string, error) {
		if len(schema) == 0 { // the coder: its first turn runs until it is killed
			if turns++; turns == 1 {
				close(coding)
				<-ctx.Done()
				return "", ctx.Err()
			}
			return "Done: the wrong edit is undone and Load caches the parsed config.", nil
		}
		if plans++; plans == 1 {
			return `{"tasks":[{"name":"Read the config once","description":"Cache the parsed config on the loader.","definition_of_done":"A second Load returns the cached value.","validation":{"command":"","query":""}}],"next":0}`, nil
		}
		return `{"tasks":[],"next":null}`, nil
	}}

	var killErr error
	var operator sync.WaitGroup
	err = runtime.Run(ctx, "recover", map[string]gimble.ModelBinding{"coder": {Adapter: codex, Model: "gpt-5.6-luna"}, "planner": {Adapter: codex, Model: "gpt-5.6-luna"}}, func(ctx context.Context) error {
		operator.Go(func() { // the operator, holding only ids from the page
			<-coding
			killErr = runtime.KillTurn(runID(project), "work.1/task.1/coder.1/turn.1", "tyler", "editing the wrong file")
		})
		planner := gimble.NewSession(ctx, "planner", repo)
		loop := gimble.Loop(ctx, "work", "The config loader in "+repo+" reads its file once per process.", planner)
		for ctx, task := range loop.Tasks {
			coder := gimble.NewSession(ctx, "coder", repo)
			result, err := coder.Generate[gimble.Text](ctx, "Do the task in the scoped context below. Leave the work uncommitted. Answer with what changed.\n\n"+gimble.ScopeText(ctx))
			if kill, ok := errors.AsType[gimble.Killed](err); ok {
				fmt.Printf("%s: turn killed by %s: %s\n", task.Name, kill.By, kill.Reason)
				result, err = coder.Generate[gimble.Text](ctx, "Your last turn was stopped by "+kill.By+": "+kill.Reason+". Undo what was wrong, then finish the task. Answer with what changed.")
			}
			if err != nil {
				return err
			}
			gimble.Set(ctx, "result", string(result))
			fmt.Println(result)
		}
		return loop.Err()
	})
	operator.Wait()
	fmt.Println("kill:", killErr)
	fmt.Println(err)

	// Output:
	// Read the config once: turn killed by tyler: editing the wrong file
	// Done: the wrong edit is undone and Load caches the parsed config.
	// kill: <nil>
	// <nil>
}
