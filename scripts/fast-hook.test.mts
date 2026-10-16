// Exercise the actual bounded coordinator in disposable checkouts and indexes.
import assert from 'node:assert/strict';
import { execFileSync, spawnSync } from 'node:child_process';
import { chmodSync, cpSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { tmpdir } from 'node:os';
import { performance } from 'node:perf_hooks';
import { fileURLToPath } from 'node:url';
import test, { before } from 'node:test';
import { commands, smokeEnvironment } from './pre-commit.mts';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
// Keep the actual Node coordinator while test PATH stubs child tools.
const node = process.execPath;
const bash = process.platform === 'darwin' ? '/bin/bash' : 'bash';
const environment = { ...process.env };
// Nested test runners need their own reporter, not the parent IPC protocol.
delete environment.NODE_TEST_CONTEXT;
before(() => {
  const result = spawnSync(node, ['scripts/pre-commit.mts', '--warm'], { cwd: root, env: environment, encoding: 'utf8', timeout: 120000 });
  assert.equal(result.status, 0, result.stdout + result.stderr);
});
function fixture(t: test.TestContext) {
  const repo = mkdtempSync(join(tmpdir(), 'gimbal-fast-hook-'));
  t.after(() => rmSync(repo, { recursive: true, force: true }));
  const git = (...args: string[]) => execFileSync('git', args, { cwd: repo });
  git('init', '--quiet');
  for (const path of [
    'go.mod', 'go.sum', 'web/package.json', 'internal/modelalias', 'internal/envfile',
    'internal/sessionstate', 'internal/pi/model', 'internal/automation/dispatch-release',
    'web/src/lib/workflow-design', 'web/src/lib/sessionstate/fixtures',
    'lefthook.yml', 'scripts/pre-commit.mts', 'scripts/pre-commit.sh', 'scripts/check-staged-content.sh',
    'scripts/check-staged-format.sh', 'scripts/check-commit-message.sh', 'web/tests/workflow/view-state.test.mjs',
  ]) { mkdirSync(dirname(join(repo, path)), { recursive: true }); cpSync(join(root, path), join(repo, path), { recursive: true }); }
  git('add', '.'); git('-c', 'user.name=Hook test', '-c', 'user.email=hook@example.invalid', 'commit', '--quiet', '-m', 'Base fixture');
  const stage = (path: string, content: string) => {
    const file = join(repo, path); mkdirSync(dirname(file), { recursive: true }); writeFileSync(file, content); git('add', '--', path); return file;
  };
  const hook = (env = environment, args: string[] = [], shell = false) => {
    const started = performance.now();
    const childEnvironment = { ...env }; delete childEnvironment.NODE_TEST_CONTEXT;
    const result = spawnSync(shell ? bash : node, [shell ? 'scripts/pre-commit.sh' : 'scripts/pre-commit.mts', ...args], { cwd: repo, env: childEnvironment, encoding: 'utf8', timeout: 15000 });
    assert.ifError(result.error); return { ...result, duration: (performance.now() - started) / 1000 };
  };
  const fake = (tools: Record<string, string>) => {
    const bin = join(repo, 'fake-bin'); mkdirSync(bin);
    for (const [tool, body] of Object.entries(tools)) { const path = join(bin, tool); writeFileSync(path, '#!/bin/sh\n' + body + '\n'); chmodSync(path, 0o755); }
    return { ...process.env, PATH: bin + ':' + process.env.PATH };
  };
  return { repo, git, stage, hook, fake };
}

test('warmup preserves configured proxy while timed checks disable dependency fetches', () => {
  const previous = process.env.GOPROXY; process.env.GOPROXY = 'https://proxy.example.invalid';
  try { assert.equal(smokeEnvironment(null).GOPROXY, process.env.GOPROXY); assert.equal(smokeEnvironment(performance.now() + 2700).GOPROXY, 'off'); }
  finally { if (previous === undefined) delete process.env.GOPROXY; else process.env.GOPROXY = previous; }
});
test('normal Bash launcher with no arguments executes uncached Go, frontend and dispatch tests', t => {
  const { stage, hook } = fixture(t); stage('fixture.go', 'package fixture\n');
  const result = hook(environment, [], true); assert.equal(result.status, 0, result.stderr);
  if (result.stderr.includes('cancelled, not passed')) assert.match(result.stderr, /WARNING: commit hook took way too long/);
  else { assert.match(result.stdout, /[1-9]\d* Go test\/subtest runs/); assert.match(result.stdout, /[1-9]\d* frontend tests/); assert.match(result.stdout, /release dispatch [1-9]\d* Go test\/subtest runs/); }
});
test('new failing Go test is not hidden by cached results', t => {
  const { stage, hook } = fixture(t);
  stage('internal/envfile/failing_test.go', 'package envfile\n\nimport "testing"\n\nfunc TestNewFailure(t *testing.T) { t.Fatal("new regression") }\n');
  const result = hook(process.env, ['--warm']); assert.notEqual(result.status, 0); assert.match(result.stderr, /new regression/);
});
test('partial staging rejects a worktree that hides an index failure without rewriting either', t => {
  const { stage, hook, git } = fixture(t);
  const file = stage('internal/envfile/failing_test.go', 'package envfile\n\nimport "testing"\n\nfunc TestNewFailure(t *testing.T) { t.Fatal("new regression") }\n');
  writeFileSync(file, 'package envfile\n'); const staged = git('show', ':internal/envfile/failing_test.go');
  const result = hook(); assert.notEqual(result.status, 0); assert.match(result.stderr, /differ from the staged index/);
  assert.deepEqual(git('show', ':internal/envfile/failing_test.go'), staged); assert.equal(readFileSync(file, 'utf8'), 'package envfile\n');
});
test('untracked smoke source including dispatch tests is rejected', t => {
  const { repo, stage, hook } = fixture(t); stage('fixture.go', 'package fixture\n');
  writeFileSync(join(repo, 'internal/automation/dispatch-release/unstaged_test.go'), 'package main\n');
  const result = hook(); assert.notEqual(result.status, 0); assert.match(result.stderr, /unstaged_test.go/);
});
test('unstaged TypeScript coordinator is rejected', t => {
  const { repo, stage, hook } = fixture(t); stage('fixture.go', 'package fixture\n');
  writeFileSync(join(repo, 'scripts/pre-commit.mts'), readFileSync(join(repo, 'scripts/pre-commit.mts'), 'utf8') + '\n// worktree only\n');
  const result = hook(); assert.notEqual(result.status, 0); assert.match(result.stderr, /scripts\/pre-commit.mts/);
});
test('unrelated untracked research does not block a commit', t => {
  const { repo, stage, hook } = fixture(t); stage('fixture.go', 'package fixture\n');
  mkdirSync(join(repo, 'ephemeral')); writeFileSync(join(repo, 'ephemeral/research.go'), 'research\n');
  const result = hook(); assert.equal(result.status, 0, result.stderr);
});
test('slow checks warn, remain unfinished, and release their process groups', t => {
  const { repo, stage, hook, fake } = fixture(t); stage('fixture.go', 'package fixture\n');
  const result = hook(fake({ go: 'echo $$ > child.pid\nsleep 10' }));
  assert.equal(result.status, 0, result.stderr); assert.ok(result.duration < 4, String(result.duration));
  assert.match(result.stderr, /WARNING: commit hook took way too long/); assert.match(result.stderr, /just warm-hook/); assert.match(result.stderr, /cancelled, not passed/); assert.doesNotMatch(result.stdout, /Smoke tests executed/);
  const report = JSON.parse(readFileSync(join(repo, '.git/gimbal-hook-last.json'), 'utf8'));
  assert.equal(report.budget_exceeded, true); assert.ok(report.incomplete_checks.some((check: string) => check.includes('go test')));
  const pid = Number(readFileSync(join(repo, 'child.pid'), 'utf8')); assert.throws(() => process.kill(pid, 0));
});
test('documentation commit uses neither Go nor Node and writes advisory timing', t => {
  const { repo, stage, hook, fake } = fixture(t); stage('README.md', '# Documentation\n');
  const result = hook(fake({ go: 'echo forbidden-tool >&2; exit 99', node: 'echo forbidden-tool >&2; exit 99', pnpm: 'exit 99' }), [], true);
  assert.equal(result.status, 0, result.stderr); assert.match(result.stdout, /Documentation-only/); assert.doesNotMatch(result.stderr, /forbidden-tool/);
  const report = JSON.parse(readFileSync(join(repo, '.git/gimbal-hook-last.json'), 'utf8')); assert.deepEqual(report.incomplete_checks, []); assert.equal(report.checks.length, 3);
});
test('documentation mandatory failures still reject without Node', t => {
  const { stage, hook, fake } = fixture(t); stage('ephemeral/results/proof.md', 'rejected\n');
  const result = hook(fake({ node: 'exit 99' }), [], true); assert.notEqual(result.status, 0); assert.match(result.stderr, /nothing a run produced/);
});
test('source commits reject unsupported Node as an execution failure', t => {
  const { stage, hook, fake } = fixture(t); stage('fixture.go', 'package fixture\n');
  const result = hook(fake({ node: 'echo v24.12.0' }), [], true); assert.notEqual(result.status, 0); assert.match(result.stderr, /require Node >=24.13.0/); assert.doesNotMatch(result.stdout, /cancelled, not passed/);
});
test('deadline kills descendant even after its launcher exits', t => {
  const { repo, stage, hook, fake } = fixture(t); stage('fixture.go', 'package fixture\n');
  const result = hook(fake({ go: 'sleep 10 &\necho $! > descendant.pid\nexit 0' }));
  assert.equal(result.status, 0, result.stderr); assert.ok(result.duration < 4, String(result.duration)); assert.match(result.stderr, /cancelled, not passed/);
  const pid = Number(readFileSync(join(repo, 'descendant.pid'), 'utf8'));
  // Linux can retain an orphan zombie briefly; it must no longer be running.
  const state = spawnSync('ps', ['-o', 'stat=', '-p', String(pid)], { encoding: 'utf8' }).stdout.trim();
  assert.ok(state === '' || state.startsWith('Z'), state);
});
test('actual parallel failure is not waived when another check times out', t => {
  const { stage, hook, fake } = fixture(t); stage('fixture.go', 'package fixture\n');
  const result = hook(fake({ go: 'sleep 10', node: 'exit 7' })); assert.notEqual(result.status, 0); assert.match(result.stderr, /failed with exit code 7/);
});
test('missing smoke executable fails rather than becoming an advisory timeout', async () => {
  await assert.rejects(commands([['definitely-missing-gimbal-hook-command']], performance.now() + 10), /failed.*ENOENT/);
});
test('benchmark runs the fixed pack with no staged change and reports revision', t => {
  const { repo, hook, git } = fixture(t); const output = join(repo, 'timing.json');
  const result = hook(process.env, ['--benchmark', '--report', output]); assert.equal(result.status, 0, result.stderr);
  const report = JSON.parse(readFileSync(output, 'utf8')); assert.equal(report.sha, git('rev-parse', 'HEAD').toString().trim());
  if (report.incomplete_checks.length) assert.match(result.stderr, /cancelled, not passed/);
  else { assert.deepEqual(report.incomplete_checks, []); assert.ok(report.checks.includes('Go smoke tests')); }
});
test('advisory report failure does not reject a successful docs check', t => {
  const { repo, stage, hook } = fixture(t); stage('README.md', '# Docs\n');
  const path = join(repo, 'file'); writeFileSync(path, 'not a directory');
  const result = hook(process.env, ['--report', join(path, 'report.json')], true); assert.equal(result.status, 0, result.stderr); assert.match(result.stderr, /WARNING/);
});
test('real Git commit through Lefthook shows slow-hook instructions and still succeeds', t => {
  if (spawnSync('lefthook', ['version']).status !== 0) { t.skip('Lefthook is not installed'); return; }
  const { repo, git, stage, fake } = fixture(t);
  execFileSync('lefthook', ['install'], { cwd: repo, env: environment });
  stage('fixture.go', 'package fixture\n');
  const result = spawnSync('git', ['-c', 'user.name=Hook test', '-c', 'user.email=hook@example.invalid', 'commit', '-m', 'test: slow hook remains advisory'], {
    cwd: repo, env: fake({ go: 'sleep 10' }), encoding: 'utf8', timeout: 15000,
  });
  assert.ifError(result.error);
  const output = result.stdout + result.stderr;
  assert.equal(result.status, 0, output);
  assert.match(output, /WARNING: commit hook took way too long/);
  assert.match(output, /just warm-hook/);
  assert.match(output, /investigate slow checks before the next commit/);
  assert.match(output, /cancelled, not passed/);
  assert.equal(git('log', '-1', '--format=%s').toString().trim(), 'test: slow hook remains advisory');
});

test('real Git commits reject breaking declarations and accept a feature', t => {
  const { repo, git, stage } = fixture(t);
  if (spawnSync('lefthook', ['version']).status === 0) {
    execFileSync('lefthook', ['install'], { cwd: repo, env: environment });
  } else {
    // CI does not install Lefthook; exercise the same command as a real Git hook.
    const hook = join(repo, '.git/hooks/commit-msg');
    writeFileSync(hook, '#!/bin/sh\nexec bash scripts/check-commit-message.sh "$1"\n');
    chmodSync(hook, 0o755);
  }
  stage('README.md', '# Documentation\n');
  const before = git('rev-parse', 'HEAD').toString();
  for (const message of ['feat!: replace API', 'fix(api)!: replace API', 'feat: replace API\n\nBREAKING CHANGE: removed API', 'chore: replace API\n\nBREAKING-CHANGE: removed API', 'Unclassified feature']) {
    const result = spawnSync('git', ['-c', 'user.name=Hook test', '-c', 'user.email=hook@example.invalid', 'commit', '-m', message], { cwd: repo, env: environment, encoding: 'utf8' });
    assert.notEqual(result.status, 0, result.stdout + result.stderr);
    assert.match(result.stdout + result.stderr, message === 'Unclassified feature' ? /Conventional Commit/ : /pre-release.*no users.*Major version increments are forbidden/);
    assert.equal(git('rev-parse', 'HEAD').toString(), before);
  }
  git('-c', 'user.name=Hook test', '-c', 'user.email=hook@example.invalid', 'commit', '-m', 'feat: improve documentation');
  assert.equal(git('log', '-1', '--format=%s').toString().trim(), 'feat: improve documentation');
});
