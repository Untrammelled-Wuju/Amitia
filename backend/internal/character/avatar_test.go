package character

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	authctx "github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/pkg/app"
)

func TestAvatarUploadReadAndReplace(t *testing.T) {
	t.Chdir(t.TempDir())
	db := newTestDB(t)
	svc := NewService(NewRepository(app.NewAppContext(db, nil)), app.NewAppContext(db, nil))
	scoped := svc.(syncScopedCharacterService)
	character, err := scoped.CreateForSpace(&CreateCharacterRequest{Name: "avatar"}, "space-avatar")
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(svc)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		space := c.GetHeader("Test-Space")
		c.Set("actorContext", &authctx.ActorContext{SpaceID: runtimeidentity.SpaceID(space)})
	})
	router.POST("/api/characters/:id/avatar", handler.UploadAvatar)
	router.GET("/api/characters/:id/avatar", handler.GetAvatar)
	path := "/api/characters/" + character.ID + "/avatar"
	request := func(method, space string, body *bytes.Buffer, contentType string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, body)
		req.Header.Set("Test-Space", space)
		req.Header.Set("Content-Type", contentType)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	for _, content := range []string{"first-avatar", "replacement-avatar"} {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile("avatar", "avatar.png")
		if err != nil {
			t.Fatal(err)
		}
		part.Write([]byte(content))
		writer.Close()
		w := request(http.MethodPost, "space-avatar", &body, writer.FormDataContentType())
		var result struct {
			Code int `json:"code"`
			Data struct {
				AvatarURL string `json:"avatarUrl"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || result.Code != 200 || result.Data.AvatarURL == "" {
			t.Fatalf("upload failed: %s", w.Body.String())
		}
		w = request(http.MethodGet, "space-avatar", &bytes.Buffer{}, "")
		if w.Code != 200 || w.Body.String() != content || w.Header().Get("Cache-Control") != "private, no-cache" {
			t.Fatalf("avatar did not refresh: %d %s", w.Code, w.Body.String())
		}
	}
	if w := request(http.MethodGet, "other-space", &bytes.Buffer{}, ""); w.Code != 404 {
		t.Fatalf("cross-space avatar exposed: %d", w.Code)
	}
	stale := httptest.NewRequest(http.MethodGet, path+"?v=obsolete.png", nil)
	stale.Header.Set("Test-Space", "space-avatar")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, stale)
	if w.Code != 404 {
		t.Fatalf("stale avatar version served: %d", w.Code)
	}
	if err := scoped.UpdateAvatarForSpace(character.ID, "/avatars/../secret.png", "space-avatar"); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join("data", "secret.png"), []byte("secret"), 0600)
	if w := request(http.MethodGet, "space-avatar", &bytes.Buffer{}, ""); w.Code != 404 {
		t.Fatalf("invalid avatar path served: %d", w.Code)
	}
}
