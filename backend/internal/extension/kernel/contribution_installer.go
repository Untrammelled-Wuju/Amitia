package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/agent_skill"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/desktop"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/event"
	"github.com/u-ai/backend/internal/extension/kernel/extension_page_host"
	"github.com/u-ai/backend/internal/extension/kernel/hook"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/schedule"
	"github.com/u-ai/backend/internal/extension/kernel/schema_ui"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/extension/kernel/ui_contribution"
	"github.com/u-ai/backend/internal/extension/kernel/ui_provider"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
)

type TypedContributionInstaller struct {
	container   *Container
	candidateNS *CandidateNamespace
}

func NewTypedContributionInstaller(container *Container) *TypedContributionInstaller {
	return &TypedContributionInstaller{container: container}
}

func (i *TypedContributionInstaller) SetContainer(container *Container) {
	if i == nil {
		return
	}
	i.container = container
}

func (i *TypedContributionInstaller) SetCandidateNamespace(ns *CandidateNamespace) {
	if i == nil {
		return
	}
	i.candidateNS = ns
}

type installOp struct {
	kind       domain.ContributionKind
	doInstall  func(ctx context.Context) error
	doRollback func(ctx context.Context)
}

type lifecycleAuditEntry struct {
	ContributionID string
	Kind           string
	OperationID    string
	Generation     int64
	StartedAt      time.Time
	FinishedAt     time.Time
	Result         string
	ErrorCode      string
}

const (
	auditResultSucceeded = "succeeded"
	auditResultFailed    = "failed"
	auditResultRollback  = "rollback"
	auditResultSkipped   = "skipped"
)

func newOperationID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func (i *TypedContributionInstaller) logAudit(entry lifecycleAuditEntry) {
	log.Printf("[lifecycle-audit] contributionID=%s kind=%s operationID=%s generation=%d startedAt=%s finishedAt=%s result=%s errorCode=%s",
		entry.ContributionID, entry.Kind, entry.OperationID, entry.Generation,
		entry.StartedAt.Format(time.RFC3339Nano), entry.FinishedAt.Format(time.RFC3339Nano),
		entry.Result, entry.ErrorCode)
}

func (i *TypedContributionInstaller) recordAudit(contrib domain.ContributionDefinition, operationID string, generation int64, startedAt time.Time, result string, err error) {
	entry := lifecycleAuditEntry{
		ContributionID: string(contrib.ID),
		Kind:           string(contrib.Kind),
		OperationID:    operationID,
		Generation:     generation,
		StartedAt:      startedAt,
		FinishedAt:     time.Now().UTC(),
		Result:         result,
	}
	if err != nil {
		entry.ErrorCode = err.Error()
	}
	i.logAudit(entry)
}

func (i *TypedContributionInstaller) InstallContributions(ctx context.Context, contribs []domain.ContributionDefinition, generation int64) error {
	if i.container == nil {
		return fmt.Errorf("contribution-installer: container not attached")
	}

	ops := make([]installOp, 0, len(contribs))
	for _, contrib := range contribs {
		op, err := i.buildInstallOp(ctx, contrib, generation)
		if err != nil {
			return fmt.Errorf("contribution-installer: build op for %s: %w", contrib.ID, err)
		}
		if op.doInstall != nil {
			ops = append(ops, op)
		}
	}

	completed := 0
	defer func() {
		if completed < len(ops) {
			for j := completed - 1; j >= 0; j-- {
				if ops[j].doRollback != nil {
					ops[j].doRollback(ctx)
				}
			}
		}
	}()

	for idx, op := range ops {
		if err := op.doInstall(ctx); err != nil {
			return fmt.Errorf("contribution-installer: install op %d (%s): %w", idx, op.kind, err)
		}
		completed = idx + 1
	}

	return nil
}

func (i *TypedContributionInstaller) buildInstallOp(ctx context.Context, contrib domain.ContributionDefinition, generation int64) (installOp, error) {
	if i.container == nil {
		return installOp{}, fmt.Errorf("container not attached")
	}

	defData, err := json.Marshal(contrib.Definition)
	if err != nil {
		return installOp{}, fmt.Errorf("marshal definition: %w", err)
	}

	switch contrib.Kind {
	case domain.ContributionKindProvider:
		return installOp{}, fmt.Errorf("legacy provider contribution must be normalized to module provider metadata: %s", contrib.ID)
	case domain.ContributionKindTool:
		return i.buildToolOp(ctx, contrib, defData, generation)
	case domain.ContributionKindEventSubscription:
		return i.buildEventSubscriptionOp(ctx, contrib, defData)
	case domain.ContributionKindHook:
		return i.buildHookOp(ctx, contrib, defData)
	case domain.ContributionKindSchedule:
		return i.buildScheduleOp(ctx, contrib, defData)
	case domain.ContributionKindAgentSkill:
		return i.buildAgentSkillOp(ctx, contrib, defData)
	case domain.ContributionKindWorkflow:
		return i.buildWorkflowOp(ctx, contrib, defData)
	case domain.ContributionKindBackgroundTask:
		return i.buildTaskDefinitionOp(ctx, contrib, defData)
	case domain.ContributionKindMCPServer:
		return i.buildMCPServerOp(ctx, contrib, defData)
	case domain.ContributionKindUIPage, domain.ContributionKindUIPanel, domain.ContributionKindUIChat, domain.ContributionKindUIContextAction, domain.ContributionKindUIDesktop:
		return i.buildUIContributionOp(ctx, contrib, defData, generation)
	case domain.ContributionKindUIProvider:
		return i.buildUIProviderOp(ctx, contrib, defData, generation)
	case domain.ContributionKindGamePlugin:
		return i.buildGamePluginOp(ctx, contrib, defData, generation)
	case domain.ContributionKindDesktopPetPlugin:
		return i.buildDesktopPetPluginOp(ctx, contrib, defData, generation)
	default:
		return installOp{}, fmt.Errorf("unsupported contribution kind: %s", contrib.Kind)
	}
}

func (i *TypedContributionInstaller) buildUIProviderOp(ctx context.Context, contrib domain.ContributionDefinition, defData []byte, generation int64) (installOp, error) {
	if i.container.UIProviderRegistry == nil {
		return installOp{}, fmt.Errorf("ui provider registry not configured")
	}
	var def ui_provider.ProviderDefinition
	if err := json.Unmarshal(defData, &def); err != nil {
		return installOp{}, fmt.Errorf("unmarshal ui provider: %w", err)
	}
	if def.ProviderID == "" {
		def.ProviderID = string(contrib.ID)
	}
	if def.ExtensionID == "" {
		def.ExtensionID = string(contrib.ExtensionID)
	}
	if def.ModuleID == "" {
		def.ModuleID = string(contrib.ModuleID)
	}
	if def.ProviderID != string(contrib.ID) || def.ExtensionID != string(contrib.ExtensionID) || def.ModuleID != string(contrib.ModuleID) {
		return installOp{}, fmt.Errorf("ui provider identity does not match manifest contribution")
	}
	def.Generation = generation
	def.Enabled = true
	if err := def.Validate(); err != nil {
		return installOp{}, err
	}
	return installOp{
		kind: domain.ContributionKindUIProvider,
		doInstall: func(ctx context.Context) error {
			if err := i.container.UIProviderRegistry.Register(def); err != nil {
				return fmt.Errorf("register ui provider: %w", err)
			}
			if i.container.UIHostNotifier != nil {
				i.container.UIHostNotifier.BroadcastExtensionChange("ui_provider_changed", string(contrib.ExtensionID), map[string]interface{}{"providerId": def.ProviderID, "capability": def.Capability})
			}
			return nil
		},
		doRollback: func(ctx context.Context) {
			i.container.UIProviderRegistry.Unregister(def.ProviderID)
		},
	}, nil
}

func (i *TypedContributionInstaller) buildToolOp(ctx context.Context, contrib domain.ContributionDefinition, defData []byte, generation int64) (installOp, error) {
	if i.container.ToolRegistry == nil {
		return installOp{}, fmt.Errorf("tool registry not configured")
	}
	var def struct {
		ToolID       string          `json:"toolId"`
		ModelName    string          `json:"modelName"`
		CapabilityID string          `json:"capabilityId,omitempty"`
		HandlerName  string          `json:"handlerName,omitempty"`
		InputSchema  json.RawMessage `json:"inputSchema"`
		OutputSchema json.RawMessage `json:"outputSchema"`
		RiskLevel    string          `json:"riskLevel,omitempty"`
		SideEffect   string          `json:"sideEffect,omitempty"`
		Permissions  json.RawMessage `json:"permissions,omitempty"`
		Scope        json.RawMessage `json:"scope,omitempty"`
		Internal     bool            `json:"internal,omitempty"`
		Runtime      map[string]any  `json:"runtime,omitempty"`
	}
	if err := json.Unmarshal(defData, &def); err != nil {
		return installOp{}, fmt.Errorf("unmarshal tool definition: %w", err)
	}

	runtimeType := ""
	runtimeID := ""
	handlerName := def.HandlerName
	if len(def.Runtime) > 0 {
		if v, ok := def.Runtime["runtimeType"].(string); ok {
			runtimeType = v
		}
		if v, ok := def.Runtime["runtimeId"].(string); ok {
			runtimeID = v
		}
		if v, ok := def.Runtime["handlerName"].(string); ok && v != "" {
			handlerName = v
		}
	}

	toolID := def.ToolID
	if toolID == "" {
		toolID = string(contrib.ID)
	}
	modelName := def.ModelName
	if modelName == "" {
		modelName = contrib.Name.Default
	}
	capID := capability.CapabilityID(def.CapabilityID)
	if capID == "" {
		capID = capability.CapabilityID(toolID)
	}

	var perms []capability.PermissionRequirement
	if len(def.Permissions) > 0 {
		_ = json.Unmarshal(def.Permissions, &perms)
	}
	var scope capability.ScopeRule
	if len(def.Scope) > 0 {
		_ = json.Unmarshal(def.Scope, &scope)
	}

	toolSource := capability.ToolSourcePlugin
	if isSystemBuiltin(contrib.Metadata) {
		toolSource = capability.ToolSourceBuiltin
	}

	toolDef := capability.ToolDefinition{
		ID:           toolID,
		ModelName:    modelName,
		CapabilityID: capID,
		ExtensionID:  string(contrib.ExtensionID),
		ModuleID:     string(contrib.ModuleID),
		Source:       toolSource,
		Name:         contrib.Name.Default,
		Description:  contrib.Description.Default,
		Version:      contrib.Version,
		InputSchema:  def.InputSchema,
		OutputSchema: def.OutputSchema,
		Enabled:      false,
		Internal:     def.Internal,
		RiskLevel:    capability.RiskLevel(def.RiskLevel),
		SideEffect:   capability.SideEffectLevel(def.SideEffect),
		Permissions:  perms,
		Scope:        scope,
		Runtime:      i.buildRuntimeBindingFromValues(contrib, handlerName, runtimeType, runtimeID, toolID),
	}

	permIDs := make([]string, 0)
	for _, p := range perms {
		if p.Capability != "" {
			permIDs = append(permIDs, p.Capability)
		}
	}
	toolDef.Idempotent = (def.SideEffect == "none" || def.SideEffect == "read_only")

	return installOp{
		kind: domain.ContributionKindTool,
		doInstall: func(ctx context.Context) error {
			if err := i.container.ToolRegistry.Replace(ctx, toolDef); err != nil {
				return fmt.Errorf("register tool %s: %w", toolID, err)
			}
			return nil
		},
		doRollback: func(ctx context.Context) {
			_ = i.container.ToolRegistry.Unregister(ctx, toolID)
		},
	}, nil
}

