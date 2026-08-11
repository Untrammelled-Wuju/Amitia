package desktop_pet_center

import (
	"time"
)

type PluginInstallState string

const (
	PluginInstallStateInstalled PluginInstallState = "installed"
	PluginInstallStateInstalling PluginInstallState = "installing"
	PluginInstallStateFailed     PluginInstallState = "failed"
	PluginInstallStateUninstalling PluginInstallState = "uninstalling"
)

type PluginEnablementState string

const (
	PluginEnablementStateEnabled  PluginEnablementState = "enabled"
	PluginEnablementStateDisabled PluginEnablementState = "disabled"
)

type PermissionSummary struct {
	Declared []string `json:"declared,omitempty"`
	Granted  []string `json:"granted,omitempty"`
}

type DesktopPetPluginSummary struct {
	ExtensionID      string               `json:"extensionId"`
	PluginID         string               `json:"pluginId"`
	Name             string               `json:"name"`
	Description      string               `json:"description"`
	Version          string               `json:"version"`
	Enabled          bool                 `json:"enabled"`
	InstallState     PluginInstallState   `json:"installState"`
	ManagementTarget string               `json:"managementTarget"`
	Publisher        string               `json:"publisher,omitempty"`
	PermissionSummary *PermissionSummary  `json:"permissionSummary,omitempty"`
}

type DesktopPetPluginDetail struct {
	ExtensionID       string               `json:"extensionId"`
	PluginID          string               `json:"pluginId"`
	Name              string               `json:"name"`
	Description       string               `json:"description"`
	Version           string               `json:"version"`
	Enabled           bool                 `json:"enabled"`
	InstallState      PluginInstallState   `json:"installState"`
	ManagementTarget  string               `json:"managementTarget"`
	Publisher         string               `json:"publisher,omitempty"`
	PermissionSummary *PermissionSummary  `json:"permissionSummary,omitempty"`
	RequiredPermissions []string           `json:"requiredPermissions,omitempty"`
	PackageVersion    string               `json:"packageVersion,omitempty"`
	InstalledAt       *time.Time           `json:"installedAt,omitempty"`
	UpdatedAt         *time.Time           `json:"updatedAt,omitempty"`
	Source            string               `json:"source,omitempty"`
}

type InstallRequest struct {
	PackagePath string `json:"packagePath"`
}

type InstallResult struct {
	ExtensionID  string `json:"extensionId"`
	Version      string `json:"version"`
	InstallState string `json:"installState"`
}

type UpdateRequest struct {
	ExtensionID string `json:"extensionId"`
}

type MutationResult struct {
	ExtensionID string `json:"extensionId"`
	Success     bool   `json:"success"`
}

type ListResponse struct {
	Plugins []DesktopPetPluginSummary `json:"plugins"`
	Total   int                       `json:"total"`
	Page    int                       `json:"page"`
	PageSize int                      `json:"pageSize"`
}
