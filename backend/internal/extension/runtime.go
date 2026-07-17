package extension

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/agent/tool"
	applog "github.com/u-ai/backend/log"
	"gorm.io/gorm"
)

type Runtime struct {
	Registry      *Registry
	Executor      *Executor
	Permissions   *DefaultPermissionEvaluator
	Repository    *Repository
	Service       *ExtensionService
	Validator     *SchemaValidator
	Plugins       *PluginRegistry
	PluginManager *PluginManager
	Workshop      *WorkshopService
	WorkflowHost  *WorkflowHostAdapter
	AgentSkills   *AgentSkillService
	Packages      *PackageService
}

func NewRuntime(ctx context.Context, db *gorm.DB, engineVersion string) (*Runtime, error) {
	validator, err := NewSchemaValidator()
	if err != nil {
		return nil, err
	}
	repository := NewRepository(db)
	registry := NewRegistry(engineVersion, validator, repository)
	permissions := NewPermissionEvaluator(repository)
	registered, err := NewLegacyToolAdapter().RegisterAll(ctx, registry)
	if err != nil {
		return nil, err
	}
	for _, skillID := range registered {
		item, getErr := registry.Get(ctx, skillID)
		if getErr != nil {
			return nil, getErr
		}
		for _, capabilityName := range item.Definition.Capabilities {
			capability, ok := Capability(capabilityName)
			if ok && capability.Risk != "high" {
				permissions.GrantSystemPolicy(skillID, capabilityName, DecisionAllowAlways)
			} else if ok && capability.Risk == "high" && hasTrigger(item.Definition.Triggers, TriggerLLM) {
				permissions.GrantSystemPolicy(skillID, capabilityName, DecisionAllowSession)
			}
		}
	}
	executor := NewExecutor(registry, validator, permissions, repository)
	service := NewService(registry, executor, repository, validator)
	agentSkills := NewAgentSkillService(repository, registry, validator)
	service.AttachLifecycleService(NewExtensionLifecycleService(registry, repository, agentSkills))
	if err := agentSkills.Restore(ctx); err != nil {
		return nil, err
	}
	if err := registerAgentSkillRuntime(ctx, registry, agentSkills); err != nil {
		return nil, err
	}
	pluginRegistry := NewPluginRegistry(engineVersion, validator)
	if err := pluginRegistry.Register(ctx, newDiagnosticPlugin(), newDiagnosticPlugin); err != nil {
		return nil, err
	}
	diagnostic := newDiagnosticPlugin().Manifest()
	for _, capability := range diagnostic.Capabilities {
		permissions.GrantSystemPolicy(diagnostic.Metadata.ID, capability, DecisionAllowAlways)
	}
	pluginManager := NewPluginManager(pluginRegistry, registry, executor, permissions, repository, validator)
	service.AttachPluginManager(pluginManager)
	if err := pluginManager.Start(ctx); err != nil {
		return nil, err
	}
	workshopRepository := NewWorkshopRepository(db)
	workflowCompiler := NewWorkflowCompiler(registry)
	workflowHost := &WorkflowHostAdapter{}
	workflowExecutor := NewWorkflowExecutor(BuildWorkflowAdapters(executor, workflowHost), validator)
	workshop := NewWorkshopService(workshopRepository, NewWorkshopGenerator(nil, registry), workflowCompiler, workflowExecutor, validator, registry, executor)
	workshop.AttachAgentSkills(agentSkills)
	if err := workshop.Restore(ctx); err != nil {
		applog.Warn("workflow skill restore warning", applog.Fields{"error_code": asExtensionError(err).Code})
	}
	packages := NewPackageService(repository, registry, validator, workflowCompiler, workshop.installer, agentSkills)
	if err := packages.Restore(ctx); err != nil {
		return nil, err
	}
	return &Runtime{Registry: registry, Executor: executor, Permissions: permissions, Repository: repository, Service: service, Validator: validator, Plugins: pluginRegistry, PluginManager: pluginManager, Workshop: workshop, WorkflowHost: workflowHost, AgentSkills: agentSkills, Packages: packages}, nil
}

func (r *Runtime) Close(ctx context.Context) error {
	if r == nil || r.PluginManager == nil {
		return nil
	}
	return r.PluginManager.Stop(ctx)
}

