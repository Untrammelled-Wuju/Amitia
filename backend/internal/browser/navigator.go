package browser

import (
	"context"
	"sync"
	"time"
)

type BrowserPageController interface {
	Navigate(ctx context.Context, targetID TargetID, url string, waitUntil string, timeout time.Duration) (*pageNavigateResult, error)
	Reload(ctx context.Context, targetID TargetID, ignoreCache bool, timeout time.Duration) (*pageNavigateResult, error)
	GoBack(ctx context.Context, targetID TargetID) (*pageNavigateResult, error)
	GoForward(ctx context.Context, targetID TargetID) (*pageNavigateResult, error)
	Stop(ctx context.Context, targetID TargetID) error
}

type pageNavigateResult struct {
	FrameID    string
	LoaderID   string
	ErrorText  string
	FinalURL   string
	Title      string
	HTTPStatus *int
	Redirected bool
	Loaded     bool
	TimedOut   bool
	DurationMS int64
}

type productionNavigator struct {
	tabResolver TabResolver
	pageCtrl    BrowserPageController
	policy      *NavigationPolicy
	tabMgr      *productionTabManager
	mu          sync.RWMutex
}

func NewProductionNavigator(tabResolver TabResolver, pageCtrl BrowserPageController, policy *NavigationPolicy) BrowserNavigator {
	return &productionNavigator{
		tabResolver: tabResolver,
		pageCtrl:    pageCtrl,
		policy:      policy,
	}
}

func (n *productionNavigator) SetTabManager(tabMgr *productionTabManager) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.tabMgr = tabMgr
}

func (n *productionNavigator) Navigate(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID, request NavigateRequest) (NavigationResult, *BrowserError) {
	if err := ctx.Err(); err != nil {
		return NavigationResult{}, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "context cancelled",
			Cause:   err,
		}
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	resolved, err := n.tabResolver.ResolveTab(ctx, sessionID, tabID)
	if err != nil {
		return NavigationResult{}, err
	}

	rawURL := request.URL
	if rawURL == "" {
		return NavigationResult{}, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "url is required",
		}
	}

	_, policyErr := n.policy.CanNavigate(rawURL)
	if policyErr != nil {
		return NavigationResult{}, policyErr
	}

	waitUntil := request.WaitUntil
	if waitUntil == "" {
		waitUntil = "dom_content_loaded"
	}

	timeout := n.policy.ResolveTimeout(request.TimeoutMS)

	startTime := time.Now()

	var navigateErr error
	var result *pageNavigateResult

	if isReloadURL(rawURL, resolved.TargetID) {
		result, navigateErr = n.pageCtrl.Reload(ctx, resolved.TargetID, false, timeout)
	} else {
		result, navigateErr = n.pageCtrl.Navigate(ctx, resolved.TargetID, rawURL, waitUntil, timeout)
	}

	if navigateErr != nil {
		return NavigationResult{
				SessionID:    sessionID,
				TabID:        tabID,
				RequestedURL: rawURL,
				WaitUntil:    waitUntil,
				DurationMS:   time.Since(startTime).Milliseconds(),
			}, &BrowserError{
				Code:    ErrCodeNavigationFailed,
				Message: "navigation failed: " + navigateErr.Error(),
				Cause:   navigateErr,
			}
	}

	n.bumpDocumentGeneration(tabID, resolved.RuntimeGeneration)

	return NavigationResult{
		SessionID:    sessionID,
		TabID:        tabID,
		RequestedURL: rawURL,
		FinalURL:     result.FinalURL,
		Title:        result.Title,
		Redirected:   result.Redirected,
		HTTPStatus:   result.HTTPStatus,
		WaitUntil:    waitUntil,
		Loaded:       result.Loaded,
		TimedOut:     result.TimedOut,
		DurationMS:   result.DurationMS,
	}, nil
}

