package channel

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type RuntimeChannelID string

type RuntimeChannel struct {
	ID RuntimeChannelID

	PluginID  domain.PluginID
	RuntimeID domain.RuntimeInstanceID
	ServiceID domain.ServiceID

	ChannelID domain.ChannelID

	Kind      domain.ChannelKind
	Direction protocol.ChannelDirection
	Frequency *protocol.FrequencyHint

	Metadata map[string]json.RawMessage
}

func NewRuntimeChannelID(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, channelID domain.ChannelID) RuntimeChannelID {
	return RuntimeChannelID(fmt.Sprintf("%s/%s/%s", runtimeID, serviceID, channelID))
}

func (id RuntimeChannelID) RuntimeID() domain.RuntimeInstanceID {
	parts := strings.SplitN(string(id), "/", 3)
	return domain.RuntimeInstanceID(parts[0])
}

func (id RuntimeChannelID) ServiceID() domain.ServiceID {
	parts := strings.SplitN(string(id), "/", 3)
	if len(parts) < 2 {
		return ""
	}
	return domain.ServiceID(parts[1])
}

func (id RuntimeChannelID) ChannelID() domain.ChannelID {
	parts := strings.SplitN(string(id), "/", 3)
	if len(parts) < 3 {
		return ""
	}
	return domain.ChannelID(parts[2])
}

func (c RuntimeChannel) Clone() RuntimeChannel {
	cloned := RuntimeChannel{
		ID:        c.ID,
		PluginID:  c.PluginID,
		RuntimeID: c.RuntimeID,
		ServiceID: c.ServiceID,
		ChannelID: c.ChannelID,
		Kind:      c.Kind,
		Direction: c.Direction,
	}
	if c.Frequency != nil {
		f := *c.Frequency
		cloned.Frequency = &f
	}
	if c.Metadata != nil {
		cloned.Metadata = make(map[string]json.RawMessage, len(c.Metadata))
		for k, v := range c.Metadata {
			cp := make(json.RawMessage, len(v))
			copy(cp, v)
			cloned.Metadata[k] = cp
		}
	}
	return cloned
}

func (c RuntimeChannel) Validate() error {
	if c.PluginID == "" {
		return domain.NewHostError(domain.ErrInvalidArgument, "channel: plugin id must not be empty")
	}
	if c.RuntimeID == "" {
		return domain.NewHostError(domain.ErrInvalidArgument, "channel: runtime id must not be empty")
	}
	if c.ServiceID == "" {
		return domain.NewHostError(domain.ErrInvalidArgument, "channel: service id must not be empty")
	}
	if c.ChannelID == "" {
		return ErrChannelIDEmpty
	}
	if !domain.IsValidChannelKind(c.Kind) {
		return ErrKindUnknown
	}
	if c.Direction != "" {
		if err := protocol.ValidateChannelDirection(c.Direction); err != nil {
			return ErrDirectionUnknown
		}
	}
	if c.Frequency != nil {
		if err := protocol.ValidateFrequencyHint(*c.Frequency); err != nil {
			return ErrFrequencyUnknown
		}
	}
	return nil
}
