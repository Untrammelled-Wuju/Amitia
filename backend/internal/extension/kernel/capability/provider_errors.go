package capability

import "errors"

var (
	ErrProviderInvalid           = errors.New("capability provider is invalid")
	ErrProviderNotFound          = errors.New("capability provider not found")
	ErrProviderAlreadyRegistered = errors.New("capability provider already registered")

	ErrProviderInstanceInvalid           = errors.New("capability provider instance is invalid")
	ErrProviderInstanceIdentityInvalid   = errors.New("capability provider instance identity is invalid")
	ErrProviderInstanceNotFound          = errors.New("capability provider instance not found")
	ErrProviderInstanceAlreadyRegistered = errors.New("capability provider instance already registered")

	ErrProviderCapabilityMismatch = errors.New("capability provider capability mismatch")
	ErrProviderPlacementMismatch  = errors.New("capability provider placement mismatch")
	ErrProviderOwnershipMismatch  = errors.New("capability provider ownership mismatch")
)
