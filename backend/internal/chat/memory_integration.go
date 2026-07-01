// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

func (s *service) detectEpisodicMoment(convID, charID string) {
	if s.episodicSvc == nil {
		return
	}
	messages := s.loadHistory(convID)
	if len(messages) == 0 {
		return
	}
	s.episodicSvc.ExtractFromConversation(charID, convID, messages)
}

func (s *service) extractProfile(convID, charID string) {
	if s.profileSvc == nil {
		return
	}
	messages := s.loadHistory(convID)
	if len(messages) == 0 {
		return
	}
	s.profileSvc.ExtractFromConversation("default", convID, messages, charID)
}

func (s *service) autoExtractMemories(convID, charID string) {
	if s.memorySvc == nil {
		return
	}
	candidates, err := s.memorySvc.GenerateCandidates(convID)
	if err != nil || len(candidates) == 0 {
		return
	}

	existingKeys := map[string]bool{}
	var existingMemories []struct {
		Key   string
		Value string
	}
	s.db.Table("memories").Select("key, value").Find(&existingMemories)
	for _, m := range existingMemories {
		existingKeys[m.Key+"|"+m.Value] = true
	}

	for _, c := range candidates {
		if c.Importance < 7 {
			continue
		}
		if existingKeys[c.Key+"|"+c.Value] {
			continue
		}
		existingKeys[c.Key+"|"+c.Value] = true
		s.memorySvc.AcceptCandidate(c.ID)
	}
}
