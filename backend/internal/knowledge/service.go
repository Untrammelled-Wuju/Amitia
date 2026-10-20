package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/embedding"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type Service interface {
	Upload(ctx context.Context, tenantID, logicalName, filename, mimeType string, body io.Reader) (*Source, *IngestJob, error)
	ListSources(tenantID string) ([]Source, error)
	GetSource(tenantID, id string) (*Source, error)
	GetSourceDetail(tenantID, id string) (*SourceDetail, error)
	UploadRevision(ctx context.Context, tenantID, sourceID, filename, mimeType string, body io.Reader) (*SourceRevision, *IngestJob, error)
	ReindexSource(ctx context.Context, tenantID, id string) (*IngestJob, error)
	RollbackSourceRevision(ctx context.Context, tenantID, sourceID string, revision int64) (*IngestJob, error)
	PurgeSourceRevision(ctx context.Context, tenantID, sourceID string, revision int64) (*SourceRevision, error)
	GarbageCollect(ctx context.Context, tenantID string) (int, error)
	DeleteSource(tenantID, id string) error
	ListJobs(tenantID, sourceID string) ([]IngestJob, error)
	GetJob(tenantID, id string) (*IngestJob, error)
	RetryJob(ctx context.Context, tenantID, id string) (*IngestJob, error)
	CancelJob(ctx context.Context, tenantID, id string) (*IngestJob, error)
	Search(ctx context.Context, tenantID string, req SearchRequest) (SearchResponse, error)
	Read(ctx context.Context, tenantID string, req ReadRequest) (ReadResponse, error)
	GetSourceOutline(ctx context.Context, tenantID, sourceID string, revision, afterOrdinal int64, limit int) (UnitOutlineResponse, error)
	GetRuntimeStatus() RuntimeStatus
	GetSLOSummary(tenantID string) (RetrievalSLOSummary, error)
	GetIndexOutboxHealth(ctx context.Context, tenantID string) (IndexOutboxHealth, error)
	AuditVectors(ctx context.Context, tenantID string, limit int) (VectorAuditResult, error)
	AuditOrphanVectors(ctx context.Context, tenantID string, limit int) (OrphanVectorAuditResult, error)
	AuditOrphanVectorPage(ctx context.Context, tenantID string, limit int, cursor string) (OrphanVectorAuditPageResult, error)
	ListOrphanAuditSessions(ctx context.Context, tenantID string, limit int) ([]OrphanAuditSessionSummary, error)
	ListOrphanAuditPageReceipts(ctx context.Context, tenantID, auditID string, afterPage, limit int) (OrphanAuditReceiptsResult, error)
	RecheckOrphanAuditLatest(ctx context.Context, tenantID, auditID string) (OrphanAuditRecheckResult, error)
	ListOrphanAuditRechecks(ctx context.Context, tenantID, auditID string, limit int) ([]OrphanAuditRecheckRecord, error)
	CreateRecoveryReview(ctx context.Context, tenantID, auditID, recheckID, key, actorHash string) (RecoveryReviewResponse, error)
	TransitionRecoveryReview(ctx context.Context, tenantID, id, actorHash, transition string) (RecoveryReviewResponse, error)
	ListRecoveryReviews(ctx context.Context, tenantID, auditID string, limit int) ([]RecoveryReviewRequest, error)
	ListRecoveryReviewEvents(ctx context.Context, tenantID, id string) (RecoveryReviewEventHistory, error)
	GetRecoveryReviewPlan(ctx context.Context, tenantID, id, actorHash string) (RecoveryReviewPlan, error)
	ListRetrievalTraces(tenantID string, limit int) ([]RetrievalTraceSummary, error)
	RunBenchmark(ctx context.Context, tenantID string, req BenchmarkRequest) (BenchmarkResponse, error)
	ListBenchmarkRuns(tenantID string, limit int) ([]BenchmarkRunSummary, error)
	Reconcile(ctx context.Context, tenantID string) error
}

type service struct {
	repo      *Repository
	storage   *Storage
	parsers   *ParserRegistry
	embedding *embedding.Service
	indexer   *Indexer
	retrieval *RetrievalRuntime
	settings  runtimeSettings
	rootCtx   context.Context
	sem       chan struct{}
	readSem   chan struct{}
	mu        sync.Mutex
	inflight  map[string]struct{}
	cancels   map[string]context.CancelFunc
}

func NewService(appCtx *app.AppContext) Service {
	settings := loadRuntimeSettings()
	repo := NewRepositoryWithSettings(appCtx.DB, settings)
	root := ""
	if config.AppCfg != nil && strings.TrimSpace(config.AppCfg.Storage.DataDir) != "" {
		root = filepath.Join(config.AppCfg.Storage.DataDir, "knowledge")
	}
	emb := embedding.NewService(appCtx.DB)
	rootCtx := appCtx.Context
	if rootCtx == nil {
		rootCtx = context.Background()
	}
	workers := settings.IngestWorkers
	if workers <= 0 {
		workers = 1
	}
	readerWorkers := settings.ReaderWorkers
	if readerWorkers <= 0 {
		readerWorkers = 1
	}
	s := &service{
		repo: repo, storage: NewStorageWithRuntime(root, settings), parsers: NewParserRegistry(settings), embedding: emb, settings: settings, rootCtx: rootCtx,
		sem: make(chan struct{}, workers), readSem: make(chan struct{}, readerWorkers), inflight: map[string]struct{}{}, cancels: map[string]context.CancelFunc{},
	}
	s.indexer = NewIndexer(repo, emb, settings)
	s.retrieval = NewRetrievalRuntime(repo, emb, settings)
	// Durable recovery: queued/running jobs are resumed from their last committed unit.
	if jobs, err := repo.RecoverableJobs(settings.JobStaleAfter); err == nil {
		for i := range jobs {
			s.schedule(rootCtx, jobs[i].ID)
		}
	}
	go s.maintenanceLoop(rootCtx)
	return s
}

func (s *service) ListSources(tenantID string) ([]Source, error) { return s.repo.ListSources(tenantID) }
func (s *service) GetSource(tenantID, id string) (*Source, error) {
	return s.repo.GetSource(tenantID, id)
}

