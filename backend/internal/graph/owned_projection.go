package graph

import (
	"context"
	"errors"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

type OwnedProjectionPort interface {
	WriteOwnedProjection(context.Context, string, map[string]any) error
	DeleteOwnedProjection(context.Context, string) error
}

func (s *SwitchableService) WriteOwnedProjection(ctx context.Context, id string, document map[string]any) error {
	return s.withService(func(current Service) error {
		port, ok := current.(OwnedProjectionPort)
		if !ok {
			return errGraphServiceUnavailable
		}
		return port.WriteOwnedProjection(ctx, id, document)
	})
}

func (s *SwitchableService) DeleteOwnedProjection(ctx context.Context, id string) error {
	return s.withService(func(current Service) error {
		port, ok := current.(OwnedProjectionPort)
		if !ok {
			return errGraphServiceUnavailable
		}
		return port.DeleteOwnedProjection(ctx, id)
	})
}

func (s *retryingService) WriteOwnedProjection(ctx context.Context, id string, document map[string]any) error {
	port, ok := s.get().(OwnedProjectionPort)
	if !ok {
		return errGraphServiceUnavailable
	}
	return port.WriteOwnedProjection(ctx, id, document)
}

func (s *retryingService) DeleteOwnedProjection(ctx context.Context, id string) error {
	port, ok := s.get().(OwnedProjectionPort)
	if !ok {
		return errGraphServiceUnavailable
	}
	return port.DeleteOwnedProjection(ctx, id)
}

func (s *service) ownedProjectionQuery(ctx context.Context, query string, parameters map[string]any) error {
	if s.client == nil || s.client.DB() == nil {
		return errors.New("图谱投影服务暂不可用")
	}
	results, err := surrealdb.Query[any](ctx, s.client.DB(), query, parameters)
	if err != nil {
		return err
	}
	if results == nil || len(*results) == 0 {
		return errors.New("图谱投影未返回保存确认")
	}
	for _, result := range *results {
		if result.Status != "OK" {
			return errors.New("图谱投影事务未确认")
		}
	}
	return nil
}

func (s *service) WriteOwnedProjection(ctx context.Context, id string, document map[string]any) error {
	owner, _ := document["ownerId"].(string)
	role, _ := document["roleId"].(string)
	source, _ := document["sourceId"].(string)
	if id == "" || owner == "" || role == "" || source == "" {
		return errors.New("图谱投影缺少数据归属")
	}
	return s.ownedProjectionQuery(ctx, "UPSERT $record CONTENT $document RETURN NONE;", map[string]any{"record": models.NewRecordID("amitia_owned_graph_v1", id), "document": document})
}

func (s *service) DeleteOwnedProjection(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("图谱投影编号无效")
	}
	return s.ownedProjectionQuery(ctx, "DELETE $record RETURN NONE;", map[string]any{"record": models.NewRecordID("amitia_owned_graph_v1", id)})
}
