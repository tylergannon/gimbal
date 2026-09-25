package program

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/checklist"
)

const testChecklist = `---
items:
  - name: First
    check: first works
    done: true
  - name: Second
    check: second works
---

# The ledger body
`

func TestLoopReconcilesHandMarkedDoneAndSelectsFailure(t *testing.T) {
	path := writeChecklist(t, testChecklist)
	var validated []string
	var got Iteration
	for iteration, err := range Loop(context.Background(), path, LoopOptions{
		Validate: func(_ context.Context, item checklist.Item) (Validation, error) {
			validated = append(validated, item.Name)
			return Validation{Passed: item.Name == "Second", Notes: item.Name + " needs work"}, nil
		},
	}) {
		if err != nil {
			t.Fatal(err)
		}
		got = iteration
		break
	}
	if want := []string{"First", "Second"}; strings.Join(validated, ",") != strings.Join(want, ",") {
		t.Fatalf("validated %v, want %v", validated, want)
	}
	if got.Item.Name != "First" || got.Number != 1 || got.Checklist != path {
		t.Fatalf("iteration = %#v", got)
	}
	if !strings.Contains(got.Feedback, `validation failed for "First": First needs work`) {
		t.Fatalf("feedback = %q", got.Feedback)
	}
	list, err := checklist.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if list.Items[0].Done || !list.Items[1].Done {
		t.Fatalf("done reconciliation = %#v", list.Items)
	}
}

func TestLoopRetriesFailureAndRereadsLedger(t *testing.T) {
	path := writeChecklist(t, testChecklist)
	passFirst := false
	var iterations []Iteration
	for iteration, err := range Loop(context.Background(), path, LoopOptions{
		Validate: func(_ context.Context, item checklist.Item) (Validation, error) {
			switch item.Name {
			case "First":
				return Validation{Passed: passFirst, Notes: "first proof missing"}, nil
			case "Second":
				return Validation{Passed: true}, nil
			case "Third":
				return Validation{Passed: false, Notes: "third proof missing"}, nil
			default:
				return Validation{}, errors.New("unexpected item")
			}
		},
	}) {
		if err != nil {
			t.Fatal(err)
		}
		iterations = append(iterations, iteration)
		if len(iterations) == 1 {
			passFirst = true
			// This is the Go body changing the ledger between laps. The next
			// selection must use a fresh load rather than the prior in-memory list.
			if err := os.WriteFile(path, []byte(`---
items:
  - name: First
    check: first works
  - name: Second
    check: second works
  - name: Third
    check: third works
---

# The ledger body
`), 0o600); err != nil {
				t.Fatal(err)
			}
			continue
		}
		break
	}
	if len(iterations) != 2 {
		t.Fatalf("iterations = %#v", iterations)
	}
	if iterations[0].Item.Name != "First" || iterations[0].Number != 1 {
		t.Fatalf("first iteration = %#v", iterations[0])
	}
	if iterations[1].Item.Name != "Third" || iterations[1].Number != 2 {
		t.Fatalf("second iteration = %#v", iterations[1])
	}
	if !strings.Contains(iterations[1].Feedback, `validation failed for "Third"`) {
		t.Fatalf("second feedback = %q", iterations[1].Feedback)
	}
}

func TestLoopEvaluatorFailureReopensAnItem(t *testing.T) {
	path := writeChecklist(t, testChecklist)
	var got Iteration
	for iteration, err := range Loop(context.Background(), path, LoopOptions{
		Validate: func(context.Context, checklist.Item) (Validation, error) { return Validation{Passed: true}, nil },
		Evaluate: func(context.Context, *checklist.Checklist) (Validation, error) {
			return Validation{Notes: "the documented goal is not met"}, nil
		},
	}) {
		if err != nil {
			t.Fatal(err)
		}
		got = iteration
		break
	}
	if got.Item.Name != "First" || !strings.Contains(got.Feedback, "goal evaluation failed: the documented goal is not met") {
		t.Fatalf("iteration = %#v", got)
	}
	list, err := checklist.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if list.Items[0].Done {
		t.Fatal("evaluator failure left representative item done")
	}
}

func TestLoopCancellationDoesNotValidate(t *testing.T) {
	path := writeChecklist(t, testChecklist)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	validated := false
	var got error
	for _, err := range Loop(ctx, path, LoopOptions{
		Validate: func(context.Context, checklist.Item) (Validation, error) {
			validated = true
			return Validation{}, nil
		},
	}) {
		got = err
	}
	if !errors.Is(got, context.Canceled) || validated {
		t.Fatalf("error = %v, validated = %t", got, validated)
	}
}

func TestLoopBreakDoesNoWorkAfterYield(t *testing.T) {
	path := writeChecklist(t, testChecklist)
	validations := 0
	evaluations := 0
	for _, err := range Loop(context.Background(), path, LoopOptions{
		Validate: func(context.Context, checklist.Item) (Validation, error) {
			validations++
			return Validation{}, nil
		},
		Evaluate: func(context.Context, *checklist.Checklist) (Validation, error) {
			evaluations++
			return Validation{}, nil
		},
	}) {
		if err != nil {
			t.Fatal(err)
		}
		break
	}
	// One lap validates both ledger entries before yielding its failed item.
	// Breaking that range must not start a second lap or run the evaluator.
	if validations != 2 || evaluations != 0 {
		t.Fatalf("validations = %d, evaluations = %d", validations, evaluations)
	}
}

func TestLoopMaxIterationsAllowsFinalValidation(t *testing.T) {
	path := writeChecklist(t, testChecklist)
	passing := false
	iterations := 0
	var got error
	for _, err := range Loop(context.Background(), path, LoopOptions{
		MaxIterations: 1,
		Validate: func(context.Context, checklist.Item) (Validation, error) {
			return Validation{Passed: passing}, nil
		},
	}) {
		if err != nil {
			got = err
			continue
		}
		iterations++
		passing = true // the only body lap repairs the work
	}
	if got != nil || iterations != 1 {
		t.Fatalf("error = %v, iterations = %d", got, iterations)
	}
}

func writeChecklist(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "checklist.md")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
