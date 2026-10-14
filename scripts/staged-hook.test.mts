// Exercise the mandatory checks against real disposable Git indexes.
import assert from 'node:assert/strict';
import { execFileSync, spawnSync } from 'node:child_process';
import { chmodSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { tmpdir } from 'node:os';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const scripts = dirname(fileURLToPath(import.meta.url));
function fixture(t: test.TestContext) {
  const repo = mkdtempSync(join(tmpdir(), 'gimbal-staged-hook-'));
  t.after(() => rmSync(repo, { recursive: true, force: true }));
  const git = (...args: string[]) => execFileSync('git', args, { cwd: repo });
  git('init', '--quiet');
  const stage = (path: string, content: string) => {
    const file = join(repo, path); mkdirSync(dirname(file), { recursive: true }); writeFileSync(file, content); git('add', '--', path); return file;
  };
  const check = (script: string, env = process.env) => spawnSync('bash', [join(scripts, script)], { cwd: repo, encoding: 'utf8', env });
  return { repo, git, stage, check };
}

test('partial staging checks the index without rewriting the worktree', t => {
  const { git, stage, check } = fixture(t);
  const file = stage('source with spaces.go', 'package p\n\nfunc f() {}\n');
  writeFileSync(file, 'package p\nfunc f(){panic(1)}\n');
  const staged = git('show', ':source with spaces.go'); const working = readFileSync(file);
  const result = check('check-staged-format.sh');
  assert.equal(result.status, 0, result.stderr);
  assert.deepEqual(readFileSync(file), working); assert.deepEqual(git('show', ':source with spaces.go'), staged);
});
test('unformatted staged source fails even when the worktree is formatted', t => {
  const { git, stage, check } = fixture(t);
  const file = stage('source with spaces.go', 'package p\nfunc f(){ }\n');
  writeFileSync(file, 'package p\n\nfunc f() {}\n');
  const result = check('check-staged-format.sh');
  assert.notEqual(result.status, 0); assert.match(result.stderr, /source with spaces.go/);
  assert.equal(git('show', ':source with spaces.go').toString(), 'package p\nfunc f(){ }\n');
});
test('web documentation formatting uses no build or frontend tools', t => {
  const { repo, stage, check } = fixture(t); stage('web/README.md', '# Documentation\n');
  const bin = join(repo, 'bin'); mkdirSync(bin);
  for (const tool of ['go', 'gofmt', 'pnpm', 'node', 'chromium']) {
    const path = join(bin, tool); writeFileSync(path, '#!/bin/sh\necho forbidden-tool >&2\nexit 99\n'); chmodSync(path, 0o755);
  }
  const result = check('check-staged-format.sh', { ...process.env, PATH: bin + ':' + process.env.PATH });
  assert.equal(result.status, 0, result.stderr);
});
test('policy rejects run artifacts and ephemeral code including executable TypeScript', t => {
  const { git, stage, check } = fixture(t);
  for (const path of ['ephemeral/probe.go', 'ephemeral/probe.mts', 'ephemeral/screenshots/screen.png']) {
    git('read-tree', '--empty'); stage(path, 'rejected content\n');
    const result = check('check-staged-content.sh'); assert.notEqual(result.status, 0); assert.ok(result.stderr.includes(path));
  }
});
test('whitespace checks reject staged trailing spaces', t => {
  const { repo, stage } = fixture(t); stage('web/README.md', 'trailing whitespace   \n');
  const result = spawnSync('git', ['diff', '--cached', '--check'], { cwd: repo, encoding: 'utf8' });
  assert.notEqual(result.status, 0); assert.match(result.stdout, /trailing whitespace/);
});
