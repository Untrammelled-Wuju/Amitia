package release

const (
	EventReleaseBuildStarted       = "desktop_pet.release.build_started"
	EventReleaseBuildCompleted     = "desktop_pet.release.build_completed"
	EventReleaseBuildFailed        = "desktop_pet.release.build_failed"
	EventReleaseArchived           = "desktop_pet.release.archived"
	EventReleaseRevoked            = "desktop_pet.release.revoked"
	EventLegacyPackageMigrated     = "desktop_pet.legacy_package.migrated"
	EventLegacyPackageMigrationFailed = "desktop_pet.legacy_package.migration_failed"
)

type ReleaseEvent struct {
	EventType             string `json:"eventType"`
	UserID                string `json:"userId"`
	PetID                 string `json:"petId"`
	ReleaseID             string `json:"releaseId"`
	ProcessingTaskID      string `json:"processingTaskId"`
	ActiveRevisionSetHash string `json:"activeRevisionSetHash"`
	QualityGateID         string `json:"qualityGateId"`
	ContentRootHash       string `json:"contentRootHash"`
	OccurredAt            string `json:"occurredAt"`
}

type EventPublisher interface {
	PublishReleaseEvent(event ReleaseEvent) error
}
