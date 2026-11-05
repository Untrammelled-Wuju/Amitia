package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
	"github.com/u-ai/backend/pkg/platform"
)

type readinessManagedComponent struct {
	descriptor runtimeorchestrator.ComponentDescriptor
	startErr   error
	readyErr   error
}

func (c *readinessManagedComponent) Descriptor() runtimeorchestrator.ComponentDescriptor {
	return c.descriptor
}

func (c *readinessManagedComponent) Start(context.Context) error {
	return c.startErr
}

func (c *readinessManagedComponent) Ready(context.Context) error {
	return c.readyErr
}

func (c *readinessManagedComponent) Stop(context.Context) error {
	return nil
}

func readinessRouter(orchestrator runtimeReadinessSource) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/readyz", func(c *gin.Context) {
		writeRuntimeReadiness(c, orchestrator)
	})
	return router
}

func readinessResponse(t *testing.T, router http.Handler) (int, map[string]any) {
	t.Helper()
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	payload := map[string]any{}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	return response.Code, payload
}

func TestReadyzReportsOptionalDegradation(t *testing.T) {
	orchestrator := runtimeorchestrator.New(platform.RuntimeDescriptor{})
	if err := orchestrator.Register(&readinessManagedComponent{
		descriptor: runtimeorchestrator.ComponentDescriptor{
			ID: "ready.required", Phase: runtimeorchestrator.PhaseInfrastructure,
			Enabled: true, Required: true,
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := orchestrator.Register(&readinessManagedComponent{
		descriptor: runtimeorchestrator.ComponentDescriptor{
			ID: "ready.optional", Phase: runtimeorchestrator.PhaseInfrastructure,
			Enabled: true, Required: false,
		},
		startErr: context.Canceled,
	}); err != nil {
		t.Fatal(err)
	}
	if err := orchestrator.StartPhase(context.Background(), runtimeorchestrator.PhaseInfrastructure); err != nil {
		t.Fatal(err)
	}

	status, payload := readinessResponse(t, readinessRouter(orchestrator))
	if status != http.StatusOK {
		t.Fatalf("status=%d payload=%+v", status, payload)
	}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("missing readiness data: %+v", payload)
	}
	if data["status"] != "degraded" || data["state"] != "ready" ||
		data["readyCount"] != float64(1) || data["degradedCount"] != float64(1) ||
		data["blockingCount"] != float64(0) || data["failedCount"] != float64(0) {
		t.Fatalf("unexpected readiness response: %+v", data)
	}
}

func TestReadyzRefreshesOptionalComponentHealth(t *testing.T) {
	orchestrator := runtimeorchestrator.New(platform.RuntimeDescriptor{})
	required := &readinessManagedComponent{
		descriptor: runtimeorchestrator.ComponentDescriptor{
			ID: "refresh.required", Phase: runtimeorchestrator.PhaseInfrastructure,
			Enabled: true, Required: true,
		},
	}
	optional := &readinessManagedComponent{
		descriptor: runtimeorchestrator.ComponentDescriptor{
			ID: "refresh.optional", Phase: runtimeorchestrator.PhaseInfrastructure,
			Enabled: true, Required: false,
		},
	}
	if err := orchestrator.Register(required); err != nil {
		t.Fatal(err)
	}
	if err := orchestrator.Register(optional); err != nil {
		t.Fatal(err)
	}
	if err := orchestrator.StartPhase(context.Background(), runtimeorchestrator.PhaseInfrastructure); err != nil {
		t.Fatal(err)
	}
	optional.readyErr = context.Canceled
	status, payload := readinessResponse(t, readinessRouter(orchestrator))
	if status != http.StatusOK {
		t.Fatalf("status=%d payload=%+v", status, payload)
	}
	data := payload["data"].(map[string]any)
	components := data["components"].([]any)
	if data["status"] != "degraded" || data["degradedCount"] != float64(1) || len(components) != 1 {
		t.Fatalf("optional failure was not exposed: %+v", data)
	}
	optional.readyErr = nil
	_, payload = readinessResponse(t, readinessRouter(orchestrator))
	data = payload["data"].(map[string]any)
	if data["status"] != "ready" || data["degradedCount"] != float64(0) {
		t.Fatalf("optional recovery was not reflected: %+v", data)
	}
}

func TestReadyzReturnsServiceUnavailableWhenRequiredComponentFails(t *testing.T) {
	orchestrator := runtimeorchestrator.New(platform.RuntimeDescriptor{})
	if err := orchestrator.Register(&readinessManagedComponent{
		descriptor: runtimeorchestrator.ComponentDescriptor{
			ID: "ready.blocked", Phase: runtimeorchestrator.PhaseInfrastructure,
			Enabled: true, Required: true,
		},
		startErr: context.Canceled,
	}); err != nil {
		t.Fatal(err)
	}
	if err := orchestrator.StartPhase(context.Background(), runtimeorchestrator.PhaseInfrastructure); err == nil {
		t.Fatal("required component start failure did not fail the phase")
	}

	status, payload := readinessResponse(t, readinessRouter(orchestrator))
	if status != http.StatusServiceUnavailable {
		t.Fatalf("status=%d payload=%+v", status, payload)
	}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("missing readiness data: %+v", payload)
	}
	if data["status"] != "blocked" || data["state"] != "blocked" ||
		data["blockingCount"] != float64(1) || data["failedCount"] != float64(1) {
		t.Fatalf("unexpected blocked readiness response: %+v", data)
	}
}

func TestReadyzWithoutOrchestratorReturnsServiceUnavailable(t *testing.T) {
	status, payload := readinessResponse(t, readinessRouter(nil))
	if status != http.StatusServiceUnavailable || payload["msg"] != "blocked" {
		t.Fatalf("status=%d payload=%+v", status, payload)
	}
}
