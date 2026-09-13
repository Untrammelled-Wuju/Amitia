package extension

import (
	"context"

	"github.com/gin-gonic/gin"
	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
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
	service := petcenter.NewServiceFromRuntimeWithPreflight(runtime.Kernel, preflight)
	return &PetPluginAPI{
		handler: petcenter.NewHTTPHandler(service),
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
