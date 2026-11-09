package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
)

var pluginBrokerSchema = json.RawMessage(`{"type":"object"}`)

func registerPluginBrokerRoutes(gateway *host_api.DefaultGateway, broker *PluginBroker) error {
	if gateway == nil || broker == nil {
		return fmt.Errorf("plugin broker routing is not configured")
	}
	routes := []host_api.Route{
		pluginBrokerRoute(host_api.MethodPluginContributionRegister, broker.registerContribution),
		pluginBrokerRoute(host_api.MethodPluginContributionUnregister, broker.unregisterContribution),
		pluginBrokerRoute(host_api.MethodPluginServiceProvide, broker.provideService),
		pluginBrokerRoute(host_api.MethodPluginServiceUnprovide, broker.unprovideService),
		pluginBrokerRoute(host_api.MethodPluginServiceRequire, broker.requireService),
		pluginBrokerRoute(host_api.MethodPluginServiceRelease, broker.releaseService),
		pluginBrokerRoute(host_api.MethodPluginAdapterInvoke, broker.invokeAdapter),
		pluginBrokerRoute(host_api.MethodPluginAdapterCancel, broker.cancelAdapterInvocation),
	}
	for _, route := range routes {
		if err := gateway.RegisterRoute(route); err != nil {
			return err
		}
	}
	return nil
}

func pluginBrokerRoute(method host_api.Method, handler host_api.Handler) host_api.Route {
	return host_api.Route{
		Method:          method,
		Version:         1,
		InputSchema:     pluginBrokerSchema,
		OutputSchema:    pluginBrokerSchema,
		Permission:      host_api.RoutePermissionForMethod(method),
		ScopePolicy:     host_api.RouteScopeForMethod(method),
		RiskLevel:       host_api.RouteRiskForMethod(method),
		SideEffectLevel: host_api.RouteSideEffectForMethod(method),
		Timeout:         30 * time.Second,
		Handler:         handler,
	}
}

func (b *PluginBroker) registerContribution(ctx context.Context, request host_api.CallRequest) (host_api.CallResult, error) {
	var descriptor plugincontract.ContributionDescriptor
	if err := json.Unmarshal(request.Input, &descriptor); err != nil {
		return pluginBrokerFailure(host_api.ErrorCodeInputInvalid, err)
	}
	module := pluginBrokerModule(request)
	if err := b.RegisterContribution(ctx, module, descriptor); err != nil {
		return pluginBrokerFailure(host_api.ErrorCodeInternal, err)
	}
	return pluginBrokerSuccess(map[string]any{"id": descriptor.ID})
}

func (b *PluginBroker) unregisterContribution(ctx context.Context, request host_api.CallRequest) (host_api.CallResult, error) {
	var input struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(request.Input, &input); err != nil {
		return pluginBrokerFailure(host_api.ErrorCodeInputInvalid, err)
	}
	if input.ID == "" {
		return pluginBrokerFailure(host_api.ErrorCodeInputInvalid, fmt.Errorf("contribution id is required"))
	}
	if err := b.UnregisterContribution(ctx, pluginBrokerModule(request), input.ID); err != nil {
		return pluginBrokerFailure(host_api.ErrorCodeInternal, err)
	}
	return pluginBrokerSuccess(map[string]any{"id": input.ID})
}

func (b *PluginBroker) provideService(_ context.Context, request host_api.CallRequest) (host_api.CallResult, error) {
	var offer plugincontract.ServiceOffer
	if err := json.Unmarshal(request.Input, &offer); err != nil {
		return pluginBrokerFailure(host_api.ErrorCodeInputInvalid, err)
	}
	if err := b.ProvideService(pluginBrokerModule(request), offer); err != nil {
		return pluginBrokerFailure(host_api.ErrorCodeInternal, err)
	}
	return pluginBrokerSuccess(map[string]any{"serviceId": offer.ServiceID})
}

func (b *PluginBroker) unprovideService(_ context.Context, request host_api.CallRequest) (host_api.CallResult, error) {
	var input struct {
		ServiceID string `json:"serviceId"`
	}
	if err := json.Unmarshal(request.Input, &input); err != nil {
		return pluginBrokerFailure(host_api.ErrorCodeInputInvalid, err)
	}
	if err := b.UnprovideService(pluginBrokerModule(request), input.ServiceID); err != nil {
		return pluginBrokerFailure(host_api.ErrorCodeInternal, err)
	}
	return pluginBrokerSuccess(map[string]any{"serviceId": input.ServiceID})
}

