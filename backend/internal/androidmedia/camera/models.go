package camera

type PermissionState string

const (
	PermissionUnknown     PermissionState = "unknown"
	PermissionGranted     PermissionState = "granted"
	PermissionDenied      PermissionState = "denied"
	PermissionRequired    PermissionState = "required"
	PermissionUnavailable PermissionState = "unavailable"
)

type CapabilityState struct {
	Supported          bool            `json:"supported"`
	PermissionState    PermissionState `json:"permissionState"`
	UserActionRequired bool            `json:"userActionRequired"`
	CameraCount        int             `json:"cameraCount"`
	DefaultLens        string          `json:"defaultLens,omitempty"`
	CaptureAvailable   bool            `json:"captureAvailable"`
	Reason             string          `json:"reason,omitempty"`
}

type CameraDevice struct {
	CameraID          string `json:"cameraId"`
	LensFacing        string `json:"lensFacing"`
	SensorOrientation int    `json:"sensorOrientation"`
	FlashAvailable    bool   `json:"flashAvailable"`
	SupportsAutoFocus bool   `json:"supportsAutoFocus"`
	SupportsZoom      bool   `json:"supportsZoom"`
	MaxWidth          int    `json:"maxWidth,omitempty"`
	MaxHeight         int    `json:"maxHeight,omitempty"`
}

type CaptureRequest struct {
	CameraID  *string `json:"cameraId,omitempty"`
	Lens      *string `json:"lens,omitempty"`
	Format    *string `json:"format,omitempty"`
	Quality   *int    `json:"quality,omitempty"`
	MaxWidth  *int    `json:"maxWidth,omitempty"`
	MaxHeight *int    `json:"maxHeight,omitempty"`
	FlashMode *string `json:"flashMode,omitempty"`
	FocusMode *string `json:"focusMode,omitempty"`
	Rotation  *int    `json:"rotation,omitempty"`
}

type CaptureResult struct {
	ResourceURI  string `json:"resourceUri"`
	MIMEType     string `json:"mimeType"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	CameraID     string `json:"cameraId"`
	LensFacing   string `json:"lensFacing"`
	Rotation     int    `json:"rotation"`
	TimestampMs  int64  `json:"timestampMs"`
	SizeBytes    int64  `json:"sizeBytes"`
	ContentHash  string `json:"contentHash,omitempty"`
	EXIFStripped bool   `json:"exifStripped"`
}

func (r CaptureResult) Valid() bool {
	return r.ResourceURI != "" &&
		r.MIMEType != "" &&
		r.Width > 0 &&
		r.Height > 0 &&
		r.SizeBytes > 0
}

const (
	LensFront    = "front"
	LensBack     = "back"
	LensExternal = "external"
	LensUnknown  = "unknown"
)

const (
	FlashOff  = "off"
	FlashOn   = "on"
	FlashAuto = "auto"
)

const (
	FocusAuto       = "auto"
	FocusContinuous = "continuous"
)

var validLenses = map[string]bool{
	LensFront:    true,
	LensBack:     true,
	LensExternal: true,
}

var validFlashModes = map[string]bool{
	FlashOff:  true,
	FlashOn:   true,
	FlashAuto: true,
}

var validFocusModes = map[string]bool{
	FocusAuto:       true,
	FocusContinuous: true,
}

var validFormats = map[string]bool{
	"jpeg": true,
	"png":  true,
	"webp": true,
}

func (r CaptureRequest) Validate() error {
	if r.CameraID != nil && r.Lens != nil {
		return &CameraError{
			Code:    CameraLensConflict,
			Message: "cameraId and lens cannot both specify different capture targets",
		}
	}
	if r.Lens != nil && !validLenses[*r.Lens] {
		return &CameraError{
			Code:    CameraNotFound,
			Message: "unknown lens: " + *r.Lens,
		}
	}
	if r.FlashMode != nil && !validFlashModes[*r.FlashMode] {
		return &CameraError{
			Code:    CameraInvalidFlashMode,
			Message: "unsupported flash mode: " + *r.FlashMode,
		}
	}
	if r.FocusMode != nil && !validFocusModes[*r.FocusMode] {
		return &CameraError{
			Code:    CameraInvalidFormat,
			Message: "unsupported focus mode: " + *r.FocusMode,
		}
	}
	if r.Format != nil && !validFormats[*r.Format] {
		return &CameraError{
			Code:    CameraInvalidFormat,
			Message: "unsupported capture format: " + *r.Format,
		}
	}
	if r.Quality != nil {
		q := *r.Quality
		if q < 1 || q > 100 {
			return &CameraError{
				Code:    CameraInvalidFormat,
				Message: "quality must be between 1 and 100",
			}
		}
	}
	if r.MaxWidth != nil && *r.MaxWidth <= 0 {
		return &CameraError{
			Code:    CameraInvalidSize,
			Message: "maxWidth must be positive",
		}
	}
	if r.MaxHeight != nil && *r.MaxHeight <= 0 {
		return &CameraError{
			Code:    CameraInvalidSize,
			Message: "maxHeight must be positive",
		}
	}
	return nil
}

func (r CaptureRequest) ResolveLens() string {
	if r.Lens != nil && validLenses[*r.Lens] {
		return *r.Lens
	}
	return LensBack
}

func (r CaptureRequest) ResolveFormat() string {
	if r.Format != nil && validFormats[*r.Format] {
		return *r.Format
	}
	return "jpeg"
}

func (r CaptureRequest) ResolveFlashMode() string {
	if r.FlashMode != nil && validFlashModes[*r.FlashMode] {
		return *r.FlashMode
	}
	return FlashOff
}

type CameraError struct {
	Code    string
	Message string
}

func (e *CameraError) Error() string {
	if e == nil {
		return ""
	}
	return e.Code + ": " + e.Message
}
