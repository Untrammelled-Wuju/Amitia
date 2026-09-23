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
	provider          browser.BrowserProvider
	maxScrolls        int
	scrollSettleDelay time.Duration
}

func NewAmitiaBrowserReader(provider browser.BrowserProvider) *AmitiaBrowserReader {
	return &AmitiaBrowserReader{provider: provider, maxScrolls: 2, scrollSettleDelay: 250 * time.Millisecond}
}

func (r *AmitiaBrowserReader) WithMaxScrolls(maxScrolls int) *AmitiaBrowserReader {
	if r == nil {
		return r
	}
	if maxScrolls < 0 {
		maxScrolls = 0
	}
	r.maxScrolls = maxScrolls
	return r
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
	defer closeBrowserSession(ctx, r.provider, session.SessionID)
	tab, berr := r.provider.Tabs().CreateTab(ctx, session.SessionID)
	if berr != nil {
		return fetchedPage{}, newError(ErrBrowserUnavailable, "create browser tab failed", true, berr)
	}
	defer closeBrowserTab(ctx, r.provider, session.SessionID, tab.TabID)
	navigation, berr := r.provider.Navigate().Navigate(ctx, session.SessionID, tab.TabID, browser.NavigateRequest{URL: rawURL, WaitUntil: "domcontentloaded", TimeoutMS: 30000})
	if berr != nil {
		return fetchedPage{}, newError(ErrFetchFailed, "browser navigation failed", true, berr)
	}
	snapshot, berr := r.provider.Observe().GetDOMSnapshot(ctx, session.SessionID, tab.TabID, 16)
	if berr != nil {
		return fetchedPage{}, newError(ErrFetchFailed, "browser DOM snapshot failed", true, berr)
	}
	snapshot, readErr := r.expandDynamicContent(ctx, session.SessionID, tab.TabID, snapshot)
	if readErr != nil {
		return fetchedPage{}, readErr
	}
	content := strings.TrimSpace(snapshot.Content)
	blocks := splitTextBlocks(content)
	return fetchedPage{URL: navigation.FinalURL, CanonicalURL: canonicalizeURL(navigation.FinalURL), Title: snapshot.Title, ContentType: "text/html+browser", Content: content, Blocks: blocks, Truncated: snapshot.Truncated, Dynamic: true, Hash: hashBytes([]byte(content))}, nil
}

func (r *AmitiaBrowserReader) expandDynamicContent(ctx context.Context, sessionID browser.BrowserSessionID, tabID browser.BrowserTabID, snapshot *browser.BrowserDOMSnapshot) (*browser.BrowserDOMSnapshot, *Error) {
	if r == nil || snapshot == nil || r.maxScrolls <= 0 || r.provider == nil {
		return snapshot, nil
	}
	if !r.provider.BrowserCapabilities().SupportsInteraction || r.provider.Interact() == nil || r.provider.Observe() == nil {
		return snapshot, nil
	}

	current := snapshot
	currentContent := strings.TrimSpace(current.Content)
	for i := 0; i < r.maxScrolls; i++ {
		if err := ctx.Err(); err != nil {
			return nil, newError(ErrCancelled, "browser read cancelled", false, err)
		}
		if _, berr := r.provider.Interact().Scroll(ctx, sessionID, tabID, "down"); berr != nil {
			// Dynamic expansion is opportunistic. Unsupported or page-specific
			// interaction failures must not discard the DOM already captured.
			return current, nil
		}
		if r.scrollSettleDelay > 0 {
			timer := time.NewTimer(r.scrollSettleDelay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, newError(ErrCancelled, "browser read cancelled", false, ctx.Err())
			case <-timer.C:
			}
		}

		next, berr := r.provider.Observe().GetDOMSnapshot(ctx, sessionID, tabID, 16)
		if berr != nil || next == nil {
			return current, nil
		}
		nextContent := strings.TrimSpace(next.Content)
		current = next
		if nextContent == currentContent {
			break
		}
		currentContent = nextContent
	}
	return current, nil
}

func (r *AmitiaBrowserReader) Screenshot(ctx context.Context, rawURL string, page *int, fullPage bool) (ScreenshotResult, *Error) {
	if !r.Available(ctx) {
		return ScreenshotResult{}, newError(ErrBrowserUnavailable, "browser runtime unavailable", true, nil)
	}
	session, berr := r.provider.Sessions().CreateSession(ctx)
	if berr != nil {
		return ScreenshotResult{}, newError(ErrBrowserUnavailable, "create browser session failed", true, berr)
	}
	defer closeBrowserSession(ctx, r.provider, session.SessionID)
	tab, berr := r.provider.Tabs().CreateTab(ctx, session.SessionID)
	if berr != nil {
		return ScreenshotResult{}, newError(ErrBrowserUnavailable, "create browser tab failed", true, berr)
	}
	defer closeBrowserTab(ctx, r.provider, session.SessionID, tab.TabID)
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

func closeBrowserTab(parent context.Context, provider browser.BrowserProvider, sessionID browser.BrowserSessionID, tabID browser.BrowserTabID) {
	if provider == nil || provider.Tabs() == nil {
		return
	}
	ctx, cancel := browserCleanupContext(parent)
	defer cancel()
	_ = provider.Tabs().CloseTab(ctx, sessionID, tabID)
}

func closeBrowserSession(parent context.Context, provider browser.BrowserProvider, sessionID browser.BrowserSessionID) {
	if provider == nil || provider.Sessions() == nil {
		return
	}
	ctx, cancel := browserCleanupContext(parent)
	defer cancel()
	_ = provider.Sessions().CloseSession(ctx, sessionID)
}

func browserCleanupContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	// Preserve request-scoped values for audit/trace propagation while
	// detaching cancellation. Cleanup always has its own hard deadline.
	return context.WithTimeout(context.WithoutCancel(parent), browserCleanupTimeout)
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
