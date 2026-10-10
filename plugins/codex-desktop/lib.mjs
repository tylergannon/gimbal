import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import net from 'node:net';
import { randomUUID, createHash } from 'node:crypto';

export const maxBytes = 4 * 1024 * 1024;
export const defaultDirectory = () => process.env.GIMBAL_DESKTOP_DIR || path.join(os.homedir(), '.gimbal', 'codex-desktop');

export function fail(code, message, details = {}) {
  return Object.assign(new Error(message), { code, details });
}

export async function appReadiness(requiredApps = [], home = os.homedir()) {
  if (!Array.isArray(requiredApps) || requiredApps.some(x => typeof x !== 'string' || !x.trim())) {
    throw fail('invalid_request', 'requiredApps must contain native application bundle identifiers.');
  }
  if (!requiredApps.length) return { approved: [], missing: [] };
  const file = path.join(home, 'Library', 'Group Containers', '2DC432GLL2.com.openai.sky.CUAService', 'Library', 'Application Support', 'Software', 'ComputerUseAppApprovals.json');
  let approved = [];
  try {
    const data = JSON.parse(await fs.readFile(file, 'utf8'));
    if (!Array.isArray(data.approvedBundleIdentifiers)) throw Error('Unrecognized native approval store');
    approved = data.approvedBundleIdentifiers;
  } catch (error) {
    if (error.code !== 'ENOENT') throw fail('readiness_unavailable', `Cannot read native app approvals: ${error.message}`);
  }
  return { approved: requiredApps.filter(x => approved.includes(x)), missing: requiredApps.filter(x => !approved.includes(x)) };
}

// This is the installed desktop's private bridge, not public app-server RPC.
export function desktopCall(pipe, caller, tool, args, timeoutMs = 30000) {
  return new Promise((resolve, reject) => {
    if (!pipe) return reject(fail('desktop_unavailable', 'Desktop did not supply CODEX_APP_TOOLS_PIPE_PATH. Start this MCP server in Codex Desktop.'));
    const socket = net.createConnection(pipe);
    let buffer = Buffer.alloc(0);
    const timer = setTimeout(() => finish(fail('outcome_unknown', `Desktop ${tool} response timed out; do not replay a creation request.`)), timeoutMs);
    let settled = false;
    function finish(error, value) {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      socket.destroy();
      error ? reject(error) : resolve(value);
    }
    socket.on('connect', () => {
      const data = Buffer.from(JSON.stringify({ id: 1, jsonrpc: '2.0', method: 'tools/call', params: {
        namespace: 'codex_app', callerSource: 'codex', threadId: caller.thread_id,
        turnId: caller.turn_id, callId: `gimbal-${randomUUID()}`, tool, arguments: args,
      } }));
      const header = Buffer.alloc(4);
      header.writeUInt32LE(data.length);
      socket.write(Buffer.concat([header, data]));
    });
    socket.on('error', error => finish(fail('outcome_unknown', `Desktop connection failed: ${error.message}`)));
    socket.on('close', () => finish(fail('outcome_unknown', 'Desktop connection closed before its reply.')));
    socket.on('data', bytes => {
      buffer = Buffer.concat([buffer, bytes]);
      if (buffer.length < 4) return;
      const length = buffer.readUInt32LE(0);
      if (length > maxBytes) return finish(fail('protocol_error', 'Desktop reply exceeds the size limit.'));
      if (buffer.length < length + 4) return;
      try {
        const reply = JSON.parse(buffer.subarray(4, length + 4));
        if (reply.error) throw fail('desktop_rejected', reply.error.message);
        const result = reply.result;
        const text = result?.contentItems?.filter(x => x.type === 'inputText').map(x => x.text).join('\n');
        if (!result?.success) throw fail('desktop_rejected', text || 'Desktop rejected the operation.');
        finish(null, JSON.parse(text));
      } catch (error) { finish(error); }
    });
  });
}

