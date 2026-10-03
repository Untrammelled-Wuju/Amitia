//go:build linux

package main

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/androidmedia"
	"github.com/u-ai/backend/internal/androidnative"
	"github.com/u-ai/backend/internal/androidnative/accessibility"
	"github.com/u-ai/backend/internal/androidnative/adb"
	"github.com/u-ai/backend/internal/androidnative/display"
	"github.com/u-ai/backend/internal/androidnative/interaction"
	"github.com/u-ai/backend/internal/androidnative/root"
	androidshizuku "github.com/u-ai/backend/internal/androidnative/shizuku"
	"github.com/u-ai/backend/internal/androidnative/uitree"
	"github.com/u-ai/backend/internal/androidnative/virtualdisplay"
	"github.com/u-ai/backend/internal/androidsystem"
	"github.com/u-ai/backend/internal/androidsystem/clipboard"
	"github.com/u-ai/backend/internal/androidsystem/devicecontrol"
	"github.com/u-ai/backend/internal/androidsystem/externalautomation"
	"github.com/u-ai/backend/internal/androidsystem/notification"
	"github.com/u-ai/backend/internal/androidsystem/overlay"
	"github.com/u-ai/backend/internal/androidsystem/share"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/imageintelligence"
	"github.com/u-ai/backend/internal/nativebridge"
	"github.com/u-ai/backend/pkg/resourceuri"
)

type systemHandlerAdapter struct {
	systemHandler SystemHandler
}

type SystemHandler interface {
	Execute(ctx context.Context, request androidsystem.SystemRequest) androidsystem.SystemResponse
}

func (a *systemHandlerAdapter) Execute(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	sysReq := androidsystem.SystemRequest{
		RequestID: request.RequestID,
		Operation: request.Operation,
		Payload:   request.Payload,
	}
	sysResp := a.systemHandler.Execute(ctx, sysReq)
	return capability.AndroidBridgeResponse{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       sysResp.RequestID,
		Status:          sysResp.Status,
		Result:          sysResp.Result,
		Error:           convertSystemError(sysResp.Error),
	}
}

func convertSystemError(err *androidsystem.SystemError) *capability.AndroidError {
	if err == nil {
		return nil
	}
	return &capability.AndroidError{
		Code:       err.Code,
		Message:    err.Message,
		DomainCode: err.DomainCode,
	}
}

func applyAndroidNativeProvider(
	builder *kernel.ContainerBuilder,
	bootstrap *runtimeBootstrap,
	imageIntelligence imageintelligence.ImageIntelligence,
	resourceResolver *resourceuri.PhysicalResolver,
	dataDir string,
) (*kernel.ContainerBuilder, error) {
	bridge := bootstrap.AndroidNativeBridge()
	if bridge == nil {
		return builder, nil
	}

	provider := androidnative.NewProvider(bridge)

	if err := registerAndroidNativeDomainHandlers(provider, bridge, imageIntelligence, resourceResolver, dataDir); err != nil {
		return nil, fmt.Errorf("android native: register domain handlers: %w", err)
	}

	if err := registerAndroidSystemDomainHandlers(provider, bridge); err != nil {
		return nil, fmt.Errorf("android native: register system domain handlers: %w", err)
	}

	return builder.WithAndroidNativeProvider(provider), nil
}

