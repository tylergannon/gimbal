package program

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tylergannon/gimbal/harness"
	"github.com/tylergannon/gimbal/internal/modelalias"
)

// CodergenRequest is one agent turn. With a JSONSchema the agent's result is
// validated against it and decoded into T; without one T should be string
// and receives the agent's final text.
type CodergenRequest struct {
	Name       string
	Prompt     string
	JSONSchema map[string]any
	Model      string
	Effort     string
}

// Codergen runs one turn on a fresh native session of the harness that
// serves the requested model and decodes the result into T.
func Codergen[T any](ctx context.Context, runtime *Runtime, request CodergenRequest) (T, error) {
	var zero T
	if runtime == nil {
		return zero, errors.New("runtime is nil")
	}
	if strings.TrimSpace(request.Prompt) == "" {
		return zero, errors.New("prompt must not be empty")
	}
	var schema json.RawMessage
	if request.JSONSchema != nil {
		encoded, err := json.Marshal(request.JSONSchema)
		if err != nil {
			return zero, fmt.Errorf("encode JSON schema: %w", err)
		}
		schema = encoded
	}
	model := request.Model
	if model == "" {
		model = runtime.DefaultModel
	}
	selection := modelalias.Selection{Name: model}
	if effort := cmpOr(request.Effort, runtime.ReasoningEffort); effort != "" {
		selection.Effort, selection.EffortPresent = effort, true
	}
	resolved, err := modelalias.ResolveModel(selection)
	if err != nil {
		return zero, err
	}
	adapter := runtime.Adapters[resolved.Harness]
	if adapter == nil {
		return zero, fmt.Errorf("no adapter for harness %q", resolved.Harness)
	}
	name := strings.TrimSpace(request.Name)
	if name == "" {
		name = "codergen"
	}

	id, started, err := runtime.startOperation("codergen", "", map[string]any{"name": name, "model": resolved.Model})
	if err != nil {
		return zero, err
	}
	logPath := filepath.Join(runtime.RunDir, "agents", id+".jsonl")
	log, err := openAgentLog(logPath)
	if err != nil {
		return zero, err
	}
	raw, runErr := runTurn(ctx, adapter, runtime, resolved, schema, request.Prompt, log.write)
	closeErr := log.close()
	if runErr == nil {
		runErr = closeErr
	}
	if runErr == nil {
		if err := json.Unmarshal(raw, &zero); err != nil {
			runErr = fmt.Errorf("decode result: %w", err)
		}
	}
	endErr := runtime.endOperation(id, "codergen", started, map[string]any{"error": errorString(runErr), "run_log": logPath})
	if runErr != nil {
		return zero, runErr
	}
	return zero, endErr
}

func runTurn(ctx context.Context, adapter harness.HarnessAdapter, runtime *Runtime, resolved modelalias.ResolvedSelection, schema json.RawMessage, prompt string, onEvent harness.OnEvent) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, runtime.Timeout)
	defer cancel()
	sessionID, err := adapter.CreateSession(resolved.Model, runtime.Workdir)
	if err != nil {
		return nil, err
	}
	return adapter.RunTurn(ctx, harness.RunTurnInput{
		SessionID:       sessionID,
		Model:           resolved.Model,
		ReasoningEffort: resolved.Effort,
		OutputSchema:    schema,
		Workdir:         runtime.Workdir,
		Parts:           []harness.ContentPart{{Type: harness.ContentPartText, Text: prompt}},
	}, onEvent)
}

// agentLog appends one timestamped JSON line per harness event.
type agentLog struct {
	mu   sync.Mutex
	file *os.File
	err  error
}

func openAgentLog(path string) (*agentLog, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open agent log: %w", err)
	}
	return &agentLog{file: file}, nil
}

func (l *agentLog) write(event harness.Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.err != nil {
		return
	}
	stamped := make(harness.Event, len(event)+1)
	maps.Copy(stamped, event)
	stamped["ts"] = time.Now().UTC().Format(time.RFC3339Nano)
	l.err = json.NewEncoder(l.file).Encode(stamped)
}

func (l *agentLog) close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.file.Close(); l.err == nil {
		l.err = err
	}
	if l.err != nil {
		return fmt.Errorf("write agent log: %w", l.err)
	}
	return nil
}

func cmpOr(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
