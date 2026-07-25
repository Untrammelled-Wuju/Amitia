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
)

func (i *Installation) IsActivated() bool {
	return i != nil && i.IsActive == 1
}

func (i *Installation) IsEnabled() bool {
	return i != nil && i.Status == StatusEnabled
}

func (i *Installation) IsInstalled() bool {
	return i != nil && i.Status == StatusInstalled
}

func (i *Installation) IsUninstalled() bool {
	return i != nil && i.Status == StatusUninstalled
}

func (i *Installation) CanEnable() bool {
	if i == nil {
		return false
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
