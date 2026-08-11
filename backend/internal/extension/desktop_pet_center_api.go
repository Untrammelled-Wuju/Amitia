package extension

import (
	"github.com/gin-gonic/gin"
	desktoppetcenter "github.com/u-ai/backend/internal/extension/kernel/desktop_pet_center"
)

type DesktopPetPluginAPI struct {
	handler *desktoppetcenter.HTTPHandler
}

func NewDesktopPetPluginAPI(runtime *Runtime) *DesktopPetPluginAPI {
	if runtime == nil || runtime.Kernel == nil {
		return &DesktopPetPluginAPI{}
	}
	service := desktoppetcenter.NewServiceFromRuntime(runtime.Kernel)
	return &DesktopPetPluginAPI{
		handler: desktoppetcenter.NewHTTPHandler(service),
	}
}

func (api *DesktopPetPluginAPI) RegisterRoutes(group *gin.RouterGroup) {
	if api.handler == nil {
		return
	}
	api.handler.RegisterRoutes(group)
}
