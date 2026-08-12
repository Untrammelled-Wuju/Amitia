package browser

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeTabResolver struct {
	tabID    BrowserTabID
	targetID TargetID
	resolveErr *BrowserError
}

func (r *fakeTabResolver) ResolveTab(_ context.Context, sessionID BrowserSessionID, tabID BrowserTabID) (ResolvedBrowserTab, *BrowserError) {
	if r.resolveErr != nil {
		return ResolvedBrowserTab{}, r.resolveErr
	}
	return ResolvedBrowserTab{
		SessionID:        sessionID,
		TabID:            tabID,
		BrowserContextID: "ctx_1",
		TargetID:         r.targetID,
		RuntimeGeneration: 1,
	}, nil
}

type fakePageController struct {
	navigateResult *pageNavigateResult
	navigateErr    error
	reloadResult   *pageNavigateResult
	reloadErr      error
	goBackResult   *pageNavigateResult
	goBackErr      error
	goForwardResult *pageNavigateResult
	goForwardErr   error
	stopErr        error
}

func (c *fakePageController) Navigate(_ context.Context, _ TargetID, _ string, _ string, _ time.Duration) (*pageNavigateResult, error) {
	return c.navigateResult, c.navigateErr
}

func (c *fakePageController) Reload(_ context.Context, _ TargetID, _ bool, _ time.Duration) (*pageNavigateResult, error) {
	return c.reloadResult, c.reloadErr
}

func (c *fakePageController) GoBack(_ context.Context, _ TargetID) (*pageNavigateResult, error) {
	return c.goBackResult, c.goBackErr
}

func (c *fakePageController) GoForward(_ context.Context, _ TargetID) (*pageNavigateResult, error) {
	return c.goForwardResult, c.goForwardErr
}

func (c *fakePageController) Stop(_ context.Context, _ TargetID) error {
	return c.stopErr
}

func newTestPolicy() *NavigationPolicy {
	return NewNavigationPolicy(BrowserConfig{
		AllowedSchemes:       []string{"http", "https", "about"},
		NavigationTimeout:    30 * time.Second,
		MaxNavigationTimeout: 120 * time.Second,
	})
}

func TestProductionNavigatorNavigateSuccess(t *testing.T) {
	resolver := &fakeTabResolver{targetID: TargetID("target_1")}
	pageCtrl := &fakePageController{
		navigateResult: &pageNavigateResult{
			FinalURL:   "https://example.com",
			Title:      "Example",
			Loaded:     true,
			DurationMS: 100,
		},
	}
	policy := newTestPolicy()
	navigator := NewProductionNavigator(resolver, pageCtrl, policy)

	result, err := navigator.Navigate(context.Background(), "bs_s1", "bt_1", NavigateRequest{
		URL: "https://example.com",
	})
	if err != nil {
		t.Fatalf("navigate failed: %v", err)
	}
	if result.FinalURL != "https://example.com" {
		t.Fatalf("expected final URL https://example.com, got: %s", result.FinalURL)
	}
	if result.Title != "Example" {
		t.Fatalf("expected title Example, got: %s", result.Title)
	}
	if !result.Loaded {
		t.Fatal("expected loaded to be true")
	}
	if result.SessionID != "bs_s1" {
		t.Fatalf("expected session ID bs_s1, got: %s", result.SessionID)
	}
	if result.TabID != "bt_1" {
		t.Fatalf("expected tab ID bt_1, got: %s", result.TabID)
	}
}

func TestProductionNavigatorNavigateEmptyURL(t *testing.T) {
	resolver := &fakeTabResolver{targetID: TargetID("target_1")}
	pageCtrl := &fakePageController{}
	policy := newTestPolicy()
	navigator := NewProductionNavigator(resolver, pageCtrl, policy)

	_, err := navigator.Navigate(context.Background(), "bs_s1", "bt_1", NavigateRequest{})
	if err == nil {
		t.Fatal("should fail with empty URL")
	}
	if err.Code != ErrCodeInvalidRequest {
		t.Fatalf("expected invalid_request, got: %s", err.Code)
	}
}

