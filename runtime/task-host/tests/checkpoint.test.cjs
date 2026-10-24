const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const crypto = require('node:crypto');
const ts = require('typescript');

const source = fs.readFileSync(path.join(__dirname, '../src/checkpoint.ts'), 'utf8');
const code = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
}).outputText;
const exportsObject = {};
vm.runInNewContext(code, { exports: exportsObject, Date, Number, Error, Buffer, require: (name) => { if (name === 'node:crypto') return crypto; throw new Error('Unexpected import: ' + name); } });
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


test('post-checkpoint stable effect plan survives the existing Owner save protocol', async () => {
  const sent = [];
  const client = new CheckpointClient({ call: async (method, params) => {
    sent.push({method, params});
    return {version: params.version};
  }}, 'run', {cursor: 1});
  await client.save({data: {phase: 'downloaded'}, nextEffectIds: ['plan.step.002', 'plan.step.003']});
  assert.equal(sent.length, 1);
  assert.equal(sent[0].method, 'task.checkpoint.save');
  assert.deepEqual(Array.from(sent[0].params.payload.nextEffectIds), ['plan.step.002', 'plan.step.003']);
  assert.equal((await client.load()).cursor, 2);
  for (const ids of [[], null, ['bad/path'], ['same','same'], Array.from({length: 33}, (_, i) => `x${i}`)]) {
    await assert.rejects(client.save({data: {}, nextEffectIds: ids}), /nextEffectIds/);
  }
  assert.equal(sent.length, 1);
});

test('bound planned effects persist only hashes and reject changed declarations', async () => {
  const sent = [];
  const client = new CheckpointClient({call: async (method, params) => {sent.push(params);return {version:params.version};}}, 'run-1', null);
  const privateArgs = { path: '/private/notes', content: 'SENSITIVE_TEST_VALUE' };
  await client.save({data:{phase:'prepare'}, nextEffects: [
    {effectId:'write.001', method:'task.host.executeTool', toolId:'native.write', input:privateArgs, timeoutMs:1500},
    {effectId:'emit.001', method:'task.host.emitEvent', type:'updated', payload:{revision:1}},
  ]});
  const saved = sent[0].payload.nextEffects;
  assert.equal(saved.length, 2);
  assert.equal(saved[0].inputHash, crypto.createHash('sha256').update(JSON.stringify(privateArgs)).digest('hex'));
  assert.equal(saved[1].payloadHash, crypto.createHash('sha256').update('{"revision":1}').digest('hex'));
  assert.equal(saved[0].timeoutMs, 1500);
  assert.equal(JSON.stringify(sent[0].payload).includes('SENSITIVE_TEST_VALUE'), false);
  assert.equal(JSON.stringify(await client.load()).includes('SENSITIVE_TEST_VALUE'), false);
  assert.equal('input' in saved[0], false);
  assert.equal('payload' in saved[1], false);
  for (const nextEffects of [
    [],
    [{effectId:'x',method:'task.host.executeTool',toolId:'native.write'}],
    [{effectId:'x',method:'task.host.executeTool',toolId:'native.write',input:{},timeoutMs:6000}],
    [{effectId:'bad/path',method:'task.host.executeTool',toolId:'native.write',input:{}}],
    [{effectId:'same',method:'task.host.executeTool',toolId:'native.write',input:{}},
     {effectId:'same',method:'task.host.executeTool',toolId:'native.write',input:{}}],
    [{effectId:'x',method:'task.host.emitEvent',type:'updated'}],
    [{effectId:'x',method:'task.host.other',type:'updated',payload:{}}],
  ]) {
    await assert.rejects(client.save({data:{}, nextEffects}), /nextEffects|计划/);
  }
  await assert.rejects(client.save({nextEffects:[{effectId:'x',method:'task.host.executeTool',toolId:'x',input:{}}],nextEffectIds:['x']}), /不可同时/);
  assert.equal(sent.length, 1);
});