func (i *TypedContributionInstaller) buildGamePluginOp(ctx context.Context, contrib domain.ContributionDefinition, defData []byte, generation int64) (installOp, error) {
	var def struct {
		ProtocolVersion string `json:"protocolVersion"`
		RuntimeModuleID string `json:"runtimeModuleId,omitempty"`
		DisplayName     string `json:"displayName,omitempty"`
	}
	if err := json.Unmarshal(defData, &def); err != nil {
		return installOp{}, fmt.Errorf("unmarshal game plugin: %w", err)
	}
	if def.ProtocolVersion == "" {
		return installOp{}, fmt.Errorf("game_plugin: protocolVersion required")
	}
	if contrib.ID == "" {
		return installOp{}, fmt.Errorf("game_plugin: id required")
	}
	return installOp{
		kind: domain.ContributionKindGamePlugin,
		doInstall: func(ctx context.Context) error {
			i.logAudit(lifecycleAuditEntry{
				ContributionID: string(contrib.ID),
				Kind:           string(contrib.Kind),
				OperationID:    newOperationID("game-plugin-install"),
				Generation:     generation,
				StartedAt:      time.Now().UTC(),
				FinishedAt:     time.Now().UTC(),
				Result:         auditResultSucceeded,
			})
			return nil
		},
		doRollback: func(ctx context.Context) {
			i.logAudit(lifecycleAuditEntry{
				ContributionID: string(contrib.ID),
				Kind:           string(contrib.Kind),
				OperationID:    newOperationID("game-plugin-rollback"),
				Generation:     generation,
				StartedAt:      time.Now().UTC(),
				FinishedAt:     time.Now().UTC(),
				Result:         auditResultRollback,
			})
		},
	}, nil
}

func (i *TypedContributionInstaller) buildDesktopPetPluginOp(ctx context.Context, contrib domain.ContributionDefinition, defData []byte, generation int64) (installOp, error) {
	if contrib.ID == "" {
		return installOp{}, fmt.Errorf("desktop_pet_plugin: id required")
	}
	return installOp{
		kind: domain.ContributionKindDesktopPetPlugin,
		doInstall: func(ctx context.Context) error {
			i.logAudit(lifecycleAuditEntry{
				ContributionID: string(contrib.ID),
				Kind:           string(contrib.Kind),
				OperationID:    newOperationID("desktop-pet-plugin-install"),
				Generation:     generation,
				StartedAt:      time.Now().UTC(),
				FinishedAt:     time.Now().UTC(),
				Result:         auditResultSucceeded,
			})
			if i.container.DesktopPetPluginBoundary != nil {
				if err := i.container.DesktopPetPluginBoundary.MarkContributionAvailable(ctx, contrib.ExtensionID, contrib); err != nil {
					log.Printf("[desktop-pet-plugin] mark contribution available failed: %v", err)
				}
			}
			return nil
		},
		doRollback: func(ctx context.Context) {
			i.logAudit(lifecycleAuditEntry{
				ContributionID: string(contrib.ID),
				Kind:           string(contrib.Kind),
				OperationID:    newOperationID("desktop-pet-plugin-rollback"),
				Generation:     generation,
				StartedAt:      time.Now().UTC(),
				FinishedAt:     time.Now().UTC(),
				Result:         auditResultRollback,
			})
		},
	}, nil
}

func (i *TypedContributionInstaller) buildEventSubscriptionOp(ctx context.Context, contrib domain.ContributionDefinition, defData []byte) (installOp, error) {
	if i.container.EventService == nil {
		return installOp{}, fmt.Errorf("event service not configured")
	}
	var def event.EventSubscriptionDefinition
	if err := json.Unmarshal(defData, &def); err != nil {
		return installOp{}, fmt.Errorf("unmarshal event subscription: %w", err)
	}
	if def.ContributionID == "" {
		def.ContributionID = string(contrib.ID)
	}
	if def.ExtensionID == "" {
		def.ExtensionID = string(contrib.ExtensionID)
	}
	if def.ModuleID == "" {
		def.ModuleID = string(contrib.ModuleID)
	}
	if err := def.Validate(); err != nil {
		return installOp{}, fmt.Errorf("validate event subscription: %w", err)
	}
	return installOp{
		kind: domain.ContributionKindEventSubscription,
		doInstall: func(ctx context.Context) error {
			if err := i.container.EventService.RegisterSubscription(ctx, def); err != nil {
				return fmt.Errorf("register event subscription: %w", err)
			}
			return nil
		},
		doRollback: func(ctx context.Context) {
			_ = i.container.EventService.UnregisterSubscription(ctx, def.ContributionID)
		},
	}, nil
}

func (i *TypedContributionInstaller) buildHookOp(ctx context.Context, contrib domain.ContributionDefinition, defData []byte) (installOp, error) {
	if i.container.HookService == nil || i.container.HookService.Lifecycle == nil {
		return installOp{}, fmt.Errorf("hook service not configured")
	}
	var def hook.HookContributionDefinition
	if err := json.Unmarshal(defData, &def); err != nil {
		return installOp{}, fmt.Errorf("unmarshal hook: %w", err)
	}
	if def.ContributionID == "" {
		def.ContributionID = string(contrib.ID)
	}
	if def.ExtensionID == "" {
		def.ExtensionID = string(contrib.ExtensionID)
	}
	if def.ModuleID == "" {
		def.ModuleID = string(contrib.ModuleID)
	}
	return installOp{
		kind: domain.ContributionKindHook,
		doInstall: func(ctx context.Context) error {
			if err := i.container.HookService.Lifecycle.InstallContribution(ctx, def); err != nil {
				return fmt.Errorf("install hook: %w", err)
			}
			return nil
		},
		doRollback: func(ctx context.Context) {
			_ = i.container.HookService.Lifecycle.UninstallContribution(ctx, def.ContributionID)
		},
	}, nil
}

func (i *TypedContributionInstaller) buildScheduleOp(ctx context.Context, contrib domain.ContributionDefinition, defData []byte) (installOp, error) {
	if i.container.ScheduleService == nil {
		return installOp{}, fmt.Errorf("schedule service not configured")
	}
	var def schedule.ScheduleContributionDefinition
	if err := json.Unmarshal(defData, &def); err != nil {
		return installOp{}, fmt.Errorf("unmarshal schedule: %w", err)
	}
	if def.ContributionID == "" {
		def.ContributionID = string(contrib.ID)
	}
	if def.ExtensionID == "" {
		def.ExtensionID = string(contrib.ExtensionID)
	}
	if def.ModuleID == "" {
		def.ModuleID = string(contrib.ModuleID)
	}
	return installOp{
		kind: domain.ContributionKindSchedule,
		doInstall: func(ctx context.Context) error {
			if err := i.container.ScheduleService.InstallDefinition(ctx, &def); err != nil {
				return fmt.Errorf("install schedule: %w", err)
			}
			return nil
		},
		doRollback: func(ctx context.Context) {
			if def.ScheduleID != "" {
				_ = i.container.ScheduleService.Uninstall(ctx, def.ScheduleID)
			}
		},
	}, nil
}

func (i *TypedContributionInstaller) buildAgentSkillOp(ctx context.Context, contrib domain.ContributionDefinition, defData []byte) (installOp, error) {
	if i.container.AgentSkillCatalog == nil {
		return installOp{}, fmt.Errorf("agent skill catalog not configured")
	}
	var def agent_skill.AgentSkillDefinition
	if err := json.Unmarshal(defData, &def); err != nil {
		return installOp{}, fmt.Errorf("unmarshal agent skill: %w", err)
	}
	if def.ExtensionID == "" {
		def.ExtensionID = string(contrib.ExtensionID)
	}
	if def.ModuleID == "" {
		def.ModuleID = string(contrib.ModuleID)
	}
	if def.ID == "" {
		def.ID = string(contrib.ID)
	}
	return installOp{
		kind: domain.ContributionKindAgentSkill,
		doInstall: func(ctx context.Context) error {
			if err := i.container.AgentSkillCatalog.Register(def); err != nil {
				return fmt.Errorf("register agent skill: %w", err)
			}
			return nil
		},
		doRollback: func(ctx context.Context) {
			_ = i.container.AgentSkillCatalog.Unregister(string(contrib.ExtensionID))
		},
	}, nil
}

