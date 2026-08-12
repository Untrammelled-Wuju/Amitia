package browser

import (
	"testing"
	"time"
)

func TestTabStorePutAndGet(t *testing.T) {
	store := newTabStore()
	record := &tabRecord{
		info: BrowserTabInfo{
			TabID:     "bt_test1",
			SessionID: "bs_s1",
			State:     TabStateReady,
		},
		sessionID:         "bs_s1",
		browserContextID:  "ctx_1",
		targetID:          "target_1",
		runtimeGeneration: 1,
		createdAt:         time.Now(),
	}

	store.put(record)

	got, ok := store.get("bt_test1")
	if !ok {
		t.Fatal("expected to find tab bt_test1")
	}
	if got.info.TabID != "bt_test1" {
		t.Fatalf("expected tab ID bt_test1, got: %s", got.info.TabID)
	}
}

func TestTabStoreGetNotFound(t *testing.T) {
	store := newTabStore()

	_, ok := store.get("bt_nonexistent")
	if ok {
		t.Fatal("expected not to find nonexistent tab")
	}
}

func TestTabStoreGetBySession(t *testing.T) {
	store := newTabStore()

	store.put(&tabRecord{
		info:              BrowserTabInfo{TabID: "bt_a", SessionID: "bs_s1", State: TabStateReady},
		sessionID:         "bs_s1",
		runtimeGeneration: 1,
		createdAt:         time.Now().Add(-2 * time.Second),
	})
	store.put(&tabRecord{
		info:              BrowserTabInfo{TabID: "bt_b", SessionID: "bs_s1", State: TabStateReady},
		sessionID:         "bs_s1",
		runtimeGeneration: 1,
		createdAt:         time.Now(),
	})
	store.put(&tabRecord{
		info:              BrowserTabInfo{TabID: "bt_c", SessionID: "bs_s1", State: TabStateClosed},
		sessionID:         "bs_s1",
		runtimeGeneration: 1,
		createdAt:         time.Now(),
	})
	store.put(&tabRecord{
		info:              BrowserTabInfo{TabID: "bt_d", SessionID: "bs_s2", State: TabStateReady},
		sessionID:         "bs_s2",
		runtimeGeneration: 1,
		createdAt:         time.Now(),
	})
	store.put(&tabRecord{
		info:              BrowserTabInfo{TabID: "bt_e", SessionID: "bs_s1", State: TabStateReady},
		sessionID:         "bs_s1",
		runtimeGeneration: 2,
		createdAt:         time.Now(),
	})

	records := store.getBySession("bs_s1", 1)
	if len(records) != 2 {
		t.Fatalf("expected 2 tabs for session bs_s1 in generation 1, got: %d", len(records))
	}

	if records[0].info.TabID != "bt_a" {
		t.Fatalf("expected first tab to be bt_a, got: %s", records[0].info.TabID)
	}
	if records[1].info.TabID != "bt_b" {
		t.Fatalf("expected second tab to be bt_b, got: %s", records[1].info.TabID)
	}
}

func TestTabStoreCountBySession(t *testing.T) {
	store := newTabStore()

	store.put(&tabRecord{
		info:              BrowserTabInfo{TabID: "bt_a", SessionID: "bs_s1", State: TabStateReady},
		sessionID:         "bs_s1",
		runtimeGeneration: 1,
	})
	store.put(&tabRecord{
		info:              BrowserTabInfo{TabID: "bt_b", SessionID: "bs_s1", State: TabStateCreated},
		sessionID:         "bs_s1",
		runtimeGeneration: 1,
	})
	store.put(&tabRecord{
		info:              BrowserTabInfo{TabID: "bt_c", SessionID: "bs_s1", State: TabStateClosing},
		sessionID:         "bs_s1",
		runtimeGeneration: 1,
	})

	count := store.countBySession("bs_s1", 1)
	if count != 2 {
		t.Fatalf("expected 2 active tabs in session, got: %d", count)
	}
}

func TestTabStoreCountActive(t *testing.T) {
	store := newTabStore()

	store.put(&tabRecord{
		info:              BrowserTabInfo{TabID: "bt_a", SessionID: "bs_s1", State: TabStateReady},
		runtimeGeneration: 1,
	})
	store.put(&tabRecord{
		info:              BrowserTabInfo{TabID: "bt_b", SessionID: "bs_s2", State: TabStateReady},
		runtimeGeneration: 1,
	})
	store.put(&tabRecord{
		info:              BrowserTabInfo{TabID: "bt_c", SessionID: "bs_s1", State: TabStateClosing},
		runtimeGeneration: 1,
	})

	count := store.countActive(1)
	if count != 2 {
		t.Fatalf("expected 2 active tabs, got: %d", count)
	}
}

