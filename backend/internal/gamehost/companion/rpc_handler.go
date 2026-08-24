package artifact

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	kernelpermission "github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/gamehost/domain"
	ghpermission "github.com/u-ai/backend/internal/gamehost/permission"
	"github.com/u-ai/backend/internal/gamehost/rpc"
)

const (
	MethodArtifactList           rpc.Method = "artifact.list"
	MethodArtifactDeployRequired rpc.Method = "artifact.deploy_required"
	MethodArtifactDeploy         rpc.Method = "artifact.deploy"
	MethodArtifactVerify         rpc.Method = "artifact.verify"
	MethodArtifactRemove         rpc.Method = "artifact.remove"
)

type PluginDescriptorResolver interface {
	Get(ctx context.Context, pluginID domain.PluginID) (domain.PluginDescriptor, error)
}

type ArtifactPermissionChecker interface {
	CheckServicePermission(ctx context.Context, runtimeID, pluginID, serviceID, permID string) ghpermission.DecisionResult
}

type ArtifactRPCHandler struct {
	manager     *ArtifactManager
	plugins     PluginDescriptorResolver
	permissions ArtifactPermissionChecker
}

func NewArtifactRPCHandler(manager *ArtifactManager, plugins PluginDescriptorResolver, permissions ArtifactPermissionChecker) (*ArtifactRPCHandler, error) {
	if manager == nil || plugins == nil || permissions == nil {
		return nil, fmt.Errorf("artifact rpc: manager, plugin resolver and permission checker are required")
	}
	return &ArtifactRPCHandler{manager: manager, plugins: plugins, permissions: permissions}, nil
}

func (h *ArtifactRPCHandler) Register(registry rpc.HandlerRegistry) error {
	if h == nil || registry == nil {
		return fmt.Errorf("artifact rpc: handler and registry are required")
	}
	for _, method := range []rpc.Method{
		MethodArtifactList,
		MethodArtifactDeployRequired,
		MethodArtifactDeploy,
		MethodArtifactVerify,
		MethodArtifactRemove,
	} {
		if err := registry.Register(method, h); err != nil {
			return err
		}
	}
	return nil
}

type artifactRPCInput struct {
	ArtifactID           string `json:"artifactId,omitempty"`
	TargetRoot           string `json:"targetRoot"`
	CompatibilityVersion string `json:"compatibilityVersion,omitempty"`
}

func (h *ArtifactRPCHandler) Handle(ctx context.Context, request rpc.RPCRequest) (rpc.RPCResponse, error) {
	if h == nil || h.manager == nil || h.plugins == nil || h.permissions == nil {
		return rpc.RPCResponse{}, fmt.Errorf("artifact rpc: handler unavailable")
	}
	if strings.TrimSpace(string(request.PluginID)) == "" || strings.TrimSpace(string(request.RuntimeID)) == "" || strings.TrimSpace(string(request.ServiceID)) == "" {
		return rpc.RPCResponse{}, domain.NewHostError(domain.ErrInvalidArgument, "artifact rpc: plugin, runtime and service identity are required")
	}

	decision := h.permissions.CheckServicePermission(
		ctx,
		string(request.RuntimeID),
		string(request.PluginID),
		string(request.ServiceID),
		kernelpermission.PermissionGameHostArtifactDeploy,
	)
	if !decision.Allowed() {
		return rpc.RPCResponse{}, domain.NewHostError(domain.ErrPermissionDenied, "artifact rpc: deployment permission denied")
	}

	descriptor, err := h.plugins.Get(ctx, request.PluginID)
	if err != nil {
		return rpc.RPCResponse{}, fmt.Errorf("artifact rpc: resolve plugin descriptor: %w", err)
	}
	if strings.TrimSpace(descriptor.ExtensionID) == "" {
		return rpc.RPCResponse{}, domain.NewHostError(domain.ErrInvalidArgument, "artifact rpc: plugin has no extension owner")
	}

	var input artifactRPCInput
	if len(request.Payload) > 0 {
		if err := json.Unmarshal(request.Payload, &input); err != nil {
			return rpc.RPCResponse{}, domain.NewHostErrorWithCause(domain.ErrInvalidArgument, "artifact rpc: invalid payload", err)
		}
	}
	input.TargetRoot = strings.TrimSpace(input.TargetRoot)
	input.CompatibilityVersion = strings.TrimSpace(input.CompatibilityVersion)
	input.ArtifactID = strings.TrimSpace(input.ArtifactID)
	if input.TargetRoot == "" {
		return rpc.RPCResponse{}, domain.NewHostError(domain.ErrInvalidArgument, "artifact rpc: targetRoot is required")
	}

	var output any
	switch request.Method {
	case MethodArtifactList:
		items, err := h.manager.List(ctx, descriptor.ExtensionID, input.TargetRoot, input.CompatibilityVersion)
		if err != nil {
			return rpc.RPCResponse{}, err
		}
		output = map[string]any{"items": items}
	case MethodArtifactDeployRequired:
		items, err := h.manager.DeployRequiredArtifacts(ctx, descriptor.ExtensionID, input.TargetRoot, input.CompatibilityVersion)
		if err != nil {
			return rpc.RPCResponse{}, err
		}
		output = map[string]any{"items": items}
	case MethodArtifactDeploy:
		if input.ArtifactID == "" {
			return rpc.RPCResponse{}, domain.NewHostError(domain.ErrInvalidArgument, "artifact rpc: artifactId is required")
		}
		item, err := h.manager.Deploy(ctx, descriptor.ExtensionID, input.ArtifactID, input.TargetRoot, input.CompatibilityVersion)
		if err != nil {
			return rpc.RPCResponse{}, err
		}
		output = item
	case MethodArtifactVerify:
		if input.ArtifactID == "" {
			return rpc.RPCResponse{}, domain.NewHostError(domain.ErrInvalidArgument, "artifact rpc: artifactId is required")
		}
		item, err := h.manager.Verify(ctx, descriptor.ExtensionID, input.ArtifactID, input.TargetRoot, input.CompatibilityVersion)
		if err != nil {
			return rpc.RPCResponse{}, err
		}
		output = item
	case MethodArtifactRemove:
		if input.ArtifactID == "" {
			return rpc.RPCResponse{}, domain.NewHostError(domain.ErrInvalidArgument, "artifact rpc: artifactId is required")
		}
		if err := h.manager.Remove(ctx, descriptor.ExtensionID, input.ArtifactID, input.TargetRoot); err != nil {
			return rpc.RPCResponse{}, err
		}
		output = map[string]any{"removed": true}
	default:
		return rpc.RPCResponse{}, domain.NewHostError(domain.ErrNotFound, "artifact rpc: method not found")
	}

	payload, err := json.Marshal(output)
	if err != nil {
		return rpc.RPCResponse{}, fmt.Errorf("artifact rpc: encode response: %w", err)
	}
	return rpc.RPCResponse{RequestID: request.ID, Payload: payload}, nil
}