func (i *TypedContributionInstaller) buildWorkflowOp(ctx context.Context, contrib domain.ContributionDefinition, defData []byte) (installOp, error) {
	if i.container.WorkflowRegistry == nil {
		return installOp{}, fmt.Errorf("workflow registry not configured")
	}
	var def workflow.WorkflowDefinition
	if err := json.Unmarshal(defData, &def); err != nil {
		return installOp{}, fmt.Errorf("unmarshal workflow: %w", err)
	}
	if def.ID == "" {
		def.ID = string(contrib.ID)
	}
	if def.ExtensionID == "" {
		def.ExtensionID = string(contrib.ExtensionID)
	}
	if def.ModuleID == "" {
		def.ModuleID = string(contrib.ModuleID)
	}
	return installOp{
		kind: domain.ContributionKindWorkflow,
		doInstall: func(ctx context.Context) error {
			if err := i.container.WorkflowRegistry.Register(def); err != nil {
				return fmt.Errorf("register workflow: %w", err)
			}
			return nil
		},
		doRollback: func(ctx context.Context) {
			_ = i.container.WorkflowRegistry.Unregister(def.ID)
		},
	}, nil
}

func (i *TypedContributionInstaller) buildTaskDefinitionOp(ctx context.Context, contrib domain.ContributionDefinition, defData []byte) (installOp, error) {
	if i.container.TaskRuntimeService == nil {
		return installOp{}, fmt.Errorf("task runtime service not configured")
	}
	var def task_runtime.TaskDefinition
	if err := json.Unmarshal(defData, &def); err != nil {
		return installOp{}, fmt.Errorf("unmarshal task definition: %w", err)
	}
	if def.TaskID == "" {
		def.TaskID = string(contrib.ID)
	}
	if def.ExtensionID == "" {
		def.ExtensionID = string(contrib.ExtensionID)
	}
	if def.ModuleID == "" {
		def.ModuleID = string(contrib.ModuleID)
	}
	if def.ContributionID == "" {
		def.ContributionID = string(contrib.ID)
	}
	return installOp{
		kind: domain.ContributionKindBackgroundTask,
		doInstall: func(ctx context.Context) error {
			if err := i.container.TaskRuntimeService.PutTaskDefinition(ctx, &def); err != nil {
				return fmt.Errorf("put task definition: %w", err)
			}
			return nil
		},
		doRollback: func(ctx context.Context) {
			_ = i.container.TaskRuntimeService.DeleteTaskDefinition(ctx, def.TaskID)
		},
	}, nil
}

func (i *TypedContributionInstaller) buildMCPServerOp(ctx context.Context, contrib domain.ContributionDefinition, defData []byte) (installOp, error) {
	mcpToolAdapter := capability.GetGlobalMCPToolAdapter()
	if mcpToolAdapter == nil {
		return installOp{}, fmt.Errorf("mcp tool adapter not available")
	}
	var def struct {
		ServerName string            `json:"serverName"`
		Command    string            `json:"command,omitempty"`
		Args       []string          `json:"args,omitempty"`
		Env        map[string]string `json:"env,omitempty"`
		URL        string            `json:"url,omitempty"`
		Tools      json.RawMessage   `json:"tools,omitempty"`
	}
	if err := json.Unmarshal(defData, &def); err != nil {
		return installOp{}, fmt.Errorf("unmarshal mcp server: %w", err)
	}
	serverID := def.ServerName
	if serverID == "" {
		serverID = string(contrib.ID)
	}
	return installOp{
		kind: domain.ContributionKindMCPServer,
		doInstall: func(ctx context.Context) error {
			if err := mcpToolAdapter.RegisterServerWithDefinition(ctx, serverID, defData, string(contrib.ExtensionID)); err != nil {
				return fmt.Errorf("register mcp server: %w", err)
			}
			return nil
		},
		doRollback: func(ctx context.Context) {
			_ = mcpToolAdapter.UnregisterServer(ctx, serverID)
		},
	}, nil
}

func (i *TypedContributionInstaller) buildUIContributionOp(ctx context.Context, contrib domain.ContributionDefinition, defData []byte, generation int64) (installOp, error) {
	if i.container.UIHost == nil {
		return installOp{}, fmt.Errorf("ui host not configured")
	}
	var uiDef ui_contribution.UIContributionDefinition
	if err := json.Unmarshal(defData, &uiDef); err != nil {
		return installOp{}, fmt.Errorf("unmarshal ui contribution: %w", err)
	}
	if uiDef.ContributionID == "" {
		uiDef.ContributionID = ui_contribution.ContributionID(contrib.ID)
	}
	if uiDef.ExtensionID == "" {
		uiDef.ExtensionID = ui_contribution.ExtensionID(contrib.ExtensionID)
	}
	if uiDef.ModuleID == "" {
		uiDef.ModuleID = ui_contribution.ModuleID(contrib.ModuleID)
	}
	if uiDef.ContributionID != ui_contribution.ContributionID(contrib.ID) || uiDef.ExtensionID != ui_contribution.ExtensionID(contrib.ExtensionID) || uiDef.ModuleID != ui_contribution.ModuleID(contrib.ModuleID) {
		return installOp{}, fmt.Errorf("ui contribution identity does not match manifest contribution")
	}
	uiDef.Integrity.Generation = generation

	hasPage := uiDef.Kind == ui_contribution.UIContributionWebPage || uiDef.Kind == ui_contribution.UIContributionSchemaPage
	hasSchema := uiDef.Entry.SchemaPath != "" || uiDef.Sandbox.Type == ui_contribution.SandboxSchemaRenderer
	if hasSchema && uiDef.Entry.SchemaPath == "" {
		return installOp{}, fmt.Errorf("schema ui contribution %s requires entry.schema_path", uiDef.ContributionID)
	}
	basePath := ""
	if hasSchema {
		if i.container.SchemaRegistry == nil {
			return installOp{}, fmt.Errorf("schema registry not configured")
		}
		basePath = resolveExtensionBundlePath(i.container.ExtRoot, string(contrib.ExtensionID))
		if basePath == "" {
			return installOp{}, fmt.Errorf("extension bundle path not found for schema %s", uiDef.ContributionID)
		}
		validationRegistry := schema_ui.NewSchemaRegistry(i.container.SchemaValidator, nil)
		if err := validationRegistry.LoadFromPathWithContext(string(uiDef.ExtensionID), string(uiDef.ContributionID), generation, "", "", basePath, uiDef.Entry.SchemaPath); err != nil {
			return installOp{}, fmt.Errorf("validate schema resource: %w", err)
		}
	}

	if contrib.Kind == domain.ContributionKindUIDesktop {
		return i.buildDesktopContributionOp(ctx, contrib, defData, uiDef, generation)
	}

	return installOp{
		kind: contrib.Kind,
		doInstall: func(ctx context.Context) error {
			if hasSchema {
				if err := i.container.SchemaRegistry.LoadFromPathWithContext(string(uiDef.ExtensionID), string(uiDef.ContributionID), generation, "", "", basePath, uiDef.Entry.SchemaPath); err != nil {
					return fmt.Errorf("load schema resource: %w", err)
				}
			}
			if err := i.container.UIHost.RegisterContribution(&uiDef); err != nil {
				if hasSchema {
					i.container.SchemaRegistry.UnregisterSchema(string(uiDef.ExtensionID), string(uiDef.ContributionID))
				}
				return fmt.Errorf("register ui contribution: %w", err)
			}
			if i.container.UIContributionRepo != nil {
				if err := i.container.UIContributionRepo.PutContribution(ctx, &uiDef); err != nil {
					_ = i.container.UIHost.UnregisterContribution(uiDef.ContributionID)
					if hasSchema {
						i.container.SchemaRegistry.UnregisterSchema(string(uiDef.ExtensionID), string(uiDef.ContributionID))
					}
					return fmt.Errorf("persist ui contribution: %w", err)
				}
			}
			if hasPage {
				if err := i.registerPage(ctx, uiDef, generation); err != nil {
					_ = i.container.UIHost.UnregisterContribution(uiDef.ContributionID)
					if i.container.UIContributionRepo != nil {
						_ = i.container.UIContributionRepo.DeleteContribution(ctx, string(uiDef.ContributionID))
					}
					if hasSchema {
						i.container.SchemaRegistry.UnregisterSchema(string(uiDef.ExtensionID), string(uiDef.ContributionID))
					}
					return fmt.Errorf("register page: %w", err)
				}
			}
			return nil
		},
		doRollback: func(ctx context.Context) {
			if hasSchema {
				i.container.SchemaRegistry.UnregisterSchema(string(uiDef.ExtensionID), string(uiDef.ContributionID))
			}
			_ = i.container.UIHost.UnregisterContribution(uiDef.ContributionID)
			if i.container.UIContributionRepo != nil {
				_ = i.container.UIContributionRepo.DeleteContribution(ctx, string(uiDef.ContributionID))
			}
			if hasPage && i.container.PageHost != nil {
				_ = i.container.PageHost.UnregisterPage(ctx, extension_page_host.ContributionID(uiDef.ContributionID))
			}
		},
	}, nil
}

func (i *TypedContributionInstaller) buildDesktopContributionOp(ctx context.Context, contrib domain.ContributionDefinition, defData []byte, uiDef ui_contribution.UIContributionDefinition, generation int64) (installOp, error) {
	if i.container.DesktopHost == nil {
		return installOp{}, fmt.Errorf("desktop host not configured")
	}
	var desktopDef desktop.DesktopContributionDefinition
	if err := json.Unmarshal(defData, &desktopDef); err != nil {
		return installOp{}, fmt.Errorf("unmarshal desktop contribution: %w", err)
	}
	if desktopDef.ContributionID == "" {
		desktopDef.ContributionID = string(contrib.ID)
	}
	if desktopDef.ExtensionID == "" {
		desktopDef.ExtensionID = string(contrib.ExtensionID)
	}
	if desktopDef.ModuleID == "" {
		desktopDef.ModuleID = string(contrib.ModuleID)
	}
	if desktopDef.Version == "" {
		desktopDef.Version = contrib.Version
	}

	contributionID := desktopDef.ContributionID

	return installOp{
		kind: domain.ContributionKindUIDesktop,
		doInstall: func(ctx context.Context) error {
			if err := i.container.UIHost.RegisterContribution(&uiDef); err != nil {
				return fmt.Errorf("register ui contribution: %w", err)
			}
			if i.container.UIContributionRepo != nil {
				_ = i.container.UIContributionRepo.PutContribution(ctx, &uiDef)
			}
			_, err := i.container.DesktopHost.RegisterContribution(ctx, desktopDef)
			if err != nil {
				return fmt.Errorf("register desktop contribution: %w", err)
			}
			return nil
		},
		doRollback: func(ctx context.Context) {
			_ = i.container.UIHost.UnregisterContribution(uiDef.ContributionID)
			if i.container.UIContributionRepo != nil {
				_ = i.container.UIContributionRepo.DeleteContribution(ctx, contributionID)
			}
			if i.container.DesktopHost != nil {
				_ = i.container.DesktopHost.UnregisterContribution(contributionID)
			}
		},
	}, nil
}