func (s *service) GetSourceDetail(tenantID, id string) (*SourceDetail, error) {
	src, err := s.repo.GetSource(tenantID, id)
	if err != nil {
		return nil, err
	}
	revisions, err := s.repo.ListSourceRevisions(tenantID, id)
	if err != nil {
		return nil, err
	}
	hasActiveJob, err := s.repo.HasActiveJobForSource(tenantID, id)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	for i := range revisions {
		revisions[i].IsActive = revisions[i].Revision == src.Revision
		revisions[i].CanRollback = !hasActiveJob && !revisions[i].IsActive && revisions[i].PurgedAt == nil &&
			(revisions[i].Status == RevisionStatusReady || revisions[i].Status == RevisionStatusReadyDegraded)
		revisions[i].CanPurge = !hasActiveJob && !revisions[i].IsActive && revisionPurgeEligible(revisions[i], now, s.revisionRetention())
	}
	latest, latestErr := s.repo.LatestJobForSource(tenantID, id)
	if errors.Is(latestErr, gorm.ErrRecordNotFound) {
		latest = nil
	} else if latestErr != nil {
		return nil, latestErr
	}
	units, err := s.repo.CountUnits(tenantID, id, src.Revision)
	if err != nil {
		return nil, err
	}
	chunks, err := s.repo.CountChunks(tenantID, id, src.Revision)
	if err != nil {
		return nil, err
	}
	health := "healthy"
	if src.Status == SourceStatusReadyDegraded || (chunks > 0 && src.IndexedChunks < chunks) {
		health = "degraded"
	}
	if src.Status == SourceStatusFailed || src.Status == SourceStatusCancelled {
		health = "unavailable"
	}
	return &SourceDetail{Source: *src, Revisions: revisions, LatestJob: latest, UnitCount: units, ChunkCount: chunks, IndexHealth: health, HasActiveJob: hasActiveJob}, nil
}
func (s *service) ListJobs(tenantID, sourceID string) ([]IngestJob, error) {
	return s.repo.ListJobs(tenantID, sourceID)
}
func (s *service) GetJob(tenantID, id string) (*IngestJob, error) { return s.repo.GetJob(tenantID, id) }
func (s *service) Search(ctx context.Context, tenantID string, req SearchRequest) (SearchResponse, error) {
	return s.retrieval.Search(ctx, tenantID, req)
}

func (s *service) RunBenchmark(ctx context.Context, tenantID string, req BenchmarkRequest) (BenchmarkResponse, error) {
	result, err := runRetrievalBenchmark(ctx, tenantID, req, s.settings, s.retrieval.Search)
	if err != nil {
		return result, err
	}
	if _, err := s.repo.RecordBenchmarkRun(tenantID, result.Summary); err != nil {
		return result, fmt.Errorf("persist benchmark summary: %w", err)
	}
	return result, nil
}

func (s *service) ListBenchmarkRuns(tenantID string, limit int) ([]BenchmarkRunSummary, error) {
	return s.repo.ListBenchmarkRuns(tenantID, limit)
}

func sourceTypeFor(filename, mime string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	if ext != "" {
		return ext
	}
	mime = strings.TrimSpace(strings.ToLower(mime))
	if mime != "" {
		return mime
	}
	return "file"
}

func (s *service) Upload(ctx context.Context, tenantID, logicalName, filename, mimeType string, body io.Reader) (*Source, *IngestJob, error) {
	tenantID = strings.TrimSpace(tenantID)
	filename = filepath.Base(strings.TrimSpace(filename))
	if tenantID == "" {
		return nil, nil, fmt.Errorf("tenant scope required")
	}
	if filename == "" || filename == "." {
		return nil, nil, fmt.Errorf("filename required")
	}
	if body == nil {
		return nil, nil, fmt.Errorf("file body required")
	}
	if strings.TrimSpace(logicalName) == "" {
		logicalName = filename
	}

	sourceID := uuid.NewString()
	stored, err := s.storage.Save(ctx, tenantID, sourceID, 1, filename, body)
	if err != nil {
		return nil, nil, err
	}

	if existing, findErr := s.repo.FindBySHA(tenantID, stored.SHA256); findErr == nil && existing != nil {
		_ = s.storage.Delete(stored.Key)
		job, _ := s.repo.LatestJobForSource(tenantID, existing.ID)
		return existing, job, nil
	} else if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
		_ = s.storage.Delete(stored.Key)
		return nil, nil, findErr
	}

	alias, err := s.repo.EnsureAlias(tenantID)
	if err != nil {
		_ = s.storage.Delete(stored.Key)
		return nil, nil, err
	}
	now := time.Now().UTC()
	src := &Source{
		ID: sourceID, TenantID: tenantID, LogicalName: strings.TrimSpace(logicalName), Filename: filename,
		MimeType: strings.TrimSpace(mimeType), SourceType: sourceTypeFor(filename, mimeType), StorageKey: stored.Key,
		SizeBytes: stored.SizeBytes, SHA256: stored.SHA256, Revision: 1, ParserVersion: ParserVersion,
		ChunkSchemaVersion: ChunkSchemaVersion, ConfigHash: s.settings.ConfigHash, EmbeddingVersion: "active", IndexVersion: alias.ActiveVersion,
		Status: SourceStatusPending, CreatedAt: now, UpdatedAt: now,
	}
	job := &IngestJob{
		ID: uuid.NewString(), TenantID: tenantID, SourceID: sourceID, SourceRevision: 1, Kind: JobKindIngest,
		Stage: "queued", Status: JobStatusQueued, CursorJSON: `{"ordinal":0}`, LastCommittedOrdinal: -1,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.CreateSource(src, job); err != nil {
		_ = s.storage.Delete(stored.Key)
		return nil, nil, err
	}
	s.schedule(s.rootCtx, job.ID)
	return src, job, nil
}

