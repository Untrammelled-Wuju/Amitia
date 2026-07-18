package emote

import "encoding/json"

const (
	RoleScopeAll                = "all_characters"
	RoleScopeSelected           = "selected_characters"
	TriggerManual               = "user_manual"
	TriggerAIRandom             = "ai_random"
	SendModeAfterAllText        = "after_all_text"
	SendModeBetweenTextMessages = "between_text_messages"
	SendModeEmoteOnly           = "emote_only"
	SendModeManual              = "manual"
	CollectionName              = "amitia_emotes"
)

type Emote struct {
	ID               string   `gorm:"column:id;primaryKey" json:"id"`
	Name             string   `gorm:"column:name" json:"name"`
	Meaning          string   `gorm:"column:meaning" json:"meaning"`
	Keywords         string   `gorm:"column:keywords" json:"-"`
	KeywordList      []string `gorm:"-" json:"keywords"`
	OriginalFilename string   `gorm:"column:original_filename" json:"originalFilename"`
	FilePath         string   `gorm:"column:file_path" json:"filePath"`
	ThumbnailPath    string   `gorm:"column:thumbnail_path" json:"thumbnailPath"`
	FallbackPath     string   `gorm:"column:fallback_path" json:"fallbackPath"`
	MimeType         string   `gorm:"column:mime_type" json:"mimeType"`
	FileExtension    string   `gorm:"column:file_extension" json:"fileExtension"`
	FileSize         int64    `gorm:"column:file_size" json:"fileSize"`
	Width            int      `gorm:"column:width" json:"width"`
	Height           int      `gorm:"column:height" json:"height"`
	IsAnimated       int      `gorm:"column:is_animated" json:"isAnimated"`
	DurationMS       int      `gorm:"column:duration_ms" json:"durationMs"`
	FrameCount       int      `gorm:"column:frame_count" json:"frameCount"`
	FileHash         string   `gorm:"column:file_hash" json:"fileHash"`
	Enabled          int      `gorm:"column:enabled" json:"enabled"`
	AIEnabled        int      `gorm:"column:ai_enabled" json:"aiEnabled"`
	RoleScope        string   `gorm:"column:role_scope" json:"roleScope"`
	VectorStatus     string   `gorm:"column:vector_status" json:"vectorStatus"`
	VectorError      string   `gorm:"column:vector_error" json:"vectorError,omitempty"`
	CreatedAt        string   `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt        string   `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt        *string  `gorm:"column:deleted_at" json:"deletedAt,omitempty"`
	CharacterIDs     []string `gorm:"-" json:"characterIds"`
	GroupIDs         []string `gorm:"-" json:"groupIds"`
}

func (Emote) TableName() string { return "emotes" }

func (e *Emote) DecodeKeywords() {
	e.KeywordList = []string{}
	_ = json.Unmarshal([]byte(e.Keywords), &e.KeywordList)
}

type Group struct {
	ID           string  `gorm:"column:id;primaryKey" json:"id"`
	Name         string  `gorm:"column:name" json:"name"`
	CoverEmoteID *string `gorm:"column:cover_emote_id" json:"coverEmoteId,omitempty"`
	SortOrder    int     `gorm:"column:sort_order" json:"sortOrder"`
	CreatedAt    string  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt    string  `gorm:"column:updated_at" json:"updatedAt"`
}

func (Group) TableName() string { return "emote_groups" }

type GroupItem struct {
	GroupID   string `gorm:"column:group_id;primaryKey"`
	EmoteID   string `gorm:"column:emote_id;primaryKey"`
	SortOrder int    `gorm:"column:sort_order"`
}

func (GroupItem) TableName() string { return "emote_group_items" }

type CharacterBinding struct {
	EmoteID     string `gorm:"column:emote_id;primaryKey"`
	CharacterID string `gorm:"column:character_id;primaryKey"`
}

func (CharacterBinding) TableName() string { return "emote_character_bindings" }

