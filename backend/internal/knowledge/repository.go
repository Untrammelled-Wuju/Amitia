package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"gorm.io/gorm"
	amitialog "log"
)

type Repository struct {
	db       *gorm.DB
	settings runtimeSettings
}

func NewRepository(db *gorm.DB) *Repository {
	return NewRepositoryWithSettings(db, loadRuntimeSettings())
}

func NewRepositoryWithSettings(db *gorm.DB, settings runtimeSettings) *Repository {
	return &Repository{db: db, settings: settings}
}

func (r *Repository) DB() *gorm.DB { return r.db }
func tenantScope(db *gorm.DB, tenantID string) *gorm.DB {
	return db.Where("tenant_id = ?", strings.TrimSpace(tenantID))
}

func sourceRevisionID(sourceID string, revision int64) string {
	return strings.TrimSpace(sourceID) + ":r" + fmt.Sprintf("%d", revision)
}

func sourceToRevision(src *Source, status string) SourceRevision {
	if status == "" {
		status = RevisionStatusPending
	}
	return SourceRevision{
		ID: sourceRevisionID(src.ID, src.Revision), TenantID: src.TenantID, SourceID: src.ID, Revision: src.Revision,
		Filename: src.Filename, MimeType: src.MimeType, SourceType: src.SourceType, StorageKey: src.StorageKey,
		SizeBytes: src.SizeBytes, SHA256: src.SHA256, ParserVersion: src.ParserVersion, ChunkSchemaVersion: src.ChunkSchemaVersion, ConfigHash: src.ConfigHash,
		EmbeddingVersion: src.EmbeddingVersion, IndexVersion: src.IndexVersion, Status: status,
		ExpectedUnits: src.ExpectedUnits, ProcessedUnits: src.ProcessedUnits, FailedUnits: src.FailedUnits, IndexedChunks: src.IndexedChunks,
		LastError: src.LastError, CreatedAt: src.CreatedAt, UpdatedAt: src.UpdatedAt,
	}
}

func (r *Repository) CreateSource(src *Source, job *IngestJob) error {
	revision := sourceToRevision(src, RevisionStatusPending)
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(src).Error; err != nil {
			return err
		}
		if err := tx.Create(&revision).Error; err != nil {
			return err
		}
		return tx.Create(job).Error
	})
}
func (r *Repository) GetSource(tenantID, id string) (*Source, error) {
	var x Source
	err := tenantScope(r.db.Model(&Source{}), tenantID).Where("id = ? AND deleted_at IS NULL", id).First(&x).Error
	return &x, err
}
func (r *Repository) GetSourceRevision(tenantID, sourceID string, revision int64) (*SourceRevision, error) {
	var x SourceRevision
	err := tenantScope(r.db.Model(&SourceRevision{}), tenantID).
		Where("source_id = ? AND revision = ?", strings.TrimSpace(sourceID), revision).First(&x).Error
	return &x, err
}

func (r *Repository) ListSourceRevisions(tenantID, sourceID string) ([]SourceRevision, error) {
	var xs []SourceRevision
	err := tenantScope(r.db.Model(&SourceRevision{}), tenantID).
		Where("source_id = ?", strings.TrimSpace(sourceID)).Order("revision DESC").Find(&xs).Error
	if xs == nil {
		xs = []SourceRevision{}
	}
	return xs, err
}

func (r *Repository) LatestSourceRevision(tenantID, sourceID string) (*SourceRevision, error) {
	var x SourceRevision
	err := tenantScope(r.db.Model(&SourceRevision{}), tenantID).
		Where("source_id = ?", strings.TrimSpace(sourceID)).Order("revision DESC").First(&x).Error
	return &x, err
}

func (r *Repository) FindSourceRevisionBySHA(tenantID, sourceID, sha string) (*SourceRevision, error) {
	var x SourceRevision
	err := tenantScope(r.db.Model(&SourceRevision{}), tenantID).
		Where("source_id = ? AND sha256 = ?", strings.TrimSpace(sourceID), strings.TrimSpace(sha)).
		Order("revision DESC").First(&x).Error
	return &x, err
}

func (r *Repository) CreateSourceRevision(revision *SourceRevision, job *IngestJob) error {
	if revision == nil || job == nil {
		return errors.New("revision and job required")
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tenantScope(tx.Model(&Source{}), revision.TenantID).
			Where("id = ? AND deleted_at IS NULL", revision.SourceID).Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Create(revision).Error; err != nil {
			return err
		}
		return tx.Create(job).Error
	})
}

