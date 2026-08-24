package notification

import (
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type Notification struct {
	ID         string
	PluginID   domain.PluginID
	RuntimeID  domain.RuntimeInstanceID
	ServiceID  domain.ServiceID
	Generation int64
	Method     string
	Payload    json.RawMessage
	Metadata   map[string]json.RawMessage
	ReceivedAt time.Time
}

type RouteContext struct {
	PluginID   domain.PluginID
	RuntimeID  domain.RuntimeInstanceID
	ServiceID  domain.ServiceID
	Generation int64
}

func (n Notification) Route() RouteContext {
	return RouteContext{
		PluginID:   n.PluginID,
		RuntimeID:  n.RuntimeID,
		ServiceID:  n.ServiceID,
		Generation: n.Generation,
	}
}

func deepCopyRaw(src json.RawMessage) json.RawMessage {
	if len(src) == 0 {
		return nil
	}
	dst := make([]byte, len(src))
	copy(dst, src)
	return dst
}

func deepCopyMetadata(src map[string]json.RawMessage) map[string]json.RawMessage {
	if src == nil {
		return nil
	}
	dst := make(map[string]json.RawMessage, len(src))
	for k, v := range src {
		dst[k] = deepCopyRaw(v)
	}
	return dst
}
