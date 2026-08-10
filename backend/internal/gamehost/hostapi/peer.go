package hostapi

import (
	"context"

	"github.com/u-ai/backend/internal/gamehost/ipc"
)

type Peer = ipc.Peer

type PeerKey = ipc.PeerKey

type PeerResolver interface {
	ResolveService(
		ctx context.Context,
		runtimeID string,
		serviceID string,
	) (pluginID string, err error)
}
