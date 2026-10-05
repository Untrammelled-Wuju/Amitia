package agent

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/devicemesh/lan"
)

func (h *LocalHandler) pauseProvider() {
	h.providerMu.Lock()
	defer h.providerMu.Unlock()
	h.providerPaused = true
	if h.providerCancel != nil {
		h.providerCancel()
	}
}

func (h *LocalHandler) resumeProvider() {
	h.providerMu.Lock()
	defer h.providerMu.Unlock()
	if h.providerCancel != nil {
		h.providerCancel()
	}
	h.providerContext, h.providerCancel = context.WithCancel(context.Background())
	h.providerPaused = false
}

type identityTransport struct {
	base     http.RoundTripper
	identity *IdentityStore
	core     string
}

func (t identityTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if err := t.identity.SignRequest(request, t.core); err != nil {
		return nil, err
	}
	return t.base.RoundTrip(request)
}

func (h *LocalHandler) handleProviderProxy(c *gin.Context) {
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	ip := net.ParseIP(host)
	if err != nil || ip == nil || !ip.IsLoopback() {
		c.JSON(403, gin.H{"message": "仅允许本机访问云端连接通道"})
		return
	}
	h.providerMu.Lock()
	providerContext, paused := h.providerContext, h.providerPaused
	h.providerMu.Unlock()
	if paused || providerContext == nil || providerContext.Err() != nil {
		h.mu.RLock()
		pending := h.pendingCore
		h.mu.RUnlock()
		message := "服务提供者切换已暂停，请完成配对后继续"
		if pending != "" {
			message = "云端服务将切换至「" + pending + "」，当前回复已中断，正在等待新服务提供者批准配对"
		}
		c.JSON(503, gin.H{"code": "provider_paused", "message": message, "successorCoreId": pending})
		return
	}
	requestContext, cancel := context.WithCancel(c.Request.Context())
	stop := context.AfterFunc(providerContext, cancel)
	defer stop()
	defer cancel()
	c.Request = c.Request.WithContext(requestContext)
	credential, err := h.credStore.LoadCredential()
	if err != nil || credential == nil || !time.Now().Before(credential.ExpiresAt) {
		c.JSON(401, gin.H{"message": "云端绑定凭证不可用，请重新配对"})
		return
	}
	path := c.Param("path")
	if (!strings.HasPrefix(path, "/api/") && path != "/readyz" && path != "/livez") || strings.Contains(path, "..") || strings.Contains(path, "\\") || (strings.HasPrefix(path, "/api/public/device-mesh/") && path != "/api/public/device-mesh/v1/pairing/status") {
		c.JSON(403, gin.H{"message": "不允许转发该服务路径"})
		return
	}
	target, err := url.Parse(credential.CloudBaseUrl)
	if err != nil || target.Host == "" || target.User != nil || (target.Scheme != "https" && target.Scheme != "http") {
		c.JSON(503, gin.H{"message": "云端服务地址无效"})
		return
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if credential.Fingerprint != "" {
		configuration, err := lan.PinnedTLS(lan.Endpoint{URL: credential.CloudBaseUrl, Fingerprint: credential.Fingerprint, CoreID: credential.SpaceID.String()})
		if err != nil {
			c.JSON(403, gin.H{"message": "云端服务身份无效"})
			return
		}
		transport.TLSClientConfig = configuration
		transport.Proxy = nil
	}
	defer transport.CloseIdleConnections()
	proxy := &httputil.ReverseProxy{
		Transport:     identityTransport{base: transport, identity: h.identity, core: credential.SpaceID.String()},
		FlushInterval: -1,
		Rewrite: func(request *httputil.ProxyRequest) {
			request.SetURL(target)
			request.Out.URL.Path = path
			request.Out.URL.RawPath = ""
			query := request.Out.URL.Query()
			query.Del("token")
			request.Out.URL.RawQuery = query.Encode()
			request.Out.Host = target.Host
			forwarded := make(http.Header)
			for _, name := range []string{"Accept", "Accept-Encoding", "Content-Type", "Range", "If-Range", "If-Match", "If-None-Match", "X-Request-ID", "X-Amitia-Target-Device-ID", "X-Device-Timezone", "Idempotency-Key", "X-Amitia-Client-Type"} {
				if values := request.In.Header.Values(name); len(values) > 0 {
					forwarded[name] = append([]string(nil), values...)
				}
			}
			request.Out.Header = forwarded
			if strings.EqualFold(request.In.Header.Get("Upgrade"), "websocket") {
				request.Out.Header.Set("Connection", "Upgrade")
				request.Out.Header.Set("Upgrade", "websocket")
				for _, name := range []string{"Sec-WebSocket-Key", "Sec-WebSocket-Version", "Sec-WebSocket-Protocol", "Sec-WebSocket-Extensions"} {
					if value := request.In.Header.Get(name); value != "" {
						request.Out.Header.Set(name, value)
					}
				}
			}
			request.Out.Header.Set("Authorization", "AmitiaDevice "+credential.Credential)
			request.Out.Header.Set("X-Amitia-Space-ID", credential.SpaceID.String())
			request.Out.Header.Set("X-Amitia-Device-ID", credential.DeviceID.String())
			request.Out.Header.Set("X-Amitia-Runtime-ID", credential.RuntimeID.String())
		},
		ModifyResponse: func(response *http.Response) error {
			response.Header.Del("Set-Cookie")
			if response.StatusCode >= 300 && response.StatusCode < 400 {
				return errors.New("云端服务重定向被拦截")
			}
			return nil
		},
		ErrorHandler: func(writer http.ResponseWriter, _ *http.Request, _ error) {
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(502)
			_, _ = writer.Write([]byte(`{"message":"云端服务连接失败，当前操作未自动重试"}`))
		},
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<20)
	proxy.ServeHTTP(c.Writer, c.Request)
}