func (i *TypedContributionInstaller) registerPage(ctx context.Context, uiDef ui_contribution.UIContributionDefinition, generation int64) error {
	if i.container.PageHost == nil {
		return fmt.Errorf("page host not configured")
	}
	entryKind := extension_page_host.PageKindWeb
	if uiDef.Kind == ui_contribution.UIContributionSchemaPage {
		entryKind = extension_page_host.PageKindSchema
	}
	perms := make([]string, 0, len(uiDef.Permissions))
	for _, p := range uiDef.Permissions {
		perms = append(perms, p.Name)
	}
	pageDef := extension_page_host.NewExtensionPageDefinition(extension_page_host.PageRegistrationInput{
		PageID:          extension_page_host.PageID(uiDef.ContributionID),
		ExtensionID:     extension_page_host.ExtensionID(uiDef.ExtensionID),
		ModuleID:        string(uiDef.ModuleID),
		ContributionID:  extension_page_host.ContributionID(uiDef.ContributionID),
		Generation:      generation,
		ContractVersion: uiDef.ContractVersion,
		EntryKind:       entryKind,
		EntryPath:       uiDef.Entry.Path,
		SchemaPath:      uiDef.Entry.SchemaPath,
		Title: extension_page_host.LocalizedText{
			Default:      uiDef.Display.Title.Default,
			Translations: uiDef.Display.Title.I18n,
		},
		Description: extension_page_host.LocalizedText{
			Default:      uiDef.Display.Description.Default,
			Translations: uiDef.Display.Description.I18n,
		},
		Icon:        uiDef.Display.Icon,
		Permissions: perms,
	})
	return i.container.PageHost.RegisterPage(ctx, pageDef)
}

func (i *TypedContributionInstaller) buildRuntimeBinding(contrib domain.ContributionDefinition) capability.RuntimeBinding {
	return i.buildRuntimeBindingWithHandler(contrib, "", "")
}

func (i *TypedContributionInstaller) buildRuntimeBindingWithHandler(contrib domain.ContributionDefinition, handlerName string, toolID string) capability.RuntimeBinding {
	return i.buildRuntimeBindingFromValues(contrib, handlerName, "", "", toolID)
}

func (i *TypedContributionInstaller) buildRuntimeBindingFromValues(contrib domain.ContributionDefinition, handlerName string, runtimeType string, runtimeID string, toolID string) capability.RuntimeBinding {
	rb := capability.RuntimeBinding{
		HandlerName: string(contrib.ModuleID),
		Metadata: map[string]any{
			"extensionId": string(contrib.ExtensionID),
			"moduleId":    string(contrib.ModuleID),
		},
	}
	if contrib.RuntimeBinding != nil {
		rb.RuntimeType = capability.RuntimeType(contrib.RuntimeBinding.RuntimeType)
		rb.RuntimeID = string(contrib.RuntimeBinding.RuntimeID)
		if contrib.RuntimeBinding.InstanceID != "" {
			rb.HandlerName = contrib.RuntimeBinding.InstanceID
		}
	}
	if runtimeType != "" {
		rb.RuntimeType = capability.RuntimeType(runtimeType)
	}
	if runtimeID != "" {
		rb.RuntimeID = runtimeID
	}
	if handlerName != "" {
		rb.HandlerName = handlerName
		rb.Metadata["handlerName"] = handlerName
	} else if toolID != "" {
		switch rb.RuntimeType {
		case capability.RuntimeTypeBrowser,
			capability.RuntimeTypeWorkspace,
			capability.RuntimeTypeInternal:
			rb.HandlerName = toolID
			rb.Metadata["handlerName"] = toolID
		}
	}
	return rb
}

func (i *TypedContributionInstaller) ActivateContributions(ctx context.Context, extID domain.ExtensionID) error {
	if i.container == nil {
		return fmt.Errorf("contribution-installer: container not attached")
	}
	contribs, err := i.container.ContributionRepository.ListContributions(ctx, extID)
	if err != nil {
		return fmt.Errorf("contribution-installer: list contributions for activate %s: %w", extID, err)
	}

	operationID := newOperationID("activate")
	generation := int64(0)
	if inst, instErr := i.container.InstallationRepository.GetInstallation(ctx, extID); instErr == nil {
		generation = inst.Generation
	}

	activated := make([]domain.ContributionDefinition, 0, len(contribs))
	seen := make(map[domain.ContributionID]bool, len(contribs))

	for _, contrib := range contribs {
		if seen[contrib.ID] {
			continue
		}
		seen[contrib.ID] = true

		startedAt := time.Now().UTC()
		if err := i.activateSingle(ctx, contrib); err != nil {
			i.recordAudit(contrib, operationID, generation, startedAt, "failed", err)
			for j := len(activated) - 1; j >= 0; j-- {
				rollbackStart := time.Now().UTC()
				rollbackErr := i.deactivateSingle(ctx, activated[j])
				i.recordAudit(activated[j], operationID, generation, rollbackStart, auditResultRollback, rollbackErr)
			}
			return fmt.Errorf("contribution-installer: activate %s (%s): %w", contrib.ID, contrib.Kind, err)
		}
		i.recordAudit(contrib, operationID, generation, startedAt, "succeeded", nil)
		activated = append(activated, contrib)
	}

	return nil
}

func (i *TypedContributionInstaller) activateSingle(ctx context.Context, contrib domain.ContributionDefinition) error {
	switch contrib.Kind {
	case domain.ContributionKindTool:
		return i.activateTool(ctx, contrib)
	case domain.ContributionKindEventSubscription:
		return i.activateEventSubscription(ctx, contrib)
	case domain.ContributionKindHook:
		return i.activateHook(ctx, contrib)
	case domain.ContributionKindSchedule:
		return i.activateSchedule(ctx, contrib)
	case domain.ContributionKindAgentSkill:
		return i.activateAgentSkill(ctx, contrib)
	case domain.ContributionKindWorkflow:
		return i.activateWorkflow(ctx, contrib)
	case domain.ContributionKindUIPage, domain.ContributionKindUIPanel, domain.ContributionKindUIChat, domain.ContributionKindUIContextAction, domain.ContributionKindUIDesktop:
		return i.activateUI(ctx, contrib)
	case domain.ContributionKindUIProvider:
		return i.activateUIProvider(ctx, contrib)
	}
	return nil
}

func (i *TypedContributionInstaller) activateUIProvider(ctx context.Context, contrib domain.ContributionDefinition) error {
	if i.container.UIProviderRegistry == nil {
		return fmt.Errorf("ui provider registry not configured")
	}
	broadcast := func() {
		if i.container.UIHostNotifier != nil {
			i.container.UIHostNotifier.BroadcastExtensionChange("ui_provider_changed", string(contrib.ExtensionID), map[string]interface{}{
				"providerId": string(contrib.ID),
				"enabled":    true,
			})
		}
	}
	if _, ok := i.container.UIProviderRegistry.Get(string(contrib.ID)); !ok {
		defData, _ := json.Marshal(contrib.Definition)
		var def ui_provider.ProviderDefinition
		if err := json.Unmarshal(defData, &def); err != nil {
			return fmt.Errorf("unmarshal ui provider for activate: %w", err)
		}
		if def.ProviderID == "" {
			def.ProviderID = string(contrib.ID)
		}
		if def.ExtensionID == "" {
			def.ExtensionID = string(contrib.ExtensionID)
		}
		if def.ModuleID == "" {
			def.ModuleID = string(contrib.ModuleID)
		}
		if def.ProviderID != string(contrib.ID) || def.ExtensionID != string(contrib.ExtensionID) || def.ModuleID != string(contrib.ModuleID) {
			return fmt.Errorf("ui provider identity does not match manifest contribution")
		}
		if contrib.RuntimeBinding != nil {
			def.Generation = contrib.RuntimeBinding.Generation
		}
		def.Enabled = true
		if err := def.Validate(); err != nil {
			return err
		}
		if err := i.container.UIProviderRegistry.Register(def); err != nil {
			return err
		}
		broadcast()
		return nil
	}
	if err := i.container.UIProviderRegistry.SetEnabled(string(contrib.ID), true); err != nil {
		return err
	}
	broadcast()
	return nil
}

func (i *TypedContributionInstaller) activateTool(ctx context.Context, contrib domain.ContributionDefinition) error {
	if i.container.ToolRegistry == nil {
		return fmt.Errorf("tool registry not configured")
	}
	defData, _ := json.Marshal(contrib.Definition)
	var def struct {
		ToolID       string          `json:"toolId"`
		ModelName    string          `json:"modelName"`
		CapabilityID string          `json:"capabilityId,omitempty"`
		InputSchema  json.RawMessage `json:"inputSchema"`
		OutputSchema json.RawMessage `json:"outputSchema"`
		RiskLevel    string          `json:"riskLevel,omitempty"`
		SideEffect   string          `json:"sideEffect,omitempty"`
		Permissions  json.RawMessage `json:"permissions,omitempty"`
		Scope        json.RawMessage `json:"scope,omitempty"`
		Internal     bool            `json:"internal,omitempty"`
	}
	_ = json.Unmarshal(defData, &def)
	toolID := def.ToolID
	if toolID == "" {
		toolID = string(contrib.ID)
	}
	modelName := def.ModelName
	if modelName == "" {
		modelName = contrib.Name.Default
	}
	capID := capability.CapabilityID(def.CapabilityID)
	if capID == "" {
		capID = capability.CapabilityID(toolID)
	}
	var perms []capability.PermissionRequirement
	if len(def.Permissions) > 0 {
		_ = json.Unmarshal(def.Permissions, &perms)
	}
	var scope capability.ScopeRule
	if len(def.Scope) > 0 {
		_ = json.Unmarshal(def.Scope, &scope)
	}
	toolSource := capability.ToolSourcePlugin
	if isSystemBuiltin(contrib.Metadata) {
		toolSource = capability.ToolSourceBuiltin
	}

	toolDef := capability.ToolDefinition{
		ID:           toolID,
		ModelName:    modelName,
		CapabilityID: capID,
		ExtensionID:  string(contrib.ExtensionID),
		ModuleID:     string(contrib.ModuleID),
		Source:       toolSource,
		Name:         contrib.Name.Default,
		Description:  contrib.Description.Default,
		Version:      contrib.Version,
		InputSchema:  def.InputSchema,
		OutputSchema: def.OutputSchema,
		Enabled:      true,
		Internal:     def.Internal,
		RiskLevel:    capability.RiskLevel(def.RiskLevel),
		SideEffect:   capability.SideEffectLevel(def.SideEffect),
		Permissions:  perms,
		Scope:        scope,
		Runtime:      i.buildRuntimeBinding(contrib),
	}
	if err := i.container.ToolRegistry.Replace(ctx, toolDef); err != nil {
		return fmt.Errorf("activate tool %s: %w", toolID, err)
	}
	return nil
}

