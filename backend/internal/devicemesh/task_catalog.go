package devicemesh

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type DeviceTaskCatalogEntry struct {
	Reference  task_runtime.DeviceTaskDefinitionReference `json:"reference"`
	Definition task_runtime.TaskDefinition                `json:"definition"`
	Target     task_runtime.TargetTaskDefinitionPin       `json:"target"`
}

type DeviceTaskCatalogPage struct {
	Capabilities task_runtime.SourceTaskCapabilities `json:"capabilities"`
	Scope        coordination.ExecutionScope         `json:"executionScope"`
	Revision     string                              `json:"revision"`
	Entries      []DeviceTaskCatalogEntry            `json:"entries"`
	NextCursor   string                              `json:"nextCursor,omitempty"`
}

func (rt *Runtime) TargetTaskCatalog(ctx context.Context, authority coordination.ExecutionScope, request task_runtime.DeviceTaskCatalogRequest) (DeviceTaskCatalogPage, error) {
	page := DeviceTaskCatalogPage{}
	current, owned := coordination.FromContext(ctx)
	if !owned || current != authority {
		return page, coordination.ErrWrongOwner
	}
	owner := authority.TargetDeviceID
	if authority.Coordinated {
		owner = authority.CoreID
	}
	if authority.ResourceOwnerID != owner || authority.RoleOwnerID != owner || len(request.Cursor) > 1024 || request.Limit < 0 || request.Limit > 8 {
		return page, coordination.ErrWrongOwner
	}
	if err := rt.validateDataRoute(ctx, authority); err != nil {
		return page, err
	}
	if err := rt.Coordination.RequireCapability(ctx, authority.SpaceID, authority.InitiatorDeviceID, authority.TargetDeviceID, "task.execute"); err != nil {
		return page, err
	}
	payload, err := json.Marshal(map[string]any{"operation": "task-catalog", "scope": authority, "taskCatalog": request})
	if err != nil {
		return page, err
	}
	reply, err := rt.InvokeDeviceHandlerWithRuntimeType(ctx, runtimeidentity.SpaceID(authority.SpaceID), runtimeidentity.DeviceID(authority.TargetDeviceID), capability.RuntimeTypeInternal, "coordination.data", payload, 30*time.Second)
	if err != nil {
		return page, err
	}
	var source task_runtime.DeviceTaskCatalogPage
	if len(reply.Structured) > 1<<20 || json.Unmarshal(reply.Structured, &source) != nil || source.DeviceID != authority.TargetDeviceID || len(source.Entries) > 8 || len(source.NextCursor) > 1024 || len(source.Revision) != 64 {
		return page, task_runtime.NewTaskError(task_runtime.ErrTaskDefinitionInvalid, "目标设备任务目录响应无效")
	}
	if revision, err := hex.DecodeString(source.Revision); err != nil || len(revision) != 32 {
		return page, task_runtime.NewTaskError(task_runtime.ErrTaskDefinitionInvalid, "目标设备任务目录版本无效")
	}
	page.Scope, page.Revision, page.NextCursor = authority, source.Revision, source.NextCursor
	page.Capabilities = source.Capabilities
	page.Entries = make([]DeviceTaskCatalogEntry, 0, len(source.Entries))
	seen := make(map[string]bool, len(source.Entries))
	for _, entry := range source.Entries {
		reference, err := task_runtime.NewDeviceTaskDefinitionReference(authority.CoreID, authority.TargetDeviceID, entry)
		if err != nil || seen[reference.CatalogID] {
			return DeviceTaskCatalogPage{}, task_runtime.NewTaskError(task_runtime.ErrTaskDefinitionInvalid, "目标设备任务目录身份或指纹无效")
		}
		seen[reference.CatalogID] = true
		page.Entries = append(page.Entries, DeviceTaskCatalogEntry{Reference: reference, Definition: entry.Definition, Target: entry.Target})
	}
	if err := rt.validateDataRoute(ctx, authority); err != nil {
		return DeviceTaskCatalogPage{}, err
	}
	if err := rt.Coordination.RequireCapability(ctx, authority.SpaceID, authority.InitiatorDeviceID, authority.TargetDeviceID, "task.execute"); err != nil {
		return DeviceTaskCatalogPage{}, err
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return DeviceTaskCatalogPage{}, err
	}
	return page, nil
}

func (rt *Runtime) TargetTaskCatalogEntry(ctx context.Context, reference task_runtime.DeviceTaskDefinitionReference) (task_runtime.DeviceTaskCatalogEntry, error) {
	entry := task_runtime.DeviceTaskCatalogEntry{}
	authority, owned := coordination.FromContext(ctx)
	if !owned || authority.CoreID != reference.CoreID || authority.TargetDeviceID != reference.DeviceID || reference.SourceTaskID == "" || len(reference.SourceTaskID) > 256 {
		return entry, coordination.ErrWrongOwner
	}
	if err := rt.validateDataRoute(ctx, authority); err != nil {
		return entry, err
	}
	if err := rt.Coordination.RequireCapability(ctx, authority.SpaceID, authority.InitiatorDeviceID, authority.TargetDeviceID, "task.execute"); err != nil {
		return entry, err
	}
	if err := rt.callOwnedData(ctx, authority, "task-catalog-entry", coordination.DataQuery{ResourceID: reference.SourceTaskID}, &entry); err != nil {
		return entry, err
	}
	actual, err := task_runtime.NewDeviceTaskDefinitionReference(authority.CoreID, authority.TargetDeviceID, entry)
	if err != nil || actual != reference {
		return task_runtime.DeviceTaskCatalogEntry{}, task_runtime.NewTaskError(task_runtime.ErrTaskDefinitionInvalid, "目标设备任务目录版本已变化，请重新选择")
	}
	if err := rt.Coordination.RequireCapability(ctx, authority.SpaceID, authority.InitiatorDeviceID, authority.TargetDeviceID, "task.execute"); err != nil {
		return task_runtime.DeviceTaskCatalogEntry{}, err
	}
	return entry, coordination.ValidateCurrent(ctx)
}
