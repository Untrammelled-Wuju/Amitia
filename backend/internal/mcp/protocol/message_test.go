package protocol

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestMessageKindsAndRequestIDs(t *testing.T) {
	stringRequest, err := Request("request-a", "tools/list", map[string]any{"cursor": "next"})
	if err != nil {
		t.Fatal(err)
	}
	if kind, _ := stringRequest.Kind(); kind != MessageRequest {
		t.Fatalf("unexpected request kind: %s", kind)
	}
	if key, _ := CanonicalID(stringRequest.ID, false); key != "s:request-a" {
		t.Fatalf("unexpected string id: %s", key)
	}

	numberRequest, err := Request(json.Number("42"), "resources/list", nil)
	if err != nil {
		t.Fatal(err)
	}
	if key, _ := CanonicalID(numberRequest.ID, false); key != "n:42" {
		t.Fatalf("unexpected numeric id: %s", key)
	}

	notification, err := Notification("notifications/tools/list_changed", nil)
	if err != nil {
		t.Fatal(err)
	}
	if kind, _ := notification.Kind(); kind != MessageNotification {
		t.Fatalf("unexpected notification kind: %s", kind)
	}

	response, err := Response(numberRequest.ID, map[string]any{"tools": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	if kind, _ := response.Kind(); kind != MessageResponse {
		t.Fatalf("unexpected response kind: %s", kind)
	}

	errorResponse, err := ErrorResponse(stringRequest.ID, NewError(ErrorInvalidParams, "invalid params", map[string]any{"field": "name"}))
	if err != nil {
		t.Fatal(err)
	}
	if kind, _ := errorResponse.Kind(); kind != MessageError {
		t.Fatalf("unexpected error kind: %s", kind)
	}
}

func TestDecodeRejectsInvalidAndOversizedMessages(t *testing.T) {
	cases := [][]byte{
		[]byte(`{"jsonrpc":"1.0","id":1,"result":{}}`),
		[]byte(`{"jsonrpc":"2.0","id":true,"method":"tools/list"}`),
		[]byte(`{"jsonrpc":"2.0","id":1,"result":{},"error":{"code":-32603,"message":"bad"}}`),
		[]byte(`{"jsonrpc":"2.0","method":""}`),
		[]byte(`not-json`),
		[]byte(`{"jsonrpc":"2.0","id":1,"result":{}} {}`),
	}
	for _, data := range cases {
		if _, err := Decode(data, 4096); !errors.Is(err, ErrInvalidMessage) {
			t.Fatalf("expected invalid message for %s, got %v", data, err)
		}
	}
	valid := []byte(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`)
	if _, err := Decode(valid, int64(len(valid)-1)); !errors.Is(err, ErrMessageTooLarge) {
		t.Fatalf("expected message too large, got %v", err)
	}
	message, err := Decode(valid, int64(len(valid)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Encode(message, 8); !errors.Is(err, ErrMessageTooLarge) {
		t.Fatalf("expected encoded message too large, got %v", err)
	}
}

func TestSupportedProtocolVersions(t *testing.T) {
	if !SupportsVersion(LatestProtocolVersion) || !SupportsVersion("2025-06-18") {
		t.Fatal("expected current stable versions to be supported")
	}
	if SupportsVersion("2099-01-01") {
		t.Fatal("unexpected unsupported version")
	}
}
