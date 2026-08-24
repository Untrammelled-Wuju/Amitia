package handshake

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/ipc"
)

type HandshakeManager struct {
	mu        sync.Mutex
	states    map[string]*stateCell
	snapshots map[string]*HandshakeSnapshot

	protocolNegotiator   *ProtocolNegotiator
	capabilityNegotiator *CapabilityNegotiator
	namespaceAdapter     NamespaceAdapter
	channelAdvertiser    ChannelAdvertiser
	runtimeValidator     RuntimeValidator
	descriptorProvider   DescriptorProvider

	timeout int64

	allowlist []string
}

type HandshakeManagerConfig struct {
	HostSupportedProtocols []string
	HostCapabilities       []domain.Capability
	NamespaceAdapter       NamespaceAdapter
	ChannelAdvertiser      ChannelAdvertiser
	RuntimeValidator       RuntimeValidator
	DescriptorProvider     DescriptorProvider

	PreReadyAllowlist []string
}

func NewHandshakeManager(config HandshakeManagerConfig) *HandshakeManager {
	pn := NewProtocolNegotiator(config.HostSupportedProtocols)
	cn := NewCapabilityNegotiator(config.HostCapabilities)

	return &HandshakeManager{
		states:               make(map[string]*stateCell),
		snapshots:            make(map[string]*HandshakeSnapshot),
		protocolNegotiator:   pn,
		capabilityNegotiator: cn,
		namespaceAdapter:     config.NamespaceAdapter,
		channelAdvertiser:    config.ChannelAdvertiser,
		runtimeValidator:     config.RuntimeValidator,
		descriptorProvider:   config.DescriptorProvider,
		allowlist:            append([]string{HelloMethod, "control.request.cancel"}, config.PreReadyAllowlist...),
	}
}

func (m *HandshakeManager) RegisterConnection(id string) *stateCell {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.states[id]; ok {
		return s
	}
	s := newStateCell(HandshakeStateAttached)
	m.states[id] = s
	return s
}

func (m *HandshakeManager) RemoveConnection(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.states, id)
	delete(m.snapshots, id)
}

func (m *HandshakeManager) GetState(id string) (HandshakeState, bool) {
	m.mu.Lock()
	s, ok := m.states[id]
	m.mu.Unlock()
	if !ok {
		return "", false
	}
	return s.Get(), true
}

func (m *HandshakeManager) GetSnapshot(id string) *HandshakeSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.snapshots[id]
}

// HasNegotiatedCapability reports whether a ready connection negotiated the
// requested GameHost v1 feature. Runtime handlers use this as the execution
// gate so a package declaration alone cannot activate a feature that the
// concrete service connection did not advertise during handshake.
func (m *HandshakeManager) HasNegotiatedCapability(connectionID string, feature domain.Capability) bool {
	if m == nil || connectionID == "" {
		return false
	}
	snapshot := m.GetSnapshot(connectionID)
	return snapshot != nil && snapshot.HasCapability(feature)
}

func (m *HandshakeManager) IsReady(id string) bool {
	m.mu.Lock()
	s, ok := m.states[id]
	m.mu.Unlock()
	if !ok {
		return false
	}
	return s.Get() == HandshakeStateReady
}

func (m *HandshakeManager) HandleHello(
	ctx context.Context,
	connID string,
	peer ipc.Peer,
	hello *HelloRequest,
) (*HelloResponse, error) {
	state := m.getStateCell(connID)
	if state == nil {
		return nil, NewHandshakeError(
			HandshakeErrorRequired,
			domain.ErrNotFound,
			"connection not registered",
		)
	}

	if !state.compareAndSwap(HandshakeStateAttached, HandshakeStateHandshaking) {
		return nil, NewHandshakeError(
			HandshakeErrorAlreadyCompleted,
			domain.ErrAlreadyExists,
			"handshake already completed for this connection",
		)
	}

	if err := m.preValidate(peer, hello); err != nil {
		state.transitionAlways(HandshakeStateRejected)
		return nil, err
	}

	response, err := m.process(ctx, peer, hello)
	if err != nil {
		state.transitionAlways(HandshakeStateRejected)
		return nil, err
	}

	snap := m.buildSnapshot(hello, response)
	m.commit(connID, state, snap)

	return response, nil
}

