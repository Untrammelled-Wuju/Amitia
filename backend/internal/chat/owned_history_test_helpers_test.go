package chat

import (
	"encoding/json"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func ownedMessageResource(ownerID, roleID, id string, body any) coordination.Resource {
	encoded, _ := json.Marshal(body)
	return coordination.Resource{
		OwnerID:  ownerID,
		Kind:     "message",
		ID:       id,
		RoleID:   roleID,
		Revision: 1,
		Body:     encoded,
	}
}

func ownedSummaryResource(ownerID, roleID, conversationID, text string) coordination.Resource {
	encoded, _ := json.Marshal(map[string]any{
		"id":             conversationID + "/summary",
		"conversationId": conversationID,
		"content":        map[string]string{"summary": text},
	})
	return coordination.Resource{
		OwnerID:  ownerID,
		Kind:     "summary",
		ID:       conversationID + "/summary",
		RoleID:   roleID,
		Revision: 1,
		Body:     encoded,
	}
}
