# Native-control gate proof: adversarial review, round 01

Reviewer: Sonnet 5.5, read-only. Only this file was written. No native
computer-use calls, tasks, service changes or experiments. SSH to amrita was
read-only. I did run the existing read-only `read-episode-log.py` helper
(a bounded `log show` query) for the two indexed windows.

## Review target

The bounded native Mac control gate (Finder Go to Folder via the Desktop worker,
started from Gimbal, Locked Use enabled) and its proof. Reviewed against `spec.md`,
`approval.md`, `plan.md`, `operating-recipe.md`, repository `AGENTS.md`, and the
original records indexed in `/tmp/gimbal-native-evidence-index.txt`.

The launch prompt narrows nothing and predicts no verdict. Scope came from the
spec's definition of done and claims C5/C6.

## Evidence inspected

- **Media.** I re-hashed both originals locally at `~/.codex/outputs/native-control-feasibility/2026-11-01/` and the `/tmp` copies on both hosts.
  - Hashes are `da31aecc…3b60` and `7b89034f…f233cf527`, as indexed.
  - I extracted the inline base64 JPEG from `function_call_output` in each child's retained rollout archive on amrita. Both hash to the same values, so the images are the unmodified tool output.
  - I viewed both. Each shows Finder "Amritanandamayi" with the Go to Folder sheet holding the exact marker, `…-1-RETRY1` and `…-2`.
- **Child archives**
  - Tool-call sequences for children `01a19a38-b98e…` and `01a19a3e-bcb5…` show: select `com.apple.finder`, click Go (index 120), click Go to Folder (index 33), a fresh AX read that shows the focused `PathTextField`, `typeText` with one argument, `getScreenshot`, an AX read of dismissal controls, `pressKey("Escape")`, then an AX read showing the original `standard window Amritanandamayi`.
  - The first attempt's failure on `typeText` with an element index is consistent with the recipe.
- **Worker ledger** `desktop-worker/sessions.json`
  - It has one owner, `01a19a25-3fbf…`, and five requests, all `cancelled`. Their thread IDs and `hostId: local` match the index.
  - The owner's binding turn ended before admission, as claimed.
- **Installed plugin**
  - The cache's `server.mjs`, `lib.mjs` and `client.mjs` hash identically to `plugins/codex-desktop/` in this worktree, and the plugin is unchanged since `2f67be83`.
  - The installed `.mcp.json` sets `env.GIMBAL_DESKTOP_DIR` to the isolated directory.
- **Gimbal runs**
  - `01M6D3GJJH8KFQNA6SGM23544T` and `01M6D3WTE3CZVCWBGTB62T2CTN` exist on amrita and are `completed`. Model calls are `gpt-6-luna`.
  - The `start.json` and `native-task.txt` files give the caller's exact submissions. The first run's `interactions.json` holds the caller's report.
- **Lifecycle logs.** My re-run of the helper reproduces the indexed ALLOW, "Screen unlock succeeded", `kLWLockFromDirectLock` timestamps for both windows.
  - Run 1 is 03:06:22.08 and 03:06:44.55. Run 2 is 03:12:54.46 and 03:13:17.34.
  - They bracket each child's native tool calls tightly. The first tool call is at 03:06:20 and the last result at 03:06:41, with relock about 3 s later; trial 2 is the same pattern. This is real corroboration, though still temporal, as disclosed.
  - Keybag `UnlockSuccess = NO` appears in both windows, as disclosed.
- **Repository.** `git log`, the worklog, and the committed spec and recipe.

## Findings

### 1. The required "click Cancel" step was not performed, and the procedure was rewritten without an amendment (issue)

The definition of done (`spec.md:262-266`) says to "click Cancel, and observe that
the dialog closed". `spec.md:262-264` also says not to "substitute another app
after failure". C5 requires "types and clicks a harmless reversible target".
`plan.md` and `approval.md` promise the same scripted procedure, and no amendment
exists.

The AX dumps (child 1 record 86, child 2 record 70) confirm the sheet exposes only
`CloseButton` and `PathTextField`, so there was no Cancel control to click.
The executor handled this without reporting it upstream first:

- The `start.json` prompts for both trials tell the child to fall back to Escape and "explicitly report that procedure variation". That pre-authorises the substitution.
- The Gimbal conversation agent's own report, `interactions.json` for the first run, says it "does not demonstrate the full approved native-control contract: the required literal Cancel click was not performed".
- The recipe discloses the difference. But its "Scope" paragraph still says the tasks "demonstrated native menu clicks, focused typing, original screenshots and restored Finder state through Gimbal". The worklog calls it a `doc_bug` assessed later.

