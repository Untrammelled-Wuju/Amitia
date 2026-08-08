package browser

import "time"

type BrowserSessionID string
type BrowserTabID string

type BrowserSessionState string

const (
	SessionStateCreated BrowserSessionState = "created"
	SessionStateReady   BrowserSessionState = "ready"
	SessionStateClosing BrowserSessionState = "closing"
	SessionStateClosed  BrowserSessionState = "closed"
	SessionStateFailed  BrowserSessionState = "failed"
)

type BrowserTabState string

const (
	TabStateLoading BrowserTabState = "loading"
	TabStateReady   BrowserTabState = "ready"
	TabStateFailed  BrowserTabState = "failed"
)

type BrowserNavigationResult struct {
	SessionID  BrowserSessionID `json:"sessionId"`
	TabID      BrowserTabID     `json:"tabId"`
	URL        string           `json:"url"`
	FinalURL   string           `json:"finalUrl"`
	Title      string           `json:"title,omitempty"`
	StatusCode int              `json:"statusCode,omitempty"`
}

type BrowserDownloadRequest struct {
	SessionID   BrowserSessionID `json:"sessionId"`
	TabID       BrowserTabID     `json:"tabId"`
	ResourceURI string           `json:"resourceURI"`
	Filename    string           `json:"filename,omitempty"`
}

type BrowserDownloadResult struct {
	ResourceURI string `json:"resourceURI"`
	Filename    string `json:"filename,omitempty"`
	SizeBytes   int64  `json:"sizeBytes,omitempty"`
	ContentType string `json:"contentType,omitempty"`
}

type BrowserUploadRequest struct {
	SessionID   BrowserSessionID `json:"sessionId"`
	TabID       BrowserTabID     `json:"tabId"`
	ResourceURI string           `json:"resourceURI"`
	TargetInput string           `json:"targetInput,omitempty"`
}

type BrowserUploadResult struct {
	ResourceURI string `json:"resourceURI"`
	Success     bool   `json:"success"`
}

type BrowserScreenshotRequest struct {
	SessionID BrowserSessionID `json:"sessionId"`
	TabID     BrowserTabID     `json:"tabId"`
	Format    string           `json:"format,omitempty"`
	Quality   int              `json:"quality,omitempty"`
}

type BrowserScreenshotResult struct {
	ResourceURI string `json:"resourceURI"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
}

type BrowserDOMSnapshot struct {
	SessionID BrowserSessionID `json:"sessionId"`
	TabID     BrowserTabID     `json:"tabId"`
	URL       string           `json:"url"`
	Title     string           `json:"title,omitempty"`
	Content   string           `json:"content,omitempty"`
	Truncated bool             `json:"truncated"`
	MaxDepth  int              `json:"maxDepth,omitempty"`
}

type BrowserElementRef struct {
	SessionID BrowserSessionID `json:"sessionId"`
	TabID     BrowserTabID     `json:"tabId"`
	Selector  string           `json:"selector"`
	StableID  string           `json:"stableId"`
}

type BrowserInteractionRequest struct {
	SessionID BrowserSessionID  `json:"sessionId"`
	TabID     BrowserTabID      `json:"tabId"`
	Element   BrowserElementRef `json:"element"`
	Action    string            `json:"action"`
	InputText string            `json:"inputText,omitempty"`
}

type BrowserInteractionResult struct {
	Success   bool   `json:"success"`
	Stale     bool   `json:"stale,omitempty"`
	ErrorHint string `json:"errorHint,omitempty"`
}

type BrowserSessionInfo struct {
	SessionID BrowserSessionID    `json:"sessionId"`
	State     BrowserSessionState `json:"state"`
	CreatedAt time.Time           `json:"createdAt"`
	URL       string              `json:"url,omitempty"`
}

type BrowserTabInfo struct {
	TabID     BrowserTabID     `json:"tabId"`
	SessionID BrowserSessionID `json:"sessionId"`
	State     BrowserTabState  `json:"state"`
	URL       string           `json:"url,omitempty"`
	Title     string           `json:"title,omitempty"`
	Active    bool             `json:"active"`
}
