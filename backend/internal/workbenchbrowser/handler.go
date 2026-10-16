package workbenchbrowser

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/browser"
	"github.com/u-ai/backend/internal/workbenchhttp"
)

type page struct {
	session     browser.BrowserSessionID
	tab         browser.BrowserTabID
	created     time.Time
	lastUsed    time.Time
	observed    bool
	observerRef string
}

type Handler struct {
	provider browser.BrowserProvider
	mu       sync.Mutex
	openMu   sync.Mutex
	pages    map[string]page
}

var validIdentifier = regexp.MustCompile(`^[a-zA-Z0-9_:.-]{1,160}$`)

const maxSessionsPerConversation = 8
const maxSessionsPerRuntime = 128
const sessionIdleTTL = 30 * time.Minute
const maxBrowserInputText = 8192
const maxBrowserScreenshotBytes = 6 * 1024 * 1024
const maxBrowserDOMBytes = 1024 * 1024

func NewHandler(provider browser.BrowserProvider) *Handler {
	return &Handler{provider: provider, pages: make(map[string]page)}
}

func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	group := r.Group("/workbench/browser")
	group.Use(func(c *gin.Context) {
		workbenchhttp.ProtectResponse(c.Writer.Header())
		if !workbenchhttp.LimitRequest(c.Writer, c.Request, workbenchhttp.BrowserCommandLimit) {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": "浏览器请求超过大小上限"})
			return
		}
		c.Next()
	})
	group.GET("/capabilities", h.capabilities)
	group.Use(func(c *gin.Context) {
		actor, authenticated := auth.FromContext(c.Request.Context())
		if !authenticated || actor == nil || actor.SpaceID == "" || actor.DeviceID == "" ||
			!actor.HasPermission(auth.PermWorkbenchBrowserControl) ||
			(actor.PrincipalType != auth.PrincipalTrustedDevice &&
				(actor.PrincipalType != auth.PrincipalLocalUI || !actor.IsLocalTrusted)) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "当前身份没有工作台浏览器权限"})
			return
		}
		c.Next()
	})
	group.POST("/open", h.open)
	group.POST("/observe", h.observe)
	group.GET("/:conversationId/:viewId", h.state)
	group.POST("/:conversationId/:viewId/navigate", h.navigate)
	group.POST("/:conversationId/:viewId/action", h.action)
	group.GET("/:conversationId/:viewId/screenshot", h.screenshot)
	group.GET("/:conversationId/:viewId/dom", h.dom)
	group.POST("/:conversationId/:viewId/input", h.input)
	group.POST("/:conversationId/:viewId/control", h.control)
	group.DELETE("/:conversationId/:viewId", h.close)
}

func validateURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Hostname() != "" && parsed.User == nil && len(raw) <= 4096
}

func ownerKey(c *gin.Context, conversationId, viewId string) (string, bool) {
	actor, ok := auth.FromContext(c.Request.Context())
	if !ok || actor == nil || actor.SpaceID == "" || !validIdentifier.MatchString(conversationId) || !validIdentifier.MatchString(viewId) {
		return "", false
	}
	return fmt.Sprintf("%q:%q:%q:%q:%q:%q", actor.SpaceID, actor.PrincipalType, actor.DeviceID, actor.SessionID, conversationId, viewId), true
}

func responseError(c *gin.Context, code int, message string) {
	c.JSON(code, gin.H{"error": message})
}

func (h *Handler) available() bool {
	if h.provider == nil {
		return false
	}
	caps := h.provider.BrowserCapabilities()
	if !caps.SupportsNavigation || !caps.SupportsScreenshot {
		return false
	}
	_, supported := h.provider.Resources().(browser.BrowserScreenshotBytes)
	return supported
}

func (h *Handler) capabilities(c *gin.Context) {
	enabled := h.available()
	c.JSON(http.StatusOK, gin.H{"available": enabled, "supportsScreenshot": enabled, "supportsDOM": enabled && h.provider.BrowserCapabilities().SupportsDOM})
}

