//go:build legacy_migration

package extension

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/migration"
	"gorm.io/gorm"
)

func packageTestService(t *testing.T) (*PackageService, *Registry, *AgentSkillService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "-")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	migrations := []migration.Migration{migration.ExtensionsMigration(), migration.PluginRuntimeMigration(), migration.ExtensionWorkshopMigration(), migration.ExtensionAgentSkillsMigration(), migration.ExtensionAgentSkillTraceMigration(), migration.ExtensionPackagesMigration(), migration.ExtensionScopeBindingsMigration(), migration.ExtensionOwnedResourcesMigration(), migration.ExtensionPackageRecoveryMigration(), migration.ExtensionArtifactRecoveryMigration(), migration.ExtensionScheduleSourceMigration(), migration.ExtensionScheduleOwnershipRepairMigration()}
	if err := (migration.Runner{DB: db, SkipBackup: true}).Apply(migrations); err != nil {
		t.Fatal(err)
	}
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(db)
	registry := NewRegistry("1.0.0", validator, repository)
	permissions := NewPermissionEvaluator(repository)
	executor := NewExecutor(registry, validator, permissions, repository)
	compiler := NewWorkflowCompiler(registry)
	workflowExecutor := NewWorkflowExecutor(BuildWorkflowAdapters(executor, &WorkflowHostAdapter{}), validator)
	workshop := NewWorkshopService(NewWorkshopRepository(db), NewWorkshopGenerator(nil, registry), compiler, workflowExecutor, validator, registry, executor)
	agentSkills := NewAgentSkillService(repository, registry, validator)
	service := NewPackageService(repository, registry, validator, compiler, workshop.installer, agentSkills)
	return service, registry, agentSkills, db
}

