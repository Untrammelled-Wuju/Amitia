package plugincontract

func (m Manifest) ResolveServiceGraph() ServiceGraphResolution {
	request := ServiceGraphRequest{}
	for _, module := range m.Modules {
		moduleRef := ModuleRef{PluginID: m.Plugin.ID, ModuleID: module.ID}
		for _, offer := range module.Services.Provides {
			request.Providers = append(request.Providers, ServiceDescriptor{
				ProviderID:   moduleRef.Key() + "/" + offer.ServiceID,
				Module:       moduleRef,
				ServiceID:    offer.ServiceID,
				Version:      offer.Version,
				ContractHash: offer.ContractHash,
				Scopes:       append([]ServiceScope(nil), offer.Scopes...),
				Placement:    offer.Placement,
				Priority:     offer.Priority,
			})
		}
		if len(module.Services.Requires) > 0 {
			request.Consumers = append(request.Consumers, ServiceConsumer{
				Module:       moduleRef,
				Requirements: append([]ServiceRequirement(nil), module.Services.Requires...),
			})
		}
	}
	return (ServiceGraphResolver{}).Resolve(request)
}
