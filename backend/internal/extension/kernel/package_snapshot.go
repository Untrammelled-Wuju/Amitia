package kernel

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/migration"
	"github.com/u-ai/backend/internal/extension/kernel/package_security"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
)

const packageConfigSnapshotSchemaVersion = "1.0.0"

type packageConfigSnapshot struct {
	Metadata      map[string]any                  `json:"metadata"`
	Contributions []domain.ContributionDefinition `json:"contributions"`
	Permissions   packageConfigPermissionSnapshot `json:"permissions"`
	ScopeBindings []sqlite.ScopeBinding           `json:"scopeBindings"`
	SchemaVersion string                          `json:"schemaVersion"`
	CapturedAt    string                          `json:"capturedAt"`
}

type packageConfigPermissionSnapshot struct {
	Requirements []sqlite.PermissionRequirement `json:"requirements"`
	Grants       []sqlite.PermissionGrant       `json:"grants"`
}

type packageResourceSnapshotEntry struct {
	Resource                domain.ResourceOwnership `json:"resource"`
	ResourceHash            string                   `json:"resourceHash"`
	RestoreStrategy         string                   `json:"restoreStrategy"`
	LogicalPath             string                   `json:"logicalPath"`
	OriginalPath            string                   `json:"originalPath"`
	ContentHash             string                   `json:"contentHash"`
	Size                    int64                    `json:"size"`
	StorageReference        string                   `json:"storageReference"`
	ContentStorageReference string                   `json:"contentStorageReference"`
}

type packageResourceSnapshot struct {
	Entries []packageResourceSnapshotEntry `json:"entries"`
}

type packageMigrationOperationSnapshot struct {
	Operation migration.MigrationOperation    `json:"operation"`
	Steps     []migration.MigrationStepRecord `json:"steps"`
}

type packageMigrationStateSnapshot struct {
	Mode        string                              `json:"mode"`
	Definitions []migration.MigrationDefinition     `json:"definitions,omitempty"`
	Operations  []packageMigrationOperationSnapshot `json:"operations,omitempty"`
}

type packageUserDataMigrationState struct {
	Mode           string            `json:"mode"`
	Snapshots      []string          `json:"snapshots,omitempty"`
	Completed      []string          `json:"completed,omitempty"`
	AffectedTables []string          `json:"affectedTables,omitempty"`
	RecordCounts   map[string]int64  `json:"recordCounts,omitempty"`
	DataExports    map[string]string `json:"dataExports,omitempty"`
}

type packageSnapshotHashPayload struct {
	ExtensionID                string `json:"extensionId"`
	SourceVersion              string `json:"sourceVersion"`
	SourceGeneration           int64  `json:"sourceGeneration"`
	ArtifactID                 string `json:"artifactId"`
	DefinitionSnapshotJSON     string `json:"definitionSnapshotJson"`
	ModuleSnapshotJSON         string `json:"moduleSnapshotJson"`
	ContributionSnapshotJSON   string `json:"contributionSnapshotJson"`
	PermissionSnapshotJSON     string `json:"permissionSnapshotJson"`
	ScopeSnapshotJSON          string `json:"scopeSnapshotJson"`
	ConfigSnapshotJSON         string `json:"configSnapshotJson"`
	SecretRefsJSON             string `json:"secretRefsJson"`
	ResourceSnapshotJSON       string `json:"resourceSnapshotJson"`
	MigrationStateSnapshotJSON string `json:"migrationStateSnapshotJson"`
	UserDataMigrationStateJSON string `json:"userDataMigrationStateJson"`
	InstalledPath              string `json:"installedPath"`
	SourceOperationID          string `json:"sourceOperationId"`
}

