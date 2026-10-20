package knowledge

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"strings"
	"time"

	"github.com/google/uuid"
	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
)

const maxOrphanPageCollections = 4
const maxOrphanPageSize = 50
const maxOrphanAuditPages = 4096
const orphanAuditCursorLifetime = 30 * time.Minute

type OrphanVectorAuditPageResult struct {
	OrphanVectorAuditResult
	AuditID           string                    `json:"auditId"`
	Page              int                       `json:"page"`
	Finished          bool                      `json:"finished"`
	Truncated         bool                      `json:"truncated"`
	NextCursor        string                    `json:"nextCursor,omitempty"`
	ObservationDigest string                    `json:"observationDigest,omitempty"`
	Totals            *OrphanAuditSessionTotals `json:"totals,omitempty"`
}

type orphanAuditCursor struct {
	Version         int    `json:"v"`
	TenantHash      string `json:"t"`
	CollectionsHash string `json:"h"`
	CollectionIndex int    `json:"i"`
	Offset          string `json:"o,omitempty"`
	AuditID         string `json:"a"`
	Page            int    `json:"p"`
	CreatedAt       int64  `json:"c"`
}

type orphanVectorPageScanner func(context.Context, string, string, int, string) ([]qdrantDB.VectorMetadata, string, error)

func observeOrphanScan(h hash.Hash, collection string, points []qdrantDB.VectorMetadata) error {
	header, err := json.Marshal(struct {
		Collection string                    `json:"collection"`
		Points     []qdrantDB.VectorMetadata `json:"points"`
	}{Collection: collection, Points: points})
	if err != nil {
		return err
	}
	_, err = h.Write(header)
	return err
}

