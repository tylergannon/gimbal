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
	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/agy"
	"github.com/tylergannon/gimble/claude"
	"github.com/tylergannon/gimble/codex"
	"github.com/tylergannon/gimble/internal/modelalias"
	"github.com/tylergannon/gimble/web"
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
	timeout                     time.Duration
}

func runPrompt(args []string, stdout, stderr io.Writer, getenv func(string) string) error {
	flags := flag.NewFlagSet("gimble run-prompt", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var options runPromptOptions
	flags.StringVar(&options.model, "model", "", "model name or alias; omitted inside Codex or Claude selects the opposite provider")
	flags.StringVar(&options.modelVersion, "model-version", "", "exact model-family version")
	flags.StringVar(&options.effort, "effort", "", "reasoning effort: low, medium, high, xhigh, or max")
	flags.StringVar(&options.outputSchema, "output-schema", "", "exact JSON Schema for structured output; omit for plain text")
	flags.StringVar(&options.workdir, "workdir", ".", "agent workspace")
	flags.StringVar(&options.logs, "logs", "", "new project log directory; default is a temporary directory")
	flags.DurationVar(&options.timeout, "timeout", 20*time.Minute, "turn timeout")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("run-prompt requires exactly one PROMPT argument")
	}
	selection, err := resolvePromptModel(options, detectPromptCaller(getenv))
	if err != nil {
		return err
	}
	workdir, err := promptWorkdir(options.workdir)
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	runtime, err := web.NewRuntime(ctx, projectDir, web.WithNoWeb())
	if err != nil {
		return err
	}
	adapter, err := promptAdapter(selection.Harness)
	if err != nil {
		return err
	}

	var output []byte
	run := func() error {
		return runtime.Run(ctx, "run-prompt", func(ctx context.Context) error {
			turnCtx, cancel := context.WithTimeout(ctx, options.timeout)
			defer cancel()
			session := gimble.NewSession(ctx, "run-prompt", adapter, selection.Model, workdir)
			if options.outputSchema == "" {
				result, err := session.Generate[gimble.Text](turnCtx, flags.Arg(0))
				output = []byte(result)
				return err
			}
			runPromptSchema = json.RawMessage(options.outputSchema)
			runPromptValidator = compiled
			result, err := session.Generate[runPromptJSON](turnCtx, flags.Arg(0))
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
	return nil
}

func writeRunPromptLogs(stderr io.Writer, projectDir string) error {
	runDir, err := soleRunDir(projectDir)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stderr, "Logs: %s\n", runDir)
	return err
}

func detectPromptCaller(getenv func(string) string) promptCaller {
	if strings.TrimSpace(getenv("CLAUDE_CODE_SESSION_ID")) != "" {
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
			name = "fable"
		case promptCallerClaude:
			name = "gpt"
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
	if options.effort != "" && resolved.Harness != "agy" {
		return modelalias.ResolvedSelection{}, fmt.Errorf("--effort is not supported by the current %s harness", resolved.Harness)
	}
	if resolved.Harness == "agy" && (resolved.Effort == "xhigh" || resolved.Effort == "max") {
		return modelalias.ResolvedSelection{}, fmt.Errorf("agy supports effort low, medium, or high, not %q", resolved.Effort)
	}
	return resolved, nil
}

func promptAdapter(name string) (gimble.HarnessAdapter, error) {
	switch name {
	case "agy":
		return agy.New(), nil
	case "claude":
		return claude.New(), nil
	case "codex":
		return codex.New(), nil
	default:
		return nil, fmt.Errorf("unknown harness %q", name)
	}
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
		return os.MkdirTemp("", "gimble-run-prompt-")
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
	if err := compiler.AddResource("urn:gimble:run-prompt", document); err != nil {
		return nil, err
	}
	return compiler.Compile("urn:gimble:run-prompt")
}
