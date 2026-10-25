package extension

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type DevConsoleAPI struct {
	runtime *Runtime
}

func NewDevConsoleAPI(runtime *Runtime) *DevConsoleAPI {
	return &DevConsoleAPI{runtime: runtime}
}

func (api *DevConsoleAPI) RegisterRoutes(group *gin.RouterGroup) {
	if api.runtime == nil || api.runtime.Kernel == nil {
		return
	}
	container := api.runtime.Kernel.Container()
	if container == nil {
		return
	}
	if container.DevConsoleHandler == nil {
		return
	}

	mux := http.NewServeMux()
	container.DevConsoleHandler.Register(mux)

	group.Any("/dev-console/*consolePath", gin.WrapH(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/api/dev-console" + strings.TrimPrefix(r.URL.Path, "/api/extensions/dev-console")
		mux.ServeHTTP(w, r)
	})))
}
