const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const ts = (() => {
  try { return require('typescript'); }
  catch {
    return require(path.resolve(path.dirname(process.execPath), '../lib/node_modules/typescript'));
  }
})();

const root = path.resolve(__dirname, '..');
const source = fs.readFileSync(path.join(root, 'src/stores/workbench.ts'), 'utf8');
const script = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  reportDiagnostics: true,
});
assert.equal(script.diagnostics.filter((d) => d.category === ts.DiagnosticCategory.Error).length, 0);
const checks = [];
function check(name, callback) {
  callback();
  checks.push(name);
}
function storeWith(storage) {
  const module = { exports: {} };
  const context = vm.createContext({
    module, exports: module.exports,
    require: (name) => {
      if (name === 'pinia') return { defineStore: (_id, build) => build };
      if (name === 'vue') return {
        ref: (value) => ({ value }),
        computed: (getter) => ({ get value() { return getter(); } }),
        watch: () => {},
      };
      throw new Error(`Unexpected dependency: ${name}`);
    },
    localStorage: storage,
    crypto: require('node:crypto').webcrypto,
    setTimeout, clearTimeout,
    console,
  });
  vm.runInContext(script.outputText, context, { filename: 'workbench.js' });
  return module.exports.useWorkbenchStore();
}
function newStorage() {
  const data = new Map();
  return {
    getItem: (key) => data.get(key) ?? null,
    setItem: (key, value) => data.set(key, String(value)),
  };
}
const storage = newStorage();
check('isolates conversation tabs', () => {
  const store = storeWith(storage);
  store.setConversation('conversation-a');
  store.openTab({ kind: 'terminal', taskId: 'task-a' });
  store.setConversation('conversation-b');
  assert.equal(store.tabs.value.length, 0);
  store.openTab({ kind: 'browser', resourceUri: 'https://example.org' });
  store.setConversation('conversation-a');
  assert.equal(store.tabs.value.length, 1);
  assert.equal(store.tabs.value[0].taskId, 'task-a');
});
check('keeps background agent commands in their intended conversation', () => {
  const store = storeWith(storage);
  store.setConversation('conversation-a');
  const current = store.openTab({ kind: 'files' });
  const command = { protocolVersion: 1, conversationId: 'conversation-b', kind: 'git', operationId: 'event-1', focus: 'background' };
  assert.ok(store.openCommand(command));
  assert.equal(store.openCommand(command), null);
  assert.equal(store.activeTab.value.id, current.id);
  store.setConversation('conversation-b');
  assert.equal(store.tabs.value.length, 1);
  assert.equal(store.visible.value, false);
});
check('accepts tasks as a conversation-only Agent tab', () => {
  const store = storeWith(newStorage());
  store.setConversation('user-conversation');
  const task = store.openCommand({ protocolVersion: 1, conversationId: 'background-conversation', kind: 'tasks', operationId: 'task-event', focus: 'background' });
  assert.equal(task.kind, 'tasks');
  assert.equal(store.tabs.value.length, 0);
  store.setConversation('background-conversation');
  assert.equal(store.tabs.value.length, 1);
  for (const injected of [{ targetDeviceId: 'other-device' }, { workspaceUri: 'workspace://other' }, { taskId: 'other-task' }, { browserRef: 'A'.repeat(32) }]) {
    assert.equal(store.openCommand({ protocolVersion: 1, conversationId: 'background-conversation', kind: 'tasks', ...injected }), null);
  }
});
check('persists idempotency for agent events after reload', () => {
  const store = storeWith(storage);
  const command = { protocolVersion: 1, conversationId: 'persisted', kind: 'files', operationId: 'event-42' };
  assert.ok(store.openCommand(command));
  store.persistSessions();
  const restarted = storeWith(storage);
  assert.equal(restarted.openCommand(command), null);
  restarted.setConversation('persisted');
  assert.equal(restarted.tabs.value.length, 1);
});
check('rejects prototype and constructor keys from persisted layout', () => {
  const poisoned = newStorage();
  poisoned.setItem('amitia:workbench:layout:v1', '{"__proto__":{"tabs":[{"id":"bad","kind":"browser","resourceUri":"https://attacker.invalid"}],"visible":true},"constructor":{"tabs":[{"id":"bad2","kind":"git"}]},"safe":{"tabs":[{"id":"ok","kind":"files","title":"safe"}]}}');
  const store = storeWith(poisoned);
  assert.equal(store.openCommand({ protocolVersion: 1, conversationId: '__proto__', kind: 'git' }), null);
  assert.equal(store.openCommand({ protocolVersion: 1, conversationId: 'constructor', kind: 'git' }), null);
  store.setConversation('safe');
  assert.equal(store.tabs.value.length, 1);
  assert.equal(store.tabs.value[0].id, 'ok');
  store.setConversation('__proto__');
  assert.equal(store.conversationId.value, 'safe');
});
check('uses conversation-qualified view identity in Web Dock', () => {
  const dock = fs.readFileSync(path.join(root, 'src/components/workbench/WorkbenchDock.vue'), 'utf8');
  assert.match(dock, /:key="`\$\{store\.conversationId\}:\$\{store\.activeTab\.id\}`"/);
});
check('suppresses stale workspace discovery in resource view', () => {
  const view = fs.readFileSync(path.join(root, 'src/components/workbench/WorkbenchResourceView.vue'), 'utf8');
  assert.match(view, /async function loadMounts\(\) \{\s*const request = sequence;/);
  assert.match(view, /if \(request !== sequence\) return;\s*mounts\.value/);
  assert.match(view, /await loadMounts\(\);\s*if \(request !== sequence\) return;/);
});
const flutter = fs.readFileSync(path.join(root, '../mobile_app/lib/features/workbench/presentation/pages/workbench_page.dart'), 'utf8');
check('Flutter switches conversations with a fresh resource epoch', () => {
  assert.match(flutter, /void didUpdateWidget\(covariant WorkbenchPage oldWidget\)/);
  assert.match(flutter, /_conversationEpoch\+\+;\s*_loadGeneration\+\+;/);
  assert.match(flutter, /_controllers\.clear\(\);\s*_visited\.clear\(\);/);
  assert.match(flutter, /epoch != _conversationEpoch \|\| conversation != widget\.conversationId/);
  assert.match(flutter, /ValueKey\('\$\{widget\.conversationId\}-\$_conversationEpoch-\$\{tab\.id\}-\$\{_resourceEpochs\[tab\.id\] \?\? 0\}'\)/);
});
check('Flutter detaches browser tabs instead of terminating their backend resources', () => {
  const close = flutter.slice(flutter.indexOf('  void _remove(int index)'), flutter.indexOf('  void _togglePinned', flutter.indexOf('  void _remove(int index)')));
  assert.ok(close.includes('_tabs.removeAt(index)'));
  assert.ok(!close.includes(".delete('/api/workbench/browser/"));
});
check('Flutter stale WebView navigation cannot update a new conversation', () => {
  assert.match(flutter, /onPageFinished: \(url\) \{\s*if \(!mounted \|\| epoch != _conversationEpoch \|\| conversation != widget\.conversationId/);
  assert.match(flutter, /onUrlChanged: \(value\) \{[\s\S]*?epoch != _conversationEpoch \|\| conversation != widget\.conversationId/);
});
console.log(JSON.stringify({ checksPassed: checks.length, checks }, null, 2));
