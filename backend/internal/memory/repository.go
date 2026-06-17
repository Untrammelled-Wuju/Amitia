package memory

import (
	"strings"

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
	Search(keyword, characterID string, limit int) ([]Memory, error)
	RecordUse(id string) error
	VectorStatus() (totalMem, embedded int64)
	MarkEmbedded(id string) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(ctx *app.AppContext) Repository {
	return &repository{db: ctx.DB}
}

func (r *repository) List(q MemoryListQuery) ([]Memory, int64, error) {
	query := r.db.Model(&Memory{})
	if q.CharacterID != "" {
		query = query.Where("character_id = ?", q.CharacterID)
	}
	if q.Source != "" {
		query = query.Where("source = ?", q.Source)
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
		"updated_at": "updated_at",
		"created_at": "created_at",
		"importance": "importance",
		"use_count":  "use_count",
		"time":       "created_at",
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
	r.db.Exec("DELETE FROM memory_events WHERE memory_id = ?", id)
	return r.db.Where("id = ?", id).Delete(&Memory{}).Error
}

func (r *repository) DeleteAll(characterID string) error {
	if characterID != "" {
		r.db.Exec("DELETE FROM memory_events WHERE character_id = ?", characterID)
		return r.db.Where("character_id = ?", characterID).Delete(&Memory{}).Error
	}
	r.db.Exec("DELETE FROM memory_events")
	return r.db.Where("1=1").Delete(&Memory{}).Error
}

func (r *repository) Search(keyword, characterID string, limit int) ([]Memory, error) {
	query := r.db.Where("(key LIKE ? OR value LIKE ?)", "%"+keyword+"%", "%"+keyword+"%")
	if characterID != "" {
		query = query.Where("character_id = ?", characterID)
	}
	var items []Memory
	err := query.Order("importance DESC, use_count DESC").Limit(limit).Find(&items).Error
	if items == nil {
		items = []Memory{}
	}
	return items, err
}

func (r *repository) RecordUse(id string) error {
	return r.db.Model(&Memory{}).Where("id = ?", id).Updates(map[string]interface{}{
		"use_count":    gorm.Expr("use_count + 1"),
		"last_used_at": gorm.Expr("datetime('now', 'localtime')"),
	}).Error
}

func (r *repository) VectorStatus() (totalMem, embedded int64) {
	r.db.Model(&Memory{}).Count(&totalMem)
	r.db.Table("memory_embeddings").Select("COUNT(DISTINCT memory_id)").Scan(&embedded)
	return
}

func (r *repository) MarkEmbedded(id string) error {
	return r.db.Exec(
		"INSERT OR REPLACE INTO memory_embeddings (memory_id, created_at) VALUES (?, datetime('now', 'localtime'))",
		id,
	).Error
}

