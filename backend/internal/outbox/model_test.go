package outbox

import (
	"testing"
)

func TestOutboxStatusConstants(t *testing.T) {
	if OutboxStatusPending != "pending" {
		t.Errorf("expected pending, got %s", OutboxStatusPending)
	}
	if OutboxStatusLeased != "leased" {
		t.Errorf("expected leased, got %s", OutboxStatusLeased)
	}
	if OutboxStatusPublished != "published" {
		t.Errorf("expected published, got %s", OutboxStatusPublished)
	}
	if OutboxStatusFailed != "failed" {
		t.Errorf("expected failed, got %s", OutboxStatusFailed)
	}
	if OutboxStatusRetry != "retry" {
		t.Errorf("expected retry, got %s", OutboxStatusRetry)
	}
	if OutboxStatusDead != "dead" {
		t.Errorf("expected dead, got %s", OutboxStatusDead)
	}
}

func TestDefaultConstants(t *testing.T) {
	if DefaultMaxRetries != 10 {
		t.Errorf("expected DefaultMaxRetries 10, got %d", DefaultMaxRetries)
	}
	if DefaultLeaseTTL == 0 {
		t.Error("expected non-zero DefaultLeaseTTL")
	}
	if DefaultRenewWindow == 0 {
		t.Error("expected non-zero DefaultRenewWindow")
	}
	if DefaultBatchSize != 20 {
		t.Errorf("expected DefaultBatchSize 20, got %d", DefaultBatchSize)
	}
}

func TestValidTransitionsCoversAllStatuses(t *testing.T) {
	vt := ValidTransitions()
	allStatuses := []OutboxStatus{
		OutboxStatusPending, OutboxStatusLeased, OutboxStatusPublished,
		OutboxStatusFailed, OutboxStatusRetry, OutboxStatusDead,
	}
	for _, s := range allStatuses {
		if _, ok := vt[s]; !ok {
			t.Errorf("missing transitions for status %s", s)
		}
	}
}