func TestProductionNavigatorNavigateTabNotFound(t *testing.T) {
	resolver := &fakeTabResolver{
		resolveErr: &BrowserError{Code: ErrCodeTabNotFound, Message: "tab not found"},
	}
	pageCtrl := &fakePageController{}
	policy := newTestPolicy()
	navigator := NewProductionNavigator(resolver, pageCtrl, policy)

	_, err := navigator.Navigate(context.Background(), "bs_s1", "bt_nonexistent", NavigateRequest{URL: "https://example.com"})
	if err == nil {
		t.Fatal("should fail when tab not found")
	}
	if !IsTabNotFound(err) {
		t.Fatalf("expected tab_not_found, got: %v", err)
	}
}

func TestProductionNavigatorNavigateContextCancelled(t *testing.T) {
	resolver := &fakeTabResolver{targetID: TargetID("target_1")}
	pageCtrl := &fakePageController{}
	policy := newTestPolicy()
	navigator := NewProductionNavigator(resolver, pageCtrl, policy)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := navigator.Navigate(ctx, "bs_s1", "bt_1", NavigateRequest{URL: "https://example.com"})
	if err == nil {
		t.Fatal("should fail with cancelled context")
	}
	if err.Code != ErrCodeInvalidRequest {
		t.Fatalf("expected invalid_request, got: %s", err.Code)
	}
}

func TestProductionNavigatorNavigateBackendFailure(t *testing.T) {
	resolver := &fakeTabResolver{targetID: TargetID("target_1")}
	pageCtrl := &fakePageController{
		navigateErr: errors.New("cdp error"),
	}
	policy := newTestPolicy()
	navigator := NewProductionNavigator(resolver, pageCtrl, policy)

	_, err := navigator.Navigate(context.Background(), "bs_s1", "bt_1", NavigateRequest{URL: "https://example.com"})
	if err == nil {
		t.Fatal("should fail when backend fails")
	}
	if err.Code != ErrCodeNavigationFailed {
		t.Fatalf("expected navigation_failed, got: %s", err.Code)
	}
}

func TestProductionNavigatorNavigateBlockedScheme(t *testing.T) {
	resolver := &fakeTabResolver{targetID: TargetID("target_1")}
	pageCtrl := &fakePageController{}
	policy := newTestPolicy()
	navigator := NewProductionNavigator(resolver, pageCtrl, policy)

	_, err := navigator.Navigate(context.Background(), "bs_s1", "bt_1", NavigateRequest{URL: "ftp://example.com"})
	if err == nil {
		t.Fatal("should fail with blocked scheme")
	}
	if err.Code != ErrCodeNavigationFailed {
		t.Fatalf("expected navigation_failed, got: %s", err.Code)
	}
}

func TestProductionNavigatorNavigateBlockedLoopback(t *testing.T) {
	resolver := &fakeTabResolver{targetID: TargetID("target_1")}
	pageCtrl := &fakePageController{}
	policy := newTestPolicy()
	navigator := NewProductionNavigator(resolver, pageCtrl, policy)

	_, err := navigator.Navigate(context.Background(), "bs_s1", "bt_1", NavigateRequest{URL: "http://127.0.0.1:8080"})
	if err == nil {
		t.Fatal("should fail with loopback address")
	}
	if err.Code != ErrCodeNavigationFailed {
		t.Fatalf("expected navigation_failed, got: %s", err.Code)
	}
}

func TestProductionNavigatorNavigateWithTimeout(t *testing.T) {
	resolver := &fakeTabResolver{targetID: TargetID("target_1")}
	pageCtrl := &fakePageController{
		navigateResult: &pageNavigateResult{
			FinalURL: "https://example.com",
			Loaded:   true,
		},
	}
	policy := newTestPolicy()
	navigator := NewProductionNavigator(resolver, pageCtrl, policy)

	result, err := navigator.Navigate(context.Background(), "bs_s1", "bt_1", NavigateRequest{
		URL:       "https://example.com",
		TimeoutMS: 5000,
	})
	if err != nil {
		t.Fatalf("navigate failed: %v", err)
	}
	if result.FinalURL != "https://example.com" {
		t.Fatalf("expected final URL https://example.com, got: %s", result.FinalURL)
	}
}