export class Worker {
  constructor(directory, caller, call, readiness = appReadiness) {
    this.directory = directory;
    this.caller = caller;
    this.call = call;
    this.readiness = readiness;
    this.ledger = { version: 1, ownerThreadId: caller.thread_id, requests: {} };
    this.tail = Promise.resolve();
  }
  async load() {
    await fs.mkdir(this.directory, { recursive: true, mode: 0o700 });
    await fs.chmod(this.directory, 0o700);
    try {
      this.ledger = JSON.parse(await fs.readFile(path.join(this.directory, 'sessions.json'), 'utf8'));
      if (this.ledger.version !== 1 || !this.ledger.requests) throw Error('Unrecognized session ledger');
      if (this.ledger.ownerThreadId !== this.caller.thread_id) throw Error(`Worker belongs to desktop task ${this.ledger.ownerThreadId}; resume that task to recover its sessions.`);
    } catch (error) { if (error.code !== 'ENOENT') throw error; }
  }
  async save() {
    const file = path.join(this.directory, 'sessions.json');
    const temporary = `${file}.${process.pid}.tmp`;
    const handle = await fs.open(temporary, 'w', 0o600);
    try { await handle.writeFile(JSON.stringify(this.ledger, null, 2)); await handle.sync(); }
    finally { await handle.close(); }
    await fs.rename(temporary, file);
  }
  handle(request) {
    // Serialize ownership mutations so simultaneous retries cannot start twice.
    const result = this.tail.then(() => this.execute(request));
    this.tail = result.catch(() => {});
    return result;
  }
  owned(threadId) {
    const entry = Object.values(this.ledger.requests).find(x => x.threadId === threadId);
    if (!entry) throw fail('not_owned', 'This worker does not own that desktop task.');
    return entry;
  }
  async execute(request) {
    if (!request || typeof request !== 'object') throw fail('invalid_request', 'Expected a JSON request.');
    const { method } = request;
    if (method === 'status') {
      const apps = await this.readiness(request.requiredApps);
      return { ownerThreadId: this.caller.thread_id, nativeApps: apps, ready: apps.missing.length === 0,
        scope: 'Desktop binding and saved native app approval only; OS permissions and sensitive-action policy still apply.',
        requests: this.ledger.requests };
    }
    if (method === 'start') {
      const { requestId, prompt, target } = request;
      if (typeof requestId !== 'string' || !requestId.trim() || typeof prompt !== 'string' || !prompt.trim() || !target) {
        throw fail('invalid_request', 'start requires requestId, prompt and a desktop create_thread target.');
      }
      const model = request.model || 'gpt-5.6-luna';
      const input = { prompt, target, model, ...(request.thinking ? { thinking: request.thinking } : {}) };
      const fingerprint = createHash('sha256').update(JSON.stringify({ input, requiredApps: request.requiredApps || [] })).digest('hex');
      const previous = Object.hasOwn(this.ledger.requests, requestId) ? this.ledger.requests[requestId] : undefined;
      if (previous) {
        if (previous.fingerprint !== fingerprint) throw fail('request_conflict', 'requestId was already used for different work.');
        if (previous.threadId) return previous;
        throw fail('outcome_unknown', 'Creation was already attempted without a confirmed identity. Inspect the desktop before reconciliation; this worker will not replay it.', { requestId });
      }
      const apps = await this.readiness(request.requiredApps);
      if (apps.missing.length) throw fail('native_app_not_provisioned', 'During one-time native app provisioning, choose Always allow in the actual app-access prompt before submitting work; review saved access in Codex Settings > Computer Use.', { missing: apps.missing });
      const entry = { fingerprint, state: 'starting', startedAt: new Date().toISOString() };
      Object.defineProperty(this.ledger.requests, requestId, { value: entry, enumerable: true, writable: true, configurable: true });
      await this.save();
      try {
        const created = await this.call('create_thread', input);
        if (!created.threadId) throw fail('outcome_unknown', 'Desktop creation did not return a stable threadId; inspect the desktop before retrying.');
        Object.assign(entry, { state: 'created', threadId: created.threadId, hostId: created.hostId || 'local' });
        await this.save();
        return entry;
      } catch (error) {
        entry.state = 'outcome_unknown';
        entry.error = error.message;
        await this.save();
        throw error;
      }
    }
    if (!['read', 'message', 'cancel'].includes(method)) throw fail('invalid_request', 'Unknown worker method.');
    const entry = this.owned(request.threadId);
    if (method === 'read') {
      const result = await this.call('read_thread', { threadId: entry.threadId, hostId: entry.hostId, includeOutputs: true, turnLimit: 5, maxOutputCharsPerItem: 4000 });
      if (result.thread?.status?.activeFlags?.includes('waitingOnApproval')) {
        let cancellationError;
        try { await this.execute({ method: 'cancel', threadId: entry.threadId }); }
        catch (error) { cancellationError = error.message; }
        throw fail('approval_required', 'Unattended task requested approval. This read attempted archive-and-stop; provision the relevant permission explicitly before new work.', {
          threadId: entry.threadId, observation: result, cancellationConfirmed: entry.state === 'cancelled', ...(cancellationError ? { cancellationError } : {}),
        });
      }
      return result;
    }
    if (method === 'message') {
      if (entry.state === 'cancelled') throw fail('cancelled', 'The task was cancelled and archived.');
      if (typeof request.prompt !== 'string' || !request.prompt.trim()) throw fail('invalid_request', 'message requires prompt.');
      return this.call('send_message_to_thread', { threadId: entry.threadId, hostId: entry.hostId, prompt: request.prompt });
    }
    if (entry.state === 'cancelled') return entry;
    entry.state = 'cancelling';
    await this.save();
    await this.call('set_thread_archived', { threadId: entry.threadId, hostId: entry.hostId, archived: true });
    const observation = await this.call('read_thread', { threadId: entry.threadId, hostId: entry.hostId, turnLimit: 1 });
    if (observation.thread?.status?.type !== 'notLoaded' || observation.turns?.some(t => t.status === 'inProgress')) {
      throw fail('cancellation_unconfirmed', 'Archive returned but task termination is not confirmed; retry cancel.', { threadId: entry.threadId, observation });
    }
    entry.state = 'cancelled';
    await this.save();
    return entry;
  }
}
