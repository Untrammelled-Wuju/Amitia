package plugin_boundary

func DefaultAdapters() []ContributionAdapter {
	return defaultAdapters()
}

func NewBoundaryWith(source KernelContributionSource, adapters []ContributionAdapter) *DesktopPetPluginBoundary {
	reconciler := NewReconciler(source, adapters)
	return NewDesktopPetPluginBoundary(reconciler)
}

func NewBoundary(source KernelContributionSource) *DesktopPetPluginBoundary {
	return NewBoundaryWith(source, defaultAdapters())
}
