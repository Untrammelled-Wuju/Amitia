package secret

import (
	kernelsecret "github.com/u-ai/backend/internal/extension/kernel/secret"
)

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

func (s *SubscriptionAdapter) RevokeGrant(leaseID kernelsecret.LeaseID) RevokeOutcome {
	entry, ok := s.inner.index.LookupByLease(leaseID)
	if !ok {
		return RevokeOutcome{RevokedCount: 0, RequestedBy: string(leaseID), Reason: "grant not found"}
	}
	extensionID := entry.ExtensionID
	runtimeID := entry.RuntimeID
	if err := s.inner.RevokeLease(leaseID, "permission revoked"); err != nil {
		return RevokeOutcome{RevokedCount: 0, RequestedBy: string(leaseID), Reason: err.Error()}
	}
	s.OnPermissionRevoked(extensionID, runtimeID)
	return RevokeOutcome{RevokedCount: 1, RequestedBy: string(leaseID), Reason: "permission revoked"}
}

func (s *SubscriptionAdapter) RevokeBySubject(subject string) RevokeOutcome {
	if subject == "" {
		return RevokeOutcome{RevokedCount: 0, RequestedBy: subject, Reason: "empty subject"}
	}
	if s.orc != nil {
		sessions := s.orc.SessionsForRuntime(subject)
		revoked := 0
		for _, sess := range sessions {
			outcome := s.orc.RevokeSession(sess.SessionID, "subject revoked")
			revoked += outcome.RevokedCount
		}
		return RevokeOutcome{RevokedCount: revoked, RequestedBy: subject, Reason: "subject revoked"}
	}
	return s.inner.RevokeRuntimeLeases(subject, "subject revoked")
}

func (s *SubscriptionAdapter) RevokeByExtension(extensionID string) RevokeOutcome {
	if extensionID == "" {
		return RevokeOutcome{RevokedCount: 0, RequestedBy: extensionID, Reason: "empty extension"}
	}
	if s.orc != nil {
		sessions := s.orc.SessionsForExtension(extensionID)
		revoked := 0
		for _, sess := range sessions {
			outcome := s.orc.RevokeSession(sess.SessionID, "extension revoked")
			revoked += outcome.RevokedCount
		}
		return RevokeOutcome{RevokedCount: revoked, RequestedBy: extensionID, Reason: "extension revoked"}
	}
	return s.inner.RevokeExtensionLeases(extensionID, "extension revoked")
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
