package qdrant

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/qdrant/go-client/qdrant"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/log"
)

var Client *qdrant.Client

func InitClient() error {
	cfg := config.AppCfg.Providers.VectorStore.Qdrant

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
	cfg := config.AppCfg.Providers.VectorStore.Qdrant
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
	return EnsureCollectionByNameWithContext(context.Background(), collectionName, vectorDim)
}

func EnsureCollectionByNameWithContext(ctx context.Context, collectionName string, vectorDim int) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

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
	return UpsertVectorsWithContext(context.Background(), points, collectionName...)
}

func UpsertVectorsWithContext(ctx context.Context, points []VectorPoint, collectionName ...string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	target := resolveCollectionName(collectionName...)

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
		Wait:           qdrant.PtrOf(true),
		Points:         qdrantPoints,
	})
	return err
}

func DeleteVectors(ids []string, collectionName ...string) error {
	return DeleteVectorsWithContext(context.Background(), ids, collectionName...)
}

func DeleteVectorsWithContext(ctx context.Context, ids []string, collectionName ...string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	if Client == nil {
		return fmt.Errorf("vector index unavailable")
	}
	target := resolveCollectionName(collectionName...)
	exists, err := Client.CollectionExists(ctx, target)
	if err != nil {
		return fmt.Errorf("check vector collection before delete: %w", err)
	}
	if !exists {
		return nil
	}

	qdrantIDs := make([]*qdrant.PointId, len(ids))
	for i, id := range ids {
		qdrantIDs[i] = qdrant.NewID(id)
	}

	_, err = Client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: target,
		Wait:           qdrant.PtrOf(true),
		Points:         qdrant.NewPointsSelector(qdrantIDs...),
	})
	return err
}

type VectorMetadata struct {
	ID      string
	Payload map[string]string
}

func GetVectorMetadataWithContext(ctx context.Context, ids []string, collectionName string) (map[string]VectorMetadata, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if Client == nil {
		return nil, fmt.Errorf("vector index unavailable")
	}
	if len(ids) == 0 {
		return map[string]VectorMetadata{}, nil
	}
	if len(ids) > 100 || collectionName == "" {
		return nil, fmt.Errorf("invalid vector metadata request")
	}
	pointIDs := make([]*qdrant.PointId, len(ids))
	for i, id := range ids {
		if _, err := uuid.Parse(id); err != nil {
			return nil, fmt.Errorf("invalid vector point ID: %w", err)
		}
		pointIDs[i] = qdrant.NewID(id)
	}
	points, err := Client.Get(ctx, &qdrant.GetPoints{
		CollectionName: collectionName,
		Ids:            pointIDs,
		WithPayload:    qdrant.NewWithPayloadInclude("chunk_id", "tenant_id", "source_id", "source_revision", "index_version"),
	})
	if err != nil {
		return nil, err
	}
	result := make(map[string]VectorMetadata, len(points))
	for _, point := range points {
		if point == nil || point.GetId() == nil {
			continue
		}
		id := point.GetId().GetUuid()
		if id == "" {
			continue
		}
		payload := make(map[string]string, 5)
		for _, key := range []string{"chunk_id", "tenant_id", "source_id", "source_revision", "index_version"} {
			if field := point.GetPayload()[key]; field != nil {
				payload[key] = field.GetStringValue()
			}
		}
		result[id] = VectorMetadata{ID: id, Payload: payload}
	}
	return result, nil
}

func SearchVectors(vector []float32, limit int, filter map[string]interface{}, collectionName ...string) ([]*qdrant.ScoredPoint, error) {
	cfg := config.AppCfg.Providers.VectorStore.Qdrant
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
		limit = config.AppCfg.Providers.VectorStore.Qdrant.Limit
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
	cfg := config.AppCfg.Providers.VectorStore.Qdrant
	if len(cfg.Collections) == 0 {
		return []string{defaultCollectionName()}
	}
	keys := []string{"amitia_emotes"}
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
	if c, ok := config.AppCfg.Providers.VectorStore.Qdrant.Collections[key]; ok {
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
		return "amitia_vectors"
	}
	if config.AppCfg.Providers.VectorStore.Qdrant.CollectionName != "" {
		return config.AppCfg.Providers.VectorStore.Qdrant.CollectionName
	}
	return "amitia_vectors"
}

