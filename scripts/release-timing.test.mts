import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { chmodSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';

for (const [elapsed, expected] of [[100, ''], [300, ''], [301, '300s goal'], [600, '300s goal'], [601, '600s maximum']] as const) {
  test(`release timing at ${elapsed}s warns without failing when over budget`, t => {
    const directory = mkdtempSync(join(tmpdir(), 'gimbal-release-timing-'));
    t.after(() => rmSync(directory, { recursive: true, force: true }));
    const summary = join(directory, 'summary');
    const gh = join(directory, 'gh');
    writeFileSync(gh, '#!/bin/sh\ntest "$1" = api && test "$2" = repos/owner/repo/actions/runs/123 || exit 99\nprintf "1000\\n"\n');
    chmodSync(gh, 0o755);
    const date = join(directory, 'date');
    writeFileSync(date, `#!/bin/sh\nprintf '${1000 + elapsed}\\n'\n`);
    chmodSync(date, 0o755);
    const result = spawnSync('bash', ['scripts/release-timing.sh'], { encoding: 'utf8', env: {
      ...process.env, PATH: directory + ':' + process.env.PATH,
      GITHUB_REPOSITORY: 'owner/repo', GITHUB_RUN_ID: '123', GITHUB_STEP_SUMMARY: summary,
    } });
    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, new RegExp(`elapsed: ${elapsed}s`));
    if (expected) { assert.match(result.stdout, /::warning::/); assert.ok(result.stdout.includes(expected)); }
    else assert.doesNotMatch(result.stdout, /::warning::/);
    assert.ok(readFileSync(summary, 'utf8').includes(`**${elapsed}s**`));
  });
}
test('missing release timing warns without blocking publication', t => {
  const directory = mkdtempSync(join(tmpdir(), 'gimbal-release-timing-'));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const gh = join(directory, 'gh');
  writeFileSync(gh, '#!/bin/sh\nexit 1\n'); chmodSync(gh, 0o755);
  const result = spawnSync('bash', ['scripts/release-timing.sh'], { encoding: 'utf8', env: {
    ...process.env, PATH: directory + ':' + process.env.PATH, GITHUB_REPOSITORY: 'owner/repo', GITHUB_RUN_ID: '123',
  } });
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /::warning::Could not read/);
});
