package gimbal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/runstore"
)

func TestMissingRequirementsPreventNativeAdmission(t *testing.T) {
	project := t.TempDir()
	marker := filepath.Join(project, "must-not-exist")
	err := Run(Project(t.Context(), project), string(runstore.NewRunID()), "missing-secret", nil, func(ctx context.Context) error {
		ctx = WithRequirements(ctx, DefaultStore, "MISSING")
		return Check(ctx, "probe", project, "touch", marker)
	})
	if err == nil {
		t.Fatal("missing requirement admitted work")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("command launched", err)
	}
}
func TestFiniteProducerAndWholeStoreConsumer(t *testing.T) {
	err := runTest(t, nil, func(ctx context.Context) error {
		_, _, _, err := RunCommand(WithYields(ctx, DefaultStore, "REQUIRED", "EXTRA"), "produce", "", "sh", "-c", `printf 'REQUIRED=one\nEXTRA=two\n' > "$ENV_FILE"`)
		if err != nil {
			return err
		}
		if err := Check(ctx, "unrelated", "", "true"); err != nil {
			return fmt.Errorf("producer declaration leaked into unrelated command: %w", err)
		}
		if err := Check(WithYields(ctx, "second", "SECOND"), "second-producer", "", "sh", "-c", `printf 'SECOND=three\n' > "$ENV_FILE"`); err != nil {
			return err
		}
		ctx = WithRequirements(ctx, DefaultStore, "REQUIRED")
		ctx = WithRequirements(ctx, "second", "SECOND")
		code, out, _, err := RunCommand(ctx, "consume", "", "sh", "-c", `test "$REQUIRED" = one && test "$EXTRA" = two && test "$SECOND" = three && printf ok`)
		if err != nil {
			return err
		}
		if code != 0 || out != "ok" {
			t.Fatalf("consumer %d %q", code, out)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestLocalAgentCallbackFollowsConsumingScope(t *testing.T) {
	adapter := &fake{answer: func(ctx context.Context, session, prompt string, schema json.RawMessage, emit func(AgentEvent) error) (string, error) {
		_, rest, ok := strings.Cut(prompt, "via GET ")
		if !ok {
			return "", fmt.Errorf("no callback")
		}
		url, _, _ := strings.Cut(rest, ". Call this endpoint")
		response, err := http.Get(url)
		if err != nil {
			return "", err
		}
		defer func() { _ = response.Body.Close() }()
		var values map[string]string
		if err := json.NewDecoder(response.Body).Decode(&values); err != nil {
			return "", err
		}
		if values["TOKEN"] != "child" || values["EXTRA"] != "visible" {
			return "", fmt.Errorf("wrong scope/store %v", values)
		}
		if strings.Contains(prompt, "TOKEN=child") {
			return "", fmt.Errorf("automatic injection")
		}
		return "done", nil
	}}
	err := runTest(t, bind(adapter, "test", RoleDevOpsTasks), func(ctx context.Context) error {
		session := NewSession(ctx, RoleDevOpsTasks, "")
		return Scope(ctx, "child", func(ctx context.Context) error {
			_, _, _, err := RunCommand(WithYields(ctx, "private", "TOKEN", "EXTRA"), "produce", "", "sh", "-c", `printf 'TOKEN=child\nEXTRA=visible\n' > "$ENV_FILE"`)
			if err != nil {
				return err
			}
			_, err = session.Generate[Text](WithRequirements(ctx, "private", "TOKEN"), "Read callback")
			return err
		})
	})
	if err != nil {
		t.Fatal(err)
	}
}
