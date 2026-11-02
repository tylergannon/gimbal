# Adversarial review: native-control spec, plan, socket audit (round 04)

## Target

Current `ephemeral/native-control/{spec.md,plan.md,socket-audit.md}` and the
worklog, against `AGENTS.md` and the restated Tyler decisions (audit Desktop
control surfaces first, favor an existing interface, stop early, Gimbal-initiated
native work while Locked Use stays on, build only what was asked). Full
re-review.

The launch prompt did not narrow scope, predict findings, or request a verdict.
Nothing was ignored.

## Round-03 disposition

- Bootstrap addressing and host id (#1): addressed. The spec now says the
  `remote-control:` label is the controlling client's address, requires the first
  check to establish the runtime owner, and admits the worker only if its default
  `local` host is Amrita's runtime. The audit has a row for the follow-up route.
  Resolved as a test requirement; see finding 2 for what that row still lacks.
- `luna` alias (#2): resolved in design. `internal/modelalias.Resolve` exists
  (`modelalias.go:130`) and the generated data carries `luna` at 5.6 and 6
  (`aliases_gen.go:18-19`), so the caller gets an explicit native slug, and the
  spec requires comparing it with Desktop's advertised choices first.
- Second candidate, lock procedure, admission load (#3–#5): addressed with an
  ordered candidate list, an observation-only lock rule, and an explicit note
  that inconclusive is common.

## Evidence inspected

- All four documents in full. `plugins/codex-desktop/` is unchanged from HEAD.
- `internal/modelalias/{modelalias.go,aliases_gen.go}`.
- Cache files in `/tmp/gimbal-native-control-audit-20261101/`, in particular
  every occurrence of `config/mcpServer/reload` in `main-0FVbxCQU.js`.
- No remote call this round. Not re-verified: the "read-only snapshot" of the
  inspector chat (see finding 3), the audit's offsets, and remote process state.

## Findings

### 1. [issue] The audit's plugin-reload evidence is for a different plugin path

`socket-audit.md` (row "Plugin discovery/reload on resumed tasks") says the
"installed main's plugin-change handler calls `config/mcpServer/reload`". In
the cached `main` bundle that string occurs exactly once (~2347946), inside the
internal-plugins sync. That code resolves the bundled marketplace, throws
`Internal plugins registered an unexpected marketplace.` unless the name is
`openai-internal-testing`, and sends the reload only when that marketplace was
just added or an internal plugin was uninstalled. It is not a general handler
for a user-added marketplace such as the scoped Gimbal worker. The row does
hedge ("does not prove CLI install triggers Desktop's handler"), but it cites
the wrong code as the reason to expect reload.

On the evidence in the cache, nothing in Desktop's main process triggers a
reload for the worker's marketplace. The spec makes discovery a live gate and
bars the only obvious remedies (Desktop restart, app-server replacement, fresh
owner), so the likeliest outcome of the worker candidate is a stop at its first
step. Better to say that now than to let the approval read as if reload were
expected to work. Rewrite the row to say the only traced reload is
internal-marketplace-only, and say in the spec what "normal refresh" is
permitted to mean (for example, whether a user-initiated Settings refresh is
allowed or counts as a Desktop change).

### 2. [issue] Candidate 1 is not defined well enough to run or to judge

The spec orders candidates: (1) "the existing remote task-control interface …
directly by Gimbal"; (2) the worker. The audit row for it says the surface is
`read_thread`/`send_message_to_thread` as used in the Sol chat's own tool set,
and that "Gimbal access to this surface is also unproved". Open points that
decide whether the candidate can be tried at all:

- Which process holds that interface, and where does the Gimbal controller run
  relative to it? If it is the controlling client's Desktop on another
  machine, a Gimbal run reaches Amrita only through that second client. The
  spec does not say whether that satisfies "one running Desktop application" and
  "Gimbal-originated", or whether native ownership then comes from Amrita's
  Desktop or the controlling one.
- The advertised follow-up tools (`send_message_to_thread`, `handoff_thread`,
  `fork_thread`) act on existing tasks. C6 needs a fresh child task. The spec
  does not say how candidate 1 creates one, or whether `create_thread` is
  exposed on that host (the local default list is create, send, handoff, fork,
  while a remote configuration builds a different list in `buildMcpCodexConfig`).
- No pass or fail check for "adequate" is stated, so candidate 1 can be waved
  through or dropped by the implementer's judgment, and the first 20 minutes
  are spent on an undefined step.

State what makes candidate 1 adequate in one line (create, read, stop, native
ownership) and where it runs, or drop it and test the worker only.

### 3. [nitpick] Audit certification leans on an observation nobody can replay

The new audit row relies on "this chat successfully obtained a read-only
metadata snapshot of the Amrita inspector through the app's existing
`read_thread` interface" and "an unloaded remote-control-host task". No file
in the work records the snapshot, and the spec's own rule (information lands
locally, proof is what was seen) suggests citing it where a reviewer can read
it. I could not verify it. Either store the snapshot text under
`ephemeral/native-control/` as a note, or label the row as an unreviewed claim.

### 4. [nitpick] Budget is ambiguous across two candidates

"Each phase has its own hard limit … Exhausting a phase ends that candidate.
All candidates together remain under 60 minutes." If each candidate gets its
own 20-minute admission, two candidates use 40 minutes before any native
action, leaving 20 for two native trials. The text does not say whether the
three 20-minute caps are per candidate or shared. Say which.

### 5. [nitpick] Plan and spec disagree on who picks the route

`plan.md` step 3 has Terra "select the smallest justified route from the
audit", while the spec fixes the order (existing interface, then worker) and
authorizes no third route. Make the plan point to the spec's order.

## Outcome

material findings remain
