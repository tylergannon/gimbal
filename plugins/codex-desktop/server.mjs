import fs from 'node:fs/promises';
import net from 'node:net';
import path from 'node:path';
import readline from 'node:readline';
import { Worker, defaultDirectory, desktopCall, fail, maxBytes } from './lib.mjs';

const directory = defaultDirectory();
const socketPath = path.join(directory, 'worker.sock');
let server;
let worker;
let binding;
let ownsSocket = false;
const pendingSockets = new Set();
const tool = { name: 'bind_worker', description: 'Bind the desktop worker to this owning task using executor metadata. Start its private local socket; no native app consent is granted.', inputSchema: { type: 'object', properties: {}, additionalProperties: false } };

async function bind(meta) {
  const raw = meta?.['x-codex-turn-metadata'];
  const caller = typeof raw === 'string' ? JSON.parse(raw) : raw;
  if (!caller?.thread_id || !caller?.turn_id) throw fail('missing_context', 'Codex Desktop must supply genuine thread/turn metadata.');
  if (worker) {
    if (worker.caller.thread_id !== caller.thread_id) throw fail('owner_conflict', 'This process is already bound to another task.');
    return { ownerThreadId: caller.thread_id, socket: socketPath };
  }
  if (!process.env.CODEX_APP_TOOLS_PIPE_PATH) throw fail('desktop_unavailable', 'Missing desktop pipe. Start this plugin in Codex Desktop.');
  const candidate = new Worker(directory, caller, (name, args) => desktopCall(process.env.CODEX_APP_TOOLS_PIPE_PATH, caller, name, args));
  await candidate.load();
  const listener = net.createServer(socket => {
    pendingSockets.add(socket);
    socket.setTimeout(35000, () => socket.destroy());
    socket.on('close', () => pendingSockets.delete(socket));
    socket.on('error', () => {});
    let data = Buffer.alloc(0);
    socket.on('data', async bytes => {
      data = Buffer.concat([data, bytes]);
      if (data.length > maxBytes) { socket.end(JSON.stringify({ error: { code: 'too_large', message: 'Request exceeds size limit.' } }) + '\n'); return; }
      const newline = data.indexOf(10);
      if (newline < 0) return;
      socket.pause();
      try {
        const result = await candidate.handle(JSON.parse(data.subarray(0, newline)));
        socket.end(JSON.stringify({ result }) + '\n');
      } catch (error) {
        socket.end(JSON.stringify({ error: { code: error.code || 'worker_error', message: error.message, ...error.details } }) + '\n');
      }
    });
  });
  await new Promise((resolve, reject) => {
    listener.once('error', reject);
    listener.listen(socketPath, resolve);
  }).catch(error => {
    listener.close();
    throw fail('socket_unavailable', `Cannot own ${socketPath}: ${error.message}. Never remove a live worker socket; verify its owner before clearing a stale entry.`);
  });
  ownsSocket = true;
  server = listener;
  await fs.chmod(socketPath, 0o600);
  await candidate.save();
  worker = candidate;
  return { ownerThreadId: caller.thread_id, socket: socketPath };
}

function reply(id, result, error) {
  process.stdout.write(JSON.stringify({ jsonrpc: '2.0', id, ...(error ? { error: { code: -32603, message: error.message } } : { result }) }) + '\n');
}

const input = readline.createInterface({ input: process.stdin });
input.on('line', async line => {
  let request;
  try {
    if (Buffer.byteLength(line) > maxBytes) throw Error('MCP request too large');
    request = JSON.parse(line);
    if (request.id == null) return;
    if (request.method === 'initialize') reply(request.id, { protocolVersion: '2024-11-05', capabilities: { tools: {} }, serverInfo: { name: 'gimbal-codex-desktop', version: '0.1.0' } });
    else if (request.method === 'tools/list') reply(request.id, { tools: [tool] });
    else if (request.method === 'tools/call' && request.params?.name === 'bind_worker') {
      binding ||= bind(request.params._meta).finally(() => { binding = null; });
      try { reply(request.id, { content: [{ type: 'text', text: JSON.stringify(await binding) }] }); }
      catch (error) { reply(request.id, { isError: true, content: [{ type: 'text', text: error.message }] }); }
    } else if (request.method === 'ping') reply(request.id, {});
    else reply(request.id, null, Error('Unsupported MCP method or tool'));
  } catch (error) { if (request?.id != null) reply(request.id, null, error); }
});

let closing = false;
async function close() {
  if (closing) return;
  closing = true;
  input.close();
  for (const socket of pendingSockets) socket.destroy();
  server?.close();
  if (ownsSocket) await fs.unlink(socketPath).catch(() => {});
  process.exit(0);
}
input.on('close', close);
process.on('SIGTERM', close);
process.on('SIGINT', close);
