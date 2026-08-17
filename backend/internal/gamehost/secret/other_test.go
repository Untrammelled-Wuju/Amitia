package secret_test

import (
	"context"
	"strconv"
	"sync"
	"testing"

	kernelsecret "github.com/u-ai/backend/internal/extension/kernel/secret"
	"github.com/u-ai/backend/internal/gamehost/secret"
)

// ===== Binding Index =====

func TestBindingIndex_RecordLookup(t *testing.T) {
	idx := secret.NewLeaseBindingIndex(nil)
	k := kernelsecret.LeaseID("lease-1")

	idx.Record(k, "rt-1", "svc-1", "ref-1", "ext-1", 1, secret.PurposeStartup)

	entry, ok := idx.LookupByService("rt-1", "svc-1", "ref-1", 1)
	if !ok || entry.KernelLease != k {
		t.Fatalf("expected lookup success")
	}
	if entry.ExtensionID != "ext-1" {
		t.Errorf("expected ext-1, got %s", entry.ExtensionID)
	}

	byLease, ok := idx.LookupByLease(k)
	if !ok || byLease.RuntimeID != "rt-1" {
		t.Fatal("expected lease lookup")
	}
}

func TestBindingIndex_ActiveLeases(t *testing.T) {
	idx := secret.NewLeaseBindingIndex(nil)
	idx.Record("lk-1", "rt-1", "svc-1", "r1", "ext", 1, secret.PurposeStartup)
	idx.Record("lk-2", "rt-1", "svc-2", "r2", "ext", 1, secret.PurposeRuntime)
	idx.Record("lk-3", "rt-2", "svc-1", "r3", "ext", 1, secret.PurposeStartup)

	svc1 := idx.ActiveLeasesByService("rt-1", "svc-1")
	if len(svc1) != 1 || svc1[0] != "lk-1" {
		t.Errorf("expected rt-1/svc-1 => [lk-1], got %v", svc1)
	}

	rt1 := idx.ActiveLeasesByRuntime("rt-1")
	if len(rt1) != 2 {
		t.Errorf("expected 2 active on rt-1, got %d: %v", len(rt1), rt1)
	}

	ext := idx.ActiveLeasesByExtension("ext")
	if len(ext) != 3 {
		t.Errorf("expected 3 ext leases, got %d", len(ext))
	}
}

func TestBindingIndex_MarkRevoked(t *testing.T) {
	idx := secret.NewLeaseBindingIndex(nil)
	idx.Record("lk-1", "rt-1", "svc-1", "r1", "ext", 1, secret.PurposeStartup)

	if !idx.MarkRevoked("lk-1") {
		t.Fatal("expected MarkRevoked true")
	}
	if idx.MarkRevoked("lk-1") {
		t.Fatal("expected MarkRevoked false on second call")
	}

	if len(idx.ActiveLeasesByService("rt-1", "svc-1")) != 0 {
		t.Error("expected no active after revoke")
	}

	rtAll := idx.ActiveLeasesByRuntime("rt-1")
	if len(rtAll) != 0 {
		t.Errorf("expected no active on runtime, got %v", rtAll)
	}
}

func TestBindingIndex_Clear(t *testing.T) {
	idx := secret.NewLeaseBindingIndex(nil)
	idx.Record("lk-1", "rt-1", "svc-1", "r1", "ext", 1, secret.PurposeStartup)
	idx.Clear()

	if len(idx.ActiveLeasesByRuntime("rt-1")) != 0 {
		t.Error("expected empty after clear")
	}
}

// ===== Subscription =====

func TestSubscriptionAdapter_OnPermissionRevoked(t *testing.T) {
	b := newFakeBroker()
	b.SeedSecret(refOpenAI, "v")
	id := newFakeIdentity()
	id.AddRuntime("rt-1", "p", "ext", "created")
	id.AddService("rt-1", "svc-1", "p", "ext", "running")
	g := &fakeGate{allow: true}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}
	s := secret.NewSubscriptionAdapter(a)

	_, _ = a.AcquireServiceLease(context.Background(), "rt-1", "p", "svc-1",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeStartup, true, 1)

	s.OnPermissionRevoked("ext", "rt-1")
	if len(a.ActiveRuntimeLeases("rt-1")) != 0 {
		t.Error("expected no active after permission revoke")
	}
}

func TestSubscriptionAdapter_OnPermissionRevoked_EmptyIDs(t *testing.T) {
	b := newFakeBroker()
	id := newFakeIdentity()
	g := &fakeGate{allow: true}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}
	s := secret.NewSubscriptionAdapter(a)

	s.OnPermissionRevoked("", "")
}

