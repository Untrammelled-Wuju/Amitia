package browser

import (
	"testing"
	"time"
)

func TestSessionStorePutAndGet(t *testing.T) {
	store := newSessionStore()
	record := &sessionRecord{
		info: BrowserSessionInfo{
			SessionID: "bs_test1",
			State:     SessionStateReady,
		},
		contextID:         "ctx_1",
		runtimeGeneration: 1,
		createdAt:         time.Now(),
	}

	store.put(record)

	got, ok := store.get("bs_test1")
	if !ok {
		t.Fatal("expected to find session bs_test1")
	}
	if got.info.SessionID != "bs_test1" {
		t.Fatalf("expected session ID bs_test1, got: %s", got.info.SessionID)
	}
}

func TestSessionStoreGetNotFound(t *testing.T) {
	store := newSessionStore()

	_, ok := store.get("bs_nonexistent")
	if ok {
		t.Fatal("expected not to find nonexistent session")
	}
}

func TestSessionStoreListActive(t *testing.T) {
	store := newSessionStore()

	store.put(&sessionRecord{
		info:              BrowserSessionInfo{SessionID: "bs_a", State: SessionStateReady},
		contextID:         "ctx_a",
		runtimeGeneration: 1,
		createdAt:         time.Now().Add(-2 * time.Second),
	})
	store.put(&sessionRecord{
		info:              BrowserSessionInfo{SessionID: "bs_b", State: SessionStateReady},
		contextID:         "ctx_b",
		runtimeGeneration: 1,
		createdAt:         time.Now(),
	})
	store.put(&sessionRecord{
		info:              BrowserSessionInfo{SessionID: "bs_c", State: SessionStateClosed},
		contextID:         "ctx_c",
		runtimeGeneration: 1,
		createdAt:         time.Now(),
	})
	store.put(&sessionRecord{
		info:              BrowserSessionInfo{SessionID: "bs_d", State: SessionStateReady},
		contextID:         "ctx_d",
		runtimeGeneration: 2,
		createdAt:         time.Now(),
	})

	records := store.listActive(1)
	if len(records) != 2 {
		t.Fatalf("expected 2 active sessions in generation 1, got: %d", len(records))
	}

	if records[0].info.SessionID != "bs_a" {
		t.Fatalf("expected first session to be bs_a, got: %s", records[0].info.SessionID)
	}
	if records[1].info.SessionID != "bs_b" {
		t.Fatalf("expected second session to be bs_b, got: %s", records[1].info.SessionID)
	}
}

func TestSessionStoreCountActiveCreating(t *testing.T) {
	store := newSessionStore()

	store.put(&sessionRecord{
		info:              BrowserSessionInfo{SessionID: "bs_a", State: SessionStateReady},
		runtimeGeneration: 1,
	})
	store.put(&sessionRecord{
		info:              BrowserSessionInfo{SessionID: "bs_b", State: SessionStateCreated},
		runtimeGeneration: 1,
	})
	store.put(&sessionRecord{
		info:              BrowserSessionInfo{SessionID: "bs_c", State: SessionStateClosing},
		runtimeGeneration: 1,
	})

	count := store.countActiveCreating(1)
	if count != 2 {
		t.Fatalf("expected 2 active/creating sessions, got: %d", count)
	}
}

func TestSessionStoreTransition(t *testing.T) {
	store := newSessionStore()

	store.put(&sessionRecord{
		info:              BrowserSessionInfo{SessionID: "bs_a", State: SessionStateReady},
		runtimeGeneration: 1,
	})

	if !store.transition("bs_a", SessionStateReady, SessionStateClosing) {
		t.Fatal("transition should succeed")
	}

	record, _ := store.get("bs_a")
	if record.info.State != SessionStateClosing {
		t.Fatalf("expected state closing, got: %s", record.info.State)
	}

	if store.transition("bs_a", SessionStateReady, SessionStateClosed) {
		t.Fatal("transition should fail due to wrong from state")
	}
}

func TestSessionStoreRemove(t *testing.T) {
	store := newSessionStore()

	store.put(&sessionRecord{
		info:              BrowserSessionInfo{SessionID: "bs_a", State: SessionStateReady},
		runtimeGeneration: 1,
	})

	store.remove("bs_a")

	_, ok := store.get("bs_a")
	if ok {
		t.Fatal("expected session to be removed")
	}
}

func TestSessionStoreClearGeneration(t *testing.T) {
	store := newSessionStore()

	store.put(&sessionRecord{
		info:              BrowserSessionInfo{SessionID: "bs_a", State: SessionStateReady},
		runtimeGeneration: 1,
	})
	store.put(&sessionRecord{
		info:              BrowserSessionInfo{SessionID: "bs_b", State: SessionStateReady},
		runtimeGeneration: 2,
	})

	store.clearGeneration(1)

	_, ok := store.get("bs_a")
	if ok {
		t.Fatal("expected bs_a to be cleared")
	}

	_, ok = store.get("bs_b")
	if !ok {
		t.Fatal("expected bs_b to still exist")
	}
}

func TestSessionStoreCount(t *testing.T) {
	store := newSessionStore()

	if store.count() != 0 {
		t.Fatalf("expected empty store count 0, got: %d", store.count())
	}

	store.put(&sessionRecord{
		info:              BrowserSessionInfo{SessionID: "bs_a", State: SessionStateReady},
		runtimeGeneration: 1,
	})

	if store.count() != 1 {
		t.Fatalf("expected store count 1, got: %d", store.count())
	}
}
