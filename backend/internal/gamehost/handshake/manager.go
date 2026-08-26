package handshake

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/ipc"
)

type HandshakeManager struct {
	mu              sync.Mutex
	states          map[string]*stateCell
	snapshots       map[string]*HandshakeSnapshot
	namespaceOwners map[string]ipc.Peer

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
		namespaceOwners:      make(map[string]ipc.Peer),
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
	if m == nil || id == "" {
		return
	}
	m.mu.Lock()
	state := m.states[id]
	owner, hasOwner := m.namespaceOwners[id]
	delete(m.states, id)
	delete(m.snapshots, id)
	delete(m.namespaceOwners, id)
	m.mu.Unlock()

	if state != nil {
		state.transitionAlways(HandshakeStateClosed)
	}
	if hasOwner && m.namespaceAdapter != nil {
		_ = m.namespaceAdapter.RemoveConnection(
			context.Background(),
			string(owner.RuntimeID),
			string(owner.ServiceID),
			id,
		)
	}
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
	return m.snapshots[id].Clone()
}

// HasNegotiatedCapability reports whether a ready connection negotiated the
// requested GameHost v1 feature. Runtime handlers use this as the execution
// gate so a package declaration alone cannot activate a feature that the
// concrete service connection did not advertise during handshake.
func (m *HandshakeManager) HasNegotiatedCapability(connectionID string, feature domain.Capability) bool {
	if m == nil || connectionID == "" || !m.IsReady(connectionID) {
		return false
	}
	snapshot := m.GetSnapshot(connectionID)
	return snapshot != nil && snapshot.HasCapability(feature)
}

// HasNegotiatedChannel reports whether a fully-ready connection advertised the
// channel during hello. Runtime delivery gates use this in addition to the
// descriptor/permission boundary so declaration alone cannot activate traffic.
func (m *HandshakeManager) HasNegotiatedChannel(connectionID string, channelID domain.ChannelID) bool {
	if m == nil || connectionID == "" || channelID == "" || !m.IsReady(connectionID) {
		return false
	}
	snapshot := m.GetSnapshot(connectionID)
	return snapshot != nil && snapshot.HasChannel(string(channelID))
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

	response, err := m.process(ctx, connID, peer, hello)
	if err != nil {
		state.transitionAlways(HandshakeStateRejected)
		return nil, err
	}

	snap := m.buildSnapshot(hello, response)
	if !m.stage(connID, state, snap, peer) {
		if m.namespaceAdapter != nil {
			_ = m.namespaceAdapter.RemoveConnection(context.Background(), string(peer.RuntimeID), string(peer.ServiceID), connID)
		}
		state.transitionAlways(HandshakeStateRejected)
		return nil, NewHandshakeError(HandshakeErrorRequired, domain.ErrInvalidState, "handshake could not be staged")
	}

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
	connID string,
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

	if err := validateNegotiatedChannelFeatures(m.descriptorProvider, string(peer.PluginID), string(peer.ServiceID), hello.Channels, negotiatedCaps); err != nil {
		return nil, err
	}

	if m.channelAdvertiser != nil {
		if err := m.channelAdvertiser.ValidateChannelAdvertisement(string(peer.PluginID), string(peer.ServiceID), hello.Channels); err != nil {
			return nil, err
		}
	}

	// Namespace routing is the only handshake step that mutates shared host state.
	// Apply it last, after every other validation has succeeded, and reconcile even
	// an empty list so removed namespaces cannot survive an upgrade/reconnect.
	if m.namespaceAdapter == nil {
		if len(hello.RPCNamespaces) > 0 {
			return nil, NewHandshakeError(HandshakeErrorNamespaceInvalid, domain.ErrInternal, "namespace registry is unavailable")
		}
	} else {
		if _, err := m.namespaceAdapter.Apply(
			ctx,
			connID,
			string(peer.PluginID),
			string(peer.RuntimeID),
			string(peer.ServiceID),
			hello.RPCNamespaces,
		); err != nil {
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
		required, known := domain.RequiredCapabilityForChannelKind(kind)
		if !known {
			return NewHandshakeError(
				HandshakeErrorCapabilityMismatch,
				domain.ErrInvalidArgument,
				"channel "+ad.ID+" has unsupported kind "+string(kind),
			)
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

func (m *HandshakeManager) stage(connID string, state *stateCell, snap *HandshakeSnapshot, peer ipc.Peer) bool {
	if state == nil || state.Get() != HandshakeStateHandshaking {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if current, ok := m.states[connID]; !ok || current != state || state.Get() != HandshakeStateHandshaking {
		return false
	}
	m.snapshots[connID] = snap
	m.namespaceOwners[connID] = peer
	return true
}

// ConfirmReady performs the second phase of the handshake. The connection is
// not reported ready until the IPC layer has successfully written the hello
// response to the plugin transport.
func (m *HandshakeManager) ConfirmReady(connID string) bool {
	if m == nil || connID == "" {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.states[connID]
	if !exists || state == nil || !state.compareAndSwap(HandshakeStateHandshaking, HandshakeStateReady) {
		return false
	}
	snapshot, exists := m.snapshots[connID]
	if !exists || snapshot == nil {
		state.transitionAlways(HandshakeStateRejected)
		return false
	}
	snapshot.ReadyAt = time.Now().UTC()
	return true
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
	if m == nil {
		return
	}
	m.mu.Lock()
	ids := make([]string, 0, len(m.states))
	for id := range m.states {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return
		}
		m.RemoveConnection(id)
	}
}
