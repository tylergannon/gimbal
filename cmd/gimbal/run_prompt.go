package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/agentexecutor"
	"github.com/tylergannon/gimbal/internal/modelalias"
	"github.com/tylergannon/gimbal/web"
)

type promptCaller string

const (
	promptCallerNone   promptCaller = ""
	promptCallerClaude promptCaller = "claude"
	promptCallerCodex  promptCaller = "codex"
)

type runPromptOptions struct {
	model, modelVersion, effort string
	outputSchema                string
	workdir, logs               string
	resume, close               string
	timeout                     time.Duration
}

func runPrompt(args []string, stdout, stderr io.Writer, getenv func(string) string) error {
	flags := flag.NewFlagSet("gimbal run-prompt", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		_, _ = fmt.Fprintln(stderr, "Run a prompt or continue a retained session with --resume ID, separate from any hosted instance. New sessions default to latest sol inside Claude, latest opus inside ChatGPT/Codex, and otherwise require --model. Resuming preserves the resolved model, effort, harness and workspace. Sessions are retained under ~/.gimbal/run-prompt/sessions without automatic expiry; --close ID releases a session and prevents further turns, preserving its history. The answer alone goes to stdout; a continuation command is printed last on stderr. TYPESAFE_API_KEY is needed only for Jev supervision through WithSupervisor. Pi uses --model pi/diffusion/<model-id> (or the deepseek-flash alias); set DIFFUSION_API_KEY. Pi runs natively inside Gimbal. Gimbal owns Pi's config and session directory. Pi effort is unsupported. Logs use a temporary directory unless --logs names a fresh empty directory; keep it separate from projects admitted to an instance.")
		_, _ = fmt.Fprintln(stderr, "Usage of gimbal run-prompt:")
		_, _ = fmt.Fprintln(stderr, "  gimbal run-prompt [flags] PROMPT")
		_, _ = fmt.Fprintln(stderr, "  gimbal run-prompt --resume ID [flags] PROMPT")
		_, _ = fmt.Fprintln(stderr, "  gimbal run-prompt --close ID")
		flags.PrintDefaults()
	}
	var options runPromptOptions
	flags.StringVar(&options.model, "model", "", "model name or alias; Pi: pi/diffusion/MODEL; OpenCode: opencode/MODEL or opencode/PROVIDER/MODEL; omitted inside Codex or Claude selects the opposite provider")
	flags.StringVar(&options.modelVersion, "model-version", "", "exact model-family version")
	flags.StringVar(&options.effort, "effort", "", "reasoning effort: low, medium, high, xhigh, or max")
	flags.StringVar(&options.outputSchema, "output-schema", "", "exact JSON Schema for structured output; omit for plain text")
	flags.StringVar(&options.workdir, "workdir", ".", "agent workspace")
	flags.StringVar(&options.logs, "logs", "", "new directory for .gimbal run records; default is a temporary directory")
	flags.StringVar(&options.resume, "resume", "", "continue this retained session")
	flags.StringVar(&options.close, "close", "", "close this retained session without sending a prompt")
	flags.DurationVar(&options.timeout, "timeout", 20*time.Minute, "turn timeout")
	if err := flags.Parse(args); err != nil {
		return err
	}
	var emptySessionFlag string
	flags.Visit(func(f *flag.Flag) {
		if (f.Name == "resume" || f.Name == "close") && strings.TrimSpace(f.Value.String()) == "" {
			emptySessionFlag = f.Name
		}
	})
	if emptySessionFlag != "" {
		return fmt.Errorf("--%s requires a nonblank session ID", emptySessionFlag)
	}
	if options.close != "" {
		if flags.NArg() != 0 || flags.NFlag() != 1 {
			return errors.New("use --close ID alone, without a prompt or other flags")
		}
		return closePromptSession(options.close, stderr)
	}
	if flags.NArg() != 1 {
		return errors.New("run-prompt requires exactly one PROMPT argument")
	}
	workdirExplicit := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "workdir" {
			workdirExplicit = true
		}
	})
	root, config, err := preparePromptSession(options, workdirExplicit, detectPromptCaller(getenv))
	if err != nil {
		return err
	}
	projectDir, err := promptProjectDir(options.logs)
	if err != nil {
		return err
	}

	var compiled *jsonschema.Schema
	if options.outputSchema != "" {
		compiled, err = compilePromptSchema(json.RawMessage(options.outputSchema))
		if err != nil {
			return fmt.Errorf("decode --output-schema: %w", err)
		}
	}
	if options.resume == "" {
		if err = agentexecutor.Create(root, config); err != nil {
			return err
		}
	}
	if _, err = fmt.Fprintf(stderr, "Session: %s\nState: %s\n", config.ID, promptSessionDir(root, string(config.ID))); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	instance, err := web.NewInstance(ctx, filepath.Join(projectDir, ".gimbal"), []string{projectDir}, web.WithNoWeb())
	if err != nil {
		return err
	}
	runtime, err := instance.Owner.Project(projectDir)
	if err != nil {
		return err
	}
	adapter := &promptSessionAdapter{Adapter: agentexecutor.New(config.Harness), id: string(config.ID)}
	adapter.Root = root
	models := map[gimbal.WorkflowRole]gimbal.ModelBinding{
		"run-prompt": {Adapter: adapter, Model: config.Model, Effort: config.Effort},
	}

	var output []byte
	run := func() error {
		turnCtx, cancel := context.WithTimeout(ctx, options.timeout)
		defer cancel()
		if options.outputSchema != "" {
			runPromptSchema = json.RawMessage(options.outputSchema)
			runPromptValidator = compiled
		}
		return runtime.Run(turnCtx, "run-prompt", models, func(ctx context.Context) error {
			session := gimbal.NewSession(ctx, "run-prompt", config.Workdir)
			if options.outputSchema == "" {
				result, err := session.Generate[gimbal.Text](ctx, flags.Arg(0))
				output = []byte(result)
				return err
			}
			result, err := session.Generate[runPromptJSON](ctx, flags.Arg(0))
			output = append([]byte(nil), result...)
			return err
		})
	}
	if options.outputSchema != "" {
		runPromptSchemaMu.Lock()
		defer runPromptSchemaMu.Unlock()
	}
	runErr := run()
	logsErr := writeRunPromptLogs(stderr, projectDir)
	if runErr != nil {
		return runErr
	}
	if logsErr != nil {
		return logsErr
	}
	if options.outputSchema == "" {
		if !bytes.HasSuffix(output, []byte("\n")) {
			output = append(output, '\n')
		}
	} else {
		output = append(output, '\n')
	}
	if _, err := stdout.Write(output); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stderr, "To continue this session, run: gimbal run-prompt --resume %s \"Your next prompt\"\n", config.ID)
	return err
}

