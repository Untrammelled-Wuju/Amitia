package reminder

type Reminder struct {
	ID                int     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID            string  `gorm:"column:user_id;not null;default:default;index" json:"-"`
	Title             string  `gorm:"column:title;not null" json:"title"`
	Content           string  `gorm:"column:content" json:"content"`
	Channel           string  `gorm:"column:channel;default:web" json:"channel"`
	ConversationID    string  `gorm:"column:conversation_id" json:"conversationId"`
	CharacterID       string  `gorm:"column:character_id" json:"characterId"`
	RemindAt          string  `gorm:"column:remind_at;not null" json:"remindAt"`
	RepeatRule        string  `gorm:"column:repeat_rule;default:none" json:"repeatRule"`
	Enabled           int     `gorm:"column:enabled;default:1" json:"enabled"`
	LastTriggeredAt   *string `gorm:"column:last_triggered_at" json:"lastTriggeredAt"`
	CreatedAt         string  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt         string  `gorm:"column:updated_at" json:"updatedAt"`
	ConversationTitle string  `gorm:"-" json:"conversationTitle"`
	CharacterName     string  `gorm:"-" json:"characterName"`
}

func (Reminder) TableName() string { return "reminders" }

type TriggerHistory struct {
	ID           string `gorm:"column:id;primaryKey" json:"id"`
	UserID       string `gorm:"column:user_id;not null;default:default;index" json:"-"`
	TriggerID    string `gorm:"column:trigger_id" json:"triggerId"`
	TriggerType  string `gorm:"column:trigger_type" json:"triggerType"`
	Title        string `gorm:"column:title" json:"title"`
	Channel      string `gorm:"column:channel" json:"channel"`
	State        string `gorm:"column:state;default:pending" json:"state"`
	Priority     string `gorm:"column:priority;default:normal" json:"priority"`
	Reason       string `gorm:"column:reason" json:"reason"`
	AttemptCount int    `gorm:"column:attempt_count;default:0" json:"attemptCount"`
	LastError    string `gorm:"column:last_error" json:"lastError"`
	CreatedAt    string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt    string `gorm:"column:updated_at" json:"updatedAt"`
}

func (TriggerHistory) TableName() string { return "trigger_histories" }

type CreateReminderRequest struct {
	Title          string `json:"title" binding:"required"`
	Content        string `json:"content"`
	Channel        string `json:"channel"`
	ConversationID string `json:"conversationId"`
	CharacterID    string `json:"characterId"`
	RemindAt       string `json:"remindAt" binding:"required"`
	RepeatRule     string `json:"repeatRule"`
	Enabled        *bool  `json:"enabled"`
}
