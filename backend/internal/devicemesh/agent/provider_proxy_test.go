package agent

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestProviderProxyPreservesDeviceAuthorityWithoutForwardingLocalSecrets(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/device-mesh/v1/business/messages" || request.URL.Query().Get("token") != "" || request.URL.Query().Get("q") != "value" {
			t.Error("转发路径或查询参数错误")
		}
		if request.Header.Get("Authorization") != "AmitiaDevice device-secret" || request.Header.Get("X-Amitia-Device-ID") != "device-a" || request.Header.Get("X-Amitia-Space-ID") != "core-b" {
			t.Error("未使用当前设备凭证")
		}
		for _, name := range []string{"Cookie", "X-Amitia-Local-Token", "X-Amitia-Web-Access", "X-Private-Configuration", "Origin"} {
			if request.Header.Get(name) != "" {
				t.Errorf("本机权限数据被转发: %s", name)
			}
		}
		writer.Header().Set("Set-Cookie", "core-cookie=private")
		writer.Header().Set("Content-Type", "text/event-stream")
		_, _ = writer.Write([]byte("data: {}\n\n"))
	}))
	defer upstream.Close()
	handler := NewLocalHandler(t.TempDir(), runtimeidentity.PlatformWindows)
	if err := handler.credStore.SaveCredential(&StoredCredential{CloudBaseUrl: upstream.URL, Credential: "device-secret", DeviceID: "device-a", RuntimeID: "runtime-a", SpaceID: "core-b", ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	handler.RegisterRoutes(router, func(c *gin.Context) {
		ctx, cancel := context.WithCancel(c.Request.Context())
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	request := httptest.NewRequest("POST", "/internal/device-mesh/provider/api/device-mesh/v1/business/messages?token=local-secret&q=value", strings.NewReader(`{"message":"hello"}`))
	request.RemoteAddr = "127.0.0.1:40000"
	for _, name := range []string{"Cookie", "X-Amitia-Local-Token", "X-Amitia-Web-Access", "X-Private-Configuration", "Origin", "Authorization"} {
		request.Header.Set(name, "local-secret")
	}
	result := httptest.NewRecorder()
	router.ServeHTTP(result, request)
	if result.Code != 200 || result.Header().Get("Set-Cookie") != "" || result.Body.String() != "data: {}\n\n" {
		t.Fatalf("响应错误: %d", result.Code)
	}
	for _, path := range []string{"/internal/device-mesh/provider/internal/device-mesh/cloud-auth", "/internal/device-mesh/provider/api/public/device-mesh/v1/bootstrap/exchange"} {
		request := httptest.NewRequest("POST", path, nil)
		request.RemoteAddr = "127.0.0.1:40000"
		result := httptest.NewRecorder()
		router.ServeHTTP(result, request)
		if result.Code != 403 {
			t.Fatal("越权路径未被拦截")
		}
	}
	request = httptest.NewRequest("GET", "/internal/device-mesh/provider/api/public/health", nil)
	request.RemoteAddr = "192.168.1.20:40000"
	result = httptest.NewRecorder()
	router.ServeHTTP(result, request)
	if result.Code != 403 {
		t.Fatal("允许远程设备借用本机权限")
	}
}

func TestProviderProxyRejectsRedirect(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, "https://outside.example/api", 307)
	}))
	defer upstream.Close()
	handler := NewLocalHandler(t.TempDir(), runtimeidentity.PlatformWindows)
	if err := handler.credStore.SaveCredential(&StoredCredential{CloudBaseUrl: upstream.URL, Credential: "device-secret", ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	handler.RegisterRoutes(router, func(c *gin.Context) {
		ctx, cancel := context.WithCancel(c.Request.Context())
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	request := httptest.NewRequest("GET", "/internal/device-mesh/provider/api/public/health", nil)
	request.RemoteAddr = "127.0.0.1:40000"
	result := httptest.NewRecorder()
	router.ServeHTTP(result, request)
	body, _ := io.ReadAll(result.Result().Body)
	if result.Code != 502 || result.Header().Get("Location") != "" || !strings.Contains(string(body), "未自动重试") {
		t.Fatal("允许跨服务重定向")
	}
}
