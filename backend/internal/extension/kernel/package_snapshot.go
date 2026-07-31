package kernel

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
	Metadata      map[string]any `json:"metadata"`
	SchemaVersion string         `json:"schemaVersion"`
	CapturedAt    string         `json:"capturedAt"`
}

type packageResourceSnapshotEntry struct {
	Resource        domain.ResourceOwnership `json:"resource"`
	ResourceHash    string                   `json:"resourceHash"`
	RestoreStrategy string                   `json:"restoreStrategy"`
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
	Mode           string           `json:"mode"`
	Snapshots      []string         `json:"snapshots,omitempty"`
	Completed      []string         `json:"completed,omitempty"`
	AffectedTables []string         `json:"affectedTables,omitempty"`
	RecordCounts   map[string]int64 `json:"recordCounts,omitempty"`
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
	configJSON, err := json.Marshal(packageConfigSnapshot{Metadata: metadata, SchemaVersion: packageConfigSnapshotSchemaVersion, CapturedAt: installed.UpdatedAt.UTC().Format(time.RFC3339Nano)})
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
	sort.Slice(resources, func(i, j int) bool { return resources[i].ResourceID < resources[j].ResourceID })
	resourceSnapshot := packageResourceSnapshot{Entries: make([]packageResourceSnapshotEntry, 0, len(resources))}
	for _, resource := range resources {
		resource.Metadata, _ = sanitizePackageSnapshotMap(resource.Metadata)
		raw, marshalErr := json.Marshal(resource)
		if marshalErr != nil {
			return "", "", "", "", "", marshalErr
		}
		resourceSnapshot.Entries = append(resourceSnapshot.Entries, packageResourceSnapshotEntry{Resource: resource, ResourceHash: packageSnapshotDigest(raw), RestoreStrategy: "repository_upsert"})
	}
	resourceJSON, err := json.Marshal(resourceSnapshot)
	if err != nil {
		return "", "", "", "", "", err
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
	}
	for _, definition := range definitions {
		for _, domain := range definition.DataDomains {
			if domain.Namespace != "" {
				affectedTableSet[domain.Namespace] = struct{}{}
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
		if userState.RecordCounts != nil {
			userState.RecordCounts[operation.OperationID] = int64(len(steps))
		}
	}
	sort.Strings(userState.Snapshots)
	sort.Strings(userState.Completed)
	for table := range affectedTableSet {
		userState.AffectedTables = append(userState.AffectedTables, table)
	}
	sort.Strings(userState.AffectedTables)
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
	return nil
}

type installationStandardColumns struct {
	LastOperationID      string
	CurrentGenerationID  string
	CurrentVersionID     string
	CurrentArtifactID    string
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
