package overlay

import (
	"github.com/u-ai/backend/internal/androidsystem"
)

const (
	OperationStatus            = "system.overlay.status"
	OperationPermissionRequest = "system.overlay.permission.request"

	OperationCreate   = "system.overlay.create"
	OperationUpdate   = "system.overlay.update"
	OperationShow     = "system.overlay.show"
	OperationHide     = "system.overlay.hide"
	OperationClose    = "system.overlay.close"
	OperationList     = "system.overlay.list"
	OperationCloseAll = "system.overlay.close_all"
)

const (
	ToolIDStatus            = "android.overlay.status"
	ToolIDPermissionRequest = "android.overlay.permission_request"
	ToolIDCreate            = "android.overlay.create"
	ToolIDUpdate            = "android.overlay.update"
	ToolIDShow              = "android.overlay.show"
	ToolIDHide              = "android.overlay.hide"
	ToolIDClose             = "android.overlay.close"
	ToolIDList              = "android.overlay.list"
	ToolIDCloseAll          = "android.overlay.close_all"
)

const (
	PermissionInspect = androidsystem.PermissionOverlayInspect
	PermissionCreate  = androidsystem.PermissionOverlayCreate
)

const (
	RuntimeIDOverlay = "android_native_overlay"
)
