package main

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/chat"
)

func TestCanonicalChatMemoryRuntimeInferenceContextWithoutAuthority(t *testing.T) {
	runtime := &canonicalChatMemoryRuntime{}
	ctx, finish, err := runtime.inferenceContext(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx == nil {
		t.Fatal("expected context")
	}
	finish()
}

func TestCanonicalChatMemoryRuntimeInferenceContextDelegates(t *testing.T) {
	rejected := errors.New("rejected")
	runtime := &canonicalChatMemoryRuntime{begin: func(context.Context) (context.Context, func(), error) {
		return nil, nil, rejected
	}}
	ctx, finish, err := runtime.inferenceContext(context.Background())
	if !errors.Is(err, rejected) {
		t.Fatalf("expected delegated error, got %v", err)
	}
	if ctx != nil || finish != nil {
		t.Fatal("expected nil context and finish")
	}
}

func TestCanonicalChatMemoryRuntimeInferenceAuthorityType(t *testing.T) {
	var begin chat.InferenceAuthority
	runtime := &canonicalChatMemoryRuntime{begin: begin}
	if _, _, err := runtime.inferenceContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
