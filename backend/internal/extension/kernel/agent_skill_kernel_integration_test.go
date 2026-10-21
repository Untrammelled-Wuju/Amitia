package kernel

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/agent_skill"
	"github.com/u-ai/backend/internal/runtimeprofile"
)

func TestAgentSkillPackagePromptActivationAndResources(t *testing.T) {
	ctx := context.Background()
	runtimeInstance, container := newPackagePipelineRuntime(t)
	files := map[string][]byte{
		"SKILL.md":            []byte("---\nname: demo-skill\ndescription: Use when the user requests a demo skill workflow\nmetadata:\n  version: 1.0.0\nactivation:\n  mode: explicit\n---\n\nFollow the demo reference.\n"),
		"references/guide.md": []byte("# Guide\n\nUse the documented demo steps.\n"),
	}
	built, err := BuildLocalAgentSkillPackage(ctx, "space-a", "demo-skill", files)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := runtimeInstance.PreviewPackage(ctx, PackagePreviewRequest{
		SpaceID: "space-a", ScopeType: "global", FileName: built.FileName, AllowUnsignedLocal: true,
	}, bytes.NewReader(built.Archive))
	if err != nil {
		t.Fatal(err)
	}
	confirmations := map[string]bool{}
	for _, item := range preview.RequiredConfirmations {
		confirmations[item] = true
	}
	confirmation, err := runtimeInstance.ConfirmPackagePreview(ctx, PackagePreviewConfirmationRequest{
		SessionID: preview.SessionID, SpaceID: "space-a", ScopeType: "global", Confirmations: confirmations,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := runtimeInstance.ExecutePackageInstall(ctx, PackageInstallRequest{
		SessionID: preview.SessionID, SpaceID: "space-a", ScopeType: "global",
		ConfirmationToken: confirmation.ConfirmationToken, IdempotencyKey: "agent-skill-integration",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := runtimeInstance.Enable(ctx, result.ExtensionID); err != nil {
		t.Fatal(err)
	}
	service := container.AgentSkillRuntime
	available := service.Available(ctx, InvocationScope{SpaceID: "space-a", TraceID: "trace-a"})
	if len(available) != 1 || available[0].Name != "demo-skill" {
		t.Fatalf("available skills = %#v catalog=%#v", available, container.AgentSkillCatalog.List(agent_skill.CatalogFilter{}))
	}
	catalog, activated, errorsList := service.PreparePrompt(ctx, InvocationScope{SpaceID: "space-a", TraceID: "trace-a"}, "run $demo-skill")
	if !strings.Contains(catalog, "demo-skill") || len(activated) != 1 || len(errorsList) != 0 {
		t.Fatalf("catalog=%q activated=%#v errors=%#v", catalog, activated, errorsList)
	}
	resource, content, err := service.ReadInstalledResource(ctx, "space-a", result.ExtensionID, "resources/demo-skill/references/guide.md")
	if err != nil {
		t.Fatal(err)
	}
	if resource.Kind != "reference" || !strings.Contains(string(content), "documented demo steps") {
		t.Fatalf("resource=%#v content=%q", resource, content)
	}
	activationOutput := service.ReadResource(ctx, InvocationScope{SpaceID: "space-a", TraceID: "trace-a"}, ReadSkillResourceInput{
		Skill: "demo-skill", Path: "resources/demo-skill/references/guide.md",
	})
	if activationOutput.Error != "" || !strings.Contains(activationOutput.Content, "documented demo steps") {
		t.Fatalf("activation resource output=%#v", activationOutput)
	}
	history, err := service.ListHistory(ctx, "space-a", result.ExtensionID, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].AgentSkillName != "demo-skill" || !history[0].Explicit {
		t.Fatalf("history=%#v", history)
	}
	if err := service.SetEnabled(ctx, "space-a", result.ExtensionID, false); err != nil {
		t.Fatal(err)
	}
	if items := service.Available(ctx, InvocationScope{SpaceID: "space-a"}); len(items) != 0 {
		t.Fatalf("disabled skill remained available: %#v", items)
	}
	if err := runtimeInstance.Disable(ctx, result.ExtensionID); err != nil {
		t.Fatal(err)
	}
	if err := runtimeInstance.Enable(ctx, result.ExtensionID); err != nil {
		t.Fatal(err)
	}
	if items := service.Available(ctx, InvocationScope{SpaceID: "space-a"}); len(items) != 0 {
		t.Fatalf("manual disable was lost after extension restart: %#v", items)
	}
	if err := service.SetEnabled(ctx, "space-a", result.ExtensionID, true); err != nil {
		t.Fatal(err)
	}
	if items := service.Available(ctx, InvocationScope{SpaceID: "space-a"}); len(items) != 1 {
		t.Fatalf("skill did not re-enable: %#v", items)
	}
	if items := service.Available(ctx, InvocationScope{SpaceID: "space-b"}); len(items) != 0 {
		t.Fatalf("skill leaked across spaces: %#v", items)
	}
	if err := runtimeInstance.Disable(ctx, result.ExtensionID); err != nil {
		t.Fatal(err)
	}
	uninstallPreview, err := runtimeInstance.PreviewPackageUninstall(ctx, result.ExtensionID, "space-a", "global", "")
	if err != nil {
		t.Fatal(err)
	}
	if !uninstallPreview.Installable {
		t.Fatalf("uninstall preview=%#v", uninstallPreview)
	}
	uninstallConfirmations := map[string]bool{}
	for _, item := range RequiredUninstallConfirmations(uninstallPreview) {
		uninstallConfirmations[item] = true
	}
	uninstallConfirmation, err := runtimeInstance.ConfirmPackageUninstall(ctx, ConfirmPackageUninstallRequest{
		ExtensionID: result.ExtensionID, SpaceID: "space-a", ScopeType: "global", Confirmations: uninstallConfirmations,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtimeInstance.ExecutePackageUninstall(ctx, ExecutePackageUninstallRequest{
		ExtensionID: result.ExtensionID, SpaceID: "space-a", ScopeType: "global",
		ConfirmationToken: uninstallConfirmation.Token, IdempotencyKey: "agent-skill-uninstall",
	}); err != nil {
		t.Fatal(err)
	}
	if items := service.Available(ctx, InvocationScope{SpaceID: "space-a"}); len(items) != 0 {
		t.Fatalf("skill remained after uninstall: %#v", items)
	}
	var overrideCount int
	if err := container.Store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM extension_agent_skill_overrides WHERE owner_space_id = ? AND skill_id = ?`, "space-a", result.ExtensionID+":demo-skill").Scan(&overrideCount); err != nil {
		t.Fatal(err)
	}
	if overrideCount != 0 {
		t.Fatalf("agent skill override remained after uninstall: %d", overrideCount)
	}
}

func TestAgentSkillSurvivesKernelRestart(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	databasePath := filepath.Join(root, "kernel.db")
	extensionRoot := filepath.Join(root, "extensions")
	container, err := NewContainerBuilder().WithDBPath(databasePath).WithExtensionRoot(extensionRoot).Build(ctx)
	if err != nil {
		t.Fatal(err)
	}
	runtimeInstance, err := NewRuntime(extensionRoot)
	if err != nil {
		t.Fatal(err)
	}
	runtimeInstance.SetContainer(container)
	files := map[string][]byte{
		"SKILL.md": []byte("---\nname: restart-skill\ndescription: Use for restart persistence verification\n---\n\nPersist across restart.\n"),
	}
	built, err := BuildLocalAgentSkillPackage(ctx, "space-restart", "restart-skill", files)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := runtimeInstance.PreviewPackage(ctx, PackagePreviewRequest{
		SpaceID: "space-restart", ScopeType: "global", FileName: built.FileName, AllowUnsignedLocal: true,
	}, bytes.NewReader(built.Archive))
	if err != nil {
		t.Fatal(err)
	}
	confirmations := map[string]bool{}
	for _, item := range preview.RequiredConfirmations {
		confirmations[item] = true
	}
	confirmation, err := runtimeInstance.ConfirmPackagePreview(ctx, PackagePreviewConfirmationRequest{
		SessionID: preview.SessionID, SpaceID: "space-restart", ScopeType: "global", Confirmations: confirmations,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := runtimeInstance.ExecutePackageInstall(ctx, PackageInstallRequest{
		SessionID: preview.SessionID, SpaceID: "space-restart", ScopeType: "global",
		ConfirmationToken: confirmation.ConfirmationToken, IdempotencyKey: "restart-skill-install",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := runtimeInstance.Enable(ctx, result.ExtensionID); err != nil {
		t.Fatal(err)
	}
	if err := container.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := NewContainerBuilder().WithDBPath(databasePath).WithExtensionRoot(extensionRoot).Build(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	if err := restarted.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if items := restarted.AgentSkillRuntime.Available(ctx, InvocationScope{SpaceID: "space-restart"}); len(items) != 1 || items[0].Name != "restart-skill" {
		t.Fatalf("skills after restart = %#v", items)
	}
}

func TestAgentSkillPackageRuntimeProfiles(t *testing.T) {
	for _, profile := range []runtimeprofile.Profile{runtimeprofile.ProfileLocal, runtimeprofile.ProfileCloudCore, runtimeprofile.ProfileDeviceAgent} {
		t.Run(profile.String(), func(t *testing.T) {
			ctx := context.Background()
			root := t.TempDir()
			container, err := NewContainerBuilder().WithRuntimeProfile(profile).WithDBPath(filepath.Join(root, "kernel.db")).WithExtensionRoot(filepath.Join(root, "extensions")).Build(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer container.Close()
			runtimeInstance, err := NewRuntime(filepath.Join(root, "extensions"))
			if err != nil {
				t.Fatal(err)
			}
			runtimeInstance.SetContainer(container)
			built, err := BuildLocalAgentSkillPackage(ctx, "space-"+profile.String(), "profile-skill", map[string][]byte{
				"SKILL.md": []byte("---\nname: profile-skill\ndescription: Use for runtime profile verification\n---\n\nProfile independent.\n"),
			})
			if err != nil {
				t.Fatal(err)
			}
			preview, err := runtimeInstance.PreviewPackage(ctx, PackagePreviewRequest{
				SpaceID: "space-" + profile.String(), ScopeType: "global", FileName: built.FileName, AllowUnsignedLocal: true,
			}, bytes.NewReader(built.Archive))
			if err != nil {
				t.Fatal(err)
			}
			confirmations := map[string]bool{}
			for _, item := range preview.RequiredConfirmations {
				confirmations[item] = true
			}
			confirmation, err := runtimeInstance.ConfirmPackagePreview(ctx, PackagePreviewConfirmationRequest{
				SessionID: preview.SessionID, SpaceID: "space-" + profile.String(), ScopeType: "global", Confirmations: confirmations,
			})
			if err != nil {
				t.Fatal(err)
			}
			result, err := runtimeInstance.ExecutePackageInstall(ctx, PackageInstallRequest{
				SessionID: preview.SessionID, SpaceID: "space-" + profile.String(), ScopeType: "global",
				ConfirmationToken: confirmation.ConfirmationToken, IdempotencyKey: "profile-install-" + profile.String(),
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := runtimeInstance.Enable(ctx, result.ExtensionID); err != nil {
				t.Fatal(err)
			}
			if items := container.AgentSkillRuntime.Available(ctx, InvocationScope{SpaceID: "space-" + profile.String()}); len(items) != 1 {
				t.Fatalf("%s skills = %#v", profile, items)
			}
		})
	}
}
