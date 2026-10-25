package inbound

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/serviceauth"
)

type fixedMediaResolver struct {
	fullPath string
	err      error
}

func (r fixedMediaResolver) ResolveResourceLink(string) (string, string, string, error) {
	if r.err != nil {
		return "", "", "", r.err
	}
	return "com.example/channel", "data", r.fullPath, nil
}

func newMediaTestRouter(t *testing.T, resolver ResourceLinkResolver) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterMediaRouter(router, staticProviderDefinitionSource{
		definitions: []*capability.CapabilityProviderDefinition{
			{
				ExtensionID:  "com.example/channel",
				ModuleID:     "channel-service",
				CapabilityID: inboundProviderCapability,
				Metadata:     map[string]any{"channelId": "example"},
			},
		},
	}, resolver)
	return router
}

func mediaProviderToken(t *testing.T) string {
	t.Helper()
	token, err := serviceauth.Token("com.example/channel", "channel-service")
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func setMediaRequestIdentity(request *http.Request, token string) {
	request.RemoteAddr = "127.0.0.1:43210"
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-Amitia-Extension-ID", "com.example/channel")
	request.Header.Set("X-Amitia-Module-ID", "channel-service")
	request.Header.Set("X-Amitia-Channel-ID", "example")
}

func TestChannelMediaRouterServesSignedResourceToLoopbackProvider(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "image.png")
	if err := os.WriteFile(filePath, []byte("\x89PNG\r\n\x1a\npayload"), 0o600); err != nil {
		t.Fatal(err)
	}
	router := newMediaTestRouter(t, fixedMediaResolver{fullPath: filePath})
	request := httptest.NewRequest(http.MethodGet, "/api/channels/media/signed-token", nil)
	setMediaRequestIdentity(request, mediaProviderToken(t))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("unexpected media response: %d %s", response.Code, response.Body.String())
	}
	if response.Body.String() != "\x89PNG\r\n\x1a\npayload" {
		t.Fatalf("unexpected body: %q", response.Body.String())
	}
}

func TestChannelMediaRouterRejectsInvalidResource(t *testing.T) {
	router := newMediaTestRouter(t, fixedMediaResolver{err: errors.New("invalid signature")})
	request := httptest.NewRequest(http.MethodGet, "/api/channels/media/bad-token", nil)
	setMediaRequestIdentity(request, mediaProviderToken(t))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("expected not found, got %d", response.Code)
	}
}

func TestChannelMediaRouterRejectsNonLoopbackCaller(t *testing.T) {
	router := newMediaTestRouter(t, fixedMediaResolver{})
	request := httptest.NewRequest(http.MethodGet, "/api/channels/media/signed-token", nil)
	setMediaRequestIdentity(request, mediaProviderToken(t))
	request.RemoteAddr = "192.0.2.1:43210"
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d", response.Code)
	}
}

func TestChannelMediaRouterSupportsRangeRequests(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "audio.mp3")
	if err := os.WriteFile(filePath, []byte("0123456789"), 0o600); err != nil {
		t.Fatal(err)
	}
	router := newMediaTestRouter(t, fixedMediaResolver{fullPath: filePath})
	request := httptest.NewRequest(http.MethodGet, "/api/channels/media/audio", nil)
	setMediaRequestIdentity(request, mediaProviderToken(t))
	request.Header.Set("Range", "bytes=2-5")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusPartialContent {
		t.Fatalf("expected partial content, got %d", response.Code)
	}
	body, _ := io.ReadAll(response.Result().Body)
	if string(body) != "2345" {
		t.Fatalf("unexpected range body: %q", body)
	}
}