func (r *Runtime) capturePackageStateSnapshots(ctx context.Context, installed domain.ExtensionInstallation) (string, string, string, string, string, error) {
	metadata, refs := sanitizePackageSnapshotMap(installed.Metadata)
	if r.container == nil {
		return "", "", "", "", "", fmt.Errorf("kernel: container unavailable for rollback snapshot")
	}
	if r.container.ContributionRepository == nil {
		return "", "", "", "", "", fmt.Errorf("kernel: contribution repository unavailable for rollback snapshot")
	}
	if r.container.PermissionRepository == nil {
		return "", "", "", "", "", fmt.Errorf("kernel: permission repository unavailable for rollback snapshot")
	}
	contributions, err := r.container.ContributionRepository.ListContributions(ctx, installed.ExtensionID)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("kernel: capture contributions for config snapshot: %w", err)
	}
	requirements, err := r.container.PermissionRepository.ListRequirements(ctx, installed.ExtensionID)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("kernel: capture permission requirements for config snapshot: %w", err)
	}
	grants, err := r.container.PermissionRepository.ListGrants(ctx, installed.ExtensionID)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("kernel: capture permission grants for config snapshot: %w", err)
	}
	if r.container.ScopeRepository == nil {
		return "", "", "", "", "", fmt.Errorf("kernel: scope repository unavailable for rollback snapshot")
	}
	scopeBindings, err := r.container.ScopeRepository.ListBindings(ctx, installed.ExtensionID)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("kernel: capture scope bindings for config snapshot: %w", err)
	}
	if len(contributions) == 0 && len(requirements) == 0 && len(grants) == 0 && len(scopeBindings) == 0 {
		return "", "", "", "", "", fmt.Errorf("kernel: config snapshot integrity check failed: no contributions, permissions, or scope bindings found for extension %s", installed.ExtensionID)
	}
	sort.Slice(contributions, func(i, j int) bool { return string(contributions[i].ID) < string(contributions[j].ID) })
	sort.Slice(requirements, func(i, j int) bool { return requirements[i].PermissionName < requirements[j].PermissionName })
	sort.Slice(grants, func(i, j int) bool { return grants[i].PermissionName < grants[j].PermissionName })
	sort.Slice(scopeBindings, func(i, j int) bool {
		if scopeBindings[i].ScopeType != scopeBindings[j].ScopeType {
			return scopeBindings[i].ScopeType < scopeBindings[j].ScopeType
		}
		return scopeBindings[i].ScopeID < scopeBindings[j].ScopeID
	})
	configJSON, err := json.Marshal(packageConfigSnapshot{
		Metadata:      metadata,
		Contributions: contributions,
		Permissions: packageConfigPermissionSnapshot{
			Requirements: requirements,
			Grants:       grants,
		},
		ScopeBindings: scopeBindings,
		SchemaVersion: packageConfigSnapshotSchemaVersion,
		CapturedAt:    installed.UpdatedAt.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return "", "", "", "", "", err
	}
	if r.container.ResourceRepository == nil {
		return "", "", "", "", "", fmt.Errorf("kernel: resource repository unavailable for rollback snapshot")
	}
	resources, err := r.container.ResourceRepository.ListResources(ctx, installed.ExtensionID)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("kernel: capture resources: %w", err)
	}
	var resourceJSON []byte
	if len(resources) == 0 {
		resourceJSON, err = json.Marshal(packageResourceSnapshot{Entries: []packageResourceSnapshotEntry{}})
		if err != nil {
			return "", "", "", "", "", err
		}
	} else {
		sort.Slice(resources, func(i, j int) bool { return resources[i].ResourceID < resources[j].ResourceID })
		resourceSnapshot := packageResourceSnapshot{Entries: make([]packageResourceSnapshotEntry, 0, len(resources))}
		absExtRoot, absErr := filepath.Abs(r.container.ExtRoot)
		if absErr != nil {
			return "", "", "", "", "", fmt.Errorf("kernel: resolve ext root: %w", absErr)
		}
		contentStore := NewResourceContentStore(absExtRoot)
		for _, resource := range resources {
			resource.Metadata, _ = sanitizePackageSnapshotMap(resource.Metadata)
			raw, marshalErr := json.Marshal(resource)
			if marshalErr != nil {
				return "", "", "", "", "", marshalErr
			}
			logicalPath := extractResourceStringField(resource, "logicalPath")
			if logicalPath == "" {
				logicalPath = resource.Reference
			}
			originalPath := resource.Reference

			var contentHash string
			var size int64
			var contentStorageRef string
			if originalPath != "" {
				absOriginal, absErr := filepath.Abs(originalPath)
				if absErr != nil {
					return "", "", "", "", "", fmt.Errorf("kernel: resolve resource path %s: %w", resource.ResourceID, absErr)
				}
				if strings.HasPrefix(absOriginal, absExtRoot+string(filepath.Separator)) {
					if _, statErr := os.Stat(absOriginal); statErr == nil {
						if storedRef, hash, sz, storeErr := contentStore.StoreContent(absOriginal); storeErr == nil {
							contentStorageRef = storedRef
							contentHash = hash
							size = sz
						}
					}
				}
			}

			resourceSnapshot.Entries = append(resourceSnapshot.Entries, packageResourceSnapshotEntry{
				Resource:                resource,
				ResourceHash:            packageSnapshotDigest(raw),
				RestoreStrategy:         "repository_upsert",
				LogicalPath:             logicalPath,
				OriginalPath:            originalPath,
				ContentHash:             contentHash,
				Size:                    size,
				StorageReference:        originalPath,
				ContentStorageReference: contentStorageRef,
			})
		}
		resourceJSON, err = json.Marshal(resourceSnapshot)
		if err != nil {
			return "", "", "", "", "", err
		}
	}
	if r.container.MigrationRepository == nil {
		return "", "", "", "", "", fmt.Errorf("kernel: migration repository unavailable for rollback snapshot")
	}
	definitions, err := r.container.MigrationRepository.ListMigrationDefinitions(ctx, string(installed.ExtensionID))
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("kernel: capture migration definitions: %w", err)
	}
	operations, err := r.container.MigrationRepository.ListMigrationOperations(ctx, string(installed.ExtensionID))
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("kernel: capture migration operations: %w", err)
	}
	sort.Slice(definitions, func(i, j int) bool { return definitions[i].MigrationID < definitions[j].MigrationID })
	sort.Slice(operations, func(i, j int) bool { return operations[i].OperationID < operations[j].OperationID })
	migrationSnapshot := packageMigrationStateSnapshot{Mode: "none"}
	userState := packageUserDataMigrationState{Mode: "none"}
	affectedTableSet := make(map[string]struct{})
	if len(definitions) > 0 || len(operations) > 0 {
		migrationSnapshot.Mode = "repository"
		migrationSnapshot.Definitions = definitions
		userState.Mode = "repository"
		userState.RecordCounts = make(map[string]int64)
		userState.DataExports = make(map[string]string)
	}
	for _, definition := range definitions {
		for _, dd := range definition.DataDomains {
			if dd.Namespace != "" {
				affectedTableSet[dd.Namespace] = struct{}{}
			}
		}
	}
	for _, operation := range operations {
		steps, stepErr := r.container.MigrationRepository.ListMigrationSteps(ctx, operation.OperationID)
		if stepErr != nil {
			return "", "", "", "", "", fmt.Errorf("kernel: capture migration steps: %w", stepErr)
		}
		sort.Slice(steps, func(i, j int) bool { return steps[i].StepID < steps[j].StepID })
		migrationSnapshot.Operations = append(migrationSnapshot.Operations, packageMigrationOperationSnapshot{Operation: operation, Steps: steps})
		if operation.SnapshotID != "" {
			userState.Snapshots = append(userState.Snapshots, operation.SnapshotID)
		}
		if operation.Status == migration.OperationStatusCompleted {
			userState.Completed = append(userState.Completed, operation.OperationID)
		}
	}
	sort.Strings(userState.Snapshots)
	sort.Strings(userState.Completed)
	for table := range affectedTableSet {
		userState.AffectedTables = append(userState.AffectedTables, table)
	}
	sort.Strings(userState.AffectedTables)
	if userState.DataExports != nil {
		db := r.container.Store.DB()
		if db != nil {
			for _, table := range userState.AffectedTables {
				exported, exportErr := captureUserDataTableSnapshot(ctx, db, string(installed.ExtensionID), table)
				if exportErr != nil {
					return "", "", "", "", "", fmt.Errorf("kernel: user data snapshot for table %s: %w", table, exportErr)
				}
				userState.DataExports[table] = exported.jsonl
				if userState.RecordCounts != nil {
					userState.RecordCounts[table] = exported.count
				}
			}
		}
	}
	migrationJSON, err := json.Marshal(migrationSnapshot)
	if err != nil {
		return "", "", "", "", "", err
	}
	userStateJSON, err := json.Marshal(userState)
	if err != nil {
		return "", "", "", "", "", err
	}
	sort.Strings(refs)
	refs = uniquePackageStrings(refs)
	refsJSON, err := json.Marshal(refs)
	if err != nil {
		return "", "", "", "", "", err
	}
	return string(configJSON), string(refsJSON), string(resourceJSON), string(migrationJSON), string(userStateJSON), nil
}

