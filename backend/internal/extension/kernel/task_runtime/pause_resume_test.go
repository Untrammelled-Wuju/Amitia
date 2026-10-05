package task_runtime

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type pauseStore struct {
	TaskStore
	mu       sync.Mutex
	run      *TaskRun
	def      *TaskDefinition
	cp       *TaskCheckpoint
	queued   int
	result   *TaskRunResult
	progress *TaskRunProgress
}

func (s *pauseStore) GetTaskRun(context.Context, string) (*TaskRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return CloneTaskRun(s.run), nil
}
func (s *pauseStore) GetTaskDefinition(context.Context, string) (*TaskDefinition, error) {
	return s.def, nil
}
func (s *pauseStore) WithinTaskTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}
func (s *pauseStore) UpdateTaskRunCAS(_ context.Context, next *TaskRun, status TaskRunStatus, generation, revision int64) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.run.Status != status || s.run.Generation != generation || s.run.Revision != revision {
		return false, nil
	}
	s.run = CloneTaskRun(next)
	return true, nil
}
func (s *pauseStore) PutCheckpoint(_ context.Context, cp *TaskCheckpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cp = cp
	return nil
}
func (s *pauseStore) GetLatestCheckpoint(context.Context, string) (*TaskCheckpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cp, nil
}
func (s *pauseStore) RemoveFromQueue(context.Context, string) error      { return nil }
func (s *pauseStore) EnqueueTask(context.Context, *TaskQueueEntry) error { s.queued++; return nil }

func (s *pauseStore) GetProgress(context.Context, string) (*TaskRunProgress, error) {
	return s.progress, nil
}
func (s *pauseStore) PutProgress(_ context.Context, id string, seq int64, payload []byte) error {
	s.progress = &TaskRunProgress{TaskRunID: id, Sequence: seq, Details: append(json.RawMessage(nil), payload...)}
	return nil
}
func (s *pauseStore) PutResult(_ context.Context, result *TaskRunResult) error {
	s.result = result
	return nil
}
func (s *pauseStore) UpdateExecutionAttempt(_ context.Context, _ string, attempt TaskExecutionAttemptID, instance string, _ time.Time, next, expected int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.run.Revision != expected {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "changed")
	}
	s.run.ExecutionAttemptID = attempt
	s.run.RuntimeInstanceID = &instance
	s.run.Revision = next
	return nil
}

