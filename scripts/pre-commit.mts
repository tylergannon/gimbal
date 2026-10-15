#!/usr/bin/env node
// Mandatory index checks and uncached smoke tests share a three-second feedback budget.
import { spawn, execFileSync } from 'node:child_process';
import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname, extname, resolve } from 'node:path';
import { performance } from 'node:perf_hooks';
import { pathToFileURL } from 'node:url';

// Reserve 300 ms for Node, the shell launcher and lefthook startup/shutdown.
const budgetSeconds = 2.7;
const goPackages = ['./internal/modelalias', './internal/envfile', './internal/sessionstate', './internal/pi/model'];
const dispatchPackage = './internal/automation/dispatch-release';
const testInputs = [
  'go.mod', 'go.sum', 'go.work', 'go.work.sum', 'lefthook.yml',
  'scripts/*.mts', 'scripts/*.sh', 'web/package.json',
  'web/tests/workflow/view-state.test.mjs', 'web/src/lib/workflow-design',
  'web/src/lib/sessionstate/fixtures', ...goPackages.map(path => path.slice(2)), dispatchPackage.slice(2),
];
const codeSuffixes = new Set(['.go', '.sh', '.js', '.mjs', '.mts', '.ts', '.svelte', '.swift', '.yml', '.yaml', '.json', '.toml', '.sql', '.css']);
class HookFailure extends Error {}
class BudgetExceeded extends Error {}

type ChildRun = {
  command: string[];
  child: ReturnType<typeof spawn>;
  output: string;
  exited: boolean;
  closed: boolean;
  failure?: Error;
  completion: Promise<void>;
};

export async function commands(pack: string[][], deadline: number | null, env = process.env): Promise<string[]> {
  const runs: ChildRun[] = [];
  let timer: ReturnType<typeof setTimeout> | undefined;
  try {
    for (const command of pack) {
      const child = spawn(command[0], command.slice(1), { detached: true, stdio: ['ignore', 'pipe', 'pipe'], env });
      const run: ChildRun = { command, child, output: '', exited: false, closed: false, completion: Promise.resolve() };
      child.stdout!.setEncoding('utf8');
      child.stderr!.setEncoding('utf8');
      child.stdout!.on('data', data => { run.output += data; });
      child.stderr!.on('data', data => { run.output += data; });
      child.on('error', error => { run.failure = error; });
      child.on('exit', () => { run.exited = true; });
      run.completion = new Promise(done => child.on('close', () => { run.closed = true; done(); }));
      runs.push(run);
    }
    const completed = Promise.all(runs.map(run => run.completion));
    if (deadline === null) {
      await completed;
    } else {
      const expired = new Promise<never>((_, reject) => {
        timer = setTimeout(() => reject(new BudgetExceeded()), Math.max(0, deadline - performance.now()));
      });
      await Promise.race([completed, expired]);
    }
    for (const run of runs) {
      if (run.failure) throw new HookFailure(`${run.command.join(' ')} failed: ${run.failure.message}`);
      if (run.child.exitCode !== 0) throw new HookFailure(`${run.command.join(' ')} failed:\n${run.output}`);
    }
    return runs.map(run => run.output);
  } catch (error) {
    if (!(error instanceof BudgetExceeded)) throw error;
    // A real failure in another parallel task takes precedence over a timeout.
    for (const run of runs) {
      if (run.failure) throw new HookFailure(`${run.command.join(' ')} failed: ${run.failure.message}`);
      if (run.exited && run.child.exitCode !== 0) {
        throw new HookFailure(`${run.command.join(' ')} failed with exit code ${run.child.exitCode ?? run.child.signalCode}\n${run.output}`);
      }
    }
    throw new BudgetExceeded(runs.filter(run => !run.closed).map(run => run.command.join(' ')).join('\n  '));
  } finally {
    if (timer) clearTimeout(timer);
    for (const run of runs) {
      // Kill the group even if its launcher exited: descendants may retain pipes.
      if (run.child.pid !== undefined) {
        try { process.kill(-run.child.pid, 'SIGKILL'); }
        catch (error) {
          const code = (error as NodeJS.ErrnoException).code;
          if (code !== 'ESRCH' && !(code === 'EPERM' && run.exited)) throw error;
        }
      }
    }
    await Promise.all(runs.map(run => run.completion));
  }
}

async function paths(command: string[]): Promise<string[]> {
  return (await commands([command], null))[0].split('\0').filter(Boolean);
}

export function smokeEnvironment(deadline: number | null): NodeJS.ProcessEnv {
  const env: NodeJS.ProcessEnv = { ...process.env, GOTOOLCHAIN: 'local', GOWORK: 'off' };
  // Explicit warmup may fetch dependencies; timed checks never do.
  if (deadline !== null) env.GOPROXY = 'off';
  return env;
}

function goRuns(output: string): number {
  return output.split('\n').filter(line => line.startsWith('{')).map(line => JSON.parse(line))
    .filter(event => event.Action === 'run' && event.Test).length;
}

