// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package adminrelease

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
	"gorm.io/gorm"
)

type UserFeedback struct {
	ID              uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	TicketID        string    `gorm:"size:64;not null;uniqueIndex" json:"ticketId"`
	Category        string    `gorm:"size:32;not null;index" json:"category"`
	Title           string    `gorm:"size:200;not null" json:"title"`
	Platform        string    `gorm:"size:64;index" json:"platform"`
	ClientType      string    `gorm:"size:32" json:"clientType"`
	AppVersion      string    `gorm:"size:64" json:"appVersion"`
	CoreVersion     string    `gorm:"size:64" json:"coreVersion"`
	RuntimeMode     string    `gorm:"size:32" json:"runtimeMode"`
	OSInfo          string    `gorm:"size:255" json:"osInfo"`
	Contact         string    `gorm:"size:200" json:"contact"`
	DeviceHash      string    `gorm:"size:128;index" json:"deviceHash"`
	LogLineCount    int       `json:"logLineCount"`
	ErrorLineCount  int       `json:"errorLineCount"`
	RedactedCount   int       `json:"redactedCount"`
	HasScreenshot   bool      `json:"hasScreenshot"`
	ScreenshotName  string    `gorm:"size:255" json:"screenshotName"`
	ScreenshotPath  string    `gorm:"size:512" json:"-"`
	ScreenshotSize  int64     `json:"screenshotSize"`
	ClientCreatedAt string    `gorm:"size:64" json:"clientCreatedAt"`
	Payload         string    `gorm:"type:longtext;not null" json:"-"`
	ReceivedAt      time.Time `gorm:"index" json:"receivedAt"`
}

func (UserFeedback) TableName() string { return "user_feedbacks" }

type feedbackPayload struct {
	TicketID       string            `json:"ticketId"`
	Category       string            `json:"category"`
	Title          string            `json:"title"`
	Description    string            `json:"description"`
	Contact        string            `json:"contact"`
	CreatedAt      string            `json:"createdAt"`
	Platform       string            `json:"platform"`
	ClientType     string            `json:"clientType"`
	AppVersion     string            `json:"appVersion"`
	CoreVersion    string            `json:"coreVersion"`
	RuntimeMode    string            `json:"runtimeMode"`
	OSInfo         string            `json:"osInfo"`
	DeviceHash     string            `json:"deviceHash"`
	Locale         string            `json:"locale"`
	Timezone       string            `json:"timezone"`
	IncludeLogs    bool              `json:"includeLogs"`
	LogLineCount   int               `json:"logLineCount"`
	RedactedCount  int               `json:"redactedCount"`
	ScreenshotName string            `json:"screenshotName"`
	ScreenshotSize int64             `json:"screenshotSize"`
	HasScreenshot  bool              `json:"hasScreenshot"`
	ErrorLines     []json.RawMessage `json:"errorLines"`
}

type ingestLimiter struct {
	mu     sync.Mutex
	hits   map[string]int
	limit  int
	window time.Duration
	reset  time.Time
}

func newIngestLimiter(limit int, window time.Duration) *ingestLimiter {
	return &ingestLimiter{hits: map[string]int{}, limit: limit, window: window, reset: time.Now().Add(window)}
}

func (l *ingestLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if now.After(l.reset) {
		l.hits = map[string]int{}
		l.reset = now.Add(l.window)
	}
	l.hits[key]++
	return l.hits[key] <= l.limit
}

func screenshotContentType(extension string) string {
	switch strings.ToLower(extension) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}

