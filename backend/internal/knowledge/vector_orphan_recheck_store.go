package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrOrphanRecheckCooldown = errors.New("orphan audit recheck cooldown")

const orphanRecheckScanTimeout = 12 * time.Second
const orphanRecheckLeaseLifetime = 20 * time.Second
const orphanRecheckCooldown = 30 * time.Second

type orphanRecheckLeaseRow struct {
	AuditID        string    `gorm:"column:audit_id;primaryKey"`
	TenantID       string    `gorm:"column:tenant_id"`
	LeaseToken     string    `gorm:"column:lease_token"`
	LeaseExpiresAt time.Time `gorm:"column:lease_expires_at"`
	NextAllowedAt  time.Time `gorm:"column:next_allowed_at"`
}

type OrphanAuditRecheckRecord struct {
	ID          string    `json:"id" gorm:"column:id;primaryKey"`
	TenantID    string    `json:"-" gorm:"column:tenant_id"`
	AuditID     string    `json:"auditId" gorm:"column:audit_id"`
	Page        int       `json:"page" gorm:"column:page"`
	Status      string    `json:"status" gorm:"column:status"`
	ResultHash  string    `json:"resultHash" gorm:"column:result_hash"`
	ReceiptHash string    `json:"receiptHash" gorm:"column:receipt_hash"`
	ObservedAt  time.Time `json:"observedAt" gorm:"column:observed_at"`
	CreatedAt   time.Time `json:"createdAt" gorm:"column:created_at"`
}

func (r *Repository) recheckAuditWithLease(ctx context.Context, tenantID, auditID string, now time.Time, discover func(context.Context) ([]string, error), scan orphanVectorPageScanner, load orphanStateLoader) (OrphanAuditRecheckResult, error) {
	var empty OrphanAuditRecheckResult
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	if strings.TrimSpace(tenantID) == "" || discover == nil || scan == nil || load == nil {
		return empty, fmt.Errorf("invalid recheck request")
	}
	if _, err := uuid.Parse(auditID); err != nil {
		return empty, fmt.Errorf("invalid audit identifier")
	}
	checkpoint, err := r.getOrphanAuditSession(ctx, tenantID, auditID)
	if err != nil {
		return empty, err
	}
	if checkpoint.Page < 1 {
		return empty, fmt.Errorf("audit has no committed page")
	}
	token := uuid.NewString()
	now = now.UTC()
	claim := r.db.WithContext(ctx).Exec(`INSERT INTO knowledge_vector_audit_recheck_leases
(audit_id, tenant_id, lease_token, lease_expires_at, next_allowed_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(audit_id) DO UPDATE SET lease_token = excluded.lease_token,
lease_expires_at = excluded.lease_expires_at, next_allowed_at = excluded.next_allowed_at
WHERE knowledge_vector_audit_recheck_leases.tenant_id = excluded.tenant_id
AND knowledge_vector_audit_recheck_leases.lease_expires_at <= ?
AND knowledge_vector_audit_recheck_leases.next_allowed_at <= ?`,
		auditID, tenantID, token, now.Add(orphanRecheckLeaseLifetime), now.Add(orphanRecheckCooldown), now, now)
	if claim.Error != nil {
		return empty, claim.Error
	}
	if claim.RowsAffected != 1 {
		return empty, ErrOrphanRecheckCooldown
	}
	scanCtx, cancel := context.WithTimeout(ctx, orphanRecheckScanTimeout)
	defer cancel()
	var result OrphanAuditRecheckResult
	collections, scanErr := discover(scanCtx)
	if scanErr == nil {
		result, scanErr = r.recheckOrphanAuditLatestWithSources(scanCtx, tenantID, auditID, now, collections, scan, load)
	}
	if result.AuditID == "" {
		result = OrphanAuditRecheckResult{AuditID: auditID, Page: checkpoint.Page, Status: "unavailable", ObservedAt: now}
	}
	if scanErr != nil {
		result.Status = "unavailable"
	}
	record := OrphanAuditRecheckRecord{
		ID: uuid.NewString(), TenantID: tenantID, AuditID: auditID, Page: result.Page,
		Status: result.Status, ObservedAt: now, CreatedAt: now,
		ReceiptHash: orphanAuditGenesisHash,
	}
	if receipt, receiptErr := getAuditReceipt(ctx, r.db, tenantID, auditID, result.Page); receiptErr == nil && receipt.valid() {
		record.ReceiptHash = receipt.ReceiptHash
	}
	result.RecheckID = record.ID
	result.ManualRecoveryAllowed = false
	serialized, err := json.Marshal(result)
	if err != nil {
		return empty, err
	}
	record.ResultHash = hashAuditBytes(serialized)
	commitCtx, stopCommit := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer stopCommit()
	err = r.db.WithContext(commitCtx).Transaction(func(tx *gorm.DB) error {
		var current orphanAuditSessionRow
		if err := tx.Table("knowledge_vector_audit_sessions").Where("id = ? AND tenant_id = ?", auditID, tenantID).Take(&current).Error; err != nil {
			return err
		}
		if current.Page != checkpoint.Page || current.LastResultJSON != checkpoint.LastResultJSON {
			return fmt.Errorf("audit checkpoint changed during recheck")
		}
		if err := tx.Table("knowledge_vector_audit_recheck_results").Create(&record).Error; err != nil {
			return err
		}
		released := tx.Exec(`UPDATE knowledge_vector_audit_recheck_leases
SET lease_expires_at = ? WHERE audit_id = ? AND tenant_id = ? AND lease_token = ?`, now, auditID, tenantID, token)
		if released.Error != nil {
			return released.Error
		}
		if released.RowsAffected != 1 {
			return fmt.Errorf("recheck lease ownership lost")
		}
		return nil
	})
	if err != nil {
		return empty, err
	}
	if scanErr != nil {
		return result, scanErr
	}
	return result, nil
}

func (s *service) ListOrphanAuditRechecks(ctx context.Context, tenantID, auditID string, limit int) ([]OrphanAuditRecheckRecord, error) {
	result := make([]OrphanAuditRecheckRecord, 0)
	if strings.TrimSpace(tenantID) == "" || limit < 1 || limit > 30 {
		return result, fmt.Errorf("invalid recheck history request")
	}
	if _, err := uuid.Parse(auditID); err != nil {
		return result, fmt.Errorf("invalid audit identifier")
	}
	if _, err := s.repo.getOrphanAuditSession(ctx, tenantID, auditID); err != nil {
		return result, err
	}
	err := s.repo.db.WithContext(ctx).Table("knowledge_vector_audit_recheck_results").
		Where("tenant_id = ? AND audit_id = ?", tenantID, auditID).
		Order("created_at DESC, id DESC").Limit(limit).Find(&result).Error
	return result, err
}
