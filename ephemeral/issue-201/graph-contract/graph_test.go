package workflow

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// This test is the contract's proof. It builds the graph of
// internal/workflows/sprint/sprints.go by hand, checks that reading it back
// gives the same value, and checks that what a reader sees in it is the
// sprint: a researcher, rounds, planner-chosen tasks with a supervised coder,
// checks, a commit, a validator, and a merge.

const sprintFile = "internal/workflows/sprint/sprints.go"

func at(line int) Source { return Source{File: sprintFile, Line: line} }

// The prompt constants, as sprints.go declares them. A prompt is one of the
// three things the graph is for, so the graph carries the composed text with
// its constants filled in and every run-time part left as a {{ hole }}.
const (
	researchPrompt = `You are about to lead the build of the goal below on this repository, Gimble, a Go library. Read AGENTS.md, docs/definition-of-done.md, ephemeral/research/api/API.md, ephemeral/research/api/SPRINTS.md, and the code the goal touches, until you know where everything it needs is. Change no files. Answer with a short summary of what exists and what the goal needs.`

	codePrompt = `Complete the task in the scoped context, following AGENTS.md and ephemeral/research/api/API.md. Demonstrate the result and leave the work uncommitted. Answer with a short summary of what changed and the evidence you gathered.`

	superviseInstruction = "Don't let it build what its task does not ask for, over-engineer what it does build, or break a rule in AGENTS.md. Object to nothing else: code quality and style are not yours to judge."

	taskValidationPrompt = `Assess the task using the recorded result and evidence.

Definition of done: {{ task.DefinitionOfDone }}

List what that evidence does not show working at the repository's 90-95% readiness standard, and nothing else; an empty list passes the task. A passing agent judgment cannot override a failed deterministic check.`

	validatePrompt = `Check this repository as the Validation section below says, changing no files and committing nothing, and report what you did not see working of the goal below.`

	mergePrompt = `The validator saw the goal working, so finish as docs/definition-of-done.md says. File each quirk and bug left as a GitHub issue with gh issue create, skipping any that gh issue list already has. Then push this branch, open a pull request for it with gh pr create that says what was built, how it was seen working, and which issues it left, and merge it with gh pr merge --squash. Answer with the pull request's URL and the issues you filed.`
)

// sprintGraph is sprints.go as this contract describes it.
//
// Two judgments are written into it and are called out in the report:
//
//   - Sprint, the entry function, is not a branch in the body. Its if/else
//     picks the harnesses and calls the same run either way, and a harness is
//     a parameter of a run. The graph anchors at Sprint and its body is run's.
//   - The plain Go `for round` and `for _, check := range checks` loops are
//     not nodes. The runtime numbers repeats already: the second round is
//     round.2. A graph node for "this happens more than once" would say
//     nothing the run does not say better.
var sprintGraph = Graph{
	Name:   "sprint",
	Source: at(58),
	Body: []Operation{
		Set{Source: at(69), Key: "input"},
		Session{Source: at(80), Name: "researcher"},
		AgentCall{
			Source:  at(81),
			Session: "researcher",
			Prompt:  researchPrompt + "\n\n{{ goal }}",
			Output:  "gimble.Text",
		},
		Session{Source: at(84), Name: "planner", From: "researcher"},
		Session{Source: at(88), Name: "validator"},
		Scope{
			Source: at(96),
			Name:   "round",
			Body: []Operation{
				Condition{
					Source: at(97),
					Branches: []Branch{{
						Source: at(97),
						Case:   "len(findings) > 0",
						Body: []Operation{
							Set{Source: at(98), Key: "what the validator did not see working"},
						},
					}},
				},
				Loop{
					Source:  at(100),
					Name:    "sprint",
					Planner: "planner",
					Goal:    "{{ goal }}",
					Body:    taskBody,
				},
			},
		},
		AgentCall{
			Source:  at(117),
			Session: "validator",
			Prompt:  validatePrompt + "\n\n## Goal\n\n{{ withoutProof(text) }}\n\n{{ validation }}",
			Output:  "review",
		},
		AgentCall{
			Source:  at(133),
			Session: "planner",
			Prompt:  mergePrompt,
			Output:  "gimble.Text",
		},
	},
	Diagnostics: []Diagnostic{{
		Source:  at(257),
		Message: `Set key is built at run time from fmt.Sprintf("repository check %d", i+1); the graph records the shape of the key, not the keys`,
	}},
}

// taskBody is runTask, inlined at the Loop that dispatches it. runTask is
// reached through an if-init statement at line 105, which is where a walker
// that only looks at statement shapes loses it.
var taskBody = []Operation{
	Session{Source: at(212), Name: "coder", From: "researcher"},
	Session{Source: at(216), Name: "supervisor"},
	AgentCall{
		Source:  at(217),
		Session: "coder",
		Prompt:  codePrompt + "\n\n{{ scope }}",
		Output:  "gimble.Text",
		Supervisors: []Supervisor{{
			Source:      at(218),
			Session:     "supervisor",
			Instruction: superviseInstruction,
		}},
	},
	Set{Source: at(223), Key: "worker result"},
	Condition{
		Source: at(225),
		Branches: []Branch{{
			Source: at(225),
			Case:   "workErr != nil",
			Body:   []Operation{Set{Source: at(226), Key: "worker error"}},
		}},
	},
	Condition{
		Source: at(230),
		Branches: []Branch{{
			Source: at(230),
			Case:   `strings.TrimSpace(task.Validation.Command) != ""`,
			Body: []Operation{
				Command{Source: at(231), Text: "{{ task.Validation.Command }}"},
				Set{Source: at(235), Key: "task command"},
			},
		}},
	},
	AgentCall{
		Source:  at(244),
		Session: "validator",
		Prompt:  taskValidationPrompt + "\n\n{{ additional validation question }}\n\n{{ scope }}",
		Output:  "review",
	},
	Set{Source: at(248), Key: "task assessment"},
	Command{Source: at(253), Text: "{{ check }}"},
	Set{Source: at(257), Key: "repository check {{ i+1 }}"},
	Command{Source: at(266), Text: "git add -A"},
	Command{Source: at(269), Text: "git status --porcelain"},
	Command{Source: at(277), Text: "git commit -m {{ message }}"},
}

