// Command notebook runs a bounded informational example through the real Codex harness.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/codex"
	"github.com/tylergannon/gimbal/internal/workflows/notebook"
	"os"
	"path/filepath"
	"time"
)

func main() {
	example := flag.String("example", "", "semantic-index or sprint-plan")
	fixture := flag.String("fixture", "", "input corpus or brief file")
	project := flag.String("project", "", "fresh local directory for run records")
	revision := flag.String("revision", "", "evaluated Gimbal Git revision")
	output := flag.String("output", "", "evaluation JSON destination")
	model := flag.String("model", "gpt-5.6-luna", "Codex model for author and independent reviewer")
	flag.Parse()
	if (*example != "semantic-index" && *example != "sprint-plan") || *fixture == "" || *project == "" || *output == "" || *revision == "" {
		fmt.Fprintln(os.Stderr, "give --example, --fixture, --project, --revision and --output")
		os.Exit(2)
	}
	input, err := os.ReadFile(*fixture)
	if err != nil {
		panic(err)
	}
	dir, err := filepath.Abs(*project)
	if err != nil {
		panic(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		panic("project directory must be fresh")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		panic(err)
	}
	adapter := codex.New()
	models := map[gimbal.WorkflowRole]gimbal.ModelBinding{}
	for _, role := range []gimbal.WorkflowRole{gimbal.RoleBulkClassification, gimbal.RoleSprintPlanning, gimbal.RoleCodeReview} {
		models[role] = gimbal.ModelBinding{Adapter: adapter, Model: *model, Effort: "low"}
	}
	started := time.Now().UTC()
	ctx, cancel := context.WithTimeout(gimbal.Project(context.Background(), dir), 8*time.Minute)
	defer cancel()
	var result any
	err = gimbal.Run(ctx, *example, models, func(ctx context.Context) error {
		var runErr error
		if *example == "semantic-index" {
			result, runErr = notebook.SemanticIndex(ctx, gimbal.Env{WorkDir: dir}, string(input))
		} else {
			result, runErr = notebook.SprintPlan(ctx, gimbal.Env{WorkDir: dir}, string(input))
		}
		return runErr
	})
	status, failure := "passed", ""
	if err != nil {
		status, failure = "failed", err.Error()
	}
	digest := sha256.Sum256(input)
	runs, _ := os.ReadDir(filepath.Join(dir, "runs"))
	ids := []string{}
	for _, run := range runs {
		if run.IsDir() {
			ids = append(ids, run.Name())
		}
	}
	record := map[string]any{"example": *example, "gimbalRevision": *revision, "model": *model, "harness": "Codex app-server", "startedAt": started.Format(time.RFC3339), "finishedAt": time.Now().UTC().Format(time.RFC3339), "fixtureSHA256": hex.EncodeToString(digest[:]), "status": status, "error": failure, "runIDs": ids, "result": result, "scope": "bounded fixture; structural checks and independent agent review; informational, not a release gate"}
	data, _ := json.MarshalIndent(record, "", "  ")
	if writeErr := os.WriteFile(*output, append(data, '\n'), 0600); writeErr != nil {
		panic(writeErr)
	}
	fmt.Printf("%s: %s; record %s; runs %v\n", *example, status, *output, ids)
	if err != nil {
		os.Exit(1)
	}
}
