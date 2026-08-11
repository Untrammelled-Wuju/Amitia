package resource

import "errors"

var (
	ErrResourceDenied       = errors.New("resource: admission denied")
	ErrSubjectInvalid       = errors.New("resource: invalid runtime identity")
	ErrRuntimeNotFound      = errors.New("resource: runtime not found")
	ErrServiceNotFound      = errors.New("resource: service not found")
	ErrGenerationMismatch   = errors.New("resource: generation mismatch")
	ErrRuntimeStopped       = errors.New("resource: runtime stopped")
	ErrRuntimeStopping      = errors.New("resource: runtime stopping")
	ErrExtensionDisabled    = errors.New("resource: extension disabled")
	ErrExtensionUninstalled = errors.New("resource: extension uninstalled")
	ErrHostShutdown         = errors.New("resource: host shutting down")
	ErrProfileInvalid       = errors.New("resource: invalid resource profile")
	ErrOverflow             = errors.New("resource: integer overflow in resource calculation")
)
