package browser

import (
	"context"
	"time"
)

type BrowserSessionID string
type BrowserTabID string
type BrowserContextID string
type TargetID string

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
	TabStateCreated BrowserTabState = "created"
	TabStateReady   BrowserTabState = "ready"
	TabStateLoading BrowserTabState = "loading"
	TabStateClosing BrowserTabState = "closing"
	TabStateClosed  BrowserTabState = "closed"
	TabStateFailed  BrowserTabState = "failed"
)

func (id BrowserContextID) String() string {
	return string(id)
}

func (id TargetID) String() string {
	return string(id)
}

type TargetInfo struct {
	TargetID         TargetID
	Type             string
	URL              string
	Title            string
	BrowserContextID BrowserContextID
	Attached         bool
}

type BrowserTabInfo struct {
	TabID       BrowserTabID     `json:"tabId"`
	SessionID   BrowserSessionID `json:"sessionId"`
	State       BrowserTabState  `json:"state"`
	URL         string           `json:"url,omitempty"`
	Title       string           `json:"title,omitempty"`
	Active      bool             `json:"active"`
	OpenerTabID BrowserTabID     `json:"openerTabId,omitempty"`
	AutoCreated bool             `json:"autoCreated,omitempty"`
	CreatedAt   time.Time        `json:"createdAt"`
	UpdatedAt   time.Time        `json:"updatedAt"`
}

type NavigateRequest struct {
	URL       string `json:"url"`
	WaitUntil string `json:"waitUntil,omitempty"`
	TimeoutMS int    `json:"timeoutMs,omitempty"`
	Referer   string `json:"referer,omitempty"`
}

type NavigationResult struct {
	SessionID    BrowserSessionID `json:"sessionId"`
	TabID        BrowserTabID     `json:"tabId"`
	RequestedURL string           `json:"requestedUrl"`
	FinalURL     string           `json:"finalUrl"`
	Title        string           `json:"title,omitempty"`
	Redirected   bool             `json:"redirected"`
	HTTPStatus   *int             `json:"httpStatus,omitempty"`
	WaitUntil    string           `json:"waitUntil"`
	Loaded       bool             `json:"loaded"`
	TimedOut     bool             `json:"timedOut"`
	DurationMS   int64            `json:"durationMs"`
}

type ResolvedBrowserTab struct {
	SessionID         BrowserSessionID
	TabID             BrowserTabID
	BrowserContextID  BrowserContextID
	TargetID          TargetID
	RuntimeGeneration uint64
}

type TabResolver interface {
	ResolveTab(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID) (ResolvedBrowserTab, *BrowserError)
}

type SessionTabCleaner interface {
	CloseAllForSession(ctx context.Context, sessionID BrowserSessionID, generation uint64) *BrowserError
}

type BrowserDownloadRequest struct {
	SessionID      BrowserSessionID   `json:"sessionId"`
	TabID          BrowserTabID       `json:"tabId"`
	ResourceURI    string             `json:"resourceURI"`
	Filename       string             `json:"filename,omitempty"`
	TriggerElement *BrowserElementRef `json:"triggerElement,omitempty"`
	WaitTimeoutMS  int64              `json:"waitTimeoutMs,omitempty"`
	Overwrite      bool               `json:"overwrite,omitempty"`
}

type BrowserDownloadResult struct {
	ResourceURI string `json:"resourceURI"`
	Filename    string `json:"filename,omitempty"`
	SizeBytes   int64  `json:"sizeBytes,omitempty"`
	ContentType string `json:"contentType,omitempty"`
	ContentHash string `json:"contentHash,omitempty"`
	DownloadID  string `json:"downloadId,omitempty"`
}

type BrowserUploadRequest struct {
	SessionID   BrowserSessionID   `json:"sessionId"`
	TabID       BrowserTabID       `json:"tabId"`
	ResourceURI string             `json:"resourceURI"`
	Element     *BrowserElementRef `json:"element,omitempty"`
	TargetInput string             `json:"targetInput,omitempty"`
}

type BrowserUploadResult struct {
	ResourceURI    string `json:"resourceURI"`
	Success        bool   `json:"success"`
	TargetStableID string `json:"targetStableId,omitempty"`
	FileSet        bool   `json:"fileSet"`
	Verified       bool   `json:"verified,omitempty"`
}