func registerAndroidNativeDomainHandlers(
	provider *androidnative.Provider,
	bridge nativebridge.Bridge,
	imageIntelligence imageintelligence.ImageIntelligence,
	resourceResolver *resourceuri.PhysicalResolver,
	dataDir string,
) error {
	accessibilityHandler := accessibility.NewAccessibilityHandler(bridge)
	for _, op := range []string{
		accessibility.OperationStatus,
		accessibility.OperationOpenSettings,
		accessibility.OperationProviderStatus,
		accessibility.OperationProviderInstall,
		accessibility.OperationProviderOpenSettings,
	} {
		if err := provider.RegisterHandler(op, accessibilityHandler); err != nil {
			return fmt.Errorf("accessibility handler: %w", err)
		}
	}

	rootHandler := root.NewRootHandler(bridge)
	for _, op := range []string{root.OperationStatus, root.OperationRequest, root.OperationExecute} {
		if err := provider.RegisterHandler(op, rootHandler); err != nil {
			return fmt.Errorf("root handler: %w", err)
		}
	}

	shizukuHandler := androidshizuku.NewHandler(bridge)
	for _, op := range androidshizuku.Operations {
		if err := provider.RegisterHandler(op, shizukuHandler); err != nil {
			return fmt.Errorf("shizuku handler: %w", err)
		}
	}

	displayService, err := display.Register(provider, bridge, nil)
	if err != nil {
		return fmt.Errorf("display handler: %w", err)
	}
	_ = displayService

	uitreeService, err := buildUITreeService(bridge)
	if err != nil {
		return fmt.Errorf("uitree service: %w", err)
	}
	uitreeHandler := uitree.NewHandler(uitreeService)
	for _, op := range []string{uitree.OperationStatus, uitree.OperationSnapshot, uitree.OperationFind, uitree.OperationGet} {
		if err := provider.RegisterHandler(op, uitreeHandler); err != nil {
			return fmt.Errorf("uitree handler: %w", err)
		}
	}

	interactionService, err := buildInteractionService(bridge, uitreeService, imageIntelligence, resourceResolver, dataDir)
	if err != nil {
		return fmt.Errorf("interaction service: %w", err)
	}
	interactionHandler := interaction.NewHandler(interactionService)
	interactionOps := []string{
		interaction.OperationStatus,
		interaction.OperationClick,
		interaction.OperationLongClick,
		interaction.OperationInputText,
		interaction.OperationClearText,
		interaction.OperationScroll,
		interaction.OperationSwipe,
		interaction.OperationNodeAction,
		interaction.OperationGlobalAction,
		interaction.OperationGesture,
		interaction.OperationScreenshot,
		interaction.OperationVisualLocate,
		interaction.OperationVisualClick,
	}
	for _, op := range interactionOps {
		if err := provider.RegisterHandler(op, interactionHandler); err != nil {
			return fmt.Errorf("interaction handler: %w", err)
		}
	}

	if _, err := virtualdisplay.Register(provider, bridge); err != nil {
		return fmt.Errorf("virtualdisplay handler: %w", err)
	}

	adbConfig := adb.DefaultConfig()
	adbHandler := adb.NewADBHandler(adbConfig)
	for _, op := range []string{adb.OperationStatus, adb.OperationDevices, adb.OperationExecute} {
		if err := provider.RegisterHandler(op, adbHandler); err != nil {
			return fmt.Errorf("adb handler: %w", err)
		}
	}

	screenshotHandler := newMediaScreenshotHandler(bridge, resourceResolver, dataDir)
	if err := provider.RegisterHandler(androidmedia.OperationScreenshotCapture, screenshotHandler); err != nil {
		return fmt.Errorf("screenshot handler: %w", err)
	}

	return nil
}

func buildUITreeService(bridge nativebridge.Bridge) (*uitree.Service, error) {
	policy := uitree.DefaultPolicy()
	var sources uitree.SourceSet
	accessibilitySource := uitree.NewAccessibilitySource(bridge, policy)
	sources.Accessibility = accessibilitySource
	adbHandler := adb.NewADBHandler(adb.DefaultConfig())
	rootHandler := root.NewRootHandler(bridge)
	if bridge != nil {
		sources.ADB = uitree.NewADBSource(adbHandler.InternalExecutor(), policy)
		sources.Root = uitree.NewRootSource(rootHandler.InternalExecutor(), policy)
	}
	return uitree.NewService(sources, policy), nil
}

func buildInteractionService(
	bridge nativebridge.Bridge,
	uitreeService *uitree.Service,
	imageIntelligence imageintelligence.ImageIntelligence,
	resourceResolver *resourceuri.PhysicalResolver,
	dataDir string,
) (*interaction.Service, error) {
	var nodeResolver uitree.NodeResolver
	var snapshotResolver uitree.SnapshotResolver
	if uitreeService != nil {
		nodeResolver = uitreeService.NodeResolver()
		snapshotResolver = uitreeService.SnapshotResolver()
	}

	accessibilityExecutor := interaction.NewBridgeAccessibilityExecutor(bridge)
	coordinateExecutor := interaction.NewBridgeCoordinateExecutor(bridge)
	policy := interaction.DefaultPolicy()
	meta := newScreenshotMetaRegistry()
	var screenshot interaction.ScreenshotProvider
	var ocr interaction.ImageIntelligenceOCR
	var understand interaction.ImageIntelligenceUnderstand
	if bridge != nil && resourceResolver != nil && dataDir != "" {
		screenshot = &bridgeScreenshotProvider{bridge: bridge, resolver: resourceResolver, dataDir: dataDir, meta: meta}
	}
	if imageIntelligence != nil {
		ocr = &imageOCRAdapter{image: imageIntelligence, meta: meta}
		understand = &imageUnderstandAdapter{image: imageIntelligence, meta: meta}
	}
	visualLocator := interaction.NewDefaultVisualLocator(screenshot, ocr, understand, nodeResolver, policy)
	verifier := interaction.NewDefaultVerifier(snapshotResolver, screenshot, policy)

	rootExecutor := interaction.NewRootExecutor(root.NewRootHandler(bridge), policy)
	adbExecutor := interaction.NewADBExecutor(adb.NewADBHandler(adb.DefaultConfig()), policy)
	shizukuExecutor := interaction.NewShizukuExecutor(bridge, policy)

	return interaction.NewService(
		nodeResolver,
		snapshotResolver,
		accessibilityExecutor,
		coordinateExecutor,
		visualLocator,
		rootExecutor,
		adbExecutor,
		shizukuExecutor,
		verifier,
		policy,
	), nil
}

