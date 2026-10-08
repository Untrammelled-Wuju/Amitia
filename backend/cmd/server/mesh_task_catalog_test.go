package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	kernelsqlite "github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func installThreeCoreCatalogTasks(t *testing.T, device *threeCoreFixture, version string, configure ...func(*task_runtime.TaskRuntimeConfig)) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "task.cjs"), []byte(`module.exports=async()=>({success:true,output:{}});`), 0600); err != nil {
		t.Fatal(err)
	}
	config := task_runtime.DefaultTaskRuntimeConfig()
	config.InstalledDefinitionValidator = func(_ context.Context, definition *task_runtime.TaskDefinition) error {
		if definition.InstalledGeneration != 2 {
			return coordination.ErrScopeExpired
		}
		return nil
	}
	config.EntryResolver = func(ctx context.Context, definition *task_runtime.TaskDefinition) (string, error) {
		return task_runtime.ResolveTaskEntry(ctx, dir, definition)
	}
	for _, apply := range configure {
		apply(&config)
	}
	service := task_runtime.NewTaskRuntimeService(kernelsqlite.NewTaskRepository(device.services.KernelContainer.DeviceRegistry.Database()), config)
	device.services.KernelContainer.TaskRuntimeService = service
	for _, id := range []string{"same-task", "second-task"} {
		definition := &task_runtime.TaskDefinition{TaskID: id, ExtensionID: "extension", ModuleID: "module", ContributionID: id, InstalledGeneration: 2, Entry: "task.cjs", Version: version}
		if err := task_runtime.PinTaskEntry(t.Context(), dir, definition); err != nil {
			t.Fatal(err)
		}
		if err := service.PutTaskDefinition(t.Context(), definition); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCoreConsoleTaskCatalogStillRequiresTargetCapabilityGrant(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	a, b := newThreeCoreFixture(t, "console-a", schemas), newThreeCoreFixture(t, "console-b", schemas)
	pairThreeCoreFixtures(t, a, b)
	installThreeCoreCatalogTasks(t, a, "1.0.0")
	if err := b.services.DeviceMesh.Coordination.InitializeCoreConsole(t.Context(), b.core, b.device.DeviceID.String()); err != nil {
		t.Fatal(err)
	}
	actor := &auth.ActorContext{PrincipalType: auth.PrincipalLocalUI, SpaceID: runtimeidentity.SpaceID(b.core), DeviceID: b.device.DeviceID, IsLocalTrusted: true, Permissions: []string{auth.PermSystemAdmin}}
	router := gin.New()
	registerMeshTaskSubmissionRouter(router.Group("/api/device-mesh/v1/business", func(c *gin.Context) {
		c.Set("actorContext", actor)
		c.Request = c.Request.WithContext(auth.WithActor(c.Request.Context(), actor))
	}), b.services, b.core)
	call := func(method, path string, body []byte, expected int) []byte {
		t.Helper()
		request := httptest.NewRequest(method, path, bytes.NewReader(body))
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != expected {
			t.Fatalf("console status=%d expected=%d body=%s", recorder.Code, expected, recorder.Body.String())
		}
		return recorder.Body.Bytes()
	}
	rolePath := "/api/device-mesh/v1/business/tasks/roles?targetDeviceId=" + a.device.DeviceID.String()
	call(http.MethodGet, rolePath, nil, 403)
	if _, err := b.services.DeviceMesh.Coordination.SetCapabilityGrant(t.Context(), b.core, b.device.DeviceID.String(), a.device.DeviceID.String(), "task.execute", 0, true); err != nil {
		t.Fatal(err)
	}
	var options struct {
		Data struct {
			Scope coordination.ExecutionScope `json:"executionScope"`
		} `json:"data"`
	}
	if err := json.Unmarshal(call(http.MethodGet, rolePath, nil, 200), &options); err != nil {
		t.Fatal(err)
	}
	options.Data.Scope.RoleID, options.Data.Scope.RoleRevision = "one", 3
	body, _ := json.Marshal(meshTaskCatalogRequest{TargetDeviceID: a.device.DeviceID.String(), RoleID: "one", RequestID: "console-catalog", ExpectedScope: &options.Data.Scope})
	var page struct {
		Data devicemesh.DeviceTaskCatalogPage `json:"data"`
	}
	if err := json.Unmarshal(call(http.MethodPost, "/api/device-mesh/v1/business/tasks/catalog", body, 200), &page); err != nil || len(page.Data.Entries) != 2 || page.Data.Scope.InitiatorDeviceID != b.device.DeviceID.String() || page.Data.Scope.ResourceOwnerID != a.device.DeviceID.String() {
		t.Fatalf("Core 控制台丢失设备归属: %+v %v", page, err)
	}
}

func readThreeCoreCatalogAuthority(t *testing.T, provider, caller, target *threeCoreFixture) coordination.ExecutionScope {
	t.Helper()
	var options struct {
		Data struct {
			Scope coordination.ExecutionScope `json:"executionScope"`
		} `json:"data"`
	}
	if err := json.Unmarshal(provider.request(t, caller, http.MethodGet, "/api/device-mesh/v1/business/tasks/roles?targetDeviceId="+target.device.DeviceID.String(), nil, http.StatusOK), &options); err != nil {
		t.Fatal(err)
	}
	options.Data.Scope.RoleID, options.Data.Scope.RoleRevision = "one", 3
	return options.Data.Scope
}

func TestDeviceTaskCatalogUsesSignedSourceMetadataAndRejectsStaleAuthority(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	a, b, c := newThreeCoreFixture(t, "catalog-a", schemas), newThreeCoreFixture(t, "catalog-b", schemas), newThreeCoreFixture(t, "catalog-c", schemas)
	pairThreeCoreFixtures(t, a, b)
	pairThreeCoreFixtures(t, c, b)
	installThreeCoreCatalogTasks(t, a, "1.0.0")
	installThreeCoreCatalogTasks(t, c, "2.0.0")
	authority := readThreeCoreCatalogAuthority(t, b, a, a)
	request := meshTaskCatalogRequest{TargetDeviceID: a.device.DeviceID.String(), RoleID: "one", RequestID: "catalog-a-read", ExpectedScope: &authority, Limit: 1}
	var first struct {
		Data devicemesh.DeviceTaskCatalogPage `json:"data"`
	}
	if err := json.Unmarshal(b.request(t, a, http.MethodPost, "/api/device-mesh/v1/business/tasks/catalog", request, http.StatusOK), &first); err != nil || len(first.Data.Entries) != 1 || first.Data.Entries[0].Reference.SourceTaskID != "same-task" || first.Data.Entries[0].Reference.DeviceID != a.device.DeviceID.String() || first.Data.Entries[0].Definition.Version != "1.0.0" || first.Data.NextCursor == "" {
		t.Fatalf("签名目录查询未返回目标设备真实安装: %+v %v", first, err)
	}
	request.Cursor = first.Data.NextCursor
	var second struct {
		Data devicemesh.DeviceTaskCatalogPage `json:"data"`
	}
	if err := json.Unmarshal(b.request(t, a, http.MethodPost, "/api/device-mesh/v1/business/tasks/catalog", request, http.StatusOK), &second); err != nil || len(second.Data.Entries) != 1 || second.Data.Entries[0].Reference.SourceTaskID != "second-task" || second.Data.Revision != first.Data.Revision || second.Data.NextCursor != "" {
		t.Fatal("签名目录后续分页不一致")
	}
	request.Cursor = ""
	for _, scenario := range []string{"role", "permission", "mode", "missing-scope", "foreign-target", "limit", "cursor"} {
		changed, expected := request, http.StatusConflict
		proof := authority
		changed.ExpectedScope = &proof
		switch scenario {
		case "role":
			proof.RoleRevision++
		case "permission":
			proof.PermissionRevision++
		case "mode":
			proof.ModeRevision++
		case "missing-scope":
			changed.ExpectedScope = nil
			expected = http.StatusBadRequest
		case "foreign-target":
			changed.TargetDeviceID = c.device.DeviceID.String()
			expected = http.StatusForbidden
		case "limit":
			changed.Limit = 9
			expected = http.StatusBadRequest
		case "cursor":
			changed.Cursor = "invalid"
		}
		b.request(t, a, http.MethodPost, "/api/device-mesh/v1/business/tasks/catalog", changed, expected)
	}
	if _, err := b.services.DeviceMesh.Coordination.SetCapabilityGrant(t.Context(), b.core, a.device.DeviceID.String(), c.device.DeviceID.String(), "task.execute", 0, true); err != nil {
		t.Fatal(err)
	}
	cAuthority := readThreeCoreCatalogAuthority(t, b, a, c)
	cRequest := meshTaskCatalogRequest{TargetDeviceID: c.device.DeviceID.String(), RoleID: "one", RequestID: "catalog-c-read", ExpectedScope: &cAuthority, Limit: 1}
	var foreign struct {
		Data devicemesh.DeviceTaskCatalogPage `json:"data"`
	}
	if err := json.Unmarshal(b.request(t, a, http.MethodPost, "/api/device-mesh/v1/business/tasks/catalog", cRequest, http.StatusOK), &foreign); err != nil || len(foreign.Data.Entries) != 1 || foreign.Data.Entries[0].Reference.SourceTaskID != "same-task" || foreign.Data.Entries[0].Definition.Version != "2.0.0" || foreign.Data.Entries[0].Reference.CatalogID == first.Data.Entries[0].Reference.CatalogID {
		t.Fatal("不同设备的同名任务版本相互覆盖")
	}
	var copied int
	if err := b.services.KernelContainer.DeviceRegistry.Database().QueryRowContext(t.Context(), "SELECT COUNT(*) FROM extension_task_definitions").Scan(&copied); err != nil || copied != 0 {
		t.Fatal("查询任务目录意外覆盖了 Core 的定义或安装")
	}
	coreRuntime := task_runtime.NewTaskRuntimeService(kernelsqlite.NewTaskRepository(b.services.KernelContainer.DeviceRegistry.Database()), task_runtime.DefaultTaskRuntimeConfig())
	for index, catalog := range []devicemesh.DeviceTaskCatalogEntry{first.Data.Entries[0], foreign.Data.Entries[0]} {
		proof := authority
		if index == 1 {
			proof = cAuthority
		}
		ctx, actual, finish, err := b.services.OwnedBusiness.OpenTaskExecution(t.Context(), business.Request{ExpectedScope: &proof, SpaceID: b.core, CoreID: b.core, DeviceID: a.device.DeviceID.String(), TargetDeviceID: catalog.Reference.DeviceID, RoleID: "one", RequestID: "catalog-import-" + catalog.Reference.DeviceID})
		if err != nil {
			t.Fatal(err)
		}
		entry, err := b.services.DeviceMesh.TargetTaskCatalogEntry(ctx, catalog.Reference)
		if err != nil {
			finish()
			t.Fatal(err)
		}
		definition, err := coreRuntime.ImportDeviceTaskDefinition(ctx, entry)
		if err != nil || definition == nil || definition.TaskID != catalog.Reference.CatalogID || definition.Version != catalog.Definition.Version || definition.InstalledGeneration != 0 {
			finish()
			t.Fatalf("签名任务元数据导入改变了来源版本: %+v %v", definition, err)
		}
		target, err := resolveMeshTaskSubmissionTarget(b.services, actual, definition)
		if err != nil || target.SourceTaskDefinitionID != catalog.Reference.SourceTaskID || target.DeviceID.String() != catalog.Reference.DeviceID || target.RuntimeSessionID == "" || target.ConnectionGeneration < 1 {
			finish()
			t.Fatalf("未安装来源插件的 Core 未固定来源任务连接: %+v %v", target, err)
		}
		changed := catalog.Reference
		changed.PortableFingerprint = string(make([]byte, 64))
		if _, err := b.services.DeviceMesh.TargetTaskCatalogEntry(ctx, changed); err == nil {
			finish()
			t.Fatal("目标设备接受了被修改的任务版本引用")
		}
		finish()
	}
	if err := b.services.KernelContainer.DeviceRegistry.Database().QueryRowContext(t.Context(), "SELECT COUNT(*) FROM extension_task_definitions").Scan(&copied); err != nil || copied != 2 {
		t.Fatalf("不同来源任务的元数据被覆盖: %d %v", copied, err)
	}
}