func (r *Runtime) BeforePrompt(ctx context.Context, scope ExecutionScope) []ContextContribution {
	if r == nil || r.PluginManager == nil {
		return nil
	}
	snapshot, err := r.pluginSnapshot(ctx, scope)
	if err != nil {
		return nil
	}
	return r.PluginManager.DispatchBeforePrompt(ctx, snapshot)
}

func (r *Runtime) AfterReply(scope ExecutionScope, reply ReplyView) bool {
	if r == nil || r.PluginManager == nil {
		return false
	}
	snapshot, err := r.pluginSnapshot(context.Background(), scope)
	if err != nil {
		return false
	}
	queued := r.PluginManager.DispatchAfterReply(snapshot, reply)
	eventData, _ := json.Marshal(map[string]string{"messageId": reply.MessageID, "characterId": reply.CharacterID, "conversationId": reply.ConversationID, "channel": reply.Channel})
	_ = r.PluginManager.EmitSystemEvent(context.Background(), ExtensionEvent{Source: "amitia://system/chat", Type: "dev.amitia.reply.completed.v1", Subject: "character/" + reply.CharacterID + "/conversation/" + reply.ConversationID, Data: eventData, TraceID: scope.TraceID, CorrelationID: scope.CorrelationID, CausationID: scope.CausationID})
	return queued
}

func (r *Runtime) pluginSnapshot(ctx context.Context, scope ExecutionScope) (ExtensionSnapshot, error) {
	if err := r.Repository.ValidateConversationScope(ctx, scope); err != nil {
		return ExtensionSnapshot{}, err
	}
	return ExtensionSnapshot{SchemaVersion: "1.0.0", User: SnapshotUser{ID: scope.UserID}, Character: SnapshotEntity{ID: scope.CharacterID}, Conversation: SnapshotEntity{ID: scope.ConversationID}, Channel: SnapshotChannel{Name: scope.Channel}, CapturedAt: time.Now().UTC()}, nil
}

func (r *Runtime) ModelTools(ctx context.Context, scope ExecutionScope) ([]tool.Tool, error) {
	scope.Trigger = TriggerLLM
	agentSkillToolsAvailable := false
	if r.AgentSkills != nil {
		if catalog, catalogErr := r.AgentSkills.ResolveCatalog(ctx, scope); catalogErr == nil && len(catalog) > 0 {
			agentSkillToolsAvailable = true
		}
	}
	definitions, err := r.Registry.Available(ctx, scope)
	if err != nil {
		return nil, err
	}
	result := make([]tool.Tool, 0, len(definitions))
	for _, definition := range definitions {
		if definition.Internal && !agentSkillToolsAvailable {
			continue
		}
		allowed := true
		identity := ExtensionIdentity{ExtensionID: definition.ID, SkillID: definition.ID, Version: definition.Version}
		for _, capability := range definition.Capabilities {
			if r.Permissions.PreviewExecution(ctx, identity, capability, scope) == DecisionDeny {
				allowed = false
				break
			}
		}
		if !allowed {
			continue
		}
		var schema struct {
			Type       string                   `json:"type"`
			Properties map[string]tool.Property `json:"properties"`
			Required   []string                 `json:"required"`
		}
		if err := json.Unmarshal(definition.InputSchema, &schema); err != nil {
			return nil, fmt.Errorf("decode tool schema %s: %w", definition.ID, err)
		}
		result = append(result, tool.Tool{Type: "function", Function: tool.Function{Name: definition.ModelName, Description: definition.Description, Parameters: tool.Parameters{Type: schema.Type, Properties: schema.Properties, Required: schema.Required}}})
	}
	return result, nil
}

func (r *Runtime) ExecuteModelTool(ctx context.Context, modelName string, input json.RawMessage, scope ExecutionScope, idempotencyKey string) (SkillResult, bool) {
	registered, err := r.Registry.GetByModelName(ctx, modelName)
	if err != nil {
		return SkillResult{Status: RunFailed, Error: asExtensionError(err), VisibleText: "tool not found: " + modelName}, false
	}
	scope.Trigger = TriggerLLM
	result, executeErr := r.Executor.Execute(ctx, ExecuteSkillRequest{SkillID: registered.Definition.ID, Input: input, Scope: scope, IdempotencyKey: idempotencyKey})
	if result.VisibleText == "" && len(result.Output) > 0 {
		result.VisibleText = string(result.Output)
	}
	if executeErr != nil && result.VisibleText == "" {
		result.VisibleText = executeErr.Error()
	}
	return result, true
}
