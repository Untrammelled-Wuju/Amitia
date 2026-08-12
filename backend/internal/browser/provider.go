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
