package pet_center

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func behaviorTestService(t *testing.T, revision string, executor BehaviorToolExecutor) *DesktopPetPluginManagementService {
	t.Helper()
	contribution := makePetPluginContrib("com.example/pet-a", "pet-overlay")
	contribution.Metadata = map[string]any{
		"behaviors": map[string]any{
			"version":  1,
			"revision": revision,
			"behaviors": []any{
				map[string]any{
					"id":          "wave",
					"name":        "挥手",
					"toolId":      "com.example/pet-a/pet-wave",
					"inputSchema": map[string]any{"type": "object"},
				},
			},
		},
	}
	return NewDesktopPetPluginManagementService(&fakeContainer{
		installations: []domain.ExtensionInstallation{{
			ExtensionID:       "com.example/pet-a",
			InstallationState: domain.InstallationStateInstalled,
			EnablementState:   domain.EnablementEnabled,
		}},
		contributions: []domain.ContributionDefinition{contribution},
	}, &fakeRuntime{}, nil, executor)
}

func behaviorTestRouter(service *DesktopPetPluginManagementService, actor *auth.ActorContext) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if actor != nil {
			c.Set("actorContext", actor)
		}
		c.Next()
	})
	NewHTTPHandler(service).RegisterRoutes(router.Group("/api/extensions"))
	return router
}

func TestInvokeBehaviorHTTPUsesAuthenticatedDeviceScope(t *testing.T) {
	var captured BehaviorInvocationScope
	service := behaviorTestService(t, "pet-r8", func(ctx context.Context, extensionID, toolID string, input json.RawMessage, scope BehaviorInvocationScope) (json.RawMessage, error) {
		captured = scope
		if extensionID != "com.example/pet-a" || toolID != "com.example/pet-a/pet-wave" {
			t.Fatalf("unexpected tool route: %s %s", extensionID, toolID)
		}
		return json.RawMessage(`{"accepted":true}`), nil
	})
	router := behaviorTestRouter(service, &auth.ActorContext{
		PrincipalType: auth.PrincipalTrustedDevice,
		SpaceID:       "space-a",
		DeviceID:      "device-a",
		RequestID:     "request-a",
	})
	body := bytes.NewBufferString(`{"arguments":{"amplitude":2},"expectedCatalogRevision":"pet-r8","characterId":"role-a","conversationId":"conversation-a","channel":"chat","sessionId":"session-a"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/extensions/pet/plugins/pet-overlay/behaviors/wave/invoke", body)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if captured.SpaceID != "space-a" || captured.DeviceID != "device-a" || captured.CharacterID != "role-a" || captured.ConversationID != "conversation-a" || captured.Channel != "chat" || captured.SessionID != "session-a" || captured.RequestID != "request-a" {
		t.Fatalf("authenticated scope mismatch: %+v", captured)
	}
	var responseBody struct {
		Code int                      `json:"code"`
		Data BehaviorInvocationResult `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &responseBody); err != nil {
		t.Fatal(err)
	}
	if responseBody.Code != 200 || responseBody.Data.BehaviorID != "wave" || string(responseBody.Data.Result) != `{"accepted":true}` {
		t.Fatalf("unexpected response: %+v", responseBody)
	}
}

func TestInvokeBehaviorHTTPRejectsStaleRevision(t *testing.T) {
	service := behaviorTestService(t, "pet-r9", func(context.Context, string, string, json.RawMessage, BehaviorInvocationScope) (json.RawMessage, error) {
		return json.RawMessage(`null`), nil
	})
	router := behaviorTestRouter(service, &auth.ActorContext{
		PrincipalType: auth.PrincipalTrustedDevice,
		SpaceID:       "space-a",
		DeviceID:      "device-a",
	})
	request := httptest.NewRequest(http.MethodPost, "/api/extensions/pet/plugins/pet-overlay/behaviors/wave/invoke", bytes.NewBufferString(`{"arguments":{},"expectedCatalogRevision":"pet-r8"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestInvokeBehaviorHTTPRequiresDeviceActor(t *testing.T) {
	service := behaviorTestService(t, "pet-r9", nil)
	router := behaviorTestRouter(service, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/extensions/pet/plugins/pet-overlay/behaviors/wave/invoke", bytes.NewBufferString(`{"arguments":{},"expectedCatalogRevision":"pet-r9"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
