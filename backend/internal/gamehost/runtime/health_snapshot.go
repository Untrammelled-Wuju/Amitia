package runtime

import (
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type ServiceHealthSnapshot struct {
	PluginID  domain.PluginID
	RuntimeID domain.RuntimeInstanceID
	ServiceID domain.ServiceID

	Health domain.HealthStatus

	LastChangedAt time.Time
	LastObservedAt time.Time
	Reason        string
}

func (s ServiceHealthSnapshot) Clone() ServiceHealthSnapshot {
	return ServiceHealthSnapshot{
		PluginID:       s.PluginID,
		RuntimeID:      s.RuntimeID,
		ServiceID:      s.ServiceID,
		Health:         s.Health,
		LastChangedAt:  s.LastChangedAt,
		LastObservedAt: s.LastObservedAt,
		Reason:         s.Reason,
	}
}

type QuarantineSnapshot struct {
	RuntimeID  domain.RuntimeInstanceID
	ServiceID  domain.ServiceID

	Quarantined bool
	Since       *time.Time
	Reason      string
}

func (s QuarantineSnapshot) Clone() QuarantineSnapshot {
	clone := QuarantineSnapshot{
		RuntimeID:  s.RuntimeID,
		ServiceID:  s.ServiceID,
		Quarantined: s.Quarantined,
		Reason:     s.Reason,
	}
	if s.Since != nil {
		t := *s.Since
		clone.Since = &t
	}
	return clone
}

type RestartSnapshot struct {
	RuntimeID  domain.RuntimeInstanceID
	ServiceID  domain.ServiceID

	Generation    int64
	RestartCount  int
	LastRestartAt *time.Time
	Restarting    bool
	Exhausted     bool
	Reason        string
}

func (s RestartSnapshot) Clone() RestartSnapshot {
	clone := RestartSnapshot{
		RuntimeID:    s.RuntimeID,
		ServiceID:    s.ServiceID,
		Generation:   s.Generation,
		RestartCount: s.RestartCount,
		Restarting:   s.Restarting,
		Exhausted:    s.Exhausted,
		Reason:       s.Reason,
	}
	if s.LastRestartAt != nil {
		t := *s.LastRestartAt
		clone.LastRestartAt = &t
	}
	return clone
}

type RuntimeHealthResult struct {
	RuntimeID domain.RuntimeInstanceID

	Health domain.HealthStatus

	ServiceHealths  []ServiceHealthSnapshot
	RestartServices []RestartSnapshot
	Quarantines     []QuarantineSnapshot

	AggregatedAt time.Time
	Reason       string
}

func (r RuntimeHealthResult) Clone() RuntimeHealthResult {
	clone := RuntimeHealthResult{
		RuntimeID:      r.RuntimeID,
		Health:         r.Health,
		AggregatedAt:   r.AggregatedAt,
		Reason:         r.Reason,
	}
	if r.ServiceHealths != nil {
		clone.ServiceHealths = make([]ServiceHealthSnapshot, len(r.ServiceHealths))
		for i, s := range r.ServiceHealths {
			clone.ServiceHealths[i] = s.Clone()
		}
	}
	if r.RestartServices != nil {
		clone.RestartServices = make([]RestartSnapshot, len(r.RestartServices))
		for i, s := range r.RestartServices {
			clone.RestartServices[i] = s.Clone()
		}
	}
	if r.Quarantines != nil {
		clone.Quarantines = make([]QuarantineSnapshot, len(r.Quarantines))
		for i, s := range r.Quarantines {
			clone.Quarantines[i] = s.Clone()
		}
	}
	return clone
}

type ServiceHealthUpdate struct {
	ServiceID domain.ServiceID
	Health    domain.HealthStatus
	Reason    string
	UpdatedAt time.Time
}

type QuarantineUpdate struct {
	ServiceID   domain.ServiceID
	Quarantined bool
	Reason      string
	Since       time.Time
}

type RestartUpdate struct {
	ServiceID    domain.ServiceID
	Generation   int64
	Restarting   bool
	Exhausted    bool
	RestartCount int
	Reason       string
	UpdatedAt    time.Time
}
