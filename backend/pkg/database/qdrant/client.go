package qdrant

import (
	"context"
	"fmt"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/log"
)

var Client *qdrant.Client

func InitClient() error {
	cfg := config.AppCfg.Qdrant

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var err error
	Client, err = qdrant.NewClient(&qdrant.Config{
		Host:                   cfg.Host,
		Port:                   cfg.Port + 1,
		UseTLS:                 false,
		SkipCompatibilityCheck: true,
	})
	if err != nil {
		return fmt.Errorf("创建Qdrant客户端失败: %w", err)
	}

	_, err = Client.HealthCheck(ctx)
	if err != nil {
		return fmt.Errorf("Qdrant健康检查失败: %w", err)
	}

	log.Info(fmt.Sprintf("Qdrant客户端连接成功 %s:%d", cfg.Host, cfg.Port))
	return nil
}

func EnsureCollection() error {
	cfg := config.AppCfg.Qdrant
	ctx := context.Background()

	exists, err := Client.CollectionExists(ctx, cfg.CollectionName)
	if err != nil {
		return fmt.Errorf("检查集合失败: %w", err)
	}

	if !exists {
		log.Info(fmt.Sprintf("创建Qdrant集合: %s", cfg.CollectionName))
		req := &qdrant.CreateCollection{
			CollectionName: cfg.CollectionName,
			VectorsConfig: &qdrant.VectorsConfig{
				Config: &qdrant.VectorsConfig_Params{
					Params: &qdrant.VectorParams{
						Size:     uint64(cfg.VectorDim),
						Distance: qdrant.Distance_Cosine,
					},
				},
			},
		}
		if err := Client.CreateCollection(ctx, req); err != nil {
			return fmt.Errorf("创建集合失败: %w", err)
		}
		log.Info(fmt.Sprintf("集合%s创建成功", cfg.CollectionName))
	} else {
		log.Info(fmt.Sprintf("使用已有集合: %s", cfg.CollectionName))
	}

	return nil
}

type VectorPoint struct {
	ID      string
	Vector  []float32
	Payload map[string]interface{}
}

func UpsertVectors(points []VectorPoint) error {
	cfg := config.AppCfg.Qdrant
	ctx := context.Background()

	qdrantPoints := make([]*qdrant.PointStruct, len(points))
	for i, p := range points {
		qdrantPoints[i] = &qdrant.PointStruct{
			Id:      qdrant.NewID(p.ID),
			Vectors: qdrant.NewVectors(p.Vector...),
			Payload: qdrant.NewValueMap(p.Payload),
		}
	}

	_, err := Client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: cfg.CollectionName,
		Points:         qdrantPoints,
	})
	return err
}

func DeleteVectors(ids []string) error {
	cfg := config.AppCfg.Qdrant
	ctx := context.Background()

	qdrantIDs := make([]*qdrant.PointId, len(ids))
	for i, id := range ids {
		qdrantIDs[i] = qdrant.NewID(id)
	}

	_, err := Client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: cfg.CollectionName,
		Points:         qdrant.NewPointsSelector(qdrantIDs...),
	})
	return err
}

func SearchVectors(vector []float32, limit int, filter map[string]interface{}) ([]*qdrant.ScoredPoint, error) {
	cfg := config.AppCfg.Qdrant
	ctx := context.Background()

	if limit <= 0 {
		limit = cfg.Limit
	}

	queryReq := &qdrant.QueryPoints{
		CollectionName: cfg.CollectionName,
		Query:          qdrant.NewQuery(vector...),
		Limit:          qdrant.PtrOf(uint64(limit)),
		WithPayload:    qdrant.NewWithPayload(true),
	}

	if len(filter) > 0 {
		queryReq.Filter = buildFilter(filter)
	}

	return Client.Query(ctx, queryReq)
}

func buildFilter(filter map[string]interface{}) *qdrant.Filter {
	conditions := make([]*qdrant.Condition, 0)
	for key, value := range filter {
		switch v := value.(type) {
		case string:
			conditions = append(conditions, qdrant.NewMatchKeyword(key, v))
		}
	}
	if len(conditions) == 0 {
		return nil
	}
	return &qdrant.Filter{
		Must: conditions,
	}
}

func GetVectorCount() (uint64, error) {
	cfg := config.AppCfg.Qdrant
	ctx := context.Background()

	info, err := Client.GetCollectionInfo(ctx, cfg.CollectionName)
	if err != nil {
		return 0, err
	}
	if info.PointsCount == nil {
		return 0, nil
	}
	return *info.PointsCount, nil
}