func (s *service) UploadRevision(ctx context.Context, tenantID, sourceID, filename, mimeType string, body io.Reader) (*SourceRevision, *IngestJob, error) {
	tenantID = strings.TrimSpace(tenantID)
	sourceID = strings.TrimSpace(sourceID)
	filename = filepath.Base(strings.TrimSpace(filename))
	if tenantID == "" || sourceID == "" {
		return nil, nil, fmt.Errorf("tenant and source required")
	}
	if filename == "" || filename == "." || body == nil {
		return nil, nil, fmt.Errorf("revision file required")
	}
	src, err := s.repo.GetSource(tenantID, sourceID)
	if err != nil {
		return nil, nil, err
	}
	active, err := s.repo.HasActiveJobForSource(tenantID, sourceID)
	if err != nil {
		return nil, nil, err
	}
	if active {
		return nil, nil, fmt.Errorf("source already has an active knowledge job")
	}
	latest, err := s.repo.LatestSourceRevision(tenantID, sourceID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, err
	}
	nextRevision := src.Revision + 1
	if latest != nil && latest.Revision >= nextRevision {
		nextRevision = latest.Revision + 1
	}
	stored, err := s.storage.Save(ctx, tenantID, sourceID, nextRevision, filename, body)
	if err != nil {
		return nil, nil, err
	}
	if same, findErr := s.repo.FindSourceRevisionBySHA(tenantID, sourceID, stored.SHA256); findErr == nil && same != nil {
		_ = s.storage.Delete(stored.Key)
		if same.Revision == src.Revision {
			job, _ := s.repo.LatestJobForSource(tenantID, sourceID)
			return same, job, nil
		}
		return nil, nil, fmt.Errorf("same content already exists as revision %d", same.Revision)
	} else if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
		_ = s.storage.Delete(stored.Key)
		return nil, nil, findErr
	}
	alias, err := s.repo.EnsureAlias(tenantID)
	if err != nil {
		_ = s.storage.Delete(stored.Key)
		return nil, nil, err
	}
	now := time.Now().UTC()
	revision := &SourceRevision{
		ID: sourceRevisionID(sourceID, nextRevision), TenantID: tenantID, SourceID: sourceID, Revision: nextRevision,
		Filename: filename, MimeType: strings.TrimSpace(mimeType), SourceType: sourceTypeFor(filename, mimeType), StorageKey: stored.Key,
		SizeBytes: stored.SizeBytes, SHA256: stored.SHA256, ParserVersion: ParserVersion, ChunkSchemaVersion: ChunkSchemaVersion, ConfigHash: s.settings.ConfigHash,
		EmbeddingVersion: "active", IndexVersion: alias.ActiveVersion, Status: RevisionStatusPending, CreatedAt: now, UpdatedAt: now,
	}
	job := &IngestJob{
		ID: uuid.NewString(), TenantID: tenantID, SourceID: sourceID, SourceRevision: nextRevision, Kind: JobKindIngest,
		Stage: "queued", Status: JobStatusQueued, CursorJSON: `{"ordinal":0}`, LastCommittedOrdinal: -1, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.CreateSourceRevision(revision, job); err != nil {
		_ = s.storage.Delete(stored.Key)
		return nil, nil, err
	}
	s.schedule(s.rootCtx, job.ID)
	return revision, job, nil
}

func (s *service) ReindexSource(ctx context.Context, tenantID, id string) (*IngestJob, error) {
	src, err := s.repo.GetSource(tenantID, id)
	if err != nil {
		return nil, err
	}
	if src.Status != SourceStatusReady && src.Status != SourceStatusReadyDegraded {
		return nil, fmt.Errorf("source must be ready before reindex")
	}
	now := time.Now().UTC()
	chunkCount, err := s.repo.CountChunks(tenantID, id, src.Revision)
	if err != nil {
		return nil, err
	}
	job := &IngestJob{
		ID: uuid.NewString(), TenantID: tenantID, SourceID: id, SourceRevision: src.Revision, Kind: JobKindReindex,
		Stage: "queued", Status: JobStatusQueued, CursorJSON: `{"ordinal":0}`, LastCommittedOrdinal: -1,
		ExpectedUnits: chunkCount, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.PrepareReindex(job, s.settings.QdrantBatchSize); err != nil {
		return nil, err
	}
	s.schedule(s.rootCtx, job.ID)
	return job, nil
}

func (s *service) RollbackSourceRevision(ctx context.Context, tenantID, sourceID string, revision int64) (*IngestJob, error) {
	tenantID = strings.TrimSpace(tenantID)
	sourceID = strings.TrimSpace(sourceID)
	if tenantID == "" || sourceID == "" || revision <= 0 {
		return nil, fmt.Errorf("tenant, source and positive revision required")
	}
	src, err := s.repo.GetSource(tenantID, sourceID)
	if err != nil {
		return nil, err
	}
	if src.Revision == revision {
		return nil, fmt.Errorf("revision %d is already active", revision)
	}
	active, err := s.repo.HasActiveJobForSource(tenantID, sourceID)
	if err != nil {
		return nil, err
	}
	if active {
		return nil, fmt.Errorf("source already has an active knowledge job")
	}
	target, err := s.repo.GetSourceRevision(tenantID, sourceID, revision)
	if err != nil {
		return nil, err
	}
	if target.PurgedAt != nil || target.Status == RevisionStatusPurged {
		return nil, fmt.Errorf("revision %d has been purged and cannot be rolled back", revision)
	}
	if target.Status != RevisionStatusReady && target.Status != RevisionStatusReadyDegraded {
		return nil, fmt.Errorf("revision %d is not rollback-ready", revision)
	}
	chunkCount, err := s.repo.CountChunks(tenantID, sourceID, revision)
	if err != nil {
		return nil, err
	}
	if chunkCount == 0 {
		return nil, fmt.Errorf("revision %d has no parsed chunks", revision)
	}
	now := time.Now().UTC()
	job := &IngestJob{
		ID: uuid.NewString(), TenantID: tenantID, SourceID: sourceID, SourceRevision: revision, Kind: JobKindRollback,
		Stage: "queued", Status: JobStatusQueued, CursorJSON: `{"ordinal":0}`, LastCommittedOrdinal: -1,
		ExpectedUnits: chunkCount, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.PrepareRevisionIndexJob(job, s.settings.QdrantBatchSize); err != nil {
		return nil, err
	}
	s.schedule(s.rootCtx, job.ID)
	return job, nil
}

func (s *service) PurgeSourceRevision(ctx context.Context, tenantID, sourceID string, revision int64) (*SourceRevision, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	purged, err := s.repo.PurgeSourceRevision(strings.TrimSpace(tenantID), strings.TrimSpace(sourceID), revision, time.Now().UTC(), s.revisionRetention())
	if err != nil {
		return nil, err
	}
	if purged != nil && strings.TrimSpace(purged.StorageKey) != "" {
		if err := s.storage.Delete(purged.StorageKey); err == nil {
			_ = s.repo.ClearPurgedRevisionStorageKey(tenantID, sourceID, revision)
		} else {
			_ = s.repo.UpdateSourceRevision(tenantID, sourceID, revision, map[string]any{"last_error": "storage_gc: " + err.Error()})
		}
	}
	return s.repo.GetSourceRevision(tenantID, sourceID, revision)
}

func (s *service) GarbageCollect(ctx context.Context, tenantID string) (int, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return 0, fmt.Errorf("tenant scope required")
	}
	now := time.Now().UTC()
	items, err := s.repo.ListRevisionGCEligible(tenantID, now, 200, s.revisionRetention())
	if err != nil {
		return 0, err
	}
	purgedCount := 0
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return purgedCount, err
		}
		if _, err := s.PurgeSourceRevision(ctx, tenantID, item.SourceID, item.Revision); err != nil {
			continue
		}
		purgedCount++
	}
	// Physical file deletion is intentionally retried independently of the DB
	// tombstone and vector outbox, because it is not transactional storage.
	leftovers, err := s.repo.ListPurgedRevisionsWithStorage(tenantID, 200)
	if err == nil {
		for _, item := range leftovers {
			if err := ctx.Err(); err != nil {
				return purgedCount, err
			}
			if err := s.storage.Delete(item.StorageKey); err == nil {
				_ = s.repo.ClearPurgedRevisionStorageKey(tenantID, item.SourceID, item.Revision)
			}
		}
	}
	return purgedCount, nil
}

