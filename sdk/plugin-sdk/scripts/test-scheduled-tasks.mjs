import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { resolve, dirname } from 'node:path';
import { Script } from 'node:vm';
const require = createRequire(import.meta.url);
const ts = require('../../../front/node_modules/typescript');
const loaded = new Map();
function load(file) {
  const absolute = resolve(file);
  if (loaded.has(absolute)) return loaded.get(absolute).exports;
  const module = { exports: {} };
  loaded.set(absolute, module);
  const output = ts.transpileModule(readFileSync(absolute, 'utf8'), { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS } }).outputText;
  const execute = new Script(`(function(require,module,exports){${output}\n})`, { filename: absolute }).runInThisContext();
  execute(name => name.startsWith('.') ? load(resolve(dirname(absolute), `${name}.ts`)) : require(name), module, module.exports);
  return module.exports;
}
const { createScheduledTaskClient, scheduledTaskTransport } = load(resolve('src/scheduled-tasks.ts'));
const { DefaultHostClient } = load(resolve('src/host.ts'));
const calls = [];
const record = { id: 'task-stable', ownerId: 'core-cloud', saved: true, generation: 3, state: 'enabled', enabled: 1 };
const rawHost = {
  calls,
  async call(method, input) {
    assert.equal(this, rawHost);
    calls.push({ method, input });
    if (method.endsWith('.list')) return { items: [record], total: 1 };
    if (method.endsWith('.history')) return { items: [{ state: 'partial', result: { targets: [{ state: 'failed' }] } }], total: 1 };
    if (method.endsWith('.execute')) return { triggerId: 'trigger-1', status: 'waiting' };
    if (['status', 'queue', 'preview', 'clear', 'capabilities'].some(action => method.endsWith(`.${action}`))) return { ready: true };
    return record;
  },
};
const client = createScheduledTaskClient(scheduledTaskTransport(rawHost));
const input = { title: '任务', characterId: 'role', timezone: 'Asia/Shanghai', remindAt: '2030-01-01T09:00:00+08:00', recurrence: { type: 'daily' }, requestId: 'stable' };
assert.equal((await client.create(input)).id, record.id);
await client.update(record.id, record.generation, input);
assert.equal(calls.at(-1).input.generation, 3);
for (const method of ['pause', 'resume', 'cancel']) await client[method](record.id, 3);
assert.deepEqual(await client.execute(record.id, 3), { triggerId: 'trigger-1', status: 'waiting' });
assert.equal((await client.history()).items[0].state, 'partial');
assert.equal((await client.list()).total, 1);
for (const method of ['status', 'queue', 'clearBackpressure', 'capabilities']) await client[method]();
await client.preview(record.id);
const hostClient = new DefaultHostClient({ call: rawHost.call.bind(rawHost) }, {}, 'trace');
await hostClient.reminders.create(input);
assert.equal(calls.at(-1).method, 'host.reminder.create');
await hostClient.proactiveTasks.create(input);
assert.equal(calls.at(-1).method, 'host.proactive_task.create');
await assert.rejects(() => new DefaultHostClient({}, {}, 'trace').proactiveTasks.create(input), /传输未接通/);
for (const invalid of [undefined, {}, { ...record, saved: false }, { ...record, generation: 0 }, { ...record, id: 1 }]) {
  await assert.rejects(() => createScheduledTaskClient({ call: async () => invalid }).create(input), /响应为空|保存未确认/);
}
await assert.rejects(() => createScheduledTaskClient({ call: async () => ({}) }).execute('task', 3), /执行请求未确认/);
await assert.rejects(() => createScheduledTaskClient({ call: async () => [] }).capabilities(), /响应无效/);
await assert.rejects(() => createScheduledTaskClient({ call: async () => { throw Error('permission denied'); } }).pause('task', 3), /permission denied/);
console.log('scheduled-tasks-sdk=ok');
