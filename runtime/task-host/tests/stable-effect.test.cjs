const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const ts = require('typescript');

const input = fs.readFileSync(path.join(__dirname, '../src/task-context.ts'), 'utf8');
const output = ts.transpileModule(input, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
}).outputText;
const exportsObject = {};
vm.runInNewContext(output, {
  exports: exportsObject,
  require: () => ({}),
  Buffer, Object, Error, JSON,
});
const { HostClient, validateTaskEffectID } = exportsObject;

function recorder() {
  const calls = [];
  return {
    calls,
    rpc: {
      call: async (method, params) => {
        calls.push({ method, params });
        if (method === 'task.host.executeTool') return { result: { ok: true } };
        return { confirmed: true, eventId: 'event-1', outboxId: 'outbox-1' };
      },
    },
  };
}

test('SDK task host sends explicit stable effect ID on both native operations', async () => {
  const { rpc, calls } = recorder();
  const host = new HostClient(rpc, 'run-001', { executeTool: true, emitEvent: true });
  assert.deepEqual(await host.executeTool('native.echo', { hello: 'world' }, 1000, { effectId: 'step.001' }), { ok: true });
  await host.emitEvent('extension.foo.updated', { n: 2 }, { effectId: 'event:001' });
  assert.equal(calls.length, 2);
  assert.deepEqual(JSON.parse(JSON.stringify(calls[0])), {
    method: 'task.host.executeTool',
    params: { task_run_id: 'run-001', effect_id: 'step.001', tool_id: 'native.echo', input: { hello: 'world' }, timeout_ms: 1000 },
  });
  assert.deepEqual(JSON.parse(JSON.stringify(calls[1])), {
    method: 'task.host.emitEvent',
    params: { task_run_id: 'run-001', effect_id: 'event:001', type: 'extension.foo.updated', payload: { n: 2 } },
  });
});

test('existing SDK calls keep their native wire shape', async () => {
  const { rpc, calls } = recorder();
  const host = new HostClient(rpc, 'run-001');
  await host.executeTool('native.echo', { ok: 1 });
  await host.emitEvent('extension.foo.updated', { ok: 2 });
  assert.equal(Object.hasOwn(calls[0].params, 'effect_id'), false);
  assert.equal(Object.hasOwn(calls[1].params, 'effect_id'), false);
});

test('invalid effect IDs reject before invoking RPC', async () => {
  const { rpc, calls } = recorder();
  const host = new HostClient(rpc, 'run-001');
  for (const effectId of ['', ' a', 'a/b', '汉字', 'a\n', 'a'.repeat(129), 1, null]) {
    await assert.rejects(host.executeTool('native.echo', {}, 1000, { effectId }));
    await assert.rejects(host.emitEvent('extension.foo.updated', {}, { effectId }));
  }
  assert.equal(calls.length, 0);
  assert.equal(validateTaskEffectID({ effectId: 'step.0001' }), 'step.0001');
});
