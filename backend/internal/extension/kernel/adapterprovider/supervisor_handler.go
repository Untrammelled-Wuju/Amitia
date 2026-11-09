package adapterprovider

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
)

type ProviderRegistration struct {
	ServiceID  string
	Descriptor plugincontract.AdapterDescriptor
}

type SupervisorHandler struct {
	registry *plugincontract.AdapterRegistry
	router   *plugincontract.ContributionRouter

	mu            sync.Mutex
	registrations map[string]ProviderRegistration
	active        map[string]*providerSession
	routes        map[string]*switchingProvider
	sessions      map[string]map[*providerSession]struct{}
}

func NewSupervisorHandler(registry *plugincontract.AdapterRegistry, router *plugincontract.ContributionRouter) *SupervisorHandler {
	return &SupervisorHandler{
		registry:      registry,
		router:        router,
		registrations: make(map[string]ProviderRegistration),
		active:        make(map[string]*providerSession),
		routes:        make(map[string]*switchingProvider),
		sessions:      make(map[string]map[*providerSession]struct{}),
	}
}

func (h *SupervisorHandler) RegisterProvider(registration ProviderRegistration) error {
	if h == nil {
		return fmt.Errorf("adapterprovider: supervisor handler is nil")
	}
	if err := validateProviderRegistration(registration); err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, exists := h.registrations[registration.ServiceID]; exists {
		return fmt.Errorf("adapterprovider: service %s is already registered", registration.ServiceID)
	}
	h.registrations[registration.ServiceID] = cloneProviderRegistration(registration)
	return nil
}

func (h *SupervisorHandler) UnregisterProvider(serviceID string) {
	if h == nil {
		return
	}
	h.mu.Lock()
	registration, hasRegistration := h.registrations[serviceID]
	switcher := h.routes[serviceID]
	delete(h.registrations, serviceID)
	delete(h.active, serviceID)
	delete(h.routes, serviceID)
	sessions := make([]*providerSession, 0, len(h.sessions[serviceID]))
	for session := range h.sessions[serviceID] {
		session.closed = true
		session.retired = true
		sessions = append(sessions, session)
	}
	delete(h.sessions, serviceID)
	if switcher != nil {
		switcher.current = nil
	}
	h.mu.Unlock()
	if switcher != nil {
		h.router.Unregister(switcher.descriptor.ID, switcher.descriptor.ContractVersion)
		_ = h.registry.Unregister(switcher.descriptor.ID, switcher.descriptor.ContractVersion)
	} else if hasRegistration {
		h.router.Unregister(registration.Descriptor.ID, registration.Descriptor.ContractVersion)
		_ = h.registry.Unregister(registration.Descriptor.ID, registration.Descriptor.ContractVersion)
	}
	for _, session := range sessions {
		_ = session.closeTransport()
	}
}

func (h *SupervisorHandler) ReplaceProvider(registration ProviderRegistration) error {
	if h == nil {
		return fmt.Errorf("adapterprovider: supervisor handler is nil")
	}
	if err := validateProviderRegistration(registration); err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if active := h.active[registration.ServiceID]; active != nil {
		previous := h.registrations[registration.ServiceID]
		if !sameAdapterIdentity(previous.Descriptor, registration.Descriptor) {
			return fmt.Errorf("adapterprovider: service %s cannot change adapter identity while active", registration.ServiceID)
		}
	}
	h.registrations[registration.ServiceID] = cloneProviderRegistration(registration)
	return nil
}

func (h *SupervisorHandler) Attach(
	ctx context.Context,
	meta trusted_service.StdioProtocolSessionMeta,
	stdin io.WriteCloser,
	stdout io.ReadCloser,
) (io.Closer, error) {
	if h == nil || h.registry == nil || h.router == nil {
		return nil, fmt.Errorf("adapterprovider: registry and router are required")
	}
	h.mu.Lock()
	registration, exists := h.registrations[meta.ServiceID]
	h.mu.Unlock()
	if !exists {
		return nil, fmt.Errorf("adapterprovider: service %s has no adapter registration", meta.ServiceID)
	}
	descriptor := registration.Descriptor
	descriptor.External = true
	transport := plugincontract.NewStdioAdapterProviderTransport(stdout, stdin)
	provider := &plugincontract.ExternalAdapterProvider{Descriptor: descriptor, Transport: transport}
	if err := provider.Health(ctx); err != nil {
		_ = transport.Close()
		return nil, err
	}

	h.mu.Lock()
	switcher, err := h.ensureSwitcherLocked(meta.ServiceID, descriptor)
	if err != nil {
		h.mu.Unlock()
		_ = transport.Close()
		return nil, err
	}
	previous := h.active[meta.ServiceID]
	session := &providerSession{
		handler:   h,
		serviceID: meta.ServiceID,
		switcher:  switcher,
		provider:  provider,
		transport: transport,
	}
	h.active[meta.ServiceID] = session
	if h.sessions[meta.ServiceID] == nil {
		h.sessions[meta.ServiceID] = make(map[*providerSession]struct{})
	}
	h.sessions[meta.ServiceID][session] = struct{}{}
	switcher.current = session
	if previous != nil {
		previous.retired = true
	}
	closePrevious := previous != nil && previous.refs == 0
	h.mu.Unlock()

	if closePrevious {
		_ = previous.Close()
	}
	return session, nil
}

