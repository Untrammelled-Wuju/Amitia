package modelprotocol

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type streamEventSink struct {
	events []ModelEvent
}

func (s *streamEventSink) Emit(ctx context.Context, event ModelEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.events = append(s.events, event)
	return nil
}

func TestParseStreamFinalizesToolCallsBeforeCompletion(t *testing.T) {
	body := strings.NewReader(strings.Join([]string{
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_weather","function":{"name":"get_weather","arguments":""}}]}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"location\":\""}}]}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"Paris\"}"}}]}}]}`,
		`data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`,
		`data: [DONE]`,
	}, "\n\n") + "\n\n")
	sink := &streamEventSink{}

	result, err := (&OpenAIChatAdapter{}).parseStream(context.Background(), body, sink)
	if err != nil {
		t.Fatalf("parseStream returned error: %v", err)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("tool call count = %d, want 1", len(result.ToolCalls))
	}
	call := result.ToolCalls[0]
	if call.ID != "call_weather" || call.Name != "get_weather" {
		t.Fatalf("unexpected tool call: %#v", call)
	}
	if call.ArgumentsJSON != `{"location":"Paris"}` {
		t.Fatalf("tool arguments = %q", call.ArgumentsJSON)
	}

	wantTypes := []ModelEventType{
		ModelEventToolCallStarted,
		ModelEventToolCallArgumentsDelta,
		ModelEventToolCallArgumentsDelta,
		ModelEventToolCallDone,
		ModelEventCompleted,
	}
	if len(sink.events) != len(wantTypes) {
		t.Fatalf("event count = %d, want %d: %#v", len(sink.events), len(wantTypes), sink.events)
	}
	for index, want := range wantTypes {
		if sink.events[index].Type != want {
			t.Fatalf("event %d type = %q, want %q", index, sink.events[index].Type, want)
		}
	}
}

func TestParseStreamFinalizesToolCallsAtEOFInIndexOrder(t *testing.T) {
	body := strings.NewReader(strings.Join([]string{
		`data: {"choices":[{"delta":{"tool_calls":[{"index":1,"id":"call_b","function":{"name":"second","arguments":"{}"}}]}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_a","function":{"name":"first","arguments":"{}"}}]}}]}`,
	}, "\n\n") + "\n\n")
	sink := &streamEventSink{}

	result, err := (&OpenAIChatAdapter{}).parseStream(context.Background(), body, sink)
	if err != nil {
		t.Fatalf("parseStream returned error: %v", err)
	}
	if len(result.ToolCalls) != 2 {
		t.Fatalf("tool call count = %d, want 2", len(result.ToolCalls))
	}
	if result.ToolCalls[0].ID != "call_a" || result.ToolCalls[1].ID != "call_b" {
		t.Fatalf("tool calls are not in index order: %#v", result.ToolCalls)
	}
}

func TestOpenAIChatBuildMessagesPreservesToolLoopState(t *testing.T) {
	req := ModelRequest{
		Messages: []ModelMessage{
			{
				Role:  "assistant",
				Parts: []ModelContentPart{{Type: ContentTypeText, Text: ""}},
				ToolCalls: []ModelToolCall{
					{ID: "call_calculate", Name: "calculate", ArgumentsJSON: `{"expression":"2+2"}`},
				},
			},
			{
				Role:       "tool",
				Parts:      []ModelContentPart{{Type: ContentTypeText, Text: `{"result":4}`}},
				ToolCallID: "call_calculate",
			},
		},
	}

	messages := (&OpenAIChatAdapter{}).buildMessages(req)
	if len(messages) != 2 {
		t.Fatalf("message count = %d, want 2", len(messages))
	}
	toolCalls, ok := messages[0]["tool_calls"].([]map[string]interface{})
	if !ok || len(toolCalls) != 1 {
		t.Fatalf("assistant tool_calls missing: %#v", messages[0]["tool_calls"])
	}
	function, ok := toolCalls[0]["function"].(map[string]interface{})
	if !ok || function["name"] != "calculate" || function["arguments"] != `{"expression":"2+2"}` {
		t.Fatalf("assistant tool call mismatch: %#v", toolCalls[0])
	}
	if messages[1]["tool_call_id"] != "call_calculate" {
		t.Fatalf("tool_call_id missing: %#v", messages[1])
	}
}

func TestOpenAIChatBuildMessagesNeverSerializesNullContent(t *testing.T) {
	req := ModelRequest{
		Messages: []ModelMessage{
			{
				Role: "assistant",
				ToolCalls: []ModelToolCall{
					{ID: "call_empty", Name: "read_file_part", ArgumentsJSON: `{}`},
				},
			},
			{
				Role:       "tool",
				ToolCallID: "call_empty",
			},
		},
	}

	messages := (&OpenAIChatAdapter{}).buildMessages(req)
	if len(messages) != 2 {
		t.Fatalf("message count = %d, want 2", len(messages))
	}
	for index, message := range messages {
		content, ok := message["content"].(string)
		if !ok || content != "" {
			t.Fatalf("message %d content = %#v, want empty string", index, message["content"])
		}
	}
	encoded, err := json.Marshal(messages)
	if err != nil {
		t.Fatalf("marshal messages: %v", err)
	}
	if strings.Contains(string(encoded), `"content":null`) {
		t.Fatalf("messages contain null content: %s", encoded)
	}
}
