package handshake

import (
	"encoding/json"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type SDKInfo struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

type ChannelAdvertisement struct {
	ID string `json:"id"`
}

type HelloRequest struct {
	SupportedProtocols []string              `json:"supportedProtocols"`
	Capabilities       []string              `json:"capabilities,omitempty"`
	RPCNamespaces      []string              `json:"rpcNamespaces,omitempty"`
	Channels           []ChannelAdvertisement `json:"channels,omitempty"`
	SDK                *SDKInfo              `json:"sdk,omitempty"`
	Metadata           map[string]json.RawMessage `json:"metadata,omitempty"`
}

type HelloResponse struct {
	Protocol     string                      `json:"protocol"`
	Capabilities []string                    `json:"capabilities"`
	RPCNamespaces []string                   `json:"rpcNamespaces,omitempty"`
	Channels     []string                    `json:"channels,omitempty"`
	Metadata     map[string]json.RawMessage  `json:"metadata,omitempty"`
}

const HelloMethod = "control.handshake.hello"

type ServiceCapabilities interface {
	GetCapability(name domain.Capability) bool
	HasCapability(name domain.Capability) bool
	ListCapabilities() []domain.Capability
}
