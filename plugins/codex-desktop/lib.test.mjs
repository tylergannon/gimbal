import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import net from 'node:net';
import { Worker, appReadiness, desktopCall } from './lib.mjs';

const caller = { thread_id: 'owner', turn_id: 'turn' };
const start = { method: 'start', requestId: 'one', prompt: 'bounded task', target: { type: 'projectless' } };
async function fixture(t, call, readiness = async () => ({ approved: [], missing: [] })) {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), 'gimbal-desktop-'));
  t.after(() => fs.rm(directory, { recursive: true, force: true }));
  const worker = new Worker(directory, caller, call, readiness);
  await worker.load();
  return worker;
}

test('concurrent identical starts create once and survive worker reconnect', async t => {
  let calls = 0;
  const call = async () => { calls++; await new Promise(r => setTimeout(r, 5)); return { threadId: 'native1', hostId: 'local' }; };
  const worker = await fixture(t, call);
  const results = await Promise.all([worker.handle(start), worker.handle(start)]);
  assert.equal(calls, 1);
  assert.equal(results[0].threadId, 'native1');
  const resumed = new Worker(worker.directory, caller, call);
  await resumed.load();
  assert.equal((await resumed.handle(start)).threadId, 'native1');
  assert.equal(calls, 1);
  await assert.rejects(resumed.handle({ ...start, prompt: 'different' }), { code: 'request_conflict' });
});

test('unknown creation is durably owned and never replayed', async t => {
  let calls = 0;
  const worker = await fixture(t, async () => { calls++; throw Error('connection lost after send'); });
  await assert.rejects(worker.handle(start), /connection lost/);
  const resumed = new Worker(worker.directory, caller, async () => { calls++; });
  await resumed.load();
  await assert.rejects(resumed.handle(start), { code: 'outcome_unknown' });
  assert.equal(calls, 1);
  assert.equal(resumed.ledger.requests.one.state, 'outcome_unknown');
});

test('unprovisioned native app fails before provider task creation', async t => {
  let calls = 0;
  const worker = await fixture(t, async () => { calls++; }, async () => ({ approved: [], missing: ['dev.fixture'] }));
  await assert.rejects(worker.handle({ ...start, requiredApps: ['dev.fixture'] }), { code: 'native_app_not_provisioned' });
  assert.equal(calls, 0);
  assert.deepEqual(worker.ledger.requests, {});
});

test('ownership rejects arbitrary provider thread IDs', async t => {
  const worker = await fixture(t, async () => assert.fail('must not call desktop'));
  for (const method of ['read', 'message', 'cancel']) {
    await assert.rejects(worker.handle({ method, threadId: 'unrelated', prompt: 'no' }), { code: 'not_owned' });
  }
});

test('archive acknowledgment is insufficient; cleanup remains retryable', async t => {
  let settled = false;
  let archives = 0;
  const worker = await fixture(t, async name => {
    if (name === 'create_thread') return { threadId: 'native1' };
    if (name === 'set_thread_archived') { archives++; return { archived: true }; }
    return { thread: { status: { type: settled ? 'notLoaded' : 'active' } }, turns: [{ status: settled ? 'interrupted' : 'inProgress' }] };
  });
  await worker.handle(start);
  await assert.rejects(worker.handle({ method: 'cancel', threadId: 'native1' }), { code: 'cancellation_unconfirmed' });
  assert.equal(worker.ledger.requests.one.state, 'cancelling');
  settled = true;
  assert.equal((await worker.handle({ method: 'cancel', threadId: 'native1' })).state, 'cancelled');
  await worker.handle({ method: 'cancel', threadId: 'native1' });
  assert.equal(archives, 2);
});

test('approval waits become explicit unattended failure with owned identity', async t => {
  let archived = false;
  const worker = await fixture(t, async name => {
    if (name === 'create_thread') return { threadId: 'native1' };
    if (name === 'set_thread_archived') { archived = true; return {}; }
    return { thread: { status: archived ? { type: 'notLoaded' } : { activeFlags: ['waitingOnApproval'] } }, turns: [] };
  });
  await worker.handle(start);
  await assert.rejects(worker.handle({ method: 'read', threadId: 'native1' }), error => error.code === 'approval_required' && error.details.threadId === 'native1' && error.details.cancellationConfirmed === true);
  assert.equal(archived, true);
});

test('another owner cannot silently adopt persistent sessions', async t => {
  const worker = await fixture(t, async () => ({ threadId: 'native1' }));
  await worker.handle(start);
  const other = new Worker(worker.directory, { ...caller, thread_id: 'other' }, async () => {});
  await assert.rejects(other.load(), /belongs to desktop task owner/);
});

test('readiness checks only requested native bundle identifiers', async t => {
  const home = await fs.mkdtemp(path.join(os.tmpdir(), 'gimbal-approval-'));
  t.after(() => fs.rm(home, { recursive: true, force: true }));
  const file = path.join(home, 'Library/Group Containers/2DC432GLL2.com.openai.sky.CUAService/Library/Application Support/Software/ComputerUseAppApprovals.json');
  await fs.mkdir(path.dirname(file), { recursive: true });
  await fs.writeFile(file, JSON.stringify({ approvedBundleIdentifiers: ['approved', 'unrelated'] }));
  assert.deepEqual(await appReadiness(['approved', 'missing'], home), { approved: ['approved'], missing: ['missing'] });
});

test('bridge decodes fragmented framed response and forwards real context', async t => {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), 'gimbal-rpc-'));
  t.after(() => fs.rm(directory, { recursive: true, force: true }));
  const socketPath = path.join(directory, 'test.sock');
  const server = net.createServer(socket => {
    socket.once('data', bytes => {
      const req = JSON.parse(bytes.subarray(4));
      assert.equal(req.params.threadId, caller.thread_id);
      assert.equal(req.params.turnId, caller.turn_id);
      const data = Buffer.from(JSON.stringify({ id: 1, result: { success: true, contentItems: [{ type: 'inputText', text: '{"threadId":"native1"}' }] } }));
      const header = Buffer.alloc(4); header.writeUInt32LE(data.length);
      socket.write(header.subarray(0, 2));
      setTimeout(() => socket.end(Buffer.concat([header.subarray(2), data])), 5);
    });
  });
  await new Promise(resolve => server.listen(socketPath, resolve));
  t.after(() => server.close());
  assert.deepEqual(await desktopCall(socketPath, caller, 'create_thread', {}), { threadId: 'native1' });
});
