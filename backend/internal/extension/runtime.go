// Deprecated: Legacy extension architecture.
// Do not add new capabilities. This implementation is retained only for
// compatibility, maintenance, testing, and migration to Extension Kernel.

package extension

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/agent/tool"
	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
	applog "github.com/u-ai/backend/log"
	"gorm.io/gorm"
)

type Runtime struct {
	Registry                    *Registry
	Executor                    *Executor
	Permissions                 *DefaultPermissionEvaluator
	Repository                  *Repository
	Service                     *ExtensionService
	Validator                   *SchemaValidator
	Plugins                     *PluginRegistry
	PluginManager               *PluginManager
	Workshop                    *WorkshopService
	WorkflowHost                *WorkflowHostAdapter
	AgentSkills                 *AgentSkillService
	Kernel                      *kernelruntime.Runtime
	WorkflowDeviceControl       WorkflowDeviceControlPlane
	workflowMutationLocks       sync.Map
	workflowDeviceSyncLoops     sync.Map
	workflowTriggerCapabilityMu sync.RWMutex
	workflowTriggerCapabilities map[string]WorkflowTriggerCapabilityStatus
	workflowTriggerAppCatalogMu sync.RWMutex
	workflowTriggerAppCatalog   []WorkflowTriggerAppCatalogItem
	workflowTriggerAppCatalogAt time.Time
	workflowTriggerIngressMu    sync.Mutex
	workflowTriggerIngress      map[string]workflowTriggerIngressWindow
	workflowWakeRuntimeMu       sync.Mutex
	workflowWakeRuntime         *workflowWakeRuntimeState
	workflowAndroidHealthMu     sync.RWMutex
	workflowAndroidHealth       WorkflowAndroidRuntimeHealthStatus
}

func (r *Runtime) AttachKernel(root string) error {
	kernel, err := kernelruntime.NewRuntime(root)
	if err != nil {
		return err
	}
	r.Kernel = kernel
	return nil
}

func (r *Runtime) AttachKernelFacade(kernel *kernelruntime.Runtime) error {
	r.Kernel = kernel
	if kernel != nil && kernel.Container() != nil {
		if err := kernel.RecoverPackageOperations(context.Background()); err != nil {
			return err
		}
	}
	return nil
}

func NewRuntime(ctx context.Context, db *gorm.DB, engineVersion string) (*Runtime, error) {
	return NewRuntimeWithOptions(ctx, db, engineVersion, RuntimeOptions{})
}

type RuntimeOptions struct {
	SkipPluginManagerStart bool
}