func TestTaskPauseWaitsForCheckpointAndRealProcessExitThenResumeQueuesNewGeneration(t *testing.T) {
	host := fixtureProcessHost(t)
	script := `
const readline=require('node:readline');
const send=(method,params)=>process.stdout.write(JSON.stringify({jsonrpc:'2.0',method,params})+'\n');
send('runtime.hello',{protocol_version:'2.0',generation:Number(process.env.AMITIA_GENERATION),instance_id:process.env.AMITIA_INSTANCE_ID,nonce:process.env.AMITIA_NONCE,definition_hash:process.env.AMITIA_DEFINITION_HASH,runtime_type:'task'});
readline.createInterface({input:process.stdin}).on('line',line=>{
 const m=JSON.parse(line);
 if(m.method==='host.welcome')send('runtime.ready',{session_id:m.params.session_id});
 if(m.method==='task.pause'){
  send('task.checkpoint',{task_run_id:'run',version:2,payload:{cursor:2,value:42}});
  send('task.pause_ack',{task_run_id:'run',checkpoint_version:2,paused:true});
  setTimeout(()=>process.exit(0),100);
 }
});`
	if err := os.WriteFile(host.config.HostPath, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	store := &pauseStore{run: &TaskRun{TaskRunID: "run", TaskDefinitionID: "task", Status: RunStatusRunning, Generation: 4, Revision: 7, InputHash: "input", ExecutionAttemptID: "attempt"}, def: &TaskDefinition{TaskID: "task", Checkpoint: true, DefinitionHash: "hash"}}
	svc := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig())
	run := CloneTaskRun(store.run)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if err := host.Start(ctx, json.RawMessage(`{}`), nil, nil, 1, 1, ProcessCallbacks{OnCheckpoint: func(version int64, payload json.RawMessage, hash string) {
		svc.handleCheckpoint(ctx, run, store.def, payload, hash, version)
	}}); err != nil {
		t.Fatal(err)
	}
	svc.activeHosts["run"] = host
	if err := svc.PauseTask(ctx, PauseTaskRequest{TaskRunID: "run", Generation: 4, Reason: "user"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-host.Done():
	default:
		t.Fatal("pause returned before process exit")
	}
	paused, _ := store.GetTaskRun(ctx, "run")
	if paused.Status != RunStatusPaused || paused.CheckpointID == nil || host.State() != "paused" {
		t.Fatalf("pause not confirmed: %+v", paused)
	}
	if err := svc.ResumeTask(ctx, ResumeTaskRequest{TaskRunID: "run", Generation: 4}); !IsTaskErrorCode(err, ErrTaskPauseInProgress) {
		t.Fatalf("resume before cleanup accepted: %v", err)
	}
	delete(svc.activeHosts, "run")
	atomic.StoreInt32(&svc.dispatching, 1)
	if err := svc.ResumeTask(ctx, ResumeTaskRequest{TaskRunID: "run", Generation: 4}); err != nil {
		t.Fatal(err)
	}
	resumed, _ := store.GetTaskRun(ctx, "run")
	if resumed.Status != RunStatusQueued || resumed.Generation != 5 || resumed.ExecutionAttemptID != "" || store.queued != 1 {
		t.Fatalf("resume did not queue a new execution: %+v", resumed)
	}
}

func TestTaskPauseWithoutProcessNeverReportsPaused(t *testing.T) {
	store := &pauseStore{run: &TaskRun{TaskRunID: "run", TaskDefinitionID: "task", Status: RunStatusRunning}, def: &TaskDefinition{TaskID: "task", Checkpoint: true}}
	svc := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig())
	if err := svc.PauseTask(t.Context(), PauseTaskRequest{TaskRunID: "run"}); err == nil {
		t.Fatal("missing process reported pause success")
	}
	if store.run.Status != RunStatusRecoveryRequired {
		t.Fatalf("unconfirmed pause status: %s", store.run.Status)
	}
}

func TestTaskProcessRejectsPauseWithoutCheckpointConfirmation(t *testing.T) {
	host := fixtureProcessHost(t)
	script := `const readline=require('node:readline');const send=(method,params)=>process.stdout.write(JSON.stringify({jsonrpc:'2.0',method,params})+'\n');send('runtime.hello',{protocol_version:'2.0',generation:Number(process.env.AMITIA_GENERATION),instance_id:process.env.AMITIA_INSTANCE_ID,nonce:process.env.AMITIA_NONCE,definition_hash:process.env.AMITIA_DEFINITION_HASH,runtime_type:'task'});readline.createInterface({input:process.stdin}).on('line',line=>{const m=JSON.parse(line);if(m.method==='host.welcome')send('runtime.ready',{session_id:m.params.session_id});if(m.method==='task.pause'){send('task.pause_ack',{task_run_id:'run',checkpoint_version:7,paused:true});process.exit(0);}});`
	if err := os.WriteFile(host.config.HostPath, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if err := host.Start(ctx, json.RawMessage(`{}`), nil, nil, 1, 1, ProcessCallbacks{}); err != nil {
		t.Fatal(err)
	}
	if _, err := host.Pause(ctx); err == nil {
		t.Fatal("unpersisted pause acknowledged")
	}
	if host.State() == "paused" {
		t.Fatal("invalid pause became paused")
	}
}

func TestTaskCancelAndWaitConfirmsActualNodeQuiescenceAndTerminalStatus(t *testing.T) {
	host := fixtureProcessHost(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	store := &pauseStore{run: &TaskRun{TaskRunID: "run", Status: RunStatusRunning, Generation: 2, ExecutionAttemptID: "attempt"}}
	svc := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig())
	svc.activeHosts["run"] = host
	if err := host.Start(ctx, json.RawMessage(`{"mode":"hang"}`), nil, nil, 1, 1, ProcessCallbacks{}); err != nil {
		t.Fatal(err)
	}
	if err := svc.CancelAndWait(ctx, "run", "user"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-host.Done():
	default:
		t.Fatal("cancel acknowledged before exit")
	}
	live, _ := store.GetTaskRun(ctx, "run")
	if live.Status != RunStatusCancelled {
		t.Fatalf("cancelled process retained non-terminal status: %s", live.Status)
	}
}
