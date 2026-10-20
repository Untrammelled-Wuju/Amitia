package knowledge

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"
)

const (
	defaultReadBeforeUnits = 1
	defaultReadAfterUnits  = 1
	defaultReadMaxUnits    = 6
	defaultReadMaxRunes    = 16000
	maxReadUnits           = 32
	maxReadRunes           = 64000
)

// Read returns canonical parsed units only. It deliberately never re-opens the
// original uploaded file and never accepts a filesystem path. This keeps the
// same tenant/revision lifecycle contract as retrieval and makes reads
// deterministic across Web, Flutter and agent tool callers.
func (s *service) Read(ctx context.Context, tenantID string, req ReadRequest) (ReadResponse, error) {
	var resp ReadResponse
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return resp, err
	}
	if s != nil && s.settings.ConfigHash != "" && !s.settings.CanonicalReaderEnabled {
		return resp, fmt.Errorf("canonical reader is disabled")
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return resp, fmt.Errorf("tenant scope required")
	}

	req.EvidenceID = strings.TrimSpace(req.EvidenceID)
	req.UnitID = strings.TrimSpace(req.UnitID)
	req.SourceID = strings.TrimSpace(req.SourceID)
	if req.BeforeUnits < 0 || req.AfterUnits < 0 {
		return resp, fmt.Errorf("beforeUnits and afterUnits must be non-negative")
	}
	if req.BeforeUnits == 0 && req.AfterUnits == 0 {
		req.BeforeUnits = defaultReadBeforeUnits
		req.AfterUnits = defaultReadAfterUnits
	}
	if req.BeforeUnits > 16 || req.AfterUnits > 16 {
		return resp, fmt.Errorf("beforeUnits and afterUnits must not exceed 16")
	}
	if req.MaxUnits <= 0 {
		req.MaxUnits = defaultReadMaxUnits
	}
	if req.MaxUnits > maxReadUnits {
		req.MaxUnits = maxReadUnits
	}
	if req.MaxRunes <= 0 {
		req.MaxRunes = defaultReadMaxRunes
	}
	if req.MaxRunes > maxReadRunes {
		req.MaxRunes = maxReadRunes
	}
	if s != nil && s.readSem != nil {
		select {
		case s.readSem <- struct{}{}:
			defer func() { <-s.readSem }()
		case <-ctx.Done():
			return resp, ctx.Err()
		}
	}

	var anchor *DocumentUnit
	anchorText := ""
	if req.UnitID != "" {
		unit, err := s.repo.GetUnit(tenantID, req.UnitID)
		if err != nil {
			return resp, err
		}
		anchor = unit
	} else if req.EvidenceID != "" {
		chunk, err := s.repo.GetChunk(tenantID, req.EvidenceID)
		if err != nil {
			return resp, err
		}
		unit, err := s.repo.GetUnit(tenantID, chunk.UnitID)
		if err != nil {
			return resp, err
		}
		anchor = unit
		anchorText = strings.TrimSpace(chunk.Content)
	} else if req.SourceID != "" && len(req.Locator) > 0 {
		if err := validateCanonicalReadLocator(req.Locator); err != nil {
			return resp, err
		}
		revision := req.SourceRevision
		if revision <= 0 {
			source, err := s.repo.GetSource(tenantID, req.SourceID)
			if err != nil {
				return resp, err
			}
			revision = source.Revision
		}
		matches, err := s.repo.FindUnitsByLocator(ctx, tenantID, req.SourceID, revision, req.Locator, req.MaxUnits)
		if err != nil {
			return resp, err
		}
		if len(matches) == 0 {
			return resp, gorm.ErrRecordNotFound
		}
		anchor = &matches[0]
		req.SourceRevision = revision
	} else {
		return resp, fmt.Errorf("read requires evidenceId, unitId, or sourceId + locator")
	}

	if anchor == nil {
		return resp, gorm.ErrRecordNotFound
	}
	if req.SourceID != "" && req.SourceID != anchor.SourceID {
		return resp, fmt.Errorf("read source does not match resolved canonical unit")
	}
	if req.SourceRevision > 0 && req.SourceRevision != anchor.SourceRevision {
		return resp, fmt.Errorf("read revision does not match resolved canonical unit")
	}
	req.SourceID = anchor.SourceID
	req.SourceRevision = anchor.SourceRevision

	if _, err := s.repo.GetSource(tenantID, anchor.SourceID); err != nil {
		return resp, err
	}
	revision, err := s.repo.GetSourceRevision(tenantID, anchor.SourceID, anchor.SourceRevision)
	if err != nil {
		return resp, err
	}
	if revision.PurgedAt != nil || revision.Status == RevisionStatusPurged {
		return resp, fmt.Errorf("knowledge revision has been purged")
	}
	if revision.Status != RevisionStatusReady && revision.Status != RevisionStatusReadyDegraded {
		return resp, fmt.Errorf("knowledge revision is not readable: %s", revision.Status)
	}

	units, err := s.repo.UnitsWindow(tenantID, anchor.SourceID, anchor.SourceRevision, anchor.Ordinal, req.BeforeUnits, req.AfterUnits, req.MaxUnits)
	if err != nil {
		return resp, err
	}
	if len(units) == 0 {
		return resp, gorm.ErrRecordNotFound
	}

	resp.SourceID = anchor.SourceID
	resp.SourceRevision = anchor.SourceRevision
	resp.AnchorUnitID = anchor.ID
	resp.Units = make([]ReadUnit, 0, len(units))
	remaining := req.MaxRunes
	for _, unit := range units {
		if err := ctx.Err(); err != nil {
			return ReadResponse{}, err
		}
		if remaining <= 0 {
			resp.Truncated = true
			break
		}
		content := unit.Content
		truncated := false
		unitBudget := remaining
		// Do not let a single very large unit starve all neighboring context.
		if len(units) > 1 {
			fairShare := req.MaxRunes / len(units)
			if fairShare < 1000 {
				fairShare = 1000
			}
			if unitBudget > fairShare*2 {
				unitBudget = fairShare * 2
			}
		}
		if utf8.RuneCountInString(content) > unitBudget {
			content = truncateAroundAnchor(content, anchorText, unitBudget)
			truncated = true
			resp.Truncated = true
		}
		used := utf8.RuneCountInString(content)
		if used > remaining {
			content = truncateAroundAnchor(content, anchorText, remaining)
			used = utf8.RuneCountInString(content)
			truncated = true
			resp.Truncated = true
		}
		remaining -= used
		resp.TotalRunes += used
		resp.Units = append(resp.Units, ReadUnit{
			ID: unit.ID, SourceID: unit.SourceID, SourceRevision: unit.SourceRevision, Ordinal: unit.Ordinal,
			UnitType: unit.UnitType, Title: unit.Title, Content: content, Locator: unit.Locator,
			Metadata: unit.Metadata, Checksum: unit.Checksum, Truncated: truncated,
		})
	}
	if len(resp.Units) == 0 {
		return ReadResponse{}, errors.New("knowledge read budget exhausted")
	}
	return resp, nil
}

func truncateAroundAnchor(content, anchor string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(content)
	if len(runes) <= maxRunes {
		return content
	}
	if maxRunes <= 2 {
		return string(runes[:maxRunes-1]) + "…"
	}
	anchor = strings.TrimSpace(anchor)
	center := len(runes) / 2
	if anchor != "" {
		needle := []rune(anchor)
		if len(needle) > 96 {
			needle = needle[:96]
		}
		if idx := strings.Index(content, string(needle)); idx >= 0 {
			center = utf8.RuneCountInString(content[:idx]) + len(needle)/2
		}
	}
	width := maxRunes - 2
	start := center - width/2
	if start < 0 {
		start = 0
	}
	if start+width > len(runes) {
		start = len(runes) - width
	}
	end := start + width
	if start == 0 {
		end++
	}
	if end == len(runes) {
		start--
	}
	prefix, suffix := "", ""
	if start > 0 {
		prefix = "…"
	}
	if end < len(runes) {
		suffix = "…"
	}
	return prefix + string(runes[start:end]) + suffix
}
