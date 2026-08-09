package checkpoint

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func ComputeDescriptorRevision(descriptor domain.PluginDescriptor) string {
	h := sha256.New()

	writeBytes := func(b []byte) {
		h.Write(b)
		h.Write([]byte{0})
	}
	writeString := func(s string) { writeBytes([]byte(s)) }
	writeBool := func(b bool) {
		if b {
			h.Write([]byte{1})
		} else {
			h.Write([]byte{0})
		}
		h.Write([]byte{0})
	}

	writeString(string(descriptor.ID))
	writeString(descriptor.ExtensionID)
	writeString(descriptor.Version)
	writeString(descriptor.ProtocolVersion)

	caps := make([]domain.Capability, len(descriptor.Capabilities))
	copy(caps, descriptor.Capabilities)
	sort.SliceStable(caps, func(i, j int) bool {
		return caps[i] < caps[j]
	})
	for _, cap := range caps {
		writeString(string(cap))
	}

	sortedServices := make([]int, len(descriptor.Services))
	for i := range descriptor.Services {
		sortedServices[i] = i
	}
	sort.SliceStable(sortedServices, func(i, j int) bool {
		return descriptor.Services[sortedServices[i]].ID < descriptor.Services[sortedServices[j]].ID
	})

	for _, idx := range sortedServices {
		svc := descriptor.Services[idx]
		writeString(string(svc.ID))
		writeString(string(svc.Kind))
		writeBool(svc.Required)

		deps := make([]domain.ServiceID, len(svc.DependsOn))
		copy(deps, svc.DependsOn)
		sort.SliceStable(deps, func(i, j int) bool {
			return deps[i] < deps[j]
		})
		for _, dep := range deps {
			writeString(string(dep))
		}
	}

	sortedChannels := make([]domain.ChannelID, len(descriptor.Channels))
	copy(sortedChannels, descriptor.Channels)
	sort.SliceStable(sortedChannels, func(i, j int) bool {
		return sortedChannels[i] < sortedChannels[j]
	})
	for _, chID := range sortedChannels {
		writeString(string(chID))
	}

	sum := h.Sum(nil)
	return "rev-" + hex.EncodeToString(sum)
}
