// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/pkg/app"
)

func TestRegisterProcessingRouterRetiresLegacyPackageRoutes(t *testing.T) {
	previous := config.AppCfg
	config.AppCfg = &config.Config{Storage: config.StorageConfig{DataDir: t.TempDir()}}
	t.Cleanup(func() { config.AppCfg = previous })
	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := router.Group("/api")
	RegisterProcessingRouter(group, &app.AppContext{DB: setupTestDB(t)}, nil)

	routes := map[string]bool{}
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = true
	}

	for _, expected := range []string{
		"POST /api/desktop-pets/generation-tasks/:taskId/process",
		"GET /api/desktop-pets/processing-tasks/:processingTaskId",
		"POST /api/desktop-pets/processing-tasks/:processingTaskId/cancel",
	} {
		if !routes[expected] {
			t.Fatalf("missing processing route %s", expected)
		}
	}

	for _, retired := range []string{
		"POST /api/desktop-pets/processing-tasks/:processingTaskId/package",
		"GET /api/desktop-pets/packages",
		"GET /api/desktop-pets/packages/:packageId/download",
	} {
		if routes[retired] {
			t.Fatalf("retired package route still registered: %s", retired)
		}
	}
}
