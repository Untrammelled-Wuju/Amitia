// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"github.com/u-ai/backend/internal/interaction"
)

func convertProcessMessageResponse(resp *ProcessMessageResponse) *interaction.ProcessResponse {
	if resp == nil {
		return nil
	}
	lines := DeduplicateAdjacentLines(resp.Lines)
	return &interaction.ProcessResponse{
		ConversationID: resp.ConversationID,
		Sequence:       resp.Sequence,
		Reply:          resp.Reply,
		Lines:          lines,
		CharacterID:    resp.CharacterID,
		CharacterName:  resp.CharacterName,
		MessageIDs:     resp.MessageIDs,
		ForceVoice:     resp.ForceVoice,
		AudioUrls:      resp.AudioUrls,
		RequestID:      resp.RequestID,
		MessagePlan:    resp.MessagePlan,
		Events:         resp.Events,
	}
}
