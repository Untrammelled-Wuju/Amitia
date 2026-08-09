package state

import (
	"strings"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

const (
	maxKeyLength    = 1024
	maxKeySegments  = 32
	separatorLength = 1
)

type StateKey struct {
	PluginID  domain.PluginID
	RuntimeID domain.RuntimeInstanceID
	ServiceID domain.ServiceID
	Key       string
}

func NewStateKey(plugin domain.PluginID, runtime domain.RuntimeInstanceID, service domain.ServiceID, key string) StateKey {
	return StateKey{
		PluginID:  plugin,
		RuntimeID: runtime,
		ServiceID: service,
		Key:       key,
	}
}

func (k StateKey) DomainKey() string {
	return k.Key
}

func (k StateKey) Identity() (domain.PluginID, domain.RuntimeInstanceID, domain.ServiceID) {
	return k.PluginID, k.RuntimeID, k.ServiceID
}

func (k StateKey) Validate() error {
	if k.PluginID == "" {
		return domain.NewHostError(domain.ErrInvalidArgument, "state key: plugin id must not be empty")
	}
	if k.RuntimeID == "" {
		return domain.NewHostError(domain.ErrInvalidArgument, "state key: runtime id must not be empty")
	}
	if k.ServiceID == "" {
		return domain.NewHostError(domain.ErrInvalidArgument, "state key: service id must not be empty")
	}
	if err := validateStateKeyString(k.Key); err != nil {
		return err
	}
	return nil
}

func validateStateKeyString(key string) error {
	if key == "" {
		return domain.NewHostError(domain.ErrInvalidArgument, "state key: key must not be empty")
	}
	if len(key) > maxKeyLength {
		return domain.NewHostError(domain.ErrInvalidArgument, "state key: key exceeds maximum length")
	}
	if len(strings.SplitN(key, "", maxKeySegments+1)) > maxKeySegments+1 {
		return domain.NewHostError(domain.ErrInvalidArgument, "state key: key segments exceed maximum")
	}
	for _, r := range key {
		if r < 0x20 {
			return domain.NewHostError(domain.ErrInvalidArgument, "state key: key contains control characters")
		}
	}
	return nil
}

func (k StateKey) Compare(other StateKey) int {
	if k.PluginID < other.PluginID {
		return -1
	}
	if k.PluginID > other.PluginID {
		return 1
	}
	if k.RuntimeID < other.RuntimeID {
		return -1
	}
	if k.RuntimeID > other.RuntimeID {
		return 1
	}
	if k.ServiceID < other.ServiceID {
		return -1
	}
	if k.ServiceID > other.ServiceID {
		return 1
	}
	if k.Key < other.Key {
		return -1
	}
	if k.Key > other.Key {
		return 1
	}
	return 0
}
