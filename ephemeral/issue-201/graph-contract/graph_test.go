package workflow

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func site(id string, line int) Site {
	return Site{ID: id, Source: Source{File: "internal/workflows/sprint/sprints.go", Line: line, Column: 2}}
}

func expr(text string) Expression { return Expression{Text: text} }

// sprintShape is the illustrative Sprint from graph-model.md with every
// Operation variant, three levels of nesting, an ancestor-owned supervisor
// reused inside a repeated task, and a nested supervisor watching the first
// supervisor's looks.
func sprintShape() Graph {
	return Graph{
		Name:      "sprint",
		Package:   "github.com/tylergannon/gimble/internal/workflows/sprint",
		Entry:     "Sprint",
		Input:     "Input",
		Source:    Source{File: "internal/workflows/sprint/sprints.go", Line: 40, Column: 1},
		Generator: "v0.0.0-handoff",
		Sessions: []Session{
			{Site: site("session.researcher", 50), Name: "researcher", Role: "researcher"},
			{Site: site("session.reviewer", 52), Name: "reviewer", Role: "reviewer"},
			{Site: site("session.lead", 53), Name: "lead", Role: "lead"},
			{Site: site("session.coder", 120), Name: "coder", OwnerScope: "loop.tasks", Role: "coder"},
		},
		Body: Sequence{
			Site: site("body", 40),
			Body: []Operation{
				SetJSON{Site: site("set.input", 55), Key: "input", Value: expr("in"), Type: "Input"},
				AgentCall{Site: site("call.research", 60), Session: "session.researcher", Prompt: expr("researchPrompt(in)"), Output: "gimble.Text"},
				Loop{
					Site:      site("loop.rounds", 70),
					Condition: expr("round < in.Rounds"),
					Body: []Operation{
						Scope{
							Site: site("scope.round", 72),
							Name: "round",
							Body: []Operation{
								Loop{
									Site:    site("loop.tasks", 80),
									Name:    "tasks",
									Planner: "session.researcher",
									Goal:    expr("goalText(in)"),
									Body: []Operation{
										AgentCall{Site: site("call.coder", 121), Session: "session.coder", Prompt: expr("task.Prompt"), Output: "gimble.Text"},
										Set{Site: site("set.worker", 130), Key: "worker result", Value: expr("string(result)")},
										Condition{
											Site: site("cond.validation", 140),
											Branches: []Branch{
												{Site: site("cond.validation.if", 140), Case: expr("task.Validation != \"\""), Body: []Operation{
													Command{Site: site("cmd.validation", 141), Arguments: []Expression{expr("ctx"), expr("\"validation\""), expr("task.Validation")}},
												}},
												{Site: site("cond.validation.else", 145), Body: []Operation{
													Exit{Site: site("exit.continue", 146), Statement: Continue},
												}},
											},
										},
										AgentCall{Site: site("call.validator", 150), Session: "session.researcher", Prompt: expr("validatePrompt(task)"), Output: "review"},
										SetJSON{Site: site("set.assessment", 155), Key: "assessment", Value: expr("assessment"), Type: "review"},
									},
									Supervision: []Supervision{{
										Target: "call.coder",
										Supervisors: []Supervisor{
											{Site: site("sup.taste", 121), Session: "session.reviewer", Instruction: expr("tasteInstruction"), Interval: expr("time.Minute"),
												Supervisors: []Supervisor{
													{Site: site("sup.lead", 121), Session: "session.lead", Instruction: expr("leadInstruction"), Interval: expr("5 * time.Minute")},
												}},
											{Site: site("sup.scope", 122), Session: "session.reviewer", Instruction: expr("scopeInstruction"), Interval: expr("time.Minute")},
										},
									}},
								},
							},
						},
					},
				},
				Group{
					Site: site("group.checks", 200),
					Name: "checks",
					Children: []GroupChild{
						{Site: site("group.checks.vet", 201), Name: "vet", Body: []Operation{
							Command{Site: site("cmd.vet", 202), Arguments: []Expression{expr("ctx"), expr("\"go\""), expr("\"vet\""), expr("\"./...\"")}},
							Set{Site: site("set.check1", 203), Key: "repository check 1", Value: expr("vetResult")},
						}},
						{Site: site("group.checks.test", 205), Name: "test", Body: []Operation{
							Command{Site: site("cmd.test", 206), Arguments: []Expression{expr("ctx"), expr("\"go\""), expr("\"test\""), expr("\"./...\"")}},
							Set{Site: site("set.check2", 207), Key: "repository check 2", Value: expr("testResult")},
						}},
					},
				},
				Sequence{Site: site("helper.summary", 220), Body: []Operation{
					AgentCall{Site: site("call.summary", 300), Session: "session.researcher", Prompt: expr("summaryPrompt()"), Output: "gimble.Text"},
					Exit{Site: site("exit.return", 301), Statement: Return},
				}},
			},
		},
		Supervision: []Supervision{{
			Target:      "call.research",
			Supervisors: []Supervisor{{Site: site("sup.research", 60), Session: "session.reviewer", Instruction: expr("researchInstruction"), Interval: expr("time.Minute")}},
		}},
	}
}

