package runtimeprojection

import (
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type RuntimePlacement string

const (
	RuntimePlacementLocal  RuntimePlacement = "local"
	RuntimePlacementCloud  RuntimePlacement = "cloud"
	RuntimePlacementDevice RuntimePlacement = "device"
)

type RuntimeExtensionProjection struct {
	ExtensionID string `json:"extensionId"`
	ModuleID    string `json:"moduleId,omitempty"`
	Desired     bool   `json:"desired"`
	Observed    string `json:"observed"`
}

type RuntimeProjection struct {
	RuntimeID   runtimeidentity.RuntimeID       `json:"runtimeId"`
	SessionID   runtimeidentity.RuntimeSessionID `json:"sessionId"`
	Identity    runtimeidentity.Identity         `json:"identity"`
	Placement   RuntimePlacement                 `json:"placement"`
	Online      bool                             `json:"online"`
	Health      string                           `json:"health"`

	Capabilities       []capability.CapabilityID       `json:"capabilities,omitempty"`
	ProviderInstances  []capability.ProviderInstanceID `json:"providerInstances,omitempty"`
	ExtensionInstances []RuntimeExtensionProjection   `json:"extensionInstances,omitempty"`

	ConnectionGeneration int64     `json:"connectionGeneration"`
	UpdatedAt            time.Time `json:"updatedAt"`
}

func (p RuntimeProjection) HasCapability(capID capability.CapabilityID) bool {
	for _, c := range p.Capabilities {
		if c == capID {
			return true
		}
	}
	return false
}

func (p RuntimeProjection) HasProviderInstance(instID capability.ProviderInstanceID) bool {
	for _, pi := range p.ProviderInstances {
		if pi == instID {
			return true
		}
	}
	return false
}

func (p RuntimeProjection) ExtensionState(extensionID string) (desired bool, observed string, found bool) {
	for _, ext := range p.ExtensionInstances {
		if ext.ExtensionID == extensionID {
			return ext.Desired, ext.Observed, true
		}
	}
	return false, "", false
}