type BrowserScreenshotRequest struct {
	SessionID BrowserSessionID `json:"sessionId"`
	TabID     BrowserTabID     `json:"tabId"`
	Format    string           `json:"format,omitempty"`
	Quality   int              `json:"quality,omitempty"`
	FullPage  bool             `json:"fullPage,omitempty"`
}

type BrowserScreenshotResult struct {
	ResourceURI        string `json:"resourceURI"`
	Width              int    `json:"width,omitempty"`
	Height             int    `json:"height,omitempty"`
	Format             string `json:"format,omitempty"`
	SizeBytes          int64  `json:"sizeBytes,omitempty"`
	ContentHash        string `json:"contentHash,omitempty"`
	RuntimeGeneration  uint64 `json:"runtimeGeneration,omitempty"`
	DocumentGeneration uint64 `json:"documentGeneration,omitempty"`
}

type BrowserDOMSnapshot struct {
	SessionID          BrowserSessionID `json:"sessionId"`
	TabID              BrowserTabID     `json:"tabId"`
	URL                string           `json:"url"`
	Title              string           `json:"title,omitempty"`
	Content            string           `json:"content,omitempty"`
	Truncated          bool             `json:"truncated"`
	MaxDepth           int              `json:"maxDepth,omitempty"`
	RuntimeGeneration  uint64           `json:"runtimeGeneration,omitempty"`
	DocumentGeneration uint64           `json:"documentGeneration,omitempty"`
	NodeCount          int              `json:"nodeCount,omitempty"`
}

type BrowserElementRef struct {
	SessionID          BrowserSessionID `json:"sessionId"`
	TabID              BrowserTabID     `json:"tabId"`
	Selector           string           `json:"selector,omitempty"`
	StableID           string           `json:"stableId"`
	RuntimeGeneration  uint64           `json:"runtimeGeneration,omitempty"`
	DocumentGeneration uint64           `json:"documentGeneration,omitempty"`
	FrameID            string           `json:"frameId,omitempty"`
}

type BrowserInteractionRequest struct {
	SessionID BrowserSessionID  `json:"sessionId"`
	TabID     BrowserTabID      `json:"tabId"`
	Element   BrowserElementRef `json:"element"`
	Action    string            `json:"action"`
	InputText string            `json:"inputText,omitempty"`
}

type BrowserInteractionResult struct {
	Success            bool   `json:"success"`
	Stale              bool   `json:"stale,omitempty"`
	ErrorHint          string `json:"errorHint,omitempty"`
	Action             string `json:"action,omitempty"`
	Strategy           string `json:"strategy,omitempty"`
	Verified           bool   `json:"verified,omitempty"`
	DocumentGeneration uint64 `json:"documentGeneration,omitempty"`
	DurationMS         int64  `json:"durationMs,omitempty"`
}

type BrowserSessionInfo struct {
	SessionID BrowserSessionID    `json:"sessionId"`
	State     BrowserSessionState `json:"state"`
	CreatedAt time.Time           `json:"createdAt"`
	URL       string              `json:"url,omitempty"`
}

type BrowserRuntimeState string

const (
	BrowserRuntimeStopped  BrowserRuntimeState = "stopped"
	BrowserRuntimeStarting BrowserRuntimeState = "starting"
	BrowserRuntimeReady    BrowserRuntimeState = "ready"
	BrowserRuntimeStopping BrowserRuntimeState = "stopping"
	BrowserRuntimeFailed   BrowserRuntimeState = "failed"
)

type BrowserRuntimeHealth string

const (
	BrowserHealthUnknown     BrowserRuntimeHealth = "unknown"
	BrowserHealthHealthy     BrowserRuntimeHealth = "healthy"
	BrowserHealthUnhealthy   BrowserRuntimeHealth = "unhealthy"
	BrowserHealthUnavailable BrowserRuntimeHealth = "unavailable"
	BrowserHealthStarting    BrowserRuntimeHealth = "starting"
)

type BrowserRuntimeInfo struct {
	State          BrowserRuntimeState `json:"state"`
	Generation     uint64              `json:"generation"`
	Engine         string              `json:"engine"`
	BrowserName    string              `json:"browserName,omitempty"`
	BrowserVersion string              `json:"browserVersion,omitempty"`
	Headless       bool                `json:"headless"`
	StartedAt      *time.Time          `json:"startedAt,omitempty"`
	ProcessAlive   bool                `json:"processAlive"`
	CDPConnected   bool                `json:"cdpConnected"`
	LastErrorCode  string              `json:"lastErrorCode,omitempty"`
}

