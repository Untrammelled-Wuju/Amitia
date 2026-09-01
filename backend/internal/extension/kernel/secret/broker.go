package secret

import (
	"context"
	"sync"
	"time"
)

type Broker struct {
	store    Store
	leases   *LeaseStore
	redactor *Redactor
	now      func() time.Time

	validateSnapshot func(ctx context.Context, permissionSnapshotID, scopeSnapshotID string) error

	mu sync.Mutex
}

type BrokerConfig struct {
	Store             Store
	SnapshotValidator func(ctx context.Context, permissionSnapshotID, scopeSnapshotID string) error
}

func NewBroker(cfg BrokerConfig) (*Broker, error) {
	if cfg.Store == nil {
		return nil, ErrSecretStoreUnavailable
	}
	return &Broker{
		store:            cfg.Store,
		leases:           NewLeaseStore(),
		redactor:         NewRedactor(),
		now:              time.Now,
		validateSnapshot: cfg.SnapshotValidator,
	}, nil
}

func (b *Broker) Redactor() *Redactor {
	return b.redactor
}

// VerifyReference validates that ref is syntactically valid and resolves in the
// encrypted store without exposing the secret to callers. The temporary value is
// zeroed before returning.
func (b *Broker) VerifyReference(ctx context.Context, ref SecretRef) error {
	if b == nil || b.store == nil {
		return ErrSecretStoreUnavailable
	}
	if ref == "" || !ref.Valid() {
		return ErrSecretRefInvalid
	}
	value, err := b.store.Get(ctx, string(ref))
	if err != nil {
		return err
	}
	zeroizeBytes(value)
	return nil
}
func (b *Broker) Store(ctx context.Context, namespace string, value []byte) (SecretRef, error) {
	if b == nil || b.store == nil {
		return "", ErrSecretStoreUnavailable
	}
	if len(value) == 0 {
		return "", ErrSecretRefInvalid
	}
	raw, err := b.store.Put(ctx, namespace, value)
	if err != nil {
		return "", err
	}
	ref, err := ParseRef(raw)
	if err != nil {
		_ = b.store.Delete(ctx, raw)
		return "", err
	}
	b.redactor.Add(value)
	return ref.Canonical(), nil
}

func (b *Broker) Issue(ctx context.Context, req LeaseRequest) (Lease, error) {
	if b.store == nil {
		return Lease{}, ErrSecretStoreUnavailable
	}
	if req.Ref == "" {
		return Lease{}, ErrSecretRefInvalid
	}
	if !req.Ref.Valid() {
		return Lease{}, ErrSecretRefInvalid
	}
	if req.Purpose == "" {
		return Lease{}, ErrSecretRefInvalid
	}
	if req.RuntimeInstanceID == "" {
		return Lease{}, ErrSecretRefInvalid
	}

	if b.validateSnapshot != nil && req.PermissionSnapshotID != "" && req.ScopeSnapshotID != "" {
		if err := b.validateSnapshot(ctx, req.PermissionSnapshotID, req.ScopeSnapshotID); err != nil {
			return Lease{}, err
		}
	}

	id, err := generateLeaseID()
	if err != nil {
		return Lease{}, err
	}

	now := b.now()
	expiresAt := now.Add(5 * time.Minute)
	if req.TTL > 0 {
		candidate := now.Add(req.TTL)
		if candidate.Before(expiresAt) {
			expiresAt = candidate
		}
	}

	maxUses := req.MaxUses
	if maxUses <= 0 {
		maxUses = 1
	}

	descriptor := Lease{
		ID:                   id,
		Ref:                  req.Ref,
		Purpose:              req.Purpose,
		InvocationID:         req.InvocationID,
		RuntimeInstanceID:    req.RuntimeInstanceID,
		UserID:               req.UserID,
		CharacterID:          req.CharacterID,
		ConversationID:       req.ConversationID,
		ExtensionID:          req.ExtensionID,
		ModuleID:             req.ModuleID,
		Generation:           req.Generation,
		PermissionSnapshotID: req.PermissionSnapshotID,
		ScopeSnapshotID:      req.ScopeSnapshotID,
		IssuedAt:             now,
		ExpiresAt:            expiresAt,
		MaxUses:              maxUses,
	}

	rawValue, err := b.store.Get(ctx, string(req.Ref))
	if err != nil {
		return Lease{}, err
	}
	valueCopy := make([]byte, len(rawValue))
	copy(valueCopy, rawValue)
	zeroizeBytes(rawValue)

	st := &leaseState{
		descriptor: descriptor,
		value:      valueCopy,
	}

	b.leases.Put(id, st)
	b.redactor.Add(valueCopy)

	return descriptor.Clone(), nil
}

