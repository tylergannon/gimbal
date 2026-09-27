# EKS infra for remote Gimbal workers: GitHub auth, secrets, namespaces, images, Temporal, web instance

Research date: 2026-09-27. Scope: infrastructure side only (per BRIEF.md),
for Gimbal workflows projected to Temporal with worker turns running as
harness processes in pods on EKS. Every claim is tagged **[V]** (verified
against a fetched source, cited) or **[I]** (inference, not confirmed
against an external source today).

## Findings

### GitHub access

1. **Installation access tokens expire in exactly one hour, with no
   documented way to extend that.** **[V]** A 40+ minute worker session must
   mint a fresh token before or during the run, not once at start. Only
   GitHub's own SDKs (Octokit) auto-regenerate; a bespoke client must call
   `POST /app/installations/{id}/access_tokens` again before expiry
   (docs.github.com, "Authenticating as a GitHub App installation" and
   "Generating an installation access token for a GitHub App"). Since a
   staged rollout starting 2026-04-27, new tokens use a stateless
   `ghs_APPID_JWT` format — a performance/reliability change, not a lifetime
   change (github.blog changelog, "GitHub App installation tokens:
   per-request override header").

2. **Clone/push over HTTPS needs only the "Contents" permission; PR
   creation needs "Pull requests" too.** **[V]** Clone URL convention:
   `https://x-access-token:<token>@github.com/owner/repo.git`. A community
   report (discussion #173881) claims the username string itself isn't
   checked server-side — treat that specific detail as **[I]**, since it is
   not GitHub's own documentation, only convention it recommends.

3. **A long-lived worker should refresh through a git credential helper,
   not a token baked into the remote URL.** **[I]** Not documented by GitHub
   for this exact case, but implied by the 1-hour lifetime plus git's
   `credential.helper` mechanism: a helper script invoked per git operation
   can mint-or-reuse a cached installation token and refresh if <5 minutes
   remain, so no token is ever written to `.git/config`. GitHub's own
   `actions/create-github-app-token` Action mints once per CI job, which is
   fine for CI but not for a Gimbal worker whose job outlives a token.

4. **Repo selection at app-install time already is the "configure repos"
   UI.** **[V]** GitHub's own installation flow offers "All repositories" or
   "Only select repositories," managed afterward from the app's settings
   page — no separate Gimbal UI needed for "which repos can Gimbal touch"
   (docs.github.com, "Installing a GitHub App from a third party";
   "Reviewing and modifying installed GitHub Apps"). Gimbal's job reduces to
   *learning* what was selected (next point).

5. **`installation` and `installation_repositories` webhooks tell Gimbal
   which repos are enabled, live, with no polling.** **[V]** These events go
   only to the app's configured webhook endpoint ("you cannot manually
   subscribe to this event"); `installation_repositories` fires
   incrementally as repos are added/removed after initial install
   (docs.github.com, "Webhook events and payloads"; community discussion
   #24379). This requires one webhook receiver reachable from GitHub —
   an ingress into whichever service is "the Gimbal web instance" (finding
   19).

6. **Installation-token rate limits are generous and unlikely to bind.**
   **[V]** Base 5,000 requests/hour per installation, +50/hour per repo
   beyond 20 repos and +50/hour per org member beyond 20 users, capped at
   12,500/hour (15,000/hour on GitHub Enterprise Cloud) (docs.github.com,
   "Rate limits for GitHub Apps"). Binds only if workers poll GitHub's API
   for status per turn rather than doing one clone/push per session.

7. **Prior art converges on: connect once via an app, pick repos at connect
   time, configure the rest with a repo-resident file, not a settings UI.**
   **[V, mixed sources]**
   - *OpenAI Codex Cloud*: pick repos at connect time; runs from
     `ghcr.io/openai/codex-universal` (Dockerfile at `openai/codex-universal`)
     plus a setup script that runs with internet on, in its own bash session
     (env vars must persist via `~/.bashrc`); reads `AGENTS.md` for
     lint/test commands (developers.openai.com, "Cloud environments").
   - *Claude Code on the web*: repo cloned into an Anthropic-managed VM;
     "GitHub authentication is managed through a secure proxy, so
     credentials never exist directly in the environment"; a saved
     "environment" holds network mode (Trusted/Custom/Full), `.env` vars,
     and an optional setup script (code.claude.com/docs; support.claude.com).
   - *Cursor Cloud Agents*: repo access via Cursor's own GitHub App
     (read-write on repo + submodules); environment as agent-led setup, a
     saved *snapshot*, or `.cursor/environment.json` checked into the repo
     (cursor.com/docs, "Cloud Agents"/"Cloud Environment Setup"/"GitHub").
   - *Google Jules*: GitHub-integrated, clones into a sandboxed cloud VM per
     task; public beta as read today (jules.google/docs; blog.google Jules
     launch post).
   - *Devin (Cognition)*: GitHub App with admin-controlled repo selection;
     secrets layered as org/personal/repo-scoped/session-scoped, dedicated
     service accounts recommended (docs.devin.ai "GitHub integration";
     fast.io summaries of Cognition's docs — secrets taxonomy **[I]**-leaning
     since fast.io is third-party, GitHub App mechanics **[V]**).

   Common shape: one-time app connect + repo picker, one config file/script
   the repo owns, one saved image/snapshot, scoped secrets, network mode as
   a named setting. None expose a UI for token lifetime or credential
   plumbing. This argues Gimbal's user-visible surface should be exactly
   that small: install the app, pick repos, optionally drop a setup
   script/`AGENTS.md`, pick a secrets scope — token minting, refresh, and
   namespace wiring stay invisible.

### Secrets on EKS

8. **AWS describes EKS Pod Identity as the simpler successor to IRSA for
   Linux EC2 workloads, without deprecating IRSA.** **[V]** Pod Identity
   needs no OIDC provider, uses one reusable IAM principal
   (`Service: pods.eks.amazonaws.com`), and centralizes credential issuance
   in a per-node DaemonSet agent rather than in each SDK (AWS EKS User
   Guide, "Learn how EKS Pod Identity grants pods access to AWS services").
   Restrictions that matter here: **not available on Fargate or Windows
   nodes, Linux EC2 only**; associations are eventually consistent (seconds
   of delay), so don't gate a pod's first API call on it in a tight startup
   path (same source). Secondary sources (builder.aws.com, several Medium
   posts) frame Pod Identity as the 2026 default for new clusters while
   IRSA stays fully supported — **[I]** for that "default" framing
   specifically, since it's not AWS's own wording.

9. **External Secrets Operator (ESO) fits better here than the Secrets
   Store CSI Driver, mainly over the daemonset requirement.** **[V]** ESO's
   AWS Secrets Manager provider supports controller Pod Identity, IRSA,
   static credentials, session tokens, and assumed roles
   (external-secrets.io, "AWS Secrets Manager" provider page). ESO runs as
   an ordinary Deployment syncing into native `Secret` objects; the CSI
   driver runs as a required privileged DaemonSet on every node and is
   **not supported on Fargate** (secondary synthesis: devopscube.com,
   kubeblog.com, AWS Security Blog "Making sense of secrets management on
   Amazon EKS for regulated institutions" — **[I]** overall, the
   Fargate-daemonset incompatibility specifically is widely repeated but not
   confirmed on an AWS-owned page fetched today). `SecretStore`
   (namespace-scoped) vs `ClusterSecretStore` (cluster-wide, must qualify
   target namespace) is ESO's own per-project isolation mechanism: one
   `SecretStore` + one Pod Identity association per project namespace,
   scoped to only that project's secrets. **[V]** for the
   `SecretStore`/`ClusterSecretStore` scoping mechanic itself.

10. **Recommended shape for harness keys and the GitHub App private key**:
    one Secrets Manager secret per credential (OpenAI, Anthropic, Diffusion
    Router, GitHub App private key + App ID + per-installation ID); one IAM
    role per project namespace scoped by resource ARN (not wildcard) to
    exactly that project's secrets; one Pod Identity association binding
    that role to the namespace's worker `ServiceAccount`; one namespace-
    scoped `SecretStore` + `ExternalSecret` per project syncing into a
    `Secret` the worker pod mounts. **[I]**, a composition of findings 8–9,
    not itself found written as a recipe on any page read today.

### Namespace-per-project on EKS

11. **The standard hardened-namespace recipe is default-deny
    `NetworkPolicy` + least-privilege `RoleBinding` + `ResourceQuota`/
    `LimitRange` + teardown on idle.** **[I]** Consistent across 2026
    secondary sources (oneuptime.com posts on namespace isolation and
    multi-tenancy), but no official Kubernetes multi-tenancy doc was
    fetched today — worth a follow-up read of kubernetes.io's own SIG docs.

12. **Idle namespaces are a real, quantified cost risk.** **[I]** One
    secondary source cites 568 of 590 workloads idle at a snapshot on a
    preview cluster, with scale-to-zero on idle cutting the bill ~67%
    (~$218k/yr) — numbers as reported, not independently verified
    (oneuptime.com). Argues for a controller that scales worker
    `Deployments` to zero when a project's task queue is empty, on top of
    Karpenter node consolidation.

13. **Namespace-per-project is the simplest option, with real
    alternatives:** **[I]**
    - *One namespace, per-project task queue + label*: cheapest
      control-plane footprint, but RBAC/NetworkPolicy stop being
      namespace-scoped — isolation moves entirely to pod-label selectors,
      weaker and easier to misconfigure.
    - *vCluster*: a full virtual API server/control plane per project
      inside a namespace of the host cluster, marketed as an EKS-sprawl fix
      with a claimed up to 80% control-plane cost cut vs. one cluster per
      tenant (vcluster.com blog, vendor-authored — cost figure **[I]**,
      mechanism description accurate). Real extra infrastructure to
      operate; likely oversized for Gimbal's initial project count.
    - *Kata Containers / gVisor via `RuntimeClass`*: addresses a different
      threat — an agent turn running arbitrary generated code inside the
      worker container — not project-to-project isolation. gVisor
      intercepts syscalls in a user-space kernel, cheaper generally but
      costly on syscall-heavy work (installs, compiles — exactly a
      coding-agent turn); Kata runs the pod in a lightweight VM with its
      own guest kernel, ~150–300ms extra scheduling latency, for a stronger
      boundary (synthesis: northflank.com, bex.co, systemshardening.com,
      sailor.sh, cloudrps.com — all **[I]** for the numbers). The
      Kubernetes SIGs `agent-sandbox` project has a "Kata Containers" use-case
      page confirming Kata as an accepted pattern for agent sandboxes
      specifically — **[V]** for that page's existence and topic. Given
      Gimbal workers run harness CLIs installing deps and executing
      generated shell/code, `RuntimeClass: kata` (or gVisor if Kata's
      overhead is too high) on the worker pod template is a reasonable
      default regardless of the namespace decision — orthogonal, additive.

14. **Karpenter fits bursty, hour-scale pods, but its exact current version
    could not be pinned down reliably today.** **[I, low confidence on
    version]** Secondary sources cite both "v1.13" and "v1.14.1 (LTS)"; a
    direct fetch of the releases page returned a summary dated August 2024
    for "latest" — internally inconsistent with 2026, so treat any specific
    version number here as unverified and re-check
    `github.com/aws/karpenter-provider-aws/releases` by hand before use.
    Independent of exact version: Karpenter provisions right-sized EC2
    capacity directly (bypassing node-group autoscaling), targets ~45–60s
    to bring up a node, supports spot vs. on-demand via
    `karpenter.sh/capacity-type` on a `NodePool`, and does spot-to-spot
    consolidation as prices move (cast.ai, cloudbolt.io, dev.to — all
    **[I]**, Karpenter's own docs not fetched today).

15. **Spot vs. on-demand for a 40+ minute non-checkpointable agent turn**:
    **[I]**, reasoned from Karpenter's consolidation behavior plus AWS
    spot's well-known two-minute interruption notice — a turn that can't
    checkpoint and must restart from scratch on interruption is a poor fit
    for spot at the pod actually running the harness process during a
    turn; spot suits the rest of the fleet (Temporal workers between turns,
    ESO, ingress). Default: on-demand-only `NodePool` for the node class
    hosting in-flight turns, spot elsewhere, with `do-not-disrupt`
    annotations or a `consolidateAfter` long enough to outlast one turn on
    the turn-hosting pool.

### Container images

16. **All observed prior art separates a base image (harness + runtimes)
    from a per-repo setup step, treating the base as a named, versioned,
    cacheable artifact, never built per session.** **[V]** Codex Cloud's
    default image is `ghcr.io/openai/codex-universal`, from a public
    Dockerfile (developers.openai.com). Cursor supports agent-led setup, a
    saved snapshot, or a `.cursor/environment.json` Dockerfile reference,
    with snapshots capturing "installed packages, system dependencies" so
    later sessions boot warm (cursor.com/docs). Claude Code on the web has
    a named, editable "environment" reused across sessions rather than
    rebuilt each time. For Gimbal this matches the brief's own assumed
    layout: one base image per harness set (Codex CLI, Claude Code, Pi,
    OpenCode, agy, pinned versions), one project layer for toolchains on
    top, built on a cadence independent of any run.

17. **Kaniko appears legacy for new on-cluster build pipelines in 2026;
    BuildKit (rootless) is the currently favored daemonless on-cluster
    builder; Buildah is the OpenShift/Podman-flavored alternative.** **[I]**
    From multiple 2026 blog sources (lucaberton.com, alexandre-vazquez.com,
    devopsboys.com, codecentric.de, pandastack.ai) describing kaniko as
    unmaintained and BuildKit as offering better cache-mount layering and
    multi-platform support — none an official source; re-check
    `GoogleContainerTools/kaniko`'s own repo/archive status directly before
    this affects a real decision. Given Gimbal is already inside a
    GitHub-App-integrated flow, **building the project layer via GitHub
    Actions → ECR is the lower-operational-load choice** versus running
    BuildKit/Buildah on-cluster: it reuses existing CI infrastructure
    (Actions runners, OIDC-to-ECR push) rather than adding a
    build-capable, likely-privileged workload inside the same cluster that
    also runs untrusted agent code. **[I]**, a judgment call.

18. **Persistent repo cache (EBS/EFS PVC with worktrees) vs. fresh shallow
    clone per session has a concrete consequence for worker affinity.**
    **[V/I mixed]** AWS's EFS CSI driver supports `ReadWriteMany` — multiple
    pods, multiple nodes, one volume, concurrently (kubernetes-sigs
    `aws-efs-csi-driver`, "multiple_pods" example — **[V]** for RWX itself,
    standard documented EFS/NFS behavior). That makes an EFS-backed
    clone-plus-worktrees cache node-independent, removing the node-affinity
    problem EBS (typically AZ-scoped, `ReadWriteOnce`) would otherwise
    force. The cost is EFS's per-operation NFS latency against local/EBS
    disk for git and toolchain workloads doing many small file ops
    (**[I]**, general knowledge, not benchmarked today). A fresh shallow
    clone per session avoids this at the cost of full clone time every turn
    and no shared object cache across sessions on one repo. Given sessions
    run minutes to an hour and a project runs many sessions, a **shared EFS
    clone with git worktrees per session** (one canonical bare clone +
    worktree per workflow run) is the more scalable design, and it works
    from any node — Karpenter can churn nodes freely under it. **[I]**
    overall recommendation.

### Temporal on Kubernetes

19. **KEDA ships an official Temporal scaler as of v2.17.0**, reading
    task-queue backlog via Temporal's `GetTaskQueueMetadata` API and scaling
    a worker `Deployment` against a configured backlog `threshold`, with
    `workerDeploymentName`/`workerDeploymentBuildId` support to scale a
    specific Worker Deployment Version's backlog. **[V]** (temporal.io
    blog, "Announcing KEDA-based auto-scaling for Temporal Workers";
    keda.sh, "Temporal" scaler docs, `/docs/2.20/scalers/temporal/`). This
    is the documented, current mechanism for backlog-based worker scaling —
    no bespoke backlog-watcher needed.

20. **Known rough edge**: an open KEDA issue (kedacore/keda#6703) reports
    the Temporal scaler failing against Temporal Cloud with API-key auth
    because TLS isn't enabled on the scaler's own client in that path
    ("connection reset by peer"). **[V]** as a reported, open bug — smoke-
    test this combination before relying on it, or use mTLS for the
    scaler's own credentials even if workers use API keys elsewhere.

21. **Temporal's worker-shutdown docs describe a graceful-shutdown period
    without naming a specific default duration on the page read today**,
    and warn that long *Local* Activities are the wrong tool and "may block
    shutdown" — a design signal, not just an operational one. **[V]**
    (docs.temporal.io, "Worker Shutdown Behavior"). For a 40-minute in-flight
    activity: configure a graceful-shutdown timeout comfortably longer than
    the activity, ensure it honors context cancellation, and rely on the
    documented fallback — after the graceful period, "the Activity context
    is canceled and the Worker will finish shutdown when the current
    Workflow Task completes." Temporal's model assumes an activity *can* be
    cancelled cleanly; for Gimbal, the harness process spawned for a turn
    must actually respond to cancellation (be killed, or finish, but not
    hang past shutdown). Heartbeating during long activities (not detailed
    on this page for standard activities) is the standard mechanism to
    detect a stuck/killed worker promptly rather than waiting out a long
    `StartToCloseTimeout` — **[I]** as applied to Gimbal specifically.

22. **Temporal Cloud connectivity**: workers authenticate via mTLS
    certificates or API keys per namespace configuration; API keys are
    described as recommended for easier rotation; the documented endpoint
    shape is the gRPC namespace endpoint `<namespace>.<account>.tmprl.cloud:7233`,
    with TLS 1.3 on all connections regardless of auth method
    (docs.temporal.io, "Authenticate with mTLS certificates"; "Manage API
    keys" — read via search synthesis rather than a direct fetch today, so
    re-confirm the exact endpoint string before it goes into Helm values).
    A `temporal-worker-cert-rotation` reference implementation using
    cert-manager exists in Temporal's own GitHub org, suggesting mTLS is a
    first-class path for k8s workers specifically. **[V]** for that repo's
    existence and stated purpose.

23. **Self-hosted Temporal via `temporalio/helm-charts` is described by the
    project itself as a "V3 Helm chart," "tested by a dedicated test
    pipeline" and used by Temporal's own pipelines — not flatly a demo
    chart in the version read today** — but the same README pushes real
    operational load onto the operator: "for production and GitOps, manage
    database credentials in a Kubernetes Secret that you (or a controller
    such as External Secrets) create and own outside this chart," Cassandra
    cannot back the visibility store (only the default store), and a `helm
    upgrade` for dynamic config changes recycles pods. **[V]** (fetched
    directly). This contradicts the older, still-repeated "demo only"
    line from a different fork's README and at least one blog — that older
    warning appears to describe an earlier state or a different fork, not
    the current README. **Net**: self-hosting is documented and maintained
    by Temporal, but real operational load (persistence choice — Postgres
    or Cassandra + Elasticsearch for visibility — schema migrations,
    credential handling, upgrade discipline) that Temporal Cloud removes
    entirely. Given Gimbal's "as simple as possible" rule and that this is
    explicitly the first target, **Temporal Cloud is the lower-risk
    starting point**; self-hosting is a later option if data residency or
    scale cost make it worth the added surface. **[I]** for the
    recommendation, built on the verified findings above.

### Where the Gimbal web instance sits, and how events get to it

24. **Today, Gimbal's durable record is a local, filesystem-based event
    log** (`run.jsonl`, per-session transcripts, command stdout/stderr
    files); the web app's documented data path is "SSR snapshot first,
    then a Server-Sent Events stream; a past run is reduced from its logs"
    (`docs/web-app.md`, this repo — internal source, directly authoritative
    on current behavior). **[V, internal]**. This works today because the
    workflow process and web server share one filesystem/process. Projected
    to Temporal workers on EKS, that assumption breaks: harness turns (and
    the events they generate) run in pods that are not the machine serving
    the web app.

25. **Three realistic shapes for getting worker events back to "the Gimbal
    web instance" (all [I], reasoned, not found as Gimbal-specific
    guidance):**
    - *Worker → HTTP → one shared instance*: closest to today's model — the
      instance stays the single owner of `run.jsonl`-equivalent state,
      workers POST events to it. Simple on the current SSE architecture,
      but the instance becomes a point of contention for every project's
      live stream and must be reachable from every worker namespace (a real
      network-policy surface).
    - *Worker → S3, instance polls or is notified*: decouples workers from
      needing a stable instance endpoint at all times; S3 event
      notifications can wake the instance. Weakens the "live" story unless
      the instance still holds a long-lived tail process.
    - *Temporal history/visibility as the only durable record*: appealing
      since Temporal already records every activity boundary durably, but
      Gimbal's event model (turns, transcript deltas, usage, steers) is
      far finer-grained than an activity boundary — this would mean either
      cramming the full run log into Temporal payloads (bad for history
      size) or still shipping the fine-grained stream elsewhere and using
      Temporal only for the coarse "which turn is running" skeleton.

    Given Gimbal's existing SSE/SSR design and its "run log is the durable
    record" principle, **worker → HTTP → shared instance, with the instance
    itself scaled/HA'd normally (not per-project), is the smallest change
    from today** and keeps Temporal in its own role rather than also being
    the log transport. This points to **one shared Gimbal web instance for
    the whole EKS install, not one per project namespace** — per-project
    isolation lives at the namespace/secrets/network-policy layer, while
    the observability plane stays a single multi-tenant service workers
    reach over a namespace-restricted NetworkPolicy allow-rule. **[I]**,
    architectural judgment consistent with, but not dictated by, finding 24.

## Minimum configuration surface

| A project must configure... | Where it could live | Is a UI unavoidable? |
| --- | --- | --- |
| Which repo(s) Gimbal can access | GitHub App install screen (repo picker) | No — GitHub's own installation UI covers this (finding 4) |
| Learning that repo access changed | Webhook (`installation`, `installation_repositories`) into the shared instance | No |
| Clone/push/PR credentials | Minted server-side from the installation token, refreshed by a worker-side credential helper | No |
| Which harness(es)/versions a project uses | Base image tag (pinned harness versions) + optional per-repo setup script/`AGENTS.md`-equivalent | No by default; yes only for a non-default harness set |
| Project-specific toolchain/deps | Repo file (Dockerfile layer or setup script), built into the project image layer on a schedule/webhook | No |
| Harness API keys (OpenAI, Anthropic, Diffusion Router) | AWS Secrets Manager entry, IAM-scoped to the project namespace, synced by ESO | Only to *enter* the value once, not a Gimbal-specific settings page |
| GitHub App private key | AWS Secrets Manager, one org-wide secret | No, one-time platform setup |
| Namespace/quota/network policy per project | Generated from a template at onboarding, triggered by the install webhook | No |
| Node/spot policy for worker pods | Karpenter `NodePool` selection via pod labels/tolerations, platform-decided | No |
| Task queue name | Convention derived from repo/installation id | No |
| Network egress the harness needs (registries, package indexes) | NetworkPolicy egress rules per project, ideally inferred from a declared list in a repo file (mirrors Claude Code's Trusted/Custom/Full modes) | Possibly yes for "Custom" cases — the one place prior art still shows a real settings surface |
| Temporal Cloud namespace / connectivity | Platform-level Terraform/Helm values | No |
| Temporal Cloud vs. self-hosted | Platform decision, one-time | No |

Nearly every row collapses into GitHub App install-time state, one or two
repo-resident files, and platform-level Terraform/Helm. The one plausible
exception is **egress network policy**, where prior art (Claude Code's
network modes) suggests some projects need a person to say "this build
also needs registry.foo.bar" — no fully-automatic inference for that is
documented anywhere.

## Risks

- **Token-refresh bugs are the likeliest cause of a silent mid-run
  failure.** A worker assuming a token lasts the whole session fails right
  at the one-hour mark — for a 40+ minute activity, close enough to
  intermittent to be nasty if the refresh path isn't load-tested.
- **KEDA + Temporal Cloud + API keys has an open, unresolved bug**
  (kedacore/keda#6703). If Temporal Cloud and KEDA backlog scaling are both
  chosen, smoke-test this combination or use mTLS for the scaler.
- **Kata/gVisor adds real per-pod-start latency** (100s of ms to seconds).
  Acceptable if paid once per session (one pod per session, reused across
  turns) rather than once per turn — measure, don't assume.
- **EFS latency under heavy small-file git/toolchain workloads** is
  unquantified today for the shared-clone-plus-worktrees design; benchmark
  against actual repo size and toolchain (Go module cache, node_modules)
  before committing over EBS+node-affinity.
- **Idle-namespace cost is real** (finding 12); namespace-per-project needs
  an idle-teardown or scale-to-zero policy from day one, not as a later
  optimization.
- **The Karpenter version fact-check failed today** (finding 14) — never
  copy a version number out of this document into IaC without re-checking
  the releases page by hand.
- **Kaniko/BuildKit/Buildah claims here are all from secondary sources**
  (finding 17); confirm kaniko's actual project status directly before it
  affects a build-pipeline decision.

## Open questions for Tyler

1. Is a single shared Gimbal web instance (finding 25) acceptable, or does
   per-project data isolation require a per-project instance after all?
   Changes the network-policy and RBAC design materially.
2. Temporal Cloud vs. self-hosted (finding 23) — this research leans Cloud
   for lower operational load, but that's a cost/control tradeoff (data
   residency, per-action pricing, willingness to run Postgres/Elasticsearch
   in-cluster) only Tyler can weigh.
3. Is namespace-per-project the right isolation unit, or does Gimbal expect
   few enough concurrent projects that one namespace with task-queue/label
   separation (finding 13) is simpler and idle-cost (finding 12) never
   becomes a real problem? Decides whether a namespace-provisioning
   controller gets built at all.
4. Does any Gimbal workflow run code untrusted enough (arbitrary
   contributor-authored code, not just Tyler's own repos) to justify
   Kata/gVisor's overhead, or is the threat model "Tyler's own projects, run
   by Tyler's own harnesses" — in which case plain `runc` may be fine and
   this sandboxing research can be shelved?
5. Should the "project setup script" surface (findings 7, 16) be a new
   Gimbal-specific file, or should Gimbal just read `AGENTS.md` itself, the
   way Codex and Claude Code already do, so a project writes its
   build/toolchain intent once for every harness?

## Sources

All fetched or searched 2026-09-27 unless noted.

- GitHub Docs, "Authenticating as a GitHub App installation" — https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/authenticating-as-a-github-app-installation
- GitHub Docs, "Generating an installation access token for a GitHub App" (Enterprise Cloud) — https://docs.github.com/en/enterprise-cloud@latest/apps/creating-github-apps/authenticating-with-a-github-app/generating-an-installation-access-token-for-a-github-app
- GitHub Changelog, "GitHub App installation tokens: Per-request override header" (2026-05-15) — https://github.blog/changelog/2026-05-15-github-app-installation-tokens-per-request-override-header/
- GitHub Community Discussion #173881, "Git clone with Github App installation token accepts any username instead of required x-access-token" — https://github.com/orgs/community/discussions/173881
- GitHub Docs, "Webhook events and payloads" — https://docs.github.com/en/webhooks/webhook-events-and-payloads
- GitHub Community Discussion #24379, "installation.created vs. installation_repositories.added" — https://github.com/orgs/community/discussions/24379
- GitHub Docs, "Rate limits for GitHub Apps" — https://docs.github.com/en/apps/creating-github-apps/registering-a-github-app/rate-limits-for-github-apps
- GitHub Docs, "Installing a GitHub App from a third party" — https://docs.github.com/en/apps/using-github-apps/installing-a-github-app-from-a-third-party
- GitHub Docs, "Reviewing and modifying installed GitHub Apps" — https://docs.github.com/en/apps/using-github-apps/reviewing-and-modifying-installed-github-apps
- GitHub Docs, "Authorizing GitHub Apps" — https://docs.github.com/en/apps/using-github-apps/authorizing-github-apps
- OpenAI, "Cloud environments" (Codex) — https://developers.openai.com/codex/cloud/environments
- Claude Code Docs, "Get started with Claude Code in the cloud" — https://code.claude.com/docs/en/web-quickstart
- Claude Help Center, "Claude Code on the web" — https://support.claude.com/en/articles/12618689-claude-code-on-the-web
- Cursor Docs, "Cloud Agents" — https://cursor.com/docs/cloud-agent.md
- Cursor Docs, "Cloud Environment Setup" — https://cursor.com/docs/cloud-agent/setup
- Cursor Docs, "GitHub" — https://cursor.com/docs/integrations/github
- Google, Jules docs index — https://jules.google/docs/
- Google Blog, "Jules: Google's autonomous AI coding agent" — https://blog.google/innovation-and-ai/models-and-research/google-labs/jules/
- Devin Docs, "GitHub integration" — https://docs.devin.ai/integrations/gh
- fast.io, "Devin AI Security: Data, Secrets, and Admin Controls" (secondary) — https://fast.io/resources/devin-ai-security-guide/
- fast.io, "How to Connect Devin AI to GitHub" (secondary) — https://fast.io/resources/devin-ai-github-integration/
- AWS EKS User Guide, "Learn how EKS Pod Identity grants pods access to AWS services" — https://docs.aws.amazon.com/eks/latest/userguide/pod-identities.html
- External Secrets Operator docs, "AWS Secrets Manager" provider — https://external-secrets.io/latest/provider/aws-secrets-manager/
- External Secrets Operator docs, "AWS Access" — https://external-secrets.io/latest/provider/aws-access/
- eksworkshop.com, "External Secrets Operator" fastpath — https://www.eksworkshop.com/docs/fastpaths/operator/secrets-manager/external-secrets
- AWS Security Blog, "Making sense of secrets management on Amazon EKS for regulated institutions" (title/summary only) — https://aws.amazon.com/blogs/security/making-sense-of-secrets-management-on-amazon-eks-for-regulated-institutions
- Kubernetes SIGs `agent-sandbox`, "Kata Containers" use case — https://agent-sandbox.sigs.k8s.io/docs/use-cases/examples/kata-containers/
- vCluster blog, "AWS EKS Multi-tenancy with vCluster" (vendor source) — https://www.vcluster.com/blog/aws-eks-multi-tenancy-with-vcluster
- kubernetes-sigs `aws-efs-csi-driver`, "multiple_pods" example — https://github.com/kubernetes-sigs/aws-efs-csi-driver/blob/master/examples/kubernetes/multiple_pods/README.md
- Temporal Blog, "Announcing KEDA-based auto-scaling for Temporal Workers" — https://temporal.io/blog/announcing-keda-based-auto-scaling-for-temporal-workers
- KEDA Docs, "Temporal" scaler (v2.20) — https://keda.sh/docs/2.20/scalers/temporal/
- kedacore/keda Issue #6703, Temporal scaler vs. Temporal Cloud API-key TLS bug — https://github.com/kedacore/keda/issues/6703
- Temporal Docs, "Worker Shutdown Behavior" — https://docs.temporal.io/encyclopedia/workers/worker-shutdown
- Temporal Docs, "Worker deployment and performance" — https://docs.temporal.io/best-practices/worker
- Temporal Docs, "Authenticate with mTLS certificates - Temporal Cloud" — https://docs.temporal.io/cloud/certificates
- Temporal Docs, "Manage API keys" — https://docs.temporal.io/cloud/api-keys
- Temporal Docs, "Security model - Temporal Cloud" — https://docs.temporal.io/cloud/security
- github.com/temporalio/helm-charts README (fetched directly) — https://github.com/temporalio/helm-charts
- Gimbal repo, `docs/web-app.md` (internal source) — read directly from the checkout.

### Secondary/blog sources (lower confidence; flagged inline above)

builder.aws.com "IRSA vs Pod Identity"; Medium/dev.to IRSA-vs-Pod-Identity
posts; devopscube.com, kubeblog.com (CSI vs ESO); oneuptime.com 2026 posts
on namespace isolation, multi-tenancy, EFS CSI; cast.ai, cloudbolt.io,
tech-insider.org, dev.to (Karpenter 2026); northflank.com, bex.co,
systemshardening.com, sailor.sh, cloudrps.com (gVisor/Kata); lucaberton.com,
alexandre-vazquez.com, devopsboys.com, codecentric.de, pandastack.ai
(kaniko/BuildKit/Buildah 2026); markaicode.com "Temporal + Kubernetes:
Production Integration Guide (2026)"; samarabbas/temporal-helm-charts
README (older fork, contradicted by the current temporalio/helm-charts
README on production-readiness wording).
