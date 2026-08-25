package protocol

import "fmt"

var ValidationError = fmt.Errorf("validation error")

type ValidationResult struct {
	Errors []string
}

func (vr *ValidationResult) AddError(format string, args ...interface{}) {
	vr.Errors = append(vr.Errors, fmt.Sprintf(format, args...))
}

func (vr *ValidationResult) HasErrors() bool {
	return len(vr.Errors) > 0
}

func (vr *ValidationResult) Error() string {
	if !vr.HasErrors() {
		return ""
	}
	result := "validation failed:"
	for _, e := range vr.Errors {
		result += "\n  - " + e
	}
	return result
}

func NewValidationResult() *ValidationResult {
	return &ValidationResult{
		Errors: make([]string, 0),
	}
}

func ValidateAll(descriptor PluginSchema) *ValidationResult {
	result := NewValidationResult()
	if err := descriptor.Validate(); err != nil {
		result.AddError("%v", err)
	}
	return result
}
