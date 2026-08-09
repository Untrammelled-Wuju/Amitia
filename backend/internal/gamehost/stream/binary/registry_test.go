package binary

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func sampleOwner() BinaryOwner {
	return BinaryOwner{
		PluginID:  "plugin-x",
		RuntimeID: "runtime-1",
		ServiceID: "service-a",
		ChannelID: "frames",
	}
}

func TestObjectRegistry_InsertWriting(t *testing.T) {
	reg := NewObjectRegistry(Options{})
	owner := sampleOwner()

	record := BinaryObjectRecord{
		ID:    NewBinaryObjectID(),
		Kind:  BinaryStorageFile,
		Owner: owner,
	}

	if err := reg.InsertWriting(context.Background(), record); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	if reg.CountActive() != 1 {
		t.Fatalf("expected 1 active, got %d", reg.CountActive())
	}
}

func TestObjectRegistry_SealObject(t *testing.T) {
	reg := NewObjectRegistry(Options{})
	owner := sampleOwner()
	id := NewBinaryObjectID()

	record := BinaryObjectRecord{ID: id, Kind: BinaryStorageFile, Owner: owner}
	if err := reg.InsertWriting(context.Background(), record); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	if err := reg.SealObject(context.Background(), id, 1024, nil); err != nil {
		t.Fatalf("seal failed: %v", err)
	}

	stored, err := reg.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if stored.State != ObjectStateReady {
		t.Fatalf("expected ready, got %s", stored.State)
	}
	if stored.Size != 1024 {
		t.Fatalf("expected 1024, got %d", stored.Size)
	}
}

func TestObjectRegistry_Seal_SizeMismatch(t *testing.T) {
	reg := NewObjectRegistry(Options{})
	owner := sampleOwner()
	id := NewBinaryObjectID()

	record := BinaryObjectRecord{ID: id, Kind: BinaryStorageFile, Owner: owner}
	reg.InsertWriting(context.Background(), record)

	if err := reg.SealObject(context.Background(), id, -1, nil); err == nil {
		t.Fatal("negative size should be rejected")
	}
}

func TestObjectRegistry_WritingObjectCannotResolve(t *testing.T) {
	reg := NewObjectRegistry(Options{})
	owner := sampleOwner()
	id := NewBinaryObjectID()

	record := BinaryObjectRecord{ID: id, Kind: BinaryStorageFile, Owner: owner, State: ObjectStateWriting}
	reg.InsertWriting(context.Background(), record)

	stored, err := reg.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if stored.State != ObjectStateWriting {
		t.Fatalf("expected writing, got %s", stored.State)
	}
}

func TestObjectRegistry_Release_NotFoundSafe(t *testing.T) {
	reg := NewObjectRegistry(Options{})
	id := NewBinaryObjectID()

	if err := reg.Release(context.Background(), id); err != nil {
		t.Fatalf("release non-existent should be safe: %v", err)
	}
}

func TestObjectRegistry_Release_Idempotent(t *testing.T) {
	reg := NewObjectRegistry(Options{})
	owner := sampleOwner()
	id := NewBinaryObjectID()

	record := BinaryObjectRecord{ID: id, Kind: BinaryStorageFile, Owner: owner, State: ObjectStateReady}
	reg.InsertWriting(context.Background(), record)
	reg.SealObject(context.Background(), id, 100, nil)

	reg.Release(context.Background(), id)
	reg.Release(context.Background(), id)

	if reg.CountActive() != 0 {
		t.Fatalf("expected 0 active after release, got %d", reg.CountActive())
	}
}

func TestObjectRegistry_Release_WrongOwner(t *testing.T) {
	reg := NewObjectRegistry(Options{})
	owner := sampleOwner()
	id := NewBinaryObjectID()

	record := BinaryObjectRecord{ID: id, Kind: BinaryStorageFile, Owner: owner, State: ObjectStateReady}
	reg.InsertWriting(context.Background(), record)
	reg.SealObject(context.Background(), id, 100, nil)

	wrongOwner := BinaryOwner{PluginID: "p", RuntimeID: "r", ServiceID: "s", ChannelID: "c"}
	_ = wrongOwner
	stored, err := reg.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("get should succeed: %v", err)
	}
	if stored.Owner.PluginID != "plugin-x" {
		t.Fatal("owner mismatch")
	}
}

func TestObjectRegistry_ListByRuntime(t *testing.T) {
	reg := NewObjectRegistry(Options{})
	owner := sampleOwner()
	id := NewBinaryObjectID()

	record := BinaryObjectRecord{ID: id, Kind: BinaryStorageFile, Owner: owner, State: ObjectStateReady}
	reg.InsertWriting(context.Background(), record)

	list, err := reg.ListByRuntime(domain.RuntimeInstanceID("runtime-1"))
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1, got %d", len(list))
	}
}

func TestObjectRegistry_RemoveByRuntime(t *testing.T) {
	reg := NewObjectRegistry(Options{})
	owner := sampleOwner()
	id := NewBinaryObjectID()

	record := BinaryObjectRecord{ID: id, Kind: BinaryStorageFile, Owner: owner, State: ObjectStateReady}
	reg.InsertWriting(context.Background(), record)

	count, err := reg.RemoveByRuntime(context.Background(), "runtime-1")
	if err != nil {
		t.Fatalf("remove failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1, got %d", count)
	}
	if reg.CountActive() != 0 {
		t.Fatal("expected 0 active")
	}
}

