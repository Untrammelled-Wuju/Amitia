package memory

import (
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
	DeleteAll() error
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
	if q.MemoryType != "" {
		query = query.Where("memory_type = ?", q.MemoryType)
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
	sortBy := q.SortBy
	if sortBy == "" || !map[string]bool{"updated_at": true, "created_at": true, "importance": true, "use_count": true}[sortBy] {
		sortBy = "updated_at"
	}
	var items []Memory
	err := query.Order(sortBy + " DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&items).Error
	if items == nil {
		items = []Memory{}
	}
	return items, total, err
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
	r.db.Where("memory_id = ?", id).Delete(&struct{}{})
	return r.db.Where("id = ?", id).Delete(&Memory{}).Error
}

func (r *repository) DeleteAll() error {
	r.db.Where("1=1").Delete(&struct{}{})
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
		"last_used_at": gorm.Expr("datetime('now')"),
	}).Error
}

func (r *repository) VectorStatus() (totalMem, embedded int64) {
	r.db.Model(&Memory{}).Count(&totalMem)
	r.db.Table("memory_embeddings").Select("COUNT(DISTINCT memory_id)").Scan(&embedded)
	return
}

func (r *repository) MarkEmbedded(id string) error {
	return r.db.Exec(
		"INSERT OR REPLACE INTO memory_embeddings (memory_id, created_at) VALUES (?, datetime('now'))",
		id,
	).Error
}