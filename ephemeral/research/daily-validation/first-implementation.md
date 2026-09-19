# Product validation: first implementation draft

Prepared while Opus tests browser/terminal recording and cleanup. This is a concrete extension of the [proposal](proposal.md), not an implemented command or a settled recording design.

## Intended result

A caller gives Gimble a product target and plain-English feature definitions. The workflow starts the product if needed, operates its CLI and browser, records the actual interactions as playable video, and returns evidence-backed results for every declared feature. Gimble is the first target; the input remains usable for other products.

The first delivery should handle a small mixed CLI/browser checklist end to end. It should establish the recording and lifecycle behavior needed to expand coverage, without claiming that the full Gimble inventory has passed.

## Proposed input

JSON and YAML represent the same data. The following YAML is an illustrative draft, not a file an existing Gimble command accepts. Paths and the available port are supplied for the particular run. The caller creates an empty disposable working directory before invoking the workflow.

```yaml
product:
  name: Gimble
  workdir: /tmp/gimble-validation-target
  start: /Users/tyler/go/bin/gimble --port 43827
  ready: curl --fail --silent --output /dev/null http://127.0.0.1:43827/
  browser_url: http://127.0.0.1:43827
  cli: /Users/tyler/go/bin/gimble

output_dir: /tmp/gimble-validation-output
timeout: 15m

features:
  - id: WEB-011
    surface: browser
    exercise: >-
      Open the product page. Use the navigation to visit Conversations,
      Runs, and About, then reload the Runs page.
    expected: >-
      Each link opens its corresponding page. The Runs page remains usable
      after reload, including its empty state in this new project.

  - id: CLI-006
    surface: cli
    setup: >-
      Create an empty text file in the disposable project and choose a second
      path that does not exist.
    exercise: >-
      In the recorded terminal, use the configured executable to count tokens
      in the empty file, then attempt to count the nonexistent file. Observe
      the output and exit status of each command.
    expected: >-
      The empty file produces a count of zero and exit status zero. The missing
      file produces an error and a nonzero exit status rather than a count.
```

Optional `product.prepare` is a caller-provided shell command or script, run once before startup. For the daily latest-build use case, it prepares the isolated target build and records its revision. The workflow should not contain Gimble-specific Git/build logic. With an already running product, omit `start`; readiness and target addresses still apply. Only a product started by the workflow belongs to its cleanup responsibility.

Feature `setup`, `exercise`, and `expected` are instructions for the agent, not a new action language. The runner provides ordinary command results and browser/terminal observations; it does not parse assertions out of prose. The negative CLI behavior above is expected product behavior and should pass when observed. An intentionally wrong expectation used to test the runner itself must fail.

## Workflow and acceptance

Use the existing ordinary-Go workflow shape: prepare with RunCommand, own foreground dependencies with Service, establish readiness, iterate through features, and have an operating agent gather evidence followed by an independent validator. Keep expected outcomes fixed during execution. Preserve failed attempts rather than silently retrying until one passes.

The validator must inspect the observations: command output and exit status, visible browser state, and the recording of the interaction. Video is a required deliverable, not sufficient evidence by itself. A trace or terminal event stream can supplement the video but cannot substitute for it.

For each declared feature, report its ID, pass/fail/blocked result, a concise observed-versus-expected explanation, and paths to relevant evidence and video. Include the target revision when known. Keep the report and recordings outside Git, using ordinary local paths; an in-app media viewer is not required for the first delivery.

Overall success requires every declared feature to pass, required recordings to be usable, and owned processes to be cleaned up. Missing tools or credentials leave affected features blocked and the run incomplete. Distinguish product failures from startup, driver, recording, and validation failures so that a browser-tool error is not mislabeled a product regression. On cancellation retain completed results, mark unfinished work explicitly, and report any missing or unfinalized recording. Never claim unvisited features passed.

The recording/cleanup details remain pending Opus's pilot. In particular, Service's shutdown grace period must not be assumed sufficient for video finalization, and a terminal child may have different process-group ownership from its browser-facing server. If the pilot identifies a limitation, reflect it in this first delivery instead of hiding it behind a blanket guarantee.

## Expand from the first pair

The existing [63-family inventory](feature-inventory.md) remains the coverage checklist. Its IDs describe feature families rather than individual independent tests.

| Next coverage | Prerequisite |
|---|---|
| CLI-003/004 and WEB-001 through WEB-006: find, watch, and inspect a real run | A controlled live workflow in the disposable target project |
| WEB-007 through WEB-010: steer, answer, and cancel | A workflow that intentionally waits long enough for the second actor |
| RT-007/008 and WEB-014: service/command ownership and inspection | A maintained workflow exercising those primitives |
| Conversations, provider differences, and restart recovery | Actual provider credentials and a separately supervised target server |
| Built-in implementation/review and remaining library behavior | Disposable repositories and existing maintained workflows/package tests |

Scheduling follows working execution and meaningful coverage. A daily invocation can use an existing scheduler; this feature does not require a new scheduling system. Research-index retrieval evals belong to issue #301 and are unrelated to this product-validation implementation.
