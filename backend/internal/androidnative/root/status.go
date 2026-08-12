package root

import "github.com/u-ai/backend/internal/androidnative"

func MapRootStatusFromResult(result map[string]any) RootStatus {
	status := RootStatus{}

	if v, ok := result["platformSupported"].(bool); ok {
		status.PlatformSupported = v
	}
	if v, ok := result["rootFramework"].(string); ok {
		status.RootFramework = v
	}
	if v, ok := result["rootManagerDetected"].(bool); ok {
		status.RootManagerDetected = v
	}
	if v, ok := result["suBinaryDetected"].(bool); ok {
		status.SUBinaryDetected = v
	}
	if v, ok := result["authorizationState"].(string); ok {
		status.Authorization = AuthorizationState(v)
	}
	if v, ok := result["rootAvailable"].(bool); ok {
		status.RootAvailable = v
	}
	if v, ok := result["backend"].(string); ok {
		status.Backend = v
	}
	if v, ok := result["state"].(string); ok {
		status.State = v
	}

	return status
}

func DeriveRootState(status RootStatus) string {
	if v := status.State; v != "" {
		return v
	}

	if !status.PlatformSupported {
		return "unsupported"
	}
	if !status.SUBinaryDetected && !status.RootManagerDetected {
		return "not_rooted"
	}
	switch status.Authorization {
	case AuthorizationUnknown:
		return "authorization_unknown"
	case AuthorizationRequired:
		return "authorization_required"
	case AuthorizationGranted:
		return "authorized"
	case AuthorizationDenied:
		return "denied"
	default:
		return "unavailable"
	}
}

const (
	RootStateUnsupported          = "unsupported"
	RootStateNotRooted            = "not_rooted"
	RootStateAuthorizationUnknown = "authorization_unknown"
	RootStateAuthorizationRequired = "authorization_required"
	RootStateAuthorized           = "authorized"
	RootStateDenied               = "denied"
	RootStateUnavailable          = "unavailable"
)

const (
	RootFrameworkMagisk   = "Magisk"
	RootFrameworkKernelSU = "KernelSU"
	RootFrameworkAPatch   = "APatch"
	RootFrameworkGeneric  = "generic-su"
	RootFrameworkUnknown  = "unknown"
)

const (
	PermissionRootInspect  = "android.root.inspect"
	PermissionRootRequest  = "android.root.request"
	PermissionRootExecute  = "android.root.execute"
	PermissionRootShell    = "android.root.shell"
)

func init() {
	_ = androidnative.PROVIDER_UNAVAILABLE
}
