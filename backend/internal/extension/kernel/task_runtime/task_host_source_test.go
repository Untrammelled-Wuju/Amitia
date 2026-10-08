package task_runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
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
	compile := fmt.Sprintf(`const fs=require('node:fs'),path=require('node:path'),ts=require(%s),root=path.join(%s,'src'),out=process.argv[1];for(const file of fs.readdirSync(root)){if(!file.endsWith('.ts'))continue;fs.writeFileSync(path.join(out,file.slice(0,-3)+'.js'),ts.transpileModule(fs.readFileSync(path.join(root,file),'utf8'),{compilerOptions:{module:ts.ModuleKind.ES2022,target:ts.ScriptTarget.ES2022}}).outputText);}fs.writeFileSync(path.join(out,'package.json'),JSON.stringify({type:'module'}));`, encodedCompiler, encodedRoot)
	if output, err := exec.Command(host.config.NodePath, "-e", compile, host.config.WorkDir).CombinedOutput(); err != nil {
		t.Fatalf("test TaskHost preparation failed: %v %s", err, output)
	}
	launcher := `import('./bootstrap.js').then(m=>m.bootstrap()).catch(error=>{console.error(error);process.exit(2)});`
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

func TestTaskProcessActualHostVerifiesGoBundlePinAndImportedBytes(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(fmt.Sprintf("dependency_changed_%t", changed), func(t *testing.T) {
			host := actualSourceTaskHost(t, `module.exports=async()=>({success:true,output:{value:require('./私有依赖.cjs')}});`)
			dependency := filepath.Join(host.config.WorkDir, "私有依赖.cjs")
			if err := os.WriteFile(dependency, []byte(`module.exports="original";`), 0600); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"\ue000.txt", "\U00010000.txt"} {
				if err := os.WriteFile(filepath.Join(host.config.WorkDir, name), []byte(name), 0600); err != nil {
					t.Fatal(err)
				}
			}
			hash, err := TaskBundleHash(t.Context(), host.config.WorkDir)
			if err != nil {
				t.Fatal(err)
			}
			host.config.BundleRoot, host.config.BundleHash = host.config.WorkDir, hash
			if changed {
				if err := os.WriteFile(dependency, []byte(`module.exports="changed";`), 0600); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			var completed atomic.Int32
			var status string
			var body json.RawMessage
			if err := host.Start(ctx, json.RawMessage(`{}`), nil, nil, 1, 1, ProcessCallbacks{OnFinished: func(state string, output json.RawMessage, _, _, _ string) {
				status, body = state, append(json.RawMessage(nil), output...)
				completed.Add(1)
			}}); err != nil {
				t.Fatal(err)
			}
			code, err := host.Wait()
			if err != nil || code != 0 || completed.Load() != 1 {
				t.Fatalf("actual bundle host failed: %d %v %s", code, err, status)
			}
			if changed {
				if status != "failed" {
					t.Fatalf("changed dependency executed: %s %s", status, body)
				}
			} else if status != "succeeded" || !strings.Contains(string(body), "original") {
				t.Fatalf("Go and Node bundle pins disagree: %s %s", status, body)
			}
		})
	}
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
