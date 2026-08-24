package permission

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/rpc"
)

const (
	MethodPermissionCheck    rpc.Method = "permission.check"
	MethodPermissionSnapshot rpc.Method = "permission.snapshot"
	MethodPermissionRequest  rpc.Method = "permission.request"
)

const (
	RPCDecisionAllowed          = "allowed"
	RPCDecisionDenied           = "denied"
	RPCDecisionApprovalRequired = "approval_required"
)

type PermissionPluginResolver interface {
	Get(ctx context.Context, pluginID domain.PluginID) (domain.PluginDescriptor, error)
}

type PermissionRPCHandler struct {
	permissions *EffectivePermissionAdapter
	plugins     PermissionPluginResolver
}

func NewPermissionRPCHandler(permissions *EffectivePermissionAdapter, plugins PermissionPluginResolver) (*PermissionRPCHandler, error) {
	if permissions == nil {
		return nil, fmt.Errorf("permission rpc: effective permission adapter is required")
	}
	if plugins == nil {
		return nil, fmt.Errorf("permission rpc: plugin resolver is required")
	}
	return &PermissionRPCHandler{permissions: permissions, plugins: plugins}, nil
}

func (h *PermissionRPCHandler) Register(registry rpc.HandlerRegistry) error {
	if h == nil || registry == nil {
		return fmt.Errorf("permission rpc: handler and registry are required")
	}
	for _, method := range []rpc.Method{MethodPermissionCheck, MethodPermissionSnapshot, MethodPermissionRequest} {
		if err := registry.Register(method, h); err != nil {
			return fmt.Errorf("register %s: %w", method, err)
		}
	}
	return nil
}

type permissionCheckInput struct {
	PermissionID string `json:"permissionId"`
	ServiceID    string `json:"serviceId,omitempty"`
	RuntimeID    string `json:"runtimeId,omitempty"`
}

type permissionSnapshotInput struct {
	RuntimeID string `json:"runtimeId,omitempty"`
	ServiceID string `json:"serviceId,omitempty"`
}

type permissionRequestInput struct {
	PermissionID string `json:"permissionId"`
	ServiceID    string `json:"serviceId,omitempty"`
}

func (h *PermissionRPCHandler) Handle(ctx context.Context, request rpc.RPCRequest) (rpc.RPCResponse, error) {
	if h == nil || h.permissions == nil || h.plugins == nil {
		return routedPermissionError(request.ID, "PERMISSION_UNAVAILABLE", "permission service is unavailable"), nil
	}
	if strings.TrimSpace(string(request.PluginID)) == "" || strings.TrimSpace(string(request.RuntimeID)) == "" || strings.TrimSpace(string(request.ServiceID)) == "" {
		return routedPermissionError(request.ID, "INVALID_IDENTITY", "trusted plugin, runtime and service identity are required"), nil
	}

	descriptor, err := h.plugins.Get(ctx, request.PluginID)
	if err != nil {
		return routedPermissionError(request.ID, "INVALID_IDENTITY", "plugin descriptor is unavailable"), nil
	}
	if descriptor.ID != request.PluginID {
		return routedPermissionError(request.ID, "IDENTITY_MISMATCH", "plugin descriptor does not match trusted identity"), nil
	}

	switch request.Method {
	case MethodPermissionCheck:
		return h.handleCheck(ctx, request, descriptor)
	case MethodPermissionSnapshot:
		return h.handleSnapshot(ctx, request, descriptor)
	case MethodPermissionRequest:
		return h.handleRequest(ctx, request, descriptor)
	default:
		return routedPermissionError(request.ID, "METHOD_NOT_FOUND", "permission method is not supported"), nil
	}
}

func (h *PermissionRPCHandler) handleCheck(ctx context.Context, request rpc.RPCRequest, descriptor domain.PluginDescriptor) (rpc.RPCResponse, error) {
	var input permissionCheckInput
	if err := decodePermissionPayload(request.Payload, &input); err != nil {
		return routedPermissionError(request.ID, "INVALID_PAYLOAD", err.Error()), nil
	}
	input.PermissionID = strings.TrimSpace(input.PermissionID)
	input.ServiceID = strings.TrimSpace(input.ServiceID)
	input.RuntimeID = strings.TrimSpace(input.RuntimeID)
	if input.PermissionID == "" {
		return routedPermissionError(request.ID, "INVALID_PAYLOAD", "permissionId is required"), nil
	}
	if err := validatePermissionIdentity(request, input.RuntimeID, input.ServiceID); err != nil {
		return routedPermissionError(request.ID, "IDENTITY_MISMATCH", err.Error()), nil
	}
	if !declaresPermission(descriptor, input.PermissionID) {
		return permissionDecisionResponse(request.ID, input.PermissionID, DecisionResult{Decision: DecisionDenied, Reason: ReasonNotDeclared}), nil
	}

	serviceID := trustedPermissionServiceID(request, input.ServiceID)
	decision := h.permissions.InspectServicePermission(ctx, string(request.RuntimeID), string(request.PluginID), serviceID, input.PermissionID)
	return permissionDecisionResponse(request.ID, input.PermissionID, decision), nil
}

