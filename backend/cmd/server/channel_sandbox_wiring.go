package main

import (
	channelsandbox "github.com/u-ai/backend/internal/channel/sandbox"
	"github.com/u-ai/backend/internal/extension/kernel"
)

type channelSandboxPolicySource struct {
	sandbox *channelsandbox.Service
}

func newChannelSandboxPolicySource(service *channelsandbox.Service) *channelSandboxPolicySource {
	if service == nil {
		return nil
	}
	return &channelSandboxPolicySource{sandbox: service}
}

func (s *channelSandboxPolicySource) ChannelSandboxPolicy(channelID string) (kernel.ChannelSandboxPolicy, bool) {
	if s == nil || s.sandbox == nil {
		return kernel.ChannelSandboxPolicy{}, false
	}
	policy, err := s.sandbox.Get("", channelID)
	if err != nil || !policy.Enabled {
		return kernel.ChannelSandboxPolicy{}, false
	}
	root, err := policy.RootOrDefault()
	if err != nil {
		return kernel.ChannelSandboxPolicy{}, false
	}
	return kernel.ChannelSandboxPolicy{Enabled: true, AllowNetwork: policy.AllowNetwork, Root: root}, true
}
