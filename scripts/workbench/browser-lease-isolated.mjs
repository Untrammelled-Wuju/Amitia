import fs from 'node:fs';
import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import path from 'node:path';
import { execFileSync } from 'node:child_process';

const require = createRequire(import.meta.url);
const ts = require(path.join(execFileSync('npm', ['root', '-g'], { encoding: 'utf8', timeout: 10000 }).trim(), 'typescript'));
const contents = fs.readFileSync(new URL('../../front/src/components/workbench/WorkbenchBrowserView.vue', import.meta.url), 'utf8');
const body = contents.match(/<script setup lang="ts">([\s\S]*?)<\/script>/)?.[1];
assert.ok(body);
const compiled = ts.transpileModule(body.replace(/^import .*?;\s*$/gm, ''), { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.None }, reportDiagnostics: true });
assert.equal(compiled.diagnostics?.filter(d => d.category === ts.DiagnosticCategory.Error).length || 0, 0);
const deferred = () => { let resolve; const promise = new Promise(done => { resolve = done; }); return { promise, resolve }; };
const settle = async () => { for (let i = 0; i < 16; i++) await Promise.resolve(); };
const hooks = {};
const pending = [];
let delayedRenew = null;
const observed = [];
const tab = { id: 'tab-lease', resourceUri: 'https://example.com', browserRef: 'opaque-agent-ref', targetDeviceId: '' };
const store = { conversationId: 'conversation-lease', updateTab() {} };
const api = {
  async get(url) {
    if (url.endsWith('/screenshot')) return { data: null };
    return { data: { ready: true, observed: true, canControl: false, url: 'https://example.com' } };
  },
  async post(url, data) {
    if (url.endsWith('/control')) {
      observed.push({ action: data.action, token: data.leaseId });
      if (data.action === 'acquire') { const task = deferred(); pending.push(task); return task.promise; }
      if (data.action === 'renew') return delayedRenew ? delayedRenew.promise : { data: { canControl: true, leaseId: data.leaseId } };
      if (data.action === 'release') return { data: { canControl: false } };
    }
    if (url.endsWith('/observe')) return { data: { ready: true, observed: true, canControl: false } };
    throw Error(`unexpected request ${url}`);
  },
};
const fn = new Function('ref', 'computed', 'onActivated', 'onDeactivated', 'onBeforeUnmount', 'watch', 'apiClient', 'useWorkbenchStore', 'defineProps', 'classifyRemoteBrowserFailure', 'URL', 'setInterval', 'clearInterval', compiled.outputText + '\nreturn { control, pause, resume, running, canControl, leaseId };');
const view = fn(
  value => ({ value }),
  get => ({ get value() { return get(); } }),
  cb => { hooks.activate = cb; },
  cb => { hooks.deactivate = cb; },
  cb => { hooks.unmount = cb; },
  () => {}, api, () => store, () => ({ tab }),
  () => ({ terminal: false, message: 'unavailable' }),
  URL, () => 1, () => {},
);
await settle();
assert.equal(view.running.value, true);
const first = view.control('acquire');
await settle();
assert.equal(pending.length, 1);
hooks.deactivate();
hooks.activate();
pending[0].resolve({ data: { canControl: true, leaseId: 'old-generation' } });
await first;
await settle();
assert.deepEqual(observed.find(row => row.action === 'release'), { action: 'release', token: 'old-generation' });
assert.equal(view.canControl.value, false);
assert.equal(view.leaseId.value, '');
const second = view.control('acquire');
await settle();
assert.equal(pending.length, 2);
pending[1].resolve({ data: { canControl: true, leaseId: 'new-generation' } });
await second;
assert.equal(view.leaseId.value, 'new-generation');
assert.equal(view.canControl.value, true);
await view.control('renew');
assert.ok(observed.some(row => row.action === 'renew' && row.token === 'new-generation'));
assert.equal(view.leaseId.value, 'new-generation');
delayedRenew = deferred();
const pendingRenew = view.control('renew');
await settle();
await view.control('release');
assert.equal(view.canControl.value, false);
delayedRenew.resolve({ data: { canControl: true, leaseId: 'new-generation' } });
await pendingRenew;
assert.equal(view.canControl.value, false, 'late renewal must not restore released control');
const finalAcquire = view.control('acquire');
await settle();
assert.equal(pending.length, 3);
pending[2].resolve({ data: { canControl: true, leaseId: 'third-generation' } });
await finalAcquire;
assert.equal(view.leaseId.value, 'third-generation');
hooks.deactivate();
await settle();
assert.ok(observed.some(row => row.action === 'release' && row.token === 'third-generation'));
assert.equal(view.leaseId.value, '');
console.log('PASS: old asynchronous acquire releases only its original generation');
console.log('PASS: active generation is used for renew');
console.log('PASS: late renewal cannot restore already released control');
console.log('PASS: tab detachment releases the captured generation');
