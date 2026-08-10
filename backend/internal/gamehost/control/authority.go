package control

import (
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

const (
	DefaultInitialEpoch uint64 = 1
)

type ControlAuthorityState struct {
	RuntimeID domain.RuntimeInstanceID
	PluginID  domain.PluginID
	Mode      domain.ControlMode
	Epoch     uint64

	UpdatedAt time.Time

	LastTransitionReason TransitionReason
	LastTransitionActor  TransitionActor
}

type ControlAuthoritySnapshot struct {
	RuntimeID domain.RuntimeInstanceID
	PluginID  domain.PluginID
	Mode      domain.ControlMode
	Epoch     uint64

	UpdatedAt time.Time

	LastTransitionReason TransitionReason
	LastTransitionActor  TransitionActor
}

func (s *ControlAuthorityState) Snapshot() ControlAuthoritySnapshot {
	return ControlAuthoritySnapshot{
		RuntimeID:            s.RuntimeID,
		PluginID:             s.PluginID,
		Mode:                 s.Mode,
		Epoch:                s.Epoch,
		UpdatedAt:            s.UpdatedAt,
		LastTransitionReason: s.LastTransitionReason,
		LastTransitionActor:  s.LastTransitionActor,
	}
}

type AuthorityToken struct {
	RuntimeID domain.RuntimeInstanceID
	Epoch     uint64
}

func (s *ControlAuthoritySnapshot) Token() AuthorityToken {
	return AuthorityToken{
		RuntimeID: s.RuntimeID,
		Epoch:     s.Epoch,
	}
}

func (s *ControlAuthoritySnapshot) IsStale(token AuthorityToken) bool {
	if s.RuntimeID != token.RuntimeID {
		return true
	}
	return s.Epoch != token.Epoch
}

func NewControlAuthorityState(runtimeID domain.RuntimeInstanceID, pluginID domain.PluginID, now time.Time) *ControlAuthorityState {
	return &ControlAuthorityState{
		RuntimeID:            runtimeID,
		PluginID:             pluginID,
		Mode:                 domain.ControlModeObserveOnly,
		Epoch:                DefaultInitialEpoch,
		UpdatedAt:            now,
		LastTransitionReason: "",
		LastTransitionActor:  "",
	}
}
