package preview

import (
	"context"
	"errors"
	"fmt"

	"github.com/u-ai/backend/internal/uiagent"
)

type RefineRequest struct {
	SessionID     string             `json:"sessionId"`
	Observation   *ObservationResult `json:"observation"`
	Feedback      string             `json:"feedback"`
	Target        *uiagent.UITarget  `json:"target"`
	ChangedPaths  []string           `json:"changedPaths,omitempty"`
	MaxIterations int                `json:"maxIterations"`
	PreviousTxID  string             `json:"previousTxId,omitempty"`
	Revision      string             `json:"revision,omitempty"`
}

type RefineResult struct {
	State              string             `json:"state"`
	Observation        *ObservationResult `json:"observation,omitempty"`
	ChangedPaths       []string           `json:"changedPaths,omitempty"`
	Iterations         int                `json:"iterations"`
	Converged          bool               `json:"converged"`
	NoopSinceLastCycle bool               `json:"noopSinceLastCycle"`
	Revision           string             `json:"revision,omitempty"`
	TransactionID      string             `json:"transactionId,omitempty"`
	RollbackToken      string             `json:"rollbackToken,omitempty"`
}

const MaxRefineIterations = 2

type AutoRefiner interface {
	Refine(ctx context.Context, req RefineRequest) (*RefineResult, error)
}

type defaultAutoRefiner struct {
	sessionManager SessionManager
	observer       Observer
}

func NewAutoRefiner(mgr SessionManager, obs Observer) AutoRefiner {
	return &defaultAutoRefiner{
		sessionManager: mgr,
		observer:       obs,
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
	if req.MaxIterations > MaxRefineIterations {
		req.MaxIterations = MaxRefineIterations
	}

	prevObservation := req.Observation
	var lastObservation *ObservationResult

	for i := 0; i < req.MaxIterations; i++ {
		obs, err := r.observer.Capture(ctx, req.SessionID)
		if err != nil {
			return &RefineResult{State: "error", Iterations: i}, err
		}

		if prevObservation != nil && sameObservationErrors(obs, prevObservation) {
			return &RefineResult{
				State:              "converged",
				Observation:        obs,
				Iterations:         i,
				Converged:          true,
				NoopSinceLastCycle: true,
				RollbackToken:      generateRollbackToken(req.SessionID, i),
			}, nil
		}

		lastObservation = obs
		prevObservation = obs

		if len(obs.Errors) == 0 {
			return &RefineResult{
				State:         "refined",
				Observation:   obs,
				Iterations:    i + 1,
				Converged:     true,
				RollbackToken: generateRollbackToken(req.SessionID, i+1),
			}, nil
		}
	}

	if lastObservation == nil {
		return &RefineResult{State: "no_change", Iterations: 0}, nil
	}

	return &RefineResult{
		State:         "needs_manual",
		Observation:   lastObservation,
		Iterations:    req.MaxIterations,
		Converged:     false,
		ChangedPaths:  lastObservation.ChangedPaths,
		RollbackToken: generateRollbackToken(req.SessionID, req.MaxIterations),
	}, nil
}

func sameObservationErrors(a, b *ObservationResult) bool {
	if a == nil || b == nil {
		return false
	}
	if len(a.Errors) != len(b.Errors) {
		return false
	}
	for i := range a.Errors {
		if a.Errors[i] != b.Errors[i] {
			return false
		}
	}
	return true
}

func generateRollbackToken(sessionID string, iteration int) string {
	return fmt.Sprintf("%s_r%d", sessionID, iteration)
}

var (
	ErrObserverNotConfigured = errors.New("auto refiner: observer not configured")
	ErrSessionRequired       = errors.New("auto refiner: session ID required")
)
