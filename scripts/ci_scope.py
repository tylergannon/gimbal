#!/usr/bin/env python3
"""Choose checks from changed paths; publication keeps one complete main gate."""

import argparse
import json
import pathlib
import re
import subprocess


def documentation(path):
    return path.startswith(("skills/", "docs/", "ephemeral/")) or (
        "/" not in path and path.endswith(".md")
    )


def plan(paths):
    relevant = [path for path in paths if not documentation(path)]
    kind = "documentation" if not relevant else "automation"
    if any(not path.startswith(("scripts/", ".github/")) and
           path not in ("lefthook.yml", "Justfile") for path in relevant):
        kind = "application"
    packages = sorted({"./" + str(pathlib.PurePosixPath(path).parent)
                       for path in relevant if path.endswith(".go")})
    if any(path in ("go.mod", "go.sum") for path in relevant):
        packages = ["./..."]
    return {"kind": kind, "release": bool(relevant), "go_packages": packages,
            "web": any(path.startswith("web/") for path in relevant)}


def needs_web_assets(packages, root):
    if not packages:
        return False
    # Include test-only imports: a package can exercise the real web server
    # without importing it in production. -e allows the absent embed output.
    imports = subprocess.check_output(
        ["go", "list", "-deps", "-test", "-e", "-f", "{{.ImportPath}}", *packages],
        cwd=root, text=True,
    )
    return any(line.split()[0] == "github.com/tylergannon/gimbal/web"
               for line in imports.splitlines() if line.strip())


def check_skill_links(root):
    for source in (root / "skills").rglob("*.md"):
        for target in re.findall(r"\]\(([^)]+)\)", source.read_text()):
            target = target.split("#", 1)[0]
            if not target or "://" in target or target.startswith("mailto:"):
                continue
            destination = (source.parent / target).resolve()
            if not destination.is_relative_to(root.resolve()) or not destination.exists():
                raise ValueError(f"{source}: missing local link {target}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base")
    parser.add_argument("--head", default="HEAD")
    parser.add_argument("--plan", default="/tmp/gimbal-ci-scope.json")
    parser.add_argument("--run-go", action="store_true")
    parser.add_argument("--go-prerequisites", action="store_true")
    parser.add_argument("--check-links", action="store_true")
    args = parser.parse_args()
    root = pathlib.Path.cwd()
    if args.check_links:
        check_skill_links(root)
    elif args.go_prerequisites:
        packages = json.loads(pathlib.Path(args.plan).read_text())["go_packages"]
        packages = [p for p in packages if p == "./..." or pathlib.Path(p).is_dir()]
        print(f"web_assets={str(needs_web_assets(packages, root)).lower()}")
    elif args.run_go:
        packages = json.loads(pathlib.Path(args.plan).read_text())["go_packages"]
        # Deleted packages have no tests left to run. Main still checks the whole app.
        packages = [p for p in packages if p == "./..." or pathlib.Path(p).is_dir()]
        if packages:
            subprocess.run(["go", "test", "-count=1", *packages], check=True)
    else:
        if not args.base:
            parser.error("--base is required to select checks")
        paths = subprocess.check_output(
            ["git", "diff", "--name-only", "-z", args.base, args.head]
        ).decode().split("\0")
        selected = plan([p for p in paths if p])
        pathlib.Path(args.plan).write_text(json.dumps(selected))
        for key, value in selected.items():
            print(f"{key}={json.dumps(value, separators=(',', ':')) if not isinstance(value, str) else value}")


if __name__ == "__main__":
    main()
