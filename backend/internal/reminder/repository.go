package reminder

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) List(userID string) ([]Reminder, error) {
	var items []Reminder
	err := ownerQuery(r.db, userID).Order("remind_at ASC").Find(&items).Error
	if items == nil {
		items = []Reminder{}
	}
	return items, err
}

func (r *Repository) Find(id int, userID string) (*Reminder, error) {
	var item Reminder
	err := ownerQuery(r.db, userID).Where("id = ?", id).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) Create(item *Reminder) error {
	return r.db.Create(item).Error
}

func (r *Repository) Update(id int, userID string, updates map[string]interface{}) error {
	return ownerQuery(r.db.Model(&Reminder{}), userID).Where("id = ?", id).Updates(updates).Error
}

func (r *Repository) Delete(id int, userID string) error {
	return ownerQuery(r.db, userID).Where("id = ?", id).Delete(&Reminder{}).Error
}

func (r *Repository) Toggle(id int, userID string) (*Reminder, error) {
	err := ownerQuery(r.db.Model(&Reminder{}), userID).Where("id = ?", id).Update("enabled", gorm.Expr("CASE WHEN enabled = 1 THEN 0 ELSE 1 END")).Error
	if err != nil {
		return nil, err
	}
	return r.Find(id, userID)
}

func (r *Repository) Due(limit int) ([]Reminder, error) {
	var items []Reminder
	err := r.db.Where("enabled = 1 AND remind_at <= ?", nowString()).Order("remind_at ASC").Limit(limit).Find(&items).Error
	if items == nil {
		items = []Reminder{}
	}
	return items, err
}

func (r *Repository) CreateHistory(item *TriggerHistory) error {
	return r.db.Create(item).Error
}

func (r *Repository) UpdateHistory(id string, updates map[string]interface{}) error {
	return r.db.Model(&TriggerHistory{}).Where("id = ?", id).Updates(updates).Error
}

func (r *Repository) ListHistory(userID string, page, pageSize int, state string) ([]TriggerHistory, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}
	query := ownerQuery(r.db.Model(&TriggerHistory{}), userID)
	if state != "" {
		query = query.Where("state = ?", state)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []TriggerHistory
	err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error
	if items == nil {
		items = []TriggerHistory{}
	}
	return items, total, err
}

func (r *Repository) DeleteHistoryBefore(userID string, before time.Time) error {
	return ownerQuery(r.db, userID).Where("created_at < ?", before.Format("2006-01-02 15:04:05")).Delete(&TriggerHistory{}).Error
}

func nowString() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func ownerQuery(db *gorm.DB, userID string) *gorm.DB {
	values := []string{strings.TrimSpace(userID), "default", "local_user", "1"}
	unique := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		unique = append(unique, value)
	}
	return db.Where("user_id IN ?", unique)
}
