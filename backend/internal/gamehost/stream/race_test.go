package stream

import (
	"context"
	"sync"
	"testing"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func TestRace_ConcurrentPublish(t *testing.T) {
	resolver := NewPolicyResolver()
	sm := NewStreamManager(resolver)

	input := PolicyInput{Kind: domain.ChannelKindEvent}
	runtimeID := domain.RuntimeInstanceID("rt-race")
	serviceID := domain.ServiceID("svc-race")
	channelID := domain.ChannelID("ch-race")

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			sm.Publish(context.Background(), input, runtimeID, serviceID, channelID, []byte{byte(n)})
		}(i)
	}
	wg.Wait()

	seq := sm.GetSequence(runtimeID, serviceID, channelID)
	if seq != Sequence(100) {
		t.Errorf("expected seq 100 after concurrent publish, got %d", seq)
	}
}

func TestRace_PublishAndRemove(t *testing.T) {
	resolver := NewPolicyResolver()
	sm := NewStreamManager(resolver)

	input := PolicyInput{Kind: domain.ChannelKindEvent}
	runtimeID := domain.RuntimeInstanceID("rt-pr")
	serviceID := domain.ServiceID("svc-pr")
	channelID := domain.ChannelID("ch-pr")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			sm.Publish(context.Background(), input, runtimeID, serviceID, channelID, []byte{byte(i)})
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		sm.RemoveStream(runtimeID, serviceID, channelID)
	}()

	wg.Wait()
}

func TestRace_ReconnectWhilePublishing(t *testing.T) {
	resolver := NewPolicyResolver()
	sm := NewStreamManager(resolver)
	rm := NewReconnectManager(sm)

	input := PolicyInput{Kind: domain.ChannelKindEvent}
	runtimeID := domain.RuntimeInstanceID("rt-rc")
	serviceID := domain.ServiceID("svc-rc")
	channelID := domain.ChannelID("ch-rc")

	for i := 0; i < 50; i++ {
		err := sm.Publish(nil, input, runtimeID, serviceID, channelID, []byte{byte(i)})
		if err != nil {
			t.Fatalf("initial publish: %v", err)
		}
	}

	gen := sm.GetGeneration(runtimeID, serviceID, channelID)
	cursor := NewCursor(runtimeID, serviceID, channelID, gen, 30)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 50; i < 100; i++ {
			sm.Publish(nil, input, runtimeID, serviceID, channelID, []byte{byte(i)})
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		rm.Resume(context.Background(), runtimeID, serviceID, channelID, cursor)
	}()

	wg.Wait()
}

func TestRace_MultiRuntimeIsolation(t *testing.T) {
	resolver := NewPolicyResolver()
	sm := NewStreamManager(resolver)

	input := PolicyInput{Kind: domain.ChannelKindEvent}
	serviceID := domain.ServiceID("svc-iso")
	channelID := domain.ChannelID("ch-iso")

	var wg sync.WaitGroup
	for r := 0; r < 5; r++ {
		wg.Add(1)
		go func(rt int) {
			defer wg.Done()
			rtID := domain.RuntimeInstanceID(domain.RuntimeInstanceID("rt-iso-" + string(rune('0'+rt))))
			for i := 0; i < 20; i++ {
				sm.Publish(nil, input, rtID, serviceID, channelID, []byte{byte(i)})
			}
		}(r)
	}
	wg.Wait()

	for r := 0; r < 5; r++ {
		rtID := domain.RuntimeInstanceID("rt-iso-" + string(rune('0'+r)))
		seq := sm.GetSequence(rtID, serviceID, channelID)
		if seq != Sequence(20) {
			t.Errorf("runtime %d: expected seq 20, got %d", r, seq)
		}
	}
}

func TestRace_ShutdownWhilePublishing(t *testing.T) {
	resolver := NewPolicyResolver()
	sm := NewStreamManager(resolver)

	input := PolicyInput{Kind: domain.ChannelKindEvent}
	runtimeID := domain.RuntimeInstanceID("rt-shutdown")
	serviceID := domain.ServiceID("svc-shutdown")
	channelID := domain.ChannelID("ch-shutdown")

	for i := 0; i < 10; i++ {
		err := sm.Publish(nil, input, runtimeID, serviceID, channelID, []byte{byte(i)})
		if err != nil {
			t.Fatalf("initial publish: %v", err)
		}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		sm.Shutdown(context.Background())
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 10; i < 20; i++ {
			sm.Publish(nil, input, runtimeID, serviceID, channelID, []byte{byte(i)})
		}
	}()

	wg.Wait()
}

func TestRace_RemoveByRuntime(t *testing.T) {
	resolver := NewPolicyResolver()
	sm := NewStreamManager(resolver)

	input := PolicyInput{Kind: domain.ChannelKindEvent}
	runtimeID := domain.RuntimeInstanceID("rt-rmall")
	serviceID := domain.ServiceID("svc-rmall")
	channelID := domain.ChannelID("ch-rmall")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			sm.Publish(nil, input, runtimeID, serviceID, channelID, []byte{byte(i)})
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		sm.RemoveByRuntime(context.Background(), runtimeID)
	}()

	wg.Wait()
}