func isSystemBuiltin(metadata map[string]any) bool {
	if metadata == nil {
		return false
	}
	v, ok := metadata["system.builtin"]
	if !ok {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	if s, ok := v.(string); ok {
		return s == "true"
	}
	return false
}

func (i *TypedContributionInstaller) activateEventSubscription(ctx context.Context, contrib domain.ContributionDefinition) error {
	defData, _ := json.Marshal(contrib.Definition)
	var def event.EventSubscriptionDefinition
	if err := json.Unmarshal(defData, &def); err != nil {
		return fmt.Errorf("unmarshal event subscription for activate: %w", err)
	}
	if def.ContributionID == "" {
		def.ContributionID = string(contrib.ID)
	}
	if def.ExtensionID == "" {
		def.ExtensionID = string(contrib.ExtensionID)
	}
	if def.ModuleID == "" {
		def.ModuleID = string(contrib.ModuleID)
	}
	def.Enabled = true
	if i.container.EventService == nil {
		return fmt.Errorf("event service not configured")
	}
	if err := i.container.EventService.RegisterSubscription(ctx, def); err != nil {
		return fmt.Errorf("activate event subscription %s: %w", def.ContributionID, err)
	}
	return nil
}

func (i *TypedContributionInstaller) activateHook(ctx context.Context, contrib domain.ContributionDefinition) error {
	if i.container.HookService == nil || i.container.HookService.Lifecycle == nil {
		return fmt.Errorf("hook service not configured")
	}
	if err := i.container.HookService.Lifecycle.EnableContribution(ctx, string(contrib.ID)); err != nil {
		return fmt.Errorf("activate hook %s: %w", contrib.ID, err)
	}
	return nil
}

func (i *TypedContributionInstaller) activateSchedule(ctx context.Context, contrib domain.ContributionDefinition) error {
	if i.container.ScheduleService == nil {
		return fmt.Errorf("schedule service not configured")
	}
	defData, _ := json.Marshal(contrib.Definition)
	var def schedule.ScheduleContributionDefinition
	if err := json.Unmarshal(defData, &def); err != nil {
		return fmt.Errorf("unmarshal schedule for activate: %w", err)
	}
	if def.ScheduleID == "" {
		return fmt.Errorf("schedule contribution %s missing scheduleId", contrib.ID)
	}
	state, err := i.container.ScheduleService.GetScheduleState(ctx, def.ScheduleID)
	if err != nil {
		return fmt.Errorf("get schedule state %s: %w", def.ScheduleID, err)
	}
	if state == nil {
		return fmt.Errorf("schedule state %s is nil", def.ScheduleID)
	}
	if err := i.container.ScheduleService.Enable(ctx, def.ScheduleID, state.Generation); err != nil {
		return fmt.Errorf("activate schedule %s: %w", def.ScheduleID, err)
	}
	return nil
}

func (i *TypedContributionInstaller) activateAgentSkill(ctx context.Context, contrib domain.ContributionDefinition) error {
	if i.container.AgentSkillCatalog == nil {
		return fmt.Errorf("agent skill catalog not configured")
	}
	if err := i.container.AgentSkillCatalog.SetEnabled(string(contrib.ExtensionID), true); err != nil {
		return fmt.Errorf("activate agent skill %s: %w", contrib.ID, err)
	}
	return nil
}

func (i *TypedContributionInstaller) activateWorkflow(ctx context.Context, contrib domain.ContributionDefinition) error {
	if i.container.WorkflowRegistry == nil {
		return fmt.Errorf("workflow registry not configured")
	}
	defData, _ := json.Marshal(contrib.Definition)
	var def workflow.WorkflowDefinition
	if err := json.Unmarshal(defData, &def); err != nil {
		return fmt.Errorf("unmarshal workflow for activate: %w", err)
	}
	if def.ID == "" {
		def.ID = string(contrib.ID)
	}
	def.Enabled = true
	if err := i.container.WorkflowRegistry.SetEnabled(def.ID, true); err != nil {
		return fmt.Errorf("activate workflow %s: %w", def.ID, err)
	}
	return nil
}

func (i *TypedContributionInstaller) activateUI(ctx context.Context, contrib domain.ContributionDefinition) error {
	defData, _ := json.Marshal(contrib.Definition)
	var uiDef ui_contribution.UIContributionDefinition
	if err := json.Unmarshal(defData, &uiDef); err != nil {
		return fmt.Errorf("unmarshal ui contribution for activate: %w", err)
	}
	if uiDef.ContributionID == "" {
		uiDef.ContributionID = ui_contribution.ContributionID(contrib.ID)
	}
	if uiDef.ExtensionID == "" {
		uiDef.ExtensionID = ui_contribution.ExtensionID(contrib.ExtensionID)
	}
	if uiDef.ModuleID == "" {
		uiDef.ModuleID = ui_contribution.ModuleID(contrib.ModuleID)
	}
	if i.container.UIHost != nil {
		if err := i.container.UIHost.Mount(uiDef.ContributionID); err != nil {
			return fmt.Errorf("mount ui contribution %s: %w", uiDef.ContributionID, err)
		}
	}
	if contrib.Kind == domain.ContributionKindUIDesktop && i.container.DesktopHost != nil {
		i.container.DesktopHost.EnableExtension(ctx, string(contrib.ExtensionID))
	}
	return nil
}

func (i *TypedContributionInstaller) DeactivateContributions(ctx context.Context, extID domain.ExtensionID) error {
	if i.container == nil {
		return fmt.Errorf("contribution-installer: container not attached")
	}
	contribs, err := i.container.ContributionRepository.ListContributions(ctx, extID)
	if err != nil {
		return fmt.Errorf("contribution-installer: list contributions for deactivate %s: %w", extID, err)
	}

	operationID := newOperationID("deactivate")
	generation := int64(0)
	if inst, instErr := i.container.InstallationRepository.GetInstallation(ctx, extID); instErr == nil {
		generation = inst.Generation
	}

	seen := make(map[domain.ContributionID]bool, len(contribs))
	var firstErr error
	for _, contrib := range contribs {
		if seen[contrib.ID] {
			continue
		}
		seen[contrib.ID] = true

		startedAt := time.Now().UTC()
		if err := i.deactivateSingle(ctx, contrib); err != nil {
			i.recordAudit(contrib, operationID, generation, startedAt, auditResultFailed, err)
			if firstErr == nil {
				firstErr = fmt.Errorf("contribution-installer: deactivate %s (%s): %w", contrib.ID, contrib.Kind, err)
			}
			continue
		}
		i.recordAudit(contrib, operationID, generation, startedAt, auditResultSucceeded, nil)
	}

	return firstErr
}

func (i *TypedContributionInstaller) deactivateSingle(ctx context.Context, contrib domain.ContributionDefinition) error {
	switch contrib.Kind {
	case domain.ContributionKindTool:
		return i.deactivateTool(ctx, contrib)
	case domain.ContributionKindEventSubscription:
		return i.deactivateEventSubscription(ctx, contrib)
	case domain.ContributionKindHook:
		return i.deactivateHook(ctx, contrib)
	case domain.ContributionKindSchedule:
		return i.deactivateSchedule(ctx, contrib)
	case domain.ContributionKindAgentSkill:
		return i.deactivateAgentSkill(ctx, contrib)
	case domain.ContributionKindWorkflow:
		return i.deactivateWorkflow(ctx, contrib)
	case domain.ContributionKindUIPage, domain.ContributionKindUIPanel, domain.ContributionKindUIChat, domain.ContributionKindUIContextAction, domain.ContributionKindUIDesktop:
		return i.deactivateUI(ctx, contrib)
	case domain.ContributionKindUIProvider:
		return i.deactivateUIProvider(ctx, contrib)
	}
	return nil
}

func (i *TypedContributionInstaller) deactivateUIProvider(ctx context.Context, contrib domain.ContributionDefinition) error {
	if i.container.UIProviderRegistry == nil {
		return nil
	}
	if _, ok := i.container.UIProviderRegistry.Get(string(contrib.ID)); !ok {
		return nil
	}
	if err := i.container.UIProviderRegistry.SetEnabled(string(contrib.ID), false); err != nil {
		return err
	}
	if i.container.UIHostNotifier != nil {
		i.container.UIHostNotifier.BroadcastExtensionChange("ui_provider_changed", string(contrib.ExtensionID), map[string]interface{}{
			"providerId": string(contrib.ID),
			"enabled":    false,
		})
	}
	return nil
}

func (i *TypedContributionInstaller) deactivateTool(ctx context.Context, contrib domain.ContributionDefinition) error {
	if i.container.ToolRegistry == nil {
		return fmt.Errorf("tool registry not configured")
	}
	defData, _ := json.Marshal(contrib.Definition)
	var def struct {
		ToolID string `json:"toolId"`
	}
	_ = json.Unmarshal(defData, &def)
	toolID := def.ToolID
	if toolID == "" {
		toolID = string(contrib.ID)
	}
	if err := i.container.ToolRegistry.Unregister(ctx, toolID); err != nil {
		return fmt.Errorf("deactivate tool %s: %w", toolID, err)
	}
	return nil
}

func (i *TypedContributionInstaller) deactivateEventSubscription(ctx context.Context, contrib domain.ContributionDefinition) error {
	if i.container.EventService == nil {
		return fmt.Errorf("event service not configured")
	}
	if err := i.container.EventService.UnregisterSubscription(ctx, string(contrib.ID)); err != nil {
		return fmt.Errorf("deactivate event subscription %s: %w", contrib.ID, err)
	}
	return nil
}

func (i *TypedContributionInstaller) deactivateHook(ctx context.Context, contrib domain.ContributionDefinition) error {
	if i.container.HookService == nil || i.container.HookService.Lifecycle == nil {
		return fmt.Errorf("hook service not configured")
	}
	if err := i.container.HookService.Lifecycle.DisableContribution(ctx, string(contrib.ID)); err != nil {
		return fmt.Errorf("deactivate hook %s: %w", contrib.ID, err)
	}
	return nil
}

func (i *TypedContributionInstaller) deactivateSchedule(ctx context.Context, contrib domain.ContributionDefinition) error {
	if i.container.ScheduleService == nil {
		return fmt.Errorf("schedule service not configured")
	}
	defData, _ := json.Marshal(contrib.Definition)
	var def schedule.ScheduleContributionDefinition
	if err := json.Unmarshal(defData, &def); err != nil {
		return fmt.Errorf("unmarshal schedule for deactivate: %w", err)
	}
	if def.ScheduleID == "" {
		return fmt.Errorf("schedule contribution %s missing scheduleId", contrib.ID)
	}
	state, err := i.container.ScheduleService.GetScheduleState(ctx, def.ScheduleID)
	if err != nil {
		return fmt.Errorf("get schedule state %s: %w", def.ScheduleID, err)
	}
	if state == nil {
		return fmt.Errorf("schedule state %s is nil", def.ScheduleID)
	}
	if err := i.container.ScheduleService.Disable(ctx, def.ScheduleID, state.Generation); err != nil {
		return fmt.Errorf("deactivate schedule %s: %w", def.ScheduleID, err)
	}
	return nil
}

func (i *TypedContributionInstaller) deactivateAgentSkill(ctx context.Context, contrib domain.ContributionDefinition) error {
	if i.container.AgentSkillCatalog == nil {
		return fmt.Errorf("agent skill catalog not configured")
	}
	if err := i.container.AgentSkillCatalog.SetEnabled(string(contrib.ExtensionID), false); err != nil {
		return fmt.Errorf("deactivate agent skill %s: %w", contrib.ID, err)
	}
	return nil
}

func (i *TypedContributionInstaller) deactivateWorkflow(ctx context.Context, contrib domain.ContributionDefinition) error {
	if i.container.WorkflowRegistry == nil {
		return fmt.Errorf("workflow registry not configured")
	}
	defData, _ := json.Marshal(contrib.Definition)
	var def workflow.WorkflowDefinition
	if err := json.Unmarshal(defData, &def); err != nil {
		return fmt.Errorf("unmarshal workflow for deactivate: %w", err)
	}
	if def.ID == "" {
		def.ID = string(contrib.ID)
	}
	if err := i.container.WorkflowRegistry.SetEnabled(def.ID, false); err != nil {
		return fmt.Errorf("deactivate workflow %s: %w", def.ID, err)
	}
	return nil
}

func (i *TypedContributionInstaller) deactivateUI(ctx context.Context, contrib domain.ContributionDefinition) error {
	if i.container.UIHost != nil {
		i.container.UIHost.DisableExtension(ui_contribution.ExtensionID(contrib.ExtensionID))
	}
	if i.container.PageHost != nil {
		i.container.PageHost.HandleExtensionDisabled(ctx, extension_page_host.ExtensionID(contrib.ExtensionID))
	}
	if i.container.DesktopHost != nil {
		i.container.DesktopHost.DisableExtension(ctx, string(contrib.ExtensionID))
	}
	return nil
}

func resolveExtensionBundlePath(extRoot, extensionID string) string {
	if extRoot == "" || extensionID == "" {
		return ""
	}
	safeID := strings.NewReplacer("/", "__", "\\", "__", ":", "_", "..", "_").Replace(extensionID)
	installedRoot := filepath.Join(extRoot, "installed", safeID)
	entries, err := os.ReadDir(installedRoot)
	if err != nil {
		return ""
	}
	for i := len(entries) - 1; i >= 0; i-- {
		if !entries[i].IsDir() {
			continue
		}
		versionDir := filepath.Join(installedRoot, entries[i].Name())
		subEntries, err := os.ReadDir(versionDir)
		if err != nil {
			continue
		}
		for _, sub := range subEntries {
			if !sub.IsDir() {
				continue
			}
			candidate := filepath.Join(versionDir, sub.Name())
			if _, err := os.Stat(filepath.Join(candidate, "manifest.json")); err == nil {
				return candidate
			}
		}
	}
	return ""
}

func (i *TypedContributionInstaller) UninstallContributions(ctx context.Context, extID domain.ExtensionID) error {
	if i.container == nil {
		return fmt.Errorf("contribution-installer: container not attached")
	}

	operationID := newOperationID("uninstall")
	generation := int64(0)
	if inst, instErr := i.container.InstallationRepository.GetInstallation(ctx, extID); instErr == nil {
		generation = inst.Generation
	}
	startedAt := time.Now().UTC()

	if i.container.EventService != nil {
		if err := i.container.EventService.RemoveSubscriptionsByExtension(ctx, string(extID)); err != nil {
			i.recordAudit(domain.ContributionDefinition{ID: "event_subscriptions", Kind: domain.ContributionKindEventSubscription, ExtensionID: extID}, operationID, generation, startedAt, auditResultFailed, err)
			return fmt.Errorf("uninstall event subscriptions for %s: %w", extID, err)
		}
	}
	if i.container.HookService != nil && i.container.HookService.Lifecycle != nil {
		if err := i.container.HookService.Lifecycle.UninstallByExtension(ctx, string(extID)); err != nil {
			i.recordAudit(domain.ContributionDefinition{ID: "hooks", Kind: domain.ContributionKindHook, ExtensionID: extID}, operationID, generation, startedAt, auditResultFailed, err)
			return fmt.Errorf("uninstall hooks for %s: %w", extID, err)
		}
	}
	if i.container.ScheduleService != nil {
		if err := i.container.ScheduleService.DeleteAllByExtension(ctx, string(extID)); err != nil {
			i.recordAudit(domain.ContributionDefinition{ID: "schedules", Kind: domain.ContributionKindSchedule, ExtensionID: extID}, operationID, generation, startedAt, auditResultFailed, err)
			return fmt.Errorf("uninstall schedules for %s: %w", extID, err)
		}
	}
	if i.container.ToolRegistry != nil {
		if _, err := i.container.ToolRegistry.UnregisterByOwner(ctx, string(extID)); err != nil {
			i.recordAudit(domain.ContributionDefinition{ID: "tools", Kind: domain.ContributionKindTool, ExtensionID: extID}, operationID, generation, startedAt, auditResultFailed, err)
			return fmt.Errorf("uninstall tools for %s: %w", extID, err)
		}
	}
	if i.container.AgentSkillCatalog != nil {
		if err := i.container.AgentSkillCatalog.Unregister(string(extID)); err != nil {
			i.recordAudit(domain.ContributionDefinition{ID: "agent_skills", Kind: domain.ContributionKindAgentSkill, ExtensionID: extID}, operationID, generation, startedAt, auditResultFailed, err)
			return fmt.Errorf("uninstall agent skills for %s: %w", extID, err)
		}
	}
	if i.container.WorkflowRegistry != nil {
		contribs, _ := i.container.ContributionRepository.ListContributions(ctx, extID)
		for _, contrib := range contribs {
			if contrib.Kind != domain.ContributionKindWorkflow {
				continue
			}
			defData, _ := json.Marshal(contrib.Definition)
			var def workflow.WorkflowDefinition
			if json.Unmarshal(defData, &def) == nil {
				wfID := def.ID
				if wfID == "" {
					wfID = string(contrib.ID)
				}
				if err := i.container.WorkflowRegistry.Unregister(wfID); err != nil {
					i.recordAudit(contrib, operationID, generation, startedAt, auditResultFailed, err)
					return fmt.Errorf("uninstall workflow %s for %s: %w", wfID, extID, err)
				}
			}
		}
	}
	if i.container.UIHost != nil {
		for _, uiDef := range i.container.UIHost.ListAll() {
			if string(uiDef.ExtensionID) == string(extID) {
				if err := i.container.UIHost.UnregisterContribution(uiDef.ContributionID); err != nil {
					i.recordAudit(domain.ContributionDefinition{ID: domain.ContributionID(uiDef.ContributionID), Kind: domain.ContributionKindUIPage, ExtensionID: extID}, operationID, generation, startedAt, auditResultFailed, err)
					return fmt.Errorf("uninstall ui contribution %s for %s: %w", uiDef.ContributionID, extID, err)
				}
			}
		}
	}
	if i.container.PageHost != nil {
		if _, err := i.container.PageHost.HandleExtensionUninstalled(ctx, extension_page_host.ExtensionID(extID)); err != nil {
			i.recordAudit(domain.ContributionDefinition{ID: "pages", Kind: domain.ContributionKindUIPage, ExtensionID: extID}, operationID, generation, startedAt, auditResultFailed, err)
			return fmt.Errorf("uninstall pages for %s: %w", extID, err)
		}
	}
	if i.container.UIContributionRepo != nil {
		if err := i.container.UIContributionRepo.DeleteByExtension(ctx, string(extID)); err != nil {
			i.recordAudit(domain.ContributionDefinition{ID: "ui_contributions", Kind: domain.ContributionKindUIPage, ExtensionID: extID}, operationID, generation, startedAt, auditResultFailed, err)
			return fmt.Errorf("delete ui contribution records for %s: %w", extID, err)
		}
	}
	if i.container.SchemaRegistry != nil {
		if _, err := i.container.SchemaRegistry.Unregister(string(extID)); err != nil {
			i.recordAudit(domain.ContributionDefinition{ID: "schemas", Kind: domain.ContributionKindUIPage, ExtensionID: extID}, operationID, generation, startedAt, auditResultFailed, err)
			return fmt.Errorf("uninstall schemas for %s: %w", extID, err)
		}
	}
	if i.container.DesktopHost != nil {
		if err := i.container.DesktopHost.UninstallContributions(ctx, string(extID)); err != nil {
			i.recordAudit(domain.ContributionDefinition{ID: "desktop", Kind: domain.ContributionKindUIDesktop, ExtensionID: extID}, operationID, generation, startedAt, auditResultFailed, err)
			return fmt.Errorf("uninstall desktop contributions for %s: %w", extID, err)
		}
	}
	if i.container.TaskRuntimeService != nil {
		if err := i.container.TaskRuntimeService.DeleteByExtension(ctx, string(extID)); err != nil {
			i.recordAudit(domain.ContributionDefinition{ID: "background_tasks", Kind: domain.ContributionKindBackgroundTask, ExtensionID: extID}, operationID, generation, startedAt, auditResultFailed, err)
			return fmt.Errorf("uninstall background tasks for %s: %w", extID, err)
		}
	}

	i.recordAudit(domain.ContributionDefinition{ID: "all", ExtensionID: extID}, operationID, generation, startedAt, auditResultSucceeded, nil)
	return nil
}

func (i *TypedContributionInstaller) RepairContributions(ctx context.Context, extID domain.ExtensionID, generation int64) error {
	if i.container == nil {
		return fmt.Errorf("contribution-installer: container not attached")
	}
	contribs, err := i.container.ContributionRepository.ListContributions(ctx, extID)
	if err != nil {
		return fmt.Errorf("contribution-installer: list contributions for repair %s: %w", extID, err)
	}
	if err := i.UninstallContributions(ctx, extID); err != nil {
		return fmt.Errorf("contribution-installer: repair uninstall failed %s: %w", extID, err)
	}
	if err := i.InstallContributions(ctx, contribs, generation); err != nil {
		return fmt.Errorf("contribution-installer: repair install failed %s: %w", extID, err)
	}
	enabled := false
	if inst, err := i.container.InstallationRepository.GetInstallation(ctx, extID); err == nil {
		enabled = inst.EnablementState == domain.EnablementEnabled
	}
	if enabled {
		if err := i.ActivateContributions(ctx, extID); err != nil {
			return fmt.Errorf("contribution-installer: repair activate failed %s: %w", extID, err)
		}
	}
	return nil
}

func (i *TypedContributionInstaller) RecoverContributions(ctx context.Context, extID domain.ExtensionID) error {
	if i.container == nil {
		return fmt.Errorf("contribution-installer: container not attached")
	}

	inst, err := i.container.InstallationRepository.GetInstallation(ctx, extID)
	if err != nil {
		return fmt.Errorf("contribution-installer: get installation for recover %s: %w", extID, err)
	}

	operationID := newOperationID("recover")
	generation := inst.Generation
	startedAt := time.Now().UTC()

	if err := i.recoverInMemoryRegistrations(ctx, extID, generation); err != nil {
		i.recordAudit(domain.ContributionDefinition{ID: "recover_register", ExtensionID: extID}, operationID, generation, startedAt, auditResultFailed, err)
		return fmt.Errorf("contribution-installer: recover in-memory registrations failed %s: %w", extID, err)
	}

	switch inst.EnablementState {
	case domain.EnablementEnabled:
		if err := i.ActivateContributions(ctx, extID); err != nil {
			i.recordAudit(domain.ContributionDefinition{ID: "recover_activate", ExtensionID: extID}, operationID, generation, startedAt, auditResultFailed, err)
			return fmt.Errorf("contribution-installer: recover activate failed %s: %w", extID, err)
		}
	case domain.EnablementDisabled, domain.EnablementPartiallyDisabled, domain.EnablementRequiresRecovery:
		if err := i.DeactivateContributions(ctx, extID); err != nil {
			i.recordAudit(domain.ContributionDefinition{ID: "recover_deactivate", ExtensionID: extID}, operationID, generation, startedAt, auditResultFailed, err)
			return fmt.Errorf("contribution-installer: recover deactivate failed %s: %w", extID, err)
		}
	}

	i.recordAudit(domain.ContributionDefinition{ID: "recover", ExtensionID: extID}, operationID, generation, startedAt, auditResultSucceeded, nil)
	return nil
}

func (i *TypedContributionInstaller) recoverInMemoryRegistrations(ctx context.Context, extID domain.ExtensionID, generation int64) error {
	if i.container.UIContributionRepo != nil && i.container.UIHost != nil {
		uiDefs, err := i.container.UIContributionRepo.ListByExtension(ctx, string(extID))
		if err != nil {
			return fmt.Errorf("list ui contributions for recover: %w", err)
		}
		for _, uiDef := range uiDefs {
			_ = i.container.UIHost.RegisterContribution(uiDef)
			if i.container.PageHost != nil && (uiDef.Kind == ui_contribution.UIContributionWebPage || uiDef.Kind == ui_contribution.UIContributionSchemaPage) {
				entryKind := extension_page_host.PageKindWeb
				if uiDef.Kind == ui_contribution.UIContributionSchemaPage {
					entryKind = extension_page_host.PageKindSchema
				}
				perms := make([]string, 0, len(uiDef.Permissions))
				for _, p := range uiDef.Permissions {
					perms = append(perms, p.Name)
				}
				pageDef := extension_page_host.NewExtensionPageDefinition(extension_page_host.PageRegistrationInput{
					PageID:          extension_page_host.PageID(uiDef.ContributionID),
					ExtensionID:     extension_page_host.ExtensionID(uiDef.ExtensionID),
					ModuleID:        string(uiDef.ModuleID),
					ContributionID:  extension_page_host.ContributionID(uiDef.ContributionID),
					Generation:      generation,
					ContractVersion: uiDef.ContractVersion,
					EntryKind:       entryKind,
					EntryPath:       uiDef.Entry.Path,
					SchemaPath:      uiDef.Entry.SchemaPath,
					Title: extension_page_host.LocalizedText{
						Default:      uiDef.Display.Title.Default,
						Translations: uiDef.Display.Title.I18n,
					},
					Description: extension_page_host.LocalizedText{
						Default:      uiDef.Display.Description.Default,
						Translations: uiDef.Display.Description.I18n,
					},
					Icon:        uiDef.Display.Icon,
					Permissions: perms,
				})
				_ = i.container.PageHost.RegisterPage(ctx, pageDef)
			}
			if i.container.SchemaRegistry != nil && uiDef.Entry.SchemaPath != "" {
				basePath := resolveExtensionBundlePath(i.container.ExtRoot, string(extID))
				if basePath != "" {
					_ = i.container.SchemaRegistry.LoadFromPathWithContext(string(uiDef.ExtensionID), string(uiDef.ContributionID), generation, "", "", basePath, uiDef.Entry.SchemaPath)
				}
			}
		}
	}

	contribs, err := i.container.ContributionRepository.ListContributions(ctx, extID)
	if err != nil {
		return fmt.Errorf("list contributions for recover: %w", err)
	}
	for _, contrib := range contribs {
		switch contrib.Kind {
		case domain.ContributionKindAgentSkill:
			if i.container.AgentSkillCatalog != nil {
				defData, _ := json.Marshal(contrib.Definition)
				var def agent_skill.AgentSkillDefinition
				if err := json.Unmarshal(defData, &def); err == nil {
					if def.ID == "" {
						def.ID = string(contrib.ID)
					}
					if def.ExtensionID == "" {
						def.ExtensionID = string(contrib.ExtensionID)
					}
					if def.ModuleID == "" {
						def.ModuleID = string(contrib.ModuleID)
					}
					_ = i.container.AgentSkillCatalog.Register(def)
				}
			}
		case domain.ContributionKindUIDesktop:
			if i.container.DesktopHost != nil {
				defData, _ := json.Marshal(contrib.Definition)
				var desktopDef desktop.DesktopContributionDefinition
				if err := json.Unmarshal(defData, &desktopDef); err == nil {
					if desktopDef.ContributionID == "" {
						desktopDef.ContributionID = string(contrib.ID)
					}
					if desktopDef.ExtensionID == "" {
						desktopDef.ExtensionID = string(contrib.ExtensionID)
					}
					if desktopDef.ModuleID == "" {
						desktopDef.ModuleID = string(contrib.ModuleID)
					}
					if desktopDef.Version == "" {
						desktopDef.Version = contrib.Version
					}
					_, _ = i.container.DesktopHost.RegisterContribution(ctx, desktopDef)
				}
			}
		}
	}

	return nil
}

func (i *TypedContributionInstaller) StopRuntimeInstances(ctx context.Context, extID domain.ExtensionID) error {
	if i.container == nil || i.container.RuntimeSupervisor == nil {
		return nil
	}
	modules, err := i.container.ModuleRepository.ListModules(ctx, extID)
	if err != nil {
		return fmt.Errorf("list modules for stop runtime: %w", err)
	}
	var stopErrs []error
	for _, mod := range modules {
		if mod.Runtime != nil && mod.Runtime.Type != "" && mod.Runtime.Type != domain.RuntimeTypeBuiltin {
			defID := runtime_supervisor.BuildRuntimeDefinitionID(string(extID), string(mod.ID), mod.Runtime.Type)
			snap := i.container.RuntimeSupervisor.Snapshot(ctx, defID)
			for _, instance := range snap.Instances {
				if stopErr := i.container.RuntimeSupervisor.Stop(ctx, instance.InstanceID, runtime_supervisor.StopReasonUninstall); stopErr != nil {
					i.container.recordReconcileFailure(ctx, extID, runtime_supervisor.ReconcileResult{
						DefinitionID: defID,
						Desired:      runtime_supervisor.DesiredStopped,
						Actual:       runtime_supervisor.ActualFailed,
						Error:        stopErr,
					})
					stopErrs = append(stopErrs, fmt.Errorf("stop runtime instance %s: %w", instance.InstanceID, stopErr))
				}
			}
		}
	}
	if len(stopErrs) > 0 {
		return fmt.Errorf("stop runtime instances for %s failed with %d error(s): %v", extID, len(stopErrs), stopErrs)
	}
	return nil
}

func (i *TypedContributionInstaller) ListScheduleIDs(ctx context.Context, extID domain.ExtensionID) ([]string, error) {
	if i.container == nil || i.container.ScheduleService == nil {
		return nil, nil
	}
	schedules, err := i.container.ScheduleService.ListSchedules(ctx, string(extID))
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(schedules))
	for _, s := range schedules {
		ids = append(ids, s.ScheduleID)
	}
	return ids, nil
}

