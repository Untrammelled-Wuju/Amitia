package plugincontract

import (
	"context"
	"encoding/json"
	"fmt"
)

const (
	ExternalAdapterMethodBuild      = "adapter.build"
	ExternalAdapterMethodActivate   = "adapter.activate"
	ExternalAdapterMethodDeactivate = "adapter.deactivate"
	ExternalAdapterMethodDiscard    = "adapter.discard"
	ExternalAdapterMethodHealth     = "adapter.health"
	ExternalAdapterMethodInvoke     = "adapter.invoke"
	ExternalAdapterMethodCancel     = "adapter.cancel"
)

type AdapterProviderTransport interface {
	Call(context.Context, string, any, any) error
	Close() error
}

type ExternalAdapterProvider struct {
	Descriptor AdapterDescriptor
	Transport  AdapterProviderTransport
}

type remoteContributionRegistration struct {
	ContributionID string `json:"id"`
	HandleID       string `json:"handleId,omitempty"`
}

func (r *remoteContributionRegistration) ID() string {
	if r == nil {
		return ""
	}
	return r.ContributionID
}

func (p *ExternalAdapterProvider) Register(registry *AdapterRegistry) error {
	if p == nil || registry == nil {
		return fmt.Errorf("plugincontract: external adapter provider and registry are required")
	}
	if p.Transport == nil {
		return fmt.Errorf("plugincontract: external adapter transport is required")
	}
	descriptor := p.Descriptor
	descriptor.External = true
	if err := registry.Register(descriptor); err != nil {
		return err
	}
	p.Descriptor = descriptor
	return nil
}

func (p *ExternalAdapterProvider) Build(ctx context.Context, module ModuleRef, contribution ContributionDescriptor) (ContributionRegistration, error) {
	if p == nil || p.Transport == nil {
		return nil, fmt.Errorf("plugincontract: external adapter transport is unavailable")
	}
	var result remoteContributionRegistration
	err := p.Transport.Call(ctx, ExternalAdapterMethodBuild, map[string]any{
		"module":       module,
		"contribution": contribution,
	}, &result)
	if err != nil {
		return nil, err
	}
	if result.ContributionID == "" {
		result.ContributionID = contribution.ID
	}
	return &result, nil
}

func (p *ExternalAdapterProvider) Activate(ctx context.Context, registration ContributionRegistration) error {
	return p.callRegistration(ctx, ExternalAdapterMethodActivate, registration)
}

func (p *ExternalAdapterProvider) Deactivate(ctx context.Context, registration ContributionRegistration) error {
	return p.callRegistration(ctx, ExternalAdapterMethodDeactivate, registration)
}

func (p *ExternalAdapterProvider) Discard(ctx context.Context, registration ContributionRegistration) error {
	return p.callRegistration(ctx, ExternalAdapterMethodDiscard, registration)
}

func (p *ExternalAdapterProvider) Health(ctx context.Context) error {
	if p == nil || p.Transport == nil {
		return fmt.Errorf("plugincontract: external adapter transport is unavailable")
	}
	var status struct {
		Healthy bool   `json:"healthy"`
		Detail  string `json:"detail,omitempty"`
	}
	if err := p.Transport.Call(ctx, ExternalAdapterMethodHealth, map[string]any{"adapterId": p.Descriptor.ID}, &status); err != nil {
		return err
	}
	if !status.Healthy {
		return fmt.Errorf("plugincontract: external adapter %s is unhealthy: %s", p.Descriptor.ID, status.Detail)
	}
	return nil
}

func (p *ExternalAdapterProvider) Invoke(ctx context.Context, request AdapterInvocationRequest) (AdapterInvocationResult, error) {
	if p == nil || p.Transport == nil {
		return AdapterInvocationResult{}, fmt.Errorf("plugincontract: external adapter transport is unavailable")
	}
	var result AdapterInvocationResult
	if err := p.Transport.Call(ctx, ExternalAdapterMethodInvoke, map[string]any{
		"adapterId":       p.Descriptor.ID,
		"contractVersion": p.Descriptor.ContractVersion,
		"request":         request,
	}, &result); err != nil {
		return AdapterInvocationResult{}, err
	}
	return result, nil
}

func (p *ExternalAdapterProvider) Cancel(ctx context.Context, request AdapterCancellationRequest) error {
	if p == nil || p.Transport == nil {
		return fmt.Errorf("plugincontract: external adapter transport is unavailable")
	}
	return p.Transport.Call(ctx, ExternalAdapterMethodCancel, map[string]any{
		"adapterId":       p.Descriptor.ID,
		"contractVersion": p.Descriptor.ContractVersion,
		"request":         request,
	}, &struct{}{})
}

func (p *ExternalAdapterProvider) callRegistration(ctx context.Context, method string, registration ContributionRegistration) error {
	if p == nil || p.Transport == nil {
		return fmt.Errorf("plugincontract: external adapter transport is unavailable")
	}
	remote, ok := registration.(*remoteContributionRegistration)
	if !ok || remote == nil {
		return fmt.Errorf("plugincontract: unexpected registration type %T", registration)
	}
	return p.Transport.Call(ctx, method, map[string]any{
		"adapterId": p.Descriptor.ID,
		"id":        remote.ContributionID,
		"handleId":  remote.HandleID,
	}, &struct{}{})
}

type AdapterProviderStatus struct {
	Healthy bool   `json:"healthy"`
	Detail  string `json:"detail,omitempty"`
}

func EncodeAdapterProviderStatus(value AdapterProviderStatus) (json.RawMessage, error) {
	return json.Marshal(value)
}
