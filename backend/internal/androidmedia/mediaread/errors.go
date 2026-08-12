package mediaread

const (
	MediaReadInvalidURI          = "MEDIA_READ_INVALID_URI"
	MediaReadResourceNotFound    = "MEDIA_READ_RESOURCE_NOT_FOUND"
	MediaReadPermissionDenied    = "MEDIA_READ_PERMISSION_DENIED"

	MediaReadUnsupportedFormat   = "MEDIA_READ_UNSUPPORTED_FORMAT"
	MediaReadInvalidImage        = "MEDIA_READ_INVALID_IMAGE"
	MediaReadTooLarge            = "MEDIA_READ_TOO_LARGE"

	MediaReadDecodeFailed        = "MEDIA_READ_DECODE_FAILED"
	MediaReadNormalizeFailed     = "MEDIA_READ_NORMALIZE_FAILED"
	MediaReadArtifactFailed      = "MEDIA_READ_ARTIFACT_FAILED"

	MediaReadTimeout             = "MEDIA_READ_TIMEOUT"
	MediaReadCancelled           = "MEDIA_READ_CANCELLED"
)

const (
	MediaReadOCROrigin           = "imageintelligence"
	MediaReadUnderstandOrigin    = "imageintelligence"
)

type MediaReadError struct {
	Code    string
	Message string
}

func (e *MediaReadError) Error() string {
	if e == nil {
		return ""
	}
	return e.Code + ": " + e.Message
}
