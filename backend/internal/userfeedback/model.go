// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package userfeedback

const (
	CategoryBug        = "bug"
	CategoryExperience = "experience"
	CategoryFeature    = "feature"
	CategoryOther      = "other"

	StatusPending  = "pending"
	StatusUploaded = "uploaded"
	StatusFailed   = "failed"

	UploadURLSettingKey = "user_feedback_upload_url"
	DefaultUploadURL    = "https://amitia.untrammelled.top/api/v1/feedbacks"

	defaultLogLineLimit   = 120
	defaultErrorLineLimit = 40
	maxLogLineLength      = 4000
	maxClientErrors       = 20
	maxScreenshotBytes    = 6 * 1024 * 1024
	maxUploadAttempts     = 8
)

var validCategories = map[string]bool{
	CategoryBug:        true,
	CategoryExperience: true,
	CategoryFeature:    true,
	CategoryOther:      true,
}

type Ticket struct {
	ID             string `gorm:"column:id;primaryKey" json:"id"`
	SpaceID        string `gorm:"column:space_id;index" json:"spaceId"`
	Category       string `gorm:"column:category" json:"category"`
	Title          string `gorm:"column:title" json:"title"`
	Description    string `gorm:"column:description" json:"description"`
	Contact        string `gorm:"column:contact" json:"contact"`
	Platform       string `gorm:"column:platform" json:"platform"`
	ClientType     string `gorm:"column:client_type" json:"clientType"`
	AppVersion     string `gorm:"column:app_version" json:"appVersion"`
	RuntimeMode    string `gorm:"column:runtime_mode" json:"runtimeMode"`
	OSInfo         string `gorm:"column:os_info" json:"osInfo"`
	DeviceHash     string `gorm:"column:device_hash" json:"deviceHash"`
	Locale         string `gorm:"column:locale" json:"locale"`
	Timezone       string `gorm:"column:timezone" json:"timezone"`
	IncludeLogs    bool   `gorm:"column:include_logs" json:"includeLogs"`
	LogLineCount   int    `gorm:"column:log_line_count" json:"logLineCount"`
	Diagnostics    string `gorm:"column:diagnostics" json:"-"`
	UploadURL      string `gorm:"column:upload_url" json:"uploadUrl"`
	Status         string `gorm:"column:status;index" json:"status"`
	Attempts       int    `gorm:"column:attempts" json:"attempts"`
	LastError      string `gorm:"column:last_error" json:"lastError"`
	RemoteID       string `gorm:"column:remote_id" json:"remoteId"`
	RemoteURL      string `gorm:"column:remote_url" json:"remoteUrl"`
	ScreenshotName string `gorm:"column:screenshot_name" json:"screenshotName"`
	ScreenshotPath string `gorm:"column:screenshot_path" json:"-"`
	ScreenshotSize int64  `gorm:"column:screenshot_size" json:"screenshotSize"`
	HasScreenshot  bool   `gorm:"column:has_screenshot" json:"hasScreenshot"`
	CreatedAt      string `gorm:"column:created_at;index" json:"createdAt"`
	UpdatedAt      string `gorm:"column:updated_at" json:"updatedAt"`
	UploadedAt     string `gorm:"column:uploaded_at" json:"uploadedAt"`
}

func (Ticket) TableName() string { return "user_feedback_tickets" }

type ClientError struct {
	Message string `json:"message"`
	Stack   string `json:"stack"`
	Source  string `json:"source"`
	At      string `json:"at"`
}

type ScreenshotInput struct {
	Name       string `json:"name"`
	MediaType  string `json:"mediaType"`
	DataBase64 string `json:"dataBase64"`
}

type TicketInput struct {
	Category     string           `json:"category"`
	Title        string           `json:"title"`
	Description  string           `json:"description"`
	Contact      string           `json:"contact"`
	IncludeLogs  *bool            `json:"includeLogs"`
	ClientErrors []ClientError    `json:"clientErrors"`
	Platform     string           `json:"platform"`
	ClientType   string           `json:"clientType"`
	AppVersion   string           `json:"appVersion"`
	RuntimeMode  string           `json:"runtimeMode"`
	OSInfo       string           `json:"osInfo"`
	Locale       string           `json:"locale"`
	Timezone     string           `json:"timezone"`
	DeviceID     string           `json:"deviceId"`
	UploadURL    string           `json:"uploadUrl"`
	Screenshot   *ScreenshotInput `json:"screenshot"`
}

type LogLine struct {
	File string `json:"file"`
	Line string `json:"line"`
}

type DiagnosticsSnapshot struct {
	GeneratedAt   string            `json:"generatedAt"`
	CoreVersion   string            `json:"coreVersion"`
	AppVersion    string            `json:"appVersion"`
	Platform      string            `json:"platform"`
	ClientType    string            `json:"clientType"`
	RuntimeMode   string            `json:"runtimeMode"`
	OSInfo        string            `json:"osInfo"`
	DeviceHash    string            `json:"deviceHash"`
	Locale        string            `json:"locale"`
	Timezone      string            `json:"timezone"`
	LogDirectory  string            `json:"logDirectory"`
	LogFiles      []string          `json:"logFiles"`
	ErrorLines    []LogLine         `json:"errorLines"`
	RecentLines   []LogLine         `json:"recentLines"`
	ClientErrors  []ClientError     `json:"clientErrors"`
	RedactedCount int               `json:"redactedCount"`
	DataSummary   map[string]string `json:"dataSummary"`
}

type Settings struct {
	UploadURL        string `json:"uploadUrl"`
	DefaultUploadURL string `json:"defaultUploadUrl"`
	LogLineLimit     int    `json:"logLineLimit"`
	ErrorLineLimit   int    `json:"errorLineLimit"`
	MaxScreenshotMB  int    `json:"maxScreenshotMB"`
}
