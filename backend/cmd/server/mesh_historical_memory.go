package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/episodic"
	"github.com/u-ai/backend/internal/memory"
	"github.com/u-ai/backend/internal/profile"
	"gorm.io/gorm"
)

type meshLegacyMemoryPosition struct {
	Fence     string `json:"fence"`
	Role      string `json:"role"`
	Kind      string `json:"kind"`
	ID        string `json:"id"`
	UpdatedAt string `json:"updatedAt"`
}

func meshLegacyMemoryPage[T any](db *gorm.DB, scope coordination.ExecutionScope, role string, query coordination.DataQuery) ([]json.RawMessage, string, error) {
	scope.RequestID, scope.TurnID, scope.ExecutionID = "", "", ""
	encoded, err := json.Marshal(scope)
	if err != nil {
		return nil, "", err
	}
	digest := sha256.Sum256(encoded)
	position := meshLegacyMemoryPosition{Fence: hex.EncodeToString(digest[:]), Role: role, Kind: query.ResourceKind}
	if query.LegacyCursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(query.LegacyCursor)
		var previous meshLegacyMemoryPosition
		if len(query.LegacyCursor) > 4096 || err != nil || json.Unmarshal(raw, &previous) != nil || previous.Fence != position.Fence || previous.Role != role || previous.Kind != query.ResourceKind || previous.ID == "" || len(previous.ID) > 512 || len(previous.UpdatedAt) > 128 {
			return nil, "", coordination.ErrWrongOwner
		}
		position = previous
		db = db.Where("COALESCE(updated_at,'') < ? OR (COALESCE(updated_at,'') = ? AND id < ?)", position.UpdatedAt, position.UpdatedAt, position.ID)
	}
	limit := query.Limit
	if limit < 1 || limit > 128 {
		limit = 128
	}
	var items []T
	if err := db.Order("COALESCE(updated_at,'') DESC,id DESC").Limit(limit + 1).Find(&items).Error; err != nil {
		return nil, "", err
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	rows, err := appendMeshJSON(items)
	if err != nil {
		return nil, "", err
	}
	if !more {
		return rows, "", nil
	}
	var last struct {
		ID        string `json:"id"`
		UpdatedAt string `json:"updatedAt"`
	}
	if json.Unmarshal(rows[len(rows)-1], &last) != nil || last.ID == "" {
		return nil, "", coordination.ErrWrongOwner
	}
	position.ID, position.UpdatedAt = last.ID, last.UpdatedAt
	raw, err := json.Marshal(position)
	return rows, base64.RawURLEncoding.EncodeToString(raw), err
}

func (p *meshLocalDataPort) historicalMemoryManagement(ctx context.Context, scope coordination.ExecutionScope, role string, query coordination.DataQuery, snapshot *coordination.DataSnapshot) error {
	if query.Cursor != "" && query.LegacyCursor == "" {
		return nil
	}
	db := p.services.DB.WithContext(ctx).Where("character_id=? AND (space_id=? OR space_id='' OR space_id='default')", role, p.legacySpaceID)
	var cursor string
	var err error
	switch query.ResourceKind {
	case "memory":
		snapshot.LegacyMemories, cursor, err = meshLegacyMemoryPage[memory.Memory](db.Where("verified_status NOT IN ('tombstone','deleted','replaced','invalidated')"), scope, role, query)
	case "profile":
		snapshot.LegacyProfiles, cursor, err = meshLegacyMemoryPage[profile.UserProfile](db, scope, role, query)
	case "episodic":
		snapshot.LegacyEpisodes, cursor, err = meshLegacyMemoryPage[episodic.EpisodicMemory](db.Where("decay_state <> 'deleted'"), scope, role, query)
	}
	if err != nil {
		return err
	}
	if cursor != "" {
		snapshot.NextCursors["legacy"+map[string]string{"memory": "Memory", "profile": "Profile", "episodic": "Episode"}[query.ResourceKind]] = cursor
	}
	return nil
}
