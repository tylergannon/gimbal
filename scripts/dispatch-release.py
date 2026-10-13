#!/usr/bin/env python3
"""Dispatch the existing tagged release workflow once and wait for publication."""

import argparse
import json
import subprocess
import time


def gh(*args):
    return subprocess.check_output(["gh", *args], text=True).strip()


def release_run(runs, sha):
    matches = [run for run in runs if run["head_sha"] == sha
               and run["event"] in ("workflow_dispatch", "push")]
    return max(matches, key=lambda run: run["id"], default=None)


def needs_dispatch(run):
    return run is None or (run["status"] == "completed" and run["conclusion"] != "success")


def dispatch_and_wait(tag, sha, repository, timeout=7200):
    endpoint = f"repos/{repository}/actions/workflows/release.yml"
    deadline = time.monotonic() + timeout
    runs = json.loads(gh("api", f"{endpoint}/runs?per_page=100"))["workflow_runs"]
    run = release_run(runs, sha)
    if needs_dispatch(run):
        previous_id = run["id"] if run else 0
        gh("api", "--method", "POST", f"{endpoint}/dispatches",
           "-f", f"ref={tag}", "-f", f"inputs[tag]={tag}")
        run = None
        while run is None:
            if time.monotonic() >= deadline:
                raise TimeoutError("Release dispatch did not appear before timeout")
            time.sleep(15)
            runs = json.loads(gh("api", f"{endpoint}/runs?per_page=100"))["workflow_runs"]
            candidate = release_run(runs, sha)
            if candidate and candidate["id"] > previous_id:
                run = candidate
    print(f"Release: {run['html_url']}", flush=True)
    while run["status"] != "completed":
        if time.monotonic() >= deadline:
            raise TimeoutError(f"Release still running: {run['html_url']}")
        time.sleep(30)
        run = json.loads(gh("api", f"repos/{repository}/actions/runs/{run['id']}"))
    if run["conclusion"] != "success":
        raise RuntimeError(f"Release {run['conclusion']}: {run['html_url']}; rerun automatic release to retry")
    print(f"Published {tag}: {run['html_url']}", flush=True)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("tag")
    parser.add_argument("sha")
    parser.add_argument("repository")
    args = parser.parse_args()
    dispatch_and_wait(args.tag, args.sha, args.repository)
