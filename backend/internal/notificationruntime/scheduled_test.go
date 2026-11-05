package notificationruntime

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/u-ai/backend/pkg/sse"
)

func TestScheduledDesktopNotificationUsesDeviceChannel(t *testing.T) {
	client := sse.Global.SubscribeDevice("scheduled-desktop-test", "notification-space", "desktop-device")
	t.Cleanup(func() { sse.Global.UnsubscribeClient(client) })
	gateway := &PushGateway{}
	endpoint := DeviceEndpoint{SpaceID: "notification-space", DeviceID: "desktop-device", Platform: "desktop"}
	envelope := PushEnvelope{NotificationID: "stable-notification", Title: "喝水", Body: "休息一下", ConversationID: "conversation", TTL: time.Hour}
	result := gateway.Send(t.Context(), endpoint, envelope)
	if !result.Accepted || result.Provider != "desktop-sse" || result.ProviderMessageID != envelope.NotificationID {
		t.Fatalf("desktop delivery not accepted: %+v", result)
	}
	select {
	case event := <-client.Events:
		payload := event["data"].(map[string]interface{})
		if event["event"] != "scheduled_notification" || payload["requestId"] != envelope.NotificationID {
			t.Fatal(event)
		}
	default:
		t.Fatal("desktop did not receive notification")
	}
	endpoint.DeviceID = "offline-device"
	result = gateway.Send(t.Context(), endpoint, envelope)
	if result.Accepted || result.ErrorCode != "device_offline" || !retryableProviderResult(result) {
		t.Fatalf("offline desktop reported success: %+v", result)
	}
}

func TestScheduledNotificationAcknowledgementIsScopedAndMonotonic(t *testing.T) {
	db, err := openNotificationTestDB(t)
	if err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(db)
	if err := repo.InitSchema(); err != nil {
		t.Fatal(err)
	}
	runtime := &Runtime{db: db, repo: repo}
	envelope, _ := json.Marshal(PushEnvelope{NotificationID: "notification", Type: "reminder.triggered"})
	now := time.Now().UTC()
	row := &NotificationOutbox{ID: "outbox", NotificationID: "notification", SpaceID: "space", DeviceID: "device", Type: "reminder.triggered", Status: "running", EnvelopeJSON: string(envelope), CreatedAt: now, UpdatedAt: now, NextAttemptAt: now, ExpiresAt: now.Add(time.Hour)}
	if err := repo.CreateOutbox(t.Context(), row); err != nil {
		t.Fatal(err)
	}
	for _, identity := range [][2]string{{"other", "device"}, {"space", "other"}, {"", "device"}} {
		if runtime.AcknowledgeScheduled(t.Context(), identity[0], identity[1], "notification", "displayed") == nil {
			t.Fatal("another principal acknowledged notification", identity)
		}
	}
	if err := runtime.AcknowledgeScheduled(t.Context(), "space", "device", "notification", "displayed"); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateOutbox(t.Context(), row.ID, map[string]any{"status": "accepted"}); err != nil {
		t.Fatal(err)
	}
	actual, err := runtime.ScheduledState(t.Context(), row.ID)
	if err != nil || actual.State != "displayed" {
		t.Fatalf("late provider result overwrote device receipt: %+v %v", actual, err)
	}
	if err := runtime.AcknowledgeScheduled(t.Context(), "space", "device", "notification", "clicked"); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AcknowledgeScheduled(t.Context(), "space", "device", "notification", "displayed"); err != nil {
		t.Fatal(err)
	}
	actual, err = runtime.ScheduledState(t.Context(), row.ID)
	if err != nil || actual.State != "clicked" {
		t.Fatalf("late receipt regressed clicked state: %+v %v", actual, err)
	}
}

