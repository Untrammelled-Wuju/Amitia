package control

import (
	"github.com/u-ai/backend/internal/gamehost/domain"
)

type TransitionActor string

const (
	ActorUser   TransitionActor = "user"
	ActorPlugin TransitionActor = "plugin"
	ActorHost   TransitionActor = "host"
	ActorSystem TransitionActor = "system"
)

type TransitionReason string

const (
	ReasonUserRequest      TransitionReason = "user_request"
	ReasonPluginRequest    TransitionReason = "plugin_request"
	ReasonHostPolicy       TransitionReason = "host_policy"
	ReasonRuntimeLifecycle TransitionReason = "runtime_lifecycle"
	ReasonSystemRecovery   TransitionReason = "system_recovery"
	ReasonEmergency        TransitionReason = "emergency"
)

type TransitionRequest struct {
	Target        domain.ControlMode
	Actor         TransitionActor
	Reason        TransitionReason
	ExpectedEpoch uint64
	UseExpected   bool
}

var validControlTransitions = map[domain.ControlMode]map[domain.ControlMode]struct{}{
	domain.ControlModeObserveOnly: {
		domain.ControlModeAssist:        {},
		domain.ControlModeSharedControl: {},
		domain.ControlModePluginControl: {},
		domain.ControlModeUserControl:   {},
		domain.ControlModeSuspended:     {},
	},
	domain.ControlModeAssist: {
		domain.ControlModeObserveOnly:   {},
		domain.ControlModeSharedControl: {},
		domain.ControlModePluginControl: {},
		domain.ControlModeUserControl:   {},
		domain.ControlModeSuspended:     {},
	},
	domain.ControlModeSharedControl: {
		domain.ControlModeObserveOnly:   {},
		domain.ControlModeAssist:        {},
		domain.ControlModePluginControl: {},
		domain.ControlModeUserControl:   {},
		domain.ControlModeSuspended:     {},
	},
	domain.ControlModePluginControl: {
		domain.ControlModeObserveOnly:   {},
		domain.ControlModeAssist:        {},
		domain.ControlModeSharedControl: {},
		domain.ControlModeUserControl:   {},
		domain.ControlModeSuspended:     {},
	},
	domain.ControlModeUserControl: {
		domain.ControlModeObserveOnly:   {},
		domain.ControlModeAssist:        {},
		domain.ControlModeSharedControl: {},
		domain.ControlModePluginControl: {},
		domain.ControlModeSuspended:     {},
	},
	domain.ControlModeSuspended: {
		domain.ControlModeObserveOnly:   {},
		domain.ControlModeAssist:        {},
		domain.ControlModeSharedControl: {},
		domain.ControlModePluginControl: {},
		domain.ControlModeUserControl:   {},
	},
}

func CanTransition(from domain.ControlMode, to domain.ControlMode) bool {
	targets, ok := validControlTransitions[from]
	if !ok {
		return false
	}
	_, exists := targets[to]
	return exists
}

func IsValidControlMode(mode domain.ControlMode) bool {
	switch mode {
	case domain.ControlModeObserveOnly,
		domain.ControlModeAssist,
		domain.ControlModeSharedControl,
		domain.ControlModePluginControl,
		domain.ControlModeUserControl,
		domain.ControlModeSuspended:
		return true
	default:
		return false
	}
}
