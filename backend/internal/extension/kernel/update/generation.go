package update

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

type GenerationState string

const (
	GenerationStatePreparing  GenerationState = "preparing"
	GenerationStateValidated  GenerationState = "validated"
	GenerationStateRuntimeReady GenerationState = "runtime_ready"
	GenerationStateActive     GenerationState = "active"
	GenerationStateDraining   GenerationState = "draining"
	GenerationStateStopped    GenerationState = "stopped"
	GenerationStateFailed     GenerationState = "failed"
)

type Generation struct {
	GenerationID    string
	ExtensionID     string
	Version         string
	Generation      int
	State           GenerationState
	DefinitionHash  string
	RuntimeHash     string
	DependencyHash  string
	CreatedAt       time.Time
	ActivatedAt     *time.Time
	StoppedAt       *time.Time
	Invocations     int
}

type GenerationManager struct {
	mu          sync.RWMutex
	generations map[string][]Generation
	active      map[string]string
}

func NewGenerationManager() *GenerationManager {
	return &GenerationManager{
		generations: make(map[string][]Generation),
		active:      make(map[string]string),
	}
}

func (m *GenerationManager) Prepare(ctx context.Context, extensionID, version, definitionHash string) Generation {
	m.mu.Lock()
	defer m.mu.Unlock()
	gens := m.generations[extensionID]
	genNum := len(gens) + 1
	gen := Generation{
		GenerationID:   fmt.Sprintf("gen-%s-%d", extensionID, genNum),
		ExtensionID:    extensionID,
		Version:        version,
		Generation:     genNum,
		State:          GenerationStatePreparing,
		DefinitionHash: definitionHash,
		CreatedAt:      time.Now().UTC(),
	}
	m.generations[extensionID] = append(gens, gen)
	return gen
}

func (m *GenerationManager) Transition(ctx context.Context, extensionID, generationID string, target GenerationState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	gens := m.generations[extensionID]
	for i := range gens {
		if gens[i].GenerationID == generationID {
			if !isValidTransition(gens[i].State, target) {
				return fmt.Errorf("update: invalid transition %s -> %s", gens[i].State, target)
			}
			gens[i].State = target
			if target == GenerationStateActive {
				now := time.Now().UTC()
				gens[i].ActivatedAt = &now
				m.active[extensionID] = generationID
				for j := range gens {
					if i != j && gens[j].State == GenerationStateActive {
						gens[j].State = GenerationStateDraining
					}
				}
			}
			if target == GenerationStateStopped {
				now := time.Now().UTC()
				gens[i].StoppedAt = &now
			}
			m.generations[extensionID] = gens
			return nil
		}
	}
	return fmt.Errorf("update: generation %s not found", generationID)
}

func isValidTransition(from, to GenerationState) bool {
	transitions := map[GenerationState][]GenerationState{
		GenerationStatePreparing:   {GenerationStateValidated, GenerationStateFailed},
		GenerationStateValidated:   {GenerationStateRuntimeReady, GenerationStateFailed},
		GenerationStateRuntimeReady: {GenerationStateActive, GenerationStateFailed},
		GenerationStateActive:      {GenerationStateDraining},
		GenerationStateDraining:    {GenerationStateStopped},
		GenerationStateStopped:     {},
		GenerationStateFailed:      {},
	}
	allowed, ok := transitions[from]
	if !ok {
		return false
	}
	for _, t := range allowed {
		if t == to {
			return true
		}
	}
	return false
}

func (m *GenerationManager) Get(ctx context.Context, extensionID, generationID string) (*Generation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	gens := m.generations[extensionID]
	for _, g := range gens {
		if g.GenerationID == generationID {
			result := g
			return &result, nil
		}
	}
	return nil, fmt.Errorf("update: generation %s not found", generationID)
}

func (m *GenerationManager) Active(ctx context.Context, extensionID string) *Generation {
	m.mu.RLock()
	defer m.mu.RUnlock()
	activeID, ok := m.active[extensionID]
	if !ok {
		return nil
	}
	gens := m.generations[extensionID]
	for _, g := range gens {
		if g.GenerationID == activeID {
			result := g
			return &result
		}
	}
	return nil
}

func (m *GenerationManager) List(ctx context.Context, extensionID string) []Generation {
	m.mu.RLock()
	defer m.mu.RUnlock()
	gens := m.generations[extensionID]
	out := make([]Generation, len(gens))
	copy(out, gens)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Generation < out[j].Generation
	})
	return out
}

func (m *GenerationManager) Drain(ctx context.Context, extensionID string) error {
	active := m.Active(ctx, extensionID)
	if active == nil {
		return errors.New("update: no active generation")
	}
	return m.Transition(ctx, extensionID, active.GenerationID, GenerationStateDraining)
}

func (m *GenerationManager) IncrementInvocation(ctx context.Context, extensionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	activeID, ok := m.active[extensionID]
	if !ok {
		return
	}
	gens := m.generations[extensionID]
	for i := range gens {
		if gens[i].GenerationID == activeID {
			gens[i].Invocations++
			m.generations[extensionID] = gens
			return
		}
	}
}

func (m *GenerationManager) CanRollback(ctx context.Context, extensionID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	gens := m.generations[extensionID]
	activeCount := 0
	stoppedCount := 0
	for _, g := range gens {
		if g.State == GenerationStateActive || g.State == GenerationStateDraining {
			activeCount++
		}
		if g.State == GenerationStateStopped {
			stoppedCount++
		}
	}
	return activeCount >= 1 && stoppedCount >= 1
}

func (m *GenerationManager) RollbackTarget(ctx context.Context, extensionID string) (*Generation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	gens := m.generations[extensionID]
	var target *Generation
	for i := range gens {
		if gens[i].State == GenerationStateStopped {
			if target == nil || gens[i].Generation > target.Generation {
				g := gens[i]
				target = &g
			}
		}
	}
	if target == nil {
		return nil, errors.New("update: no rollback target available")
	}
	return target, nil
}

func (m *GenerationManager) RemoveGeneration(ctx context.Context, extensionID, generationID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	gens := m.generations[extensionID]
	for i, g := range gens {
		if g.GenerationID == generationID {
			if g.State == GenerationStateActive {
				return errors.New("update: cannot remove active generation")
			}
			m.generations[extensionID] = append(gens[:i], gens[i+1:]...)
			if m.active[extensionID] == generationID {
				delete(m.active, extensionID)
			}
			return nil
		}
	}
	return fmt.Errorf("update: generation %s not found", generationID)
}
