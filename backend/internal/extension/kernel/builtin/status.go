package builtin

import "github.com/u-ai/backend/internal/extension/kernel/domain"

type ExtensionStatus struct {
	ExtensionID    domain.ExtensionID `json:"extensionId"`
	Builtin        bool               `json:"builtin"`
	Required       bool               `json:"required"`
	DisableAllowed bool               `json:"disableAllowed"`
	Component      string             `json:"component,omitempty"`
	Enabled        bool               `json:"enabled"`
	Healthy        bool               `json:"healthy"`
}

func BuildStatus(def Definition, enablement domain.EnablementState) ExtensionStatus {
	return ExtensionStatus{
		ExtensionID:    def.Extension.ID,
		Builtin:        true,
		Required:       def.Required,
		DisableAllowed: def.DisableAllowed,
		Component:      GetComponentName(def.Extension),
		Enabled:        enablement == domain.EnablementEnabled,
		Healthy:        true,
	}
}
