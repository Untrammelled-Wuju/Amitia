package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
)

type OrphanAuditRecheckResult struct {
	AuditID               string    `json:"auditId"`
	RecheckID             string    `json:"recheckId,omitempty"`
	ManualRecoveryAllowed bool      `json:"manualRecoveryAllowed"`
	Page                  int       `json:"page"`
	Status                string    `json:"status"`
	ObservedAt            time.Time `json:"observedAt"`
	ChainAnchored         bool      `json:"chainAnchored"`
	SampleMatched         bool      `json:"sampleMatched"`
	CountsMatched         bool      `json:"countsMatched"`
	PreviousSampled       int       `json:"previousSampled"`
	CurrentSampled        int       `json:"currentSampled"`
	PreviousSuspected     int       `json:"previousSuspected"`
	CurrentSuspected      int       `json:"currentSuspected"`
}

func (r *Repository) recheckOrphanAuditLatestWithSources(ctx context.Context, tenantID, auditID string, now time.Time, collections []string, scan orphanVectorPageScanner, load orphanStateLoader) (OrphanAuditRecheckResult, error) {
	result := OrphanAuditRecheckResult{AuditID: auditID, Status: "unavailable", ObservedAt: now.UTC()}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if strings.TrimSpace(tenantID) == "" || len(collections) > 0 && scan == nil || load == nil {
		return result, fmt.Errorf("invalid orphan audit recheck request")
	}
	if _, err := uuid.Parse(auditID); err != nil {
		return result, fmt.Errorf("invalid orphan audit identifier")
	}
	row, err := r.getOrphanAuditSession(ctx, tenantID, auditID)
	if err != nil {
		return result, err
	}
	result.Page = row.Page
	if row.Page < 1 || row.Page > maxOrphanAuditPages {
		return result, fmt.Errorf("invalid orphan audit checkpoint")
	}
	if row.CollectionsHash != orphanAuditFingerprint(strings.Join(collections, "\x00")) {
		result.Status = "collections_changed"
		return result, nil
	}
	_, anchored, err := auditReceiptAnchoredPrefix(ctx, r.db, tenantID, auditID, row.Page)
	if err != nil {
		return result, err
	}
	result.ChainAnchored = anchored
	if !anchored {
		result.Status = "unanchored"
		return result, nil
	}
	receipt, err := getAuditReceipt(ctx, r.db, tenantID, auditID, row.Page)
	if err != nil {
		return result, err
	}
	if !receipt.valid() || receipt.ResultJSON != row.LastResultJSON || receipt.RequestHash != hashAuditBytes([]byte(row.LastCursor)) {
		result.Status = "checkpoint_mismatch"
		return result, nil
	}
	var previous OrphanVectorAuditPageResult
	if err := json.Unmarshal([]byte(receipt.ResultJSON), &previous); err != nil {
		return result, err
	}
	result.PreviousSampled = previous.Sampled
	result.PreviousSuspected = previous.SuspectedOrphans
	if len(previous.ObservationDigest) != 64 {
		result.Status = "legacy_no_digest"
		return result, nil
	}
	if now.After(row.ExpiresAt) {
		result.Status = "expired"
		return result, nil
	}
	if row.LastCursor != "" {
		if _, err := readOrphanAuditCursor(row.LastCursor, tenantID, collections, now); err != nil {
			result.Status = "expired"
			return result, nil
		}
	}
	current, err := auditTenantOrphanVectorPage(ctx, tenantID, collections, row.PageLimit, row.LastCursor, now, scan, load)
	if err != nil {
		return result, err
	}
	result.CurrentSampled = current.Sampled
	result.CurrentSuspected = current.SuspectedOrphans
	result.SampleMatched = previous.ObservationDigest == current.ObservationDigest
	result.CountsMatched = previous.Sampled == current.Sampled && previous.SuspectedOrphans == current.SuspectedOrphans &&
		previous.MetadataMismatches == current.MetadataMismatches && previous.InFlight == current.InFlight &&
		previous.CollectionsChecked == current.CollectionsChecked && previous.CollectionsKnown == current.CollectionsKnown
	if result.SampleMatched && result.CountsMatched {
		result.Status = "matched"
	} else {
		result.Status = "changed"
	}
	return result, nil
}

func (s *service) RecheckOrphanAuditLatest(ctx context.Context, tenantID, auditID string) (OrphanAuditRecheckResult, error) {
	return s.repo.recheckAuditWithLease(ctx, tenantID, auditID, time.Now().UTC(), func(scanCtx context.Context) ([]string, error) {
		return qdrantDB.ListKnowledgeCollectionNamesWithContext(scanCtx, configuredKnowledgeCollectionBase())
	}, qdrantDB.ScanTenantVectorMetadataPageWithContext, s.repo.orphanAuditState)
}
