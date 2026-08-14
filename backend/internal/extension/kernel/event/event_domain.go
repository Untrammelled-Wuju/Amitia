package event

import "strings"

type EventDomain string

const (
	EventDomainSystem             EventDomain = "system"
	EventDomainExtension          EventDomain = "extension"
	EventDomainInteraction        EventDomain = "interaction"
	EventDomainDevice             EventDomain = "device"
	EventDomainRuntime            EventDomain = "runtime"
	EventDomainTask               EventDomain = "task"
	EventDomainCapabilityProvider EventDomain = "capability_provider"
	EventDomainSync               EventDomain = "sync"
)

func (d EventDomain) String() string {
	return string(d)
}

func (d EventDomain) IsValid() bool {
	switch d {
	case EventDomainSystem, EventDomainExtension, EventDomainInteraction,
		EventDomainDevice, EventDomainRuntime, EventDomainTask,
		EventDomainCapabilityProvider, EventDomainSync:
		return true
	}
	return false
}

func ParseEventDomain(raw string) EventDomain {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "system":
		return EventDomainSystem
	case "extension":
		return EventDomainExtension
	case "interaction":
		return EventDomainInteraction
	case "device":
		return EventDomainDevice
	case "runtime":
		return EventDomainRuntime
	case "task":
		return EventDomainTask
	case "capability_provider":
		return EventDomainCapabilityProvider
	case "sync":
		return EventDomainSync
	}
	return EventDomainSystem
}
