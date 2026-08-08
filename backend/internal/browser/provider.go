package browser

import "context"

type BrowserCapabilities struct {
	SupportsNavigation  bool     `json:"supportsNavigation"`
	SupportsDOM         bool     `json:"supportsDom"`
	SupportsInteraction bool     `json:"supportsInteraction"`
	SupportsDownload    bool     `json:"supportsDownload"`
	SupportsUpload      bool     `json:"supportsUpload"`
	SupportsScreenshot  bool     `json:"supportsScreenshot"`
	RiskLevels          []string `json:"riskLevels,omitempty"`
}

type BrowserSessionManager interface {
	CreateSession(ctx context.Context) (BrowserSessionInfo, *BrowserError)
	CloseSession(ctx context.Context, sessionID BrowserSessionID) *BrowserError
	GetSession(ctx context.Context, sessionID BrowserSessionID) (BrowserSessionInfo, *BrowserError)
	ListSessions(ctx context.Context) ([]BrowserSessionInfo, *BrowserError)
}

type BrowserTabManager interface {
	OpenTab(ctx context.Context, sessionID BrowserSessionID, url string) (BrowserTabInfo, *BrowserError)
	CloseTab(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID) *BrowserError
	ActivateTab(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID) *BrowserError
	ListTabs(ctx context.Context, sessionID BrowserSessionID) ([]BrowserTabInfo, *BrowserError)
}

type BrowserNavigator interface {
	Navigate(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID, url string) (*BrowserNavigationResult, *BrowserError)
	Reload(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID) (*BrowserNavigationResult, *BrowserError)
	GoBack(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID) (*BrowserNavigationResult, *BrowserError)
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

type BrowserProvider interface {
	BrowserCapabilities() BrowserCapabilities
	Sessions() BrowserSessionManager
	Tabs() BrowserTabManager
	Navigate() BrowserNavigator
	Observe() BrowserObserver
	Interact() BrowserInteractor
	Resources() BrowserResourceTransfer
}
