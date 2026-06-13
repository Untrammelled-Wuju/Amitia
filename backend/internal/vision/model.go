package vision

type VisionConfig struct {
	ID          int    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name        string `gorm:"column:name;not null" json:"name"`
	ApiKey      string `gorm:"column:api_key" json:"apiKey"`
	ModelName   string `gorm:"column:model_name;default:doubao-vision-pro-32k" json:"modelName"`
	BaseUrl     string `gorm:"column:base_url;default:https://ark.cn-beijing.volces.com/api/v3" json:"baseUrl"`
	IsActive    int    `gorm:"column:is_active;default:0" json:"isActive"`
	HasApiKey   bool   `gorm:"-" json:"hasApiKey"`
	CreatedAt   string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   string `gorm:"column:updated_at" json:"updatedAt"`
}

func (VisionConfig) TableName() string { return "vision_configs" }

type CreateVisionConfigRequest struct {
	Name      string `json:"name"`
	ApiKey    string `json:"apiKey"`
	ModelName string `json:"modelName"`
	BaseUrl   string `json:"baseUrl"`
	IsActive  int    `json:"isActive"`
}

type VisionTestResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Latency int64  `json:"latency"`
}

type VisionPreset struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	ModelName   string `json:"modelName"`
	BaseUrl     string `json:"baseUrl"`
	Description string `json:"description"`
}
