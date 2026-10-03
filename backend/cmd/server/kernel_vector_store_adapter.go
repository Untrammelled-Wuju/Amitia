package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/qdrant/go-client/qdrant"
	"github.com/u-ai/backend/internal/embedding"
	"github.com/u-ai/backend/internal/extension/kernel"
	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
	"gorm.io/gorm"
)

type kernelVectorStoreAdapter struct {
	embedding *embedding.Service
}

func newKernelVectorStoreAdapter(db *gorm.DB) *kernelVectorStoreAdapter {
	return &kernelVectorStoreAdapter{embedding: embedding.NewService(db)}
}

func (a *kernelVectorStoreAdapter) Upsert(ctx context.Context, namespace, collection string, points []kernel.ExtensionVectorPoint) error {
	if a == nil || a.embedding == nil {
		return fmt.Errorf("embedding service is not configured")
	}
	if qdrantDB.Client == nil {
		return fmt.Errorf("qdrant is unavailable")
	}
	target := namespacedVectorCollection(namespace, collection)
	vectors := make([]qdrantDB.VectorPoint, 0, len(points))
	vectorDim := 0
	for index, point := range points {
		vector := point.Vector
		if len(vector) == 0 {
			embedded, err := a.embedding.Embed(point.Text)
			if err != nil {
				return fmt.Errorf("embed point %d: %w", index, err)
			}
			vector = embedded
		}
		if len(vector) == 0 {
			return fmt.Errorf("point %d vector is empty", index)
		}
		if vectorDim == 0 {
			vectorDim = len(vector)
		} else if len(vector) != vectorDim {
			return fmt.Errorf("point %d vector dimension mismatch", index)
		}
		vectors = append(vectors, qdrantDB.VectorPoint{ID: point.ID, Vector: vector, Payload: point.Payload})
	}
	if vectorDim == 0 {
		return fmt.Errorf("vector points are empty")
	}
	if err := qdrantDB.EnsureCollectionByName(target, vectorDim); err != nil {
		return err
	}
	return qdrantDB.UpsertVectors(vectors, target)
}

func (a *kernelVectorStoreAdapter) Search(ctx context.Context, namespace, collection, query string, vector []float32, limit int, filter map[string]string) ([]kernel.ExtensionVectorSearchResult, error) {
	if a == nil || a.embedding == nil {
		return nil, fmt.Errorf("embedding service is not configured")
	}
	if qdrantDB.Client == nil {
		return nil, fmt.Errorf("qdrant is unavailable")
	}
	if len(vector) == 0 {
		embedded, err := a.embedding.Embed(strings.TrimSpace(query))
		if err != nil {
			return nil, err
		}
		vector = embedded
	}
	target := namespacedVectorCollection(namespace, collection)
	searchFilter := make(map[string]interface{}, len(filter))
	for key, value := range filter {
		searchFilter[key] = value
	}
	points, err := qdrantDB.SearchVectors(vector, limit, searchFilter, target)
	if err != nil {
		return nil, err
	}
	results := make([]kernel.ExtensionVectorSearchResult, 0, len(points))
	for _, point := range points {
		id := point.Id.GetUuid()
		if id == "" {
			id = strconv.FormatUint(point.Id.GetNum(), 10)
		}
		payload := make(map[string]any, len(point.Payload))
		for key, value := range point.Payload {
			payload[key] = qdrantValueToAny(value)
		}
		results = append(results, kernel.ExtensionVectorSearchResult{
			ID:      id,
			Score:   float64(point.Score),
			Payload: payload,
		})
	}
	return results, nil
}

func qdrantValueToAny(value *qdrant.Value) any {
	if value == nil {
		return nil
	}
	switch kind := value.Kind.(type) {
	case *qdrant.Value_NullValue:
		return nil
	case *qdrant.Value_DoubleValue:
		return kind.DoubleValue
	case *qdrant.Value_IntegerValue:
		return kind.IntegerValue
	case *qdrant.Value_StringValue:
		return kind.StringValue
	case *qdrant.Value_BoolValue:
		return kind.BoolValue
	case *qdrant.Value_ListValue:
		items := make([]any, 0, len(kind.ListValue.GetValues()))
		for _, item := range kind.ListValue.GetValues() {
			items = append(items, qdrantValueToAny(item))
		}
		return items
	case *qdrant.Value_StructValue:
		items := make(map[string]any, len(kind.StructValue.GetFields()))
		for key, item := range kind.StructValue.GetFields() {
			items[key] = qdrantValueToAny(item)
		}
		return items
	default:
		return nil
	}
}

func (a *kernelVectorStoreAdapter) Delete(ctx context.Context, namespace, collection string, ids []string) error {
	if qdrantDB.Client == nil {
		return fmt.Errorf("qdrant is unavailable")
	}
	return qdrantDB.DeleteVectors(ids, namespacedVectorCollection(namespace, collection))
}

func namespacedVectorCollection(namespace, collection string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(namespace)))
	return "amitia_ext_" + hex.EncodeToString(sum[:8]) + "_" + strings.TrimSpace(collection)
}
