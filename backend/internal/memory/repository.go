// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type Repository interface {
	List(q MemoryListQuery) ([]Memory, int64, error)
	FindByID(id string) (*Memory, error)
	Create(m *Memory) error
	Update(id string, updates map[string]interface{}) error
	Delete(id string) error
	DeleteAll(characterID string) error
	Search(keyword, characterID, userID string, limit int) ([]Memory, error)
	SearchByKey(key, characterID string) ([]Memory, error)
	RecordUse(id string) error
	VectorStatus() (totalMem, embedded int64)
	MarkEmbedded(id string) error
	UnmarkEmbedded(id string) error
	GetConversationMessages(conversationID string, limit int) ([]map[string]interface{}, error)
	GetRankedByImportance(characterID string, limit int) ([]Memory, error)
	ListCandidates() ([]MemoryCandidateModel, error)
	CreateCandidate(c *MemoryCandidateModel) error
	UpdateCandidate(id string, updates map[string]interface{}) error
	DeleteCandidate(id string) error
	GetCandidateByID(id string) (*MemoryCandidateModel, error)
	DeleteAllCandidates() error
	FindByDerivationKey(derivationKey string) (*Memory, error)
	ListDerivationsByOutput(outputMemoryID string) ([]MemoryDerivation, error)
	ListDerivationsByInput(inputMemoryID string) ([]MemoryDerivation, error)
	CreateDerivation(d *MemoryDerivation) error
	FindDerivedMemoryBySourceIDs(sourceIDs []string, kind string) (*Memory, error)
	StreamExportable(characterID string, limit, offset int) ([]Memory, error)
	StreamExportableByIDs(ids []string, limit, offset int) ([]Memory, error)
	CountExportable(characterID string) (int64, error)
	CountExportableByIDs(ids []string) (int64, error)
	ListEventsByMemoryIDs(memoryIDs []string) ([]MemoryEventV1, error)
	ListTemporalByMemoryIDs(memoryIDs []string) ([]MemoryTemporalV1, error)
	ListDerivationsByMemoryIDs(memoryIDs []string) ([]MemoryDerivationV1, error)
	IsNewID(id string) (bool, error)
	AppendRestoredEvents(events []MemoryEventV1) error
}

var ErrRestoreEventConflict = errors.New("restore event content conflict")

type repository struct {
	db *gorm.DB
}

func NewRepository(ctx *app.AppContext) Repository {
	return &repository{db: ctx.DB}
}

func (r *repository) List(q MemoryListQuery) ([]Memory, int64, error) {
	query := r.db.Model(&Memory{})
	query = applyMemoryScopeQuery(query, q.CharacterID, q.UserID)
	if q.Source != "" {
		query = query.Where("source = ?", q.Source)
	}
	if q.ScopeType != "" {
		switch q.ScopeType {
		case "user_global":
			query = query.Where("scope = ?", "user")
		case "user_character":
			query = query.Where("scope = ?", "character")
		case "world":
			query = query.Where("scope = ?", "world")
		case "character_self":
			query = query.Where("scope = ?", "character")
		}
	}
	memoryTypeFilter := q.MemoryType
	if memoryTypeFilter == "" {
		memoryTypeFilter = q.Type
	}
	if memoryTypeFilter != "" {
		query = query.Where("memory_type = ?", memoryTypeFilter)
	}
	if q.Keyword != "" {
		query = query.Where("(key LIKE ? OR value LIKE ?)", "%"+q.Keyword+"%", "%"+q.Keyword+"%")
	}
	if q.VerifiedStatus != "" {
		query = query.Where("verified_status = ?", q.VerifiedStatus)
	}
	if q.MinConfidence > 0 {
		query = query.Where("confidence >= ?", q.MinConfidence)
	}
	if q.RetentionLevel >= RetentionL1 && q.RetentionLevel <= RetentionL5 {
		query = query.Where("retention_level = ?", q.RetentionLevel)
	}
	if q.DecayState != "" {
		query = query.Where("decay_state = ?", q.DecayState)
	}
	if q.Pinned != nil {
		query = query.Where("pinned = ?", *q.Pinned)
	}
	var total int64
	query.Count(&total)
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 50
	}
	sortRaw := q.SortBy
	if sortRaw == "" {
		sortRaw = q.Sort
	}
	sortBy, sortDir := parseSort(sortRaw)
	var items []Memory
	err := query.Order(sortBy + " " + sortDir).Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&items).Error
	if items == nil {
		items = []Memory{}
	}
	return items, total, err
}

