const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const ts = (() => { try { return require('typescript'); } catch (error) {
  console.error("[original-error]", error);
return require(path.resolve(path.dirname(process.execPath), '../lib/node_modules/typescript')); } })();
const root = path.resolve(__dirname, '..', 'src/components/workbench');
const modules = new Map();
function load(name) {
  if (modules.has(name)) return modules.get(name).exports;
  const filename = path.join(root, `${name}.ts`);
  const compiled = ts.transpileModule(fs.readFileSync(filename, 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 }, reportDiagnostics: true,
  });
  assert.deepEqual(compiled.diagnostics.filter(d => d.category === ts.DiagnosticCategory.Error), []);
  const mod = { exports: {} };
  modules.set(name, mod);
  const context = vm.createContext({ module: mod, exports: mod.exports, require: (dependency) => {
    assert.match(dependency, /^\.\/workbench-task-/);
    return load(dependency.slice(2));
  }, Date, Set, Number, Error, Array });
  vm.runInContext(compiled.outputText, context, { filename });
  return mod.exports;
}
const { parseWorkbenchTaskList } = load('workbench-task-list');
let passed = 0;
function check(name, run) { run(); passed++; }
function valid() {
  return {
    protocolVersion: 1, conversationId: 'conv-1', source: 'durable-checkpoint', truncated: false, liveObservationTruncated: false,
    summary: { total: 1, active: 1, paused: 0, succeeded: 0, failed: 0, runtimeObserved: 1, checkpointOnly: 0, needsReconciliation: 0, codingWorkersRequiringReview: 1 },
    tasks: [{ coordinationId: 'coord-1', status: 'running', updatedAt: '2026-10-15T00:01:02Z', recoveryState: 'runtime-observed', assignmentCount: 1, completedCount: 0, failedCount: 0,
      assignments: [{ id: 'assignment-1', status: 'running', role: 'coding_worker', objective: 'Build code', reviewRequired: true }] }],
  };
}
function rejected(name, modify) {
  check(name, () => { const response = valid(); modify(response); assert.throws(() => parseWorkbenchTaskList(response, 'conv-1')); });
}
check('accepts scoped and bounded live response', () => {
  const result = parseWorkbenchTaskList(valid(), 'conv-1');
  assert.equal(result.tasks[0].assignments[0].reviewRequired, true);
  assert.equal(result.summary.runtimeObserved, 1);
});
check('accepts empty snapshot with zeroed summary', () => {
  const response = valid(); response.tasks = []; response.summary = { ...response.summary, total: 0, active: 0, runtimeObserved: 0, codingWorkersRequiringReview: 0 };
  const parsed = parseWorkbenchTaskList(response, 'conv-1');
  assert.equal(parsed.tasks.length, 0);
});
rejected('refuses mismatched conversation', payload => { payload.conversationId = 'conv-other'; });
rejected('refuses protocol downgrade', payload => { payload.protocolVersion = 0; });
rejected('refuses unknown source', payload => { payload.source = 'unverified'; });
rejected('refuses oversized result arrays', payload => { payload.tasks = Array(31).fill(payload.tasks[0]); });
rejected('refuses duplicate coordination ids', payload => { payload.tasks.push(payload.tasks[0]); payload.summary.total = 2; });
rejected('refuses child identity mismatch', payload => { payload.tasks[0].assignmentCount = 2; });
rejected('refuses duplicate child identity', payload => { payload.tasks[0].assignments.push(payload.tasks[0].assignments[0]); payload.tasks[0].assignmentCount = 2; });
rejected('refuses unbounded objective content', payload => { payload.tasks[0].assignments[0].objective = 'x'.repeat(221); });
rejected('refuses impossible completed totals', payload => { payload.tasks[0].completedCount = 2; });
rejected('refuses fabricated summary count', payload => { payload.summary.total = 2; });
rejected('refuses forged active count', payload => { payload.summary.active = 0; });
rejected('refuses forged recovery breakdown', payload => { payload.summary.runtimeObserved = 0; });
rejected('refuses forged review count', payload => { payload.summary.codingWorkersRequiringReview = 0; });
rejected('refuses impossible completed children', payload => { payload.tasks[0].completedCount = 1; });
rejected('refuses impossible failed children', payload => { payload.tasks[0].failedCount = 1; });
check('accepts consistent completed task and worker', () => {
  const response = valid();
  response.tasks[0].status = 'succeeded';
  response.tasks[0].assignments[0].status = 'succeeded';
  response.tasks[0].completedCount = 1;
  response.summary.active = 0;
  response.summary.succeeded = 1;
  const parsed = parseWorkbenchTaskList(response, 'conv-1');
  assert.equal(parsed.tasks[0].failedCount, 0);
  assert.equal(parsed.summary.succeeded, 1);
});
rejected('refuses negative statistics', payload => { payload.summary.failed = -1; });
rejected('refuses fractional statistics', payload => { payload.summary.runtimeObserved = 0.2; });
rejected('refuses invalid recovery status', payload => { payload.tasks[0].recoveryState = 'arbitrary'; });
rejected('refuses invalid timestamps', payload => { payload.tasks[0].updatedAt = 'not-date'; });
rejected('refuses invalid assignment state', payload => { payload.tasks[0].assignments[0].status = 'injected'; });
check('browser task view consumes scoped parser', () => {
  const source = fs.readFileSync(path.join(root, 'WorkbenchTasksView.vue'), 'utf8');
  assert.match(source, /parseWorkbenchTaskList\(response\.data, id\)/);
  assert.match(source, /generationId === historyGeneration && conversationId === props\.conversationId/);
  assert.match(source, /document\.addEventListener\('visibilitychange', resumeOnVisible\)/);
});
check('Flutter task view consumes scoped parser', () => {
  const source = fs.readFileSync(path.join(root, '../../../../mobile_app/lib/features/workbench/presentation/pages/workbench_tasks_view.dart'), 'utf8');
  assert.match(source, /WorkbenchTaskSnapshot\.fromResponse\(response, id\)/);
  const parser = fs.readFileSync(path.join(root, '../../../../mobile_app/lib/features/workbench/presentation/pages/workbench_task_snapshot.dart'), 'utf8');
  assert.match(parser, /value\['conversationId'\] != conversationId/);
  assert.match(parser, /value\['protocolVersion'\] != 1/);
});
console.log(JSON.stringify({ passed, failed: 0 }, null, 2));
