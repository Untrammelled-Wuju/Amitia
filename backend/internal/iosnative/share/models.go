package share

type ShareState string

const (
	ShareStateReady    ShareState = "ready"
	ShareStateBusy     ShareState = "busy"
	ShareStateDisabled ShareState = "disabled"
)

type IOSShareStatus struct {
	Supported             bool       `json:"supported"`
	CanSend               bool       `json:"canSend"`
	CanReceive            bool       `json:"canReceive"`
	ShareExtensionInstalled bool    `json:"shareExtensionInstalled"`
	NativeHostReady       bool       `json:"nativeHostReady"`
	MaxResources          int        `json:"maxResources"`
	MaxSingleResourceBytes int64    `json:"maxSingleResourceBytes"`
	MaxTotalBytes         int64      `json:"maxTotalBytes"`
	MaxTextBytes          int64      `json:"maxTextBytes"`
	State                 ShareState `json:"state"`
}

type IOSShareSendRequest struct {
	Text        string           `json:"text,omitempty"`
	Subject     string           `json:"subject,omitempty"`
	URL         string           `json:"url,omitempty"`
	Resources   []string         `json:"resources,omitempty"`
	ShareTitle  string           `json:"shareTitle,omitempty"`
	Preview     *IOSSharePreview `json:"preview,omitempty"`
}

type IOSSharePreview struct {
	Title           string `json:"title,omitempty"`
	Subtitle        string `json:"subtitle,omitempty"`
	ImageResourceURI string `json:"imageResourceUri,omitempty"`
}

type IOSShareSendResult struct {
	Status             string `json:"status"`
	ResourceCount      int    `json:"resourceCount"`
	UserActionRequired bool   `json:"userActionRequired"`
	OperationID        string `json:"operationId,omitempty"`
}

type IOSIncomingShareItem struct {
	ItemID       string `json:"itemId"`
	Type         string `json:"type"`
	UTType       string `json:"utType,omitempty"`
	MIMEType     string `json:"mimeType,omitempty"`
	RelativePath string `json:"relativePath,omitempty"`
	DisplayName  string `json:"displayName,omitempty"`
	SizeBytes    int64  `json:"sizeBytes"`
	SHA256       string `json:"sha256,omitempty"`
}

type IOSIncomingShareManifest struct {
	Version        int                    `json:"version"`
	ShareID        string                 `json:"shareId"`
	CreatedAt      string                 `json:"createdAt"`
	Text           string                 `json:"text,omitempty"`
	Subject        string                 `json:"subject,omitempty"`
	SourceHostHint string                 `json:"sourceHostHint,omitempty"`
	Items          []IOSIncomingShareItem `json:"items"`
	TotalBytes     int64                  `json:"totalBytes"`
	Complete       bool                   `json:"complete"`
}

type IOSPendingShare struct {
	ShareID        string                 `json:"shareId"`
	CreatedAt      string                 `json:"createdAt"`
	Text           string                 `json:"text,omitempty"`
	Subject        string                 `json:"subject,omitempty"`
	ResourceCount  int                    `json:"resourceCount"`
	TotalBytes     int64                  `json:"totalBytes"`
	SourceHostHint string                 `json:"sourceHostHint,omitempty"`
	Stale          bool                   `json:"stale"`
}

type IOSPendingShares struct {
	Shares     []IOSPendingShare `json:"shares"`
	TotalCount int               `json:"totalCount"`
}

type IOSConsumeShareRequest struct {
	ShareID string `json:"shareId"`
}

type IOSConsumeShareResult struct {
	Consumed      bool   `json:"consumed"`
	ResourceCount int    `json:"resourceCount"`
	TotalBytes    int64  `json:"totalBytes"`
}

type IOSPeekShareRequest struct {
	ShareID string `json:"shareId"`
}

type IOSPeekShareResult struct {
	Found        bool   `json:"found"`
	ResourceCount int   `json:"resourceCount"`
}

type IOSDismissShareRequest struct {
	ShareID string `json:"shareId"`
}

type IOSStagingCleanupRequest struct {
	RemoveStale      bool  `json:"removeStale"`
	MaxStaleAgeHours int   `json:"maxStaleAgeHours"`
}

type IOSStagingCleanupResult struct {
	Removed  int    `json:"removed"`
	Scanned  int    `json:"scanned"`
}

type IOSLimitedDeleteConfirmRequest struct {
	PhotoIDs []string `json:"photoIds"`
	Confirm  bool     `json:"confirm"`
}

type IOSLimitedDeleteConfirmResult struct {
	Confirmed bool     `json:"confirmed"`
	PhotoIDs  []string `json:"photoIds"`
}
