package task_runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func sourceTaskApprovalFixture(t *testing.T) (context.Context, SourceTaskApprovalBinding) {
	t.Helper()
	_, _, authority, _, _ := taskAuthorityFixture(t)
	binding := SourceTaskApprovalBinding{Scope: authority, TaskRunID: "run", TaskGeneration: 1, InputHash: hashBytes([]byte("private-input")), Target: TargetTaskDefinitionPin{DeviceID: authority.TargetDeviceID, TaskID: "source-task", ExtensionID: "extension", ModuleID: "module", InstalledGeneration: 2, DefinitionFingerprint: hashBytes([]byte("definition")), PortableFingerprint: hashBytes([]byte("portable"))}, ExecutionTarget: TaskExecutionTarget{SpaceID: "core", DeviceID: "device", RuntimeID: "runtime", RuntimeSessionID: "session", ConnectionGeneration: 1}}
	binding.ExecutionTarget.SpaceID, binding.ExecutionTarget.DeviceID = runtimeidentity.SpaceID(authority.CoreID), runtimeidentity.DeviceID(authority.TargetDeviceID)
	return coordination.WithScope(t.Context(), authority), binding
}

func TestSourceTaskSingleApprovalClaimsOneConcurrentExecutionAndRejectsReplay(t *testing.T) {
	ctx, binding := sourceTaskApprovalFixture(t)
	ledger := NewSourceTaskApprovalLedger()
	value, err := ledger.Request(ctx, binding)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := ledger.Request(ctx, binding)
	if err != nil || repeated.ID != value.ID {
		t.Fatal("相同任务创建了重复审批")
	}
	if err := ledger.Claim(ctx, value.ID, binding, "early", "lease", time.Now().Add(time.Minute)); err == nil {
		t.Fatal("未批准的任务开始执行")
	}
	if _, err := ledger.Decide(ctx, value.ID, value.Revision+1, binding, true); !errors.Is(err, coordination.ErrRequestConflict) {
		t.Fatal("旧审批界面覆盖了当前决定")
	}
	approved, err := ledger.Decide(ctx, value.ID, value.Revision, binding, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := ledger.Claim(ctx, value.ID, binding, "missing-record", "lease", time.Now().Add(time.Minute)); err == nil {
		t.Fatal("没有实际权限记录的审批开始执行")
	}
	if err := ledger.SetApprovalRecord(ctx, value.ID, approved.Revision, binding, "source-record", time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := ledger.SetApprovalRecord(ctx, value.ID, approved.Revision, binding, "replaced-record", time.Now().Add(time.Minute)); !errors.Is(err, coordination.ErrRequestConflict) {
		t.Fatal("已有批准记录被替换")
	}
	start := make(chan struct{})
	winner := make(chan string, 32)
	var workers sync.WaitGroup
	for index := range 32 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			attempt := fmt.Sprintf("attempt-%d", index)
			if err := ledger.Claim(ctx, value.ID, binding, attempt, "lease", time.Now().Add(time.Minute)); err == nil {
				winner <- attempt
			}
		}()
	}
	close(start)
	workers.Wait()
	if len(winner) != 1 {
		t.Fatalf("单次审批被多个代次消费: %d", len(winner))
	}
	attempt := <-winner
	if err := ledger.Claim(ctx, value.ID, binding, attempt, "lease", time.Now().Add(time.Minute)); err != nil {
		t.Fatal("相同执行的网络重试被误判为重放")
	}
	if err := ledger.ValidateClaim(ctx, value.ID, binding, attempt, "lease"); err != nil {
		t.Fatal(err)
	}
	current, _ := ledger.Get(value.ID)
	if _, err := ledger.RevokeCurrent(ctx, value.ID, current.Revision-1, binding); !errors.Is(err, coordination.ErrRequestConflict) {
		t.Fatal("旧审批版本撤销了当前执行")
	}
	for _, change := range []string{"input", "installation", "connection", "task", "generation", "core", "role", "mode"} {
		copy := binding
		switch change {
		case "input":
			copy.InputHash = hashBytes([]byte("other-input"))
		case "installation":
			copy.Target.InstalledGeneration++
		case "connection":
			copy.ExecutionTarget.ConnectionGeneration++
		case "task":
			copy.TaskRunID = "other-task"
		case "generation":
			copy.TaskGeneration++
		case "core":
			copy.Scope.CoreID = "other-core"
		case "role":
			copy.Scope.RoleRevision++
		case "mode":
			copy.Scope.ModeRevision++
		}
		if err := ledger.ValidateClaim(ctx, value.ID, copy, attempt, "lease"); err == nil {
			t.Fatalf("单次审批接受了改变的 %s", change)
		}
	}
	if _, err := ledger.RevokeCurrent(ctx, value.ID, current.Revision, binding); err != nil {
		t.Fatal(err)
	}
	if err := ledger.ValidateClaim(ctx, value.ID, binding, attempt, "lease"); err == nil {
		t.Fatal("撤销后继续使用单次审批")
	}
	ledger.Close()
	if _, found := ledger.Get(value.ID); found || len(ledger.List()) != 0 {
		t.Fatal("设备服务停止后保留了单次审批")
	}
	if _, err := ledger.Request(ctx, binding); !IsTaskErrorCode(err, ErrTaskDependencyUnavailable) {
		t.Fatal("设备服务停止后创建了新审批")
	}
}

func TestSourceTaskSingleApprovalExpiresBoundsAndRechecksAuthority(t *testing.T) {
	ctx, binding := sourceTaskApprovalFixture(t)
	ledger := NewSourceTaskApprovalLedger()
	now := time.Now().UTC()
	ledger.now = func() time.Time { return now }
	value, err := ledger.Request(ctx, binding)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(5 * time.Minute)
	if _, err := ledger.Decide(ctx, value.ID, value.Revision, binding, true); !errors.Is(err, coordination.ErrRequestConflict) {
		t.Fatal("过期审批被批准")
	}
	var revoked atomic.Bool
	guarded := coordination.WithAdditionalGuard(ctx, func(context.Context) error {
		if revoked.Load() {
			return coordination.ErrScopeExpired
		}
		return nil
	})
	value, err = ledger.Request(guarded, binding)
	if err != nil {
		t.Fatal(err)
	}
	revoked.Store(true)
	if _, err := ledger.Decide(guarded, value.ID, value.Revision, binding, true); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatal("角色或设备撤销后批准旧任务")
	}
	for index := 0; index < 63; index++ {
		copy := binding
		copy.TaskRunID = fmt.Sprintf("other-%d", index)
		if _, err := ledger.Request(ctx, copy); err != nil {
			t.Fatal(err)
		}
	}
	copy := binding
	copy.TaskRunID = "overflow"
	if _, err := ledger.Request(ctx, copy); !errors.Is(err, coordination.ErrPendingLimit) {
		t.Fatal("审批队列缺少总量上限")
	}
}
