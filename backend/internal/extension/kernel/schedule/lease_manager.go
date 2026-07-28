package schedule

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type LeaseManager struct {
	store  ScheduleStore
	clock  Clock
	config ScheduleConfig

	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running bool
}

func NewLeaseManager(store ScheduleStore, clock Clock, config ScheduleConfig) *LeaseManager {
	if clock == nil {
		clock = NewRealClock()
	}
	return &LeaseManager{
		store:  store,
		clock:  clock,
		config: config,
	}
}

func (m *LeaseManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return fmt.Errorf("lease manager already running")
	}
	m.ctx, m.cancel = context.WithCancel(ctx)
	m.running = true
	m.wg.Add(1)
	go m.reclaimLoop()
	return nil
}

func (m *LeaseManager) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	if m.cancel != nil {
		m.cancel()
	}
	m.mu.Unlock()
	m.wg.Wait()
}

func (m *LeaseManager) AcquireLease(ctx context.Context, triggerID, owner string) (bool, error) {
	expiresAt := m.clock.Now().Add(m.config.LeaseDuration)
	return m.store.AcquireTriggerLease(ctx, triggerID, owner, expiresAt)
}

func (m *LeaseManager) ReleaseLease(ctx context.Context, triggerID string) error {
	return m.store.ReleaseTriggerLease(ctx, triggerID)
}

func (m *LeaseManager) reclaimLoop() {
	defer m.wg.Done()
	ticker := time.NewTicker(m.config.LeaseReclaimInterval)
	defer ticker.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.reclaimOnce()
		}
	}
}

func (m *LeaseManager) reclaimOnce() {
	now := m.clock.Now()
	count, err := m.store.ReclaimExpiredLeases(m.ctx, now)
	if err != nil {
		return
	}
	_ = count
}

func (m *LeaseManager) CreateLeaseRecord(ctx context.Context, triggerID, scheduleID, owner string) (*ScheduleLease, error) {
	now := m.clock.Now()
	lease := &ScheduleLease{
		LeaseID:    "lease-" + uuid.NewString(),
		TriggerID:  triggerID,
		ScheduleID: scheduleID,
		LeaseOwner: owner,
		AcquiredAt: now.UTC(),
		ExpiresAt:  now.Add(m.config.LeaseDuration).UTC(),
	}
	return lease, nil
}
