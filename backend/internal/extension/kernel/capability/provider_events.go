package capability

import (
	"context"
	"sort"
	"time"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type ProviderEventSink interface {
	ProviderRegistered(ctx context.Context, payload ProviderRegisteredPayload) error
	ProviderUpdated(ctx context.Context, payload ProviderUpdatedPayload) error
	ProviderUnregistered(ctx context.Context, payload ProviderUnregisteredPayload) error

	ProviderInstanceRegistered(ctx context.Context, payload ProviderInstanceEventPayload) error
	ProviderInstanceUpdated(ctx context.Context, payload ProviderInstanceEventPayload) error
	ProviderInstanceUnregistered(ctx context.Context, payload ProviderInstanceEventPayload) error
	ProviderInstanceAvailabilityChanged(ctx context.Context, payload ProviderInstanceAvailabilityChangedPayload) error
	ProviderInstanceHealthChanged(ctx context.Context, payload ProviderInstanceHealthChangedPayload) error
}

type ProviderRegisteredPayload struct {
	ProviderID   ProviderID   `json:"providerId"`
	CapabilityID CapabilityID `json:"capabilityId"`

	Kind      ProviderKind      `json:"kind"`
	Placement ProviderPlacement `json:"placement"`

	ExtensionID string `json:"extensionId,omitempty"`
	ModuleID    string `json:"moduleId,omitempty"`

	Priority int `json:"priority,omitempty"`

	Revision int64 `json:"revision,omitempty"`

	OccurredAt time.Time `json:"occurredAt"`
}

type ProviderUpdatedPayload struct {
	ProviderID   ProviderID   `json:"providerId"`
	CapabilityID CapabilityID `json:"capabilityId"`

	Kind      ProviderKind      `json:"kind"`
	Placement ProviderPlacement `json:"placement"`

	ExtensionID string `json:"extensionId,omitempty"`
	ModuleID    string `json:"moduleId,omitempty"`

	Priority int `json:"priority,omitempty"`

	ChangedFields []string `json:"changedFields,omitempty"`

	Revision int64 `json:"revision,omitempty"`

	OccurredAt time.Time `json:"occurredAt"`
}

type ProviderUnregisteredPayload struct {
	ProviderID   ProviderID   `json:"providerId"`
	CapabilityID CapabilityID `json:"capabilityId"`

	ExtensionID string `json:"extensionId,omitempty"`
	ModuleID    string `json:"moduleId,omitempty"`

	RemovedInstanceIDs []ProviderInstanceID `json:"removedInstanceIds,omitempty"`

	OccurredAt time.Time `json:"occurredAt"`
}

type ProviderInstanceEventPayload struct {
	ProviderInstanceID ProviderInstanceID `json:"providerInstanceId"`
	ProviderID         ProviderID         `json:"providerId"`
	CapabilityID       CapabilityID       `json:"capabilityId"`

	Placement ProviderPlacement `json:"placement"`

	UserID    runtimeidentity.UserID    `json:"userId,omitempty"`
	DeviceID  runtimeidentity.DeviceID  `json:"deviceId,omitempty"`
	RuntimeID runtimeidentity.RuntimeID `json:"runtimeId,omitempty"`

	RuntimeInstanceID string `json:"runtimeInstanceId,omitempty"`

	Health       HealthStatus              `json:"health"`
	Availability ProviderAvailabilityState `json:"availability"`

	Revision int64 `json:"revision,omitempty"`

	OccurredAt time.Time `json:"occurredAt"`
}

type ProviderInstanceAvailabilityChangedPayload struct {
	ProviderInstanceEventPayload

	Previous ProviderAvailabilityState `json:"previous"`
	Current  ProviderAvailabilityState `json:"current"`
}

type ProviderInstanceHealthChangedPayload struct {
	ProviderInstanceEventPayload

	Previous HealthStatus `json:"previous"`
	Current  HealthStatus `json:"current"`
}

func sortInstanceIDs(ids []ProviderInstanceID) []ProviderInstanceID {
	if len(ids) == 0 {
		return nil
	}
	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})
	return ids
}
