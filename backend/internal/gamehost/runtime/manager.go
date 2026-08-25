package runtime

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/gamehost/contracts"
	"github.com/u-ai/backend/internal/gamehost/domain"
)

type RuntimeIDGenerator interface {
	NewRuntimeID() domain.RuntimeInstanceID
}

type uuidRuntimeIDGenerator struct{}

func (g uuidRuntimeIDGenerator) NewRuntimeID() domain.RuntimeInstanceID {
	return domain.RuntimeInstanceID("rt_" + uuid.NewString())
}

type Manager struct {
	mu       sync.RWMutex
	runtimes map[domain.RuntimeInstanceID]*domain.RuntimeInstance
	byPlugin map[domain.PluginID]map[domain.RuntimeInstanceID]struct{}

	idGenerator RuntimeIDGenerator
	clock       func() time.Time

	generations            map[domain.RuntimeInstanceID]int64
	lifecycleIntents       map[domain.RuntimeInstanceID]string
	emergencyLatches       map[domain.RuntimeInstanceID]bool
	emergencyLatchResolver func(domain.RuntimeInstanceID) bool
}

type ManagerOptions struct {
	IDGenerator RuntimeIDGenerator
	Clock       func() time.Time
}

func NewManager(opts ManagerOptions) *Manager {
	idGen := opts.IDGenerator
	if idGen == nil {
		idGen = uuidRuntimeIDGenerator{}
	}
	clock := opts.Clock
	if clock == nil {
		clock = time.Now
	}
	return &Manager{
		runtimes:         make(map[domain.RuntimeInstanceID]*domain.RuntimeInstance),
		byPlugin:         make(map[domain.PluginID]map[domain.RuntimeInstanceID]struct{}),
		idGenerator:      idGen,
		clock:            clock,
		generations:      make(map[domain.RuntimeInstanceID]int64),
		lifecycleIntents: make(map[domain.RuntimeInstanceID]string),
		emergencyLatches: make(map[domain.RuntimeInstanceID]bool),
	}
}

func (m *Manager) Create(ctx context.Context, pluginID domain.PluginID) (*domain.RuntimeInstance, error) {
	if pluginID == "" {
		return nil, &TopologyError{Code: ErrInvalidArgument, Message: "plugin id must not be empty"}
	}

	now := m.clock()
	id := m.idGenerator.NewRuntimeID()

	rt, err := domain.NewRuntimeInstance(id, pluginID, now)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.runtimes[id] = rt
	if m.byPlugin[pluginID] == nil {
		m.byPlugin[pluginID] = make(map[domain.RuntimeInstanceID]struct{})
	}
	m.byPlugin[pluginID][id] = struct{}{}

	return cloneRuntimeInstance(rt), nil
}

func (m *Manager) EnsurePrimaryRuntime(ctx context.Context, pluginID domain.PluginID) (*domain.RuntimeInstance, bool, error) {
	if pluginID == "" {
		return nil, false, &TopologyError{Code: ErrInvalidArgument, Message: "plugin id must not be empty"}
	}

	m.mu.RLock()
	if ids, ok := m.byPlugin[pluginID]; ok && len(ids) > 0 {
		var primaryID domain.RuntimeInstanceID
		for id := range ids {
			primaryID = id
			break
		}
		rt := m.runtimes[primaryID]
		m.mu.RUnlock()
		return cloneRuntimeInstance(rt), false, nil
	}
	m.mu.RUnlock()

	now := m.clock()
	id := m.idGenerator.NewRuntimeID()

	rt, err := domain.NewRuntimeInstance(id, pluginID, now)
	if err != nil {
		return nil, false, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if existingIDs, ok := m.byPlugin[pluginID]; ok && len(existingIDs) > 0 {
		for existingID := range existingIDs {
			return cloneRuntimeInstance(m.runtimes[existingID]), false, nil
		}
	}

	m.runtimes[id] = rt
	if m.byPlugin[pluginID] == nil {
		m.byPlugin[pluginID] = make(map[domain.RuntimeInstanceID]struct{})
	}
	m.byPlugin[pluginID][id] = struct{}{}

	return cloneRuntimeInstance(rt), true, nil
}

func (m *Manager) Get(ctx context.Context, runtimeID domain.RuntimeInstanceID) (*domain.RuntimeInstance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rt, ok := m.runtimes[runtimeID]
	if !ok {
		return nil, &TopologyError{Code: ErrNotFound, Message: "runtime not found: " + string(runtimeID)}
	}
	return cloneRuntimeInstance(rt), nil
}

func (m *Manager) List(ctx context.Context) ([]domain.RuntimeInstance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]domain.RuntimeInstance, 0, len(m.runtimes))
	for _, rt := range m.runtimes {
		result = append(result, *cloneRuntimeInstance(rt))
	}
	return result, nil
}

func (m *Manager) GetRuntime(runtimeID domain.RuntimeInstanceID) (*RuntimeInstanceRef, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rt, ok := m.runtimes[runtimeID]
	if !ok {
		return nil, &TopologyError{Code: ErrNotFound, Message: "runtime not found: " + string(runtimeID)}
	}
	return &RuntimeInstanceRef{
		ID:       rt.ID,
		PluginID: rt.PluginID,
		State:    rt.State,
	}, nil
}

func (m *Manager) UpdateRuntimeState(runtimeID domain.RuntimeInstanceID, next domain.RuntimeState, reason string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rt, ok := m.runtimes[runtimeID]
	if !ok {
		return &TopologyError{Code: ErrNotFound, Message: "runtime not found: " + string(runtimeID)}
	}

	return rt.Transition(next, reason, now)
}

