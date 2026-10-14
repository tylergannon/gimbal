#!/usr/bin/env python3
"""Mandatory index checks plus smoke tests with a three-second feedback budget."""

import argparse
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import time


# Leave 300 ms for Python and lefthook startup/shutdown.
BUDGET_SECONDS = 2.7
GO_PACKAGES = (
    "./internal/modelalias", "./internal/envfile",
    "./internal/sessionstate", "./internal/pi/model",
)
TEST_INPUTS = (
    "go.mod", "go.sum", "go.work", "go.work.sum", "lefthook.yml",
    "scripts/*.py", "scripts/*.sh", "web/package.json",
    "web/tests/workflow/view-state.test.mjs", "web/src/lib/workflow-design",
    "web/src/lib/sessionstate/fixtures",
    *(package.removeprefix("./") for package in GO_PACKAGES),
)
CODE_SUFFIXES = {".go", ".py", ".sh", ".js", ".mjs", ".ts", ".svelte", ".swift", ".yml", ".yaml", ".json", ".toml", ".sql", ".css"}


class HookFailure(Exception):
    pass


class BudgetExceeded(Exception):
    pass


def stop(process):
    # A launcher may exit while a descendant still owns its stdout pipe.
    # Kill the group even in that case so deadline cleanup cannot wait forever.
    try:
        os.killpg(process.pid, signal.SIGKILL)
    except ProcessLookupError:
        pass
    except PermissionError:
        # macOS can report EPERM for an already reaped process group.
        if process.poll() is None:
            raise
    if process.stdout is not None and not process.stdout.closed:
        process.communicate()
    else:
        process.wait()


def commands(commands, deadline, env=None):
    """Parallel children share one deadline, including their descendant processes."""
    processes = []
    try:
        for command in commands:
            processes.append((command, subprocess.Popen(
                command, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                text=True, start_new_session=True, env=env,
            )))
        outputs = []
        for command, process in processes:
            remaining = None if deadline is None else deadline - time.monotonic()
            if remaining is not None and remaining <= 0:
                raise subprocess.TimeoutExpired(command, BUDGET_SECONDS)
            output, _ = process.communicate(timeout=remaining)
            if process.returncode:
                raise HookFailure(f"{' '.join(command)} failed:\n{output}")
            outputs.append(output)
        return outputs
    except subprocess.TimeoutExpired as error:
        # A timeout is performance feedback, not evidence that a test passed.
        # Preserve any actual failure already observed in another parallel task.
        for command, process in processes:
            if process.poll() not in (None, 0):
                raise HookFailure(f"{' '.join(command)} failed with exit code {process.returncode}") from error
        unfinished = [" ".join(command) for command, process in processes
                      if process.poll() is None or command == error.cmd]
        raise BudgetExceeded("\n  ".join(unfinished)) from error
    finally:
        for _, process in processes:
            stop(process)


def paths(command, deadline):
    return commands([command], deadline)[0].rstrip("\0").split("\0")


