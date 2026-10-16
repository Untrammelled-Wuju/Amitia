// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package userfeedback

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/system"
	"gorm.io/gorm"
)

type Service struct {
	repo     Repository
	settings *system.SettingsStore
	once     sync.Once
}

func NewService(db *gorm.DB) *Service {
	return &Service{repo: NewRepository(db), settings: system.NewSettingsStore(db)}
}

func (s *Service) uploadURL() string {
	value, _, err := s.settings.Get(UploadURLSettingKey)
	if err != nil || strings.TrimSpace(value) == "" {
		return readEnvOrDefault("AMITIA_FEEDBACK_UPLOAD_URL", DefaultUploadURL)
	}
	return strings.TrimSpace(value)
}

func (s *Service) Settings() *Settings {
	return &Settings{
		UploadURL:        s.uploadURL(),
		DefaultUploadURL: readEnvOrDefault("AMITIA_FEEDBACK_UPLOAD_URL", DefaultUploadURL),
		LogLineLimit:     defaultLogLineLimit,
		ErrorLineLimit:   defaultErrorLineLimit,
		MaxScreenshotMB:  maxScreenshotBytes / 1024 / 1024,
	}
}

func (s *Service) SaveUploadURL(raw string) (*Settings, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		value = readEnvOrDefault("AMITIA_FEEDBACK_UPLOAD_URL", DefaultUploadURL)
	}
	if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
		return nil, fmt.Errorf("上报地址必须以 http:// 或 https:// 开头")
	}
	if _, err := s.settings.Upsert(UploadURLSettingKey, value); err != nil {
		return nil, err
	}
	return s.Settings(), nil
}

type PreviewResult struct {
	Payload   map[string]any       `json:"payload"`
	Text      string               `json:"text"`
	UploadURL string               `json:"uploadUrl"`
	Snapshot  *DiagnosticsSnapshot `json:"snapshot"`
}

func (s *Service) Preview(input *TicketInput) (*PreviewResult, error) {
	ticket, snapshot, err := s.prepare(input)
	if err != nil {
		return nil, err
	}
	payload := buildPayload(ticket, snapshot)
	return &PreviewResult{
		Payload:   payload,
		Text:      renderPreviewText(payload),
		UploadURL: ticket.UploadURL,
		Snapshot:  snapshot,
	}, nil
}

func (s *Service) Create(spaceID string, input *TicketInput) (*Ticket, error) {
	ticket, snapshot, err := s.prepare(input)
	if err != nil {
		return nil, err
	}
	ticket.ID = uuid.NewString()
	ticket.SpaceID = strings.TrimSpace(spaceID)
	if snapshot != nil {
		snapshot.GeneratedAt = time.Now().Format(time.RFC3339)
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("生成诊断信息失败: %w", err)
	}
	ticket.Diagnostics = string(encoded)
	if input != nil && input.Screenshot != nil && strings.TrimSpace(input.Screenshot.DataBase64) != "" {
		name, path, _, size, screenshotErr := s.saveScreenshot(ticket.ID, input.Screenshot)
		if screenshotErr != nil {
			return nil, screenshotErr
		}
		ticket.ScreenshotName = name
		ticket.ScreenshotPath = path
		ticket.ScreenshotSize = size
		ticket.HasScreenshot = path != ""
	}
	if err := s.repo.Create(ticket); err != nil {
		return nil, fmt.Errorf("保存反馈失败: %w", err)
	}
	s.StartWorker()
	go s.attemptUpload(ticket.ID)
	stored, err := s.repo.GetByID(ticket.SpaceID, ticket.ID)
	if err != nil {
		return ticket, nil
	}
	return stored, nil
}

func (s *Service) List(spaceID string, limit int) ([]Ticket, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.repo.ListBySpace(strings.TrimSpace(spaceID), limit)
}

func (s *Service) Retry(spaceID, id string) (*Ticket, error) {
	ticket, err := s.repo.GetByID(strings.TrimSpace(spaceID), strings.TrimSpace(id))
	if err != nil {
		return nil, fmt.Errorf("反馈记录不存在")
	}
	if ticket.Status != StatusUploaded {
		s.attemptUpload(ticket.ID)
	}
	updated, reloadErr := s.repo.GetByID(ticket.SpaceID, ticket.ID)
	if reloadErr != nil {
		return ticket, nil
	}
	return updated, nil
}

