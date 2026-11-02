package extension

import (
	"net/http"

	"github.com/gin-gonic/gin"
	extensionkernel "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/extension_center"
	"github.com/u-ai/backend/internal/extension/kernel/extension_detail"
)

func RegisterExtensionCenterRoutes(group *gin.RouterGroup, runtime *Runtime) {
	if group == nil || runtime == nil || runtime.Kernel == nil {
		return
	}
	RegisterExtensionCenterRoutesForContainer(group, runtime.Kernel.Container())
}

func RegisterExtensionCenterRoutesForContainer(group *gin.RouterGroup, container *extensionkernel.Container) {
	if group == nil {
		return
	}
	if container == nil || container.ExtensionCenterService == nil || container.ExtensionDetailService == nil {
		return
	}
	centerHandler := extension_center.NewHTTPHandler(container.ExtensionCenterService)
	detailHandler := extension_detail.NewHTTPHandler(container.ExtensionDetailService)
	centerMux := http.NewServeMux()
	centerHandler.Register(centerMux)
	detailHandler.Register(centerMux)
	group.Any("/extension-center/*centerPath", func(c *gin.Context) {
		originalPath := c.Request.URL.Path
		c.Request.URL.Path = "/api/extension-center" + c.Param("centerPath")
		centerMux.ServeHTTP(c.Writer, c.Request)
		c.Request.URL.Path = originalPath
	})
}
