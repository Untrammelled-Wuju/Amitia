// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/surrealdb/surrealdb.go"
	"github.com/u-ai/backend/config"
)

type Service interface {
	SyncNode(entityType, entityID, label string, properties map[string]interface{}) error
	SyncEdge(sourceID, targetID, relationType string, weight float64) error
	DeleteNode(entityID string) error
	DeleteNodeIfOrphan(entityID string) error
	DeleteNodesByProperty(entityType, propertyKey, propertyValue string) error
	QueryNeighbors(entityID string, depth int, userID string) (map[string]interface{}, error)
	FindPaths(sourceID, targetID string, maxDepth int) ([]map[string]interface{}, error)
	DeleteOrphanNodes() error
	GetStats(userID string) (map[string]interface{}, error)
	GetAllNodes(userID string) ([]map[string]interface{}, error)
	GetAllEdges(userID string) ([]map[string]interface{}, error)
	Name() string
	Process(ctx context.Context, convID string, messages []map[string]string, newReply string) error
}

type service struct {
	client *Client
}

func NewService(client *Client) Service {
	return &service{client: client}
}

func (s *service) Name() string { return "图谱关系" }

func (s *service) Process(ctx context.Context, convID string, messages []map[string]string, newReply string) error {
	return nil
}

func (s *service) SyncNode(entityType, entityID, label string, properties map[string]interface{}) error {
	if s.client == nil || s.client.DB() == nil {
		return nil
	}
	id := sanitizeRecordID(fmt.Sprintf("%s:%s", entityType, entityID))
	content := map[string]interface{}{
		"entity_type": entityType,
		"label":       label,
		"properties":  properties,
	}
	body, _ := json.Marshal(content)
	query := fmt.Sprintf(
		"UPSERT entity_node:`%s` CONTENT %s",
		id, string(body),
	)
	_, err := surrealdb.Query[any](context.Background(), s.client.DB(), query, nil)
	return err
}

func (s *service) SyncEdge(sourceID, targetID, relationType string, weight float64) error {
	if s.client == nil || s.client.DB() == nil {
		return nil
	}
	relation, _ := json.Marshal(relationType)
	query := fmt.Sprintf(
		"RELATE entity_node:`%s`->entity_edge->entity_node:`%s` SET relation_type=%s, weight=%f",
		sanitizeRecordID(sourceID), sanitizeRecordID(targetID), string(relation), weight,
	)
	_, err := surrealdb.Query[any](context.Background(), s.client.DB(), query, nil)
	return err
}

func (s *service) DeleteNode(entityID string) error {
	if s.client == nil || s.client.DB() == nil {
		return nil
	}
	id := sanitizeRecordID(entityID)
	queries := []string{
		fmt.Sprintf("DELETE entity_edge WHERE in = entity_node:`%s` OR out = entity_node:`%s`", id, id),
		fmt.Sprintf("DELETE entity_node:`%s`", id),
	}
	for _, query := range queries {
		if _, err := surrealdb.Query[any](context.Background(), s.client.DB(), query, nil); err != nil {
			return err
		}
	}
	return nil
}

func (s *service) DeleteNodeIfOrphan(entityID string) error {
	if s.client == nil || s.client.DB() == nil {
		return nil
	}
	id := sanitizeRecordID(entityID)
	query := fmt.Sprintf("DELETE entity_node:`%s` WHERE count(<-entity_edge) = 0 AND count(->entity_edge) = 0", id)
	_, err := surrealdb.Query[any](context.Background(), s.client.DB(), query, nil)
	return err
}

func (s *service) DeleteNodesByProperty(entityType, propertyKey, propertyValue string) error {
	if s.client == nil || s.client.DB() == nil {
		return nil
	}
	typ, _ := json.Marshal(entityType)
	key := strings.ReplaceAll(propertyKey, "`", "")
	value, _ := json.Marshal(propertyValue)
	queries := []string{
		fmt.Sprintf("DELETE entity_edge WHERE in IN (SELECT VALUE id FROM entity_node WHERE entity_type = %s AND properties.%s = %s) OR out IN (SELECT VALUE id FROM entity_node WHERE entity_type = %s AND properties.%s = %s)", string(typ), key, string(value), string(typ), key, string(value)),
		fmt.Sprintf("DELETE entity_node WHERE entity_type = %s AND properties.%s = %s", string(typ), key, string(value)),
	}
	for _, query := range queries {
		if _, err := surrealdb.Query[any](context.Background(), s.client.DB(), query, nil); err != nil {
			return err
		}
	}
	return nil
}

