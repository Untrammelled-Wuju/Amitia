//go:build rsi_coreadapter

package coreadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

const (
	MemorySearchProviderID = "builtin.core/core/memory.search"
	MemorySearchServiceID  = "memory.search"
)

var ErrMemorySearchAdapter = errors.New("coreadapter: memory search adapter unavailable")

type MemorySearcher interface {
	SearchOwnedMemories(ctx context.Context, request business.Request, input business.MemoryManagementQuery) (business.MemoryManagementResponse, error)
}

type MemorySearchAdapter struct {
	searcher   MemorySearcher
	providerID string
}

func NewMemorySearchAdapter(searcher MemorySearcher) *MemorySearchAdapter {
	return &MemorySearchAdapter{searcher: searcher, providerID: MemorySearchProviderID}
}

func (a *MemorySearchAdapter) Supports(binding capability.RuntimeBinding) bool {
	if binding.RuntimeType != capability.RuntimeTypeBuiltin {
		return false
	}
	if strings.TrimSpace(binding.ProviderID) == "" {
		return true
	}
	return binding.ProviderID == a.providerID
}

func (a *MemorySearchAdapter) Execute(
	ctx context.Context,
	binding capability.RuntimeBinding,
	invocation capability.ToolInvocationContext,
	input json.RawMessage,
) capability.UnifiedToolResult {
	result := capability.UnifiedToolResult{InvocationID: invocation.InvocationID, ToolID: a.providerID}
	if !a.Supports(binding) {
		result.Status = capability.ToolResultStatusFailed
		result.Error = &capability.ToolError{
			Code:        capability.ErrorCodeExecutionFailed,
			Message:     fmt.Sprintf("coreadapter: binding %q is not supported", binding.ProviderID),
			UserVisible: false,
			Cause:       ErrMemorySearchAdapter,
		}
		return result
	}
	response, err := a.ExecuteRequest(ctx, RequestFromInvocation(binding, invocation), input)
	if err != nil {
		result.Status = capability.ToolResultStatusFailed
		result.Error = &capability.ToolError{
			Code:        capability.ErrorCodeExecutionFailed,
			Message:     err.Error(),
			UserVisible: true,
			Cause:       err,
		}
		return result
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		result.Status = capability.ToolResultStatusFailed
		result.Error = &capability.ToolError{
			Code:        capability.ErrorCodeExecutionFailed,
			Message:     err.Error(),
			UserVisible: false,
			Cause:       err,
		}
		return result
	}
	result.Status = capability.ToolResultStatusSuccess
	result.Structured = encoded
	result.Content = []capability.ToolContent{{Type: capability.ToolContentText, Text: string(encoded)}}
	return result
}

func (a *MemorySearchAdapter) ExecuteRequest(
	ctx context.Context,
	request business.Request,
	input json.RawMessage,
) (business.MemoryManagementResponse, error) {
	if a == nil || a.searcher == nil {
		return business.MemoryManagementResponse{}, ErrMemorySearchAdapter
	}
	trimmed := strings.TrimSpace(string(input))
	if trimmed == "" {
		return business.MemoryManagementResponse{}, fmt.Errorf("%w: input is empty", ErrMemorySearchAdapter)
	}
	var query business.MemoryManagementQuery
	if err := json.Unmarshal(input, &query); err != nil {
		return business.MemoryManagementResponse{}, fmt.Errorf("%w: %w", ErrMemorySearchAdapter, err)
	}
	return a.searcher.SearchOwnedMemories(ctx, request, query)
}

func (a *MemorySearchAdapter) Health(ctx context.Context, binding capability.RuntimeBinding) capability.HealthStatus {
	if a == nil || a.searcher == nil || !a.Supports(binding) {
		return capability.HealthUnhealthy
	}
	return capability.HealthReady
}

func RequestFromInvocation(binding capability.RuntimeBinding, invocation capability.ToolInvocationContext) business.Request {
	request := business.Request{
		SpaceID:         invocation.SpaceID,
		CoreID:          metadataString(binding.Metadata, "coreId"),
		RoleID:          metadataString(binding.Metadata, "roleId"),
		TargetDeviceID:  metadataString(binding.Metadata, "targetDeviceId"),
		ResourceOwnerID: metadataString(binding.Metadata, "resourceOwnerId"),
		RoleOwnerID:     metadataString(binding.Metadata, "roleOwnerId"),
		RequestID:       invocation.InvocationID,
	}
	if invocation.ExecContext != nil {
		if request.SpaceID == "" {
			request.SpaceID = string(invocation.ExecContext.SpaceID)
		}
		if invocation.ExecContext.RuntimeTarget != nil {
			request.DeviceID = string(invocation.ExecContext.RuntimeTarget.DeviceID)
		}
	}
	if request.DeviceID == "" {
		request.DeviceID = metadataString(binding.Metadata, "deviceId")
	}
	if request.CoreID == "" {
		request.CoreID = binding.RuntimeID
	}
	if request.RoleID == "" {
		request.RoleID = invocation.CharacterID
	}
	return request
}

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, ok := metadata[key]
	if !ok {
		return ""
	}
	text, _ := value.(string)
	return strings.TrimSpace(text)
}
