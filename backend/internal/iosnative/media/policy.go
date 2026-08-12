package media

import "fmt"

const (
	DefaultPhotoListLimit = 50
	MaxPhotoListLimit     = 200
	DefaultSelectionLimit = 1
	MaxSelectionLimit     = 10
	DefaultMaxTotalBytes  = 50 * 1024 * 1024
	MaxExportBytes        = 100 * 1024 * 1024
	DefaultVideoMaxDurationMs = 300000
	MaxVideoMaxDurationMs     = 600000
	DefaultAudioMaxDurationMs = 300000
	MaxAudioMaxDurationMs     = 600000
	DefaultAudioSampleRate    = 44100
	DefaultAudioChannels      = 1
)

var AllowedPickerKinds = []string{"image", "video", "live_photo"}

var AllowedRepresentations = []string{"current", "original", "compatible"}

var AllowedFlashModes = []string{"auto", "on", "off"}

var AllowedTorchModes = []string{"auto", "on", "off"}

var AllowedQualities = []string{"high", "medium", "low"}

var AllowedFormats = []string{"heif", "jpeg", "m4a", "caf"}

var AllowedAudioFormats = []string{"m4a", "caf"}

func IsValidPickerKind(k string) bool {
	for _, v := range AllowedPickerKinds {
		if v == k {
			return true
		}
	}
	return false
}

func IsValidRepresentation(r string) bool {
	for _, v := range AllowedRepresentations {
		if v == r {
			return true
		}
	}
	return false
}

func IsValidFlashMode(f string) bool {
	for _, v := range AllowedFlashModes {
		if v == f {
			return true
		}
	}
	return false
}

func IsValidTorchMode(t string) bool {
	for _, v := range AllowedTorchModes {
		if v == t {
			return true
		}
	}
	return false
}

func IsValidQuality(q string) bool {
	for _, v := range AllowedQualities {
		if v == q {
			return true
		}
	}
	return false
}

func IsValidFormat(f string) bool {
	for _, v := range AllowedFormats {
		if v == f {
			return true
		}
	}
	return false
}

func IsValidAudioFormat(f string) bool {
	for _, v := range AllowedAudioFormats {
		if v == f {
			return true
		}
	}
	return false
}

func ClampPhotoListLimit(n int) int {
	if n <= 0 {
		return DefaultPhotoListLimit
	}
	if n > MaxPhotoListLimit {
		return MaxPhotoListLimit
	}
	return n
}

func ClampSelectionLimit(n int) int {
	if n <= 0 {
		return DefaultSelectionLimit
	}
	if n > MaxSelectionLimit {
		return MaxSelectionLimit
	}
	return n
}

func ClampMaxTotalBytes(n int64) int64 {
	if n <= 0 {
		return DefaultMaxTotalBytes
	}
	if n > MaxExportBytes {
		return MaxExportBytes
	}
	return n
}

func ClampVideoDuration(ms int64) int64 {
	if ms <= 0 {
		return DefaultVideoMaxDurationMs
	}
	if ms > MaxVideoMaxDurationMs {
		return MaxVideoMaxDurationMs
	}
	return ms
}

func ClampAudioDuration(ms int64) int64 {
	if ms <= 0 {
		return DefaultAudioMaxDurationMs
	}
	if ms > MaxAudioMaxDurationMs {
		return MaxAudioMaxDurationMs
	}
	return ms
}

func ValidatePickerRequest(req MediaPickerRequest) error {
	for _, k := range req.Kinds {
		if !IsValidPickerKind(k) {
			return fmt.Errorf("%v: invalid picker kind %q", ErrInvalidMediaType, k)
		}
	}
	if req.SelectionLimit > MaxSelectionLimit {
		return fmt.Errorf("%v: selection limit %d exceeds max %d", ErrInvalidRequest, req.SelectionLimit, MaxSelectionLimit)
	}
	if req.MaxTotalBytes > MaxExportBytes {
		return fmt.Errorf("%v: maxTotalBytes exceeds limit", ErrContentTooLarge)
	}
	return nil
}

func ValidatePhotoExportRequest(req PhotoExportRequest) error {
	if req.AssetRef == "" {
		return fmt.Errorf("%v: assetRef is required", ErrPhotoAssetNotFound)
	}
	if !IsValidRepresentation(req.Representation) {
		return fmt.Errorf("%v: invalid representation %q", ErrInvalidRepresentation, req.Representation)
	}
	if req.MaxBytes > MaxExportBytes {
		return fmt.Errorf("%v: maxBytes exceeds limit", ErrContentTooLarge)
	}
	return nil
}

func ValidateCameraCaptureRequest(req CameraCaptureRequest) error {
	if req.Flash != "" && !IsValidFlashMode(req.Flash) {
		return fmt.Errorf("%v: invalid flash mode %q", ErrInvalidFlashMode, req.Flash)
	}
	if req.Quality != "" && !IsValidQuality(req.Quality) {
		return fmt.Errorf("%v: invalid quality %q", ErrInvalidQuality, req.Quality)
	}
	if req.Format != "" && !IsValidFormat(req.Format) {
		return fmt.Errorf("%v: invalid format %q", ErrInvalidFormat, req.Format)
	}
	return nil
}

func ValidateVideoRecordRequest(req VideoRecordRequest) error {
	if req.MaxDurationMs <= 0 {
		return fmt.Errorf("%v: maxDurationMs is required", ErrInvalidRequest)
	}
	if req.MaxDurationMs > MaxVideoMaxDurationMs {
		return fmt.Errorf("%v: maxDurationMs exceeds limit", ErrVideoRecordDurationExceeded)
	}
	if req.Torch != "" && !IsValidTorchMode(req.Torch) {
		return fmt.Errorf("%v: invalid torch mode %q", ErrInvalidTorchMode, req.Torch)
	}
	if req.Quality != "" && !IsValidQuality(req.Quality) {
		return fmt.Errorf("%v: invalid quality %q", ErrInvalidQuality, req.Quality)
	}
	return nil
}

func ValidateAudioRecordRequest(req AudioRecordRequest) error {
	if req.MaxDurationMs <= 0 {
		return fmt.Errorf("%v: maxDurationMs is required", ErrInvalidRequest)
	}
	if req.MaxDurationMs > MaxAudioMaxDurationMs {
		return fmt.Errorf("%v: maxDurationMs exceeds limit", ErrAudioRecordDurationExceeded)
	}
	if req.Format != "" && !IsValidAudioFormat(req.Format) {
		return fmt.Errorf("%v: invalid audio format %q", ErrInvalidFormat, req.Format)
	}
	return nil
}
