import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { createRequire } from 'node:module';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const ts = require(path.join(execFileSync('npm', ['root', '-g'], { encoding: 'utf8' }).trim(), 'typescript'));
const read = name => fs.readFileSync(path.join(root, name), 'utf8');
const dockSource = read('front/src/components/workbench/WorkbenchDock.vue');
const terminalSource = read('front/src/components/workbench/WorkbenchTerminalView.vue');
const browserSource = read('front/src/components/workbench/WorkbenchBrowserView.vue');
const dartSource = read('mobile_app/lib/features/workbench/presentation/pages/workbench_terminal_view.dart');

function loadDock(store) {
  const script = dockSource.match(/<script setup lang="ts">([\s\S]*?)<\/script>/)?.[1];
  assert.ok(script);
  const source = script.replace(/^import .+;$/gm, '') + '\nmodule.exports = { closeTab, closeUnpinned };';
  const compiled = ts.transpileModule(source, {
    fileName: 'WorkbenchDock.ts', compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022, strict: true }, reportDiagnostics: true,
  });
  assert.equal(compiled.diagnostics.filter(x => x.category === ts.DiagnosticCategory.Error).length, 0);
  const module = { exports: {} };
  vm.runInNewContext(compiled.outputText, {
    module, exports: module.exports, defineProps() { return {}; }, useWorkbenchStore() { return store; }, onBeforeUnmount() {}, window: {}, document: {},
  }, { timeout: 5000 });
  return module.exports;
}

test('closing an unpinned terminal detaches its view without terminating the backend process', () => {
  const called = [];
  const store = {
    tabs: [{ id: 't-1', kind: 'terminal', pinned: false }],
    closeTab(id) { called.push(['closeTab', id]); }, closeAll() { called.push(['closeAll']); },
  };
  loadDock(store).closeTab('t-1');
  assert.deepEqual(called, [['closeTab', 't-1']]);
});

test('closing a browser tab does not delete its Chromium session', () => {
  const called = [];
  const store = {
    tabs: [{ id: 'b-1', kind: 'browser', pinned: false, targetDeviceId: '' }],
    closeTab(id) { called.push(id); }, closeAll() { throw new Error('unexpected closeAll'); },
  };
  loadDock(store).closeTab('b-1');
  assert.deepEqual(called, ['b-1']);
});

test('bulk tab close is only a UI operation and respects store-level pinning', () => {
  const called = [];
  loadDock({ tabs: [], closeAll() { called.push('all'); }, closeTab() { throw new Error('unexpected close'); } }).closeUnpinned();
  assert.deepEqual(called, ['all']);
});

test('pinned tabs cannot be removed through the Dock close handler', () => {
  const called = [];
  loadDock({ tabs: [{ id: 'p-1', kind: 'terminal', pinned: true }], closeTab(id) { called.push(id); }, closeAll() {} }).closeTab('p-1');
  assert.deepEqual(called, []);
});

test('terminal termination remains an explicit button action, never a tab close signal', () => {
  assert.match(terminalSource, /@click="closeTerminal">结束会话/);
  assert.match(terminalSource, /async function closeTerminal\(\)/);
  assert.match(terminalSource, /await terminalApi\.close\(id, conversation\)/);
  assert.doesNotMatch(terminalSource, /amitia:workbench:terminal-close/);
  assert.doesNotMatch(terminalSource, /if \(disposed\) \{ await terminalApi\.close/);
});

test('browser termination remains explicit and detaching releases the observer view', () => {
  assert.match(browserSource, /@click="stop"/);
  assert.match(browserSource, /async function stop\(\)/);
  assert.match(browserSource, /await apiClient\.delete\(baseURL\(\)\)/);
  assert.match(browserSource, /onBeforeUnmount\(\(\) => \{ pause\(\); replaceFrame\(''\); \}\)/);
  assert.match(browserSource, /function pause\(\) \{[\s\S]*?revision\+\+;/);
});

test('Flutter terminal view disposal and late open completion never kill the task', () => {
  const disposeSection = dartSource.split('void dispose() {')[1]?.split('Future<void> _loadMounts()')[0] || '';
  assert.doesNotMatch(disposeSection, /_client\.close/);
  const openSection = dartSource.split('Future<void> _open() async {')[1]?.split('Future<void> _send(')[0] || '';
  assert.match(openSection, /if \(!mounted \|\| _closed\) return;/);
  assert.doesNotMatch(openSection, /_client\.close/);
  assert.match(dartSource, /Future<void> _stop\(\) async/);
});
