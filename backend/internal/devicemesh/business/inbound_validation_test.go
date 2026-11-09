package business

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestValidateInboundRequestRejectsCallerInput(t *testing.T) {
	cases := []struct {
		name    string
		request Request
	}{
		{name: "empty message", request: Request{RequestID: "id", Message: "  "}},
		{name: "empty request id", request: Request{Message: "hi"}},
		{name: "request id too long", request: Request{RequestID: strings.Repeat("r", 129), Message: "hi"}},
		{name: "message above limit", request: Request{RequestID: "id", Message: strings.Repeat("长", (128<<10)/3+1)}},
		{name: "unknown message style", request: Request{RequestID: "id", Message: "hi", MessageStyle: "cards"}},
		{name: "negative model config", request: Request{RequestID: "id", Message: "hi", ModelConfigID: -1}},
		{name: "unknown reasoning effort", request: Request{RequestID: "id", Message: "hi", ReasoningEffort: "maximum"}},
		{name: "unknown permission mode", request: Request{RequestID: "id", Message: "hi", PermissionMode: "yolo"}},
		{
			name: "forwarded context without conversation",
			request: Request{RequestID: "id", Message: "hi", Context: &ForwardedContext{
				PreviousCoreID: "core",
				Messages:       []ContextMessage{{ID: "m1", OwnerID: "o", Role: "user", Content: "hi"}},
			}},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := validateInboundRequest(&testCase.request)
			if err == nil {
				t.Fatal("invalid input accepted")
			}
			if !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("error not classified as invalid request: %v", err)
			}
			described := DescribeRuntimeError(err)
			if described.Retryable {
				t.Fatalf("invalid request reported as retryable: %+v", described)
			}
			if described.ErrorCode != "invalid_request" {
				t.Fatalf("error code %q, want invalid_request", described.ErrorCode)
			}
		})
	}
}

func TestValidateInboundRequestAcceptsNormalTurn(t *testing.T) {
	request := Request{RequestID: " id ", Message: "  你好  ", MessageStyle: "bubble", ReasoningEffort: "high", PermissionMode: "request_approval"}
	if err := validateInboundRequest(&request); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	if request.RequestID != "id" || request.Message != "你好" {
		t.Fatalf("request not trimmed: %q %q", request.RequestID, request.Message)
	}
	body, _ := json.Marshal(request)
	if !strings.Contains(string(body), "你好") {
		t.Fatal("normalised request lost message")
	}
}
