package secret_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	kernelsecret "github.com/u-ai/backend/internal/extension/kernel/secret"
	"github.com/u-ai/backend/internal/gamehost/secret"
)

type fakeBroker struct {
	leases   map[kernelsecret.LeaseID]*kernelsecret.Lease
	store    map[string][]byte
	issueErr error
	mu       sync.Mutex
	seq      int
}

func newFakeBroker() *fakeBroker {
	return &fakeBroker{
		leases: make(map[kernelsecret.LeaseID]*kernelsecret.Lease),
		store:  make(map[string][]byte),
	}
}

func (b *fakeBroker) SeedSecret(ref, value string) {
	b.store[ref] = []byte(value)
}

func (b *fakeBroker) Issue(ctx context.Context, req kernelsecret.LeaseRequest) (kernelsecret.Lease, error) {
	if b.issueErr != nil {
		return kernelsecret.Lease{}, b.issueErr
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.store[string(req.Ref)]; !ok {
		return kernelsecret.Lease{}, kernelsecret.ErrSecretNotFound
	}

	b.seq++
	id := kernelsecret.LeaseID("lease-" + strconv.Itoa(b.seq))
	l := kernelsecret.Lease{
		ID:                id,
		Ref:               req.Ref,
		Purpose:           req.Purpose,
		RuntimeInstanceID: req.RuntimeInstanceID,
		ExtensionID:       req.ExtensionID,
		ModuleID:          req.ModuleID,
		Generation:        req.Generation,
		IssuedAt:          time.Now().UTC(),
		ExpiresAt:         time.Now().UTC().Add(5 * time.Minute),
		MaxUses:           1,
	}
	b.leases[id] = &l
	return l.Clone(), nil
}

func (b *fakeBroker) RevokeLease(leaseID kernelsecret.LeaseID) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if l, ok := b.leases[leaseID]; ok {
		l.Revoked = true
	}
	return nil
}

func (b *fakeBroker) GetLease(leaseID kernelsecret.LeaseID) (kernelsecret.Lease, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	l, ok := b.leases[leaseID]
	if !ok {
		return kernelsecret.Lease{}, false
	}
	return l.Clone(), true
}

func (b *fakeBroker) RevokeByRuntimeInstance(instanceID string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	count := 0
	for _, l := range b.leases {
		if l.RuntimeInstanceID == instanceID {
			l.Revoked = true
			count++
		}
	}
	return count
}

func (b *fakeBroker) RevokeAll() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	count := 0
	for _, l := range b.leases {
		if !l.Revoked {
			l.Revoked = true
			count++
		}
	}
	return count
}

type fakeIdentity struct {
	mu       sync.RWMutex
	runtimes map[string]identityEntry
	services map[string]identityEntry
	enabled  map[string]bool
}

type identityEntry struct {
	pluginID    string
	extensionID string
	state       string
	generation  int64
}

func newFakeIdentity() *fakeIdentity {
	return &fakeIdentity{
		runtimes: make(map[string]identityEntry),
		services: make(map[string]identityEntry),
		enabled:  make(map[string]bool),
	}
}

func (f *fakeIdentity) AddRuntime(rtID, pluginID, extID, state string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runtimes[rtID] = identityEntry{pluginID: pluginID, extensionID: extID, state: state, generation: 1}
	f.enabled[extID] = true
}

func (f *fakeIdentity) AddService(rtID, svcID, pluginID, extID, state string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.services[rtID+"/"+svcID] = identityEntry{pluginID: pluginID, extensionID: extID, state: state}
}

func (f *fakeIdentity) ResolveRuntime(ctx context.Context, rtID string) (string, string, string, int64, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	e, ok := f.runtimes[rtID]
	if !ok {
		return "", "", "", 0, errors.New("runtime not found")
	}
	return e.pluginID, e.extensionID, e.state, e.generation, nil
}

func (f *fakeIdentity) ResolveService(ctx context.Context, rtID, svcID string) (string, string, string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	e, ok := f.services[rtID+"/"+svcID]
	if !ok {
		return "", "", "", errors.New("service not found")
	}
	return e.pluginID, e.extensionID, e.state, nil
}

func (f *fakeIdentity) ExtensionEnabled(ctx context.Context, extID string) (bool, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.enabled[extID], nil
}

func (f *fakeIdentity) DisableExtension(extID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.enabled[extID] = false
}

type fakeGate struct {
	mu    sync.Mutex
	allow bool
	calls int
}