func TestTabStoreTransition(t *testing.T) {
	store := newTabStore()

	store.put(&tabRecord{
		info:              BrowserTabInfo{TabID: "bt_a", SessionID: "bs_s1", State: TabStateReady},
		runtimeGeneration: 1,
	})

	if !store.transition("bt_a", TabStateReady, TabStateClosing) {
		t.Fatal("transition should succeed")
	}

	record, _ := store.get("bt_a")
	if record.info.State != TabStateClosing {
		t.Fatalf("expected state closing, got: %s", record.info.State)
	}

	if store.transition("bt_a", TabStateReady, TabStateClosed) {
		t.Fatal("transition should fail due to wrong from state")
	}
}

func TestTabStoreUpdateTabInfo(t *testing.T) {
	store := newTabStore()

	store.put(&tabRecord{
		info:              BrowserTabInfo{TabID: "bt_a", SessionID: "bs_s1", State: TabStateReady},
		runtimeGeneration: 1,
	})

	store.updateTabInfo("bt_a", "https://example.com", "Example", true)

	record, _ := store.get("bt_a")
	if record.info.URL != "https://example.com" {
		t.Fatalf("expected URL https://example.com, got: %s", record.info.URL)
	}
	if record.info.Title != "Example" {
		t.Fatalf("expected title Example, got: %s", record.info.Title)
	}
	if !record.info.Active {
		t.Fatal("expected tab to be active")
	}
}

func TestTabStoreClearActive(t *testing.T) {
	store := newTabStore()

	store.put(&tabRecord{
		info:              BrowserTabInfo{TabID: "bt_a", SessionID: "bs_s1", State: TabStateReady, Active: true},
		sessionID:         "bs_s1",
		runtimeGeneration: 1,
	})
	store.put(&tabRecord{
		info:              BrowserTabInfo{TabID: "bt_b", SessionID: "bs_s1", State: TabStateReady, Active: true},
		sessionID:         "bs_s1",
		runtimeGeneration: 1,
	})

	store.clearActive("bs_s1")

	recordA, _ := store.get("bt_a")
	if recordA.info.Active {
		t.Fatal("expected bt_a to be inactive")
	}
	recordB, _ := store.get("bt_b")
	if recordB.info.Active {
		t.Fatal("expected bt_b to be inactive")
	}
}

func TestTabStoreRemove(t *testing.T) {
	store := newTabStore()

	store.put(&tabRecord{
		info:              BrowserTabInfo{TabID: "bt_a", SessionID: "bs_s1", State: TabStateReady},
		runtimeGeneration: 1,
	})

	store.remove("bt_a")

	_, ok := store.get("bt_a")
	if ok {
		t.Fatal("expected tab to be removed")
	}
}

func TestTabStoreClearGeneration(t *testing.T) {
	store := newTabStore()

	store.put(&tabRecord{
		info:              BrowserTabInfo{TabID: "bt_a", SessionID: "bs_s1", State: TabStateReady},
		runtimeGeneration: 1,
	})
	store.put(&tabRecord{
		info:              BrowserTabInfo{TabID: "bt_b", SessionID: "bs_s1", State: TabStateReady},
		runtimeGeneration: 2,
	})

	store.clearGeneration(1)

	_, ok := store.get("bt_a")
	if ok {
		t.Fatal("expected bt_a to be cleared")
	}

	_, ok = store.get("bt_b")
	if !ok {
		t.Fatal("expected bt_b to still exist")
	}
}

func TestTabStoreCloseAllForSession(t *testing.T) {
	store := newTabStore()

	store.put(&tabRecord{
		info:              BrowserTabInfo{TabID: "bt_a", SessionID: "bs_s1", State: TabStateReady},
		sessionID:         "bs_s1",
		runtimeGeneration: 1,
	})
	store.put(&tabRecord{
		info:              BrowserTabInfo{TabID: "bt_b", SessionID: "bs_s1", State: TabStateReady},
		sessionID:         "bs_s1",
		runtimeGeneration: 1,
	})
	store.put(&tabRecord{
		info:              BrowserTabInfo{TabID: "bt_c", SessionID: "bs_s2", State: TabStateReady},
		sessionID:         "bs_s2",
		runtimeGeneration: 1,
	})

	records := store.closeAllForSession("bs_s1", 1)
	if len(records) != 2 {
		t.Fatalf("expected 2 closed tabs, got: %d", len(records))
	}

	_, ok := store.get("bt_a")
	if ok {
		t.Fatal("expected bt_a to be removed")
	}

	_, ok = store.get("bt_c")
	if !ok {
		t.Fatal("expected bt_c to still exist")
	}
}

func TestTabStoreCount(t *testing.T) {
	store := newTabStore()

	if store.count() != 0 {
		t.Fatalf("expected empty store count 0, got: %d", store.count())
	}

	store.put(&tabRecord{
		info:              BrowserTabInfo{TabID: "bt_a", SessionID: "bs_s1", State: TabStateReady},
		runtimeGeneration: 1,
	})

	if store.count() != 1 {
		t.Fatalf("expected store count 1, got: %d", store.count())
	}
}
