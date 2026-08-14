package host_registry

import "errors"

var (
	ErrInvalidRegistryEntry    = errors.New("host_registry: invalid registry entry")
	ErrRegistryEntryNotFound   = errors.New("host_registry: registry entry not found")
	ErrDevicePresenceNotFound  = errors.New("host_registry: device presence not found")
	ErrRuntimePresenceNotFound = errors.New("host_registry: runtime presence not found")
	ErrInvalidHostEntry        = ErrInvalidRegistryEntry
	ErrHostNotFound            = ErrRegistryEntryNotFound

	ErrStaleRuntimeSessionBinding    = errors.New("host_registry: stale runtime session binding")
	ErrRuntimeSessionBindingConflict = errors.New("host_registry: runtime session binding conflict")
)
