//go:build rsi_resource

package resource

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

func governanceLog(t *testing.T, format string, args ...any) {
	t.Helper()
	t.Logf("m07-s01 "+format, args...)
}

func adoptSpec(id string, kind ManagedKind, owner ResourceOwner, closer func(context.Context) error) LeaseSpec {
	return LeaseSpec{ID: id, Kind: kind, Owner: owner, Close: closer}
}

func noopClose(context.Context) error { return nil }

func systemOwner() ResourceOwner { return NewSystemOwner() }

func TestGovernanceAdoptsEveryInventoryKind(t *testing.T) {
	ctx := context.Background()
	governor := NewGovernor()
	kinds := []ManagedKind{ManagedEventSubscription, ManagedPriorityQueue, ManagedBackpressure, ManagedOutbox, ManagedResourceOwnership, ManagedTaskEntry}
	for _, kind := range kinds {
		if _, err := governor.Adopt(ctx, adoptSpec(string(kind), kind, systemOwner(), noopClose)); err != nil {
			t.Fatalf("adopt %s: %v", kind, err)
		}
	}
	if governor.Len() != len(kinds) {
		t.Fatalf("len=%d want %d", governor.Len(), len(kinds))
	}
	for _, kind := range kinds {
		lease, ok := governor.Lookup(string(kind))
		if !ok || lease.Kind != kind || lease.Owner.OwnerID != "system" {
			t.Fatalf("lookup %s = %+v ok=%v", kind, lease, ok)
		}
	}
	governanceLog(t, "adopt kinds=6 leases=6")
}

func TestGovernanceRejectsUnknownKindAndBadSpecs(t *testing.T) {
	ctx := context.Background()
	governor := NewGovernor()
	if _, err := governor.Adopt(ctx, adoptSpec("x", ManagedKind("queue_v2"), systemOwner(), noopClose)); !errors.Is(err, ErrLeaseInvalid) {
		t.Fatalf("unknown kind err=%v", err)
	}
	if _, err := governor.Adopt(ctx, LeaseSpec{Kind: ManagedPriorityQueue, Owner: systemOwner(), Close: noopClose}); !errors.Is(err, ErrLeaseInvalid) {
		t.Fatalf("empty id err=%v", err)
	}
	if _, err := governor.Adopt(ctx, adoptSpec("y", ManagedPriorityQueue, ResourceOwner{}, noopClose)); !errors.Is(err, ErrLeaseInvalid) {
		t.Fatalf("bad owner err=%v", err)
	}
	if _, err := governor.Adopt(ctx, adoptSpec("z", ManagedPriorityQueue, systemOwner(), nil)); !errors.Is(err, ErrLeaseInvalid) {
		t.Fatalf("nil closer err=%v", err)
	}
	if governor.Len() != 0 {
		t.Fatalf("invalid adopt left %d leases", governor.Len())
	}
	governanceLog(t, "reject invalid=4 leases=0")
}

func TestGovernanceDuplicateRejected(t *testing.T) {
	ctx := context.Background()
	governor := NewGovernor()
	spec := adoptSpec("dup", ManagedOutbox, systemOwner(), noopClose)
	if _, err := governor.Adopt(ctx, spec); err != nil {
		t.Fatalf("first adopt: %v", err)
	}
	if _, err := governor.Adopt(ctx, spec); !errors.Is(err, ErrLeaseDuplicate) {
		t.Fatalf("duplicate err=%v", err)
	}
	if governor.Len() != 1 {
		t.Fatalf("len=%d want 1", governor.Len())
	}
	governanceLog(t, "duplicate rejected=true leases=1")
}

func TestGovernanceCloseAllReverseOrder(t *testing.T) {
	ctx := context.Background()
	governor := NewGovernor()
	var mu sync.Mutex
	closed := make([]string, 0, 3)
	for _, id := range []string{"first", "second", "third"} {
		name := id
		if _, err := governor.Adopt(ctx, LeaseSpec{ID: name, Kind: ManagedEventSubscription, Owner: systemOwner(), Close: func(context.Context) error {
			mu.Lock()
			closed = append(closed, name)
			mu.Unlock()
			return nil
		}}); err != nil {
			t.Fatalf("adopt %s: %v", name, err)
		}
	}
	if err := governor.CloseAll(ctx); err != nil {
		t.Fatalf("CloseAll: %v", err)
	}
	if strings.Join(closed, ",") != "third,second,first" {
		t.Fatalf("close order=%v", closed)
	}
	if !governor.Closed() || governor.Len() != 0 {
		t.Fatalf("closed=%v len=%d", governor.Closed(), governor.Len())
	}
	governanceLog(t, "close_all reverse=true order=third,second,first")
}

