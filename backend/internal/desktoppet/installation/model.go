// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package installation

type Installation struct {
	ID               string `gorm:"column:id;primaryKey;type:text" json:"id"`
	UserID           string `gorm:"column:user_id;type:text" json:"userId"`
	CharacterID      string `gorm:"column:character_id;type:text" json:"characterId"`
	PackageID        string `gorm:"column:package_id;type:text" json:"packageId"`
	PackageVersion   string `gorm:"column:package_version;type:text" json:"packageVersion"`
	Name             string `gorm:"column:name;type:text" json:"name"`
	Status           string `gorm:"column:status;type:text" json:"status"`
	IsActive         int    `gorm:"column:is_active;type:integer" json:"isActive"`
	InstallPath      string `gorm:"column:install_path;type:text" json:"installPath"`
	ManifestPath     string `gorm:"column:manifest_path;type:text" json:"manifestPath"`
	PreviewPath      string `gorm:"column:preview_path;type:text" json:"previewPath"`
	DefaultActionKey string `gorm:"column:default_action_key;type:text" json:"defaultActionKey"`
	CanvasWidth      int    `gorm:"column:canvas_width;type:integer" json:"canvasWidth"`
	CanvasHeight     int    `gorm:"column:canvas_height;type:integer" json:"canvasHeight"`
	PackageHash      string `gorm:"column:package_hash;type:text" json:"packageHash"`
	InstalledAt      string `gorm:"column:installed_at;type:text" json:"installedAt"`
	LastEnabledAt    string `gorm:"column:last_enabled_at;type:text" json:"lastEnabledAt"`
	LastDisabledAt   string `gorm:"column:last_disabled_at;type:text" json:"lastDisabledAt"`
	CreatedAt        string `gorm:"column:created_at;type:text" json:"createdAt"`
	UpdatedAt        string `gorm:"column:updated_at;type:text" json:"updatedAt"`
	PetID            string `gorm:"column:pet_id;type:text;default:''" json:"petId"`
	CurrentReleaseID string `gorm:"column:current_release_id;type:text;default:''" json:"currentReleaseId"`
	LifecycleState   string `gorm:"column:lifecycle_state;type:text;default:'installed'" json:"lifecycleState"`
	DesiredState     string `gorm:"column:desired_state;type:text;default:'disabled'" json:"desiredState"`
	RuntimeSyncState string `gorm:"column:runtime_sync_state;type:text;default:'pending'" json:"runtimeSyncState"`
	StateRevision    int    `gorm:"column:state_revision;type:integer;default:0" json:"stateRevision"`
	InstallStorageKey string `gorm:"column:install_storage_key;type:text;default:''" json:"installStorageKey"`
	IntegrityRoot    string `gorm:"column:integrity_root;type:text;default:''" json:"integrityRoot"`
	LastErrorCode    string `gorm:"column:last_error_code;type:text;default:''" json:"lastErrorCode"`
	LastErrorMessage string `gorm:"column:last_error_message;type:text;default:''" json:"lastErrorMessage"`
}

func (Installation) TableName() string { return "desktop_pet_installations" }

