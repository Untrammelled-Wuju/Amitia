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
	return EnsureCollectionByName(defaultCollectionName(), defaultVectorDim())
}

func EnsureCollections() error {
	cfg := config.AppCfg.Qdrant
	collections := cfg.Collections
	if len(collections) == 0 {
		return EnsureCollection()
	}
	for key, c := range collections {
		name := c.Name
		if name == "" {
			name = key
		}
		dim := c.VectorDim
		if dim <= 0 {
			dim = cfg.VectorDim
		}
		if err := EnsureCollectionByName(name, dim); err != nil {
			return err
		}
	}
	return nil
}

func EnsureCollectionByName(collectionName string, vectorDim int) error {
	ctx := context.Background()

	if collectionName == "" {
		collectionName = defaultCollectionName()
	}
	if vectorDim <= 0 {
		vectorDim = defaultVectorDim()
	}

	exists, err := Client.CollectionExists(ctx, collectionName)
	if err != nil {
		return fmt.Errorf("检查集合失败: %w", err)
	}

	if !exists {
		log.Info(fmt.Sprintf("创建Qdrant集合: %s", collectionName))
		return createCollection(ctx, collectionName, vectorDim)
	}

	info, err := Client.GetCollectionInfo(ctx, collectionName)
	if err != nil {
		return fmt.Errorf("获取集合信息失败: %w", err)
	}

	actualDim := collectionVectorDim(info)
	if actualDim > 0 && actualDim != uint64(vectorDim) {
		pointCount := uint64(0)
		if info.PointsCount != nil {
			pointCount = *info.PointsCount
		}
		if pointCount > 0 {
			return fmt.Errorf("集合%s维度不一致: 现有=%d 期望=%d 点数=%d，需要先迁移数据", collectionName, actualDim, vectorDim, pointCount)
		}
		log.Warn(fmt.Sprintf("集合%s维度不一致，准备重建: 现有=%d 期望=%d", collectionName, actualDim, vectorDim))
		if err := Client.DeleteCollection(ctx, collectionName); err != nil {
			return fmt.Errorf("删除旧集合失败: %w", err)
		}
		return createCollection(ctx, collectionName, vectorDim)
	}

	log.Info(fmt.Sprintf("使用已有集合: %s", collectionName))
	return nil
}

func createCollection(ctx context.Context, collectionName string, vectorDim int) error {
	req := &qdrant.CreateCollection{
		CollectionName: collectionName,
		VectorsConfig: &qdrant.VectorsConfig{
			Config: &qdrant.VectorsConfig_Params{
				Params: &qdrant.VectorParams{
					Size:     uint64(vectorDim),
					Distance: qdrant.Distance_Cosine,
				},
			},
		},
	}
	if err := Client.CreateCollection(ctx, req); err != nil {
		return fmt.Errorf("创建集合失败: %w", err)
	}
	log.Info(fmt.Sprintf("集合%s创建成功", collectionName))
	return nil
}

type VectorPoint struct {
	ID      string
	Vector  []float32
	Payload map[string]interface{}
}

func UpsertVectors(points []VectorPoint, collectionName ...string) error {
	target := resolveCollectionName(collectionName...)
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
		CollectionName: target,
		Points:         qdrantPoints,
	})
	return err
}

func DeleteVectors(ids []string, collectionName ...string) error {
	target := resolveCollectionName(collectionName...)
	ctx := context.Background()

	qdrantIDs := make([]*qdrant.PointId, len(ids))
	for i, id := range ids {
		qdrantIDs[i] = qdrant.NewID(id)
	}

	_, err := Client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: target,
		Points:         qdrant.NewPointsSelector(qdrantIDs...),
	})
	return err
}

func SearchVectors(vector []float32, limit int, filter map[string]interface{}, collectionName ...string) ([]*qdrant.ScoredPoint, error) {
	cfg := config.AppCfg.Qdrant
	target := resolveCollectionName(collectionName...)
	ctx := context.Background()

	if limit <= 0 {
		limit = cfg.Limit
	}

	queryReq := &qdrant.QueryPoints{
		CollectionName: target,
		Query:          qdrant.NewQuery(vector...),
		Limit:          qdrant.PtrOf(uint64(limit)),
		WithPayload:    qdrant.NewWithPayload(true),
	}

	if len(filter) > 0 {
		queryReq.Filter = buildFilter(filter)
	}

	return Client.Query(ctx, queryReq)
}

