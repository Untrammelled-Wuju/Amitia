package extension

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestAgentSkillActivationsResolveInstalledSlugToExtensionID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	root := t.TempDir()
	container, err := kernelruntime.NewContainerBuilder().
		WithDBPath(filepath.Join(root, "kernel.db")).
		WithExtensionRoot(filepath.Join(root, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = container.Close() })
	runtime, err := kernelruntime.NewRuntime(filepath.Join(root, "extensions"))
	if err != nil {
		t.Fatal(err)
	}
	runtime.SetContainer(container)

	built, err := kernelruntime.BuildLocalAgentSkillPackage(ctx, "space-a", "history-skill", map[string][]byte{
		"SKILL.md": []byte("---\nname: history-skill\ndescription: Activation history regression\nmetadata:\n  version: 1.0.0\nactivation:\n  mode: explicit\n---\n\nActivation history regression.\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := runtime.PreviewPackage(ctx, kernelruntime.PackagePreviewRequest{SpaceID: "space-a", ScopeType: "global", FileName: built.FileName, AllowUnsignedLocal: true}, bytes.NewReader(built.Archive))
	if err != nil {
		t.Fatal(err)
	}
	confirmations := map[string]bool{}
	for _, item := range preview.RequiredConfirmations {
		confirmations[item] = true
	}
	confirmation, err := runtime.ConfirmPackagePreview(ctx, kernelruntime.PackagePreviewConfirmationRequest{SessionID: preview.SessionID, SpaceID: "space-a", ScopeType: "global", Confirmations: confirmations})
	if err != nil {
		t.Fatal(err)
	}
	installed, err := runtime.ExecutePackageInstall(ctx, kernelruntime.PackageInstallRequest{SessionID: preview.SessionID, SpaceID: "space-a", ScopeType: "global", ConfirmationToken: confirmation.ConfirmationToken, IdempotencyKey: "agent-skill-activation-api"})
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.Enable(ctx, installed.ExtensionID); err != nil {
		t.Fatal(err)
	}
	service := container.AgentSkillRuntime
	if _, _, activationErrors := service.PreparePrompt(ctx, kernelruntime.InvocationScope{SpaceID: "space-a", TraceID: "trace-history"}, "use $history-skill"); len(activationErrors) != 0 {
		t.Fatalf("activation failed: %#v", activationErrors)
	}
	definition, err := service.GetInstalled(ctx, "space-a", installed.ExtensionID)
	if err != nil {
		t.Fatal(err)
	}

	api := NewKernelAPI(&Runtime{Kernel: runtime})
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/extensions/kernel/agent-skills/"+definition.ID+"/activations", nil)
	c.Params = gin.Params{{Key: "id", Value: definition.ID}}
	c.Set("actorContext", &auth.ActorContext{SpaceID: runtimeidentity.SpaceID("space-a")})
	api.agentSkillActivations(c)
	if recorder.Code != http.StatusOK {
		t.Fatalf("activations status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var items []kernelruntime.AgentSkillActivation
	if err := json.Unmarshal(recorder.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ExtensionID != installed.ExtensionID || items[0].AgentSkillName != "history-skill" {
		t.Fatalf("activation history lost: %#v", items)
	}
}
