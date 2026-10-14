#!/usr/bin/env python3
"""Share the generated source/frontend within one exact-revision release run."""
import io
import json
from pathlib import Path
import subprocess
import sys
import tarfile


def git(*args):
    return subprocess.check_output(['git', *args]).decode().strip()


def paths(*args):
    return [p for p in git(*args).split('\0') if p]


def pack(archive):
    if not Path('web/build/skgo.manifest.json').stat().st_size:
        raise ValueError('Frontend manifest is empty')
    changed = paths('diff', '--name-only', '-z', '--diff-filter=AM', 'HEAD')
    changed += paths('ls-files', '--others', '--exclude-standard', '-z')
    changed = [p for p in changed if not p.startswith(('ephemeral/', 'docs/'))]
    metadata = {'sha': git('rev-parse', 'HEAD'),
                'deleted': paths('diff', '--name-only', '-z', '--diff-filter=D', 'HEAD')}
    Path(archive).parent.mkdir(parents=True, exist_ok=True)
    with tarfile.open(archive, 'w:gz') as output:
        raw = json.dumps(metadata).encode()
        entry = tarfile.TarInfo('release-inputs.json')
        entry.size = len(raw)
        output.addfile(entry, io.BytesIO(raw))
        output.add('web/build')
        for name in sorted(set(changed)):
            output.add(name)


def restore(archive):
    with tarfile.open(archive) as source:
        metadata = json.load(source.extractfile('release-inputs.json'))
        if metadata['sha'] != git('rev-parse', 'HEAD'):
            raise ValueError('Prepared inputs belong to a different release revision')
        for name in metadata['deleted']:
            path = Path(name)
            if not path.resolve().is_relative_to(Path.cwd().resolve()):
                raise ValueError('Deleted output escapes the checkout')
            path.unlink(missing_ok=True)
        source.extractall(filter='data')
    Path('release-inputs.json').unlink()


if __name__ == '__main__':
    {'pack': pack, 'restore': restore}[sys.argv[1]](sys.argv[2])
