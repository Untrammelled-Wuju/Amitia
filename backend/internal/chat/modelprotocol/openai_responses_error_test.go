package modelprotocol

import (
	"context"
	"strings"
	"testing"
)

func TestOpenAIResponsesStreamPreservesFailureDetail(t *testing.T) {
	body := strings.NewReader("data: {\"type\":\"response.failed\",\"response\":{\"status\":\"failed\",\"error\":{\"code\":\"unsupported_model\",\"message\":\"model does not support responses\"}}}\n\n")
	sink := &streamEventSink{}

	_, err := (&OpenAIResponsesAdapter{}).parseStream(context.Background(), body, sink)
	if err == nil || !strings.Contains(err.Error(), "model does not support responses") || !strings.Contains(err.Error(), "unsupported_model") {
		t.Fatalf("failure detail lost: %v", err)
	}
	if len(sink.events) != 1 || sink.events[0].Type != ModelEventFailed || sink.events[0].Error == nil || !strings.Contains(sink.events[0].Error.Message, "model does not support responses") {
		t.Fatalf("failure event detail lost: %#v", sink.events)
	}
}

func TestOpenAIResponsesNonStreamingPreservesFailureDetail(t *testing.T) {
	response := []byte(`{"status":"failed","error":{"code":"unsupported_model","message":"model does not support responses"}}`)
	_, err := (&OpenAIResponsesAdapter{}).parseResponse(response)
	if err == nil || !strings.Contains(err.Error(), "model does not support responses") {
		t.Fatalf("failure detail lost: %v", err)
	}
}
