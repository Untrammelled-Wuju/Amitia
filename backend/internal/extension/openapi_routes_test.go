package extension

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/pkg/app"
)

func TestOpenAPICoversExtensionBusinessRoutes(t *testing.T) {
	var document struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	raw, err := openAPIFS.ReadFile("schema/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	originalConfig := config.AppCfg
	if config.AppCfg == nil {
		config.AppCfg = &config.Config{}
	}
	defer func() { config.AppCfg = originalConfig }()
	engine := gin.New()
	RegisterRouter(engine.Group("/api"), &app.AppContext{}, &Runtime{})
	for _, route := range engine.Routes() {
		path := strings.TrimPrefix(route.Path, "/api")
		if path == "/extensions/openapi.json" {
			continue
		}
		path = strings.NewReplacer(":", "{", "/", "/").Replace(path)
		parts := strings.Split(path, "/")
		for index, part := range parts {
			if strings.HasPrefix(part, "{") && !strings.HasSuffix(part, "}") {
				parts[index] = part + "}"
			}
		}
		path = strings.Join(parts, "/")
		methods, ok := document.Paths[path]
		if !ok {
			t.Errorf("OpenAPI missing path %s", path)
			continue
		}
		if _, ok := methods[strings.ToLower(route.Method)]; !ok {
			t.Errorf("OpenAPI missing method %s %s", route.Method, path)
		}
	}
}
