package kernel

import (
	"context"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/androidmedia/camera"
	"github.com/u-ai/backend/internal/androidmedia/screenshot"
	"github.com/u-ai/backend/internal/androidnative/accessibility"
	"github.com/u-ai/backend/internal/androidnative/adb"
	"github.com/u-ai/backend/internal/androidnative/display"
	"github.com/u-ai/backend/internal/androidnative/interaction"
	"github.com/u-ai/backend/internal/androidnative/root"
	"github.com/u-ai/backend/internal/androidnative/shizuku"
	"github.com/u-ai/backend/internal/androidnative/uitree"
	"github.com/u-ai/backend/internal/androidnative/virtualdisplay"
	"github.com/u-ai/backend/internal/androidsystem/clipboard"
	"github.com/u-ai/backend/internal/androidsystem/devicecontrol"
	"github.com/u-ai/backend/internal/androidsystem/externalautomation"
	"github.com/u-ai/backend/internal/androidsystem/notification"
	"github.com/u-ai/backend/internal/androidsystem/overlay"
	"github.com/u-ai/backend/internal/androidsystem/share"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func registerAndroidNativeToolsIfPresent(
	ctx context.Context,
	registry *capability.ToolRegistry,
	providerRegistry *capability.ProviderRegistry,
	provider capability.AndroidProvider,
) error {
	if provider == nil {
		return nil
	}

	tools := collectAndroidNativeToolDefinitions()

	for _, def := range tools {
		capabilityID := def.CapabilityID
		if capabilityID == "" {
			capabilityID = capability.CapabilityID(def.ID)
		}
		def.CapabilityID = capabilityID
		providerID := capability.ProviderID("builtin.android_native/" + strings.ReplaceAll(string(capabilityID), "/", "."))
		def.ProviderID = string(providerID)
		def.RoutingMode = capability.RoutingModeProviderRequired
		if err := registry.Register(ctx, def); err != nil {
			return fmt.Errorf("register android native tool %s: %w", def.ID, err)
		}
		if providerRegistry == nil {
			continue
		}
		providerDefinition := capability.CapabilityProviderDefinition{
			ID:           providerID,
			CapabilityID: capabilityID,
			Kind:         capability.ProviderKindBuiltin,
			Placement:    capability.ProviderPlacementCore,
			Runtime:      def.Runtime,
			Priority:     100,
			Platforms:    []runtimeidentity.Platform{runtimeidentity.PlatformAndroid},
			Metadata: map[string]any{
				"androidNative": true,
				"toolId":        def.ID,
				"modelName":     def.ModelName,
			},
		}
		if err := providerRegistry.RegisterDefinition(providerDefinition); err != nil {
			return fmt.Errorf("register android native provider %s: %w", providerID, err)
		}
		instance := capability.CapabilityProviderInstance{
			ID:           capability.ProviderInstanceID("builtin.android_native/" + strings.ReplaceAll(string(capabilityID), "/", ".") + "/instance"),
			ProviderID:   providerID,
			CapabilityID: capabilityID,
			Placement:    capability.ProviderPlacementCore,
			Health:       capability.HealthReady,
			Availability: capability.ProviderAvailabilityAvailable,
			Metadata: map[string]any{
				"androidNative": true,
			},
		}
		if err := providerRegistry.RegisterInstance(instance); err != nil {
			return fmt.Errorf("register android native provider instance %s: %w", instance.ID, err)
		}
	}

	return nil
}

func collectAndroidNativeToolDefinitions() []capability.ToolDefinition {
	var defs []capability.ToolDefinition

	defs = append(defs, accessibility.BuildAccessibilityTools()...)
	defs = append(defs, root.BuildRootTools()...)
	defs = append(defs, display.BuildDisplayTools()...)
	defs = append(defs, uitree.BuildUITreeTools()...)
	defs = append(defs, interaction.BuildInteractionTools()...)
	defs = append(defs, virtualdisplay.BuildVirtualDisplayTools()...)
	defs = append(defs, adb.BuildADBTools()...)
	defs = append(defs, shizuku.BuildShizukuTools()...)
	defs = append(defs, overlay.BuildOverlayTools()...)
	defs = append(defs, externalautomation.BuildExternalAutomationTools()...)
	defs = append(defs, notification.BuildNotificationTools()...)
	defs = append(defs, clipboard.BuildClipboardTools()...)
	defs = append(defs, devicecontrol.BuildTools()...)
	defs = append(defs, share.BuildShareTools()...)

	if screenshotTool, err := screenshot.BuildToolDefinition(); err == nil {
		defs = append(defs, screenshotTool)
		captureAlias := screenshotTool
		captureAlias.ID = "android.screen.capture_screenshot"
		captureAlias.ModelName = "capture_screenshot"
		captureAlias.Name = "Capture Screenshot"
		captureAlias.Description = "Capture one static Android screenshot and return an amitia:// resource reference. This is the compatibility alias for capture_screenshot."
		captureAlias.ToolVersion.Revision = "b34-screen-shot-alias-v1"
		defs = append(defs, captureAlias)
	}

	if cameraTools, err := camera.BuildCameraTools(); err == nil {
		defs = append(defs, cameraTools...)
	}

	return defs
}
