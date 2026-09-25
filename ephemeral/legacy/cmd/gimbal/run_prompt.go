package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/tylergannon/gimbal/internal/modelalias"
	"github.com/tylergannon/gimbal/program"
)

type promptCaller string

const (
	promptCallerNone   promptCaller = ""
	promptCallerClaude promptCaller = "claude"
	promptCallerCodex  promptCaller = "codex"
)

type runPromptOptions struct {
	model        string
	modelVersion string
	effort       string
	outputSchema string
	workdir      string
	logs         string
	timeout      time.Duration
}

func newRunPromptCommand() *cobra.Command {
	var options runPromptOptions
	command := &cobra.Command{
		Use:   "run-prompt [flags] PROMPT",
		Short: "Run one coding-agent prompt",
		Long:  "Run one coding-agent prompt with real tool access. Output is plain text unless --output-schema supplies an exact JSON Schema.",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			return runPrompt(command, options, detectPromptCaller(os.Getenv), args[0])
		},
	}
	command.Flags().StringVar(&options.model, "model", "", "model name or alias; omitted inside Codex or Claude selects the opposite provider")
	command.Flags().StringVar(&options.modelVersion, "model-version", "", "exact model-family version")
	command.Flags().StringVar(&options.effort, "effort", "", "reasoning effort: low, medium, high, xhigh, or max")
	command.Flags().StringVar(&options.outputSchema, "output-schema", "", "exact JSON Schema for structured output; omit for plain text")
	command.Flags().StringVar(&options.workdir, "workdir", ".", "agent workspace")
	command.Flags().StringVar(&options.logs, "logs", "", "new run log directory; default is a temporary directory")
	command.Flags().DurationVar(&options.timeout, "timeout", 20*time.Minute, "turn timeout")
	return command
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
			return modelalias.ResolvedSelection{}, fmt.Errorf("--model is required outside Codex or Claude")
		}
	}
	selection := modelalias.Selection{Name: name}
	if options.modelVersion != "" {
		selection.Version = options.modelVersion
		selection.VersionPresent = true
	}
	if options.effort != "" {
		selection.Effort = options.effort
		selection.EffortPresent = true
	}
	return modelalias.ResolveModel(selection)
}

func runPrompt(command *cobra.Command, options runPromptOptions, caller promptCaller, prompt string) error {
	selection, err := resolvePromptModel(options, caller)
	if err != nil {
		return err
	}
	runtime, err := program.NewRuntime(program.Config{
		Workdir: options.workdir, RunDir: options.logs,
		DefaultModel: selection.Model, ReasoningEffort: selection.Effort, Timeout: options.timeout,
	})
	if err != nil {
		return err
	}
	defer runtime.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	request := program.CodergenRequest{Name: "run-prompt", Prompt: prompt, Model: selection.Model, Effort: selection.Effort}
	out := command.OutOrStdout()
	if options.outputSchema == "" {
		text, err := program.Codergen[string](ctx, runtime, request)
		if err != nil {
			return err
		}
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		if _, err := fmt.Fprint(out, text); err != nil {
			return err
		}
	} else {
		if err := json.Unmarshal([]byte(options.outputSchema), &request.JSONSchema); err != nil {
			return fmt.Errorf("decode --output-schema: %w", err)
		}
		result, err := program.Codergen[json.RawMessage](ctx, runtime, request)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "%s\n", result); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(command.ErrOrStderr(), "Logs: %s\n", runtime.RunDir)
	return err
}
