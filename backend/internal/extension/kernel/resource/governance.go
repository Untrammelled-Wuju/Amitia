//go:build rsi_resource

package resource

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrLeaseInvalid   = errors.New("resource: invalid lease")
	ErrLeaseDuplicate = errors.New("resource: lease already adopted")
	ErrLeaseUnknown   = errors.New("resource: lease not adopted")
	ErrGovernorClosed = errors.New("resource: governor is closed")
)

type LeaseSpec struct {
	ID    string
	Kind  ManagedKind
	Owner ResourceOwner
	Close func(context.Context) error
	Stats func() map[string]int64
}

type Lease struct {
	ID        string
	Kind      ManagedKind
	Owner     ResourceOwner
	AdoptedAt time.Time
}

type governanceLease struct {
	lease Lease
	spec  LeaseSpec
}

type Governor struct {
	mu     sync.Mutex
	leases map[string]*governanceLease
	order  []string
	closed bool
	now    func() time.Time
}

func NewGovernor() *Governor {
	return NewGovernorWithClock(time.Now)
}

func NewGovernorWithClock(now func() time.Time) *Governor {
	if now == nil {
		now = time.Now
	}
	return &Governor{leases: map[string]*governanceLease{}, now: now}
}

func validManagedKind(kind ManagedKind) bool {
	for _, candidate := range []ManagedKind{ManagedEventSubscription, ManagedPriorityQueue, ManagedBackpressure, ManagedOutbox, ManagedResourceOwnership, ManagedTaskEntry} {
		if candidate == kind {
			return true
		}
	}
	return false
}

func (g *Governor) Adopt(ctx context.Context, spec LeaseSpec) (Lease, error) {
	if g == nil {
		return Lease{}, fmt.Errorf("%w: governor is nil", ErrLeaseInvalid)
	}
	if err := ctx.Err(); err != nil {
		return Lease{}, err
	}
	if spec.ID == "" {
		return Lease{}, fmt.Errorf("%w: id is required", ErrLeaseInvalid)
	}
	if !validManagedKind(spec.Kind) {
		return Lease{}, fmt.Errorf("%w: kind=%q", ErrLeaseInvalid, spec.Kind)
	}
	if !spec.Owner.OwnerType.IsValid() || spec.Owner.OwnerID == "" {
		return Lease{}, fmt.Errorf("%w: owner=%+v", ErrLeaseInvalid, spec.Owner)
	}
	if spec.Close == nil {
		return Lease{}, fmt.Errorf("%w: closer is required", ErrLeaseInvalid)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return Lease{}, ErrGovernorClosed
	}
	if _, exists := g.leases[spec.ID]; exists {
		return Lease{}, fmt.Errorf("%w: %s", ErrLeaseDuplicate, spec.ID)
	}
	lease := Lease{ID: spec.ID, Kind: spec.Kind, Owner: spec.Owner, AdoptedAt: g.now().UTC()}
	g.leases[spec.ID] = &governanceLease{lease: lease, spec: spec}
	g.order = append(g.order, spec.ID)
	return lease, nil
}

func (g *Governor) Lookup(id string) (Lease, bool) {
	if g == nil {
		return Lease{}, false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	entry, ok := g.leases[id]
	if !ok {
		return Lease{}, false
	}
	return entry.lease, true
}

func (g *Governor) Stats(id string) (map[string]int64, bool) {
	if g == nil {
		return nil, false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	entry, ok := g.leases[id]
	if !ok || entry.spec.Stats == nil {
		return nil, false
	}
	return entry.spec.Stats(), true
}

func (g *Governor) Release(ctx context.Context, id string) error {
	if g == nil {
		return fmt.Errorf("%w: governor is nil", ErrLeaseInvalid)
	}
	g.mu.Lock()
	entry, ok := g.leases[id]
	if !ok {
		g.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrLeaseUnknown, id)
	}
	delete(g.leases, id)
	for index, existing := range g.order {
		if existing == id {
			g.order = append(g.order[:index], g.order[index+1:]...)
			break
		}
	}
	g.mu.Unlock()
	return entry.spec.Close(ctx)
}

func (g *Governor) CloseAll(ctx context.Context) error {
	if g == nil {
		return fmt.Errorf("%w: governor is nil", ErrLeaseInvalid)
	}
	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		return ErrGovernorClosed
	}
	g.closed = true
	ids := append([]string{}, g.order...)
	entries := make([]*governanceLease, 0, len(ids))
	for _, id := range ids {
		if entry, ok := g.leases[id]; ok {
			entries = append(entries, entry)
		}
	}
	g.leases = map[string]*governanceLease{}
	g.order = nil
	g.mu.Unlock()
	var failures []error
	for index := len(entries) - 1; index >= 0; index-- {
		if err := entries[index].spec.Close(ctx); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", entries[index].spec.ID, err))
		}
	}
	return errors.Join(failures...)
}

func (g *Governor) Len() int {
	if g == nil {
		return 0
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.leases)
}

func (g *Governor) Closed() bool {
	if g == nil {
		return true
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.closed
}
