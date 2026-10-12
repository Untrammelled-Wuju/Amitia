package interaction

import (
	"context"
	"testing"
)

type generatedConversationProcessor struct{}

func (generatedConversationProcessor) ProcessMessageCtx(context.Context, *ProcessRequest) (*ProcessResponse, error) {
	return &ProcessResponse{ConversationID: "generated-conversation", Reply: "saved reply"}, nil
}

func TestOrchestratorReturnsGeneratedConversationID(t *testing.T) {
	for _, conversationID := range []string{"", "requested-conversation"} {
		t.Run(conversationID, func(t *testing.T) {
			orchestrator := NewOrchestratorWithStores(DefaultOrchestratorConfig(), generatedConversationProcessor{}, NewInMemoryTracker(), nil)
			orchestrator.SetReady(true)
			result, err := orchestrator.Process(context.Background(), &ProcessRequest{SpaceID: "space", CharacterID: "character", ConversationID: conversationID, Channel: "web", RequestID: "request", Message: "hello"})
			if err != nil {
				t.Fatal(err)
			}
			expected := conversationID
			if expected == "" {
				expected = "generated-conversation"
			}
			if result.Response == nil || result.Response.ConversationID != expected {
				t.Fatalf("created conversation cannot be continued: %+v", result.Response)
			}
		})
	}
}
