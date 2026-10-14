import pathlib
import tempfile
import unittest

from ci_scope import check_skill_links, plan


class ScopeTests(unittest.TestCase):
    def test_documentation_does_not_release_or_require_application_tools(self):
        self.assertEqual(plan(["AGENTS.md", "CHANGELOG.md", "skills/gimbal-release/SKILL.md"]),
                         {"kind": "documentation", "release": False, "go_packages": [], "web": False})

    def test_automation_only_changes_use_focused_pr_checks(self):
        result = plan(["scripts/release_timing.py", ".github/workflows/release.yml", "Justfile"])
        self.assertEqual(result["kind"], "automation")
        self.assertTrue(result["release"])
        self.assertEqual(result["go_packages"], [])

    def test_runtime_prompt_markdown_is_application_code(self):
        self.assertEqual(plan(["internal/builtin/prompt.md"])["kind"], "application")

    def test_go_and_web_changes_select_relevant_pr_tests(self):
        result = plan(["engine.go", "internal/envfile/file.go", "web/src/routes/+page.svelte"])
        self.assertEqual(result["go_packages"], ["./.", "./internal/envfile"])
        self.assertTrue(result["web"])

    def test_dependency_changes_widen_unit_test_scope(self):
        self.assertEqual(plan(["go.mod"])["go_packages"], ["./..."])

    def test_empty_release_metadata_diff_does_not_release(self):
        self.assertFalse(plan([])["release"])

    def test_missing_relative_instruction_link_is_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            source = root / "skills/example/SKILL.md"
            source.parent.mkdir(parents=True)
            source.write_text("[missing](references/missing.md)")
            with self.assertRaises(ValueError):
                check_skill_links(root)
            source.write_text("[external](https://example.com) [self](SKILL.md#section)")
            check_skill_links(root)


if __name__ == "__main__":
    unittest.main()
