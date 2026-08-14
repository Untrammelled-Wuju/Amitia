package secret_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	kernelsecret "github.com/u-ai/backend/internal/extension/kernel/secret"
	"github.com/u-ai/backend/internal/gamehost/secret"
)

func TestLifecycleOrchestrator_AcquireRuntimeStartup_AllRequiredGranted(t *testing.T) {
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
	o := secret.NewLifecycleOrchestrator(a)

	h := o.AcquireRuntimeStartup(context.Background(), secret.RuntimeSecretManifest{
		RuntimeID:  "rt-1",
		PluginID:   "p",
		ServiceID:  "svc-1",
		Generation: 1,
		Startup: []secret.ServiceSecretManifest{
			{Ref: kernelsecret.SecretRef(refOpenAI), Purpose: secret.PurposeStartup, Required: true},
		},
	})

	if h.Failed {
		t.Fatalf("expected success, got failure: %v", h.LastError)
	}
	if len(h.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(h.Results))
	}
	if !h.Results[0].Result.Granted {
		t.Fatal("expected first result granted")
	}
}

func TestLifecycleOrchestrator_AcquireRuntimeStartup_RequiredFails_RevokesAcquired(t *testing.T) {
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
	o := secret.NewLifecycleOrchestrator(a)

	b.issueErr = errors.New("store unavailable")

	h := o.AcquireRuntimeStartup(context.Background(), secret.RuntimeSecretManifest{
		RuntimeID:  "rt-1",
		PluginID:   "p",
		ServiceID:  "svc-1",
		Generation: 1,
		Startup: []secret.ServiceSecretManifest{
			{Ref: kernelsecret.SecretRef(refOpenAI), Purpose: secret.PurposeStartup, Required: true},
		},
	})

	if !h.Failed {
		t.Fatal("expected failure")
	}
	if len(a.ActiveServiceLeases("rt-1", "svc-1")) != 0 {
		t.Error("expected no active leases on failure path")
	}
}

func TestLifecycleOrchestrator_AcquireRuntimeStartup_NoSecrets_Success(t *testing.T) {
	b := newFakeBroker()
	id := newFakeIdentity()
	g := &fakeGate{allow: true}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}
	o := secret.NewLifecycleOrchestrator(a)

	h := o.AcquireRuntimeStartup(context.Background(), secret.RuntimeSecretManifest{
		RuntimeID:  "rt-1",
		PluginID:   "p",
		ServiceID:  "svc-1",
		Generation: 1,
	})

	if h.Failed {
		t.Fatal("should not fail on empty manifest")
	}
	if len(h.Results) != 0 {
		t.Error("expected 0 results on empty manifest")
	}
}

func TestLifecycleOrchestrator_AcquireRuntimeStartup_AllRequiredFail_NoLeaseLeak(t *testing.T) {
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

	b.issueErr = errors.New("store unavailable")
	o := secret.NewLifecycleOrchestrator(a)

	h := o.AcquireRuntimeStartup(context.Background(), secret.RuntimeSecretManifest{
		RuntimeID:  "rt-1",
		PluginID:   "p",
		ServiceID:  "svc-1",
		Generation: 1,
		Startup: []secret.ServiceSecretManifest{
			{Ref: kernelsecret.SecretRef(refOpenAI), Purpose: secret.PurposeStartup, Required: true},
		},
	})

	if !h.Failed {
		t.Fatal("expected failure when all issues fail")
	}
	if len(a.ActiveServiceLeases("rt-1", "svc-1")) != 0 {
		t.Error("expected no active leases on store failure")
	}
}

func TestLifecycleOrchestrator_Stop_TriggersRevoke(t *testing.T) {
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
	o := secret.NewLifecycleOrchestrator(a)

	_, _ = a.AcquireServiceLease(context.Background(), "rt-1", "p", "svc-1",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeStartup, true, 1)

	outcome := o.RevokeServiceOnStop("rt-1", "svc-1", "test")
	if outcome.RevokedCount != 1 {
		t.Fatalf("expected 1 revoked, got %d", outcome.RevokedCount)
	}
	if len(a.ActiveServiceLeases("rt-1", "svc-1")) != 0 {
		t.Error("expected zero leases after stop")
	}
}

func TestLifecycleOrchestrator_RuntimeStop_TriggersRevoke(t *testing.T) {
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
	o := secret.NewLifecycleOrchestrator(a)

	_, _ = a.AcquireServiceLease(context.Background(), "rt-1", "p", "svc-1",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeStartup, true, 1)

	outcome := o.RevokeRuntimeOnStop("rt-1", "runtime stop")
	if outcome.RevokedCount != 1 {
		t.Fatalf("expected 1 revoked, got %d", outcome.RevokedCount)
	}
	if len(a.ActiveRuntimeLeases("rt-1")) != 0 {
		t.Error("expected zero runtime leases after stop")
	}
}

func TestLifecycleOrchestrator_Disable_TriggersRevoke(t *testing.T) {
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
	o := secret.NewLifecycleOrchestrator(a)

	_, _ = a.AcquireServiceLease(context.Background(), "rt-1", "p", "svc-1",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeStartup, true, 1)

	outcome := o.RevokeOnDisable("ext-d", "extension disabled")
	if outcome.RevokedCount != 1 {
		t.Fatalf("expected 1 revoked, got %d", outcome.RevokedCount)
	}
	if len(a.ActiveExtensionLeases("ext-d")) != 0 {
		t.Error("expected zero extension leases after disable")
	}
}

func TestLifecycleOrchestrator_Uninstall_TriggersRevoke(t *testing.T) {
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
	o := secret.NewLifecycleOrchestrator(a)

	_, _ = a.AcquireServiceLease(context.Background(), "rt-1", "p", "svc-1",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeStartup, true, 1)

	outcome := o.RevokeOnUninstall("ext", "uninstalled")
	if outcome.RevokedCount != 1 {
		t.Fatalf("expected 1 revoked, got %d", outcome.RevokedCount)
	}
}

func TestLifecycleOrchestrator_ConcurrentAcquireAndStop(t *testing.T) {
	b := newFakeBroker()
	b.SeedSecret(refOpenAI, "v")
	b.SeedSecret(refDB, "v2")
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
		go func(i int) {
			defer wg.Done()
			_, _ = a.AcquireServiceLease(context.Background(), "rt-1", "p", "svc-1",
				kernelsecret.SecretRef(refOpenAI), secret.PurposeStartup, false, int64(i%3))
		}(i)
	}
	wg.Wait()

	o := secret.NewLifecycleOrchestrator(a)
	outcome := o.RevokeRuntimeOnStop("rt-1", "test")
	if outcome.RevokedCount < 1 {
		t.Errorf("expected some revoked, got %d", outcome.RevokedCount)
	}
}
