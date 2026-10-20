package knowledge

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
)

const maxOrphanAuditCollections = 4

type OrphanVectorAuditResult struct {
	Status             string    `json:"status"`
	Sampled            int       `json:"sampled"`
	SuspectedOrphans   int       `json:"suspectedOrphans"`
	MetadataMismatches int       `json:"metadataMismatches"`
	InFlight           int       `json:"inFlight"`
	CollectionsChecked int       `json:"collectionsChecked"`
	CollectionsKnown   int       `json:"collectionsKnown"`
	SampleLimit        int       `json:"sampleLimit"`
	Exhaustive         bool      `json:"exhaustive"`
	ObservedAt         time.Time `json:"observedAt"`
}

type orphanVectorRow struct {
	ID             string `gorm:"column:id"`
	SourceID       string `gorm:"column:source_id"`
	SourceRevision int64  `gorm:"column:source_revision"`
	IndexVersion   string `gorm:"column:index_version"`
	Unindexable    int    `gorm:"column:unindexable"`
}

type orphanOutboxRow struct {
	SourceID       string `gorm:"column:aggregate_id"`
	SourceRevision int64  `gorm:"column:source_revision"`
}

type orphanVectorState struct {
	Chunks  map[string]orphanVectorRow
	Pending map[string]bool
}

type tenantVectorScanner func(context.Context, string, string, int) ([]qdrantDB.VectorMetadata, error)
type orphanStateLoader func(context.Context, string, []qdrantDB.VectorMetadata) (orphanVectorState, error)

func orphanSourceRevisionKey(sourceID string, revision int64) string {
	return sourceID + "\x00" + strconv.FormatInt(revision, 10)
}

func (r *Repository) orphanAuditState(ctx context.Context, tenantID string, points []qdrantDB.VectorMetadata) (orphanVectorState, error) {
	state := orphanVectorState{Chunks: map[string]orphanVectorRow{}, Pending: map[string]bool{}}
	if strings.TrimSpace(tenantID) == "" {
		return state, fmt.Errorf("tenant scope required")
	}
	if len(points) == 0 {
		return state, nil
	}
	if len(points) > maxVectorAuditSample {
		return state, fmt.Errorf("orphan sample exceeds limit")
	}
	ids := make([]string, 0, len(points))
	sources := make([]string, 0, len(points))
	for _, point := range points {
		if point.Payload["tenant_id"] != tenantID {
			return state, fmt.Errorf("vector tenant isolation mismatch")
		}
		ids = append(ids, point.ID)
		if source := strings.TrimSpace(point.Payload["source_id"]); source != "" {
			sources = append(sources, source)
		}
	}
	var chunks []orphanVectorRow
	if err := r.db.WithContext(ctx).Raw(`SELECT c.id, c.source_id, c.source_revision, c.index_version,
		CASE WHEN c.chunk_index < 0 OR COALESCE(c.status, '') <> 'indexed'
		OR s.id IS NULL OR s.deleted_at IS NOT NULL OR COALESCE(s.status, '') NOT IN ('ready', 'ready_degraded')
		OR rev.source_id IS NULL OR rev.purged_at IS NOT NULL OR COALESCE(rev.status, '') NOT IN ('ready', 'ready_degraded')
		THEN 1 ELSE 0 END AS unindexable
		FROM knowledge_chunks c
		LEFT JOIN knowledge_sources s ON s.id = c.source_id AND s.tenant_id = c.tenant_id
		LEFT JOIN knowledge_source_revisions rev ON rev.source_id = c.source_id
		AND rev.tenant_id = c.tenant_id AND rev.revision = c.source_revision
		WHERE c.tenant_id = ? AND c.id IN ?`, tenantID, ids).Scan(&chunks).Error; err != nil {
		return state, err
	}
	for _, chunk := range chunks {
		state.Chunks[chunk.ID] = chunk
	}
	if len(sources) == 0 {
		return state, nil
	}
	var pending []orphanOutboxRow
	if err := r.db.WithContext(ctx).Raw(`SELECT DISTINCT aggregate_id, source_revision FROM knowledge_index_outbox
		WHERE tenant_id = ? AND aggregate_type = 'knowledge_source' AND aggregate_id IN ?
		AND operation IN ('upsert', 'delete') AND status IN ('pending', 'failed')`, tenantID, sources).Scan(&pending).Error; err != nil {
		return state, err
	}
	for _, ob := range pending {
		state.Pending[orphanSourceRevisionKey(ob.SourceID, ob.SourceRevision)] = true
	}
	return state, nil
}

