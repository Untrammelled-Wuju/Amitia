package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/artifact"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/imagegen"
	"github.com/u-ai/backend/internal/imageintelligence"
	"github.com/u-ai/backend/internal/imageprovider"
	"github.com/u-ai/backend/internal/imageprovider/tongyi"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type productionSimulationArtifacts struct{}

func (productionSimulationArtifacts) Create(context.Context, artifact.CreateRequest) (artifact.Artifact, error) {
	return artifact.Artifact{ID: "generated-image", Kind: artifact.KindImage, MIMEType: "image/png", Filename: "generated.png", SizeBytes: 68}, nil
}

func TestImageGenerationProductionSimulation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pngData, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	var submitCalls atomic.Int64
	var providerServer *httptest.Server
	providerServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/services/aigc/text2image/image-synthesis":
			submitCalls.Add(1)
			if r.Header.Get("Authorization") != "Bearer simulation-key" {
				t.Errorf("unexpected authorization: %s", r.Header.Get("Authorization"))
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			writeImageGenerationSimulationJSON(t, w, map[string]any{
				"request_id": "submit-request",
				"output":     map[string]any{"task_id": "simulation-task", "task_status": "PENDING"},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/tasks/simulation-task":
			writeImageGenerationSimulationJSON(t, w, map[string]any{
				"request_id": "query-request",
				"output": map[string]any{
					"task_id":     "simulation-task",
					"task_status": "SUCCEEDED",
					"results":     []map[string]any{{"url": providerServer.URL + "/generated.png"}},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/generated.png":
			w.Header().Set("Content-Type", "image/png")
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write(pngData); err != nil {
				t.Error(err)
			}
		default:
			t.Errorf("unexpected provider request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer providerServer.Close()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "production-simulation.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&imagegen.ImageGenConfig{}); err != nil {
		t.Fatal(err)
	}
	registry := imageprovider.NewRegistry()
	adapter := tongyi.NewWithClient(providerServer.Client())
	if err := registry.RegisterWithDescriptor(tongyi.ProviderName, adapter, imageprovider.ProviderDescriptor{Name: tongyi.ProviderName, DisplayName: "通义", DefaultModel: tongyi.DefaultModel, DefaultBaseURL: providerServer.URL}); err != nil {
		t.Fatal(err)
	}
	configService := imagegen.NewService(imagegen.NewRepository(db), registry)
	config, err := configService.Create(&imagegen.CreateImageGenConfigRequest{
		Name:      "production simulation",
		ApiType:   tongyi.ProviderName,
		ApiKey:    "simulation-key",
		ModelName: tongyi.DefaultModel,
		BaseUrl:   providerServer.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := configService.Activate(config.ID); err != nil {
		t.Fatal(err)
	}
	facade := imageintelligence.NewImageIntelligenceFactory(nil, configService, registry, nil, productionSimulationArtifacts{}).Build()

	router := gin.New()
	group := router.Group("/api", func(c *gin.Context) {
		actor := &auth.ActorContext{PrincipalType: auth.PrincipalDeviceRuntime, SpaceID: runtimeidentity.SpaceID("space-production")}
		c.Set("actorContext", actor)
		c.Request = c.Request.WithContext(auth.WithActor(c.Request.Context(), actor))
		c.Next()
	})
	imageintelligence.RegisterRouter(group, facade)
	body, err := json.Marshal(map[string]any{"prompt": "production simulation tree", "width": 1024, "height": 1024, "count": 1})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/image-intelligence/generate", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if submitCalls.Load() != 1 {
		t.Fatalf("provider submit calls=%d", submitCalls.Load())
	}
	var envelope struct {
		Code int `json:"code"`
		Data struct {
			Provider string `json:"provider"`
			Images   []struct {
				ResourceURI string `json:"resourceUri"`
				MIMEType    string `json:"mimeType"`
			} `json:"images"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Code != http.StatusOK || envelope.Data.Provider != tongyi.ProviderName || len(envelope.Data.Images) != 1 || envelope.Data.Images[0].ResourceURI != "amitia://artifacts/generated-image" {
		t.Fatalf("unexpected generation response: %#v body=%s", envelope, response.Body.String())
	}
}

func writeImageGenerationSimulationJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Error(err)
	}
}
