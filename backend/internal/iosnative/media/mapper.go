package media

import "github.com/u-ai/backend/internal/nativebridge"

func AuthorizationStatusFromNative(native string) AuthorizationStatus {
	switch native {
	case "authorized":
		return AuthAuthorized
	case "denied":
		return AuthDenied
	case "restricted":
		return AuthRestricted
	case "limited":
		return AuthLimited
	default:
		return AuthNotDetermined
	}
}

func MapErrorToNativeBridge(err error) (string, string) {
	if err == nil {
		return "", ""
	}
	return ErrOutcomeUnknown, err.Error()
}

func MapCodeToMessage(code string) string {
	switch code {
	case ErrMediaUnsupported:
		return "media is not supported on this device"
	case ErrPhotosPickerUnsupported:
		return "photos picker is not supported"
	case ErrPhotosPickerCancelled:
		return "photos picker was cancelled by user"
	case ErrPhotoLibraryNotAuthorized:
		return "photo library not authorized"
	case ErrPhotoLibraryDenied:
		return "photo library access denied"
	case ErrPhotoLibraryRestricted:
		return "photo library access restricted"
	case ErrPhotoAssetNotFound:
		return "photo asset not found"
	case ErrPhotoExportFailed:
		return "photo export failed"
	case ErrPhotoSaveFailed:
		return "photo save failed"
	case ErrCameraUnsupported:
		return "camera is not supported"
	case ErrCameraNotAuthorized:
		return "camera not authorized"
	case ErrCameraDenied:
		return "camera access denied"
	case ErrCameraDeviceUnavailable:
		return "camera device unavailable"
	case ErrCameraCaptureFailed:
		return "camera capture failed"
	case ErrVideoRecordFailed:
		return "video record failed"
	case ErrVideoRecordInterrupted:
		return "video record was interrupted"
	case ErrMicrophoneNotAuthorized:
		return "microphone not authorized"
	case ErrMicrophoneDenied:
		return "microphone access denied"
	case ErrAudioRecordFailed:
		return "audio record failed"
	case ErrAudioRecordInterrupted:
		return "audio record was interrupted"
	case ErrInvalidRequest:
		return "invalid request"
	case ErrInvalidMediaType:
		return "invalid media type"
	case ErrInvalidRepresentation:
		return "invalid representation"
	case ErrInvalidFlashMode:
		return "invalid flash mode"
	case ErrInvalidFormat:
		return "invalid format"
	case ErrContentTooLarge:
		return "content exceeds maximum size"
	case ErrNativeBridgeUnavailable:
		return "ios native bridge is not available"
	default:
		return code
	}
}

func NewMediaError(request nativebridge.Request, code, message string) nativebridge.Response {
	return nativebridge.Response{
		ProtocolVersion: request.ProtocolVersion,
		RequestId:       request.RequestId,
		Status:          "error",
		Error: &nativebridge.Error{
			Code:    code,
			Message: message,
		},
	}
}
