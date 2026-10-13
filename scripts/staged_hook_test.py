"""Regression tests for staged-index checks; no application builds or tests."""

import os
import pathlib
import subprocess
import tempfile
import unittest


SCRIPTS = pathlib.Path(__file__).resolve().parent


class StagedHookTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.repo = pathlib.Path(self.temporary.name)
        self.git("init", "--quiet")

    def git(self, *args):
        return subprocess.run(
            ["git", *args], cwd=self.repo, check=True, capture_output=True
        ).stdout

    def stage(self, path, content):
        file = self.repo / path
        file.parent.mkdir(parents=True, exist_ok=True)
        file.write_text(content)
        self.git("add", "--", path)
        return file

    def check(self, script):
        return subprocess.run(
            ["bash", str(SCRIPTS / script)],
            cwd=self.repo,
            capture_output=True,
            text=True,
        )

    def test_partial_staging_checks_index_without_rewriting_worktree(self):
        file = self.stage("source with spaces.go", "package p\n\nfunc f() {}\n")
        file.write_text("package p\nfunc f(){panic(1)}\n")
        staged = self.git("show", ":source with spaces.go")
        working = file.read_bytes()
        result = self.check("check-staged-format.sh")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(file.read_bytes(), working)
        self.assertEqual(self.git("show", ":source with spaces.go"), staged)

    def test_unformatted_staged_file_is_rejected_even_if_worktree_is_formatted(self):
        file = self.stage("source with spaces.go", "package p\nfunc f(){ }\n")
        file.write_text("package p\n\nfunc f() {}\n")
        result = self.check("check-staged-format.sh")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("source with spaces.go", result.stderr)
        self.assertEqual(self.git("show", ":source with spaces.go"), b"package p\nfunc f(){ }\n")

    def test_web_readme_does_not_require_go_or_frontend_tools(self):
        self.stage("web/README.md", "# Documentation\n")
        # Fail loudly if the format check calls any build/test/frontend tool.
        bin_dir = self.repo / "bin"
        bin_dir.mkdir()
        for tool in ("go", "gofmt", "pnpm", "node", "chromium"):
            executable = bin_dir / tool
            executable.write_text("#!/bin/sh\necho forbidden-tool >&2\nexit 99\n")
            executable.chmod(0o755)
        result = subprocess.run(
            ["bash", str(SCRIPTS / "check-staged-format.sh")],
            cwd=self.repo,
            env=dict(os.environ, PATH=str(bin_dir) + os.pathsep + os.environ["PATH"]),
            capture_output=True,
            text=True,
        )
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_policy_retains_run_artifact_and_ephemeral_code_rejection(self):
        for path in ("ephemeral/probe.go", "ephemeral/screenshots/screen.png"):
            with self.subTest(path=path):
                self.git("read-tree", "--empty")
                self.stage(path, "rejected content\n")
                result = self.check("check-staged-content.sh")
                self.assertNotEqual(result.returncode, 0)
                self.assertIn(path, result.stderr)

    def test_whitespace_check_rejects_staged_trailing_spaces(self):
        self.stage("web/README.md", "trailing whitespace   \n")
        result = subprocess.run(
            ["git", "diff", "--cached", "--check"],
            cwd=self.repo,
            capture_output=True,
            text=True,
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("trailing whitespace", result.stdout)


if __name__ == "__main__":
    unittest.main()
