package notificationruntime

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newNotificationTestRepository(t *testing.T) *Repository {
	t.Helper()
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:notification-runtime-%d?mode=memory&cache=shared", time.Now().UnixNano())),
		&gorm.Config{},
	)
	if err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(db)
	if err := repository.InitSchema(); err != nil {
		t.Fatal(err)
	}
	return repository
}

func TestOutboxRecoveryClaimAndExpiry(t *testing.T) {
	repository := newNotificationTestRepository(t)
	now := time.Now().UTC()
	rows := []*NotificationOutbox{
		{
			ID: "queued", NotificationID: "notice-queued", SpaceID: "space", DeviceID: "device",
			Type: "run.start", EnvelopeJSON: "{}", Status: "queued",
			NextAttemptAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour),
		},
		{
			ID: "processing", NotificationID: "notice-processing", SpaceID: "space", DeviceID: "device",
			Type: "run.update", EnvelopeJSON: "{}", Status: "processing",
			NextAttemptAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour),
		},
		{
			ID: "expired", NotificationID: "notice-expired", SpaceID: "space", DeviceID: "device",
			Type: "run.start", EnvelopeJSON: "{}", Status: "queued",
			NextAttemptAt: now.Add(-time.Hour), ExpiresAt: now.Add(-time.Minute),
		},
	}
	for _, row := range rows {
		if err := repository.CreateOutbox(t.Context(), row); err != nil {
			t.Fatal(err)
		}
	}
	if err := repository.RecoverOutbox(t.Context()); err != nil {
		t.Fatal(err)
	}
	due, err := repository.ListDueOutbox(t.Context(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 2 || due[0].ID != "queued" || due[1].ID != "processing" {
		t.Fatalf("unexpected due outbox rows: %+v", due)
	}
	claimed, err := repository.ClaimOutbox(t.Context(), "queued")
	if err != nil {
		t.Fatal(err)
	}
	if claimed.Status != "processing" {
		t.Fatalf("claimed status=%s, want processing", claimed.Status)
	}
	if _, err := repository.ClaimOutbox(t.Context(), "queued"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("second claim error=%v, want record not found", err)
	}
	if err := repository.ExpireOutbox(t.Context()); err != nil {
		t.Fatal(err)
	}
	var expired NotificationOutbox
	if err := repository.db.First(&expired, "id = ?", "expired").Error; err != nil {
		t.Fatal(err)
	}
	if expired.Status != "failed" || expired.LastError != "notification TTL expired" {
		t.Fatalf("expired row was not finalized: %+v", expired)
	}
}

func TestRunTerminalAndDeliveryUpdates(t *testing.T) {
	repository := newNotificationTestRepository(t)
	for revision := int64(1); revision <= 2; revision++ {
		if err := repository.MarkRunTerminal(t.Context(), "space", "device", "run-a", revision); err != nil {
			t.Fatal(err)
		}
	}
	terminal, err := repository.IsRunTerminal(t.Context(), "space", "device", "run-a")
	if err != nil || !terminal {
		t.Fatalf("terminal state missing: terminal=%v err=%v", terminal, err)
	}

	delivery := &DeliveryLog{
		ID: "delivery-a", NotificationID: "notice-a", SpaceID: "space", DeviceID: "device",
		Provider: "fcm", Type: "message", Status: "queued", ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	if err := repository.CreateDelivery(t.Context(), delivery); err != nil {
		t.Fatal(err)
	}
	if err := repository.UpdateDelivery(t.Context(), delivery.ID, map[string]any{
		"status": "accepted", "provider_message_id": "provider-a",
	}); err != nil {
		t.Fatal(err)
	}
	var updated DeliveryLog
	if err := repository.db.First(&updated, "id = ?", delivery.ID).Error; err != nil {
		t.Fatal(err)
	}
	if updated.Status != "accepted" || updated.ProviderMessageID != "provider-a" {
		t.Fatalf("delivery update lost: %+v", updated)
	}
}

func TestPreferencesCancelOnlyMatchingPendingNotifications(t *testing.T) {
	repository := newNotificationTestRepository(t)
	device := &DeviceEndpoint{
		SpaceID: "space", DeviceID: "device", Platform: "android",
		PushEnabled: true, LiveActivityPushToStartToken: "push-to-start",
	}
	if err := repository.UpsertDevice(t.Context(), device); err != nil {
		t.Fatal(err)
	}
	if err := repository.UpdatePreferences(t.Context(), "space", "device", map[string]any{
		"message_push_enabled": false,
		"ignored_field":        "must not persist",
	}); err != nil {
		t.Fatal(err)
	}
	devices, err := repository.ListDevices(t.Context(), "space")
	if err != nil || len(devices) != 1 || devices[0].MessagePushEnabled {
		t.Fatalf("device preference update failed: devices=%+v err=%v", devices, err)
	}

	now := time.Now().UTC()
	envelopes := map[string]PushEnvelope{
		"message": {Type: "message.new"},
		"run":     {Type: "run.update"},
		"cleanup": {Type: "run.end"},
	}
	for id, envelope := range envelopes {
		raw, err := json.Marshal(envelope)
		if err != nil {
			t.Fatal(err)
		}
		if err := repository.CreateOutbox(t.Context(), &NotificationOutbox{
			ID: id, NotificationID: "notice-" + id, SpaceID: "space", DeviceID: "device",
			Type: envelope.Type, EnvelopeJSON: string(raw), Status: "queued",
			NextAttemptAt: now, ExpiresAt: now.Add(time.Hour),
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := repository.CancelPendingOutboxByPreferences(
		t.Context(), "space", "device", false, true, false, false, false,
	); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]string{
		"message": "failed", "run": "queued", "cleanup": "queued",
	} {
		var row NotificationOutbox
		if err := repository.db.First(&row, "id = ?", id).Error; err != nil {
			t.Fatal(err)
		}
		if row.Status != want {
			t.Fatalf("%s status=%s, want %s", id, row.Status, want)
		}
	}
	if err := repository.ClearLiveActivityPushToStartToken(t.Context(), "space", "device"); err != nil {
		t.Fatal(err)
	}
	updated, err := repository.GetDeviceForPlatform(t.Context(), "space", "device", "android")
	if err != nil || updated.LiveActivityPushToStartToken != "" {
		t.Fatalf("push-to-start token was not cleared: %+v err=%v", updated, err)
	}
}

func TestLiveActivityTokenListingAndEnd(t *testing.T) {
	repository := newNotificationTestRepository(t)
	token := &LiveActivityToken{
		SpaceID: "space", DeviceID: "device", RunID: "run-a", ActivityID: "activity-a",
		UpdateToken: "update-a", ConversationID: "conversation-a", Revision: 2,
	}
	if err := repository.UpsertLiveActivityToken(t.Context(), token); err != nil {
		t.Fatal(err)
	}
	active, err := repository.ListActiveLiveActivities(t.Context(), "space", "device")
	if err != nil || len(active) != 1 || active[0].RunID != "run-a" {
		t.Fatalf("active activity listing failed: %+v err=%v", active, err)
	}
	conversation, err := repository.ListActiveLiveActivitiesForConversation(
		t.Context(), "space", "device", "conversation-a",
	)
	if err != nil || len(conversation) != 1 || conversation[0].ActivityID != "activity-a" {
		t.Fatalf("conversation activity listing failed: %+v err=%v", conversation, err)
	}
	if err := repository.EndLiveActivity(t.Context(), "space", "device", "run-a", 3); err != nil {
		t.Fatal(err)
	}
	active, err = repository.ListActiveLiveActivities(t.Context(), "space", "device")
	if err != nil || len(active) != 0 {
		t.Fatalf("ended activity remained active: %+v err=%v", active, err)
	}
}
