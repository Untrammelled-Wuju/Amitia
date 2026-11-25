package plugincontract

import "github.com/u-ai/backend/internal/devicemesh/business"

const CoreMemorySearchServiceID = "memory.search"

func CoreMemorySearchSpec() CoreCapabilitySpec {
	return CoreCapabilitySpec{
		ServiceID: CoreMemorySearchServiceID,
		Version:   CoreDescriptorVersion,
		Summary:   "Search memories owned by the current owner scope",
		Tags:      []string{"memory", "read", "search"},
		Scopes:    []ServiceScope{ServiceScopeDevice, ServiceScopeConversation},
		Permissions: []DescriptorPermission{
			{Capability: "memory.read", Risk: "low"},
		},
		Placement: ServicePlacementDevice,
		Input:     business.MemoryManagementQuery{},
		Output:    business.MemoryManagementResponse{},
	}
}

func DefaultCoreDescriptorCatalog() (CoreDescriptorCatalog, error) {
	return BuildCoreDescriptorCatalog([]CoreCapabilitySpec{CoreMemorySearchSpec()})
}
