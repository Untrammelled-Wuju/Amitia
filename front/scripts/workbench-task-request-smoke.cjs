const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const ts = (() => { try { return require('typescript'); } catch (error) {
  console.error("[original-error]", error);
return require(path.resolve(path.dirname(process.execPath), '../lib/node_modules/typescript')); } })();
const root = path.resolve(__dirname, '..');
const file = path.join(root, 'src/components/workbench/workbench-task-request.ts');
const compiled = ts.transpileModule(fs.readFileSync(file, 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 }, reportDiagnostics: true });
assert.equal(compiled.diagnostics.filter(d => d.category === ts.DiagnosticCategory.Error).length, 0);
const mod = { exports: {} };
vm.runInNewContext(compiled.outputText, { module: mod, exports: mod.exports, Promise, AbortController, DOMException, setTimeout, clearTimeout }, { filename: file });
const { withTaskRequestDeadline } = mod.exports;
let passed = 0;
async function check(name, action) { await action(); passed++; }
async function rejected(promise) { await assert.rejects(promise, err => err?.name === 'AbortError'); }
async function main() {
  await check('returns a valid response', async () => {
    const ctrl = new AbortController();
    let forwarded;
    const result = await withTaskRequestDeadline(signal => { forwarded = signal; return Promise.resolve({ version: 1 }); }, ctrl.signal, 1000);
    assert.equal(result.version, 1);
    ctrl.abort();
    assert.equal(forwarded.aborted, false);
  });
  await check('does not deliver a resolved response after cancellation', async () => {
    const ctrl = new AbortController();
    let resolveRequest;
    const pending = withTaskRequestDeadline(() => new Promise(resolve => { resolveRequest = resolve; }), ctrl.signal, 1000);
    ctrl.abort();
    await rejected(pending);
    resolveRequest({ id: 'late' });
  });
  await check('cancels hung transport after deadline', async () => {
    const ctrl = new AbortController();
    let requestSignal;
    const started = Date.now();
    await rejected(withTaskRequestDeadline(signal => { requestSignal = signal; return new Promise(() => {}); }, ctrl.signal, 15));
    assert.equal(requestSignal.aborted, true);
    assert.ok(Date.now() - started < 500);
  });
  await check('forwards external abort into transport', async () => {
    const ctrl = new AbortController();
    let requestSignal;
    const pending = withTaskRequestDeadline(signal => { requestSignal = signal; return new Promise(() => {}); }, ctrl.signal, 1000);
    ctrl.abort();
    await rejected(pending);
    assert.equal(requestSignal.aborted, true);
  });
  await check('supports abort before request starts', async () => {
    const ctrl = new AbortController(); ctrl.abort();
    let started = false;
    await rejected(withTaskRequestDeadline(() => { started = true; return new Promise(() => {}); }, ctrl.signal, 100));
    assert.equal(started, false);
  });
  await check('does not turn an application error into success', async () => {
    const ctrl = new AbortController();
    await assert.rejects(withTaskRequestDeadline(() => Promise.reject(new Error('gateway unreachable')), ctrl.signal, 1000), /gateway unreachable/);
  });
  const view = fs.readFileSync(path.join(root, 'src/components/workbench/WorkbenchTasksView.vue'), 'utf8');
  const history = fs.readFileSync(path.join(root, 'src/components/workbench/workbench-task-checkpoints.ts'), 'utf8');
  await check('Web list and history both use bounded requests and foreground replacement', async () => {
    assert.match(view, /withTaskRequestDeadline\(signal => apiClient\.get\('/);
    assert.match(view, /function resumeOnVisible\(\) \{ if \(!document\.hidden\) void load\(true\); \}/);
    assert.match(view, /if \(force\) invalidate\(\);/);
    assert.match(history, /withTaskRequestDeadline\(active => apiClient\.get\('/);
  });
  const mobile = fs.readFileSync(path.join(root, '../mobile_app/lib/features/workbench/presentation/pages/workbench_tasks_view.dart'), 'utf8');
  const checkpoints = fs.readFileSync(path.join(root, '../mobile_app/lib/features/workbench/presentation/pages/workbench_task_checkpoints.dart'), 'utf8');
  await check('Flutter uses request timeout and force refresh on manual reload', async () => {
    assert.match(mobile, /Future<void> refresh\(\{bool force = false\}\)/);
    assert.match(mobile, /\.timeout\(const Duration\(seconds: 25\)\)/);
    assert.match(mobile, /onRefresh: \(\) => refresh\(force: true\)/);
    assert.match(checkpoints, /\.timeout\(const Duration\(seconds: 25\)\)/);
  });
  console.log(JSON.stringify({ passed, failed: 0 }, null, 2));
}
main().catch(err => { console.error(err); process.exitCode = 1; });
