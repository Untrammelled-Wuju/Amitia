package activebinding

type ActiveBinding struct {
	GenerationActionID      string `gorm:"column:generation_action_id;primaryKey;type:text" json:"generationActionId"`
	ActiveAttemptID         string `gorm:"column:active_attempt_id;type:text" json:"activeAttemptId"`
	ActivePrimaryArtifactID string `gorm:"column:active_primary_artifact_id;type:text" json:"activePrimaryArtifactId"`
	ArtifactContentHash     string `gorm:"column:artifact_content_hash;type:text" json:"artifactContentHash"`
	BindingRevision         int    `gorm:"column:binding_revision;type:integer" json:"bindingRevision"`
	BoundAt                 string `gorm:"column:bound_at;type:text" json:"boundAt"`
	BoundReason             string `gorm:"column:bound_reason;type:text" json:"boundReason"`
	CreatedAt               string `gorm:"column:created_at;type:text" json:"createdAt"`
	UpdatedAt               string `gorm:"column:updated_at;type:text" json:"updatedAt"`
}

func (ActiveBinding) TableName() string { return "desktop_pet_generation_action_active_bindings" }