type tableSnapshotResult struct {
	jsonl string
	count int64
}

func captureUserDataTableSnapshot(ctx context.Context, db *sql.DB, extensionID, table string) (tableSnapshotResult, error) {
	result := tableSnapshotResult{}
	rows, queryErr := db.QueryContext(ctx, fmt.Sprintf("SELECT * FROM %s", table))
	if queryErr != nil {
		return result, fmt.Errorf("kernel: query table %s for snapshot: %w", table, queryErr)
	}
	defer rows.Close()

	columns, colErr := rows.Columns()
	if colErr != nil {
		return result, fmt.Errorf("kernel: get columns for %s: %w", table, colErr)
	}
	idColumn := ""
	for _, col := range columns {
		if col == "id" || col == "entity_id" {
			idColumn = col
			break
		}
	}
	namespace := strings.TrimPrefix(table, migration.ExtensionNamespacePrefix(extensionID))

	var lines []string
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if scanErr := rows.Scan(valuePtrs...); scanErr != nil {
			return result, fmt.Errorf("kernel: scan row for %s: %w", table, scanErr)
		}
		payload := make(map[string]interface{}, len(columns))
		entityID := ""
		for i, col := range columns {
			val := normalizeSQLValue(values[i])
			payload[col] = val
			if col == idColumn {
				entityID = fmt.Sprintf("%v", val)
			}
		}
		payloadHash := computeUserDataPayloadHash(payload)
		record := userDataRecord{
			SchemaVersion: "1.0.0",
			ExtensionID:   extensionID,
			Namespace:     namespace,
			EntityType:    "record",
			EntityID:      entityID,
			Operation:     "upsert",
			Payload:       payload,
			PayloadHash:   payloadHash,
		}
		jsonBytes, marshalErr := json.Marshal(record)
		if marshalErr != nil {
			return result, fmt.Errorf("kernel: marshal row for %s: %w", table, marshalErr)
		}
		lines = append(lines, string(jsonBytes))
		result.count++
	}
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("kernel: iterate rows for %s: %w", table, err)
	}
	if result.count == 0 {
		return result, fmt.Errorf("kernel: user data snapshot integrity check failed: table %s export is empty", table)
	}
	result.jsonl = strings.Join(lines, "\n")
	return result, nil
}

