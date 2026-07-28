package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/agent_skill"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/desktop"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/event"
	"github.com/u-ai/backend/internal/extension/kernel/extension_page_host"
	"github.com/u-ai/backend/internal/extension/kernel/hook"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/schedule"
	"github.com/u-ai/backend/internal/extension/kernel/schema_ui"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/extension/kernel/ui_contribution"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
)

type TypedContributionInstaller struct {
	container *Container
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

type installOp struct {
	kind       domain.ContributionKind
	doInstall  func(ctx context.Context) error
	doRollback func(ctx context.Context)
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
	case domain.ContributionKindBackgroundService:
		return i.buildTaskDefinitionOp(ctx, contrib, defData)
	case domain.ContributionKindMCPServer:
		return i.buildMCPServerOp(ctx, contrib, defData)
	case domain.ContributionKindUIPage, domain.ContributionKindUIPanel, domain.ContributionKindUIChat, domain.ContributionKindUIContextAction, domain.ContributionKindUIDesktop:
		return i.buildUIContributionOp(ctx, contrib, defData, generation)
	default:
		return installOp{}, fmt.Errorf("unsupported contribution kind: %s", contrib.Kind)
	}
}

func (i *TypedContributionInstaller) buildToolOp(ctx context.Context, contrib domain.ContributionDefinition, defData []byte, generation int64) (installOp, error) {
	if i.container.ToolRegistry == nil {
		return installOp{}, fmt.Errorf("tool registry not configured")
	}
	var def struct {
		ToolID       string          `json:"toolId"`
		ModelName    string          `json:"modelName"`
		InputSchema  json.RawMessage `json:"inputSchema"`
		OutputSchema json.RawMessage `json:"outputSchema"`
		RiskLevel    string          `json:"riskLevel,omitempty"`
		SideEffect   string          `json:"sideEffect,omitempty"`
		Permissions  json.RawMessage `json:"permissions,omitempty"`
		Scope        json.RawMessage `json:"scope,omitempty"`
	}
	if err := json.Unmarshal(defData, &def); err != nil {
		return installOp{}, fmt.Errorf("unmarshal tool definition: %w", err)
	}

	toolID := def.ToolID
	if toolID == "" {
		toolID = string(contrib.ID)
	}
	modelName := def.ModelName
	if modelName == "" {
		modelName = contrib.Name.Default
	}

	var perms []capability.PermissionRequirement
	if len(def.Permissions) > 0 {
		_ = json.Unmarshal(def.Permissions, &perms)
	}
	var scope capability.ScopeRule
	if len(def.Scope) > 0 {
		_ = json.Unmarshal(def.Scope, &scope)
	}

	toolDef := capability.ToolDefinition{
		ID:           toolID,
		ModelName:    modelName,
		ExtensionID:  string(contrib.ExtensionID),
		ModuleID:     string(contrib.ModuleID),
		Source:       capability.ToolSourcePlugin,
		Name:         contrib.Name.Default,
		Description:  contrib.Description.Default,
		Version:      contrib.Version,
		InputSchema:  def.InputSchema,
		OutputSchema: def.OutputSchema,
		Enabled:      false,
		RiskLevel:    capability.RiskLevel(def.RiskLevel),
		SideEffect:   capability.SideEffectLevel(def.SideEffect),
		Permissions:  perms,
		Scope:        scope,
		Runtime:      i.buildRuntimeBinding(contrib),
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
			_ = i.container.HookService.Lifecycle.UninstallByExtension(ctx, string(contrib.ExtensionID))
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
				_ = i.container.ScheduleService.DeleteAllByExtension(ctx, string(contrib.ExtensionID))
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
		kind: domain.ContributionKindBackgroundService,
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
				_ = i.container.UIContributionRepo.DeleteByExtension(ctx, string(contrib.ExtensionID))
			}
			if hasPage && i.container.PageHost != nil {
				_, _ = i.container.PageHost.HandleExtensionUninstalled(ctx, extension_page_host.ExtensionID(contrib.ExtensionID))
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

	extID := desktopDef.ExtensionID
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
				_ = i.container.UIContributionRepo.DeleteByExtension(ctx, extID)
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
	return rb
}

func (i *TypedContributionInstaller) ActivateContributions(ctx context.Context, extID domain.ExtensionID) {
	if i.container == nil {
		return
	}
	contribs, err := i.container.ContributionRepository.ListContributions(ctx, extID)
	if err != nil {
		log.Printf("[contribution-installer] list contributions for activate %s: %v", extID, err)
		return
	}
	for _, contrib := range contribs {
		i.activateSingle(ctx, contrib)
	}
}

func (i *TypedContributionInstaller) activateSingle(ctx context.Context, contrib domain.ContributionDefinition) {
	switch contrib.Kind {
	case domain.ContributionKindTool:
		i.activateTool(ctx, contrib)
	case domain.ContributionKindEventSubscription:
		i.activateEventSubscription(ctx, contrib)
	case domain.ContributionKindHook:
		i.activateHook(ctx, contrib)
	case domain.ContributionKindSchedule:
		i.activateSchedule(ctx, contrib)
	case domain.ContributionKindAgentSkill:
		i.activateAgentSkill(ctx, contrib)
	case domain.ContributionKindWorkflow:
		i.activateWorkflow(ctx, contrib)
	case domain.ContributionKindUIPage, domain.ContributionKindUIPanel, domain.ContributionKindUIChat, domain.ContributionKindUIContextAction, domain.ContributionKindUIDesktop:
		i.activateUI(ctx, contrib)
	}
}

func (i *TypedContributionInstaller) activateTool(ctx context.Context, contrib domain.ContributionDefinition) {
	if i.container.ToolRegistry == nil {
		return
	}
	defData, _ := json.Marshal(contrib.Definition)
	var def struct {
		ToolID       string          `json:"toolId"`
		ModelName    string          `json:"modelName"`
		InputSchema  json.RawMessage `json:"inputSchema"`
		OutputSchema json.RawMessage `json:"outputSchema"`
		RiskLevel    string          `json:"riskLevel,omitempty"`
		SideEffect   string          `json:"sideEffect,omitempty"`
		Permissions  json.RawMessage `json:"permissions,omitempty"`
		Scope        json.RawMessage `json:"scope,omitempty"`
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
	var perms []capability.PermissionRequirement
	if len(def.Permissions) > 0 {
		_ = json.Unmarshal(def.Permissions, &perms)
	}
	var scope capability.ScopeRule
	if len(def.Scope) > 0 {
		_ = json.Unmarshal(def.Scope, &scope)
	}
	toolDef := capability.ToolDefinition{
		ID:           toolID,
		ModelName:    modelName,
		ExtensionID:  string(contrib.ExtensionID),
		ModuleID:     string(contrib.ModuleID),
		Source:       capability.ToolSourcePlugin,
		Name:         contrib.Name.Default,
		Description:  contrib.Description.Default,
		Version:      contrib.Version,
		InputSchema:  def.InputSchema,
		OutputSchema: def.OutputSchema,
		Enabled:      true,
		RiskLevel:    capability.RiskLevel(def.RiskLevel),
		SideEffect:   capability.SideEffectLevel(def.SideEffect),
		Permissions:  perms,
		Scope:        scope,
		Runtime:      i.buildRuntimeBinding(contrib),
	}
	if err := i.container.ToolRegistry.Replace(ctx, toolDef); err != nil {
		log.Printf("[contribution-installer] activate tool %s: %v", toolID, err)
	}
}

func (i *TypedContributionInstaller) activateEventSubscription(ctx context.Context, contrib domain.ContributionDefinition) {
	defData, _ := json.Marshal(contrib.Definition)
	var def event.EventSubscriptionDefinition
	if err := json.Unmarshal(defData, &def); err != nil {
		return
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
	if i.container.EventService != nil {
		_ = i.container.EventService.RegisterSubscription(ctx, def)
	}
}

func (i *TypedContributionInstaller) activateHook(ctx context.Context, contrib domain.ContributionDefinition) {
	if i.container.HookService == nil || i.container.HookService.Lifecycle == nil {
		return
	}
	_ = i.container.HookService.Lifecycle.EnableContribution(ctx, string(contrib.ID))
}

func (i *TypedContributionInstaller) activateSchedule(ctx context.Context, contrib domain.ContributionDefinition) {
	if i.container.ScheduleService == nil {
		return
	}
	defData, _ := json.Marshal(contrib.Definition)
	var def schedule.ScheduleContributionDefinition
	if err := json.Unmarshal(defData, &def); err != nil {
		return
	}
	if def.ScheduleID == "" {
		return
	}
	state, err := i.container.ScheduleService.GetScheduleState(ctx, def.ScheduleID)
	if err != nil || state == nil {
		return
	}
	_ = i.container.ScheduleService.Enable(ctx, def.ScheduleID, state.Generation)
}

func (i *TypedContributionInstaller) activateAgentSkill(ctx context.Context, contrib domain.ContributionDefinition) {
	if i.container.AgentSkillCatalog == nil {
		return
	}
	_ = i.container.AgentSkillCatalog.SetEnabled(string(contrib.ExtensionID), true)
}

func (i *TypedContributionInstaller) activateWorkflow(ctx context.Context, contrib domain.ContributionDefinition) {
	if i.container.WorkflowRegistry == nil {
		return
	}
	defData, _ := json.Marshal(contrib.Definition)
	var def workflow.WorkflowDefinition
	if err := json.Unmarshal(defData, &def); err != nil {
		return
	}
	if def.ID == "" {
		def.ID = string(contrib.ID)
	}
	def.Enabled = true
	_ = i.container.WorkflowRegistry.SetEnabled(def.ID, true)
}

func (i *TypedContributionInstaller) activateUI(ctx context.Context, contrib domain.ContributionDefinition) {
	defData, _ := json.Marshal(contrib.Definition)
	var uiDef ui_contribution.UIContributionDefinition
	if err := json.Unmarshal(defData, &uiDef); err != nil {
		return
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
		_ = i.container.UIHost.Mount(uiDef.ContributionID)
	}
	if contrib.Kind == domain.ContributionKindUIDesktop && i.container.DesktopHost != nil {
		i.container.DesktopHost.EnableExtension(ctx, string(contrib.ExtensionID))
	}
}

func (i *TypedContributionInstaller) DeactivateContributions(ctx context.Context, extID domain.ExtensionID) {
	if i.container == nil {
		return
	}
	contribs, err := i.container.ContributionRepository.ListContributions(ctx, extID)
	if err != nil {
		log.Printf("[contribution-installer] list contributions for deactivate %s: %v", extID, err)
		return
	}
	for _, contrib := range contribs {
		i.deactivateSingle(ctx, contrib)
	}
}

func (i *TypedContributionInstaller) deactivateSingle(ctx context.Context, contrib domain.ContributionDefinition) {
	switch contrib.Kind {
	case domain.ContributionKindTool:
		i.deactivateTool(ctx, contrib)
	case domain.ContributionKindEventSubscription:
		i.deactivateEventSubscription(ctx, contrib)
	case domain.ContributionKindHook:
		i.deactivateHook(ctx, contrib)
	case domain.ContributionKindSchedule:
		i.deactivateSchedule(ctx, contrib)
	case domain.ContributionKindAgentSkill:
		i.deactivateAgentSkill(ctx, contrib)
	case domain.ContributionKindWorkflow:
		i.deactivateWorkflow(ctx, contrib)
	case domain.ContributionKindUIPage, domain.ContributionKindUIPanel, domain.ContributionKindUIChat, domain.ContributionKindUIContextAction, domain.ContributionKindUIDesktop:
		i.deactivateUI(ctx, contrib)
	}
}

func (i *TypedContributionInstaller) deactivateTool(ctx context.Context, contrib domain.ContributionDefinition) {
	if i.container.ToolRegistry == nil {
		return
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
	_ = i.container.ToolRegistry.Unregister(ctx, toolID)
}

func (i *TypedContributionInstaller) deactivateEventSubscription(ctx context.Context, contrib domain.ContributionDefinition) {
	if i.container.EventService == nil {
		return
	}
	_ = i.container.EventService.UnregisterSubscription(ctx, string(contrib.ID))
}

func (i *TypedContributionInstaller) deactivateHook(ctx context.Context, contrib domain.ContributionDefinition) {
	if i.container.HookService == nil || i.container.HookService.Lifecycle == nil {
		return
	}
	_ = i.container.HookService.Lifecycle.DisableContribution(ctx, string(contrib.ID))
}

func (i *TypedContributionInstaller) deactivateSchedule(ctx context.Context, contrib domain.ContributionDefinition) {
	if i.container.ScheduleService == nil {
		return
	}
	defData, _ := json.Marshal(contrib.Definition)
	var def schedule.ScheduleContributionDefinition
	if err := json.Unmarshal(defData, &def); err != nil {
		return
	}
	if def.ScheduleID == "" {
		return
	}
	state, err := i.container.ScheduleService.GetScheduleState(ctx, def.ScheduleID)
	if err != nil || state == nil {
		return
	}
	_ = i.container.ScheduleService.Disable(ctx, def.ScheduleID, state.Generation)
}

func (i *TypedContributionInstaller) deactivateAgentSkill(ctx context.Context, contrib domain.ContributionDefinition) {
	if i.container.AgentSkillCatalog == nil {
		return
	}
	_ = i.container.AgentSkillCatalog.SetEnabled(string(contrib.ExtensionID), false)
}

func (i *TypedContributionInstaller) deactivateWorkflow(ctx context.Context, contrib domain.ContributionDefinition) {
	if i.container.WorkflowRegistry == nil {
		return
	}
	defData, _ := json.Marshal(contrib.Definition)
	var def workflow.WorkflowDefinition
	if err := json.Unmarshal(defData, &def); err != nil {
		return
	}
	if def.ID == "" {
		def.ID = string(contrib.ID)
	}
	_ = i.container.WorkflowRegistry.SetEnabled(def.ID, false)
}

func (i *TypedContributionInstaller) deactivateUI(ctx context.Context, contrib domain.ContributionDefinition) {
	if i.container.UIHost != nil {
		i.container.UIHost.DisableExtension(ui_contribution.ExtensionID(contrib.ExtensionID))
	}
	if i.container.PageHost != nil {
		i.container.PageHost.HandleExtensionDisabled(ctx, extension_page_host.ExtensionID(contrib.ExtensionID))
	}
	if i.container.DesktopHost != nil {
		i.container.DesktopHost.DisableExtension(ctx, string(contrib.ExtensionID))
	}
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

func (i *TypedContributionInstaller) UninstallContributions(ctx context.Context, extID domain.ExtensionID) {
	if i.container == nil {
		return
	}
	if i.container.EventService != nil {
		_ = i.container.EventService.RemoveSubscriptionsByExtension(ctx, string(extID))
	}
	if i.container.HookService != nil && i.container.HookService.Lifecycle != nil {
		_ = i.container.HookService.Lifecycle.UninstallByExtension(ctx, string(extID))
	}
	if i.container.ScheduleService != nil {
		_ = i.container.ScheduleService.DeleteAllByExtension(ctx, string(extID))
	}
	if i.container.ToolRegistry != nil {
		i.container.ToolRegistry.UnregisterByOwner(ctx, string(extID))
	}
	if i.container.AgentSkillCatalog != nil {
		_ = i.container.AgentSkillCatalog.Unregister(string(extID))
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
				_ = i.container.WorkflowRegistry.Unregister(wfID)
			}
		}
	}
	if i.container.UIHost != nil {
		for _, uiDef := range i.container.UIHost.ListAll() {
			if string(uiDef.ExtensionID) == string(extID) {
				_ = i.container.UIHost.UnregisterContribution(uiDef.ContributionID)
			}
		}
	}
	if i.container.PageHost != nil {
		_, _ = i.container.PageHost.HandleExtensionUninstalled(ctx, extension_page_host.ExtensionID(extID))
	}
	if i.container.UIContributionRepo != nil {
		_ = i.container.UIContributionRepo.DeleteByExtension(ctx, string(extID))
	}
	if i.container.SchemaRegistry != nil {
		i.container.SchemaRegistry.Unregister(string(extID))
	}
	if i.container.DesktopHost != nil {
		i.container.DesktopHost.UninstallContributions(ctx, string(extID))
	}
	if i.container.TaskRuntimeService != nil {
		_ = i.container.TaskRuntimeService.DeleteByExtension(ctx, string(extID))
	}
}

func (i *TypedContributionInstaller) RepairContributions(ctx context.Context, extID domain.ExtensionID, generation int64) {
	if i.container == nil {
		return
	}
	contribs, err := i.container.ContributionRepository.ListContributions(ctx, extID)
	if err != nil {
		log.Printf("[contribution-installer] list contributions for repair %s: %v", extID, err)
		return
	}
	i.UninstallContributions(ctx, extID)
	if err := i.InstallContributions(ctx, contribs, generation); err != nil {
		log.Printf("[contribution-installer] repair install failed %s: %v", extID, err)
		return
	}
	enabled := false
	if inst, err := i.container.InstallationRepository.GetInstallation(ctx, extID); err == nil {
		enabled = inst.EnablementState == domain.EnablementEnabled
	}
	if enabled {
		i.ActivateContributions(ctx, extID)
	}
}

func (i *TypedContributionInstaller) StopRuntimeInstances(ctx context.Context, extID domain.ExtensionID) {
	if i.container == nil || i.container.RuntimeSupervisor == nil {
		return
	}
	snap := i.container.RuntimeSupervisor.Snapshot(ctx, runtime_supervisor.DefinitionID(extID))
	for _, instance := range snap.Instances {
		_ = i.container.RuntimeSupervisor.Stop(ctx, instance.InstanceID, runtime_supervisor.StopReasonUninstall)
	}
}

func (i *TypedContributionInstaller) String() string {
	return fmt.Sprintf("TypedContributionInstaller(container=%v)", i.container != nil)
}
