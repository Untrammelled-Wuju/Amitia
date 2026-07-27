package extension

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/extension/kernel/ui_handler"
)

type UIAPI struct {
	runtime *Runtime
}

func NewUIAPI(runtime *Runtime) *UIAPI {
	return &UIAPI{runtime: runtime}
}

func (api *UIAPI) RegisterRoutes(extensions *gin.RouterGroup, parent *gin.RouterGroup) {
	if api.runtime == nil || api.runtime.Kernel == nil {
		return
	}
	container := api.runtime.Kernel.Container()
	if container == nil {
		return
	}
	if container.UIHost == nil || container.SlotRegistry == nil || container.PageHost == nil {
		return
	}

	handler := ui_handler.NewHTTPHandler(
		container.UIHost,
		container.SlotRegistry,
		container.PageHost,
		container.SandboxHost,
		container.ChatExtensionRegistry,
	)
	handler.SetExtensionRoot(container.ExtRoot)

	mux := http.NewServeMux()
	handler.Register(mux)

	extensions.Any("/ui/*uiPath", gin.WrapH(mux))
	extensions.POST("/:id/pages/:pageId/open", gin.WrapH(mux))
	extensions.GET("/:id/ui", gin.WrapH(mux))

	if parent != nil {
		parent.Any("/extension/*extPath", gin.WrapH(mux))
	}
}
