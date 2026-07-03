package mindruntime

import (
	"context"
	"sort"
	"sync"
	"time"
)

type StartupPhase string

const (
	StartupPhaseDatabase StartupPhase = "database"
	StartupPhaseConfig   StartupPhase = "config"
	StartupPhaseModels   StartupPhase = "models"
	StartupPhaseNetwork  StartupPhase = "network"
	StartupPhaseRuntime  StartupPhase = "runtime"
	StartupPhaseReady    StartupPhase = "ready"
)

var DefaultStartupOrder = []StartupPhase{
	StartupPhaseDatabase,
	StartupPhaseConfig,
	StartupPhaseModels,
	StartupPhaseNetwork,
	StartupPhaseRuntime,
	StartupPhaseReady,
}

var startupPhasePriority = map[StartupPhase]int{
	StartupPhaseDatabase: 1,
	StartupPhaseConfig:   2,
	StartupPhaseModels:   3,
	StartupPhaseNetwork:  4,
	StartupPhaseRuntime:  5,
	StartupPhaseReady:    6,
}

type StartupStatus string

const (
	StartupStatusPending    StartupStatus = "pending"
	StartupStatusInProgress StartupStatus = "in_progress"
	StartupStatusComplete   StartupStatus = "complete"
	StartupStatusFailed     StartupStatus = "failed"
	StartupStatusSkipped    StartupStatus = "skipped"
)

type PhaseResult struct {
	Phase     StartupPhase     `json:"phase"`
	Status    StartupStatus    `json:"status"`
	StartedAt time.Time        `json:"startedAt"`
	EndedAt   time.Time        `json:"endedAt,omitempty"`
	Duration  time.Duration    `json:"duration"`
	Error     string           `json:"error,omitempty"`
	Checks    []ComponentCheck `json:"checks,omitempty"`
}

type StartupComponent interface {
	PhaseName() StartupPhase
	Startup(ctx context.Context) error
	HealthCheck() HealthCheckResult
}

type StartupSequence struct {
	Phases map[StartupPhase][]StartupComponent
	mu     sync.RWMutex
}

func NewStartupSequence() *StartupSequence {
	return &StartupSequence{
		Phases: make(map[StartupPhase][]StartupComponent),
	}
}

func (s *StartupSequence) Register(component StartupComponent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	phase := component.PhaseName()
	s.Phases[phase] = append(s.Phases[phase], component)
}

func (s *StartupSequence) Execute(ctx context.Context) []PhaseResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	order := make([]StartupPhase, 0, len(s.Phases))
	for phase := range s.Phases {
		order = append(order, phase)
	}
	sort.SliceStable(order, func(i, j int) bool {
		return startupPhasePriority[order[i]] < startupPhasePriority[order[j]]
	})
	results := make([]PhaseResult, 0, len(order))
	for _, phase := range order {
		components := s.Phases[phase]
		result := PhaseResult{
			Phase:     phase,
			Status:    StartupStatusPending,
			StartedAt: time.Now().UTC(),
		}
		allPassed := true
		for _, comp := range components {
			result.Status = StartupStatusInProgress
			if err := comp.Startup(ctx); err != nil {
				allPassed = false
				result.Status = StartupStatusFailed
				result.Error = err.Error()
				break
			}
			hc := comp.HealthCheck()
			result.Checks = append(result.Checks, hc.Checks...)
			if !hc.Healthy {
				allPassed = false
				result.Status = StartupStatusFailed
				if result.Error == "" {
					result.Error = hc.Summary
				}
				break
			}
		}
		if allPassed && result.Status != StartupStatusFailed {
			result.Status = StartupStatusComplete
		}
		result.EndedAt = time.Now().UTC()
		result.Duration = result.EndedAt.Sub(result.StartedAt)
		results = append(results, result)
		if result.Status == StartupStatusFailed {
			for _, remaining := range order[len(results):] {
				results = append(results, PhaseResult{
					Phase:     remaining,
					Status:    StartupStatusSkipped,
					StartedAt: result.EndedAt,
					EndedAt:   result.EndedAt,
				})
			}
			break
		}
	}
	return results
}

func (s *StartupSequence) AllReady(results []PhaseResult) bool {
	for _, r := range results {
		if r.Phase == StartupPhaseReady && r.Status == StartupStatusComplete {
			return true
		}
	}
	return false
}

type ReadyGate struct {
	conditions map[string]bool
	mu         sync.RWMutex
	readyCh    chan struct{}
	once       sync.Once
}

func NewReadyGate(dependencies []string) *ReadyGate {
	gate := &ReadyGate{
		conditions: make(map[string]bool),
		readyCh:    make(chan struct{}),
	}
	for _, dep := range dependencies {
		gate.conditions[dep] = false
	}
	return gate
}

func (g *ReadyGate) SignalReady(dependency string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.conditions[dependency]; ok {
		g.conditions[dependency] = true
	}
	allReady := true
	for _, ready := range g.conditions {
		if !ready {
			allReady = false
			break
		}
	}
	if allReady {
		g.once.Do(func() {
			close(g.readyCh)
		})
	}
}

func (g *ReadyGate) Wait(ctx context.Context) error {
	select {
	case <-g.readyCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *ReadyGate) IsReady() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	for _, ready := range g.conditions {
		if !ready {
			return false
		}
	}
	return true
}

type ShutdownSequence struct {
	Phases map[StartupPhase][]StartupComponent
	mu     sync.RWMutex
}

func NewShutdownSequence() *ShutdownSequence {
	return &ShutdownSequence{
		Phases: make(map[StartupPhase][]StartupComponent),
	}
}

func (s *ShutdownSequence) Register(component StartupComponent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	phase := component.PhaseName()
	s.Phases[phase] = append(s.Phases[phase], component)
}

func (s *ShutdownSequence) Execute(ctx context.Context) []PhaseResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	order := make([]StartupPhase, 0, len(s.Phases))
	for phase := range s.Phases {
		order = append(order, phase)
	}
	sort.SliceStable(order, func(i, j int) bool {
		return startupPhasePriority[order[i]] > startupPhasePriority[order[j]]
	})
	results := make([]PhaseResult, 0, len(order))
	for _, phase := range order {
		components := s.Phases[phase]
		result := PhaseResult{
			Phase:     phase,
			Status:    StartupStatusInProgress,
			StartedAt: time.Now().UTC(),
		}
		allPassed := true
		for _, comp := range components {
			hc := comp.HealthCheck()
			if !hc.Healthy {
				allPassed = false
				result.Status = StartupStatusFailed
				result.Error = hc.Summary
				break
			}
		}
		if allPassed {
			result.Status = StartupStatusComplete
		}
		result.EndedAt = time.Now().UTC()
		result.Duration = result.EndedAt.Sub(result.StartedAt)
		results = append(results, result)
	}
	return results
}

func StartupPhasePriority(phase StartupPhase) int {
	p, ok := startupPhasePriority[phase]
	if !ok {
		return 99
	}
	return p
}
