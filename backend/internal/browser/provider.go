package browser

import (
	"context"
	"encoding/json"
)

type BrowserCapabilities struct {
	SupportsNavigation  bool     `json:"supportsNavigation"`
	SupportsDOM         bool     `json:"supportsDom"`
	SupportsInteraction bool     `json:"supportsInteraction"`
	SupportsDownload    bool     `json:"supportsDownload"`
	SupportsUpload      bool     `json:"supportsUpload"`
	SupportsScreenshot  bool     `json:"supportsScreenshot"`
	SupportsDevTools    bool     `json:"supportsDevTools"`
	RiskLevels          []string `json:"riskLevels,omitempty"`
}

type BrowserProvider interface {
	BrowserCapabilities() BrowserCapabilities
	Runtime() BrowserRuntime
	Sessions() BrowserSessionManager
	Tabs() BrowserTabManager
	Navigate() BrowserNavigator
	Observe() BrowserObserver
	Interact() BrowserInteractor
	Resources() BrowserResourceTransfer
}

type BrowserSessionManager interface {
	CreateSession(ctx context.Context) (BrowserSessionInfo, *BrowserError)
	CloseSession(ctx context.Context, sessionID BrowserSessionID) *BrowserError
	GetSession(ctx context.Context, sessionID BrowserSessionID) (BrowserSessionInfo, *BrowserError)
	ListSessions(ctx context.Context) ([]BrowserSessionInfo, *BrowserError)
}

type BrowserTabManager interface {
	CreateTab(ctx context.Context, sessionID BrowserSessionID) (BrowserTabInfo, *BrowserError)
	CloseTab(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID) *BrowserError
	GetTab(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID) (BrowserTabInfo, *BrowserError)
	ListTabs(ctx context.Context, sessionID BrowserSessionID) ([]BrowserTabInfo, *BrowserError)
	ActivateTab(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID) *BrowserError
}

type BrowserNavigator interface {
	Navigate(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID, request NavigateRequest) (NavigationResult, *BrowserError)
	Reload(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID, request NavigateRequest) (NavigationResult, *BrowserError)
	GoBack(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID) (NavigationResult, *BrowserError)
	GoForward(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID) (NavigationResult, *BrowserError)
	Stop(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID) *BrowserError
}

type BrowserObserver interface {
	GetDOMSnapshot(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID, maxDepth int) (*BrowserDOMSnapshot, *BrowserError)
	FindElement(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID, selector string) (*BrowserElementRef, *BrowserError)
	ScrollToElement(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID, element BrowserElementRef) *BrowserError
}

type BrowserInteractor interface {
	Click(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID, element BrowserElementRef) (*BrowserInteractionResult, *BrowserError)
	Input(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID, element BrowserElementRef, text string) (*BrowserInteractionResult, *BrowserError)
	Select(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID, element BrowserElementRef, value string) (*BrowserInteractionResult, *BrowserError)
	Hover(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID, element BrowserElementRef) (*BrowserInteractionResult, *BrowserError)
	Scroll(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID, direction string) (*BrowserInteractionResult, *BrowserError)
}

type BrowserResourceTransfer interface {
	Download(ctx context.Context, request BrowserDownloadRequest) (*BrowserDownloadResult, *BrowserError)
	Upload(ctx context.Context, request BrowserUploadRequest) (*BrowserUploadResult, *BrowserError)
	Screenshot(ctx context.Context, request BrowserScreenshotRequest) (*BrowserScreenshotResult, *BrowserError)
}

// BrowserDevToolsProvider is an optional provider extension. Keeping DevTools
// outside BrowserProvider preserves compatibility with test/alternate providers
// that only implement the stable browser automation contract.
type BrowserDevToolsProvider interface {
	DevTools() BrowserDevTools
}

// BrowserDevTools provides bounded Chromium DevTools operations for one Amitia
// browser tab. The kernel keeps the raw operation surface narrow and validates
// model inputs through ToolDefinition schemas before reaching this interface.
type BrowserDevTools interface {
	Execute(ctx context.Context, operation string, sessionID BrowserSessionID, tabID BrowserTabID, input json.RawMessage) (json.RawMessage, *BrowserError)
}
