// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/surrealdb/surrealdb.go"
)

type Service interface {
	SyncNode(entityType, entityID, label string, properties map[string]interface{}) error
	SyncEdge(sourceID, targetID, relationType string, weight float64) error
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
	id := fmt.Sprintf("%s:%s", entityType, entityID)
	props, _ := json.Marshal(properties)
	query := fmt.Sprintf(
		"UPSERT entity_node:`%s` CONTENT {entity_type: \"%s\", label: \"%s\", properties: %s}",
		id, entityType, label, string(props),
	)
	_, err := surrealdb.Query[any](context.Background(), s.client.DB(), query, nil)
	return err
}

func (s *service) SyncEdge(sourceID, targetID, relationType string, weight float64) error {
	if s.client == nil || s.client.DB() == nil {
		return nil
	}
	query := fmt.Sprintf(
		"RELATE entity_node:`%s`->entity_edge->entity_node:`%s` SET relation_type=\"%s\", weight=%f",
		sourceID, targetID, relationType, weight,
	)
	_, err := surrealdb.Query[any](context.Background(), s.client.DB(), query, nil)
	return err
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
		"SELECT * FROM entity_node:`%s` WHERE out->entity_edge->in->entity_node:`%s` LIMIT 20",
		sourceID, targetID,
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
	_ = nodeResult
	_ = err

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
