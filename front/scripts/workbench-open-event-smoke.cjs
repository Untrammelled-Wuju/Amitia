const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const ts = (() => {
  try { return require('typescript'); }
  catch { return require(path.resolve(path.dirname(process.execPath), '../lib/node_modules/typescript')); }
})();

const root = path.resolve(__dirname, '..');
const source = fs.readFileSync(path.join(root, 'src/runtime/workbench-agent-event.ts'), 'utf8');
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  reportDiagnostics: true,
});
assert.equal(compiled.diagnostics.filter(x => x.category === ts.DiagnosticCategory.Error).length, 0);
const bridgeExports = {};
vm.runInNewContext(compiled.outputText, {
  exports: bridgeExports,
  require: (name) => {
    if (name === '@/ui-runtime/deviceIdentity') return { resolveUIHostDeviceId: async () => 'device-A' };
    if (name === '@/runtime/runtime-adapter') return { getDeploymentConfig: async () => ({ mode: 'cloud' }) };
    throw new Error(`Unknown module ${name}`);
  },
  URL,
}, { filename: 'workbench-agent-event.js' });
const checks = [];
function check(name, run) { run(); checks.push(name); }
function event(kind, payload = {}, conversationId = 'conversation-A') {
  return { type: 'workbench.open', version: 1, eventId: 'event-1', conversationId, payload: { kind, ...payload } };
}
check('task event produces conversation-scoped tasks view', () => {
  const cmd = bridgeExports.workbenchCommandFromEvent(event('tasks', { focus: 'background' }), 'fallback');
  assert.equal(cmd.kind, 'tasks');
  assert.equal(cmd.conversationId, 'conversation-A');
  assert.equal(cmd.focus, 'background');
  assert.equal(cmd.resourceUri, '');
});
check('task view rejects foreign device or workspace resource', () => {
  for (const payload of [{ targetDeviceId: 'device-other' }, { workspaceUri: 'workspace://other' }, { resourceUri: 'file:///etc/passwd' }, { taskId: 'other-task' }, { browserRef: 'x'.repeat(32) }]) {
    assert.equal(bridgeExports.workbenchCommandFromEvent(event('tasks', payload), 'fallback'), null);
  }
});
check('task event rejects invalid conversation identity', () => {
  assert.equal(bridgeExports.workbenchCommandFromEvent(event('tasks', {}, '../other'), 'fallback'), null);
});
check('browser page reference retains its original device binding', () => {
  const cmd = bridgeExports.workbenchCommandFromEvent(event('browser', { resourceUri: 'https://example.org', targetDeviceId: 'device-B', browserRef: 'A'.repeat(32) }), 'fallback');
  assert.equal(cmd.targetDeviceId, 'device-B');
  assert.equal(cmd.browserRef, 'A'.repeat(32));
});
check('browser credential URL is rejected', () => {
  assert.equal(bridgeExports.workbenchCommandFromEvent(event('browser', { resourceUri: 'https://user:password@example.org' }), 'fallback'), null);
});
check('unknown event types remain unsupported', () => {
  assert.equal(bridgeExports.workbenchCommandFromEvent(event('unknown'), 'fallback'), null);
});
check('both published schemas include tasks', () => {
  for (const name of ['open-event-v1.schema.json','open-tab-v1.schema.json']) {
    const schema = JSON.parse(fs.readFileSync(path.join(root, '../contracts/workbench', name), 'utf8'));
    const field = name.includes('event') ? schema.properties.payload.properties.kind : schema.properties.kind;
    assert.ok(field.enum.includes('tasks'));
  }
});
check('Flutter event ingestion handles tasks without granting device resources', () => {
  const flutter = fs.readFileSync(path.join(root, '../mobile_app/lib/features/workbench/runtime/workbench_mobile_events.dart'), 'utf8');
  assert.match(flutter, /'terminal', 'tasks'\}\.contains\(kind\)/);
  assert.match(flutter, /if \(kind == 'tasks' && \(uri\.isNotEmpty \|\| workspace\.isNotEmpty \|\| explicitTarget\.isNotEmpty/);
});
check('Flutter resource identity includes owner and resource fields', () => {
  const flutter = fs.readFileSync(path.join(root, '../mobile_app/lib/features/workbench/presentation/pages/workbench_page.dart'), 'utf8');
  for (const field of ['type', 'uri', 'workspaceUri', 'targetDeviceId', 'browserRef', 'taskId']) {
    assert.ok(flutter.includes(`${field} == other.${field}`));
  }
  assert.match(flutter, /_resourceEpochs\[tab\.id\] = \+\+_resourceSerial/);
  assert.match(flutter, /identical\(_controllers\[tab\.id\], controller\)/);
  assert.match(flutter, /resourceEpoch != \(_resourceEpochs\[tab\.id\] \?\? 0\)/);
});
console.log(JSON.stringify({ checksPassed: checks.length, checks }, null, 2));
