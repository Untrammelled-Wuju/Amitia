package ipc

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type Peer struct {
	PluginID  domain.PluginID
	RuntimeID domain.RuntimeInstanceID
	ServiceID domain.ServiceID
}

type PeerKey struct {
	RuntimeID domain.RuntimeInstanceID
	ServiceID domain.ServiceID
}

func (p Peer) Key() PeerKey {
	return PeerKey{
		RuntimeID: p.RuntimeID,
		ServiceID: p.ServiceID,
	}
}

func (p Peer) Validate() error {
	if p.PluginID == "" {
		return fmt.Errorf("peer plugin id must not be empty")
	}
	if p.RuntimeID == "" {
		return fmt.Errorf("peer runtime id must not be empty")
	}
	if p.ServiceID == "" {
		return fmt.Errorf("peer service id must not be empty")
	}
	return nil
}

type RuntimePeerResolver interface {
	ResolveService(
		ctx context.Context,
		runtimeID domain.RuntimeInstanceID,
		serviceID domain.ServiceID,
	) (pluginID domain.PluginID, err error)
}
