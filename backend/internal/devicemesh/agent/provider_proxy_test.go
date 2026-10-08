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
	"github.com/gorilla/websocket"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestProviderProxyRealtimeKeepsOriginalOriginAndCancelsUpgradedConnection(t *testing.T) {
	const origin = "http://localhost:15178"
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Origin") != origin || request.Header.Get("X-Amitia-Local-Token") != "" || request.URL.Query().Get("token") != "" {
			t.Error("realtime origin changed or local authority leaked")
		}
		if request.Header.Get("Authorization") != "AmitiaDevice device-secret" {
			t.Error("proxy did not use paired credential")
		}
		if request.URL.Path == "/api/device-mesh/v1/business/realtime/tickets" {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"ticket":"one-use"}`))
			return
		}
		if request.URL.Path != "/api/device-mesh/v1/business/realtime/session" || request.URL.Query().Get("ticket") != "one-use" {
			t.Error("realtime session lost original ticket route")
		}
		upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return r.Header.Get("Origin") == origin }}
		conn, err := upgrader.Upgrade(writer, request, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.WriteMessage(websocket.TextMessage, []byte("connected"))
		_, _, _ = conn.ReadMessage()
	}))
	defer upstream.Close()
	handler := NewLocalHandler(t.TempDir(), runtimeidentity.PlatformWindows)
	defer handler.pauseProvider()
	if err := handler.credStore.SaveCredential(&StoredCredential{CloudBaseUrl: upstream.URL, Credential: "device-secret", DeviceID: "device-a", RuntimeID: "runtime-a", SpaceID: "core-b", ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	handler.RegisterRoutes(router, func(c *gin.Context) { c.Next() })
	source := httptest.NewServer(router)
	defer source.Close()
	path := source.URL + "/internal/device-mesh/provider/api/device-mesh/v1/business/realtime/"
	request, _ := http.NewRequest(http.MethodPost, path+"tickets?token=local-secret", strings.NewReader(`{}`))
	request.Header.Set("Origin", origin)
	request.Header.Set("X-Amitia-Local-Token", "local-secret")
	response, err := source.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal("original realtime ticket request did not reach Core")
	}
	request, _ = http.NewRequest(http.MethodPost, path+"tickets", strings.NewReader(`{}`))
	request.Header.Set("Origin", "http://bad.example/path")
	response, err = source.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 403 {
		t.Fatal("malformed realtime origin was forwarded")
	}
	headers := http.Header{"Origin": []string{origin}, "X-Amitia-Local-Token": []string{"local-secret"}}
	conn, _, err := websocket.DefaultDialer.Dial(strings.Replace(path, "http://", "ws://", 1)+"session?ticket=one-use&token=local-secret", headers)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, data, err := conn.ReadMessage(); err != nil || string(data) != "connected" {
		t.Fatalf("upgraded proxy not connected: %v", err)
	}
	handler.pauseProvider()
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("old Core websocket remained active after provider pause")
	}
}

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
		if request.Header.Get("X-Amitia-Role-Authority") != "original-owner-intent" {
			t.Error("角色数据归属标识未保留")
		}
		if request.Header.Get("X-Amitia-Expected-Core-ID") != "original-core" {
			t.Error("配置原 Core 标识未保留")
		}
		if request.Header.Get("X-Amitia-Expected-Configuration-Policy") != "1:2:3" {
			t.Error("配置权限版本未保留")
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
	request.Header.Set("X-Amitia-Role-Authority", "original-owner-intent")
	request.Header.Set("X-Amitia-Expected-Core-ID", "original-core")
	request.Header.Set("X-Amitia-Expected-Configuration-Policy", "1:2:3")
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
