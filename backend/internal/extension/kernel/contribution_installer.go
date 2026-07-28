package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/u-ai/backend/internal/extension/kernel/agent_skill"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/event"
	"github.com/u-ai/backend/internal/extension/kernel/extension_page_host"
	"github.com/u-ai/backend/internal/extension/kernel/hook"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/schedule"
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

func (i *TypedContributionInstaller) InstallContributions(ctx context.Context, contribs []domain.ContributionDefinition) {
	for _, contrib := range contribs {
		i.installSingle(ctx, contrib)
	}
}

func (i *TypedContributionInstaller) installSingle(ctx context.Context, contrib domain.ContributionDefinition) {
	if i.container == nil {
		return
	}
	defData, err := json.Marshal(contrib.Definition)
	if err != nil {
		log.Printf("[contribution-installer] marshal definition failed for %s: %v", contrib.ID, err)
		return
	}

	switch contrib.Kind {
	case domain.ContributionKindTool:
		i.installTool(ctx, contrib, defData)
	case domain.ContributionKindEventSubscription:
		i.installEventSubscription(ctx, contrib, defData)
	case domain.ContributionKindHook:
		i.installHook(ctx, contrib, defData)
	case domain.ContributionKindSchedule:
		i.installSchedule(ctx, contrib, defData)
	case domain.ContributionKindAgentSkill:
		i.installAgentSkill(ctx, contrib, defData)
	case domain.ContributionKindWorkflow:
		i.installWorkflow(ctx, contrib, defData)
	case domain.ContributionKindBackgroundService:
		i.installTaskDefinition(ctx, contrib, defData)
	case domain.ContributionKindUIPage, domain.ContributionKindUIPanel, domain.ContributionKindUIChat, domain.ContributionKindUIContextAction, domain.ContributionKindUIDesktop:
		i.installUIContribution(ctx, contrib, defData)
	}
}

func (i *TypedContributionInstaller) installTool(ctx context.Context, contrib domain.ContributionDefinition, defData []byte) {
	if i.container.ToolRegistry == nil {
		return
	}
	var def struct {
		ToolID       string          `json:"toolId"`
		ModelName    string          `json:"modelName"`
		InputSchema  json.RawMessage `json:"inputSchema"`
		OutputSchema json.RawMessage `json:"outputSchema"`
		RiskLevel    string          `json:"riskLevel,omitempty"`
		SideEffect   string          `json:"sideEffect,omitempty"`
	}
	if err := json.Unmarshal(defData, &def); err != nil {
		log.Printf("[contribution-installer] unmarshal tool %s: %v", contrib.ID, err)
		return
	}
	toolID := def.ToolID
	if toolID == "" {
		toolID = string(contrib.ID)
	}
	modelName := def.ModelName
	if modelName == "" {
		modelName = contrib.Name.Default
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
	}
	if err := i.container.ToolRegistry.Replace(ctx, toolDef); err != nil {
		log.Printf("[contribution-installer] register tool %s: %v", toolID, err)
	}
}

