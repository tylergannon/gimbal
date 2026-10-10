#!/usr/bin/env node
import { createConnection } from 'node:net';
import { homedir } from 'node:os';
import { join, resolve } from 'node:path';
import { createInterface } from 'node:readline';

const usage = `Usage: node client.mjs [--state-dir DIR] [--timeout-ms MS]
Read one JSON request line from stdin and print one JSON response line.
Methods: status, start, read, message, cancel. See README.md for fields.
State defaults to GIMBAL_DESKTOP_DIR or ~/.gimbal/codex-desktop.
Timeout defaults to 60000 ms. Requests are never automatically retried.`;

function options(args) {
  let stateDir = process.env.GIMBAL_DESKTOP_DIR || join(homedir(), '.gimbal', 'codex-desktop');
  let timeoutMs = 60_000;
  for (let i = 0; i < args.length; i++) {
    if (args[i] === '--help') return { help: true };
    if (args[i] === '--state-dir' && args[i + 1]) stateDir = args[++i];
    else if (args[i] === '--timeout-ms' && args[i + 1]) {
      timeoutMs = Number(args[++i]);
      if (!Number.isSafeInteger(timeoutMs) || timeoutMs <= 0 || timeoutMs > 2_147_483_647) {
        throw new Error('--timeout-ms must be a positive integer no greater than 2147483647');
      }
    } else throw new Error(`Unknown option or missing value: ${args[i]}`);
  }
  return { stateDir: resolve(stateDir), timeoutMs };
}

async function readRequest() {
  const lines = createInterface({ input: process.stdin, crlfDelay: Infinity });
  try {
    for await (const line of lines) {
      let request;
      try { request = JSON.parse(line); }
      catch { throw new Error('stdin must contain one JSON request line'); }
      if (!request || Array.isArray(request) || typeof request !== 'object' ||
          !['status', 'start', 'read', 'message', 'cancel'].includes(request.method)) {
        throw new Error('Request method must be status, start, read, message, or cancel');
      }
      return request;
    }
    throw new Error('No JSON request on stdin');
  } finally {
    lines.close();
    process.stdin.pause();
  }
}

function exchange(request, socketPath, timeoutMs) {
  return new Promise((resolveResponse, reject) => {
    const socket = createConnection(socketPath);
    let pending = '';
    let settled = false;
    const finish = (error, response) => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      socket.destroy();
      if (error) reject(error);
      else resolveResponse(response);
    };
    const timer = setTimeout(() => finish(new Error(
      `Worker timed out after ${timeoutMs} ms; the request may still have completed. Preserve the start requestId when reconciling.`,
    )), timeoutMs);
    socket.setEncoding('utf8');
    socket.on('connect', () => socket.write(`${JSON.stringify(request)}\n`));
    socket.on('error', (error) => finish(new Error(`Worker socket ${socketPath}: ${error.message}`)));
    socket.on('end', () => finish(new Error('Worker closed the socket before a complete JSON response line')));
    socket.on('data', (chunk) => {
      pending += chunk;
      const newline = pending.indexOf('\n');
      if (newline < 0) return;
      let response;
      try { response = JSON.parse(pending.slice(0, newline)); }
      catch { finish(new Error('Worker returned invalid JSON')); return; }
      if (!response || Array.isArray(response) || typeof response !== 'object' ||
          Object.hasOwn(response, 'result') === Object.hasOwn(response, 'error') ||
          (Object.hasOwn(response, 'error') && typeof response.error?.message !== 'string')) {
        finish(new Error('Worker returned an invalid response envelope'));
        return;
      }
      finish(null, response);
    });
  });
}

try {
  const opts = options(process.argv.slice(2));
  if (opts.help) process.stdout.write(`${usage}\n`);
  else {
    const response = await exchange(await readRequest(), join(opts.stateDir, 'worker.sock'), opts.timeoutMs);
    process.stdout.write(`${JSON.stringify(response)}\n`);
    if (Object.hasOwn(response, 'error')) {
      process.stderr.write(`${response.error.code || 'worker_error'}: ${response.error.message}\n`);
      process.exitCode = 1;
    }
  }
} catch (error) {
  process.stderr.write(`${error.message}\n`);
  process.exitCode = 1;
}
