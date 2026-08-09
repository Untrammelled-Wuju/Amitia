package service_definition

import "fmt"

type ServiceDefinitionErrorCode string

const (
	ErrDefinitionNotFound          ServiceDefinitionErrorCode = "definition_not_found"
	ErrDefinitionInvalid           ServiceDefinitionErrorCode = "definition_invalid"
	ErrDefinitionConflict          ServiceDefinitionErrorCode = "definition_conflict"
	ErrDefinitionMappingFailed     ServiceDefinitionErrorCode = "definition_mapping_failed"
	ErrDefinitionRegisterFailed    ServiceDefinitionErrorCode = "definition_register_failed"
	ErrDefinitionReplaceFailed     ServiceDefinitionErrorCode = "definition_replace_failed"
	ErrDefinitionRemoveFailed      ServiceDefinitionErrorCode = "definition_remove_failed"
	ErrDefinitionSourceMismatch    ServiceDefinitionErrorCode = "definition_source_mismatch"
	ErrDefinitionValidationFailed  ServiceDefinitionErrorCode = "definition_validation_failed"
	ErrDefinitionProviderUnavailable ServiceDefinitionErrorCode = "definition_provider_unavailable"
	ErrUnsupportedServiceKind      ServiceDefinitionErrorCode = "unsupported_service_kind"
	ErrServiceDefinitionNotFound   ServiceDefinitionErrorCode = "service_definition_not_found"
)

type ServiceDefinitionError struct {
	Code    ServiceDefinitionErrorCode
	Message string
	Cause   error
}

func (e *ServiceDefinitionError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *ServiceDefinitionError) Unwrap() error {
	return e.Cause
}

func NewServiceDefinitionError(code ServiceDefinitionErrorCode, message string) *ServiceDefinitionError {
	return &ServiceDefinitionError{
		Code:    code,
		Message: message,
	}
}

func NewServiceDefinitionErrorWithCause(code ServiceDefinitionErrorCode, message string, cause error) *ServiceDefinitionError {
	return &ServiceDefinitionError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

func IsServiceDefinitionError(err error, code ServiceDefinitionErrorCode) bool {
	se, ok := err.(*ServiceDefinitionError)
	if !ok {
		return false
	}
	return se.Code == code
}
