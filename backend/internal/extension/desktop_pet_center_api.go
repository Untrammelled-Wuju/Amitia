package extension

import (
	"context"

	"github.com/gin-gonic/gin"
	kerneldomain "github.com/u-ai/backend/internal/extension/kernel/domain"
	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
	desktoppetcenter "github.com/u-ai/backend/internal/extension/kernel/desktop_pet_center"
)

type DesktopPetPluginAPI struct {
	handler *desktoppetcenter.HTTPHandler
}

func NewDesktopPetPluginAPI(runtime *Runtime) *DesktopPetPluginAPI {
	if runtime == nil || runtime.Kernel == nil {
		return &DesktopPetPluginAPI{}
	}
	guard := kernelruntime.NewTargetMutationGuard(runtime.Kernel.PreviewArchiveTarget)
	preflight := &preflightAdapter{guard: guard}
	service := desktoppetcenter.NewServiceFromRuntimeWithPreflight(runtime.Kernel, preflight)
	return &DesktopPetPluginAPI{
		handler: desktoppetcenter.NewHTTPHandler(service),
	}
}

type preflightAdapter struct {
	guard *kernelruntime.TargetMutationGuard
}

func (p *preflightAdapter) ValidateArchiveTarget(ctx context.Context, archivePath string, expected kerneldomain.ManagementTarget) (*desktoppetcenter.PackageTargetPreview, error) {
	preview, err := p.guard.ValidateArchiveTarget(ctx, archivePath, expected)
	if err != nil {
		return nil, err
	}
	return &desktoppetcenter.PackageTargetPreview{
		ExtensionID:      preview.ExtensionID,
		ManagementTarget: expected,
		Installable:      preview.Installable,
	}, nil
}

func (api *DesktopPetPluginAPI) RegisterRoutes(group *gin.RouterGroup) {
	if api.handler == nil {
		return
	}
	api.handler.RegisterRoutes(group)
}
