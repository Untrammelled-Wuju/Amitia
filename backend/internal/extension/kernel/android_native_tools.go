package kernel

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/androidmedia/camera"
	"github.com/u-ai/backend/internal/androidnative/accessibility"
	"github.com/u-ai/backend/internal/androidnative/adb"
	"github.com/u-ai/backend/internal/androidnative/display"
	"github.com/u-ai/backend/internal/androidnative/interaction"
	"github.com/u-ai/backend/internal/androidnative/root"
	"github.com/u-ai/backend/internal/androidnative/uitree"
	"github.com/u-ai/backend/internal/androidnative/virtualdisplay"
	"github.com/u-ai/backend/internal/androidsystem/clipboard"
	"github.com/u-ai/backend/internal/androidsystem/devicecontrol"
	"github.com/u-ai/backend/internal/androidsystem/externalautomation"
	"github.com/u-ai/backend/internal/androidsystem/notification"
	"github.com/u-ai/backend/internal/androidsystem/overlay"
	"github.com/u-ai/backend/internal/androidsystem/share"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func registerAndroidNativeToolsIfPresent(
	ctx context.Context,
	registry *capability.ToolRegistry,
	provider capability.AndroidProvider,
) error {
	if provider == nil {
		return nil
	}

	tools := collectAndroidNativeToolDefinitions()

	for _, def := range tools {
		if err := registry.Register(ctx, def); err != nil {
			return fmt.Errorf("register android native tool %s: %w", def.ID, err)
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
	defs = append(defs, overlay.BuildOverlayTools()...)
	defs = append(defs, externalautomation.BuildExternalAutomationTools()...)
	defs = append(defs, notification.BuildNotificationTools()...)
	defs = append(defs, clipboard.BuildClipboardTools()...)
	defs = append(defs, devicecontrol.BuildTools()...)
	defs = append(defs, share.BuildShareTools()...)

	cameraTools, err := camera.BuildCameraTools()
	if err != nil {
		return nil
	}
	defs = append(defs, cameraTools...)

	return defs
}
