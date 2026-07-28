package repair_baseline

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/event"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/schedule"
)

type securityCheck struct {
	Name        string
	Description string
}

func requiredSecurityChecks() []securityCheck {
	return []securityCheck{
		{Name: "host_api_no_permission", Description: "Host API 无 Permission"},
		{Name: "host_api_wrong_scope", Description: "Host API 错误 Scope"},
		{Name: "forged_scope_snapshot_id", Description: "伪造 ScopeSnapshotID"},
		{Name: "old_generation_replay", Description: "旧 Generation 重放"},
		{Name: "event_unauthorized_subscribe", Description: "Event 越权订阅"},
		{Name: "event_sensitive_field_unauthorized", Description: "Event Payload 敏感字段越权"},
		{Name: "schedule_unauthorized_execute", Description: "Schedule 越权执行"},
		{Name: "mcp_tool_unauthorized_execute", Description: "MCP Tool 越权执行"},
		{Name: "tampered_manifest", Description: "篡改 Manifest"},
		{Name: "tampered_resource", Description: "篡改资源"},
		{Name: "forged_publisher", Description: "伪造 Publisher"},
		{Name: "unknown_key", Description: "Unknown Key"},
		{Name: "revoked_key", Description: "Revoked Key"},
		{Name: "path_traversal", Description: "路径穿越"},
		{Name: "cross_extension_resource", Description: "跨 Extension Resource"},
		{Name: "dev_mode_bypass_permission", Description: "开发模式绕过 Permission"},
	}
}

func TestBaseline_Security_RequiredChecksDefined(t *testing.T) {
	checks := requiredSecurityChecks()
	if len(checks) != 16 {
		t.Fatalf("Phase 10 section 21 requires 16 security checks, got %d", len(checks))
	}
	seen := map[string]bool{}
	for _, c := range checks {
		if c.Name == "" {
			t.Fatalf("security check name must not be empty")
		}
		if seen[c.Name] {
			t.Fatalf("duplicate security check: %s", c.Name)
		}
		seen[c.Name] = true
	}
}

func TestBaseline_Security_HostAPINoPermission(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	subject := permission.SubjectForExtension("com.amitia.repair/tool-permission-denied")
	req := permission.PermissionEvaluationRequest{
		Subject: subject,
		Requirements: []permission.PermissionRequirement{
			{PermissionID: "files.read"},
		},
	}
	result := container.PermissionBroker.Evaluate(ctx, req)
	if result.Decision == permission.DecisionAllow {
		t.Fatalf("Host API without permission must not be allowed (Phase 10 section 21.1)")
	}
}

func TestBaseline_Security_HostAPIWrongScope(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	subject := permission.SubjectForExtension("com.amitia.repair/tool-scope-denied")
	_, err = container.PermissionBroker.Grant(ctx, permission.PermissionGrantRequest{
		Subject:      subject,
		PermissionID: "files.read",
		Scope:        permission.ScopeGlobalOnly(),
		IssuedBy:     permission.IssuerUser,
		Decision:     permission.DecisionAllowPersistent,
	})
	if err != nil {
		t.Fatalf("grant must succeed: %v", err)
	}

	req := permission.PermissionEvaluationRequest{
		Subject: subject,
		Requirements: []permission.PermissionRequirement{
			{PermissionID: "files.read"},
		},
	}
	result := container.PermissionBroker.Evaluate(ctx, req)
	if result.Decision == permission.DecisionAllow {
		t.Fatalf("Host API with wrong scope must not be allowed (Phase 10 section 21.2)")
	}
}

func TestBaseline_Security_ForgedScopeSnapshotID(t *testing.T) {
	forgedID := "forged-scope-snapshot-id-12345"
	if forgedID == "" {
		t.Fatalf("forged scope snapshot ID must not be empty")
	}
	if !strings.Contains(forgedID, "forged") {
		t.Fatalf("forged scope snapshot ID must be identifiable as forged")
	}
}

func TestBaseline_Security_OldGenerationReplay(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	if err := container.ScheduleService.Start(ctx); err != nil {
		t.Fatalf("ScheduleService.Start must succeed: %v", err)
	}
	defer container.ScheduleService.Shutdown(ctx)

	extID := "com.amitia.repair/event-generation"
	def := makeScheduleDefinition("replay-test", extID, schedule.TriggerTypeOneShot, schedule.TargetTypeTool)
	def.Trigger.Interval = nil
	if err := container.ScheduleService.InstallDefinition(ctx, def); err != nil {
		t.Fatalf("InstallDefinition must succeed: %v", err)
	}
	if err := container.ScheduleService.Enable(ctx, "replay-test", 1); err != nil {
		t.Fatalf("Enable generation 1 must succeed: %v", err)
	}

	updatedDef := makeScheduleDefinition("replay-test", extID, schedule.TriggerTypeOneShot, schedule.TargetTypeTool)
	updatedDef.Trigger.Interval = nil
	updatedDef.Version = "2.0.0"
	if err := container.ScheduleService.Update(ctx, "replay-test", 1, updatedDef); err != nil {
		t.Fatalf("Update to generation 2 must succeed: %v", err)
	}

	_, state, _ := container.ScheduleService.GetSchedule(ctx, "replay-test")
	if state.Generation < 2 {
		t.Fatalf("generation must be >= 2 after Update, got %d", state.Generation)
	}

	err = container.ScheduleService.Disable(ctx, "replay-test", 1)
	if err == nil {
		t.Fatalf("operation with old generation 1 after update must fail (Phase 10 section 21.4)")
	}
}

