package handshake

import (
	"sort"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type ProtocolNegotiator struct {
	hostSupported []string
}

func NewProtocolNegotiator(hostSupported []string) *ProtocolNegotiator {
	sorted := make([]string, len(hostSupported))
	copy(sorted, hostSupported)
	sort.Strings(sorted)
	return &ProtocolNegotiator{hostSupported: sorted}
}

func (n *ProtocolNegotiator) Negotiate(peerSupported []string) (string, error) {
	if len(peerSupported) == 0 {
		return "", NewHandshakeError(
			HandshakeErrorProtocolMismatch,
			domain.ErrProtocolMismatch,
			"peer must declare at least one supported protocol",
		)
	}

	for _, peer := range peerSupported {
		for _, host := range n.hostSupported {
			if peer == host {
				return peer, nil
			}
		}
	}

	return "", NewHandshakeError(
		HandshakeErrorProtocolMismatch,
		domain.ErrProtocolMismatch,
		"no common protocol version",
	)
}

type CapabilityNegotiator struct {
	hostSupported map[domain.Capability]struct{}
}

func NewCapabilityNegotiator(hostCaps []domain.Capability) *CapabilityNegotiator {
	m := make(map[domain.Capability]struct{}, len(hostCaps))
	for _, c := range hostCaps {
		m[c] = struct{}{}
	}
	return &CapabilityNegotiator{hostSupported: m}
}

func (n *CapabilityNegotiator) Negotiate(
	descriptorCaps []domain.Capability,
	serviceAdvertised []domain.Capability,
) ([]domain.Capability, error) {
	descriptorSet := make(map[domain.Capability]struct{}, len(descriptorCaps))
	for _, c := range descriptorCaps {
		descriptorSet[c] = struct{}{}
	}

	advertisedSet := make(map[domain.Capability]struct{}, len(serviceAdvertised))
	for _, c := range serviceAdvertised {
		if _, ok := descriptorSet[c]; !ok {
			return nil, NewHandshakeError(
				HandshakeErrorCapabilityMismatch,
				domain.ErrInvalidArgument,
				"capability not declared by package descriptor: "+string(c),
			)
		}
		advertisedSet[c] = struct{}{}
	}

	result := make([]domain.Capability, 0)
	for c := range advertisedSet {
		if _, ok := n.hostSupported[c]; !ok {
			continue
		}
		result = append(result, c)
	}

	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result, nil
}

func ValidateCapabilitiesNoDuplicate(caps []domain.Capability) error {
	seen := make(map[domain.Capability]struct{}, len(caps))
	for _, c := range caps {
		if _, ok := seen[c]; ok {
			return NewHandshakeError(
				HandshakeErrorCapabilityMismatch,
				domain.ErrInvalidArgument,
				"duplicate capability advertisement: "+string(c),
			)
		}
		seen[c] = struct{}{}
	}
	return nil
}