func (s *Service) prepare(input *TicketInput) (*Ticket, *DiagnosticsSnapshot, error) {
	normalized, err := normalizeInput(input)
	if err != nil {
		return nil, nil, err
	}
	snapshot, err := buildSnapshot(normalized, normalized.ClientErrors)
	if err != nil {
		return nil, nil, err
	}
	includeLogs := normalized.IncludeLogs == nil || *normalized.IncludeLogs
	now := time.Now().Format(time.RFC3339)
	description, descriptionRedacted := redactText(normalized.Description, 0)
	snapshot.RedactedCount += descriptionRedacted
	ticket := &Ticket{
		Category:     normalized.Category,
		Title:        normalized.Title,
		Description:  description,
		Contact:      normalized.Contact,
		Platform:     snapshot.Platform,
		ClientType:   snapshot.ClientType,
		AppVersion:   snapshot.AppVersion,
		RuntimeMode:  snapshot.RuntimeMode,
		OSInfo:       snapshot.OSInfo,
		DeviceHash:   snapshot.DeviceHash,
		Locale:       snapshot.Locale,
		Timezone:     snapshot.Timezone,
		IncludeLogs:  includeLogs,
		LogLineCount: len(snapshot.RecentLines),
		UploadURL:    firstNonEmpty(normalized.UploadURL, s.uploadURL()),
		Status:       StatusPending,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	return ticket, snapshot, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func (s *Service) attemptUpload(id string) {
	ticket, err := s.repo.Get(id)
	if err != nil {
		return
	}
	snapshot := decodeSnapshot(ticket)
	ticket.Attempts++
	result, uploadErr := s.uploadTicket(context.Background(), ticket, snapshot)
	now := time.Now().Format(time.RFC3339)
	if uploadErr != nil {
		ticket.Status = StatusFailed
		ticket.LastError = truncateError(uploadErr.Error())
		ticket.UpdatedAt = now
		_ = s.repo.Update(ticket)
		return
	}
	ticket.Status = StatusUploaded
	ticket.LastError = ""
	ticket.UploadedAt = now
	ticket.UpdatedAt = now
	if result != nil {
		ticket.RemoteID = result.RemoteID
		ticket.RemoteURL = result.RemoteURL
	}
	_ = s.repo.Update(ticket)
}

func truncateError(message string) string {
	value := strings.TrimSpace(message)
	if len(value) > 1000 {
		return value[:1000]
	}
	return value
}

func (s *Service) StartWorker() {
	s.once.Do(func() {
		go func() {
			ticker := time.NewTicker(3 * time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				items, err := s.repo.ListPending(10)
				if err != nil {
					continue
				}
				for index := range items {
					s.attemptUpload(items[index].ID)
				}
			}
		}()
	})
}

func normalizeInput(input *TicketInput) (*TicketInput, error) {
	if input == nil {
		return nil, fmt.Errorf("反馈内容不能为空")
	}
	category := strings.ToLower(strings.TrimSpace(input.Category))
	if category == "" {
		category = CategoryBug
	}
	if !validCategories[category] {
		return nil, fmt.Errorf("反馈类型无效")
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return nil, fmt.Errorf("请填写反馈标题")
	}
	if len([]rune(title)) > 120 {
		return nil, fmt.Errorf("标题最多 120 个字符")
	}
	description := strings.TrimSpace(input.Description)
	if description == "" {
		return nil, fmt.Errorf("请描述遇到的问题")
	}
	if len([]rune(description)) > 8000 {
		return nil, fmt.Errorf("描述最多 8000 个字符")
	}
	contact := []rune(strings.TrimSpace(input.Contact))
	if len(contact) > 200 {
		contact = contact[:200]
	}
	normalized := &TicketInput{
		Category:     category,
		Title:        title,
		Description:  description,
		Contact:      string(contact),
		IncludeLogs:  input.IncludeLogs,
		ClientErrors: input.ClientErrors,
		Platform:     strings.TrimSpace(input.Platform),
		ClientType:   strings.TrimSpace(input.ClientType),
		AppVersion:   strings.TrimSpace(input.AppVersion),
		RuntimeMode:  strings.TrimSpace(input.RuntimeMode),
		OSInfo:       strings.TrimSpace(input.OSInfo),
		Locale:       strings.TrimSpace(input.Locale),
		Timezone:     strings.TrimSpace(input.Timezone),
		DeviceID:     strings.TrimSpace(input.DeviceID),
		UploadURL:    strings.TrimSpace(input.UploadURL),
		Screenshot:   input.Screenshot,
	}
	return normalized, nil
}

func decodeSnapshot(ticket *Ticket) *DiagnosticsSnapshot {
	snapshot := &DiagnosticsSnapshot{}
	if ticket == nil || strings.TrimSpace(ticket.Diagnostics) == "" {
		return snapshot
	}
	_ = json.Unmarshal([]byte(ticket.Diagnostics), snapshot)
	if snapshot.RecentLines == nil {
		snapshot.RecentLines = []LogLine{}
	}
	if snapshot.ErrorLines == nil {
		snapshot.ErrorLines = []LogLine{}
	}
	if snapshot.ClientErrors == nil {
		snapshot.ClientErrors = []ClientError{}
	}
	if snapshot.LogFiles == nil {
		snapshot.LogFiles = []string{}
	}
	return snapshot
}
