package kernel

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type KernelProviderInvoker struct {
	invocationService *capability.ProviderInvocationService
}

func NewKernelProviderInvoker(invocationService *capability.ProviderInvocationService) *KernelProviderInvoker {
	return &KernelProviderInvoker{invocationService: invocationService}
}

func (k *KernelProviderInvoker) Invoke(ctx context.Context, req ProviderInvokeRequest) (ProviderInvokeResponse, error) {
	if k.invocationService == nil {
		return ProviderInvokeResponse{Success: false}, fmt.Errorf("kernel provider invocation: service not configured")
	}

	invReq := capability.ProviderInvocationRequest{
		CapabilityID:       capability.CapabilityID(req.Target),
		Input:              req.Payload,
		PreferredPlacement: capability.ProviderPlacementCore,
		AllowCore:          true,
		AllowDevice:        true,
	}

	result, err := k.invocationService.Invoke(ctx, invReq)
	if err != nil {
		return ProviderInvokeResponse{Success: false}, err
	}

	output := result.Output
	if output == nil {
		output = []byte("{}")
	}

	return ProviderInvokeResponse{
		Success: true,
		Result:  output,
	}, nil
}
