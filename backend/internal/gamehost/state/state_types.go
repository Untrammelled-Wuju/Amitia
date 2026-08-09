package state

import (
	"context"
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type StateUpdate struct {
	ID         string
	PluginID   domain.PluginID
	RuntimeID  domain.RuntimeInstanceID
	ServiceID  domain.ServiceID
	Key        string
	Payload    json.RawMessage
	Metadata   map[string]json.RawMessage
	ReceivedAt time.Time
}

type StateSnapshot struct {
	PluginID        domain.PluginID
	RuntimeID       domain.RuntimeInstanceID
	ServiceID       domain.ServiceID
	Key             string
	Payload         json.RawMessage
	Metadata        map[string]json.RawMessage
	SourceMessageID string
	Version         uint64
	UpdatedAt       time.Time
}

type StateFilter struct {
	PluginID  *domain.PluginID
	RuntimeID *domain.RuntimeInstanceID
	ServiceID *domain.ServiceID
	KeyPrefix string
}

func (f StateFilter) Match(snapshot StateSnapshot) bool {
	if f.PluginID != nil && snapshot.PluginID != *f.PluginID {
		return false
	}
	if f.RuntimeID != nil && snapshot.RuntimeID != *f.RuntimeID {
		return false
	}
	if f.ServiceID != nil && snapshot.ServiceID != *f.ServiceID {
		return false
	}
	if f.KeyPrefix != "" && !hasPrefix(snapshot.Key, f.KeyPrefix) {
		return false
	}
	return true
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

type StateStore interface {
	Put(ctx context.Context, update StateUpdate) (StateSnapshot, error)
	Get(ctx context.Context, key StateKey) (StateSnapshot, error)
	List(ctx context.Context, filter StateFilter) ([]StateSnapshot, error)
	Remove(ctx context.Context, key StateKey) error
	RemoveByService(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) error
	RemoveByRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) error
	RemoveByPlugin(ctx context.Context, pluginID domain.PluginID) error
	Count(ctx context.Context) int
	CountByRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) int
}