func (n *productionNavigator) Reload(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID, request NavigateRequest) (NavigationResult, *BrowserError) {
	if err := ctx.Err(); err != nil {
		return NavigationResult{}, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "context cancelled",
			Cause:   err,
		}
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	resolved, err := n.tabResolver.ResolveTab(ctx, sessionID, tabID)
	if err != nil {
		return NavigationResult{}, err
	}

	timeout := n.policy.ResolveTimeout(request.TimeoutMS)
	waitUntil := request.WaitUntil
	if waitUntil == "" {
		waitUntil = "dom_content_loaded"
	}

	ignoreCache := false
	startTime := time.Now()

	result, navigateErr := n.pageCtrl.Reload(ctx, resolved.TargetID, ignoreCache, timeout)
	if navigateErr != nil {
		return NavigationResult{
				SessionID:  sessionID,
				TabID:      tabID,
				WaitUntil:  waitUntil,
				DurationMS: time.Since(startTime).Milliseconds(),
			}, &BrowserError{
				Code:    ErrCodeNavigationFailed,
				Message: "reload failed: " + navigateErr.Error(),
				Cause:   navigateErr,
			}
	}

	n.bumpDocumentGeneration(tabID, resolved.RuntimeGeneration)

	return NavigationResult{
		SessionID:  sessionID,
		TabID:      tabID,
		FinalURL:   result.FinalURL,
		Title:      result.Title,
		Redirected: result.Redirected,
		HTTPStatus: result.HTTPStatus,
		WaitUntil:  waitUntil,
		Loaded:     result.Loaded,
		TimedOut:   result.TimedOut,
		DurationMS: result.DurationMS,
	}, nil
}

func (n *productionNavigator) GoBack(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID) (NavigationResult, *BrowserError) {
	if err := ctx.Err(); err != nil {
		return NavigationResult{}, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "context cancelled",
			Cause:   err,
		}
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	resolved, err := n.tabResolver.ResolveTab(ctx, sessionID, tabID)
	if err != nil {
		return NavigationResult{}, err
	}

	result, navigateErr := n.pageCtrl.GoBack(ctx, resolved.TargetID)
	if navigateErr != nil {
		return NavigationResult{
				SessionID: sessionID,
				TabID:     tabID,
			}, &BrowserError{
				Code:    ErrCodeNavigationFailed,
				Message: "go back failed: " + navigateErr.Error(),
				Cause:   navigateErr,
			}
	}

	return NavigationResult{
		SessionID:  sessionID,
		TabID:      tabID,
		FinalURL:   result.FinalURL,
		Title:      result.Title,
		Redirected: result.Redirected,
		HTTPStatus: result.HTTPStatus,
		WaitUntil:  "dom_content_loaded",
		Loaded:     result.Loaded,
		TimedOut:   result.TimedOut,
		DurationMS: result.DurationMS,
	}, nil
}

func (n *productionNavigator) GoForward(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID) (NavigationResult, *BrowserError) {
	if err := ctx.Err(); err != nil {
		return NavigationResult{}, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "context cancelled",
			Cause:   err,
		}
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	resolved, err := n.tabResolver.ResolveTab(ctx, sessionID, tabID)
	if err != nil {
		return NavigationResult{}, err
	}

	result, navigateErr := n.pageCtrl.GoForward(ctx, resolved.TargetID)
	if navigateErr != nil {
		return NavigationResult{
				SessionID: sessionID,
				TabID:     tabID,
			}, &BrowserError{
				Code:    ErrCodeNavigationFailed,
				Message: "go forward failed: " + navigateErr.Error(),
				Cause:   navigateErr,
			}
	}

	return NavigationResult{
		SessionID:  sessionID,
		TabID:      tabID,
		FinalURL:   result.FinalURL,
		Title:      result.Title,
		Redirected: result.Redirected,
		HTTPStatus: result.HTTPStatus,
		WaitUntil:  "dom_content_loaded",
		Loaded:     result.Loaded,
		TimedOut:   result.TimedOut,
		DurationMS: result.DurationMS,
	}, nil
}

func (n *productionNavigator) Stop(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID) *BrowserError {
	if err := ctx.Err(); err != nil {
		return &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "context cancelled",
			Cause:   err,
		}
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	resolved, err := n.tabResolver.ResolveTab(ctx, sessionID, tabID)
	if err != nil {
		return err
	}

	if stopErr := n.pageCtrl.Stop(ctx, resolved.TargetID); stopErr != nil {
		return &BrowserError{
			Code:    ErrCodeNavigationFailed,
			Message: "stop loading failed: " + stopErr.Error(),
			Cause:   stopErr,
		}
	}
	return nil
}

func (n *productionNavigator) bumpDocumentGeneration(tabID BrowserTabID, runtimeGeneration uint64) {
	if n.tabMgr != nil {
		n.tabMgr.store.bumpDocumentGeneration(tabID)
	}
}

func isReloadURL(rawURL string, _ TargetID) bool {
	return false
}
