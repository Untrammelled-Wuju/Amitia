package binary

import (
	"github.com/u-ai/backend/internal/gamehost/domain"
)

type BinaryOwner struct {
	PluginID  domain.PluginID
	RuntimeID domain.RuntimeInstanceID
	ServiceID domain.ServiceID

	ChannelID domain.ChannelID
}

func (o BinaryOwner) Validate() error {
	if o.PluginID == "" {
		return ErrOwnerRequired
	}
	if o.RuntimeID == "" {
		return ErrOwnerRequired
	}
	if o.ServiceID == "" {
		return ErrOwnerRequired
	}
	if o.ChannelID == "" {
		return ErrChannelIDEmpty
	}
	return nil
}

func (o BinaryOwner) Key() string {
	return string(o.PluginID) + "/" + string(o.RuntimeID) + "/" + string(o.ServiceID) + "/" + string(o.ChannelID)
}
