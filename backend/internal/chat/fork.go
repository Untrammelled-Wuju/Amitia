package chat

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type ForkConversationRequest struct {
	SourceConversationID string
	BoundarySequence     int64
	Title                string
}

func (s *service) ForkConversation(req ForkConversationRequest) (*Conversation, error) {
	sourceID := strings.TrimSpace(req.SourceConversationID)
	if sourceID == "" {
		return nil, fmt.Errorf("fork conversation requires source conversation id")
	}
	source, err := s.repo.GetConversation(sourceID)
	if err != nil {
		return nil, err
	}
	messages, err := s.repo.GetAllMessagesByConv(source.ID)
	if err != nil {
		return nil, err
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = fmt.Sprintf("%s (fork)", source.Title)
	}
	fork := *source
	fork.ID = uuid.NewString()
	fork.Title = title
	fork.MessageCount = 0
	if err := s.repo.CreateConversation(&fork); err != nil {
		return nil, err
	}
	copied := 0
	for index := range messages {
		message := messages[index]
		if req.BoundarySequence > 0 && message.Sequence > req.BoundarySequence {
			break
		}
		message.ID = uuid.NewString()
		message.ConversationID = fork.ID
		if err := s.repo.CreateMessage(&message); err != nil {
			return nil, err
		}
		copied++
	}
	if err := s.repo.UpdateConversation(fork.ID, map[string]interface{}{"message_count": copied}); err != nil {
		return nil, err
	}
	fork.MessageCount = copied
	return &fork, nil
}