func parseSort(raw string) (col, dir string) {
	col = "updated_at"
	dir = "DESC"
	if raw == "" {
		return
	}
	lower := strings.ToLower(raw)
	if strings.HasSuffix(lower, "_desc") {
		dir = "DESC"
		raw = raw[:len(raw)-5]
	} else if strings.HasSuffix(lower, "_asc") {
		dir = "ASC"
		raw = raw[:len(raw)-4]
	}
	validCols := map[string]string{
		"updated_at":      "updated_at",
		"created_at":      "created_at",
		"importance":      "importance",
		"confidence":      "confidence",
		"use_count":       "use_count",
		"retention_level": "retention_level",
		"memory_strength": "memory_strength",
		"reinforce_count": "reinforce_count",
		"retrieved_count": "retrieved_count",
		"injected_count":  "injected_count",
		"time":            "created_at",
	}
	mapped, ok := validCols[strings.ToLower(raw)]
	if ok {
		col = mapped
	}
	return
}

func (r *repository) FindByID(id string) (*Memory, error) {
	var m Memory
	err := r.db.Where("id = ?", id).First(&m).Error
	return &m, err
}

func (r *repository) Create(m *Memory) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	return r.db.Create(m).Error
}

func (r *repository) Update(id string, updates map[string]interface{}) error {
	return r.db.Model(&Memory{}).Where("id = ?", id).Updates(updates).Error
}

func (r *repository) Delete(id string) error {
	return r.db.Where("id = ?", id).Delete(&Memory{}).Error
}

func (r *repository) DeleteAll(characterID string) error {
	if characterID != "" {
		return r.db.Where("character_id = ?", characterID).Delete(&Memory{}).Error
	}
	return r.db.Where("1=1").Delete(&Memory{}).Error
}

func (r *repository) Search(keyword, characterID, userID string, limit int) ([]Memory, error) {
	query := r.db.Where("(key LIKE ? OR value LIKE ?)", "%"+keyword+"%", "%"+keyword+"%")
	query = applyMemoryScopeQuery(query, characterID, userID)
	var items []Memory
	err := query.Order("importance DESC, confidence DESC, updated_at DESC").Limit(limit).Find(&items).Error
	if items == nil {
		items = []Memory{}
	}
	return items, err
}

func (r *repository) SearchByKey(key, characterID string) ([]Memory, error) {
	query := r.db.Where("key = ?", key)
	if characterID != "" {
		query = applyMemoryScopeQuery(query, characterID, "")
	}
	var items []Memory
	err := query.Order("confidence DESC").Find(&items).Error
	if items == nil {
		items = []Memory{}
	}
	return items, err
}

func (r *repository) RecordUse(id string) error {
	nowStr := time.Now().Format("2006-01-02 15:04:05")
	return r.db.Model(&Memory{}).Where("id = ?", id).Updates(map[string]interface{}{
		"use_count":      gorm.Expr("use_count + 1"),
		"injected_count": gorm.Expr("injected_count + 1"),
		"last_used_at":   nowStr,
	}).Error
}

func (r *repository) VectorStatus() (totalMem, embedded int64) {
	r.db.Model(&Memory{}).Count(&totalMem)
	r.db.Table("memory_embeddings").Select("COUNT(DISTINCT memory_id)").Scan(&embedded)
	return
}

func (r *repository) MarkEmbedded(id string) error {
	nowStr := time.Now().Format("2006-01-02 15:04:05")
	return r.db.Exec(
		"INSERT OR REPLACE INTO memory_embeddings (memory_id, created_at) VALUES (?, ?)",
		id, nowStr,
	).Error
}