func (m *Manager) ListRuntimes() []*RuntimeInstanceRef {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*RuntimeInstanceRef, 0, len(m.runtimes))
	for _, rt := range m.runtimes {
		result = append(result, &RuntimeInstanceRef{
			ID:       rt.ID,
			PluginID: rt.PluginID,
			State:    rt.State,
		})
	}
	return result
}

func (m *Manager) GetRuntimeState(runtimeID domain.RuntimeInstanceID) (domain.RuntimeState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rt, ok := m.runtimes[runtimeID]
	if !ok {
		return "", &TopologyError{Code: ErrNotFound, Message: "runtime not found: " + string(runtimeID)}
	}
	return rt.State, nil
}

func (m *Manager) UpdateRuntimeHealth(runtimeID domain.RuntimeInstanceID, health domain.HealthStatus, reason string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rt, ok := m.runtimes[runtimeID]
	if !ok {
		return &TopologyError{Code: ErrNotFound, Message: "runtime not found: " + string(runtimeID)}
	}

	hs := rt.Health
	hs.Status = health
	hs.Message = reason
	return rt.UpdateHealth(hs, now)
}

func (m *Manager) RemoveRuntime(runtimeID domain.RuntimeInstanceID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rt, ok := m.runtimes[runtimeID]
	if !ok {
		return &TopologyError{Code: ErrNotFound, Message: "runtime not found: " + string(runtimeID)}
	}

	pluginID := rt.PluginID
	delete(m.runtimes, runtimeID)
	if ids, ok := m.byPlugin[pluginID]; ok {
		delete(ids, runtimeID)
		if len(ids) == 0 {
			delete(m.byPlugin, pluginID)
		}
	}
	return nil
}

func (m *Manager) GetCurrentGeneration(runtimeID domain.RuntimeInstanceID) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.runtimes[runtimeID]; !ok {
		return 0, &TopologyError{Code: ErrNotFound, Message: "runtime not found: " + string(runtimeID)}
	}
	return m.generations[runtimeID], nil
}

func (m *Manager) AllocateGeneration(runtimeID domain.RuntimeInstanceID) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.runtimes[runtimeID]; !ok {
		return 0, &TopologyError{Code: ErrNotFound, Message: "runtime not found: " + string(runtimeID)}
	}
	m.generations[runtimeID]++
	return m.generations[runtimeID], nil
}

func (m *Manager) GetLifecycleIntent(runtimeID domain.RuntimeInstanceID) (string, error) {
	m.mu.RLock()
	if _, ok := m.runtimes[runtimeID]; !ok {
		m.mu.RUnlock()
		return "", &TopologyError{Code: ErrNotFound, Message: "runtime not found: " + string(runtimeID)}
	}
	intent := m.lifecycleIntents[runtimeID]
	resolver := m.emergencyLatchResolver
	m.mu.RUnlock()
	if intent != "" {
		return intent, nil
	}
	// Emergency intent is safety state, not an ephemeral lifecycle hint. When a
	// durable emergency latch survives a host restart, expose the matching
	// lifecycle intent as well so every recovery/control policy sees one
	// consistent state even if the runtime received a new instance ID.
	if resolver != nil && resolver(runtimeID) {
		return "emergency", nil
	}
	return "", nil
}

func (m *Manager) SetLifecycleIntent(runtimeID domain.RuntimeInstanceID, intent string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.runtimes[runtimeID]; !ok {
		return &TopologyError{Code: ErrNotFound, Message: "runtime not found: " + string(runtimeID)}
	}
	m.lifecycleIntents[runtimeID] = intent
	return nil
}

func (m *Manager) SetEmergencyLatchResolver(resolver func(domain.RuntimeInstanceID) bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.emergencyLatchResolver = resolver
}

func (m *Manager) IsEmergencyLatched(runtimeID domain.RuntimeInstanceID) bool {
	m.mu.RLock()
	latched := m.emergencyLatches[runtimeID]
	resolver := m.emergencyLatchResolver
	m.mu.RUnlock()
	if latched {
		return true
	}
	return resolver != nil && resolver(runtimeID)
}

func (m *Manager) SetEmergencyLatch(runtimeID domain.RuntimeInstanceID, latched bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.emergencyLatches[runtimeID] = latched
}

func cloneRuntimeInstance(rt *domain.RuntimeInstance) *domain.RuntimeInstance {
	if rt == nil {
		return nil
	}
	copy := &domain.RuntimeInstance{
		ID:          rt.ID,
		PluginID:    rt.PluginID,
		State:       rt.State,
		StateReason: rt.StateReason,
		Health:      rt.Health,
		CreatedAt:   rt.CreatedAt,
		UpdatedAt:   rt.UpdatedAt,
		Metadata:    make(map[string]string, len(rt.Metadata)),
	}
	if rt.StartedAt != nil {
		t := *rt.StartedAt
		copy.StartedAt = &t
	}
	if rt.StoppedAt != nil {
		t := *rt.StoppedAt
		copy.StoppedAt = &t
	}
	if rt.SuspendedAt != nil {
		t := *rt.SuspendedAt
		copy.SuspendedAt = &t
	}
	if rt.FailedAt != nil {
		t := *rt.FailedAt
		copy.FailedAt = &t
	}
	for k, v := range rt.Metadata {
		copy.Metadata[k] = v
	}
	return copy
}

var _ contracts.RuntimeManager = (*Manager)(nil)
var _ RuntimeManager = (*Manager)(nil)
var _ RuntimeHealthAccessor = (*Manager)(nil)
