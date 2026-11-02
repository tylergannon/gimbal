# Gimbal native Mac control: proposed contract and feasibility gate

**Draft for Tyler's explicit approval. Not approved for installation,
implementation or runtime experiments.** November 1, 2026.

## Overview and goals

Recover repeatable native Mac computer use initiated by Gimbal on
Amritanandamayi while configured Locked Use remains enabled, without manual
unlock. Normal authorized temporary unlock and automatic relock are expected.
One running Desktop application and one explicitly owned worker are acceptable;
Tyler prefers sharing that application instance. Desktop independence is not
required. Private integration is acceptable as a candidate, not presumed viable.

De-risk availability before engineering an integration. Audit existing Desktop
control/socket settings first; favor an existing adequate interface over a new
bridge. Stop early on an unavailable prerequisite, with a scoped explanation.
Successful feasibility authorizes no unreviewed architecture expansion.

Approval requested here covers the bounded feasibility experiment and its scoped
provisioning, not a speculative integration build. The software-cell proposal
below remains conditional. After a passing gate, name the demonstrated route,
complete any underivable seam decisions and obtain approval of that concrete
integration amendment before implementing product changes. No extra approval is
needed for routine read-only audit work or draft/review notes.

## Authority and established evidence

The user decisions above and repository AGENTS.md are authoritative. Historical
success, current source evidence and uncertainty are in:

- `/Users/tyler/.codex/worktrees/release-qualification-design/gimbal-view/NATIVE-COMPUTER-USE.md`
- its `ephemeral/release-qualification/native-control-{history,source,external}.md`
- this directory's `socket-audit.md` and `plan.md`.

Earlier Desktop-owned work succeeded; standalone Gimbal Finder observation
failed. Static helper analysis found a responsible-application identity gate.
The exact failed sender and runtime gate evaluation were not captured. Do not
claim a proven root cause. An actual route-specific discriminating explanation
is sufficient if the closed helper prevents complete causal attribution.

## Cells, seams and budgets

This is a bounded recovery of existing integration, not a new public Gimbal API.
No exported Go concept is proposed. Code is conditional on feasibility success.

| Cell | Responsibility / boundary | Contract | Maximum code volume |
| --- | --- | --- | --- |
| Desktop worker | Existing `plugins/codex-desktop/` runtime and scoped registration; changes with Desktop/private bridge | Genuine owner binds once; local user-only socket; idempotent submission with stable request identity; read/cancel only owned tasks; unknown outcome is retained; no fabricated trust | 500 physical nonblank lines across server/lib/client runtime files; tests excluded. Current physical total including blanks is 381 |
| Gimbal caller | Ordinary workflow in designated Gimbal task worktree; changes with workflow/task policy | Initiates the native task through the selected real interface, retains ownership and outcome, observes boundedly, stops/cancels owned work; does not reimplement private bridge internals | 150 nonblank runtime lines, tests excluded; no committed proof-only program |

The audit and operating recipe are documentation, not additional software cells.
If the existing worker is unnecessary, delete this prospective worker seam from
the contract through an approved amendment before engineering another interface.
Do not build a HarnessAdapter to fill features the current worker doesn't offer.
Any code-budget breach or new public concept is a spec defect, not permission to
invent another layer. Final file placement must be approved if it changes this
decomposition. A purely operational solution is a valid preferred outcome.

Proposed file ownership, if that existing route passes: the worker remains in
`plugins/codex-desktop/{server,lib,client}.mjs`; package tests remain beside it.
The caller is ordinary Go in an existing product workflow that needs native work,
with its actual owning file selected in the post-gate amendment. There is no new
generic workflow package. If the gate needs a temporary invocation, keep it outside
git; it does not become maintained proof code. Exceeding either budget means stop
or an operational-only answer; it does not authorize raising the cap.

External seam: actual Desktop-owned task creation, observation, cancellation and
native Computer Use authorization. This seam must be demonstrated live; protocol
tests cannot establish Desktop acceptance or Locked Use.

## Central semantics

    RECORD Attempt:
        requestIdentity : stable identity for this logical submission
        owner           : genuine loaded Desktop task, if worker route is used
        task            : known task identity, or unknown
        outcome         : pending | succeeded | failed | outcome_unknown

    audit available configuration and interfaces
    independently review audit coverage
    obtain Tyler's explicit approval of this spec and definition of done
    admit the scoped existing Desktop worker or stop
    verify actual task admission and required native capability
    if a prerequisite is unavailable or time budget expires:
        reconcile any owned task and retain a bounded negative/inconclusive finding
        stop; do not build around an unproven route
    otherwise:
        demonstrate native action with protected unlock/relock
        repeat in a fresh task
        finish only necessary integration and document operating/recovery recipe

