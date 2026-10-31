package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/embedding"
	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
	amitialog "log"
)

type IndexPayload struct {
	ChunkIDs     []string `json:"chunkIds"`
	IndexVersion string   `json:"indexVersion"`
}

type Indexer struct {
	repo      *Repository
	embedding *embedding.Service
	settings  runtimeSettings
	gate      *resourceGate
}

func NewIndexer(repo *Repository, embeddingSvc *embedding.Service, runtime ...runtimeSettings) *Indexer {
	settings := loadRuntimeSettings()
	if len(runtime) > 0 {
		settings = runtime[0]
	}
	return &Indexer{repo: repo, embedding: embeddingSvc, settings: settings, gate: newResourceGate(settings.IndexWorkers)}
}

func configuredKnowledgeCollectionBase() string {
	name := qdrantDB.ResolveConfiguredCollection("knowledge_embeddings")
	if strings.TrimSpace(name) == "" {
		return "knowledge_embeddings"
	}
	return name
}

func safeIndexVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		v = DefaultIndexVersion
	}
	var b strings.Builder
	for _, r := range v {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

func knowledgeCollectionName(version string) string {
	return configuredKnowledgeCollectionBase() + "_" + safeIndexVersion(version)
}

func configuredKnowledgeVectorDim() int {
	if config.AppCfg == nil {
		return 2560
	}
	cfg := config.AppCfg.Providers.VectorStore.Qdrant
	if c, ok := cfg.Collections["knowledge_embeddings"]; ok && c.VectorDim > 0 {
		return c.VectorDim
	}
	if cfg.VectorDim > 0 {
		return cfg.VectorDim
	}
	return 2560
}

func (x *Indexer) Available() bool {
	return x != nil && x.embedding != nil && qdrantDB.Client != nil
}

func (x *Indexer) Process(ctx context.Context, outbox IndexOutbox) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	release, err := x.gate.Acquire(ctx)
	if err != nil {
		return err
	}
	defer release()
	var payload IndexPayload
	if err := json.Unmarshal([]byte(outbox.PayloadJSON), &payload); err != nil {
		return fmt.Errorf("decode index payload: %w", err)
	}
	if len(payload.ChunkIDs) == 0 {
		return nil
	}
	if payload.IndexVersion == "" {
		payload.IndexVersion = DefaultIndexVersion
	}
	if strings.EqualFold(strings.TrimSpace(outbox.Operation), "delete") {
		if qdrantDB.Client == nil {
			return errors.New("vector index unavailable")
		}
		return qdrantDB.DeleteVectorsWithContext(ctx, payload.ChunkIDs, knowledgeCollectionName(payload.IndexVersion))
	}
	if !x.Available() {
		return errors.New("vector index unavailable")
	}
	indexable, err := x.repo.RevisionIndexable(outbox.TenantID, outbox.AggregateID, outbox.SourceRevision)
	if err != nil {
		return err
	}
	if !indexable {
		// The canonical revision was tombstoned/failed before this leased
		// secondary-index operation started. Treat the stale upsert as consumed.
		return nil
	}
	chunks, err := x.repo.ChunksByIDs(outbox.TenantID, payload.ChunkIDs)
	if err != nil {
		return err
	}
	byID := make(map[string]Chunk, len(chunks))
	texts := make([]string, 0, len(chunks))
	ordered := make([]Chunk, 0, len(chunks))
	for _, c := range chunks {
		if c.ChunkIndex < 0 {
			continue
		}
		byID[c.ID] = c
	}
	for _, id := range payload.ChunkIDs {
		if c, ok := byID[id]; ok {
			ordered = append(ordered, c)
			texts = append(texts, c.EmbeddingText)
		}
	}
	if len(texts) == 0 {
		return nil
	}
	embedBatch := x.settings.EmbeddingBatchSize
	if embedBatch <= 0 {
		embedBatch = defaultRuntimeSettings().EmbeddingBatchSize
	}
	vectors := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += embedBatch {
		if err := ctx.Err(); err != nil {
			return err
		}
		end := start + embedBatch
		if end > len(texts) {
			end = len(texts)
		}
		batch, batchErr := x.embedding.BatchEmbed(texts[start:end])
		if batchErr != nil {
			return batchErr
		}
		if len(batch) != end-start {
			return fmt.Errorf("embedding count mismatch: batch=%d vectors=%d", end-start, len(batch))
		}
		vectors = append(vectors, batch...)
	}
	if len(vectors) != len(ordered) {
		return fmt.Errorf("embedding count mismatch: chunks=%d vectors=%d", len(ordered), len(vectors))
	}
	collection := knowledgeCollectionName(payload.IndexVersion)
	if err := qdrantDB.EnsureCollectionByNameWithContext(ctx, collection, configuredKnowledgeVectorDim()); err != nil {
		return err
	}
	points := make([]qdrantDB.VectorPoint, 0, len(ordered))
	for i, c := range ordered {
		pointPayload := map[string]interface{}{
			"chunk_id":        c.ID,
			"tenant_id":       c.TenantID,
			"source_id":       c.SourceID,
			"source_revision": fmt.Sprintf("%d", c.SourceRevision),
			"index_version":   payload.IndexVersion,
			"conflict_group":  c.ConflictGroup,
		}
		if c.ValidFrom != nil {
			pointPayload["valid_from"] = c.ValidFrom.UTC().Format(time.RFC3339Nano)
		}
		if c.ValidTo != nil {
			pointPayload["valid_to"] = c.ValidTo.UTC().Format(time.RFC3339Nano)
		}
		points = append(points, qdrantDB.VectorPoint{ID: c.ID, Vector: vectors[i], Payload: pointPayload})
	}
	qdrantBatch := x.settings.QdrantBatchSize
	if qdrantBatch <= 0 {
		qdrantBatch = defaultRuntimeSettings().QdrantBatchSize
	}
	for start := 0; start < len(points); start += qdrantBatch {
		if err := ctx.Err(); err != nil {
			return err
		}
		end := start + qdrantBatch
		if end > len(points) {
			end = len(points)
		}
		if err := qdrantDB.UpsertVectorsWithContext(ctx, points[start:end], collection); err != nil {
			return err
		}
	}
	// Re-check canonical state after the external write. A source can be
	// tombstoned while embeddings/Qdrant are in flight. If that happened, an
	// earlier delete outbox may already have been consumed, so emit a fresh
	// corrective delete after this upsert rather than leaving an orphan point.
	indexable, err = x.repo.RevisionIndexable(outbox.TenantID, outbox.AggregateID, outbox.SourceRevision)
	if err != nil {
		return err
	}
	if !indexable {
		if err := x.repo.EnqueueVectorDeleteCorrection(outbox.TenantID, outbox.AggregateID, outbox.SourceRevision, payload.ChunkIDs, payload.IndexVersion); err != nil {
			return err
		}
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return x.repo.MarkChunksIndexed(outbox.TenantID, payload.ChunkIDs, "active", payload.IndexVersion)
}

var errOutboxLeaseLost = errors.New("knowledge index outbox lease lost")

func (x *Indexer) processClaimedOutbox(ctx context.Context, outbox IndexOutbox, lease time.Duration) error {
	return x.withOutboxLease(ctx, outbox, lease, func(workCtx context.Context) error {
		return x.Process(workCtx, outbox)
	})
}

func (x *Indexer) withOutboxLease(ctx context.Context, outbox IndexOutbox, lease time.Duration, work func(context.Context) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if lease <= 0 {
		lease = defaultRuntimeSettings().OutboxLease
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	valid, err := x.repo.RenewOutboxLease(outbox.TenantID, outbox.ID, outbox.LeaseToken, time.Now().UTC(), lease)
	if err != nil {
		return err
	}
	if !valid {
		return errOutboxLeaseLost
	}
	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := make(chan struct{})
	ended := make(chan error, 1)
	interval := lease / 3
	if interval < 10*time.Millisecond {
		interval = 10 * time.Millisecond
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				ended <- nil
				return
			case <-workCtx.Done():
				ended <- nil
				return
			case <-ticker.C:
				valid, err := x.repo.RenewOutboxLease(outbox.TenantID, outbox.ID, outbox.LeaseToken, time.Now().UTC(), lease)
				if err != nil || !valid {
					cancel()
					if err != nil {
						ended <- fmt.Errorf("renew index lease: %w", err)
					} else {
						ended <- errOutboxLeaseLost
					}
					return
				}
			}
		}
	}()
	processErr := work(workCtx)
	close(stop)
	leaseErr := <-ended
	if leaseErr != nil {
		return leaseErr
	}
	return processErr
}