func orphanAuditFingerprint(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func makeOrphanAuditCursor(state orphanAuditCursor) (string, error) {
	b, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func readOrphanAuditCursor(value, tenantID string, collections []string, now time.Time) (orphanAuditCursor, error) {
	var state orphanAuditCursor
	if len(value) == 0 || len(value) > 1024 {
		return state, fmt.Errorf("invalid orphan audit cursor")
	}
	b, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || json.Unmarshal(b, &state) != nil {
		return state, fmt.Errorf("invalid orphan audit cursor")
	}
	if state.Version != 1 || state.TenantHash != orphanAuditFingerprint(tenantID) ||
		state.CollectionsHash != orphanAuditFingerprint(strings.Join(collections, "\x00")) ||
		state.CollectionIndex < 0 || state.CollectionIndex >= len(collections) ||
		state.Page < 1 || state.Page >= maxOrphanAuditPages ||
		state.CreatedAt <= 0 || now.Unix() < state.CreatedAt || now.Sub(time.Unix(state.CreatedAt, 0)) > orphanAuditCursorLifetime {
		return orphanAuditCursor{}, fmt.Errorf("expired or mismatched orphan audit cursor")
	}
	if _, err = uuid.Parse(state.AuditID); err != nil {
		return orphanAuditCursor{}, fmt.Errorf("invalid orphan audit identifier")
	}
	if state.Offset != "" {
		if _, err = uuid.Parse(state.Offset); err != nil {
			return orphanAuditCursor{}, fmt.Errorf("invalid orphan audit offset")
		}
	}
	return state, nil
}

func auditTenantOrphanVectorPage(ctx context.Context, tenantID string, collections []string, limit int, cursor string, now time.Time, scan orphanVectorPageScanner, load orphanStateLoader) (OrphanVectorAuditPageResult, error) {
	result := OrphanVectorAuditPageResult{OrphanVectorAuditResult: OrphanVectorAuditResult{Status: "empty", SampleLimit: limit, CollectionsKnown: len(collections), ObservedAt: now.UTC()}}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if strings.TrimSpace(tenantID) == "" || limit < 1 || limit > maxOrphanPageSize || scan == nil || load == nil {
		return result, fmt.Errorf("invalid orphan vector audit page request")
	}
	if len(collections) == 0 {
		if cursor != "" {
			return result, fmt.Errorf("orphan audit collections changed")
		}
		result.Finished = true
		result.ObservationDigest = orphanAuditFingerprint("")
		return result, nil
	}
	var state orphanAuditCursor
	if cursor != "" {
		var err error
		state, err = readOrphanAuditCursor(cursor, tenantID, collections, now)
		if err != nil {
			return result, err
		}
	} else {
		seed := make([]byte, 16)
		if _, err := rand.Read(seed); err != nil {
			return result, fmt.Errorf("create orphan audit session: %w", err)
		}
		state = orphanAuditCursor{Version: 1, TenantHash: orphanAuditFingerprint(tenantID), CollectionsHash: orphanAuditFingerprint(strings.Join(collections, "\x00")), AuditID: uuid.UUID(seed).String(), CreatedAt: now.Unix(), Page: 0}
	}
	result.AuditID = state.AuditID
	result.Page = state.Page + 1
	nextIndex, nextOffset := state.CollectionIndex, state.Offset
	observation := sha256.New()
	for calls := 0; calls < maxOrphanPageCollections && nextIndex < len(collections) && result.Sampled < limit; calls++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		batch := limit - result.Sampled
		if batch > 25 {
			batch = 25
		}
		points, after, err := scan(ctx, tenantID, collections[nextIndex], batch, nextOffset)
		if err != nil {
			result.Status = "unavailable"
			return result, fmt.Errorf("orphan audit page scan unavailable: %w", err)
		}
		if len(points) > batch || after == nextOffset && after != "" {
			return result, fmt.Errorf("orphan audit scanner invalid continuation")
		}
		if after != "" {
			if _, err := uuid.Parse(after); err != nil || len(points) == 0 {
				return result, fmt.Errorf("orphan audit scanner invalid offset")
			}
		}
		if err := observeOrphanScan(observation, collections[nextIndex], points); err != nil {
			return result, err
		}
		canonical, err := load(ctx, tenantID, points)
		if err != nil {
			result.Status = "unavailable"
			return result, fmt.Errorf("orphan audit page canonical lookup unavailable: %w", err)
		}
		orphans, mismatches, inflight, err := classifyOrphanVectorPoints(points, tenantID, collections[nextIndex], canonical)
		if err != nil {
			return result, err
		}
		result.Sampled += len(points)
		result.CollectionsChecked++
		result.SuspectedOrphans += orphans
		result.MetadataMismatches += mismatches
		result.InFlight += inflight
		if after != "" {
			nextOffset = after
		} else {
			nextOffset = ""
			nextIndex++
		}
	}
	result.ObservationDigest = hex.EncodeToString(observation.Sum(nil))
	result.Finished = nextIndex >= len(collections)
	result.Truncated = !result.Finished && result.Page >= maxOrphanAuditPages
	if !result.Finished && result.Page < maxOrphanAuditPages {
		state.CollectionIndex = nextIndex
		state.Offset = nextOffset
		state.Page = result.Page
		next, err := makeOrphanAuditCursor(state)
		if err != nil {
			return result, err
		}
		result.NextCursor = next
	}
	if result.Sampled > 0 {
		result.Status = "sampled"
	}
	if result.SuspectedOrphans > 0 || result.MetadataMismatches > 0 {
		result.Status = "suspected"
	}
	return result, nil
}

func (s *service) AuditOrphanVectorPage(ctx context.Context, tenantID string, limit int, cursor string) (OrphanVectorAuditPageResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if limit <= 0 {
		limit = 25
	}
	if limit > maxOrphanPageSize {
		limit = maxOrphanPageSize
	}
	if strings.TrimSpace(tenantID) == "" {
		return OrphanVectorAuditPageResult{}, fmt.Errorf("tenant scope required")
	}
	collections, err := qdrantDB.ListKnowledgeCollectionNamesWithContext(ctx, configuredKnowledgeCollectionBase())
	if err != nil {
		return OrphanVectorAuditPageResult{}, err
	}
	return s.repo.auditOrphanPageWithStore(ctx, tenantID, collections, limit, cursor, time.Now().UTC(), qdrantDB.ScanTenantVectorMetadataPageWithContext, s.repo.orphanAuditState)
}
