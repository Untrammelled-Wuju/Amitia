package secret

type SubscriptionAdapter struct {
	inner *SecretLeaseAdapter
}

func NewSubscriptionAdapter(inner *SecretLeaseAdapter) *SubscriptionAdapter {
	return &SubscriptionAdapter{inner: inner}
}

func (s *SubscriptionAdapter) OnPermissionRevoked(extensionID, runtimeID string) {
	if extensionID == "" && runtimeID == "" {
		return
	}
	if runtimeID != "" {
		s.inner.RevokeRuntimeLeases(runtimeID, "permission revoked")
		return
	}
	s.inner.RevokeExtensionLeases(extensionID, "permission revoked")
}

func (s *SubscriptionAdapter) OnExtensionDisabled(extensionID string) {
	if extensionID == "" {
		return
	}
	s.inner.RevokeExtensionLeases(extensionID, "extension disabled")
}

func (s *SubscriptionAdapter) OnExtensionUninstalled(extensionID string) {
	if extensionID == "" {
		return
	}
	s.inner.RevokeExtensionLeases(extensionID, "extension uninstalled")
}

func (s *SubscriptionAdapter) OnServiceStopped(runtimeID, serviceID string) {
	if runtimeID == "" || serviceID == "" {
		return
	}
	s.inner.RevokeServiceLeases(runtimeID, serviceID, "service stopped")
}

func (s *SubscriptionAdapter) OnRuntimeStopped(runtimeID string) {
	if runtimeID == "" {
		return
	}
	s.inner.RevokeRuntimeLeases(runtimeID, "runtime stopped")
}

func (s *SubscriptionAdapter) OnRuntimeRestarted(runtimeID string) {
	if runtimeID == "" {
		return
	}
	s.inner.RevokeRuntimeLeases(runtimeID, "runtime restarted")
}

func (s *SubscriptionAdapter) OnRuntimeGenerationChanged(runtimeID string, oldGeneration int64) {
	if runtimeID == "" || oldGeneration <= 0 {
		return
	}
	s.inner.RevokeRuntimeGenerationLeases(runtimeID, oldGeneration, "generation changed")
}