type WorkflowTriggerCapabilityStatus struct {
	ID                 string    `json:"id"`
	Supported          bool      `json:"supported"`
	Available          bool      `json:"available"`
	PermissionRequired bool      `json:"permissionRequired"`
	Permission         string    `json:"permission"`
	Reason             string    `json:"reason"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

type WorkflowTriggerAppCatalogItem struct {
	PackageName string `json:"packageName"`
	Label       string `json:"label"`
}

type workflowTriggerIngressWindow struct {
	StartedAt time.Time
	Count     int
}

func (r *Runtime) AllowWorkflowTriggerIngress(userID, eventType string) (bool, time.Duration) {
	if r == nil {
		return false, time.Minute
	}
	userID = strings.TrimSpace(userID)
	eventType = strings.TrimSpace(eventType)
	if userID == "" || eventType == "" {
		return false, time.Minute
	}
	now := time.Now().UTC()
	windowSize := time.Minute
	keys := []struct {
		key   string
		limit int
	}{
		{key: "user:" + userID, limit: 600},
		{key: "event:" + userID + "\x00" + eventType, limit: 120},
	}
	r.workflowTriggerIngressMu.Lock()
	defer r.workflowTriggerIngressMu.Unlock()
	if r.workflowTriggerIngress == nil {
		r.workflowTriggerIngress = make(map[string]workflowTriggerIngressWindow)
	}
	for _, item := range keys {
		state := r.workflowTriggerIngress[item.key]
		if state.StartedAt.IsZero() || now.Sub(state.StartedAt) >= windowSize {
			state = workflowTriggerIngressWindow{StartedAt: now}
			r.workflowTriggerIngress[item.key] = state
		}
		if state.Count >= item.limit {
			retryAfter := state.StartedAt.Add(windowSize).Sub(now)
			if retryAfter < time.Second {
				retryAfter = time.Second
			}
			return false, retryAfter
		}
	}
	for _, item := range keys {
		state := r.workflowTriggerIngress[item.key]
		state.Count++
		r.workflowTriggerIngress[item.key] = state
	}
	if len(r.workflowTriggerIngress) > 2048 {
		for key, state := range r.workflowTriggerIngress {
			if state.StartedAt.IsZero() || now.Sub(state.StartedAt) >= 2*windowSize {
				delete(r.workflowTriggerIngress, key)
			}
		}
	}
	return true, 0
}

func (r *Runtime) SetWorkflowTriggerCapabilityStatuses(items []WorkflowTriggerCapabilityStatus) {
	if r == nil {
		return
	}
	r.workflowTriggerCapabilityMu.Lock()
	defer r.workflowTriggerCapabilityMu.Unlock()
	if r.workflowTriggerCapabilities == nil {
		r.workflowTriggerCapabilities = make(map[string]WorkflowTriggerCapabilityStatus)
	}
	now := time.Now().UTC()
	for _, item := range items {
		item.ID = strings.TrimSpace(item.ID)
		if item.ID == "" {
			continue
		}
		item.Permission = strings.TrimSpace(item.Permission)
		item.Reason = strings.TrimSpace(item.Reason)
		item.UpdatedAt = now
		r.workflowTriggerCapabilities[item.ID] = item
	}
}

func (r *Runtime) WorkflowTriggerCapabilityStatuses() map[string]WorkflowTriggerCapabilityStatus {
	if r == nil {
		return nil
	}
	r.workflowTriggerCapabilityMu.RLock()
	defer r.workflowTriggerCapabilityMu.RUnlock()
	result := make(map[string]WorkflowTriggerCapabilityStatus, len(r.workflowTriggerCapabilities))
	for key, item := range r.workflowTriggerCapabilities {
		result[key] = item
	}
	return result
}

func (r *Runtime) SetWorkflowTriggerAppCatalog(items []WorkflowTriggerAppCatalogItem) {
	if r == nil {
		return
	}
	copyItems := append([]WorkflowTriggerAppCatalogItem(nil), items...)
	r.workflowTriggerAppCatalogMu.Lock()
	r.workflowTriggerAppCatalog = copyItems
	r.workflowTriggerAppCatalogAt = time.Now().UTC()
	r.workflowTriggerAppCatalogMu.Unlock()
}

func (r *Runtime) WorkflowTriggerAppCatalog() ([]WorkflowTriggerAppCatalogItem, time.Time) {
	if r == nil {
		return nil, time.Time{}
	}
	r.workflowTriggerAppCatalogMu.RLock()
	defer r.workflowTriggerAppCatalogMu.RUnlock()
	return append([]WorkflowTriggerAppCatalogItem(nil), r.workflowTriggerAppCatalog...), r.workflowTriggerAppCatalogAt
}

func NewRuntimeWithOptions(ctx context.Context, db *gorm.DB, engineVersion string, options RuntimeOptions) (*Runtime, error) {
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
	workshopRepository := NewWorkshopRepository(db)
	workflowCompiler := NewWorkflowCompiler(registry)
	workflowHost := &WorkflowHostAdapter{}
	workflowExecutor := NewWorkflowExecutor(BuildWorkflowAdapters(executor, workflowHost), validator)
	workshop := NewWorkshopService(workshopRepository, NewWorkshopGenerator(nil, registry), workflowCompiler, workflowExecutor, validator, registry, executor)
	workshop.AttachAgentSkills(agentSkills)
	if err := workshop.Restore(ctx); err != nil {
		applog.Warn("workflow skill restore warning", applog.Fields{"error_code": asExtensionError(err).Code})
	}
	var pluginManager *PluginManager
	if !options.SkipPluginManagerStart {
		kernelruntime.GlobalLegacyCallCounter().IncPluginStart()
		pluginManager = NewPluginManager(pluginRegistry, registry, executor, permissions, repository, validator)
		if err := pluginManager.Start(ctx); err != nil {
			return nil, fmt.Errorf("plugin manager start: %w", err)
		}
	}
	service.AttachPluginManager(pluginManager)
	return &Runtime{Registry: registry, Executor: executor, Permissions: permissions, Repository: repository, Service: service, Validator: validator, Plugins: pluginRegistry, PluginManager: pluginManager, Workshop: workshop, WorkflowHost: workflowHost, AgentSkills: agentSkills}, nil
}

func (r *Runtime) Close(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.workflowWakeRuntimeMu.Lock()
	wakeRuntime := r.workflowWakeRuntime
	r.workflowWakeRuntime = nil
	r.workflowWakeRuntimeMu.Unlock()
	if wakeRuntime != nil {
		wakeRuntime.close()
	}
	if r.PluginManager == nil {
		return nil
	}
	return r.PluginManager.Stop(ctx)
}

func (r *Runtime) LegacyWorkerStatus() LegacyWorkerStatus {
	if r == nil || r.PluginManager == nil {
		return LegacyWorkerStatus{}
	}
	return LegacyWorkerStatus{
		PluginManagerStarted: true,
	}
}

type LegacyWorkerStatus struct {
	PluginManagerStarted bool
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

func (r *Runtime) RunPackageStartupCleanup(ctx context.Context) error {
	if r.Repository == nil || r.Repository.db == nil {
		return nil
	}
	if !r.Repository.db.Migrator().HasTable("extension_package_import_sessions") {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := r.Repository.db.WithContext(ctx).Model(&packageImportSessionRecord{}).Where("expires_at < ? AND status NOT IN ?", now, []string{"installed", "expired"}).Updates(map[string]interface{}{"status": "expired", "package_blob": []byte{}, "updated_at": now}).Error; err != nil {
		return err
	}
	r.Repository.RetryOwnedResourceCleanup(ctx)
	return nil
}

func (r *Runtime) DetectLegacyPackagesReadOnly(ctx context.Context) (LegacyMigrationReport, error) {
	if r.Kernel == nil {
		return LegacyMigrationReport{}, fmt.Errorf("extension kernel unavailable")
	}
	detector := NewLegacyMigrationDetector(r.Kernel, r.Repository.db)
	return detector.Detect(ctx)
}