func (s *Server) handleIngestFeedback(c *gin.Context) {
	if s.feedback != nil && !s.feedback.allow(c.ClientIP()) {
		util.ErrorResponse(c, 429, "提交过于频繁，请稍后再试", nil)
		return
	}
	if token := strings.TrimSpace(s.cfg.FeedbackToken); token != "" {
		if strings.TrimSpace(c.GetHeader("X-Amitia-Feedback-Token")) != token {
			util.ErrorResponse(c, 401, "上报凭据无效", nil)
			return
		}
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, s.cfg.FeedbackMaxBytes)
	if err := c.Request.ParseMultipartForm(8 << 20); err != nil {
		util.ErrorResponse(c, 400, "解析上传内容失败", nil)
		return
	}
	raw := c.Request.FormValue("payload")
	if strings.TrimSpace(raw) == "" {
		util.ErrorResponse(c, 400, "缺少反馈内容", nil)
		return
	}
	var payload feedbackPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		util.ErrorResponse(c, 400, "反馈内容格式无效", nil)
		return
	}
	payload.TicketID = strings.TrimSpace(payload.TicketID)
	payload.Title = strings.TrimSpace(payload.Title)
	if payload.TicketID == "" || payload.Title == "" {
		util.ErrorResponse(c, 400, "反馈标识或标题缺失", nil)
		return
	}

	var existing UserFeedback
	if err := s.db.Where("ticket_id = ?", payload.TicketID).First(&existing).Error; err == nil {
		util.SuccessMsgResponse(c, "反馈已存在", gin.H{"id": existing.ID, "url": ""})
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		util.ErrorResponse(c, 500, "读取反馈记录失败", nil)
		return
	}

	record := &UserFeedback{
		TicketID:        payload.TicketID,
		Category:        truncateText(payload.Category, 32),
		Title:           truncateText(payload.Title, 200),
		Platform:        truncateText(payload.Platform, 64),
		ClientType:      truncateText(payload.ClientType, 32),
		AppVersion:      truncateText(payload.AppVersion, 64),
		CoreVersion:     truncateText(payload.CoreVersion, 64),
		RuntimeMode:     truncateText(payload.RuntimeMode, 32),
		OSInfo:          truncateText(payload.OSInfo, 255),
		Contact:         truncateText(payload.Contact, 200),
		DeviceHash:      truncateText(payload.DeviceHash, 128),
		LogLineCount:    payload.LogLineCount,
		ErrorLineCount:  len(payload.ErrorLines),
		RedactedCount:   payload.RedactedCount,
		ClientCreatedAt: truncateText(payload.CreatedAt, 64),
		Payload:         raw,
		ReceivedAt:      time.Now(),
	}
	if name, storedPath, size, err := s.saveFeedbackScreenshot(c, payload.TicketID); err != nil {
		util.ErrorResponse(c, 400, err.Error(), nil)
		return
	} else if storedPath != "" {
		record.HasScreenshot = true
		record.ScreenshotName = name
		record.ScreenshotPath = storedPath
		record.ScreenshotSize = size
	}
	if err := s.db.Create(record).Error; err != nil {
		util.ErrorResponse(c, 500, "保存反馈失败", nil)
		return
	}
	util.SuccessMsgResponse(c, "反馈已接收", gin.H{"id": record.ID, "url": ""})
}

func (s *Server) saveFeedbackScreenshot(c *gin.Context, ticketID string) (string, string, int64, error) {
	header, err := c.FormFile("screenshot")
	if err != nil || header == nil {
		return "", "", 0, nil
	}
	file, openErr := header.Open()
	if openErr != nil {
		return "", "", 0, fmt.Errorf("读取截图失败")
	}
	defer file.Close()
	name := filepath.Base(strings.TrimSpace(header.Filename))
	extension := strings.ToLower(filepath.Ext(name))
	switch extension {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif":
	default:
		return "", "", 0, fmt.Errorf("截图格式不受支持")
	}
	if err := os.MkdirAll(s.cfg.FeedbackDir, 0o755); err != nil {
		return "", "", 0, fmt.Errorf("创建反馈目录失败")
	}
	target := filepath.Join(s.cfg.FeedbackDir, ticketID+extension)
	output, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return "", "", 0, fmt.Errorf("保存截图失败")
	}
	written, copyErr := io.Copy(output, file)
	closeErr := output.Close()
	if copyErr != nil {
		_ = os.Remove(target)
		return "", "", 0, fmt.Errorf("保存截图失败")
	}
	if closeErr != nil {
		_ = os.Remove(target)
		return "", "", 0, fmt.Errorf("保存截图失败")
	}
	return name, target, written, nil
}

func truncateText(value string, limit int) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) <= limit {
		return trimmed
	}
	return trimmed[:limit]
}

