package jsonrpc

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestB20WaitCreditBlocksWithoutCredit(t *testing.T) {
	s := NewStream("wait-1", "test", "duplex", 1024, 0, 8)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- s.WaitCredit(ctx)
	}()

	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected deadline exceeded, got %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("WaitCredit should have returned")
	}
}

func TestB20WaitCreditResumesOnAddCredit(t *testing.T) {
	s := NewStream("wait-2", "test", "duplex", 1024, 0, 8)

	done := make(chan error, 1)
	go func() {
		done <- s.WaitCredit(context.Background())
	}()

	time.Sleep(20 * time.Millisecond)
	s.AddCredit(1)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("WaitCredit should succeed after AddCredit, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("WaitCredit should have resumed after AddCredit")
	}
}

func TestB20WaitCreditStopsOnContextCancel(t *testing.T) {
	s := NewStream("wait-3", "test", "duplex", 1024, 0, 8)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- s.WaitCredit(ctx)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("WaitCredit should have returned on cancel")
	}
}

func TestB20WaitCreditStopsOnStreamClose(t *testing.T) {
	s := NewStream("wait-4", "test", "duplex", 1024, 0, 8)

	done := make(chan error, 1)
	go func() {
		done <- s.WaitCredit(context.Background())
	}()

	time.Sleep(20 * time.Millisecond)
	s.Close("test")

	select {
	case err := <-done:
		if !errors.Is(err, ErrStreamClosed) {
			t.Fatalf("expected ErrStreamClosed, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("WaitCredit should have returned on close")
	}
}

func TestB20AcceptChunkSequenceValid(t *testing.T) {
	s := NewStream("seq-1", "test", "inbound", 1024, 0, 8)

	err := s.AcceptChunk(StreamChunk{StreamID: "seq-1", Sequence: 1, Data: []byte("chunk1")})
	if err != nil {
		t.Fatalf("accept chunk 1: %v", err)
	}

	err = s.AcceptChunk(StreamChunk{StreamID: "seq-1", Sequence: 2, Data: []byte("chunk2")})
	if err != nil {
		t.Fatalf("accept chunk 2: %v", err)
	}

	err = s.AcceptChunk(StreamChunk{StreamID: "seq-1", Sequence: 3, Data: []byte("chunk3"), Last: true})
	if err != nil {
		t.Fatalf("accept chunk 3: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	for i := 0; i < 3; i++ {
		_, err := s.RecvChunk(ctx)
		if err != nil {
			t.Fatalf("recv chunk %d: %v", i, err)
		}
	}
}

func TestB20AcceptChunkRejectsDuplicateSequence(t *testing.T) {
	s := NewStream("seq-2", "test", "inbound", 1024, 0, 8)

	err := s.AcceptChunk(StreamChunk{StreamID: "seq-2", Sequence: 1, Data: []byte("chunk1")})
	if err != nil {
		t.Fatalf("accept chunk 1: %v", err)
	}

	err = s.AcceptChunk(StreamChunk{StreamID: "seq-2", Sequence: 1, Data: []byte("duplicate")})
	if err == nil {
		t.Fatal("expected error for duplicate sequence")
	}
}

func TestB20AcceptChunkRejectsSkipSequence(t *testing.T) {
	s := NewStream("seq-3", "test", "inbound", 1024, 0, 8)

	err := s.AcceptChunk(StreamChunk{StreamID: "seq-3", Sequence: 1, Data: []byte("chunk1")})
	if err != nil {
		t.Fatalf("accept chunk 1: %v", err)
	}

	err = s.AcceptChunk(StreamChunk{StreamID: "seq-3", Sequence: 3, Data: []byte("skip")})
	if err == nil {
		t.Fatal("expected error for skipped sequence")
	}
}

func TestB20AcceptChunkLastDoesNotLoseData(t *testing.T) {
	s := NewStream("seq-4", "test", "inbound", 1024, 0, 8)

	err := s.AcceptChunk(StreamChunk{StreamID: "seq-4", Sequence: 1, Data: []byte("first")})
	if err != nil {
		t.Fatalf("accept first: %v", err)
	}

	err = s.AcceptChunk(StreamChunk{StreamID: "seq-4", Sequence: 2, Data: []byte("last"), Last: true})
	if err != nil {
		t.Fatalf("accept last: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	data, err := s.RecvChunk(ctx)
	if err != nil {
		t.Fatalf("recv first: %v", err)
	}
	if string(data) != "first" {
		t.Fatalf("expected 'first', got %s", data)
	}

	data, err = s.RecvChunk(ctx)
	if err != nil {
		t.Fatalf("recv last: %v", err)
	}
	if string(data) != "last" {
		t.Fatalf("expected 'last', got %s", data)
	}
}

func TestB20WaitCreditNoGoroutineLeak(t *testing.T) {
	s := NewStream("leak-1", "test", "duplex", 1024, 0, 8)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.WaitCredit(context.Background())
		}()
	}

	time.Sleep(20 * time.Millisecond)
	s.Close("test")
	wg.Wait()
}
