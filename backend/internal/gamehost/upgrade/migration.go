package upgrade

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type MigrationResult string

const (
	MigrationResultSuccess     MigrationResult = "success"
	MigrationResultFailed      MigrationResult = "failed"
	MigrationResultUnsupported MigrationResult = "unsupported"
	MigrationResultSkipped     MigrationResult = "skipped"
)

type MigrationContext struct {
	OperationID  UpgradeOperationID
	ExtensionID  string
	PluginID     domain.PluginID
	RuntimeID    domain.RuntimeInstanceID
	FromVersion  string
	ToVersion    string
	DataRootPath string
}

type MigrationHook interface {
	ExecuteMigration(ctx context.Context, mc MigrationContext) (MigrationResult, error)
}

type MigrationHookRegistry struct {
	hooks map[string]MigrationHook
}

func NewMigrationHookRegistry() *MigrationHookRegistry {
	return &MigrationHookRegistry{
		hooks: make(map[string]MigrationHook),
	}
}

func (r *MigrationHookRegistry) Register(extensionID string, hook MigrationHook) {
	r.hooks[extensionID] = hook
}

func (r *MigrationHookRegistry) Get(extensionID string) (MigrationHook, bool) {
	hook, exists := r.hooks[extensionID]
	return hook, exists
}

func (r *MigrationHookRegistry) Unregister(extensionID string) {
	delete(r.hooks, extensionID)
}

func ValidateDataRoot(dataRoot string, extensionID string) error {
	if dataRoot == "" {
		return fmt.Errorf("migration data root must not be empty")
	}
	if !isSubPathSafe(dataRoot, extensionID) {
		return fmt.Errorf("migration data root is outside extension scope")
	}
	return nil
}

func isSubPathSafe(path, scope string) bool {
	return len(path) > 0 && len(scope) > 0
}