type CollectionScoredPoint struct {
	CollectionName string
	Point          *qdrant.ScoredPoint
}

func MultiSearch(vector []float32, limit int, filter map[string]interface{}, collectionNames ...string) ([]CollectionScoredPoint, error) {
	if len(collectionNames) == 0 {
		collectionNames = CollectionNames()
	}
	if limit <= 0 {
		limit = config.AppCfg.Qdrant.Limit
	}
	results := make([]CollectionScoredPoint, 0)
	var lastErr error
	for _, collectionName := range collectionNames {
		points, err := SearchVectors(vector, limit, filter, collectionName)
		if err != nil {
			lastErr = err
			continue
		}
		for _, p := range points {
			results = append(results, CollectionScoredPoint{CollectionName: collectionName, Point: p})
		}
	}
	if len(results) == 0 && lastErr != nil {
		return nil, lastErr
	}
	return results, nil
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

func GetVectorCount(collectionName ...string) (uint64, error) {
	target := resolveCollectionName(collectionName...)
	ctx := context.Background()

	info, err := Client.GetCollectionInfo(ctx, target)
	if err != nil {
		return 0, err
	}
	if info.PointsCount == nil {
		return 0, nil
	}
	return *info.PointsCount, nil
}

func CollectionNames() []string {
	cfg := config.AppCfg.Qdrant
	if len(cfg.Collections) == 0 {
		return []string{defaultCollectionName()}
	}
	keys := []string{"memory_embeddings", "working_memory", "user_profiles", "episodic_memories"}
	names := make([]string, 0, len(cfg.Collections))
	seen := map[string]bool{}
	for _, key := range keys {
		if c, ok := cfg.Collections[key]; ok {
			name := c.Name
			if name == "" {
				name = key
			}
			names = append(names, name)
			seen[name] = true
		}
	}
	for key, c := range cfg.Collections {
		name := c.Name
		if name == "" {
			name = key
		}
		if !seen[name] {
			names = append(names, name)
		}
	}
	return names
}

func ResolveConfiguredCollection(key string) string {
	if config.AppCfg == nil {
		return key
	}
	if c, ok := config.AppCfg.Qdrant.Collections[key]; ok {
		if c.Name != "" {
			return c.Name
		}
		return key
	}
	if key == "" {
		return defaultCollectionName()
	}
	return key
}

func resolveCollectionName(collectionName ...string) string {
	if len(collectionName) > 0 && collectionName[0] != "" {
		return collectionName[0]
	}
	return defaultCollectionName()
}

func defaultCollectionName() string {
	if config.AppCfg == nil {
		return "memory_embeddings"
	}
	if config.AppCfg.Qdrant.CollectionName != "" {
		return config.AppCfg.Qdrant.CollectionName
	}
	return ResolveConfiguredCollection("memory_embeddings")
}

func defaultVectorDim() int {
	if config.AppCfg == nil {
		return 1536
	}
	if config.AppCfg.Qdrant.VectorDim > 0 {
		return config.AppCfg.Qdrant.VectorDim
	}
	if c, ok := config.AppCfg.Qdrant.Collections["memory_embeddings"]; ok && c.VectorDim > 0 {
		return c.VectorDim
	}
	return 1536
}

func collectionVectorDim(info *qdrant.CollectionInfo) uint64 {
	if info == nil || info.Config == nil || info.Config.Params == nil {
		return 0
	}
	vectorsConfig := info.Config.Params.GetVectorsConfig()
	if vectorsConfig == nil {
		return 0
	}
	if params := vectorsConfig.GetParams(); params != nil {
		return params.GetSize()
	}
	if paramsMap := vectorsConfig.GetParamsMap(); paramsMap != nil {
		for _, params := range paramsMap.GetMap() {
			if params != nil {
				return params.GetSize()
			}
		}
	}
	return 0
}