func (r *repository) UnmarkEmbedded(id string) error {
	return r.db.Exec("DELETE FROM memory_embeddings WHERE memory_id = ?", id).Error
}

func (r *repository) GetConversationMessages(conversationID string, limit int) ([]map[string]interface{}, error) {
	var messages []map[string]interface{}
	query := r.db.Table("messages").
		Select("role, content").
		Where("conversation_id = ?", conversationID).
		Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&messages).Error
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	if messages == nil {
		messages = []map[string]interface{}{}
	}
	return messages, err
}

func (r *repository) GetRankedByImportance(characterID string, limit int) ([]Memory, error) {
	query := applyMemoryScopeQuery(r.db.Model(&Memory{}), characterID, "").
		Order("importance DESC, confidence DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	var items []Memory
	err := query.Find(&items).Error
	if items == nil {
		items = []Memory{}
	}
	return items, err
}

func applyMemoryScopeQuery(query *gorm.DB, characterID, userID string) *gorm.DB {
	characterID = strings.TrimSpace(characterID)
	userID = strings.TrimSpace(userID)
	if characterID != "" && userID != "" {
		return query.Where("((scope IN (?, ?) AND character_id = ?) OR ((scope IS NULL OR scope = '' OR scope NOT IN (?, ?)) AND character_id = ?))", "user", "user_global", userID, "user", "user_global", characterID)
	}
	if userID != "" {
		return query.Where("scope IN (?, ?) AND character_id = ?", "user", "user_global", userID)
	}
	if characterID != "" {
		return query.Where("character_id = ?", characterID)
	}
	return query
}

func (r *repository) ListCandidates() ([]MemoryCandidateModel, error) {
	var items []MemoryCandidateModel
	err := r.db.Order("created_at DESC").Find(&items).Error
	if items == nil {
		items = []MemoryCandidateModel{}
	}
	return items, err
}

func (r *repository) CreateCandidate(c *MemoryCandidateModel) error {
	return r.db.Create(c).Error
}

func (r *repository) UpdateCandidate(id string, updates map[string]interface{}) error {
	return r.db.Model(&MemoryCandidateModel{}).Where("id = ?", id).Updates(updates).Error
}

func (r *repository) DeleteCandidate(id string) error {
	return r.db.Where("id = ?", id).Delete(&MemoryCandidateModel{}).Error
}

func (r *repository) GetCandidateByID(id string) (*MemoryCandidateModel, error) {
	var c MemoryCandidateModel
	err := r.db.Where("id = ?", id).First(&c).Error
	return &c, err
}

func (r *repository) DeleteAllCandidates() error {
	return r.db.Where("1=1").Delete(&MemoryCandidateModel{}).Error
}

func (r *repository) FindByDerivationKey(derivationKey string) (*Memory, error) {
	var m Memory
	err := r.db.Where("derivation_key = ? AND derivation_key != ''", derivationKey).First(&m).Error
	return &m, err
}

func (r *repository) ListDerivationsByOutput(outputMemoryID string) ([]MemoryDerivation, error) {
	var items []MemoryDerivation
	err := r.db.Where("output_memory_id = ?", outputMemoryID).Order("ordinal ASC").Find(&items).Error
	if items == nil {
		items = []MemoryDerivation{}
	}
	return items, err
}

func (r *repository) ListDerivationsByInput(inputMemoryID string) ([]MemoryDerivation, error) {
	var items []MemoryDerivation
	err := r.db.Where("input_memory_id = ?", inputMemoryID).Order("created_at DESC").Find(&items).Error
	if items == nil {
		items = []MemoryDerivation{}
	}
	return items, err
}

func (r *repository) CreateDerivation(d *MemoryDerivation) error {
	return r.db.Create(d).Error
}

func (r *repository) FindDerivedMemoryBySourceIDs(sourceIDs []string, kind string) (*Memory, error) {
	if len(sourceIDs) == 0 {
		return nil, fmt.Errorf("no source ids")
	}
	args := make([]interface{}, 0, len(sourceIDs)+1)
	for _, id := range sourceIDs {
		args = append(args, id)
	}
	args = append(args, kind)
	placeholder := strings.TrimSuffix(strings.Repeat("?,", len(sourceIDs)), ",")
	var derivations []MemoryDerivation
	err := r.db.Where("input_memory_id IN ("+placeholder+") AND derivation_kind = ?", args...).Find(&derivations).Error
	if err != nil {
		return nil, err
	}
	seen := make(map[string]int)
	for _, d := range derivations {
		seen[d.OutputMemoryID]++
	}
	for outputID, count := range seen {
		if count == len(sourceIDs) {
			return r.FindByID(outputID)
		}
	}
	return nil, nil
}

func (r *repository) StreamExportable(characterID string, limit, offset int) ([]Memory, error) {
	query := r.db.Model(&Memory{}).Order("id ASC")
	if characterID != "" {
		query = query.Where("character_id = ?", characterID)
	}
	var items []Memory
	err := query.Offset(offset).Limit(limit).Find(&items).Error
	if items == nil {
		items = []Memory{}
	}
	return items, err
}

func (r *repository) StreamExportableByIDs(ids []string, limit, offset int) ([]Memory, error) {
	if len(ids) == 0 {
		return []Memory{}, nil
	}
	batch := ids
	if offset+limit < len(ids) {
		batch = ids[offset : offset+limit]
	} else if offset < len(ids) {
		batch = ids[offset:]
	} else {
		return []Memory{}, nil
	}
	var items []Memory
	err := r.db.Where("id IN ?", batch).Order("id ASC").Find(&items).Error
	if items == nil {
		items = []Memory{}
	}
	return items, err
}

func (r *repository) CountExportable(characterID string) (int64, error) {
	var count int64
	query := r.db.Model(&Memory{})
	if characterID != "" {
		query = query.Where("character_id = ?", characterID)
	}
	err := query.Count(&count).Error
	return count, err
}

func (r *repository) CountExportableByIDs(ids []string) (int64, error) {
	return int64(len(ids)), nil
}

func (r *repository) ListEventsByMemoryIDs(memoryIDs []string) ([]MemoryEventV1, error) {
	if len(memoryIDs) == 0 {
		return []MemoryEventV1{}, nil
	}
	var rows []struct {
		ID           string `gorm:"column:id"`
		MemoryID     string `gorm:"column:memory_id"`
		Version      int    `gorm:"column:version"`
		EventType    string `gorm:"column:event_type"`
		OperationID  string `gorm:"column:operation_id"`
		SnapshotHash string `gorm:"column:snapshot_hash"`
		EventReason  string `gorm:"column:event_reason"`
		CreatedAt    string `gorm:"column:created_at"`
	}
	err := r.db.Table("memory_events").Where("memory_id IN ?", memoryIDs).Order("memory_id ASC, version ASC").Find(&rows).Error
	result := make([]MemoryEventV1, 0, len(rows))
	for _, row := range rows {
		result = append(result, MemoryEventV1{
			ID:           row.ID,
			MemoryID:     row.MemoryID,
			Version:      row.Version,
			EventType:    row.EventType,
			OperationID:  row.OperationID,
			SnapshotHash: row.SnapshotHash,
			EventReason:  row.EventReason,
			CreatedAt:    row.CreatedAt,
		})
	}
	if result == nil {
		return []MemoryEventV1{}, nil
	}
	return result, err
}

func (r *repository) ListTemporalByMemoryIDs(memoryIDs []string) ([]MemoryTemporalV1, error) {
	if len(memoryIDs) == 0 {
		return []MemoryTemporalV1{}, nil
	}
	type temporalRow struct {
		MemoryID          string  `gorm:"column:memory_id"`
		OccurredAtUTC     *string `gorm:"column:occurred_at_utc"`
		EndedAtUTC        *string `gorm:"column:ended_at_utc"`
		Timezone          string  `gorm:"column:timezone"`
		LocalDate         string  `gorm:"column:local_date"`
		Daypart           string  `gorm:"column:daypart"`
		TemporalPrecision string  `gorm:"column:temporal_precision"`
		ValidFromUTC      *string `gorm:"column:valid_from_utc"`
		ValidToUTC        *string `gorm:"column:valid_to_utc"`
		AnchorIDsJSON     string  `gorm:"column:anchor_ids_json"`
		SourceTimeText    string  `gorm:"column:source_time_text"`
		CreatedAtUTC      string  `gorm:"column:created_at_utc"`
		UpdatedAtUTC      string  `gorm:"column:updated_at_utc"`
	}
	var rows []temporalRow
	err := r.db.Table("memory_temporal_metadata").Where("memory_id IN ?", memoryIDs).Find(&rows).Error
	result := make([]MemoryTemporalV1, 0, len(rows))
	for _, row := range rows {
		var anchorIDs []string
		if row.AnchorIDsJSON != "" {
			_ = json.Unmarshal([]byte(row.AnchorIDsJSON), &anchorIDs)
		}
		result = append(result, MemoryTemporalV1{
			MemoryID:          row.MemoryID,
			OccurredAtUTC:     row.OccurredAtUTC,
			EndedAtUTC:        row.EndedAtUTC,
			Timezone:          row.Timezone,
			LocalDate:         row.LocalDate,
			Daypart:           row.Daypart,
			TemporalPrecision: row.TemporalPrecision,
			ValidFromUTC:      row.ValidFromUTC,
			ValidToUTC:        row.ValidToUTC,
			AnchorIDs:         anchorIDs,
			SourceTimeText:    row.SourceTimeText,
			CreatedAtUTC:      row.CreatedAtUTC,
			UpdatedAtUTC:      row.UpdatedAtUTC,
		})
	}
	if result == nil {
		return []MemoryTemporalV1{}, nil
	}
	return result, err
}

func (r *repository) ListDerivationsByMemoryIDs(memoryIDs []string) ([]MemoryDerivationV1, error) {
	if len(memoryIDs) == 0 {
		return []MemoryDerivationV1{}, nil
	}
	var rows []MemoryDerivation
	err := r.db.Where("output_memory_id IN ? OR input_memory_id IN ?", memoryIDs, memoryIDs).Find(&rows).Error
	result := make([]MemoryDerivationV1, 0, len(rows))
	for _, row := range rows {
		result = append(result, MemoryDerivationV1{
			ID:                row.ID,
			OutputMemoryID:    row.OutputMemoryID,
			InputMemoryID:     row.InputMemoryID,
			InputVersion:      row.InputVersion,
			InputSnapshotHash: row.InputSnapshotHash,
			DerivationKind:    row.DerivationKind,
			Ordinal:           row.Ordinal,
			OperationID:       row.OperationID,
			CreatedAt:         row.CreatedAt,
		})
	}
	if result == nil {
		return []MemoryDerivationV1{}, nil
	}
	return result, err
}

func (r *repository) IsNewID(id string) (bool, error) {
	var count int64
	err := r.db.Model(&Memory{}).Where("id = ?", id).Count(&count).Error
	return count == 0, err
}

func (r *repository) AppendRestoredEvents(events []MemoryEventV1) error {
	if len(events) == 0 {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, e := range events {
			var existing MemoryEventV1
			err := tx.Table("memory_events").Where("id = ?", e.ID).Take(&existing).Error
			if err == nil {
				if !eventsEqual(existing, e) {
					return fmt.Errorf("%w: event ID %s content mismatch", ErrRestoreEventConflict, e.ID)
				}
				continue
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err := tx.Table("memory_events").Create(&e).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func eventsEqual(a, b MemoryEventV1) bool {
	return a.MemoryID == b.MemoryID &&
		a.Version == b.Version &&
		a.EventType == b.EventType &&
		a.OperationID == b.OperationID &&
		a.SnapshotHash == b.SnapshotHash &&
		a.EventReason == b.EventReason
}