func TestTestDevicePushPersistsDisplayReceipt(t *testing.T) {
	db, err := openNotificationTestDB(t)
	if err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(db)
	if err := repo.InitSchema(); err != nil {
		t.Fatal(err)
	}
	endpoint := &DeviceEndpoint{
		SpaceID:                    "space",
		DeviceID:                   "desktop",
		Platform:                   "desktop",
		PreferredProvider:          "desktop-sse",
		PushEnabled:                true,
		SystemNotificationsEnabled: true,
		MessagePushEnabled:         true,
		ExecutionActivityEnabled:   true,
		CallPushEnabled:            true,
		ReminderPushEnabled:        true,
		PreviewMode:                "full",
		SoundEnabled:               true,
	}
	if err := repo.UpsertDevice(t.Context(), endpoint); err != nil {
		t.Fatal(err)
	}
	client := sse.Global.SubscribeDevice("test-device-receipt", endpoint.SpaceID, endpoint.DeviceID)
	t.Cleanup(func() { sse.Global.UnsubscribeClient(client) })
	runtime := &Runtime{db: db, repo: repo, gateway: &PushGateway{}}

	result, err := runtime.TestDevicePush(t.Context(), endpoint.SpaceID, endpoint.DeviceID)
	if err != nil || result["accepted"] != true {
		t.Fatalf("test push failed: %+v %v", result, err)
	}
	notificationID, _ := result["providerMessageId"].(string)
	if notificationID == "" {
		t.Fatalf("missing provider message id: %+v", result)
	}
	select {
	case event := <-client.Events:
		payload, _ := event["data"].(map[string]interface{})
		if event["event"] != "scheduled_notification" || payload["requestId"] != notificationID {
			t.Fatalf("unexpected desktop event: %+v", event)
		}
	default:
		t.Fatal("desktop did not receive the test notification")
	}
	if err := runtime.AcknowledgeScheduled(t.Context(), endpoint.SpaceID, endpoint.DeviceID, notificationID, "displayed"); err != nil {
		t.Fatal(err)
	}
	state, err := runtime.ScheduledState(t.Context(), notificationID)
	if err != nil || state.State != "displayed" {
		t.Fatalf("test notification receipt was not persisted: %+v %v", state, err)
	}
}

func TestAttentionNotificationIsIdempotentAndSuppressedInForeground(t *testing.T) {
	db, err := openNotificationTestDB(t)
	if err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(db)
	if err := repo.InitSchema(); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	endpoint := &DeviceEndpoint{
		SpaceID: "space", DeviceID: "desktop", Platform: "desktop",
		PushEnabled: true, SystemNotificationsEnabled: true, PreviewMode: "full", SoundEnabled: true,
		Foreground: false, PresenceUpdatedAt: now,
	}
	if err := repo.UpsertDevice(t.Context(), endpoint); err != nil {
		t.Fatal(err)
	}
	runtime := &Runtime{db: db, repo: repo, gateway: &PushGateway{}, deliveryQueue: make(chan deliveryJob, 8)}
	notification := AttentionNotification{SpaceID: "space", AttentionID: "agent-approval:one", Kind: "agent_approval", Title: "Amitia 需要你的处理", Body: "工具审批", ConversationID: "conversation", RequestID: "request"}
	first, err := runtime.QueueAttention(t.Context(), notification)
	if err != nil || len(first) != 1 || first[0].State != "queued" {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	second, err := runtime.QueueAttention(t.Context(), notification)
	if err != nil || len(second) != 1 || second[0].ID != first[0].ID {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	var count int64
	if err := db.Model(&NotificationOutbox{}).Where("type = ?", "attention.required").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	if err := repo.UpdatePresence(t.Context(), "space", "desktop", true, "conversation"); err != nil {
		t.Fatal(err)
	}
	suppressed, err := runtime.QueueAttention(t.Context(), AttentionNotification{SpaceID: "space", AttentionID: "task-approval:two", Kind: "task_approval", Body: "设备审批", ConversationID: "conversation"})
	if err != nil || len(suppressed) != 1 || suppressed[0].State != "suppressed" {
		t.Fatalf("suppressed=%+v err=%v", suppressed, err)
	}
	if err := repo.UpdatePresence(t.Context(), "space", "desktop", false, ""); err != nil {
		t.Fatal(err)
	}
	changed, err := runtime.QueueAttention(t.Context(), AttentionNotification{SpaceID: "space", AttentionID: "agent-approval:one", Kind: "agent_approval", State: "failed", Body: "工具审批状态已变化", ConversationID: "conversation"})
	if err != nil || len(changed) != 1 || changed[0].ID == first[0].ID || changed[0].State != "queued" {
		t.Fatalf("changed=%+v first=%+v err=%v", changed, first, err)
	}
}