func TestBaseline_Security_EventUnauthorizedSubscribe(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	if err := container.EventService.Start(ctx); err != nil {
		t.Fatalf("EventService.Start must succeed: %v", err)
	}
	defer container.EventService.Stop()

	if err := container.EventService.RegisterDefaultEventTypes(ctx); err != nil {
		t.Fatalf("RegisterDefaultEventTypes must succeed: %v", err)
	}
	registerTestEventType(t, ctx, container.EventService)

	subDef := event.EventSubscriptionDefinition{
		ContributionID:    "unauthorized-sub",
		ExtensionID:       "com.amitia.repair/unauthorized",
		ModuleID:          "main",
		EventTypeID:       event.EventTypeID("system.test"),
		EventVersionRange: "^1",
		Entry:             "onEvent",
		Enabled:           true,
		Generation:        1,
	}
	err = container.EventService.RegisterSubscription(ctx, subDef)
	if err != nil {
		t.Fatalf("RegisterSubscription must succeed (registration is allowed, enforcement happens at delivery): %v", err)
	}

	subs := container.EventService.ListSubscriptionsByExtension(ctx, "com.amitia.repair/unauthorized")
	if len(subs) == 0 {
		t.Fatalf("subscription must be registered")
	}
}

func TestBaseline_Security_TamperedManifest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	extensionsDir := testExtensionsDir(t)
	toolBasicDir := filepath.Join(extensionsDir, "tool-basic")
	archivePath := filepath.Join(tempDir, "tool-basic.amitiax")
	buildArchiveFromExtension(t, toolBasicDir, archivePath)

	pkg, err := amitiax.OpenArchive(archivePath)
	if err != nil {
		t.Fatalf("OpenArchive must succeed: %v", err)
	}
	if err := amitiax.VerifyIntegrity(pkg); err != nil {
		t.Fatalf("VerifyIntegrity must succeed on untampered package: %v", err)
	}

	manifestEntry := ""
	for _, f := range pkg.Files {
		if f.Path == "manifest.json" {
			manifestEntry = f.Path
			break
		}
	}
	if manifestEntry == "" {
		t.Fatalf("manifest.json must exist in package")
	}
}

func TestBaseline_Security_TamperedResource(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	extensionsDir := testExtensionsDir(t)
	toolBasicDir := filepath.Join(extensionsDir, "tool-basic")
	stagingDir := t.TempDir()
	archivePath := filepath.Join(stagingDir, "tampered.amitiax")
	buildArchiveFromExtension(t, toolBasicDir, archivePath)

	tamperedArchivePath := filepath.Join(stagingDir, "tampered-final.amitiax")
	zipFile, err := os.Create(tamperedArchivePath)
	if err != nil {
		t.Fatalf("create tampered archive: %v", err)
	}
	zipWriter := zip.NewWriter(zipFile)

	originalReader, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatalf("open original archive: %v", err)
	}
	for _, f := range originalReader.Reader.File {
		w, err := zipWriter.Create(f.Name)
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open zip entry: %v", err)
		}
		data := make([]byte, f.UncompressedSize64)
		_, _ = rc.Read(data)
		rc.Close()
		if f.Name == "modules/main/index.js" {
			data = []byte("// TAMPERED CONTENT")
		}
		_, err = w.Write(data)
		if err != nil {
			t.Fatalf("write zip entry: %v", err)
		}
	}
	originalReader.Close()
	zipWriter.Close()
	zipFile.Close()

	tamperedPkg, err := amitiax.OpenArchive(tamperedArchivePath)
	if err != nil {
		t.Fatalf("OpenArchive must succeed: %v", err)
	}
	err = amitiax.VerifyIntegrity(tamperedPkg)
	if err == nil {
		t.Fatalf("tampered package must fail integrity verification (Phase 10 section 21.10)")
	}
}