async function smoke(deadline: number | null): Promise<void> {
  const outputs = await commands([
    ['go', 'test', '-mod=readonly', '-count=1', '-vet=off', '-json', ...goPackages],
    ['node', '--test', '--test-reporter=tap', 'web/tests/workflow/view-state.test.mjs'],
    ['go', 'test', '-mod=readonly', '-count=1', '-vet=off', '-json', dispatchPackage, '-run', '^TestDispatch'],
  ], deadline, smokeEnvironment(deadline));
  const nodeRuns = outputs[1].match(/^# tests (\d+)$/m)?.[1];
  const dispatchRuns = goRuns(outputs[2]);
  if (!nodeRuns || dispatchRuns === 0) throw new HookFailure('Smoke pack did not report frontend and release dispatch test execution.');
  console.log(`Smoke tests executed: ${goRuns(outputs[0])} Go test/subtest runs; ${nodeRuns} frontend tests; release dispatch ${dispatchRuns} Go test/subtest runs.`);
}

async function main(): Promise<number> {
  let warm = false;
  let benchmark = false;
  let reportPath: string | undefined;
  for (let i = 2; i < process.argv.length; i++) {
    switch (process.argv[i]) {
      case '--warm': warm = true; break;
      case '--benchmark': benchmark = true; break;
      case '--report': reportPath = process.argv[++i]; if (!reportPath) throw new HookFailure('--report requires a path'); break;
      case '--help': console.log('Usage: bash scripts/pre-commit.sh [--warm] [--benchmark] [--report PATH]'); return 0;
      default: throw new HookFailure(`Unknown option: ${process.argv[i]}`);
    }
  }
  const started = performance.now();
  const deadline = warm ? null : started + budgetSeconds * 1000;
  const incomplete: string[] = [];
  const finished: string[] = [];
  try {
    const [major, minor] = process.versions.node.split('.').map(Number);
    if (major < 24 || (major === 24 && minor < 13)) throw new HookFailure('Commit smoke checks require Node >=24.13.0.');
    if (warm) { await smoke(deadline); return 0; }
    // Mandatory policy and index checks are never waived for elapsed time.
    const staged = await paths(['git', 'diff', '--cached', '--name-only', '-z']);
    await commands([
      ['bash', 'scripts/check-staged-content.sh'], ['git', 'diff', '--cached', '--check'],
      ['bash', 'scripts/check-staged-format.sh'],
    ], null);
    finished.push('staged-content-policy', 'staged-whitespace', 'staged-go-format');
    if (benchmark || staged.some(path => codeSuffixes.has(extname(path)) || path === 'go.mod' || path === 'go.sum')) {
      const dirty = await paths(['git', 'diff', '--name-only', '-z', '--', ...testInputs]);
      const untracked = await paths(['git', 'ls-files', '--others', '--exclude-standard', '-z', '--', ...testInputs]);
      const different = [...dirty, ...untracked];
      if (different.length) throw new HookFailure('Smoke-test inputs differ from the staged index. Stage or stash these inputs before committing; tests cannot certify different source:\n  ' + different.join('\n  '));
      await smoke(deadline);
      finished.push('Go smoke tests', 'frontend URL-state tests', 'release dispatch tests');
    } else {
      console.log('Documentation-only commit: staged content, whitespace and formatting checked.');
    }
    const elapsed = (performance.now() - started) / 1000;
    if (elapsed > budgetSeconds) console.error('WARNING: commit hook took way too long please fix me. Run `just warm-hook` outside the hook; investigate slow checks before the next commit. This commit may proceed.');
    console.log(`Commit checks completed in ${elapsed.toFixed(3)}s (three-second feedback budget).`);
    return 0;
  } catch (error) {
    if (error instanceof BudgetExceeded) {
      incomplete.push(...error.message.split('\n  ').filter(Boolean));
      console.error('WARNING: commit hook took way too long please fix me (three-second budget).');
      console.error(`Smoke checks unfinished after ${((performance.now() - started) / 1000).toFixed(3)}s; cancelled, not passed:\n  ${error.message}`);
      console.error('Commit may proceed. Run `just warm-hook` outside the hook; investigate slow checks before the next commit. CI validates the unfinished checks.');
      return 0;
    }
    console.error(`ERROR: ${(error as Error).message}`);
    return 1;
  } finally {
    if (!warm || reportPath) {
      try {
        const revision = execFileSync('git', ['rev-parse', 'HEAD'], { encoding: 'utf8' }).trim();
        const path = reportPath ?? execFileSync('git', ['rev-parse', '--git-path', 'gimbal-hook-last.json'], { encoding: 'utf8' }).trim();
        const elapsed = (performance.now() - started) / 1000;
        mkdirSync(dirname(path), { recursive: true });
        writeFileSync(path, JSON.stringify({ elapsed_seconds: elapsed, budget_exceeded: incomplete.length > 0 || elapsed > 3, incomplete_checks: incomplete, sha: revision, checks: finished }, null, 2) + '\n');
      } catch (error) { console.error(`WARNING: could not record advisory hook timing: ${(error as Error).message}`); }
    }
  }
}

if (process.argv[1] && pathToFileURL(resolve(process.argv[1])).href === import.meta.url) {
  main().then(code => { process.exitCode = code; }).catch(error => { console.error(`ERROR: ${error.message}`); process.exitCode = 1; });
}
