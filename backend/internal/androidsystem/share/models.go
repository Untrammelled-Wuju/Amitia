package share

type ShareCapabilityState struct {
	Supported              bool   `json:"supported"`
	CanSend                bool   `json:"canSend"`
	CanReceive             bool   `json:"canReceive"`
	NativeHostReady        bool   `json:"nativeHostReady"`
	MaxResources           int    `json:"maxResources"`
	MaxSingleResourceBytes int64  `json:"maxSingleResourceBytes"`
	MaxTotalBytes          int64  `json:"maxTotalBytes"`
	State                  string `json:"state"`
}

type ShareSendRequest struct {
	Text         string   `json:"text,omitempty"`
	Subject      string   `json:"subject,omitempty"`
	Resources    []string `json:"resources,omitempty"`
	MIMEType     string   `json:"mimeType,omitempty"`
	ChooserTitle string   `json:"chooserTitle,omitempty"`
}

type SharedResource struct {
	ResourceURI string `json:"resourceUri"`
	MIMEType    string `json:"mimeType,omitempty"`
	ExportToken string `json:"exportToken,omitempty"`
}

type ShareSendResult struct {
	Status            string `json:"status"`
	ResourceCount     int    `json:"resourceCount"`
	MIMEType          string `json:"mimeType,omitempty"`
	UserActionRequired bool  `json:"userActionRequired"`
}

type IncomingShare struct {
	ShareID   string           `json:"shareId"`
	Text      string           `json:"text,omitempty"`
	Subject   string           `json:"subject,omitempty"`
	Resources []SharedResource `json:"resources,omitempty"`
	ReceivedAt int64           `json:"receivedAt"`
}

type StatusResult struct {
	Supported              bool   `json:"supported"`
	CanSend                bool   `json:"canSend"`
	CanReceive             bool   `json:"canReceive"`
	NativeHostReady        bool   `json:"nativeHostReady"`
	MaxResources           int    `json:"maxResources"`
	MaxSingleResourceBytes int64  `json:"maxSingleResourceBytes"`
	MaxTotalBytes          int64  `json:"maxTotalBytes"`
	State                  string `json:"state"`
}

type SendResult struct {
	Status             string `json:"status"`
	ResourceCount     int    `json:"resourceCount"`
	MIMEType          string `json:"mimeType,omitempty"`
	UserActionRequired bool  `json:"userActionRequired"`
}

const (
	MaxResourcesCount    = 10
	MaxSingleResourceBytes = 100 * 1024 * 1024  // 100 MiB
	MaxTotalBytes        = 250 * 1024 * 1024  // 250 MiB
	MaxShareTextBytes    = 1 * 1024 * 1024    // 1 MiB
	MaxSubjectBytes      = 8 * 1024           // 8 KiB
	ChooserTitleMaxBytes = 256
)

const (
	StateAvailable       = "available"
	StateUnsupported     = "unsupported"
	StateHostUnavailable = "host_unavailable"
	StateUINotReady      = "ui_context_required"
)

const (
	OperationStatus          = "share.status"
	OperationSend            = "share.send"
	OperationReceivePending  = "share.receive.pending"
	OperationReceiveConsume  = "share.receive.consume"
)

const (
	ToolIDStatus = "android.share.send.status"
	ToolIDSend   = "android.share.send"
)

const (
	PermissionSend    = "android.share.send"
	PermissionReceive = "android.share.receive"
)

const (
	ExportTTLMinutes   = 15
	ShareExportDirName = "share-export"
)