// decoded is sprintGraph after one encode/decode cycle. The encoder writes a
// nil list as [], so a decoded graph has an empty Supervisors where the
// fixture left it nil. That is the only difference, and the tests below work
// from the decoded graph so they prove what a viewer would actually read.
func decoded(t *testing.T) (Graph, []byte) {
	t.Helper()
	data, err := json.Marshal(sprintGraph)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var g Graph
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return g, data
}

// TestSprintGraphRoundTrip proves encoding is canonical and lossless: the
// decoded graph re-encodes to the same bytes, and decoding those bytes again
// gives the same value.
func TestSprintGraphRoundTrip(t *testing.T) {
	first, data := decoded(t)
	again, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("remarshal: %v", err)
	}
	if string(again) != string(data) {
		t.Errorf("re-encoding changed the bytes\n got: %s\nwant: %s", again, data)
	}
	var second Graph
	if err := json.Unmarshal(again, &second); err != nil {
		t.Fatalf("second unmarshal: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Errorf("decoding is not stable\n got: %+v\nwant: %+v", second, first)
	}
}

// TestNilBodyIsRejected proves a body is never silently dropped: the encoder
// refuses a nil ordered body rather than writing null.
func TestNilBodyIsRejected(t *testing.T) {
	_, err := json.Marshal(Graph{Name: "empty", Source: at(1)})
	if err == nil {
		t.Fatal("encoded a graph with a nil body without an error")
	}
}

// TestOperationsAreTagged proves the wire shape: every operation is an object
// whose "kind" is the snake_case type name.
func TestOperationsAreTagged(t *testing.T) {
	_, data := decoded(t)
	for _, kind := range []string{
		`"kind":"set"`, `"kind":"session"`, `"kind":"agent_call"`,
		`"kind":"scope"`, `"kind":"loop"`, `"kind":"condition"`, `"kind":"command"`,
	} {
		if !strings.Contains(string(data), kind) {
			t.Errorf("no operation encoded with %s", kind)
		}
	}
}

// TestUnknownKindIsRejected proves an operation object is decoded strictly,
// so a graph written by a newer extractor fails loudly instead of silently
// losing a step.
func TestUnknownKindIsRejected(t *testing.T) {
	var g Graph
	err := json.Unmarshal([]byte(`{"name":"x","source":{"file":"a.go","line":1},"body":[{"kind":"teleport"}],"diagnostics":[]}`), &g)
	if err == nil {
		t.Fatal("decoded an unknown operation kind without an error")
	}
	if !strings.Contains(err.Error(), "teleport") {
		t.Errorf("error does not name the unknown kind: %v", err)
	}
}

// TestGraphIsTheSprint is the readable check: what a viewer would show.
func TestGraphIsTheSprint(t *testing.T) {
	sessions := map[string]string{}
	calls := map[string]string{}
	var commands []string
	var supervised []string

	var walk func([]Operation)
	walk = func(body []Operation) {
		for _, op := range body {
			switch op := op.(type) {
			case Session:
				sessions[op.Name] = op.From
			case AgentCall:
				calls[op.Session] = op.Prompt
				for _, s := range op.Supervisors {
					supervised = append(supervised, op.Session+" watched by "+s.Session)
				}
			case Command:
				commands = append(commands, op.Text)
			case Scope:
				walk(op.Body)
			case Loop:
				walk(op.Body)
			case Condition:
				for _, b := range op.Branches {
					walk(b.Body)
				}
			case Group:
				for _, c := range op.Children {
					walk(c.Body)
				}
			}
		}
	}
	g, _ := decoded(t)
	walk(g.Body)

	for name, from := range map[string]string{
		"researcher": "", "planner": "researcher", "validator": "",
		"coder": "researcher", "supervisor": "",
	} {
		got, ok := sessions[name]
		if !ok {
			t.Errorf("no session named %q", name)
			continue
		}
		if got != from {
			t.Errorf("session %q forks %q, want %q", name, got, from)
		}
	}
	for _, name := range []string{"researcher", "coder", "validator", "planner"} {
		if calls[name] == "" {
			t.Errorf("session %q never speaks", name)
		}
	}
	if len(supervised) != 1 || supervised[0] != "coder watched by supervisor" {
		t.Errorf("supervision is %v, want the coder watched by the supervisor", supervised)
	}
	if len(commands) != 5 {
		t.Errorf("got %d commands %v, want the task command, the checks, and the three git steps", len(commands), commands)
	}
	if !strings.Contains(calls["planner"], "gh pr merge") {
		t.Error("the planner does not merge")
	}
}
