package capability

import (
	"strings"
	"time"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type FilterPlacement string

func (p FilterPlacement) String() string {
	return string(p)
}

type Owner struct {
	UserID    runtimeidentity.UserID    `json:"userId"`
	DeviceID  runtimeidentity.DeviceID  `json:"deviceId"`
	RuntimeID runtimeidentity.RuntimeID `json:"runtimeId"`
}

func (o Owner) IsValid() bool {
	return o.UserID != "" || o.DeviceID != "" || o.RuntimeID != ""
}

type ProviderID string
type ProviderInstanceID string

func (id ProviderID) String() string {
	return string(id)
}

func (id ProviderID) IsEmpty() bool {
	return strings.TrimSpace(string(id)) == ""
}

func (id ProviderInstanceID) String() string {
	return string(id)
}

func (id ProviderInstanceID) IsEmpty() bool {
	return strings.TrimSpace(string(id)) == ""
}

func ParseProviderID(raw string) ProviderID {
	return ProviderID(strings.TrimSpace(raw))
}

func ParseProviderInstanceID(raw string) ProviderInstanceID {
	return ProviderInstanceID(strings.TrimSpace(raw))
}

type ProviderKind string

const (
	ProviderKindBuiltin   ProviderKind = "builtin"
	ProviderKindExtension ProviderKind = "extension"
	ProviderKindMCP       ProviderKind = "mcp"
	ProviderKindRuntime   ProviderKind = "runtime"
	ProviderKindInternal  ProviderKind = "internal"
)

func (k ProviderKind) String() string {
	return string(k)
}

func (k ProviderKind) IsValid() bool {
	switch k {
	case ProviderKindBuiltin, ProviderKindExtension, ProviderKindMCP, ProviderKindRuntime, ProviderKindInternal:
		return true
	}
	return false
}

func ParseProviderKind(raw string) ProviderKind {
	return ProviderKind(strings.TrimSpace(raw))
}

type ProviderPlacement string

const (
	ProviderPlacementCore   ProviderPlacement = "core"
	ProviderPlacementDevice ProviderPlacement = "device"
)

func (p ProviderPlacement) String() string {
	return string(p)
}

func (p ProviderPlacement) IsValid() bool {
	switch p {
	case ProviderPlacementCore, ProviderPlacementDevice:
		return true
	}
	return false
}

func ParseProviderPlacement(raw string) ProviderPlacement {
	return ProviderPlacement(strings.TrimSpace(raw))
}

type CapabilityProviderDefinition struct {
	ID           ProviderID        `json:"id"`
	CapabilityID CapabilityID      `json:"capabilityId"`
	Kind         ProviderKind      `json:"kind"`
	Placement    ProviderPlacement `json:"placement"`

	ExtensionID string `json:"extensionId,omitempty"`
	ModuleID    string `json:"moduleId,omitempty"`

	Runtime RuntimeBinding `json:"runtime"`

	Priority int `json:"priority,omitempty"`

	Platforms []runtimeidentity.Platform `json:"platforms,omitempty"`

	Metadata map[string]any `json:"metadata,omitempty"`

	Revision int64 `json:"revision"`
}

func (d CapabilityProviderDefinition) Normalize() CapabilityProviderDefinition {
	d.ID = ParseProviderID(string(d.ID))
	d.CapabilityID = ParseCapabilityID(string(d.CapabilityID))
	d.ExtensionID = strings.TrimSpace(d.ExtensionID)
	d.ModuleID = strings.TrimSpace(d.ModuleID)
	d.Platforms = normalizePlatforms(d.Platforms)
	d.Metadata = cloneStringAnyMap(d.Metadata)
	return d
}

func (d CapabilityProviderDefinition) Validate() error {
	if d.ID.IsEmpty() {
		return ErrProviderInvalid
	}
	if d.CapabilityID.IsEmpty() {
		return ErrProviderInvalid
	}
	if !d.Kind.IsValid() {
		return ErrProviderInvalid
	}
	if !d.Placement.IsValid() {
		return ErrProviderInvalid
	}
	if d.Runtime.RuntimeType == "" {
		return ErrProviderInvalid
	}
	if d.Kind == ProviderKindExtension && d.ExtensionID == "" {
		return ErrProviderInvalid
	}
	if d.ModuleID != "" && d.ExtensionID == "" {
		return ErrProviderInvalid
	}
	return nil
}

type CapabilityProviderInstance struct {
	ID           ProviderInstanceID `json:"id"`
	ProviderID   ProviderID         `json:"providerId"`
	CapabilityID CapabilityID       `json:"capabilityId"`
	Placement    ProviderPlacement  `json:"placement"`

	ExtensionID string `json:"extensionId,omitempty"`
	ModuleID    string `json:"moduleId,omitempty"`

	UserID    runtimeidentity.UserID    `json:"userId,omitempty"`
	DeviceID  runtimeidentity.DeviceID  `json:"deviceId,omitempty"`
	RuntimeID runtimeidentity.RuntimeID `json:"runtimeId,omitempty"`

	RuntimeInstanceID string `json:"runtimeInstanceId,omitempty"`

	RuntimeSessionID runtimeidentity.RuntimeSessionID `json:"runtimeSessionId,omitempty"`

	Health       HealthStatus              `json:"health"`
	Availability ProviderAvailabilityState `json:"availability"`

	RegisteredAt time.Time `json:"registeredAt"`
	UpdatedAt    time.Time `json:"updatedAt"`

	Metadata map[string]any `json:"metadata,omitempty"`

	Revision int64 `json:"revision"`
}

func (p CapabilityProviderInstance) Normalize() CapabilityProviderInstance {
	p.ID = ParseProviderInstanceID(string(p.ID))
	p.ProviderID = ParseProviderID(string(p.ProviderID))
	p.CapabilityID = ParseCapabilityID(string(p.CapabilityID))
	p.UserID = runtimeidentity.ParseUserID(string(p.UserID))
	p.DeviceID = runtimeidentity.ParseDeviceID(string(p.DeviceID))
	p.RuntimeID = runtimeidentity.ParseRuntimeID(string(p.RuntimeID))
	p.RuntimeInstanceID = strings.TrimSpace(p.RuntimeInstanceID)
	p.Metadata = cloneStringAnyMap(p.Metadata)
	p.RegisteredAt = p.RegisteredAt.UTC()
	p.UpdatedAt = p.UpdatedAt.UTC()
	return p
}

func (p CapabilityProviderInstance) Validate() error {
	if p.ID.IsEmpty() {
		return ErrProviderInstanceInvalid
	}
	if p.ProviderID.IsEmpty() {
		return ErrProviderInstanceInvalid
	}
	if p.CapabilityID.IsEmpty() {
		return ErrProviderInstanceInvalid
	}
	if !p.Placement.IsValid() {
		return ErrProviderInstanceInvalid
	}
	if !p.Availability.IsValid() {
		return ErrProviderInstanceInvalid
	}
	if !p.Health.IsValid() {
		return ErrProviderInstanceInvalid
	}
	if err := p.ValidateIdentity(); err != nil {
		return err
	}
	return nil
}

func (p CapabilityProviderInstance) ValidateIdentity() error {
	if p.Placement == ProviderPlacementDevice {
		if p.UserID == "" || p.DeviceID == "" || p.RuntimeID == "" {
			return ErrProviderInstanceIdentityInvalid
		}
	}
	return nil
}

func (p CapabilityProviderInstance) IsExecutable() bool {
	if p.Availability != ProviderAvailabilityAvailable || p.Health != HealthReady {
		return false
	}
	if p.Placement == ProviderPlacementDevice {
		return p.RuntimeSessionID != ""
	}
	return true
}

func normalizePlatforms(src []runtimeidentity.Platform) []runtimeidentity.Platform {
	if len(src) == 0 {
		return nil
	}
	seen := make(map[runtimeidentity.Platform]struct{}, len(src))
	result := make([]runtimeidentity.Platform, 0, len(src))
	for _, p := range src {
		parsed, err := runtimeidentity.ParsePlatform(p.String())
		if err != nil || !parsed.IsKnown() {
			continue
		}
		p = parsed
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		result = append(result, p)
	}
	sortPlatforms(result)
	return result
}

func sortPlatforms(items []runtimeidentity.Platform) {
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[i] > items[j] {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func cloneRuntimeBinding(binding RuntimeBinding) RuntimeBinding {
	binding.Metadata = cloneStringAnyMap(binding.Metadata)
	return binding
}

func cloneProviderDefinition(def *CapabilityProviderDefinition) *CapabilityProviderDefinition {
	if def == nil {
		return nil
	}
	cp := *def
	cp.Platforms = append([]runtimeidentity.Platform(nil), def.Platforms...)
	cp.Metadata = cloneStringAnyMap(def.Metadata)
	cp.Runtime = cloneRuntimeBinding(def.Runtime)
	return &cp
}

func cloneProviderInstance(inst *CapabilityProviderInstance) *CapabilityProviderInstance {
	if inst == nil {
		return nil
	}
	cp := *inst
	cp.Metadata = cloneStringAnyMap(inst.Metadata)
	return &cp
}
