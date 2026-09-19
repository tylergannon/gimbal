# Graph and state examples for design

Read beside `run-interface-handoff.md`. These examples supply actual shapes
and realistic content without requiring a designer to reverse-engineer the
repository. Sources are pinned to `0908812`. Runtime examples are **invented
design specimens**, not captured runs, tests, or evidence of behavior.

## 1. A complete small workflow graph

This is a JSON transcription of the generated `interview.Graph` in
`internal/workflows/interview/workflow_gen.go`. Empty diagnostics are shown
as `[]`. Field names and operation discriminators follow the generated
TypeScript types in `web/src/lib/workflow/types.ts`.

```json
{
  "name": "interview",
  "source": {"file": "internal/workflows/interview/interview.go", "line": 21},
  "body": [
    {"kind": "set", "file": "internal/workflows/interview/interview.go", "line": 22, "key": "topic"},
    {"kind": "session", "file": "internal/workflows/interview/interview.go", "line": 23, "name": "interviewer", "from": ""},
    {"kind": "interview", "file": "internal/workflows/interview/interview.go", "line": 24, "name": "preferences", "session": "interviewer"},
    {"kind": "set", "file": "internal/workflows/interview/interview.go", "line": 28, "key": "preferences"}
  ],
  "diagnostics": []
}
```

The interview is one declared operation. It may generate several questions
and agent turns. Those do not require several copies of the static node.
The `topic` and `preferences` writes are context, not agent conversations.
The graph doesn't contain the topic value, interview purpose, model,
questions, answers, or completion time.

### Same interview, four moments

| Moment | What the person sees | Interaction |
| --- | --- | --- |
| Thinking | `preferences`, agent activity in `interviewer.1/turn.1` | Read activity; steer if available. No question yet. |
| Waiting | “What would make a weekend trip feel restful to you?” | Answer this question, or clearly choose to end the interview. |
| Thinking again | Prior Q/A remains; `interviewer.1/turn.2` is active | Read history or steer without leaving the interview's location. |
| Complete | Returned exchanges and final run outcome | Review, not another active answer form. |

Invented answer: “A quiet mountain cabin, easy walks, and no crowds.”
Invented next question: “Which matters most: scenery, cabin amenities, or
nearby food?” Use substantive text like this rather than lorem ipsum.

## 2. A larger real workflow shape

Below is a **readable projection**, not a wire format or a proposed layout,
of the generated `implement-interview` graph. It omits guard conditions,
source positions, and full prompt strings for readability. The exact graph
is in `internal/workflows/implementinterview/workflow_gen.go` at `0908812`.

```text
implement-interview
  context: requirements-file, reference-directory, repository
  group reconnaissance — children concurrent
    backend
      create session api-research
      call api-research: inspect backend APIs; write local reference notes
    frontend
      create session frontend-research
      call frontend-research: inspect UI APIs; write local reference notes
  create sessions sprint-planning, coding,
                  implementation-scope-review, architectural-critique
  PromiseLoop implementation — planner: sprint-planning
    task body — repeated for each planner assignment
      call coding: implement the selected task
        watched by implementation-scope-review
        watched by architectural-critique
      if the assignment supplies a validation command
        command task-check; record its result
      command build; record its result
      command vet; record its result
      command test; record its result
      create session qa-orchestration
      call qa-orchestration: independently verify the implementation
      context: independent assessment, worker report,
               deterministic checks passed
      exit loop if completion demonstrated, or task bound reached
```

Potential views of this same information include a spatial map, nested
execution lanes, or a compact outline with a linked map. Do not treat this
text indentation as a mandated interface.

### Why instance selection matters

These are illustrative runtime identities compatible with that shape:

| Object | Identity | Meaning |
| --- | --- | --- |
| Backend scope | `reconnaissance.1/backend.1` | One concurrent branch. |
| Frontend scope | `reconnaissance.1/frontend.1` | Its sibling, not necessarily the next step. |
| Coding session | `coding.1` | Conversation created at the root and reused. |
| First assignment | `implementation.1/task.1` | Scope where coding turn 1 executes. |
| Second assignment | `implementation.1/task.2` | Scope where coding turn 2 executes. |
| Coding turn 2 | `coding.1/turn.2` | Session identity alone does not tell its execution scope. |
| Second validator | `implementation.1/task.2/qa-orchestration.1` | Fresh conversation owned by task 2. |
| Second build | `implementation.1/task.2/build.1` | Distinct from task 1's build. |

For a design exploration, imagine task 1 fails its command check, the
planner assigns a correction, and task 2 passes. A static call can then
have multiple runtime outcomes. Do not paint the entire call permanently
failed, or hide the earlier failure because the latest attempt passed.

The scope's status `ended` means it ended, not necessarily that its task
met the definition of done. Use the recorded outcomes, not a green badge
inferred from termination alone.

## 3. Concurrent interview placement

These are **partial observation rows** for an invented workflow with two
interviews, not the graph of example 1 or a full API response. Both questions
have the name `preferences`. Their scope, session, and question identity
separate them. Question IDs below are illustrative opaque identifiers.

```json
{
  "run": {"id": "design-example", "name": "plan-trip", "status": "running", "error": "", "started": 1800000000000, "ended": 0},
  "interviews": {
    "01M2RWXNFFYQA4HHQWMQ252CYF": {
      "run": "design-example", "question_id": "01M2RWXNFFYQA4HHQWMQ252CYF",
      "name": "preferences", "scope": "research.1/lodging.1",
      "session": "research.1/lodging.1/interviewer.1",
      "question": "Would you trade reliable Wi-Fi for a more secluded cabin?",
      "status": "pending", "answer": "", "asked": 1800000030000, "answered": 0
    },
    "01M2RWXNFFYQA4HHQWMQ252CYG": {
      "run": "design-example", "question_id": "01M2RWXNFFYQA4HHQWMQ252CYG",
      "name": "preferences", "scope": "research.1/transport.1",
      "session": "research.1/transport.1/interviewer.1",
      "question": "What is the longest drive you would enjoy?",
      "status": "pending", "answer": "", "asked": 1800000035000, "answered": 0
    }
  }
}
```

The question event carries the interview name and question ID; its lifecycle
envelope supplies scope and session. Answer delivery uses the run plus
question ID. An attention summary may lead to the question, but the answer
must remain visibly associated with the correct interview, not a generic
global chat box. A question can become stale while the person is viewing it.

## 4. Content and states to use across concepts

These are design situations, not additional backend features to implement.

| Situation | Useful content | Distinction to preserve |
| --- | --- | --- |
| Runs list | Two active runs, one awaiting an answer; a completed interview; a failed development run; a cancelled run | “Needs answer” can be derived from pending questions, not a new terminal run status. |
| Agent detail | Role, model, full prompt, thinking/text, a tool call/result, final output/error | Conversation versus one turn; live deltas versus final outcome. |
| Supervisor | “No objections” on two looks; a later “Keep the change scoped to the requested API” steer | Monitoring, objection, and landed delivery are different facts; none is formal approval. |
| Commands | `go test ./...`, exit code 1, a useful failure excerpt | Command failure may lead to another attempt rather than immediate run failure. |
| Connection loss | Previously running turn, last known data remains visible | Unknown current state, not invented success/failure. |
| Missing graph | Historical run still has scopes, turns, commands, and exchanges | Execution review remains possible without a source map. |
| Scale | 30 tasks, reused coding session, several turns per task | Overview should not require hundreds of full transcript cards. |

There is no prescribed color, node geometry, panel arrangement, or graph
library in these examples. Compare concepts using the same content.