Authentic context must come from the installed host. Socket presence, correct
IDs, a completed workflow, inventory, and screenshots obtained outside the
Gimbal-initiated route do not satisfy the target behavior.

## Feasibility and resource limits

After approval, maximum 60 minutes of active experiment work before a go/no-go
report; a tool wait counts. Split into at most 20 minutes to establish route
admission, 20 minutes for one native interaction, 20 minutes for fresh-task
repetition and reconciliation. One candidate route: the existing Desktop worker.
One retry only when new evidence explains a changed condition. No unchanged
failure loop. An unmet prerequisite ends its candidate early; ambiguous evidence
is inconclusive, not a proof that all routes are impossible.

Each of the three phases has its own hard 20-minute limit; admission cannot borrow
later time. Exhausting a phase ends the candidate. The whole gate is at most 60 minutes.
Reconciliation remains owned even when a time limit prevents a passing verdict.

The worker must establish native ownership as well as transport compatibility.
A Desktop restart,
app-server replacement or disruption of unrelated active work is outside this
gate and must be proposed separately. Installing only the scoped existing worker
after approval is allowed; normal MCP discovery,
genuine binding, and existing consent apply. No broad approval-policy change.

“Normal refresh” permits the shipped CLI marketplace/plugin registration commands
and a first-party plugin refresh operation only if already exposed in the genuine
owner context. It does not permit an invented reload message, Desktop restart,
app-server replacement or native Settings automation. Static
source currently establishes only an internal-marketplace reload path, so failure
to discover the scoped worker on resume is a credible early inconclusive result.
It rules out this bounded activation attempt, not the private interface generally.
The approval explicitly includes one activation remedy: if the verified genuine
Desktop inspector cannot discover the worker on resume, it may create one fresh
local Desktop execution-owner chat named “Gimbal native-control worker” through
the ordinary advertised task-creation tool. Verify effective create/read/stop
before doing so. This is the sole extra owner allowed; it stays on the same Amrita
Desktop instance, is not a coordinator, and uses the isolated worker directory.
If fresh-task discovery also fails, stop. This is an alternative activation of
the same candidate, not another route or permission to exceed admission's budget.

The audit has not established a directly usable Gimbal endpoint into Desktop's
native-capable runtime, so this gate tests only the existing scoped worker through
the named bootstrap. Client-coordination IPC and a possible direct remote-control
integration remain audited follow-on possibilities; their Gimbal-callable
fresh-task/lifecycle contracts are unestablished. Failure of the worker does not
rule them out. If new evidence establishes an adequate direct endpoint, stop and
propose that smaller route instead of installing the worker. Do not build it
under this unamended gate. Worker stale-turn failure ends this gate.

Before submitting any native work, verify peer admission and the effective
task-create/read/stop interfaces and their input/result schemas in the genuine
Desktop context. Verify a way to reconcile and stop an owned task using a bounded
harmless task first; stop if this is unavailable. Do not discover missing cleanup
after launching native interaction. Static tool definitions alone are insufficient.

In the actual worker context, `status` with the required Finder bundle ID must
also confirm saved approval. Its `appReadiness` reads a protected Computer Use
container file; an SSH read was denied during review and does not establish
whether Desktop's MCP child can read it. Verify only approved/missing/error status,
not a raw consent-store dump. If access is denied or consent is absent, stop with
that precise prerequisite failure. Do not omit `requiredApps`, change container
permissions, edit consent, or silently bypass this check.

Admission includes a ready Desktop renderer accepting the real caller/context,
not only a successful socket connection. After binding, allow the owner's binding
turn to finish, then verify the worker still admits the same stored context.
Repeat the admission check while locked before native interaction. Record whether
the renderer is ready in that observed lock/display state. If stale owner-turn
metadata stops working, stop that candidate; do not keep the turn artificially
alive, fabricate current IDs, or silently redefine ownership.

