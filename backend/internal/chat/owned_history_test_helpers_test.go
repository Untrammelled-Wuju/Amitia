package chat

import (
	"encoding/json"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func legacyMessageResource(ownerID, roleID, id string, body any) coordination.Resource {
	encoded, _ := json.Marshal(body)
	return coordination.Resource{
		OwnerID:  ownerID,
		Kind:     "message",
		ID:       id,
		RoleID:   roleID,
		SourceID: "legacy",
		Revision: 0,
		Origin:   "legacy",
		ReadOnly: true,
		Body:     encoded,
	}
}

func legacySummaryResource(ownerID, roleID, conversationID, text string) coordination.Resource {
	encoded, _ := json.Marshal(map[string]any{
		"id":             conversationID + "/summary",
		"conversationId": conversationID,
		"origin":         "legacy",
		"readOnly":       true,
		"content":        map[string]string{"summary": text},
	})
	return coordination.Resource{
		OwnerID:  ownerID,
		Kind:     "summary",
		ID:       conversationID + "/summary",
		RoleID:   roleID,
		SourceID: "legacy",
		Revision: 0,
		Origin:   "legacy",
		ReadOnly: true,
		Body:     encoded,
	}
}
