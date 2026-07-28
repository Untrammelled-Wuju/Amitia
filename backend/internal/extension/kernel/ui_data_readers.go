package kernel

import (
	"context"
	"encoding/json"
	"log"
)

type DefaultCharacterReader struct{}

func NewDefaultCharacterReader() *DefaultCharacterReader {
	return &DefaultCharacterReader{}
}

func (r *DefaultCharacterReader) ReadCharacter(ctx context.Context, characterID string) (json.RawMessage, bool, error) {
	log.Printf("[CharacterReader] characterID=%s", characterID)
	data, _ := json.Marshal(map[string]any{
		"characterId": characterID,
		"available":   false,
	})
	return data, false, nil
}

type DefaultConversationReader struct{}

func NewDefaultConversationReader() *DefaultConversationReader {
	return &DefaultConversationReader{}
}

func (r *DefaultConversationReader) ReadConversation(ctx context.Context, conversationID string, limit int, offset int) ([]json.RawMessage, bool, error) {
	log.Printf("[ConversationReader] conversationID=%s limit=%d offset=%d", conversationID, limit, offset)
	return []json.RawMessage{}, false, nil
}

type DefaultMemoryQueryService struct{}

func NewDefaultMemoryQueryService() *DefaultMemoryQueryService {
	return &DefaultMemoryQueryService{}
}

func (s *DefaultMemoryQueryService) Query(ctx context.Context, extensionID string, query string, limit int) ([]json.RawMessage, error) {
	log.Printf("[MemoryQueryService] ext=%s query=%s limit=%d", extensionID, query, limit)
	return []json.RawMessage{}, nil
}
