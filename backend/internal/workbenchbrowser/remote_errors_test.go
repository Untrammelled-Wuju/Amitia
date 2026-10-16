package workbenchbrowser

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/browser"
)

func TestRemoteReadErrorCodesAndScope(t *testing.T) {
	provider := browser.NewDisabledProvider()
	invalid, err := ReadAgentTab(context.Background(), provider, "space", RemoteReadRequest{Operation: "click", ConversationID: "conversation", BrowserRef: "012345678901234567890123456789ab"})
	if err == nil || invalid.Ready {
		t.Fatal("invalid operation accepted")
	}
	result, ok := RemoteReadFailureResult(err)
	if !ok || result.ErrorCode != RemoteReadInvalidRequest || result.Retryable {
		t.Fatalf("wrong invalid-request result: %+v", result)
	}
	ref := ExposeAgentTab("space", "conversation", "session", "tab")
	if ref == "" {
		t.Fatal("reference allocation failed")
	}
	defer RevokeAgentTab(ref)
	_, err = ReadAgentTab(context.Background(), provider, "other-space", RemoteReadRequest{Operation: "state", ConversationID: "conversation", BrowserRef: ref})
	result, ok = RemoteReadFailureResult(err)
	if !ok || result.ErrorCode != RemoteReadCapabilityRevoked || result.Retryable {
		t.Fatalf("scope leak: %+v", result)
	}
	_, err = ReadAgentTab(context.Background(), provider, "space", RemoteReadRequest{Operation: "state", ConversationID: "conversation", BrowserRef: ref})
	result, ok = RemoteReadFailureResult(err)
	if !ok || result.ErrorCode != RemoteReadRuntimeUnavailable || !result.Retryable {
		t.Fatalf("runtime error misclassified: %+v", result)
	}
	if _, ok := RemoteReadFailureResult(errors.New("unexpected internal failure")); ok {
		t.Fatal("internal failure exposed to client")
	}
}

func TestRemoteFailureTransportRoundTrip(t *testing.T) {
	err := &RemoteReadFailure{Code: RemoteReadCapabilityRevoked, Message: "expired"}
	payload, ok := RemoteReadFailureResult(err)
	if !ok || payload.Ready || payload.Observed || payload.Retryable {
		t.Fatal("expired reference must not look ready")
	}
}

type revokingProvider struct {
	browser.BrowserProvider
	observer  browser.BrowserObserver
	resources browser.BrowserResourceTransfer
	tabs      browser.BrowserTabManager
}

func (p revokingProvider) Tabs() browser.BrowserTabManager            { return p.tabs }
func (p revokingProvider) Observe() browser.BrowserObserver           { return p.observer }
func (p revokingProvider) Resources() browser.BrowserResourceTransfer { return p.resources }

type returningTab struct{ browser.BrowserTabManager }

func (returningTab) GetTab(_ context.Context, _ browser.BrowserSessionID, _ browser.BrowserTabID) (browser.BrowserTabInfo, *browser.BrowserError) {
	return browser.BrowserTabInfo{URL: "https://example.org", Title: "Example"}, nil
}

type revokingSnapshot struct {
	browser.BrowserObserver
	ref string
}

func (o revokingSnapshot) GetDOMSnapshot(_ context.Context, _ browser.BrowserSessionID, _ browser.BrowserTabID, _ int) (*browser.BrowserDOMSnapshot, *browser.BrowserError) {
	RevokeAgentTab(o.ref)
	return &browser.BrowserDOMSnapshot{URL: "https://example.org", Content: "private"}, nil
}

func TestRevokedDuringRemoteReadDoesNotReturnContent(t *testing.T) {
	ref := ExposeAgentTab("scope-revoked-during-read", "conversation", "session", "tab")
	if ref == "" {
		t.Fatal("missing reference")
	}
	provider := revokingProvider{tabs: returningTab{}, observer: revokingSnapshot{ref: ref}}
	result, err := ReadAgentTab(context.Background(), provider, "scope-revoked-during-read", RemoteReadRequest{Operation: "dom", ConversationID: "conversation", BrowserRef: ref})
	if result.Content != "" || result.Ready {
		t.Fatal("content escaped after authorization revocation")
	}
	failure, ok := RemoteReadFailureResult(err)
	if !ok || failure.ErrorCode != RemoteReadCapabilityRevoked {
		t.Fatalf("expected capability revocation: %+v", failure)
	}
}

func TestTerminalBrowserTabErrors(t *testing.T) {
	for _, code := range []browser.BrowserErrorCode{browser.ErrCodeTabNotFound, browser.ErrCodeSessionNotFound, browser.ErrCodeTabStale, browser.ErrCodeSessionStale, browser.ErrCodeTabCrashed} {
		failure := classifyBrowserObservationError(&browser.BrowserError{Code: code})
		if failure.Code != RemoteReadCapabilityRevoked || failure.Retryable {
			t.Fatalf("closed browser tab incorrectly retryable: %s", code)
		}
	}
	failure := classifyBrowserObservationError(&browser.BrowserError{Code: browser.ErrCodeBrowserRuntimeNotReady})
	if failure.Code != RemoteReadRuntimeUnavailable || !failure.Retryable {
		t.Fatal("transient browser startup incorrectly terminal")
	}
}
