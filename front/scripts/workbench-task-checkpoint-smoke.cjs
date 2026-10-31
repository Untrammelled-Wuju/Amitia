const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const ts = (() => { try { return require('typescript'); } catch (error) {
  console.error("[original-error]", error);
return require(path.resolve(path.dirname(process.execPath), '../lib/node_modules/typescript')); } })();
const root = path.resolve(__dirname, '..');
const filename = path.join(root, 'src/components/workbench/workbench-task-checkpoints.ts');
const compiled = ts.transpileModule(fs.readFileSync(filename, 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 }, reportDiagnostics: true });
assert.equal(compiled.diagnostics.filter(d => d.category === ts.DiagnosticCategory.Error).length, 0);
const mod = { exports: {} };
vm.runInNewContext(compiled.outputText, { module: mod, exports: mod.exports, require: dependency => {
  if (dependency === '@/composables/useApi') return { apiClient: { get: () => { throw new Error('only parser under test'); } } };
  if (dependency === './workbench-task-request') return { withTaskRequestDeadline: () => { throw new Error('only parser under test'); } };
  throw new Error('unexpected module');
}, Date, Number, Error, Array }, { filename });
const { parseTaskCheckpoints } = mod.exports;
let passed = 0;
function check(action) { action(); passed++; }
function valid() {
  return { protocolVersion: 1, conversationId: 'conv-1', coordinationId: 'task-1', source: 'sampled-durable-checkpoints', truncated: false,
    records: [{ updatedAt: '2026-10-15T01:00:00Z', interactionStatus: 'running', coordinationStatus: 'running', assignmentCount: 2, completedCount: 1, failedCount: 0, parentTerminal: false, consistent: true }] };
}
function reject(modify) { check(() => { const input = valid(); modify(input); assert.throws(() => parseTaskCheckpoints(input, 'conv-1', 'task-1')); }); }
check(() => assert.equal(parseTaskCheckpoints(valid(), 'conv-1', 'task-1').records.length, 1));
check(() => { const response = valid(); response.records = []; assert.equal(parseTaskCheckpoints(response, 'conv-1', 'task-1').records.length, 0); });
reject(x => { x.conversationId = 'other'; });
reject(x => { x.coordinationId = 'other'; });
reject(x => { x.protocolVersion = 2; });
reject(x => { x.records[0].completedCount = 3; });
reject(x => { x.records[0].failedCount = 2; });
reject(x => { x.records[0].assignmentCount = 0; });
reject(x => { x.records[0].coordinationStatus = 'running.fake'; });
reject(x => { x.records[0].interactionStatus = 'x'.repeat(65); });
reject(x => { x.records[0].updatedAt = 'not-a-date'; });
reject(x => { x.records = Array(21).fill(x.records[0]); });
console.log(JSON.stringify({ passed, failed: 0 }, null, 2));
