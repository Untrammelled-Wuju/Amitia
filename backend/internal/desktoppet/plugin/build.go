package plugin

import (
	"fmt"

	ghdomain "github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/integration/service_definition"
)

const (
	PetSupportedProtocol  = "amitia.desktop.pet/1"
	RendererServiceSuffix = "/renderer"
)

type PetServiceDescriptor struct {
	RuntimeID   string
	ExtensionID string
	ModuleID    string
	EntryPoint  string
	Env         map[string]string
}

func (d PetServiceDescriptor) ServiceID() string {
	return d.RuntimeID
}

func (d PetServiceDescriptor) RendererServiceID() string {
	return d.RuntimeID + RendererServiceSuffix
}

func (d PetServiceDescriptor) ToServiceRuntimeView() service_definition.ServiceRuntimeView {
	return service_definition.ServiceRuntimeView{
		ExtensionID: d.ExtensionID,
		ModuleID:    d.ModuleID,
		RuntimeType: service_definition.ServiceRuntimeType,
		Name:        "desktop-pet-renderer",
		Description: "Desktop Pet render surface registered by pet runtime",
		EntryPoint:  d.EntryPoint,
		Env:         d.Env,
		Enabled:     true,
	}
}

func (d PetServiceDescriptor) DefinitionID() string {
	return d.ToServiceRuntimeView().ToDefinitionID()
}

func (d PetServiceDescriptor) SinkID() string {
	return d.RuntimeID + "/output"
}

func (d PetServiceDescriptor) SinkNamespace() string {
	return PetSupportedProtocol
}

func (d PetServiceDescriptor) SinkKind() ghdomain.ServiceKind {
	return ghdomain.ServiceKindProcess
}

func (d PetServiceDescriptor) Validate() error {
	if d.RuntimeID == "" {
		return fmt.Errorf("pet service descriptor: runtimeID is required")
	}
	if d.ExtensionID == "" {
		return fmt.Errorf("pet service descriptor: extensionID is required")
	}
	if d.ModuleID == "" {
		return fmt.Errorf("pet service descriptor: moduleID is required")
	}
	if d.EntryPoint == "" {
		return fmt.Errorf("pet service descriptor: entryPoint is required")
	}
	return nil
}
