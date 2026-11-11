package notificationruntime

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/pkg/sse"
)

func TestAckableNotificationContractCoversReceipts(t *testing.T) {
	for _, notificationType := range []string{"reminder.triggered", "proactive.message", "attention.required", "system.test", " System.Test "} {
		if !AckableNotificationType(notificationType) {
			t.Fatalf("%q should accept display receipts", notificationType)
		}
		if outboxAcceptedStatus(notificationType) != "accepted" {
			t.Fatalf("%q should wait for a device receipt", notificationType)
		}
	}
	for _, notificationType := range []string{"message.completed", "run.start", "run.update", "call.incoming", "call.ended", "liveactivity.update", ""} {
		if AckableNotificationType(notificationType) {
			t.Fatalf("%q must not accept display receipts", notificationType)
		}
		if outboxAcceptedStatus(notificationType) != "delivered" {
			t.Fatalf("%q should terminate as delivered", notificationType)
		}
	}
}

func TestNonAckableDesktopDeliveryTerminatesWithoutReceipt(t *testing.T) {
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
		ReminderPushEnabled:        true,
		PreviewMode:                "full",
		SoundEnabled:               true,
	}
	if err := repo.UpsertDevice(t.Context(), endpoint); err != nil {
		t.Fatal(err)
	}
	client := sse.Global.SubscribeDevice("non-ackable-desktop", endpoint.SpaceID, endpoint.DeviceID)
	t.Cleanup(func() { sse.Global.UnsubscribeClient(client) })
	runtime := &Runtime{db: db, repo: repo, gateway: &PushGateway{}}
	envelope := PushEnvelope{NotificationID: "completed-message", Type: "message.completed", SpaceID: endpoint.SpaceID, Title: "回复完成", Body: "内容", TTL: time.Hour}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	row := &NotificationOutbox{ID: "completed-outbox", NotificationID: envelope.NotificationID, SpaceID: endpoint.SpaceID, DeviceID: endpoint.DeviceID, Type: envelope.Type, EnvelopeJSON: string(encoded), Status: "queued", CreatedAt: now, UpdatedAt: now, NextAttemptAt: now, ExpiresAt: now.Add(time.Hour)}
	if err := repo.CreateOutbox(t.Context(), row); err != nil {
		t.Fatal(err)
	}
	runtime.deliverOutbox(t.Context(), row.ID)
	select {
	case event := <-client.Events:
		data, _ := event["data"].(map[string]interface{})
		payload, _ := data["payload"].(map[string]interface{})
		if event["event"] != "scheduled_notification" || payload["ackable"] != false {
			t.Fatalf("non-ackable notification was not delivered as display-only: %+v", event)
		}
	default:
		t.Fatal("desktop did not receive the completed message notification")
	}
	state, err := runtime.ScheduledState(t.Context(), row.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.State != "delivered" {
		t.Fatalf("non-ackable outbox did not reach a terminal state: %+v", state)
	}
	if err := runtime.AcknowledgeScheduled(t.Context(), endpoint.SpaceID, endpoint.DeviceID, envelope.NotificationID, "displayed"); err == nil || !strings.Contains(err.Error(), "通知类型不支持展示回执") {
		t.Fatalf("receipt for a non-ackable type must keep the original rejection: %v", err)
	}
}

func TestExpireOutboxTerminatesAcceptedReceipt(t *testing.T) {
	db, err := openNotificationTestDB(t)
	if err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(db)
	if err := repo.InitSchema(); err != nil {
		t.Fatal(err)
	}
	runtime := &Runtime{db: db, repo: repo}
	envelope, err := json.Marshal(PushEnvelope{NotificationID: "reminder", Type: "reminder.triggered"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	row := &NotificationOutbox{ID: "reminder-outbox", NotificationID: "reminder", SpaceID: "space", DeviceID: "device", Type: "reminder.triggered", EnvelopeJSON: string(envelope), Status: "accepted", CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now, NextAttemptAt: now, ExpiresAt: now.Add(-time.Minute)}
	if err := repo.CreateOutbox(t.Context(), row); err != nil {
		t.Fatal(err)
	}
	if err := repo.ExpireOutbox(t.Context()); err != nil {
		t.Fatal(err)
	}
	state, err := runtime.ScheduledState(t.Context(), row.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.State != "expired" || !strings.Contains(state.Error, "notification receipt expired") {
		t.Fatalf("accepted outbox was not reaped: %+v", state)
	}
	if err := runtime.AcknowledgeScheduled(t.Context(), "space", "device", "reminder", "displayed"); err == nil || !strings.Contains(err.Error(), "通知回执已过期") {
		t.Fatalf("expired receipt must keep the original rejection: %v", err)
	}
}