func TestRoundTrip(t *testing.T) {
	want := sprintShape()
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var got Graph
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("decode: %v\n%s", err, data)
	}
	// Nil and empty slices both encode as [] and decode as empty, so the
	// proof of the round trip is that the decoded graph encodes to the same
	// bytes and decodes to the same value again.
	again, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != string(data) {
		t.Fatalf("second encoding differs\n%s\n%s", data, again)
	}
	var twice Graph
	if err := json.Unmarshal(again, &twice); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, twice) {
		t.Fatalf("second decoding differs\n got: %#v\nwant: %#v", twice, got)
	}
	if len(data) < 4000 {
		t.Fatalf("fixture too small to exercise nesting: %d bytes", len(data))
	}
}

func TestEveryVariantHasAKind(t *testing.T) {
	all := []Operation{AgentCall{}, Command{}, Set{}, SetJSON{}, Exit{}, Sequence{}, Condition{}, Loop{}, Scope{}, Group{}}
	seen := map[string]bool{}
	for _, op := range all {
		kind, err := kindOf(op)
		if err != nil {
			t.Fatal(err)
		}
		if seen[kind] {
			t.Fatalf("duplicate kind %q", kind)
		}
		seen[kind] = true
		target, err := newOperation(kind)
		if err != nil {
			t.Fatal(err)
		}
		if reflect.TypeOf(deref(target)) != reflect.TypeOf(op) {
			t.Fatalf("kind %q decodes to %T, want %T", kind, deref(target), op)
		}
	}
	if !seen["set_json"] {
		t.Fatalf("SetJSON must snake to set_json as polytype.Snake does: %v", seen)
	}
}

func TestWireShape(t *testing.T) {
	data, err := json.Marshal(sprintShape())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`{"kind":"set_json","id":"set.input"`,
		`{"kind":"agent_call","id":"call.research"`,
		`{"kind":"loop","id":"loop.rounds"`,
		`{"kind":"scope","id":"scope.round"`,
		`{"kind":"condition","id":"cond.validation"`,
		`{"kind":"command","id":"cmd.validation"`,
		`{"kind":"exit","id":"exit.continue"`,
		`{"kind":"group","id":"group.checks"`,
		`{"kind":"sequence","id":"helper.summary"`,
		`"statement":"continue"`,
		`"target":"call.coder"`,
	} {
		if !strings.Contains(string(data), want) {
			t.Errorf("wire JSON lacks %s\n%s", want, data)
		}
	}
	// Nil slices encode as [], as polytype's generated encoder does.
	empty, err := json.Marshal(Graph{Body: Sequence{Body: []Operation{Group{}}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"sessions":[]`, `"diagnostics":[]`, `"supervision":[]`, `"children":[]`} {
		if !strings.Contains(string(empty), want) {
			t.Errorf("empty graph lacks %s\n%s", want, empty)
		}
	}
	if strings.Contains(string(empty), "null") {
		t.Errorf("empty graph contains null\n%s", empty)
	}
}

func TestDecodeRejects(t *testing.T) {
	cases := map[string]string{
		"unknown kind":   `{"body":{"id":"b","source":{"file":"","line":0,"column":0},"body":[{"kind":"launch","id":"x"}]}}`,
		"missing kind":   `{"body":{"id":"b","source":{"file":"","line":0,"column":0},"body":[{"id":"x"}]}}`,
		"foreign field":  `{"body":{"id":"b","source":{"file":"","line":0,"column":0},"body":[{"kind":"set","id":"x","source":{"file":"","line":0,"column":0},"key":"k","value":{"text":"","constant":false},"prompt":{"text":"","constant":false}}]}}`,
		"nested foreign": `{"body":{"id":"b","source":{"file":"","line":0,"column":0},"body":[{"kind":"scope","id":"s","source":{"file":"","line":0,"column":0},"name":"n","body":[{"kind":"exit","id":"e","source":{"file":"","line":0,"column":0},"statement":"return","name":"no"}],"supervision":[]}]}}`,
		"unknown root":   `{"digest":"abc"}`,
	}
	for name, input := range cases {
		var g Graph
		if err := json.Unmarshal([]byte(input), &g); err == nil {
			t.Errorf("%s: decoded without error", name)
		} else {
			t.Logf("%s: %v", name, err)
		}
	}
}

func TestSupervisorNesting(t *testing.T) {
	var g Graph
	data, _ := json.Marshal(sprintShape())
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	rounds := g.Body.Body[2].(Loop)
	round := rounds.Body[0].(Scope)
	tasks := round.Body[0].(Loop)
	if len(tasks.Supervision) != 1 || tasks.Supervision[0].Target != "call.coder" {
		t.Fatalf("task supervision lost: %#v", tasks.Supervision)
	}
	if g.Sessions[1].Role != "reviewer" || g.Sessions[1].OwnerScope != "" {
		t.Fatalf("reviewer role or root ownership lost: %#v", g.Sessions[1])
	}
	if g.Sessions[3].Role != "coder" || g.Sessions[3].OwnerScope != tasks.ID {
		t.Fatalf("coder role or task ownership lost: %#v", g.Sessions[3])
	}
	taste := tasks.Supervision[0].Supervisors[0]
	if taste.Session != "session.reviewer" || len(taste.Supervisors) != 1 || taste.Supervisors[0].Session != "session.lead" {
		t.Fatalf("nested supervisor lost: %#v", taste)
	}
	if g.Supervision[0].Supervisors[0].Session != taste.Session {
		t.Fatal("ancestor-owned reviewer should be the same session at both attachments")
	}
}
