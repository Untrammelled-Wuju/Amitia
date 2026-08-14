// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package episodic

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/system/dataportability"
	"gorm.io/gorm"
)

const (
	ComponentIDEpisodicRecords      = "episodic.records.v1"
	logicalNameEpisodicRecords      = "episodic.records"
)

type EpisodicBackupContributor struct {
	DB *gorm.DB
}

func NewEpisodicBackupContributor(db *gorm.DB) *EpisodicBackupContributor {
	return &EpisodicBackupContributor{DB: db}
}

func (c *EpisodicBackupContributor) ID() string   { return "episodic" }
func (c *EpisodicBackupContributor) Name() string { return "Episodic Memory" }

type episodicRecordV1 struct {
	ID               string `json:"id"`
	UserID           string `json:"userId"`
	SceneType        string `json:"sceneType"`
	Title            string `json:"title"`
	Content          string `json:"content"`
	ContextBefore    string `json:"contextBefore"`
	ContextAfter     string `json:"contextAfter"`
	TriggerKeywords  string `json:"triggerKeywords"`
	SentimentScore   int    `json:"sentimentScore"`
	MessageIDStart   string `json:"messageIdStart"`
	MessageIDEnd     string `json:"messageIdEnd"`
	MessageTimeStart string `json:"messageTimeStart"`
	MessageTimeEnd   string `json:"messageTimeEnd"`
	SourceConvID     string `json:"sourceConvId"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}

func (c *EpisodicBackupContributor) Plan(ctx context.Context, req dataportability.BackupRequest) ([]dataportability.BackupComponentPlan, error) {
	var count int64
	c.DB.WithContext(ctx).Model(&EpisodicMemory{}).Count(&count)
	return []dataportability.BackupComponentPlan{
		{
			ID:            ComponentIDEpisodicRecords,
			Kind:          dataportability.KindNDJSON,
			LogicalName:   logicalNameEpisodicRecords,
			Required:      false,
			SourceOfTruth: true,
			ItemCount:     count,
			EstimatedSize: count * 1024,
		},
	}, nil
}

func (c *EpisodicBackupContributor) Export(ctx context.Context, req dataportability.BackupRequest, out dataportability.BackupWriter) error {
	compW, err := out.CreateComponent(ComponentIDEpisodicRecords, logicalNameEpisodicRecords, dataportability.KindNDJSON)
	if err != nil {
		return err
	}
	defer compW.Close()

	query := c.DB.WithContext(ctx).Table("episodic_memories").Select(
		"id, user_id, scene_type, title, content, context_before, context_after, trigger_keywords, sentiment_score, message_id_start, message_id_end, message_time_start, message_time_end, source_conv_id, created_at, updated_at",
	)

	rows, err := query.Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var rec episodicRecordV1
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

func (c *EpisodicBackupContributor) PreviewImport(ctx context.Context, req dataportability.ImportPreviewRequest, in dataportability.BackupReader) ([]dataportability.ImportComponentPreview, error) {
	rc, err := in.ReadComponent(ComponentIDEpisodicRecords)
	if err != nil {
		return []dataportability.ImportComponentPreview{{
			ComponentID: ComponentIDEpisodicRecords,
			Kind:        dataportability.KindNDJSON,
			LogicalName: logicalNameEpisodicRecords,
		}}, nil
	}
	defer rc.Close()

	preview := dataportability.ImportComponentPreview{
		ComponentID: ComponentIDEpisodicRecords,
		Kind:        dataportability.KindNDJSON,
		LogicalName: logicalNameEpisodicRecords,
	}

	scanner := bufio.NewScanner(rc)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec episodicRecordV1
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		preview.ItemCount++

		var existing struct{ ID string }
		c.DB.WithContext(ctx).Table("episodic_memories").Select("id").Where("id = ?", rec.ID).Scan(&existing)
		if existing.ID != "" {
			preview.Collisions = append(preview.Collisions, dataportability.ComponentCollision{
				SourceID:   rec.ID,
				TargetID:   existing.ID,
				EntityType: "episodic_memory",
				Policy:     dataportability.CollisionDuplicate,
			})
		}
	}

	return []dataportability.ImportComponentPreview{preview}, nil
}

func (c *EpisodicBackupContributor) Import(ctx context.Context, req dataportability.ImportRequest, in dataportability.BackupReader) error {
	rc, err := in.ReadComponent(ComponentIDEpisodicRecords)
	if err != nil {
		return err
	}
	defer rc.Close()

	scanner := bufio.NewScanner(rc)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec episodicRecordV1
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}

		m := EpisodicMemory{
			ID:               rec.ID,
			UserID:           rec.UserID,
			SceneType:        rec.SceneType,
			Title:            rec.Title,
			Content:          rec.Content,
			ContextBefore:    rec.ContextBefore,
			ContextAfter:     rec.ContextAfter,
			TriggerKeywords:  rec.TriggerKeywords,
			SentimentScore:   rec.SentimentScore,
			MessageIDStart:   rec.MessageIDStart,
			MessageIDEnd:     rec.MessageIDEnd,
			MessageTimeStart: rec.MessageTimeStart,
			MessageTimeEnd:   rec.MessageTimeEnd,
			SourceConvID:     rec.SourceConvID,
			CreatedAt:        rec.CreatedAt,
			UpdatedAt:        rec.UpdatedAt,
		}

		var existing struct{ ID string }
		c.DB.WithContext(ctx).Table("episodic_memories").Select("id").Where("id = ?", rec.ID).Scan(&existing)

		if existing.ID != "" {
			switch req.CharacterPolicy {
			case dataportability.CollisionSkip:
				continue
			case dataportability.CollisionReplace:
				c.DB.WithContext(ctx).Where("id = ?", rec.ID).Updates(&m)
			default:
				m.ID = uuid.New().String()
				c.DB.WithContext(ctx).Create(&m)
			}
		} else {
			c.DB.WithContext(ctx).Create(&m)
		}
	}

	return scanner.Err()
}

func init() {
	_ = fmt.Sprintf
	_ = io.EOF
}
