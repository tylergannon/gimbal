# Practical user testing

`gimble run validate-product --suite-file /absolute/path/to/suite.yaml --no-web`

The fixed workflow has three tester slots (one to three workloads), one screenshot
review, and one final triage turn. Assign useful jobs, not feature checklists.
Product A is always tested through its public interface and user documentation;
testers never inspect A's source. When A works on another project B, the tester
can read B but should normally rely on A to do its job.

Before running, build/install the desired product version, prepare separate
project-B workspaces, and save issue/task text locally. The example assignments
expect `issue.md` and `change.md` in their respective workspaces. Choose unused
ports and install `playwright-cli` and its browser. The observer's `--work-dir`
should be separate from these test workspaces. Each workload can supply a
foreground `start` command with a `ready` check, or point `url` at an existing
instance. For a CLI-only product, supply the URL of a loopback terminal such as
GoTTY; its startup command can live in the workload's `start` field. Ordinary CLI
output can also accompany a browser workload's screenshots.

For Gimble workloads, launch the delegated workflow with its own `--port` and
have the tester navigate the recorded browser to that listener for live monitoring.
A separately started Gimble server is useful for history, but can retain a stale
snapshot of a run owned by another process. The owning listener also serves
`GET /api/runs/{runID}` and `/api/runs/{runID}/events`. Preserve `.gimble` when a
delegated scaffold operation copies files into the active project.

Inputs are JSON or YAML; paths resolve from the suite file. `product` names A,
`guides` lists public local usage documents, `workloads` supplies the assignments,
and `output_dir` receives a unique run directory. `timeout` defaults to one hour.
`playwright_cli` optionally overrides the browser executable. Workspaces must not
overlap; shared external services/accounts should also be isolated by the caller.

The tester role `product-operation` defaults to Luna. `product-visual-review`
defaults to Gemini Flash and opens screenshots to check readability and captions.
`product-triage` defaults to GPT-6 Astra and combines findings. Their corresponding
CLI flags can override models. The final agent checks existing issues and files
new actionable ones in `issue_repo`. Omit `issue_repo` for report-only operation.
Publishing requires authenticated `gh`; task permissions such as creating a PR in
B belong explicitly in that workload's assignment.

Results include each tester's Markdown report, ordered captioned screenshots,
measured elapsed time, and a browser video for optional human review. Flash writes
`visual-review.md`; triage writes `findings.md` with issue URLs or proposed issues.
`reports.json` points to workload reports and records execution errors. A failed
task is a useful user-testing finding. A failed agent turn or failed cleanup makes
the command unsuccessful even if other workloads produced useful reports. Check
the final command exit/run status; report files alone do not certify completion.
