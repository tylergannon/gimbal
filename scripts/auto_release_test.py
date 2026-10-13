import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch


def module(name):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).with_name(f"{name}.py"))
    loaded = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(loaded)
    return loaded


prepare = module("prepare-release")
dispatch = module("dispatch-release")


class ReleaseGitTest(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.root = Path(self.directory.name)
        self.before = Path.cwd()
        self.remote = self.root / "remote.git"
        self.repo = self.root / "repo"
        subprocess.run(["git", "init", "--bare", str(self.remote)], check=True, capture_output=True)
        subprocess.run(["git", "clone", str(self.remote), str(self.repo)], check=True, capture_output=True)
        os.chdir(self.repo)
        prepare.git("config", "user.name", "Release test")
        prepare.git("config", "user.email", "release@example.invalid")
        prepare.git("checkout", "-b", "main")
        Path("CHANGELOG.md").write_text(
            "# Changelog\n\n## [Unreleased]\n\n## [0.13.3] - 2026-10-12\n\nOld release.\n\n"
            "[Unreleased]: https://github.com/tylergannon/gimbal/compare/v0.13.3...main\n")
        prepare.git("add", ".")
        prepare.git("commit", "-m", "Initial release")
        prepare.git("tag", "v0.13.3")
        prepare.git("push", "origin", "main", "--tags")
        self.source = self.commit("Workspace release", entries="### Added\n\n- Native workspace.\n")

    def tearDown(self):
        os.chdir(self.before)
        self.directory.cleanup()

    def commit(self, subject, entries=None):
        if entries is not None:
            path = Path("CHANGELOG.md")
            path.write_text(path.read_text().replace("## [Unreleased]\n", "## [Unreleased]\n\n" + entries))
        Path("source.txt").write_text(subject)
        prepare.git("add", ".")
        prepare.git("commit", "-m", subject)
        prepare.git("push", "origin", "main")
        return prepare.git("rev-parse", "HEAD")

    def test_patch_changelog_atomic_push_and_source_identity(self):
        tag, sha = prepare.prepare(self.source)
        self.assertEqual(tag, "v0.13.4")
        self.assertEqual(prepare.git("ls-remote", "origin", "refs/heads/main").split()[0], sha)
        self.assertEqual(prepare.git("diff", "--name-only", self.source, sha), "CHANGELOG.md")
        self.assertIn(f"Release-Source: {self.source}", prepare.git("show", "-s", "--format=%B", sha))
        text = Path("CHANGELOG.md").read_text()
        self.assertIn("## [Unreleased]\n\n## [0.13.4] - ", text)
        self.assertEqual(text.count("- Native workspace."), 1)
        self.assertIn("[0.13.4]: https://github.com/tylergannon/gimbal/compare/v0.13.3...v0.13.4", text)

    def test_rerun_reuses_same_release_and_metadata_push_does_not_bump(self):
        first = prepare.prepare(self.source)
        self.assertEqual(prepare.prepare(self.source), first)
        self.assertEqual(prepare.prepare(first[1]), first)
        self.assertNotIn("v0.13.5", prepare.git("tag", "--list"))

    def test_old_source_retry_cannot_downgrade_newer_release(self):
        old_source = self.source
        prepare.prepare(old_source)
        prepare.git("checkout", "-B", "main", "origin/main")
        newer = self.commit("Next source")
        self.assertEqual(prepare.prepare(newer)[0], "v0.13.5")
        self.assertEqual(prepare.prepare(old_source), ("", ""))

    def test_newer_main_before_preparation_skips_stale_source(self):
        newer = self.commit("Newer workspace")
        self.assertEqual(prepare.prepare(self.source), ("", ""))
        self.assertEqual(prepare.git("ls-remote", "origin", "refs/heads/main").split()[0], newer)
        self.assertNotIn("v0.13.4", prepare.git("tag", "--list"))

    def test_main_race_rejects_atomic_push_without_publishing_tag(self):
        other = self.root / "other"
        subprocess.run(["git", "clone", "--branch", "main", str(self.remote), str(other)],
                       check=True, capture_output=True)
        real_run = subprocess.run

        def race(args, **kwargs):
            if args[:3] == ["git", "push", "--atomic"]:
                real_run(["git", "-C", str(other), "config", "user.name", "Race"], check=True)
                real_run(["git", "-C", str(other), "config", "user.email", "race@example.invalid"], check=True)
                (other / "newer.txt").write_text("newer main")
                real_run(["git", "-C", str(other), "add", "."], check=True)
                real_run(["git", "-C", str(other), "commit", "-m", "Race advance"], check=True)
                real_run(["git", "-C", str(other), "push", "origin", "main"], check=True)
            return real_run(args, **kwargs)

        with patch.object(subprocess, "run", side_effect=race):
            self.assertEqual(prepare.prepare(self.source), ("", ""))
        self.assertEqual(prepare.git("ls-remote", "origin", "refs/tags/v0.13.4"), "")
        self.assertEqual(prepare.git("show", "origin/main:newer.txt"), "newer main")

    def test_empty_unreleased_uses_actual_commit_subjects(self):
        path = Path("CHANGELOG.md")
        path.write_text(path.read_text().replace("\n### Added\n\n- Native workspace.\n", ""))
        source = self.commit("New icon")
        prepare.prepare(source)
        self.assertIn("- New icon", path.read_text())
        self.assertIn("- Workspace release", path.read_text())


class ReleaseDispatchTest(unittest.TestCase):
    def run_data(self, status="in_progress", conclusion=None, identifier=12):
        return {"id": identifier, "head_sha": "abc", "event": "workflow_dispatch",
                "status": status, "conclusion": conclusion, "html_url": "https://example.invalid/run"}

    def test_active_run_reused_and_waited_without_dispatch(self):
        calls = []
        active = self.run_data()
        complete = self.run_data("completed", "success")

        def gh(*args):
            calls.append(args)
            return json.dumps({"workflow_runs": [active]} if "/runs?" in args[1] else complete)

        with patch.object(dispatch, "gh", side_effect=gh), patch.object(dispatch.time, "sleep"):
            dispatch.dispatch_and_wait("v0.13.4", "abc", "owner/repo")
        self.assertFalse(any("POST" in call for call in calls))

    def test_successful_run_never_dispatched_again(self):
        run = self.run_data("completed", "success")
        with patch.object(dispatch, "gh", return_value=json.dumps({"workflow_runs": [run]})) as gh:
            dispatch.dispatch_and_wait("v0.13.4", "abc", "owner/repo")
        self.assertEqual(gh.call_count, 1)

    def test_failed_run_retries_same_tag_with_tag_ref(self):
        failed = self.run_data("completed", "failure")
        complete = self.run_data("completed", "success", 13)
        with patch.object(dispatch, "gh", side_effect=[json.dumps({"workflow_runs": [failed]}),
                                                       "", json.dumps({"workflow_runs": [complete]})]) as gh, \
                patch.object(dispatch.time, "sleep"):
            dispatch.dispatch_and_wait("v0.13.4", "abc", "owner/repo")
        self.assertIn("ref=v0.13.4", gh.call_args_list[1].args)
        self.assertIn("inputs[tag]=v0.13.4", gh.call_args_list[1].args)

    def test_failed_publication_is_failure_not_success(self):
        queued = self.run_data("queued")
        failed = self.run_data("completed", "failure")
        with patch.object(dispatch, "gh", side_effect=[json.dumps({"workflow_runs": [queued]}),
                                                       json.dumps(failed)]), \
                patch.object(dispatch.time, "sleep"):
            with self.assertRaisesRegex(RuntimeError, "Release failure"):
                dispatch.dispatch_and_wait("v0.13.4", "abc", "owner/repo")


if __name__ == "__main__":
    unittest.main()
