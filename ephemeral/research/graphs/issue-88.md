# Add a risk-based fast path for small Tractor releases

URL: https://github.com/tylergannon/gimble/issues/88
State: closed
Milestone: None
Updated: 2026-09-13T16:47:16Z

## Problem

Tractor applies the full release proof pipeline to every change regardless of risk. A small manifest-serialization change currently requires full race/vet/lint/plugin checks, a real multi-agent canonical workflow, a separate release PR, and another canonical run on the squash-merge commit because commit identity changes the binary hash.

This makes tiny releases slow, expensive, and noisy without proportionate evidence value.

This policy was created by coding agents, not requested by Tyler. It is an example of agent-authored process expanding until the ceremony obstructs ordinary software delivery. The correction must reduce agent discretion to add unrequested gates and must treat user time and token consumption as real costs.

## Desired outcome

Define a risk-based release path that keeps strong proof for runtime/workflow changes while allowing narrowly scoped changes to use focused behavioral proof.

At minimum:

- avoid rerunning equivalent expensive proof solely because a squash merge or version-only commit changes build identity;
- run the canonical multi-agent workflow only when the changed surface can affect it;
- combine feature and version publication when release intent is already known;
- preserve an explicit full-proof path for engine, workflow, harness, and release-critical changes.

The fast path must remain evidence-based: it should name the changed behavior and require a focused executable demonstration, not merely skip checks.

## Operating rules

### Minimal narration

Use one short starting message, report only a real blocker or meaningful state change, and finish with a concise result. Do not narrate every test, commit, upload, wait, or routine tool call.

This keeps the user's attention on decisions that need them and stops status prose from consuming a material share of the task's token budget.

### Upload proof only when the artifact demonstrates something material

Ordinary tests and CI output are enough for ordinary changes. Upload durable artifacts when the claim needs independent inspection, such as UI behavior, an external integration, or a complex workflow run. Do not upload JSON merely to prove that JSON was written.

This preserves proof where it adds confidence without turning routine implementation into artifact administration.

### Do not invent abstractions beyond the issue

Implement the requested contract directly. Do not add history models, migrations, generalized policy, or adjacent semantics unless the issue requires them or the existing design makes them unavoidable.

For issue #71, this means adding flat fields to the existing manifest, not designing an invocation-history model. Smaller scope means less code, documentation, testing, and rework.

### Do not add agent-authored ceremony

Do not create extra plans, review loops, PRs, validation layers, or release gates merely because they appear rigorous. Every step must be required or catch a plausible failure for the specific changed surface. If a step cannot name the realistic defect it detects, remove it.

This makes rigor serve delivery instead of displacing it.

## Default small-change path

For a narrow, low-risk issue:

1. Implement the issue literally.
2. Run focused behavioral tests and the ordinary CI suite.
3. Squash-merge.
4. Continue to the next issue.
5. Publish one release after the requested batch.

Reserve the full canonical workflow for changes that can affect engine traversal, workflow execution, harness behavior, or the release system itself.

