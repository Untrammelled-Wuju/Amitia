package capability

import "errors"

type providerExecutionError struct {
	base error
}

func (e providerExecutionError) Error() string {
	return e.base.Error()
}

func (e providerExecutionError) Unwrap() error {
	return e.base
}

func (e providerExecutionError) IsProviderExecutionError() bool {
	return true
}

var (
	ErrProviderExecutionTargetInvalid      = providerExecutionError{base: errors.New("provider execution target is invalid")}
	ErrProviderExecutionProviderNotFound   = providerExecutionError{base: errors.New("provider execution provider not found")}
	ErrProviderExecutionInstanceNotFound   = providerExecutionError{base: errors.New("provider execution instance not found")}
	ErrProviderExecutionBindingMismatch    = providerExecutionError{base: errors.New("provider execution binding mismatch")}
	ErrProviderExecutionCapabilityMismatch = providerExecutionError{base: errors.New("provider execution capability mismatch")}
	ErrProviderExecutionPlacementMismatch  = providerExecutionError{base: errors.New("provider execution placement mismatch")}
	ErrProviderExecutionIdentityMismatch   = providerExecutionError{base: errors.New("provider execution identity mismatch")}
	ErrProviderExecutionUnavailable        = providerExecutionError{base: errors.New("provider execution instance unavailable")}
	ErrProviderRuntimeBindingInvalid       = providerExecutionError{base: errors.New("provider runtime binding invalid")}
)
