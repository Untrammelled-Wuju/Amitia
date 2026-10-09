package artifact

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestHTMLPreviewAuthenticationIsolationAndCleanup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil)
	router := gin.New()
	api := router.Group("/api", func(c *gin.Context) {
		if owner := c.GetHeader("X-Test-Owner"); owner != "" {
			c.Set("actorContext", &auth.ActorContext{SpaceID: runtimeidentity.SpaceID(owner)})
		}
	})
	handler.Register(api)
	handler.RegisterPublicMedia(router)
	request := func(method, path, body, owner string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-Owner", owner)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}
	if response := request("POST", "/api/artifacts/v1/previews", `{"html":"<h1>test</h1>"}`, ""); response.Code != 401 {
		t.Fatalf("unauthorized creation: %d", response.Code)
	}
	response := request("POST", "/api/artifacts/v1/previews", `{"html":"<h1>test</h1><script>window.ready=1</script>"}`, "space-a")
	if response.Code != 200 {
		t.Fatalf("preview creation: %d %s", response.Code, response.Body.String())
	}
	var preview struct {
		URL string `json:"url"`
		ID  string `json:"previewId"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	response = request("GET", preview.URL, "", "")
	if response.Code != 200 || !strings.Contains(response.Body.String(), "window.ready=1") {
		t.Fatal("ticket did not serve preview")
	}
	if response.Header().Get("Content-Security-Policy") != previewPolicy || !strings.Contains(previewPolicy, "sandbox allow-scripts") || response.Header().Get("Content-Disposition") != "inline" || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("preview isolation headers missing")
	}
	if response := request("GET", preview.URL+"tampered", "", ""); response.Code != http.StatusUnauthorized {
		t.Fatal("tampered ticket accepted")
	}
	request("DELETE", "/api/artifacts/v1/previews/"+preview.ID, "", "space-b")
	if response := request("GET", preview.URL, "", ""); response.Code != 200 {
		t.Fatal("another owner removed the preview")
	}
	request("DELETE", "/api/artifacts/v1/previews/"+preview.ID, "", "space-a")
	if response := request("GET", preview.URL, "", ""); response.Code != 404 {
		t.Fatal("deleted preview remained available")
	}
}

func TestHTMLPreviewRejectsExpiredDocument(t *testing.T) {
	handler := NewHandler(nil)
	ticket, _, err := handler.ticketSigner.Issue("preview", "owner")
	if err != nil {
		t.Fatal(err)
	}
	handler.previews["preview"] = previewDocument{source: "<h1>test</h1>", owner: "owner", expiresAt: time.Now().Add(-time.Minute)}
	router := gin.New()
	handler.RegisterPublicMedia(router)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("GET", "/media/artifact-previews/preview/"+ticket, nil))
	if response.Code != 404 {
		t.Fatal("expired document was served")
	}
}
