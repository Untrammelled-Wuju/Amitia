package system

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/runtimeprofile"
	"github.com/u-ai/backend/internal/timeoutpolicy"
	"github.com/u-ai/backend/pkg/app"
	"net/http/httptest"
	"testing"
)

func TestTaskMaintenanceRouterUsesCanonicalService(t *testing.T) {
	svc := newSetupServiceTest(t)
	svc.runtimeProfile = runtimeprofile.ProfileCloudCore
	if err := svc.AttachTaskRuntime(task_runtime.NewTaskRuntimeService(nil, task_runtime.DefaultTaskRuntimeConfig())); err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("actorContext", &auth.ActorContext{PrincipalType: auth.PrincipalLocalUI, SpaceID: runtimeidentity.SpaceID("core"), IsLocalTrusted: true, Permissions: auth.OwnerDevicePermissions()})
		c.Next()
	})
	RegisterSystemRouter(router.Group("/api"), &app.AppContext{DB: svc.db}, svc, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("GET", "/api/runtime/tasks/config", nil))
	var result struct {
		Code int
		Data TaskMaintenanceConfig
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Code != 200 || !result.Data.Applied || result.Data.RuntimeProfile != "cloud-core" {
		t.Fatalf("canonical runtime not wired: %s", response.Body.String())
	}
}

func TestTaskMaintenancePolicyMigratesAppliesAndRestoresAcrossProfiles(t *testing.T) {
	restore := timeoutpolicy.Configure(timeoutpolicy.Default())
	defer restore()
	for _, profile := range []runtimeprofile.Profile{runtimeprofile.ProfileLocal, runtimeprofile.ProfileCloudCore, runtimeprofile.ProfileDeviceAgent} {
		t.Run(string(profile), func(t *testing.T) {
			svc := newSetupServiceTest(t)
			svc.runtimeProfile = profile
			svc.setAppSetting("long_running_max_tasks", "7")
			svc.setAppSetting("long_running_timeout", "10")
			runtime := task_runtime.NewTaskRuntimeService(nil, task_runtime.DefaultTaskRuntimeConfig())
			if err := svc.AttachTaskRuntime(runtime); err != nil {
				t.Fatal(err)
			}
			config, err := svc.TaskMaintenanceConfig(t.Context())
			if err != nil || config.MaxTasks != 7 || config.Timeout.Seconds != 600 || !config.Applied || config.RuntimeProfile != string(profile) {
				t.Fatalf("migration: %+v %v", config, err)
			}
			for _, key := range []string{"long_running_max_tasks", "long_running_timeout"} {
				if value := svc.getAppSetting(key); value != "" {
					t.Fatal("legacy setting is still active")
				}
			}
			request := TaskMaintenanceUpdate{MaxTasks: 2, Timeout: timeoutpolicy.Settings{Seconds: 90}, ExpectedRevision: config.Revision, ExpectedTimeoutRevision: config.TimeoutRevision}
			saved, err := svc.UpdateTaskMaintenanceConfig(t.Context(), request)
			if err != nil || runtime.MaxConcurrent() != 2 || saved.Timeout.Seconds != 90 || saved.Revision != config.Revision+1 {
				t.Fatalf("apply: %+v %v", saved, err)
			}
			if _, err := svc.UpdateTaskMaintenanceConfig(t.Context(), request); err == nil {
				t.Fatal("stale configuration accepted")
			}
			reloaded := &service{db: svc.db, runtimeProfile: profile}
			reloaded.loadTimeoutSettings()
			other := task_runtime.NewTaskRuntimeService(nil, task_runtime.DefaultTaskRuntimeConfig())
			if err := reloaded.AttachTaskRuntime(other); err != nil {
				t.Fatal(err)
			}
			got, err := reloaded.TaskMaintenanceConfig(t.Context())
			if err != nil || got.MaxTasks != 2 || got.Timeout.Seconds != 90 {
				t.Fatalf("reload: %+v %v", got, err)
			}
		})
	}
}

func TestTaskMaintenanceFailureRollsBackPersistenceAndExecutor(t *testing.T) {
	restore := timeoutpolicy.Configure(timeoutpolicy.Default())
	defer restore()
	svc := newSetupServiceTest(t)
	runtime := task_runtime.NewTaskRuntimeService(nil, task_runtime.DefaultTaskRuntimeConfig())
	if err := svc.AttachTaskRuntime(runtime); err != nil {
		t.Fatal(err)
	}
	initial, err := svc.TaskMaintenanceConfig(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.db.Exec(`CREATE TRIGGER reject_task_timeout BEFORE INSERT ON app_settings WHEN NEW.key='operation_timeout_policy' BEGIN SELECT RAISE(ABORT,'injected failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	_, err = svc.UpdateTaskMaintenanceConfig(t.Context(), TaskMaintenanceUpdate{MaxTasks: 8, Timeout: timeoutpolicy.Settings{Seconds: 300}, ExpectedRevision: initial.Revision, ExpectedTimeoutRevision: initial.TimeoutRevision})
	if err == nil {
		t.Fatal("database failure reported success")
	}
	got, err := svc.TaskMaintenanceConfig(t.Context())
	if err != nil || got != initial || runtime.MaxConcurrent() != initial.MaxTasks {
		t.Fatalf("failed save changed runtime: %+v %v", got, err)
	}
	if _, err := svc.UpdateTaskMaintenanceConfig(t.Context(), TaskMaintenanceUpdate{MaxTasks: 0, Timeout: timeoutpolicy.Default()}); err == nil {
		t.Fatal("invalid policy accepted")
	}
}

func TestTaskMaintenancePreservesUnifiedTimeoutAndRejectsStaleTimeoutRevision(t *testing.T) {
	restore := timeoutpolicy.Configure(timeoutpolicy.Default())
	defer restore()
	svc := newSetupServiceTest(t)
	if err := svc.UpdateTimeoutSettings(timeoutpolicy.Settings{Disabled: true, Seconds: 120}); err != nil {
		t.Fatal(err)
	}
	svc.setAppSetting("long_running_timeout", "20")
	if err := svc.AttachTaskRuntime(task_runtime.NewTaskRuntimeService(nil, task_runtime.DefaultTaskRuntimeConfig())); err != nil {
		t.Fatal(err)
	}
	initial, err := svc.TaskMaintenanceConfig(t.Context())
	if err != nil || !initial.Timeout.Disabled || initial.Timeout.Seconds != 120 {
		t.Fatalf("unified timeout overwritten: %+v %v", initial, err)
	}
	if err := svc.UpdateTimeoutSettings(timeoutpolicy.Settings{Seconds: 60}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateTaskMaintenanceConfig(t.Context(), TaskMaintenanceUpdate{MaxTasks: 2, Timeout: initial.Timeout, ExpectedRevision: initial.Revision, ExpectedTimeoutRevision: initial.TimeoutRevision}); err == nil {
		t.Fatal("stale unified timeout overwritten")
	}
	if svc.taskRuntime.MaxConcurrent() != initial.MaxTasks {
		t.Fatal("stale write partially applied concurrency")
	}
}
