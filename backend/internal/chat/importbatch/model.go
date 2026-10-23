package importbatch

type Batch struct {
	ID                        string `gorm:"column:id;primaryKey;type:text" json:"id"`
	SpaceID                   string `gorm:"column:space_id;type:text;not null" json:"spaceId"`
	DeviceID                  string `gorm:"column:device_id;type:text;not null;default:''" json:"deviceId"`
	RoleID                    string `gorm:"column:role_id;type:text;not null;default:''" json:"roleId"`
	RoleRevision              int64  `gorm:"column:role_revision;type:integer;not null;default:0" json:"roleRevision"`
	Title                     string `gorm:"column:title;type:text;not null;default:''" json:"title"`
	Status                    string `gorm:"column:status;type:text;not null;default:'ready'" json:"status"`
	DetectedFormat            string `gorm:"column:detected_format;type:text;not null;default:''" json:"detectedFormat"`
	MessageCount              int    `gorm:"column:message_count;type:integer;not null;default:0" json:"messageCount"`
	ContentHash               string `gorm:"column:content_hash;type:text;not null;default:''" json:"contentHash"`
	IdempotencyKey            string `gorm:"column:idempotency_key;type:text;not null;default:''" json:"idempotencyKey"`
	SummaryText               string `gorm:"column:summary_text;type:text;not null;default:''" json:"summary"`
	PromotedConversationID    string `gorm:"column:promoted_conversation_id;type:text;not null;default:''" json:"promotedConversationId"`
	PromotedConversationOwner string `gorm:"column:promoted_conversation_owner_id;type:text;not null;default:''" json:"promotedConversationOwnerId"`
	PromotedCoreID            string `gorm:"column:promoted_core_id;type:text;not null;default:''" json:"promotedCoreId"`
	LegacyConversationID      string `gorm:"column:legacy_conversation_id;type:text;not null;default:''" json:"legacyConversationId"`
	ErrorCode                 string `gorm:"column:error_code;type:text;not null;default:''" json:"errorCode"`
	ErrorMessage              string `gorm:"column:error_message;type:text;not null;default:''" json:"errorMessage"`
	CreatedAt                 string `gorm:"column:created_at;type:text;not null;default:''" json:"createdAt"`
	UpdatedAt                 string `gorm:"column:updated_at;type:text;not null;default:''" json:"updatedAt"`
	PromotedAt                string `gorm:"column:promoted_at;type:text;not null;default:''" json:"promotedAt"`
}

func (Batch) TableName() string {
	return "chat_import_batches"
}

type Item struct {
	ID         string  `gorm:"column:id;primaryKey;type:text" json:"id"`
	BatchID    string  `gorm:"column:batch_id;type:text;not null;index" json:"batchId"`
	Sequence   int64   `gorm:"column:sequence;type:integer;not null;default:0" json:"sequence"`
	Speaker    string  `gorm:"column:speaker;type:text;not null;default:''" json:"speaker"`
	Role       string  `gorm:"column:role;type:text;not null;default:'user'" json:"role"`
	Content    string  `gorm:"column:content;type:text;not null" json:"content"`
	Confidence float64 `gorm:"column:confidence;type:real;not null;default:0" json:"confidence"`
	Timestamp  string  `gorm:"column:timestamp;type:text;not null;default:''" json:"timestamp"`
	LineNo     int     `gorm:"column:line_no;type:integer;not null;default:0" json:"lineNo"`
	CreatedAt  string  `gorm:"column:created_at;type:text;not null;default:''" json:"createdAt"`
}

func (Item) TableName() string {
	return "chat_import_items"
}

const (
	StatusReady    = "ready_for_promotion"
	StatusPromoted = "promoted"
	StatusFailed   = "failed"
)
