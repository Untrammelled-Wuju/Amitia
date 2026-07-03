package mindruntime

import (
	"context"
	"sync"
	"time"
)

type LifecyclePhase string

const (
	LifecyclePhaseInit         LifecyclePhase = "init"
	LifecyclePhaseRunning      LifecyclePhase = "running"
	LifecyclePhaseDraining     LifecyclePhase = "draining"
	LifecyclePhaseShuttingDown LifecyclePhase = "shutting_down"
	LifecyclePhaseTerminated   LifecyclePhase = "terminated"
)

type LifecycleState struct {
	Phase     LifecyclePhase `json:"phase"`
	StartedAt time.Time      `json:"startedAt"`
	StoppedAt time.Time      `json:"stoppedAt,omitempty"`
	Error     string         `json:"error,omitempty"`
}

type LifecycleComponent interface {
	Start(ctx context.Context) error
	Shutdown(ctx context.Context) error
	Drain(ctx context.Context) error
	Phase() LifecyclePhase
	State() LifecycleState
}

type DrainConfig struct {
	Timeout      time.Duration `json:"timeout"`
	PollInterval time.Duration `json:"pollInterval"`
	MaxInflight  int           `json:"maxInflight"`
}

func DefaultDrainConfig() DrainConfig {
	return DrainConfig{
		Timeout:      30 * time.Second,
		PollInterval: 500 * time.Millisecond,
		MaxInflight:  0,
	}
}

type ShutdownOrder struct {
	Components []LifecycleComponent `json:"components"`
	mu         sync.Mutex
}

func NewShutdownOrder(components []LifecycleComponent) *ShutdownOrder {
	return &ShutdownOrder{Components: components}
}

func (o *ShutdownOrder) Execute(ctx context.Context) []LifecycleState {
	results := make([]LifecycleState, 0, len(o.Components))
	for i := len(o.Components) - 1; i >= 0; i-- {
		c := o.Components[i]
		state := c.State()
		drainCtx, drainCancel := context.WithTimeout(ctx, DefaultDrainConfig().Timeout)
		if drainErr := c.Drain(drainCtx); drainErr != nil {
			state.Error = drainErr.Error()
		}
		drainCancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 15*time.Second)
		if shutdownErr := c.Shutdown(shutdownCtx); shutdownErr != nil {
			if state.Error != "" {
				state.Error += "; " + shutdownErr.Error()
			} else {
				state.Error = shutdownErr.Error()
			}
		}
		shutdownCancel()
		state.Phase = LifecyclePhaseTerminated
		state.StoppedAt = time.Now().UTC()
		results = append(results, state)
	}
	return results
}

type LifecycleDeadline struct {
	RequestID    string        `json:"requestId"`
	Timeout      time.Duration `json:"timeout"`
	CreatedAt    time.Time     `json:"createdAt"`
	PropagatedTo []string      `json:"propagatedTo,omitempty"`
	Expired      bool          `json:"expired"`
}

func NewLifecycleDeadline(requestID string, timeout time.Duration) LifecycleDeadline {
	now := time.Now().UTC()
	return LifecycleDeadline{
		RequestID:    requestID,
		Timeout:      timeout,
		CreatedAt:    now,
		PropagatedTo: make([]string, 0),
		Expired:      false,
	}
}

func (d *LifecycleDeadline) Remaining() time.Duration {
	elapsed := time.Since(d.CreatedAt)
	remaining := d.Timeout - elapsed
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (d *LifecycleDeadline) IsExpired() bool {
	if d.Expired {
		return true
	}
	if d.Remaining() <= 0 {
		d.Expired = true
		return true
	}
	return false
}

func (d *LifecycleDeadline) Propagate(target string) {
	if d.IsExpired() {
		return
	}
	d.PropagatedTo = append(d.PropagatedTo, target)
}

func (d *LifecycleDeadline) Context(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, d.Remaining())
}

type ShutdownSignal int

const (
	ShutdownSignalGraceful ShutdownSignal = iota
	ShutdownSignalForce
	ShutdownSignalDrain
)

func (s ShutdownSignal) String() string {
	switch s {
	case ShutdownSignalGraceful:
		return "graceful"
	case ShutdownSignalForce:
		return "force"
	case ShutdownSignalDrain:
		return "drain"
	default:
		return "unknown"
	}
}
