package gimble

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tylergannon/polytype"
)

// Task is one assignment selected by a PromiseLoop planner.
type Task struct {
	// Name is a short label for recognizing the work.
	Name string `json:"name"`
	// Description states the desired result and any necessary, non-obvious
	// information. It leaves the approach to the worker.
	Description string `json:"description"`
	// DefinitionOfDone says how to recognize successful completion of this
	// assignment. It does not declare that the enclosing goal is complete.
	DefinitionOfDone string `json:"definition_of_done"`
	// Validation describes evidence the workflow can gather. Either field may
	// be empty; the workflow still assesses the task against DefinitionOfDone.
	Validation struct {
		// Command is a known executable check.
		Command string `json:"command"`
		// Query is a question for a validator agent.
		Query string `json:"query"`
	} `json:"validation"`
}

// plan is the planner's whole answer for one dispatch: the revised backlog
// and the index of the next task, or null to end dispatch.
type plan struct {
	Tasks []Task                 `json:"tasks"`
	Next  polytype.Nullable[int] `json:"next"`
}

// answer is the planner's plan with its content checks attached to the
// schema check, so one Generate re-ask covers an inconsistent plan as well
// as a wrong-shaped one.
type answer struct{ plan }

func (a answer) ValidateJSON(raw []byte) error {
	if err := a.plan.ValidateJSON(raw); err != nil {
		return err
	}
	var p plan
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	return validatePlan(p)
}

type promiseLoop struct {
	ctx     context.Context
	name    string
	goal    string
	planner *Session
	err     error
}

// PromiseLoop opens planner-directed dispatch for goal. The planner keeps a
// revisable backlog and selects each task. Range over Tasks and check Err
// afterward to distinguish normal completion from planner, persistence, or
// cancellation failure.
//
// An operator can send this loop a message while it is dispatching. The
// message waits for the planner's next decision rather than being dropped
// between turns.
func PromiseLoop(ctx context.Context, name, goal string, planner *Session) *promiseLoop {
	return &promiseLoop{ctx: ctx, name: name, goal: goal, planner: planner}
}

// Tasks yields planner-selected assignments. Each ctx is a child scope that
// contains the structured task and ends when the loop body returns. Values the
// body records in that scope are shown to the planner before its next decision,
// alongside the values currently visible from the loop's parent scopes.
//
// The planner may revise, reorder, and extend the backlog as work reveals what
// matters. It ends dispatch by returning no task. That decision is distinct
// from validation and from fulfillment of the enclosing goal.
func (l *promiseLoop) Tasks(yield func(context.Context, Task) bool) {
	if l.planner == nil {
		l.err = errors.New("gimble: PromiseLoop requires a planner session")
		return
	}
	parent, err := current(l.ctx)
	if err != nil {
		l.err = err
		return
	}
	loopScope := parent.child(l.name)
	loopScope.loop, loopScope.dispatching = true, true
	l.err = loopScope.do(l.ctx, func(ctx context.Context) error {
		// A message an operator sent but the planner never read is
		// recorded as dropped, so every message has one record saying
		// whether it reached a decision.
		defer func() {
			for _, message := range loopScope.endDispatch() {
				loopScope.run.event(loopScope.key, "", "", Steer{Target: loopScope.key, Source: "person", Message: message})
				logf("%s: a message was dropped, dispatch ended first: %s", loopScope.key, oneLine(message))
			}
		}()
		dir := filepath.Join(loopScope.run.dir, "scopes", filepath.FromSlash(loopScope.key))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("gimble: %w", err)
		}
		file := filepath.Join(dir, "backlog.md")

		tasks := []Task{}
		var previous string
		for {
			backlogText, err := backlogJSON(l.goal, tasks)
			if err != nil {
				return fmt.Errorf("gimble: %w", err)
			}
			// What an operator sent while the last task ran is read here,
			// at the decision it was sent for, and recorded as landed
			// because the planner is about to see it.
			messages := loopScope.takeMessages()
			for _, message := range messages {
				loopScope.run.event(loopScope.key, "", "", Steer{Target: loopScope.key, Source: "person", Message: message, Landed: true})
				logf("%s: a message reached the planner: %s", loopScope.key, oneLine(message))
			}
			prompt, err := planPrompt(ctx, l.name, l.planner.workdir, string(backlogText), scopeText(ctx), previous, messages)
			if err != nil {
				return err
			}
			a, err := dispatch[answer](ctx, l.planner, prompt, nil)
			if err != nil {
				return err
			}
			p := a.plan
			tasks = p.Tasks
			if tasks == nil {
				tasks = []Task{}
			}

			revisedText, err := backlogJSON(l.goal, tasks)
			if err != nil {
				return fmt.Errorf("gimble: %w", err)
			}
			if err := os.WriteFile(file, []byte("---\n"+string(revisedText)+"\n---\n"), 0o644); err != nil {
				return fmt.Errorf("gimble: %w", err)
			}

			if !p.Next.Present {
				loopScope.run.event(loopScope.key, "", "", PlannerDecision{})
				logf("%s: the planner ended dispatch", loopScope.key)
				return nil
			}

			task := tasks[p.Next.Value]
			logf("%s: task: %s", loopScope.key, oneLine(task.Name))
			loopScope.run.event(loopScope.key, "", "", PlannerDecision{Task: optionalTask(task)})
			more := true
			taskScope := loopScope.child("task")
			taskCtx := context.WithValue(ctx, taskKey{}, task)
			if err := taskScope.do(taskCtx, func(ctx context.Context) error {
				raw, err := json.Marshal(task)
				if err != nil {
					return fmt.Errorf("gimble: encode task: %w", err)
				}
				store(ctx, "task", raw)
				more = yield(ctx, task)
				previous = taskScope.localText()
				// A task killed by an operator is the task scope's own
				// outcome, so ScopeEnded records it.
				if killed, ok := errors.AsType[Killed](context.Cause(ctx)); ok {
					return killed
				}
				return nil
			}); err != nil {
				// A kill of the task alone is a failed task, not a broken
				// loop: the planner sees the reason on the next lap. A kill
				// of the loop itself reaches here too, as the inherited
				// cause; that one ends dispatch, like any other error.
				var killed Killed
				if ctx.Err() != nil || !errors.As(err, &killed) {
					return err
				}
				logf("%s: task killed: %v", loopScope.key, killed)
				previous += "\n\n## task failed\n\n" + killed.Error()
			}
			if !more {
				return nil
			}
		}
	})
}

