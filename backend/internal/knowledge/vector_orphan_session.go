package knowledge

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
	"gorm.io/gorm"
	amitialog "log"
)

const orphanAuditHistoryLifetime = 7 * 24 * time.Hour

type OrphanAuditSessionTotals struct {
	AuditID            string `json:"auditId"`
	Pages              int    `json:"pages"`
	Sampled            int    `json:"sampled"`
	SuspectedOrphans   int    `json:"suspectedOrphans"`
	MetadataMismatches int    `json:"metadataMismatches"`
	InFlight           int    `json:"inFlight"`
}

type OrphanAuditSessionSummary struct {
	OrphanAuditSessionTotals
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type orphanAuditSessionRow struct {
	ID                 string    `gorm:"column:id;primaryKey"`
	TenantID           string    `gorm:"column:tenant_id"`
	CollectionsHash    string    `gorm:"column:collections_hash"`
	PageLimit          int       `gorm:"column:page_limit"`
	Page               int       `gorm:"column:page"`
	NextCursor         string    `gorm:"column:next_cursor"`
	LastCursor         string    `gorm:"column:last_cursor"`
	LastResultJSON     string    `gorm:"column:last_result_json"`
	Sampled            int       `gorm:"column:sampled"`
	SuspectedOrphans   int       `gorm:"column:suspected_orphans"`
	MetadataMismatches int       `gorm:"column:metadata_mismatches"`
	InFlight           int       `gorm:"column:in_flight"`
	Status             string    `gorm:"column:status"`
	CreatedAt          time.Time `gorm:"column:created_at"`
	UpdatedAt          time.Time `gorm:"column:updated_at"`
	ExpiresAt          time.Time `gorm:"column:expires_at"`
}

func (row orphanAuditSessionRow) totals() OrphanAuditSessionTotals {
	return OrphanAuditSessionTotals{
		AuditID: row.ID, Pages: row.Page, Sampled: row.Sampled,
		SuspectedOrphans: row.SuspectedOrphans, MetadataMismatches: row.MetadataMismatches, InFlight: row.InFlight,
	}
}

func (row orphanAuditSessionRow) summary() OrphanAuditSessionSummary {
	return OrphanAuditSessionSummary{
		OrphanAuditSessionTotals: row.totals(), Status: row.Status,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func sameAuditCursor(a, b string) bool {
	if len(a) == 0 || len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func (r *Repository) getOrphanAuditSession(ctx context.Context, tenantID, auditID string) (orphanAuditSessionRow, error) {
	var row orphanAuditSessionRow
	err := r.db.WithContext(ctx).Table("knowledge_vector_audit_sessions").
		Where("tenant_id = ? AND id = ?", tenantID, auditID).Take(&row).Error
	return row, err
}

func storedAuditPage(row orphanAuditSessionRow) (OrphanVectorAuditPageResult, error) {
	var result OrphanVectorAuditPageResult
	if err := json.Unmarshal([]byte(row.LastResultJSON), &result); err != nil || result.AuditID != row.ID || result.Page != row.Page {
		return result, fmt.Errorf("stored orphan audit page invalid")
	}
	result.Totals = nil
	totals := row.totals()
	result.Totals = &totals
	return result, nil
}

func (r *Repository) auditOrphanPageWithStore(ctx context.Context, tenantID string, collections []string, limit int, cursor string, now time.Time, scan orphanVectorPageScanner, load orphanStateLoader) (OrphanVectorAuditPageResult, error) {
	var empty OrphanVectorAuditPageResult
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	if strings.TrimSpace(tenantID) == "" || limit < 1 || limit > maxOrphanPageSize {
		return empty, fmt.Errorf("invalid orphan audit session request")
	}
	if len(collections) == 0 && cursor != "" {
		return empty, fmt.Errorf("orphan audit collections changed")
	}
	fingerprint := orphanAuditFingerprint(strings.Join(collections, "\x00"))
	var previous orphanAuditSessionRow
	if cursor != "" {
		state, err := readOrphanAuditCursor(cursor, tenantID, collections, now)
		if err != nil {
			return empty, err
		}
		previous, err = r.getOrphanAuditSession(ctx, tenantID, state.AuditID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return empty, fmt.Errorf("orphan audit session not found")
		}
		if err != nil {
			return empty, err
		}
		if previous.CollectionsHash != fingerprint || previous.PageLimit != limit || now.After(previous.ExpiresAt) {
			return empty, fmt.Errorf("orphan audit session expired or scope changed")
		}
		if state.Page < previous.Page {
			receipt, receiptErr := getAuditReceipt(ctx, r.db, tenantID, previous.ID, state.Page+1)
			if receiptErr == nil {
				if receipt.RequestHash != hashAuditBytes([]byte(cursor)) {
					return empty, fmt.Errorf("orphan audit replay request mismatch")
				}
				return auditReceiptResult(receipt)
			}
			if !errors.Is(receiptErr, gorm.ErrRecordNotFound) {
				return empty, receiptErr
			}
			if sameAuditCursor(cursor, previous.LastCursor) {
				return storedAuditPage(previous)
			}
			return empty, fmt.Errorf("orphan audit historic page unavailable")
		}
		if !sameAuditCursor(cursor, previous.NextCursor) || previous.Status != "running" || previous.Page != state.Page {
			return empty, fmt.Errorf("orphan audit continuation rejected")
		}
	}
	result, err := auditTenantOrphanVectorPage(ctx, tenantID, collections, limit, cursor, now, scan, load)
	if len(collections) == 0 && err == nil {
		result.AuditID = uuid.NewString()
		result.Page = 1
		result.Finished = true
	}
	if err != nil {
		return empty, err
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	status := "running"
	if result.Finished {
		status = "finished"
	} else if result.Truncated {
		status = "truncated"
	}
	page := orphanAuditSessionRow{
		ID: result.AuditID, TenantID: tenantID, CollectionsHash: fingerprint, PageLimit: limit,
		Page: result.Page, NextCursor: result.NextCursor, LastCursor: cursor,
		Sampled: result.Sampled, SuspectedOrphans: result.SuspectedOrphans,
		MetadataMismatches: result.MetadataMismatches, InFlight: result.InFlight,
		Status: status, CreatedAt: now.UTC(), UpdatedAt: now.UTC(),
		ExpiresAt: now.Add(orphanAuditCursorLifetime).UTC(),
	}
	if cursor != "" {
		page.Sampled += previous.Sampled
		page.SuspectedOrphans += previous.SuspectedOrphans
		page.MetadataMismatches += previous.MetadataMismatches
		page.InFlight += previous.InFlight
		page.CreatedAt = previous.CreatedAt
		page.ExpiresAt = previous.ExpiresAt
	}
	totals := page.totals()
	result.Totals = &totals
	serialized, err := json.Marshal(result)
	if err != nil {
		return empty, err
	}
	page.LastResultJSON = string(serialized)
	commitErr := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		previousHash := orphanAuditGenesisHash
		if cursor == "" {
			if err := tx.Table("knowledge_vector_audit_sessions").Create(&page).Error; err != nil {
				return err
			}
		} else {
			var err error
			previousHash, err = auditReceiptPreviousHash(ctx, tx, previous)
			if err != nil {
				return err
			}
			updates := map[string]any{
				"page": page.Page, "next_cursor": page.NextCursor, "last_cursor": page.LastCursor,
				"last_result_json": page.LastResultJSON, "sampled": page.Sampled,
				"suspected_orphans": page.SuspectedOrphans, "metadata_mismatches": page.MetadataMismatches,
				"in_flight": page.InFlight, "status": page.Status, "updated_at": page.UpdatedAt,
			}
			committed := tx.Table("knowledge_vector_audit_sessions").
				Where("id = ? AND tenant_id = ? AND page = ? AND next_cursor = ? AND status = 'running'", page.ID, tenantID, previous.Page, cursor).
				Updates(updates)
			if committed.Error != nil {
				return committed.Error
			}
			if committed.RowsAffected != 1 {
				return fmt.Errorf("orphan audit page changed during scan")
			}
		}
		receipt := makeAuditReceipt(tenantID, page.ID, page.Page, cursor, previousHash, page.LastResultJSON, now)
		return tx.Table("knowledge_vector_audit_pages").Create(&receipt).Error
	})
	if commitErr != nil {
		if cursor != "" {
			receipt, err := getAuditReceipt(ctx, r.db, tenantID, page.ID, page.Page)
			if err == nil && receipt.RequestHash == hashAuditBytes([]byte(cursor)) {
				return auditReceiptResult(receipt)
			}
		}
		return empty, commitErr
	}
	if cursor == "" {
		if err := r.db.WithContext(ctx).Exec(`DELETE FROM knowledge_vector_recovery_review_events
WHERE tenant_id = ? AND review_id IN (SELECT id FROM knowledge_vector_recovery_reviews WHERE tenant_id = ? AND audit_id IN
(SELECT id FROM knowledge_vector_audit_sessions WHERE tenant_id = ? AND updated_at < ?))`, tenantID, tenantID, tenantID, now.Add(-orphanAuditHistoryLifetime)).Error; err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		if err := r.db.WithContext(ctx).Table("knowledge_vector_recovery_reviews").
			Where("tenant_id = ? AND audit_id IN (SELECT id FROM knowledge_vector_audit_sessions WHERE tenant_id = ? AND updated_at < ?)", tenantID, tenantID, now.Add(-orphanAuditHistoryLifetime)).
			Delete(&RecoveryReviewRequest{}).Error; err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		if err := r.db.WithContext(ctx).Table("knowledge_vector_audit_recheck_results").
			Where("tenant_id = ? AND audit_id IN (SELECT id FROM knowledge_vector_audit_sessions WHERE tenant_id = ? AND updated_at < ?)", tenantID, tenantID, now.Add(-orphanAuditHistoryLifetime)).
			Delete(&OrphanAuditRecheckRecord{}).Error; err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		if err := r.db.WithContext(ctx).Exec(`DELETE FROM knowledge_vector_audit_recheck_leases
WHERE tenant_id = ? AND audit_id IN (SELECT id FROM knowledge_vector_audit_sessions WHERE tenant_id = ? AND updated_at < ?)`, tenantID, tenantID, now.Add(-orphanAuditHistoryLifetime)).Error; err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		if err := r.db.WithContext(ctx).Table("knowledge_vector_audit_pages").
			Where("tenant_id = ? AND audit_id IN (SELECT id FROM knowledge_vector_audit_sessions WHERE tenant_id = ? AND updated_at < ?)", tenantID, tenantID, now.Add(-orphanAuditHistoryLifetime)).
			Delete(&orphanAuditPageReceiptRow{}).Error; err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		if err := r.db.WithContext(ctx).Table("knowledge_vector_audit_sessions").
			Where("tenant_id = ? AND updated_at < ?", tenantID, now.Add(-orphanAuditHistoryLifetime)).Delete(&orphanAuditSessionRow{}).Error; err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
	}
	return result, nil
}

func (s *service) ListOrphanAuditSessions(ctx context.Context, tenantID string, limit int) ([]OrphanAuditSessionSummary, error) {
	items := make([]OrphanAuditSessionSummary, 0)
	if strings.TrimSpace(tenantID) == "" {
		return items, fmt.Errorf("tenant scope required")
	}
	if limit < 1 || limit > 30 {
		return items, fmt.Errorf("invalid audit history page size")
	}
	var rows []orphanAuditSessionRow
	err := s.repo.db.WithContext(ctx).Table("knowledge_vector_audit_sessions").
		Select("id, page, sampled, suspected_orphans, metadata_mismatches, in_flight, status, created_at, updated_at, expires_at").
		Where("tenant_id = ? AND updated_at >= ?", tenantID, time.Now().UTC().Add(-orphanAuditHistoryLifetime)).
		Order("updated_at DESC, id DESC").Limit(limit).Find(&rows).Error
	if err != nil {
		return items, err
	}
	for _, row := range rows {
		summary := row.summary()
		if row.Status == "running" && time.Now().UTC().After(row.ExpiresAt) {
			summary.Status = "expired"
		}
		items = append(items, summary)
	}
	return items, nil
}

var _ orphanVectorPageScanner = qdrantDB.ScanTenantVectorMetadataPageWithContext
