package uiagent

import (
	"github.com/u-ai/backend/internal/extension/kernel/capability/acquisition"
)

type UIActionType string

const (
	ActionCreate          UIActionType = "create"
	ActionModify          UIActionType = "modify"
	ActionRemove          UIActionType = "remove"
	ActionMove            UIActionType = "move"
	ActionRestyle         UIActionType = "restyle"
	ActionRestructure     UIActionType = "restructure"
	ActionBindData        UIActionType = "bind_data"
	ActionAddAction       UIActionType = "add_action"
	ActionReplaceComponent UIActionType = "replace_component"
)

type UITargetType string

const (
	UITargetSource       UITargetType = "source"
	UITargetSchema       UITargetType = "schema"
	UITargetContribution UITargetType = "contribution"
)

type PreviewStrategy string

const (
	PreviewStructural PreviewStrategy = "structural"
	PreviewRenderTree PreviewStrategy = "render_tree"
	PreviewScreenshot PreviewStrategy = "screenshot"
)

type RollbackStrategy string

const (
	RollbackEditTransaction RollbackStrategy = "edit_transaction"
	RollbackSchemaRevision  RollbackStrategy = "schema_revision"
	RollbackContributionRev RollbackStrategy = "contribution_revision"
)

type UIRisk string

const (
	UIRiskLow    UIRisk = "low"
	UIRiskMedium UIRisk = "medium"
	UIRiskHigh   UIRisk = "high"
)

type DeploymentTarget = acquisition.DeploymentTarget