For the worker route, the specific bootstrap candidate is the existing Amrita
inspector chat “Inspect Mac qualification tools without…”
(`01a193e2-c488-7b03-9fe7-033f6b4e0ba5`, host
`remote-control:env_e_6a4d4d99990c8322b549a2c0ba9fecef`). A read-only snapshot
found it not loaded; its prior positive native comparator is historical evidence.
The remote-control host label is its address from this controlling client; it is
not proof that the resumed task is local to Amrita's Desktop. The first check
must establish its actual runtime owner and host-relative addressing. The existing
worker's app-tools calls default to `local`; it is admissible only if that local
host is Amrita's intended Desktop runtime and returned child host IDs are usable
for read/stop there. Do not feed the controlling client's remote host label into
that local bridge or infer locality from the directory path.
Approval explicitly authorizes a targeted follow-up to that existing chat to
verify genuine Desktop context and bind the scoped worker after normal discovery.
The existing host/task routing can address it without depending on the proposed
worker. It must re-establish real Desktop identity and admission, not infer either
from its title or previous success. Failure to resume with genuine Desktop context
is an early no-go for this bootstrap. Only after genuine Desktop context is
verified is the single fresh-owner activation remedy above permitted. The owner
is an execution task, not a coordinating chat. Sol keeps all
coordination here. Approval also explicitly authorizes the bounded native execution
child tasks submitted by Gimbal after admission/cleanup checks. Keep the owner
loaded across both trials, but do not require its initial turn to stay active.
“Fresh task” in C6 means a new child execution task under the same owner, not
re-owning its ledger or proving unattended Desktop cold start.

Both native trials must be actual runs through the existing Gimbal controller,
using ordinary workflow operations to submit and observe worker requests. A
temporary uncommitted workflow outside git may provide that invocation before a
maintained caller is needed. A shell-only invocation of the worker is insufficient.
Feasibility passes only with admission/cleanup established and C5/C6 observed
through those Gimbal runs. It does not imply C1–C4 or maintained integration are
complete; those remain final delivery claims after the concrete amendment.

For native task models, the temporary caller uses Gimbal's existing
`internal/modelalias.Resolve` on the unversioned `luna` selection and passes its
resolved provider-native model explicitly to the worker. The implementation is in
`internal/modelalias/modelalias.go`; generated alias data lives beside it. No
version is hard-coded in the caller and no new resolver is built. Before the
harmless admission task, compare that resolved model with Desktop's advertised
model choices. Record both alias and actual selection. A mismatch is a model
configuration blocker, not evidence that native control is impossible. Never use
the worker's legacy hard-coded default or silently choose another model.

Set `GIMBAL_DESKTOP_DIR` explicitly to
`/Users/tyler/Library/Application Support/Gimbal Qualification/desktop-worker`
in the scoped MCP registration and every caller. The historical
`~/.gimbal/codex-desktop` ledger belongs to another owner and is preserved;
do not move, delete or re-own it. If the isolated directory now exists and belongs
to another live owner, stop and reconcile instead of replacing it.

Use the already-staged qualification marketplace at
`/private/tmp/gimbal-native-qualification-_h1f4m_4/desktop-integration`, whose
`plugins/gimbal-codex-desktop/.mcp.json` explicitly sets that path in its
`mcpServers.gimbal_desktop.env` map. A read-only recheck confirmed that map;
the shipped source's `env_vars` whitelist alone does not set it. After approval,
copy the staged marketplace to the owned qualification `desktop-integration`
directory before normal registration. Verify the installed effective `env` value
before any bind; an unset or different value is an admission failure. Scoped
configuration/provisioning is authorized by the gate; runtime source-code changes
are not. Do not install the unmodified default registration accidentally.
Before copying, compare staged runtime files against the actual revision under
test and record that revision; review found them equal to `2f67be83`. Missing or
changed staging is a stop for reconciliation, not permission to trust its name.
The durable recipe must always require the scoped registration/env delta; installing
the repository default without it is not the qualified operating procedure.

Experiments are serialized. Reuse the existing qualification service and Desktop.
One owned worker, one active native task at a time. Do not create another
app-server merely to probe. Preserve unrelated services and the release-design
worktree. Lost responses never justify blind resubmission or task duplication.