func packageWorkflowArchive(t *testing.T, version string, extra map[string][]byte) []byte {
	t.Helper()
	input := json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"name":{"type":"string"}},"required":["name"]}`)
	output := json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"message":{"type":"string"}},"required":["message"]}`)
	config := json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{}}`)
	manifest := Manifest{Schema: "https://schemas.amitia.dev/extensions/v1/manifest.schema.json", APIVersion: "extensions.amitia.dev/v1alpha1", Kind: "Skill", Metadata: ManifestMetadata{ID: "dev.local.package.greeting", Name: "greeting", Version: version, Description: "Generate a greeting", Author: "Local", License: "MIT"}, Compatibility: ManifestCompatibility{EngineMin: "1.0.0", EngineMaxExclusive: "2.0.0"}, Entry: SkillEntry{Kind: "workflow", Path: "workflows/main.json"}, Capabilities: []string{}, Triggers: []SkillTrigger{TriggerManual}, Execution: ManifestExecution{TimeoutMS: 30000, Retryable: true, Idempotent: true}, InputSchema: input, OutputSchema: output, ConfigSchema: config, DefaultConfig: json.RawMessage(`{}`), Enabled: false, AllowManual: true}
	manifestRaw, _ := json.Marshal(manifest)
	workflow := WorkflowDefinition{SchemaVersion: "1.0.0", Steps: []WorkflowStep{{ID: "result", Type: "transform", Input: json.RawMessage(`{"op":"pick","value":{"message":"hello"},"fields":["message"]}`), OnError: WorkflowErrorPolicy{Mode: "fail"}}}, Output: json.RawMessage(`{"$ref":"steps.result"}`), Limits: DefaultWorkflowLimits()}
	workflowRaw, _ := json.Marshal(workflow)
	files := map[string][]byte{"manifest.json": manifestRaw, "workflows/main.json": workflowRaw, "schemas/input.schema.json": input, "schemas/output.schema.json": output, "schemas/config.schema.json": config, "LICENSE": []byte("MIT\n")}
	for name, content := range extra {
		files[name] = content
	}
	files["checksums.sha256"] = buildChecksums(files)
	raw, err := stablePackageZIP(files)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestPackageArchiveSecurityAndChecksums(t *testing.T) {
	limits := DefaultPackageLimits()
	for name, entries := range map[string]map[string][]byte{
		"traversal":  {"../escape": []byte("x")},
		"drive":      {"C:/escape": []byte("x")},
		"collision":  {"A.txt": []byte("a"), "a.txt": []byte("b")},
		"executable": {"payload.txt": append([]byte{'M', 'Z'}, bytes.Repeat([]byte{0}, 16)...)},
		"nested":     {"nested.zip": []byte("PK")},
	} {
		t.Run(name, func(t *testing.T) {
			raw := packageUnsafeZIP(t, entries, nil)
			if _, _, err := readPackageZIP(raw, limits); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
	mode := uint32(os.ModeSymlink | 0o777)
	if _, _, err := readPackageZIP(packageUnsafeZIP(t, map[string][]byte{"link": []byte("target")}, &mode), limits); err == nil {
		t.Fatal("symlink should be rejected")
	}
	bombLimits := limits
	bombLimits.MaxCompressionRatio = 2
	if _, _, err := readPackageZIP(packageUnsafeZIP(t, map[string][]byte{"bomb.txt": bytes.Repeat([]byte("a"), 4096)}, nil), bombLimits); err == nil {
		t.Fatal("compression bomb should be rejected")
	}
	files := map[string][]byte{"manifest.json": []byte(`{}`), "a.txt": []byte("a")}
	if err := validateChecksums(files); asExtensionError(err).Code != ErrPackageChecksumMissing {
		t.Fatalf("unexpected missing checksum error: %v", err)
	}
	files["checksums.sha256"] = buildChecksums(files)
	files["a.txt"] = []byte("changed")
	if err := validateChecksums(files); asExtensionError(err).Code != ErrPackageChecksumMismatch {
		t.Fatalf("unexpected mismatch error: %v", err)
	}
}

func TestPackagePreviewExecutesAssertions(t *testing.T) {
	service, _, _, db := packageTestService(t)
	tests := []WorkshopTestCase{{ID: "wrong-output", Name: "错误输出", Mode: string(WorkflowDryRun), Input: json.RawMessage(`{"name":"A"}`), Config: json.RawMessage(`{}`), Assertions: []TestAssertion{{Type: "equals", Path: "output.message", Expected: "not-hello"}}}}
	testsRaw, _ := json.Marshal(tests)
	preview, err := service.PreviewImport(context.Background(), PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "greeting.amitiax", Raw: packageWorkflowArchive(t, "1.0.0", map[string][]byte{"tests/cases.json": testsRaw})})
	if err != nil {
		t.Fatal(err)
	}
	if preview.TestStatus != "failed" || preview.TestReport == nil || preview.TestReport.FailedCount != 1 || preview.Compatible || !containsString(preview.Errors, ErrPackageTestFailed) {
		t.Fatalf("failed assertion was not enforced: %+v", preview)
	}
	if _, err := service.installLegacyPackage(context.Background(), InstallPackageRequest{SessionID: preview.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true}); asExtensionError(err).Code != ErrPackageInstallFailed {
		t.Fatalf("install should re-run and reject failed assertions: %v", err)
	}
	var operation packageOperationRecord
	if err := db.Order("created_at DESC").First(&operation).Error; err != nil {
		t.Fatal(err)
	}
	if operation.ExtensionID != preview.ID || operation.Status != "failed" || operation.ErrorCode != ErrPackageInstallFailed || operation.CompletedAt == "" {
		t.Fatalf("failed validation operation was not tracked: %+v", operation)
	}
}

func TestPackageRecoveryReconcilesActivatedOperation(t *testing.T) {
	service, _, _, db := packageTestService(t)
	ctx := context.Background()
	preview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "greeting.amitiax", Raw: packageWorkflowArchive(t, "1.0.0", nil)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: preview.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	operation := packageOperationRecord{ID: uuid.NewString(), ExtensionID: preview.ID, ExtensionVersion: preview.Version, Operation: string(PackageOperationUpgrade), PreviousVersion: "0.9.0", TargetVersion: preview.Version, UserID: "1", ScopeType: "global", Status: "activating", TraceID: uuid.NewString(), CreatedAt: now}
	if err := db.Create(&operation).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.recoverPackageOperations(ctx); err != nil {
		t.Fatal(err)
	}
	var recovered packageOperationRecord
	if err := db.Where("id = ?", operation.ID).First(&recovered).Error; err != nil || recovered.Status != "succeeded" || recovered.CompletedAt == "" {
		t.Fatalf("unexpected recovered operation: %+v %v", recovered, err)
	}
	var version packageVersionRecord
	if err := db.Where("extension_id = ? AND version = ?", preview.ID, preview.Version).First(&version).Error; err != nil || version.ArtifactStatus != "active" || version.ActivationStatus != "active" {
		t.Fatalf("unexpected recovered version: %+v %v", version, err)
	}
}

func TestPackageSignatureTrustDoesNotGrant(t *testing.T) {
	service, _, _, db := packageTestService(t)
	raw := packageWorkflowArchive(t, "1.0.0", nil)
	files, _, err := readPackageZIP(raw, DefaultPackageLimits())
	if err != nil {
		t.Fatal(err)
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	fingerprintHash := packageHash(publicKey)
	fingerprint := "sha256:" + fingerprintHash
	digest := "sha256:" + packageCanonicalDigest(files)
	document := packageSignatureDocument{Algorithm: "ed25519", KeyID: fingerprint, PublicKey: base64.StdEncoding.EncodeToString(publicKey), Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, []byte(digest))), SignedDigest: digest, DisplayName: "Test signer"}
	files["signature.json"], _ = json.Marshal(document)
	signedRaw, _ := stablePackageZIP(files)
	preview, err := service.PreviewImport(context.Background(), PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "greeting.amitiax", Raw: signedRaw})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Signature.Status != PackageSignatureUntrusted {
		t.Fatalf("unexpected signature: %+v", preview.Signature)
	}
	if err := service.repository.SetPackageSignerTrust(context.Background(), fingerprint, true); err != nil {
		t.Fatal(err)
	}
	preview, err = service.PreviewImport(context.Background(), PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "greeting.amitiax", Raw: signedRaw})
	if err != nil || preview.Signature.Status != PackageSignatureTrusted {
		t.Fatalf("trusted signature missing: %+v %v", preview.Signature, err)
	}
	var grants int64
	if err := db.Model(&grantRecord{}).Count(&grants).Error; err != nil || grants != 0 {
		t.Fatalf("signature created grant: %d %v", grants, err)
	}
}

func TestPackageWorkflowLifecycle(t *testing.T) {
	service, registry, _, db := packageTestService(t)
	ctx := context.Background()
	initialRaw := packageWorkflowArchive(t, "1.0.0", nil)
	preview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "greeting-1.0.0.amitiax", Raw: initialRaw})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: preview.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Enabled {
		t.Fatal("new install must be disabled")
	}
	registered, err := registry.Get(ctx, preview.ID)
	if err != nil || registered.Definition.Enabled {
		t.Fatalf("registry install invalid: %+v %v", registered, err)
	}
	var initialArtifact packageArtifactRecord
	if err := db.Where("extension_id = ? AND extension_version = ?", preview.ID, "1.0.0").First(&initialArtifact).Error; err != nil {
		t.Fatal(err)
	}
	exportedFiles, err := service.exportAmitiaxFiles(initialArtifact)
	if err != nil || len(exportedFiles) == 0 {
		t.Fatalf("legacy export files failed: %v", err)
	}
	upgradePreview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "greeting-1.1.0.amitiax", Raw: packageWorkflowArchive(t, "1.1.0", map[string][]byte{"docs/README.md": []byte("new")})})
	if err != nil || upgradePreview.Conflict != PackageConflictUpgrade || upgradePreview.UpgradeDiff == nil || upgradePreview.RollbackVersion != "1.0.0" {
		t.Fatalf("upgrade preview invalid: %+v %v", upgradePreview, err)
	}
	upgradeResult, err := service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: upgradePreview.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true, ConfirmVersionChange: true, ExpectedExtensionID: preview.ID})
	if err != nil || upgradeResult.Operation != PackageOperationUpgrade {
		t.Fatal(err)
	}
	versions, err := service.repository.ListPackageVersions(ctx, preview.ID, "1", "global", "")
	if err != nil || len(versions) != 2 || !versions[0].Active {
		t.Fatalf("versions invalid: %+v %v", versions, err)
	}
	fromVersion, err := service.repository.GetPackageVersion(ctx, preview.ID, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	toVersion, err := service.repository.GetPackageVersion(ctx, preview.ID, "1.1.0")
	if err != nil {
		t.Fatal(err)
	}
	diff := packageVersionDiffRecords(preview.ID, fromVersion, toVersion, nil, nil)
	if diff.FromVersion != "1.0.0" {
		t.Fatalf("diff failed: %+v", diff)
	}
	if _, err := service.rollbackLegacyPackage(ctx, preview.ID, "1.0.0", "1", "global", ""); err != nil {
		t.Fatal(err)
	}
	uninstallPreview, err := service.legacyPreviewUninstall(ctx, preview.ID, "1", "global", "")
	if err != nil || uninstallPreview.CurrentVersion != "1.0.0" || !uninstallPreview.ArtifactArchived {
		t.Fatalf("uninstall preview invalid: %+v %v", uninstallPreview, err)
	}
	if _, err := service.uninstallLegacyPackage(ctx, preview.ID, "1", "global", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Get(ctx, preview.ID); err == nil {
		t.Fatal("uninstalled skill remains registered")
	}
	reinstallPreview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "greeting-1.0.0.amitiax", Raw: initialRaw})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: reinstallPreview.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true}); err != nil {
		t.Fatal(err)
	}
	var runs int64
	if err := db.Model(&runRecord{}).Count(&runs).Error; err != nil {
		t.Fatal(err)
	}
}

func TestPackageAgentSkillsLifecycle(t *testing.T) {
	service, registry, agentSkills, _ := packageTestService(t)
	ctx := context.Background()
	skill := []byte("---\nname: code-review\ndescription: Review code when a user asks for quality or security feedback.\nlicense: MIT\nmetadata:\n  version: 1.0.0\n---\nReview carefully.\n")
	raw := packageUnsafeZIP(t, map[string][]byte{"code-review/SKILL.md": skill, "code-review/scripts/check.py": []byte("print('disabled')")}, nil)
	preview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "code-review.zip", Raw: raw})
	if err != nil || preview.Scripts != 1 {
		t.Fatalf("agent preview failed: %+v %v", preview, err)
	}
	result, err := service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: preview.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true, ConfirmScripts: true})
	if err != nil {
		t.Fatal(err)
	}
	registered, err := registry.Get(ctx, result.ExtensionID)
	if err != nil || registered.Handler != nil {
		t.Fatalf("instructions runtime contract invalid: %+v %v", registered, err)
	}
	if err := agentSkills.Enable(ctx, ExecutionScope{UserID: "1"}, result.ExtensionID); err != nil {
		t.Fatal(err)
	}
	catalog, err := agentSkills.ResolveCatalog(ctx, ExecutionScope{UserID: "1"})
	if err != nil || len(catalog) != 1 {
		t.Fatalf("catalog refresh failed: %+v %v", catalog, err)
	}
	var agentArtifact packageArtifactRecord
	if err := service.repository.db.WithContext(ctx).Where("extension_id = ?", result.ExtensionID).First(&agentArtifact).Error; err != nil {
		t.Fatal(err)
	}
	files, err := service.exportAgentSkillsFiles(agentArtifact, "code-review")
	if err != nil || files["code-review/SKILL.md"] == nil {
		t.Fatalf("agentskills export invalid: %v", err)
	}
}

func TestUnifiedLifecycleKeepsInstructionsStateConsistent(t *testing.T) {
	packages, registry, agentSkills, db := packageTestService(t)
	ctx := context.Background()
	skill := []byte("---\nname: lifecycle-check\ndescription: Check lifecycle consistency when users request it.\nlicense: MIT\nmetadata:\n  version: 1.0.0\n---\nCheck lifecycle state.\n")
	raw := packageUnsafeZIP(t, map[string][]byte{"lifecycle-check/SKILL.md": skill}, nil)
	preview, err := packages.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "lifecycle-check.zip", Raw: raw})
	if err != nil {
		t.Fatal(err)
	}
	installed, err := packages.installLegacyPackage(ctx, InstallPackageRequest{SessionID: preview.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true})
	if err != nil {
		t.Fatal(err)
	}
	validator, _ := NewSchemaValidator()
	repository := NewRepository(db)
	service := NewService(registry, nil, repository, validator)
	service.AttachLifecycleService(NewExtensionLifecycleService(registry, repository, agentSkills))
	scope := ExecutionScope{UserID: "1"}
	if err := service.EnableSkill(ctx, scope, installed.ExtensionID); err != nil {
		t.Fatal(err)
	}
	catalog, err := agentSkills.ResolveCatalog(ctx, scope)
	if err != nil || len(catalog) != 1 || catalog[0].ExtensionID != installed.ExtensionID {
		t.Fatalf("generic enable did not refresh catalog: %+v %v", catalog, err)
	}
	if err := agentSkills.Disable(ctx, scope, installed.ExtensionID); err != nil {
		t.Fatal(err)
	}
	detail, err := service.GetSkill(ctx, scope, installed.ExtensionID)
	if err != nil || detail.Enabled {
		t.Fatalf("agent disable did not update generic state: %+v %v", detail, err)
	}
}

func TestPackageSessionIsolationConflictAndSecretProtection(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	ctx := context.Background()
	raw := packageWorkflowArchive(t, "1.0.0", nil)
	preview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "greeting.amitiax", Raw: raw})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: preview.SessionID, UserID: "2", ScopeType: "global", ConfirmUnsigned: true}); asExtensionError(err).Code != ErrPackageImportSessionExpired {
		t.Fatalf("cross-user session was accepted: %v", err)
	}
	if _, err := service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: preview.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: preview.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true}); asExtensionError(err).Code != ErrPackageImportSessionConsumed {
		t.Fatalf("consumed session was reused: %v", err)
	}
	conflict, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "greeting.amitiax", Raw: packageWorkflowArchive(t, "1.0.0", map[string][]byte{"docs/README.md": []byte("different")})})
	if err != nil || conflict.Conflict != PackageConflictDifferent {
		t.Fatalf("same-version content conflict missing: %+v %v", conflict, err)
	}
	if err := scanPackageExportSecrets(map[string][]byte{"README.md": []byte("api_key=sk-testvalue123456")}); asExtensionError(err).Code != ErrPackageSecretDetected {
		t.Fatalf("secret export was accepted: %v", err)
	}
}

func TestPackageConfigMigrationAndDependencyGuard(t *testing.T) {
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	schema := json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"kept":{"type":"string"},"added":{"type":"integer","default":2}},"required":["kept","added"]}`)
	migrated, err := migratePackageConfig(json.RawMessage(`{"kept":"yes","removed":true}`), schema, json.RawMessage(`{}`), validator)
	if err != nil || string(migrated) != `{"added":2,"kept":"yes"}` {
		t.Fatalf("config migration invalid: %s %v", migrated, err)
	}
	missingSchema := json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","properties":{"requiredValue":{"type":"string"}},"required":["requiredValue"]}`)
	if _, err := migratePackageConfig(json.RawMessage(`{}`), missingSchema, json.RawMessage(`{}`), validator); asExtensionError(err).Code != ErrPackageConfigMigrationRequired {
		t.Fatalf("missing required config was accepted: %v", err)
	}
	service, _, _, db := packageTestService(t)
	ctx := context.Background()
	preview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "greeting.amitiax", Raw: packageWorkflowArchive(t, "1.0.0", nil)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: preview.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	dependent := extensionRecord{ID: uuid.NewString(), ExtensionID: "dev.local.dependent", Kind: "Skill", Name: "dependent", CurrentVersion: "1.0.0", Source: string(SkillSourceWorkflow), Enabled: 0, ManifestJSON: "{}", NormalizedManifestJSON: "{}", CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&dependent).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&packageDependencyRecord{ExtensionID: dependent.ExtensionID, ExtensionVersion: dependent.CurrentVersion, DependencyID: preview.ID, Required: 1, CreatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := service.uninstallLegacyPackage(ctx, preview.ID, "1", "global", ""); asExtensionError(err).Code != ErrPackageDependencyInUse {
		t.Fatalf("dependency guard missing: %v", err)
	}
}

func packageUnsafeZIP(t *testing.T, entries map[string][]byte, mode *uint32) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range entries {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		if mode != nil {
			header.SetMode(os.FileMode(*mode))
		}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