func TestSubscriptionAdapter_OnExtensionDisabled(t *testing.T) {
	b := newFakeBroker()
	b.SeedSecret(refOpenAI, "v")
	id := newFakeIdentity()
	id.AddRuntime("rt-1", "p", "ext-d", "created")
	id.AddService("rt-1", "svc-1", "p", "ext-d", "running")
	g := &fakeGate{allow: true}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}
	s := secret.NewSubscriptionAdapter(a)

	_, _ = a.AcquireServiceLease(context.Background(), "rt-1", "p", "svc-1",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeStartup, true, 1)

	s.OnExtensionDisabled("ext-d")
	if len(a.ActiveExtensionLeases("ext-d")) != 0 {
		t.Error("expected no active after disable")
	}
}

func TestSubscriptionAdapter_OnExtensionUninstalled(t *testing.T) {
	b := newFakeBroker()
	b.SeedSecret(refOpenAI, "v")
	id := newFakeIdentity()
	id.AddRuntime("rt-1", "p", "ext", "created")
	id.AddService("rt-1", "svc-1", "p", "ext", "running")
	g := &fakeGate{allow: true}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}
	s := secret.NewSubscriptionAdapter(a)

	_, _ = a.AcquireServiceLease(context.Background(), "rt-1", "p", "svc-1",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeStartup, true, 1)
	s.OnExtensionUninstalled("ext")
	if len(a.ActiveExtensionLeases("ext")) != 0 {
		t.Error("expected no active after uninstall")
	}
}

func TestSubscriptionAdapter_OnServiceStopped(t *testing.T) {
	b := newFakeBroker()
	b.SeedSecret(refOpenAI, "v")
	id := newFakeIdentity()
	id.AddRuntime("rt-1", "p", "ext", "created")
	id.AddService("rt-1", "svc-1", "p", "ext", "running")
	g := &fakeGate{allow: true}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}
	s := secret.NewSubscriptionAdapter(a)

	_, _ = a.AcquireServiceLease(context.Background(), "rt-1", "p", "svc-1",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeStartup, true, 1)
	s.OnServiceStopped("rt-1", "svc-1")
	if len(a.ActiveServiceLeases("rt-1", "svc-1")) != 0 {
		t.Error("expected no active after service stop")
	}
}

func TestSubscriptionAdapter_OnRuntimeStopped(t *testing.T) {
	b := newFakeBroker()
	b.SeedSecret(refOpenAI, "v")
	id := newFakeIdentity()
	id.AddRuntime("rt-1", "p", "ext", "created")
	id.AddService("rt-1", "svc-1", "p", "ext", "running")
	g := &fakeGate{allow: true}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}
	s := secret.NewSubscriptionAdapter(a)

	_, _ = a.AcquireServiceLease(context.Background(), "rt-1", "p", "svc-1",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeStartup, true, 1)
	s.OnRuntimeStopped("rt-1")
	if len(a.ActiveRuntimeLeases("rt-1")) != 0 {
		t.Error("expected no active after runtime stop")
	}
}

func TestSubscriptionAdapter_OnRuntimeRestarted(t *testing.T) {
	b := newFakeBroker()
	b.SeedSecret(refOpenAI, "v")
	id := newFakeIdentity()
	id.AddRuntime("rt-1", "p", "ext", "created")
	id.AddService("rt-1", "svc-1", "p", "ext", "running")
	g := &fakeGate{allow: true}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}
	s := secret.NewSubscriptionAdapter(a)

	_, _ = a.AcquireServiceLease(context.Background(), "rt-1", "p", "svc-1",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeStartup, true, 1)
	s.OnRuntimeRestarted("rt-1")
	if len(a.ActiveRuntimeLeases("rt-1")) != 0 {
		t.Error("expected no active after runtime restart")
	}
}

// ===== Race Tests =====

func TestRace_AcquireAndStop(t *testing.T) {
	b := newFakeBroker()
	b.SeedSecret(refOpenAI, "v")
	id := newFakeIdentity()
	id.AddRuntime("rt-1", "p", "ext", "created")
	id.AddService("rt-1", "svc-1", "p", "ext", "running")
	g := &fakeGate{allow: true}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = a.AcquireServiceLease(context.Background(), "rt-1", "p", "svc-1",
				kernelsecret.SecretRef(refOpenAI), secret.PurposeStartup, false, 1)
		}()
	}
	wg.Wait()
}

func TestRace_AcquireParallelDifferentRuntimes(t *testing.T) {
	b := newFakeBroker()
	b.SeedSecret(refOpenAI, "v")
	id := newFakeIdentity()
	g := &fakeGate{allow: true}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		rtID := "rt-" + strconv.Itoa(i)
		id.AddRuntime(rtID, "p", "ext", "created")
		id.AddService(rtID, "svc-1", "p", "ext", "running")
	}
	for i := 0; i < 20; i++ {
		rtID := "rt-" + strconv.Itoa(i)
		wg.Add(1)
		go func(rt string) {
			defer wg.Done()
			_, _ = a.AcquireServiceLease(context.Background(), rt, "p", "svc-1",
				kernelsecret.SecretRef(refOpenAI), secret.PurposeStartup, true, 1)
		}(rtID)
	}
	wg.Wait()
}

