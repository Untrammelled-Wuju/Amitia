package permission

import "errors"

var (
	ErrPermissionExecutionContextInvalid  = errors.New("permission execution context invalid")
	ErrPermissionExecutionBindingMismatch = errors.New("permission execution binding mismatch")
	ErrPermissionDeviceBindingMismatch    = errors.New("permission device binding mismatch")
	ErrPermissionRuntimeBindingMismatch   = errors.New("permission runtime binding mismatch")
	ErrPermissionProviderBindingMismatch  = errors.New("permission provider binding mismatch")
)