type RuntimeSettings struct {
	ID                     string  `gorm:"column:id;primaryKey;type:text" json:"id"`
	InstallationID         string  `gorm:"column:installation_id;type:text" json:"installationId"`
	AlwaysOnTop            int     `gorm:"column:always_on_top;type:integer" json:"alwaysOnTop"`
	LaunchOnStartup        int     `gorm:"column:launch_on_startup;type:integer" json:"launchOnStartup"`
	Scale                  float64 `gorm:"column:scale;type:real" json:"scale"`
	PositionX              int     `gorm:"column:position_x;type:integer" json:"positionX"`
	PositionY              int     `gorm:"column:position_y;type:integer" json:"positionY"`
	ScreenID               string  `gorm:"column:screen_id;type:text" json:"screenId"`
	IdleEnabled            int     `gorm:"column:idle_enabled;type:integer" json:"idleEnabled"`
	IdleIntervalMinSeconds int     `gorm:"column:idle_interval_min_seconds;type:integer" json:"idleIntervalMinSeconds"`
	IdleIntervalMaxSeconds int     `gorm:"column:idle_interval_max_seconds;type:integer" json:"idleIntervalMaxSeconds"`
	ClickThroughMode       string  `gorm:"column:click_through_mode;type:text" json:"clickThroughMode"`
	SoundEnabled           int     `gorm:"column:sound_enabled;type:integer" json:"soundEnabled"`
	SettingsRevision       int     `gorm:"column:settings_revision;type:integer" json:"settingsRevision"`
	RestoreOnAppStart      int     `gorm:"column:restore_on_app_start;type:integer" json:"restoreOnAppStart"`
	PositionMode           string  `gorm:"column:position_mode;type:text" json:"positionMode"`
	DisplayFingerprint     string  `gorm:"column:display_fingerprint;type:text" json:"displayFingerprint"`
	RelativeX              float64 `gorm:"column:relative_x;type:real" json:"relativeX"`
	RelativeY              float64 `gorm:"column:relative_y;type:real" json:"relativeY"`
	LastWindowWidth        int     `gorm:"column:last_window_width;type:integer" json:"lastWindowWidth"`
	LastWindowHeight       int     `gorm:"column:last_window_height;type:integer" json:"lastWindowHeight"`
	PositionUpdatedAt      string  `gorm:"column:position_updated_at;type:text" json:"positionUpdatedAt"`
	CreatedAt              string  `gorm:"column:created_at;type:text" json:"createdAt"`
	UpdatedAt              string  `gorm:"column:updated_at;type:text" json:"updatedAt"`
}

func (RuntimeSettings) TableName() string { return "desktop_pet_runtime_settings" }

const (
	StatusInstalling   = "installing"
	StatusInstalled    = "installed"
	StatusEnabled      = "enabled"
	StatusDisabled     = "disabled"
	StatusInvalid      = "invalid"
	StatusUninstalling = "uninstalling"
	StatusUninstalled  = "uninstalled"

	LifecyclePreparing    = "preparing"
	LifecycleStaging      = "staging"
	LifecycleVerifying    = "verifying"
	LifecycleInstalled    = "installed"
	LifecycleUpgrading    = "upgrading"
	LifecycleUninstalling = "uninstalling"
	LifecycleUninstalled  = "uninstalled"
	LifecycleInvalid      = "invalid"
	LifecycleRecoveryReq  = "recovery_required"

	DesiredEnabled  = "enabled"
	DesiredDisabled = "disabled"

	SyncPending   = "pending"
	SyncConfirmed = "confirmed"
	SyncFailed    = "failed"
	SyncOffline   = "offline"
)

func (i *Installation) IsActivated() bool {
	return i != nil && i.IsActive == 1
}

func (i *Installation) IsEnabled() bool {
	if i == nil {
		return false
	}
	return i.Status == StatusEnabled || i.DesiredState == DesiredEnabled
}

func (i *Installation) IsInstalled() bool {
	if i == nil {
		return false
	}
	return i.Status == StatusInstalled || i.LifecycleState == LifecycleInstalled
}

func (i *Installation) IsUninstalled() bool {
	if i == nil {
		return false
	}
	return i.Status == StatusUninstalled || i.LifecycleState == LifecycleUninstalled
}

func (i *Installation) CanEnable() bool {
	if i == nil {
		return false
	}
	if i.LifecycleState == LifecycleUninstalled || i.LifecycleState == LifecycleUninstalling || i.LifecycleState == LifecycleInvalid {
		return false
	}
	if i.LifecycleState != "" {
		return i.LifecycleState == LifecycleInstalled || i.DesiredState == DesiredDisabled
	}
	return i.Status == StatusInstalled || i.Status == StatusDisabled
}

func (i *Installation) CanUninstall() bool {
	if i == nil {
		return false
	}
	switch i.Status {
	case StatusInstalled, StatusEnabled, StatusDisabled:
		return true
	default:
		return false
	}
}

func (i *Installation) HasReleaseBinding() bool {
	return i != nil && i.CurrentReleaseID != ""
}