func TestProductionNavigatorNavigateWithWaitUntil(t *testing.T) {
	resolver := &fakeTabResolver{targetID: TargetID("target_1")}
	pageCtrl := &fakePageController{
		navigateResult: &pageNavigateResult{
			FinalURL: "https://example.com",
			Loaded:   true,
		},
	}
	policy := newTestPolicy()
	navigator := NewProductionNavigator(resolver, pageCtrl, policy)

	result, err := navigator.Navigate(context.Background(), "bs_s1", "bt_1", NavigateRequest{
		URL:       "https://example.com",
		WaitUntil: "load",
	})
	if err != nil {
		t.Fatalf("navigate failed: %v", err)
	}
	if result.WaitUntil != "load" {
		t.Fatalf("expected waitUntil load, got: %s", result.WaitUntil)
	}
}

func TestProductionNavigatorReloadSuccess(t *testing.T) {
	resolver := &fakeTabResolver{targetID: TargetID("target_1")}
	pageCtrl := &fakePageController{
		reloadResult: &pageNavigateResult{
			FinalURL: "https://example.com",
			Loaded:   true,
		},
	}
	policy := newTestPolicy()
	navigator := NewProductionNavigator(resolver, pageCtrl, policy)

	result, err := navigator.Reload(context.Background(), "bs_s1", "bt_1", NavigateRequest{})
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	if result.FinalURL != "https://example.com" {
		t.Fatalf("expected final URL https://example.com, got: %s", result.FinalURL)
	}
}

func TestProductionNavigatorReloadTabNotFound(t *testing.T) {
	resolver := &fakeTabResolver{
		resolveErr: &BrowserError{Code: ErrCodeTabNotFound, Message: "tab not found"},
	}
	pageCtrl := &fakePageController{}
	policy := newTestPolicy()
	navigator := NewProductionNavigator(resolver, pageCtrl, policy)

	_, err := navigator.Reload(context.Background(), "bs_s1", "bt_nonexistent", NavigateRequest{})
	if err == nil {
		t.Fatal("should fail when tab not found")
	}
	if !IsTabNotFound(err) {
		t.Fatalf("expected tab_not_found, got: %v", err)
	}
}

func TestProductionNavigatorGoBackSuccess(t *testing.T) {
	resolver := &fakeTabResolver{targetID: TargetID("target_1")}
	pageCtrl := &fakePageController{
		goBackResult: &pageNavigateResult{
			FinalURL: "https://previous.com",
			Loaded:   true,
		},
	}
	policy := newTestPolicy()
	navigator := NewProductionNavigator(resolver, pageCtrl, policy)

	result, err := navigator.GoBack(context.Background(), "bs_s1", "bt_1")
	if err != nil {
		t.Fatalf("go back failed: %v", err)
	}
	if result.FinalURL != "https://previous.com" {
		t.Fatalf("expected final URL https://previous.com, got: %s", result.FinalURL)
	}
}

func TestProductionNavigatorGoBackTabNotFound(t *testing.T) {
	resolver := &fakeTabResolver{
		resolveErr: &BrowserError{Code: ErrCodeTabNotFound, Message: "tab not found"},
	}
	pageCtrl := &fakePageController{}
	policy := newTestPolicy()
	navigator := NewProductionNavigator(resolver, pageCtrl, policy)

	_, err := navigator.GoBack(context.Background(), "bs_s1", "bt_nonexistent")
	if err == nil {
		t.Fatal("should fail when tab not found")
	}
	if !IsTabNotFound(err) {
		t.Fatalf("expected tab_not_found, got: %v", err)
	}
}

func TestProductionNavigatorGoForwardSuccess(t *testing.T) {
	resolver := &fakeTabResolver{targetID: TargetID("target_1")}
	pageCtrl := &fakePageController{
		goForwardResult: &pageNavigateResult{
			FinalURL: "https://next.com",
			Loaded:   true,
		},
	}
	policy := newTestPolicy()
	navigator := NewProductionNavigator(resolver, pageCtrl, policy)

	result, err := navigator.GoForward(context.Background(), "bs_s1", "bt_1")
	if err != nil {
		t.Fatalf("go forward failed: %v", err)
	}
	if result.FinalURL != "https://next.com" {
		t.Fatalf("expected final URL https://next.com, got: %s", result.FinalURL)
	}
}

func TestProductionNavigatorGoForwardTabNotFound(t *testing.T) {
	resolver := &fakeTabResolver{
		resolveErr: &BrowserError{Code: ErrCodeTabNotFound, Message: "tab not found"},
	}
	pageCtrl := &fakePageController{}
	policy := newTestPolicy()
	navigator := NewProductionNavigator(resolver, pageCtrl, policy)

	_, err := navigator.GoForward(context.Background(), "bs_s1", "bt_nonexistent")
	if err == nil {
		t.Fatal("should fail when tab not found")
	}
	if !IsTabNotFound(err) {
		t.Fatalf("expected tab_not_found, got: %v", err)
	}
}

