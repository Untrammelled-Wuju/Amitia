package workbenchbrowser

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/browser"
)

func TestRemoteReadRequestStrictOperations(t *testing.T) {
	ref := "012345678901234567890123456789ab"
	for _, operation := range []string{"state", "dom", "screenshot"} {
		if !(RemoteReadRequest{Operation: operation, ConversationID: "conversation-1", BrowserRef: ref}).Valid() {
			t.Fatalf("safe operation rejected: %s", operation)
		}
	}
	for _, operation := range []string{"", "navigate", "control", "input", "close", "open", "../state", "SCReenshot"} {
		if (RemoteReadRequest{Operation: operation, ConversationID: "conversation-1", BrowserRef: ref}).Valid() {
			t.Fatalf("mutation accepted: %s", operation)
		}
	}
	if (RemoteReadRequest{Operation: "state", ConversationID: "../invalid", BrowserRef: ref}).Valid() {
		t.Fatal("invalid conversation accepted")
	}
	for _, malformed := range []string{"bad", "012345678901234567890123456789!?", "012345678901234567890123456789//", "１２３４５６７８９０１２３４５６７８９０１２３４５６７８９０１２"} {
		if (RemoteReadRequest{Operation: "state", ConversationID: "conversation-1", BrowserRef: malformed}).Valid() {
			t.Fatalf("invalid capability accepted: %q", malformed)
		}
	}
}

func TestRemoteReadRefScopeAndRevocation(t *testing.T) {
	provider := browser.NewDisabledProvider()
	ref := ExposeAgentTab("space", "conversation", "session", "tab")
	if ref == "" {
		t.Fatal("missing ref")
	}
	request := RemoteReadRequest{Operation: "state", ConversationID: "conversation", BrowserRef: ref}
	if _, err := ReadAgentTab(context.Background(), provider, "other-space", request); err == nil {
		t.Fatal("cross-space read permitted")
	}
	request.ConversationID = "other-conversation"
	if _, err := ReadAgentTab(context.Background(), provider, "space", request); err == nil {
		t.Fatal("cross-conversation read permitted")
	}
	RevokeAgentTab(ref)
	request.ConversationID = "conversation"
	if _, err := ReadAgentTab(context.Background(), provider, "space", request); err == nil {
		t.Fatal("revoked read permitted")
	}
}

func TestRemoteReadResultRejectsCrossOperationAndOversizeData(t *testing.T) {
	base := RemoteReadResult{Ready: true, Observed: true, URL: "https://example.com", Title: "Test", State: "ready"}
	if !ValidRemoteReadResult("state", base) {
		t.Fatal("valid state rejected")
	}
	wrong := base
	wrong.Content = "injected"
	if ValidRemoteReadResult("state", wrong) {
		t.Fatal("state accepted unrequested DOM")
	}
	wrong = base
	wrong.ErrorCode = RemoteReadCapabilityRevoked
	if ValidRemoteReadResult("state", wrong) {
		t.Fatal("error reported as successful state")
	}
	dom := base
	dom.Content = "<main>你好</main>"
	if !ValidRemoteReadResult("dom", dom) {
		t.Fatal("valid DOM rejected")
	}
	dom.Content = strings.Repeat("x", maxBrowserDOMBytes+1)
	if ValidRemoteReadResult("dom", dom) {
		t.Fatal("oversize DOM accepted")
	}
	screenshot := base
	screenshot.ContentType = "image/jpeg"
	screenshot.ImageBase64 = base64.StdEncoding.EncodeToString([]byte{0xff, 0xd8, 0xff, 0xd9})
	if !ValidRemoteReadResult("screenshot", screenshot) {
		t.Fatal("valid JPEG response rejected")
	}
	if ValidRemoteReadResult("dom", screenshot) {
		t.Fatal("DOM operation accepted screenshot payload")
	}
	screenshot.ImageBase64 = base64.StdEncoding.EncodeToString([]byte("not an image"))
	if ValidRemoteReadResult("screenshot", screenshot) {
		t.Fatal("invalid JPEG accepted")
	}
	screenshot.ImageBase64 = "!!!"
	if ValidRemoteReadResult("screenshot", screenshot) {
		t.Fatal("invalid base64 accepted")
	}
	screenshot.ImageBase64 = strings.Repeat("A", base64.StdEncoding.EncodedLen(MaxRemoteImageBytes)+4)
	if ValidRemoteReadResult("screenshot", screenshot) {
		t.Fatal("oversize image accepted")
	}
	if ValidRemoteReadResult("unknown", base) {
		t.Fatal("unexpected remote operation accepted")
	}
}

func TestRemoteReadFailureRejectsMalformedAndUntrustedPayloads(t *testing.T) {
	cases := []struct {
		code  RemoteReadErrorCode
		retry bool
	}{
		{RemoteReadInvalidRequest, false},
		{RemoteReadCapabilityRevoked, false},
		{RemoteReadRuntimeUnavailable, true},
		{RemoteReadUnsupported, false},
		{RemoteReadResourceExceeded, false},
	}
	for _, item := range cases {
		base := RemoteReadResult{ErrorCode: item.code, Error: "sensitive content", Retryable: item.retry}
		sanitized, ok := NormalizeRemoteReadFailure(base)
		if !ok || sanitized.Error == base.Error || sanitized.ErrorCode != item.code || sanitized.Retryable != item.retry {
			t.Fatalf("valid failure not sanitized: %s", item.code)
		}
		tampered := []RemoteReadResult{
			{ErrorCode: item.code, Error: "", Retryable: item.retry},
			{ErrorCode: item.code, Error: base.Error, Retryable: !item.retry},
			{ErrorCode: item.code, Error: base.Error, Retryable: item.retry, Ready: true},
			{ErrorCode: item.code, Error: base.Error, Retryable: item.retry, Content: "<iframe>"},
			{ErrorCode: item.code, Error: base.Error, Retryable: item.retry, ImageBase64: "AA=="},
		}
		for _, altered := range tampered {
			if _, ok := NormalizeRemoteReadFailure(altered); ok {
				t.Fatalf("malformed failure accepted for %s", item.code)
			}
		}
	}
	unknown := RemoteReadResult{ErrorCode: "arbitrary_remote_code", Error: "spoofed", Retryable: false}
	if _, ok := NormalizeRemoteReadFailure(unknown); ok {
		t.Fatal("arbitrary remote error code accepted")
	}
}