func (s *service) QueryNeighbors(entityID string, depth int, userID string) (map[string]interface{}, error) {
	types := []string{"memory:", "profile:", "episodic:", "worldbook:"}

	queryWithFilter := func(id string) (map[string]interface{}, error) {
		userFilter := ""
		if userID != "" {
			userFilter = fmt.Sprintf(" WHERE properties.user_id = \"%s\"", userID)
		}
		query := fmt.Sprintf("SELECT ->entity_edge->entity_node AS neighbors FROM entity_node:`%s`%s LIMIT 100", id, userFilter)
		results, err := surrealdb.Query[any](context.Background(), s.client.DB(), query, nil)
		if err != nil {
			return nil, err
		}
		return s.toMap(results), nil
	}

	result, err := queryWithFilter(entityID)
	if err != nil {
		return nil, err
	}
	if arr, ok := result["result"].([]interface{}); ok && len(arr) > 0 {
		return result, nil
	}

	for _, t := range types {
		result, err = queryWithFilter(t + entityID)
		if err != nil {
			return nil, err
		}
		if arr, ok := result["result"].([]interface{}); ok && len(arr) > 0 {
			return result, nil
		}
	}

	return result, nil
}

func (s *service) FindPaths(sourceID, targetID string, maxDepth int) ([]map[string]interface{}, error) {
	query := fmt.Sprintf(
		"SELECT id, in, out, relation_type, weight FROM entity_edge WHERE in = entity_node:`%s` AND out = entity_node:`%s` LIMIT 20",
		sanitizeRecordID(sourceID), sanitizeRecordID(targetID),
	)
	results, err := surrealdb.Query[any](context.Background(), s.client.DB(), query, nil)
	if err != nil {
		return nil, err
	}
	raw := s.toMap(results)
	if arr, ok := raw["result"].([]interface{}); ok {
		paths := make([]map[string]interface{}, 0)
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				paths = append(paths, m)
			}
		}
		return paths, nil
	}
	return nil, nil
}

func (s *service) DeleteOrphanNodes() error {
	_, err := surrealdb.Query[any](context.Background(), s.client.DB(),
		"DELETE entity_node WHERE count(<-entity_edge) = 0 AND count(->entity_edge) = 0", nil)
	return err
}

func (s *service) GetStats(userID string) (map[string]interface{}, error) {
	nodeCount := 0
	edgeCount := 0
	var byType []map[string]interface{}

	nodeFilter := ""
	if userID != "" {
		nodeFilter = fmt.Sprintf(" WHERE properties.user_id = \"%s\"", userID)
	}

	nodeResult, err := surrealdb.Query[any](context.Background(), s.client.DB(),
		fmt.Sprintf("SELECT count() FROM entity_node%s GROUP ALL", nodeFilter), nil)
	if err == nil && nodeResult != nil && len(*nodeResult) > 0 {
		raw := (*nodeResult)[0].Result
		if arr, ok := raw.([]interface{}); ok && len(arr) > 0 {
			if m, ok := arr[0].(map[string]interface{}); ok {
				if c, ok := m["count"]; ok {
					switch v := c.(type) {
					case float64:
						nodeCount = int(v)
					case int:
						nodeCount = v
					case int64:
						nodeCount = int(v)
					case uint64:
						nodeCount = int(v)
					}
				}
			}
		}
	}

	edgeResult, err := surrealdb.Query[any](context.Background(), s.client.DB(),
		"SELECT count() FROM entity_edge GROUP ALL", nil)
	if err == nil && edgeResult != nil && len(*edgeResult) > 0 {
		raw := (*edgeResult)[0].Result
		if arr, ok := raw.([]interface{}); ok && len(arr) > 0 {
			if m, ok := arr[0].(map[string]interface{}); ok {
				if c, ok := m["count"]; ok {
					switch v := c.(type) {
					case float64:
						edgeCount = int(v)
					case int:
						edgeCount = v
					case int64:
						edgeCount = int(v)
					case uint64:
						edgeCount = int(v)
					}
				}
			}
		}
	}

	typeResult, err := surrealdb.Query[any](context.Background(), s.client.DB(),
		fmt.Sprintf("SELECT entity_type, count() FROM entity_node%s GROUP BY entity_type", nodeFilter), nil)
	if err == nil && typeResult != nil && len(*typeResult) > 0 {
		raw := (*typeResult)[0].Result
		if arr, ok := raw.([]interface{}); ok {
			byType = make([]map[string]interface{}, 0, len(arr))
			for _, item := range arr {
				if m, ok := item.(map[string]interface{}); ok {
					byType = append(byType, m)
				}
			}
		}
	}

	for _, t := range byType {
		if c, ok := t["count"]; ok {
			switch v := c.(type) {
			case float64:
				nodeCount += int(v)
			case int:
				nodeCount += v
			case int64:
				nodeCount += int(v)
			case uint64:
				nodeCount += int(v)
			case json.Number:
				if n, e := v.Int64(); e == nil {
					nodeCount += int(n)
				}
			}
		}
	}

	return map[string]interface{}{
		"nodeCount": nodeCount,
		"edgeCount": edgeCount,
		"byType":    byType,
	}, nil
}