func (m *HandshakeManager) HandleHelloFromEnvelope(
	ctx context.Context,
	connID string,
	peer ipc.Peer,
	payload json.RawMessage,
) (*HelloResponse, error) {
	var hello HelloRequest
	if err := json.Unmarshal(payload, &hello); err != nil {
		return nil, NewHandshakeError(
			HandshakeErrorProtocolMismatch,
			domain.ErrInvalidArgument,
			"invalid hello payload: "+err.Error(),
		)
	}
	return m.HandleHello(ctx, connID, peer, &hello)
}

func (m *HandshakeManager) process(
	ctx context.Context,
	peer ipc.Peer,
	hello *HelloRequest,
) (*HelloResponse, error) {
	if err := ValidateProtocols(hello.SupportedProtocols); err != nil {
		return nil, err
	}

	if err := ValidateCapabilities(hello.Capabilities); err != nil {
		return nil, err
	}

	if err := ValidateChannelAdvertisements(hello.Channels); err != nil {
		return nil, err
	}

	if err := validateDeclaredSinks(m.descriptorProvider, string(peer.PluginID), hello.Sinks); err != nil {
		return nil, err
	}

	negotiatedProtocol, err := m.protocolNegotiator.Negotiate(hello.SupportedProtocols)
	if err != nil {
		return nil, err
	}

	advertised := make([]domain.Capability, 0, len(hello.Capabilities))
	for _, c := range hello.Capabilities {
		advertised = append(advertised, domain.Capability(c))
	}

	descriptorCaps := m.descriptorCaps(peer.PluginID)
	negotiatedCaps, err := m.capabilityNegotiator.Negotiate(descriptorCaps, advertised)
	if err != nil {
		return nil, err
	}

	var allowCustomRPC bool
	for _, c := range negotiatedCaps {
		if c == domain.CapabilityCustomRPC {
			allowCustomRPC = true
			break
		}
	}

	if len(hello.RPCNamespaces) > 0 && !allowCustomRPC {
		return nil, NewHandshakeError(
			HandshakeErrorCapabilityMismatch,
			domain.ErrInvalidArgument,
			"rpcNegotiation requires custom_rpc capability",
		)
	}

	if m.namespaceAdapter != nil && len(hello.RPCNamespaces) > 0 {
		_, err := m.namespaceAdapter.Apply(ctx, string(peer.PluginID), string(peer.RuntimeID), string(peer.ServiceID), hello.RPCNamespaces)
		if err != nil {
			return nil, err
		}
	}

	if err := validateNegotiatedChannelFeatures(m.descriptorProvider, string(peer.PluginID), string(peer.ServiceID), hello.Channels, negotiatedCaps); err != nil {
		return nil, err
	}

	if m.channelAdvertiser != nil {
		if err := m.channelAdvertiser.ValidateChannelAdvertisement(string(peer.PluginID), string(peer.ServiceID), hello.Channels); err != nil {
			return nil, err
		}
	}

	negotiatedCapStrings := make([]string, 0, len(negotiatedCaps))
	for _, c := range negotiatedCaps {
		negotiatedCapStrings = append(negotiatedCapStrings, string(c))
	}

	ns := make([]string, len(hello.RPCNamespaces))
	copy(ns, hello.RPCNamespaces)

	channelIDs := make([]string, 0, len(hello.Channels))
	for _, ch := range hello.Channels {
		channelIDs = append(channelIDs, ch.ID)
	}

	return &HelloResponse{
		Protocol:      negotiatedProtocol,
		Capabilities:  negotiatedCapStrings,
		RPCNamespaces: ns,
		Channels:      channelIDs,
	}, nil
}