func (s *service) DeleteSource(tenantID, id string) error {
	if _, err := s.repo.GetSource(tenantID, id); err != nil {
		return err
	}
	jobs, err := s.repo.ListJobs(tenantID, id)
	if err != nil {
		return err
	}
	if err := s.repo.SoftDeleteSource(tenantID, id, s.revisionRetention()); err != nil {
		return err
	}
	for i := range jobs {
		if jobs[i].Status != JobStatusQueued && jobs[i].Status != JobStatusRunning {
			continue
		}
		s.mu.Lock()
		cancel := s.cancels[jobs[i].ID]
		s.mu.Unlock()
		if cancel != nil {
			cancel()
		}
	}
	return nil
}

func (s *service) RetryJob(ctx context.Context, tenantID, id string) (*IngestJob, error) {
	job, err := s.repo.GetJob(tenantID, id)
	if err != nil {
		return nil, err
	}
	if job.Status == JobStatusRunning || job.Status == JobStatusQueued {
		return job, nil
	}
	if job.Status == JobStatusCompleted {
		return nil, fmt.Errorf("completed job cannot be retried; use source reindex")
	}
	if job.Status != JobStatusFailed && job.Status != JobStatusCancelled {
		return nil, fmt.Errorf("job status %s is not retryable", job.Status)
	}
	if _, err := s.repo.GetSource(tenantID, job.SourceID); err != nil {
		return nil, err
	}
	kind := strings.TrimSpace(job.Kind)
	if kind == "" {
		kind = JobKindIngest
	}
	prepared, err := s.repo.PrepareRetry(tenantID, id, job.SourceID, job.SourceRevision, kind)
	if err != nil {
		return nil, err
	}
	if prepared {
		s.schedule(s.rootCtx, id)
	}
	return s.repo.GetJob(tenantID, id)
}

func (s *service) CancelJob(ctx context.Context, tenantID, id string) (*IngestJob, error) {
	job, err := s.repo.GetJob(tenantID, id)
	if err != nil {
		return nil, err
	}
	if job.Status == JobStatusCompleted || job.Status == JobStatusFailed || job.Status == JobStatusCancelled {
		return job, nil
	}
	s.mu.Lock()
	cancel := s.cancels[id]
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	now := time.Now().UTC()
	if job.Kind == JobKindReindex || job.Kind == JobKindRollback {
		_, err = s.repo.CancelJobIfActive(tenantID, id, now)
	} else {
		src, srcErr := s.repo.GetSource(tenantID, job.SourceID)
		if srcErr != nil {
			return nil, srcErr
		}
		updateActiveSource := src.Revision == job.SourceRevision && src.Status != SourceStatusReady && src.Status != SourceStatusReadyDegraded
		_, err = s.repo.CancelIngestJobIfActive(tenantID, id, job.SourceID, job.SourceRevision, now, updateActiveSource, s.revisionRetention())
	}
	if err != nil {
		return nil, err
	}
	return s.repo.GetJob(tenantID, id)
}

func (s *service) schedule(ctx context.Context, jobID string) {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return
	}
	s.mu.Lock()
	if _, exists := s.inflight[jobID]; exists {
		s.mu.Unlock()
		return
	}
	jobCtx, cancel := context.WithCancel(ctx)
	s.inflight[jobID] = struct{}{}
	s.cancels[jobID] = cancel
	s.mu.Unlock()
	go func() {
		defer func() {
			cancel()
			s.mu.Lock()
			delete(s.inflight, jobID)
			delete(s.cancels, jobID)
			s.mu.Unlock()
		}()
		select {
		case s.sem <- struct{}{}:
			defer func() { <-s.sem }()
		case <-jobCtx.Done():
			return
		}
		s.processJob(jobCtx, jobID)
	}()
}

