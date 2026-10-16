import fs from 'node:fs';
import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import { execFileSync } from 'node:child_process';
import path from 'node:path';

const require = createRequire(import.meta.url);
const ts = require(path.join(execFileSync('npm', ['root', '-g'], { encoding: 'utf8', timeout: 10000 }).trim(), 'typescript'));

const source = new URL('../../front/src/components/workbench/WorkbenchBrowserView.vue', import.meta.url);
const contents = fs.readFileSync(source, 'utf8');
const match = contents.match(/<script setup lang="ts">([\s\S]*?)<\/script>/);
assert.ok(match, 'missing browser view setup script');
const moduleBody = match[1].replace(/^import .*?;\s*$/gm, '');
const compiled = ts.transpileModule(moduleBody, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.None }, reportDiagnostics: true });
assert.deepEqual((compiled.diagnostics || []).filter(item => item.category === ts.DiagnosticCategory.Error), []);
const deferred = () => {
  let resolve;
  let reject;
  const promise = new Promise((done, fail) => { resolve = done; reject = fail; });
  return { promise, resolve, reject };
};
const settle = async () => { for (let i = 0; i < 12; i++) await Promise.resolve(); };
const pendingScreenshots = [];
const pendingDOM = [];
const hooks = {};
const blobs = [];
const tab = { id: 'tab-1', resourceUri: 'https://example.org', browserRef: 'opaque-ref', targetDeviceId: 'device-A' };
const store = { conversationId: 'conversation-1', updateTab() {} };
const apiClient = { async post(path, data) {
  assert.equal(path, '/api/device-mesh/v1/workbench/browser/read');
  switch (data.operation) {
    case 'state': return { data: { ready: true, observed: true, url: 'https://example.org' } };
    case 'screenshot': {
      const item = deferred();
      pendingScreenshots.push(item);
      return item.promise;
    }
    case 'dom': {
      const item = deferred();
      pendingDOM.push(item);
      return item.promise;
    }
    default: throw Error('unknown remote operation');
  }
} };
class URLMock extends URL {
  static createObjectURL(blob) { const id = `blob:browser-${blobs.length + 1}`; blobs.push({ id, blob }); return id; }
  static revokeObjectURL() {}
}
const fn = new Function('ref', 'computed', 'onActivated', 'onDeactivated', 'onBeforeUnmount', 'watch', 'apiClient', 'useWorkbenchStore', 'defineProps', 'classifyRemoteBrowserFailure', 'URL', 'setInterval', 'clearInterval', compiled.outputText + '\nreturn { refreshFrame, loadDOM, frameURL, domContent, running, stop };');
const view = fn(
  value => ({ value }),
  get => ({ get value() { return get(); } }),
  cb => { hooks.activate = cb; },
  cb => { hooks.deactivate = cb; },
  cb => { hooks.unmount = cb; },
  () => {},
  apiClient, () => store, () => ({ tab }),
  () => ({ terminal: false, message: 'temporary' }),
  URLMock,
  () => 1,
  () => {}
);
await settle();
assert.equal(view.running.value, true, 'remote observation should become ready');
assert.equal(pendingScreenshots.length, 1, 'first frame should be pending');
hooks.deactivate();
hooks.activate();
await settle();
assert.equal(pendingScreenshots.length, 2, 'new activation must not be blocked by old screenshot');
const jpeg = Buffer.from([0xff, 0xd8, 0xff, 0xd9]).toString('base64');
pendingScreenshots[1].resolve({ data: { ready: true, observed: true, imageBase64: jpeg } });
await settle();
assert.equal(view.frameURL.value, 'blob:browser-1');
pendingScreenshots[0].resolve({ data: { ready: true, observed: true, imageBase64: jpeg } });
await settle();
assert.equal(view.frameURL.value, 'blob:browser-1', 'stale request overwrote newer frame');
const pending = view.loadDOM();
await settle();
assert.equal(pendingDOM.length, 1);
hooks.deactivate();
hooks.activate();
pendingDOM[0].resolve({ data: { ready: true, observed: true, content: 'stale DOM' } });
await pending;
assert.equal(view.domContent.value, '', 'stale DOM overwrote new view');
await view.stop();
assert.equal(view.running.value, false, 'stop must close observation');
hooks.unmount();
console.log('PASS: prior screenshot aborted on deactivate');
console.log('PASS: newer screenshot remains authoritative');
console.log('PASS: stale DOM does not overwrite reactivated view');
console.log('PASS: explicit stop prevents further observation');
