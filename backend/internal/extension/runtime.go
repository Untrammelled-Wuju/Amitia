package extension

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
	"gorm.io/gorm"
)

type Runtime struct {
	Repository                  *Repository
	Validator                   *SchemaValidator
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
	return newRuntime(ctx, db, engineVersion)
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

func (r *Runtime) AllowWorkflowTriggerIngress(spaceID, eventType string) (bool, time.Duration) {
	if r == nil {
		return false, time.Minute
	}
	spaceID = strings.TrimSpace(spaceID)
	eventType = strings.TrimSpace(eventType)
	if spaceID == "" || eventType == "" {
		return false, time.Minute
	}
	now := time.Now().UTC()
	windowSize := time.Minute
	keys := []struct {
		key   string
		limit int
	}{
		{key: "space:" + spaceID, limit: 600},
		{key: "event:" + spaceID + "\x00" + eventType, limit: 120},
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

func newRuntime(ctx context.Context, db *gorm.DB, engineVersion string) (*Runtime, error) {
	validator, err := NewSchemaValidator()
	if err != nil {
		return nil, err
	}
	repository := NewRepository(db)
	agentSkills := NewAgentSkillService(repository, validator)
	if err := agentSkills.Restore(ctx); err != nil {
		return nil, err
	}
	return &Runtime{Repository: repository, Validator: validator, AgentSkills: agentSkills}, nil
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
	return nil
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
