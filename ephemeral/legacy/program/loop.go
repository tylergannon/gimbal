// Package program provides small Go-native building blocks for authoring
// Gimbal programs.
package program

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"strings"

	"github.com/tylergannon/gimbal/checklist"
)

// LoopOptions supplies the program-specific work around a checklist loop.
// Validate is required: the iterator, rather than its caller, owns reconciling
// every item's done field with its validation result. Evaluate is an optional
// checklist-level definition-of-done gate. A non-positive MaxIterations does
// not impose a limit.
type LoopOptions struct {
	Validate      func(context.Context, checklist.Item) (Validation, error)
	Evaluate      func(context.Context, *checklist.Checklist) (Validation, error)
	MaxIterations int
}

// Iteration is one item offered to a program's loop body. Number is the
// one-based number of times this iterator has yielded. Checklist is the path
// to the ledger re-read to select this item.
type Iteration struct {
	Item      checklist.Item
	Number    int
	Feedback  string
	Checklist string
}

// Loop iterates the open work in path. Before every yielded item it reloads
// the ledger, validates every item (including hand-marked done items), and
// reconciles done to those results. The caller performs the work for the
// yielded item; its changes become visible on the next iteration.
//
// If every item validates, Evaluate decides whether the checklist is complete.
// A failing evaluation re-opens the first item and yields it with the
// evaluator's feedback, so a Go-authored body has a concrete opportunity to
// address the unmet checklist-level goal.
func Loop(ctx context.Context, path string, opts LoopOptions) iter.Seq2[Iteration, error] {
	return func(yield func(Iteration, error) bool) {
		if opts.Validate == nil {
			yield(Iteration{}, errors.New("program loop: Validate is required"))
			return
		}

		for number := 1; ; number++ {
			if !yieldContextError(ctx, yield) {
				return
			}

			list, err := checklist.Load(path)
			if err != nil {
				yield(Iteration{}, err)
				return
			}

			feedback, err := reconcile(ctx, path, list, opts.Validate)
			if err != nil {
				yield(Iteration{}, err)
				return
			}

			// Re-read after the reconciliation writes. Besides preventing stale
			// selection, this lets a body edit the ledger between its yield and
			// the next lap.
			list, err = checklist.Load(path)
			if err != nil {
				yield(Iteration{}, err)
				return
			}
			item, _, open := list.Open()
			if !open {
				if opts.Evaluate == nil {
					if !yieldContextError(ctx, yield) {
						return
					}
					return
				}
				if !yieldContextError(ctx, yield) {
					return
				}
				verdict, evaluateErr := opts.Evaluate(ctx, list)
				if evaluateErr != nil {
					yield(Iteration{}, evaluateErr)
					return
				}
				if !yieldContextError(ctx, yield) {
					return
				}
				if verdict.Passed {
					return
				}
				if len(list.Items) == 0 {
					yield(Iteration{}, errors.New("program loop: evaluator failed an empty checklist"))
					return
				}

				// A checklist-level failure needs a body lap. Its first item is the
				// stable, deterministic representative for that work.
				item = list.Items[0]
				if err := checklist.UnmarkDone(path, item.Name); err != nil {
					yield(Iteration{}, err)
					return
				}
				list, err = checklist.Load(path)
				if err != nil {
					yield(Iteration{}, err)
					return
				}
				item, _, _ = list.Find(item.Name)
				feedback = appendFeedback(feedback, "goal evaluation failed", verdict.Notes)
			}
			if opts.MaxIterations > 0 && number > opts.MaxIterations {
				yield(Iteration{}, fmt.Errorf("program loop: reached max iterations (%d)", opts.MaxIterations))
				return
			}

			iteration := Iteration{Item: item, Number: number, Feedback: feedback, Checklist: path}
			if !yield(iteration, nil) {
				return
			}
		}
	}
}

func reconcile(ctx context.Context, path string, list *checklist.Checklist, validate func(context.Context, checklist.Item) (Validation, error)) (string, error) {
	var feedback string
	for _, item := range list.Items {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		result, err := validate(ctx, item)
		if err != nil {
			return "", fmt.Errorf("validate checklist item %q: %w", item.Name, err)
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if result.Passed {
			err = checklist.MarkDone(path, item.Name)
		} else {
			err = checklist.UnmarkDone(path, item.Name)
			feedback = appendFeedback(feedback, fmt.Sprintf("validation failed for %q", item.Name), result.Notes)
		}
		if err != nil {
			return "", err
		}
	}
	return feedback, nil
}

func appendFeedback(feedback, prefix, notes string) string {
	message := prefix
	if notes = strings.TrimSpace(notes); notes != "" {
		message += ": " + notes
	}
	if feedback == "" {
		return message
	}
	return feedback + "\n" + message
}

// yieldContextError reports cancellation without beginning another unit of
// work. Its boolean mirrors yield so callers preserve range break semantics.
func yieldContextError(ctx context.Context, yield func(Iteration, error) bool) bool {
	if err := ctx.Err(); err != nil {
		yield(Iteration{}, err)
		return false
	}
	return true
}
