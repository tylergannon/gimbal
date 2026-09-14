#!/usr/bin/env python3
"""Independent end-to-end acceptance probe; uses the actual distributed CLI."""

import json
import hashlib
from pathlib import Path
import subprocess
import sys
import tempfile

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
BINARY = Path(sys.argv[1]).resolve()
BAD = "./ephemeral/issue-162/validation/testdata/dispatch"
GOOD = "./ephemeral/issue-162/validation/testdata/explicit"
NAMED = "./ephemeral/issue-162/validation/testdata/named"
ID = "[GIMBLE101-SIMPLE-WORKFLOWS/NO-DYNAMIC-WORKERS]"
results = []


def check(name, args, succeeds, contains=None, count=None):
    process = subprocess.run(args, cwd=ROOT, text=True, capture_output=True)
    output = process.stdout + process.stderr
    passed = (process.returncode == 0) == succeeds
    if contains is not None:
        passed = passed and contains in output
    if count is not None:
        passed = passed and output.count(contains) == count
    result = {
        "name": name,
        "argv": args,
        "exit": process.returncode,
        "stdout": process.stdout,
        "stderr": process.stderr,
        "passed": passed,
    }
    results.append(result)
    print(f"{'PASS' if passed else 'FAIL'} {name} (exit {process.returncode})", flush=True)


check("map dispatch runs without lint", ["go", "run", BAD], True, "worker ran", 2)
check("explicit dispatch runs without lint", ["go", "run", GOOD], True, "worker ran", 2)
check("standalone rejects direct lookup and helper alias", [str(BINARY), "lint", BAD], False, ID, 2)
check("standalone accepts switch, helpers, group callbacks, and known alias", [str(BINARY), "lint", GOOD], True)
check("vet protocol rejects direct lookup and helper alias", ["go", "vet", f"-vettool={BINARY}", BAD], False, ID, 2)
check("vet protocol accepts explicit dispatch", ["go", "vet", f"-vettool={BINARY}", GOOD], True)
check("normal CLI help still works", [str(BINARY), "-h"], True, "-port")
check("named workflow callback runs without lint", ["go", "run", NAMED], True)
check("named workflow callback is analyzed", [str(BINARY), "lint", NAMED], False, ID, 1)
check("prompt ending in cfg retains normal CLI routing", [str(BINARY), "run-prompt", "-h", "settings.cfg"], True, "-model")
check("socket ending in cfg retains normal CLI routing", [str(BINARY), "-h", "-uds", "/tmp/gimble-162.cfg"], True, "-port")
check("generic writes and captured outer loop contexts detect duplicates", [str(BINARY), "lint", "./ephemeral/issue-162/validation/testdata/setchecks"], False, "[GIMBLE103-SET-MISUSE/DUPLICATE-KEY]", 3)
check("mutually exclusive writes and fresh task contexts pass", [str(BINARY), "lint", "./ephemeral/issue-162/validation/testdata/fresh"], True)
check("helper without context parameter runs without lint", ["go", "run", "./ephemeral/issue-162/validation/testdata/helper"], True)
check("direct helper reachability does not depend on its parameter types", [str(BINARY), "lint", "./ephemeral/issue-162/validation/testdata/helper"], False, ID, 1)
with tempfile.TemporaryDirectory(prefix="gimble-162-routing-") as directory:
    config = Path(directory) / "vet.cfg"
    config.write_text(json.dumps({"ImportPath": "example.com/probe", "GoFiles": []}))
    check("run-prompt wins even when argument names a vet config", [str(BINARY), "run-prompt", "-h", str(config)], True, "-model")
    check("server help wins even when socket argument names a vet config", [str(BINARY), "-h", "-uds", str(config)], True, "-port")

HERE.joinpath("results.json").write_text(json.dumps(results, indent=2) + "\n")
metadata = {
    "head": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip(),
    "binary": str(BINARY),
    "binary_bytes": BINARY.stat().st_size,
    "binary_sha256": hashlib.sha256(BINARY.read_bytes()).hexdigest(),
    "build_info": subprocess.check_output(["go", "version", "-m", str(BINARY)], text=True),
    "source_diff": subprocess.check_output(["git", "diff", "HEAD", "--", "cmd", "internal/gimblelint", "go.mod", "go.sum", "Justfile"], cwd=ROOT, text=True),
}
HERE.joinpath("metadata.json").write_text(json.dumps(metadata, indent=2) + "\n")
sys.exit(0 if all(result["passed"] for result in results) else 1)
