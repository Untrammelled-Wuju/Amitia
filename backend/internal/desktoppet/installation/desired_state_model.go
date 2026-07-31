package installation

type RuntimeDesiredState struct {
	ID              string  `gorm:"column:id;primaryKey;type:text" json:"id"`
	InstallationID  string  `gorm:"column:installation_id;type:text" json:"installationId"`
	UserID          string  `gorm:"column:user_id;type:text" json:"userId"`
	DeviceID        string  `gorm:"column:device_id;type:text;default:''" json:"deviceId"`
	DesiredEnabled  int     `gorm:"column:desired_enabled;type:integer;default:0" json:"desiredEnabled"`
	DesiredVisible  int     `gorm:"column:desired_visible;type:integer;default:0" json:"desiredVisible"`
	DesiredReleaseID string `gorm:"column:desired_release_id;type:text;default:''" json:"desiredReleaseId"`
	DesiredActionKey string `gorm:"column:desired_action_key;type:text;default:''" json:"desiredActionKey"`
	PositionX       *float64 `gorm:"column:position_x;type:real" json:"positionX,omitempty"`
	PositionY       *float64 `gorm:"column:position_y;type:real" json:"positionY,omitempty"`
	Scale           float64 `gorm:"column:scale;type:real;default:1.0" json:"scale"`
	Opacity         float64 `gorm:"column:opacity;type:real;default:1.0" json:"opacity"`
	AlwaysOnTop     int     `gorm:"column:always_on_top;type:integer;default:1" json:"alwaysOnTop"`
	ClickThroughMode string `gorm:"column:click_through_mode;type:text;default:'off'" json:"clickThroughMode"`
	PositionPolicy  string  `gorm:"column:position_policy;type:text;default:''" json:"positionPolicy"`
	Revision        int64   `gorm:"column:revision;type:integer;default:0" json:"revision"`
	UpdatedAt       string  `gorm:"column:updated_at;type:text" json:"updatedAt"`
	CreatedAt       string  `gorm:"column:created_at;type:text" json:"createdAt"`
}

func (RuntimeDesiredState) TableName() string { return "desktop_pet_runtime_desired_states" }

type InstallationCommitJournal struct {
	ID              string `gorm:"column:id;primaryKey;type:text" json:"id"`
	OperationID     string `gorm:"column:operation_id;type:text" json:"operationId"`
	InstallationID  string `gorm:"column:installation_id;type:text" json:"installationId"`
	UserID          string `gorm:"column:user_id;type:text" json:"userId"`
	DeviceID        string `gorm:"column:device_id;type:text;default:''" json:"deviceId"`
	OperationType   string `gorm:"column:operation_type;type:text" json:"operationType"`
	ReleaseID       string `gorm:"column:release_id;type:text" json:"releaseId"`
	TargetReleaseID string `gorm:"column:target_release_id;type:text;default:''" json:"targetReleaseId"`
	State           string `gorm:"column:state;type:text;default:'operation_created'" json:"state"`
	StagingPathKey  string `gorm:"column:staging_path_key;type:text;default:''" json:"stagingPathKey"`
	PublishedPathKey string `gorm:"column:published_path_key;type:text;default:''" json:"publishedPathKey"`
	TrashPathKey    string `gorm:"column:trash_path_key;type:text;default:''" json:"trashPathKey"`
	PreviousReleaseID string `gorm:"column:previous_release_id;type:text;default:''" json:"previousReleaseId"`
	RollbackReason  string `gorm:"column:rollback_reason;type:text;default:''" json:"rollbackReason"`
	ErrorCode       string `gorm:"column:error_code;type:text;default:''" json:"errorCode"`
	ErrorMessage    string `gorm:"column:error_message;type:text;default:''" json:"errorMessage"`
	CreatedAt       string `gorm:"column:created_at;type:text" json:"createdAt"`
	UpdatedAt       string `gorm:"column:updated_at;type:text" json:"updatedAt"`
}

func (InstallationCommitJournal) TableName() string { return "desktop_pet_installation_commit_journals" }

type InstallationSwitchJournal struct {
	ID                 string `gorm:"column:id;primaryKey;type:text" json:"id"`
	OperationID        string `gorm:"column:operation_id;type:text" json:"operationId"`
	UserID             string `gorm:"column:user_id;type:text" json:"userId"`
	DeviceID           string `gorm:"column:device_id;type:text;default:''" json:"deviceId"`
	OldInstallationID  string `gorm:"column:old_installation_id;type:text;default:''" json:"oldInstallationId"`
	NewInstallationID  string `gorm:"column:new_installation_id;type:text" json:"newInstallationId"`
	OldDesiredRevision int64  `gorm:"column:old_desired_revision;type:integer;default:0" json:"oldDesiredRevision"`
	NewDesiredRevision int64  `gorm:"column:new_desired_revision;type:integer;default:0" json:"newDesiredRevision"`
	BindingRevision    int    `gorm:"column:binding_revision;type:integer;default:0" json:"bindingRevision"`
	State              string `gorm:"column:state;type:text;default:'pending'" json:"state"`
	CreatedAt          string `gorm:"column:created_at;type:text" json:"createdAt"`
	UpdatedAt          string `gorm:"column:updated_at;type:text" json:"updatedAt"`
}

func (InstallationSwitchJournal) TableName() string { return "desktop_pet_installation_switch_journals" }

type LegacyInstallationMapping struct {
	ID                   string `gorm:"column:id;primaryKey;type:text" json:"id"`
	LegacyInstallationID string `gorm:"column:legacy_installation_id;type:text" json:"legacyInstallationId"`
	NewInstallationID    string `gorm:"column:new_installation_id;type:text" json:"newInstallationId"`
	UserID               string `gorm:"column:user_id;type:text" json:"userId"`
	LegacyPackageID      string `gorm:"column:legacy_package_id;type:text" json:"legacyPackageId"`
	PetID                string `gorm:"column:pet_id;type:text;default:''" json:"petId"`
	ReleaseID            string `gorm:"column:release_id;type:text;default:''" json:"releaseId"`
	MigrationStatus      string `gorm:"column:migration_status;type:text;default:'pending'" json:"migrationStatus"`
	SourceContentHash     string `gorm:"column:source_content_hash;type:text;default:''" json:"sourceContentHash"`
	ErrorCode            string `gorm:"column:error_code;type:text;default:''" json:"errorCode"`
	ErrorMessage         string `gorm:"column:error_message;type:text;default:''" json:"errorMessage"`
	CreatedAt            string `gorm:"column:created_at;type:text" json:"createdAt"`
	UpdatedAt            string `gorm:"column:updated_at;type:text" json:"updatedAt"`
}

func (LegacyInstallationMapping) TableName() string { return "desktop_pet_legacy_installation_mappings" }