func extractResourceStringField(resource domain.ResourceOwnership, field string) string {
	if resource.Metadata == nil {
		return ""
	}
	val, ok := resource.Metadata[field]
	if !ok {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	return fmt.Sprint(val)
}

func extractResourceInt64Field(resource domain.ResourceOwnership, field string) int64 {
	if resource.Metadata == nil {
		return 0
	}
	val, ok := resource.Metadata[field]
	if !ok {
		return 0
	}
	switch v := val.(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	case json.Number:
		n, _ := v.Int64()
		return n
	default:
		return 0
	}
}

func computePackageSnapshotHash(point PackageRollbackPoint) (string, error) {
	payload := packageSnapshotHashPayload{ExtensionID: point.ExtensionID, SourceVersion: point.SourceVersion, SourceGeneration: point.SourceGeneration, ArtifactID: point.ArtifactID, DefinitionSnapshotJSON: point.DefinitionSnapshotJSON, ModuleSnapshotJSON: point.ModuleSnapshotJSON, ContributionSnapshotJSON: point.ContributionSnapshotJSON, PermissionSnapshotJSON: point.PermissionSnapshotJSON, ScopeSnapshotJSON: point.ScopeSnapshotJSON, ConfigSnapshotJSON: point.ConfigSnapshotJSON, SecretRefsJSON: point.SecretRefsJSON, ResourceSnapshotJSON: point.ResourceSnapshotJSON, MigrationStateSnapshotJSON: point.MigrationStateSnapshotJSON, UserDataMigrationStateJSON: point.UserDataMigrationStateJSON, InstalledPath: point.InstalledPath, SourceOperationID: point.SourceOperationID}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return packageSnapshotDigest(raw), nil
}

func validatePackageSnapshot(point PackageRollbackPoint) error {
	if point.RetentionState != "active" && point.RetentionState != "forward_recovery" {
		return fmt.Errorf("kernel: rollback point retention state %s", point.RetentionState)
	}
	retention := point.RetentionUntil
	if retention == "" {
		retention = point.ExpiresAt
	}
	if retention != "" {
		expiresAt, err := time.Parse(time.RFC3339Nano, retention)
		if err != nil || !time.Now().UTC().Before(expiresAt) {
			return fmt.Errorf("kernel: rollback point expired")
		}
	}
	expected, err := computePackageSnapshotHash(point)
	if err != nil {
		return err
	}
	if point.SnapshotHash == "" || point.SnapshotHash != expected {
		return fmt.Errorf("kernel: rollback point snapshot hash mismatch")
	}
	return nil
}

func packageSnapshotManualRecoveryReason(forward, target PackageRollbackPoint) string {
	var forwardState packageMigrationStateSnapshot
	var targetState packageMigrationStateSnapshot
	if json.Unmarshal([]byte(forward.MigrationStateSnapshotJSON), &forwardState) != nil || json.Unmarshal([]byte(target.MigrationStateSnapshotJSON), &targetState) != nil {
		return "migration snapshot corrupt"
	}
	targetOperations := make(map[string]struct{}, len(targetState.Operations))
	for _, entry := range targetState.Operations {
		targetOperations[entry.Operation.OperationID] = struct{}{}
	}
	for _, entry := range forwardState.Operations {
		operation := entry.Operation
		if _, exists := targetOperations[operation.OperationID]; exists || operation.Status != migration.OperationStatusCompleted {
			continue
		}
		if operation.Reversibility == migration.ReversibilityIrreversible {
			return "completed irreversible migration " + operation.OperationID
		}
	}
	return ""
}

type packageMigrationStateDiff struct {
	OperationsToRollback []packageMigrationOperationSnapshot `json:"operationsToRollback"`
	NewOperations        []packageMigrationOperationSnapshot `json:"newOperations"`
	RequiresManual       bool                                `json:"requiresManual"`
	ManualReason         string                              `json:"manualReason,omitempty"`
}

func diffPackageMigrationStates(forward, target PackageRollbackPoint) packageMigrationStateDiff {
	var diff packageMigrationStateDiff
	var forwardState packageMigrationStateSnapshot
	var targetState packageMigrationStateSnapshot
	if json.Unmarshal([]byte(forward.MigrationStateSnapshotJSON), &forwardState) != nil {
		diff.RequiresManual = true
		diff.ManualReason = "forward migration snapshot corrupt"
		return diff
	}
	if json.Unmarshal([]byte(target.MigrationStateSnapshotJSON), &targetState) != nil {
		diff.RequiresManual = true
		diff.ManualReason = "target migration snapshot corrupt"
		return diff
	}
	targetOperations := make(map[string]packageMigrationOperationSnapshot, len(targetState.Operations))
	for _, entry := range targetState.Operations {
		targetOperations[entry.Operation.OperationID] = entry
	}
	for _, entry := range forwardState.Operations {
		if _, exists := targetOperations[entry.Operation.OperationID]; exists {
			continue
		}
		diff.NewOperations = append(diff.NewOperations, entry)
		if entry.Operation.Status == migration.OperationStatusCompleted {
			diff.OperationsToRollback = append(diff.OperationsToRollback, entry)
			if entry.Operation.Reversibility == migration.ReversibilityIrreversible {
				diff.RequiresManual = true
				if diff.ManualReason != "" {
					diff.ManualReason += "; "
				}
				diff.ManualReason += "completed irreversible migration " + entry.Operation.OperationID
			}
		}
	}
	return diff
}

func (r *Runtime) restorePackageRepositorySnapshots(ctx context.Context, extensionID domain.ExtensionID, point PackageRollbackPoint, installation *domain.ExtensionInstallation) error {
	var config packageConfigSnapshot
	if err := json.Unmarshal([]byte(point.ConfigSnapshotJSON), &config); err != nil {
		return fmt.Errorf("kernel: config snapshot corrupt: %w", err)
	}
	if installation.Metadata == nil {
		installation.Metadata = map[string]any{}
	}
	for key, value := range config.Metadata {
		if !isPackageOperationalMetadataKey(key) {
			installation.Metadata[key] = value
		}
	}
	var snapshot packageResourceSnapshot
	if err := json.Unmarshal([]byte(point.ResourceSnapshotJSON), &snapshot); err != nil {
		return fmt.Errorf("kernel: resource snapshot corrupt: %w", err)
	}
	for _, entry := range snapshot.Entries {
		raw, err := json.Marshal(entry.Resource)
		if err != nil {
			return err
		}
		if entry.RestoreStrategy != "repository_upsert" || entry.ResourceHash != packageSnapshotDigest(raw) {
			return fmt.Errorf("kernel: resource snapshot integrity failed: %s", entry.Resource.ResourceID)
		}
	}
	absExtRoot, absErr := filepath.Abs(r.container.ExtRoot)
	if absErr != nil {
		return fmt.Errorf("kernel: resolve ext root: %w", absErr)
	}
	contentStore := NewResourceContentStore(absExtRoot)
	for _, entry := range snapshot.Entries {
		if err := validatePackageResourceSnapshotEntry(entry); err != nil {
			return NewPackageError(PackageErrCodeResourceSnapshotInvalid, 400, fmt.Errorf("kernel: resource %s snapshot entry validation failed at stage entry_validation: %w", entry.Resource.ResourceID, err))
		}
		if err := contentStore.VerifyContent(entry.ContentStorageReference, entry.ContentHash); err != nil {
			return NewPackageError(PackageErrCodeResourceSnapshotInvalid, 500, fmt.Errorf("kernel: resource %s content verification failed at stage content_verification: %w", entry.Resource.ResourceID, err))
		}
		absOriginal, absErr := filepath.Abs(resolveResourceRestorePath(entry))
		if absErr != nil {
			return NewPackageError(PackageErrCodeResourceSnapshotInvalid, 400, fmt.Errorf("kernel: resolve restore path for resource %s at stage path_validation: %w", entry.Resource.ResourceID, absErr))
		}
		if !strings.HasPrefix(absOriginal, absExtRoot+string(filepath.Separator)) {
			return NewPackageError(PackageErrCodeResourceSnapshotInvalid, 400, fmt.Errorf("kernel: restore path %s for resource %s escapes ext root at stage path_validation", absOriginal, entry.Resource.ResourceID))
		}
		data, readErr := contentStore.ReadContent(entry.ContentStorageReference)
		if readErr != nil {
			return NewPackageError(PackageErrCodeResourceSnapshotInvalid, 500, fmt.Errorf("kernel: read content for resource %s at stage content_read: %w", entry.Resource.ResourceID, readErr))
		}
		sum := sha256.Sum256(data)
		actualHash := "sha256:" + hex.EncodeToString(sum[:])
		if actualHash != entry.ContentHash {
			return NewPackageError(PackageErrCodeResourceSnapshotInvalid, 500, fmt.Errorf("kernel: resource %s content hash mismatch at stage content_verification: expected %s got %s", entry.Resource.ResourceID, entry.ContentHash, actualHash))
		}
		if info, statErr := os.Stat(absOriginal); statErr == nil && !info.IsDir() {
			existingHash, hashErr := hashFileContent(absOriginal)
			if hashErr != nil {
				return NewPackageError(PackageErrCodeResourceSnapshotInvalid, 500, fmt.Errorf("kernel: resource %s failed to hash existing target at stage target_hash: %w", entry.Resource.ResourceID, hashErr))
			}
			if existingHash == entry.ContentHash {
				continue
			}
		}
		if mkErr := os.MkdirAll(filepath.Dir(absOriginal), 0o700); mkErr != nil {
			return NewPackageError(PackageErrCodeResourceSnapshotInvalid, 500, fmt.Errorf("kernel: create directory for restored resource %s at stage prepare: %w", entry.Resource.ResourceID, mkErr))
		}
		tmp, tmpErr := os.CreateTemp(filepath.Dir(absOriginal), ".restore-*.tmp")
		if tmpErr != nil {
			return NewPackageError(PackageErrCodeResourceSnapshotInvalid, 500, fmt.Errorf("kernel: create temp file for restored resource %s at stage prepare: %w", entry.Resource.ResourceID, tmpErr))
		}
		tmpPath := tmp.Name()
		if _, writeErr := tmp.Write(data); writeErr != nil {
			tmp.Close()
			os.Remove(tmpPath)
			return NewPackageError(PackageErrCodeResourceSnapshotInvalid, 500, fmt.Errorf("kernel: write restored resource %s at stage write: %w", entry.Resource.ResourceID, writeErr))
		}
		if syncErr := tmp.Sync(); syncErr != nil {
			tmp.Close()
			os.Remove(tmpPath)
			return NewPackageError(PackageErrCodeResourceSnapshotInvalid, 500, fmt.Errorf("kernel: sync restored resource %s at stage write: %w", entry.Resource.ResourceID, syncErr))
		}
		if closeErr := tmp.Close(); closeErr != nil {
			os.Remove(tmpPath)
			return NewPackageError(PackageErrCodeResourceSnapshotInvalid, 500, fmt.Errorf("kernel: close restored resource %s at stage write: %w", entry.Resource.ResourceID, closeErr))
		}
		if renameErr := os.Rename(tmpPath, absOriginal); renameErr != nil {
			os.Remove(tmpPath)
			return NewPackageError(PackageErrCodeResourceSnapshotInvalid, 500, fmt.Errorf("kernel: rename restored resource %s at stage commit: %w", entry.Resource.ResourceID, renameErr))
		}
		if dirSyncErr := syncDir(filepath.Dir(absOriginal)); dirSyncErr != nil {
			return NewPackageError(PackageErrCodeResourceSnapshotInvalid, 500, fmt.Errorf("kernel: sync directory after restore %s at stage commit: %w", entry.Resource.ResourceID, dirSyncErr))
		}
		if verifyHash, verifyErr := hashFileContent(absOriginal); verifyErr != nil || verifyHash != entry.ContentHash {
			return NewPackageError(PackageErrCodeResourceSnapshotInvalid, 500, fmt.Errorf("kernel: resource %s restored file hash verification failed at stage post_verify: expected %s got %s err=%v", entry.Resource.ResourceID, entry.ContentHash, verifyHash, verifyErr))
		}
	}
	current, err := r.container.ResourceRepository.ListResources(ctx, extensionID)
	if err != nil {
		return err
	}
	for _, resource := range current {
		if err := r.container.ResourceRepository.DeleteResource(ctx, resource.ResourceID); err != nil {
			return err
		}
	}
	for _, entry := range snapshot.Entries {
		if err := r.container.ResourceRepository.PutResource(ctx, entry.Resource); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runtime) restorePackageMigrationState(ctx context.Context, point PackageRollbackPoint) error {
	if r.container.MigrationRepository == nil {
		return fmt.Errorf("kernel: migration repository unavailable for restore")
	}
	var snapshot packageMigrationStateSnapshot
	if err := json.Unmarshal([]byte(point.MigrationStateSnapshotJSON), &snapshot); err != nil {
		return fmt.Errorf("kernel: migration snapshot corrupt: %w", err)
	}
	if snapshot.Mode != "none" && snapshot.Mode != "repository" {
		return fmt.Errorf("kernel: migration snapshot mode invalid")
	}
	for i := range snapshot.Definitions {
		if err := r.container.MigrationRepository.SaveMigrationDefinition(ctx, &snapshot.Definitions[i]); err != nil {
			return err
		}
	}
	for i := range snapshot.Operations {
		entry := snapshot.Operations[i]
		if err := r.container.MigrationRepository.SaveMigrationOperation(ctx, &entry.Operation); err != nil {
			return err
		}
		for stepIndex := range entry.Steps {
			if err := r.container.MigrationRepository.SaveMigrationStep(ctx, &entry.Steps[stepIndex]); err != nil {
				return err
			}
		}
	}
	return r.restoreUserDataSnapshot(ctx, point)
}

func (r *Runtime) restoreUserDataSnapshot(ctx context.Context, point PackageRollbackPoint) error {
	if point.UserDataMigrationStateJSON == "" {
		return nil
	}
	if r.container.UserDataSnapshotStore == nil {
		return NewPackageError(PackageErrCodeUserDataSnapshotStoreUnavailable, 503, fmt.Errorf("kernel: user data snapshot required but user data snapshot store is unavailable for extension %s", point.ExtensionID))
	}
	operationID := point.SourceOperationID
	if operationID == "" {
		operationID = "restore-" + point.RollbackPointID
	}
	return r.container.UserDataSnapshotStore.RestoreUserDataFromSnapshot(ctx, point.ExtensionID, operationID, point.UserDataMigrationStateJSON)
}

type installationStandardColumns struct {
	LastOperationID     string
	CurrentGenerationID string
	CurrentVersionID    string
	CurrentArtifactID   string
}

func (r *Runtime) getInstallationStandardColumns(ctx context.Context, extensionID string) (installationStandardColumns, error) {
	var cols installationStandardColumns
	db := r.container.PackageRepository.DB()
	if db == nil {
		return cols, fmt.Errorf("kernel: installation database unavailable for standard columns")
	}
	err := db.QueryRowContext(ctx, `SELECT COALESCE(last_operation_id, ''), COALESCE(current_generation_id, ''), COALESCE(current_version_id, ''), COALESCE(current_artifact_id, '') FROM extension_installations WHERE extension_id = ?`, extensionID).Scan(&cols.LastOperationID, &cols.CurrentGenerationID, &cols.CurrentVersionID, &cols.CurrentArtifactID)
	if err != nil {
		if err == sql.ErrNoRows {
			return cols, fmt.Errorf("kernel: installation row missing for extension %s", extensionID)
		}
		return cols, fmt.Errorf("kernel: query installation standard columns: %w", err)
	}
	return cols, nil
}

func (r *Runtime) restoreForwardPackagePoint(ctx context.Context, point PackageRollbackPoint) error {
	if err := validatePackageSnapshot(point); err != nil {
		return err
	}
	var definition domain.ExtensionDefinition
	var modules []domain.ModuleDefinition
	var contributions []domain.ContributionDefinition
	var permissions struct {
		Requirements []sqlite.PermissionRequirement `json:"requirements"`
		Grants       []sqlite.PermissionGrant       `json:"grants"`
	}
	var scopes []sqlite.ScopeBinding
	if err := json.Unmarshal([]byte(point.DefinitionSnapshotJSON), &definition); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(point.ModuleSnapshotJSON), &modules); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(point.ContributionSnapshotJSON), &contributions); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(point.PermissionSnapshotJSON), &permissions); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(point.ScopeSnapshotJSON), &scopes); err != nil {
		return err
	}
	installation, err := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(point.ExtensionID))
	if err != nil {
		return err
	}
	artifact, err := r.container.PackageRepository.GetArtifact(ctx, point.ArtifactID)
	if err != nil {
		return err
	}
	installedTreeHash := package_security.ComputeDirHash(point.InstalledPath, r.container.PackageSecurity.GetHasher())
	if installedTreeHash == "" {
		return fmt.Errorf("kernel: forward recovery installed tree unavailable")
	}
	installation.InstalledVersion = definition.Version
	installation.PackageID = artifact.ArtifactID
	installation.EnablementState = domain.EnablementDisabled
	installation.UpdatedAt = time.Now().UTC()
	stdCols, stdErr := r.getInstallationStandardColumns(ctx, point.ExtensionID)
	if stdErr != nil {
		return stdErr
	}
	currentMetadata := installation.Metadata
	installation.Metadata = map[string]any{"installedPath": point.InstalledPath, "artifactId": artifact.ArtifactID, "archiveHash": artifact.ArchiveHash, "manifestHash": artifact.ManifestHash, "contentTreeHash": artifact.ContentTreeHash, "artifactHash": artifact.ArtifactHash, "installedTreeHash": installedTreeHash}
	if stdCols.LastOperationID != "" {
		installation.Metadata["lastOperationId"] = stdCols.LastOperationID
	}
	if stdCols.CurrentGenerationID != "" {
		installation.Metadata["generationId"] = stdCols.CurrentGenerationID
	}
	if stdCols.CurrentVersionID != "" {
		installation.Metadata["currentVersionId"] = stdCols.CurrentVersionID
	}
	if stdCols.CurrentArtifactID != "" {
		installation.Metadata["currentArtifactId"] = stdCols.CurrentArtifactID
	}
	for _, key := range []string{"ownerUserId", "scopeType", "scopeId"} {
		if value, exists := currentMetadata[key]; exists {
			installation.Metadata[key] = value
		}
	}
	err = r.container.TransactionManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := r.container.PermissionRepository.DeleteRequirements(txCtx, installation.ExtensionID); err != nil {
			return err
		}
		grants, err := r.container.PermissionRepository.ListGrants(txCtx, installation.ExtensionID)
		if err != nil {
			return err
		}
		for _, grant := range grants {
			if err := r.container.PermissionRepository.DeleteGrant(txCtx, installation.ExtensionID, grant.PermissionName); err != nil {
				return err
			}
		}
		if err := r.container.ScopeRepository.DeleteBindings(txCtx, installation.ExtensionID); err != nil {
			return err
		}
		if err := r.container.ContributionRepository.DeleteContributions(txCtx, installation.ExtensionID); err != nil {
			return err
		}
		if err := r.container.ModuleRepository.DeleteModules(txCtx, installation.ExtensionID); err != nil {
			return err
		}
		if err := r.container.DefinitionRepository.PutExtension(txCtx, definition); err != nil {
			return err
		}
		for _, module := range modules {
			if err := r.container.ModuleRepository.PutModule(txCtx, module); err != nil {
				return err
			}
		}
		for _, contribution := range contributions {
			if err := r.container.ContributionRepository.PutContribution(txCtx, contribution); err != nil {
				return err
			}
		}
		for _, requirement := range permissions.Requirements {
			if err := r.container.PermissionRepository.PutRequirement(txCtx, requirement); err != nil {
				return err
			}
		}
		for _, grant := range permissions.Grants {
			if err := r.container.PermissionRepository.PutGrant(txCtx, grant); err != nil {
				return err
			}
		}
		for _, scope := range scopes {
			if err := r.container.ScopeRepository.PutBinding(txCtx, scope); err != nil {
				return err
			}
		}
		if err := r.restorePackageRepositorySnapshots(txCtx, installation.ExtensionID, point, &installation); err != nil {
			return err
		}
		return r.container.InstallationRepository.PutInstallation(txCtx, installation)
	})
	if err != nil {
		return err
	}
	return r.restorePackageMigrationState(ctx, point)
}

