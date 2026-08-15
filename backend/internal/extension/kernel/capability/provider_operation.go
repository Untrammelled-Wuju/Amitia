package capability

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type ProviderOperation struct {
	CapabilityID CapabilityID
	Action       string
	Input        json.RawMessage
}

type ProviderOperationResult struct {
	CapabilityID       CapabilityID
	ProviderID         ProviderID
	ProviderInstanceID ProviderInstanceID
	Output             json.RawMessage
	ExecutionTarget    InvocationExecutionTarget
}

type ProviderInvocationService struct {
	capabilityService *CapabilityService
	adapterRegistry   *RuntimeAdapterRegistry
}

func NewProviderInvocationService(
	capabilityService *CapabilityService,
	adapterRegistry *RuntimeAdapterRegistry,
) *ProviderInvocationService {
	return &ProviderInvocationService{
		capabilityService: capabilityService,
		adapterRegistry:   adapterRegistry,
	}
}

func (s *ProviderInvocationService) Invoke(
	ctx context.Context,
	request ProviderInvocationRequest,
) (ProviderInvocationResult, error) {
	if s.capabilityService == nil {
		return ProviderInvocationResult{}, fmt.Errorf("provider invocation service: capability service not configured")
	}

	resolveReq := CapabilityResolutionRequest{
		CapabilityID:       request.CapabilityID,
		UserID:             request.UserID,
		PreferredPlacement: request.PreferredPlacement,
		RequiredPlacement:  request.RequiredPlacement,
		PreferredDeviceID:  request.PreferredDeviceID,
		RequiredDeviceID:   request.RequiredDeviceID,
		AllowCore:          request.AllowCore,
		AllowDevice:        request.AllowDevice,
	}

	resolution, err := s.capabilityService.Resolve(resolveReq)
	if err != nil {
		return ProviderInvocationResult{CapabilityID: request.CapabilityID}, fmt.Errorf("resolve capability %s: %w", request.CapabilityID, err)
	}

	return ProviderInvocationResult{
		CapabilityID:       request.CapabilityID,
		ProviderID:         resolution.Provider.ID,
		ProviderInstanceID: resolution.ProviderInstance.ID,
		ExecutionTarget:    resolution.ExecutionTarget,
	}, nil
}

func (s *ProviderInvocationService) InvokeLocal(
	ctx context.Context,
	op ProviderOperation,
	userID runtimeidentity.UserID,
) (ProviderOperationResult, error) {
	req := ProviderInvocationRequest{
		CapabilityID:       op.CapabilityID,
		Input:              op.Input,
		UserID:             userID,
		PreferredPlacement: ProviderPlacementCore,
		AllowCore:          true,
	}

	result, err := s.Invoke(ctx, req)
	if err != nil {
		return ProviderOperationResult{CapabilityID: op.CapabilityID}, err
	}

	return ProviderOperationResult{
		CapabilityID:       result.CapabilityID,
		ProviderID:         result.ProviderID,
		ProviderInstanceID: result.ProviderInstanceID,
		Output:             result.Output,
		ExecutionTarget:    result.ExecutionTarget,
	}, nil
}