func (h *Handler) requirePage(c *gin.Context) (page, string, bool) {
	key, ok := ownerKey(c, c.Param("conversationId"), c.Param("viewId"))
	if !ok {
		responseError(c, http.StatusForbidden, "工作区身份无效")
		return page{}, "", false
	}
	h.mu.Lock()
	record, exists := h.pages[key]
	if exists {
		record.lastUsed = time.Now()
		h.pages[key] = record
	}
	h.mu.Unlock()
	if !exists {
		responseError(c, http.StatusNotFound, "浏览器会话不存在，请重新打开")
		return page{}, "", false
	}
	if record.observed {
		actor, authenticated := auth.FromContext(c.Request.Context())
		if !authenticated || actor == nil {
			responseError(c, http.StatusForbidden, "浏览器观察身份无效")
			return page{}, "", false
		}
		session, tab, allowed := ResolveAgentTab(string(actor.SpaceID), c.Param("conversationId"), record.observerRef)
		if !allowed || session != record.session || tab != record.tab {
			releaseAgentControl(record.observerRef, key, record.session, record.tab)
			h.mu.Lock()
			if current, exists := h.pages[key]; exists && current.observed && current.observerRef == record.observerRef && current.created.Equal(record.created) {
				delete(h.pages, key)
			}
			h.mu.Unlock()
			responseError(c, http.StatusGone, "Agent 浏览器观察许可已失效")
			return page{}, "", false
		}
	}
	return record, key, true
}