func isPackageOperationalMetadataKey(key string) bool {
	switch key {
	case "installedPath", "artifactId", "archiveHash", "manifestHash", "contentTreeHash", "artifactHash", "installedTreeHash", "ownerUserId", "scopeType", "scopeId", "operationId", "generation", "lastOperationId", "generationId", "currentVersionId", "currentArtifactId":
		return true
	default:
		return false
	}
}

func sanitizePackageSnapshotMap(value map[string]any) (map[string]any, []string) {
	if value == nil {
		return map[string]any{}, nil
	}
	refs := []string{}
	cleaned := sanitizePackageSnapshotValue(value, "", &refs)
	result, ok := cleaned.(map[string]any)
	if !ok {
		return map[string]any{}, refs
	}
	return result, refs
}

func sanitizePackageSnapshotValue(value any, key string, refs *[]string) any {
	if isPackageSensitiveKey(key) {
		if ref := packageSecretReference(value); ref != "" {
			*refs = append(*refs, ref)
			return map[string]any{"secretRef": ref}
		}
		return "[REDACTED]"
	}
	switch current := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(current))
		for childKey, child := range current {
			result[childKey] = sanitizePackageSnapshotValue(child, childKey, refs)
		}
		return result
	case []any:
		result := make([]any, len(current))
		for i := range current {
			result[i] = sanitizePackageSnapshotValue(current[i], key, refs)
		}
		return result
	default:
		return value
	}
}

