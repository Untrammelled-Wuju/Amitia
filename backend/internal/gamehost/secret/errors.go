package secret

import "fmt"

var (
	ErrLeaseDenied          = fmt.Errorf("gamehost.secret: lease denied")
	ErrServiceNotReady      = fmt.Errorf("gamehost.secret: service not ready")
	ErrRuntimeInvalid       = fmt.Errorf("gamehost.secret: runtime invalid")
	ErrServiceInvalid       = fmt.Errorf("gamehost.secret: service invalid")
	ErrSecretRefInvalid     = fmt.Errorf("gamehost.secret: secret ref invalid")
	ErrBindingInvalid       = fmt.Errorf("gamehost.secret: binding invalid")
	ErrPermissionDenied     = fmt.Errorf("gamehost.secret: permission denied")
	ErrScopeDenied          = fmt.Errorf("gamehost.secret: scope denied")
	ErrGenerationMismatch   = fmt.Errorf("gamehost.secret: generation mismatch")
	ErrLeaseRevoked         = fmt.Errorf("gamehost.secret: lease revoked")
	ErrRuntimeStopped       = fmt.Errorf("gamehost.secret: runtime stopped")
	ErrServiceStopped       = fmt.Errorf("gamehost.secret: service stopped")
	ErrExtensionDisabled    = fmt.Errorf("gamehost.secret: extension disabled")
	ErrExtensionUninstalled = fmt.Errorf("gamehost.secret: extension uninstalled")
	ErrHostShutdown         = fmt.Errorf("gamehost.secret: host shutdown")
	ErrPartialAcquisition   = fmt.Errorf("gamehost.secret: partial acquisition failure")
	ErrBindingConflict      = fmt.Errorf("gamehost.secret: binding conflict")
	ErrSecretStoreFailure   = fmt.Errorf("gamehost.secret: secret store failure")
)