func (h *PermissionRPCHandler) handleRequest(ctx context.Context, request rpc.RPCRequest, descriptor domain.PluginDescriptor) (rpc.RPCResponse, error) {
	var input permissionRequestInput
	if err := decodePermissionPayload(request.Payload, &input); err != nil {
		return routedPermissionError(request.ID, "INVALID_PAYLOAD", err.Error()), nil
	}
	input.PermissionID = strings.TrimSpace(input.PermissionID)
	input.ServiceID = strings.TrimSpace(input.ServiceID)
	if input.PermissionID == "" {
		return routedPermissionError(request.ID, "INVALID_PAYLOAD", "permissionId is required"), nil
	}
	if err := validatePermissionIdentity(request, "", input.ServiceID); err != nil {
		return routedPermissionError(request.ID, "IDENTITY_MISMATCH", err.Error()), nil
	}
	if !declaresPermission(descriptor, input.PermissionID) {
		return permissionDecisionResponse(request.ID, input.PermissionID, DecisionResult{Decision: DecisionDenied, Reason: ReasonNotDeclared}), nil
	}

	serviceID := trustedPermissionServiceID(request, input.ServiceID)
	decision := h.permissions.CheckServicePermission(ctx, string(request.RuntimeID), string(request.PluginID), serviceID, input.PermissionID)
	return permissionDecisionResponse(request.ID, input.PermissionID, decision), nil
}

func (h *PermissionRPCHandler) handleSnapshot(ctx context.Context, request rpc.RPCRequest, descriptor domain.PluginDescriptor) (rpc.RPCResponse, error) {
	var input permissionSnapshotInput
	if err := decodePermissionPayload(request.Payload, &input); err != nil {
		return routedPermissionError(request.ID, "INVALID_PAYLOAD", err.Error()), nil
	}
	input.RuntimeID = strings.TrimSpace(input.RuntimeID)
	input.ServiceID = strings.TrimSpace(input.ServiceID)
	if err := validatePermissionIdentity(request, input.RuntimeID, input.ServiceID); err != nil {
		return routedPermissionError(request.ID, "IDENTITY_MISMATCH", err.Error()), nil
	}

	serviceID := trustedPermissionServiceID(request, input.ServiceID)
	subject, err := h.permissions.MapServiceSubject(string(request.RuntimeID), string(request.PluginID), serviceID)
	if err != nil {
		return routedPermissionError(request.ID, "INVALID_IDENTITY", err.Error()), nil
	}

	view := h.permissions.ResolveRuntimePermissions(ctx, subject, descriptor.RequiredPermissions...)
	grantedPerms := view.AllowedPermissions()
	sort.Strings(grantedPerms)
	grantedScopes := []string{"extension:" + subject.ExtensionID}
	if moduleID := subject.EffectiveModuleID(); moduleID != "" {
		grantedScopes = []string{"module:" + moduleID}
	}

	snapshotID := permissionViewSnapshotID(request, view.Revision, grantedPerms, grantedScopes)
	payload, err := json.Marshal(map[string]any{
		"snapshotId":    snapshotID,
		"revision":      view.Revision,
		"grantedPerms":  grantedPerms,
		"grantedScopes": grantedScopes,
		"isValid":       true,
	})
	if err != nil {
		return rpc.RPCResponse{}, err
	}
	return rpc.RPCResponse{RequestID: request.ID, Payload: payload}, nil
}

func decodePermissionPayload(payload json.RawMessage, target any) error {
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("invalid permission payload: %w", err)
	}
	return nil
}

func validatePermissionIdentity(request rpc.RPCRequest, runtimeID, serviceID string) error {
	if runtimeID != "" && runtimeID != string(request.RuntimeID) {
		return fmt.Errorf("runtimeId does not match trusted identity")
	}
	if serviceID != "" && serviceID != string(request.ServiceID) {
		return fmt.Errorf("serviceId does not match trusted identity")
	}
	return nil
}

func trustedPermissionServiceID(request rpc.RPCRequest, requested string) string {
	if strings.TrimSpace(requested) != "" {
		return strings.TrimSpace(requested)
	}
	return string(request.ServiceID)
}

func declaresPermission(descriptor domain.PluginDescriptor, permissionID string) bool {
	for _, declared := range descriptor.RequiredPermissions {
		if strings.TrimSpace(declared) == permissionID {
			return true
		}
	}
	return false
}

func permissionDecisionResponse(requestID, permissionID string, decision DecisionResult) rpc.RPCResponse {
	decisionValue := RPCDecisionDenied
	switch decision.Decision {
	case DecisionAllowed:
		decisionValue = RPCDecisionAllowed
	case DecisionRequireApproval:
		decisionValue = RPCDecisionApprovalRequired
	}
	payload, _ := json.Marshal(map[string]any{
		"permissionId": permissionID,
		"decision":     decisionValue,
		"reason":       string(decision.Reason),
		"detail":       decision.Detail,
	})
	return rpc.RPCResponse{RequestID: requestID, Payload: payload}
}

func routedPermissionError(requestID, code, message string) rpc.RPCResponse {
	return rpc.RPCResponse{
		RequestID: requestID,
		Error: &rpc.RPCRoutedError{
			Code:    code,
			Message: message,
		},
	}
}

func permissionViewSnapshotID(request rpc.RPCRequest, revision string, perms, scopes []string) string {
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%s\x00%s\x00%s\x00%d\x00%s", request.PluginID, request.RuntimeID, request.ServiceID, request.Generation, revision)
	for _, permissionID := range perms {
		_, _ = fmt.Fprintf(h, "\x00p:%s", permissionID)
	}
	for _, scope := range scopes {
		_, _ = fmt.Fprintf(h, "\x00s:%s", scope)
	}
	sum := h.Sum(nil)
	return "ghps-" + hex.EncodeToString(sum[:16])
}

var _ rpc.Handler = (*PermissionRPCHandler)(nil)
