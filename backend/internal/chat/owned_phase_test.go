package chat

import (
	"github.com/u-ai/backend/internal/devicemesh/business"
	"testing"
)

func TestOwnedReasoningEndsBeforeFirstTextDelta(t *testing.T) {
	var events []business.Event
	sink := &ownedReplySink{inference: business.Inference{Emit: func(event business.Event) error { events = append(events, event); return nil }}}
	if err := sink.Emit(t.Context(), ModelEvent{Type: ModelEventReasoningSummaryDelta, TextDelta: "reason"}); err != nil {
		t.Fatal(err)
	}
	if err := sink.Emit(t.Context(), ModelEvent{Type: ModelEventTextDelta, TextDelta: "reply"}); err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || !events[0].Reasoning || events[1].Type != "reasoning-completed" || events[2].Reasoning || events[2].Text != "reply" {
		t.Fatalf("events=%+v", events)
	}
}
