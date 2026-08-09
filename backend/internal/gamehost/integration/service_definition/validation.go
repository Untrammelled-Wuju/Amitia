package service_definition

import (
	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
)

type DefinitionValidationService struct {
	provider ServiceDefinitionBatchProvider
	mapper   *DefinitionMapper
}

func NewDefinitionValidationService(provider ServiceDefinitionBatchProvider, mapper *DefinitionMapper) (*DefinitionValidationService, error) {
	if provider == nil {
		return nil, &ServiceDefinitionError{
			Code:    ErrDefinitionValidationFailed,
			Message: "provider is nil",
		}
	}
	if mapper == nil {
		mapper = NewDefinitionMapper()
	}
	return &DefinitionValidationService{
		provider: provider,
		mapper:   mapper,
	}, nil
}

func (v *DefinitionValidationService) ValidateForRegistration(view ServiceRuntimeView) error {
	return validateView(view)
}

func (v *DefinitionValidationService) ValidateExisting(definitionID string) (*trusted_service.ServiceRuntimeDefinition, error) {
	if definitionID == "" {
		return nil, &ServiceDefinitionError{
			Code:    ErrDefinitionValidationFailed,
			Message: "definition id is empty",
		}
	}
	def, err := v.provider.GetForService(definitionID)
	if err != nil {
		return nil, &ServiceDefinitionError{
			Code:    ErrDefinitionProviderUnavailable,
			Message: "failed to retrieve definition",
			Cause:   err,
		}
	}
	if def == nil {
		return nil, &ServiceDefinitionError{
			Code:    ErrDefinitionValidationFailed,
			Message: "definition not found: " + definitionID,
		}
	}
	return def, nil
}

func (v *DefinitionValidationService) ValidateAll() (*ValidationReport, error) {
	report := &ValidationReport{}

	all := v.provider.ListAll()
	for _, def := range all {
		if def.ServiceID == "" {
			report.InvalidCount++
			report.Errors = append(report.Errors, ServiceDefinitionError{
				Code:    ErrDefinitionValidationFailed,
				Message: "definition has empty service id",
			})
			continue
		}
		if def.ExtensionID == "" {
			report.InvalidCount++
			report.Errors = append(report.Errors, ServiceDefinitionError{
				Code:    ErrDefinitionValidationFailed,
				Message: "definition has empty extension id: " + def.ServiceID,
			})
			continue
		}
		if len(def.Executables) == 0 {
			report.InvalidCount++
			report.Errors = append(report.Errors, ServiceDefinitionError{
				Code:    ErrDefinitionValidationFailed,
				Message: "definition has no executables: " + def.ServiceID,
			})
			continue
		}
		report.ValidCount++
	}

	report.IsValid = report.InvalidCount == 0
	return report, nil
}

type ValidationReport struct {
	IsValid      bool                    `json:"isValid"`
	ValidCount   int                     `json:"validCount"`
	InvalidCount int                     `json:"invalidCount"`
	Errors       []ServiceDefinitionError `json:"errors,omitempty"`
}