func (r *Repository) UpdateSourceRevision(tenantID, sourceID string, revision int64, updates map[string]any) error {
	updates = cloneUpdates(updates)
	updates["updated_at"] = time.Now().UTC()
	res := tenantScope(r.db.Model(&SourceRevision{}), tenantID).
		Where("source_id = ? AND revision = ?", sourceID, revision).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func revisionPurgeEligible(target SourceRevision, now time.Time, retention ...time.Duration) bool {
	window := DefaultRevisionRetention
	if len(retention) > 0 && retention[0] > 0 {
		window = retention[0]
	}
	if target.PurgedAt != nil || target.Status == RevisionStatusPurged {
		return false
	}
	if target.RetainUntil != nil {
		return !target.RetainUntil.After(now)
	}
	if target.Status == RevisionStatusFailed || target.Status == RevisionStatusCancelled {
		return !target.UpdatedAt.Add(window).After(now)
	}
	return false
}

func (r *Repository) ListRevisionGCEligible(tenantID string, now time.Time, limit int, retention ...time.Duration) ([]SourceRevision, error) {
	if limit <= 0 {
		limit = 100
	}
	window := DefaultRevisionRetention
	if len(retention) > 0 && retention[0] > 0 {
		window = retention[0]
	}
	cutoff := now.Add(-window)
	var xs []SourceRevision
	err := tenantScope(r.db.Model(&SourceRevision{}), tenantID).
		Where("purged_at IS NULL").
		Where(`NOT EXISTS (
			SELECT 1 FROM knowledge_sources ks
			WHERE ks.tenant_id = knowledge_source_revisions.tenant_id
			  AND ks.id = knowledge_source_revisions.source_id
			  AND ks.revision = knowledge_source_revisions.revision
			  AND ks.deleted_at IS NULL
		)`).
		Where("(retain_until IS NOT NULL AND retain_until <= ?) OR (retain_until IS NULL AND status IN ? AND updated_at <= ?)", now, []string{RevisionStatusFailed, RevisionStatusCancelled}, cutoff).
		Order("COALESCE(retain_until, updated_at) ASC").Limit(limit).Find(&xs).Error
	if xs == nil {
		xs = []SourceRevision{}
	}
	return xs, err
}

func (r *Repository) ListPurgedRevisionsWithStorage(tenantID string, limit int) ([]SourceRevision, error) {
	if limit <= 0 {
		limit = 100
	}
	var xs []SourceRevision
	err := tenantScope(r.db.Model(&SourceRevision{}), tenantID).
		Where("purged_at IS NOT NULL AND TRIM(storage_key) <> ''").
		Order("purged_at ASC").Limit(limit).Find(&xs).Error
	if xs == nil {
		xs = []SourceRevision{}
	}
	return xs, err
}

func (r *Repository) ClearPurgedRevisionStorageKey(tenantID, sourceID string, revision int64) error {
	res := tenantScope(r.db.Model(&SourceRevision{}), tenantID).
		Where("source_id = ? AND revision = ? AND purged_at IS NOT NULL", sourceID, revision).
		Updates(map[string]any{"storage_key": "", "updated_at": time.Now().UTC()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// PurgeSourceRevision removes canonical parsed data only after the revision is
// outside its rollback window and is not active. Vector removal is emitted as
// a durable outbox delete so Qdrant outages cannot make GC partially durable.
func (r *Repository) PurgeSourceRevision(tenantID, sourceID string, revision int64, now time.Time, retention ...time.Duration) (*SourceRevision, error) {
	var purged SourceRevision
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var src Source
		if err := tenantScope(tx.Model(&Source{}), tenantID).
			Where("id = ?", sourceID).First(&src).Error; err != nil {
			return err
		}
		if src.DeletedAt == nil && src.Revision == revision {
			return fmt.Errorf("active revision cannot be purged")
		}
		var target SourceRevision
		if err := tenantScope(tx.Model(&SourceRevision{}), tenantID).
			Where("source_id = ? AND revision = ?", sourceID, revision).First(&target).Error; err != nil {
			return err
		}
		if target.PurgedAt != nil || target.Status == RevisionStatusPurged {
			purged = target
			return nil
		}
		if !revisionPurgeEligible(target, now, retention...) {
			return fmt.Errorf("revision is still inside the rollback retention window")
		}
		var activeJobs int64
		if err := tenantScope(tx.Model(&IngestJob{}), tenantID).
			Where("source_id = ? AND source_revision = ? AND status IN ?", sourceID, revision, []string{JobStatusQueued, JobStatusRunning}).
			Count(&activeJobs).Error; err != nil {
			return err
		}
		if activeJobs > 0 {
			return fmt.Errorf("revision has an active knowledge job")
		}
		var chunks []Chunk
		if err := tenantScope(tx.Model(&Chunk{}), tenantID).
			Where("source_id = ? AND source_revision = ? AND chunk_index >= 0", sourceID, revision).
			Find(&chunks).Error; err != nil {
			return err
		}
		ids := make([]string, 0, len(chunks))
		for _, chunk := range chunks {
			ids = append(ids, chunk.ID)
		}
		if err := tenantScope(tx.Model(&IndexOutbox{}), tenantID).
			Where("aggregate_type = 'knowledge_source' AND aggregate_id = ? AND source_revision = ?", sourceID, revision).
			Where("(operation <> 'delete' OR status = 'done')").
			Where("NOT (operation = 'upsert' AND lease_until > ? AND status IN ?)", now, []string{"pending", "failed"}).
			Delete(&IndexOutbox{}).Error; err != nil {
			return err
		}
		if err := r.enqueueVectorDeleteOutboxes(tx, tenantID, sourceID, revision, ids, indexVersionsForDelete(tx, tenantID, target.IndexVersion), now); err != nil {
			return err
		}
		if err := tenantScope(tx.Model(&Chunk{}), tenantID).
			Where("source_id = ? AND source_revision = ?", sourceID, revision).Delete(&Chunk{}).Error; err != nil {
			return err
		}
		if err := tenantScope(tx.Model(&DocumentUnit{}), tenantID).
			Where("source_id = ? AND source_revision = ?", sourceID, revision).Delete(&DocumentUnit{}).Error; err != nil {
			return err
		}
		if err := tenantScope(tx.Model(&SourceRevision{}), tenantID).
			Where("source_id = ? AND revision = ? AND purged_at IS NULL", sourceID, revision).
			Updates(map[string]any{
				"status": RevisionStatusPurged, "expected_units": 0, "processed_units": 0,
				"failed_units": 0, "indexed_chunks": 0, "last_error": "", "purged_at": now, "updated_at": now,
			}).Error; err != nil {
			return err
		}
		target.Status = RevisionStatusPurged
		target.ExpectedUnits = 0
		target.ProcessedUnits = 0
		target.FailedUnits = 0
		target.IndexedChunks = 0
		target.PurgedAt = &now
		target.UpdatedAt = now
		purged = target
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &purged, nil
}

func (r *Repository) HasActiveJobForSource(tenantID, sourceID string) (bool, error) {
	var count int64
	err := tenantScope(r.db.Model(&IngestJob{}), tenantID).
		Where("source_id = ? AND status IN ?", sourceID, []string{JobStatusQueued, JobStatusRunning}).Count(&count).Error
	return count > 0, err
}
func (r *Repository) FindBySHA(tenantID, sha string) (*Source, error) {
	var x Source
	err := tenantScope(r.db.Model(&Source{}), tenantID).Where("sha256 = ? AND deleted_at IS NULL", sha).Order("created_at DESC").First(&x).Error
	return &x, err
}

func (r *Repository) ValidateSourceIDs(tenantID string, sourceIDs []string) ([]string, error) {
	cleaned := make([]string, 0, len(sourceIDs))
	seen := map[string]struct{}{}
	for _, id := range sourceIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		cleaned = append(cleaned, id)
	}
	if len(cleaned) == 0 {
		return []string{}, nil
	}
	var existing []string
	if err := r.db.Model(&Source{}).
		Where("tenant_id = ? AND id IN ? AND deleted_at IS NULL", tenantID, cleaned).
		Pluck("id", &existing).Error; err != nil {
		return nil, err
	}
	if len(existing) != len(cleaned) {
		return nil, fmt.Errorf("sourceIds contain unavailable source")
	}
	allowed := make(map[string]struct{}, len(existing))
	for _, id := range existing {
		allowed[id] = struct{}{}
	}
	ordered := make([]string, 0, len(cleaned))
	for _, id := range cleaned {
		if _, ok := allowed[id]; ok {
			ordered = append(ordered, id)
		}
	}
	return ordered, nil
}

func (r *Repository) ListSources(tenantID string) ([]Source, error) {
	var xs []Source
	err := tenantScope(r.db.Model(&Source{}), tenantID).Where("deleted_at IS NULL").Order("created_at DESC").Find(&xs).Error
	if xs == nil {
		xs = []Source{}
	}
	return xs, err
}
func (r *Repository) UpdateSource(tenantID, id string, updates map[string]any) error {
	updates["updated_at"] = time.Now().UTC()
	res := tenantScope(r.db.Model(&Source{}), tenantID).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func indexVersionsForDelete(tx *gorm.DB, tenantID, revisionIndexVersion string) []string {
	seen := map[string]struct{}{}
	versions := make([]string, 0, 3)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		versions = append(versions, value)
	}
	add(revisionIndexVersion)
	var alias IndexAlias
	if err := tx.Where("tenant_id = ?", tenantID).First(&alias).Error; err == nil {
		add(alias.ActiveVersion)
		add(alias.PreviousVersion)
	}
	if len(versions) == 0 {
		add(DefaultIndexVersion)
	}
	return versions
}

func (r *Repository) enqueueVectorDeleteOutboxes(tx *gorm.DB, tenantID, sourceID string, revision int64, chunkIDs []string, indexVersions []string, now time.Time) error {
	if len(chunkIDs) == 0 {
		return nil
	}
	batchSize := r.settings.QdrantBatchSize
	if batchSize <= 0 {
		batchSize = defaultRuntimeSettings().QdrantBatchSize
	}
	for _, version := range indexVersions {
		for start := 0; start < len(chunkIDs); start += batchSize {
			end := start + batchSize
			if end > len(chunkIDs) {
				end = len(chunkIDs)
			}
			payload, _ := json.Marshal(IndexPayload{ChunkIDs: append([]string(nil), chunkIDs[start:end]...), IndexVersion: version})
			row := IndexOutbox{
				ID: uuid.NewString(), TenantID: tenantID, AggregateType: "knowledge_source", AggregateID: sourceID,
				SourceRevision: revision, Operation: "delete", PayloadJSON: string(payload), Status: "pending",
				CreatedAt: now, UpdatedAt: now,
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Repository) enqueueSupersededVectorDeletes(tx *gorm.DB, tenantID, sourceID string, revision int64, now time.Time) error {
	var chunks []Chunk
	if err := tenantScope(tx.Model(&Chunk{}), tenantID).
		Select("id, index_version").
		Where("source_id = ? AND source_revision = ? AND chunk_index >= 0", sourceID, revision).
		Find(&chunks).Error; err != nil {
		return err
	}
	if len(chunks) == 0 {
		return nil
	}
	var sourceRevision SourceRevision
	if err := tenantScope(tx.Model(&SourceRevision{}), tenantID).
		Where("source_id = ? AND revision = ?", sourceID, revision).
		First(&sourceRevision).Error; err != nil {
		return err
	}
	versions := indexVersionsForDelete(tx, tenantID, sourceRevision.IndexVersion)
	seen := make(map[string]struct{}, len(versions))
	for _, version := range versions {
		seen[version] = struct{}{}
	}
	ids := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		ids = append(ids, chunk.ID)
		version := strings.TrimSpace(chunk.IndexVersion)
		if version != "" {
			if _, ok := seen[version]; !ok {
				seen[version] = struct{}{}
				versions = append(versions, version)
			}
		}
	}
	return r.enqueueVectorDeleteOutboxes(tx, tenantID, sourceID, revision, ids, versions, now)
}

func (r *Repository) SoftDeleteSource(tenantID, id string, retention ...time.Duration) error {
	now := time.Now().UTC()
	window := DefaultRevisionRetention
	if len(retention) > 0 && retention[0] > 0 {
		window = retention[0]
	}
	retainUntil := now.Add(window)
	return r.db.Transaction(func(tx *gorm.DB) error {
		var src Source
		if err := tenantScope(tx.Model(&Source{}), tenantID).
			Where("id = ? AND deleted_at IS NULL", id).First(&src).Error; err != nil {
			return err
		}
		res := tenantScope(tx.Model(&Source{}), tenantID).Where("id = ? AND deleted_at IS NULL", id).
			Updates(map[string]any{"status": SourceStatusDeleted, "deleted_at": now, "updated_at": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		if err := tenantScope(tx.Model(&SourceRevision{}), tenantID).
			Where("source_id = ? AND purged_at IS NULL", id).
			Updates(map[string]any{"retain_until": retainUntil, "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tenantScope(tx.Model(&IngestJob{}), tenantID).
			Where("source_id = ? AND status IN ?", id, []string{JobStatusQueued, JobStatusRunning}).
			Updates(map[string]any{
				"status": JobStatusCancelled, "stage": "cancelled", "last_error": "source deleted",
				"finished_at": now, "heartbeat_at": now, "updated_at": now,
			}).Error; err != nil {
			return err
		}

		// Any queued/failed upsert can race the tombstone and recreate stale
		// vectors after deletion. Remove outstanding work first; in-flight workers
		// are fenced by their lease token and the delete outboxes below are the
		// durable final state.
		if err := tenantScope(tx.Model(&IndexOutbox{}), tenantID).
			Where("aggregate_type = 'knowledge_source' AND aggregate_id = ? AND operation = 'upsert' AND status IN ?", id, []string{"pending", "failed"}).
			Where("(lease_until IS NULL OR lease_until <= ?)", now).
			Delete(&IndexOutbox{}).Error; err != nil {
			return err
		}
		var revisions []SourceRevision
		if err := tenantScope(tx.Model(&SourceRevision{}), tenantID).
			Where("source_id = ? AND purged_at IS NULL", id).Find(&revisions).Error; err != nil {
			return err
		}
		for _, revision := range revisions {
			var chunks []Chunk
			if err := tenantScope(tx.Model(&Chunk{}), tenantID).
				Where("source_id = ? AND source_revision = ? AND chunk_index >= 0", id, revision.Revision).
				Find(&chunks).Error; err != nil {
				return err
			}
			ids := make([]string, 0, len(chunks))
			for _, chunk := range chunks {
				ids = append(ids, chunk.ID)
			}
			if err := r.enqueueVectorDeleteOutboxes(tx, tenantID, id, revision.Revision, ids, indexVersionsForDelete(tx, tenantID, revision.IndexVersion), now); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repository) GetJobUnscoped(id string) (*IngestJob, error) {
	var x IngestJob
	err := r.db.Model(&IngestJob{}).Where("id = ?", id).First(&x).Error
	return &x, err
}

func (r *Repository) GetJob(tenantID, id string) (*IngestJob, error) {
	var x IngestJob
	err := tenantScope(r.db.Model(&IngestJob{}), tenantID).Where("id = ?", id).First(&x).Error
	return &x, err
}

// ClaimJob acquires the durable worker lease for a queued job or a stale
// running job. The conditional UPDATE is the cross-process exclusion point:
// only one worker can move/refresh a row whose heartbeat is stale. In-memory
// inflight tracking alone is not sufficient when Amitia runs multiple backend
// processes against the same canonical database.
func (r *Repository) ClaimJob(id string, now time.Time, staleBefore time.Time) (*IngestJob, bool, error) {
	updates := map[string]any{
		"status":       JobStatusRunning,
		"stage":        "starting",
		"heartbeat_at": now,
		"finished_at":  nil,
		"updated_at":   now,
	}
	res := r.db.Model(&IngestJob{}).
		Where("id = ?", strings.TrimSpace(id)).
		Where("status = ? OR (status = ? AND (heartbeat_at IS NULL OR heartbeat_at < ?))", JobStatusQueued, JobStatusRunning, staleBefore).
		Where("EXISTS (SELECT 1 FROM knowledge_sources ks WHERE ks.tenant_id = knowledge_ingest_jobs.tenant_id AND ks.id = knowledge_ingest_jobs.source_id AND ks.deleted_at IS NULL)").
		Updates(updates)
	if res.Error != nil {
		return nil, false, res.Error
	}
	if res.RowsAffected != 1 {
		return nil, false, nil
	}
	job, err := r.GetJobUnscoped(id)
	if err != nil {
		return nil, false, err
	}
	return job, true, nil
}

// CancelJobIfActive is intentionally status-conditional. Without this guard a
// request racing with normal completion could turn a completed job back into a
// cancelled job and leave source/job state inconsistent.
func (r *Repository) CancelJobIfActive(tenantID, id string, now time.Time) (bool, error) {
	res := tenantScope(r.db.Model(&IngestJob{}), tenantID).
		Where("id = ? AND status IN ?", id, []string{JobStatusQueued, JobStatusRunning}).
		Updates(map[string]any{
			"status":       JobStatusCancelled,
			"stage":        "cancelled",
			"last_error":   "cancelled by user",
			"finished_at":  now,
			"heartbeat_at": now,
			"updated_at":   now,
		})
	return res.RowsAffected == 1, res.Error
}

func (r *Repository) CompleteJobIfRunning(id string, updates map[string]any) (bool, error) {
	updates["updated_at"] = time.Now().UTC()
	res := r.db.Model(&IngestJob{}).Where("id = ? AND status = ?", id, JobStatusRunning).Updates(updates)
	return res.RowsAffected == 1, res.Error
}

func (r *Repository) FailJobIfRunning(id string, updates map[string]any) (bool, error) {
	updates["updated_at"] = time.Now().UTC()
	res := r.db.Model(&IngestJob{}).Where("id = ? AND status = ?", id, JobStatusRunning).Updates(updates)
	return res.RowsAffected == 1, res.Error
}

// FinalizeJobAndSourceIfRunning atomically closes a running ingestion job and
// updates the canonical source state. A cancellation racing with finalization
// wins if it changes the job first; in that case neither source completion nor
// failure state is written here.
func (r *Repository) FinalizeJobAndSourceIfRunning(tenantID, jobID, sourceID string, jobUpdates, sourceUpdates map[string]any) (bool, error) {
	changed := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		jobValues := cloneUpdates(jobUpdates)
		jobValues["updated_at"] = now
		res := tenantScope(tx.Model(&IngestJob{}), tenantID).
			Where("id = ? AND source_id = ? AND status = ?", jobID, sourceID, JobStatusRunning).
			Updates(jobValues)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return nil
		}
		sourceValues := cloneUpdates(sourceUpdates)
		sourceValues["updated_at"] = now
		srcRes := tenantScope(tx.Model(&Source{}), tenantID).
			Where("id = ? AND deleted_at IS NULL", sourceID).
			Updates(sourceValues)
		if srcRes.Error != nil {
			return srcRes.Error
		}
		if srcRes.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		changed = true
		return nil
	})
	return changed, err
}

// CancelJobAndSourceIfActive makes cancellation a single canonical database
// transition so a job cannot be cancelled while the source remains processing.
// FinalizeIngestJobIfRunning atomically closes the job, updates its immutable
// source revision and, when sourceUpdates is non-nil, flips the logical source
// pointer to that fully prepared revision. This is the blue/green cutover point:
// a new revision is never visible to retrieval before this transaction commits.
func (r *Repository) FinalizeIngestJobIfRunning(tenantID, jobID, sourceID string, revision int64, jobUpdates, revisionUpdates, sourceUpdates map[string]any, retention ...time.Duration) (bool, error) {
	changed := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		jobValues := cloneUpdates(jobUpdates)
		jobValues["updated_at"] = now
		res := tenantScope(tx.Model(&IngestJob{}), tenantID).
			Where("id = ? AND source_id = ? AND source_revision = ? AND status = ?", jobID, sourceID, revision, JobStatusRunning).
			Updates(jobValues)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return nil
		}

		var current Source
		if sourceUpdates != nil {
			if err := tenantScope(tx.Model(&Source{}), tenantID).
				Where("id = ? AND deleted_at IS NULL", sourceID).First(&current).Error; err != nil {
				return err
			}
		}

		revisionValues := cloneUpdates(revisionUpdates)
		revisionValues["updated_at"] = now
		if sourceUpdates != nil && current.Revision != revision {
			// The newly activated revision must not carry an old GC deadline.
			revisionValues["retain_until"] = nil
			revisionValues["purged_at"] = nil
		}
		revRes := tenantScope(tx.Model(&SourceRevision{}), tenantID).
			Where("source_id = ? AND revision = ? AND purged_at IS NULL", sourceID, revision).Updates(revisionValues)
		if revRes.Error != nil {
			return revRes.Error
		}
		if revRes.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}

		if sourceUpdates != nil {
			if current.Revision != revision {
				window := DefaultRevisionRetention
				if len(retention) > 0 && retention[0] > 0 {
					window = retention[0]
				}
				retainUntil := now.Add(window)
				oldRes := tenantScope(tx.Model(&SourceRevision{}), tenantID).
					Where("source_id = ? AND revision = ? AND purged_at IS NULL", sourceID, current.Revision).
					Updates(map[string]any{"retain_until": retainUntil, "updated_at": now})
				if oldRes.Error != nil {
					return oldRes.Error
				}
			}
			sourceValues := cloneUpdates(sourceUpdates)
			sourceValues["updated_at"] = now
			srcRes := tenantScope(tx.Model(&Source{}), tenantID).
				Where("id = ? AND deleted_at IS NULL", sourceID).Updates(sourceValues)
			if srcRes.Error != nil {
				return srcRes.Error
			}
			if srcRes.RowsAffected != 1 {
				return gorm.ErrRecordNotFound
			}
		}
		changed = true
		return nil
	})
	return changed, err
}

// CancelIngestJobIfActive keeps a failed/cancelled candidate revision isolated.
// The currently active logical source is only changed when updateActiveSource is
// true (initial ingestion / active-revision retry).
func (r *Repository) CancelIngestJobIfActive(tenantID, jobID, sourceID string, revision int64, now time.Time, updateActiveSource bool, retention ...time.Duration) (bool, error) {
	changed := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		res := tenantScope(tx.Model(&IngestJob{}), tenantID).
			Where("id = ? AND source_id = ? AND source_revision = ? AND status IN ?", jobID, sourceID, revision, []string{JobStatusQueued, JobStatusRunning}).
			Updates(map[string]any{
				"status": JobStatusCancelled, "stage": "cancelled", "last_error": "cancelled by user",
				"finished_at": now, "heartbeat_at": now, "updated_at": now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return nil
		}
		window := DefaultRevisionRetention
		if len(retention) > 0 && retention[0] > 0 {
			window = retention[0]
		}
		revRes := tenantScope(tx.Model(&SourceRevision{}), tenantID).
			Where("source_id = ? AND revision = ?", sourceID, revision).
			Updates(map[string]any{"status": RevisionStatusCancelled, "last_error": "ingestion cancelled", "retain_until": now.Add(window), "updated_at": now})
		if revRes.Error != nil {
			return revRes.Error
		}
		if updateActiveSource {
			srcRes := tenantScope(tx.Model(&Source{}), tenantID).Where("id = ? AND deleted_at IS NULL", sourceID).
				Updates(map[string]any{"status": SourceStatusCancelled, "last_error": "ingestion cancelled", "updated_at": now})
			if srcRes.Error != nil {
				return srcRes.Error
			}
		}
		changed = true
		return nil
	})
	return changed, err
}

func (r *Repository) CancelJobAndSourceIfActive(tenantID, jobID, sourceID string, now time.Time) (bool, error) {
	changed := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		res := tenantScope(tx.Model(&IngestJob{}), tenantID).
			Where("id = ? AND source_id = ? AND status IN ?", jobID, sourceID, []string{JobStatusQueued, JobStatusRunning}).
			Updates(map[string]any{
				"status": JobStatusCancelled, "stage": "cancelled", "last_error": "cancelled by user",
				"finished_at": now, "heartbeat_at": now, "updated_at": now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return nil
		}
		srcRes := tenantScope(tx.Model(&Source{}), tenantID).
			Where("id = ? AND deleted_at IS NULL", sourceID).
			Updates(map[string]any{"status": SourceStatusCancelled, "last_error": "ingestion cancelled", "updated_at": now})
		if srcRes.Error != nil {
			return srcRes.Error
		}
		if srcRes.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		changed = true
		return nil
	})
	return changed, err
}

// PrepareRetry atomically clears the failed/cancelled revision, resets the job
// checkpoint and returns the source to pending. It prevents a partially reset
// retry when a process crashes between separate mutations.
func (r *Repository) requireRetryableRevision(tx *gorm.DB, tenantID, sourceID string, revision int64) error {
	if strings.TrimSpace(tenantID) == "" || strings.TrimSpace(sourceID) == "" || revision <= 0 {
		return gorm.ErrRecordNotFound
	}
	var count int64
	if err := tx.Model(&SourceRevision{}).
		Joins("JOIN knowledge_sources ks ON ks.id = knowledge_source_revisions.source_id AND ks.tenant_id = knowledge_source_revisions.tenant_id").
		Where("knowledge_source_revisions.tenant_id = ? AND knowledge_source_revisions.source_id = ? AND knowledge_source_revisions.revision = ?", tenantID, sourceID, revision).
		Where("ks.deleted_at IS NULL AND knowledge_source_revisions.purged_at IS NULL AND knowledge_source_revisions.status <> ?", RevisionStatusPurged).
		Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) requireQuiescentRevisionOutbox(tx *gorm.DB, tenantID, sourceID string, revision int64, now time.Time) error {
	var active int64
	if err := tenantScope(tx.Model(&IndexOutbox{}), tenantID).
		Where("aggregate_type = 'knowledge_source' AND aggregate_id = ? AND source_revision = ?", sourceID, revision).
		Where("status IN ? AND lease_until > ?", []string{"pending", "failed"}, now).
		Count(&active).Error; err != nil {
		return err
	}
	if active != 0 {
		return fmt.Errorf("knowledge revision has in-flight vector operations")
	}
	return nil
}

func (r *Repository) PrepareRetry(tenantID, jobID, sourceID string, revision int64, kind string) (bool, error) {
	changed := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var job IngestJob
		if err := tenantScope(tx.Model(&IngestJob{}), tenantID).
			Where("id = ? AND source_id = ? AND source_revision = ?", jobID, sourceID, revision).First(&job).Error; err != nil {
			return err
		}
		if job.Status == JobStatusQueued || job.Status == JobStatusRunning {
			return nil
		}
		if err := r.requireRetryableRevision(tx, tenantID, sourceID, revision); err != nil {
			return err
		}
		if err := r.requireQuiescentRevisionOutbox(tx, tenantID, sourceID, revision, time.Now().UTC()); err != nil {
			return err
		}
		if strings.TrimSpace(kind) == "" {
			kind = job.Kind
		}
		if strings.TrimSpace(kind) == "" {
			kind = JobKindIngest
		}
		if kind == JobKindIngest {
			if err := r.enqueueSupersededVectorDeletes(tx, tenantID, sourceID, revision, time.Now().UTC()); err != nil {
				return err
			}
		}
		if err := tenantScope(tx.Model(&IndexOutbox{}), tenantID).
			Where("aggregate_type = 'knowledge_source' AND aggregate_id = ? AND source_revision = ? AND operation = 'upsert'", sourceID, revision).
			Delete(&IndexOutbox{}).Error; err != nil {
			return err
		}
		if kind == JobKindIngest {
			if err := tenantScope(tx.Model(&Chunk{}), tenantID).
				Where("source_id = ? AND source_revision = ?", sourceID, revision).Delete(&Chunk{}).Error; err != nil {
				return err
			}
			if err := tenantScope(tx.Model(&DocumentUnit{}), tenantID).
				Where("source_id = ? AND source_revision = ?", sourceID, revision).Delete(&DocumentUnit{}).Error; err != nil {
				return err
			}
		} else if kind == JobKindReindex || kind == JobKindRollback {
			var sourceRevision SourceRevision
			if err := tenantScope(tx.Model(&SourceRevision{}), tenantID).
				Where("source_id = ? AND revision = ?", sourceID, revision).First(&sourceRevision).Error; err != nil {
				return err
			}
			var chunks []Chunk
			if err := tenantScope(tx.Model(&Chunk{}), tenantID).
				Where("source_id = ? AND source_revision = ? AND chunk_index >= 0", sourceID, revision).
				Order("unit_id ASC, chunk_index ASC").Find(&chunks).Error; err != nil {
				return err
			}
			if len(chunks) == 0 {
				return fmt.Errorf("source has no parsed chunks to reindex")
			}
			for start := 0; start < len(chunks); start += 64 {
				end := start + 64
				if end > len(chunks) {
					end = len(chunks)
				}
				ids := make([]string, 0, end-start)
				for _, chunk := range chunks[start:end] {
					ids = append(ids, chunk.ID)
				}
				payload, _ := json.Marshal(IndexPayload{ChunkIDs: ids, IndexVersion: sourceRevision.IndexVersion})
				now := time.Now().UTC()
				if err := tx.Create(&IndexOutbox{ID: uuid.NewString(), TenantID: tenantID, AggregateType: "knowledge_source", AggregateID: sourceID, SourceRevision: revision, Operation: "upsert", PayloadJSON: string(payload), Status: "pending", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
					return err
				}
			}
			if err := tenantScope(tx.Model(&Chunk{}), tenantID).
				Where("source_id = ? AND source_revision = ? AND chunk_index >= 0", sourceID, revision).
				Updates(map[string]any{"status": "parsed", "updated_at": time.Now().UTC()}).Error; err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		res := tenantScope(tx.Model(&IngestJob{}), tenantID).
			Where("id = ? AND source_id = ? AND status NOT IN ?", jobID, sourceID, []string{JobStatusQueued, JobStatusRunning}).
			Updates(map[string]any{
				"status": JobStatusQueued, "stage": "queued", "kind": kind, "retry_count": gorm.Expr("retry_count + 1"),
				"last_error": "", "finished_at": nil, "heartbeat_at": nil,
				"cursor_json": `{"ordinal":0}`, "last_committed_ordinal": -1,
				"processed_units": 0, "failed_units": 0, "indexed_chunks": 0, "updated_at": now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return nil
		}
		if kind == JobKindIngest {
			revRes := tenantScope(tx.Model(&SourceRevision{}), tenantID).
				Where("source_id = ? AND revision = ?", sourceID, revision).
				Updates(map[string]any{
					"status": RevisionStatusPending, "processed_units": 0, "failed_units": 0,
					"indexed_chunks": 0, "last_error": "", "retain_until": nil, "updated_at": now,
				})
			if revRes.Error != nil {
				return revRes.Error
			}
		}
		var src Source
		if err := tenantScope(tx.Model(&Source{}), tenantID).Where("id = ? AND deleted_at IS NULL", sourceID).First(&src).Error; err != nil {
			return err
		}
		if src.Revision == revision && kind == JobKindIngest {
			if err := tenantScope(tx.Model(&Source{}), tenantID).Where("id = ?", sourceID).Updates(map[string]any{
				"status": SourceStatusPending, "processed_units": 0, "failed_units": 0,
				"indexed_chunks": 0, "last_error": "", "updated_at": now,
			}).Error; err != nil {
				return err
			}
		}
		changed = true
		return nil
	})
	return changed, err
}

func cloneUpdates(values map[string]any) map[string]any {
	out := make(map[string]any, len(values)+1)
	for key, value := range values {
		out[key] = value
	}
	return out
}
func (r *Repository) ListJobs(tenantID, sourceID string) ([]IngestJob, error) {
	var xs []IngestJob
	q := tenantScope(r.db.Model(&IngestJob{}), tenantID)
	if sourceID != "" {
		q = q.Where("source_id = ?", sourceID)
	}
	err := q.Order("created_at DESC").Find(&xs).Error
	if xs == nil {
		xs = []IngestJob{}
	}
	return xs, err
}
func (r *Repository) recoverableJobScope(db *gorm.DB, staleAfter ...time.Duration) *gorm.DB {
	window := r.settings.JobStaleAfter
	if window <= 0 {
		window = defaultRuntimeSettings().JobStaleAfter
	}
	if len(staleAfter) > 0 && staleAfter[0] > 0 {
		window = staleAfter[0]
	}
	staleBefore := time.Now().UTC().Add(-window)
	return db.Where("status = ? OR (status = ? AND (heartbeat_at IS NULL OR heartbeat_at < ?))", JobStatusQueued, JobStatusRunning, staleBefore)
}

func (r *Repository) RecoverableJobs(staleAfter ...time.Duration) ([]IngestJob, error) {
	var xs []IngestJob
	err := r.recoverableJobScope(r.db.Model(&IngestJob{}), staleAfter...).Order("created_at ASC").Find(&xs).Error
	return xs, err
}

func (r *Repository) RecoverableJobsForTenant(tenantID string, staleAfter ...time.Duration) ([]IngestJob, error) {
	var xs []IngestJob
	q := tenantScope(r.db.Model(&IngestJob{}), tenantID)
	err := r.recoverableJobScope(q, staleAfter...).Order("created_at ASC").Find(&xs).Error
	return xs, err
}
func (r *Repository) UpdateJob(id string, updates map[string]any) error {
	updates["updated_at"] = time.Now().UTC()
	res := r.db.Model(&IngestJob{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) UpdateJobScoped(tenantID, id string, updates map[string]any) error {
	updates["updated_at"] = time.Now().UTC()
	res := tenantScope(r.db.Model(&IngestJob{}), tenantID).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) ResetJobForRetry(tenantID, id string) error {
	res := tenantScope(r.db.Model(&IngestJob{}), tenantID).Where("id = ?", id).Updates(map[string]any{"status": JobStatusQueued, "stage": "queued", "retry_count": gorm.Expr("retry_count + 1"), "last_error": "", "finished_at": nil, "updated_at": time.Now().UTC()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) requireWritableRevision(tx *gorm.DB, tenantID, sourceID string, revision int64) error {
	if strings.TrimSpace(tenantID) == "" || strings.TrimSpace(sourceID) == "" || revision <= 0 {
		return gorm.ErrRecordNotFound
	}
	var count int64
	err := tx.Model(&SourceRevision{}).
		Joins("JOIN knowledge_sources ks ON ks.id = knowledge_source_revisions.source_id AND ks.tenant_id = knowledge_source_revisions.tenant_id").
		Where("knowledge_source_revisions.tenant_id = ? AND knowledge_source_revisions.source_id = ? AND knowledge_source_revisions.revision = ?", strings.TrimSpace(tenantID), strings.TrimSpace(sourceID), revision).
		Where("ks.deleted_at IS NULL AND knowledge_source_revisions.purged_at IS NULL").
		Where("knowledge_source_revisions.status IN ?", []string{RevisionStatusPending, RevisionStatusProcessing, RevisionStatusReady, RevisionStatusReadyDegraded}).
		Count(&count).Error
	if err != nil {
		return err
	}
	if count != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) ReplaceParsedData(tenantID, sourceID string, revision int64, units []DocumentUnit, chunks []Chunk, outboxes []IndexOutbox) error {
	unitIDs := make(map[string]struct{}, len(units))
	for _, unit := range units {
		if unit.TenantID != tenantID || unit.SourceID != sourceID || unit.SourceRevision != revision || strings.TrimSpace(unit.ID) == "" {
			return fmt.Errorf("knowledge unit does not belong to source revision")
		}
		unitIDs[unit.ID] = struct{}{}
	}
	for _, chunk := range chunks {
		if chunk.TenantID != tenantID || chunk.SourceID != sourceID || chunk.SourceRevision != revision {
			return fmt.Errorf("knowledge chunk does not belong to source revision")
		}
		if _, ok := unitIDs[chunk.UnitID]; !ok {
			return fmt.Errorf("knowledge chunk has no source revision unit")
		}
	}
	for _, outbox := range outboxes {
		if outbox.TenantID != tenantID || outbox.AggregateType != "knowledge_source" || outbox.AggregateID != sourceID || outbox.SourceRevision != revision || outbox.Operation != "upsert" {
			return fmt.Errorf("knowledge outbox does not belong to source revision")
		}
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := r.requireWritableRevision(tx, tenantID, sourceID, revision); err != nil {
			return err
		}
		if err := r.requireQuiescentRevisionOutbox(tx, tenantID, sourceID, revision, time.Now().UTC()); err != nil {
			return err
		}
		if err := r.enqueueSupersededVectorDeletes(tx, tenantID, sourceID, revision, time.Now().UTC()); err != nil {
			return err
		}
		if err := tenantScope(tx.Model(&IndexOutbox{}), tenantID).
			Where("aggregate_type = 'knowledge_source' AND aggregate_id = ? AND source_revision = ? AND operation = 'upsert'", sourceID, revision).
			Delete(&IndexOutbox{}).Error; err != nil {
			return err
		}
		if err := tenantScope(tx.Where("source_id = ? AND source_revision = ?", sourceID, revision), tenantID).Delete(&Chunk{}).Error; err != nil {
			return err
		}
		if err := tenantScope(tx.Where("source_id = ? AND source_revision = ?", sourceID, revision), tenantID).Delete(&DocumentUnit{}).Error; err != nil {
			return err
		}
		if len(units) > 0 {
			if err := tx.CreateInBatches(units, 100).Error; err != nil {
				return err
			}
		}
		if len(chunks) > 0 {
			if err := tx.CreateInBatches(chunks, 100).Error; err != nil {
				return err
			}
		}
		if len(outboxes) > 0 {
			if err := tx.CreateInBatches(outboxes, 100).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func applyTemporalChunkScope(q *gorm.DB, temporal temporalQuery) *gorm.DB {
	if temporal.Mode == TemporalModeAll {
		return q
	}
	at := temporal.AsOf
	if at.IsZero() {
		at = time.Now().UTC()
	}
	return q.Where("(knowledge_chunks.valid_from IS NULL OR knowledge_chunks.valid_from <= ?) AND (knowledge_chunks.valid_to IS NULL OR knowledge_chunks.valid_to > ?)", at, at)
}

func (r *Repository) VisibleChunksByIDs(tenantID string, ids []string) ([]Chunk, error) {
	return r.VisibleChunksByIDsAt(tenantID, ids, temporalQuery{Mode: TemporalModeCurrent, AsOf: time.Now().UTC()})
}

func (r *Repository) VisibleChunksByIDsAt(tenantID string, ids []string, temporal temporalQuery) ([]Chunk, error) {
	return r.VisibleChunksByIDsAtContext(context.Background(), tenantID, ids, temporal)
}

func (r *Repository) VisibleChunksByIDsAtContext(ctx context.Context, tenantID string, ids []string, temporal temporalQuery) ([]Chunk, error) {
	if len(ids) == 0 {
		return []Chunk{}, nil
	}
	var xs []Chunk
	q := r.db.WithContext(ctx).Model(&Chunk{}).
		Where("knowledge_chunks.tenant_id = ?", tenantID).
		Joins("JOIN knowledge_sources ks ON ks.id = knowledge_chunks.source_id AND ks.tenant_id = knowledge_chunks.tenant_id AND ks.revision = knowledge_chunks.source_revision").
		Where("knowledge_chunks.id IN ?", ids).
		Where("knowledge_chunks.chunk_index >= 0").
		Where("ks.status IN ? AND ks.deleted_at IS NULL", []string{SourceStatusReady, SourceStatusReadyDegraded})
	q = applyTemporalChunkScope(q, temporal)
	err := q.Find(&xs).Error
	if xs == nil {
		xs = []Chunk{}
	}
	return xs, err
}

func (r *Repository) ExactCandidates(tenantID, query string, sourceIDs []string, limit int) ([]Chunk, error) {
	return r.ExactCandidatesAt(tenantID, query, sourceIDs, limit, temporalQuery{Mode: TemporalModeCurrent, AsOf: time.Now().UTC()})
}

func (r *Repository) ExactCandidatesAt(tenantID, query string, sourceIDs []string, limit int, temporal temporalQuery) ([]Chunk, error) {
	return r.ExactCandidatesAtContext(context.Background(), tenantID, query, sourceIDs, limit, temporal)
}

func (r *Repository) ExactCandidatesAtContext(ctx context.Context, tenantID, query string, sourceIDs []string, limit int, temporal temporalQuery) ([]Chunk, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []Chunk{}, nil
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	like := "%" + strings.ToLower(query) + "%"
	q := r.db.WithContext(ctx).Model(&Chunk{}).
		Where("knowledge_chunks.tenant_id = ?", tenantID).
		Joins("JOIN knowledge_sources ks ON ks.id = knowledge_chunks.source_id AND ks.tenant_id = knowledge_chunks.tenant_id AND ks.revision = knowledge_chunks.source_revision").
		Where("knowledge_chunks.chunk_index >= 0").
		Where("ks.status IN ? AND ks.deleted_at IS NULL", []string{SourceStatusReady, SourceStatusReadyDegraded}).
		Where("LOWER(knowledge_chunks.content) LIKE ? OR LOWER(knowledge_chunks.title) LIKE ? OR LOWER(ks.filename) LIKE ? OR LOWER(ks.logical_name) LIKE ?", like, like, like, like)
	q = applyTemporalChunkScope(q, temporal)
	if len(sourceIDs) > 0 {
		q = q.Where("knowledge_chunks.source_id IN ?", sourceIDs)
	}
	var xs []Chunk
	if err := q.Order("knowledge_chunks.updated_at DESC").Limit(limit).Find(&xs).Error; err != nil {
		return nil, err
	}
	if xs == nil {
		xs = []Chunk{}
	}
	return xs, nil
}

func (r *Repository) LatestJobForSource(tenantID, sourceID string) (*IngestJob, error) {
	var x IngestJob
	err := tenantScope(r.db.Model(&IngestJob{}), tenantID).Where("source_id = ?", sourceID).Order("created_at DESC").First(&x).Error
	return &x, err
}

func (r *Repository) CreateJob(job *IngestJob) error {
	return r.db.Create(job).Error
}

// RevisionIndexable is a write-side fence for secondary indexes. Historical
// revisions may legitimately be rebuilt for rollback, so indexability does not
// require the revision to be active. It does require the logical source to
// remain live and the immutable revision to remain non-terminal/non-purged.
func (r *Repository) RevisionIndexable(tenantID, sourceID string, revision int64) (bool, error) {
	if strings.TrimSpace(tenantID) == "" || strings.TrimSpace(sourceID) == "" || revision <= 0 {
		return false, nil
	}
	var count int64
	err := r.db.Model(&SourceRevision{}).
		Joins("JOIN knowledge_sources ks ON ks.id = knowledge_source_revisions.source_id AND ks.tenant_id = knowledge_source_revisions.tenant_id").
		Where("knowledge_source_revisions.tenant_id = ? AND knowledge_source_revisions.source_id = ? AND knowledge_source_revisions.revision = ?", tenantID, sourceID, revision).
		Where("ks.deleted_at IS NULL").
		Where("knowledge_source_revisions.purged_at IS NULL").
		Where("knowledge_source_revisions.status IN ?", []string{RevisionStatusPending, RevisionStatusProcessing, RevisionStatusReady, RevisionStatusReadyDegraded}).
		Count(&count).Error
	return count == 1, err
}

// EnqueueVectorDeleteCorrection records a durable cleanup after an upsert that
// lost a race with a canonical tombstone/failure. The corrective delete is
// intentionally a new outbox row: an earlier delete may already have completed
// before the stale upsert reached Qdrant.
func (r *Repository) EnqueueVectorDeleteCorrection(tenantID, sourceID string, revision int64, chunkIDs []string, indexVersion string) error {
	if len(chunkIDs) == 0 {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		return r.enqueueVectorDeleteOutboxes(tx, tenantID, sourceID, revision, chunkIDs, []string{indexVersion}, time.Now().UTC())
	})
}

func (r *Repository) ChunksByIDs(tenantID string, ids []string) ([]Chunk, error) {
	if len(ids) == 0 {
		return []Chunk{}, nil
	}
	var xs []Chunk
	err := tenantScope(r.db.Model(&Chunk{}), tenantID).Where("id IN ?", ids).Find(&xs).Error
	return xs, err
}
func (r *Repository) ChunksForSource(tenantID, sourceID string, revision int64) ([]Chunk, error) {
	var xs []Chunk
	err := tenantScope(r.db.Model(&Chunk{}), tenantID).Where("source_id = ? AND source_revision = ?", sourceID, revision).Find(&xs).Error
	return xs, err
}
func (r *Repository) GetChunk(tenantID, id string) (*Chunk, error) {
	var x Chunk
	err := tenantScope(r.db.Model(&Chunk{}), tenantID).Where("id = ?", id).First(&x).Error
	return &x, err
}

func (r *Repository) UnitOutlines(ctx context.Context, tenantID, sourceID string, revision, afterOrdinal int64, limit int) ([]UnitOutline, *int64, error) {
	if strings.TrimSpace(sourceID) == "" || revision <= 0 {
		return []UnitOutline{}, nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	if afterOrdinal < -1 {
		afterOrdinal = -1
	}
	var units []DocumentUnit
	err := tenantScope(r.db.WithContext(ctx).Model(&DocumentUnit{}), tenantID).
		Select("id, source_id, source_revision, ordinal, unit_type, title, locator_json, checksum").
		Where("source_id = ? AND source_revision = ? AND ordinal > ?", strings.TrimSpace(sourceID), revision, afterOrdinal).
		Order("ordinal ASC").Limit(limit + 1).Find(&units).Error
	if err != nil {
		return nil, nil, err
	}
	hasMore := len(units) > limit
	if hasMore {
		units = units[:limit]
	}
	out := make([]UnitOutline, 0, len(units))
	for _, unit := range units {
		out = append(out, UnitOutline{
			ID: unit.ID, SourceID: unit.SourceID, SourceRevision: unit.SourceRevision, Ordinal: unit.Ordinal,
			UnitType: unit.UnitType, Title: unit.Title, Locator: unmarshalMap(unit.LocatorJSON), Checksum: unit.Checksum,
		})
	}
	var next *int64
	if hasMore && len(out) > 0 {
		value := out[len(out)-1].Ordinal
		next = &value
	}
	return out, next, nil
}

func (r *Repository) GetUnit(tenantID, id string) (*DocumentUnit, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var x DocumentUnit
	err := tenantScope(r.db.Model(&DocumentUnit{}), tenantID).Where("id = ?", id).First(&x).Error
	if err == nil {
		x.Locator = unmarshalMap(x.LocatorJSON)
		x.Metadata = unmarshalMap(x.MetadataJSON)
	}
	return &x, err
}

func (r *Repository) UnitsWindow(tenantID, sourceID string, revision, anchorOrdinal int64, before, after, maxUnits int) ([]DocumentUnit, error) {
	if before < 0 {
		before = 0
	}
	if after < 0 {
		after = 0
	}
	if before > 16 {
		before = 16
	}
	if after > 16 {
		after = 16
	}
	if maxUnits <= 0 {
		maxUnits = 8
	}
	if maxUnits > 32 {
		maxUnits = 32
	}
	low, high := anchorOrdinal-int64(before), anchorOrdinal+int64(after)
	if low < 0 {
		low = 0
	}
	var xs []DocumentUnit
	err := tenantScope(r.db.Model(&DocumentUnit{}), tenantID).
		Where("source_id = ? AND source_revision = ? AND ordinal >= ? AND ordinal <= ?", strings.TrimSpace(sourceID), revision, low, high).
		Order("ordinal ASC").Find(&xs).Error
	if len(xs) > maxUnits {
		anchorIndex := 0
		for i := range xs {
			if xs[i].Ordinal == anchorOrdinal {
				anchorIndex = i
				break
			}
		}
		start := anchorIndex - before
		if start < 0 {
			start = 0
		}
		if start+maxUnits <= anchorIndex {
			start = anchorIndex - maxUnits + 1
		}
		if start+maxUnits > len(xs) {
			start = len(xs) - maxUnits
		}
		if start < 0 {
			start = 0
		}
		xs = xs[start : start+maxUnits]
	}
	for i := range xs {
		xs[i].Locator = unmarshalMap(xs[i].LocatorJSON)
		xs[i].Metadata = unmarshalMap(xs[i].MetadataJSON)
	}
	if xs == nil {
		xs = []DocumentUnit{}
	}
	return xs, err
}

func (r *Repository) FindUnitsByLocator(ctx context.Context, tenantID, sourceID string, revision int64, locator map[string]any, limit int) ([]DocumentUnit, error) {
	if strings.TrimSpace(sourceID) == "" || revision <= 0 || len(locator) == 0 {
		return []DocumentUnit{}, nil
	}
	if err := validateCanonicalReadLocator(locator); err != nil {
		return nil, err
	}
	if _, err := r.GetSource(tenantID, sourceID); err != nil {
		return nil, err
	}
	currentRevision, err := r.GetSourceRevision(tenantID, sourceID, revision)
	if err != nil {
		return nil, err
	}
	if currentRevision.PurgedAt != nil || (currentRevision.Status != RevisionStatusReady && currentRevision.Status != RevisionStatusReadyDegraded) {
		return nil, gorm.ErrRecordNotFound
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if limit <= 0 {
		limit = 16
	}
	if limit > 64 {
		limit = 64
	}
	// Locator JSON is intentionally treated as canonical metadata rather than as
	// a database-specific JSON query contract. Scan the revision in bounded
	// batches and perform exact subset matching in Go so SQLite/MySQL deployments
	// behave the same way. Do not cap the scan to the first N units: large PDFs
	// and spreadsheets can legitimately target pages/rows far beyond 2K units.
	const batchSize = 2048
	out := make([]DocumentUnit, 0, limit)
	var lastOrdinal int64 = -1
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var candidates []DocumentUnit
		err := tenantScope(r.db.WithContext(ctx).Model(&DocumentUnit{}), tenantID).
			Where("source_id = ? AND source_revision = ? AND ordinal > ?", strings.TrimSpace(sourceID), revision, lastOrdinal).
			Order("ordinal ASC").Limit(batchSize).Find(&candidates).Error
		if err != nil {
			return nil, err
		}
		if len(candidates) == 0 {
			break
		}
		for _, unit := range candidates {
			parsed := unmarshalMap(unit.LocatorJSON)
			if !locatorSubsetMatch(parsed, locator) {
				continue
			}
			unit.Locator = parsed
			unit.Metadata = unmarshalMap(unit.MetadataJSON)
			out = append(out, unit)
			if len(out) >= limit {
				return out, nil
			}
		}
		lastOrdinal = candidates[len(candidates)-1].Ordinal
		if len(candidates) < batchSize {
			break
		}
	}
	return out, nil
}

func validateCanonicalReadLocator(locator map[string]any) error {
	if len(locator) == 0 || len(locator) > 16 {
		return fmt.Errorf("read locator must contain between 1 and 16 fields")
	}
	for key, value := range locator {
		if strings.TrimSpace(key) == "" || len(key) > 64 || strings.TrimSpace(key) != key || strings.ContainsAny(key, "\r\n\t") {
			return fmt.Errorf("read locator contains an invalid field")
		}
		switch v := value.(type) {
		case string:
			if strings.TrimSpace(v) == "" || len(v) > 512 || strings.ContainsAny(v, "\x00\r\n") {
				return fmt.Errorf("read locator contains an invalid string value")
			}
		case int, int64, float64, json.Number, bool:
		default:
			return fmt.Errorf("read locator value must be a scalar")
		}
	}
	return nil
}

func locatorAliases(key string) []string {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "page":
		return []string{"pageNumber", "page_number"}
	case "pagenumber", "page_number":
		return []string{"page"}
	case "row":
		return []string{"rowStart", "rowEnd", "row_start", "row_end"}
	case "rowstart", "row_start":
		return []string{"row"}
	case "paragraph":
		return []string{"paragraphIndex", "paragraph_index"}
	case "sheet":
		return []string{"sheetName", "sheet_name"}
	}
	return nil
}

func locatorSubsetMatch(actual, requested map[string]any) bool {
	for key, expected := range requested {
		key = strings.TrimSpace(key)
		if key == "" {
			return false
		}
		got, ok := actual[key]
		if !ok {
			for _, alias := range locatorAliases(key) {
				if candidate, exists := actual[alias]; exists {
					got, ok = candidate, true
					break
				}
			}
		}
		if !ok {
			return false
		}
		left, lerr := json.Marshal(got)
		right, rerr := json.Marshal(expected)
		if lerr != nil || rerr != nil || string(left) != string(right) {
			// JSON numbers can arrive as int/float depending on the client. Compare
			// their textual values as a final compatibility path.
			if fmt.Sprint(got) != fmt.Sprint(expected) {
				return false
			}
		}
	}
	return true
}

func (r *Repository) GetParentChunk(tenantID, id string) (*Chunk, error) {
	return r.GetParentChunkAt(tenantID, id, temporalQuery{Mode: TemporalModeCurrent, AsOf: time.Now().UTC()})
}

func (r *Repository) GetParentChunkAt(tenantID, id string, temporal temporalQuery) (*Chunk, error) {
	return r.GetParentChunkAtContext(context.Background(), tenantID, id, temporal)
}

func (r *Repository) GetParentChunkAtContext(ctx context.Context, tenantID, id string, temporal temporalQuery) (*Chunk, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var x Chunk
	q := tenantScope(r.db.WithContext(ctx).Model(&Chunk{}), tenantID).Where("id = ? AND chunk_index < 0", id)
	q = applyTemporalChunkScope(q, temporal)
	err := q.First(&x).Error
	return &x, err
}
func (r *Repository) AdjacentChildChunks(tenantID, sourceID string, revision int64, unitID string, centerIndex, radius int) ([]Chunk, error) {
	if radius <= 0 {
		radius = 1
	}
	if radius > 2 {
		radius = 2
	}
	var xs []Chunk
	err := tenantScope(r.db.Model(&Chunk{}), tenantID).
		Where("source_id = ? AND source_revision = ? AND unit_id = ?", sourceID, revision, unitID).
		Where("chunk_index >= ? AND chunk_index <= ? AND chunk_index >= 0", centerIndex-radius, centerIndex+radius).
		Order("chunk_index ASC").Find(&xs).Error
	if xs == nil {
		xs = []Chunk{}
	}
	return xs, err
}

func (r *Repository) MarkChunksIndexed(tenantID string, ids []string, embeddingVersion, indexVersion string) error {
	if len(ids) == 0 {
		return nil
	}
	return tenantScope(r.db.Model(&Chunk{}), tenantID).Where("id IN ?", ids).Updates(map[string]any{"status": "indexed", "embedding_version": embeddingVersion, "index_version": indexVersion, "updated_at": time.Now().UTC()}).Error
}

func (r *Repository) pendingOutboxScope(db *gorm.DB, now time.Time) *gorm.DB {
	maxRetries := r.settings.OutboxMaxRetries
	if maxRetries <= 0 {
		maxRetries = defaultRuntimeSettings().OutboxMaxRetries
	}
	return db.Where("status IN ? AND retry_count < ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?) AND (lease_until IS NULL OR lease_until <= ?)", []string{"pending", "failed"}, maxRetries, now, now)
}

func (r *Repository) claimNextOutbox(tenantID, sourceID string, revision int64, now time.Time, leaseFor time.Duration) (*IndexOutbox, bool, error) {
	if leaseFor <= 0 {
		leaseFor = r.settings.OutboxLease
		if leaseFor <= 0 {
			leaseFor = defaultRuntimeSettings().OutboxLease
		}
	}
	var claimed *IndexOutbox
	err := r.db.Transaction(func(tx *gorm.DB) error {
		q := tenantScope(tx.Model(&IndexOutbox{}), tenantID)
		q = r.pendingOutboxScope(q, now)
		q = q.Where(`(operation <> 'delete' OR NOT EXISTS (
			SELECT 1 FROM knowledge_index_outbox AS pending_upsert
			WHERE pending_upsert.tenant_id = knowledge_index_outbox.tenant_id
			AND pending_upsert.aggregate_type = knowledge_index_outbox.aggregate_type
			AND pending_upsert.aggregate_id = knowledge_index_outbox.aggregate_id
			AND pending_upsert.source_revision = knowledge_index_outbox.source_revision
			AND pending_upsert.operation = 'upsert'
			AND pending_upsert.status IN ('pending', 'failed')
			AND pending_upsert.lease_until > ?))`, now)
		q = q.Where(`(operation <> 'upsert' OR NOT EXISTS (
			SELECT 1 FROM knowledge_index_outbox AS pending_delete
			WHERE pending_delete.tenant_id = knowledge_index_outbox.tenant_id
			AND pending_delete.aggregate_type = knowledge_index_outbox.aggregate_type
			AND pending_delete.aggregate_id = knowledge_index_outbox.aggregate_id
			AND pending_delete.source_revision = knowledge_index_outbox.source_revision
			AND pending_delete.operation = 'delete'
			AND pending_delete.status IN ('pending', 'failed')
			AND (pending_delete.created_at < knowledge_index_outbox.created_at
			OR (pending_delete.created_at = knowledge_index_outbox.created_at AND pending_delete.id < knowledge_index_outbox.id))))`)
		if strings.TrimSpace(sourceID) != "" {
			q = q.Where("aggregate_type = 'knowledge_source' AND aggregate_id = ?", strings.TrimSpace(sourceID))
		}
		if revision > 0 {
			q = q.Where("source_revision = ?", revision)
		}
		var candidate IndexOutbox
		if err := q.Order("created_at ASC").First(&candidate).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		token := uuid.NewString()
		leaseUntil := now.Add(leaseFor)
		maxRetries := r.settings.OutboxMaxRetries
		if maxRetries <= 0 {
			maxRetries = defaultRuntimeSettings().OutboxMaxRetries
		}
		res := tenantScope(tx.Model(&IndexOutbox{}), tenantID).
			Where("id = ?", candidate.ID).
			Where("status IN ? AND retry_count < ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?) AND (lease_until IS NULL OR lease_until <= ?)", []string{"pending", "failed"}, maxRetries, now, now)
		if candidate.Operation == "upsert" {
			res = res.Where(`NOT EXISTS (
				SELECT 1 FROM knowledge_index_outbox AS pending_delete
				WHERE pending_delete.tenant_id = knowledge_index_outbox.tenant_id
				AND pending_delete.aggregate_type = knowledge_index_outbox.aggregate_type
				AND pending_delete.aggregate_id = knowledge_index_outbox.aggregate_id
				AND pending_delete.source_revision = knowledge_index_outbox.source_revision
				AND pending_delete.operation = 'delete'
				AND pending_delete.status IN ('pending', 'failed')
				AND (pending_delete.created_at < knowledge_index_outbox.created_at
				OR (pending_delete.created_at = knowledge_index_outbox.created_at AND pending_delete.id < knowledge_index_outbox.id)))`)
		}
		if candidate.Operation == "delete" {
			res = res.Where(`NOT EXISTS (
				SELECT 1 FROM knowledge_index_outbox AS pending_upsert
				WHERE pending_upsert.tenant_id = knowledge_index_outbox.tenant_id
				AND pending_upsert.aggregate_type = knowledge_index_outbox.aggregate_type
				AND pending_upsert.aggregate_id = knowledge_index_outbox.aggregate_id
				AND pending_upsert.source_revision = knowledge_index_outbox.source_revision
				AND pending_upsert.operation = 'upsert'
				AND pending_upsert.status IN ('pending', 'failed')
				AND pending_upsert.lease_until > ?)`, now)
		}
		res = res.Updates(map[string]any{"lease_token": token, "lease_until": leaseUntil, "updated_at": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return nil
		}
		candidate.LeaseToken = token
		candidate.LeaseUntil = &leaseUntil
		candidate.UpdatedAt = now
		claimed = &candidate
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return claimed, claimed != nil, nil
}

// ClaimNextOutboxForTenant acquires one due secondary-index operation using a
// canonical DB lease. Multiple backend processes may call this concurrently;
// only the winner of the conditional UPDATE owns the returned row.
func (r *Repository) ClaimNextOutboxForTenant(tenantID string, now time.Time, leaseFor time.Duration) (*IndexOutbox, bool, error) {
	return r.claimNextOutbox(tenantID, "", 0, now, leaseFor)
}

func (r *Repository) ClaimNextOutboxForSourceRevision(tenantID, sourceID string, revision int64, now time.Time, leaseFor time.Duration) (*IndexOutbox, bool, error) {
	if strings.TrimSpace(sourceID) == "" || revision <= 0 {
		return nil, false, fmt.Errorf("source and positive revision required")
	}
	return r.claimNextOutbox(tenantID, sourceID, revision, now, leaseFor)
}

// FinishOutboxLease is lease-token conditional so a stale worker cannot mark a
// row done after another process has legitimately reclaimed the expired lease.
func (r *Repository) RenewOutboxLease(tenantID, id, leaseToken string, now time.Time, leaseFor time.Duration) (bool, error) {
	if strings.TrimSpace(tenantID) == "" || strings.TrimSpace(id) == "" || strings.TrimSpace(leaseToken) == "" || leaseFor <= 0 {
		return false, fmt.Errorf("tenant, outbox id, token and positive lease required")
	}
	res := tenantScope(r.db.Model(&IndexOutbox{}), tenantID).
		Where("id = ? AND lease_token = ? AND lease_until > ? AND status IN ?", id, leaseToken, now, []string{"pending", "failed"}).
		Updates(map[string]any{"lease_until": now.Add(leaseFor), "updated_at": now})
	return res.RowsAffected == 1, res.Error
}

func (r *Repository) FinishOutboxLease(tenantID, id, leaseToken string, updates map[string]any) (bool, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(leaseToken) == "" {
		return false, fmt.Errorf("outbox id and lease token required")
	}
	values := cloneUpdates(updates)
	values["lease_token"] = ""
	values["lease_until"] = nil
	values["updated_at"] = time.Now().UTC()
	res := tenantScope(r.db.Model(&IndexOutbox{}), tenantID).
		Where("id = ? AND lease_token = ? AND lease_until > ?", id, leaseToken, time.Now().UTC()).Updates(values)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

func (r *Repository) RequeueExhaustedVectorDeletes(tenantID string, limit int, now time.Time) (int64, error) {
	if strings.TrimSpace(tenantID) == "" {
		return 0, fmt.Errorf("tenant scope required")
	}
	if limit <= 0 {
		return 0, nil
	}
	if limit > 1000 {
		limit = 1000
	}
	maxRetries := r.settings.OutboxMaxRetries
	if maxRetries <= 0 {
		maxRetries = defaultRuntimeSettings().OutboxMaxRetries
	}
	ids := r.db.Model(&IndexOutbox{}).Select("id").
		Where("tenant_id = ? AND operation = 'delete' AND status = 'failed' AND retry_count >= ?", tenantID, maxRetries).
		Where("(lease_until IS NULL OR lease_until <= ?)", now).
		Order("created_at ASC").Limit(limit)
	res := tenantScope(r.db.Model(&IndexOutbox{}), tenantID).
		Where("id IN (?)", ids).
		Where("operation = 'delete' AND status = 'failed' AND retry_count >= ? AND (lease_until IS NULL OR lease_until <= ?)", maxRetries, now).
		Updates(map[string]any{
			"status": "pending", "retry_count": 0, "last_error": "", "next_attempt_at": nil,
			"lease_token": "", "lease_until": nil, "updated_at": now,
		})
	return res.RowsAffected, res.Error
}

func (r *Repository) ListKnowledgeTenantIDs() ([]string, error) {
	var tenantIDs []string
	err := r.db.Raw(`
		SELECT DISTINCT tenant_id FROM knowledge_sources WHERE TRIM(tenant_id) <> ''
		UNION
		SELECT DISTINCT tenant_id FROM knowledge_index_outbox WHERE TRIM(tenant_id) <> ''
		UNION
		SELECT DISTINCT tenant_id FROM knowledge_ingest_jobs WHERE TRIM(tenant_id) <> ''
		ORDER BY tenant_id ASC`).Scan(&tenantIDs).Error
	if tenantIDs == nil {
		tenantIDs = []string{}
	}
	return tenantIDs, err
}
func (r *Repository) EnsureAlias(tenantID string) (*IndexAlias, error) {
	var a IndexAlias
	err := r.db.Where("tenant_id = ?", tenantID).First(&a).Error
	if err == nil {
		return &a, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	a = IndexAlias{TenantID: tenantID, ActiveVersion: DefaultIndexVersion, UpdatedAt: time.Now().UTC()}
	if err := r.db.Create(&a).Error; err != nil {
		return nil, err
	}
	return &a, nil
}
func (r *Repository) PromoteAlias(tenantID, version string) error {
	a, err := r.EnsureAlias(tenantID)
	if err != nil {
		return err
	}
	version = strings.TrimSpace(version)
	if version == "" {
		return errors.New("index version required")
	}
	return r.db.Model(&IndexAlias{}).Where("tenant_id = ?", tenantID).Updates(map[string]any{"previous_version": a.ActiveVersion, "active_version": version, "updated_at": time.Now().UTC()}).Error
}

func (r *Repository) LexicalCandidates(tenantID, query string, sourceIDs []string, limit int) ([]Chunk, error) {
	return r.LexicalCandidatesAt(tenantID, query, sourceIDs, limit, temporalQuery{Mode: TemporalModeCurrent, AsOf: time.Now().UTC()})
}

func (r *Repository) LexicalCandidatesAt(tenantID, query string, sourceIDs []string, limit int, temporal temporalQuery) ([]Chunk, error) {
	return r.LexicalCandidatesAtContext(context.Background(), tenantID, query, sourceIDs, limit, temporal)
}

func (r *Repository) LexicalCandidatesAtContext(ctx context.Context, tenantID, query string, sourceIDs []string, limit int, temporal temporalQuery) ([]Chunk, error) {
	terms := tokenizeQuery(query)
	if len(terms) == 0 {
		return []Chunk{}, nil
	}
	if limit <= 0 {
		limit = 30
	}
	if limit > 200 {
		limit = 200
	}
	q := r.db.WithContext(ctx).Model(&Chunk{}).
		Where("knowledge_chunks.tenant_id = ?", tenantID).
		Joins("JOIN knowledge_sources ks ON ks.id = knowledge_chunks.source_id AND ks.tenant_id = knowledge_chunks.tenant_id AND ks.revision = knowledge_chunks.source_revision").
		Where("knowledge_chunks.status IN ?", []string{"parsed", "indexed", "lexical_ready"}).
		Where("knowledge_chunks.chunk_index >= 0").
		Where("ks.status IN ? AND ks.deleted_at IS NULL", []string{SourceStatusReady, SourceStatusReadyDegraded})
	q = applyTemporalChunkScope(q, temporal)
	if len(sourceIDs) > 0 {
		q = q.Where("knowledge_chunks.source_id IN ?", sourceIDs)
	}
	cond := make([]string, 0, len(terms))
	args := make([]any, 0, len(terms)*2)
	for _, t := range terms {
		like := "%" + strings.ToLower(t) + "%"
		cond = append(cond, "(LOWER(knowledge_chunks.content) LIKE ? OR LOWER(knowledge_chunks.title) LIKE ? OR LOWER(ks.filename) LIKE ? OR LOWER(ks.logical_name) LIKE ?)")
		args = append(args, like, like, like, like)
	}
	q = q.Where("("+strings.Join(cond, " OR ")+")", args...)
	var xs []Chunk
	err := q.Order("knowledge_chunks.updated_at DESC").Limit(limit * 4).Find(&xs).Error
	if err != nil {
		return nil, err
	}
	// Rank in-memory by term frequency so SQLite remains canonical and portable.
	type ranked struct {
		c Chunk
		s int
	}
	rs := make([]ranked, 0, len(xs))
	for _, c := range xs {
		txt := strings.ToLower(c.Title + "\n" + c.Content)
		score := 0
		for _, t := range terms {
			score += strings.Count(txt, strings.ToLower(t)) * 2
		}
		rs = append(rs, ranked{c, score})
	}
	for i := 0; i < len(rs); i++ {
		for j := i + 1; j < len(rs); j++ {
			if rs[j].s > rs[i].s {
				rs[i], rs[j] = rs[j], rs[i]
			}
		}
	}
	if len(rs) > limit {
		rs = rs[:limit]
	}
	out := make([]Chunk, len(rs))
	for i, x := range rs {
		out[i] = x.c
	}
	return out, nil
}

func tokenizeQuery(q string) []string {
	f := strings.FieldsFunc(strings.TrimSpace(strings.ToLower(q)), func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == ',' || r == ';' || r == '，' || r == '。' || r == '；' || r == '：' || r == ':' || r == '?' || r == '？'
	})
	out := make([]string, 0, len(f)*3)
	seen := map[string]bool{}
	add := func(x string) {
		x = strings.TrimSpace(x)
		if x == "" || seen[x] {
			return
		}
		seen[x] = true
		out = append(out, x)
	}
	for _, x := range f {
		x = strings.TrimSpace(x)
		if x == "" {
			continue
		}
		add(x)
		runes := []rune(x)
		hasHan := false
		for _, rr := range runes {
			if unicode.Is(unicode.Han, rr) {
				hasHan = true
				break
			}
		}
		// Chinese queries commonly arrive without whitespace. Bigrams/trigrams
		// provide a portable sparse fallback when SQLite FTS tokenizers are not
		// available, while the original token is retained for exact phrases.
		if hasHan && len(runes) >= 3 {
			for n := 2; n <= 3; n++ {
				for i := 0; i+n <= len(runes); i++ {
					add(string(runes[i : i+n]))
					if len(out) >= 24 {
						break
					}
				}
				if len(out) >= 24 {
					break
				}
			}
		}
		if len(out) >= 24 {
			break
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		// Prefer more specific terms before short CJK n-grams.
		return len([]rune(out[i])) > len([]rune(out[j]))
	})
	if len(out) > 24 {
		out = out[:24]
	}
	return out
}

func (r *Repository) ResetRevisionData(tenantID, sourceID string, revision int64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := r.requireWritableRevision(tx, tenantID, sourceID, revision); err != nil {
			return err
		}
		if err := r.requireQuiescentRevisionOutbox(tx, tenantID, sourceID, revision, time.Now().UTC()); err != nil {
			return err
		}
		if err := r.enqueueSupersededVectorDeletes(tx, tenantID, sourceID, revision, time.Now().UTC()); err != nil {
			return err
		}
		if err := tenantScope(tx.Model(&IndexOutbox{}), tenantID).Where("aggregate_type = 'knowledge_source' AND aggregate_id = ? AND source_revision = ? AND operation = 'upsert'", sourceID, revision).Delete(&IndexOutbox{}).Error; err != nil {
			return err
		}
		if err := tenantScope(tx.Model(&Chunk{}), tenantID).Where("source_id = ? AND source_revision = ?", sourceID, revision).Delete(&Chunk{}).Error; err != nil {
			return err
		}
		return tenantScope(tx.Model(&DocumentUnit{}), tenantID).Where("source_id = ? AND source_revision = ?", sourceID, revision).Delete(&DocumentUnit{}).Error
	})
}

func (r *Repository) InsertUnitBundle(unit DocumentUnit, chunks []Chunk, outbox *IndexOutbox) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := r.requireWritableRevision(tx, unit.TenantID, unit.SourceID, unit.SourceRevision); err != nil {
			return err
		}
		for _, chunk := range chunks {
			if chunk.TenantID != unit.TenantID || chunk.SourceID != unit.SourceID || chunk.SourceRevision != unit.SourceRevision || chunk.UnitID != unit.ID {
				return fmt.Errorf("knowledge chunk does not belong to unit revision")
			}
		}
		if outbox != nil && (outbox.TenantID != unit.TenantID || outbox.AggregateType != "knowledge_source" || outbox.AggregateID != unit.SourceID || outbox.SourceRevision != unit.SourceRevision || outbox.Operation != "upsert") {
			return fmt.Errorf("knowledge outbox does not belong to unit revision")
		}
		if err := tx.Create(&unit).Error; err != nil {
			return err
		}
		if len(chunks) > 0 {
			if err := tx.CreateInBatches(chunks, 100).Error; err != nil {
				return err
			}
		}
		if outbox != nil {
			return tx.Create(outbox).Error
		}
		return nil
	})
}
func (r *Repository) CountChildChunks(tenantID, sourceID string, revision int64) (int64, error) {
	var n int64
	err := tenantScope(r.db.Model(&Chunk{}), tenantID).Where("source_id = ? AND source_revision = ? AND chunk_index >= 0", sourceID, revision).Count(&n).Error
	return n, err
}
func (r *Repository) CountIndexedChunks(tenantID, sourceID string, revision int64) (int64, error) {
	var n int64
	err := tenantScope(r.db.Model(&Chunk{}), tenantID).Where("source_id = ? AND source_revision = ? AND chunk_index >= 0 AND status = 'indexed'", sourceID, revision).Count(&n).Error
	return n, err
}
func (r *Repository) PendingOutboxesForSourceRevision(tenantID, sourceID string, revision int64, limit int) ([]IndexOutbox, error) {
	if limit <= 0 {
		limit = 1000
	}
	var xs []IndexOutbox
	now := time.Now().UTC()
	maxRetries := r.settings.OutboxMaxRetries
	if maxRetries <= 0 {
		maxRetries = defaultRuntimeSettings().OutboxMaxRetries
	}
	q := tenantScope(r.db.Model(&IndexOutbox{}), tenantID).
		Where("aggregate_type = 'knowledge_source' AND aggregate_id = ?", sourceID).
		Where("source_revision = ?", revision).
		Where("status IN ? AND retry_count < ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)", []string{"pending", "failed"}, maxRetries, now)
	err := q.Order("created_at ASC").Limit(limit).Find(&xs).Error
	return xs, err
}

func (r *Repository) CountUnits(tenantID, sourceID string, revision int64) (int64, error) {
	var n int64
	err := tenantScope(r.db.Model(&DocumentUnit{}), tenantID).Where("source_id = ? AND source_revision = ?", sourceID, revision).Count(&n).Error
	return n, err
}

func (r *Repository) CountChunks(tenantID, sourceID string, revision int64) (int64, error) {
	var n int64
	err := tenantScope(r.db.Model(&Chunk{}), tenantID).Where("source_id = ? AND source_revision = ? AND chunk_index >= 0", sourceID, revision).Count(&n).Error
	return n, err
}

func (r *Repository) AllChunksForSource(tenantID, sourceID string) ([]Chunk, error) {
	var xs []Chunk
	err := tenantScope(r.db.Model(&Chunk{}), tenantID).Where("source_id = ? AND chunk_index >= 0", sourceID).Find(&xs).Error
	if xs == nil {
		xs = []Chunk{}
	}
	return xs, err
}

func (r *Repository) PrepareRevisionIndexJob(job *IngestJob, batchSize int) error {
	if job == nil {
		return errors.New("knowledge index job required")
	}
	if job.Kind != JobKindReindex && job.Kind != JobKindRollback {
		return fmt.Errorf("unsupported knowledge index job kind %q", job.Kind)
	}
	if batchSize <= 0 {
		batchSize = r.settings.QdrantBatchSize
		if batchSize <= 0 {
			batchSize = defaultRuntimeSettings().QdrantBatchSize
		}
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := r.requireWritableRevision(tx, job.TenantID, job.SourceID, job.SourceRevision); err != nil {
			return err
		}
		if err := r.requireQuiescentRevisionOutbox(tx, job.TenantID, job.SourceID, job.SourceRevision, time.Now().UTC()); err != nil {
			return err
		}
		var active int64
		if err := tenantScope(tx.Model(&IngestJob{}), job.TenantID).
			Where("source_id = ? AND status IN ?", job.SourceID, []string{JobStatusQueued, JobStatusRunning}).Count(&active).Error; err != nil {
			return err
		}
		if active > 0 {
			return fmt.Errorf("source already has an active knowledge job")
		}
		var sourceRevision SourceRevision
		if err := tenantScope(tx.Model(&SourceRevision{}), job.TenantID).
			Where("source_id = ? AND revision = ?", job.SourceID, job.SourceRevision).First(&sourceRevision).Error; err != nil {
			return err
		}
		var chunks []Chunk
		if err := tenantScope(tx.Model(&Chunk{}), job.TenantID).
			Where("source_id = ? AND source_revision = ? AND chunk_index >= 0", job.SourceID, job.SourceRevision).
			Order("unit_id ASC, chunk_index ASC").Find(&chunks).Error; err != nil {
			return err
		}
		if len(chunks) == 0 {
			return fmt.Errorf("source has no parsed chunks to reindex")
		}
		if err := tenantScope(tx.Model(&IndexOutbox{}), job.TenantID).
			Where("aggregate_type = 'knowledge_source' AND aggregate_id = ? AND source_revision = ? AND status IN ? AND operation = 'upsert'", job.SourceID, job.SourceRevision, []string{"pending", "failed"}).
			Delete(&IndexOutbox{}).Error; err != nil {
			return err
		}
		ids := make([]string, 0, batchSize)
		flush := func() error {
			if len(ids) == 0 {
				return nil
			}
			payload, _ := json.Marshal(IndexPayload{ChunkIDs: append([]string(nil), ids...), IndexVersion: sourceRevision.IndexVersion})
			now := time.Now().UTC()
			outbox := IndexOutbox{ID: uuid.NewString(), TenantID: job.TenantID, AggregateType: "knowledge_source", AggregateID: job.SourceID, SourceRevision: job.SourceRevision, Operation: "upsert", PayloadJSON: string(payload), Status: "pending", CreatedAt: now, UpdatedAt: now}
			if err := tx.Create(&outbox).Error; err != nil {
				return err
			}
			ids = ids[:0]
			return nil
		}
		for _, chunk := range chunks {
			ids = append(ids, chunk.ID)
			if len(ids) >= batchSize {
				if err := flush(); err != nil {
					return err
				}
			}
		}
		if err := flush(); err != nil {
			return err
		}
		if err := tenantScope(tx.Model(&Chunk{}), job.TenantID).
			Where("source_id = ? AND source_revision = ? AND chunk_index >= 0", job.SourceID, job.SourceRevision).
			Updates(map[string]any{"status": "parsed", "updated_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
		return tx.Create(job).Error
	})
}

func (r *Repository) PrepareReindex(job *IngestJob, batchSize int) error {
	if job != nil && strings.TrimSpace(job.Kind) == "" {
		job.Kind = JobKindReindex
	}
	return r.PrepareRevisionIndexJob(job, batchSize)
}

func (r *Repository) RecordBenchmarkRun(tenantID string, summary BenchmarkSummary) (*BenchmarkRunSummary, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, fmt.Errorf("tenant scope required")
	}
	scenarioJSON, err := json.Marshal(summary.ScenarioCounts)
	if err != nil {
		return nil, fmt.Errorf("marshal benchmark scenario counts: %w", err)
	}
	violationsJSON, err := json.Marshal(summary.Violations)
	if err != nil {
		return nil, fmt.Errorf("marshal benchmark violations: %w", err)
	}
	now := time.Now().UTC()
	record := BenchmarkRunRecord{
		ID: uuid.NewString(), TenantID: tenantID, ConfigHash: strings.TrimSpace(summary.ConfigHash), K: summary.K,
		Cases: summary.Cases, SuccessfulCases: summary.SuccessfulCases, ErrorCases: summary.ErrorCases, FailedCases: summary.FailedCases,
		PositiveCases: summary.PositiveCases, NoAnswerCases: summary.NoAnswerCases, CitationObservedCases: summary.CitationObservedCases,
		ScenarioCountsJSON: string(scenarioJSON), RecallAtK: summary.RecallAtK, MRR: summary.MRR, NDCGAtK: summary.NDCGAtK,
		ContextPrecision: summary.ContextPrecision, ContextRecall: summary.ContextRecall, NoAnswerAccuracy: summary.NoAnswerAccuracy,
		CitationPrecision: summary.CitationPrecision, CitationRecall: summary.CitationRecall, Groundedness: summary.Groundedness,
		ErrorRate: summary.ErrorRate, P50LatencyMs: summary.P50LatencyMs, P95LatencyMs: summary.P95LatencyMs,
		Passed: summary.Passed, ViolationsJSON: string(violationsJSON), CreatedAt: now,
	}
	if err := r.db.Create(&record).Error; err != nil {
		return nil, err
	}
	out := benchmarkRunSummaryFromRecord(record)
	return &out, nil
}

func (r *Repository) ListBenchmarkRuns(tenantID string, limit int) ([]BenchmarkRunSummary, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, fmt.Errorf("tenant scope required")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	var rows []BenchmarkRunRecord
	if err := tenantScope(r.db.Model(&BenchmarkRunRecord{}), tenantID).
		Order("created_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]BenchmarkRunSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, benchmarkRunSummaryFromRecord(row))
	}
	return out, nil
}

func (r *Repository) DeleteRetrievalTracesBefore(tenantID string, before time.Time, limit int) (int64, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return 0, fmt.Errorf("tenant scope required")
	}
	if limit <= 0 {
		return 0, nil
	}
	var ids []string
	if err := tenantScope(r.db.Table("retrieval_traces"), tenantID).
		Where("created_at < ?", before.UTC()).Order("created_at ASC").Limit(limit).Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.db.Exec("DELETE FROM retrieval_traces WHERE tenant_id = ? AND id IN ?", tenantID, ids)
	return result.RowsAffected, result.Error
}

func (r *Repository) DeleteBenchmarkRunsBefore(tenantID string, before time.Time, limit int) (int64, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return 0, fmt.Errorf("tenant scope required")
	}
	if limit <= 0 {
		return 0, nil
	}
	var ids []string
	if err := tenantScope(r.db.Model(&BenchmarkRunRecord{}), tenantID).
		Where("created_at < ?", before.UTC()).Order("created_at ASC").Limit(limit).Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	result := tenantScope(r.db.Model(&BenchmarkRunRecord{}), tenantID).Where("id IN ?", ids).Delete(&BenchmarkRunRecord{})
	return result.RowsAffected, result.Error
}

func benchmarkRunSummaryFromRecord(record BenchmarkRunRecord) BenchmarkRunSummary {
	scenarios := map[string]int{}
	if strings.TrimSpace(record.ScenarioCountsJSON) != "" {
		if err := json.Unmarshal([]byte(record.ScenarioCountsJSON), &scenarios); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
	}
	if scenarios == nil {
		scenarios = map[string]int{}
	}
	violations := []string{}
	if strings.TrimSpace(record.ViolationsJSON) != "" {
		if err := json.Unmarshal([]byte(record.ViolationsJSON), &violations); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
	}
	if violations == nil {
		violations = []string{}
	}
	return BenchmarkRunSummary{
		ID: record.ID, ConfigHash: record.ConfigHash, K: record.K, Cases: record.Cases,
		SuccessfulCases: record.SuccessfulCases, ErrorCases: record.ErrorCases, FailedCases: record.FailedCases,
		PositiveCases: record.PositiveCases, NoAnswerCases: record.NoAnswerCases, CitationObservedCases: record.CitationObservedCases,
		ScenarioCounts: scenarios, RecallAtK: record.RecallAtK, MRR: record.MRR, NDCGAtK: record.NDCGAtK,
		ContextPrecision: record.ContextPrecision, ContextRecall: record.ContextRecall, NoAnswerAccuracy: record.NoAnswerAccuracy,
		CitationPrecision: record.CitationPrecision, CitationRecall: record.CitationRecall, Groundedness: record.Groundedness,
		ErrorRate: record.ErrorRate, P50LatencyMs: record.P50LatencyMs, P95LatencyMs: record.P95LatencyMs,
		Passed: record.Passed, Violations: violations, CreatedAt: record.CreatedAt,
	}
}
