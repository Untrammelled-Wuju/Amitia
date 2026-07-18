package emote

import (
	"fmt"
	"sort"
	"strings"

	"github.com/u-ai/backend/internal/embedding"
	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
)

type SemanticService struct {
	repo      *Repository
	embedding *embedding.Service
}

func NewSemanticService(repo *Repository) *SemanticService {
	return &SemanticService{repo: repo, embedding: embedding.NewService(repo.DB())}
}

func SemanticText(item *Emote) string {
	if item == nil {
		return ""
	}
	item.DecodeKeywords()
	return fmt.Sprintf("名称：%s\n含义：%s\n关键词：%s", strings.TrimSpace(item.Name), strings.TrimSpace(item.Meaning), strings.Join(item.KeywordList, "、"))
}

func (s *SemanticService) Sync(item *Emote) error {
	if item == nil {
		return nil
	}
	if item.Enabled != 1 || item.AIEnabled != 1 || strings.TrimSpace(item.Meaning) == "" || item.DeletedAt != nil {
		return s.Delete(item.ID)
	}
	if qdrantDB.Client == nil {
		return fmt.Errorf("qdrant_unavailable")
	}
	vector, err := s.embedding.Embed(SemanticText(item))
	if err != nil {
		return fmt.Errorf("embedding_failed: %w", err)
	}
	payload := map[string]interface{}{"emote_id": item.ID, "enabled": "true", "ai_enabled": "true", "role_scope": item.RoleScope, "semantic_version": "1"}
	if err := qdrantDB.UpsertVectors([]qdrantDB.VectorPoint{{ID: item.ID, Vector: vector, Payload: payload}}, CollectionName); err != nil {
		return fmt.Errorf("qdrant_unavailable: %w", err)
	}
	return nil
}

func (s *SemanticService) Delete(id string) error {
	if qdrantDB.Client == nil {
		return nil
	}
	return qdrantDB.DeleteVectors([]string{id}, CollectionName)
}

func (s *SemanticService) Search(query, characterID string, limit int) ([]DecisionCandidate, error) {
	if limit <= 0 {
		limit = 5
	}
	if qdrantDB.Client == nil {
		return s.searchText(query, characterID, limit), fmt.Errorf("qdrant_unavailable")
	}
	vector, err := s.embedding.Embed(query)
	if err != nil {
		return s.searchText(query, characterID, limit), fmt.Errorf("embedding_failed: %w", err)
	}
	points, err := qdrantDB.SearchVectors(vector, limit, map[string]interface{}{"enabled": "true", "ai_enabled": "true"}, CollectionName)
	if err != nil {
		return s.searchText(query, characterID, limit), fmt.Errorf("qdrant_unavailable: %w", err)
	}
	out := []DecisionCandidate{}
	for _, point := range points {
		value := point.Payload["emote_id"]
		if value == nil {
			continue
		}
		id := value.GetStringValue()
		item, getErr := s.repo.Get(id)
		if getErr != nil || !s.repo.CanCharacterUse(item, characterID) {
			continue
		}
		out = append(out, DecisionCandidate{Emote: *item, Score: float64(point.Score)})
	}
	return out, nil
}

func (s *SemanticService) searchText(query, characterID string, limit int) []DecisionCandidate {
	tokens := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	if len(tokens) == 0 {
		return []DecisionCandidate{}
	}
	items, _, err := s.repo.List("", "", "", 1, 200)
	if err != nil {
		return []DecisionCandidate{}
	}
	out := []DecisionCandidate{}
	for _, item := range items {
		copyItem := item
		if !s.repo.CanCharacterUse(&copyItem, characterID) {
			continue
		}
		haystack := strings.ToLower(SemanticText(&copyItem))
		matches := 0
		for _, token := range tokens {
			if len([]rune(token)) >= 2 && strings.Contains(haystack, token) {
				matches++
			}
		}
		if matches > 0 {
			out = append(out, DecisionCandidate{Emote: copyItem, Score: 0.36 + 0.1*float64(matches)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}
