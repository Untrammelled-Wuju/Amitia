package stream

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func TestReconnectManager_ResumeSuccess(t *testing.T) {
	resolver := NewPolicyResolver()
	sm := NewStreamManager(resolver)
	rm := NewReconnectManager(sm)

	input := PolicyInput{Kind: domain.ChannelKindEvent}
	runtimeID := domain.RuntimeInstanceID("rt-conn")
	serviceID := domain.ServiceID("svc-conn")
	channelID := domain.ChannelID("ch-conn")

	for i := 0; i < 10; i++ {
		err := sm.Publish(nil, input, runtimeID, serviceID, channelID, []byte{byte(i)})
		if err != nil {
			t.Fatalf("publish: %v", err)
		}
	}

	gen := sm.GetGeneration(runtimeID, serviceID, channelID)
	cursor := NewCursor(runtimeID, serviceID, channelID, gen, 7)

	result, err := rm.Resume(context.Background(), runtimeID, serviceID, channelID, cursor)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if result.Replayed != 3 {
		t.Errorf("expected 3 replayed, got %d", result.Replayed)
	}
	if result.Latest != Sequence(10) {
		t.Errorf("expected latest 10, got %d", result.Latest)
	}
}

func TestReconnectManager_CursorAtLatest(t *testing.T) {
	resolver := NewPolicyResolver()
	sm := NewStreamManager(resolver)
	rm := NewReconnectManager(sm)

	input := PolicyInput{Kind: domain.ChannelKindEvent}
	runtimeID := domain.RuntimeInstanceID("rt-latest")
	serviceID := domain.ServiceID("svc-latest")
	channelID := domain.ChannelID("ch-latest")

	for i := 0; i < 5; i++ {
		err := sm.Publish(nil, input, runtimeID, serviceID, channelID, []byte{byte(i)})
		if err != nil {
			t.Fatalf("publish: %v", err)
		}
	}

	gen := sm.GetGeneration(runtimeID, serviceID, channelID)
	cursor := NewCursor(runtimeID, serviceID, channelID, gen, 5)

	result, err := rm.Resume(context.Background(), runtimeID, serviceID, channelID, cursor)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if result.Replayed != 0 {
		t.Errorf("expected 0 replayed at latest, got %d", result.Replayed)
	}
}

func TestReconnectManager_CursorAhead(t *testing.T) {
	resolver := NewPolicyResolver()
	sm := NewStreamManager(resolver)
	rm := NewReconnectManager(sm)

	input := PolicyInput{Kind: domain.ChannelKindEvent}
	runtimeID := domain.RuntimeInstanceID("rt-ahead")
	serviceID := domain.ServiceID("svc-ahead")
	channelID := domain.ChannelID("ch-ahead")

	err := sm.Publish(nil, input, runtimeID, serviceID, channelID, []byte("a"))
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	gen := sm.GetGeneration(runtimeID, serviceID, channelID)
	cursor := NewCursor(runtimeID, serviceID, channelID, gen, 999)

	_, err = rm.Resume(context.Background(), runtimeID, serviceID, channelID, cursor)
	if err != ErrCursorAhead {
		t.Errorf("expected ErrCursorAhead, got %v", err)
	}
}

func TestReconnectManager_GenerationMismatch(t *testing.T) {
	resolver := NewPolicyResolver()
	sm := NewStreamManager(resolver)
	rm := NewReconnectManager(sm)

	input := PolicyInput{Kind: domain.ChannelKindEvent}
	runtimeID := domain.RuntimeInstanceID("rt-gen")
	serviceID := domain.ServiceID("svc-gen")
	channelID := domain.ChannelID("ch-gen")

	err := sm.Publish(nil, input, runtimeID, serviceID, channelID, []byte("a"))
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	cursor := NewCursor(runtimeID, serviceID, channelID, "wrong-gen", 1)
	_, err = rm.Resume(context.Background(), runtimeID, serviceID, channelID, cursor)
	if err != ErrGenerationMismatch {
		t.Errorf("expected ErrGenerationMismatch, got %v", err)
	}
}

func TestReconnectManager_WrongRuntime(t *testing.T) {
	resolver := NewPolicyResolver()
	sm := NewStreamManager(resolver)
	rm := NewReconnectManager(sm)

	input := PolicyInput{Kind: domain.ChannelKindEvent}
	runtimeID := domain.RuntimeInstanceID("rt-wrong")
	serviceID := domain.ServiceID("svc-wrong")
	channelID := domain.ChannelID("ch-wrong")

	err := sm.Publish(nil, input, runtimeID, serviceID, channelID, []byte("a"))
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	gen := sm.GetGeneration(runtimeID, serviceID, channelID)
	cursor := NewCursor("other-rt", serviceID, channelID, gen, 1)

	_, err = rm.Resume(context.Background(), "other-rt", serviceID, channelID, cursor)
	if err != ErrStreamClosed {
		t.Errorf("expected ErrStreamClosed (stream not found), got %v", err)
	}
}

func TestReconnectManager_ResumeAfterStreamRemove(t *testing.T) {
	resolver := NewPolicyResolver()
	sm := NewStreamManager(resolver)
	rm := NewReconnectManager(sm)

	input := PolicyInput{Kind: domain.ChannelKindEvent}
	runtimeID := domain.RuntimeInstanceID("rt-rm")
	serviceID := domain.ServiceID("svc-rm")
	channelID := domain.ChannelID("ch-rm")

	err := sm.Publish(nil, input, runtimeID, serviceID, channelID, []byte("a"))
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	gen := sm.GetGeneration(runtimeID, serviceID, channelID)
	sm.RemoveStream(runtimeID, serviceID, channelID)

	cursor := NewCursor(runtimeID, serviceID, channelID, gen, 1)
	_, err = rm.Resume(context.Background(), runtimeID, serviceID, channelID, cursor)
	if err != ErrStreamClosed {
		t.Errorf("expected ErrStreamClosed after remove, got %v", err)
	}
}

func TestReconnectManager_LatestSequence(t *testing.T) {
	resolver := NewPolicyResolver()
	sm := NewStreamManager(resolver)
	rm := NewReconnectManager(sm)

	input := PolicyInput{Kind: domain.ChannelKindEvent}
	runtimeID := domain.RuntimeInstanceID("rt-seq")
	serviceID := domain.ServiceID("svc-seq")
	channelID := domain.ChannelID("ch-seq")

	for i := 0; i < 5; i++ {
		err := sm.Publish(nil, input, runtimeID, serviceID, channelID, []byte{byte(i)})
		if err != nil {
			t.Fatalf("publish: %v", err)
		}
	}

	latest := rm.LatestSequence(runtimeID, serviceID, channelID)
	if latest != Sequence(5) {
		t.Errorf("expected latest 5, got %d", latest)
	}
}

func TestReconnectManager_StreamGeneration(t *testing.T) {
	resolver := NewPolicyResolver()
	sm := NewStreamManager(resolver)
	rm := NewReconnectManager(sm)

	input := PolicyInput{Kind: domain.ChannelKindEvent}
	runtimeID := domain.RuntimeInstanceID("rt-g")
	serviceID := domain.ServiceID("svc-g")
	channelID := domain.ChannelID("ch-g")

	err := sm.Publish(nil, input, runtimeID, serviceID, channelID, []byte("a"))
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	gen := rm.StreamGeneration(runtimeID, serviceID, channelID)
	if gen == StreamGenerationZero {
		t.Error("expected non-zero generation")
	}

	if gen != sm.GetGeneration(runtimeID, serviceID, channelID) {
		t.Error("mismatched generation")
	}
}
