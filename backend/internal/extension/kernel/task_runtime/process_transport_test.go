package task_runtime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const processFixture = `
const readline = require('node:readline');
const send = (method, params) => process.stdout.write(JSON.stringify({jsonrpc:'2.0', method, params})+'\n');
send('runtime.hello',{protocol_version:'2.0',generation:Number(process.env.AMITIA_GENERATION),instance_id:process.env.AMITIA_INSTANCE_ID,nonce:process.env.AMITIA_NONCE,definition_hash:process.env.AMITIA_DEFINITION_HASH,runtime_type:'task'});
readline.createInterface({input:process.stdin}).on('line', line => {
 const message=JSON.parse(line);
 if(message.method==='host.welcome') send('runtime.ready',{session_id:message.params.session_id});
 if(message.method==='task.execute') {
  const p=message.params;
  if(p.input.mode==='early') process.exit(0);
  if(p.input.mode==='hang') return;
  if(p.input.mode==='failed') {send('task.finished',{task_run_id:p.task_run_id,status:'failed',error:{code:'expected',message:'failure'}});process.exit(1);}
  const id=p.input.mode==='wrong'?'another-run':p.task_run_id;
  send('task.progress',{task_run_id:id,sequence:1,current:1,total:2});
  send('task.checkpoint',{task_run_id:id,version:1,payload:{cursor:1}});
  send('task.finished',{task_run_id:id,status:'succeeded',result:{mode:'inline_json',data:{input:p.input,checkpoint:p.checkpoint,attempt:p.attempt}}});
  process.exit(0);
 }
});
`

