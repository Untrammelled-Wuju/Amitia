const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const ts = require('typescript');

const source = fs.readFileSync(path.join(__dirname, '../src/checkpoint.ts'), 'utf8');
const code = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
}).outputText;
const exportsObject = {};
vm.runInNewContext(code, { exports: exportsObject, Date, Number, Error });
const { CheckpointClient } = exportsObject;

test('restored checkpoint versions remain monotonic', async () => {
  const sent = [];
  const client = new CheckpointClient({ call: async (method, params) => {sent.push({ method, params });return {version:params.version};} }, 'run', { cursor: 7, data: { value: 42 } });
  await client.save({ cursor: 1, data: { value: 43 } });
  await client.save({ data: { value: 44 } });
  assert.equal(sent[0].params.version, 8);
  assert.equal(sent[0].params.payload.cursor, 8);
  assert.equal(sent[1].params.version, 9);
  assert.equal((await client.load()).cursor, 9);
});

test('invalid restored versions and exhausted counters are refused', async () => {
  for (const cursor of [-1, 1.5, NaN, Infinity, '7']) {
    assert.throws(() => new CheckpointClient({ notify() {} }, 'run', { cursor }));
  }
  let sent = false;
  const client = new CheckpointClient({ call() { sent = true; } }, 'run', { cursor: Number.MAX_SAFE_INTEGER });
  await assert.rejects(client.save({ data: {} }));
  assert.equal(sent, false);
});

test('failed or pending owner confirmation cannot advance the checkpoint or allow another save', async () => {
  let release;
  const client = new CheckpointClient({ call: () => new Promise(resolve => {release=resolve;}) }, 'run', {cursor:7});
  const pending = client.save({data:{private:'unconfirmed'}});
  const confirmation = client.waitForConfirmation();
  assert.equal((await client.load()).cursor,7);
  await assert.rejects(client.save({data:{other:true}}), /正在保存/);
  release({version:9});
  await assert.rejects(pending, /版本不一致/);
  await assert.rejects(confirmation, /版本不一致/);
  assert.equal((await client.load()).cursor,7);
  await assert.rejects(client.save({data:{retry:true}}), /尚未确认/);
});
