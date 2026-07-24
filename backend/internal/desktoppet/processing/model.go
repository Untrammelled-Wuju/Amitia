// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

type ProcessingTask struct {
	ID                         string  `gorm:"column:id;primaryKey" json:"id"`
	GenerationTaskID           string  `gorm:"column:generation_task_id" json:"generationTaskId"`
	ProcessingVersion          int     `gorm:"column:processing_version" json:"processingVersion"`
	Status                     string  `gorm:"column:status" json:"status"`
	CurrentStage               string  `gorm:"column:current_stage" json:"currentStage"`
	Progress                   int     `gorm:"column:progress" json:"progress"`
	OutputWidth                int     `gorm:"column:output_width" json:"outputWidth"`
	OutputHeight               int     `gorm:"column:output_height" json:"outputHeight"`
	TargetCharacterHeightRatio float64 `gorm:"column:target_character_height_ratio" json:"targetCharacterHeightRatio"`
	AnchorMode                 string  `gorm:"column:anchor_mode" json:"anchorMode"`
	BackgroundMode             string  `gorm:"column:background_mode" json:"backgroundMode"`
	OutputFormat               string  `gorm:"column:output_format" json:"outputFormat"`
	DefaultFPS                 int     `gorm:"column:default_fps" json:"defaultFps"`
	ExecutionID                string  `gorm:"column:execution_id" json:"executionId"`
	WorkerID                   string  `gorm:"column:worker_id" json:"workerId"`
	LeaseExpiresAt             string  `gorm:"column:lease_expires_at" json:"leaseExpiresAt"`
	LastHeartbeatAt            string  `gorm:"column:last_heartbeat_at" json:"lastHeartbeatAt"`
	CancelRequestedAt          string  `gorm:"column:cancel_requested_at" json:"cancelRequestedAt"`
	ErrorCode                  string  `gorm:"column:error_code" json:"errorCode"`
	ErrorMessage               string  `gorm:"column:error_message" json:"errorMessage"`
	StartedAt                  string  `gorm:"column:started_at" json:"startedAt"`
	CompletedAt                string  `gorm:"column:completed_at" json:"completedAt"`
	CreatedAt                  string  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt                  string  `gorm:"column:updated_at" json:"updatedAt"`
}

func (ProcessingTask) TableName() string { return "desktop_pet_processing_tasks" }

type ProcessingAction struct {
	ID                     string  `gorm:"column:id;primaryKey" json:"id"`
	ProcessingTaskID       string  `gorm:"column:processing_task_id" json:"processingTaskId"`
	GenerationTaskActionID string  `gorm:"column:generation_task_action_id" json:"generationTaskActionId"`
	ActionKey              string  `gorm:"column:action_key" json:"actionKey"`
	ActionNameSnapshot     string  `gorm:"column:action_name_snapshot" json:"actionNameSnapshot"`
	SourceAttemptNumber    int     `gorm:"column:source_attempt_number" json:"sourceAttemptNumber"`
	Status                 string  `gorm:"column:status" json:"status"`
	Progress               int     `gorm:"column:progress" json:"progress"`
	SourceFrameCount       int     `gorm:"column:source_frame_count" json:"sourceFrameCount"`
	ProcessedFrameCount    int     `gorm:"column:processed_frame_count" json:"processedFrameCount"`
	LoopType               string  `gorm:"column:loop_type" json:"loopType"`
	FPS                    int     `gorm:"column:fps" json:"fps"`
	FrameDurationMS        int     `gorm:"column:frame_duration_ms" json:"frameDurationMs"`
	AnchorType             string  `gorm:"column:anchor_type" json:"anchorType"`
	AnchorX                float64 `gorm:"column:anchor_x" json:"anchorX"`
	AnchorY                float64 `gorm:"column:anchor_y" json:"anchorY"`
	BoundingBox            string  `gorm:"column:bounding_box" json:"boundingBox"`
	Excluded               int     `gorm:"column:excluded" json:"excluded"`
	ErrorCode              string  `gorm:"column:error_code" json:"errorCode"`
	ErrorMessage           string  `gorm:"column:error_message" json:"errorMessage"`
	CreatedAt              string  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt              string  `gorm:"column:updated_at" json:"updatedAt"`
	StartedAt              string  `gorm:"column:started_at" json:"startedAt"`
	CompletedAt            string  `gorm:"column:completed_at" json:"completedAt"`
}

func (ProcessingAction) TableName() string { return "desktop_pet_processing_actions" }

type ProcessedFrame struct {
	ID                 string  `gorm:"column:id;primaryKey" json:"id"`
	ProcessingActionID string  `gorm:"column:processing_action_id" json:"processingActionId"`
	SourceFrameID      string  `gorm:"column:source_frame_id" json:"sourceFrameId"`
	FrameIndex         int     `gorm:"column:frame_index" json:"frameIndex"`
	Status             string  `gorm:"column:status" json:"status"`
	SourcePath         string  `gorm:"column:source_path" json:"sourcePath"`
	ProcessedPath      string  `gorm:"column:processed_path" json:"processedPath"`
	Width              int     `gorm:"column:width" json:"width"`
	Height             int     `gorm:"column:height" json:"height"`
	ContentHash        string  `gorm:"column:content_hash" json:"contentHash"`
	SubjectBox         string  `gorm:"column:subject_box" json:"subjectBox"`
	AnchorX            float64 `gorm:"column:anchor_x" json:"anchorX"`
	AnchorY            float64 `gorm:"column:anchor_y" json:"anchorY"`
	AlphaCoverage      float64 `gorm:"column:alpha_coverage" json:"alphaCoverage"`
	QualityFlags       string  `gorm:"column:quality_flags" json:"qualityFlags"`
	ErrorCode          string  `gorm:"column:error_code" json:"errorCode"`
	ErrorMessage       string  `gorm:"column:error_message" json:"errorMessage"`
	CreatedAt          string  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt          string  `gorm:"column:updated_at" json:"updatedAt"`
}

func (ProcessedFrame) TableName() string { return "desktop_pet_processed_frames" }

type Package struct {
	ID               string `gorm:"column:id;primaryKey" json:"id"`
	UserID           string `gorm:"column:user_id" json:"userId"`
	CharacterID      string `gorm:"column:character_id" json:"characterId"`
	GenerationTaskID string `gorm:"column:generation_task_id" json:"generationTaskId"`
	ProcessingTaskID string `gorm:"column:processing_task_id" json:"processingTaskId"`
	Name             string `gorm:"column:name" json:"name"`
	Version          int    `gorm:"column:version" json:"version"`
	Status           string `gorm:"column:status" json:"status"`
	DefaultActionKey string `gorm:"column:default_action_key" json:"defaultActionKey"`
	CanvasWidth      int    `gorm:"column:canvas_width" json:"canvasWidth"`
	CanvasHeight     int    `gorm:"column:canvas_height" json:"canvasHeight"`
	PackagePath      string `gorm:"column:package_path" json:"packagePath"`
	ManifestPath     string `gorm:"column:manifest_path" json:"manifestPath"`
	PreviewPath      string `gorm:"column:preview_path" json:"previewPath"`
	ActionCount      int    `gorm:"column:action_count" json:"actionCount"`
	PackageHash      string `gorm:"column:package_hash" json:"packageHash"`
	IncludedActions  string `gorm:"column:included_actions" json:"includedActions"`
	CreatedAt        string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt        string `gorm:"column:updated_at" json:"updatedAt"`
}

func (Package) TableName() string { return "desktop_pet_packages" }
