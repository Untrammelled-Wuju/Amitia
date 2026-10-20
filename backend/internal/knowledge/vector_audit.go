package knowledge

import (
	"context"
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
)

const maxVectorAuditSample = 100

type VectorAuditResult struct {
	Status             string    `json:"status"`
	Sampled            int       `json:"sampled"`
	Missing            int       `json:"missing"`
	Mismatched         int       `json:"mismatched"`
	CollectionsChecked int       `json:"collectionsChecked"`
	SampleLimit        int       `json:"sampleLimit"`
	Exhaustive         bool      `json:"exhaustive"`
	ObservedAt         time.Time `json:"observedAt"`
}

type vectorAuditCandidate struct {
	ID             string `gorm:"column:id"`
	TenantID       string `gorm:"column:tenant_id"`
	SourceID       string `gorm:"column:source_id"`
	SourceRevision int64  `gorm:"column:source_revision"`
	IndexVersion   string `gorm:"column:index_version"`
}

type vectorMetadataFetcher func(context.Context, []string, string) (map[string]qdrantDB.VectorMetadata, error)

const vectorAuditEligibleSQL = `SELECT c.id, c.tenant_id, c.source_id, c.source_revision, c.index_version
        FROM knowledge_chunks AS c
        INNER JOIN knowledge_sources AS s
          ON s.tenant_id = c.tenant_id AND s.id = c.source_id
        INNER JOIN knowledge_source_revisions AS rev
          ON rev.tenant_id = c.tenant_id AND rev.source_id = c.source_id AND rev.revision = c.source_revision
        WHERE c.tenant_id = ? AND c.chunk_index >= 0 AND c.status = 'indexed'
          AND s.deleted_at IS NULL AND s.status IN ('ready', 'ready_degraded')
          AND rev.purged_at IS NULL AND rev.status IN ('ready', 'ready_degraded')
          AND NOT EXISTS (
            SELECT 1 FROM knowledge_index_outbox AS o
            WHERE o.tenant_id = c.tenant_id AND o.aggregate_type = 'knowledge_source'
              AND o.aggregate_id = c.source_id AND o.source_revision = c.source_revision
              AND o.status IN ('pending', 'failed') AND o.operation IN ('upsert', 'delete')
          )`

func vectorAuditDailyOffset(total, limit int64, tenantID string, now time.Time) int64 {
	if total <= limit {
		return 0
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(tenantID))
	_, _ = h.Write([]byte(now.UTC().Format("2006-01-02")))
	return int64(h.Sum64() % uint64(total-limit+1))
}

func (r *Repository) vectorAuditCandidates(ctx context.Context, tenantID string, limit int) ([]vectorAuditCandidate, error) {
	if strings.TrimSpace(tenantID) == "" {
		return nil, fmt.Errorf("tenant scope required")
	}
	if limit < 1 || limit > maxVectorAuditSample {
		return nil, fmt.Errorf("invalid vector audit limit")
	}
	var total int64
	if err := r.db.WithContext(ctx).Raw("SELECT COUNT(*) FROM ("+vectorAuditEligibleSQL+") AS eligible", tenantID).Scan(&total).Error; err != nil {
		return nil, err
	}
	rows := make([]vectorAuditCandidate, 0)
	if total == 0 {
		return rows, nil
	}
	offset := vectorAuditDailyOffset(total, int64(limit), tenantID, time.Now())
	err := r.db.WithContext(ctx).Raw(vectorAuditEligibleSQL+" ORDER BY c.id ASC LIMIT ? OFFSET ?", tenantID, limit, offset).Scan(&rows).Error
	return rows, err
}

func validateVectorAuditPoint(expected vectorAuditCandidate, actual qdrantDB.VectorMetadata) bool {
	metadata := actual.Payload
	version := strings.TrimSpace(expected.IndexVersion)
	if version == "" {
		version = DefaultIndexVersion
	}
	return strings.EqualFold(strings.TrimSpace(actual.ID), expected.ID) &&
		metadata["chunk_id"] == expected.ID &&
		metadata["tenant_id"] == expected.TenantID &&
		metadata["source_id"] == expected.SourceID &&
		metadata["source_revision"] == fmt.Sprintf("%d", expected.SourceRevision) &&
		metadata["index_version"] == version
}

func auditVectorCandidates(ctx context.Context, candidates []vectorAuditCandidate, limit int, fetch vectorMetadataFetcher) (VectorAuditResult, error) {
	result := VectorAuditResult{Status: "empty", SampleLimit: limit, ObservedAt: time.Now().UTC(), Exhaustive: false}
	result.Sampled = len(candidates)
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if len(candidates) == 0 {
		return result, nil
	}
	if fetch == nil {
		result.Status = "unavailable"
		return result, fmt.Errorf("vector index unavailable")
	}
	grouped := map[string][]vectorAuditCandidate{}
	for _, candidate := range candidates {
		grouped[knowledgeCollectionName(candidate.IndexVersion)] = append(grouped[knowledgeCollectionName(candidate.IndexVersion)], candidate)
	}
	for collection, group := range grouped {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		ids := make([]string, len(group))
		for i, candidate := range group {
			ids[i] = candidate.ID
		}
		points, err := fetch(ctx, ids, collection)
		if err != nil {
			result.Status = "unavailable"
			return result, fmt.Errorf("vector audit collection unavailable: %w", err)
		}
		result.CollectionsChecked++
		for _, candidate := range group {
			actual, exists := points[candidate.ID]
			if !exists {
				result.Missing++
			} else if !validateVectorAuditPoint(candidate, actual) {
				result.Mismatched++
			}
		}
	}
	result.Status = "healthy"
	if result.Missing > 0 || result.Mismatched > 0 {
		result.Status = "degraded"
	}
	return result, nil
}

func (s *service) AuditVectors(ctx context.Context, tenantID string, limit int) (VectorAuditResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > maxVectorAuditSample {
		limit = maxVectorAuditSample
	}
	rows, err := s.repo.vectorAuditCandidates(ctx, tenantID, limit)
	if err != nil {
		return VectorAuditResult{}, err
	}
	return auditVectorCandidates(ctx, rows, limit, qdrantDB.GetVectorMetadataWithContext)
}