func (s *service) GetAllNodes(userID string) ([]map[string]interface{}, error) {
	if s.client == nil || s.client.DB() == nil {
		return nil, nil
	}
	nodeFilter := ""
	if userID != "" {
		nodeFilter = fmt.Sprintf(" WHERE properties.user_id = \"%s\"", userID)
	}
	query := fmt.Sprintf("SELECT id, entity_type, label, properties FROM entity_node%s", nodeFilter)
	results, err := surrealdb.Query[any](context.Background(), s.client.DB(), query, nil)
	if err != nil {
		return nil, err
	}
	if results == nil || len(*results) == 0 {
		return nil, nil
	}
	raw := (*results)[0].Result
	if arr, ok := raw.([]interface{}); ok {
		nodes := make([]map[string]interface{}, 0, len(arr))
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				nodes = append(nodes, m)
			}
		}
		return nodes, nil
	}
	return nil, nil
}

func (s *service) GetAllEdges(userID string) ([]map[string]interface{}, error) {
	if s.client == nil || s.client.DB() == nil {
		return nil, nil
	}
	query := "SELECT id, in, out, relation_type, weight FROM entity_edge"
	results, err := surrealdb.Query[any](context.Background(), s.client.DB(), query, nil)
	if err != nil {
		return nil, err
	}
	if results == nil || len(*results) == 0 {
		return nil, nil
	}
	raw := (*results)[0].Result
	if arr, ok := raw.([]interface{}); ok {
		edges := make([]map[string]interface{}, 0, len(arr))
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				edges = append(edges, m)
			}
		}
		return edges, nil
	}
	return nil, nil
}

func (s *service) toMap(results *[]surrealdb.QueryResult[any]) map[string]interface{} {
	if results == nil || len(*results) == 0 {
		return map[string]interface{}{}
	}
	m := map[string]interface{}{}
	for i, r := range *results {
		m[fmt.Sprintf("result")] = r.Result
		_ = i
	}
	return m
}

func extractCount(results *[]surrealdb.QueryResult[any]) int {
	if results == nil || len(*results) == 0 {
		return 0
	}
	result := (*results)[0].Result
	if arr, ok := result.([]interface{}); ok && len(arr) > 0 {
		if m, ok := arr[0].(map[string]interface{}); ok {
			if c, ok := m["count"].(float64); ok {
				return int(c)
			}
		}
	}
	return 0
}

func extractTypes(results *[]surrealdb.QueryResult[any]) []map[string]interface{} {
	if results == nil || len(*results) == 0 {
		return nil
	}
	result := (*results)[0].Result
	if arr, ok := result.([]interface{}); ok {
		out := make([]map[string]interface{}, 0)
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				if t, exists := m["entity_type"]; exists {
					if ts, ok := t.(string); ok {
						m["entity_type"] = strings.TrimSpace(ts)
					}
				}
				out = append(out, m)
			}
		}
		return out
	}
	return nil
}