func validateNegotiatedChannelFeatures(provider DescriptorProvider, pluginID, serviceID string, advertised []ChannelAdvertisement, negotiated []domain.Capability) error {
	if len(advertised) == 0 {
		return nil
	}
	if provider == nil {
		return NewHandshakeError(HandshakeErrorCapabilityMismatch, domain.ErrPermissionDenied, "channel feature validation requires a plugin descriptor")
	}
	declared, err := provider.DescriptorChannelDescriptors(pluginID, serviceID)
	if err != nil {
		return NewHandshakeError(HandshakeErrorCapabilityMismatch, domain.ErrPermissionDenied, "failed to resolve declared channel features")
	}
	byID := make(map[string]domain.ChannelKind, len(declared))
	for _, ch := range declared {
		byID[string(ch.ID)] = ch.Kind
	}
	negotiatedSet := make(map[domain.Capability]struct{}, len(negotiated))
	for _, feature := range negotiated {
		negotiatedSet[feature] = struct{}{}
	}
	for _, ad := range advertised {
		kind, ok := byID[ad.ID]
		if !ok {
			continue // declaration membership is rejected by ChannelAdvertiser below
		}
		required := domain.CapabilityEventStreaming
		if kind == domain.ChannelKindState {
			required = domain.CapabilityStateStreaming
		}
		if _, ok := negotiatedSet[required]; !ok {
			return NewHandshakeError(
				HandshakeErrorCapabilityMismatch,
				domain.ErrPermissionDenied,
				"channel "+ad.ID+" requires negotiated capability "+string(required),
			)
		}
	}
	return nil
}

func (m *HandshakeManager) preValidate(peer ipc.Peer, hello *HelloRequest) error {
	if m.runtimeValidator != nil {
		exists, err := m.runtimeValidator.RuntimeExists(string(peer.RuntimeID))
		if err != nil {
			return NewHandshakeError(
				HandshakeErrorRuntimeNotFound,
				domain.ErrInternal,
				"failed to validate runtime",
			)
		}
		if !exists {
			return NewHandshakeError(
				HandshakeErrorRuntimeNotFound,
				domain.ErrNotFound,
				"runtime does not exist: "+string(peer.RuntimeID),
			)
		}

		if err := m.runtimeValidator.ServiceBelongsToRuntime(
			string(peer.RuntimeID),
			string(peer.ServiceID),
			string(peer.PluginID),
		); err != nil {
			return NewHandshakeError(
				HandshakeErrorServiceNotFound,
				domain.ErrNotFound,
				"service does not belong to runtime: "+err.Error(),
			)
		}
	}
	return nil
}

func (m *HandshakeManager) buildSnapshot(
	hello *HelloRequest,
	response *HelloResponse,
) *HandshakeSnapshot {
	caps := make([]domain.Capability, 0, len(response.Capabilities))
	for _, c := range response.Capabilities {
		caps = append(caps, domain.Capability(c))
	}

	ns := make([]string, len(response.RPCNamespaces))
	copy(ns, response.RPCNamespaces)

	channels := make([]string, len(response.Channels))
	copy(channels, response.Channels)

	var sdkName, sdkVersion string
	if hello.SDK != nil {
		sdkName = hello.SDK.Name
		sdkVersion = hello.SDK.Version
	}

	return NewHandshakeSnapshot(
		response.Protocol,
		caps,
		ns,
		channels,
		sdkName,
		sdkVersion,
	)
}

func (m *HandshakeManager) commit(connID string, state *stateCell, snap *HandshakeSnapshot) {
	m.mu.Lock()
	m.snapshots[connID] = snap
	m.mu.Unlock()

	if !state.compareAndSwap(HandshakeStateHandshaking, HandshakeStateReady) {
		m.mu.Lock()
		delete(m.snapshots, connID)
		m.mu.Unlock()
	}
}

func (m *HandshakeManager) descriptorCaps(pluginID domain.PluginID) []domain.Capability {
	// Package declarations are the authorization boundary for host features.
	// Missing/unreadable declarations must fail closed rather than inheriting the
	// complete host feature set.
	if m.descriptorProvider == nil {
		return nil
	}
	caps, err := m.descriptorProvider.DescriptorCapabilities(string(pluginID))
	if err != nil || len(caps) == 0 {
		return nil
	}
	result := make([]domain.Capability, 0, len(caps))
	for _, c := range caps {
		feature := domain.Capability(c)
		if domain.ValidateCapability(feature) == nil {
			result = append(result, feature)
		}
	}
	return result
}

func (m *HandshakeManager) getStateCell(connID string) *stateCell {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.states[connID]
	if !ok {
		return nil
	}
	return s
}

func (m *HandshakeManager) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.states)
}

func (m *HandshakeManager) Shutdown(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.states {
		s.transitionAlways(HandshakeStateClosed)
	}
}
