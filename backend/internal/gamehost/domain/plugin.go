package domain

import (
	"strings"
)

const (
	maxPluginNameLength      = 256
	maxVersionLength         = 64
	maxProtocolVersionLength = 128
)

type PluginID string

type PluginDescriptor struct {
	ID              PluginID
	ExtensionID     string
	Name            string
	Version         string
	ProtocolVersion string

	Capabilities []Capability
	Services     []ServiceDescriptor
	Channels     []ChannelDescriptor
	ControlSinks []ControlSinkDeclaration

	Metadata map[string]string
}

func (p PluginDescriptor) Validate() error {
	if p.ID == "" {
		return NewHostError(ErrInvalidArgument, "plugin id must not be empty")
	}
	if p.ExtensionID == "" {
		return NewHostError(ErrInvalidArgument, "extension id must not be empty")
	}
	if p.Name == "" {
		return NewHostError(ErrInvalidArgument, "plugin name must not be empty")
	}
	if len(p.Name) > maxPluginNameLength {
		return NewHostError(ErrInvalidArgument, "plugin name exceeds maximum length")
	}
	if p.Version == "" {
		return NewHostError(ErrInvalidArgument, "plugin version must not be empty")
	}
	if len(p.Version) > maxVersionLength {
		return NewHostError(ErrInvalidArgument, "plugin version exceeds maximum length")
	}
	if p.ProtocolVersion == "" {
		return NewHostError(ErrInvalidArgument, "protocol version must not be empty")
	}
	if len(p.ProtocolVersion) > maxProtocolVersionLength {
		return NewHostError(ErrInvalidArgument, "protocol version exceeds maximum length")
	}

	if strings.ContainsAny(string(p.ID), "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f") {
		return NewHostError(ErrInvalidArgument, "plugin id contains control characters")
	}
	if strings.ContainsAny(p.ExtensionID, "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f") {
		return NewHostError(ErrInvalidArgument, "extension id contains control characters")
	}
	if strings.ContainsAny(p.Name, "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f") {
		return NewHostError(ErrInvalidArgument, "plugin name contains control characters")
	}

	seenCapabilities := make(map[Capability]struct{})
	for _, cap := range p.Capabilities {
		if err := ValidateCapability(cap); err != nil {
			return err
		}
		if _, exists := seenCapabilities[cap]; exists {
			return NewHostErrorWithCause(ErrInvalidArgument, "duplicate capability",
				NewHostError(ErrInvalidArgument, string(cap)))
		}
		seenCapabilities[cap] = struct{}{}
	}

	seenServiceIDs := make(map[ServiceID]struct{})
	for _, svc := range p.Services {
		if err := svc.Validate(); err != nil {
			return err
		}
		if _, exists := seenServiceIDs[svc.ID]; exists {
			return NewHostErrorWithCause(ErrInvalidArgument, "duplicate service id",
				NewHostError(ErrInvalidArgument, string(svc.ID)))
		}
		seenServiceIDs[svc.ID] = struct{}{}
	}

	for _, svc := range p.Services {
		for _, depID := range svc.DependsOn {
			if _, exists := seenServiceIDs[depID]; !exists {
				return NewHostErrorWithCause(ErrInvalidArgument, "service depends on unknown service",
					NewHostError(ErrInvalidArgument, string(depID)))
			}
		}
	}

	seenChannelIDs := make(map[ChannelID]struct{})
	for _, ch := range p.Channels {
		if err := ch.Validate(); err != nil {
			return err
		}
		if _, exists := seenServiceIDs[ch.ServiceID]; !exists {
			return NewHostErrorWithCause(ErrInvalidArgument, "channel references unknown service",
				NewHostError(ErrInvalidArgument, string(ch.ServiceID)))
		}
		if _, exists := seenChannelIDs[ch.ID]; exists {
			return NewHostErrorWithCause(ErrInvalidArgument, "duplicate channel id",
				NewHostError(ErrInvalidArgument, string(ch.ID)))
		}
		seenChannelIDs[ch.ID] = struct{}{}
	}

	seenSinkIDs := make(map[string]struct{}, len(p.ControlSinks))
	for _, sink := range p.ControlSinks {
		if err := sink.Validate(); err != nil {
			return err
		}
		if _, exists := seenServiceIDs[sink.ServiceID]; len(seenServiceIDs) > 0 && !exists {
			return NewHostErrorWithCause(ErrInvalidArgument, "control sink references unknown service",
				NewHostError(ErrInvalidArgument, string(sink.ServiceID)))
		}
		if _, exists := seenSinkIDs[sink.ID]; exists {
			return NewHostErrorWithCause(ErrInvalidArgument, "duplicate control sink id",
				NewHostError(ErrInvalidArgument, sink.ID))
		}
		seenSinkIDs[sink.ID] = struct{}{}
	}

	if p.Metadata != nil {
		if err := validateMetadata(p.Metadata); err != nil {
			return NewHostErrorWithCause(ErrInvalidArgument, "plugin metadata validation failed", err)
		}
	}

	return nil
}