// Err returns the error that ended dispatch, if any: persistence, malformed
// planner data, cancellation, or the planner's harness. A failed task
// validation recorded by the workflow is feedback, not a PromiseLoop error.
func (l *promiseLoop) Err() error {
	return l.err
}

// backlogJSON renders goal and tasks as the JSON object written to
// backlog.md (inside its frontmatter) and shown to the planner in its
// prompt, so both channels always agree.
func backlogJSON(goal string, tasks []Task) ([]byte, error) {
	return json.MarshalIndent(struct {
		Goal  string `json:"goal"`
		Tasks []Task `json:"tasks"`
	}{Goal: goal, Tasks: tasks}, "", "  ")
}

// validatePlan checks a plan's content: every task must be well-formed, no
// two tasks may share a name, and a present Next must index into Tasks.
func validatePlan(p plan) error {
	seen := make(map[string]bool, len(p.Tasks))
	for i, task := range p.Tasks {
		if err := validateTask(task); err != nil {
			return fmt.Errorf("task %d: %w", i+1, err)
		}
		name := strings.TrimSpace(task.Name)
		if seen[name] {
			return fmt.Errorf("task %d: duplicate task name %q", i+1, task.Name)
		}
		seen[name] = true
	}
	if p.Next.Present && (p.Next.Value < 0 || p.Next.Value >= len(p.Tasks)) {
		return fmt.Errorf("next %d is out of range for %d tasks", p.Next.Value, len(p.Tasks))
	}
	return nil
}

func validateTask(task Task) error {
	switch {
	case strings.TrimSpace(task.Name) == "":
		return errors.New("name is blank")
	case strings.TrimSpace(task.Description) == "":
		return errors.New("description is blank")
	case strings.TrimSpace(task.DefinitionOfDone) == "":
		return errors.New("definition of done is blank")
	default:
		return nil
	}
}

// WrapUp is the message that tells a loop to stop taking on work. It is an
// ordinary message to a loop's planner, sent by an operator who can see
// that what is left is not worth another lap; the planner's prompt states
// what it means, and the planner still writes the decision.
const WrapUp = "Wrap this up: end dispatch at this decision."

func planPrompt(ctx context.Context, name, workdir, backlogText, scoped, previous string, messages []string) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "You plan the loop %q in %s. Its backlog is shown below.\n\n", name, workdir)
	b.WriteString("Choose the next assignment that offers the greatest concrete gain toward the goal, based on current evidence, priorities, and real dependencies. Size it for one worker to understand, complete, and demonstrate in one working session. A later task may offer more gain than repairing a nonblocking earlier defect; keep deferred defects visible.\n\n")
	b.WriteString("Treat recorded deterministic results as authoritative: a prose claim or agent judgment cannot override a nonzero command exit. If a check relevant to the goal or an assignment's Definition of Done failed and no later recorded run passed, work remains.\n\n")
	b.WriteString("Inspect the workspace only to plan; do not perform or validate an assignment yourself.\n\n")
	b.WriteString("Describe the desired result and necessary non-obvious facts. Trust the worker to choose the approach. Do not supply procedural checklists, obvious advice, speculative code, or a numerical progress score.\n\n")
	var dynamic strings.Builder
	if strings.TrimSpace(scoped) != "" {
		dynamic.WriteString("Scoped context:\n\n" + scoped + "\n\n")
	}
	if strings.TrimSpace(previous) != "" {
		dynamic.WriteString("Previous task record:\n\n" + previous + "\n\n")
	}
	dynamic.WriteString("Backlog now:\n\n" + backlogText + "\n\n")
	if len(messages) > 0 {
		dynamic.WriteString("The person watching this run sent you this, for this decision:\n\n- " + strings.Join(messages, "\n- ") + "\n\n")
		dynamic.WriteString("Weigh it as you would any other evidence, except an instruction to wrap up: that one means end dispatch at this decision, with whatever is in flight finished or dropped.\n\n")
	}
	dynamicText, err := budgetRenderedText(ctx, "planner-context", dynamic.String(), contextTokenLimit)
	if err != nil {
		return "", err
	}
	b.WriteString(dynamicText)
	b.WriteString("Return the full revised task list in `tasks` and the index of the chosen task in `next`, or `next: null` to end dispatch; that does not certify that the goal is fulfilled.")
	return b.String(), nil
}
