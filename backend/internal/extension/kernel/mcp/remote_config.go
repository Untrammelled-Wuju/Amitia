// migration-only: temporary compatibility adapter
// remove at step 65 cutover
package mcp

import (
	"time"

	"github.com/u-ai/backend/internal/mcp/protocol"
)

const (
	TransportTypeStreamableHTTP = "streamable_http"
	TransportTypeStdio          = "stdio"
)

type MCPRemoteSpec struct {
	ServerID string

	Endpoint string

	Timeout time.Duration

	MaxMessageBytes int64

	AllowLoopback   bool
	AllowPrivate    bool
	AllowPublicHTTP bool

	MaxRedirects int

	CredentialRef string

	StaticHeaders map[string]string
	Capabilities  protocol.ClientCapabilities
}

type MCPRemoteResolvedSpec struct {
	ServerID        string
	Endpoint        string
	Timeout         time.Duration
	MaxMessageBytes int64
	StaticHeaders   map[string]string
	Capabilities    protocol.ClientCapabilities
}

type RemoteTransportState string

const (
	RemoteStateStopped  RemoteTransportState = "stopped"
	RemoteStateStarting RemoteTransportState = "starting"
	RemoteStateRunning  RemoteTransportState = "running"
	RemoteStateClosing  RemoteTransportState = "closing"
	RemoteStateError    RemoteTransportState = "error"
)

func (spec MCPRemoteResolvedSpec) TimeoutOrDefault() time.Duration {
	if spec.Timeout <= 0 {
		return 30 * time.Second
	}
	return spec.Timeout
}

func (spec MCPRemoteResolvedSpec) MaxBytesOrDefault() int64 {
	if spec.MaxMessageBytes <= 0 {
		return 4 << 20
	}
	return spec.MaxMessageBytes
}