func (s *service) startJobHeartbeat(ctx context.Context, jobID string) func() {
	done := make(chan struct{})
	var once sync.Once
	go func() {
		interval := s.settings.JobHeartbeatInterval
		if interval <= 0 {
			interval = defaultRuntimeSettings().JobHeartbeatInterval
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case t := <-ticker.C:
				_ = s.repo.UpdateJob(jobID, map[string]any{"heartbeat_at": t.UTC()})
			}
		}
	}()
	return func() { once.Do(func() { close(done) }) }
}

func (s *service) processJob(ctx context.Context, jobID string) {
	now := time.Now().UTC()
	staleAfter := s.settings.JobStaleAfter
	if staleAfter <= 0 {
		staleAfter = defaultRuntimeSettings().JobStaleAfter
	}
	job, claimed, err := s.repo.ClaimJob(jobID, now, now.Add(-staleAfter))
	if err != nil || !claimed || job == nil {
		return
	}
	if strings.TrimSpace(job.Kind) == "" {
		job.Kind = JobKindIngest
	}
	src, err := s.repo.GetSource(job.TenantID, job.SourceID)
	if err != nil {
		s.failJob(job, nil, nil, err)
		return
	}
	switch job.Kind {
	case JobKindReindex:
		s.processReindexJob(ctx, job, src)
		return
	case JobKindRollback:
		s.processRollbackJob(ctx, job, src)
		return
	}

	revision, err := s.repo.GetSourceRevision(job.TenantID, job.SourceID, job.SourceRevision)
	if err != nil {
		s.failJob(job, src, nil, err)
		return
	}
	path, err := s.storage.Resolve(revision.StorageKey)
	if err != nil {
		s.failJob(job, src, revision, err)
		return
	}
	ref := SourceRef{Path: path, Filename: revision.Filename, MimeType: revision.MimeType}
	parser, err := s.parsers.For(ref)
	if err != nil {
		s.failJob(job, src, revision, err)
		return
	}

	_ = s.repo.UpdateJob(job.ID, map[string]any{"stage": "inspect", "heartbeat_at": now, "last_error": ""})
	_ = s.repo.UpdateSourceRevision(job.TenantID, job.SourceID, job.SourceRevision, map[string]any{"status": RevisionStatusProcessing, "last_error": ""})
	// A replacement revision is blue/green: while it is processing the current
	// active source stays ready and visible. Only initial ingestion exposes the
	// processing state through knowledge_sources.
	updateActiveProgress := src.Revision == job.SourceRevision && src.Status != SourceStatusReady && src.Status != SourceStatusReadyDegraded
	if updateActiveProgress {
		_ = s.repo.UpdateSource(job.TenantID, src.ID, map[string]any{"status": SourceStatusProcessing, "last_error": ""})
	}
	stopHeartbeat := s.startJobHeartbeat(ctx, job.ID)
	defer stopHeartbeat()

	info, err := parser.Inspect(ctx, ref)
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return
		}
		s.failJob(job, src, revision, err)
		return
	}
	_ = s.repo.UpdateJob(job.ID, map[string]any{"stage": "parse", "expected_units": info.EstimatedUnits})
	_ = s.repo.UpdateSourceRevision(job.TenantID, job.SourceID, job.SourceRevision, map[string]any{"expected_units": info.EstimatedUnits})
	if updateActiveProgress {
		_ = s.repo.UpdateSource(job.TenantID, src.ID, map[string]any{"expected_units": info.EstimatedUnits})
	}

	cursor := Cursor{Ordinal: job.LastCommittedOrdinal + 1}
	if cursor.Ordinal < 0 {
		cursor.Ordinal = 0
	}
	if job.LastCommittedOrdinal < 0 {
		if err := s.repo.ResetRevisionData(job.TenantID, src.ID, job.SourceRevision); err != nil {
			s.failJob(job, src, revision, err)
			return
		}
	}
	ordinal := cursor.Ordinal
	processed := job.ProcessedUnits
	parseErr := parser.StreamUnits(ctx, ref, cursor, func(raw RawUnit) error {
		content := cleanText(raw.Content)
		if content == "" {
			ordinal++
			return nil
		}
		created := time.Now().UTC()
		temporal := deriveTemporalFields(raw.Metadata, content)
		unit := DocumentUnit{
			ID: uuid.NewString(), TenantID: job.TenantID, SourceID: src.ID, SourceRevision: job.SourceRevision,
			Ordinal: ordinal, UnitType: raw.UnitType, Title: cleanText(raw.Title), Content: content,
			LocatorJSON: marshalMap(raw.Locator), MetadataJSON: marshalMap(raw.Metadata), Checksum: checksumText(content),
			ValidFrom: temporal.ValidFrom, ValidTo: temporal.ValidTo, SupersedesJSON: marshalStringList(temporal.Supersedes),
			ConflictGroup: temporal.ConflictGroup, CreatedAt: created,
		}
		chunks := buildChunks(job.TenantID, src.ID, job.SourceRevision, unit, revision.IndexVersion, s.settings)
		childIDs := make([]string, 0, len(chunks))
		for _, c := range chunks {
			if c.ChunkIndex >= 0 {
				childIDs = append(childIDs, c.ID)
			}
		}
		var outbox *IndexOutbox
		if len(childIDs) > 0 {
			payload, _ := json.Marshal(IndexPayload{ChunkIDs: childIDs, IndexVersion: revision.IndexVersion})
			outbox = &IndexOutbox{ID: uuid.NewString(), TenantID: job.TenantID, AggregateType: "knowledge_source", AggregateID: src.ID, SourceRevision: job.SourceRevision, Operation: "upsert", PayloadJSON: string(payload), Status: "pending", CreatedAt: created, UpdatedAt: created}
		}
		if err := s.repo.InsertUnitBundle(unit, chunks, outbox); err != nil {
			return err
		}
		processed++
		current := ordinal
		ordinal++
		checkpoint, _ := json.Marshal(Cursor{Ordinal: ordinal})
		heartbeat := time.Now().UTC()
		if err := s.repo.UpdateJob(job.ID, map[string]any{"last_committed_ordinal": current, "cursor_json": string(checkpoint), "processed_units": processed, "heartbeat_at": heartbeat}); err != nil {
			return err
		}
		if err := s.repo.UpdateSourceRevision(job.TenantID, src.ID, job.SourceRevision, map[string]any{"processed_units": processed}); err != nil {
			return err
		}
		if updateActiveProgress {
			return s.repo.UpdateSource(job.TenantID, src.ID, map[string]any{"processed_units": processed})
		}
		return nil
	})
	if parseErr != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return
		}
		s.failJob(job, src, revision, parseErr)
		return
	}
	if err := ctx.Err(); err != nil {
		return
	}

	_ = s.repo.UpdateJob(job.ID, map[string]any{"stage": "index"})
	indexed, degraded, warnings := s.indexer.ProcessSource(ctx, job.TenantID, src.ID, job.SourceRevision)
	if err := ctx.Err(); err != nil {
		return
	}
	childCount, countErr := s.repo.CountChildChunks(job.TenantID, src.ID, job.SourceRevision)
	if countErr != nil {
		s.failJob(job, src, revision, countErr)
		return
	}
	status := SourceStatusReady
	revisionStatus := RevisionStatusReady
	lastError := ""
	if degraded || indexed < childCount {
		status = SourceStatusReadyDegraded
		revisionStatus = RevisionStatusReadyDegraded
		if len(warnings) > 0 {
			lastError = strings.Join(warnings, " | ")
		}
	}
	finished := time.Now().UTC()
	sourceUpdates := map[string]any{
		"filename": revision.Filename, "mime_type": revision.MimeType, "source_type": revision.SourceType,
		"storage_key": revision.StorageKey, "size_bytes": revision.SizeBytes, "sha256": revision.SHA256,
		"revision": job.SourceRevision, "parser_version": revision.ParserVersion, "chunk_schema_version": revision.ChunkSchemaVersion, "config_hash": revision.ConfigHash,
		"embedding_version": revision.EmbeddingVersion, "index_version": revision.IndexVersion,
		"status": status, "expected_units": info.EstimatedUnits, "processed_units": processed,
		"failed_units": int64(0), "indexed_chunks": indexed, "last_error": lastError,
	}
	completed, completeErr := s.repo.FinalizeIngestJobIfRunning(job.TenantID, job.ID, src.ID, job.SourceRevision, map[string]any{
		"status": JobStatusCompleted, "stage": "completed", "processed_units": processed, "indexed_chunks": indexed,
		"last_error": lastError, "finished_at": finished, "heartbeat_at": finished,
	}, map[string]any{
		"status": revisionStatus, "expected_units": info.EstimatedUnits, "processed_units": processed,
		"failed_units": int64(0), "indexed_chunks": indexed, "last_error": lastError, "activated_at": finished,
	}, sourceUpdates, s.revisionRetention())
	if completeErr != nil || !completed {
		return
	}

}