func defaultVectorDim() int {
	if config.AppCfg == nil {
		return 1536
	}
	if config.AppCfg.Providers.VectorStore.Qdrant.VectorDim > 0 {
		return config.AppCfg.Providers.VectorStore.Qdrant.VectorDim
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

func ScanTenantVectorMetadataWithContext(ctx context.Context, tenantID, collectionName string, limit int) ([]VectorMetadata, error) {
	points, _, err := ScanTenantVectorMetadataPageWithContext(ctx, tenantID, collectionName, limit, "")
	return points, err
}

func ScanTenantVectorMetadataPageWithContext(ctx context.Context, tenantID, collectionName string, limit int, offset string) ([]VectorMetadata, string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	if Client == nil {
		return nil, "", fmt.Errorf("vector index unavailable")
	}
	if strings.TrimSpace(tenantID) == "" || strings.TrimSpace(collectionName) == "" || limit <= 0 || limit > 100 {
		return nil, "", fmt.Errorf("invalid vector scan request")
	}
	request := &qdrant.ScrollPoints{
		CollectionName: collectionName,
		Filter:         &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewMatchKeyword("tenant_id", tenantID)}},
		Limit:          qdrant.PtrOf(uint32(limit)),
		WithPayload:    qdrant.NewWithPayloadInclude("chunk_id", "tenant_id", "source_id", "source_revision", "index_version"),
	}
	if offset != "" {
		if _, err := uuid.Parse(offset); err != nil {
			return nil, "", fmt.Errorf("invalid vector scan offset: %w", err)
		}
		request.Offset = qdrant.NewID(offset)
	}
	points, nextOffset, err := Client.ScrollAndOffset(ctx, request)
	if err != nil {
		return nil, "", err
	}
	if len(points) > limit {
		return nil, "", fmt.Errorf("vector scan exceeded page bound")
	}
	result := make([]VectorMetadata, 0, len(points))
	for _, point := range points {
		if point == nil || point.GetId() == nil {
			return nil, "", fmt.Errorf("vector scan returned invalid point")
		}
		id := point.GetId().GetUuid()
		if _, parseErr := uuid.Parse(id); parseErr != nil {
			return nil, "", fmt.Errorf("vector scan returned invalid point ID: %w", parseErr)
		}
		payload := make(map[string]string, 5)
		for _, key := range []string{"chunk_id", "tenant_id", "source_id", "source_revision", "index_version"} {
			if field := point.GetPayload()[key]; field != nil {
				payload[key] = field.GetStringValue()
			}
		}
		if payload["tenant_id"] != tenantID {
			return nil, "", fmt.Errorf("vector scan tenant isolation mismatch")
		}
		result = append(result, VectorMetadata{ID: id, Payload: payload})
	}
	next := ""
	if nextOffset != nil {
		next = nextOffset.GetUuid()
		if _, err := uuid.Parse(next); err != nil || next == offset || len(points) == 0 {
			return nil, "", fmt.Errorf("vector scan returned invalid continuation offset")
		}
	}
	return result, next, nil
}

func ListKnowledgeCollectionNamesWithContext(ctx context.Context, base string) ([]string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if Client == nil || strings.TrimSpace(base) == "" {
		return nil, fmt.Errorf("vector index unavailable")
	}
	names, err := Client.ListCollections(ctx)
	if err != nil {
		return nil, err
	}
	filtered := make([]string, 0)
	for _, name := range names {
		if strings.HasPrefix(name, base+"_") && len(name) > len(base)+1 {
			filtered = append(filtered, name)
		}
	}
	sort.Strings(filtered)
	return filtered, nil
}
