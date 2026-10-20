package knowledge

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// GetSourceOutline exposes canonical unit structure without returning document
// bodies. It is intended for parser/locator diagnostics and always remains
// tenant/source/revision scoped.
func (s *service) GetSourceOutline(ctx context.Context, tenantID, sourceID string, revision, afterOrdinal int64, limit int) (UnitOutlineResponse, error) {
	resp := UnitOutlineResponse{SourceID: strings.TrimSpace(sourceID), Items: []UnitOutline{}}
	if s == nil || s.repo == nil {
		return resp, gorm.ErrInvalidDB
	}
	if ctx == nil {
		ctx = context.Background()
	}
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return resp, gorm.ErrRecordNotFound
	}
	source, err := s.repo.GetSource(tenantID, sourceID)
	if err != nil {
		return resp, err
	}
	if revision <= 0 {
		revision = source.Revision
	}
	revisionRow, err := s.repo.GetSourceRevision(tenantID, sourceID, revision)
	if err != nil {
		return resp, err
	}
	if revisionRow.PurgedAt != nil || revisionRow.Status == RevisionStatusPurged {
		return resp, fmt.Errorf("knowledge revision has been purged")
	}
	items, next, err := s.repo.UnitOutlines(ctx, tenantID, sourceID, revision, afterOrdinal, limit)
	if err != nil {
		return resp, err
	}
	resp.SourceRevision = revision
	resp.Items = items
	resp.NextAfterOrdinal = next
	return resp, nil
}
