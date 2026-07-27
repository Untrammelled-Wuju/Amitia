package trusted_service

import (
	"testing"
	"time"
)

func TestQuarantine_QuarantineAndCheck(t *testing.T) {
	m := NewQuarantineManager()

	if m.IsQuarantined("svc-1") {
		t.Fatal("expected not quarantined initially")
	}

	err := m.Quarantine("svc-1", "inst-1", QuarantineFrequentCrash, "crashed 5 times", nil)
	if err != nil {
		t.Fatalf("quarantine failed: %v", err)
	}

	if !m.IsQuarantined("svc-1") {
		t.Fatal("expected quarantined after Quarantine()")
	}

	rec, err := m.Get("svc-1")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if rec.Reason != QuarantineFrequentCrash {
		t.Fatalf("expected reason %s, got %s", QuarantineFrequentCrash, rec.Reason)
	}
	if rec.Detail != "crashed 5 times" {
		t.Fatalf("expected detail, got %s", rec.Detail)
	}
}

func TestQuarantine_DoubleQuarantineFails(t *testing.T) {
	m := NewQuarantineManager()
	_ = m.Quarantine("svc-1", "inst-1", QuarantineFrequentCrash, "first", nil)

	err := m.Quarantine("svc-1", "inst-1", QuarantineHostAPIViolation, "second", nil)
	if err == nil {
		t.Fatal("expected error on double quarantine")
	}
}

func TestQuarantine_Release(t *testing.T) {
	m := NewQuarantineManager()
	_ = m.Quarantine("svc-1", "inst-1", QuarantineFrequentCrash, "test", nil)

	err := m.Release("svc-1", "admin", "fixed")
	if err != nil {
		t.Fatalf("release failed: %v", err)
	}

	if m.IsQuarantined("svc-1") {
		t.Fatal("expected not quarantined after release")
	}
}

func TestQuarantine_ReleaseNotQuarantined(t *testing.T) {
	m := NewQuarantineManager()
	err := m.Release("svc-1", "admin", "test")
	if err == nil {
		t.Fatal("expected error releasing non-quarantined service")
	}
}

func TestQuarantine_ListActive(t *testing.T) {
	m := NewQuarantineManager()
	_ = m.Quarantine("svc-1", "inst-1", QuarantineFrequentCrash, "test1", nil)
	_ = m.Quarantine("svc-2", "inst-2", QuarantineHostAPIViolation, "test2", nil)

	active := m.ListActive()
	if len(active) != 2 {
		t.Fatalf("expected 2 active, got %d", len(active))
	}

	_ = m.Release("svc-1", "admin", "fixed")
	active = m.ListActive()
	if len(active) != 1 {
		t.Fatalf("expected 1 active after release, got %d", len(active))
	}
}

func TestQuarantine_History(t *testing.T) {
	m := NewQuarantineManager()
	_ = m.Quarantine("svc-1", "inst-1", QuarantineFrequentCrash, "test1", nil)
	_ = m.Release("svc-1", "admin", "fixed")
	_ = m.Quarantine("svc-2", "inst-2", QuarantineHostAPIViolation, "test2", nil)

	history := m.History()
	if len(history) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(history))
	}

	var releasedCount int
	for _, h := range history {
		if h.ReleasedAt != nil {
			releasedCount++
		}
	}
	if releasedCount != 1 {
		t.Fatalf("expected 1 released entry, got %d", releasedCount)
	}
}

func TestQuarantine_Evidence(t *testing.T) {
	m := NewQuarantineManager()
	evidence := map[string]any{
		"crash_count": 5,
		"last_exit":   "SIGSEGV",
		"timestamp":   time.Now().UTC(),
	}
	_ = m.Quarantine("svc-1", "inst-1", QuarantineFrequentCrash, "crash loop", evidence)

	rec, _ := m.Get("svc-1")
	if rec.Evidence == nil {
		t.Fatal("expected evidence map")
	}
	if rec.Evidence["crash_count"] != 5 {
		t.Fatalf("expected crash_count=5, got %v", rec.Evidence["crash_count"])
	}
}
