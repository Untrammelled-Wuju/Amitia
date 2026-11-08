package extension

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	kerneldomain "github.com/u-ai/backend/internal/extension/kernel/domain"
	petcenter "github.com/u-ai/backend/internal/extension/kernel/pet_center"
)

type PetPluginAPI struct {
	handler *petcenter.HTTPHandler
}

func NewPetPluginAPI(runtime *Runtime) *PetPluginAPI {
	if runtime == nil || runtime.Kernel == nil {
		return &PetPluginAPI{}
	}
	guard := kernelruntime.NewTargetMutationGuard(runtime.Kernel.PreviewArchiveTarget)
	preflight := &preflightAdapter{guard: guard}
	service := petcenter.NewServiceFromRuntimeWithPreflight(runtime.Kernel, preflight, petBehaviorToolExecutor(runtime))
	return &PetPluginAPI{
		handler: petcenter.NewHTTPHandler(service),
	}
}

func petBehaviorToolExecutor(runtime *Runtime) petcenter.BehaviorToolExecutor {
	return func(ctx context.Context, extensionID, toolID string, input json.RawMessage, scope petcenter.BehaviorInvocationScope) (json.RawMessage, error) {
		if runtime == nil || runtime.Kernel == nil || runtime.Kernel.Container() == nil {
			return nil, errors.New("pet behavior extension kernel is unavailable")
		}
		container := runtime.Kernel.Container()
		if container.ToolRegistry == nil || container.ToolFacade == nil {
			return nil, errors.New("pet behavior canonical ToolFacade is unavailable")
		}
		definition, ok := container.ToolRegistry.Get(ctx, strings.TrimSpace(toolID))
		if !ok {
			return nil, fmt.Errorf("pet behavior tool %s is not registered", toolID)
		}
		if definition.ExtensionID != extensionID {
			return nil, fmt.Errorf("pet behavior tool %s belongs to extension %s, not %s", toolID, definition.ExtensionID, extensionID)
		}
		if !definition.Enabled {
			return nil, fmt.Errorf("pet behavior tool %s is disabled", toolID)
		}
		result, found := container.ToolFacade.ExecuteTool(
			ctx,
			capability.CapabilityID(toolID),
			input,
			kernelruntime.InvocationScope{
				SpaceID:        scope.SpaceID,
				DeviceID:       scope.DeviceID,
				CharacterID:    scope.CharacterID,
				ConversationID: scope.ConversationID,
				Channel:        scope.Channel,
				SessionID:      scope.SessionID,
				RequestID:      scope.RequestID,
				ToolCallID:     scope.RequestID,
				Source:         "desktop-pet",
			},
			"pet-behavior-"+scope.RequestID,
			"pet-behavior-"+scope.RequestID,
		)
		if !found {
			return nil, fmt.Errorf("pet behavior tool %s is unavailable to ToolFacade", toolID)
		}
		if result.Error != nil {
			return nil, fmt.Errorf("pet behavior tool %s failed: code=%s message=%s detail=%s retryable=%t visible=%s", toolID, result.Error.Code, result.Error.Message, result.Error.Detail, result.Error.Retryable, result.VisibleText)
		}
		if strings.EqualFold(result.Status, "FAILED") {
			return nil, fmt.Errorf("pet behavior tool %s failed with status %s: %s", toolID, result.Status, result.VisibleText)
		}
		if len(result.Output) == 0 {
			return json.RawMessage(`null`), nil
		}
		if !json.Valid(result.Output) {
			return nil, fmt.Errorf("pet behavior tool %s returned invalid JSON: %s", toolID, string(result.Output))
		}
		return append(json.RawMessage(nil), result.Output...), nil
	}
}

type preflightAdapter struct {
	guard *kernelruntime.TargetMutationGuard
}

func (p *preflightAdapter) ValidateArchiveTarget(ctx context.Context, archivePath string, expected kerneldomain.ManagementTarget) (*petcenter.PackageTargetPreview, error) {
	preview, err := p.guard.ValidateArchiveTarget(ctx, archivePath, expected)
	if err != nil {
		return nil, err
	}
	return &petcenter.PackageTargetPreview{
		ExtensionID:      preview.ExtensionID,
		ManagementTarget: expected,
		Installable:      preview.Installable,
	}, nil
}

func (api *PetPluginAPI) RegisterRoutes(group *gin.RouterGroup) {
	if api.handler == nil {
		return
	}
	api.handler.RegisterRoutes(group)
}