func isPackageSensitiveKey(key string) bool {
	lower := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	return strings.Contains(lower, "secret") || strings.Contains(lower, "token") || strings.Contains(lower, "password") || strings.Contains(lower, "credential") || strings.Contains(lower, "api_key") || strings.Contains(lower, "apikey") || strings.Contains(lower, "private_key")
}

func packageSecretReference(value any) string {
	if text, ok := value.(string); ok && (strings.HasPrefix(text, "secret://") || strings.HasPrefix(text, "secret-ref:") || strings.HasPrefix(text, "$secret:")) {
		return text
	}
	if object, ok := value.(map[string]any); ok {
		for _, key := range []string{"secretRef", "refId", "ref_id"} {
			if text, valid := object[key].(string); valid && text != "" {
				return text
			}
		}
	}
	return ""
}

func packageSnapshotDigest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func validatePackageResourceSnapshotEntry(entry packageResourceSnapshotEntry) error {
	if entry.Resource.ResourceID == "" {
		return fmt.Errorf("resource id missing")
	}
	if entry.ContentHash == "" {
		return fmt.Errorf("content hash missing")
	}
	if entry.ContentStorageReference == "" {
		return fmt.Errorf("content storage reference missing")
	}
	if entry.Size <= 0 {
		return fmt.Errorf("invalid size %d", entry.Size)
	}
	if entry.LogicalPath == "" && entry.OriginalPath == "" && entry.StorageReference == "" {
		return fmt.Errorf("no restore target path available")
	}
	if entry.ResourceHash == "" {
		return fmt.Errorf("resource hash missing")
	}
	raw, err := json.Marshal(entry.Resource)
	if err != nil {
		return fmt.Errorf("resource marshalling failed: %w", err)
	}
	if packageSnapshotDigest(raw) != entry.ResourceHash {
		return fmt.Errorf("resource hash mismatch")
	}
	return nil
}

