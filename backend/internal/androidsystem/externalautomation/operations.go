package externalautomation

import (
	"github.com/u-ai/backend/internal/androidsystem"
)

const (
	OperationStatus       = "external_automation.status"
	OperationResolveApp   = "external_automation.resolve_app"
	OperationOpenApp      = "external_automation.open_app"
	OperationResolveURI   = "external_automation.resolve_uri"
	OperationOpenURI      = "external_automation.open_uri"
	OperationOpenSettings = "external_automation.open_settings"
	OperationInvokeIntent = "external_automation.invoke_intent"
	OperationForeground   = "external_automation.foreground"
	OperationWaitForeground = "external_automation.wait_foreground"
)

const (
	ToolIDStatus       = "android.external_automation.status"
	ToolIDResolveApp   = "android.external_automation.resolve_app"
	ToolIDOpenApp      = "android.external_automation.open_app"
	ToolIDResolveURI   = "android.external_automation.resolve_uri"
	ToolIDOpenURI      = "android.external_automation.open_uri"
	ToolIDOpenSettings = "android.external_automation.open_settings"
	ToolIDInvokeIntent = "android.external_automation.invoke_intent"
	ToolIDForeground   = "android.external_automation.foreground"
	ToolIDWaitForeground = "android.external_automation.wait_foreground"
)

const (
	PermissionInspect  = androidsystem.PermissionExternalAutomationInspect
	PermissionLaunch   = androidsystem.PermissionExternalAutomationLaunch
	PermissionOpenURI  = androidsystem.PermissionExternalAutomationOpenURI
	PermissionSettings = androidsystem.PermissionExternalAutomationSettings
	PermissionIntent   = androidsystem.PermissionExternalAutomationIntent
)

const (
	RuntimeIDExternalAutomation = "android_native_external_automation"
)
