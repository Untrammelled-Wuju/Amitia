package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
)

func TestWorldbookRoutesPreserveOriginalParseErrorsQA(t *testing.T) {
	for _, action := range []string{"manage", "import", "match"} {
		limit := 320 << 10
		if action == "import" {
			limit = 9 << 20
		}
		for _, input := range []struct{ name, body, original string }{
			{"malformed", "{", "unexpected EOF"},
			{"empty", "", "EOF"},
			{"type", `{"requestId":123}`, "json: cannot unmarshal"},
			{"oversized", `{"padding":"` + strings.Repeat("x", limit) + `"}`, "http: request body too large"},
		} {
			t.Run(action+"/"+input.name, func(t *testing.T) {
				router := gin.New()
				router.Use(func(c *gin.Context) {
					c.Set("actorContext", &auth.ActorContext{SpaceID: "qa-space", DeviceID: "qa-device"})
					c.Next()
				})
				registerMeshWorldbookRouter(router.Group("/worldbook-qa"), nil, "qa-core")
				request := httptest.NewRequest(http.MethodPost, "/worldbook-qa/worldbook/"+action, strings.NewReader(input.body))
				request.Header.Set("Content-Type", "application/json")
				response := httptest.NewRecorder()
				router.ServeHTTP(response, request)
				var payload struct {
					Message string `json:"message"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
					t.Fatal(err)
				}
				if response.Code != http.StatusBadRequest || !strings.Contains(payload.Message, input.original) {
					t.Fatalf("original error %q missing: status=%d body=%s", input.original, response.Code, response.Body.String())
				}
			})
		}
	}
}
