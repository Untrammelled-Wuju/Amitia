package task_runtime

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestOwnedRuntimeBindingDeniesBeforeInitializationAndKeepsAtomicDependencies(t *testing.T) {
	binding := &OwnedRuntimeBinding{}
	config := DefaultTaskRuntimeConfig()
	binding.Apply(&config)
	_, _, authority, run, definition := taskAuthorityFixture(t)
	run.TaskDefinitionID = definition.TaskID
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(definition)
	run.Input = json.RawMessage(`{"private":"owner-input"}`)
	run.InputHash = hashBytes(run.Input)
	ctx := coordination.WithScope(t.Context(), authority)
	if err := config.OwnedInputs.SaveInput(ctx, run); !IsTaskErrorCode(err, ErrTaskScopeDenied) {
		t.Fatalf("依赖未就绪时允许保存: %v", err)
	}
	if _, _, err := config.OwnedExecutionGuard(ctx, authority, run); !IsTaskErrorCode(err, ErrTaskScopeDenied) {
		t.Fatalf("依赖未就绪时允许恢复授权: %v", err)
	}
	guard := func(parent context.Context, expected coordination.ExecutionScope, _ *TaskRun) (context.Context, func(), error) {
		return coordination.WithScope(parent, expected), func() {}, nil
	}
	if err := binding.Bind(nil, &taskInputData{}); !IsTaskErrorCode(err, ErrTaskScopeDenied) {
		t.Fatalf("不完整依赖被绑定: %v", err)
	}
	data := &taskInputData{}
	var successes atomic.Int32
	var workers sync.WaitGroup
	for i := 0; i < 32; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if binding.Bind(guard, data) == nil {
				successes.Add(1)
			}
		}()
	}
	workers.Wait()
	if successes.Load() != 1 {
		t.Fatalf("运行依赖发生重复绑定: %d", successes.Load())
	}
	if err := config.OwnedInputs.SaveInput(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Input = nil
	if input, err := config.OwnedInputs.Input(ctx, run); err != nil || string(input) != `{"private":"owner-input"}` {
		t.Fatalf("已初始化端口没有使用原数据所有者: %s %v", input, err)
	}
	if err := binding.Bind(guard, &taskInputData{}); !IsTaskErrorCode(err, ErrTaskScopeDenied) {
		t.Fatalf("运行期间替换了所有者依赖: %v", err)
	}
	if restored, finish, err := config.OwnedExecutionGuard(ctx, authority, run); err != nil {
		t.Fatal(err)
	} else {
		finish()
		if scope, ok := coordination.FromContext(restored); !ok || scope != authority {
			t.Fatal("运行绑定改变了原授权范围")
		}
	}
}
