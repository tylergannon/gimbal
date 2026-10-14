"""Exercise the real bounded hook in disposable Git indexes."""

import os
import json
import importlib.util
import contextlib
import io
from pathlib import Path
import shutil
import subprocess
import tempfile
import time
import unittest
from unittest.mock import patch


ROOT = Path(__file__).resolve().parent.parent


class HookEnvironmentTest(unittest.TestCase):
    def test_explicit_warmup_preserves_normal_proxy_but_timed_checks_disable_it(self):
        spec = importlib.util.spec_from_file_location("bounded_hook", ROOT / "scripts/pre-commit.py")
        hook = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(hook)
        outputs = ['{"Action":"run","Test":"TestFixture"}\n', '# tests 12\n', 'Ran 4 tests in 0.001s\n']
        with patch.dict(os.environ, GOPROXY="https://proxy.example.invalid"), contextlib.redirect_stdout(io.StringIO()):
            with patch.object(hook, "commands", return_value=outputs) as commands:
                hook.smoke(None)
                self.assertEqual(commands.call_args.args[2]["GOPROXY"], "https://proxy.example.invalid")
                hook.smoke(time.monotonic() + 2.7)
                self.assertEqual(commands.call_args.args[2]["GOPROXY"], "off")


class FastHookTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        # Compiler preparation is explicit test setup, outside the hook budget.
        subprocess.run(["python3", "-B", "scripts/pre-commit.py", "--warm"],
                       cwd=ROOT, check=True, capture_output=True)

    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.repo = Path(self.temporary.name)
        self.git("init", "--quiet")
        for path in ("go.mod", "go.sum", "web/package.json"):
            destination = self.repo / path
            destination.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(ROOT / path, destination)
        for path in ("internal/modelalias", "internal/envfile", "internal/sessionstate",
                     "internal/pi/model", "web/src/lib/workflow-design",
                     "web/src/lib/sessionstate/fixtures"):
            shutil.copytree(ROOT / path, self.repo / path)
        for path in ("scripts/pre-commit.py", "scripts/check-staged-content.sh",
                     "scripts/check-staged-format.sh", "scripts/auto_release_test.py",
                     "scripts/prepare-release.py", "scripts/dispatch-release.py",
                     "web/tests/workflow/view-state.test.mjs"):
            destination = self.repo / path
            destination.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(ROOT / path, destination)
        self.git("add", ".")
        self.git("-c", "user.name=Hook test", "-c", "user.email=hook@example.invalid",
                 "commit", "--quiet", "-m", "Base fixture")

    def git(self, *args):
        return subprocess.run(["git", *args], cwd=self.repo, check=True,
                              capture_output=True).stdout

    def stage(self, path, content):
        destination = self.repo / path
        destination.parent.mkdir(parents=True, exist_ok=True)
        destination.write_text(content)
        self.git("add", "--", path)
        return destination

    def hook(self, env=None, args=()):
        started = time.monotonic()
        result = subprocess.run(["python3", "-B", "scripts/pre-commit.py", *args],
                                cwd=self.repo, capture_output=True, text=True,
                                env=env, timeout=15)
        return result, time.monotonic() - started

    def test_code_commit_executes_uncached_go_frontend_and_release_tests(self):
        self.stage("fixture.go", "package fixture\n")
        result, duration = self.hook()
        self.assertEqual(result.returncode, 0, result.stderr)
        if "cancelled, not passed" in result.stdout:
            self.assertIn("** *** took way too long please fix me *** **", result.stdout)
        else:
            self.assertRegex(result.stdout, r"[1-9][0-9]+ Go test/subtest runs")
            self.assertRegex(result.stdout, r"[1-9][0-9]+ frontend tests")
            self.assertIn("ran 4 tests", result.stdout)

    def test_new_failing_go_test_is_not_hidden_by_cached_results(self):
        self.stage("internal/envfile/failing_test.go",
                   'package envfile\n\nimport "testing"\n\n'
                   'func TestNewFailure(t *testing.T) { t.Fatal("new regression") }\n')
        # Exercise failure detection without turning compiler speed into a gate.
        result, duration = self.hook(args=("--warm",))
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("new regression", result.stderr)

    def test_partial_staging_rejects_worktree_that_hides_index_failure(self):
        path = self.stage("internal/envfile/failing_test.go",
                          'package envfile\n\nimport "testing"\n\n'
                          'func TestNewFailure(t *testing.T) { t.Fatal("new regression") }\n')
        path.write_text("package envfile\n")
        staged = self.git("show", ":internal/envfile/failing_test.go")
        result, _ = self.hook()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("differ from the staged index", result.stderr)
        self.assertEqual(self.git("show", ":internal/envfile/failing_test.go"), staged)
        self.assertEqual(path.read_text(), "package envfile\n")

    def test_relevant_untracked_test_source_is_rejected(self):
        self.stage("fixture.go", "package fixture\n")
        (self.repo / "internal/envfile/unstaged_test.go").write_text("package envfile\n")
        result, _ = self.hook()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("unstaged_test.go", result.stderr)

    def test_unrelated_untracked_research_does_not_block(self):
        self.stage("fixture.go", "package fixture\n")
        (self.repo / "ephemeral").mkdir()
        (self.repo / "ephemeral/research.go").write_text("research\n")
        result, _ = self.hook()
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_slow_test_warns_without_success_claim_or_rejecting_commit(self):
        self.stage("fixture.go", "package fixture\n")
        executable = self.repo / "fake-bin/go"
        executable.parent.mkdir()
        executable.write_text("#!/bin/sh\nsleep 10\n")
        executable.chmod(0o755)
        environment = dict(os.environ, PATH=str(executable.parent) + os.pathsep + os.environ["PATH"])
        result, duration = self.hook(environment)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("** *** took way too long please fix me *** **", result.stdout)
        self.assertIn("just warm-hook", result.stdout)
        self.assertIn("cancelled, not passed", result.stdout)
        self.assertNotIn("Smoke tests executed", result.stdout)
        report = json.loads((self.repo / ".git/gimbal-hook-last.json").read_text())
        self.assertTrue(report["budget_exceeded"])
        self.assertTrue(any("go test" in check for check in report["incomplete_checks"]))

    def test_documentation_commit_does_not_need_go_or_node(self):
        self.stage("README.md", "# Documentation\n")
        bin_dir = self.repo / "fake-bin"
        bin_dir.mkdir()
        for name in ("go", "node", "pnpm"):
            executable = bin_dir / name
            executable.write_text("#!/bin/sh\necho forbidden-tool >&2\nexit 99\n")
            executable.chmod(0o755)
        result, duration = self.hook(dict(os.environ, PATH=str(bin_dir) + os.pathsep + os.environ["PATH"]))
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("Documentation-only", result.stdout)

    def test_deadline_kills_descendant_after_launcher_exits(self):
        self.stage("fixture.go", "package fixture\n")
        executable = self.repo / "fake-bin/go"
        executable.parent.mkdir()
        executable.write_text("#!/bin/sh\nsleep 10 &\nexit 0\n")
        executable.chmod(0o755)
        result, duration = self.hook(dict(os.environ, PATH=str(executable.parent) + os.pathsep + os.environ["PATH"]))
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("** *** took way too long please fix me *** **", result.stdout)
        self.assertIn("cancelled, not passed", result.stdout)

    def test_actual_parallel_failure_is_not_waived_when_another_check_times_out(self):
        self.stage("fixture.go", "package fixture\n")
        bin_dir = self.repo / "fake-bin"
        bin_dir.mkdir()
        for name, body in (("go", "sleep 10"), ("node", "exit 7")):
            executable = bin_dir / name
            executable.write_text("#!/bin/sh\n" + body + "\n")
            executable.chmod(0o755)
        result, _ = self.hook(dict(os.environ, PATH=str(bin_dir) + os.pathsep + os.environ["PATH"]))
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("failed with exit code 7", result.stderr)

    def test_benchmark_runs_fixed_pack_without_staged_changes_and_reports_revision(self):
        output = self.repo / "timing.json"
        result = subprocess.run(["python3", "-B", "scripts/pre-commit.py", "--benchmark",
                                 "--report", str(output)], cwd=self.repo, capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        report = json.loads(output.read_text())
        if report["budget_exceeded"]:
            self.assertIn("** *** took way too long please fix me *** **", result.stdout)
        else:
            self.assertEqual(report["incomplete_checks"], [])
        self.assertEqual(report["sha"], self.git("rev-parse", "HEAD").decode().strip())
        if report["incomplete_checks"]:
            self.assertIn("cancelled, not passed", result.stdout)
        else:
            self.assertIn("Go smoke tests", report["checks"])


if __name__ == "__main__":
    unittest.main()
