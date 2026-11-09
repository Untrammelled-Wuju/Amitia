package kernel

import (
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/adapterprovider"
	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
)

func registerPackageAdapterProviders(
	handler *adapterprovider.SupervisorHandler,
	contract *plugincontract.Manifest,
	trust plugincontract.TrustLevel,
) error {
	if handler == nil || contract == nil {
		return nil
	}
	for _, declaration := range contract.AdapterProviders {
		registration := adapterprovider.ProviderRegistration{
			ServiceID: declaration.ServiceID,
			Descriptor: plugincontract.AdapterDescriptor{
				ID:                       declaration.AdapterID,
				ContractVersion:          declaration.ContractVersion,
				SupportedKinds:           append([]string(nil), declaration.SupportedKinds...),
				Operations:               append([]string(nil), declaration.Operations...),
				ContributionSchema:       append([]byte(nil), declaration.ContributionSchema...),
				RequiredHostCapabilities: append([]string(nil), declaration.RequiredHostCapabilities...),
				PermissionRequirements:   append([]string(nil), declaration.PermissionRequirements...),
				Platforms:                append([]string(nil), declaration.Platforms...),
				Trust:                    trust,
				External:                 true,
			},
		}
		if err := handler.ReplaceProvider(registration); err != nil {
			return fmt.Errorf("register adapter provider %s: %w", declaration.AdapterID, err)
		}
	}
	return nil
}

func unregisterPackageAdapterProviders(handler *adapterprovider.SupervisorHandler, contract *plugincontract.Manifest) {
	if handler == nil || contract == nil {
		return
	}
	for _, declaration := range contract.AdapterProviders {
		handler.UnregisterProvider(declaration.ServiceID)
	}
}