func (g *fakeGate) CheckSecretUse(ctx context.Context, extID, pluginID, rtID, svcID, ref string) (secret.SecretPermissionDecision, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls++
	if g.allow {
		return secret.SecretPermissionDecision{Allowed: true}, nil
	}
	return secret.SecretPermissionDecision{Allowed: false, Reason: "denied"}, fmt.Errorf("permission denied")
}

func (g *fakeGate) Calls() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.calls
}

const (
	refOpenAI = "secret://provider/openai"
	refDB     = "secret://provider/db"
)

// ===== Basic Acquire =====

func TestAcquireServiceLease_ValidService_Granted(t *testing.T) {
	b := newFakeBroker()
	b.SeedSecret(refOpenAI, "sk-real")
	id := newFakeIdentity()
	id.AddRuntime("rt-1", "plugin-x", "ext-x", "created")
	id.AddService("rt-1", "svc-1", "plugin-x", "ext-x", "running")
	g := &fakeGate{allow: true}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	res, err := a.AcquireServiceLease(context.Background(), "rt-1", "plugin-x", "svc-1",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeStartup, true, 1)
	if err != nil {
		t.Fatalf("expected grant, got err=%v", err)
	}
	if !res.Granted {
		t.Fatalf("expected granted=true")
	}
	if res.LeaseID == "" {
		t.Fatal("expected non-empty lease id")
	}
	if g.Calls() != 1 {
		t.Errorf("expected 1 gate call, got %d", g.Calls())
	}
}

func TestAcquireServiceLease_UnknownRuntime_Deny(t *testing.T) {
	b := newFakeBroker()
	id := newFakeIdentity()
	g := &fakeGate{allow: true}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	_, err = a.AcquireServiceLease(context.Background(), "rt-unknown", "p", "s",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeRuntime, true, 1)
	if !errors.Is(err, secret.ErrRuntimeInvalid) {
		t.Fatalf("expected ErrRuntimeInvalid, got %v", err)
	}
}

func TestAcquireServiceLease_UnknownService_Deny(t *testing.T) {
	b := newFakeBroker()
	id := newFakeIdentity()
	id.AddRuntime("rt-1", "p", "ext", "created")
	g := &fakeGate{allow: true}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	_, err = a.AcquireServiceLease(context.Background(), "rt-1", "p", "s-unknown",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeRuntime, true, 1)
	if !errors.Is(err, secret.ErrServiceInvalid) {
		t.Fatalf("expected ErrServiceInvalid, got %v", err)
	}
}

func TestAcquireServiceLease_PluginMismatch_Deny(t *testing.T) {
	b := newFakeBroker()
	id := newFakeIdentity()
	id.AddRuntime("rt-1", "p1", "ext", "created")
	id.AddService("rt-1", "svc-1", "p2", "ext", "running")
	g := &fakeGate{allow: true}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	_, err = a.AcquireServiceLease(context.Background(), "rt-1", "p1", "svc-1",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeRuntime, true, 1)
	if !errors.Is(err, secret.ErrBindingInvalid) {
		t.Fatalf("expected ErrBindingInvalid, got %v", err)
	}
}

func TestAcquireServiceLease_ExtensionExtMismatch_Deny(t *testing.T) {
	b := newFakeBroker()
	id := newFakeIdentity()
	id.AddRuntime("rt-1", "p", "ext-a", "created")
	id.AddService("rt-1", "svc-1", "p", "ext-b", "running")
	g := &fakeGate{allow: true}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	_, err = a.AcquireServiceLease(context.Background(), "rt-1", "p", "svc-1",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeRuntime, true, 1)
	if !errors.Is(err, secret.ErrBindingInvalid) {
		t.Fatalf("expected ErrBindingInvalid, got %v", err)
	}
}

// ===== Permission / Gate =====

func TestAcquireServiceLease_PermissionDenied(t *testing.T) {
	b := newFakeBroker()
	b.SeedSecret(refOpenAI, "v")
	id := newFakeIdentity()
	id.AddRuntime("rt-1", "p", "ext", "created")
	id.AddService("rt-1", "svc-1", "p", "ext", "running")
	g := &fakeGate{allow: false}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	_, err = a.AcquireServiceLease(context.Background(), "rt-1", "p", "svc-1",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeRuntime, true, 1)
	if !errors.Is(err, secret.ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied, got %v", err)
	}
}

func TestAcquireServiceLease_ExtensionDisabled_Deny(t *testing.T) {
	b := newFakeBroker()
	b.SeedSecret(refOpenAI, "v")
	id := newFakeIdentity()
	id.AddRuntime("rt-1", "p", "ext-d", "created")
	id.AddService("rt-1", "svc-1", "p", "ext-d", "running")
	id.DisableExtension("ext-d")
	g := &fakeGate{allow: true}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	_, err = a.AcquireServiceLease(context.Background(), "rt-1", "p", "svc-1",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeRuntime, true, 1)
	if !errors.Is(err, secret.ErrExtensionDisabled) {
		t.Fatalf("expected ErrExtensionDisabled, got %v", err)
	}
}

