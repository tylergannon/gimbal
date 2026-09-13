# Beta milestone: triage and proposed issues

Written 2026-09-13. Milestone: https://github.com/tylergannon/gimble/milestone/1

## The bar

Not stable, API not stable, but works and is useful: Gimble is used to build
Gimble. Concretely: (1) a coherent API plus docs and skills so an agent can
write any requested workflow shape as easily as any Go program, with loops
written and observed enough to be managed well; (2) tokens and wall time per
scope, with posted token prices as a comparable cost proxy; (3) a web UI that
looks decent and lets a person watch a run and steer any session; (4) proof:
one Gimble feature built by a Gimble workflow while Tyler watches and steers
from the page.

## In the milestone (20)

API coherence: #165 delete Session.Interrupt; #159 infallible Set/SetJSON;
#126 completion and join contracts.

Loops in practice: #168 sprint workflow (validator cannot reshape the goal,
plain local prompts, prompts visible); #120 supervision validated live; #110
a failed supervisor look reaches the log; #106 `just attest`, `vet`, `test`.

Tokens, time, cost: #157 usage by scope (placement, scope and turn times,
the tree); #155 reducer session info; #151 Codex cache write (PR #158).

Web UI: #130 snapshot then stream (largely delivered by PR #138; close or
trim to what is left); #135 resumed-turn and tool attribution; #147 task
shown twice; #117 rescope to the OpenCode event model (approval requests,
harness errors, retries visible on the page).

Steering: #107 steer reports landed or dropped; #113 Claude steer live.

Running many runs and agents: #118 project.jsonl seq; #111 same-second run
ids; #163 daemon descriptor limit; #112 vet in a fresh clone.

## Out of the milestone

- Gimble, later: #150 filesystem scope context (Tyler: not now; risk noted
  below); #105 Get/GetJSON/Each (undecided, see #150); #162 Set analyzer;
  #152 per-message cost; #154 attest program logs; #116 log-kind tests;
  #164 Codex ghost threads (daemon limitation); #97 review-process notes.
- Recommend closing as done or superseded: #108 (Close in #160), #20
  (daemon attach in #160), #13 (usage half in #156; quota half has no
  consumer), #11 (internal/modelalias exists).
- Tractor-era, before the 2026-09-10 restart, not Gimble: #10 #12 #28 #39
  #43 #48 #51 #52 #56 #58 #59 #65 #67 #69 #78 #88. Close, or label legacy.

## Proposed new issues, for discussion

1. Steer and cancel from the page. A steer box on every session card, a
   turn-running indicator so the person can see whether a steer will land,
   cancel on the run. The page's Go needs to reach a live session by run and
   session id, so the runtime keeps a registry of live runs and their
   sessions. Steer records `Source: "person"`. Depends on #107, #113.
2. Cost from posted prices. One per-model price table (input, cache read,
   cache write, output, reasoning, $/Mtok, dated) applied in one place
   during the fold. Unknown model shows "no price", never $0. Decide whether
   Claude's stated cost is replaced by the table for comparability. The
   proxy is for comparing models, not for quota; no quota arithmetic.
3. Duration cells. #157 declined a duration column. Tyler wants to
   interrogate where time went, so add a duration cell beside the six token
   cells at run, scope, and turn level, and say how a scope's wall time
   relates to its children when they overlap.
4. Runs index and the look of the page. `/` becomes the runs list (live
   and past, status, started, duration, cost), run to scope tree to
   transcript navigation, one consistent visual design in light and dark.
   Decide how much design investment.
5. A workflow-writing skill and shape examples. `.claude/skills` and
   `.agents/skills` gain a Gimble skill: the shapes (one turn, fork and
   bake-off, critique round, supervised worker, Loop with planner, worktree
   per candidate, validation command), how to run and watch, how to read
   the run log. Each shape is a compiling Example in the root package.
6. Loop practice runs. Three or four small loop workflows on the cheap tier
   with different shapes (planner ends, validation command, supervisor,
   Group inside a task), each with notes on what went wrong and what
   steering did. Output: loop-management notes that feed the skill.
7. Godoc as the tutorial. Settle #105 and Start, then make `go doc -all`
   self-sufficient: package doc walks Run, Session, Generate, Scope, Group,
   Loop, supervisors, with an Example for each. Could fold into 5.
8. The beta proof. Pick one milestone issue (small and well specified:
   #112, #111, #110) and build it with the sprint workflow on real models
   while Tyler watches and steers from the page. Record screenshots and
   the steers that landed under `ephemeral/attest/beta-proof/`. Exit
   criterion for the milestone. Depends on 1, #168, #157.

Not for beta: starting a run from the page (`gimble.Start`, the form);
quota windows.

## Risks

- Prompt growth: `ScopeText` reached 12 KB in #141's runs; a long dogfood
  run may need #150's index sooner than planned.
- The Codex daemon's 256-descriptor ceiling (#163) caps fan-out on macOS
  until raised.
- Tool attribution on the page (#135) is wrong in a way that will confuse a
  person deciding whether to steer.
