package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
)

func (p *meshLocalDataPort) semanticSearch(ctx context.Context, role string, query coordination.DataQuery) ([]coordination.Resource, error) {
	fallback := func() ([]coordination.Resource, error) { return p.store.SemanticSearch(ctx, role, query, 32) }
	if err := coordination.ValidateQueryVector(query); err != nil {
		return nil, err
	}
	if qdrantDB.Client == nil || len(query.VectorModel) != 64 || len(query.Vector) == 0 {
		return fallback()
	}
	if _, err := hex.DecodeString(query.VectorModel); err != nil {
		return fallback()
	}
	ready, err := p.store.ProjectionsReady(ctx, role, "vector")
	if err != nil {
		return nil, err
	}
	if !ready {
		return fallback()
	}
	searchCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	rows, err := qdrantDB.Client.Query(searchCtx, &qdrant.QueryPoints{
		CollectionName: fmt.Sprintf("amitia_owned_v1_%d_%s", len(query.Vector), query.VectorModel[:16]),
		Query:          qdrant.NewQuery(query.Vector...), Limit: qdrant.PtrOf(uint64(96)),
		WithPayload: qdrant.NewWithPayload(true),
		Filter: &qdrant.Filter{Must: []*qdrant.Condition{
			qdrant.NewMatchKeyword("ownerId", p.ownerID), qdrant.NewMatchKeyword("roleId", role), qdrant.NewMatchKeyword("modelFingerprint", query.VectorModel),
		}},
	})
	if err != nil {
		return fallback()
	}
	resources := make([]coordination.Resource, 0, 64)
	seen := map[string]bool{}
	for _, row := range rows {
		if row.Score <= 0 {
			continue
		}
		id := row.Payload["resourceId"].GetStringValue()
		vector, err := p.store.Get(ctx, "vector", id)
		if err != nil {
			return nil, err
		}
		if vector == nil || vector.RoleID != role || vector.Deleted || vector.Revision != row.Payload["revision"].GetIntegerValue() || projectionPointID(*vector) != row.Id.GetUuid() {
			return fallback()
		}
		source, err := p.store.Get(ctx, "memory", vector.SourceID)
		if err != nil {
			return nil, err
		}
		if source == nil || source.RoleID != role || source.Deleted || source.Revision != row.Payload["sourceRevision"].GetIntegerValue() {
			return fallback()
		}
		if !coordination.ProjectionUsable(coordination.ProjectionJob{Resource: *vector, Source: source}, time.Now()) {
			continue
		}
		var document struct {
			Content struct {
				Model  string    `json:"modelFingerprint"`
				Values []float32 `json:"values"`
			} `json:"content"`
		}
		if json.Unmarshal(vector.Body, &document) != nil || document.Content.Model != query.VectorModel || len(document.Content.Values) != len(query.Vector) {
			return fallback()
		}
		if seen[source.ID] {
			continue
		}
		seen[source.ID] = true
		resources = append(resources, *source, *vector)
		if len(seen) == 32 {
			break
		}
	}
	ready, err = p.store.ProjectionsReady(ctx, role, "vector")
	if err != nil {
		return nil, err
	}
	if !ready || len(rows) == 96 && len(seen) < 32 {
		return fallback()
	}
	return resources, nil
}
