package webresearch

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/browser"
)

type BrowserReader interface {
	Read(ctx context.Context, rawURL string) (fetchedPage, *Error)
	Screenshot(ctx context.Context, rawURL string, page *int, fullPage bool) (ScreenshotResult, *Error)
	Available(ctx context.Context) bool
}

type AmitiaBrowserReader struct {
	provider browser.BrowserProvider
}

func NewAmitiaBrowserReader(provider browser.BrowserProvider) *AmitiaBrowserReader {
	return &AmitiaBrowserReader{provider: provider}
}

func (r *AmitiaBrowserReader) Available(ctx context.Context) bool {
	if r == nil || r.provider == nil || r.provider.Runtime() == nil {
		return false
	}
	health := r.provider.Runtime().Health(ctx)
	return health == browser.BrowserHealthHealthy
}

func (r *AmitiaBrowserReader) Read(ctx context.Context, rawURL string) (fetchedPage, *Error) {
	if !r.Available(ctx) {
		return fetchedPage{}, newError(ErrBrowserUnavailable, "browser runtime unavailable", true, nil)
	}
	session, berr := r.provider.Sessions().CreateSession(ctx)
	if berr != nil {
		return fetchedPage{}, newError(ErrBrowserUnavailable, "create browser session failed", true, berr)
	}
	defer closeBrowserSession(r.provider, session.SessionID)
	tab, berr := r.provider.Tabs().CreateTab(ctx, session.SessionID)
	if berr != nil {
		return fetchedPage{}, newError(ErrBrowserUnavailable, "create browser tab failed", true, berr)
	}
	defer closeBrowserTab(r.provider, session.SessionID, tab.TabID)
	navigation, berr := r.provider.Navigate().Navigate(ctx, session.SessionID, tab.TabID, browser.NavigateRequest{URL: rawURL, WaitUntil: "domcontentloaded", TimeoutMS: 30000})
	if berr != nil {
		return fetchedPage{}, newError(ErrFetchFailed, "browser navigation failed", true, berr)
	}
	snapshot, berr := r.provider.Observe().GetDOMSnapshot(ctx, session.SessionID, tab.TabID, 16)
	if berr != nil {
		return fetchedPage{}, newError(ErrFetchFailed, "browser DOM snapshot failed", true, berr)
	}
	content := strings.TrimSpace(snapshot.Content)
	blocks := splitTextBlocks(content)
	return fetchedPage{URL: navigation.FinalURL, CanonicalURL: canonicalizeURL(navigation.FinalURL), Title: snapshot.Title, ContentType: "text/html+browser", Content: content, Blocks: blocks, Truncated: snapshot.Truncated, Dynamic: true, Hash: hashBytes([]byte(content))}, nil
}

func (r *AmitiaBrowserReader) Screenshot(ctx context.Context, rawURL string, page *int, fullPage bool) (ScreenshotResult, *Error) {
	if !r.Available(ctx) {
		return ScreenshotResult{}, newError(ErrBrowserUnavailable, "browser runtime unavailable", true, nil)
	}
	session, berr := r.provider.Sessions().CreateSession(ctx)
	if berr != nil {
		return ScreenshotResult{}, newError(ErrBrowserUnavailable, "create browser session failed", true, berr)
	}
	defer closeBrowserSession(r.provider, session.SessionID)
	tab, berr := r.provider.Tabs().CreateTab(ctx, session.SessionID)
	if berr != nil {
		return ScreenshotResult{}, newError(ErrBrowserUnavailable, "create browser tab failed", true, berr)
	}
	defer closeBrowserTab(r.provider, session.SessionID, tab.TabID)
	navigationURL := rawURL
	if page != nil {
		navigationURL = withDocumentPageFragment(rawURL, *page)
		fullPage = false
	}
	if _, berr := r.provider.Navigate().Navigate(ctx, session.SessionID, tab.TabID, browser.NavigateRequest{URL: navigationURL, WaitUntil: "domcontentloaded", TimeoutMS: 30000}); berr != nil {
		return ScreenshotResult{}, newError(ErrFetchFailed, "browser navigation failed", true, berr)
	}
	shot, berr := r.provider.Resources().Screenshot(ctx, browser.BrowserScreenshotRequest{SessionID: session.SessionID, TabID: tab.TabID, Format: "png", FullPage: fullPage})
	if berr != nil {
		return ScreenshotResult{}, newError(ErrFetchFailed, "browser screenshot failed", true, berr)
	}
	return ScreenshotResult{ResourceURI: shot.ResourceURI, Width: shot.Width, Height: shot.Height, Format: shot.Format, SizeBytes: shot.SizeBytes}, nil
}

const browserCleanupTimeout = 5 * time.Second

func closeBrowserTab(provider browser.BrowserProvider, sessionID browser.BrowserSessionID, tabID browser.BrowserTabID) {
	if provider == nil || provider.Tabs() == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), browserCleanupTimeout)
	defer cancel()
	_ = provider.Tabs().CloseTab(ctx, sessionID, tabID)
}

func closeBrowserSession(provider browser.BrowserProvider, sessionID browser.BrowserSessionID) {
	if provider == nil || provider.Sessions() == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), browserCleanupTimeout)
	defer cancel()
	_ = provider.Sessions().CloseSession(ctx, sessionID)
}

func withDocumentPageFragment(rawURL string, zeroBasedPage int) string {
	if zeroBasedPage < 0 {
		return rawURL
	}
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return rawURL
	}
	parsed.Fragment = "page=" + strconv.Itoa(zeroBasedPage+1)
	return parsed.String()
}