func TestProductionNavigatorStopSuccess(t *testing.T) {
	resolver := &fakeTabResolver{targetID: TargetID("target_1")}
	pageCtrl := &fakePageController{}
	policy := newTestPolicy()
	navigator := NewProductionNavigator(resolver, pageCtrl, policy)

	err := navigator.Stop(context.Background(), "bs_s1", "bt_1")
	if err != nil {
		t.Fatalf("stop failed: %v", err)
	}
}

func TestProductionNavigatorStopTabNotFound(t *testing.T) {
	resolver := &fakeTabResolver{
		resolveErr: &BrowserError{Code: ErrCodeTabNotFound, Message: "tab not found"},
	}
	pageCtrl := &fakePageController{}
	policy := newTestPolicy()
	navigator := NewProductionNavigator(resolver, pageCtrl, policy)

	err := navigator.Stop(context.Background(), "bs_s1", "bt_nonexistent")
	if err == nil {
		t.Fatal("should fail when tab not found")
	}
	if !IsTabNotFound(err) {
		t.Fatalf("expected tab_not_found, got: %v", err)
	}
}

func TestProductionNavigatorStopContextCancelled(t *testing.T) {
	resolver := &fakeTabResolver{targetID: TargetID("target_1")}
	pageCtrl := &fakePageController{}
	policy := newTestPolicy()
	navigator := NewProductionNavigator(resolver, pageCtrl, policy)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := navigator.Stop(ctx, "bs_s1", "bt_1")
	if err == nil {
		t.Fatal("should fail with cancelled context")
	}
	if err.Code != ErrCodeInvalidRequest {
		t.Fatalf("expected invalid_request, got: %s", err.Code)
	}
}

func TestProductionNavigatorNavigateRedirected(t *testing.T) {
	resolver := &fakeTabResolver{targetID: TargetID("target_1")}
	pageCtrl := &fakePageController{
		navigateResult: &pageNavigateResult{
			FinalURL:   "https://redirected.com",
			Redirected: true,
			Loaded:     true,
		},
	}
	policy := newTestPolicy()
	navigator := NewProductionNavigator(resolver, pageCtrl, policy)

	result, err := navigator.Navigate(context.Background(), "bs_s1", "bt_1", NavigateRequest{
		URL: "https://example.com",
	})
	if err != nil {
		t.Fatalf("navigate failed: %v", err)
	}
	if !result.Redirected {
		t.Fatal("expected redirected to be true")
	}
	if result.FinalURL != "https://redirected.com" {
		t.Fatalf("expected final URL https://redirected.com, got: %s", result.FinalURL)
	}
}

func TestProductionNavigatorNavigateTimedOut(t *testing.T) {
	resolver := &fakeTabResolver{targetID: TargetID("target_1")}
	pageCtrl := &fakePageController{
		navigateResult: &pageNavigateResult{
			FinalURL: "https://example.com",
			TimedOut: true,
		},
	}
	policy := newTestPolicy()
	navigator := NewProductionNavigator(resolver, pageCtrl, policy)

	result, err := navigator.Navigate(context.Background(), "bs_s1", "bt_1", NavigateRequest{
		URL: "https://example.com",
	})
	if err != nil {
		t.Fatalf("navigate should not fail on timeout: %v", err)
	}
	if !result.TimedOut {
		t.Fatal("expected timedOut to be true")
	}
}

func TestProductionNavigatorNavigateWithHTTPStatus(t *testing.T) {
	resolver := &fakeTabResolver{targetID: TargetID("target_1")}
	httpStatus := 200
	pageCtrl := &fakePageController{
		navigateResult: &pageNavigateResult{
			FinalURL:   "https://example.com",
			HTTPStatus: &httpStatus,
			Loaded:     true,
		},
	}
	policy := newTestPolicy()
	navigator := NewProductionNavigator(resolver, pageCtrl, policy)

	result, err := navigator.Navigate(context.Background(), "bs_s1", "bt_1", NavigateRequest{
		URL: "https://example.com",
	})
	if err != nil {
		t.Fatalf("navigate failed: %v", err)
	}
	if result.HTTPStatus == nil {
		t.Fatal("expected HTTP status to be set")
	}
	if *result.HTTPStatus != 200 {
		t.Fatalf("expected HTTP status 200, got: %d", *result.HTTPStatus)
	}
}

