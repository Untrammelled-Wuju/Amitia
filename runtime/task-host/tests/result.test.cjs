const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const ts = require('typescript');

const source = fs.readFileSync(path.join(__dirname, '../src/result.ts'), 'utf8');
const code = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
}).outputText;
const exportsObject = {};
vm.runInNewContext(code, { exports: exportsObject, Buffer, JSON, Error });

test('task output limits use UTF-8 bytes before storing owner artifacts', async () => {
  let calls = 0;
  const output = '中'.repeat(24000);
  const result = await exportsObject.processResult({ success: true, output }, {
    call: async (method, params) => {
      calls++;
      assert.equal(method, 'task.artifact.saveData');
      assert.equal(params.data, output);
      assert.equal(params.task_run_id, 'run');
      return { artifactId: 'owner-artifact', handle: 'owner-handle' };
    },
  }, 'run');
  assert.equal(calls, 1);
  assert.equal(result.result.mode, 'artifact');
  assert.equal(result.result.artifact_id, 'owner-artifact');
});

test('unserializable outputs report failure without a false successful result', async () => {
  const cycle = {};
  cycle.self = cycle;
  for (const output of [cycle, 1n, () => {}, Symbol('value')]) {
    const result = await exportsObject.processResult({ success: true, output }, {
      call: () => { throw new Error('unexpected artifact write'); },
    }, 'run');
    assert.equal(result.status, 'failed');
    assert.equal(result.error.code, 'task_output_not_serializable');
    assert.equal(result.result, undefined);
  }
});
