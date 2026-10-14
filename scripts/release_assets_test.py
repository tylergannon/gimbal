import importlib.util
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('assets', Path(__file__).with_name('release-assets.py'))
assets = importlib.util.module_from_spec(spec)
spec.loader.exec_module(assets)


class PreparedInputsTest(unittest.TestCase):
    def setUp(self):
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        previous = Path.cwd()
        self.addCleanup(os.chdir, previous)
        os.chdir(directory.name)
        self.git('init', '-q')
        self.git('config', 'user.email', 'test@example.invalid')
        self.git('config', 'user.name', 'Test')
        Path('.gitignore').write_text('web/build/\ndist/\n')
        Path('obsolete.go').write_text('old generated code')
        Path('binding.go').write_text('old binding')
        self.git('add', '.')
        self.git('commit', '-qm', 'source')
        Path('web/build').mkdir(parents=True)
        Path('web/build/skgo.manifest.json').write_text('{"fixture": "built frontend"}')

    def git(self, *args):
        subprocess.run(['git', *args], check=True, capture_output=True)

    def test_restores_built_frontend_new_changed_and_deleted_generated_source(self):
        Path('obsolete.go').unlink()
        Path('binding.go').write_text('fresh binding')
        Path('new.go').write_text('new generated code')
        assets.pack('dist/inputs.tar.gz')
        self.git('restore', '.')
        Path('new.go').unlink()
        Path('web/build/skgo.manifest.json').unlink()
        assets.restore('dist/inputs.tar.gz')
        self.assertEqual(Path('binding.go').read_text(), 'fresh binding')
        self.assertEqual(Path('new.go').read_text(), 'new generated code')
        self.assertFalse(Path('obsolete.go').exists())
        self.assertEqual(Path('web/build/skgo.manifest.json').read_text(), '{"fixture": "built frontend"}')

    def test_refuses_inputs_from_another_revision_before_writing(self):
        Path('binding.go').write_text('candidate binding')
        assets.pack('dist/inputs.tar.gz')
        self.git('restore', '.')
        Path('other.txt').write_text('new revision')
        self.git('add', 'other.txt')
        self.git('commit', '-qm', 'different source')
        with self.assertRaisesRegex(ValueError, 'different release revision'):
            assets.restore('dist/inputs.tar.gz')
        self.assertEqual(Path('binding.go').read_text(), 'old binding')

    def test_missing_or_empty_frontend_is_not_a_prepared_release(self):
        Path('web/build/skgo.manifest.json').write_text('')
        with self.assertRaisesRegex(ValueError, 'manifest is empty'):
            assets.pack('dist/inputs.tar.gz')


if __name__ == '__main__':
    unittest.main()
