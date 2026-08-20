package extension

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
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
	handler.SetProviderRegistry(container.UIProviderRegistry)

	if container.UIHostNotifier != nil {
		handler.SetDialogResolver(container.UIHostNotifier)
	}

	if container.ClipboardHostBridge != nil {
		handler.SetClipboardResolver(container.ClipboardHostBridge)
	}

	if container.PermissionBroker != nil && container.ScopeManager != nil {
		handler.SetAuthorizer(permission.NewUISessionAuthorizer(container.PermissionBroker, container.ScopeManager))
	}

	if container.ScopeSnapshotCreator != nil {
		handler.SetScopeSnapshotCreator(container.ScopeSnapshotCreator)
	}

	if container.SchemaRegistry != nil {
		handler.SetSchemaLookup(func(extensionID, contributionID string) (json.RawMessage, bool) {
			compiled, ok := container.SchemaRegistry.Get(extensionID, contributionID)
			if !ok || compiled == nil || compiled.Document == nil {
				return nil, false
			}
			data, err := json.Marshal(compiled.Document)
			if err != nil {
				return nil, false
			}
			return data, true
		})
	}

	mux := http.NewServeMux()
	handler.Register(mux)

	extensions.Any("/ui/*uiPath", gin.WrapH(mux))

	if parent != nil {
		parent.Any("/extension/*extPath", gin.WrapH(mux))
	}
}
