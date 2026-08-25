package service_definition

import (
	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
)

type ServiceRuntimeView struct {
	ExtensionID         string
	ModuleID            string
	RuntimeType         string
	Name                string
	Description         string
	PublisherID         string
	PublisherTrust      string
	EntryPoint          string
	ExecutablePath      string
	ExecutableSHA256    string
	Arguments           []string
	IntegrityValue      string
	Dependencies        []trusted_service.LibraryDep
	SandboxReadOnlyRoot string
	Env                 map[string]string
	Metadata            map[string]string
	Network             trusted_service.ServiceNetworkPolicy
	Limits              trusted_service.ServiceResourceLimits
	Enabled             bool
	ExtensionState      string
}

func (v ServiceRuntimeView) ToDefinitionID() string {
	return BuildServiceDefinitionID(v.ExtensionID, v.ModuleID)
}

func (v ServiceRuntimeView) IsValidProcessService() bool {
	return IsValidServiceRuntimeType(v.RuntimeType) && v.EntryPoint != ""
}

type ServiceDefinitionSource interface {
	GetExtensionIDs() ([]string, error)
	GetServiceViewsByExtension(extensionID string) ([]ServiceRuntimeView, error)
}

type ServiceDefinitionBatchProvider interface {
	Register(def *trusted_service.ServiceRuntimeDefinition) error
	Remove(definitionID string) error
	HasDefinition(definitionID string) bool
	ListAll() []*trusted_service.ServiceRuntimeDefinition
	ListByExtension(extensionID string) []*trusted_service.ServiceRuntimeDefinition
	GetForService(serviceID string) (*trusted_service.ServiceRuntimeDefinition, error)
}

type ServiceDefinitionProvider = ServiceDefinitionBatchProvider
