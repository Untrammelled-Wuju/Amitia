package affect

import (
	"encoding/json"
	"time"

	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type AffectRow struct {
	CharacterID string    `gorm:"column:character_id;primaryKey"`
	StateJSON   string    `gorm:"column:state_json;type:text;not null"`
	Version     string    `gorm:"column:version;not null"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null"`
}

func (AffectRow) TableName() string {
	return "affect_states"
}

type sqliteRepository struct {
	db *gorm.DB
}

func NewSQLiteRepository(ctx *app.AppContext) Repository {
	return &sqliteRepository{db: ctx.DB}
}

func (r *sqliteRepository) LoadState(characterID string) (*AffectState, error) {
	var row AffectRow
	err := r.db.Where("character_id = ?", characterID).First(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	var state AffectState
	if err := json.Unmarshal([]byte(row.StateJSON), &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func (r *sqliteRepository) SaveState(characterID string, state AffectState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	row := AffectRow{
		CharacterID: characterID,
		StateJSON:   string(data),
		Version:     string(state.Version),
		UpdatedAt:   state.UpdatedAt,
	}
	query := "INSERT INTO affect_states (character_id, state_json, version, updated_at) VALUES (?, ?, ?, ?) ON CONFLICT(character_id) DO UPDATE SET state_json = excluded.state_json, version = excluded.version, updated_at = excluded.updated_at"
	return r.db.Exec(query, row.CharacterID, row.StateJSON, row.Version, row.UpdatedAt).Error
}

func AutoMigrateAffect(db *gorm.DB) error {
	return db.AutoMigrate(&AffectRow{})
}