// ===== Scope Isolation =====

func TestAcquireServiceLease_CrossRuntime_Deny(t *testing.T) {
	b := newFakeBroker()
	id := newFakeIdentity()
	id.AddRuntime("rt-1", "p", "ext", "created")
	id.AddRuntime("rt-2", "p", "ext", "created")
	id.AddService("rt-1", "svc-1", "p", "ext", "running")
	g := &fakeGate{allow: true}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	_, err = a.AcquireServiceLease(context.Background(), "rt-2", "p", "svc-1",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeRuntime, true, 1)
	if !errors.Is(err, secret.ErrServiceInvalid) {
		t.Fatalf("expected ErrServiceInvalid for cross-runtime, got %v", err)
	}
}

func TestAcquireServiceLease_CrossPlugin_Deny(t *testing.T) {
	b := newFakeBroker()
	id := newFakeIdentity()
	id.AddRuntime("rt-1", "p1", "ext", "created")
	id.AddService("rt-1", "svc-1", "p1", "ext", "running")
	g := &fakeGate{allow: true}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	_, err = a.AcquireServiceLease(context.Background(), "rt-1", "p2", "svc-1",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeRuntime, true, 1)
	if !errors.Is(err, secret.ErrBindingInvalid) {
		t.Fatalf("expected ErrBindingInvalid for cross-plugin, got %v", err)
	}
}

// ===== Revocation =====

func TestRevokeServiceLeases_ClearsService(t *testing.T) {
	b := newFakeBroker()
	b.SeedSecret(refOpenAI, "value")
	id := newFakeIdentity()
	id.AddRuntime("rt-1", "p", "ext", "created")
	id.AddService("rt-1", "svc-1", "p", "ext", "running")
	g := &fakeGate{allow: true}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	res, err := a.AcquireServiceLease(context.Background(), "rt-1", "p", "svc-1",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeStartup, true, 1)
	if err != nil {
		t.Fatal(err)
	}

	outcome := a.RevokeServiceLeases("rt-1", "svc-1", "test revoke")
	if outcome.RevokedCount != 1 {
		t.Fatalf("expected 1 revoked, got %d", outcome.RevokedCount)
	}
	if len(a.ActiveServiceLeases("rt-1", "svc-1")) != 0 {
		t.Error("expected zero active leases after revoke")
	}

	l, _ := b.GetLease(res.LeaseID)
	if !l.Revoked {
		t.Fatal("expected kernel lease revoked")
	}
}

func TestRevokeRuntimeLeases_ClearsAll(t *testing.T) {
	b := newFakeBroker()
	b.SeedSecret(refOpenAI, "value")
	b.SeedSecret(refDB, "value2")
	id := newFakeIdentity()
	id.AddRuntime("rt-1", "p", "ext", "created")
	id.AddService("rt-1", "svc-1", "p", "ext", "running")
	id.AddService("rt-1", "svc-2", "p", "ext", "running")
	g := &fakeGate{allow: true}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	_, _ = a.AcquireServiceLease(context.Background(), "rt-1", "p", "svc-1",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeStartup, true, 1)
	_, _ = a.AcquireServiceLease(context.Background(), "rt-1", "p", "svc-2",
		kernelsecret.SecretRef(refDB), secret.PurposeRuntime, false, 1)

	outcome := a.RevokeRuntimeLeases("rt-1", "runtime stop")
	if outcome.RevokedCount < 1 {
		t.Fatalf("expected >= 1 revoked, got %d", outcome.RevokedCount)
	}
	if len(a.ActiveRuntimeLeases("rt-1")) != 0 {
		t.Error("expected zero active leases after runtime revoke")
	}
}

func TestRevokeExtensionLeases_ClearsExtension(t *testing.T) {
	b := newFakeBroker()
	b.SeedSecret(refOpenAI, "value")
	id := newFakeIdentity()
	id.AddRuntime("rt-1", "p", "ext-a", "created")
	id.AddRuntime("rt-2", "p", "ext-a", "created")
	id.AddService("rt-1", "svc-1", "p", "ext-a", "running")
	id.AddService("rt-2", "svc-1", "p", "ext-a", "running")
	g := &fakeGate{allow: true}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	_, _ = a.AcquireServiceLease(context.Background(), "rt-1", "p", "svc-1",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeStartup, true, 1)
	_, _ = a.AcquireServiceLease(context.Background(), "rt-2", "p", "svc-1",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeStartup, true, 1)

	outcome := a.RevokeExtensionLeases("ext-a", "disable")
	if outcome.RevokedCount != 2 {
		t.Fatalf("expected 2 revoked, got %d", outcome.RevokedCount)
	}
	if len(a.ActiveExtensionLeases("ext-a")) != 0 {
		t.Error("expected zero active leases for extension")
	}
}