The disclosure is honest. The gap is that the spec's observable definition of done
is unmet as written. Tyler approved the literal procedure, not a variation. A
"gate passed" conclusion needs an explicit amendment or Tyler's acceptance that
Escape equals Cancel, because Escape is a keyboard dismissal, not a click on the
sheet. The 2nd trial deliberately varied only the click route into the sheet, not
the dismissal.

Impact: C5/C6 are partially met. The typing, screenshot and restore observations
are solid. The click-dismiss observation is absent.

### 2. Lock-state requirements have no retained evidence I could verify, and the spec says unknown cannot pass (issue)

The spec requires CG session lock `true` before and after each protected trial
(`spec.md:241-246`). It also requires that "unknown cannot pass" and that admission
be re-checked while locked before native interaction (`spec.md:158-163`). It asks
for `IOConsoleLocked`, online-display and `IODisplayWrangler` observations recorded
separately.

The index asserts these only as root observations, and only partly:

- Trial 2 has a before (03:12:14) and an after (03:14:25).
- Retry trial 1 has only an after (03:08:52). The index gives no before reading for it.
- The first attempt's pre-state is not given.
- Neither the index nor any inspected file has a retained record of the CG, IOConsole, display-wrangler or locked admission re-check. They are not in the owner transcript, the run directories, the inputs directory or the worklog.

Impact: the protected-lifecycle claim in C5/C6 is the central claim, and its
pre-lock premise cannot be verified from retained records. The unlock/relock logs
(which I verified) show a lock transition occurred around each trial. They do not
show the required before/after indicator values, and the helper's own caveat applies:
records are not task-tagged. By the spec's own rule, missing corroboration leaves
the claim unmet.

### 3. Resolved model selection and the Desktop model-comparison step have no retained record (issue)

`spec.md:200-208` requires the caller to resolve `luna` through
`internal/modelalias.Resolve` and pass the resolved native model. It also requires
comparing that model with Desktop's advertised choices before admission, and
recording both alias and actual selection.

The submissions contain a hand-typed `"model": "gpt-6-luna", "thinking": "high"`
in `start.json`. Nothing I inspected shows `Resolve` being called, and
`aliases_gen.go` lists both `gpt-5.6-luna` and `gpt-6-luna`, so "latest" is plausible
but unrecorded. There is also no retained record of the advertised-model comparison.
The children's session metadata does show `gpt-6-luna`. That shows the model was
accepted, not that the spec's resolution and comparison steps were run.

Impact: low-to-moderate. The model worked, so this is a process gap against a
stated requirement, not a behavioural failure.

### 4. The "Gimbal caller" is an LLM conversation agent shelling out to `client.mjs`, so the C3 caller semantics were not exercised (nitpick, bordering on issue)

The spec describes "ordinary workflow operations" and a temporary uncommitted
workflow. The actual caller is the existing `conversation` workflow. A luna agent
was told by a human message to run `client.mjs` start, read and cancel.

This is disclosed in the recipe ("existing controller input machinery") and gates
only C5/C6, so it does not break the gate. But the "Gimbal initiated" claim is
about as indirect as it can be: the shell call is made by a model following prose.
The cancel of every child was also the caller's own instruction, not a
caller-side policy.

The recipe correctly says C1-C4 and the maintained caller remain unproved. The
remaining risk is a reader taking "through Gimbal" as meaning the Go caller.

### 5. No-manual-unlock and no-config-change are asserted, not evidenced (nitpick)

The index says "No manual unlock or settings changes were reported", and admits the
LaunchAgent check is scoped, not exhaustive. The unlock log shows the screen unlock
used the authorization socket ALLOW path ("using password", same as the lock
helper), which is the expected temporary-unlock route. I found nothing
contradicting the claim, and the screen-lock timing fits an automatic cycle. But
the claim rests on reports from the executing agent.

## Verified and sound

- Original media provenance: the hashes match the base64 payloads inside the retained rollouts.
- Worker ownership and cleanup: the ledger shows one owner and five requests, all cancelled.
- Installed worker equals the repository code, with the scoped `env` set.
- The lifecycle logs bracket both native sequences.
- The Escape/Cancel discrepancy and the keybag caveat are disclosed rather than hidden.

## Outcome

material findings remain
