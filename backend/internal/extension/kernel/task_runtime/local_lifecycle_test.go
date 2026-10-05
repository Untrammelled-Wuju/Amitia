package task_runtime

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/script_host"
	"github.com/u-ai/backend/internal/scriptruntime/nodeenv"
)

type lifecycleNodeResolver struct{ path string }

func (r lifecycleNodeResolver) Resolve(context.Context) (nodeenv.Environment, error) {
	return nodeenv.Environment{NodeBinary: r.path}, nil
}

type lifecycleHostResolver struct{ path string }

func (r lifecycleHostResolver) Resolve(context.Context, script_host.Kind) (script_host.Artifact, error) {
	return script_host.Artifact{EntryPath: r.path}, nil
}

func TestTaskLocalLifecycleStartsWithoutCheckpointAndContinuesProgressSequence(t *testing.T) {
	host := fixtureProcessHost(t)
	store := &pauseStore{run: &TaskRun{TaskRunID: "run", TaskDefinitionID: "task", Status: RunStatusQueued, Generation: 3, Revision: 1, Input: json.RawMessage(`{"mode":"normal"}`), InputHash: "input", Attempt: 1, MaxAttempts: 1}, def: &TaskDefinition{TaskID: "task", DefinitionHash: "hash"}, progress: &TaskRunProgress{Sequence: 10}}
	config := DefaultTaskRuntimeConfig()
	config.WorkspaceRoot = t.TempDir()
	config.NodeEnvironmentResolver = lifecycleNodeResolver{host.config.NodePath}
	config.HostArtifactResolver = lifecycleHostResolver{host.config.HostPath}
	config.EntryResolver = func(context.Context, *TaskDefinition) (string, error) { return host.config.EntryPath, nil }
	svc := NewTaskRuntimeService(store, config)
	atomic.StoreInt32(&svc.dispatching, 1)
	svc.executeTaskRun(t.Context(), CloneTaskRun(store.run))
	if store.run.Status != RunStatusSucceeded || store.result == nil {
		t.Fatalf("queued execution did not complete: %+v", store.run)
	}
	if store.progress.Sequence != 11 {
		t.Fatalf("resumed sequence reset: %d", store.progress.Sequence)
	}
	if store.run.RuntimeInstanceID == nil || store.run.ExecutionAttemptID == "" || store.run.Revision < 5 {
		t.Fatal("execution identity was not persisted")
	}
}