func fixtureProcessHost(t *testing.T) *TaskProcessHost {
	t.Helper()
	node := os.Getenv("AMITIA_TEST_NODE")
	if node == "" {
		t.Skip("项目 Node 测试路径未配置")
	}
	dir := t.TempDir()
	hostPath := filepath.Join(dir, "task-host.cjs")
	if err := os.WriteFile(hostPath, []byte(processFixture), 0600); err != nil {
		t.Fatal(err)
	}
	host, err := NewTaskProcessHost(ProcessHostConfig{InstanceID: "instance", TaskRunID: "run", ExtensionID: "extension", ModuleID: "module", DefHash: "hash", NodePath: node, HostPath: hostPath, EntryPath: hostPath, WorkDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	return host
}

func TestTaskProcessRunsRealNodeAndConfirmsResultAfterExit(t *testing.T) {
	host := fixtureProcessHost(t)
	host.config.Generation = 5
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	var progress, checkpoints atomic.Int32
	var result json.RawMessage
	err := host.Start(ctx, json.RawMessage(`{"mode":"normal","value":42}`), json.RawMessage(`{"cursor":7}`), nil, 2, 3, ProcessCallbacks{
		OnProgress: func(_ int64, _, _, _ *float64, _, _ string) { progress.Add(1) },
		OnCheckpoint: func(_ int64, _ json.RawMessage, hash string) {
			if hash == "" {
				t.Error("missing checkpoint hash")
			}
			checkpoints.Add(1)
		},
		OnFinished: func(status string, value json.RawMessage, _, _, _ string) {
			if status != "succeeded" {
				t.Error(status)
			}
			result = append(json.RawMessage(nil), value...)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	code, err := host.Wait()
	if err != nil || code != 0 || host.State() != "finished" || progress.Load() != 1 || checkpoints.Load() != 1 {
		t.Fatalf("code=%d error=%v state=%s", code, err, host.State())
	}
	var value struct {
		Input      struct{ Value int }
		Checkpoint struct{ Cursor int }
		Attempt    int
	}
	if json.Unmarshal(result, &value) != nil || value.Input.Value != 42 || value.Checkpoint.Cursor != 7 || value.Attempt != 2 {
		t.Fatalf("result=%s", result)
	}
}

func TestTaskProcessRejectsWrongProtocolAndStaleGenerationBeforeExecution(t *testing.T) {
	for _, test := range []struct{ name, old, next string }{
		{"protocol", "protocol_version:'2.0'", "protocol_version:'1.0'"},
		{"generation", "generation:Number(process.env.AMITIA_GENERATION)", "generation:Number(process.env.AMITIA_GENERATION)-1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			host := fixtureProcessHost(t)
			host.config.Generation = 7
			if err := os.WriteFile(host.config.HostPath, []byte(strings.Replace(processFixture, test.old, test.next, 1)), 0600); err != nil {
				t.Fatal(err)
			}
			var completed, progressed atomic.Int32
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			if err := host.Start(ctx, json.RawMessage(`{"mode":"normal"}`), nil, nil, 1, 1, ProcessCallbacks{
				OnFinished: func(_ string, _ json.RawMessage, _, _, _ string) { completed.Add(1) },
				OnProgress: func(_ int64, _, _, _ *float64, _, _ string) { progressed.Add(1) },
			}); err != nil {
				t.Fatal(err)
			}
			if code, err := host.Wait(); code == 0 || err == nil || completed.Load() != 0 || progressed.Load() != 0 {
				t.Fatalf("invalid handshake executed task: %d %v", code, err)
			}
		})
	}
}

func TestTaskProcessRejectsUnconfirmedOrForeignResults(t *testing.T) {
	for _, mode := range []string{"early", "wrong"} {
		t.Run(mode, func(t *testing.T) {
			host := fixtureProcessHost(t)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			var completed atomic.Int32
			if err := host.Start(ctx, json.RawMessage(`{"mode":"`+mode+`"}`), nil, nil, 1, 1, ProcessCallbacks{OnFinished: func(_ string, _ json.RawMessage, _, _, _ string) { completed.Add(1) }}); err != nil {
				t.Fatal(err)
			}
			code, err := host.Wait()
			if err == nil || code == 0 || completed.Load() != 0 {
				t.Fatalf("unconfirmed result accepted: %d %v", code, err)
			}
		})
	}
}

func TestPinnedTaskProcessRejectsMissingHostCapabilitiesBeforeExecution(t *testing.T) {
	for _, features := range []string{"[]", "['entry_pin']", "['checkpoint_ack']", "['entry_pin','checkpoint_ack']"} {
		t.Run(features, func(t *testing.T) {
			host := fixtureProcessHost(t)
			host.config.EntryHash = strings.Repeat("a", 64)
			host.config.BundleRoot, host.config.BundleHash = host.config.WorkDir, strings.Repeat("b", 64)
			data := strings.Replace(processFixture, "runtime_type:'task'", "runtime_type:'task',features:"+features, 1)
			if err := os.WriteFile(host.config.HostPath, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			var progressed, finished atomic.Int32
			if err := host.Start(ctx, json.RawMessage(`{"mode":"normal"}`), nil, nil, 1, 1, ProcessCallbacks{
				OnProgress: func(_ int64, _, _, _ *float64, _, _ string) { progressed.Add(1) },
				OnFinished: func(_ string, _ json.RawMessage, _, _, _ string) { finished.Add(1) },
			}); err != nil {
				t.Fatal(err)
			}
			if code, err := host.Wait(); code == 0 || err == nil || progressed.Load() != 0 || finished.Load() != 0 {
				t.Fatalf("host without required capabilities executed: %d %v", code, err)
			}
		})
	}
}

func TestTaskProcessCancellationStopsChildAndDoesNotConfirmResult(t *testing.T) {
	host := fixtureProcessHost(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	var completed atomic.Int32
	if err := host.Start(ctx, json.RawMessage(`{"mode":"hang"}`), nil, nil, 1, 1, ProcessCallbacks{OnFinished: func(_ string, _ json.RawMessage, _, _, _ string) { completed.Add(1) }}); err != nil {
		t.Fatal(err)
	}
	if err := host.Cancel(ctx, "user"); err != nil {
		t.Fatal(err)
	}
	code, err := host.Wait()
	if err == nil || code == 0 || completed.Load() != 0 {
		t.Fatalf("cancelled result accepted: %d %v", code, err)
	}
}

func TestTaskProcessRejectsForgedHandshakeBeforeExecution(t *testing.T) {
	host := fixtureProcessHost(t)
	data := strings.Replace(processFixture, "nonce:process.env.AMITIA_NONCE", "nonce:'forged'", 1)
	if err := os.WriteFile(host.config.HostPath, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	var completed atomic.Int32
	if err := host.Start(ctx, json.RawMessage(`{"mode":"normal"}`), nil, nil, 1, 1, ProcessCallbacks{OnFinished: func(_ string, _ json.RawMessage, _, _, _ string) { completed.Add(1) }}); err != nil {
		t.Fatal(err)
	}
	code, err := host.Wait()
	if err == nil || code == 0 || completed.Load() != 0 {
		t.Fatalf("forged handshake accepted: %d %v", code, err)
	}
}

func TestTaskProcessStoppedBeforeStartDoesNotHangWait(t *testing.T) {
	host := fixtureProcessHost(t)
	host.ForceStop()
	host.ForceStop()
	if err := host.Start(t.Context(), json.RawMessage(`{}`), nil, nil, 1, 1, ProcessCallbacks{}); err == nil {
		t.Fatal("stopped host started")
	}
	if _, err := host.Wait(); err != nil || host.State() != "stopped" {
		t.Fatalf("stopped host: %v %s", err, host.State())
	}
}

func TestTaskProcessPreservesConfirmedFailedExit(t *testing.T) {
	host := fixtureProcessHost(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	var completed atomic.Int32
	if err := host.Start(ctx, json.RawMessage(`{"mode":"failed"}`), nil, nil, 1, 1, ProcessCallbacks{OnFinished: func(status string, _ json.RawMessage, _, code, _ string) {
		if status != "failed" || code != "expected" {
			t.Error("failed result changed")
		}
		completed.Add(1)
	}}); err != nil {
		t.Fatal(err)
	}
	code, err := host.Wait()
	if err != nil || code != 0 || completed.Load() != 1 {
		t.Fatalf("confirmed failure treated as crash: %d %v", code, err)
	}
}

func TestTaskProcessCancellationBeforeStartPreventsExecution(t *testing.T) {
	host := fixtureProcessHost(t)
	if err := host.Cancel(t.Context(), "cancel before start"); err != nil {
		t.Fatal(err)
	}
	if err := host.Start(t.Context(), json.RawMessage(`{}`), nil, nil, 1, 1, ProcessCallbacks{}); err == nil {
		t.Fatal("cancelled host started")
	}
	code, err := host.Wait()
	if code == 0 || err == nil || host.State() != "cancelled" {
		t.Fatalf("cancel before start was not preserved: %d %v %s", code, err, host.State())
	}
}
