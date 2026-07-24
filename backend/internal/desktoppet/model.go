// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package desktoppet

type ActionDefinition struct {
	ID                       int    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ActionKey                string `gorm:"column:action_key;uniqueIndex" json:"actionKey"`
	Name                     string `gorm:"column:name" json:"name"`
	Description              string `gorm:"column:description" json:"description"`
	CategoryKey              string `gorm:"column:category_key" json:"categoryKey"`
	CategoryName             string `gorm:"column:category_name" json:"categoryName"`
	SupportsDefaultIdle      int    `gorm:"column:supports_default_idle" json:"supportsDefaultIdle"`
	Recommended              int    `gorm:"column:recommended" json:"recommended"`
	Enabled                  int    `gorm:"column:enabled" json:"enabled"`
	SortOrder                int    `gorm:"column:sort_order" json:"sortOrder"`
	DefinitionVersion        int    `gorm:"column:definition_version" json:"definitionVersion"`
	DefaultFrameCount        int    `gorm:"column:default_frame_count" json:"defaultFrameCount"`
	EstimatedGenerationCount int    `gorm:"column:estimated_generation_count" json:"estimatedGenerationCount"`
	CreatedAt                string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt                string `gorm:"column:updated_at" json:"updatedAt"`
}

func (ActionDefinition) TableName() string { return "desktop_pet_action_definitions" }

type GenerationTask struct {
	ID                       string `gorm:"column:id;primaryKey" json:"id"`
	UserID                   string `gorm:"column:user_id" json:"userId"`
	CharacterID              string `gorm:"column:character_id" json:"characterId"`
	ModelConfigID            int    `gorm:"column:model_config_id" json:"modelConfigId"`
	Name                     string `gorm:"column:name" json:"name"`
	SourceImagePath          string `gorm:"column:source_image_path" json:"sourceImagePath"`
	SourceImageOriginalName  string `gorm:"column:source_image_original_name" json:"sourceImageOriginalName"`
	SourceImageMimeType      string `gorm:"column:source_image_mime_type" json:"sourceImageMimeType"`
	SourceImageSize          int    `gorm:"column:source_image_size" json:"sourceImageSize"`
	SourceImageWidth         int    `gorm:"column:source_image_width" json:"sourceImageWidth"`
	SourceImageHeight        int    `gorm:"column:source_image_height" json:"sourceImageHeight"`
	SourceImageHash          string `gorm:"column:source_image_hash" json:"sourceImageHash"`
	Prompt                   string `gorm:"column:prompt" json:"prompt"`
	NegativePrompt           string `gorm:"column:negative_prompt" json:"negativePrompt"`
	OutputWidth              int    `gorm:"column:output_width" json:"outputWidth"`
	OutputHeight             int    `gorm:"column:output_height" json:"outputHeight"`
	Status                   string `gorm:"column:status" json:"status"`
	CurrentStage             string `gorm:"column:current_stage" json:"currentStage"`
	Progress                 int    `gorm:"column:progress" json:"progress"`
	SelectedActionCount      int    `gorm:"column:selected_action_count" json:"selectedActionCount"`
	EstimatedGenerationCount int    `gorm:"column:estimated_generation_count" json:"estimatedGenerationCount"`
	ErrorCode                string `gorm:"column:error_code" json:"errorCode"`
	ErrorMessage             string `gorm:"column:error_message" json:"errorMessage"`
	CreatedAt                string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt                string `gorm:"column:updated_at" json:"updatedAt"`
	StartedAt                string `gorm:"column:started_at" json:"startedAt"`
	CompletedAt              string `gorm:"column:completed_at" json:"completedAt"`
}

func (GenerationTask) TableName() string { return "desktop_pet_generation_tasks" }

type GenerationTaskAction struct {
	ID                       string `gorm:"column:id;primaryKey" json:"id"`
	TaskID                   string `gorm:"column:task_id" json:"taskId"`
	ActionDefinitionID       int    `gorm:"column:action_definition_id" json:"actionDefinitionId"`
	ActionKey                string `gorm:"column:action_key" json:"actionKey"`
	ActionNameSnapshot       string `gorm:"column:action_name_snapshot" json:"actionNameSnapshot"`
	ActionDescriptionSnapshot string `gorm:"column:action_description_snapshot" json:"actionDescriptionSnapshot"`
	CategoryKeySnapshot      string `gorm:"column:category_key_snapshot" json:"categoryKeySnapshot"`
	CategoryNameSnapshot     string `gorm:"column:category_name_snapshot" json:"categoryNameSnapshot"`
	DefinitionVersion        int    `gorm:"column:definition_version" json:"definitionVersion"`
	SupportsDefaultIdle      int    `gorm:"column:supports_default_idle" json:"supportsDefaultIdle"`
	SortOrder                int    `gorm:"column:sort_order" json:"sortOrder"`
	FrameCount               int    `gorm:"column:frame_count" json:"frameCount"`
	EstimatedGenerationCount int    `gorm:"column:estimated_generation_count" json:"estimatedGenerationCount"`
	Status                   string `gorm:"column:status" json:"status"`
	Progress                 int    `gorm:"column:progress" json:"progress"`
	ErrorCode                string `gorm:"column:error_code" json:"errorCode"`
	ErrorMessage             string `gorm:"column:error_message" json:"errorMessage"`
	CreatedAt                string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt                string `gorm:"column:updated_at" json:"updatedAt"`
	StartedAt                string `gorm:"column:started_at" json:"startedAt"`
	CompletedAt              string `gorm:"column:completed_at" json:"completedAt"`
}