func (x *Indexer) ProcessSource(ctx context.Context, tenantID, sourceID string, revision int64) (indexed int64, degraded bool, warnings []string) {
	if ctx == nil {
		ctx = context.Background()
	}
	if !x.Available() {
		return 0, true, []string{"向量索引当前不可用，知识库已保留 SQLite 词法检索能力"}
	}
	maxOutboxes := x.settings.MaxOutboxesPerJob
	if maxOutboxes <= 0 {
		maxOutboxes = 100000
	}
	for processed := 0; processed < maxOutboxes; processed++ {
		if err := ctx.Err(); err != nil {
			return indexed, true, dedupeStrings(append(warnings, err.Error()))
		}
		lease := x.settings.OutboxLease
		if lease <= 0 {
			lease = defaultRuntimeSettings().OutboxLease
		}
		ob, claimed, err := x.repo.ClaimNextOutboxForSourceRevision(tenantID, sourceID, revision, time.Now().UTC(), lease)
		if err != nil {
			return indexed, true, dedupeStrings(append(warnings, err.Error()))
		}
		if !claimed || ob == nil {
			break
		}
		if err := x.processClaimedOutbox(ctx, *ob, lease); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				if _, err := x.repo.FinishOutboxLease(tenantID, ob.ID, ob.LeaseToken, map[string]any{
					"status": "pending", "last_error": err.Error(), "next_attempt_at": time.Now().UTC(),
				}); err != nil {
					amitialog.Printf("[original-error] %v", err)
				}
				return indexed, true, dedupeStrings(append(warnings, err.Error()))
			}
			next := time.Now().UTC().Add(x.settings.outboxRetryDelay(ob.RetryCount))
			if _, err := x.repo.FinishOutboxLease(tenantID, ob.ID, ob.LeaseToken, map[string]any{
				"status": "failed", "retry_count": ob.RetryCount + 1, "last_error": err.Error(), "next_attempt_at": next,
			}); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			degraded = true
			warnings = append(warnings, err.Error())
			continue
		}
		if changed, finishErr := x.repo.FinishOutboxLease(tenantID, ob.ID, ob.LeaseToken, map[string]any{"status": "done", "last_error": "", "next_attempt_at": nil}); finishErr != nil || !changed {
			degraded = true
			if finishErr != nil {
				warnings = append(warnings, finishErr.Error())
			} else {
				warnings = append(warnings, "secondary-index lease changed before completion")
			}
		}
	}
	indexed, _ = x.repo.CountIndexedChunks(tenantID, sourceID, revision)
	return indexed, degraded, dedupeStrings(warnings)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func dedupeStrings(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
