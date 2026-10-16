const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const ts = (() => { try { return require('typescript'); } catch { return require(path.resolve(path.dirname(process.execPath), '../lib/node_modules/typescript')); } })();
const file = path.resolve(__dirname,'../src/components/workbench/WorkbenchTasksView.vue');
const script = fs.readFileSync(file,'utf8').match(/<script setup lang="ts">([\s\S]*?)<\/script>/)?.[1];
assert.ok(script);
const output = ts.transpileModule(script+'\nmodule.exports.__test = { toggleHistory, load, tasks, history, historyBusy, expandedId, resetHistory };', {fileName:file, compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022}, reportDiagnostics:true});
assert.equal((output.diagnostics||[]).filter(d=>d.category===ts.DiagnosticCategory.Error).length,0);
const callbacks={};
const state={conversationId:'conv-a'};
const responses=[];
const historyCalls=[];
const listeners=[];
const vue={
  ref: value=>({value}),
  watch: (_,cb)=>{callbacks.changed=cb;},
  onActivated: cb=>{callbacks.activated=cb;},
  onDeactivated: cb=>{callbacks.deactivated=cb;},
  onBeforeUnmount: cb=>{callbacks.unmount=cb;},
};
const service={get:async()=>{const next=responses.shift();if(!next) throw Error('unplanned task request');return {data:next};}};
const modules=(request)=>{
  if(request==='vue') return vue;
  if(request.includes('useApi')) return {apiClient:service};
  if(request.includes('workbench-task-summary')) return {normalizeTaskSummary: () => ({total:0})};
  if(request.includes('workbench-task-list')) return {parseWorkbenchTaskList: data=>data};
  if(request.includes('workbench-task-checkpoints')) return {loadTaskCheckpoints: (_conv,id,signal)=>new Promise((resolve,reject)=>{historyCalls.push({id,signal,resolve,reject});})};
  if(request.includes('workbench-task-request')) return {withTaskRequestDeadline: task=>task(new AbortController().signal)};
  return {};
};
const fakeDocument={hidden:false,addEventListener(){},removeEventListener(){}};
const fakeWindow={addEventListener(){},removeEventListener(){}};
const sandbox={module:{exports:{}},exports:{},require:modules,defineProps:()=>state,document:fakeDocument,window:fakeWindow,
  AbortController,Promise,Date,setInterval:()=>1,clearInterval(){},console};
vm.runInNewContext(output.outputText,sandbox,{filename:file});
const subject=sandbox.module.exports.__test;
const tick=()=>new Promise(done=>setImmediate(done));
let passed=0;
async function check(name,fn){await fn();passed++;}
const task=(stamp)=>({coordinationId:'goal-a',updatedAt:stamp});
const data=items=>({tasks:items,summary:{total:items.length},truncated:false,liveObservationTruncated:false});
async function main(){
  await check('initial activation fetches current conversation',async()=>{
    responses.push(data([task('2026-10-14T12:00:00Z')]));callbacks.activated();await tick();
    assert.equal(subject.tasks.value[0].coordinationId,'goal-a');
  });
  await check('opening a history issues one request for selected task',async()=>{
    subject.toggleHistory('goal-a');await tick();
    assert.equal(historyCalls.length,1);
    assert.equal(historyCalls[0].id,'goal-a');
  });
  await check('a new task checkpoint version aborts its predecessor and requests fresh evidence',async()=>{
    responses.push(data([task('2026-10-14T12:01:00Z')]));await subject.load(true);await tick();
    assert.equal(historyCalls.length,2);
    assert.equal(historyCalls[0].signal.aborted,true);
    assert.equal(historyCalls[1].signal.aborted,false);
    assert.equal(subject.history.value,null);
  });
  await check('late old history response cannot replace current checkpoint records',async()=>{
    historyCalls[0].resolve({records:[{revision:'old'}]});await tick();
    assert.equal(subject.history.value,null);
    historyCalls[1].resolve({records:[{revision:'new'}]});await tick();
    assert.equal(subject.history.value.records[0].revision,'new');
    assert.equal(subject.historyBusy.value,false);
  });
  await check('when a task is removed, expanded history is cleared',async()=>{
    responses.push(data([]));await subject.load(true);await tick();
    assert.equal(subject.expandedId.value,'');
    assert.equal(subject.history.value,null);
  });
  await check('deactivating the view fences late responses',async()=>{
    responses.push(data([task('2026-10-14T12:03:00Z')]));await subject.load(true);
    subject.toggleHistory('goal-a');await tick();
    const pending=historyCalls.at(-1);
    callbacks.deactivated();pending.resolve({records:[{revision:'late'}]});await tick();
    assert.equal(subject.history.value,null);
    assert.equal(subject.expandedId.value,'');
  });
  callbacks.unmount();
  console.log(JSON.stringify({passed,failed:0,scope:'actual Vue task view script executed with simulated lifecycle and pending backend responses'},null,2));
}
main().catch(e=>{console.error(e);process.exitCode=1;});