func (b *PluginBroker) requireService(_ context.Context, request host_api.CallRequest) (host_api.CallResult, error) {
	var requirement plugincontract.ServiceRequirement
	if err := json.Unmarshal(request.Input, &requirement); err != nil {
		return pluginBrokerFailure(host_api.ErrorCodeInputInvalid, err)
	}
	providerIDs, err := b.RequireService(pluginBrokerModule(request), requirement)
	if err != nil {
		return pluginBrokerFailure(host_api.ErrorCodeInternal, err)
	}
	return pluginBrokerSuccess(map[string]any{
		"serviceId":   requirement.ServiceID,
		"providerIds": providerIDs,
	})
}

func (b *PluginBroker) releaseService(_ context.Context, request host_api.CallRequest) (host_api.CallResult, error) {
	var input struct {
		ServiceID string `json:"serviceId"`
	}
	if err := json.Unmarshal(request.Input, &input); err != nil {
		return pluginBrokerFailure(host_api.ErrorCodeInputInvalid, err)
	}
	if err := b.ReleaseService(pluginBrokerModule(request), input.ServiceID); err != nil {
		return pluginBrokerFailure(host_api.ErrorCodeInternal, err)
	}
	return pluginBrokerSuccess(map[string]any{"serviceId": input.ServiceID})
}

func (b *PluginBroker) invokeAdapter(ctx context.Context, request host_api.CallRequest) (host_api.CallResult, error) {
	var input struct {
		AdapterID       string          `json:"adapterId"`
		ContractVersion int             `json:"contractVersion"`
		InvocationID    string          `json:"invocationId,omitempty"`
		ContributionID  string          `json:"contributionId,omitempty"`
		Operation       string          `json:"operation"`
		Input           json.RawMessage `json:"input,omitempty"`
	}
	if err := json.Unmarshal(request.Input, &input); err != nil {
		return pluginBrokerFailure(host_api.ErrorCodeInputInvalid, err)
	}
	if input.AdapterID == "" || input.ContractVersion <= 0 || input.Operation == "" {
		return pluginBrokerFailure(host_api.ErrorCodeInputInvalid, fmt.Errorf("adapterId, contractVersion and operation are required"))
	}
	if len(input.Input) > 4*1024*1024 {
		return pluginBrokerFailure(host_api.ErrorCodeInputInvalid, fmt.Errorf("adapter invocation input exceeds 4 MiB"))
	}
	result, err := b.InvokeAdapter(ctx, pluginBrokerModule(request), input.AdapterID, input.ContractVersion, plugincontract.AdapterInvocationRequest{
		InvocationID:   input.InvocationID,
		ContributionID: input.ContributionID,
		Operation:      input.Operation,
		Input:          input.Input,
	})
	if err != nil {
		return pluginBrokerFailure(host_api.ErrorCodeInternal, err)
	}
	return pluginBrokerSuccess(result)
}

func (b *PluginBroker) cancelAdapterInvocation(ctx context.Context, request host_api.CallRequest) (host_api.CallResult, error) {
	var input struct {
		AdapterID       string `json:"adapterId"`
		ContractVersion int    `json:"contractVersion"`
		InvocationID    string `json:"invocationId"`
		Reason          string `json:"reason,omitempty"`
	}
	if err := json.Unmarshal(request.Input, &input); err != nil {
		return pluginBrokerFailure(host_api.ErrorCodeInputInvalid, err)
	}
	if input.AdapterID == "" || input.ContractVersion <= 0 || input.InvocationID == "" {
		return pluginBrokerFailure(host_api.ErrorCodeInputInvalid, fmt.Errorf("adapterId, contractVersion and invocationId are required"))
	}
	if err := b.CancelAdapterInvocation(ctx, pluginBrokerModule(request), input.AdapterID, input.ContractVersion, plugincontract.AdapterCancellationRequest{
		InvocationID: input.InvocationID,
		Reason:       input.Reason,
	}); err != nil {
		return pluginBrokerFailure(host_api.ErrorCodeInternal, err)
	}
	return pluginBrokerSuccess(map[string]any{"invocationId": input.InvocationID})
}

func pluginBrokerModule(request host_api.CallRequest) plugincontract.ModuleRef {
	return plugincontract.ModuleRef{
		PluginID: string(request.RuntimeIdentity.ExtensionID),
		ModuleID: string(request.RuntimeIdentity.ModuleID),
	}
}

func pluginBrokerSuccess(value any) (host_api.CallResult, error) {
	output, err := json.Marshal(value)
	if err != nil {
		return host_api.CallResult{}, err
	}
	return host_api.CallResult{Status: host_api.StatusSuccess, Output: output}, nil
}

func pluginBrokerFailure(code string, err error) (host_api.CallResult, error) {
	return host_api.CallResult{
		Status: host_api.StatusFailed,
		Error:  &host_api.Error{Code: code, Message: err.Error()},
	}, nil
}
