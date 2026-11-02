package browser

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestChromiumPipeTransportIntegration(t *testing.T) {
	executable := os.Getenv("AMITIA_BROWSER_EXECUTABLE")
	if executable == "" {
		t.Skip("AMITIA_BROWSER_EXECUTABLE is not set")
	}
	root, err := os.MkdirTemp("", "amitia-browser-pipe-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	config := NewBrowserConfigResolver(BrowserConfig{
		Enabled:           true,
		ExecutablePath:    executable,
		Headless:          true,
		UserDataRoot:      root,
		StartupTimeout:    20 * time.Second,
		ShutdownTimeout:   5 * time.Second,
		AllowedSchemes:    []string{"http", "https"},
		MaxSessions:       1,
		MaxTabsPerSession: 1,
		MaxTabsTotal:      1,
	}).Resolve()
	process := newBrowserProcess(BrowserExecutable{Path: executable, Kind: browserKindChrome}, config, config.UserDataRoot+"/runtime-1")
	if _, _, err := process.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		process.kill()
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) && isProcessAliveByPID(process.pid) {
			time.Sleep(20 * time.Millisecond)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := process.connectCDP(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestChromiumProviderNavigationIntegration(t *testing.T) {
	executable := os.Getenv("AMITIA_BROWSER_EXECUTABLE")
	if executable == "" {
		t.Skip("AMITIA_BROWSER_EXECUTABLE is not set")
	}
	root, err := os.MkdirTemp("", "amitia-browser-provider-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	config := NewBrowserConfigResolver(BrowserConfig{
		Enabled:              true,
		ExecutablePath:       executable,
		Headless:             true,
		UserDataRoot:         root,
		StartupTimeout:       20 * time.Second,
		ShutdownTimeout:      5 * time.Second,
		AllowedSchemes:       []string{"http", "https"},
		MaxSessions:          1,
		MaxTabsPerSession:    1,
		MaxTabsTotal:         1,
		NavigationTimeout:    10 * time.Second,
		MaxNavigationTimeout: 10 * time.Second,
	}).Resolve()
	provider, err := NewProductionProvider(config, NewBrowserEngineFactory())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, browserErr := provider.Runtime().Start(ctx); browserErr != nil {
		t.Fatal(browserErr)
	}
	defer func() {
		if browserErr := provider.Runtime().Stop(context.Background()); browserErr != nil {
			t.Error(browserErr)
		}
	}()
	session, browserErr := provider.Sessions().CreateSession(ctx)
	if browserErr != nil {
		t.Fatal(browserErr)
	}
	tab, browserErr := provider.Tabs().CreateTab(ctx, session.SessionID)
	if browserErr != nil {
		t.Fatal(browserErr)
	}
	result, browserErr := provider.Navigate().Navigate(ctx, session.SessionID, tab.TabID, NavigateRequest{URL: "https://example.com"})
	if browserErr != nil {
		t.Fatal(browserErr)
	}
	if result.FinalURL == "" || result.Title == "" {
		t.Fatalf("unexpected navigation result: %#v", result)
	}
	tabManager := provider.(*productionProvider).tabs.(*productionTabManager)
	record, ok := tabManager.store.get(tab.TabID)
	if !ok {
		t.Fatal("tab record not found")
	}
	targetInfo, targetErr := tabManager.backend.TargetInfo(ctx, record.targetID)
	if targetErr != nil {
		t.Fatal(targetErr)
	}
	if !strings.Contains(targetInfo.URL, "example.com") {
		t.Fatalf("unexpected target info: %#v", targetInfo)
	}
	pageSessionID := tabManager.engine().Pages().(*chromiumPageController).getSession(record.targetID)
	var evaluated struct {
		Result struct {
			Value string `json:"value"`
		} `json:"result"`
	}
	if err := tabManager.engine().cdpClient().Call(ctx, "Runtime.evaluate", pageSessionID, map[string]any{"expression": "document.documentElement.outerHTML", "returnByValue": true}, &evaluated); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(evaluated.Result.Value, "Example Domain") {
		t.Fatalf("unexpected evaluated document: %q", evaluated.Result.Value)
	}
	var rawDocument struct {
		Root struct {
			DocumentURL    string `json:"documentURL"`
			ChildNodeCount int    `json:"childNodeCount"`
			Children       []struct {
				NodeName string `json:"nodeName"`
			} `json:"children"`
		} `json:"root"`
	}
	if err := tabManager.engine().cdpClient().Call(ctx, "DOM.getDocument", pageSessionID, map[string]any{"depth": 4, "pierce": false}, &rawDocument); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rawDocument.Root.DocumentURL, "example.com") {
		t.Fatalf("unexpected raw document: %#v", rawDocument.Root)
	}
	snapshot, browserErr := provider.Observe().GetDOMSnapshot(ctx, session.SessionID, tab.TabID, 4)
	if browserErr != nil {
		t.Fatal(browserErr)
	}
	if snapshot.URL == "" || !strings.Contains(snapshot.Content, "Example Domain") {
		t.Fatalf("unexpected DOM snapshot: %#v", snapshot)
	}
	screenshot, browserErr := provider.Resources().Screenshot(ctx, BrowserScreenshotRequest{
		SessionID: session.SessionID,
		TabID:     tab.TabID,
		Format:    ScreenshotFormatJPEG,
		Quality:   70,
	})
	if browserErr != nil {
		t.Fatal(browserErr)
	}
	if screenshot.SizeBytes == 0 {
		t.Fatalf("unexpected screenshot: %#v", screenshot)
	}
	if _, browserErr := provider.Navigate().Navigate(ctx, session.SessionID, tab.TabID, NavigateRequest{URL: "https://httpbin.org/forms/post"}); browserErr != nil {
		t.Fatal(browserErr)
	}
	back, browserErr := provider.Navigate().GoBack(ctx, session.SessionID, tab.TabID)
	if browserErr != nil {
		t.Fatal(browserErr)
	}
	if !strings.Contains(back.FinalURL, "example.com") {
		t.Fatalf("unexpected back result: %#v", back)
	}
	forward, browserErr := provider.Navigate().GoForward(ctx, session.SessionID, tab.TabID)
	if browserErr != nil {
		t.Fatal(browserErr)
	}
	if !strings.Contains(forward.FinalURL, "httpbin.org") {
		t.Fatalf("unexpected forward result: %#v", forward)
	}
	reloaded, browserErr := provider.Navigate().Reload(ctx, session.SessionID, tab.TabID, NavigateRequest{URL: "https://httpbin.org/forms/post"})
	if browserErr != nil {
		t.Fatal(browserErr)
	}
	if !strings.Contains(reloaded.FinalURL, "httpbin.org") {
		t.Fatalf("unexpected reload result: %#v", reloaded)
	}
}
