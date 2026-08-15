package secret

type SubscriptionAdapter struct {
	inner *SecretLeaseAdapter
	orc   *LifecycleOrchestrator
}

func NewSubscriptionAdapter(inner *SecretLeaseAdapter) *SubscriptionAdapter {
	return &SubscriptionAdapter{inner: inner}
}

func NewSubscriptionAdapterWithOrchestrator(inner *SecretLeaseAdapter, orc *LifecycleOrchestrator) *SubscriptionAdapter {
	return &SubscriptionAdapter{inner: inner, orc: orc}
}

func (s *SubscriptionAdapter) OnPermissionRevoked(extensionID, runtimeID string) {
	if extensionID == "" && runtimeID == "" {
		return
	}
	if runtimeID != "" {
		if s.orc != nil {
			s.revokeRuntimeSessions(runtimeID, "permission revoked")
			return
		}
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
	if s.orc != nil {
		for _, sess := range s.orc.SessionsForService(runtimeID, serviceID) {
			s.orc.RevokeSession(sess.SessionID, "service stopped")
		}
		return
	}
	s.inner.RevokeServiceLeases(runtimeID, serviceID, "service stopped")
}

func (s *SubscriptionAdapter) OnRuntimeStopped(runtimeID string) {
	if runtimeID == "" {
		return
	}
	if s.orc != nil {
		s.revokeRuntimeSessions(runtimeID, "runtime stopped")
		return
	}
	s.inner.RevokeRuntimeLeases(runtimeID, "runtime stopped")
}

func (s *SubscriptionAdapter) OnRuntimeRestarted(runtimeID string) {
	if runtimeID == "" {
		return
	}
	if s.orc != nil {
		s.revokeRuntimeSessions(runtimeID, "runtime restarted")
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

func (s *SubscriptionAdapter) OnServicePermissionRevoked(runtimeID, serviceID string, generation int64) {
	if runtimeID == "" || serviceID == "" {
		return
	}
	if s.orc != nil {
		for _, sess := range s.orc.SessionsForService(runtimeID, serviceID) {
			if generation <= 0 || sess.Generation == generation {
				s.orc.RevokeSession(sess.SessionID, "service permission revoked")
			}
		}
		return
	}
	if generation > 0 {
		s.inner.RevokeRuntimeGenerationLeases(runtimeID, generation, "service permission revoked")
		return
	}
	s.inner.RevokeServiceLeases(runtimeID, serviceID, "service permission revoked")
}

func (s *SubscriptionAdapter) revokeRuntimeSessions(runtimeID, reason string) {
	s.inner.RevokeRuntimeLeases(runtimeID, reason)
}
