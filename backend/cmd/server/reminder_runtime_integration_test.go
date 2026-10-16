package main

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/config"
	agenttool "github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	kernelsqlite "github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/schedule"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/internal/reminder"
	"github.com/u-ai/backend/internal/requestidentity"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/runtimeprofile"
	"github.com/u-ai/backend/internal/spaceidentity"
)

type reminderRuntimeFixture struct {
	provider *threeCoreFixture
	client   *threeCoreFixture
	clock    *schedule.FakeClock
	scanner  *schedule.ScheduleScanner
	item     reminder.Reminder
}

func newReminderRuntimeFixture(t *testing.T, coordinated bool) *reminderRuntimeFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	if requestidentity.CanonicalSpaceID() == "" {
		if _, err := spaceidentity.InitializeDefault(config.AppCfg.Storage.DataDir); err != nil {
			t.Fatal(err)
		}
	}
	core := requestidentity.CanonicalSpaceID()
	schemas := newThreeCoreSchemas(t)
	provider := newThreeCoreFixture(t, core, schemas)
	client := newThreeCoreFixture(t, "reminder-source", schemas)
	pairThreeCoreFixtures(t, client, provider)
	policy, err := provider.services.DeviceMesh.Coordination.Get(t.Context(), core, client.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	if coordinated {
		if _, err := provider.services.DeviceMesh.Coordination.ChangeMode(t.Context(), core, client.device.DeviceID.String(), policy.ModeRevision, true, "one"); err != nil {
			t.Fatal(err)
		}
	}
	db := provider.services.KernelContainer.DeviceRegistry.Database()
	repo := kernelsqlite.NewScheduleRepository(db)
	clock := schedule.NewFakeClock(time.Now().UTC())
	configuration := schedule.DefaultScheduleConfig()
	scheduler, err := schedule.NewScheduleService(schedule.ScheduleDeps{Store: repo, Clock: clock, Config: configuration})
	if err != nil {
		t.Fatal(err)
	}
	provider.services.KernelContainer.ScheduleService = scheduler
	provider.services.KernelContainer.HostAPIGateway = host_api.NewDefaultGateway()
	provider.services.RuntimeProfile = runtimeprofile.ProfileCloudCore
	if err := wireReminderService(provider.services); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { agenttool.SetScheduleCreator(nil) })
	rt := provider.services.DeviceMesh
	authMW := security.AuthenticationMiddleware(security.AuthConfig{Mode: "network", SpaceID: core, DeviceCredentials: rt.CredentialSvc, DeviceRegistry: rt.DeviceReg, Coordination: rt.Coordination})
	reminder.RegisterRouter(provider.router.Group("/api", authMW), provider.services.Reminders, "")
	encoded := provider.request(t, client, http.MethodPost, "/api/reminders", map[string]any{"requestId": "runtime-reminder", "title": "喝水", "content": "到期喝水", "characterId": "one", "remindAt": clock.Now().Add(time.Minute).Format(time.RFC3339Nano), "timezone": "UTC", "recurrence": map[string]any{"type": "none"}, "systemNotification": false}, http.StatusOK)
	var response struct {
		Data reminder.Reminder `json:"data"`
	}
	if err := json.Unmarshal(encoded, &response); err != nil {
		t.Fatal(err)
	}
	if !response.Data.Saved || response.Data.OwnerID != core || response.Data.ExecutionMode != "cloud-core" || response.Data.ConversationID == "" {
		t.Fatalf("真实创建回执无效：%s", encoded)
	}
	calculator := schedule.NewScheduleCalculator(clock)
	planner := schedule.NewTriggerPlanner(repo, clock, configuration, calculator, schedule.NewMisfireService(repo, clock))
	scanner := schedule.NewScheduleScanner(repo, clock, configuration, calculator, planner)
	return &reminderRuntimeFixture{provider: provider, client: client, clock: clock, scanner: scanner, item: response.Data}
}

