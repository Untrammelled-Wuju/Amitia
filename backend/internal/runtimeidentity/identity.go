package runtimeidentity

import "strings"

type SpaceID string
type DeviceID string
type RuntimeID string
type RuntimeSessionID string

type Identity struct {
	SpaceID          SpaceID
	DeviceID         DeviceID
	RuntimeID        RuntimeID
	RuntimeSessionID RuntimeSessionID
}

func (i Identity) IsDeviceScoped() bool {
	return i.SpaceID != "" && i.DeviceID != ""
}

func (i Identity) IsRuntimeScoped() bool {
	return i.SpaceID != "" && i.DeviceID != "" && i.RuntimeID != ""
}

func (i Identity) IsSessionScoped() bool {
	return i.SpaceID != "" && i.DeviceID != "" && i.RuntimeID != "" && i.RuntimeSessionID != ""
}

func ParseSpaceID(raw string) SpaceID {
	return SpaceID(strings.TrimSpace(raw))
}

func ParseDeviceID(raw string) DeviceID {
	return DeviceID(strings.TrimSpace(raw))
}

func ParseRuntimeID(raw string) RuntimeID {
	return RuntimeID(strings.TrimSpace(raw))
}

func ParseRuntimeSessionID(raw string) RuntimeSessionID {
	return RuntimeSessionID(strings.TrimSpace(raw))
}

func (id SpaceID) String() string {
	return string(id)
}

func (id DeviceID) String() string {
	return string(id)
}

func (id RuntimeID) String() string {
	return string(id)
}

func (id RuntimeSessionID) String() string {
	return string(id)
}
