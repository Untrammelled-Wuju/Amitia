package androidmedia

const (
	ANDROID_MEDIA_FFMPEG_UNAVILABLE           = "ANDROID_MEDIA_FFMPEG_UNAVAILABLE"
	ANDROID_MEDIA_FFMPEG_RUNTIME_PACKAGE_MISSING = "ANDROID_MEDIA_FFMPEG_RUNTIME_PACKAGE_MISSING"
	ANDROID_MEDIA_FFMPEG_ARCH_UNSUPPORTED      = "ANDROID_MEDIA_FFMPEG_ARCH_UNSUPPORTED"

	SCREENSHOT_UNSUPPORTED              = "SCREENSHOT_UNSUPPORTED"
	SCREENSHOT_ACCESSIBILITY_DISABLED   = "SCREENSHOT_ACCESSIBILITY_DISABLED"
	SCREENSHOT_ACCESSIBILITY_NOT_CONNECTED = "SCREENSHOT_ACCESSIBILITY_NOT_CONNECTED"
	SCREENSHOT_CAPABILITY_NOT_DECLARED  = "SCREENSHOT_CAPABILITY_NOT_DECLARED"
	SCREENSHOT_INVALID_DISPLAY          = "SCREENSHOT_INVALID_DISPLAY"
	SCREENSHOT_INTERVAL_TOO_SHORT       = "SCREENSHOT_INTERVAL_TOO_SHORT"
	SCREENSHOT_SECURE_CONTENT           = "SCREENSHOT_SECURE_CONTENT"
	SCREENSHOT_CAPTURE_FAILED           = "SCREENSHOT_CAPTURE_FAILED"
	SCREENSHOT_ENCODE_FAILED            = "SCREENSHOT_ENCODE_FAILED"
	SCREENSHOT_TOO_LARGE                = "SCREENSHOT_TOO_LARGE"
	SCREENSHOT_ARTIFACT_WRITE_FAILED    = "SCREENSHOT_ARTIFACT_WRITE_FAILED"
	SCREENSHOT_RESOURCE_INVALID         = "SCREENSHOT_RESOURCE_INVALID"
	SCREENSHOT_CANCELLED                = "SCREENSHOT_CANCELLED"
	SCREENSHOT_RESOURCE_EXHAUSTED       = "SCREENSHOT_RESOURCE_EXHAUSTED"
	BLOCKED_ANDROID_NATIVE_HOST_SOURCE  = "BLOCKED_ANDROID_NATIVE_HOST_SOURCE"
	BLOCKED_BY_FROZEN_A_CONTRACT       = "BLOCKED_BY_FROZEN_A_CONTRACT"
)

const (
	PermissionScreenCapture = "android.media.screen_capture"
)

const (
	OperationScreenshotCapture = "media.screenshot.capture"
	OperationScreenshotStatus  = "media.screenshot.status"
)

const (
	OperationScreenFrameStatus  = "screen_frame.status"
	OperationScreenFrameStart   = "screen_frame.start"
	OperationScreenFrameLatest  = "screen_frame.latest"
	OperationScreenFrameStop    = "screen_frame.stop"
)

const (
	PermissionContinuousCapture = "android.media.screen_capture.continuous"
	PermissionCamera            = "android.media.camera"
	PermissionMediaRead         = "android.media.read"
	PermissionFFmpeg            = "android.media.ffmpeg"
)

const (
	ToolIDCameraStatus  = "android.camera.status"
	ToolIDCameraList    = "android.camera.list"
	ToolIDCameraCapture = "android.camera.capture"
)

const (
	ToolIDMediaReadInfo  = "android.media.read.info"
	ToolIDMediaReadImage = "android.media.read.image"
)

const (
	ToolIDScreenshot    = "android.screen.screenshot"
	HandlerScreenshot   = OperationScreenshotCapture
)

const (
	ToolIDFFmpegStatus = "android.media.ffmpeg.status"
)

const (
	DefaultScreenshotFormat     = "png"
	DefaultJPEGQuality          = 90
	DefaultMaxScreenshotPixels  = 16_000_000
	DefaultMaxEncodedBytes      = 50 * 1024 * 1024
)