func TestGovernanceCloseAllAggregatesErrors(t *testing.T) {
	ctx := context.Background()
	governor := NewGovernor()
	failureA := errors.New("close a failed")
	failureB := errors.New("close b failed")
	for _, spec := range []LeaseSpec{
		{ID: "a", Kind: ManagedPriorityQueue, Owner: systemOwner(), Close: func(context.Context) error { return failureA }},
		{ID: "b", Kind: ManagedBackpressure, Owner: systemOwner(), Close: noopClose},
		{ID: "c", Kind: ManagedOutbox, Owner: systemOwner(), Close: func(context.Context) error { return failureB }},
	} {
		if _, err := governor.Adopt(ctx, spec); err != nil {
			t.Fatalf("adopt %s: %v", spec.ID, err)
		}
	}
	err := governor.CloseAll(ctx)
	if !errors.Is(err, failureA) || !errors.Is(err, failureB) {
		t.Fatalf("aggregate err=%v", err)
	}
	governanceLog(t, "close_all errors=2 attempted_all=true")
}

func TestGovernanceReleaseSemantics(t *testing.T) {
	ctx := context.Background()
	governor := NewGovernor()
	closed := false
	if _, err := governor.Adopt(ctx, LeaseSpec{ID: "lease", Kind: ManagedResourceOwnership, Owner: systemOwner(), Close: func(context.Context) error {
		closed = true
		return nil
	}}); err != nil {
		t.Fatalf("adopt: %v", err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := governor.Release(cancelled, "lease"); err != nil {
		t.Fatalf("release with cancelled ctx must still close: %v", err)
	}
	if !closed {
		t.Fatal("release did not close the resource")
	}
	if err := governor.Release(ctx, "lease"); !errors.Is(err, ErrLeaseUnknown) {
		t.Fatalf("second release err=%v", err)
	}
	governanceLog(t, "release closes_on_cancel=true second=ErrLeaseUnknown")
}

func TestGovernanceClosedRejectsAdopt(t *testing.T) {
	ctx := context.Background()
	governor := NewGovernor()
	if err := governor.CloseAll(ctx); err != nil {
		t.Fatalf("CloseAll empty: %v", err)
	}
	if _, err := governor.Adopt(ctx, adoptSpec("late", ManagedOutbox, systemOwner(), noopClose)); !errors.Is(err, ErrGovernorClosed) {
		t.Fatalf("adopt after close err=%v", err)
	}
	if err := governor.CloseAll(ctx); !errors.Is(err, ErrGovernorClosed) {
		t.Fatalf("double CloseAll err=%v", err)
	}
	governanceLog(t, "closed rejects_adopt=true rejects_double_close=true")
}

func TestGovernanceStatsAndCancelAdopt(t *testing.T) {
	ctx := context.Background()
	governor := NewGovernor()
	if _, err := governor.Adopt(ctx, LeaseSpec{ID: "stats", Kind: ManagedTaskEntry, Owner: systemOwner(), Close: noopClose, Stats: func() map[string]int64 {
		return map[string]int64{"queued": 3}
	}}); err != nil {
		t.Fatalf("adopt: %v", err)
	}
	stats, ok := governor.Stats("stats")
	if !ok || stats["queued"] != 3 {
		t.Fatalf("stats=%v ok=%v", stats, ok)
	}
	if _, ok := governor.Stats("missing"); ok {
		t.Fatal("missing lease reported stats")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := governor.Adopt(cancelled, adoptSpec("late", ManagedOutbox, systemOwner(), noopClose)); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled adopt err=%v", err)
	}
	governanceLog(t, "stats exposed=true cancelled_adopt=context.Canceled")
}

func TestGovernanceInventoryCoversSingleOwners(t *testing.T) {
	inventory, err := DefaultGovernanceInventory()
	if err != nil {
		t.Fatalf("DefaultGovernanceInventory: %v", err)
	}
	if len(inventory.Entries) != 7 {
		t.Fatalf("entries=%d want 7", len(inventory.Entries))
	}
	if kinds := inventory.Kinds(); len(kinds) != 6 {
		t.Fatalf("kinds=%v", kinds)
	}
	if inventory.Count(ManagedPriorityQueue) != 1 {
		t.Fatalf("priority queue implementations=%d want 1", inventory.Count(ManagedPriorityQueue))
	}
	if owners := inventory.Owners(ManagedEventSubscription); len(owners) != 1 || owners[0] != "event" {
		t.Fatalf("event owners=%v", owners)
	}
	if owners := inventory.Owners(ManagedOutbox); len(owners) != 1 || owners[0] != "scheduler" {
		t.Fatalf("outbox owners=%v", owners)
	}
	for _, entry := range inventory.Entries {
		if entry.Kind == ManagedTaskEntry && entry.Symmetric {
			t.Fatal("task entry must not claim a closer")
		}
		if entry.Kind != ManagedTaskEntry && !entry.Symmetric {
			t.Fatalf("%s must be symmetric", entry.Kind)
		}
	}
	governanceLog(t, "inventory entries=7 kinds=6 single_owner=true")
}
