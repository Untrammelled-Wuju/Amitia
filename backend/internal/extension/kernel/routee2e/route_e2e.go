//go:build rsi_routee2e

package routee2e

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/extension/kernel/coreadapter"
	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
)

var (
	ErrNoProvider       = errors.New("routee2e: no provider could serve the request")
	ErrEmptyPluginReply = errors.New("routee2e: plugin provider returned no output")
)

type PluginInvoker interface {
	Invoke(context.Context, plugincontract.AdapterInvocationRequest) (plugincontract.AdapterInvocationResult, error)
}

type Attempt struct {
	ProviderID string `json:"providerId"`
	Outcome    string `json:"outcome"`
	Error      string `json:"error,omitempty"`
}

type Receipt struct {
	InvocationID string    `json:"invocationId"`
	ServiceID    string    `json:"serviceId"`
	ProviderID   string    `json:"providerId"`
	Revision     uint64    `json:"revision"`
	Fallback     bool      `json:"fallback"`
	Hops         []string  `json:"hops"`
	Candidates   int       `json:"candidates"`
	Attempts     []Attempt `json:"attempts"`
}

type Request struct {
	InvocationID string
	CapabilityID string
	ServiceID    string
	SpaceID      string
	DeviceID     string
	RoleID       string
	Input        json.RawMessage
}

type Route struct {
	Graph        plugincontract.ServiceGraphResolution
	Snapshot     *plugincontract.BindingSnapshot
	Core         *coreadapter.MemorySearchAdapter
	PluginID     string
	PluginModule plugincontract.ModuleRef
	Plugin       PluginInvoker
}

func (r *Route) Execute(ctx context.Context, request Request) (Receipt, json.RawMessage, error) {
	receipt := Receipt{InvocationID: request.InvocationID, ServiceID: request.ServiceID}
	if r == nil || r.Snapshot == nil {
		return receipt, nil, fmt.Errorf("%w: route is not configured", ErrNoProvider)
	}
	selected, err := plugincontract.SelectProviderCandidates(
		r.Graph,
		r.Snapshot,
		plugincontract.BindingRequest{
			CapabilityID: request.CapabilityID,
			SpaceID:      request.SpaceID,
			DeviceID:     request.DeviceID,
		},
		request.ServiceID,
	)
	if err != nil {
		return receipt, nil, fmt.Errorf("%w: %w", ErrNoProvider, err)
	}
	receipt.Revision = selected.Revision
	receipt.Candidates = len(selected.Candidates)
	for index, candidate := range selected.Candidates {
		output, attemptErr := r.invoke(ctx, request, candidate)
		if attemptErr != nil {
			receipt.Attempts = append(receipt.Attempts, Attempt{
				ProviderID: candidate.ProviderID,
				Outcome:    "failed",
				Error:      attemptErr.Error(),
			})
			continue
		}
		receipt.ProviderID = candidate.ProviderID
		receipt.Fallback = index > 0
		receipt.Hops = hopsFor(candidate.ProviderID, r.PluginID)
		receipt.Attempts = append(receipt.Attempts, Attempt{ProviderID: candidate.ProviderID, Outcome: "served"})
		return receipt, output, nil
	}
	return receipt, nil, fmt.Errorf("%w: tried %d candidates", ErrNoProvider, len(selected.Candidates))
}

func (r *Route) invoke(ctx context.Context, request Request, candidate plugincontract.ServiceDescriptor) (json.RawMessage, error) {
	if candidate.ProviderID == r.PluginID {
		return r.invokePlugin(ctx, request)
	}
	return r.invokeCore(ctx, request)
}

func (r *Route) invokeCore(ctx context.Context, request Request) (json.RawMessage, error) {
	if r.Core == nil {
		return nil, fmt.Errorf("%w: core adapter is missing", ErrNoProvider)
	}
	response, err := r.Core.ExecuteRequest(ctx, businessRequest(request), request.Input)
	if err != nil {
		return nil, err
	}
	return json.Marshal(response)
}

func (r *Route) invokePlugin(ctx context.Context, request Request) (json.RawMessage, error) {
	if r.Plugin == nil {
		return nil, fmt.Errorf("%w: plugin invoker is missing", ErrNoProvider)
	}
	result, err := r.Plugin.Invoke(ctx, plugincontract.AdapterInvocationRequest{
		InvocationID: request.InvocationID,
		Module:       r.PluginModule,
		Operation:    request.ServiceID,
		Input:        request.Input,
	})
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(result.Output))) == 0 {
		return nil, ErrEmptyPluginReply
	}
	return result.Output, nil
}

func (r *Route) DirectCore(ctx context.Context, request Request) (json.RawMessage, error) {
	return r.invokeCore(ctx, request)
}

func businessRequest(request Request) business.Request {
	return business.Request{
		SpaceID:   request.SpaceID,
		DeviceID:  request.DeviceID,
		CoreID:    "core",
		RoleID:    request.RoleID,
		RequestID: request.InvocationID,
	}
}

func hopsFor(providerID string, pluginID string) []string {
	if providerID == pluginID {
		return []string{"core", "plugin", "core"}
	}
	return []string{"core"}
}
