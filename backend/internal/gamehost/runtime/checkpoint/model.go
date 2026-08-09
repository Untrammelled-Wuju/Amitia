package checkpoint

import (
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/runtime"
)

const (
	MetadataSchemaVersion   = 1
	MaxReasonLength         = 256
	MaxCheckpointSize       = 1 << 20
	MaxMetadataEntries      = 64
	MaxMetadataValueLength  = 4096
	MaxServicesPerCheckpoint = 64
)

type RuntimeMetadata struct {
	SchemaVersion int `json:"schemaVersion"`

	RuntimeID domain.RuntimeInstanceID `json:"runtimeId"`
	PluginID  domain.PluginID          `json:"pluginId"`

	ExtensionID string `json:"extensionId"`

	PluginVersion string `json:"pluginVersion"`

	CreatedAt time.Time `json:"createdAt"`

	DescriptorRevision string `json:"descriptorRevision,omitempty"`

	Metadata map[string]json.RawMessage `json:"metadata,omitempty"`
}

type ServiceCheckpoint struct {
	ServiceID domain.ServiceID           `json:"serviceId"`
	State     runtime.ServiceRuntimeState `json:"state"`
	Required  bool                       `json:"required"`
	UpdatedAt time.Time                  `json:"updatedAt"`
	Reason    string                     `json:"reason,omitempty"`
}

type RuntimeCheckpoint struct {
	SchemaVersion int `json:"schemaVersion"`

	RuntimeID domain.RuntimeInstanceID `json:"runtimeId"`
	PluginID  domain.PluginID          `json:"pluginId"`

	RuntimeState domain.RuntimeState `json:"runtimeState"`

	Services []ServiceCheckpoint `json:"services"`

	DescriptorRevision string `json:"descriptorRevision,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	LastKnownGoodAt *time.Time `json:"lastKnownGoodAt,omitempty"`

	CleanShutdown bool `json:"cleanShutdown"`

	Reason string `json:"reason,omitempty"`

	Metadata map[string]json.RawMessage `json:"metadata,omitempty"`
}

func (m RuntimeMetadata) Clone() RuntimeMetadata {
	clone := RuntimeMetadata{
		SchemaVersion:     m.SchemaVersion,
		RuntimeID:         m.RuntimeID,
		PluginID:          m.PluginID,
		ExtensionID:       m.ExtensionID,
		PluginVersion:     m.PluginVersion,
		CreatedAt:         m.CreatedAt,
		DescriptorRevision: m.DescriptorRevision,
	}
	if m.Metadata != nil {
		clone.Metadata = make(map[string]json.RawMessage, len(m.Metadata))
		for k, v := range m.Metadata {
			clone.Metadata[k] = v
		}
	}
	return clone
}

func (c RuntimeCheckpoint) Clone() RuntimeCheckpoint {
	clone := RuntimeCheckpoint{
		SchemaVersion:      c.SchemaVersion,
		RuntimeID:          c.RuntimeID,
		PluginID:           c.PluginID,
		RuntimeState:       c.RuntimeState,
		DescriptorRevision: c.DescriptorRevision,
		CreatedAt:          c.CreatedAt,
		UpdatedAt:          c.UpdatedAt,
		CleanShutdown:      c.CleanShutdown,
		Reason:             c.Reason,
	}
	if c.Services != nil {
		clone.Services = make([]ServiceCheckpoint, len(c.Services))
		copy(clone.Services, c.Services)
	}
	if c.LastKnownGoodAt != nil {
		t := *c.LastKnownGoodAt
		clone.LastKnownGoodAt = &t
	}
	if c.Metadata != nil {
		clone.Metadata = make(map[string]json.RawMessage, len(c.Metadata))
		for k, v := range c.Metadata {
			clone.Metadata[k] = v
		}
	}
	return clone
}