func (s *service) processRollbackJob(ctx context.Context, job *IngestJob, src *Source) {
	if job == nil || src == nil {
		return
	}
	revision, err := s.repo.GetSourceRevision(job.TenantID, job.SourceID, job.SourceRevision)
	if err != nil {
		s.failReindexJob(job, err)
		return
	}
	if revision.PurgedAt != nil || revision.Status == RevisionStatusPurged {
		s.failReindexJob(job, fmt.Errorf("rollback target revision has been purged"))
		return
	}
	stopHeartbeat := s.startJobHeartbeat(ctx, job.ID)
	defer stopHeartbeat()
	_ = s.repo.UpdateJob(job.ID, map[string]any{"stage": "rollback_index", "last_error": ""})
	indexed, degraded, warnings := s.indexer.ProcessSource(ctx, job.TenantID, src.ID, job.SourceRevision)
	if errors.Is(ctx.Err(), context.Canceled) {
		return
	}
	childCount, err := s.repo.CountChildChunks(job.TenantID, src.ID, job.SourceRevision)
	if err != nil {
		s.failReindexJob(job, err)
		return
	}
	if childCount == 0 {
		s.failReindexJob(job, fmt.Errorf("rollback target contains no retrievable chunks"))
		return
	}
	status := SourceStatusReady
	revisionStatus := RevisionStatusReady
	lastError := ""
	if degraded || indexed < childCount {
		status = SourceStatusReadyDegraded
		revisionStatus = RevisionStatusReadyDegraded
		if len(warnings) > 0 {
			lastError = strings.Join(warnings, " | ")
		}
	}
	finished := time.Now().UTC()
	sourceUpdates := map[string]any{
		"filename": revision.Filename, "mime_type": revision.MimeType, "source_type": revision.SourceType,
		"storage_key": revision.StorageKey, "size_bytes": revision.SizeBytes, "sha256": revision.SHA256,
		"revision": revision.Revision, "parser_version": revision.ParserVersion, "chunk_schema_version": revision.ChunkSchemaVersion, "config_hash": revision.ConfigHash,
		"embedding_version": revision.EmbeddingVersion, "index_version": revision.IndexVersion,
		"status": status, "expected_units": revision.ExpectedUnits, "processed_units": revision.ProcessedUnits,
		"failed_units": revision.FailedUnits, "indexed_chunks": indexed, "last_error": lastError,
	}
	_, _ = s.repo.FinalizeIngestJobIfRunning(job.TenantID, job.ID, src.ID, job.SourceRevision, map[string]any{
		"status": JobStatusCompleted, "stage": "completed", "processed_units": childCount, "indexed_chunks": indexed,
		"last_error": lastError, "finished_at": finished, "heartbeat_at": finished,
	}, map[string]any{
		"status": revisionStatus, "indexed_chunks": indexed, "last_error": lastError,
		"activated_at": finished, "retain_until": nil,
	}, sourceUpdates, s.revisionRetention())
}

