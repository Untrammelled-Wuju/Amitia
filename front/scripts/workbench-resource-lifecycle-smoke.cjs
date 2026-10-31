const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const ts = (() => { try { return require('typescript'); } catch (error) {
  console.error("[original-error]", error);
return require(path.resolve(path.dirname(process.execPath), '../lib/node_modules/typescript')); } })();
const root = path.resolve(__dirname, '..');
const componentPath = path.join(root, 'src/components/workbench/WorkbenchResourceView.vue');
const historyPath = path.join(root, 'src/components/workbench/WorkbenchTasksView.vue');
const helperPath = path.join(root, 'src/components/workbench/workbench-resource-lifecycle.ts');
function compile(code, filename) {
  const result = ts.transpileModule(code, { fileName: filename, compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS }, reportDiagnostics: true });
  const errors = (result.diagnostics || []).filter(item => item.category === ts.DiagnosticCategory.Error);
  assert.equal(errors.length, 0, ts.formatDiagnostics(errors, { getCanonicalFileName: x => x, getCurrentDirectory: () => root, getNewLine: () => '\n' }));
  return result.outputText;
}
function script(filepath) {
  const source = fs.readFileSync(filepath, 'utf8');
  const match = source.match(/<script setup lang="ts">([\s\S]*?)<\/script>/);
  assert.ok(match, `Vue script setup missing: ${filepath}`);
  return match[1];
}
function moduleFrom(source, filepath, requireFn, globals={}) {
  const mod = { exports: {} };
  vm.runInNewContext(compile(source, filepath), { module: mod, exports: mod.exports, require: requireFn, console, Promise, AbortController, setTimeout, clearTimeout, Date, URL, ...globals }, { filename: filepath });
  return mod.exports;
}
const lifecycle = moduleFrom(fs.readFileSync(helperPath, 'utf8'), helperPath, () => ({}));
let passed=0;
function check(message, action) { return Promise.resolve().then(action).then(() => { passed++; }); }
async function flush() { await new Promise(resolve => setImmediate(resolve)); await Promise.resolve(); }
function fakeVue(hooks) {
  return {
    ref: value => ({ value }),
    computed: getter => ({ get value() { return getter(); } }),
    watch: (fn, callback) => { hooks.watch = callback; },
    onActivated: cb => { hooks.activated = cb; },
    onDeactivated: cb => { hooks.deactivated = cb; },
    onBeforeUnmount: cb => { hooks.unmount = cb; },
  };
}
function startResource(kind, targetDeviceId='') {
  const calls=[];
  const hooks={};
  const props={ tab: { id: 't', kind, workspaceUri: 'ws://one', resourceUri: 'ws://one/a.ts', targetDeviceId } };
  const store={ conversationId:'conv-a', openTab(){}, tabs: [] };
  const apiClient={
    get: async (route) => { calls.push(route); if (route === '/api/workspaces') return { data: [{id:'a',name:'repo',rootUri:'ws://one',available:true,kind:'local'}] }; if (route === '/api/workspaces/list') return {data:{entries:[],hasMore:false}}; if (route === '/api/workspaces/read') return {data:{}}; throw Error(`unexpected ${route}`); },
    post: async route => { calls.push(route); return {data: route.endsWith('/status') ? { entries: [], clean:true } : { entries: [] }}; },
  };
  const modules = request => {
    if (request === 'vue') return fakeVue(hooks);
    if (request.includes('stores/workbench')) return {useWorkbenchStore:()=>store};
    if (request.includes('composables/useApi')) return {apiClient};
    if (request.includes('workbench-resource-lifecycle')) return lifecycle;
    if (request.includes('workbench-document-chunks')) return { parseWorkbenchDocumentChunk: () => ({isText: true,text:'',offset:0,hasMore:false}) };
    return {};
  };
  const body = script(componentPath);
  const exported = moduleFrom(body+'\nmodule.exports.__test = { initialize, clearResourceState };',componentPath,modules,{ defineProps: () => props });
  return { calls, hooks, props, state: exported.__test };
}
async function run() {
  await check('view-kind router does not fetch workspace for tasks, browser, terminal or file', () => {
    for(const kind of ['tasks','browser','terminal','file']) assert.equal(lifecycle.needsWorkbenchWorkspaceData(kind),false);
    for(const kind of ['files','git','diff']) assert.equal(lifecycle.needsWorkbenchWorkspaceData(kind),true);
    for(const kind of ['files','git','diff']) assert.equal(lifecycle.needsWorkbenchWorkspaceData(kind,'other-device'),false);
  });
  for (const kind of ['tasks','browser','terminal']) {
    await check(`opening ${kind} does not trigger Workspace/Git calls`, async()=>{
      const subject = startResource(kind);
      await flush();
      assert.deepEqual(subject.calls,[]);
      subject.hooks.activated();subject.hooks.deactivated();subject.hooks.activated();
      await flush();
      assert.deepEqual(subject.calls,[]);
    });
  }
  await check('remote file workspace is never sent to this device',async()=>{
    const x=startResource('files','other-device');await flush();assert.deepEqual(x.calls,[]);
  });
  await check('local files call mount and directory APIs, and refresh upon KeepAlive reactivation',async()=>{
    const x=startResource('files');await flush();
    assert.deepEqual(x.calls,['/api/workspaces','/api/workspaces/list']);
    x.hooks.activated();x.hooks.deactivated();x.hooks.activated();await flush();
    assert.deepEqual(x.calls,['/api/workspaces','/api/workspaces/list','/api/workspaces','/api/workspaces/list']);
  });
  await check('local Git calls only the existing dedicated Git service',async()=>{
    const x=startResource('git');await flush();
    assert.ok(x.calls.includes('/api/workspaces/git/status'));
    assert.ok(x.calls.includes('/api/workspaces/git/log'));
    assert.ok(x.calls.includes('/api/workspaces/git/worktrees'));
  });
  await check('source view cleanup invalidates outstanding requests',async()=>{
    const x=startResource('files');await flush();
    x.hooks.activated();x.hooks.deactivated();x.state.clearResourceState();
    x.hooks.unmount();
  });
  await check('Flutter checkpoint component key includes task update generation',()=>{
    const raw=fs.readFileSync(path.resolve(root,'../mobile_app/lib/features/workbench/presentation/pages/workbench_tasks_view.dart'),'utf8');
    assert.match(raw,/ValueKey\('\$\{widget\.conversationId\}:\$coordinationId:\$\{task\['updatedAt'\]\}'\)/);
  });
  await check('Web task view links expanded history to task update identity',()=>{
    const raw=script(historyPath);
    assert.match(raw,/expanded\.updatedAt !== historyTaskUpdatedAt/);
    assert.match(raw,/resetHistory\(\)/);
  });
  console.log(JSON.stringify({passed,failed:0,scope:'real Vue script setup evaluated in isolated Vue/API adapters; no full browser or Flutter runtime'},null,2));
}
run().catch(error=>{console.error(error);process.exitCode=1;});
