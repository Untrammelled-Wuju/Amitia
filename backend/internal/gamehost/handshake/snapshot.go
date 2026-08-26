package handshake

import (
	"sort"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type HandshakeSnapshot struct {
	Protocol      string
	Capabilities  []domain.Capability
	RPCNamespaces []string
	Channels      []string
	SDKName       string
	SDKVersion    string
	ReadyAt       time.Time
}

func NewHandshakeSnapshot(
	protocol string,
	capabilities []domain.Capability,
	rpcNamespaces []string,
	channels []string,
	sdkName string,
	sdkVersion string,
) *HandshakeSnapshot {
	caps := make([]domain.Capability, len(capabilities))
	copy(caps, capabilities)

	nss := make([]string, len(rpcNamespaces))
	copy(nss, rpcNamespaces)

	chs := make([]string, len(channels))
	copy(chs, channels)

	sort.Slice(caps, func(i, j int) bool { return caps[i] < caps[j] })
	sort.Strings(nss)
	sort.Strings(chs)

	return &HandshakeSnapshot{
		Protocol:      protocol,
		Capabilities:  caps,
		RPCNamespaces: nss,
		Channels:      chs,
		SDKName:       sdkName,
		SDKVersion:    sdkVersion,
	}
}

func (s *HandshakeSnapshot) Clone() *HandshakeSnapshot {
	if s == nil {
		return nil
	}
	clone := NewHandshakeSnapshot(
		s.Protocol,
		s.Capabilities,
		s.RPCNamespaces,
		s.Channels,
		s.SDKName,
		s.SDKVersion,
	)
	clone.ReadyAt = s.ReadyAt
	return clone
}

func (s *HandshakeSnapshot) HasCapability(name domain.Capability) bool {
	if s == nil {
		return false
	}
	for _, c := range s.Capabilities {
		if c == name {
			return true
		}
	}
	return false
}

func (s *HandshakeSnapshot) HasChannel(channelID string) bool {
	if s == nil || channelID == "" {
		return false
	}
	for _, id := range s.Channels {
		if id == channelID {
			return true
		}
	}
	return false
}
