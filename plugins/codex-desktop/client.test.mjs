import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { once } from 'node:events';
import { mkdtemp, readFile, rm } from 'node:fs/promises';
import { createServer } from 'node:net';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const clientPath = fileURLToPath(new URL('./client.mjs', import.meta.url));
const launchPath = fileURLToPath(new URL('./scripts/launch', import.meta.url));

async function fixture(t, handle) {
  // Keep socket paths below macOS's Unix-domain path limit.
  const directory = await mkdtemp('/tmp/gimbal-client-');
  const sockets = new Set();
  const server = createServer((socket) => {
    sockets.add(socket);
    socket.on('close', () => sockets.delete(socket));
    socket.on('error', () => {});
    handle(socket);
  });
  server.listen(join(directory, 'worker.sock'));
  await once(server, 'listening');
  t.after(async () => {
    for (const socket of sockets) socket.destroy();
    await new Promise((resolve) => server.close(resolve));
    await rm(directory, { recursive: true, force: true });
  });
  return directory;
}

async function runClient(input, args = [], env = {}) {
  const child = spawn(process.execPath, [clientPath, ...args], {
    env: { ...process.env, ...env }, stdio: ['pipe', 'pipe', 'pipe'],
  });
  let stdout = '';
  let stderr = '';
  child.stdout.on('data', (chunk) => { stdout += chunk; });
  child.stderr.on('data', (chunk) => { stderr += chunk; });
  child.stdin.on('error', () => {});
  child.stdin.end(input);
  const [code] = await once(child, 'close');
  return { code, stdout, stderr };
}

test('sends one request and accepts a fragmented Unicode response', async (t) => {
  let received;
  const directory = await fixture(t, (socket) => {
    socket.once('data', (data) => {
      received = data.toString();
      const response = Buffer.from('{"result":{"text":"café ☀"}}\n');
      const split = response.indexOf(Buffer.from('é')) + 1;
      socket.write(response.subarray(0, split));
      setTimeout(() => socket.end(response.subarray(split)), 5);
    });
  });
  const request = { method: 'start', requestId: 'job-7', prompt: 'Read this', target: { type: 'projectless' } };
  const result = await runClient(`${JSON.stringify(request)}\n`, ['--state-dir', directory]);
  assert.equal(result.code, 0);
  assert.equal(result.stderr, '');
  assert.deepEqual(JSON.parse(received), request);
  assert.ok(received.endsWith('\n'));
  assert.deepEqual(JSON.parse(result.stdout), { result: { text: 'café ☀' } });
});

test('uses GIMBAL_DESKTOP_DIR and reports worker errors without losing their envelope', async (t) => {
  const response = { error: { code: 'approval_pending', message: 'Approve the required app manually' } };
  const directory = await fixture(t, (socket) => {
    socket.once('data', () => socket.end(`${JSON.stringify(response)}\n`));
  });
  const result = await runClient('{"method":"status"}\n', [], { GIMBAL_DESKTOP_DIR: directory });
  assert.equal(result.code, 1);
  assert.deepEqual(JSON.parse(result.stdout), response);
  assert.match(result.stderr, /approval_pending: Approve the required app manually/);
});

test('deadline fails visibly and does not reconnect or replay', async (t) => {
  let requests = 0;
  const directory = await fixture(t, (socket) => socket.on('data', () => { requests++; }));
  const result = await runClient('{"method":"status"}\n', ['--state-dir', directory, '--timeout-ms', '100']);
  assert.equal(result.code, 1);
  assert.equal(result.stdout, '');
  assert.match(result.stderr, /timed out after 100 ms/);
  assert.equal(requests, 1);
});

for (const [name, response, diagnostic] of [
  ['malformed JSON', 'hello\n', /invalid JSON/],
  ['missing envelope', '{"ready":true}\n', /invalid response envelope/],
  ['unfinished frame', '{"result":', /before a complete JSON response line/],
]) {
  test(`rejects ${name}`, async (t) => {
    const directory = await fixture(t, (socket) => socket.once('data', () => socket.end(response)));
    const result = await runClient('{"method":"status"}\n', ['--state-dir', directory]);
    assert.equal(result.code, 1);
    assert.equal(result.stdout, '');
    assert.match(result.stderr, diagnostic);
  });
}

test('socket failure is actionable', async (t) => {
  const directory = await mkdtemp('/tmp/gimbal-client-');
  t.after(() => rm(directory, { recursive: true, force: true }));
  const result = await runClient('{"method":"status"}\n', ['--state-dir', directory]);
  assert.equal(result.code, 1);
  assert.match(result.stderr, /worker\.sock.*ENOENT/);
});

test('rejects malformed requests and invalid timeouts before connecting', async () => {
  for (const [input, args, message] of [
    ['broken\n', [], /JSON request line/],
    ['', [], /No JSON request/],
    ['{"method":"erase"}\n', [], /Request method/],
    ['{"method":"status"}\n', ['--timeout-ms', '0'], /positive integer/],
  ]) {
    const result = await runClient(input, args);
    assert.equal(result.code, 1);
    assert.equal(result.stdout, '');
    assert.match(result.stderr, message);
  }
});

test('launcher fails visibly when Codex did not supply its runtime', async () => {
  const child = spawn(launchPath, [], { env: { ...process.env, CODEX_MCP_NODE_PATH: '' } });
  let stderr = '';
  child.stderr.on('data', (chunk) => { stderr += chunk; });
  const [code] = await once(child, 'close');
  assert.equal(code, 127);
  assert.match(stderr, /needs CODEX_MCP_NODE_PATH/);
});

test('shipped launcher initializes MCP from another working directory using the supplied runtime', async () => {
  const child = spawn(launchPath, [], {
    cwd: '/tmp',
    env: { ...process.env, CODEX_MCP_NODE_PATH: process.execPath, CODEX_APP_TOOLS_PIPE_PATH: '' },
  });
  let stdout = '';
  let stderr = '';
  child.stdout.on('data', (chunk) => { stdout += chunk; });
  child.stderr.on('data', (chunk) => { stderr += chunk; });
  child.stdin.end([
    JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'initialize', params: { protocolVersion: '2024-11-05', capabilities: {}, clientInfo: { name: 'launcher-test', version: '1' } } }),
    JSON.stringify({ jsonrpc: '2.0', id: 2, method: 'tools/list' }),
    '',
  ].join('\n'));
  const [code] = await once(child, 'close');
  assert.equal(code, 0, stderr);
  const responses = stdout.trim().split('\n').map(line => JSON.parse(line));
  assert.equal(responses.find(response => response.id === 1).result.serverInfo.name, 'gimbal-codex-desktop');
  assert.deepEqual(responses.find(response => response.id === 2).result.tools.map(tool => tool.name), ['bind_worker']);
});

test('plugin policy approves only binding and passes the required environment', async () => {
  const config = JSON.parse(await readFile(new URL('./.mcp.json', import.meta.url), 'utf8'));
  const server = config.mcpServers.gimbal_desktop;
  assert.equal(server.command, './scripts/launch');
  assert.equal(server.default_tools_approval_mode, 'prompt');
  assert.deepEqual(server.tools, { bind_worker: { approval_mode: 'approve' } });
  assert.deepEqual(server.env_vars, ['CODEX_APP_TOOLS_PIPE_PATH', 'CODEX_MCP_NODE_PATH', 'HOME', 'GIMBAL_DESKTOP_DIR']);
});
