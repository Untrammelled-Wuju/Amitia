import fs from 'node:fs';
import assert from 'node:assert/strict';
import path from 'node:path';
import { createRequire } from 'node:module';
import { execFileSync } from 'node:child_process';

const require = createRequire(import.meta.url);
const ts = require(path.join(execFileSync('npm', ['root', '-g'], { encoding: 'utf8', timeout: 10000 }).trim(), 'typescript'));
const source = fs.readFileSync(new URL('../../front/src/components/workbench/WorkbenchBrowserView.vue', import.meta.url), 'utf8');
const script = source.match(/<script setup lang="ts">([\s\S]*?)<\/script>/)?.[1];
assert.ok(script);
const compiled = ts.transpileModule(script.replace(/^import .*?;\s*$/gm, ''), { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.None }, reportDiagnostics: true });
assert.equal((compiled.diagnostics || []).filter(item => item.category === ts.DiagnosticCategory.Error).length, 0);
const settle = async () => { for (let i = 0; i < 20; i++) await Promise.resolve(); };
const sent = [];
const updates = [];
let resolveNavigation;
const tab = { id: 'tab', browserRef: 'agent-ref', targetDeviceId: '', resourceUri: 'https://example.org' };
const store = { conversationId: 'conv', updateTab(_id, value) { updates.push(value); } };
const apiClient = {
  async get(path) {
    if (path.endsWith('/screenshot')) return { data: null };
    return { data: { ready: true, observed: true, canControl: true, leaseId: 'generation-1', url: 'https://example.org' } };
  },
  async post(path, body) {
    sent.push({ path, body });
    if (path.endsWith('/navigate')) {
      if (body.url === 'https://late.example/') return new Promise(resolve => { resolveNavigation = resolve; });
      return { data: { url: body.url, title: 'Next' } };
    }
    if (path.endsWith('/action')) return { data: { url: 'https://example.org', title: 'Action' } };
    if (path.endsWith('/input')) return { data: { ok: true } };
    if (path.endsWith('/control')) return { data: { canControl: false } };
    throw Error(`unexpected POST ${path}`);
  },
};
const hooks = {};
const fn = new Function('ref', 'computed', 'onActivated', 'onDeactivated', 'onBeforeUnmount', 'watch', 'apiClient', 'useWorkbenchStore', 'defineProps', 'classifyRemoteBrowserFailure', 'URL', 'setInterval', 'clearInterval', compiled.outputText + '\nreturn { navigate, action, browserInput, leaseId, canControl, observed, running, address, pause };');
const view = fn(
  value => ({ value }),
  get => ({ get value() { return get(); } }),
  cb => { hooks.activated = cb; },
  cb => { hooks.deactivated = cb; },
  cb => { hooks.unmounted = cb; },
  () => {}, apiClient, () => store, () => ({ tab }),
  () => ({ terminal: false, message: 'unavailable' }),
  URL, () => 1, () => {},
);
await settle();
assert.equal(view.running.value, true);
assert.equal(view.canControl.value, true);
assert.equal(view.leaseId.value, 'generation-1');
view.address.value = 'https://next.example/';
await view.navigate();
await view.action('reload');
await view.browserInput({ action: 'text', text: 'hello' });
const mutations = sent.filter(row => /\/(navigate|action|input)$/.test(row.path));
assert.equal(mutations.length, 3);
for (const mutation of mutations) assert.equal(mutation.body.leaseId, 'generation-1');
view.address.value = 'https://late.example/';
const late = view.navigate();
await settle();
assert.equal(typeof resolveNavigation, 'function');
view.leaseId.value = 'generation-2';
const count = updates.length;
resolveNavigation({ data: { url: 'https://late.example/', title: 'Old Generation' } });
await late;
assert.equal(updates.length, count);
assert.equal(view.leaseId.value, 'generation-2');
hooks.deactivated();
assert.equal(view.leaseId.value, '');
console.log('PASS: navigation, action and input carry the active lease token');
console.log('PASS: late mutation response cannot overwrite the new lease generation');
console.log('PASS: tab detachment invalidates the current generation');