func (p PluginDescriptor) Clone() PluginDescriptor {
	cloned := PluginDescriptor{
		ID:              p.ID,
		ExtensionID:     p.ExtensionID,
		Name:            p.Name,
		Version:         p.Version,
		ProtocolVersion: p.ProtocolVersion,
	}

	if p.Capabilities != nil {
		cloned.Capabilities = make([]Capability, len(p.Capabilities))
		copy(cloned.Capabilities, p.Capabilities)
	}

	if p.Services != nil {
		cloned.Services = make([]ServiceDescriptor, len(p.Services))
		for i, svc := range p.Services {
			svcCopy := svc
			if svc.DependsOn != nil {
				svcCopy.DependsOn = make([]ServiceID, len(svc.DependsOn))
				copy(svcCopy.DependsOn, svc.DependsOn)
			}
			if svc.Metadata != nil {
				svcCopy.Metadata = make(map[string]string, len(svc.Metadata))
				for k, v := range svc.Metadata {
					svcCopy.Metadata[k] = v
				}
			}
			cloned.Services[i] = svcCopy
		}
	}

	if p.Channels != nil {
		cloned.Channels = make([]ChannelDescriptor, len(p.Channels))
		for i, ch := range p.Channels {
			chCopy := ch
			if ch.Metadata != nil {
				chCopy.Metadata = make(map[string]string, len(ch.Metadata))
				for k, v := range ch.Metadata {
					chCopy.Metadata[k] = v
				}
			}
			cloned.Channels[i] = chCopy
		}
	}

	if p.ControlSinks != nil {
		cloned.ControlSinks = append([]ControlSinkDeclaration(nil), p.ControlSinks...)
	}

	if p.Metadata != nil {
		cloned.Metadata = make(map[string]string, len(p.Metadata))
		for k, v := range p.Metadata {
			cloned.Metadata[k] = v
		}
	}

	return cloned
}

func (p PluginDescriptor) HasCapability(capability Capability) bool {
	for _, cap := range p.Capabilities {
		if cap == capability {
			return true
		}
	}
	return false
}

// EqualPluginDescriptor 比较两个PluginDescriptor是否等价，忽略Metadata差异
func EqualPluginDescriptor(a, b PluginDescriptor) bool {
	if a.ID != b.ID ||
		a.ExtensionID != b.ExtensionID ||
		a.Name != b.Name ||
		a.Version != b.Version ||
		a.ProtocolVersion != b.ProtocolVersion {
		return false
	}

	if len(a.Capabilities) != len(b.Capabilities) {
		return false
	}
	capMap := make(map[Capability]struct{}, len(a.Capabilities))
	for _, cap := range a.Capabilities {
		capMap[cap] = struct{}{}
	}
	for _, cap := range b.Capabilities {
		if _, ok := capMap[cap]; !ok {
			return false
		}
	}

	if len(a.Services) != len(b.Services) {
		return false
	}
	svcMap := make(map[ServiceID]ServiceDescriptor, len(a.Services))
	for _, svc := range a.Services {
		svcMap[svc.ID] = svc
	}
	for _, svc := range b.Services {
		other, ok := svcMap[svc.ID]
		if !ok || other.Name != svc.Name || other.Kind != svc.Kind || other.Required != svc.Required || !equalServiceIDs(other.DependsOn, svc.DependsOn) {
			return false
		}
	}

	if len(a.Channels) != len(b.Channels) {
		return false
	}
	chMap := make(map[ChannelID]ChannelDescriptor, len(a.Channels))
	for _, ch := range a.Channels {
		chMap[ch.ID] = ch
	}
	for _, ch := range b.Channels {
		other, ok := chMap[ch.ID]
		if !ok || other.ServiceID != ch.ServiceID || other.Kind != ch.Kind || other.SchemaID != ch.SchemaID {
			return false
		}
	}

	if len(a.ControlSinks) != len(b.ControlSinks) {
		return false
	}
	sinkMap := make(map[string]ControlSinkDeclaration, len(a.ControlSinks))
	for _, sink := range a.ControlSinks {
		sinkMap[sink.ID] = sink
	}
	for _, sink := range b.ControlSinks {
		other, ok := sinkMap[sink.ID]
		if !ok || other.ServiceID != sink.ServiceID || other.Kind != sink.Kind {
			return false
		}
	}

	return true
}

func equalServiceIDs(a, b []ServiceID) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[ServiceID]int, len(a))
	for _, id := range a {
		counts[id]++
	}
	for _, id := range b {
		counts[id]--
		if counts[id] < 0 {
			return false
		}
	}
	for _, n := range counts {
		if n != 0 {
			return false
		}
	}
	return true
}
