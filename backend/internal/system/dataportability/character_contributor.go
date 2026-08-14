package dataportability

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CharacterContributor struct {
	DB *gorm.DB
}

func NewCharacterContributor(db *gorm.DB) *CharacterContributor {
	return &CharacterContributor{DB: db}
}

func (c *CharacterContributor) ID() string   { return "character" }
func (c *CharacterContributor) Name() string { return "Character Records" }
func (c *CharacterContributor) Dependencies() []string { return nil }

type characterExportRecord struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	Personality     string `json:"personality"`
	SystemPrompt    string `json:"system_prompt"`
	BasePrompt      string `json:"base_prompt"`
	CharacterBase   string `json:"character_base"`
	Avatar          string `json:"avatar"`
	Status          string `json:"status"`
	IsActive        bool   `json:"is_active"`
	ConversationID  string `json:"conversation_id"`
	CardDataJSON    string `json:"card_data_json"`
	VoiceConfigID   string `json:"voice_config_id"`
	Gender          string `json:"gender"`
	SortOrder       int    `json:"sort_order"`
}

func (c *CharacterContributor) Plan(ctx context.Context, req BackupRequest) ([]BackupComponentPlan, error) {
	var count int64
	if req.Scope == ScopeCharacter && req.CharacterID != "" {
		c.DB.WithContext(ctx).Model(&characterExportRecord{}).Where("id = ?", req.CharacterID).Count(&count)
	} else {
		c.DB.WithContext(ctx).Model(&characterExportRecord{}).Count(&count)
	}

	return []BackupComponentPlan{
		{
			ID:            "character.records.v1",
			Kind:          KindDataset,
			LogicalName:   "character.records",
			Required:      true,
			SourceOfTruth: true,
			ItemCount:     count,
			EstimatedSize: estimateRecordsSize(count, 2048),
		},
	}, nil
}

func (c *CharacterContributor) Export(ctx context.Context, req BackupRequest, out BackupWriter) error {
	compW, err := out.CreateComponent("character.records.v1", "character.records", KindDataset)
	if err != nil {
		return err
	}
	defer compW.Close()

	query := c.DB.WithContext(ctx).Table("characters").Select(
		"id, name, description, personality, SUBSTR(base_prompt, 1, 10000) as system_prompt, base_prompt, character_base, avatar, status, is_active, conversation_id, card_data_json, voice_config_id, gender, sort_order",
	)
	if req.Scope == ScopeCharacter && req.CharacterID != "" {
		query = query.Where("id = ?", req.CharacterID)
	}

	rows, err := query.Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var rec characterExportRecord
		if err := c.DB.ScanRows(rows, &rec); err != nil {
			continue
		}
		data, err := json.Marshal(rec)
		if err != nil {
			continue
		}
		compW.Write(data)
		compW.Write([]byte("\n"))
	}
	return rows.Err()
}

func (c *CharacterContributor) PreviewImport(ctx context.Context, req ImportPreviewRequest, in BackupReader) ([]ImportComponentPreview, error) {
	rc, err := in.ReadComponent("character.records.v1")
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	preview := ImportComponentPreview{
		ComponentID: "character.records.v1",
		Kind:        KindDataset,
		LogicalName: "character.records",
	}

	data, err := readAllContrib(rc)
	if err != nil {
		return nil, err
	}

	lines := splitLines(data)
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var rec characterExportRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		preview.ItemCount++

		var existing struct {
			ID   string
			Name string
		}
		c.DB.WithContext(ctx).Table("characters").Select("id, name").Where("id = ?", rec.ID).Scan(&existing)
		if existing.ID != "" {
			preview.Collisions = append(preview.Collisions, ComponentCollision{
				SourceID:   rec.ID,
				TargetID:   existing.ID,
				EntityType: "character",
				Policy:     CollisionDuplicate,
			})
		}
	}

	return []ImportComponentPreview{preview}, nil
}

func (c *CharacterContributor) Import(ctx context.Context, req ImportRequest, in BackupReader) error {
	rc, err := in.ReadComponent("character.records.v1")
	if err != nil {
		return err
	}
	defer rc.Close()

	data, err := readAllContrib(rc)
	if err != nil {
		return err
	}

	idMap := req.IdentityMap
	lines := splitLines(data)
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var rec characterExportRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}

		newID := rec.ID
		var existing struct{ ID string }
		c.DB.WithContext(ctx).Table("characters").Select("id").Where("id = ?", rec.ID).Scan(&existing)
		if existing.ID != "" {
			switch req.CharacterPolicy {
			case CollisionSkip:
				if idMap != nil {
					idMap.AddCharacter(rec.ID, existing.ID)
				}
				continue
			case CollisionReplace:
				newID = rec.ID
			default:
				newID = uuid.New().String()
			}
		}

		updates := map[string]interface{}{
			"name":            rec.Name,
			"description":     rec.Description,
			"personality":     rec.Personality,
			"base_prompt":     rec.BasePrompt,
			"character_base":  rec.CharacterBase,
			"avatar":          rec.Avatar,
			"status":          rec.Status,
			"is_active":       false,
			"conversation_id": "",
			"card_data_json":  rec.CardDataJSON,
			"voice_config_id": "",
			"gender":          rec.Gender,
			"sort_order":      rec.SortOrder,
		}

		if newID == rec.ID && existing.ID != "" {
			c.DB.WithContext(ctx).Table("characters").Where("id = ?", rec.ID).Updates(updates)
		} else {
			updates["id"] = newID
			c.DB.WithContext(ctx).Table("characters").Create(updates)
		}

		if idMap != nil && newID != rec.ID {
			idMap.AddCharacter(rec.ID, newID)
		}
	}

	return nil
}

func readAllContrib(r io.Reader) ([]byte, error) {
	return readAllContribImpl(r)
}

func readAllContribImpl(r io.Reader) ([]byte, error) {
	result := make([]byte, 0, 4096)
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			result = append(result, buf[:n]...)
		}
		if err != nil {
			if err.Error() == "EOF" {
				return result, nil
			}
			return result, err
		}
	}
}

func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			line := data[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			lines = append(lines, line)
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}

func estimateRecordsSize(count int64, avgRecordSize int64) int64 {
	return count * avgRecordSize
}

func init() {
	_ = hex.EncodeToString
	_ = fmt.Sprintf
}