func (GenerationTaskAction) TableName() string { return "desktop_pet_generation_task_actions" }

type ActionItemResponse struct {
	ID                       int    `json:"id"`
	Key                      string `json:"key"`
	Name                     string `json:"name"`
	Description              string `json:"description"`
	SupportsDefaultIdle      bool   `json:"supportsDefaultIdle"`
	Recommended              bool   `json:"recommended"`
	DefaultFrameCount        int    `json:"defaultFrameCount"`
	EstimatedGenerationCount int    `json:"estimatedGenerationCount"`
	DefinitionVersion        int    `json:"definitionVersion"`
}

type ActionCategoryResponse struct {
	Key       string              `json:"key"`
	Name      string              `json:"name"`
	SortOrder int                 `json:"sortOrder"`
	Actions   []ActionItemResponse `json:"actions"`
}

type PresetResponse struct {
	Key        string   `json:"key"`
	Name       string   `json:"name"`
	Description string  `json:"description"`
	ActionKeys []string `json:"actionKeys"`
}

type ActionDefinitionsResponse struct {
	Categories []ActionCategoryResponse `json:"categories"`
	Presets    []PresetResponse         `json:"presets"`
}

type TaskSummaryResponse struct {
	ID                       string `json:"id"`
	Name                     string `json:"name"`
	CharacterID              string `json:"characterId"`
	ModelConfigID            int    `json:"modelConfigId"`
	Status                   string `json:"status"`
	CurrentStage             string `json:"currentStage"`
	Progress                 int    `json:"progress"`
	SelectedActionCount      int    `json:"selectedActionCount"`
	EstimatedGenerationCount int    `json:"estimatedGenerationCount"`
	CreatedAt                string `json:"createdAt"`
}

type TaskActionResponse struct {
	ID                       string `json:"id"`
	ActionKey                string `json:"actionKey"`
	ActionName               string `json:"actionName"`
	ActionDescription        string `json:"actionDescription"`
	CategoryKey              string `json:"categoryKey"`
	CategoryName             string `json:"categoryName"`
	DefinitionVersion        int    `json:"definitionVersion"`
	SupportsDefaultIdle      bool   `json:"supportsDefaultIdle"`
	SortOrder                int    `json:"sortOrder"`
	FrameCount               int    `json:"frameCount"`
	EstimatedGenerationCount int    `json:"estimatedGenerationCount"`
	Status                   string `json:"status"`
	Progress                 int    `json:"progress"`
	ErrorCode                string `json:"errorCode"`
	ErrorMessage             string `json:"errorMessage"`
}

type TaskDetailResponse struct {
	ID                       string              `json:"id"`
	Name                     string              `json:"name"`
	CharacterID              string              `json:"characterId"`
	CharacterName            string              `json:"characterName"`
	ModelConfigID            int                 `json:"modelConfigId"`
	ModelName                string              `json:"modelName"`
	Status                   string              `json:"status"`
	CurrentStage             string              `json:"currentStage"`
	Progress                 int                 `json:"progress"`
	SelectedActionCount      int                 `json:"selectedActionCount"`
	EstimatedGenerationCount int                 `json:"estimatedGenerationCount"`
	ReferenceImageUrl        string              `json:"referenceImageUrl"`
	ErrorMessage             string              `json:"errorMessage"`
	CreatedAt                string              `json:"createdAt"`
	UpdatedAt                string              `json:"updatedAt"`
	StartedAt                string              `json:"startedAt"`
	CompletedAt              string              `json:"completedAt"`
	Actions                  []TaskActionResponse `json:"actions"`
}

type TaskListItemResponse struct {
	ID                       string `json:"id"`
	Name                     string `json:"name"`
	CharacterID              string `json:"characterId"`
	CharacterName            string `json:"characterName"`
	ModelConfigID            int    `json:"modelConfigId"`
	ModelName                string `json:"modelName"`
	Status                   string `json:"status"`
	CurrentStage             string `json:"currentStage"`
	Progress                 int    `json:"progress"`
	SelectedActionCount      int    `json:"selectedActionCount"`
	EstimatedGenerationCount int    `json:"estimatedGenerationCount"`
	CreatedAt                string `json:"createdAt"`
}

type TaskListResponse struct {
	Total    int64                  `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"pageSize"`
	Items    []TaskListItemResponse `json:"items"`
}
