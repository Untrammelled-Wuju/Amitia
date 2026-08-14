package permission

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type ExecutionPlacement string

const (
	ExecutionPlacementLocal  ExecutionPlacement = "local"
	ExecutionPlacementCore   ExecutionPlacement = "core"
	ExecutionPlacementDevice ExecutionPlacement = "device"
)

func (p ExecutionPlacement) String() string {
	return string(p)
}

func (p ExecutionPlacement) IsValid() bool {
	switch p {
	case ExecutionPlacementLocal, ExecutionPlacementCore, ExecutionPlacementDevice:
		return true
	default:
		return false
	}
}

func (p ExecutionPlacement) IsRemote() bool {
	return p == ExecutionPlacementDevice
}

func ParseExecutionPlacement(raw string) ExecutionPlacement {
	return ExecutionPlacement(strings.TrimSpace(raw))
}

type PermissionExecutionContext struct {
	Placement ExecutionPlacement `json:"placement,omitempty"`

	UserID           runtimeidentity.UserID           `json:"userId,omitempty"`
	DeviceID         runtimeidentity.DeviceID         `json:"deviceId,omitempty"`
	RuntimeID        runtimeidentity.RuntimeID        `json:"runtimeId,omitempty"`
	RuntimeSessionID runtimeidentity.RuntimeSessionID `json:"runtimeSessionId,omitempty"`

	ProviderID         string `json:"providerId,omitempty"`
	ProviderInstanceID string `json:"providerInstanceId,omitempty"`

	ExtensionID string `json:"extensionId,omitempty"`
	ModuleID    string `json:"moduleId,omitempty"`

	Source string `json:"source,omitempty"`
}

func (c PermissionExecutionContext) Normalize() PermissionExecutionContext {
	c.Placement = ParseExecutionPlacement(c.Placement.String())
	c.UserID = runtimeidentity.ParseUserID(c.UserID.String())
	c.DeviceID = runtimeidentity.ParseDeviceID(c.DeviceID.String())
	c.RuntimeID = runtimeidentity.ParseRuntimeID(c.RuntimeID.String())
	c.RuntimeSessionID = runtimeidentity.ParseRuntimeSessionID(c.RuntimeSessionID.String())
	c.ProviderID = strings.TrimSpace(c.ProviderID)
	c.ProviderInstanceID = strings.TrimSpace(c.ProviderInstanceID)
	c.ExtensionID = strings.TrimSpace(c.ExtensionID)
	c.ModuleID = strings.TrimSpace(c.ModuleID)
	c.Source = strings.TrimSpace(c.Source)
	return c
}

func (c PermissionExecutionContext) Validate() error {
	if c.Placement != "" && !c.Placement.IsValid() {
		return ErrPermissionExecutionContextInvalid
	}

	if c.Placement == ExecutionPlacementDevice {
		if c.UserID == "" || c.DeviceID == "" || c.RuntimeID == "" {
			return ErrPermissionExecutionContextInvalid
		}
	}

	if c.ProviderInstanceID != "" && c.ProviderID == "" {
		return ErrPermissionExecutionContextInvalid
	}

	if c.ModuleID != "" && c.ExtensionID == "" {
		return ErrPermissionExecutionContextInvalid
	}

	return nil
}

func (c PermissionExecutionContext) IsDeviceExecution() bool {
	return c.Placement == ExecutionPlacementDevice
}

func (c PermissionExecutionContext) HasProvider() bool {
	return c.ProviderID != ""
}

func (c PermissionExecutionContext) HasProviderInstance() bool {
	return c.ProviderInstanceID != ""
}

func (c PermissionExecutionContext) HasRuntimeIdentity() bool {
	return c.UserID != "" || c.DeviceID != "" || c.RuntimeID != ""
}

func (c PermissionExecutionContext) IsEmpty() bool {
	return c.Placement == "" && c.UserID == "" && c.DeviceID == "" && c.RuntimeID == "" && c.RuntimeSessionID == "" && c.ProviderID == "" && c.ProviderInstanceID == "" && c.ExtensionID == "" && c.ModuleID == "" && c.Source == ""
}

func (c PermissionExecutionContext) BindingKey() string {
	input := strings.Join([]string{
		c.Placement.String(),
		c.UserID.String(),
		c.DeviceID.String(),
		c.RuntimeID.String(),
		c.RuntimeSessionID.String(),
		c.ProviderID,
		c.ProviderInstanceID,
		c.ExtensionID,
		c.ModuleID,
	}, "|")
	h := sha256.Sum256([]byte(input))
	return "pexb_" + hex.EncodeToString(h[:16])
}

func (c PermissionExecutionContext) StableBindingKey() string {
	input := strings.Join([]string{
		c.Placement.String(),
		c.UserID.String(),
		c.DeviceID.String(),
		c.RuntimeID.String(),
		c.ProviderID,
		c.ProviderInstanceID,
		c.ExtensionID,
		c.ModuleID,
	}, "|")
	h := sha256.Sum256([]byte(input))
	return "pexs_" + hex.EncodeToString(h[:16])
}

func ExecutionContextFromInvocation(inv capability.ToolInvocationContext) PermissionExecutionContext {
	extID := inv.ExecutionTarget.ExtensionID
	if extID == "" {
		extID = inv.ExtensionID
	}

	moduleID := inv.ExecutionTarget.ModuleID
	if moduleID == "" {
		moduleID = inv.ModuleID
	}

	userID := inv.ExecutionTarget.UserID
	if userID == "" {
		userID = runtimeidentity.ParseUserID(inv.UserID)
	}

	return PermissionExecutionContext{
		Placement:          ParseExecutionPlacement(inv.ExecutionTarget.Placement),
		UserID:             userID,
		DeviceID:           inv.ExecutionTarget.DeviceID,
		RuntimeID:          inv.ExecutionTarget.RuntimeID,
		RuntimeSessionID:   inv.ExecutionTarget.RuntimeSessionID,
		ProviderID:         inv.ExecutionTarget.ProviderID,
		ProviderInstanceID: inv.ExecutionTarget.ProviderInstanceID,
		ExtensionID:        extID,
		ModuleID:           moduleID,
		Source:             string(inv.Source),
	}
}
