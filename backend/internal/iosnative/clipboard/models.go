package clipboard

type PatternKind string

const (
	PatternProbableWebURL    PatternKind = "probableWebURL"
	PatternProbableWebSearch PatternKind = "probableWebSearch"
	PatternNumber            PatternKind = "number"
	PatternEmailAddress      PatternKind = "emailAddress"
	PatternPhoneNumber       PatternKind = "phoneNumber"
	PatternPostalAddress     PatternKind = "postalAddress"
	PatternCreditCardNumber  PatternKind = "creditCardNumber"
)

type ContentType string

const (
	ContentTypeTextPlain  ContentType = "text/plain"
	ContentTypeTextHTML   ContentType = "text/html"
	ContentTypeTextRTF    ContentType = "text/rtf"
	ContentTypeTextURI    ContentType = "text/uri-list"
	ContentTypeImagePNG   ContentType = "image/png"
	ContentTypeImageJPEG  ContentType = "image/jpeg"
	ContentTypeImageGIF   ContentType = "image/gif"
	ContentTypeImageWEBP  ContentType = "image/webp"
	ContentTypeImageHEIC  ContentType = "image/heic"
	ContentTypeFileURL    ContentType = "public.file-url"
)

type Sensitivity string

const (
	SensitivityNormal   Sensitivity = "normal"
	SensitivitySensitive Sensitivity = "sensitive"
	SensitivitySecret   Sensitivity = "secret"
)

type ClipboardStatus struct {
	Supported               bool   `json:"supported"`
	CanRead                 bool   `json:"canRead"`
	CanWrite                bool   `json:"canWrite"`
	ChangeCount             int    `json:"changeCount"`
	ItemCount               int    `json:"itemCount"`
	HasStrings              bool   `json:"hasStrings"`
	HasURLs                 bool   `json:"hasUrls"`
	HasImages               bool   `json:"hasImages"`
	HasColors               bool   `json:"hasColors"`
	UserIntentRecommended   bool   `json:"userIntentRecommended"`
	UIPasteControlSupported bool   `json:"uiPasteControlSupported"`
	Generation              uint64 `json:"generation"`
}

type ClipboardDetectionResult struct {
	ChangeCount int                     `json:"changeCount"`
	ItemCount   int                     `json:"itemCount"`
	Matches     []ClipboardDetectedItem `json:"matches"`
}

type ClipboardDetectedItem struct {
	Index    int               `json:"index"`
	Patterns []PatternMatch    `json:"patterns"`
	Values   []DetectedValue   `json:"values,omitempty"`
}

type PatternMatch struct {
	Pattern PatternKind `json:"pattern"`
	Matched bool        `json:"matched"`
}

type DetectedValue struct {
	Pattern PatternKind `json:"pattern"`
	Value   string      `json:"value,omitempty"`
}

type ClipboardReadRequest struct {
	PreferredTypes    []string `json:"preferredTypes,omitempty"`
	ItemIndexes       []int    `json:"itemIndexes,omitempty"`
	MaxItems          int      `json:"maxItems,omitempty"`
	MaxBytes          int64    `json:"maxBytes,omitempty"`
	MaterializeBinary bool     `json:"materializeBinary,omitempty"`
}

type ClipboardReadResult struct {
	ChangeCount int             `json:"changeCount"`
	Items       []ClipboardItem `json:"items"`
	Truncated   bool            `json:"truncated"`
	TotalBytes  int64           `json:"totalBytes"`
	Generation  uint64          `json:"generation"`
}

type ClipboardItem struct {
	Index           int                        `json:"index"`
	Representations []ClipboardRepresentation  `json:"representations"`
}

type ClipboardRepresentation struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	URL         string `json:"url,omitempty"`
	ResourceURI string `json:"resourceUri,omitempty"`
	Bytes       int64  `json:"bytes"`
	ContentHash string `json:"contentHash,omitempty"`
	Truncated   bool   `json:"truncated"`
}

type ClipboardWriteItem struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	URL         string `json:"url,omitempty"`
	ResourceURI string `json:"resourceUri,omitempty"`
}

type ClipboardWriteRequest struct {
	Items               []ClipboardWriteItem `json:"items"`
	LocalOnly           bool                 `json:"localOnly"`
	ExpirationSeconds   *int                 `json:"expirationSeconds,omitempty"`
}

type ClipboardWriteResult struct {
	Written     bool   `json:"written"`
	ItemCount   int    `json:"itemCount"`
	TotalBytes  int64  `json:"totalBytes"`
	LocalOnly   bool   `json:"localOnly"`
	ExpiresAt   string `json:"expiresAt,omitempty"`
	ChangeCount int    `json:"changeCount"`
	Generation  uint64 `json:"generation"`
}

type ClipboardClearResult struct {
	Cleared     bool `json:"cleared"`
	ChangeCount int  `json:"changeCount"`
}

type ClipboardPastePayload struct {
	Items  []ClipboardItem `json:"items"`
	Source string          `json:"source"`
}

type ClipboardWritePolicy struct {
	Sensitivity       Sensitivity `json:"sensitivity"`
	LocalOnly         *bool       `json:"localOnly,omitempty"`
	ExpirationSeconds *int        `json:"expirationSeconds,omitempty"`
}