func (b *Broker) Consume(ctx context.Context, leaseID LeaseID, use LeaseUseContext) ([]byte, error) {
	if b.store == nil {
		return nil, ErrSecretStoreUnavailable
	}

	res, ok := b.leases.Consume(leaseID)
	if !ok {
		return nil, ErrSecretNotFound
	}
	if res.revoked {
		return nil, ErrSecretLeaseRevoked
	}
	if res.expired {
		return nil, ErrSecretLeaseExpired
	}
	if res.exhausted && res.value == nil {
		return nil, ErrSecretLeaseExhausted
	}

	lease, _ := b.leases.Get(leaseID)
	if !matchLeaseCaller(&lease, &use) {
		return nil, ErrSecretLeaseScopeMismatch
	}

	if res.exhausted {
		b.RevokeLease(leaseID)
	}

	return res.value, nil
}

func (b *Broker) WithSecret(ctx context.Context, leaseID LeaseID, use LeaseUseContext, fn func([]byte) error) error {
	value, err := b.Consume(ctx, leaseID, use)
	if err != nil {
		return err
	}
	defer zeroizeBytes(value)
	return fn(value)
}

func (b *Broker) RevokeLease(leaseID LeaseID) error {
	b.leases.MarkRevoked(leaseID)
	return nil
}

func (b *Broker) RevokeByInvocation(invocationID string) int {
	if invocationID == "" {
		return 0
	}
	affected := b.leases.ListByInvocation(invocationID)
	for _, l := range affected {
		_ = b.RevokeLease(l.ID)
	}
	return len(affected)
}

func (b *Broker) RevokeByRuntimeInstance(instanceID string) int {
	if instanceID == "" {
		return 0
	}
	affected := b.leases.ListByRuntimeInstance(instanceID)
	for _, l := range affected {
		_ = b.RevokeLease(l.ID)
	}
	return len(affected)
}

func (b *Broker) RevokeByExtensionGeneration(extensionID string, generation int64) int {
	if extensionID == "" {
		return 0
	}
	affected := b.leases.ListByExtensionGeneration(extensionID, generation)
	for _, l := range affected {
		_ = b.RevokeLease(l.ID)
	}
	return len(affected)
}

func (b *Broker) GetLease(leaseID LeaseID) (Lease, bool) {
	return b.leases.Get(leaseID)
}

func (b *Broker) ListLeases() []Lease {
	return b.leases.List()
}

func (b *Broker) ListLeasesByInvocation(invocationID string) []Lease {
	return b.leases.ListByInvocation(invocationID)
}

func (b *Broker) CleanupExpired() int {
	return b.leases.CleanupExpired()
}

func (b *Broker) RevokeAll() int {
	all := b.leases.List()
	for _, l := range all {
		_ = b.RevokeLease(l.ID)
	}
	return len(all)
}

func matchLeaseCaller(descriptor *Lease, use *LeaseUseContext) bool {
	if use.InvocationID != "" && descriptor.InvocationID != "" && use.InvocationID != descriptor.InvocationID {
		return false
	}
	if use.RuntimeInstanceID != "" && descriptor.RuntimeInstanceID != "" && use.RuntimeInstanceID != descriptor.RuntimeInstanceID {
		return false
	}
	if use.ExtensionID != "" && descriptor.ExtensionID != "" && use.ExtensionID != descriptor.ExtensionID {
		return false
	}
	if use.ModuleID != "" && descriptor.ModuleID != "" && use.ModuleID != descriptor.ModuleID {
		return false
	}
	if use.Generation != 0 && descriptor.Generation != 0 && use.Generation != descriptor.Generation {
		return false
	}
	return true
}

func zeroizeBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
