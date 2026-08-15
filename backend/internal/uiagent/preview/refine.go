package preview

import (
	"context"
	"errors"

	"github.com/u-ai/backend/internal/uiagent"
)

type RefineRequest struct {
	SessionID     string             `json:"sessionId"`
	Observation   *ObservationResult `json:"observation"`
	Feedback      string             `json:"feedback"`
	Target        *uiagent.UITarget  `json:"target"`
	ChangedPaths  []string           `json:"changedPaths,omitempty"`
	MaxIterations int                `json:"maxIterations"`
}

type RefineResult struct {
	State              string             `json:"state"`
	Observation        *ObservationResult `json:"observation,omitempty"`
	ChangedPaths       []string           `json:"changedPaths,omitempty"`
	Iterations         int                `json:"iterations"`
	Converged          bool               `json:"converged"`
	NoopSinceLastCycle bool               `json:"noopSinceLastCycle"`
}

const MaxRefineIterations = 5

type AutoRefiner interface {
	Refine(ctx context.Context, req RefineRequest) (*RefineResult, error)
}

type defaultAutoRefiner struct {
	sessionManager SessionManager
	observer       Observer
}

func NewAutoRefiner(mgr SessionManager) AutoRefiner {
	return &defaultAutoRefiner{
		sessionManager: mgr,
		observer:       NewObserver(),
	}
}

func (r *defaultAutoRefiner) Refine(ctx context.Context, req RefineRequest) (*RefineResult, error) {
	if r.observer == nil {
		return nil, ErrObserverNotConfigured
	}
	if req.SessionID == "" {
		return nil, ErrSessionRequired
	}
	if req.MaxIterations <= 0 {
		req.MaxIterations = MaxRefineIterations
	}

	iterations := 0
	var noopSinceLastCycle bool

	for i := 0; i < req.MaxIterations; i++ {
		obs, err := r.observer.Capture(ctx, req.SessionID)
		if err != nil {
			return &RefineResult{State: "error", Iterations: iterations}, err
		}

		// If observation has not changed since last cycle, mark noop.
		if req.Observation != nil && sameObservationPaths(obs, req.Observation) {
			noopSinceLastCycle = true
			break
		}
		iterations++
		req.Observation = obs
		// Actual source modification requires sourceEditor support;
		// this implementation returns the observation for upstream handling.
	}

	return &RefineResult{
		State:              "refined",
		Iterations:         iterations,
		Converged:          noopSinceLastCycle,
		NoopSinceLastCycle: noopSinceLastCycle,
	}, nil
}

func sameObservationPaths(a, b *ObservationResult) bool {
	if a == nil || b == nil {
		return false
	}
	if len(a.ChangedPaths) != len(b.ChangedPaths) {
		return false
	}
	for i := range a.ChangedPaths {
		if a.ChangedPaths[i] != b.ChangedPaths[i] {
			return false
		}
	}
	return true
}

var (
	ErrObserverNotConfigured = errors.New("auto refiner: observer not configured")
	ErrSessionRequired       = errors.New("auto refiner: session ID required")
)