func TestRevokeServiceLeavesOtherServicesUnaffected(t *testing.T) {
	b := newFakeBroker()
	b.SeedSecret(refOpenAI, "v")
	b.SeedSecret(refDB, "v2")
	id := newFakeIdentity()
	id.AddRuntime("rt-1", "p", "ext", "created")
	id.AddService("rt-1", "svc-1", "p", "ext", "running")
	id.AddService("rt-1", "svc-2", "p", "ext", "running")
	g := &fakeGate{allow: true}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	_, _ = a.AcquireServiceLease(context.Background(), "rt-1", "p", "svc-1",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeStartup, true, 1)
	res2, _ := a.AcquireServiceLease(context.Background(), "rt-1", "p", "svc-2",
		kernelsecret.SecretRef(refDB), secret.PurposeStartup, true, 1)

	a.RevokeServiceLeases("rt-1", "svc-1", "only svc-1")

	if len(a.ActiveServiceLeases("rt-1", "svc-1")) != 0 {
		t.Error("svc-1 should be revoked")
	}
	active := a.ActiveServiceLeases("rt-1", "svc-2")
	if len(active) != 1 || active[0] != res2.LeaseID {
		t.Errorf("svc-2 should remain active, got %v", active)
	}
}

// ===== Stop / Shutdown =====

func TestAcquireServiceLease_WhenServiceStopping_Deny(t *testing.T) {
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

	a.MarkServiceStopping("rt-1", "svc-1")
	_, err = a.AcquireServiceLease(context.Background(), "rt-1", "p", "svc-1",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeRuntime, true, 1)
	if !errors.Is(err, secret.ErrServiceStopped) {
		t.Fatalf("expected ErrServiceStopped, got %v", err)
	}
}

func TestAcquireServiceLease_WhenShutdown_Deny(t *testing.T) {
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

	a.Shutdown()
	_, err = a.AcquireServiceLease(context.Background(), "rt-1", "p", "svc-1",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeRuntime, true, 1)
	if !errors.Is(err, secret.ErrHostShutdown) {
		t.Fatalf("expected ErrHostShutdown, got %v", err)
	}
}

func TestRevokeAll_Clears(t *testing.T) {
	b := newFakeBroker()
	b.SeedSecret(refOpenAI, "v")
	b.SeedSecret(refDB, "v2")
	id := newFakeIdentity()
	id.AddRuntime("rt-1", "p", "ext", "created")
	id.AddRuntime("rt-2", "p2", "ext2", "created")
	id.AddService("rt-1", "svc-1", "p", "ext", "running")
	id.AddService("rt-2", "svc-1", "p2", "ext2", "running")
	g := &fakeGate{allow: true}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	_, _ = a.AcquireServiceLease(context.Background(), "rt-1", "p", "svc-1",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeStartup, true, 1)
	_, _ = a.AcquireServiceLease(context.Background(), "rt-2", "p2", "svc-1",
		kernelsecret.SecretRef(refDB), secret.PurposeStartup, true, 1)

	outcome := a.RevokeAll()
	if outcome.RevokedCount != 2 {
		t.Fatalf("expected 2 revoked on shutdown, got %d", outcome.RevokedCount)
	}
}

// ===== Idempotency =====

func TestRevokeServiceLeases_Idempotent(t *testing.T) {
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

	_, _ = a.AcquireServiceLease(context.Background(), "rt-1", "p", "svc-1",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeStartup, true, 1)

	o1 := a.RevokeServiceLeases("rt-1", "svc-1", "first")
	o2 := a.RevokeServiceLeases("rt-1", "svc-1", "second")
	if o1.RevokedCount != 1 {
		t.Errorf("first revoke count=%d", o1.RevokedCount)
	}
	if o2.RevokedCount != 0 {
		t.Errorf("second (idempotent) revoke count=%d", o2.RevokedCount)
	}
}

// ===== Service / Runtime / Extension Mismatch Tests =====