func (i *TypedContributionInstaller) DiscardCandidateContributions(ctx context.Context, extID domain.ExtensionID, generation int64, contribs []domain.ContributionDefinition, scheduleIDs []string) error {
	if i.container == nil {
		return fmt.Errorf("contribution-installer: container not attached")
	}

	operationID := newOperationID("discard-candidate")
	startedAt := time.Now().UTC()
	var firstErr error

	scheduleIDSet := make(map[string]bool, len(scheduleIDs))
	for _, id := range scheduleIDs {
		scheduleIDSet[id] = true
	}

	for _, contrib := range contribs {
		if err := i.discardSingleContribution(ctx, contrib, scheduleIDSet); err != nil {
			i.recordAudit(contrib, operationID, generation, startedAt, auditResultFailed, err)
			if firstErr == nil {
				firstErr = fmt.Errorf("discard candidate contribution %s: %w", contrib.ID, err)
			}
		}
	}

	if i.container.OperationRepository != nil {
		now := time.Now().UTC()
		op := sqlite.Operation{
			OperationID:   operationID,
			OperationType: "discard_candidate",
			ExtensionID:   extID,
			Status:        "succeeded",
			StartedAt:     startedAt,
			FinishedAt:    &now,
		}
		if firstErr != nil {
			op.Status = "failed"
			op.ErrorMessage = firstErr.Error()
		}
		_ = i.container.OperationRepository.PutOperation(ctx, op)
	}

	result := auditResultSucceeded
	if firstErr != nil {
		result = auditResultFailed
	}
	i.recordAudit(domain.ContributionDefinition{ID: "candidate_discard", ExtensionID: extID}, operationID, generation, startedAt, result, firstErr)
	return firstErr
}