type CharacterSettings struct {
	CharacterID              string  `gorm:"column:character_id;primaryKey" json:"characterId"`
	Enabled                  int     `gorm:"column:enabled" json:"enabled"`
	BaseProbability          float64 `gorm:"column:base_probability" json:"baseProbability"`
	MaxProbability           float64 `gorm:"column:max_probability" json:"maxProbability"`
	MaxPerHour               int     `gorm:"column:max_per_hour" json:"maxPerHour"`
	MinReplyGap              int     `gorm:"column:min_reply_gap" json:"minReplyGap"`
	SameEmoteCooldownMinutes int     `gorm:"column:same_emote_cooldown_minutes" json:"sameEmoteCooldownMinutes"`
	AllowEmoteOnly           int     `gorm:"column:allow_emote_only" json:"allowEmoteOnly"`
	CreatedAt                string  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt                string  `gorm:"column:updated_at" json:"updatedAt"`
}

func (CharacterSettings) TableName() string { return "character_emote_settings" }

func DefaultSettings(characterID string) CharacterSettings {
	return CharacterSettings{CharacterID: characterID, Enabled: 1, BaseProbability: 0.10, MaxProbability: 0.30, MaxPerHour: 5, MinReplyGap: 3, SameEmoteCooldownMinutes: 30, AllowEmoteOnly: 0}
}

type SendRecord struct {
	ID                 string  `gorm:"column:id;primaryKey" json:"id"`
	EmoteID            *string `gorm:"column:emote_id" json:"emoteId,omitempty"`
	CharacterID        string  `gorm:"column:character_id" json:"characterId"`
	ConversationID     string  `gorm:"column:conversation_id" json:"conversationId"`
	MessageID          string  `gorm:"column:message_id" json:"messageId"`
	ResponseID         string  `gorm:"column:response_id" json:"responseId"`
	Platform           string  `gorm:"column:platform" json:"platform"`
	TriggerType        string  `gorm:"column:trigger_type" json:"triggerType"`
	TriggerProbability float64 `gorm:"column:trigger_probability" json:"triggerProbability"`
	RandomSample       float64 `gorm:"column:random_sample" json:"randomSample"`
	TriggerHit         int     `gorm:"column:trigger_hit" json:"triggerHit"`
	DecisionReason     string  `gorm:"column:decision_reason" json:"decisionReason"`
	SendMode           string  `gorm:"column:send_mode" json:"sendMode"`
	DeliveryKey        string  `gorm:"column:delivery_key" json:"deliveryKey"`
	Status             string  `gorm:"column:status" json:"status"`
	FailureReason      string  `gorm:"column:failure_reason" json:"failureReason,omitempty"`
	CreatedAt          string  `gorm:"column:created_at" json:"createdAt"`
	SentAt             *string `gorm:"column:sent_at" json:"sentAt,omitempty"`
}

func (SendRecord) TableName() string { return "emote_send_records" }

type UpdateRequest struct {
	Name         *string  `json:"name"`
	Meaning      *string  `json:"meaning"`
	Keywords     []string `json:"keywords"`
	Enabled      *bool    `json:"enabled"`
	AIEnabled    *bool    `json:"aiEnabled"`
	RoleScope    *string  `json:"roleScope"`
	CharacterIDs []string `json:"characterIds"`
	GroupIDs     []string `json:"groupIds"`
}

type ImportConfig struct {
	SourceName   string   `json:"sourceName"`
	RelativePath string   `json:"relativePath"`
	Name         string   `json:"name"`
	Meaning      string   `json:"meaning"`
	Keywords     []string `json:"keywords"`
	AIEnabled    bool     `json:"aiEnabled"`
	RoleScope    string   `json:"roleScope"`
	CharacterIDs []string `json:"characterIds"`
	GroupIDs     []string `json:"groupIds"`
	FolderGroup  string   `json:"folderGroup"`
}

type ImportResult struct {
	SourceName       string `json:"sourceName"`
	Status           string `json:"status"`
	EmoteID          string `json:"emoteId,omitempty"`
	DuplicateEmoteID string `json:"duplicateEmoteId,omitempty"`
	ErrorCode        string `json:"errorCode,omitempty"`
	ErrorMessage     string `json:"errorMessage,omitempty"`
	AIWasDisabled    bool   `json:"aiWasDisabled,omitempty"`
}

type DecisionCandidate struct {
	Emote Emote
	Score float64
}
