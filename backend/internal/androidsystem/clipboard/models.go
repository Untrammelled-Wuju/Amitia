package clipboard

type ClipboardCapabilityState struct {
	Supported          bool     `json:"supported"`
	CanWrite           bool     `json:"canWrite"`
	CanRead            bool     `json:"canRead"`
	AppForeground      bool     `json:"appForeground"`
	AppHasInputFocus   bool     `json:"appHasInputFocus"`
	ReadRequiresForeground bool  `json:"readRequiresForeground"`
	HasPrimaryClip     bool     `json:"hasPrimaryClip"`
	SupportedMimeTypes []string `json:"supportedMimeTypes"`
	MaxTextBytes       int      `json:"maxTextBytes"`
	State              string   `json:"state"`
	Reason             string   `json:"reason"`
}

type ClipboardReadResult struct {
	HasContent  bool   `json:"hasContent"`
	Text        string `json:"text,omitempty"`
	MIMEType    string `json:"mimeType"`
	ItemCount   int    `json:"itemCount"`
	Truncated   bool   `json:"truncated"`
	Sensitive   bool   `json:"sensitive"`
	Generation  uint64 `json:"generation"`
}

type ClipboardWriteRequest struct {
	Text      string `json:"text"`
	Sensitive *bool  `json:"sensitive,omitempty"`
}

type StatusResult struct {
	Supported          bool     `json:"supported"`
	CanWrite           bool     `json:"canWrite"`
	CanRead            bool     `json:"canRead"`
	AppForeground      bool     `json:"appForeground"`
	AppHasInputFocus   bool     `json:"appHasInputFocus"`
	ReadRequiresForeground bool  `json:"readRequiresForeground"`
	HasPrimaryClip     bool     `json:"hasPrimaryClip"`
	SupportedMimeTypes []string `json:"supportedMimeTypes"`
	MaxTextBytes       int      `json:"maxTextBytes"`
	State              string   `json:"state"`
	Reason             string   `json:"reason"`
}

type ReadResult struct {
	HasContent bool   `json:"hasContent"`
	Text       string `json:"text,omitempty"`
	MIMEType   string `json:"mimeType"`
	ItemCount  int    `json:"itemCount"`
	Truncated  bool   `json:"truncated"`
	Sensitive  bool   `json:"sensitive"`
	Generation uint64 `json:"generation"`
}

type WriteResult struct {
	Written   bool   `json:"written"`
	Bytes     int    `json:"bytes"`
	Sensitive bool   `json:"sensitive"`
	Generation uint64 `json:"generation"`
}

type ClearResult struct {
	Cleared bool `json:"cleared"`
}

const (
	MaxClipboardTextBytes = 64 * 1024
)

const (
	StateAvailable              = "available"
	StateForegroundRequired     = "foreground_required"
	StateFocusRequired          = "focus_required"
	StateForegroundStateUnknown = "foreground_state_unknown"
	StateEmpty                  = "empty"
	StateUnsupported            = "unsupported"
	StateHostUnavailable        = "host_unavailable"
	StatePermissionDenied       = "permission_denied"
)

const (
	OperationStatus    = "clipboard.status"
	OperationReadText  = "clipboard.read_text"
	OperationWriteText = "clipboard.write_text"
	OperationClear     = "clipboard.clear"
)

const (
	ToolIDStatus    = "android.clipboard.status"
	ToolIDReadText  = "android.clipboard.read_text"
	ToolIDWriteText = "android.clipboard.write_text"
	ToolIDClear     = "android.clipboard.clear"
)

const (
	MIMETextPlain = "text/plain"
	MIMETextHTML  = "text/html"
)