func resolveResourceRestorePath(entry packageResourceSnapshotEntry) string {
	if entry.OriginalPath != "" {
		return entry.OriginalPath
	}
	if entry.StorageReference != "" {
		return entry.StorageReference
	}
	return entry.LogicalPath
}

func hashFileContent(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

func ComputeRollbackSnapshotRequirement(input RollbackSnapshotRequirementInput) RollbackSnapshotRequirement {
	req := RollbackSnapshotRequirement{
		ManifestNoDataChange: input.ManifestNoDataChange,
	}

	req.ConfigChanged = input.ConfigBeforeHash != "" && input.ConfigAfterHash != "" && input.ConfigBeforeHash != input.ConfigAfterHash
	req.ResourcesChanged = (input.ResourceBeforeTreeHash != "" && input.ResourceAfterTreeHash != "" && input.ResourceBeforeTreeHash != input.ResourceAfterTreeHash) ||
		len(input.ResourceSetDiff.Added) > 0 || len(input.ResourceSetDiff.Removed) > 0 || len(input.ResourceSetDiff.Changed) > 0
	req.UserDataChanged = input.UserDataBeforeHash != "" && input.UserDataAfterHash != "" && input.UserDataBeforeHash != input.UserDataAfterHash
	req.MigrationPlanPresent = input.MigrationPlan != nil
	req.MigrationDefinitionPresent = len(input.MigrationDefinitions) > 0
	req.MigrationOperationPresent = len(input.MigrationOperations) > 0

	configSourceMissing := (input.ConfigBeforeHash != "") != (input.ConfigAfterHash != "")
	resourceSourceMissing := (input.ResourceBeforeTreeHash != "") != (input.ResourceAfterTreeHash != "")
	userDataSourceMissing := (input.UserDataBeforeHash != "") != (input.UserDataAfterHash != "")
	missingSource := configSourceMissing || resourceSourceMissing || userDataSourceMissing

	anyChange := req.ConfigChanged || req.ResourcesChanged || req.UserDataChanged ||
		req.MigrationPlanPresent || req.MigrationDefinitionPresent || req.MigrationOperationPresent

	req.Required = anyChange || missingSource || !input.ManifestNoDataChange
	req.NoDataChange = !req.Required

	if req.Required {
		switch {
		case missingSource:
			req.Reason = "before/after evidence count mismatch, fail-closed"
		case !input.ManifestNoDataChange:
			req.Reason = "manifest does not declare no-data-change, fail-closed"
		case req.MigrationPlanPresent:
			req.Reason = "migration plan present"
		case req.MigrationDefinitionPresent:
			req.Reason = "migration definitions present"
		case req.MigrationOperationPresent:
			req.Reason = "migration operations present"
		case req.ConfigChanged:
			req.Reason = "config changed"
		case req.ResourcesChanged:
			req.Reason = "resources changed"
		case req.UserDataChanged:
			req.Reason = "user data changed"
		default:
			req.Reason = "changes detected"
		}
	} else {
		req.Reason = "no data change detected"
	}

	req.RequirementHash = computeSnapshotRequirementHash(req)
	return req
}

func computeRollbackSnapshotRequirementFromPoint(point PackageRollbackPoint) RollbackSnapshotRequirement {
	input := RollbackSnapshotRequirementInput{ManifestNoDataChange: true}
	corrupt := false
	if point.ConfigSnapshotJSON != "" {
		input.ConfigBeforeHash = packageSnapshotDigest([]byte(point.ConfigSnapshotJSON))
	}
	if point.ResourceSnapshotJSON != "" {
		input.ResourceBeforeTreeHash = packageSnapshotDigest([]byte(point.ResourceSnapshotJSON))
	}
	if point.UserDataMigrationStateJSON != "" {
		input.UserDataBeforeHash = packageSnapshotDigest([]byte(point.UserDataMigrationStateJSON))
	}
	if point.MigrationStateSnapshotJSON != "" {
		var migrationState packageMigrationStateSnapshot
		if json.Unmarshal([]byte(point.MigrationStateSnapshotJSON), &migrationState) != nil {
			corrupt = true
		} else {
			if migrationState.Mode != "" && migrationState.Mode != "none" {
				input.MigrationDefinitions = migrationState.Definitions
			}
			for i := range migrationState.Operations {
				input.MigrationOperations = append(input.MigrationOperations, migrationState.Operations[i].Operation)
			}
		}
	}
	req := ComputeRollbackSnapshotRequirement(input)
	if corrupt && req.NoDataChange {
		req.Required = true
		req.NoDataChange = false
		req.Reason = "migration state snapshot corrupt, fail-closed"
		req.RequirementHash = computeSnapshotRequirementHash(req)
	}
	return req
}

func computeSnapshotRequirementHash(req RollbackSnapshotRequirement) string {
	canonical := fmt.Sprintf(`{"required":%v,"configChanged":%v,"resourcesChanged":%v,"userDataChanged":%v,"migrationPlanPresent":%v,"migrationDefinitionPresent":%v,"migrationOperationPresent":%v,"migrationStateUnverified":%v,"manifestNoDataChange":%v,"noDataChange":%v}`,
		req.Required, req.ConfigChanged, req.ResourcesChanged, req.UserDataChanged,
		req.MigrationPlanPresent, req.MigrationDefinitionPresent, req.MigrationOperationPresent,
		req.MigrationStateUnverified, req.ManifestNoDataChange, req.NoDataChange)
	h := sha256.Sum256([]byte(canonical))
	return "sha256:" + hex.EncodeToString(h[:])
}

func computeVerifiedResourceTreeHash(resources []domain.ResourceOwnership, absExtRoot string) (string, error) {
	if len(resources) == 0 {
		return "", nil
	}
	type treeEntry struct {
		normalizedPath string
		size           int64
		contentHash    string
	}
	var entries []treeEntry
	for _, resource := range resources {
		originalPath := extractResourceStringField(resource, "originalPath")
		if originalPath == "" {
			originalPath = resource.Reference
		}
		if originalPath == "" {
			continue
		}
		absPath, absErr := filepath.Abs(originalPath)
		if absErr != nil {
			return "", fmt.Errorf("kernel: resolve resource path %s: %w", resource.ResourceID, absErr)
		}
		if !strings.HasPrefix(absPath, absExtRoot+string(filepath.Separator)) {
			return "", fmt.Errorf("kernel: resource %s path %s escapes ext root", resource.ResourceID, absPath)
		}
		relPath, relErr := filepath.Rel(absExtRoot, absPath)
		if relErr != nil {
			return "", fmt.Errorf("kernel: compute relative path for %s: %w", resource.ResourceID, relErr)
		}
		normalizedPath := filepath.ToSlash(strings.ToLower(relPath))

		info, statErr := os.Stat(absPath)
		if statErr != nil {
			return "", fmt.Errorf("kernel: resource %s file not found: %w", resource.ResourceID, statErr)
		}

		file, openErr := os.Open(absPath)
		if openErr != nil {
			return "", fmt.Errorf("kernel: open resource file %s: %w", resource.ResourceID, openErr)
		}
		hasher := sha256.New()
		if _, copyErr := io.Copy(hasher, file); copyErr != nil {
			file.Close()
			return "", fmt.Errorf("kernel: hash resource file %s: %w", resource.ResourceID, copyErr)
		}
		file.Close()
		contentHash := "sha256:" + hex.EncodeToString(hasher.Sum(nil))

		entries = append(entries, treeEntry{
			normalizedPath: normalizedPath,
			size:           info.Size(),
			contentHash:    contentHash,
		})
	}
	if len(entries) != len(resources) {
		return "", fmt.Errorf("kernel: some resources missing or unreadable for tree hash")
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].normalizedPath < entries[j].normalizedPath
	})
	hasher := sha256.New()
	for _, entry := range entries {
		hasher.Write([]byte(entry.normalizedPath))
		hasher.Write([]byte{0})
		hasher.Write([]byte(fmt.Sprintf("%d", entry.size)))
		hasher.Write([]byte{0})
		hasher.Write([]byte(entry.contentHash))
		hasher.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}
