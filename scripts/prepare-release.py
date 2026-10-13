#!/usr/bin/env python3
"""Atomically record a checked main revision and its next patch release tag."""

import argparse
import datetime
import os
import re
import subprocess
from pathlib import Path


def git(*args):
    return subprocess.check_output(["git", *args], text=True).strip()


def prepare(source, remote="origin"):
    source = git("rev-parse", f"{source}^{{commit}}")
    git("fetch", remote, "main", "--tags")
    tags = [tag for tag in git("tag", "--list").splitlines()
            if re.fullmatch(r"v\d+\.\d+\.\d+", tag)]
    tags.sort(key=lambda tag: tuple(map(int, tag[1:].split("."))))
    for tag in reversed(tags):
        sha = git("rev-parse", f"{tag}^{{commit}}")
        message = git("show", "-s", "--format=%B", sha)
        if sha == source or f"Release-Source: {source}" in message.splitlines():
            # An old workflow retry must not republish an older updater feed/tap.
            return (tag, sha) if tag == tags[-1] else ("", "")
    if git("rev-parse", f"{remote}/main") != source:
        return "", ""  # A later main push will check and release the newer source.
    if not tags:
        raise RuntimeError("No stable version tag exists; establish the initial version first")
    previous = tags[-1]
    subprocess.run(["git", "merge-base", "--is-ancestor", previous, source], check=True)
    major, minor, patch = map(int, previous[1:].split("."))
    tag = f"v{major}.{minor}.{patch + 1}"
    git("checkout", "--detach", source)
    if git("status", "--porcelain"):
        raise RuntimeError("Release checkout must be clean")
    changelog = Path("CHANGELOG.md")
    text = changelog.read_text()
    heading = "## [Unreleased]\n"
    start = text.index(heading) + len(heading)
    end = text.index("\n## [", start)
    entries = text[start:end].strip()
    if not entries:
        subjects = git("log", "--format=%s", f"{previous}..{source}").splitlines()
        entries = "### Changed\n\n" + "\n".join(f"- {subject}" for subject in subjects)
    today = datetime.datetime.now(datetime.timezone.utc).date().isoformat()
    text = text[:start] + f"\n## [{tag[1:]}] - {today}\n\n{entries}\n" + text[end:]
    compare = "https://github.com/tylergannon/gimbal/compare/"
    text = re.sub(r"^\[Unreleased\]: .*?$",
                  f"[Unreleased]: {compare}{tag}...main\n[{tag[1:]}]: {compare}{previous}...{tag}",
                  text, count=1, flags=re.MULTILINE)
    changelog.write_text(text)
    git("add", "CHANGELOG.md")
    git("-c", "core.hooksPath=/dev/null", "commit", "-m",
        f"chore: release {tag}\n\nRelease-Source: {source}")
    sha = git("rev-parse", "HEAD")
    git("tag", "-a", tag, "-m", f"Gimbal {tag}\n\nRelease-Source: {source}")
    pushed = subprocess.run(["git", "push", "--atomic", remote,
                             "HEAD:refs/heads/main", f"refs/tags/{tag}"], check=False)
    if pushed.returncode:
        git("tag", "-d", tag)
        git("fetch", remote, "main", "--tags")
        if git("rev-parse", f"{remote}/main") != source:
            return "", ""
        raise RuntimeError("Atomic release push failed; no release was dispatched")
    return tag, sha


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("source")
    args = parser.parse_args()
    tag, sha = prepare(args.source)
    result = f"tag={tag}\nsha={sha}\n"
    if os.environ.get("GITHUB_OUTPUT"):
        with open(os.environ["GITHUB_OUTPUT"], "a") as output:
            output.write(result)
    print(result, end="")
