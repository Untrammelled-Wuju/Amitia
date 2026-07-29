package desktop

import (
	"encoding/json"
	"fmt"
	"time"
)

type LocalizedText struct {
	Default      string            `json:"default"`
	Translations map[string]string `json:"translations,omitempty"`
}

func (l LocalizedText) Get(lang string) string {
	if l.Translations != nil {
		if v, ok := l.Translations[lang]; ok {
			return v
		}
	}
	return l.Default
}

type DesktopOrderDefinition struct {
	Group    string `json:"group,omitempty"`
	Priority int    `json:"priority"`
	Before   string `json:"before,omitempty"`
	After    string `json:"after,omitempty"`
}

type DesktopVisibilityRule struct {
	Platform          []string `json:"platform,omitempty"`
	WindowFocused     *bool    `json:"windowFocused,omitempty"`
	ExtensionEnabled  *bool    `json:"extensionEnabled,omitempty"`
	CharacterScope    []string `json:"characterScope,omitempty"`
	ConversationScope []string `json:"conversationScope,omitempty"`
	MessageType       []string `json:"messageType,omitempty"`
	SelectionExists   *bool    `json:"selectionExists,omitempty"`
	RuntimeReady      *bool    `json:"runtimeReady,omitempty"`
	PermissionGranted []string `json:"permissionGranted,omitempty"`
}

type DesktopEnabledRule struct {
	Platform          []string `json:"platform,omitempty"`
	ExtensionEnabled  *bool    `json:"extensionEnabled,omitempty"`
	RuntimeReady      *bool    `json:"runtimeReady,omitempty"`
	PermissionGranted []string `json:"permissionGranted,omitempty"`
}

type DesktopActionBinding struct {
	ActionType string          `json:"actionType"`
	TargetID   string          `json:"targetId,omitempty"`
	Input      json.RawMessage `json:"input,omitempty"`
}

var allowedActionTypes = map[string]bool{
	"host_action":       true,
	"tool_invoke":       true,
	"workflow_execute":  true,
	"task_enqueue":      true,
	"extension_command": true,
	"navigation":        true,
	"dialog_open":       true,
}

var forbiddenActionTypes = map[string]bool{
	"raw_ipc":        true,
	"electron_call":  true,
	"shell":          true,
	"raw_http":       true,
	"raw_sql":        true,
	"file_path_open": true,
}

func (a *DesktopActionBinding) Validate() error {
	if a.ActionType == "" {
		return ErrInvalidActionType
	}
	if forbiddenActionTypes[a.ActionType] {
		return fmt.Errorf("%w: %s", ErrForbiddenActionType, a.ActionType)
	}
	if !allowedActionTypes[a.ActionType] {
		return fmt.Errorf("%w: %s", ErrInvalidActionType, a.ActionType)
	}
	return nil
}

type DesktopShortcutDefinition struct {
	Accelerator string `json:"accelerator"`
	Scope       string `json:"scope,omitempty"`
	Global      bool   `json:"global"`
	Repeatable  bool   `json:"repeatable,omitempty"`
}

type PermissionRequirement struct {
	PermissionID string `json:"permissionId"`
	Scope        string `json:"scope,omitempty"`
	Required     bool   `json:"required"`
}

type ScopeRule struct {
	ScopeType string `json:"scopeType"`
	ScopeID   string `json:"scopeId,omitempty"`
}

type DependencyRequirement struct {
	ExtensionID string `json:"extensionId"`
	MinVersion  string `json:"minVersion,omitempty"`
}

type DesktopContributionDefinition struct {
	ContributionID string `json:"contributionId"`
	ExtensionID    string `json:"extensionId"`
	ModuleID       string `json:"moduleId"`

	DesktopType     DesktopType `json:"desktopType"`
	ContractID      string      `json:"contractId"`
	ContractVersion int         `json:"contractVersion"`

	Target string                 `json:"target"`
	Order  DesktopOrderDefinition `json:"order"`

	Label          LocalizedText `json:"label"`
	Description    LocalizedText `json:"description,omitempty"`
	IconResourceID *string       `json:"iconResourceId,omitempty"`

	Visibility  DesktopVisibilityRule `json:"visibility,omitempty"`
	EnabledRule DesktopEnabledRule    `json:"enabledRule,omitempty"`

	Action   DesktopActionBinding       `json:"action"`
	Shortcut *DesktopShortcutDefinition `json:"shortcut,omitempty"`

	PermissionRequirements []PermissionRequirement `json:"permissionRequirements,omitempty"`
	ScopeRule              ScopeRule               `json:"scopeRule,omitempty"`
	DependencyRequirements []DependencyRequirement `json:"dependencyRequirements,omitempty"`

	DefinitionHash string `json:"definitionHash"`
	Version        string `json:"version"`
}

type ContributionStatus string

const (
	ContributionStatusDeclared          ContributionStatus = "declared"
	ContributionStatusPendingPermission ContributionStatus = "pending_permission"
	ContributionStatusRegistered        ContributionStatus = "registered"
	ContributionStatusConflict          ContributionStatus = "conflict"
	ContributionStatusUnsupported       ContributionStatus = "unsupported"
	ContributionStatusDisabled          ContributionStatus = "disabled"
	ContributionStatusFailed            ContributionStatus = "failed"
	ContributionStatusQuarantined       ContributionStatus = "quarantined"
)

type ResolvedDesktopContribution struct {
	Definition     DesktopContributionDefinition `json:"definition"`
	Status         ContributionStatus            `json:"status"`
	Generation     int64                         `json:"generation"`
	EffectiveLabel string                        `json:"effectiveLabel"`
	EffectiveIcon  *string                       `json:"effectiveIcon,omitempty"`
	ResolvedAt     time.Time                     `json:"resolvedAt"`
	ConflictReason string                        `json:"conflictReason,omitempty"`
}
