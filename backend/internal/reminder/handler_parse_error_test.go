package reminder

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
)

func TestReminderHandlersPreserveJSONErrors(t *testing.T) {
	routes := []struct {
		method      string
		path        string
		invalidType string
	}{
		{"POST", "/api/reminders", `{"title":123}`},
		{"PUT", "/api/reminders/test", `{"generation":"bad"}`},
		{"DELETE", "/api/reminders/test", `{"generation":"bad"}`},
		{"POST", "/api/reminders/test/toggle", `{"generation":"bad"}`},
		{"POST", "/api/reminders/test/trigger", `{"generation":"bad"}`},
		{"PUT", "/api/reminders/cleanup-config", `{"cleanupDays":123}`},
	}
	for _, route := range routes {
		for _, input := range []struct{ body, original string }{
			{"{", "unexpected EOF"},
			{"", "EOF"},
			{route.invalidType, "json: cannot unmarshal"},
		} {
			t.Run(route.method+route.path+input.body, func(t *testing.T) {
				router := gin.New()
				router.Use(func(c *gin.Context) {
					c.Set("actorContext", &auth.ActorContext{SpaceID: "space", DeviceID: "device"})
					c.Next()
				})
				registerReminderRoutes(router.Group("/api"), NewHandler(nil, "device"))
				request := httptest.NewRequest(route.method, route.path, strings.NewReader(input.body))
				request.Header.Set("Content-Type", "application/json")
				response := httptest.NewRecorder()
				router.ServeHTTP(response, request)
				var payload struct {
					Code    int    `json:"code"`
					Message string `json:"msg"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
					t.Fatal(err)
				}
				if response.Code != http.StatusBadRequest || payload.Code != http.StatusBadRequest {
					t.Fatalf("expected HTTP and business 400, got %d: %s", response.Code, response.Body.String())
				}
				if !strings.Contains(payload.Message, input.original) {
					t.Fatalf("missing original error %q in %q", input.original, payload.Message)
				}
			})
		}
	}
}
