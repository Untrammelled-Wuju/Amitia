package kernel

import (
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/adapterprovider"
	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
)

func TestRegisterPackageAdapterProviders(t *testing.T) {
	registry, err := plugincontract.NewBuiltinAdapterRegistry()
	if err != nil {
		t.Fatal(err)
	}
	router := plugincontract.NewContributionRouter(registry)
	handler := adapterprovider.NewSupervisorHandler(registry, router)
	contract := &plugincontract.Manifest{
		AdapterProviders: []plugincontract.AdapterProviderDeclaration{{
			ServiceID:          "adapter-service",
			AdapterID:          "external.remote",
			ContractVersion:    1,
			SupportedKinds:     []string{"remote"},
			ContributionSchema: json.RawMessage(`{"type":"object"}`),
		}},
	}
	if err := registerPackageAdapterProviders(handler, contract, plugincontract.TrustVerified); err != nil {
		t.Fatal(err)
	}
	unregisterPackageAdapterProviders(handler, contract)
}
