package dataportability

import (
	"context"
	"encoding/json"
	"errors"
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

func (c *CharacterContributor) ID() string             { return "character" }
func (c *CharacterContributor) Name() string           { return "Character Records" }
func (c *CharacterContributor) Dependencies() []string { return nil }
func (c *CharacterContributor) ComponentIDs() []string { return []string{"character.records.v1"} }

type characterExportRecord struct {
	ID                  string  `json:"id"`
	SpaceID             string  `json:"spaceId"`
	GlobalMemoryEnabled bool    `json:"globalMemoryEnabled"`
	Name                string  `json:"name"`
	Avatar              string  `json:"avatar"`
	Identity            string  `json:"identity"`
	Personality         string  `json:"personality"`
	SpeakingStyle       string  `json:"speakingStyle"`
	RelationshipStyle   string  `json:"relationshipStyle"`
	CharacterBase       string  `json:"characterBase"`
	BoundaryRules       string  `json:"boundaryRules"`
	PersonalitySliders  string  `json:"personalitySliders"`
	Description         string  `json:"description"`
	BasePrompt          string  `json:"basePrompt"`
	GeneratedPrompt     string  `json:"generatedPrompt"`
	IsDefault           int     `json:"isDefault"`
	Status              string  `json:"status"`
	PersonalityConfig   string  `json:"personalityConfig"`
	ChatStyleConfig     string  `json:"chatStyleConfig"`
	SceneRules          string  `json:"sceneRules"`
	IsActive            int     `json:"isActive"`
	SortOrder           int     `json:"sortOrder"`
	CreatedAt           string  `json:"createdAt"`
	UpdatedAt           string  `json:"updatedAt"`
	Gender              string  `json:"gender"`
	GenderLabel         *string `json:"genderLabel"`
	Pronoun             string  `json:"pronoun"`
	SelfReference       string  `json:"selfReference"`
	UserAddressingStyle *string `json:"userAddressingStyle"`
	GenderExpression    int     `json:"genderExpression"`
	LifeIdentity        string  `json:"lifeIdentity"`
	VoiceConfigID       string  `json:"voiceConfigId"`
	VoiceType           string  `json:"voiceType"`
	VoiceSpeed          float64 `json:"voiceSpeed"`
	VoicePitch          float64 `json:"voicePitch"`
	VoiceVolume         float64 `json:"voiceVolume"`
	CustomVoiceID       string  `json:"customVoiceId"`
	VoiceMode           string  `json:"voiceMode"`
	Emotion             string  `json:"emotion"`
	EmotionScale        int     `json:"emotionScale"`
	SilenceDuration     int     `json:"silenceDuration"`
	CardDataJSON        string  `json:"cardData"`
	Revision            int64   `json:"revision"`
}

func (c *CharacterContributor) Plan(ctx context.Context, req BackupRequest) ([]BackupComponentPlan, error) {
	if req.EffectiveScope() == ScopeSettings || req.EffectiveScope() == ScopeMemory {
		return nil, nil
	}
	var count int64
	if req.EffectiveScope() == ScopeCharacter && req.CharacterID != "" {
		c.DB.WithContext(ctx).Table("characters").Where("id = ?", req.CharacterID).Where("deleted_at IS NULL").Count(&count)
	} else {
		c.DB.WithContext(ctx).Table("characters").Where("deleted_at IS NULL").Count(&count)
	}
	if count == 0 {
		return nil, ErrBackupNotFound
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
	if req.EffectiveScope() == ScopeSettings || req.EffectiveScope() == ScopeMemory {
		return nil
	}
	compW, err := out.CreateComponent("character.records.v1", "character.records", KindDataset)
	if err != nil {
		return err
	}
	defer compW.Close()

	query := c.DB.WithContext(ctx).Table("characters").Where("deleted_at IS NULL")
	if req.EffectiveScope() == ScopeCharacter && req.CharacterID != "" {
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
	opts := RestoreOptions{
		OperationID:        req.OperationID,
		Purpose:            RestorePurposeOrdinary,
		CharacterPolicy:    req.CharacterPolicy,
		DefaultCharacterID: req.DefaultCharacterID,
		ActivateImported:   req.ActivateImported,
		IdentityMap:        req.IdentityMap,
		SecretProvider:     req.SecretProvider,
	}
	return c.RestoreCharacters(ctx, in, opts)
}

func (c *CharacterContributor) RestoreCharacters(ctx context.Context, in BackupReader, opts RestoreOptions) error {
	rc, err := in.ReadComponent("character.records.v1")
	if err != nil {
		return err
	}
	defer rc.Close()

	data, err := readAllContrib(rc)
	if err != nil {
		return err
	}
	if len(data) > 512<<20 {
		return ErrImportPackageInvalid
	}

	idMap := opts.IdentityMap
	lines := splitLines(data)
	return c.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, line := range lines {
			if len(line) == 0 {
				continue
			}
			var rec characterExportRecord
			if err := json.Unmarshal(line, &rec); err != nil {
				return fmt.Errorf("%w: character record", ErrImportPackageInvalid)
			}

			newID := rec.ID
			var existing struct{ ID string }
			if err := tx.WithContext(ctx).Table("characters").Select("id").Where("id = ?", rec.ID).Scan(&existing).Error; err != nil {
				return err
			}
			if existing.ID != "" {
				switch opts.CharacterPolicy {
				case CollisionSkip:
					if idMap != nil {
						idMap.AddCharacter(rec.ID, existing.ID)
					}
					continue
				case CollisionReplace:
					newID = rec.ID
				case CollisionDuplicate:
					newID = uuid.New().String()
				default:
					return ErrImportCollisionPolicyInvalid
				}
			}

			updates := map[string]interface{}{
				"space_id":              rec.SpaceID,
				"global_memory_enabled": rec.GlobalMemoryEnabled,
				"name":                  rec.Name,
				"avatar":                rec.Avatar,
				"identity":              rec.Identity,
				"personality":           rec.Personality,
				"speaking_style":        rec.SpeakingStyle,
				"relationship_style":    rec.RelationshipStyle,
				"character_base":        rec.CharacterBase,
				"boundary_rules":        rec.BoundaryRules,
				"personality_sliders":   rec.PersonalitySliders,
				"description":           rec.Description,
				"base_prompt":           rec.BasePrompt,
				"generated_prompt":      rec.GeneratedPrompt,
				"is_default":            rec.IsDefault,
				"status":                rec.Status,
				"personality_config":    rec.PersonalityConfig,
				"chat_style_config":     rec.ChatStyleConfig,
				"scene_rules":           rec.SceneRules,
				"is_active":             false,
				"sort_order":            rec.SortOrder,
				"created_at":            rec.CreatedAt,
				"updated_at":            rec.UpdatedAt,
				"gender":                rec.Gender,
				"gender_label":          rec.GenderLabel,
				"pronoun":               rec.Pronoun,
				"self_reference":        rec.SelfReference,
				"user_addressing_style": rec.UserAddressingStyle,
				"gender_expression":     rec.GenderExpression,
				"life_identity":         rec.LifeIdentity,
				"voice_config_id":       "",
				"voice_type":            rec.VoiceType,
				"voice_speed":           rec.VoiceSpeed,
				"voice_pitch":           rec.VoicePitch,
				"voice_volume":          rec.VoiceVolume,
				"custom_voice_id":       rec.CustomVoiceID,
				"voice_mode":            rec.VoiceMode,
				"emotion":               rec.Emotion,
				"emotion_scale":         rec.EmotionScale,
				"silence_duration":      rec.SilenceDuration,
				"card_data_json":        rec.CardDataJSON,
				"revision":              rec.Revision,
			}

			var writeErr error
			if newID == rec.ID && existing.ID != "" {
				writeErr = tx.WithContext(ctx).Table("characters").Where("id = ?", rec.ID).Updates(updates).Error
			} else {
				updates["id"] = newID
				writeErr = tx.WithContext(ctx).Table("characters").Create(updates).Error
			}
			if writeErr != nil {
				return writeErr
			}

			if idMap != nil {
				idMap.AddCharacter(rec.ID, newID)
			}
		}
		return nil
	})
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
			if errors.Is(err, io.EOF) {
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