def smoke(deadline):
    environment = dict(os.environ, GOTOOLCHAIN="local", GOWORK="off")
    # Explicit warmup may resolve modules using the developer's normal proxy.
    # Timed commit/benchmark checks never fetch dependencies.
    if deadline is not None:
        environment["GOPROXY"] = "off"
    outputs = commands([
        ["go", "test", "-mod=readonly", "-count=1", "-vet=off", "-json", *GO_PACKAGES],
        ["node", "--test", "--test-reporter=tap", "web/tests/workflow/view-state.test.mjs"],
        [sys.executable, "-B", "-m", "unittest", "discover", "-s", "scripts",
         "-p", "auto_release_test.py", "-k", "ReleaseDispatchTest"],
    ], deadline, environment)
    events = [json.loads(line) for line in outputs[0].splitlines() if line.startswith("{")]
    go_runs = sum(event["Action"] == "run" and "Test" in event for event in events)
    node_runs = next(line.removeprefix("# tests ") for line in outputs[1].splitlines()
                     if line.startswith("# tests "))
    python_summary = next(line for line in outputs[2].splitlines() if line.startswith("Ran "))
    print(f"Smoke tests executed: {go_runs} Go test/subtest runs; "
          f"{node_runs} frontend tests; release dispatch {python_summary.lower()}.")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--warm", action="store_true", help="Run the same smoke tests outside the commit deadline")
    parser.add_argument("--benchmark", action="store_true", help="Run the fixed smoke pack even with no staged source")
    parser.add_argument("--report", type=Path, help="Write advisory timing JSON to this path")
    args = parser.parse_args()
    started = time.monotonic()
    deadline = None if args.warm else started + BUDGET_SECONDS
    incomplete = []
    finished = []
    try:
        if args.warm:
            smoke(deadline)
            return 0
        # Content policy and index correctness are mandatory, even if slow.
        # Only smoke-test work is cancelled when feedback exceeds its budget.
        staged = paths(["git", "diff", "--cached", "--name-only", "-z"], None)
        commands([
            ["bash", "scripts/check-staged-content.sh"],
            ["git", "diff", "--cached", "--check"],
            ["bash", "scripts/check-staged-format.sh"],
        ], None)
        finished.extend(["staged-content-policy", "staged-whitespace", "staged-go-format"])
        code_change = any(Path(path).suffix in CODE_SUFFIXES or path in {"go.mod", "go.sum"}
                          for path in staged)
        if code_change or args.benchmark:
            dirty = paths(["git", "diff", "--name-only", "-z", "--", *TEST_INPUTS], None)
            untracked = paths(["git", "ls-files", "--others", "--exclude-standard", "-z",
                               "--", *TEST_INPUTS], None)
            different = [path for path in dirty + untracked if path]
            if different:
                raise HookFailure(
                    "Smoke-test inputs differ from the staged index. Stage or stash these "
                    "inputs before committing; tests cannot certify different source:\n  "
                    + "\n  ".join(different)
                )
            smoke(deadline)
            finished.extend(["Go smoke tests", "frontend URL-state tests", "release dispatch tests"])
        else:
            print("Documentation-only commit: staged content, whitespace and formatting checked.")
        elapsed = time.monotonic() - started
        if elapsed > BUDGET_SECONDS:
            print("** *** took way too long please fix me *** **")
        print(f"Commit checks completed in {elapsed:.3f}s (three-second feedback budget).")
        return 0
    except BudgetExceeded as error:
        incomplete.extend(str(error).split("\n  "))
        print("** *** took way too long please fix me *** **")
        print(f"Smoke checks unfinished after {time.monotonic() - started:.3f}s; cancelled, not passed:\n  {error}")
        print("Commit may proceed. Run `just warm-hook` outside the hook; CI validates the unfinished checks.")
        return 0
    except (HookFailure, OSError, ValueError) as error:
        print(f"ERROR: {error}", file=sys.stderr)
        return 1
    finally:
        # Performance reporting is advisory and never turns a valid commit into
        # a failure. Git metadata stays outside tracked source; no network calls.
        if not args.warm or args.report:
            try:
                revision = subprocess.run(["git", "rev-parse", "HEAD"],
                                          capture_output=True, text=True).stdout.strip()
                path = args.report
                if path is None:
                    path = Path(subprocess.check_output(
                        ["git", "rev-parse", "--git-path", "gimbal-hook-last.json"], text=True).strip())
                elapsed = time.monotonic() - started
                report = {"elapsed_seconds": elapsed, "budget_exceeded": bool(incomplete) or elapsed > 3,
                          "incomplete_checks": incomplete, "sha": revision, "checks": finished}
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text(json.dumps(report, indent=2) + "\n")
            except (OSError, subprocess.SubprocessError) as error:
                print(f"WARNING: could not record advisory hook timing: {error}", file=sys.stderr)


if __name__ == "__main__":
    sys.exit(main())
