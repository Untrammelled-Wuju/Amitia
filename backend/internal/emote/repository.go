package emote

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) DB() *gorm.DB { return r.db }

func (r *Repository) Get(id string) (*Emote, error) {
	var item Emote
	if err := r.db.Where("id = ? AND deleted_at IS NULL", id).First(&item).Error; err != nil {
		return nil, err
	}
	r.populate(&item)
	return &item, nil
}

func (r *Repository) GetByHash(hash string) (*Emote, error) {
	var item Emote
	if err := r.db.Where("file_hash = ? AND deleted_at IS NULL", hash).First(&item).Error; err != nil {
		return nil, err
	}
	r.populate(&item)
	return &item, nil
}

func (r *Repository) List(groupID, view, query string, page, pageSize int) ([]Emote, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 60
	}
	db := r.db.Model(&Emote{}).Where("emotes.deleted_at IS NULL")
	if groupID != "" {
		db = db.Joins("JOIN emote_group_items ON emote_group_items.emote_id = emotes.id").Where("emote_group_items.group_id = ?", groupID)
	}
	if view == "unassigned" {
		db = db.Where("NOT EXISTS (SELECT 1 FROM emote_group_items WHERE emote_group_items.emote_id = emotes.id)")
	}
	if view == "recent" {
		db = db.Joins("JOIN (SELECT emote_id, MAX(created_at) AS last_used FROM emote_send_records WHERE emote_id IS NOT NULL GROUP BY emote_id) recent ON recent.emote_id = emotes.id").Order("recent.last_used DESC")
	}
	query = strings.TrimSpace(query)
	if query != "" {
		like := "%" + query + "%"
		db = db.Where("emotes.name LIKE ? OR emotes.meaning LIKE ? OR emotes.keywords LIKE ?", like, like, like)
	}
	var total int64
	if err := db.Distinct("emotes.id").Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []Emote
	if view != "recent" {
		db = db.Order("emotes.created_at DESC")
	}
	if err := db.Select("emotes.*").Distinct().Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	for i := range items {
		r.populate(&items[i])
	}
	if items == nil {
		items = []Emote{}
	}
	return items, total, nil
}

func (r *Repository) populate(item *Emote) {
	item.DecodeKeywords()
	item.CharacterIDs = []string{}
	item.GroupIDs = []string{}
	r.db.Table("emote_character_bindings").Where("emote_id = ?", item.ID).Pluck("character_id", &item.CharacterIDs)
	r.db.Table("emote_group_items").Where("emote_id = ?", item.ID).Pluck("group_id", &item.GroupIDs)
}

func (r *Repository) Create(item *Emote, groupIDs, characterIDs []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(item).Error; err != nil {
			return err
		}
		if err := replaceRelations(tx, item.ID, groupIDs, characterIDs); err != nil {
			return err
		}
		return nil
	})
}

func (r *Repository) Update(item *Emote, groupIDs, characterIDs []string, replaceGroups, replaceCharacters bool) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(item).Error; err != nil {
			return err
		}
		if replaceGroups {
			if err := tx.Where("emote_id = ?", item.ID).Delete(&GroupItem{}).Error; err != nil {
				return err
			}
			for i, groupID := range uniqueStrings(groupIDs) {
				if err := tx.Create(&GroupItem{GroupID: groupID, EmoteID: item.ID, SortOrder: i}).Error; err != nil {
					return err
				}
			}
		}
		if replaceCharacters {
			if err := tx.Where("emote_id = ?", item.ID).Delete(&CharacterBinding{}).Error; err != nil {
				return err
			}
			for _, characterID := range uniqueStrings(characterIDs) {
				if err := tx.Create(&CharacterBinding{EmoteID: item.ID, CharacterID: characterID}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func replaceRelations(tx *gorm.DB, emoteID string, groupIDs, characterIDs []string) error {
	for i, groupID := range uniqueStrings(groupIDs) {
		if err := tx.Create(&GroupItem{GroupID: groupID, EmoteID: emoteID, SortOrder: i}).Error; err != nil {
			return err
		}
	}
	for _, characterID := range uniqueStrings(characterIDs) {
		if err := tx.Create(&CharacterBinding{EmoteID: emoteID, CharacterID: characterID}).Error; err != nil {
			return err
		}
	}
	return nil
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

func (r *Repository) SoftDelete(id string) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	return r.db.Model(&Emote{}).Where("id = ? AND deleted_at IS NULL", id).Updates(map[string]interface{}{"deleted_at": now, "enabled": 0, "ai_enabled": 0, "updated_at": now}).Error
}

func (r *Repository) ListGroups() ([]Group, error) {
	var groups []Group
	err := r.db.Order("sort_order ASC, created_at ASC").Find(&groups).Error
	if groups == nil {
		groups = []Group{}
	}
	return groups, err
}

func (r *Repository) CreateGroup(group *Group) error { return r.db.Create(group).Error }

func (r *Repository) UpdateGroup(id string, updates map[string]interface{}) error {
	res := r.db.Model(&Group{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) DeleteGroup(id string) error { return r.db.Delete(&Group{}, "id = ?", id).Error }

func (r *Repository) AddToGroup(groupID string, emoteIDs []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&Group{}).Where("id = ?", groupID).Count(&count).Error; err != nil || count == 0 {
			if err != nil {
				return err
			}
			return gorm.ErrRecordNotFound
		}
		for i, id := range uniqueStrings(emoteIDs) {
			if err := tx.Exec("INSERT OR IGNORE INTO emote_group_items (group_id, emote_id, sort_order) VALUES (?, ?, ?)", groupID, id, i).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repository) RemoveFromGroup(groupID, emoteID string) error {
	return r.db.Where("group_id = ? AND emote_id = ?", groupID, emoteID).Delete(&GroupItem{}).Error
}

func (r *Repository) ReorderGroups(ids []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for i, id := range uniqueStrings(ids) {
			if err := tx.Model(&Group{}).Where("id = ?", id).Update("sort_order", i).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repository) GetSettings(characterID string) (CharacterSettings, error) {
	settings := DefaultSettings(characterID)
	err := r.db.Where("character_id = ?", characterID).First(&settings).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return settings, nil
	}
	return settings, err
}

func (r *Repository) SaveSettings(settings *CharacterSettings) error {
	return r.db.Save(settings).Error
}

func (r *Repository) CharacterExists(id string) bool {
	var count int64
	r.db.Table("characters").Where("id = ?", id).Count(&count)
	return count > 0
}

func (r *Repository) GroupExists(id string) bool {
	var count int64
	r.db.Table("emote_groups").Where("id = ?", id).Count(&count)
	return count > 0
}

func (r *Repository) CanCharacterUse(item *Emote, characterID string) bool {
	if item == nil || item.Enabled != 1 || item.AIEnabled != 1 || item.DeletedAt != nil {
		return false
	}
	if item.RoleScope == RoleScopeAll {
		return true
	}
	if item.RoleScope != RoleScopeSelected {
		return false
	}
	var count int64
	r.db.Model(&CharacterBinding{}).Where("emote_id = ? AND character_id = ?", item.ID, characterID).Count(&count)
	return count > 0
}
