package sqlite

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
)

func TestTaskListFiltersPersistentDeviceAuthorityBeforePagination(t *testing.T) {
	db := openWorkflowTestDB(t)
	repository := NewTaskRepository(db)
	snapshots := scope.NewSQLiteScopeStore(db)
	for index, item := range []struct{ id, space, initiator, target string }{
		{"initiated", "core", "device", "other"},
		{"targeted", "core", "other", "device"},
		{"foreign", "core", "other", "third"},
		{"other-space", "other", "device", "device"},
		{"legacy", "", "", ""},
	} {
		run := &task_runtime.TaskRun{TaskRunID: item.id, TaskDefinitionID: "task", ExtensionID: "extension", ModuleID: "module", InvocationID: item.id, DefinitionFingerprint: "pin", Status: task_runtime.RunStatusQueued, Generation: 1, Revision: 1, CreatedAt: time.Now().UTC().Add(time.Duration(index) * time.Second)}
		if item.space != "" {
			run.ScopeSnapshotID = item.id + "-snapshot"
			encoded, _ := json.Marshal(coordination.ExecutionScope{SpaceID: item.space, CoreID: item.space, InitiatorDeviceID: item.initiator, TargetDeviceID: item.target})
			if err := snapshots.SaveSnapshot(t.Context(), scope.ScopeSnapshot{SnapshotID: run.ScopeSnapshotID, SpaceID: item.space, InvocationID: item.id, ExtensionID: "extension", ModuleID: "module", OwnedExecutionScope: encoded}); err != nil {
				t.Fatal(err)
			}
		}
		if err := repository.PutTaskRun(t.Context(), run); err != nil {
			t.Fatal(err)
		}
	}
	filter := task_runtime.ListTasksFilter{ScopedSpaceID: "core", ScopedDeviceID: "device", Limit: 1}
	page, err := repository.ListTaskRuns(t.Context(), filter)
	if err != nil || len(page) != 1 || page[0].TaskRunID != "targeted" {
		t.Fatalf("其他设备任务占用了第一页: %+v %v", page, err)
	}
	if _, err := json.Marshal(page); err != nil {
		t.Fatalf("只保存调度元数据的任务不能显示在设备页面: %v", err)
	}
	filter.Offset = 1
	page, err = repository.ListTaskRuns(t.Context(), filter)
	if err != nil || len(page) != 1 || page[0].TaskRunID != "initiated" {
		t.Fatalf("归属分页不连续: %+v %v", page, err)
	}
	filter.ScopedDeviceID = ""
	if _, err := repository.ListTaskRuns(t.Context(), filter); err == nil {
		t.Fatal("缺少设备身份时查询降级为全局任务")
	}
}