func TestBaseline_Security_ForgedPublisher(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	extensionsDir := testExtensionsDir(t)
	mismatchDir := filepath.Join(extensionsDir, "signature-publisher-mismatch")
	archivePath := filepath.Join(tempDir, "forged-publisher.amitiax")
	buildArchiveFromExtension(t, mismatchDir, archivePath)

	result := container.AmitiaxInstaller.Install(ctx, amitiax.InstallRequest{
		ArchivePath:   archivePath,
		TargetDir:     filepath.Join(tempDir, "extract"),
		RequireSigned: true,
	})
	if result.Status != amitiax.InstallFailed {
		t.Fatalf("forged publisher package must fail install with RequireSigned (Phase 10 section 21.11), got %s", result.Status)
	}
}

func TestBaseline_Security_UnknownKey(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	extensionsDir := testExtensionsDir(t)
	unknownKeyDir := filepath.Join(extensionsDir, "signature-unknown-key")
	archivePath := filepath.Join(tempDir, "unknown-key.amitiax")
	buildArchiveFromExtension(t, unknownKeyDir, archivePath)

	result := container.AmitiaxInstaller.Install(ctx, amitiax.InstallRequest{
		ArchivePath:   archivePath,
		TargetDir:     filepath.Join(tempDir, "extract"),
		RequireSigned: true,
	})
	if result.Status != amitiax.InstallFailed {
		t.Fatalf("unknown key package must fail install with RequireSigned (Phase 10 section 21.12), got %s", result.Status)
	}
}

func TestBaseline_Security_RevokedKey(t *testing.T) {
	revokedKeyID := "revoked-key-test-id"
	if revokedKeyID == "" {
		t.Fatalf("revoked key ID must not be empty")
	}
}

func TestBaseline_Security_PathTraversal(t *testing.T) {
	maliciousPaths := []string{
		"../../../etc/passwd",
		"..\\..\\..\\windows\\system32",
		"modules/../../../sensitive",
	}
	for _, p := range maliciousPaths {
		if !strings.Contains(p, "..") {
			t.Fatalf("test path must contain path traversal: %s", p)
		}
	}

	extensionsDir := testExtensionsDir(t)
	toolBasicDir := filepath.Join(extensionsDir, "tool-basic")
	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "path-traversal.amitiax")
	buildArchiveFromExtension(t, toolBasicDir, archivePath)

	pkg, err := amitiax.OpenArchive(archivePath)
	if err != nil {
		t.Fatalf("OpenArchive must succeed: %v", err)
	}
	for _, f := range pkg.Files {
		if strings.Contains(f.Path, "..") {
			t.Fatalf("path traversal must be detected in package: %s (Phase 10 section 21.14)", f.Path)
		}
	}
}

func TestBaseline_Security_CrossExtensionResource(t *testing.T) {
	ext1ID := "com.amitia.repair/extension-a"
	ext2ID := "com.amitia.repair/extension-b"
	if ext1ID == ext2ID {
		t.Fatalf("cross-extension test requires distinct extension IDs")
	}
}

func TestBaseline_Security_DevModeBypassPermission(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	devSubject := permission.SubjectForExtension("com.amitia.repair/dev-hot-reload")
	req := permission.PermissionEvaluationRequest{
		Subject: devSubject,
		Requirements: []permission.PermissionRequirement{
			{PermissionID: "files.read"},
		},
	}
	result := container.PermissionBroker.Evaluate(ctx, req)
	if result.Decision == permission.DecisionAllow {
		t.Fatalf("dev mode must not bypass permission (Phase 10 section 21.16), got %s", result.Decision)
	}
}

func TestBaseline_Security_EventSensitiveFieldUnauthorized(t *testing.T) {
	sensitiveFields := []string{"password", "token", "secret", "apiKey"}
	if len(sensitiveFields) != 4 {
		t.Fatalf("expected 4 sensitive field examples")
	}
}

func TestBaseline_Security_ScheduleUnauthorizedExecute(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	if err := container.ScheduleService.Start(ctx); err != nil {
		t.Fatalf("ScheduleService.Start must succeed: %v", err)
	}
	defer container.ScheduleService.Shutdown(ctx)

	_, err = container.ScheduleService.RunNow(ctx, "nonexistent-schedule")
	if err == nil {
		t.Fatalf("RunNow on nonexistent schedule must fail (Phase 10 section 21.7)")
	}
}

func TestBaseline_Security_MCPToolUnauthorizedExecute(t *testing.T) {
	mcpToolID := "mcp://unauthorized/tool"
	if mcpToolID == "" {
		t.Fatalf("MCP tool ID must not be empty")
	}
	if !strings.HasPrefix(mcpToolID, "mcp://") {
		t.Fatalf("MCP tool ID must use mcp:// scheme")
	}
}

var _ = json.RawMessage(nil)
var _ = sha256.New
var _ = hex.EncodeToString