func classifyOrphanVectorPoints(points []qdrantDB.VectorMetadata, tenantID, version string, state orphanVectorState) (orphans, mismatches, inflight int, err error) {
	for _, point := range points {
		if point.Payload["tenant_id"] != tenantID {
			return 0, 0, 0, fmt.Errorf("vector tenant isolation mismatch")
		}
		revision, parseErr := strconv.ParseInt(strings.TrimSpace(point.Payload["source_revision"]), 10, 64)
		if parseErr == nil && state.Pending[orphanSourceRevisionKey(point.Payload["source_id"], revision)] {
			inflight++
			continue
		}
		chunk, exists := state.Chunks[point.ID]
		if !exists || chunk.Unindexable != 0 {
			orphans++
			continue
		}
		if parseErr != nil || point.Payload["chunk_id"] != point.ID || point.Payload["source_id"] != chunk.SourceID ||
			revision != chunk.SourceRevision || knowledgeCollectionName(point.Payload["index_version"]) != version || knowledgeCollectionName(chunk.IndexVersion) != version {
			mismatches++
		}
	}
	return
}

func auditTenantOrphanVectors(ctx context.Context, tenantID string, versions []string, limit int, scan tenantVectorScanner, load orphanStateLoader) (OrphanVectorAuditResult, error) {
	result := OrphanVectorAuditResult{Status: "empty", SampleLimit: limit, ObservedAt: time.Now().UTC(), Exhaustive: false, CollectionsKnown: len(versions)}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if strings.TrimSpace(tenantID) == "" || limit < 1 || limit > maxVectorAuditSample || scan == nil || load == nil {
		return result, fmt.Errorf("invalid orphan vector audit request")
	}
	collectionLimit := len(versions)
	if collectionLimit > maxOrphanAuditCollections {
		collectionLimit = maxOrphanAuditCollections
	}
	start := vectorAuditDailyOffset(int64(len(versions)), int64(collectionLimit), tenantID, time.Now())
	for i := 0; i < collectionLimit && result.Sampled < limit; i++ {
		collection := versions[(int(start)+i)%len(versions)]
		if err := ctx.Err(); err != nil {
			return result, err
		}
		remainingCollections := collectionLimit - i
		batch := (limit - result.Sampled + remainingCollections - 1) / remainingCollections
		if batch > 25 {
			batch = 25
		}
		points, err := scan(ctx, tenantID, collection, batch)
		if err != nil {
			result.Status = "unavailable"
			return result, fmt.Errorf("orphan vector collection unavailable: %w", err)
		}
		if len(points) > batch {
			return result, fmt.Errorf("vector scanner exceeded bounded request")
		}
		state, err := load(ctx, tenantID, points)
		if err != nil {
			result.Status = "unavailable"
			return result, fmt.Errorf("orphan vector canonical check unavailable: %w", err)
		}
		orphans, mismatches, inflight, err := classifyOrphanVectorPoints(points, tenantID, collection, state)
		if err != nil {
			return result, err
		}
		result.Sampled += len(points)
		result.CollectionsChecked++
		result.SuspectedOrphans += orphans
		result.MetadataMismatches += mismatches
		result.InFlight += inflight
	}
	result.Status = "sampled"
	if result.Sampled == 0 {
		result.Status = "empty"
	}
	if result.SuspectedOrphans > 0 || result.MetadataMismatches > 0 {
		result.Status = "suspected"
	}
	return result, nil
}

func (s *service) AuditOrphanVectors(ctx context.Context, tenantID string, limit int) (OrphanVectorAuditResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > maxVectorAuditSample {
		limit = maxVectorAuditSample
	}
	if strings.TrimSpace(tenantID) == "" {
		return OrphanVectorAuditResult{}, fmt.Errorf("tenant scope required")
	}
	versions, err := qdrantDB.ListKnowledgeCollectionNamesWithContext(ctx, configuredKnowledgeCollectionBase())
	if err != nil {
		return OrphanVectorAuditResult{}, err
	}
	return auditTenantOrphanVectors(ctx, tenantID, versions, limit, qdrantDB.ScanTenantVectorMetadataWithContext, s.repo.orphanAuditState)
}