var _ Service = (*service)(nil)

type retryingService struct {
	cfg    config.SurrealConfig
	mu     sync.Mutex
	cached Service
}

func NewRetryingService(cfg config.SurrealConfig) Service {
	return &retryingService{cfg: cfg}
}

func (r *retryingService) get() Service {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cached != nil {
		return r.cached
	}
	client, err := NewClient(r.cfg)
	if err != nil {
		return NewStubService()
	}
	r.cached = NewService(client)
	return r.cached
}

func (r *retryingService) Name() string { return "图谱关系" }

func (r *retryingService) Process(ctx context.Context, convID string, messages []map[string]string, newReply string) error {
	return nil
}

func (r *retryingService) SyncNode(entityType, entityID, label string, properties map[string]interface{}) error {
	return r.get().SyncNode(entityType, entityID, label, properties)
}

func (r *retryingService) SyncEdge(sourceID, targetID, relationType string, weight float64) error {
	return r.get().SyncEdge(sourceID, targetID, relationType, weight)
}

func (r *retryingService) DeleteNode(entityID string) error {
	return r.get().DeleteNode(entityID)
}

func (r *retryingService) DeleteNodeIfOrphan(entityID string) error {
	return r.get().DeleteNodeIfOrphan(entityID)
}

func (r *retryingService) DeleteNodesByProperty(entityType, propertyKey, propertyValue string) error {
	return r.get().DeleteNodesByProperty(entityType, propertyKey, propertyValue)
}

func (r *retryingService) QueryNeighbors(entityID string, depth int, userID string) (map[string]interface{}, error) {
	return r.get().QueryNeighbors(entityID, depth, userID)
}

func (r *retryingService) FindPaths(sourceID, targetID string, maxDepth int) ([]map[string]interface{}, error) {
	return r.get().FindPaths(sourceID, targetID, maxDepth)
}

func (r *retryingService) DeleteOrphanNodes() error {
	return r.get().DeleteOrphanNodes()
}

func (r *retryingService) GetStats(userID string) (map[string]interface{}, error) {
	return r.get().GetStats(userID)
}

func (r *retryingService) GetAllNodes(userID string) ([]map[string]interface{}, error) {
	return r.get().GetAllNodes(userID)
}

func (r *retryingService) GetAllEdges(userID string) ([]map[string]interface{}, error) {
	return r.get().GetAllEdges(userID)
}

type stubService struct{}

func NewStubService() Service {
	return &stubService{}
}

func (s *stubService) Name() string { return "图谱关系" }

func (s *stubService) Process(ctx context.Context, convID string, messages []map[string]string, newReply string) error {
	return nil
}

func (s *stubService) SyncNode(entityType, entityID, label string, properties map[string]interface{}) error {
	return nil
}

func (s *stubService) SyncEdge(sourceID, targetID, relationType string, weight float64) error {
	return nil
}

func (s *stubService) DeleteNode(entityID string) error {
	return nil
}

func (s *stubService) DeleteNodeIfOrphan(entityID string) error {
	return nil
}

func (s *stubService) DeleteNodesByProperty(entityType, propertyKey, propertyValue string) error {
	return nil
}

func (s *stubService) QueryNeighbors(entityID string, depth int, userID string) (map[string]interface{}, error) {
	return nil, nil
}

func (s *stubService) FindPaths(sourceID, targetID string, maxDepth int) ([]map[string]interface{}, error) {
	return nil, nil
}

func (s *stubService) DeleteOrphanNodes() error {
	return nil
}

func (s *stubService) GetStats(userID string) (map[string]interface{}, error) {
	return map[string]interface{}{"nodeCount": 0, "edgeCount": 0, "byType": nil}, nil
}

func (s *stubService) GetAllNodes(userID string) ([]map[string]interface{}, error) {
	return nil, nil
}

func (s *stubService) GetAllEdges(userID string) ([]map[string]interface{}, error) {
	return nil, nil
}

func sanitizeRecordID(id string) string {
	return strings.ReplaceAll(id, "`", "")
}
