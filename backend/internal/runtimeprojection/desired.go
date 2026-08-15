package runtimeprojection

import (
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type ExtensionDesiredState struct {
	ExtensionID string `json:"extensionId"`
	ModuleID    string `json:"moduleId,omitempty"`
	Enabled     bool   `json:"enabled"`
}

type ExtensionObservedState struct {
	ExtensionID string `json:"extensionId"`
	ModuleID    string `json:"moduleId,omitempty"`
	RuntimeID   string `json:"runtimeId,omitempty"`
	Ready       bool   `json:"ready"`
	Health      string `json:"health"`
}

type ProviderInstanceObservedState struct {
	ProviderInstanceID capability.ProviderInstanceID `json:"providerInstanceId"`
	RuntimeID          string                        `json:"runtimeId,omitempty"`
	Available          bool                          `json:"available"`
	Health             string                        `json:"health"`
	RuntimeSessionID   string                        `json:"runtimeSessionId,omitempty"`
}

func DeriveExtensionObserved(desired ExtensionDesiredState, observed ExtensionObservedState) string {
	if !desired.Enabled {
		return "disabled"
	}
	if observed.Ready {
		return "ready"
	}
	return "pending"
}

func ReconcileProviderAvailability(
	desired capability.ProviderPlacement,
	observed ProviderInstanceObservedState,
) ProviderInstanceObservedState {
	if observed.RuntimeID == "" {
		observed.Available = false
		observed.Health = "unavailable"
	}
	return observed
}
