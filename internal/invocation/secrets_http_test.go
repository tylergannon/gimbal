package invocation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/internal/secretshttp"
	"github.com/tylergannon/gimbal/runstore"
)

func TestHTTPSecretsWorkflowProducerAndConsumer(t *testing.T) {
	project := t.TempDir()
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	identityFile, tokenFile := filepath.Join(project, "identity"), filepath.Join(project, "token")
	if err := os.WriteFile(identityFile, []byte(identity.String()), 0600); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("workflow-fixture-", 3)
	if err := os.WriteFile(tokenFile, []byte(token), 0600); err != nil {
		t.Fatal(err)
	}
	store, err := secretshttp.Open(filepath.Join(project, "encrypted"), identityFile)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(secretshttp.Handler(store, token))
	defer server.Close()
	definition, ok := gimbal.DefaultRegistry.Secrets("http")
	if !ok || !definition.SharedAcrossProcesses {
		t.Fatal("shared HTTP backend not registered")
	}
	raw, _ := json.Marshal(secretshttp.Config{Endpoint: server.URL, TokenFile: tokenFile})
	if err := definition.ValidateConfiguration(raw); err != nil {
		t.Fatal(err)
	}
	selected, err := definition.Construct(t.Context(), raw)
	if err != nil {
		t.Fatal(err)
	}
	ctx := effects.WithConnection(gimbal.Project(t.Context(), project), backend.Connection{Secrets: selected})
	err = gimbal.Run(ctx, string(runstore.NewRunID()), "http-secrets", nil, func(ctx context.Context) error {
		if err := gimbal.Check(gimbal.WithYields(ctx, "db", "URL", "EXTRA"), "produce", project, "sh", "-c", `printf 'URL=workflow-address\nEXTRA=whole-store\n' > "$ENV_FILE"`); err != nil {
			return err
		}
		marker := filepath.Join(project, "must-not-start")
		if err := gimbal.Check(gimbal.WithRequirements(ctx, "db", "MISSING"), "missing", project, "touch", marker); err == nil {
			return fmt.Errorf("missing prerequisite admitted command")
		}
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			return fmt.Errorf("missing prerequisite started command")
		}
		return gimbal.Check(gimbal.WithRequirements(ctx, "db", "URL"), "consume", project, "sh", "-c", `test "$URL" = workflow-address && test "$EXTRA" = whole-store`)
	})
	if err != nil {
		t.Fatal(err)
	}
}