func (f *reminderRuntimeFixture) disconnectClient(t *testing.T) {
	t.Helper()
	f.client.local.Stop()
	f.provider.services.DeviceMesh.Hub.CloseDevice(runtimeidentity.SpaceID(f.provider.core), f.client.device.DeviceID)
	deadline := time.Now().Add(3 * time.Second)
	for {
		_, connected := f.provider.services.DeviceMesh.Hub.GetByDevice(runtimeidentity.SpaceID(f.provider.core), f.client.device.DeviceID)
		if !connected {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("客户端连接未真实断开")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (f *reminderRuntimeFixture) executeDue(t *testing.T) *schedule.ExecuteResult {
	t.Helper()
	f.clock.Advance(2 * time.Minute)
	if _, err := f.scanner.ScanOnce(t.Context()); err != nil {
		t.Fatal(err)
	}
	scheduler := f.provider.services.KernelContainer.ScheduleService
	triggers, err := scheduler.GetTriggers(t.Context(), f.item.ID, 10)
	if err != nil || len(triggers) != 1 || triggers[0].Manual {
		t.Fatalf("到期任务没有由真实 Kernel 扫描创建：%+v %v", triggers, err)
	}
	result, err := scheduler.GetExecutor().Execute(t.Context(), triggers[0])
	if err != nil {
		t.Fatalf("调度执行失败：%+v %v", result, err)
	}
	return result
}

func (f *reminderRuntimeFixture) ownedMessages(t *testing.T, owner string) []coordination.Resource {
	t.Helper()
	rows, err := coordination.NewOwnershipStore(f.provider.services.KernelContainer.DeviceRegistry.Database(), owner).List(context.Background(), "message", "one", false)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestReminderRuntimeCloudCoreExecutesAfterActualClientDisconnect(t *testing.T) {
	f := newReminderRuntimeFixture(t, true)
	f.disconnectClient(t)
	result := f.executeDue(t)
	if result.Status != schedule.RunStatusCompleted || f.provider.model.calls.Load() != 1 {
		t.Fatalf("云核心依赖离线客户端或虚报完成：%+v", result)
	}
	rows := f.ownedMessages(t, f.provider.core)
	if len(rows) != 1 || len(f.ownedMessages(t, f.client.device.DeviceID.String())) != 0 {
		t.Fatalf("消息没有只写入云核心真实 OwnershipStore：%+v", rows)
	}
	var message map[string]any
	if json.Unmarshal(rows[0].Body, &message) != nil || message["role"] != "assistant" || message["conversationId"] != f.item.ConversationID {
		t.Fatalf("角色对话消息未实际保存：%s", rows[0].Body)
	}
	caller := reminder.Caller{SpaceID: f.provider.core, DeviceID: f.client.device.DeviceID.String()}
	history, total, err := f.provider.services.Reminders.History(t.Context(), caller, reminder.UserReminder, f.item.ID, 1, 20, "")
	if err != nil || total != 1 || len(history) != 1 || history[0].State != "completed" || history[0].Result == nil {
		t.Fatalf("真实执行历史缺失保存确认：%+v %d %v", history, total, err)
	}
	var delivery reminder.DeliveryResult
	if json.Unmarshal(history[0].Result, &delivery) != nil || !delivery.MessageSaved || delivery.MessageID != rows[0].ID {
		t.Fatalf("执行历史伪造保存回执：%s", history[0].Result)
	}
}

func TestReminderRuntimeDeviceMeshOfflineSourceIsBlockedWithoutCoreCopy(t *testing.T) {
	f := newReminderRuntimeFixture(t, false)
	f.disconnectClient(t)
	result := f.executeDue(t)
	if (result.Status != schedule.RunStatusBlocked && result.Status != schedule.RunStatusFailed) || result.ErrorCode != schedule.ErrCodeScopeDenied || f.provider.model.calls.Load() != 0 {
		t.Fatalf("离线设备私有任务被虚报成功：%+v", result)
	}
	if len(f.ownedMessages(t, f.provider.core)) != 0 || len(f.ownedMessages(t, f.client.device.DeviceID.String())) != 0 {
		t.Fatal("Core 保存了设备私有消息副本")
	}
	clientRows, err := coordination.NewOwnershipStore(f.client.services.KernelContainer.DeviceRegistry.Database(), f.client.device.DeviceID.String()).List(t.Context(), "message", "one", false)
	if err != nil || len(clientRows) != 0 {
		t.Fatalf("离线源设备发生了未经确认的消息写入：%+v %v", clientRows, err)
	}
}

func TestReminderRuntimeDoesNotTransferAfterOwnerOrProviderChanges(t *testing.T) {
	for _, change := range []string{"owner", "provider"} {
		t.Run(change, func(t *testing.T) {
			f := newReminderRuntimeFixture(t, true)
			service := f.provider.services.DeviceMesh.Coordination
			if change == "owner" {
				policy, err := service.Get(t.Context(), f.provider.core, f.client.device.DeviceID.String())
				if err != nil {
					t.Fatal(err)
				}
				if _, err := service.ChangeMode(t.Context(), f.provider.core, f.client.device.DeviceID.String(), policy.ModeRevision, false, "one"); err != nil {
					t.Fatal(err)
				}
			} else if _, _, err := service.BindProvider(t.Context(), "successor-core"); err != nil {
				t.Fatal(err)
			}
			result := f.executeDue(t)
			if (result.Status != schedule.RunStatusBlocked && result.Status != schedule.RunStatusFailed) || result.ErrorCode != schedule.ErrCodeScopeDenied || f.provider.model.calls.Load() != 0 {
				t.Fatalf("旧任务跟随 Owner/provider 转移执行：%+v", result)
			}
			for _, owner := range []string{f.provider.core, f.client.device.DeviceID.String(), "successor-core"} {
				if len(f.ownedMessages(t, owner)) != 0 {
					t.Fatalf("消息迁移至 %s", owner)
				}
			}
			item, err := f.provider.services.Reminders.Find(t.Context(), reminder.Caller{SpaceID: f.provider.core, DeviceID: f.client.device.DeviceID.String()}, f.item.ID)
			if err != nil || item.OwnerID != f.item.OwnerID {
				t.Fatalf("持久化任务 Owner 被自动迁移：%+v %v", item, err)
			}
		})
	}
}
