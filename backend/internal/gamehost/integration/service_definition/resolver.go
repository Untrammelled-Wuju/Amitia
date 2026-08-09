package service_definition

import (
	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
)

type DefinitionResolver struct {
	provider ServiceDefinitionBatchProvider
}

func NewDefinitionResolver(provider ServiceDefinitionBatchProvider) (*DefinitionResolver, error) {
	if provider == nil {
		return nil, &ServiceDefinitionError{Code: ErrDefinitionValidationFailed, Message: "provider is nil"}
	}
	return &DefinitionResolver{
		provider: provider,
	}, nil
}

func (r *DefinitionResolver) HasDefinition(definitionID string) bool {
	if definitionID == "" {
		return false
	}
	return r.provider.HasDefinition(definitionID)
}

func (r *DefinitionResolver) Resolve(definitionID string) (*trusted_service.ServiceRuntimeDefinition, error) {
	if definitionID == "" {
		return nil, &ServiceDefinitionError{Code: ErrDefinitionInvalid, Message: "definition id must not be empty"}
	}

	def, err := r.provider.GetForService(definitionID)
	if err != nil {
		return nil, &ServiceDefinitionError{
			Code:    ErrServiceDefinitionNotFound,
			Message: "definition not found: " + definitionID,
			Cause:   err,
		}
	}
	if def == nil {
		return nil, &ServiceDefinitionError{
			Code:    ErrServiceDefinitionNotFound,
			Message: "definition not found: " + definitionID,
		}
	}
	return def, nil
}

func (r *DefinitionResolver) ResolveForService(serviceID string) (*trusted_service.ServiceRuntimeDefinition, []string, error) {
	if serviceID == "" {
		return nil, nil, &ServiceDefinitionError{Code: ErrDefinitionInvalid, Message: "service id must not be empty"}
	}

	def, err := r.provider.GetForService(serviceID)
	if err != nil {
		return nil, nil, nil
	}
	if def == nil {
		return nil, nil, nil
	}

	suggestion := "service:" + BuildServiceDefinitionID(def.ExtensionID, def.ModuleID)
	return def, []string{suggestion}, nil
}

func (r *DefinitionResolver) ListDefinitions() []*trusted_service.ServiceRuntimeDefinition {
	return r.provider.ListAll()
}

func (r *DefinitionResolver) Count() int {
	return len(r.provider.ListAll())
}

func (r *DefinitionResolver) ExtensionIDs() []string {
	seen := make(map[string]bool)
	var ids []string
	for _, def := range r.provider.ListAll() {
		if !seen[def.ExtensionID] {
			seen[def.ExtensionID] = true
			ids = append(ids, def.ExtensionID)
		}
	}
	return ids
}