func (h *SupervisorHandler) ensureSwitcherLocked(serviceID string, descriptor plugincontract.AdapterDescriptor) (*switchingProvider, error) {
	if switcher := h.routes[serviceID]; switcher != nil {
		if !sameAdapterIdentity(switcher.descriptor, descriptor) {
			return nil, fmt.Errorf("adapterprovider: service %s changed adapter identity without restart", serviceID)
		}
		return switcher, nil
	}
	if existing, ok := h.registry.Get(descriptor.ID, descriptor.ContractVersion); ok {
		if !existing.External || !sameAdapterIdentity(existing, descriptor) {
			return nil, fmt.Errorf("adapterprovider: adapter %s v%d conflicts with an existing adapter", descriptor.ID, descriptor.ContractVersion)
		}
	} else if err := h.registry.Register(descriptor); err != nil {
		return nil, err
	}
	switcher := &switchingProvider{
		handler:    h,
		serviceID:  serviceID,
		descriptor: descriptor,
	}
	if err := h.router.Register(descriptor.ID, descriptor.ContractVersion, switcher); err != nil {
		_ = h.registry.Unregister(descriptor.ID, descriptor.ContractVersion)
		return nil, err
	}
	h.routes[serviceID] = switcher
	return switcher, nil
}

func (h *SupervisorHandler) removeSession(session *providerSession, unregisterRoute bool) {
	h.mu.Lock()
	if h.active[session.serviceID] == session {
		delete(h.active, session.serviceID)
	}
	if sessions := h.sessions[session.serviceID]; sessions != nil {
		delete(sessions, session)
		if len(sessions) == 0 {
			delete(h.sessions, session.serviceID)
		}
	}
	switcher := h.routes[session.serviceID]
	if unregisterRoute && switcher == session.switcher {
		delete(h.routes, session.serviceID)
		switcher.current = nil
	} else {
		unregisterRoute = false
	}
	h.mu.Unlock()
	if unregisterRoute {
		h.router.Unregister(switcher.descriptor.ID, switcher.descriptor.ContractVersion)
		_ = h.registry.Unregister(switcher.descriptor.ID, switcher.descriptor.ContractVersion)
	}
}

func (h *SupervisorHandler) release(session *providerSession) {
	h.mu.Lock()
	if session.refs > 0 {
		session.refs--
	}
	closeSession := session.refs == 0 && session.retired
	h.mu.Unlock()
	if closeSession {
		_ = session.Close()
	}
}

type switchingProvider struct {
	handler    *SupervisorHandler
	serviceID  string
	descriptor plugincontract.AdapterDescriptor
	current    *providerSession
}

func (p *switchingProvider) Build(ctx context.Context, module plugincontract.ModuleRef, contribution plugincontract.ContributionDescriptor) (plugincontract.ContributionRegistration, error) {
	p.handler.mu.Lock()
	session := p.current
	if session == nil || session.closed {
		p.handler.mu.Unlock()
		return nil, fmt.Errorf("adapterprovider: service %s has no active provider", p.serviceID)
	}
	session.refs++
	p.handler.mu.Unlock()
	registration, err := session.provider.Build(ctx, module, contribution)
	if err != nil {
		p.handler.release(session)
		return nil, err
	}
	return &sessionRegistration{inner: registration, session: session}, nil
}

func (p *switchingProvider) Activate(ctx context.Context, registration plugincontract.ContributionRegistration) error {
	scoped, err := p.scopedRegistration(registration)
	if err != nil {
		return err
	}
	return scoped.session.provider.Activate(ctx, scoped.inner)
}