func (i *TypedContributionInstaller) discardSingleContribution(ctx context.Context, contrib domain.ContributionDefinition, scheduleIDSet map[string]bool) error {
	if i.container == nil {
		return nil
	}

	defData, err := json.Marshal(contrib.Definition)
	if err != nil {
		return fmt.Errorf("marshal definition: %w", err)
	}

	switch contrib.Kind {
	case domain.ContributionKindTool:
		return i.discardTool(ctx, contrib, defData)
	case domain.ContributionKindEventSubscription:
		return i.discardEventSubscription(ctx, contrib, defData)
	case domain.ContributionKindHook:
		return i.discardHook(ctx, contrib, defData)
	case domain.ContributionKindSchedule:
		return i.discardSchedule(ctx, contrib, defData, scheduleIDSet)
	case domain.ContributionKindAgentSkill:
		return nil
	case domain.ContributionKindWorkflow:
		return i.discardWorkflow(ctx, contrib, defData)
	case domain.ContributionKindBackgroundTask:
		return i.discardTaskDefinition(ctx, contrib, defData)
	case domain.ContributionKindMCPServer:
		return i.discardMCPServer(ctx, contrib, defData)
	case domain.ContributionKindUIPage, domain.ContributionKindUIPanel, domain.ContributionKindUIChat, domain.ContributionKindUIContextAction:
		return i.discardUIContribution(ctx, contrib, defData)
	case domain.ContributionKindUIDesktop:
		return i.discardDesktopContribution(ctx, contrib, defData)
	case domain.ContributionKindUIProvider:
		return i.discardUIProvider(ctx, contrib, defData)
	default:
		return nil
	}
}

