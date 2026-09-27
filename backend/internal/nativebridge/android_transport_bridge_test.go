package nativebridge

import "testing"

func TestNormalizeAndroidRequestFillsProtocolFields(t *testing.T) {
	req := normalizeAndroidRequest(Request{
		Operation: "ui.tree.snapshot",
	})

	if req.ProtocolVersion != AndroidBridgeProtocolVersion {
		t.Fatalf("protocol version = %d, want %d", req.ProtocolVersion, AndroidBridgeProtocolVersion)
	}
	if req.Platform != "android" {
		t.Fatalf("platform = %q, want android", req.Platform)
	}
	if req.RequestId == "" {
		t.Fatal("expected generated request ID")
	}
}

func TestNormalizeAndroidRequestPreservesExplicitValues(t *testing.T) {
	req := normalizeAndroidRequest(Request{
		ProtocolVersion: 7,
		RequestId:       "request-1",
		Platform:        "android-preview",
	})

	if req.ProtocolVersion != 7 {
		t.Fatalf("protocol version = %d, want 7", req.ProtocolVersion)
	}
	if req.RequestId != "request-1" {
		t.Fatalf("request ID = %q, want request-1", req.RequestId)
	}
	if req.Platform != "android-preview" {
		t.Fatalf("platform = %q, want android-preview", req.Platform)
	}
}