func TestRace_AcquireVsRevoke(t *testing.T) {
	b := newFakeBroker()
	b.SeedSecret(refOpenAI, "v")
	id := newFakeIdentity()
	id.AddRuntime("rt-1", "p", "ext", "created")
	id.AddService("rt-1", "svc-1", "p", "ext", "running")
	g := &fakeGate{allow: true}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	done := make(chan struct{})

	go func() {
		for i := 0; i < 50; i++ {
			_, _ = a.AcquireServiceLease(context.Background(), "rt-1", "p", "svc-1",
				kernelsecret.SecretRef(refOpenAI), secret.PurposeStartup, false, int64(i%5))
		}
		close(done)
	}()

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.RevokeServiceLeases("rt-1", "svc-1", "test")
			a.RevokeRuntimeLeases("rt-1", "test")
		}()
	}
	<-done
	wg.Wait()
}

func TestSubscriptionAdapter_RevokeGrant_RealIdentity(t *testing.T) {
	b := newFakeBroker()
	b.SeedSecret(refOpenAI, "v")
	id := newFakeIdentity()
	id.AddRuntime("rt-1", "p", "ext-1", "created")
	id.AddService("rt-1", "svc-1", "p", "ext-1", "running")
	g := &fakeGate{allow: true}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}
	s := secret.NewSubscriptionAdapter(a)

	res, err := a.AcquireServiceLease(context.Background(), "rt-1", "p", "svc-1",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeStartup, true, 1)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if !res.Granted {
		t.Fatal("expected granted")
	}

	outcome := s.RevokeGrant(res.LeaseID)
	if outcome.RevokedCount != 1 {
		t.Errorf("expected 1 revoked, got %d", outcome.RevokedCount)
	}
	if len(a.ActiveRuntimeLeases("rt-1")) != 0 {
		t.Error("expected no active leases after RevokeGrant")
	}
}

func TestSubscriptionAdapter_RevokeGrant_NotFound(t *testing.T) {
	b := newFakeBroker()
	id := newFakeIdentity()
	g := &fakeGate{allow: true}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}
	s := secret.NewSubscriptionAdapter(a)

	outcome := s.RevokeGrant(kernelsecret.LeaseID("nonexistent"))
	if outcome.RevokedCount != 0 {
		t.Errorf("expected 0 revoked for missing grant, got %d", outcome.RevokedCount)
	}
}

func TestSubscriptionAdapter_RevokeBySubject(t *testing.T) {
	b := newFakeBroker()
	b.SeedSecret(refOpenAI, "v")
	id := newFakeIdentity()
	id.AddRuntime("rt-1", "p", "ext", "created")
	id.AddService("rt-1", "svc-1", "p", "ext", "running")
	g := &fakeGate{allow: true}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}
	s := secret.NewSubscriptionAdapter(a)

	_, err = a.AcquireServiceLease(context.Background(), "rt-1", "p", "svc-1",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeStartup, true, 1)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}

	outcome := s.RevokeBySubject("rt-1")
	if outcome.RevokedCount == 0 {
		t.Error("expected at least one revoked by subject")
	}
	if len(a.ActiveRuntimeLeases("rt-1")) != 0 {
		t.Error("expected no active leases after RevokeBySubject")
	}
}

func TestSubscriptionAdapter_RevokeByExtension(t *testing.T) {
	b := newFakeBroker()
	b.SeedSecret(refOpenAI, "v")
	id := newFakeIdentity()
	id.AddRuntime("rt-1", "p", "ext-1", "created")
	id.AddService("rt-1", "svc-1", "p", "ext-1", "running")
	g := &fakeGate{allow: true}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}
	s := secret.NewSubscriptionAdapter(a)

	_, err = a.AcquireServiceLease(context.Background(), "rt-1", "p", "svc-1",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeStartup, true, 1)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}

	outcome := s.RevokeByExtension("ext-1")
	if outcome.RevokedCount == 0 {
		t.Error("expected at least one revoked by extension")
	}
	if len(a.ActiveExtensionLeases("ext-1")) != 0 {
		t.Error("expected no active leases after RevokeByExtension")
	}
}

func TestSubscriptionAdapter_RevokeBySubject_Empty(t *testing.T) {
	b := newFakeBroker()
	id := newFakeIdentity()
	g := &fakeGate{allow: true}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}
	s := secret.NewSubscriptionAdapter(a)

	outcome := s.RevokeBySubject("")
	if outcome.RevokedCount != 0 {
		t.Error("expected 0 for empty subject")
	}
}

func TestSubscriptionAdapter_RevokeByExtension_Empty(t *testing.T) {
	b := newFakeBroker()
	id := newFakeIdentity()
	g := &fakeGate{allow: true}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}
	s := secret.NewSubscriptionAdapter(a)

	outcome := s.RevokeByExtension("")
	if outcome.RevokedCount != 0 {
		t.Error("expected 0 for empty extension")
	}
}