type BrowserExecutable struct {
	Path    string
	Kind    string
	Version string
}

type BrowserConfig struct {
	Enabled               bool          `json:"enabled"`
	ExecutablePath        string        `json:"executablePath,omitempty"`
	Headless              bool          `json:"headless"`
	StartupTimeout        time.Duration `json:"startupTimeout"`
	ShutdownTimeout       time.Duration `json:"shutdownTimeout"`
	UserDataRoot          string        `json:"userDataRoot,omitempty"`
	MaxBrowserMemoryBytes int64         `json:"maxBrowserMemoryBytes"`
	AllowedSchemes        []string      `json:"allowedSchemes"`
	MaxSessions           int           `json:"maxSessions"`
	MaxTabsPerSession     int           `json:"maxTabsPerSession"`
	MaxTabsTotal          int           `json:"maxTabsTotal"`
	NavigationTimeout     time.Duration `json:"navigationTimeout"`
	MaxNavigationTimeout  time.Duration `json:"maxNavigationTimeout"`
}

const (
	DefaultMaxSessions          = 8
	DefaultMaxTabsPerSession    = 8
	DefaultMaxTabsTotal         = 32
	DefaultNavigationTimeout    = 30 * time.Second
	DefaultMaxNavigationTimeout = 120 * time.Second
)

type CDPSessionID string
type FrameID string
type BackendNodeID int64

type ResolvedBrowserElement struct {
	SessionID          BrowserSessionID
	TabID              BrowserTabID
	RuntimeGeneration  uint64
	DocumentGeneration uint64
	TargetID           TargetID
	CDPSessionID       CDPSessionID
	FrameID            FrameID
	BackendNodeID      BackendNodeID
	Selector           string
}

type ElementResolver interface {
	ResolveElement(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID, ref BrowserElementRef) (ResolvedBrowserElement, *BrowserError)
}

type DocumentBumper interface {
	BumpDocumentGeneration(ctx context.Context, tabID BrowserTabID, runtimeGeneration uint64) *BrowserError
}

type BrowserDownloadID string

type downloadRecord struct {
	id                BrowserDownloadID
	sessionID         BrowserSessionID
	tabID             BrowserTabID
	runtimeGeneration uint64
	guid              string
	frameID           string
	sourceURL         string
	suggestedFilename string
	receivedBytes     int64
	totalBytes        int64
	state             string
	stagedPath        string
	startedAt         time.Time
	completedAt       *time.Time
	claimed           bool
}

type BrowserRecoveryPolicy struct {
	Enabled            bool          `json:"enabled"`
	AutoRestartRuntime bool          `json:"autoRestartRuntime"`
	RestoreSessions    bool          `json:"restoreSessions"`
	RestoreTabs        bool          `json:"restoreTabs"`
	RestoreLastSafeURL bool          `json:"restoreLastSafeURL"`
	MaxAttempts        int           `json:"maxAttempts"`
	Backoff            time.Duration `json:"backoff"`
}

type BrowserRecoveryResult struct {
	RuntimeGeneration      uint64   `json:"runtimeGeneration"`
	SessionsRecovered      int      `json:"sessionsRecovered"`
	TabsRecovered          int      `json:"tabsRecovered"`
	TabsFailed             int      `json:"tabsFailed"`
	AuthStateRestored      bool     `json:"authStateRestored"`
	ElementRefsInvalidated bool     `json:"elementRefsInvalidated"`
	DownloadsInvalidated   bool     `json:"downloadsInvalidated"`
	Warnings               []string `json:"warnings,omitempty"`
}

type sessionRecoveryDescriptor struct {
	sessionID   BrowserSessionID
	state       BrowserSessionState
	createdAt   time.Time
	recoverable bool
}

type tabRecoveryState struct {
	tabID              BrowserTabID
	lastCommittedURL   string
	active             bool
	recoverable        bool
	lastNavigationKind string
}

const (
	DownloadStatePending    = "pending"
	DownloadStateInProgress = "in_progress"
	DownloadStateCompleted  = "completed"
	DownloadStateFailed     = "failed"
	DownloadStateCancelled  = "cancelled"

	ScreenshotFormatPNG  = "png"
	ScreenshotFormatJPEG = "jpeg"
	ScreenshotFormatWebP = "webp"
)
