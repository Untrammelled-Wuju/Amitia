package extension

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/ui_handler"
	"github.com/u-ai/backend/internal/extension/kernel/ui_provider"
	"github.com/u-ai/backend/internal/runtimeidentity"
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
	handler.SetProviderContextResolver(func(r *http.Request, platform string) ui_provider.ResolveContext {
		resolveContext := ui_provider.ResolveContext{
			Platform:       platform,
			RuntimeProfile: container.RuntimeProfile.String(),
			Architecture:   strings.TrimSpace(r.Header.Get("X-Amitia-Architecture")),
			AppVersion:     strings.TrimSpace(r.Header.Get("X-Amitia-App-Version")),
			LocalRuntime:   container.RuntimeProfile.String() == "local" || container.RuntimeProfile.String() == "device-agent",
		}
		// UI compatibility metadata is supplied by the rendering client. Headers are
		// preferred for native hosts, while query fallbacks keep the existing UI API
		// usable from Web/Flutter clients that share the generic transport layer.
		if resolveContext.Architecture == "" {
			resolveContext.Architecture = strings.TrimSpace(r.URL.Query().Get("architecture"))
		}
		if resolveContext.AppVersion == "" {
			resolveContext.AppVersion = strings.TrimSpace(r.URL.Query().Get("appVersion"))
		}
		if actor, ok := auth.FromContext(r.Context()); ok && actor != nil {
			resolveContext.UserID = actor.UserID.String()
			resolveContext.DeviceID = actor.DeviceID.String()
		}
		if resolveContext.DeviceID == "" {
			resolveContext.DeviceID = strings.TrimSpace(r.URL.Query().Get("deviceId"))
		}
		if resolveContext.DeviceID == "" {
			resolveContext.DeviceID = strings.TrimSpace(r.Header.Get("X-Amitia-Device-ID"))
		}

		userID := runtimeidentity.ParseUserID(resolveContext.UserID)
		deviceID := runtimeidentity.ParseDeviceID(resolveContext.DeviceID)
		if userID != "" && deviceID != "" && container.DeviceRegistry != nil {
			if device, err := container.DeviceRegistry.GetDevice(r.Context(), deviceID); err == nil && device != nil {
				if device.UserID != userID {
					resolveContext.DeviceID = ""
					deviceID = ""
				} else if resolveContext.Platform == "" && device.Platform != "" {
					resolveContext.Platform = device.Platform.String()
				}
			}
		}

		capabilitySet := map[string]struct{}{}
		if userID != "" && deviceID != "" && container.DeviceRuntimeSessions != nil {
			if sessions, err := container.DeviceRuntimeSessions.ListActiveSessions(r.Context()); err == nil {
				for _, session := range sessions {
					if session.UserID != userID || session.DeviceID != deviceID || !session.IsActive() {
						continue
					}
					resolveContext.DeviceOnline = true
					if resolveContext.RuntimeVersion == "" {
						resolveContext.RuntimeVersion = session.RuntimeVersion
					}
					for _, capability := range session.Capabilities {
						capabilitySet[capability] = struct{}{}
					}
				}
			}
		}
		if !resolveContext.DeviceOnline && userID != "" && deviceID != "" && container.DeviceRegistry != nil {
			if presence, err := container.DeviceRegistry.GetDevicePresence(r.Context(), userID, deviceID); err == nil {
				resolveContext.DeviceOnline = presence.State == host_registry.PresenceStateReady
			}
		}
		for capability := range capabilitySet {
			resolveContext.DeviceCapabilities = append(resolveContext.DeviceCapabilities, capability)
		}
		return resolveContext.Normalize()
	})

	if container.UIHostNotifier != nil {
		handler.SetDialogResolver(container.UIHostNotifier)
		handler.SetProviderChangeBroadcaster(func(eventType string, extra map[string]interface{}) {
			container.UIHostNotifier.BroadcastExtensionChange(eventType, "builtin.amitia.ui", extra)
		})
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
