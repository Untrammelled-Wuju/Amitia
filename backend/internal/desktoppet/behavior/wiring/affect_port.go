package wiring

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/behavior"
)

type AffectAdapter struct {
	db     *sql.DB
	owners []string
}

func NewOwnedAffectAdapter(db *sql.DB, owners []string) *AffectAdapter {
	return &AffectAdapter{db: db, owners: append([]string(nil), owners...)}
}

func (a *AffectAdapter) GetAffectSnapshot(ctx context.Context, spaceID, characterID string) (*behavior.AffectBehaviorSnapshot, error) {
	return a.getOwnedAffectSnapshot(ctx, characterID)
}

func (a *AffectAdapter) getOwnedAffectSnapshot(ctx context.Context, characterID string) (*behavior.AffectBehaviorSnapshot, error) {
	if a.db == nil || len(a.owners) == 0 {
		return defaultAffectSnapshot(), nil
	}
	var body []byte
	var updatedAt string
	var err error
	for _, owner := range a.owners {
		err = a.db.QueryRowContext(ctx, `SELECT CAST(body AS BLOB),updated_at FROM kernel_device_owned_resources WHERE owner_id=? AND kind='psyche' AND role_id=? AND deleted=0 AND resource_id IN (?,?) ORDER BY CASE WHEN resource_id=? THEN 0 ELSE 1 END LIMIT 1`, owner, characterID, "role/"+characterID, "role", "role/"+characterID).Scan(&body, &updatedAt)
		if err == nil {
			break
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("affect adapter: load owned state failed: %w", err)
		}
	}
	if errors.Is(err, sql.ErrNoRows) {
		return defaultAffectSnapshot(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("affect adapter: load owned state failed: %w", err)
	}
	var state struct {
		Valence   float64 `json:"valence"`
		Arousal   float64 `json:"arousal"`
		Dominance float64 `json:"dominance"`
		Stress    float64 `json:"stress"`
	}
	if err := json.Unmarshal(body, &state); err != nil {
		return nil, fmt.Errorf("affect adapter: decode owned state failed: %w", err)
	}
	return &behavior.AffectBehaviorSnapshot{
		Version:    "owned-psyche:" + updatedAt,
		Valence:    state.Valence,
		Arousal:    state.Arousal,
		Tension:    1 - state.Dominance,
		Stress:     state.Stress,
		Label:      mapEmotionLabel(state.Valence, state.Arousal),
		Confidence: 0.9,
		UpdatedAt:  parseAffectTime(updatedAt),
	}, nil
}

func defaultAffectSnapshot() *behavior.AffectBehaviorSnapshot {
	return &behavior.AffectBehaviorSnapshot{Version: "v0", Label: "neutral", Confidence: 0.5, UpdatedAt: time.Now()}
}

func parseAffectTime(value string) time.Time {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed
		}
	}
	return time.Now()
}

func mapEmotionLabel(valence, arousal float64) string {
	if valence > 0.3 && arousal > 0.3 {
		return "happy"
	}
	if valence > 0.3 && arousal <= 0.3 {
		return "calm"
	}
	if valence <= -0.3 && arousal > 0.3 {
		return "angry"
	}
	if valence <= -0.3 && arousal <= 0.3 {
		return "sad"
	}
	if arousal > 0.5 {
		return "excited"
	}
	return "neutral"
}

var _ behavior.CharacterAffectPort = (*AffectAdapter)(nil)
