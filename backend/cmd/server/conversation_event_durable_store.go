package main

import (
	"context"
	"encoding/json"

	"github.com/u-ai/backend/internal/conversationstream"
	"github.com/u-ai/backend/internal/extension/kernel/event"
)

type conversationEventDurableStore struct {
	service *event.Service
}

var conversationEventFileStore *conversationstream.FileStore

func (s conversationEventDurableStore) Persist(ctx context.Context, value conversationstream.AgentUIEvent) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, _, err = s.service.PublishConversationUIEvent(ctx, value.ConversationID, payload, "agent-ui:"+value.EventID)
	return err
}

func (s conversationEventDurableStore) ListAfter(ctx context.Context, conversationID string, afterSequence int64, limit int) ([]conversationstream.AgentUIEvent, error) {
	records, err := s.service.ListAgentUIEventsAfterSequence(ctx, conversationID, afterSequence, limit)
	if err != nil {
		return nil, err
	}
	result := make([]conversationstream.AgentUIEvent, 0, len(records))
	for _, record := range records {
		var value conversationstream.AgentUIEvent
		if err := json.Unmarshal(record.Payload, &value); err != nil || value.Version != conversationstream.ProtocolVersion {
			continue
		}
		result = append(result, value)
	}
	return result, nil
}

func (s conversationEventDurableStore) LatestSequence(ctx context.Context, conversationID string) (int64, error) {
	return s.service.LatestAgentUISequence(ctx, conversationID)
}

func wireConversationEventDurableStore(service *event.Service) {
	if conversationEventFileStore != nil {
		conversationstream.DefaultManager().SetDurableStore(conversationEventFileStore)
		return
	}
	if service == nil {
		conversationstream.DefaultManager().SetDurableStore(nil)
		return
	}
	conversationstream.DefaultManager().SetDurableStore(conversationEventDurableStore{service: service})
}