func (s *Server) handleListFeedbacks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	query := s.db.Model(&UserFeedback{})
	if value := strings.TrimSpace(c.Query("category")); value != "" {
		query = query.Where("category = ?", value)
	}
	if value := strings.TrimSpace(c.Query("platform")); value != "" {
		query = query.Where("platform = ?", value)
	}
	if value := strings.TrimSpace(c.Query("keyword")); value != "" {
		like := "%" + value + "%"
		query = query.Where("title LIKE ? OR contact LIKE ? OR device_hash LIKE ? OR payload LIKE ?", like, like, like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		util.ErrorResponse(c, 500, "读取反馈数量失败", nil)
		return
	}
	items := []UserFeedback{}
	if err := query.Order("received_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		util.ErrorResponse(c, 500, "读取反馈列表失败", nil)
		return
	}
	util.SuccessResponse(c, gin.H{"items": items, "total": total, "page": page, "pageSize": pageSize})
}

func (s *Server) handleGetFeedback(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.ErrorResponse(c, 400, "反馈编号无效", nil)
		return
	}
	var record UserFeedback
	if err := s.db.First(&record, id).Error; err != nil {
		util.ErrorResponse(c, 404, "反馈记录不存在", nil)
		return
	}
	var payload json.RawMessage
	if err := json.Unmarshal([]byte(record.Payload), &payload); err != nil {
		payload = json.RawMessage("{}")
	}
	util.SuccessResponse(c, gin.H{
		"id":             record.ID,
		"ticketId":       record.TicketID,
		"receivedAt":     record.ReceivedAt,
		"payload":        payload,
		"hasScreenshot":  record.HasScreenshot,
		"screenshotName": record.ScreenshotName,
		"screenshotSize": record.ScreenshotSize,
		"screenshotURL":  screenshotURL(record),
	})
}

func screenshotURL(record UserFeedback) string {
	if !record.HasScreenshot || record.ScreenshotPath == "" {
		return ""
	}
	return fmt.Sprintf("/api/v1/admin/feedbacks/%d/screenshot", record.ID)
}

func (s *Server) handleFeedbackScreenshot(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.ErrorResponse(c, 400, "反馈编号无效", nil)
		return
	}
	var record UserFeedback
	if err := s.db.First(&record, id).Error; err != nil {
		util.ErrorResponse(c, 404, "反馈记录不存在", nil)
		return
	}
	if !record.HasScreenshot || record.ScreenshotPath == "" {
		util.ErrorResponse(c, 404, "该反馈没有截图", nil)
		return
	}
	if _, statErr := os.Stat(record.ScreenshotPath); statErr != nil {
		util.ErrorResponse(c, 404, "截图文件已丢失", nil)
		return
	}
	c.Header("Content-Type", screenshotContentType(filepath.Ext(record.ScreenshotPath)))
	c.File(record.ScreenshotPath)
}

func (s *Server) handleExportFeedbacks(c *gin.Context) {
	items := []UserFeedback{}
	if err := s.db.Order("received_at DESC").Limit(5000).Find(&items).Error; err != nil {
		util.ErrorResponse(c, 500, "读取反馈列表失败", nil)
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"amitia-feedbacks-%s.csv\"", time.Now().Format("20060102-150405")))
	c.Writer.WriteString("\uFEFF")
	writer := csv.NewWriter(c.Writer)
	_ = writer.Write([]string{"编号", "反馈标识", "类型", "标题", "平台", "客户端", "应用版本", "Core 版本", "运行模式", "联系方式", "设备标识", "日志行数", "报错行数", "收到时间"})
	for _, item := range items {
		_ = writer.Write([]string{
			strconv.FormatUint(item.ID, 10),
			item.TicketID,
			item.Category,
			item.Title,
			item.Platform,
			item.ClientType,
			item.AppVersion,
			item.CoreVersion,
			item.RuntimeMode,
			item.Contact,
			item.DeviceHash,
			strconv.Itoa(item.LogLineCount),
			strconv.Itoa(item.ErrorLineCount),
			item.ReceivedAt.Format(time.RFC3339),
		})
	}
	writer.Flush()
}