func (s *service) processReindexJob(ctx context.Context, job *IngestJob, src *Source) {
	if job == nil || src == nil {
		return
	}
	stopHeartbeat := s.startJobHeartbeat(ctx, job.ID)
	defer stopHeartbeat()
	_ = s.repo.UpdateJob(job.ID, map[string]any{"stage": "index", "last_error": ""})
	indexed, degraded, warnings := s.indexer.ProcessSource(ctx, job.TenantID, src.ID, job.SourceRevision)
	if errors.Is(ctx.Err(), context.Canceled) {
		return
	}
	childCount, err := s.repo.CountChildChunks(job.TenantID, src.ID, job.SourceRevision)
	if err != nil {
		s.failReindexJob(job, err)
		return
	}
	status := SourceStatusReady
	revisionStatus := RevisionStatusReady
	lastError := ""
	if degraded || indexed < childCount {
		status = SourceStatusReadyDegraded
		revisionStatus = RevisionStatusReadyDegraded
		if len(warnings) > 0 {
			lastError = strings.Join(warnings, " | ")
		}
	}
	finished := time.Now().UTC()
	sourceUpdates := map[string]any(nil)
	if src.Revision == job.SourceRevision {
		sourceUpdates = map[string]any{"status": status, "indexed_chunks": indexed, "last_error": lastError}
	}
	_, _ = s.repo.FinalizeIngestJobIfRunning(job.TenantID, job.ID, src.ID, job.SourceRevision, map[string]any{
		"status": JobStatusCompleted, "stage": "completed", "processed_units": childCount, "indexed_chunks": indexed,
		"last_error": lastError, "finished_at": finished, "heartbeat_at": finished,
	}, map[string]any{"status": revisionStatus, "indexed_chunks": indexed, "last_error": lastError}, sourceUpdates, s.revisionRetention())
}

func (s *service) failReindexJob(job *IngestJob, err error) {
	if job == nil || err == nil {
		return
	}
	now := time.Now().UTC()
	_, _ = s.repo.FailJobIfRunning(job.ID, map[string]any{
		"status": JobStatusFailed, "stage": "failed", "last_error": err.Error(), "finished_at": now, "heartbeat_at": now,
	})
}

func (s *service) failJob(job *IngestJob, src *Source, revision *SourceRevision, err error) {
	if job == nil || err == nil {
		return
	}
	now := time.Now().UTC()
	if revision == nil {
		_, _ = s.repo.FailJobIfRunning(job.ID, map[string]any{"status": JobStatusFailed, "stage": "failed", "last_error": err.Error(), "finished_at": now, "heartbeat_at": now})
		return
	}
	var sourceUpdates map[string]any
	if src != nil && src.Revision == job.SourceRevision && src.Status != SourceStatusReady && src.Status != SourceStatusReadyDegraded {
		sourceUpdates = map[string]any{"status": SourceStatusFailed, "last_error": err.Error()}
	}
	_, _ = s.repo.FinalizeIngestJobIfRunning(job.TenantID, job.ID, job.SourceID, job.SourceRevision, map[string]any{
		"status": JobStatusFailed, "stage": "failed", "last_error": err.Error(), "finished_at": now, "heartbeat_at": now,
	}, map[string]any{"status": RevisionStatusFailed, "last_error": err.Error(), "retain_until": now.Add(s.revisionRetention())}, sourceUpdates, s.revisionRetention())
}

func (s *service) revisionRetention() time.Duration {
	retention := s.settings.RevisionRetention
	if retention <= 0 {
		retention = defaultRuntimeSettings().RevisionRetention
	}
	return retention
}

func (s *service) maintenanceLoop(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	// Delay the first maintenance pass slightly so application bootstrap and DB
	// migrations can settle before background reconciliation starts.
	initialDelay := s.settings.MaintenanceInitialDelay
	if initialDelay < 0 {
		initialDelay = 0
	}
	timer := time.NewTimer(initialDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}
	for {
		tenantIDs, err := s.repo.ListKnowledgeTenantIDs()
		if err == nil {
			for _, tenantID := range tenantIDs {
				if ctx.Err() != nil {
					return
				}
				passTimeout := s.settings.MaintenanceTenantPassTimeout
				if passTimeout <= 0 {
					passTimeout = defaultRuntimeSettings().MaintenanceTenantPassTimeout
				}
				passCtx, cancel := context.WithTimeout(ctx, passTimeout)
				_ = s.reconcileTenant(passCtx, tenantID, s.settings.MaxOutboxesPerPass)
				cancel()
			}
		}
		interval := s.settings.MaintenanceInterval
		if interval <= 0 {
			interval = defaultRuntimeSettings().MaintenanceInterval
		}
		ticker := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			ticker.Stop()
			return
		case <-ticker.C:
		}
	}
}

func (s *service) ListRetrievalTraces(tenantID string, limit int) ([]RetrievalTraceSummary, error) {
	return s.repo.ListRetrievalTraces(tenantID, limit)
}

func (s *service) GetRuntimeStatus() RuntimeStatus {
	settings := s.settings
	return RuntimeStatus{
		ConfigHash:                   settings.ConfigHash,
		RetrievalPolicyEnabled:       settings.RetrievalPolicyEnabled,
		RetrievalPolicyShadow:        settings.RetrievalPolicyShadow,
		CanonicalReaderEnabled:       settings.CanonicalReaderEnabled,
		AgenticInvestigationEnabled:  settings.AgenticInvestigationEnabled,
		ReaderWorkers:                settings.ReaderWorkers,
		RetrievalTraceRetentionHours: int64(settings.RetrievalTraceRetention / time.Hour),
		BenchmarkRetentionHours:      int64(settings.BenchmarkRetention / time.Hour),
		DiagnosticCleanupBatchSize:   settings.DiagnosticCleanupBatchSize,
	}
}

func (s *service) GetSLOSummary(tenantID string) (RetrievalSLOSummary, error) {
	window := s.settings.SLOWindow
	if window <= 0 {
		window = time.Hour
	}
	minSamples := s.settings.SLOMinSamples
	if minSamples <= 0 {
		minSamples = 1
	}
	targets := map[string]time.Duration{
		RetrievalLevelFast:    s.settings.FastSLOTarget,
		RetrievalLevelDeep:    s.settings.DeepSLOTarget,
		RetrievalLevelAgentic: s.settings.AgenticSLOTarget,
	}
	return s.repo.RetrievalSLOSummary(tenantID, s.settings.ConfigHash, window, minSamples, targets)
}

