package render

import (
	"context"
	"fmt"
	"sync"

	"github.com/u-ai/backend/internal/desktoppet/plugin"
	"github.com/u-ai/backend/internal/gamehost/control"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/log"
)

const (
	PetRendererSinkID = "pet-renderer"
	CommandOutputKind = "command-output"
)

type PetRenderSink struct {
	runtimeID  string
	serviceID  string
	namespace  string
	sinkKind   control.ControlOutputKind
	outputGate *control.PluginOutputGate
	mu         sync.RWMutex
}

func NewPetRenderSink(runtimeID, serviceID string, outputGate *control.PluginOutputGate) (*PetRenderSink, error) {
	if runtimeID == "" {
		return nil, fmt.Errorf("pet render sink: runtimeID is required")
	}
	if serviceID == "" {
		return nil, fmt.Errorf("pet render sink: serviceID is required")
	}
	if outputGate == nil {
		return nil, fmt.Errorf("pet render sink: outputGate is required")
	}
	return &PetRenderSink{
		runtimeID:  runtimeID,
		serviceID:  serviceID,
		namespace:  plugin.PetSupportedProtocol,
		sinkKind:   control.KindCustomRPC,
		outputGate: outputGate,
	}, nil
}

func (s *PetRenderSink) Namespace() string {
	return s.namespace
}

func (s *PetRenderSink) SinkID() string {
	return PetRendererSinkID
}

func (s *PetRenderSink) Kind() control.ControlOutputKind {
	return s.sinkKind
}

func (s *PetRenderSink) Validate() error {
	if s.sinkKind == "" {
		return fmt.Errorf("pet render sink: sink kind must not be empty")
	}
	if s.sinkKind == CommandOutputKind {
		return fmt.Errorf("pet render sink: pet renderer must not use command-output kind, use a real renderer kind instead")
	}
	if s.namespace != plugin.PetSupportedProtocol {
		return fmt.Errorf("pet render sink: invalid namespace %s, expected %s", s.namespace, plugin.PetSupportedProtocol)
	}
	return nil
}

func (s *PetRenderSink) Dispatch(ctx context.Context, payload []byte, epoch uint64, generation uint64, svcID domain.ServiceID, pluginID domain.PluginID) error {
	s.mu.RLock()
	gate := s.outputGate
	s.mu.RUnlock()

	if gate == nil {
		return fmt.Errorf("pet render sink: output gate not configured")
	}

	intent := control.ControlOutputIntent{
		OutputID:       fmt.Sprintf("pet-render-%s-%d", s.runtimeID, epoch),
		RuntimeID:      domain.RuntimeInstanceID(s.runtimeID),
		ServiceID:      svcID,
		AuthorityEpoch: epoch,
		Kind:           s.sinkKind,
		Payload:        payload,
	}

	peer := control.TrustedPluginIdentity{
		PluginID:   pluginID,
		RuntimeID:  domain.RuntimeInstanceID(s.runtimeID),
		ServiceID:  svcID,
		Generation: generation,
	}

	req := control.OutputCheckRequest{
		Intent:  intent,
		Peer:    peer,
		Payload: payload,
	}

	decision, permit := gate.Check(ctx, req)
	if decision.Deny() {
		return fmt.Errorf("pet render sink: output denied reason=%s", decision.Reason)
	}
	if permit == nil {
		return fmt.Errorf("pet render sink: no permit returned")
	}
	log.Logger.Debugf("pet render sink: dispatched runtimeID=%s epoch=%d permit=%s", s.runtimeID, epoch, permit.PermitID)
	return nil
}

func (s *PetRenderSink) SetOutputGate(gate *control.PluginOutputGate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.outputGate = gate
}

func (s *PetRenderSink) GetRuntimeID() string {
	return s.runtimeID
}

func (s *PetRenderSink) GetServiceID() string {
	return s.serviceID
}
