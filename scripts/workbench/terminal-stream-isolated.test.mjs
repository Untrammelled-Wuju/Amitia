import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { createRequire } from 'node:module';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const sourceRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const moduleFile = path.join(sourceRoot, 'front/src/components/workbench/useWorkbenchTerminalStream.ts');
const typescriptRoot = execFileSync('npm', ['root', '-g'], { encoding: 'utf8', timeout: 10000 }).trim();
const ts = require(path.join(typescriptRoot, 'typescript'));
const result = ts.transpileModule(fs.readFileSync(moduleFile, 'utf8'), {
  fileName: moduleFile,
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022, strict: true },
  reportDiagnostics: true,
});
assert.equal((result.diagnostics || []).filter(d => d.category === ts.DiagnosticCategory.Error).length, 0);

function harness({ deferredToken = false, stalledSocket = false } = {}) {
  const requests = [];
  const sockets = [];
  let tokenResolver;
  let timeoutHandler;
  let cleared = false;
  class Socket {
    static OPEN = 1;
    readyState = 0;
    constructor(url) {
      this.url = url;
      sockets.push(this);
      if (!stalledSocket) queueMicrotask(() => { this.readyState = Socket.OPEN; this.onopen?.(); });
    }
    send(data) { this.lastInput = JSON.parse(data); }
    close() { this.readyState = 3; this.onclose?.(); }
  }
  const client = { post: (...args) => {
    requests.push(args);
    if (deferredToken) return new Promise(resolve => { tokenResolver = resolve; });
    return Promise.resolve({ data: { ticket: 'b'.repeat(48) } });
  } };
  const module = { exports: {} };
  const context = {
    module, exports: module.exports, URL, AbortController, TextEncoder, WebSocket: Socket,
    setTimeout: fn => { timeoutHandler = fn; return 1; },
    clearTimeout: () => { cleared = true; },
    window: { location: { origin: 'http://localhost:18899', protocol: 'http:' } },
    require: key => {
      if (key === '@/composables/useApi') return { apiClient: client };
      if (key === '@/runtime/runtime-adapter') return { getApiBaseURLForPath: async () => 'http://localhost:18899' };
      if (key === './useWorkbenchTerminal') return { encodeInput: value => Buffer.from(value).toString('base64') };
      throw new Error('unexpected dependency: ' + key);
    },
  };
  vm.runInNewContext(result.outputText, context, { filename: moduleFile, timeout: 5000 });
  return { connect: module.exports.connectTerminalStream, requests, sockets, expire: () => timeoutHandler?.(), cleared: () => cleared, resolveToken: () => tokenResolver?.({ data: { ticket: 'b'.repeat(48) } }) };
}

test('WebSocket works and receives only the matching session and valid cursor', async () => {
  const h = harness();
  const seen = [];
  const stream = await h.connect('session-a', 'conversation-a', 7, snap => seen.push(snap.cursor), () => {});
  assert.equal(h.requests.length, 1);
  assert.equal(h.requests[0][2].timeout, 12000);
  assert.equal(new URL(h.sockets[0].url).searchParams.get('cursor'), '7');
  assert.equal(stream.input('hello'), true);
  assert.equal(h.sockets[0].lastInput.type, 'input');
  assert.equal(stream.resize(30.5, 90), false);
  assert.equal(stream.resize(30, 90), true);
  h.sockets[0].onmessage({ data: JSON.stringify({ type: 'snapshot', snapshot: { id: 'session-a', cursor: 8 } }) });
  h.sockets[0].onmessage({ data: JSON.stringify({ type: 'snapshot', snapshot: { id: 'session-b', cursor: 9 } }) });
  h.sockets[0].onmessage({ data: JSON.stringify({ type: 'snapshot', snapshot: { id: 'session-a', cursor: -1 } }) });
  assert.deepEqual(seen, [8]);
  stream.close();
  assert.equal(stream.input('later'), false);
  assert.equal(h.cleared(), true);
});

test('cancellation before issuing ticket is immediate', async () => {
  const h = harness();
  const controller = new AbortController();
  controller.abort();
  await assert.rejects(h.connect('a', 'c', 0, () => {}, () => {}, controller.signal), /取消/);
  assert.equal(h.requests.length, 0);
  await assert.rejects(h.connect('a', 'c', -1, () => {}, () => {}), /游标/);
});

test('cancellation while waiting for ticket does not open a socket', async () => {
  const h = harness({ deferredToken: true });
  const controller = new AbortController();
  const promise = h.connect('a', 'c', 0, () => {}, () => {}, controller.signal);
  controller.abort();
  h.resolveToken();
  await assert.rejects(promise, /取消/);
  assert.equal(h.sockets.length, 0);
});

test('cancellation while waiting for socket rejects and cleans transport', async () => {
  const h = harness({ stalledSocket: true });
  const controller = new AbortController();
  let disconnected = 0;
  const promise = h.connect('a', 'c', 0, () => {}, () => { disconnected++; }, controller.signal);
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(h.sockets.length, 1);
  controller.abort();
  await assert.rejects(promise, /取消/);
  assert.equal(h.sockets[0].readyState, 3);
  assert.equal(disconnected, 0);
});

test('handshake timeout closes orphaned WebSocket and does not pretend connected', async () => {
  const h = harness({ stalledSocket: true });
  const promise = h.connect('a', 'c', 0, () => {}, () => {});
  await new Promise(resolve => setImmediate(resolve));
  h.expire();
  await assert.rejects(promise, /超时/);
  assert.equal(h.sockets[0].readyState, 3);
});