func TestNavigationPolicyValidateURLValid(t *testing.T) {
	policy := newTestPolicy()

	class, err := policy.ValidateURL("https://example.com")
	if err != nil {
		t.Fatalf("validate URL failed: %v", err)
	}
	if class != NavClassPublic {
		t.Fatalf("expected public class, got: %s", class)
	}
}

func TestNavigationPolicyValidateURLEmptyScheme(t *testing.T) {
	policy := newTestPolicy()

	_, err := policy.ValidateURL("example.com")
	if err == nil {
		t.Fatal("should fail with empty scheme")
	}
}

func TestNavigationPolicyValidateURLBlockedScheme(t *testing.T) {
	policy := newTestPolicy()

	_, err := policy.ValidateURL("ftp://example.com")
	if err == nil {
		t.Fatal("should fail with blocked scheme")
	}
}

func TestNavigationPolicyValidateURLLoopback(t *testing.T) {
	policy := newTestPolicy()

	class, err := policy.ValidateURL("http://127.0.0.1:8080")
	if err != nil {
		t.Fatalf("validate URL failed: %v", err)
	}
	if class != NavClassLoopback {
		t.Fatalf("expected loopback class, got: %s", class)
	}
}

func TestNavigationPolicyValidateURLMetadata(t *testing.T) {
	policy := newTestPolicy()

	_, err := policy.ValidateURL("http://169.254.169.254/latest/meta-data/")
	if err == nil {
		t.Fatal("should fail with metadata endpoint")
	}
}

func TestNavigationPolicyValidateURLTooLong(t *testing.T) {
	policy := newTestPolicy()

	longURL := "https://example.com/" + string(make([]byte, 17*1024))
	_, err := policy.ValidateURL(longURL)
	if err == nil {
		t.Fatal("should fail with too long URL")
	}
}

func TestNavigationPolicyResolveTimeout(t *testing.T) {
	policy := newTestPolicy()

	timeout := policy.ResolveTimeout(0)
	if timeout != policy.DefaultTimeout {
		t.Fatalf("expected default timeout, got: %v", timeout)
	}

	timeout = policy.ResolveTimeout(5000)
	if timeout != 5*time.Second {
		t.Fatalf("expected 5s timeout, got: %v", timeout)
	}

	timeout = policy.ResolveTimeout(200000)
	if timeout != policy.MaxTimeout {
		t.Fatalf("expected max timeout, got: %v", timeout)
	}
}

func TestNavigationPolicyCanNavigatePublic(t *testing.T) {
	policy := newTestPolicy()

	class, err := policy.CanNavigate("https://example.com")
	if err != nil {
		t.Fatalf("can navigate failed: %v", err)
	}
	if class != NavClassPublic {
		t.Fatalf("expected public class, got: %s", class)
	}
}

func TestNavigationPolicyCanNavigateLoopbackBlocked(t *testing.T) {
	policy := newTestPolicy()

	_, err := policy.CanNavigate("http://127.0.0.1:8080")
	if err == nil {
		t.Fatal("should fail with loopback blocked")
	}
}

func TestNavigationPolicyCanNavigatePrivateBlocked(t *testing.T) {
	policy := newTestPolicy()

	_, err := policy.CanNavigate("http://192.168.1.1")
	if err == nil {
		t.Fatal("should fail with private blocked")
	}
}

func TestNavigationPolicyClassifyHostLocalhost(t *testing.T) {
	policy := newTestPolicy()

	class := policy.classifyHost("localhost")
	if class != NavClassLoopback {
		t.Fatalf("expected loopback class, got: %s", class)
	}
}

func TestNavigationPolicyClassifyHostLinkLocal(t *testing.T) {
	policy := newTestPolicy()

	class := policy.classifyHost("169.254.1.1")
	if class != NavClassLinkLocal {
		t.Fatalf("expected link_local class, got: %s", class)
	}
}

func TestNavigationPolicyClassifyHostPublic(t *testing.T) {
	policy := newTestPolicy()

	class := policy.classifyHost("example.com")
	if class != NavClassPublic {
		t.Fatalf("expected public class, got: %s", class)
	}
}