func TestAcquireServiceLease_InvalidEmptyFields(t *testing.T) {
	b := newFakeBroker()
	id := newFakeIdentity()
	g := &fakeGate{allow: true}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	_, err = a.AcquireServiceLease(context.Background(), "", "p", "s",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeRuntime, true, 1)
	if !errors.Is(err, secret.ErrRuntimeInvalid) {
		t.Fatalf("expected ErrRuntimeInvalid for empty runtime, got %v", err)
	}

	_, err = a.AcquireServiceLease(context.Background(), "rt-1", "", "s",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeRuntime, true, 1)
	if !errors.Is(err, secret.ErrServiceInvalid) {
		t.Fatalf("expected ErrServiceInvalid for empty plugin, got %v", err)
	}

	_, err = a.AcquireServiceLease(context.Background(), "rt-1", "p", "",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeRuntime, true, 1)
	if !errors.Is(err, secret.ErrServiceInvalid) {
		t.Fatalf("expected ErrServiceInvalid for empty service, got %v", err)
	}

	_, err = a.AcquireServiceLease(context.Background(), "rt-1", "p", "s",
		kernelsecret.SecretRef("invalid"), secret.PurposeRuntime, true, 1)
	if !errors.Is(err, secret.ErrSecretRefInvalid) {
		t.Fatalf("expected ErrSecretRefInvalid for invalid ref, got %v", err)
	}
}

func TestAcquireServiceLease_SecretNotFound_FailsStore(t *testing.T) {
	b := newFakeBroker() // no secret seeded
	id := newFakeIdentity()
	id.AddRuntime("rt-1", "p", "ext", "created")
	id.AddService("rt-1", "svc-1", "p", "ext", "running")
	g := &fakeGate{allow: true}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	_, err = a.AcquireServiceLease(context.Background(), "rt-1", "p", "svc-1",
		kernelsecret.SecretRef("secret://nonexistent/key"), secret.PurposeStartup, true, 1)
	if !errors.Is(err, secret.ErrSecretStoreFailure) {
		t.Fatalf("expected ErrSecretStoreFailure for missing secret, got %v", err)
	}
}

// ===== Active Leases Empty Case =====

func TestActiveLeases_EmptyWhenNoLeases(t *testing.T) {
	b := newFakeBroker()
	id := newFakeIdentity()
	g := &fakeGate{allow: true}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	if len(a.ActiveServiceLeases("rt-1", "svc-1")) != 0 {
		t.Error("expected no active service leases")
	}
	if len(a.ActiveRuntimeLeases("rt-1")) != 0 {
		t.Error("expected no active runtime leases")
	}
	if len(a.ActiveExtensionLeases("ext-1")) != 0 {
		t.Error("expected no active extension leases")
	}
}

func TestRevokeRuntimeLeases_EmptyRuntime(t *testing.T) {
	b := newFakeBroker()
	id := newFakeIdentity()
	g := &fakeGate{allow: true}
	a, err := secret.NewSecretLeaseAdapter(b, id, g)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	o := a.RevokeRuntimeLeases("", "test")
	if o.RevokedCount != 0 {
		t.Errorf("expected 0 revokes for empty runtime, got %d", o.RevokedCount)
	}
}

// ===== ClearServiceStopping =====

func TestClearServiceStopping_AllowsAcquireAfterClear(t *testing.T) {
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

	a.MarkServiceStopping("rt-1", "svc-1")
	a.ClearServiceStopping("rt-1", "svc-1")

	_, err = a.AcquireServiceLease(context.Background(), "rt-1", "p", "svc-1",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeStartup, true, 1)
	if err != nil {
		t.Fatalf("expected grant after clear, got err=%v", err)
	}
}

// ===== Reset =====

func TestReset_ClearsAllState(t *testing.T) {
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

	_, _ = a.AcquireServiceLease(context.Background(), "rt-1", "p", "svc-1",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeStartup, true, 1)
	a.Shutdown()
	a.Reset()

	_, err = a.AcquireServiceLease(context.Background(), "rt-1", "p", "svc-1",
		kernelsecret.SecretRef(refOpenAI), secret.PurposeStartup, true, 1)
	if err != nil {
		t.Fatalf("expected grant after reset, got err=%v", err)
	}
	if len(a.ActiveServiceLeases("rt-1", "svc-1")) != 1 {
		t.Error("expected exactly 1 active lease after reset + reacquire")
	}
}

// ===== Nil gate =====

func TestNilGate_ConstructorFails(t *testing.T) {
	b := newFakeBroker()
	b.SeedSecret(refOpenAI, "v")
	id := newFakeIdentity()
	id.AddRuntime("rt-1", "p", "ext", "created")
	id.AddService("rt-1", "svc-1", "p", "ext", "running")

	_, err := secret.NewSecretLeaseAdapter(b, id, nil)
	if err == nil {
		t.Fatal("expected constructor to fail with nil gate, got success")
	}
}
