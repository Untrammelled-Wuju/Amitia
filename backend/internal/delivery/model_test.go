package delivery

import (
	"testing"
	"time"
)

func TestNewDeliveryIntentDefaults(t *testing.T) {
	intent := NewDeliveryIntent("int-1", "ws", "peer-1", "text/plain", []byte("hello"))
	if intent.ID == "" {
		t.Error("expected non-empty ID")
	}
	if intent.InteractionID != "int-1" {
		t.Errorf("expected int-1, got %s", intent.InteractionID)
	}
	if intent.Channel != "ws" {
		t.Errorf("expected ws, got %s", intent.Channel)
	}
	if intent.Status != DeliveryStatusPending {
		t.Errorf("expected pending, got %s", intent.Status)
	}
	if intent.MaxRetries != 5 {
		t.Errorf("expected MaxRetries 5, got %d", intent.MaxRetries)
	}
	if intent.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestNewOutputLeaseDefaults(t *testing.T) {
	lease := NewOutputLease("int-1", "char-1", "user-1", "ws")
	if lease.ID == "" {
		t.Error("expected non-empty ID")
	}
	if lease.InteractionID != "int-1" {
		t.Errorf("expected int-1, got %s", lease.InteractionID)
	}
	if lease.Status != "active" {
		t.Errorf("expected active, got %s", lease.Status)
	}
	if lease.ExpiresAt.Before(lease.AcquiredAt) {
		t.Error("expected ExpiresAt after AcquiredAt")
	}
}

func TestOutputLeaseIsExpired(t *testing.T) {
	lease := NewOutputLease("int-1", "char-1", "user-1", "ws")
	lease.ExpiresAt = time.Now().UTC().Add(-1 * time.Second)
	if !lease.IsExpired() {
		t.Error("expected expired lease")
	}
}

func TestOutputLeaseIsNotExpired(t *testing.T) {
	lease := NewOutputLease("int-1", "char-1", "user-1", "ws")
	if lease.IsExpired() {
		t.Error("expected non-expired lease")
	}
}

func TestOutputLeasePreempt(t *testing.T) {
	lease := NewOutputLease("int-1", "char-1", "user-1", "ws")
	lease.Preempt("new-int-1")
	if lease.Status != "preempted" {
		t.Errorf("expected preempted, got %s", lease.Status)
	}
	if lease.PreemptedBy != "new-int-1" {
		t.Errorf("expected new-int-1, got %s", lease.PreemptedBy)
	}
	if lease.ReleasedAt == nil {
		t.Error("expected ReleasedAt to be set")
	}
}

func TestOutputLeaseRelease(t *testing.T) {
	lease := NewOutputLease("int-1", "char-1", "user-1", "ws")
	lease.Release()
	if lease.Status != "released" {
		t.Errorf("expected released, got %s", lease.Status)
	}
	if lease.ReleasedAt == nil {
		t.Error("expected ReleasedAt to be set")
	}
}

func TestGenerateDeliveryID(t *testing.T) {
	id := GenerateDeliveryID("int-a", "ws", "peer-x", "msg-1")
	expected := "di-int-a-ws-peer-x-msg-1"
	if id != expected {
		t.Errorf("expected %s, got %s", expected, id)
	}
}

func TestNewDeliveryIntentUsesGenerateDeliveryID(t *testing.T) {
	intent := NewDeliveryIntent("int-x", "qq", "peer-y", "text", []byte("data"))
	expectedPrefix := "di-int-x-qq-peer-y-"
	if len(intent.ID) <= len(expectedPrefix) {
		t.Errorf("expected ID with prefix %s, got %s", expectedPrefix, intent.ID)
	}
	if intent.ContentType != "text" {
		t.Errorf("expected ContentType text, got %s", intent.ContentType)
	}
}

func TestDeliveryStatusValues(t *testing.T) {
	if DeliveryStatusPending != "pending" {
		t.Errorf("expected pending, got %s", DeliveryStatusPending)
	}
	if DeliveryStatusDelivered != "delivered" {
		t.Errorf("expected delivered, got %s", DeliveryStatusDelivered)
	}
	if DeliveryStatusFailed != "failed" {
		t.Errorf("expected failed, got %s", DeliveryStatusFailed)
	}
	if DeliveryStatusUnknown != "unknown" {
		t.Errorf("expected unknown, got %s", DeliveryStatusUnknown)
	}
}
