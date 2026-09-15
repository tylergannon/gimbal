"""Attest native update/audit against intentionally changed local fixtures."""
import hashlib
import json
from pathlib import Path
import subprocess

root = Path(__file__).resolve().parent
repo = root.parents[2]
cache, index = root / 'cache', root / 'index'

def hashes(path):
    return {str(f.relative_to(path)): hashlib.sha256(f.read_bytes()).hexdigest()
            for f in path.rglob('*') if f.is_file()}

def invoke(mode, expected):
    result = subprocess.run([str(repo / 'bin/gimble'), 'index', '-from', str(cache),
                             '-to', str(index), '-repo', str(root/'project'),
                             '-mode', mode, '-no-web'], capture_output=True, text=True)
    (root / ('index-' + mode + ('-stale' if expected else '-clean') + '.txt')).write_text(result.stdout + result.stderr)
    assert (result.returncode != 0) == bool(expected), (mode, result.returncode, result.stderr)
    return result.stdout + result.stderr

before = hashes(index)
state = json.loads((index / '.semantic-index/state.json').read_text())
changed, deleted = 'operations/backups.md', 'billing/currency.md'
source = cache / changed
text = source.read_text()
assert '20' in text
source.write_text(text.replace('20', '24'))
(cache / deleted).unlink()
source_expected = hashes(cache)
(root/'source-intended-update.json').write_text(json.dumps(source_expected, indent=2)+'\n')
invoke('audit', 1)
assert hashes(index) == before, 'stale audit changed the index'
output = invoke('update', 0)
assert output.count('turn started') == 1, output
assert hashes(cache) == source_expected, 'update changed source corpus'
after = hashes(index)
for path, entry in state['sources'].items():
    if path not in (changed, deleted):
        assert before[entry['leaf']] == after[entry['leaf']], path
assert not (index / state['sources'][deleted]['leaf']).exists(), 'deleted source leaf survived'
assert before[state['sources'][changed]['leaf']] != after[state['sources'][changed]['leaf']], 'changed leaf not refreshed'
output = invoke('audit', 0)
assert 'turn started' not in output
assert hashes(index) == after, 'clean audit changed the index'
assert hashes(cache) == source_expected
(root/'incremental-result.json').write_text(json.dumps({
    'changed_source': changed, 'deleted_source': deleted,
    'model_reads_during_update': 1, 'unchanged_leaves_preserved': 10,
    'stale_audit_failed_without_writes': True, 'clean_audit_passed_without_writes': True,
    'source_preserved': True}, indent=2)+'\n')
print('PASS: stale audit, one-reader update, deleted leaf removed, ten leaves unchanged, clean audit, source preservation')
