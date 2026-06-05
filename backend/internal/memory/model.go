package memory



type Memory struct {
	ID           string     `gorm:"column:id;primaryKey" json:"id"`
	CharacterID  string     `gorm:"column:character_id" json:"characterId"`
	MemoryType   string     `gorm:"column:memory_type;default:custom" json:"memoryType"`
	Source       string     `gorm:"column:source;default:manual" json:"source"`
	Key          string     `gorm:"column:key;not null" json:"key"`
	Value        string     `gorm:"column:value;not null" json:"value"`
	Importance   int        `gorm:"column:importance;default:0" json:"importance"`
	UseCount     int        `gorm:"column:use_count;default:0" json:"useCount"`
	LastUsedAt   *string `gorm:"column:last_used_at" json:"lastUsedAt"`
	CreatedAt    string  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt    string  `gorm:"column:updated_at" json:"updatedAt"`
}

func (Memory) TableName() string { return "memories" }



type CreateMemoryRequest struct {
	CharacterID string `json:"characterId"`
	MemoryType  string `json:"memoryType"`
	Key         string `json:"key" binding:"required"`
	Value       string `json:"value" binding:"required"`
	Importance  int    `json:"importance"`
	Source      string `json:"source"`
}

type UpdateMemoryRequest struct {
	Key         *string `json:"key"`
	Value       *string `json:"value"`
	MemoryType  *string `json:"memoryType"`
	CharacterID *string `json:"characterId"`
	Importance  *int    `json:"importance"`
}

type SearchMemoryRequest struct {
	Keyword     string `json:"keyword" binding:"required"`
	CharacterID string `json:"characterId"`
	Limit       int    `json:"limit"`
}

type MemoryListQuery struct {
	Page        int    `form:"page"`
	PageSize    int    `form:"pageSize"`
	CharacterID string `form:"characterId"`
	MemoryType  string `form:"memoryType"`
	Keyword     string `form:"keyword"`
	SortBy      string `form:"sortBy"`
}

type MemoryListResponse struct {
	Items    []Memory `json:"items"`
	Total    int64    `json:"total"`
	Page     int      `json:"page"`
	PageSize int      `json:"pageSize"`
}