func (i *TypedContributionInstaller) installEventSubscription(ctx context.Context, contrib domain.ContributionDefinition, defData []byte) {
	if i.container.EventService == nil {
		return
	}
	var def event.EventSubscriptionDefinition
	if err := json.Unmarshal(defData, &def); err != nil {
		log.Printf("[contribution-installer] unmarshal event subscription %s: %v", contrib.ID, err)
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
	if err := i.container.EventService.RegisterSubscription(ctx, def); err != nil {
		log.Printf("[contribution-installer] register event subscription %s: %v", contrib.ID, err)
	}
}

func (i *TypedContributionInstaller) installHook(ctx context.Context, contrib domain.ContributionDefinition, defData []byte) {
	if i.container.HookService == nil || i.container.HookService.Lifecycle == nil {
		return
	}
	var def hook.HookContributionDefinition
	if err := json.Unmarshal(defData, &def); err != nil {
		log.Printf("[contribution-installer] unmarshal hook %s: %v", contrib.ID, err)
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
	if err := i.container.HookService.Lifecycle.InstallContribution(ctx, def); err != nil {
		log.Printf("[contribution-installer] install hook %s: %v", contrib.ID, err)
	}
}

func (i *TypedContributionInstaller) installSchedule(ctx context.Context, contrib domain.ContributionDefinition, defData []byte) {
	if i.container.ScheduleService == nil {
		return
	}
	var def schedule.ScheduleContributionDefinition
	if err := json.Unmarshal(defData, &def); err != nil {
		log.Printf("[contribution-installer] unmarshal schedule %s: %v", contrib.ID, err)
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
	if err := i.container.ScheduleService.InstallDefinition(ctx, &def); err != nil {
		log.Printf("[contribution-installer] install schedule %s: %v", contrib.ID, err)
	}
}

func (i *TypedContributionInstaller) installAgentSkill(ctx context.Context, contrib domain.ContributionDefinition, defData []byte) {
	if i.container.AgentSkillCatalog == nil {
		return
	}
	var def agent_skill.AgentSkillDefinition
	if err := json.Unmarshal(defData, &def); err != nil {
		log.Printf("[contribution-installer] unmarshal agent skill %s: %v", contrib.ID, err)
		return
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
	if err := i.container.AgentSkillCatalog.Register(def); err != nil {
		log.Printf("[contribution-installer] register agent skill %s: %v", contrib.ID, err)
	}
}

func (i *TypedContributionInstaller) installWorkflow(ctx context.Context, contrib domain.ContributionDefinition, defData []byte) {
	if i.container.WorkflowRegistry == nil {
		return
	}
	var def workflow.WorkflowDefinition
	if err := json.Unmarshal(defData, &def); err != nil {
		log.Printf("[contribution-installer] unmarshal workflow %s: %v", contrib.ID, err)
		return
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
	if err := i.container.WorkflowRegistry.Register(def); err != nil {
		log.Printf("[contribution-installer] register workflow %s: %v", contrib.ID, err)
	}
}

func (i *TypedContributionInstaller) installTaskDefinition(ctx context.Context, contrib domain.ContributionDefinition, defData []byte) {
	if i.container.TaskRuntimeService == nil {
		return
	}
	var def task_runtime.TaskDefinition
	if err := json.Unmarshal(defData, &def); err != nil {
		log.Printf("[contribution-installer] unmarshal task definition %s: %v", contrib.ID, err)
		return
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
	if err := i.container.TaskRuntimeService.PutTaskDefinition(ctx, &def); err != nil {
		log.Printf("[contribution-installer] put task definition %s: %v", contrib.ID, err)
	}
}

func (i *TypedContributionInstaller) installUIContribution(ctx context.Context, contrib domain.ContributionDefinition, defData []byte) {
	if i.container.UIHost == nil {
		return
	}
	var uiDef ui_contribution.UIContributionDefinition
	if err := json.Unmarshal(defData, &uiDef); err != nil {
		log.Printf("[contribution-installer] unmarshal ui contribution %s: %v", contrib.ID, err)
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
	_ = i.container.UIHost.RegisterContribution(&uiDef)
	if i.container.UIContributionRepo != nil {
		_ = i.container.UIContributionRepo.PutContribution(ctx, &uiDef)
	}
	if uiDef.Kind == ui_contribution.UIContributionWebPage || uiDef.Kind == ui_contribution.UIContributionSchemaPage {
		i.registerPage(ctx, uiDef)
	}
}

func (i *TypedContributionInstaller) registerPage(ctx context.Context, uiDef ui_contribution.UIContributionDefinition) {
	if i.container.PageHost == nil {
		return
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
		Generation:      1,
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
		i.installSingle(ctx, contrib)
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
		ToolID    string `json:"toolId"`
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
}

func (i *TypedContributionInstaller) RepairContributions(ctx context.Context, extID domain.ExtensionID) {
	if i.container == nil {
		return
	}
	contribs, err := i.container.ContributionRepository.ListContributions(ctx, extID)
	if err != nil {
		log.Printf("[contribution-installer] list contributions for repair %s: %v", extID, err)
		return
	}
	i.UninstallContributions(ctx, extID)
	i.InstallContributions(ctx, contribs)
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
