package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/pkg/app"
)

func TestModelScenarioRealTLSCoreConfigurationAndModeSwitch(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	a, b := newThreeCoreFixture(t, "scenario-device", schemas), newThreeCoreFixture(t, "scenario-cloud", schemas)
	pairThreeCoreFixtures(t, a, b)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Model    string
			Stream   bool
			Messages []map[string]any
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		if input.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"%s\"}}]}\n\ndata: [DONE]\n\n", input.Model)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		text := `{"title":"模型路由验证"}`
		if strings.Contains(fmt.Sprint(input.Messages), `"items"`) {
			text = `{"items":[]}`
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": text}}}})
	}))
	defer provider.Close()
	db := b.services.DB
	if err := db.Exec("DELETE FROM model_configs").Error; err != nil {
		t.Fatal(err)
	}
	for _, cfg := range []chat.ModelConfig{{ID: 1, Name: "default", ModelName: "default", IsActive: 1}, {ID: 2, Name: "assigned", ModelName: "cloud-assigned"}} {
		cfg.APIType, cfg.Protocol, cfg.BaseURL, cfg.MaxTokens = "openai-compatible", "openai_chat", provider.URL, 128
		if err := db.Create(&cfg).Error; err != nil {
			t.Fatal(err)
		}
	}
	ctx := app.NewAppContext(db, nil)
	model := chat.NewService(chat.NewRepository(ctx), ctx, nil, nil, nil)
	b.services.OwnedBusiness = business.NewEngine(b.services.DeviceMesh.Coordination, b.services.DeviceMesh, model.(business.Model))
	auth := security.AuthenticationMiddleware(security.AuthConfig{Mode: "network", SpaceID: b.core, DeviceCredentials: b.services.DeviceMesh.CredentialSvc, DeviceRegistry: b.services.KernelContainer.DeviceRegistry, Coordination: b.services.DeviceMesh.Coordination})
	chat.RegisterModelRouter(b.router.Group("/api", auth), model)
	b.request(t, a, http.MethodGet, "/api/model/routes", nil, http.StatusForbidden)
	policy, err := b.services.DeviceMesh.Coordination.Get(t.Context(), b.core, a.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	b.request(t, a, http.MethodPut, "/api/device-mesh/v1/coordination/me", map[string]any{"coordinated": true, "expectedRevision": policy.ModeRevision, "selectedRole": "one"}, http.StatusOK)
	policy, err = b.services.DeviceMesh.Coordination.Get(t.Context(), b.core, a.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	policy, err = b.services.DeviceMesh.Coordination.GrantAdministrator(t.Context(), b.core, a.device.DeviceID.String(), policy.PermissionRevision, true)
	if err != nil {
		t.Fatal(err)
	}
	headers := map[string]string{security.ExpectedCoreHeader: b.core, security.ExpectedConfigurationPolicyHeader: fmt.Sprintf("%d:%d:%d", policy.ProviderEpoch, policy.ModeRevision, policy.PermissionRevision)}
	b.request(t, a, http.MethodPut, "/api/model/routes", map[string]any{"routes": []any{map[string]any{"scenario": "chat", "modelConfigId": 2}}}, http.StatusOK, headers)
	for index, coordinated := range []bool{true, false} {
		if !coordinated {
			current, err := b.services.DeviceMesh.Coordination.Get(t.Context(), b.core, a.device.DeviceID.String())
			if err != nil {
				t.Fatal(err)
			}
			b.request(t, a, http.MethodPut, "/api/device-mesh/v1/coordination/me", map[string]any{"coordinated": false, "expectedRevision": current.ModeRevision}, http.StatusOK)
		}
		var result struct{ Data business.Response }
		body := b.request(t, a, http.MethodPost, "/api/device-mesh/v1/business/messages", map[string]any{"requestId": fmt.Sprintf("route-%d", index), "characterId": "one", "message": "你好"}, http.StatusOK)
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatal(err)
		}
		owner := b.core
		if !coordinated {
			owner = a.device.DeviceID.String()
		}
		if !result.Data.Saved || result.Data.Generation.Text != "cloud-assigned" || result.Data.Scope.ResourceOwnerID != owner {
			t.Fatalf("wrong mode result: %s", body)
		}
	}
	b.request(t, a, http.MethodPut, "/api/model/routes", map[string]any{"routes": []any{map[string]any{"scenario": "chat", "modelConfigId": 1}}}, http.StatusForbidden, headers)
}