func TestObjectRegistry_RemoveByRuntime_OtherRuntimePreserved(t *testing.T) {
	reg := NewObjectRegistry(Options{})

	ownerA := BinaryOwner{PluginID: "p", RuntimeID: "runtime-a", ServiceID: "s", ChannelID: "c"}
	ownerB := BinaryOwner{PluginID: "p", RuntimeID: "runtime-b", ServiceID: "s", ChannelID: "c"}

	idA := NewBinaryObjectID()
	idB := NewBinaryObjectID()

	reg.InsertWriting(context.Background(), BinaryObjectRecord{ID: idA, Kind: BinaryStorageFile, Owner: ownerA, State: ObjectStateReady})
	reg.InsertWriting(context.Background(), BinaryObjectRecord{ID: idB, Kind: BinaryStorageFile, Owner: ownerB, State: ObjectStateReady})

	reg.RemoveByRuntime(context.Background(), "runtime-a")

	if reg.CountActive() != 1 {
		t.Fatalf("expected 1 active, got %d", reg.CountActive())
	}
}

func TestObjectRegistry_ActiveObjectLimit(t *testing.T) {
	reg := NewObjectRegistry(Options{MaxActiveObjects: 2})
	owner := sampleOwner()

	for i := 0; i < 3; i++ {
		record := BinaryObjectRecord{ID: NewBinaryObjectID(), Kind: BinaryStorageFile, Owner: owner}
		err := reg.InsertWriting(context.Background(), record)
		if i < 2 && err != nil {
			t.Fatalf("insert %d should succeed: %v", i, err)
		}
		if i == 2 && err == nil {
			t.Fatal("third insert should fail due to limit")
		}
	}
}

func TestObjectRegistry_ConcurrentCreate(t *testing.T) {
	reg := NewObjectRegistry(Options{})
	owner := sampleOwner()

	var wg sync.WaitGroup
	const n = 50

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			record := BinaryObjectRecord{ID: NewBinaryObjectID(), Kind: BinaryStorageFile, Owner: owner}
			reg.InsertWriting(context.Background(), record)
		}()
	}
	wg.Wait()

	if reg.CountActive() != n {
		t.Fatalf("expected %d active, got %d", n, reg.CountActive())
	}
}

func TestObjectRegistry_ResolveReleaseRace(t *testing.T) {
	reg := NewObjectRegistry(Options{})
	owner := sampleOwner()
	id := NewBinaryObjectID()

	record := BinaryObjectRecord{ID: id, Kind: BinaryStorageFile, Owner: owner, State: ObjectStateReady}
	reg.InsertWriting(context.Background(), record)
	reg.SealObject(context.Background(), id, 100, nil)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			reg.Get(context.Background(), id)
		}()
		go func() {
			defer wg.Done()
			reg.Release(context.Background(), id)
		}()
	}
	wg.Wait()

	reg.Release(context.Background(), id)
}

func TestBinaryOwner_Validate(t *testing.T) {
	owner := sampleOwner()
	if err := owner.Validate(); err != nil {
		t.Fatalf("valid owner rejected: %v", err)
	}
}

func TestBinaryOwner_EmptyPluginRejected(t *testing.T) {
	owner := BinaryOwner{RuntimeID: "r", ServiceID: "s", ChannelID: "c"}
	if err := owner.Validate(); err == nil {
		t.Fatal("empty plugin should be rejected")
	}
}

func TestBinaryOwner_Key(t *testing.T) {
	owner := sampleOwner()
	expected := "plugin-x/runtime-1/service-a/frames"
	if owner.Key() != expected {
		t.Fatalf("expected %s, got %s", expected, owner.Key())
	}
}

func TestLifetimePolicy_MessageTTL(t *testing.T) {
	policy := DefaultLifetimePolicy()
	now := time.Now()
	expiry := policy.ExpiryTime(BinaryLifetimeMessage, now)
	if !expiry.After(now) {
		t.Fatal("message expiry should be in the future")
	}
}

func TestLifetimePolicy_RuntimeLifetimeNoExpiry(t *testing.T) {
	policy := DefaultLifetimePolicy()
	now := time.Now().UTC()
	expiry := policy.ExpiryTime(BinaryLifetimeRuntime, now)
	if !expiry.IsZero() {
		t.Fatal("runtime lifetime should not have a fixed expiry")
	}
}

func TestContextCancellation_InsertWriting(t *testing.T) {
	reg := NewObjectRegistry(Options{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	record := BinaryObjectRecord{ID: NewBinaryObjectID(), Kind: BinaryStorageFile, Owner: sampleOwner()}
	if err := reg.InsertWriting(ctx, record); err == nil {
		t.Fatal("cancelled context should return error")
	}
}

func TestContextCancellation_SealObject(t *testing.T) {
	reg := NewObjectRegistry(Options{})
	id := NewBinaryObjectID()
	owner := sampleOwner()

	reg.InsertWriting(context.Background(), BinaryObjectRecord{ID: id, Kind: BinaryStorageFile, Owner: owner})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := reg.SealObject(ctx, id, 100, nil); err == nil {
		t.Fatal("cancelled context should return error")
	}
}

var _ = domain.PluginID("")