Treat lock state as an observed admission condition. Use adjacent read-only
`CGSSessionScreenIsLocked` from `CGSessionCopyCurrentDictionary`, with
`IOConsoleLocked` recorded separately, and `CGGetOnlineDisplayList` plus
`IODisplayWrangler` power/activity observations for display state. The CG session
lock indicator must be true before and after each protected native trial; unknown
cannot pass. Do not require the two lock indicators to agree: prior research found
they can differ. Correlate the native action with authorization ALLOW, successful
automatic unlock and relock logs; record limits if logs are unavailable. Missing
corroboration leaves the protected-lifecycle claim unmet.
If the Mac is not locked, record that the
locked check is unmet and end that phase as inconclusive; do not wait indefinitely,
request manual unlock, manipulate the lock screen, or alter sleep/lock settings.
Unknown lock state also cannot pass. The 20-minute admission cap may commonly
produce an inconclusive result; it limits investment rather than proving a route
impossible. Plugin discovery on the resumed owner is deliberately a live gate:
static reload support does not guarantee it. If the newly registered tool is not
discovered through normal refresh or the single explicitly approved fresh-owner
activation remedy, stop without any further owner creation.

## Observable definition of done

The native target is Finder (`com.apple.finder`). Its saved persistent approval
is not yet confirmed from the actual worker context; the admission status check
above must confirm it. In each native trial, observe Finder's accessibility and
window, open its Go to Folder dialog, type a unique trial marker into the dialog
without submitting navigation, capture an original native screenshot showing that
input, click Cancel, and observe that the dialog closed and the original window
remains. Do not submit the marker as a path, edit files or consent dialogs, or
substitute another app after failure. This gives reversible native typing and
clicking with a visible result, under the same C5/C6 lock evidence requirements.

| Claim | Classification / cells | Required observation |
| --- | --- | --- |
| C1 Authentic worker availability and ownership | Cell-local, Desktop worker | Actual discovery/bind and owner identity are verified if this route is used; unloaded/missing owner fails clearly without fabricating context |
| C2 Submission identity survives uncertainty | Cell-local, Desktop worker | Same logical request does not create a duplicate; conflict/unknown outcome preserves actionable ownership. Focused existing tests plus relevant runtime reconciliation |
| C3 Caller handles real task outcomes | Cell-local, Gimbal caller | Failed observation and pending approval are reported as such; no completion claim from run status alone; cancellation/unknown outcome remains owned |
| C4 Scoped lifecycle | Cell-local, Desktop worker | Worker socket access is scoped, no second owner replaces a live worker, and owned task termination is confirmed. Missing cleanup support is an admission failure. A later transient failure must remain explicitly owned/unconfirmed, but does not count as a passing C4 |
| C5 Gimbal-originated native work while locked | Integration, both cells + Desktop/native helper | Gimbal initiates a genuine task; it reads native accessibility, captures an original native screenshot, types and clicks a harmless reversible target, and observes the result. Adjacent lock/display observations and correlated authorization/unlock/relock evidence support the claim. No manual unlock or safeguard change |
| C6 Repeatability | Integration, both cells + Desktop/native helper | A fresh task repeats C5 without manual unlock. New task means new execution context, not continuing the successful task. Document Desktop/worker persistence limits; no cold-start guarantee inferred |

An operating recipe identifies exact interface and launch/configuration settings,
actual component versions/revision, owner lifetime, consent prerequisites,
bounded failure recovery and remaining limits. It includes the independently
reviewed socket-audit checklist. The independent Sonnet validator inspects the
actual application behavior and owning revisions, not only the implementer's
description. Evidence is reported in chat/PR under repository policy; original
media may remain outside git with usable links. Never commit screenshots, run
logs, text dumps or proof programs. Review and research notes are retained.

## Exclusions

- Release qualification scheduling/promotion and the broader resource-fix PR.
- Desktop-independent operation, arbitrary hosts or all lock/display states.
- Manual unlock as recovery, disabled Locked Use, changed security policy,
  identity spoofing, synthetic metadata, private authorization calls, helper patches.
- New coordinating chats, extra app-server instances for probing, whole-machine
  process caps, or shutdown of unrelated Desktop resources.
- Full new harness semantics, generic workflow abstractions, or unattended
  reliability from one successful screenshot.

## Deferred decisions / stop conditions

- Final route is selected only after independent audit review; a listener alone
  cannot establish native authorization. A different architecture requires an
  approved contract amendment before implementation.
- Missing native-app consent that requires human action is a precise blocker;
  do not approve dialogs or weaken settings automatically.
- Failure to restore lock must stop native testing immediately and be reported;
  do not continue merely because interaction succeeded.
- If no route passes within the time/attempt budget, return a candidate-specific
  no-go or inconclusive finding and options. Do not claim universal impossibility
  or begin a larger experimental build.
- Owner restart/unload recovery is documented only as observed; automatic
  cold-start provisioning is not silently added.
