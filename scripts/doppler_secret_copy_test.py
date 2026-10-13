import contextlib
import importlib.util
import io
from pathlib import Path
import unittest
from unittest.mock import patch
import urllib.error

spec = importlib.util.spec_from_file_location(
    "importer", Path(__file__).with_name("copy-secrets-to-doppler.py"))
importer = importlib.util.module_from_spec(spec)
spec.loader.exec_module(importer)


class CopyTests(unittest.TestCase):
    def setUp(self):
        self.env = {name: "synthetic-" + name for name in importer.GROUPS["release"]}
        self.env.update(DOPPLER_IMPORT_TOKEN="synthetic-token", UNRELATED="do-not-copy")
        self.before = {"DOPPLER_PROJECT": "gimbal", "DOPPLER_CONFIG": "stg",
                       "UNRELATED": "preserve-existing"}

    def test_only_allowlisted_values_and_exact_verification(self):
        self.env["TAP_DEPLOY_KEY"] = "synthetic\nmultiline\nkey\n"
        values = {n: self.env[n] for n in importer.GROUPS["release"]}
        with patch.object(importer, "request", side_effect=[self.before, {}, values]) as call:
            output = io.StringIO()
            with contextlib.redirect_stdout(output):
                importer.copy("release", self.env)
        self.assertEqual(call.call_args_list[1].args[2],
                         {"project": "gimbal", "config": "stg", "secrets": values})
        self.assertNotIn("synthetic", output.getvalue())
        self.assertIn("Verified 8 secrets", output.getvalue())

    def test_missing_source_prevents_any_request(self):
        del self.env["TAP_DEPLOY_KEY"]
        with patch.object(importer, "request") as call, self.assertRaises(RuntimeError):
            importer.copy("release", self.env)
        call.assert_not_called()

    def test_conflict_prevents_write(self):
        self.before["TAP_DEPLOY_KEY"] = "different-existing-secret"
        with patch.object(importer, "request", return_value=self.before) as call:
            with self.assertRaises(RuntimeError):
                importer.copy("release", self.env)
        self.assertEqual(call.call_count, 1)

    def test_wrong_destination_prevents_write(self):
        self.before["DOPPLER_CONFIG"] = "dev"
        with patch.object(importer, "request", return_value=self.before) as call:
            with self.assertRaises(RuntimeError):
                importer.copy("release", self.env)
        self.assertEqual(call.call_count, 1)

    def test_preparation_copies_only_tag_key(self):
        self.env["RELEASE_TAG_SSH_KEY"] = "synthetic-tag-key"
        values = {"RELEASE_TAG_SSH_KEY": "synthetic-tag-key"}
        with patch.object(importer, "request", side_effect=[self.before, {}, values]) as call:
            with contextlib.redirect_stdout(io.StringIO()):
                importer.copy("preparation", self.env)
        self.assertEqual(call.call_args_list[1].args[2]["secrets"], values)

    def test_http_error_never_exposes_response_body(self):
        error = urllib.error.HTTPError(importer.UPDATE, 403, "synthetic-secret", {},
                                       io.BytesIO(b"synthetic-secret-response"))
        with patch.object(importer.urllib.request, "urlopen", side_effect=error):
            with self.assertRaisesRegex(RuntimeError, r"^Doppler request failed: HTTP 403$"):
                importer.request("synthetic-token", importer.UPDATE, {})


if __name__ == "__main__":
    unittest.main()
