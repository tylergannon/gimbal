package claude

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/workflows/implementation"
)

// Run with GIMBAL_LIVE=1 to exercise Claude's completion decisions in real turns.
func TestLiveCompletionScope(t *testing.T) {
	if os.Getenv("GIMBAL_LIVE") != "1" {
		t.Skip("set GIMBAL_LIVE=1 to run real Claude turns")
	}
	project, workdir := t.TempDir(), t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	ctx = gimbal.Project(ctx, project)
	adapter := New()
	model := os.Getenv("GIMBAL_CLAUDE_MODEL")
	if model == "" {
		model = "claude-haiku-4-5-20251001"
	}
	var selected string
	var assessment implementation.Assessment
	var receipt gimbal.Text
	var followup implementation.Assessment
	err := gimbal.Run(ctx, "claude-completion-scope", map[gimbal.WorkflowRole]gimbal.ModelBinding{
		"planner":   {Adapter: adapter, Model: model},
		"validator": {Adapter: adapter, Model: model},
		"worker":    {Adapter: adapter, Model: model},
	}, func(ctx context.Context) error {
		planner := gimbal.NewSession(ctx, "planner", workdir)
		loop := gimbal.PromiseLoop(ctx, "implementation", "Create the absent file final.txt. Select one task, but do not perform it.", planner)
		for taskCtx, task := range loop.Tasks {
			selected = task.Name
			validator := gimbal.NewSession(taskCtx, "validator", workdir)
			var err error
			assessment, err = validator.Generate[implementation.Assessment](taskCtx, "Inspect final.txt in the current directory. Report the unmet requirement in SubstantialGaps and set ValidationPassed false. Finish this assessment without creating the file.")
			if err != nil {
				return err
			}
			break
		}
		if err := loop.Err(); err != nil {
			return err
		}
		worker := gimbal.NewSession(ctx, "worker", workdir)
		var err error
		receipt, err = worker.Generate[gimbal.Text](ctx, "Run `sleep 6; printf 'finished-390\\n' > receipt.txt` with the Bash tool in the background. Wait for its completion notification, then read receipt.txt and report its exact contents. Do not finish this response while that command is pending.")
		if err != nil {
			return err
		}
		followup, err = worker.Generate[implementation.Assessment](ctx, "Without using tools, assess whether the prior background command produced its requested receipt. Set ValidationPassed true if it did and include the filename and exact receipt text in Observed. Return empty gap lists.")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if selected == "" {
		t.Fatal("unfinished goal produced no planner task")
	}
	if assessment.ValidationPassed || len(assessment.SubstantialGaps) == 0 {
		t.Fatalf("QA assessment = %+v, want completed negative verdict", assessment)
	}
	if _, err := os.Stat(filepath.Join(workdir, "final.txt")); !os.IsNotExist(err) {
		t.Fatalf("goal file should still be absent: %v", err)
	}
	if !strings.Contains(string(receipt), "finished-390") {
		t.Errorf("receipt = %q, want completed command receipt", receipt)
	}
	if !followup.ValidationPassed || !strings.Contains(followup.Observed, "receipt.txt") || !strings.Contains(followup.Observed, "finished-390") {
		t.Errorf("different-schema follow-up = %+v, want prior receipt", followup)
	}
	t.Logf("%s selected %q for unfinished goal; QA reported %v; background receipt %q; follow-up %+v", model, selected, assessment.SubstantialGaps, receipt, followup)
}
