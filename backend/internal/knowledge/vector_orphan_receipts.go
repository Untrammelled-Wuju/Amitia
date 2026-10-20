package knowledge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const orphanAuditGenesisHash = "0000000000000000000000000000000000000000000000000000000000000000"

type orphanAuditPageReceiptRow struct {
	AuditID      string    `gorm:"column:audit_id;primaryKey"`
	TenantID     string    `gorm:"column:tenant_id"`
	Page         int       `gorm:"column:page;primaryKey"`
	RequestHash  string    `gorm:"column:request_hash"`
	ResultJSON   string    `gorm:"column:result_json"`
	ResultHash   string    `gorm:"column:result_hash"`
	PreviousHash string    `gorm:"column:previous_hash"`
	ReceiptHash  string    `gorm:"column:receipt_hash"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}

type OrphanAuditPageReceipt struct {
	AuditID      string    `json:"auditId"`
	Page         int       `json:"page"`
	RequestHash  string    `json:"requestHash"`
	ResultHash   string    `json:"resultHash"`
	PreviousHash string    `json:"previousHash"`
	ReceiptHash  string    `json:"receiptHash"`
	CreatedAt    time.Time `json:"createdAt"`
	Verified     bool      `json:"verified"`
}

type OrphanAuditReceiptsResult struct {
	AuditID       string                   `json:"auditId"`
	Items         []OrphanAuditPageReceipt `json:"items"`
	NextPage      int                      `json:"nextPage"`
	HasMore       bool                     `json:"hasMore"`
	ChainAnchored bool                     `json:"chainAnchored"`
}

func hashAuditBytes(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func auditReceiptDigest(row orphanAuditPageReceiptRow) string {
	canonical, _ := json.Marshal(struct {
		Version      int    `json:"v"`
		AuditID      string `json:"a"`
		TenantID     string `json:"t"`
		Page         int    `json:"p"`
		RequestHash  string `json:"q"`
		ResultHash   string `json:"r"`
		PreviousHash string `json:"prev"`
	}{1, row.AuditID, row.TenantID, row.Page, row.RequestHash, row.ResultHash, row.PreviousHash})
	return hashAuditBytes(canonical)
}

func makeAuditReceipt(tenantID, auditID string, page int, cursor, previousHash, resultJSON string, now time.Time) orphanAuditPageReceiptRow {
	row := orphanAuditPageReceiptRow{
		AuditID: auditID, TenantID: tenantID, Page: page,
		RequestHash: hashAuditBytes([]byte(cursor)), ResultJSON: resultJSON,
		ResultHash: hashAuditBytes([]byte(resultJSON)), PreviousHash: previousHash,
		CreatedAt: now.UTC(),
	}
	row.ReceiptHash = auditReceiptDigest(row)
	return row
}

func (row orphanAuditPageReceiptRow) valid() bool {
	if row.Page < 1 || row.AuditID == "" || strings.TrimSpace(row.TenantID) == "" || len(row.PreviousHash) != 64 ||
		hashAuditBytes([]byte(row.ResultJSON)) != row.ResultHash || auditReceiptDigest(row) != row.ReceiptHash {
		return false
	}
	var payload OrphanVectorAuditPageResult
	return json.Unmarshal([]byte(row.ResultJSON), &payload) == nil && payload.AuditID == row.AuditID && payload.Page == row.Page && payload.Totals != nil && payload.Totals.Pages == row.Page && payload.Totals.AuditID == row.AuditID
}

func (row orphanAuditPageReceiptRow) public(verified bool) OrphanAuditPageReceipt {
	return OrphanAuditPageReceipt{
		AuditID: row.AuditID, Page: row.Page, RequestHash: row.RequestHash,
		ResultHash: row.ResultHash, PreviousHash: row.PreviousHash,
		ReceiptHash: row.ReceiptHash, CreatedAt: row.CreatedAt, Verified: verified,
	}
}

func auditReceiptResult(row orphanAuditPageReceiptRow) (OrphanVectorAuditPageResult, error) {
	var result OrphanVectorAuditPageResult
	if !row.valid() {
		return result, fmt.Errorf("orphan audit receipt verification failed")
	}
	if err := json.Unmarshal([]byte(row.ResultJSON), &result); err != nil {
		return result, err
	}
	return result, nil
}

func getAuditReceipt(ctx context.Context, db *gorm.DB, tenantID, auditID string, page int) (orphanAuditPageReceiptRow, error) {
	var receipt orphanAuditPageReceiptRow
	err := db.WithContext(ctx).Table("knowledge_vector_audit_pages").
		Where("tenant_id = ? AND audit_id = ? AND page = ?", tenantID, auditID, page).Take(&receipt).Error
	return receipt, err
}

func auditReceiptPreviousHash(ctx context.Context, db *gorm.DB, previous orphanAuditSessionRow) (string, error) {
	if previous.Page == 0 {
		return orphanAuditGenesisHash, nil
	}
	prior, err := getAuditReceipt(ctx, db, previous.TenantID, previous.ID, previous.Page)
	if err == nil {
		if !prior.valid() {
			return "", fmt.Errorf("prior orphan audit receipt corrupted")
		}
		return prior.ReceiptHash, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	if previous.LastResultJSON == "" {
		return "", fmt.Errorf("missing legacy orphan audit checkpoint")
	}
	return hashAuditBytes([]byte("legacy:" + previous.ID + ":" + previous.LastResultJSON)), nil
}

func auditReceiptAnchoredPrefix(ctx context.Context, db *gorm.DB, tenantID, auditID string, throughPage int) (string, bool, error) {
	if throughPage < 0 || throughPage > maxOrphanAuditPages {
		return "", false, fmt.Errorf("invalid orphan audit chain extent")
	}
	prior := orphanAuditGenesisHash
	if throughPage == 0 {
		return prior, true, nil
	}
	var receipts []orphanAuditPageReceiptRow
	if err := db.WithContext(ctx).Table("knowledge_vector_audit_pages").
		Where("tenant_id = ? AND audit_id = ? AND page >= 1 AND page <= ?", tenantID, auditID, throughPage).
		Order("page ASC").Limit(throughPage).Find(&receipts).Error; err != nil {
		return "", false, err
	}
	if len(receipts) != throughPage {
		return "", false, nil
	}
	for i, receipt := range receipts {
		if err := ctx.Err(); err != nil {
			return "", false, err
		}
		if receipt.Page != i+1 || receipt.PreviousHash != prior || !receipt.valid() {
			return "", false, nil
		}
		prior = receipt.ReceiptHash
	}
	return prior, true, nil
}

func (s *service) ListOrphanAuditPageReceipts(ctx context.Context, tenantID, auditID string, afterPage, limit int) (OrphanAuditReceiptsResult, error) {
	result := OrphanAuditReceiptsResult{AuditID: auditID, Items: make([]OrphanAuditPageReceipt, 0)}
	if strings.TrimSpace(tenantID) == "" || afterPage < 0 || afterPage >= maxOrphanAuditPages || limit < 1 || limit > 50 {
		return result, fmt.Errorf("invalid orphan audit receipt request")
	}
	if _, err := uuid.Parse(auditID); err != nil {
		return result, fmt.Errorf("invalid orphan audit identifier")
	}
	session, err := s.repo.getOrphanAuditSession(ctx, tenantID, auditID)
	if err != nil {
		return result, err
	}
	var rows []orphanAuditPageReceiptRow
	err = s.repo.db.WithContext(ctx).Table("knowledge_vector_audit_pages").
		Where("tenant_id = ? AND audit_id = ? AND page > ?", tenantID, auditID, afterPage).
		Order("page ASC").Limit(limit + 1).Find(&rows).Error
	if err != nil {
		return result, err
	}
	result.HasMore = len(rows) > limit
	if result.HasMore {
		rows = rows[:limit]
	}
	prior, anchored, err := auditReceiptAnchoredPrefix(ctx, s.repo.db, tenantID, auditID, afterPage)
	if err != nil {
		return result, err
	}
	result.ChainAnchored = anchored
	next := afterPage
	sequential := anchored
	if !anchored && afterPage > 0 {
		previous, previousErr := getAuditReceipt(ctx, s.repo.db, tenantID, auditID, afterPage)
		if previousErr == nil && previous.valid() {
			prior = previous.ReceiptHash
		}
	}
	for _, row := range rows {
		valid := row.valid() && row.Page == next+1 && row.PreviousHash == prior
		sequential = sequential && valid
		if !sequential {
			result.ChainAnchored = false
		}
		result.Items = append(result.Items, row.public(valid && sequential))
		next = row.Page
		prior = row.ReceiptHash
	}
	result.NextPage = next
	if afterPage < session.Page && len(rows) == 0 {
		result.ChainAnchored = false
	}
	if len(rows) == 0 && afterPage > session.Page {
		return result, fmt.Errorf("orphan audit receipt page out of bounds")
	}
	return result, nil
}
