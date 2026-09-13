package modelprotocol

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type contextCaptureSink struct {
	contexts []context.Context
	events   []ModelEvent
}

func (s *contextCaptureSink) Emit(ctx context.Context, event ModelEvent) error {
	s.contexts = append(s.contexts, ctx)
	s.events = append(s.events, event)
	return ctx.Err()
}

func TestParseStreamPropagatesRequestContextToSink(t *testing.T) {
	ctx := context.WithValue(context.Background(), contextKey("trace"), "voice")
	sink := &contextCaptureSink{}
	body := strings.NewReader("data: {\"choices\":[{\"delta\":{\"content\":\"你好\"}}]}\n\ndata: [DONE]\n\n")

	result, err := (&OpenAIChatAdapter{}).parseStream(ctx, body, sink)
	if err != nil {
		t.Fatalf("parseStream returned error: %v", err)
	}
	if result.Text != "你好" {
		t.Fatalf("unexpected result text: %q", result.Text)
	}
	if len(sink.contexts) != 2 {
		t.Fatalf("expected two sink events, got %d", len(sink.contexts))
	}
	for _, got := range sink.contexts {
		if got != ctx {
			t.Fatalf("sink received the wrong context")
		}
	}
	if sink.events[0].Type != ModelEventTextDelta || sink.events[1].Type != ModelEventCompleted {
		t.Fatalf("unexpected sink events: %#v", sink.events)
	}
}

func TestParseStreamReturnsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	sink := &contextCaptureSink{}

	_, err := (&OpenAIChatAdapter{}).parseStream(ctx, strings.NewReader("data: [DONE]\n\n"), sink)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

type contextKey string
