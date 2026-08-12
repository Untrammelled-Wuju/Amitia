package screenshot

type CaptureResult struct {
	ResourceURI  string `json:"resourceUri"`
	MIMEType     string `json:"mimeType"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	DisplayID    int    `json:"displayId"`
	TimestampMs  int64  `json:"timestampMs"`
	SizeBytes    int64  `json:"sizeBytes"`
	ContentHash  string `json:"contentHash,omitempty"`
}

func (r CaptureResult) Valid() bool {
	return r.ResourceURI != "" &&
		r.MIMEType != "" &&
		r.Width > 0 &&
		r.Height > 0 &&
		r.SizeBytes > 0
}

type CapabilityState struct {
	Supported              bool     `json:"supported"`
	CaptureBackend         string   `json:"captureBackend"`
	AccessibilityEnabled   bool     `json:"accessibilityEnabled"`
	AccessibilityConnected bool     `json:"accessibilityConnected"`
	CanTakeScreenshot      bool     `json:"canTakeScreenshot"`
	SupportsMultipleDisplays bool   `json:"supportsMultipleDisplays"`
	SupportedFormats       []string `json:"supportedFormats"`
	MaxPixels              int64    `json:"maxPixels"`
	Reason                 string   `json:"reason"`
}