func (s *service) Reconcile(ctx context.Context, tenantID string) error {
	maxOutboxes := s.settings.ManualMaxOutboxes
	if maxOutboxes <= 0 {
		maxOutboxes = defaultRuntimeSettings().ManualMaxOutboxes
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := s.repo.RequeueExhaustedVectorDeletes(tenantID, maxOutboxes, time.Now().UTC()); err != nil {
		return err
	}
	if err := s.reconcileTenant(ctx, tenantID, maxOutboxes); err != nil {
		return err
	}
	health, err := s.GetIndexOutboxHealth(ctx, tenantID)
	if err != nil {
		return err
	}
	if health.PendingUpserts > 0 || health.PendingDeletes > 0 || health.ExhaustedDeletes > 0 || health.RetryingDeletes > 0 || health.ExhaustedUpserts > 0 || health.RetryingUpserts > 0 || health.ActiveLeases > 0 {
		return fmt.Errorf("knowledge reconcile incomplete: pending=%d, retrying=%d, exhausted=%d, active leases=%d", health.PendingUpserts+health.PendingDeletes, health.RetryingUpserts+health.RetryingDeletes, health.ExhaustedUpserts+health.ExhaustedDeletes, health.ActiveLeases)
	}
	return nil
}

func (s *service) reconcileTenant(ctx context.Context, tenantID string, maxOutboxes int) error {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return fmt.Errorf("tenant scope required")
	}
	if maxOutboxes <= 0 {
		maxOutboxes = defaultRuntimeSettings().MaxOutboxesPerPass
	}
	jobs, err := s.repo.RecoverableJobsForTenant(tenantID, s.settings.JobStaleAfter)
	if err != nil {
		return err
	}
	for i := range jobs {
		s.schedule(s.rootCtx, jobs[i].ID)
	}
	// GC is canonical-first and only selects revisions outside their rollback
	// retention window. Vector tombstones emitted here are leased and drained
	// below; Qdrant downtime therefore cannot make the canonical purge partial.
	if _, err := s.GarbageCollect(ctx, tenantID); err != nil {
		return err
	}
	if err := s.cleanupDiagnosticHistory(ctx, tenantID); err != nil {
		return err
	}
	for processed := 0; processed < maxOutboxes; processed++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		lease := s.settings.OutboxLease
		if lease <= 0 {
			lease = defaultRuntimeSettings().OutboxLease
		}
		ob, claimed, claimErr := s.repo.ClaimNextOutboxForTenant(tenantID, time.Now().UTC(), lease)
		if claimErr != nil {
			return claimErr
		}
		if !claimed || ob == nil {
			break
		}
		if err := s.indexer.processClaimedOutbox(ctx, *ob, lease); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				_, _ = s.repo.FinishOutboxLease(tenantID, ob.ID, ob.LeaseToken, map[string]any{
					"status": "pending", "last_error": err.Error(), "next_attempt_at": time.Now().UTC(),
				})
				return err
			}
			next := time.Now().UTC().Add(s.settings.outboxRetryDelay(ob.RetryCount))
			_, _ = s.repo.FinishOutboxLease(tenantID, ob.ID, ob.LeaseToken, map[string]any{
				"status": "failed", "retry_count": ob.RetryCount + 1, "last_error": err.Error(), "next_attempt_at": next,
			})
			continue
		}
		changed, finishErr := s.repo.FinishOutboxLease(tenantID, ob.ID, ob.LeaseToken, map[string]any{"status": "done", "last_error": "", "next_attempt_at": nil})
		if finishErr != nil {
			return finishErr
		}
		if changed {
			s.refreshSourceIndexState(ob.TenantID, ob.AggregateID)
		}
	}
	return nil
}

func (s *service) cleanupDiagnosticHistory(ctx context.Context, tenantID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	batch := s.settings.DiagnosticCleanupBatchSize
	if batch <= 0 {
		batch = defaultRuntimeSettings().DiagnosticCleanupBatchSize
	}
	traceRetention := s.settings.RetrievalTraceRetention
	if traceRetention <= 0 {
		traceRetention = defaultRuntimeSettings().RetrievalTraceRetention
	}
	benchmarkRetention := s.settings.BenchmarkRetention
	if benchmarkRetention <= 0 {
		benchmarkRetention = defaultRuntimeSettings().BenchmarkRetention
	}
	now := time.Now().UTC()
	if _, err := s.repo.DeleteRetrievalTracesBefore(tenantID, now.Add(-traceRetention), batch); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := s.repo.DeleteBenchmarkRunsBefore(tenantID, now.Add(-benchmarkRetention), batch)
	return err
}

func (s *service) refreshSourceIndexState(tenantID, sourceID string) {
	src, err := s.repo.GetSource(tenantID, sourceID)
	if err != nil || src == nil || src.Status == SourceStatusDeleted {
		return
	}
	childCount, err := s.repo.CountChildChunks(tenantID, sourceID, src.Revision)
	if err != nil {
		return
	}
	indexed, err := s.repo.CountIndexedChunks(tenantID, sourceID, src.Revision)
	if err != nil {
		return
	}
	updates := map[string]any{"indexed_chunks": indexed}
	revisionUpdates := map[string]any{"indexed_chunks": indexed}
	if src.Status == SourceStatusReadyDegraded && indexed >= childCount {
		updates["status"] = SourceStatusReady
		updates["last_error"] = ""
		revisionUpdates["status"] = RevisionStatusReady
		revisionUpdates["last_error"] = ""
	}
	_ = s.repo.UpdateSource(tenantID, sourceID, updates)
	_ = s.repo.UpdateSourceRevision(tenantID, sourceID, src.Revision, revisionUpdates)
	if job, err := s.repo.LatestJobForSource(tenantID, sourceID); err == nil && job != nil {
		_ = s.repo.UpdateJob(job.ID, map[string]any{"indexed_chunks": indexed})
	}
}
