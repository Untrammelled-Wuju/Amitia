package qdrant

import (
	"context"
	"fmt"

	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
	"github.com/u-ai/backend/log"
)

type QdrantFilter struct {
	UserID      string
	CharacterID string
	ScopeType   string
	MemoryKind  string
	Status      string
	Revision    int
}

type FilterBuilder struct {
	CharacterID string
	ScopeType   string
	MemoryKind  string
	Status      string
	UserID      string
}
func (fb FilterBuilder) Validate() error {
	if fb.CharacterID == "" {
		return fmt.Errorf("FilterBuilder: CharacterID cannot be empty")
	}
	return nil
}

func (fb FilterBuilder) Build() QdrantFilter {
	return QdrantFilter{
		UserID:      fb.UserID,
		CharacterID: fb.CharacterID,
		ScopeType:   fb.ScopeType,
		MemoryKind:  fb.MemoryKind,
		Status:      fb.Status,
		Revision:    0,
	}
}

type QdrantClient struct{
}

func NewQdrantClient() *QdrantClient {
	return &QdrantClient{}
}

func (c *QdrantClient) Name() string {
	return "vector sync"
}

func (c *QdrantClient) Process(ctx context.Context, convID string, messages []map[string]string, newReply string) error {
	return nil
}

func (c *QdrantClient) SearchWithFilter(
	ctx context.Context,
	collection string,
	vector []float32,
	filter QdrantFilter,
	limit int,
) ([]qdrantDB.CollectionScoredPoint, error) {
	filterMap := make(map[string]interface{})

	if filter.CharacterID != "" {
		filterMap["character_id"] = filter.CharacterID
	}
	if filter.UserID != "" {
		filterMap["user_id"] = filter.UserID
	}
	if filter.ScopeType != "" {
		filterMap["scope_type"] = filter.ScopeType
	}
	if filter.MemoryKind != "" {
		filterMap["memory_kind"] = filter.MemoryKind
	}
	if filter.Status != "" {
		filterMap["status"] = filter.Status
	}

	collectionName := collection
	if collectionName == "" {
		collectionName = "memory_embeddings"
	}

	if limit <= 0 {
		limit = 10
	}

	results, err := qdrantDB.MultiSearch(vector, limit, filterMap, collectionName)
	if err != nil {
		log.Error("SearchWithFilter search failed",
			"collection", collectionName,
			"characterID", filter.CharacterID,
			"scopeType", filter.ScopeType,
			"error", err.Error(),
		)
		return nil, fmt.Errorf("SearchWithFilter: %w", err)
	}

	log.Info("SearchWithFilter completed",
		"collection", collectionName,
		"characterID", filter.CharacterID,
		"scopeType", filter.ScopeType,
		"results", len(results),
	)

	return results, nil
}
