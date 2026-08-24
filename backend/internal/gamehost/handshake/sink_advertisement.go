package handshake

import "github.com/u-ai/backend/internal/gamehost/domain"

func ValidateSinkAdvertisements(sinks []SinkAdvertisement) error {
	seen := make(map[string]struct{}, len(sinks))
	for _, sink := range sinks {
		if sink.SinkID == "" || sink.ServiceID == "" || sink.Kind == "" {
			return NewHandshakeError(HandshakeErrorCapabilityMismatch, domain.ErrInvalidArgument, "sink advertisement requires sinkId, serviceId and kind")
		}
		if sink.Kind != "effect" {
			return NewHandshakeError(HandshakeErrorCapabilityMismatch, domain.ErrInvalidArgument, "unsupported sink kind: "+sink.Kind)
		}
		key := sink.ServiceID + "/" + sink.SinkID
		if _, exists := seen[key]; exists {
			return NewHandshakeError(HandshakeErrorCapabilityMismatch, domain.ErrInvalidArgument, "duplicate sink advertisement: "+key)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validateDeclaredSinks(provider DescriptorProvider, pluginID string, advertised []SinkAdvertisement) error {
	if err := ValidateSinkAdvertisements(advertised); err != nil {
		return err
	}
	if len(advertised) == 0 {
		return nil
	}
	if provider == nil {
		return NewHandshakeError(HandshakeErrorCapabilityMismatch, domain.ErrInvalidArgument, "sink advertisement requires a package descriptor provider")
	}
	declared, err := provider.DescriptorControlSinks(pluginID)
	if err != nil {
		return NewHandshakeError(HandshakeErrorCapabilityMismatch, domain.ErrInternal, "failed to query descriptor control sinks")
	}
	allowed := make(map[string]domain.ControlSinkDeclaration, len(declared))
	for _, sink := range declared {
		allowed[string(sink.ServiceID)+"/"+sink.ID] = sink
	}
	for _, sink := range advertised {
		decl, ok := allowed[sink.ServiceID+"/"+sink.SinkID]
		if !ok || decl.Kind != sink.Kind {
			return NewHandshakeError(HandshakeErrorCapabilityMismatch, domain.ErrInvalidArgument, "control sink not declared in package descriptor: "+sink.ServiceID+"/"+sink.SinkID)
		}
	}
	return nil
}