func (h *Handler) observe(c *gin.Context) {
	if !h.available() {
		responseError(c, http.StatusServiceUnavailable, "Browser Runtime 未启用")
		return
	}
	var req struct {
		ConversationID string `json:"conversationId"`
		ViewID         string `json:"viewId"`
		BrowserRef     string `json:"browserRef"`
	}
	if c.ShouldBindJSON(&req) != nil || !validBrowserRef(req.BrowserRef) {
		responseError(c, http.StatusBadRequest, "观察请求无效")
		return
	}
	key, ok := ownerKey(c, req.ConversationID, req.ViewID)
	if !ok {
		responseError(c, http.StatusForbidden, "工作区身份无效")
		return
	}
	actor, ok := auth.FromContext(c.Request.Context())
	if !ok || actor == nil {
		responseError(c, http.StatusForbidden, "没有授权的浏览器身份")
		return
	}
	session, tabID, allowed := ResolveAgentTab(string(actor.SpaceID), req.ConversationID, req.BrowserRef)
	if !allowed {
		responseError(c, http.StatusForbidden, "Agent 浏览器引用失效或无访问权限")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	tab, err := h.provider.Tabs().GetTab(ctx, session, tabID)
	if err != nil {
		responseError(c, http.StatusNotFound, "Agent 浏览器页面已结束")
		return
	}
	h.openMu.Lock()
	defer h.openMu.Unlock()
	h.pruneIdle(c.Request.Context())
	h.mu.Lock()
	existing, exists := h.pages[key]
	if exists && (!existing.observed || existing.session != session || existing.tab != tabID) {
		h.mu.Unlock()
		responseError(c, http.StatusConflict, "该标签已有其他浏览器会话")
		return
	}
	if !exists && len(h.pages) >= maxSessionsPerRuntime {
		h.mu.Unlock()
		responseError(c, http.StatusTooManyRequests, "浏览器观察标签已达上限")
		return
	}
	if exists && existing.observed && existing.observerRef != req.BrowserRef {
		releaseAgentControl(existing.observerRef, key, existing.session, existing.tab)
	}
	now := time.Now()
	h.pages[key] = page{session: session, tab: tabID, observed: true, observerRef: req.BrowserRef, created: now, lastUsed: now}
	h.mu.Unlock()
	c.JSON(http.StatusOK, gin.H{"ready": true, "observed": true, "url": tab.URL, "title": tab.Title})
}

func (h *Handler) open(c *gin.Context) {
	if !h.available() {
		responseError(c, http.StatusServiceUnavailable, "Browser Runtime 未启用或缺少必要能力")
		return
	}
	var req struct {
		ConversationID string `json:"conversationId"`
		ViewID         string `json:"viewId"`
		URL            string `json:"url"`
	}
	if c.ShouldBindJSON(&req) != nil || (req.URL != "" && !validateURL(req.URL)) {
		responseError(c, http.StatusBadRequest, "浏览器请求无效")
		return
	}
	key, ok := ownerKey(c, req.ConversationID, req.ViewID)
	if !ok {
		responseError(c, http.StatusForbidden, "工作区身份无效")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 35*time.Second)
	defer cancel()
	h.openMu.Lock()
	defer h.openMu.Unlock()
	h.pruneIdle(c.Request.Context())
	h.mu.Lock()
	record, exists := h.pages[key]
	h.mu.Unlock()
	if exists && record.observed {
		responseError(c, http.StatusConflict, "该标签正在观察 Agent 浏览器，无法转换成独立会话")
		return
	}
	if exists {
		if _, checkErr := h.provider.Tabs().GetTab(ctx, record.session, record.tab); checkErr != nil {
			_ = h.provider.Sessions().CloseSession(ctx, record.session)
			h.mu.Lock()
			delete(h.pages, key)
			h.mu.Unlock()
			exists = false
		}
	}
	if !exists {
		prefix := strings.TrimSuffix(key, fmt.Sprintf("%q", req.ViewID))
		count := 0
		h.mu.Lock()
		for candidate := range h.pages {
			if strings.HasPrefix(candidate, prefix) {
				count++
			}
		}
		full := count >= maxSessionsPerConversation || len(h.pages) >= maxSessionsPerRuntime
		h.mu.Unlock()
		if full {
			responseError(c, http.StatusTooManyRequests, "浏览器会话数量已达上限，请关闭不再使用的标签")
			return
		}
		status := h.provider.Runtime().Status(ctx)
		if status.State == browser.BrowserRuntimeStopped || status.State == browser.BrowserRuntimeFailed {
			if _, err := h.provider.Runtime().Start(ctx); err != nil {
				responseError(c, http.StatusServiceUnavailable, err.Message)
				return
			}
		}
		session, err := h.provider.Sessions().CreateSession(ctx)
		if err != nil {
			responseError(c, http.StatusServiceUnavailable, err.Message)
			return
		}
		tab, err := h.provider.Tabs().CreateTab(ctx, session.SessionID)
		if err != nil {
			_ = h.provider.Sessions().CloseSession(ctx, session.SessionID)
			responseError(c, http.StatusServiceUnavailable, err.Message)
			return
		}
		now := time.Now()
		record = page{session: session.SessionID, tab: tab.TabID, created: now, lastUsed: now}
		h.mu.Lock()
		h.pages[key] = record
		h.mu.Unlock()
	}
	if req.URL != "" {
		result, err := h.provider.Navigate().Navigate(ctx, record.session, record.tab, browser.NavigateRequest{URL: req.URL})
		if err != nil {
			responseError(c, http.StatusBadGateway, err.Message)
			return
		}
		c.JSON(http.StatusOK, gin.H{"ready": true, "url": result.FinalURL, "title": result.Title})
		return
	}
	tab, err := h.provider.Tabs().GetTab(ctx, record.session, record.tab)
	if err != nil {
		responseError(c, http.StatusBadGateway, err.Message)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ready": true, "url": tab.URL, "title": tab.Title})
}

func (h *Handler) state(c *gin.Context) {
	record, _, ok := h.requirePage(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
	defer cancel()
	tab, err := h.provider.Tabs().GetTab(ctx, record.session, record.tab)
	if err != nil {
		responseError(c, http.StatusBadGateway, err.Message)
		return
	}
	key, _ := ownerKey(c, c.Param("conversationId"), c.Param("viewId"))
	canControl := record.observed && hasAgentControl(record.observerRef, key, record.session, record.tab)
	token := ""
	if canControl {
		actor, ok := auth.FromContext(c.Request.Context())
		if ok && actor != nil && actor.HasPermission(auth.PermWorkbenchBrowserControl) &&
			(actor.PrincipalType == auth.PrincipalTrustedDevice || (actor.PrincipalType == auth.PrincipalLocalUI && actor.IsLocalTrusted)) {
			token = agentControlLeaseToken(record.observerRef, key, record.session, record.tab)
		} else {
			canControl = false
		}
	}
	if token == "" {
		canControl = false
	}
	c.JSON(http.StatusOK, gin.H{"ready": true, "url": tab.URL, "title": tab.Title, "state": tab.State, "createdAt": record.created, "observed": record.observed, "canControl": canControl, "leaseId": token})
}

func (h *Handler) beginBrowserMutation(c *gin.Context, record page, token string) (func(), bool) {
	if !record.observed {
		return func() {}, true
	}
	key, ok := ownerKey(c, c.Param("conversationId"), c.Param("viewId"))
	if !ok {
		responseError(c, http.StatusForbidden, "工作区身份无效")
		return nil, false
	}
	actor, authenticated := auth.FromContext(c.Request.Context())
	if !authenticated || actor == nil || !actor.HasPermission(auth.PermWorkbenchBrowserControl) ||
		(actor.PrincipalType != auth.PrincipalLocalUI && actor.PrincipalType != auth.PrincipalTrustedDevice) ||
		(actor.PrincipalType == auth.PrincipalLocalUI && !actor.IsLocalTrusted) {
		responseError(c, http.StatusForbidden, "当前设备没有浏览器接管权限")
		return nil, false
	}
	finish, allowed := BeginManualTabMutation(record.observerRef, key, token, record.session, record.tab)
	if !allowed {
		responseError(c, http.StatusForbidden, "Agent 浏览器控制许可已失效或正在由 Agent 操作")
		return nil, false
	}
	return finish, true
}

func (h *Handler) control(c *gin.Context) {
	record, key, ok := h.requirePage(c)
	if !ok {
		return
	}
	if !record.observed {
		responseError(c, http.StatusConflict, "独立浏览器会话不需要接管")
		return
	}
	actor, authenticated := auth.FromContext(c.Request.Context())
	if !authenticated || actor == nil || !actor.HasPermission(auth.PermWorkbenchBrowserControl) ||
		(actor.PrincipalType != auth.PrincipalLocalUI && actor.PrincipalType != auth.PrincipalTrustedDevice) ||
		(actor.PrincipalType == auth.PrincipalLocalUI && !actor.IsLocalTrusted) {
		responseError(c, http.StatusForbidden, "当前设备没有浏览器接管权限")
		return
	}
	var request struct {
		Action  string `json:"action"`
		LeaseID string `json:"leaseId"`
	}
	if c.ShouldBindJSON(&request) != nil {
		responseError(c, http.StatusBadRequest, "接管请求无效")
		return
	}
	switch request.Action {
	case "acquire":
		deadline, token, granted := acquireAgentControlWithToken(record.observerRef, key, record.session, record.tab)
		if !granted {
			responseError(c, http.StatusConflict, "浏览器正由另一个观察者控制")
			return
		}
		c.JSON(http.StatusOK, gin.H{"canControl": true, "leaseId": token, "expiresAt": deadline, "leaseSeconds": int(agentControlLeaseTTL.Seconds())})
	case "renew":
		deadline, granted := renewAgentControl(record.observerRef, key, request.LeaseID, record.session, record.tab)
		if !granted {
			responseError(c, http.StatusConflict, "控制权已改变或到期，请重新申请")
			return
		}
		c.JSON(http.StatusOK, gin.H{"canControl": true, "leaseId": request.LeaseID, "expiresAt": deadline, "leaseSeconds": int(agentControlLeaseTTL.Seconds())})
	case "release":
		if !releaseAgentControlWithToken(record.observerRef, key, request.LeaseID, record.session, record.tab) {
			responseError(c, http.StatusConflict, "控制权代次不匹配")
			return
		}
		c.JSON(http.StatusOK, gin.H{"canControl": false, "leaseId": ""})
	default:
		responseError(c, http.StatusBadRequest, "不支持的控制权操作")
	}
}

func (h *Handler) navigate(c *gin.Context) {
	record, _, ok := h.requirePage(c)
	if !ok {
		return
	}
	var req struct {
		URL     string `json:"url"`
		LeaseID string `json:"leaseId"`
	}
	if c.ShouldBindJSON(&req) != nil || !validateURL(req.URL) {
		responseError(c, http.StatusBadRequest, "必须提供有效的 HTTP(S) 地址")
		return
	}
	finish, permitted := h.beginBrowserMutation(c, record, req.LeaseID)
	if !permitted {
		return
	}
	defer finish()
	ctx, cancel := context.WithTimeout(c.Request.Context(), 35*time.Second)
	defer cancel()
	result, err := h.provider.Navigate().Navigate(ctx, record.session, record.tab, browser.NavigateRequest{URL: req.URL})
	if err != nil {
		responseError(c, http.StatusBadGateway, err.Message)
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": result.FinalURL, "title": result.Title, "loaded": result.Loaded})
}

func (h *Handler) action(c *gin.Context) {
	record, _, ok := h.requirePage(c)
	if !ok {
		return
	}
	var req struct {
		Action  string `json:"action"`
		LeaseID string `json:"leaseId"`
	}
	if c.ShouldBindJSON(&req) != nil {
		responseError(c, http.StatusBadRequest, "操作请求无效")
		return
	}
	finish, permitted := h.beginBrowserMutation(c, record, req.LeaseID)
	if !permitted {
		return
	}
	defer finish()
	ctx, cancel := context.WithTimeout(c.Request.Context(), 35*time.Second)
	defer cancel()
	var result browser.NavigationResult
	var err *browser.BrowserError
	switch req.Action {
	case "back":
		result, err = h.provider.Navigate().GoBack(ctx, record.session, record.tab)
	case "forward":
		result, err = h.provider.Navigate().GoForward(ctx, record.session, record.tab)
	case "reload":
		result, err = h.provider.Navigate().Reload(ctx, record.session, record.tab, browser.NavigateRequest{})
	case "stop":
		err = h.provider.Navigate().Stop(ctx, record.session, record.tab)
	default:
		responseError(c, http.StatusBadRequest, "不支持的浏览器操作")
		return
	}
	if err != nil {
		responseError(c, http.StatusBadGateway, err.Message)
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": result.FinalURL, "title": result.Title})
}

func (h *Handler) screenshot(c *gin.Context) {
	record, _, ok := h.requirePage(c)
	if !ok {
		return
	}
	capture, supported := h.provider.Resources().(browser.BrowserScreenshotBytes)
	if !supported {
		responseError(c, http.StatusNotImplemented, "当前浏览器截图无法直接传输")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	data, err := capture.ScreenshotBytes(ctx, browser.BrowserScreenshotRequest{SessionID: record.session, TabID: record.tab, Format: "jpeg", Quality: 70})
	if err != nil {
		responseError(c, http.StatusBadGateway, err.Message)
		return
	}
	if len(data) > maxBrowserScreenshotBytes {
		responseError(c, http.StatusRequestEntityTooLarge, "浏览器截图超过 6 MiB 限额")
		return
	}
	c.Header("Cache-Control", "no-store, private")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, "image/jpeg", data)
}

func (h *Handler) dom(c *gin.Context) {
	record, _, ok := h.requirePage(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	snapshot, err := h.provider.Observe().GetDOMSnapshot(ctx, record.session, record.tab, 3)
	if err != nil {
		responseError(c, http.StatusBadGateway, err.Message)
		return
	}
	body, wasTrimmed := boundDOMContent(snapshot.Content)
	c.Header("Cache-Control", "no-store, private")
	c.JSON(http.StatusOK, gin.H{"url": snapshot.URL, "title": snapshot.Title, "content": body, "truncated": snapshot.Truncated || wasTrimmed})
}

func (h *Handler) close(c *gin.Context) {
	record, key, ok := h.requirePage(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	h.openMu.Lock()
	defer h.openMu.Unlock()
	h.mu.Lock()
	current, exists := h.pages[key]
	h.mu.Unlock()
	if !exists || current.session != record.session || current.tab != record.tab || !current.created.Equal(record.created) {
		responseError(c, http.StatusConflict, "浏览器会话已变更")
		return
	}
	if record.observed {
		releaseAgentControl(record.observerRef, key, record.session, record.tab)
		h.mu.Lock()
		delete(h.pages, key)
		h.mu.Unlock()
		c.JSON(http.StatusOK, gin.H{"closed": true, "detached": true})
		return
	}
	if err := h.provider.Sessions().CloseSession(ctx, record.session); err != nil {
		responseError(c, http.StatusBadGateway, err.Message)
		return
	}
	h.mu.Lock()
	delete(h.pages, key)
	h.mu.Unlock()
	c.JSON(http.StatusOK, gin.H{"closed": true})
}

func (h *Handler) input(c *gin.Context) {
	record, _, ok := h.requirePage(c)
	if !ok {
		return
	}
	input, supported := h.provider.Resources().(browser.BrowserScreenInput)
	if !supported {
		responseError(c, http.StatusNotImplemented, "当前浏览器不支持输入")
		return
	}
	var req struct {
		Action  string  `json:"action"`
		X       float64 `json:"x"`
		Y       float64 `json:"y"`
		Text    string  `json:"text"`
		Amount  float64 `json:"amount"`
		LeaseID string  `json:"leaseId"`
	}
	if c.ShouldBindJSON(&req) != nil {
		responseError(c, http.StatusBadRequest, "浏览器输入无效")
		return
	}
	finish, permitted := h.beginBrowserMutation(c, record, req.LeaseID)
	if !permitted {
		return
	}
	defer finish()
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	var err *browser.BrowserError
	switch req.Action {
	case "click":
		if !isFinite(req.X) || !isFinite(req.Y) || req.X < 0 || req.Y < 0 || req.X > 16384 || req.Y > 16384 {
			responseError(c, http.StatusBadRequest, "点击坐标无效")
			return
		}
		err = input.ScreenClick(ctx, record.session, record.tab, req.X, req.Y)
	case "text":
		if len(req.Text) > maxBrowserInputText {
			responseError(c, http.StatusBadRequest, "输入文字过长")
			return
		}
		err = input.ScreenText(ctx, record.session, record.tab, req.Text)
	case "scroll":
		if !isFinite(req.Amount) || math.Abs(req.Amount) > 2000 {
			responseError(c, http.StatusBadRequest, "滚动量无效")
			return
		}
		err = input.ScreenScroll(ctx, record.session, record.tab, req.Amount)
	default:
		responseError(c, http.StatusBadRequest, "不支持的输入操作")
		return
	}
	if err != nil {
		responseError(c, http.StatusBadGateway, err.Message)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func (h *Handler) pruneIdle(ctx context.Context) {
	now := time.Now()
	stale := make([]page, 0)
	h.mu.Lock()
	for key, record := range h.pages {
		if len(stale) >= 8 {
			break
		}
		if now.Sub(record.lastUsed) > sessionIdleTTL {
			if record.observed {
				releaseAgentControl(record.observerRef, key, record.session, record.tab)
			}
			if !record.observed {
				stale = append(stale, record)
			}
			delete(h.pages, key)
		}
	}
	h.mu.Unlock()
	for _, record := range stale {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		_ = h.provider.Sessions().CloseSession(cleanupCtx, record.session)
		cancel()
	}
}

// boundDOMContent bounds observation memory and avoids truncating a multi-byte
// UTF-8 character, so JSON clients receive a valid document fragment.
func boundDOMContent(content string) (string, bool) {
	if len(content) <= maxBrowserDOMBytes {
		return content, false
	}
	content = content[:maxBrowserDOMBytes]
	for len(content) > 0 && !utf8.ValidString(content) {
		content = content[:len(content)-1]
	}
	return content, true
}
