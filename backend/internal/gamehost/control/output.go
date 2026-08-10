package control

import (
	"encoding/json"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type ControlOutputKind string

const (
	KindCustomRPC  ControlOutputKind = "custom_rpc"
	KindChannel    ControlOutputKind = "channel"
	KindBinary     ControlOutputKind = "binary"
	KindInternal   ControlOutputKind = "internal"
	KindHostAction ControlOutputKind = "host_action"
)

type ControlOutputIntent struct {
	OutputID       string
	RuntimeID      domain.RuntimeInstanceID
	ServiceID      domain.ServiceID
	AuthorityEpoch uint64
	Kind           ControlOutputKind
	Payload        json.RawMessage
}

type TrustedPluginIdentity struct {
	PluginID    domain.PluginID
	RuntimeID   domain.RuntimeInstanceID
	ServiceID   domain.ServiceID
	Generation  uint64
	ConnectedAt int64
}

func (t TrustedPluginIdentity) Empty() bool {
	return t.PluginID == ""
}
