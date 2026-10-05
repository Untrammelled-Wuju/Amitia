package task_runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func actualSourceTaskHost(t *testing.T, handler string) *TaskProcessHost {
	t.Helper()
	host := fixtureProcessHost(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	sourceRoot, err := filepath.Abs(filepath.Join(cwd, "../../../../../runtime/task-host"))
	if err != nil {
		t.Fatal(err)
	}
	compiler := filepath.Join(sourceRoot, "node_modules/typescript/lib/typescript.js")
	if _, err := os.Stat(compiler); err != nil {
		t.Fatal("项目 Task Host TypeScript 依赖不可用")
	}
	encodedRoot, _ := json.Marshal(sourceRoot)
	encodedCompiler, _ := json.Marshal(compiler)
	launcher := fmt.Sprintf(`
const fs=require('node:fs');
const path=require('node:path');
const {pathToFileURL,fileURLToPath}=require('node:url');
const {registerHooks}=require('node:module');
const ts=require(%s);
const sourceRoot=path.join(%s,'src');
registerHooks({
 resolve(specifier,context,nextResolve){
  if(context.parentURL && context.parentURL.startsWith('file:')){
   const parent=fileURLToPath(context.parentURL);
   if(parent.startsWith(sourceRoot+path.sep) && specifier.startsWith('./') && specifier.endsWith('.js')){
    return {url:pathToFileURL(path.resolve(path.dirname(parent),specifier.slice(0,-3)+'.ts')).href,shortCircuit:true};
   }
  }
  return nextResolve(specifier,context);
 },
 load(url,context,nextLoad){
  if(url.startsWith('file:')){
   const file=fileURLToPath(url);
   if(file.startsWith(sourceRoot+path.sep) && file.endsWith('.ts')){
    const source=ts.transpileModule(fs.readFileSync(file,'utf8'),{compilerOptions:{module:ts.ModuleKind.ES2022,target:ts.ScriptTarget.ES2022}}).outputText;
    return {source,format:'module',shortCircuit:true};
   }
  }
  return nextLoad(url,context);
 }
});
import(pathToFileURL(path.join(sourceRoot,'bootstrap.ts')).href).then(m=>m.bootstrap()).catch(()=>process.exit(2));
`, encodedCompiler, encodedRoot)
	if err := os.WriteFile(host.config.HostPath, []byte(launcher), 0600); err != nil {
		t.Fatal(err)
	}
	host.config.EntryPath = filepath.Join(host.config.WorkDir, "task-handler.cjs")
	host.config.Generation = 9
	if err := os.WriteFile(host.config.EntryPath, []byte(handler), 0600); err != nil {
		t.Fatal(err)
	}
	host.config.EntryHash = "sha256:" + hashBytes([]byte(handler))
	return host
}

func TestTaskProcessActualHostStorageUsesAcknowledgedOwnerPort(t *testing.T) {
	host := actualSourceTaskHost(t, `module.exports=async(input,ctx)=>{
 const missing=await ctx.storage.get('cursor');
 await ctx.storage.set('cursor',{private:input.value});
 const saved=await ctx.storage.get('cursor');
 await ctx.storage.delete('cursor');
 const removed=await ctx.storage.get('cursor');
 return {success:true,output:{missing,saved,removed}};
};`)
	_, _, authority, run, definition := taskAuthorityFixture(t)
	run.TaskRunID, run.TaskDefinitionID = host.config.TaskRunID, definition.TaskID
	run.Input = json.RawMessage(`{"value":"owner-only"}`)
	run.InputHash, run.Generation, run.ExecutionAttemptID = hashBytes(run.Input), host.config.Generation, "attempt"
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(definition)
	data := &taskInputData{}
	ctx, cancel := context.WithTimeout(coordination.WithScope(t.Context(), authority), 15*time.Second)
	defer cancel()
	if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Input = nil
	port := AcknowledgedTaskStoragePort{Data: data}
	var requests int
	var result json.RawMessage
	if err := host.Start(ctx, json.RawMessage(`{"value":"owner-only"}`), nil, nil, 1, 1, ProcessCallbacks{
		OnRequest: func(current context.Context, requestID, method string, params json.RawMessage) (json.RawMessage, error) {
			requests++
			return port.Call(current, run, requestID, method, params)
		},
		OnFinished: func(status string, output json.RawMessage, _, _, message string) {
			if status != "succeeded" {
				t.Error("source Task Host storage failed:", message)
			}
			result = append(json.RawMessage(nil), output...)
		},
	}); err != nil {
		t.Fatal(err)
	}
	if code, err := host.Wait(); code != 0 || err != nil || requests != 5 {
		t.Fatalf("source Task Host storage RPC failed: %d %v requests=%d", code, err, requests)
	}
	var output struct {
		Missing any `json:"missing"`
		Saved   struct {
			Private string `json:"private"`
		} `json:"saved"`
		Removed any `json:"removed"`
	}
	if err := json.Unmarshal(result, &output); err != nil || output.Missing != nil || output.Removed != nil || output.Saved.Private != "owner-only" {
		t.Fatalf("source Task Host storage result changed: %s %v", result, err)
	}
}

func TestTaskProcessActualHostRechecksAuthorityWhileHandlerIsIdle(t *testing.T) {
	host := actualSourceTaskHost(t, `module.exports=async(input,ctx)=>{
 await ctx.progress.report({current:0,total:1,stage:'started'});
 await new Promise((resolve,reject)=>ctx.signal.addEventListener('abort',()=>reject(new Error('aborted')),{once:true}));
 return {success:true,output:{late:true}};
};`)
	var valid atomic.Bool
	valid.Store(true)
	_, _, authority, _, _ := taskAuthorityFixture(t)
	ctx, cancel := context.WithTimeout(coordination.WithAdditionalGuard(coordination.WithScope(t.Context(), authority), func(context.Context) error {
		if !valid.Load() {
			return coordination.ErrScopeExpired
		}
		return nil
	}), 15*time.Second)
	defer cancel()
	started := make(chan struct{}, 1)
	var completed atomic.Int32
	if err := host.Start(ctx, json.RawMessage(`{}`), nil, nil, 1, 1, ProcessCallbacks{
		OnProgress: func(_ int64, _, _, _ *float64, _, _ string) {
			select {
			case started <- struct{}{}:
			default:
			}
		},
		OnFinished: func(_ string, _ json.RawMessage, _, _, _ string) { completed.Add(1) },
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		host.ForceStop()
		t.Fatal("source handler did not enter idle execution")
	}
	valid.Store(false)
	stopped := make(chan error, 1)
	go func() { _, err := host.Wait(); stopped <- err }()
	select {
	case err := <-stopped:
		if err == nil || completed.Load() != 0 {
			t.Fatalf("expired idle execution published a result: %v", err)
		}
	case <-time.After(2 * time.Second):
		host.ForceStop()
		t.Fatal("idle execution did not stop after authority changed")
	}
}

func TestTaskProcessActualHostLargeUnicodeResultUsesOwnerArtifact(t *testing.T) {
	host := actualSourceTaskHost(t, `module.exports=async(input,ctx)=>({success:true,output:{private:'中'.repeat(24000)}});`)
	_, _, authority, run, definition := taskAuthorityFixture(t)
	run.TaskRunID, run.TaskDefinitionID = host.config.TaskRunID, definition.TaskID
	run.Input = json.RawMessage(`{}`)
	run.InputHash, run.Generation, run.ExecutionAttemptID = hashBytes(run.Input), host.config.Generation, "attempt"
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(definition)
	ctx, cancel := context.WithTimeout(coordination.WithScope(t.Context(), authority), 15*time.Second)
	defer cancel()
	data := &taskInputData{}
	if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Input = nil
	port := AcknowledgedTaskArtifactPort{Data: data}
	var artifactID string
	var result json.RawMessage
	if err := host.Start(ctx, json.RawMessage(`{}`), nil, nil, 1, 1, ProcessCallbacks{
		OnRequest: func(current context.Context, requestID, method string, params json.RawMessage) (json.RawMessage, error) {
			return port.Call(current, run, requestID, method, params)
		},
		OnFinished: func(status string, output json.RawMessage, artifact, _, message string) {
			if status != "succeeded" {
				t.Error("large unicode source result failed:", message)
			}
			artifactID, result = artifact, append(json.RawMessage(nil), output...)
		},
	}); err != nil {
		t.Fatal(err)
	}
	if code, err := host.Wait(); code != 0 || err != nil || artifactID == "" || len(result) != 0 {
		t.Fatalf("large source result copied into inline output: %d %v %s", code, err, result)
	}
	content, hash, err := port.Result(ctx, run, artifactID)
	if err != nil || hash != hashBytes(content) || len(content) <= 64<<10 {
		t.Fatalf("large source result missing at owner: %v", err)
	}
	var output struct {
		Private string `json:"private"`
	}
	if json.Unmarshal(content, &output) != nil || output.Private != strings.Repeat("中", 24000) {
		t.Fatal("owner unicode artifact output changed")
	}
}

func TestTaskProcessActualHostSourceLoadsHandlerAndRestoresCheckpoint(t *testing.T) {
	host := actualSourceTaskHost(t, `module.exports=async(input,ctx)=>{
 const previous=await ctx.checkpoint.load();
 await ctx.progress.report({current:1,total:1,stage:'done'});
 await ctx.checkpoint.save({data:{value:input.value}});
 return {success:true,output:{previous:previous.cursor,value:input.value,attempt:ctx.attempt}};
};`)
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	var checkpoints, progress atomic.Int32
	var result json.RawMessage
	if err := host.Start(ctx, json.RawMessage(`{"value":42}`), json.RawMessage(`{"cursor":7,"data":{"value":1}}`), nil, 2, 3, ProcessCallbacks{
		OnProgress: func(_ int64, _, _, _ *float64, _, _ string) { progress.Add(1) },
		OnCheckpointConfirmed: func(version int64, payload json.RawMessage, hash string) error {
			if version != 8 || hash == "" || !json.Valid(payload) {
				t.Errorf("actual host checkpoint invalid: %d %s", version, payload)
			}
			checkpoints.Add(1)
			return nil
		},
		OnFinished: func(status string, value json.RawMessage, _, _, _ string) {
			if status != "succeeded" {
				t.Error("actual host failed:", status)
			}
			result = append(json.RawMessage(nil), value...)
		},
	}); err != nil {
		t.Fatal(err)
	}
	if code, err := host.Wait(); code != 0 || err != nil || checkpoints.Load() != 1 || progress.Load() != 1 {
		t.Fatalf("actual source host failed: %d %v checkpoints=%d progress=%d", code, err, checkpoints.Load(), progress.Load())
	}
	var output struct{ Previous, Value, Attempt int }
	if err := json.Unmarshal(result, &output); err != nil || output.Previous != 7 || output.Value != 42 || output.Attempt != 2 {
		t.Fatalf("actual host result mismatch: %s %v", result, err)
	}
}

func TestTaskProcessActualHostPauseNeverPublishesCancelledResult(t *testing.T) {
	host := actualSourceTaskHost(t, `module.exports=async(input,ctx)=>{
 await ctx.checkpoint.save({data:{value:input.value}});
 await new Promise((resolve,reject)=>ctx.signal.addEventListener('abort',()=>reject(new Error('aborted')),{once:true}));
 return {success:true,output:'unexpected'};
};`)
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	checkpointReady := make(chan struct{}, 1)
	var completed atomic.Int32
	if err := host.Start(ctx, json.RawMessage(`{"value":42}`), nil, nil, 1, 1, ProcessCallbacks{
		OnCheckpointConfirmed: func(version int64, _ json.RawMessage, _ string) error {
			if version == 1 {
				checkpointReady <- struct{}{}
			}
			return nil
		},
		OnFinished: func(_ string, _ json.RawMessage, _, _, _ string) { completed.Add(1) },
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-checkpointReady:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	version, err := host.Pause(ctx)
	if err != nil || version != 2 || host.State() != "paused" || completed.Load() != 0 {
		t.Fatalf("actual host pause published a result or failed: %d %v %s results=%d", version, err, host.State(), completed.Load())
	}
}

func TestTaskProcessActualHostUnconfirmedCheckpointRetainsPreviousCursor(t *testing.T) {
	host := actualSourceTaskHost(t, `module.exports=async(input,ctx)=>{
 let refused=false;
 try {await ctx.checkpoint.save({data:{private:'not-confirmed'}});} catch {refused=true;}
 const previous=await ctx.checkpoint.load();
 return {success:true,output:{refused,cursor:previous.cursor}};
};`)
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	var confirmations atomic.Int32
	var output json.RawMessage
	if err := host.Start(ctx, json.RawMessage(`{}`), json.RawMessage(`{"cursor":7,"data":{"confirmed":true}}`), nil, 1, 1, ProcessCallbacks{
		OnCheckpointConfirmed: func(_ int64, _ json.RawMessage, _ string) error {
			confirmations.Add(1)
			return errors.New("所有者离线，保存结果尚未确认")
		},
		OnFinished: func(status string, result json.RawMessage, _, _, _ string) {
			if status != "succeeded" {
				t.Error("拒绝结果未返回调用方")
			}
			output = append(json.RawMessage(nil), result...)
		},
	}); err != nil {
		t.Fatal(err)
	}
	if code, err := host.Wait(); err != nil || code != 0 || confirmations.Load() != 1 {
		t.Fatalf("所有者拒绝未传回宿主: %d %v calls=%d", code, err, confirmations.Load())
	}
	var result struct {
		Refused bool
		Cursor  int
	}
	if err := json.Unmarshal(output, &result); err != nil || !result.Refused || result.Cursor != 7 {
		t.Fatalf("未确认检查点推进了游标: %s %v", output, err)
	}
}
