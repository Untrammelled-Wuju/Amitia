package temporal

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository interface {
	InitSchema() error
	GetProfile(ownerType, ownerID string) (*Profile, error)
	SaveProfile(*Profile) error
	ListProfiles() ([]Profile, error)
	CharacterExists(characterID string) (bool, error)
	ListAnchors(AnchorQuery) ([]Anchor, error)
	GetAnchor(id string) (*Anchor, error)
	SaveAnchor(*Anchor) error
	ListAllAnchors(status string, limit int) ([]Anchor, error)
	ListDueAnchors(now time.Time, limit int) ([]Anchor, error)
	DeleteAnchor(id, userID, characterID string) error
	CreateEvent(*Event) (bool, error)
	ListEvents(userID, characterID string, limit int) ([]Event, error)
	SaveMemoryTemporalMetadata(*MemoryTemporalMetadata) error
	GetMemoryTemporalMetadata(memoryIDs []string) (map[string]MemoryTemporalMetadata, error)
}

type SQLiteRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *SQLiteRepository { return &SQLiteRepository{db: db} }

func (r *SQLiteRepository) InitSchema() error {
	return r.db.AutoMigrate(&Profile{}, &Anchor{}, &Event{}, &MemoryTemporalMetadata{})
}

func (r *SQLiteRepository) GetProfile(ownerType, ownerID string) (*Profile, error) {
	var profile Profile
	err := r.db.Where("owner_type = ? AND owner_id = ?", ownerType, ownerID).First(&profile).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &profile, err
}

func (r *SQLiteRepository) SaveProfile(profile *Profile) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "owner_type"}, {Name: "owner_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"timezone_mode", "timezone", "locale", "calendar_system", "week_start", "holiday_region", "hemisphere", "daypart_config_json", "quiet_hours_json", "auto_detect_timezone", "travel_mode", "awareness_level", "source", "confidence", "pending_timezone", "enabled", "holiday_awareness", "daypart_awareness", "anniversary_awareness", "memory_resonance", "allow_shared_date_mention", "version", "updated_at_utc"}),
	}).Create(profile).Error
}

func (r *SQLiteRepository) ListProfiles() ([]Profile, error) {
	var profiles []Profile
	err := r.db.Find(&profiles).Error
	return profiles, err
}

func (r *SQLiteRepository) CharacterExists(characterID string) (bool, error) {
	var count int64
	err := r.db.Table("characters").Where("id = ?", characterID).Count(&count).Error
	return count > 0, err
}

func (r *SQLiteRepository) ListAnchors(query AnchorQuery) ([]Anchor, error) {
	limit := query.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	db := r.db.Where("user_id = ?", query.UserID)
	if query.CharacterID != "" {
		db = db.Where("character_id = ? OR character_id = ''", query.CharacterID)
	}
	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}
	var anchors []Anchor
	err := db.Order("importance DESC, updated_at_utc DESC").Limit(limit).Find(&anchors).Error
	return anchors, err
}

func (r *SQLiteRepository) GetAnchor(id string) (*Anchor, error) {
	var anchor Anchor
	err := r.db.Where("id = ?", id).First(&anchor).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &anchor, err
}

func (r *SQLiteRepository) SaveAnchor(anchor *Anchor) error { return r.db.Save(anchor).Error }

func (r *SQLiteRepository) ListAllAnchors(status string, limit int) ([]Anchor, error) {
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	db := r.db
	if status != "" {
		db = db.Where("status = ?", status)
	}
	var anchors []Anchor
	err := db.Order("next_occurrence_at_utc ASC").Limit(limit).Find(&anchors).Error
	return anchors, err
}

func (r *SQLiteRepository) ListDueAnchors(now time.Time, limit int) ([]Anchor, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	var anchors []Anchor
	err := r.db.Where("status = 'active' AND next_occurrence_at_utc IS NOT NULL AND next_occurrence_at_utc <= ?", utc(now)).Order("next_occurrence_at_utc ASC").Limit(limit).Find(&anchors).Error
	return anchors, err
}

func (r *SQLiteRepository) DeleteAnchor(id, userID, characterID string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("anchor_id = ?", id).Delete(&Event{}).Error; err != nil {
			return err
		}
		db := tx.Where("id = ? AND user_id = ?", id, userID)
		if characterID != "" {
			db = db.Where("character_id = ?", characterID)
		}
		return db.Delete(&Anchor{}).Error
	})
}

func (r *SQLiteRepository) CreateEvent(event *Event) (bool, error) {
	result := r.db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "idempotency_key"}}, DoNothing: true}).Create(event)
	return result.RowsAffected > 0, result.Error
}

func (r *SQLiteRepository) ListEvents(userID, characterID string, limit int) ([]Event, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	db := r.db.Where("user_id = ?", userID)
	if characterID != "" {
		db = db.Where("character_id = ? OR character_id = ''", characterID)
	}
	var events []Event
	err := db.Order("occurred_at_utc DESC").Limit(limit).Find(&events).Error
	return events, err
}

func (r *SQLiteRepository) SaveMemoryTemporalMetadata(metadata *MemoryTemporalMetadata) error {
	return r.db.Clauses(clause.OnConflict{UpdateAll: true}).Create(metadata).Error
}

func (r *SQLiteRepository) GetMemoryTemporalMetadata(memoryIDs []string) (map[string]MemoryTemporalMetadata, error) {
	result := map[string]MemoryTemporalMetadata{}
	if len(memoryIDs) == 0 {
		return result, nil
	}
	var items []MemoryTemporalMetadata
	if err := r.db.Where("memory_id IN ?", memoryIDs).Find(&items).Error; err != nil {
		return nil, err
	}
	for _, item := range items {
		result[item.MemoryID] = item
	}
	return result, nil
}

func utc(value time.Time) time.Time { return value.UTC().Round(0) }