func (p *switchingProvider) Deactivate(ctx context.Context, registration plugincontract.ContributionRegistration) error {
	scoped, err := p.scopedRegistration(registration)
	if err != nil {
		return err
	}
	return scoped.session.provider.Deactivate(ctx, scoped.inner)
}

func (p *switchingProvider) Discard(ctx context.Context, registration plugincontract.ContributionRegistration) error {
	scoped, err := p.scopedRegistration(registration)
	if err != nil {
		return err
	}
	err = scoped.session.provider.Discard(ctx, scoped.inner)
	scoped.release()
	return err
}

func (p *switchingProvider) Invoke(ctx context.Context, request plugincontract.AdapterInvocationRequest) (plugincontract.AdapterInvocationResult, error) {
	p.handler.mu.Lock()
	session := p.current
	if session == nil || session.closed {
		p.handler.mu.Unlock()
		return plugincontract.AdapterInvocationResult{}, fmt.Errorf("adapterprovider: service %s has no active provider", p.serviceID)
	}
	session.refs++
	p.handler.mu.Unlock()
	defer p.handler.release(session)
	return session.provider.Invoke(ctx, request)
}

func (p *switchingProvider) Cancel(ctx context.Context, request plugincontract.AdapterCancellationRequest) error {
	p.handler.mu.Lock()
	session := p.current
	if session == nil || session.closed {
		p.handler.mu.Unlock()
		return fmt.Errorf("adapterprovider: service %s has no active provider", p.serviceID)
	}
	session.refs++
	p.handler.mu.Unlock()
	defer p.handler.release(session)
	return session.provider.Cancel(ctx, request)
}

func (p *switchingProvider) scopedRegistration(registration plugincontract.ContributionRegistration) (*sessionRegistration, error) {
	scoped, ok := registration.(*sessionRegistration)
	if !ok || scoped == nil || scoped.session == nil || scoped.session.handler != p.handler {
		return nil, fmt.Errorf("adapterprovider: unexpected registration type %T", registration)
	}
	return scoped, nil
}

type sessionRegistration struct {
	inner   plugincontract.ContributionRegistration
	session *providerSession
	once    sync.Once
}

func (r *sessionRegistration) ID() string {
	if r == nil || r.inner == nil {
		return ""
	}
	return r.inner.ID()
}

func (r *sessionRegistration) release() {
	if r == nil {
		return
	}
	r.once.Do(func() {
		r.session.handler.release(r.session)
	})
}

type providerSession struct {
	handler   *SupervisorHandler
	serviceID string
	switcher  *switchingProvider
	provider  *plugincontract.ExternalAdapterProvider
	transport *plugincontract.StdioAdapterProviderTransport

	refs      int
	retired   bool
	closed    bool
	closeOnce sync.Once
}

func (s *providerSession) Close() error {
	if s == nil {
		return nil
	}
	s.handler.mu.Lock()
	current := s.handler.active[s.serviceID] == s
	s.closed = true
	s.retired = true
	s.handler.mu.Unlock()
	s.handler.removeSession(s, current)
	return s.closeTransport()
}

func (s *providerSession) closeTransport() error {
	if s == nil {
		return nil
	}
	var closeErr error
	s.closeOnce.Do(func() {
		closeErr = s.transport.Close()
	})
	return closeErr
}

func validateProviderRegistration(registration ProviderRegistration) error {
	if registration.ServiceID == "" {
		return fmt.Errorf("adapterprovider: service id is required")
	}
	return registration.Descriptor.Validate()
}

func cloneProviderRegistration(registration ProviderRegistration) ProviderRegistration {
	registration.Descriptor.SupportedKinds = append([]string(nil), registration.Descriptor.SupportedKinds...)
	registration.Descriptor.ContributionSchema = append([]byte(nil), registration.Descriptor.ContributionSchema...)
	registration.Descriptor.RequiredHostCapabilities = append([]string(nil), registration.Descriptor.RequiredHostCapabilities...)
	registration.Descriptor.PermissionRequirements = append([]string(nil), registration.Descriptor.PermissionRequirements...)
	registration.Descriptor.Platforms = append([]string(nil), registration.Descriptor.Platforms...)
	registration.Descriptor.Operations = append([]string(nil), registration.Descriptor.Operations...)
	return registration
}

func sameAdapterIdentity(left, right plugincontract.AdapterDescriptor) bool {
	return left.ID == right.ID && left.ContractVersion == right.ContractVersion
}