func writeRunPromptLogs(stderr io.Writer, projectDir string) error {
	runDir, err := soleRunDir(filepath.Join(projectDir, ".gimbal"))
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stderr, "Logs: %s\n", runDir)
	return err
}

func detectPromptCaller(getenv func(string) string) promptCaller {
	if strings.TrimSpace(getenv("CLAUDE_CODE_SESSION_ID")) != "" || strings.TrimSpace(getenv("CLAUDECODE")) != "" {
		return promptCallerClaude
	}
	if strings.TrimSpace(getenv("CODEX_THREAD_ID")) != "" {
		return promptCallerCodex
	}
	return promptCallerNone
}

func resolvePromptModel(options runPromptOptions, caller promptCaller) (modelalias.ResolvedSelection, error) {
	name := strings.TrimSpace(options.model)
	if name == "" {
		switch caller {
		case promptCallerCodex:
			name = "opus"
		case promptCallerClaude:
			name = "sol"
		default:
			return modelalias.ResolvedSelection{}, errors.New("--model is required outside Codex or Claude")
		}
	}
	selection := modelalias.Selection{Name: name}
	if options.modelVersion != "" {
		selection.Version, selection.VersionPresent = options.modelVersion, true
	}
	if options.effort != "" {
		selection.Effort, selection.EffortPresent = options.effort, true
	}
	resolved, err := modelalias.Resolve(selection)
	if err != nil {
		return modelalias.ResolvedSelection{}, err
	}
	if resolved.Harness == "agy" && (resolved.Effort == "xhigh" || resolved.Effort == "max") {
		return modelalias.ResolvedSelection{}, fmt.Errorf("agy supports effort low, medium, or high, not %q", resolved.Effort)
	}
	return resolved, nil
}

func promptWorkdir(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve workdir: %w", err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", fmt.Errorf("workdir %s: %w", absolute, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("workdir %s is not a directory", absolute)
	}
	return absolute, nil
}

func promptProjectDir(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return os.MkdirTemp("", "gimbal-run-prompt-")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve logs: %w", err)
	}
	entries, err := os.ReadDir(absolute)
	if err == nil && len(entries) != 0 {
		return "", fmt.Errorf("log directory %s is not empty", absolute)
	}
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect log directory: %w", err)
	}
	if err := os.MkdirAll(absolute, 0o755); err != nil {
		return "", fmt.Errorf("prepare log directory: %w", err)
	}
	return absolute, nil
}

func soleRunDir(projectDir string) (string, error) {
	entries, err := os.ReadDir(filepath.Join(projectDir, "runs"))
	if err != nil {
		return "", err
	}
	if len(entries) != 1 || !entries[0].IsDir() {
		return "", fmt.Errorf("expected one run directory in %s", projectDir)
	}
	return filepath.Join(projectDir, "runs", entries[0].Name()), nil
}

var (
	runPromptSchemaMu  sync.Mutex
	runPromptSchema    json.RawMessage
	runPromptValidator *jsonschema.Schema
)

type runPromptJSON json.RawMessage

func (runPromptJSON) Schema() json.RawMessage { return runPromptSchema }

func (runPromptJSON) ValidateJSON(raw []byte) error {
	if runPromptValidator == nil {
		return errors.New("run-prompt output schema is not configured")
	}
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	return runPromptValidator.Validate(value)
}

func (r *runPromptJSON) UnmarshalJSON(raw []byte) error {
	*r = append((*r)[:0], raw...)
	return nil
}

func compilePromptSchema(raw json.RawMessage) (*jsonschema.Schema, error) {
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("urn:gimbal:run-prompt", document); err != nil {
		return nil, err
	}
	return compiler.Compile("urn:gimbal:run-prompt")
}
