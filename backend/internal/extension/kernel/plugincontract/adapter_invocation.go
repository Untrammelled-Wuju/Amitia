package plugincontract

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type AdapterInvocationRequest struct {
	InvocationID   string          `json:"invocationId"`
	Module         ModuleRef       `json:"module"`
	ContributionID string          `json:"contributionId,omitempty"`
	Operation      string          `json:"operation"`
	Input          json.RawMessage `json:"input,omitempty"`
}

type AdapterInvocationResult struct {
	InvocationID string          `json:"invocationId"`
	Output       json.RawMessage `json:"output,omitempty"`
}

type AdapterCancellationRequest struct {
	InvocationID string `json:"invocationId"`
	Reason       string `json:"reason,omitempty"`
}

type AdapterInvoker interface {
	Invoke(context.Context, AdapterInvocationRequest) (AdapterInvocationResult, error)
}

type AdapterCanceller interface {
	Cancel(context.Context, AdapterCancellationRequest) error
}

func (r *ContributionRouter) InvokeAdapter(
	ctx context.Context,
	adapterID string,
	contractVersion int,
	request AdapterInvocationRequest,
) (AdapterInvocationResult, error) {
	handler, err := r.adapterHandler(adapterID, contractVersion)
	if err != nil {
		return AdapterInvocationResult{}, err
	}
	invoker, ok := handler.(AdapterInvoker)
	if !ok {
		return AdapterInvocationResult{}, fmt.Errorf("plugincontract: adapter %s v%d does not support invocation", adapterID, contractVersion)
	}
	if strings.TrimSpace(request.InvocationID) == "" || strings.TrimSpace(request.Operation) == "" {
		return AdapterInvocationResult{}, fmt.Errorf("plugincontract: invocation id and operation are required")
	}
	result, err := invoker.Invoke(ctx, request)
	if err != nil {
		return AdapterInvocationResult{}, err
	}
	if result.InvocationID == "" {
		result.InvocationID = request.InvocationID
	}
	return result, nil
}

func (r *ContributionRouter) CancelAdapterInvocation(
	ctx context.Context,
	adapterID string,
	contractVersion int,
	request AdapterCancellationRequest,
) error {
	handler, err := r.adapterHandler(adapterID, contractVersion)
	if err != nil {
		return err
	}
	canceller, ok := handler.(AdapterCanceller)
	if !ok {
		return fmt.Errorf("plugincontract: adapter %s v%d does not support cancellation", adapterID, contractVersion)
	}
	if strings.TrimSpace(request.InvocationID) == "" {
		return fmt.Errorf("plugincontract: invocation id is required")
	}
	return canceller.Cancel(ctx, request)
}

func (r *ContributionRouter) adapterHandler(adapterID string, contractVersion int) (ContributionHandler, error) {
	if r == nil || r.adapters == nil {
		return nil, fmt.Errorf("plugincontract: contribution router is unavailable")
	}
	if _, ok := r.adapters.Get(adapterID, contractVersion); !ok {
		return nil, fmt.Errorf("plugincontract: adapter %s v%d is not registered", adapterID, contractVersion)
	}
	r.mu.RLock()
	handler := r.routes[contributionRouteKey(adapterID, contractVersion)]
	r.mu.RUnlock()
	if handler == nil {
		return nil, fmt.Errorf("plugincontract: contribution handler %s v%d is not registered", adapterID, contractVersion)
	}
	return handler, nil
}
