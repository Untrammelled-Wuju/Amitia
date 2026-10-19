package business

import (
	"encoding/json"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func nextOwnedMessageSequence(snapshot coordination.DataSnapshot, conversation string) int64 {
	var latest int64
	read := func(encoded json.RawMessage, isConversation bool) {
		var row struct {
			ID                  string `json:"id"`
			ConversationID      string `json:"conversationId"`
			Sequence            int64  `json:"sequence"`
			LastMessageSequence int64  `json:"lastMessageSequence"`
		}
		if json.Unmarshal(encoded, &row) != nil {
			return
		}
		if isConversation && (row.ID == conversation || row.ConversationID == conversation) {
			latest = max(latest, row.LastMessageSequence)
		}
		if !isConversation && row.ConversationID == conversation {
			latest = max(latest, row.Sequence)
		}
	}
	for _, resource := range snapshot.Resources {
		if !resource.Deleted && (resource.Kind == "message" || resource.Kind == "conversation") {
			read(resource.Body, resource.Kind == "conversation")
		}
	}
	for _, raw := range snapshot.LegacyMessages {
		read(raw, false)
	}
	return latest + 1
}