func (i *TypedContributionInstaller) discardUIProvider(ctx context.Context, contrib domain.ContributionDefinition, defData []byte) error {
	if i.container.UIProviderRegistry == nil {
		return nil
	}
	i.container.UIProviderRegistry.Unregister(string(contrib.ID))
	if i.container.UIHostNotifier != nil {
		i.container.UIHostNotifier.BroadcastExtensionChange("ui_provider_changed", string(contrib.ExtensionID), map[string]interface{}{
			"providerId": string(contrib.ID),
			"removed":    true,
		})
	}
	return nil
}

func (i *TypedContributionInstaller) discardTool(ctx context.Context, contrib domain.ContributionDefinition, defData []byte) error {
	if i.container.ToolRegistry == nil {
		return nil
	}
	var def struct {
		ToolID string `json:"toolId"`
	}
	_ = json.Unmarshal(defData, &def)
	toolID := def.ToolID
	if toolID == "" {
		toolID = string(contrib.ID)
	}
	_ = i.container.ToolRegistry.Unregister(ctx, toolID)
	return nil
}

func (i *TypedContributionInstaller) discardEventSubscription(ctx context.Context, contrib domain.ContributionDefinition, defData []byte) error {
	if i.container.EventService == nil {
		return nil
	}
	var def event.EventSubscriptionDefinition
	_ = json.Unmarshal(defData, &def)
	contributionID := def.ContributionID
	if contributionID == "" {
		contributionID = string(contrib.ID)
	}
	_ = i.container.EventService.UnregisterSubscription(ctx, contributionID)
	return nil
}

func (i *TypedContributionInstaller) discardHook(ctx context.Context, contrib domain.ContributionDefinition, defData []byte) error {
	if i.container.HookService == nil || i.container.HookService.Lifecycle == nil {
		return nil
	}
	var def hook.HookContributionDefinition
	_ = json.Unmarshal(defData, &def)
	contributionID := def.ContributionID
	if contributionID == "" {
		contributionID = string(contrib.ID)
	}
	_ = i.container.HookService.Lifecycle.UninstallContribution(ctx, contributionID)
	return nil
}

func (i *TypedContributionInstaller) discardSchedule(ctx context.Context, contrib domain.ContributionDefinition, defData []byte, scheduleIDSet map[string]bool) error {
	if i.container.ScheduleService == nil {
		return nil
	}
	var def schedule.ScheduleContributionDefinition
	_ = json.Unmarshal(defData, &def)
	if def.ScheduleID != "" {
		_ = i.container.ScheduleService.Uninstall(ctx, def.ScheduleID)
		return nil
	}
	schedules, err := i.container.ScheduleService.ListSchedules(ctx, string(contrib.ExtensionID))
	if err != nil {
		return nil
	}
	contributionID := def.ContributionID
	if contributionID == "" {
		contributionID = string(contrib.ID)
	}
	for _, s := range schedules {
		if s.ContributionID != contributionID {
			continue
		}
		if len(scheduleIDSet) > 0 && !scheduleIDSet[s.ScheduleID] {
			continue
		}
		_ = i.container.ScheduleService.Uninstall(ctx, s.ScheduleID)
	}
	return nil
}

func (i *TypedContributionInstaller) discardWorkflow(ctx context.Context, contrib domain.ContributionDefinition, defData []byte) error {
	if i.container.WorkflowRegistry == nil {
		return nil
	}
	var def workflow.WorkflowDefinition
	_ = json.Unmarshal(defData, &def)
	wfID := def.ID
	if wfID == "" {
		wfID = string(contrib.ID)
	}
	_ = i.container.WorkflowRegistry.Unregister(wfID)
	return nil
}

func (i *TypedContributionInstaller) discardTaskDefinition(ctx context.Context, contrib domain.ContributionDefinition, defData []byte) error {
	if i.container.TaskRuntimeService == nil {
		return nil
	}
	var def task_runtime.TaskDefinition
	_ = json.Unmarshal(defData, &def)
	taskID := def.TaskID
	if taskID == "" {
		taskID = string(contrib.ID)
	}
	_ = i.container.TaskRuntimeService.DeleteTaskDefinition(ctx, taskID)
	return nil
}

func (i *TypedContributionInstaller) discardMCPServer(ctx context.Context, contrib domain.ContributionDefinition, defData []byte) error {
	mcpToolAdapter := capability.GetGlobalMCPToolAdapter()
	if mcpToolAdapter == nil {
		return nil
	}
	var def struct {
		ServerName string `json:"serverName"`
	}
	_ = json.Unmarshal(defData, &def)
	serverID := def.ServerName
	if serverID == "" {
		serverID = string(contrib.ID)
	}
	_ = mcpToolAdapter.UnregisterServer(ctx, serverID)
	return nil
}

func (i *TypedContributionInstaller) discardUIContribution(ctx context.Context, contrib domain.ContributionDefinition, defData []byte) error {
	var uiDef ui_contribution.UIContributionDefinition
	_ = json.Unmarshal(defData, &uiDef)
	contributionID := string(uiDef.ContributionID)
	if contributionID == "" {
		contributionID = string(contrib.ID)
	}
	if i.container.SchemaRegistry != nil {
		i.container.SchemaRegistry.UnregisterSchema(string(contrib.ExtensionID), contributionID)
	}
	if i.container.UIHost != nil {
		_ = i.container.UIHost.UnregisterContribution(ui_contribution.ContributionID(contributionID))
	}
	if i.container.UIContributionRepo != nil {
		_ = i.container.UIContributionRepo.DeleteContribution(ctx, contributionID)
	}
	if i.container.PageHost != nil {
		_ = i.container.PageHost.UnregisterPage(ctx, extension_page_host.ContributionID(contributionID))
	}
	return nil
}

func (i *TypedContributionInstaller) discardDesktopContribution(ctx context.Context, contrib domain.ContributionDefinition, defData []byte) error {
	var def desktop.DesktopContributionDefinition
	_ = json.Unmarshal(defData, &def)
	contributionID := def.ContributionID
	if contributionID == "" {
		contributionID = string(contrib.ID)
	}
	if i.container.UIHost != nil {
		_ = i.container.UIHost.UnregisterContribution(ui_contribution.ContributionID(contributionID))
	}
	if i.container.UIContributionRepo != nil {
		_ = i.container.UIContributionRepo.DeleteContribution(ctx, contributionID)
	}
	if i.container.DesktopHost != nil {
		_ = i.container.DesktopHost.UnregisterContribution(contributionID)
	}
	return nil
}

func (i *TypedContributionInstaller) String() string {
	return fmt.Sprintf("TypedContributionInstaller(container=%v)", i.container != nil)
}