func registerAndroidSystemDomainHandlers(
	provider *androidnative.Provider,
	bridge nativebridge.Bridge,
) error {
	clipboardHandler := clipboard.NewClipboardHandler(
		clipboard.NewNativeBridgeClipboardClient(bridge),
	)
	for _, op := range []string{
		clipboard.OperationStatus,
		clipboard.OperationReadText,
		clipboard.OperationWriteText,
		clipboard.OperationClear,
	} {
		if err := provider.RegisterHandler(op, &systemHandlerAdapter{systemHandler: clipboardHandler}); err != nil {
			return fmt.Errorf("clipboard handler: %w", err)
		}
	}

	shareHandler := share.NewShareHandler(
		share.NewNativeBridgeShareClient(bridge),
	)
	for _, op := range []string{share.OperationStatus, share.OperationSend} {
		if err := provider.RegisterHandler(op, &systemHandlerAdapter{systemHandler: shareHandler}); err != nil {
			return fmt.Errorf("share handler: %w", err)
		}
	}

	overlayHandler := overlay.NewOverlayHandler(
		overlay.NewNativeBridgeOverlayClient(bridge),
	)
	overlayOps := []string{
		overlay.OperationStatus,
		overlay.OperationPermissionRequest,
		overlay.OperationCreate,
		overlay.OperationUpdate,
		overlay.OperationShow,
		overlay.OperationHide,
		overlay.OperationClose,
		overlay.OperationList,
		overlay.OperationCloseAll,
	}
	for _, op := range overlayOps {
		if err := provider.RegisterHandler(op, &systemHandlerAdapter{systemHandler: overlayHandler}); err != nil {
			return fmt.Errorf("overlay handler: %w", err)
		}
	}

	externalAutomationHandler := externalautomation.NewExternalAutomationHandler(
		externalautomation.NewNativeBridgeExternalAutomationClient(bridge),
	)
	externalAutomationOps := []string{
		externalautomation.OperationStatus,
		externalautomation.OperationResolveApp,
		externalautomation.OperationOpenApp,
		externalautomation.OperationResolveURI,
		externalautomation.OperationOpenURI,
		externalautomation.OperationOpenSettings,
		externalautomation.OperationInvokeIntent,
		externalautomation.OperationForeground,
		externalautomation.OperationWaitForeground,
	}
	for _, op := range externalAutomationOps {
		if err := provider.RegisterHandler(op, &systemHandlerAdapter{systemHandler: externalAutomationHandler}); err != nil {
			return fmt.Errorf("external automation handler: %w", err)
		}
	}

	deviceControlHandler := devicecontrol.NewHandler(bridge)
	for _, op := range devicecontrol.Operations {
		if err := provider.RegisterHandler(op, &systemHandlerAdapter{systemHandler: deviceControlHandler}); err != nil {
			return fmt.Errorf("device control handler: %w", err)
		}
	}

	notificationProvider := notification.NewNativeBridgeNotificationProvider(bridge)
	notificationHandler := notification.NewNotificationHandler(notificationProvider)
	notificationOps := []string{
		notification.OperationStatus,
		notification.OperationList,
		notification.OperationGet,
		notification.OperationPost,
		notification.OperationCancelOwn,
		notification.OperationDismiss,
		notification.OperationOpen,
		notification.OperationInvokeAction,
	}
	for _, op := range notificationOps {
		if err := provider.RegisterHandler(op, &systemHandlerAdapter{systemHandler: notificationHandler}); err != nil {
			return fmt.Errorf("notification handler: %w", err)
		}
	}

	return nil
}
