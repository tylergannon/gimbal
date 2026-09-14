# #193: Run page shows scope values as raw JSON text, not as the value

Captured 2026-09-14 from https://github.com/tylergannon/gimble/issues/193. Read beta-implementation-handoff.md and beta-bug-triage.md for final session decisions and coordination notes.

Found in the loop practice runs (#178): shape 1 run `01M2EQC626WZFR69JBN913T41M.planner-ends`, page saved at `ephemeral/attest/loop-practice/planner-ends/page.html`.

**Expected:** a scope value set with `gimble.Set(ctx, "files in the workspace", "spring.txt (72 bytes)\nsummer.txt (82 bytes)")` is shown on the run page as that text, on two lines, the way `ScopeText` renders it for the planner (a JSON string as its text, anything else as indented JSON).

**What happened:** the scope's values are rendered as the raw `ValueSet.Value` JSON in a `<p>`:

```
<p class="svelte-c1hvgl">files in the workspace · "spring.txt (72 bytes)\nsummer.txt (82 bytes)"</p>
<p class="svelte-c1hvgl">task · {"name":"Create spring haiku","description":"Create `spring.txt` in the workspace. ...
```

so a string value shows with its quotes and `\n` escapes on one line, and the task record is a one-line JSON blob. A multi-line command transcript (`"$ test -f data.txt ...\nexit 0\n"` in shape 2 run `01M2EQC7S1CTSVRBX7KRR83SXN.validation-command`) is unreadable this way, and it is the record an operator wants to read first.

Held until #173 merges if it touches the run route; filed so the practice notes can point at it.
