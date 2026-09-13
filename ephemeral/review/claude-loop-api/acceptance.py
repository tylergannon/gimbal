#!/usr/bin/env python3
"""External black-box acceptance for semverbump. Usage: acceptance.py <workspace>"""
import subprocess, sys, os, tempfile
ws = sys.argv[1]
binary = os.path.join(tempfile.mkdtemp(), "semverbump")
b = subprocess.run(["go", "build", "-o", binary, "."], cwd=ws, capture_output=True, text=True)
if b.returncode != 0:
    print("build failed:\n" + b.stderr[:1500]); print("0/28 acceptance cases passed"); sys.exit(1)
OK = [
 ("1.2.3 major", "2.0.0"), ("1.2.3 minor", "1.3.0"), ("1.2.3 patch", "1.2.4"),
 ("v1.2.3 patch", "1.2.4"), ("1.2.3+build.7 patch", "1.2.4"), ("1.2.3-rc.1+meta minor", "1.3.0"),
 ("1.2.3-rc.1 release", "1.2.3"), ("1.2.3-alpha release", "1.2.3"),
 ("1.2.3-rc.1 patch", "1.2.3"), ("1.2.3-rc.1 minor", "1.3.0"), ("1.2.3-rc.1 major", "2.0.0"),
 ("1.2.0-beta.2 minor", "1.2.0"), ("2.0.0-alpha.1 major", "2.0.0"), ("1.2.0-beta.2 patch", "1.2.0"),
 ("0.0.0 patch", "0.0.1"), ("10.20.30 minor", "10.21.0"),
]
BAD = ["1.2.3 release", "1.2 patch", "01.2.3 patch", "1.2.3- patch", "1.2.3-alpha..1 patch",
       "1.2.3-01 patch", "a.b.c patch", "1.2.3 bump", "1.2.3", "", "1.2.3 patch extra", "-1.2.3 patch"]
passed = 0; total = len(OK) + len(BAD); fails = []
def run(args):
    return subprocess.run([binary] + args, capture_output=True, text=True, cwd=ws)
for spec, want in OK:
    r = run(spec.split())
    if r.returncode == 0 and r.stdout == want + "\n" and r.stderr == "":
        passed += 1
    else:
        fails.append(f"FAIL {spec!r}: want {want!r} exit 0; got exit {r.returncode} stdout {r.stdout!r} stderr {r.stderr.strip()[:120]!r}")
for spec in BAD:
    r = run(spec.split())
    if r.returncode == 2 and r.stdout == "" and r.stderr.strip() != "":
        passed += 1
    else:
        fails.append(f"FAIL {spec!r}: want exit 2, empty stdout, diagnostic on stderr; got exit {r.returncode} stdout {r.stdout!r} stderr {r.stderr.strip()[:120]!r}")
print("\n".join(fails))
print(f"{passed}/{total} acceptance cases passed")
sys.exit(0 if passed == total else 1)
