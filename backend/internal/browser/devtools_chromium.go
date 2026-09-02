package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

const devToolsEventLimit = 500

type devToolsEvent struct {
	Method    string          `json:"method"`
	SessionID string          `json:"sessionId"`
	Captured  time.Time       `json:"capturedAt"`
	Params    json.RawMessage `json:"params"`
}

type chromiumDevTools struct {
	engine   *chromiumEngine
	resolver TabResolver

	mu          sync.Mutex
	console     map[string][]devToolsEvent
	network     map[string][]devToolsEvent
	subscribed  map[string]bool
	unsubscribe map[string][]func()
}

func NewChromiumDevTools(engine BrowserEngine, resolver TabResolver) BrowserDevTools {
	ce, _ := engine.(*chromiumEngine)
	if ce == nil || resolver == nil {
		return &unsupportedDevTools{}
	}
	return &chromiumDevTools{
		engine: ce, resolver: resolver,
		console: map[string][]devToolsEvent{}, network: map[string][]devToolsEvent{},
		subscribed: map[string]bool{}, unsubscribe: map[string][]func(){},
	}
}

func (d *chromiumDevTools) Execute(ctx context.Context, operation string, sessionID BrowserSessionID, tabID BrowserTabID, input json.RawMessage) (json.RawMessage, *BrowserError) {
	client, cdpSession, generation, berr := d.resolveCDPSession(ctx, sessionID, tabID)
	if berr != nil {
		return nil, berr
	}
	key := fmt.Sprintf("%d:%s", generation, cdpSession)
	if berr := d.ensureEventCapture(ctx, client, cdpSession, key); berr != nil {
		return nil, berr
	}

	var result any
	var err error
	switch operation {
	case "console_messages":
		result, err = d.consoleMessages(key, input)
	case "network_requests":
		result, err = d.networkRequests(key, input)
	case "evaluate":
		result, err = d.evaluate(ctx, client, cdpSession, input, false)
	case "run_code":
		result, err = d.evaluate(ctx, client, cdpSession, input, true)
	case "handle_dialog":
		result, err = d.handleDialog(ctx, client, cdpSession, input)
	case "resize":
		result, err = d.resize(ctx, client, cdpSession, input)
	case "press_key":
		result, err = d.pressKey(ctx, client, cdpSession, input)
	case "drag":
		result, err = d.drag(ctx, client, cdpSession, input)
	case "wait_for":
		result, err = d.waitFor(ctx, client, cdpSession, input)
	case "cookies":
		result, err = d.cookies(ctx, client, cdpSession, input)
	default:
		return nil, &BrowserError{Code: ErrCodeUnsupportedAction, Message: "unsupported DevTools operation: " + operation}
	}
	if err != nil {
		return nil, &BrowserError{Code: ErrCodeInteractionFailed, Message: "browser DevTools operation failed", Cause: err}
	}
	out, err := json.Marshal(result)
	if err != nil {
		return nil, &BrowserError{Code: ErrCodeInteractionFailed, Message: "encode DevTools result failed", Cause: err}
	}
	return out, nil
}

func (d *chromiumDevTools) resolveCDPSession(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID) (*cdpClient, string, uint64, *BrowserError) {
	resolved, berr := d.resolver.ResolveTab(ctx, sessionID, tabID)
	if berr != nil {
		return nil, "", 0, berr
	}
	if !d.engine.runtimeReady() {
		return nil, "", 0, &BrowserError{Code: ErrCodeBrowserRuntimeNotReady, Message: "browser runtime is not ready"}
	}
	client := d.engine.cdpClient()
	if client == nil {
		return nil, "", 0, &BrowserError{Code: ErrCodeBrowserRuntimeNotReady, Message: "CDP client is unavailable"}
	}
	controller, _ := d.engine.Pages().(*chromiumPageController)
	if controller == nil {
		return nil, "", 0, &BrowserError{Code: ErrCodeProviderUnavailable, Message: "Chromium page controller unavailable"}
	}
	cdpSession := controller.ensureSession(ctx, client, resolved.TargetID)
	if cdpSession == "" {
		return nil, "", 0, &BrowserError{Code: ErrCodeBrowserRuntimeNotReady, Message: "failed to attach DevTools session to tab"}
	}
	return client, cdpSession, resolved.RuntimeGeneration, nil
}

func (d *chromiumDevTools) ensureEventCapture(ctx context.Context, client *cdpClient, cdpSession, key string) *BrowserError {
	d.mu.Lock()
	if d.subscribed[key] {
		d.mu.Unlock()
		return nil
	}
	// Reserve the key while domains are being enabled. On failure the
	// reservation is removed so a later call can retry instead of leaving a
	// permanently half-initialized DevTools session.
	d.subscribed[key] = true
	d.mu.Unlock()
	rollback := func() {
		d.mu.Lock()
		delete(d.subscribed, key)
		d.mu.Unlock()
	}
	if err := client.Call(ctx, "Runtime.enable", cdpSession, nil, nil); err != nil {
		rollback()
		return &BrowserError{Code: ErrCodeInteractionFailed, Message: "enable Runtime domain failed", Cause: err}
	}
	if err := client.Call(ctx, "Network.enable", cdpSession, map[string]any{"maxTotalBufferSize": 10 * 1024 * 1024, "maxResourceBufferSize": 2 * 1024 * 1024}, nil); err != nil {
		rollback()
		return &BrowserError{Code: ErrCodeInteractionFailed, Message: "enable Network domain failed", Cause: err}
	}
	consoleMethods := []string{"Runtime.consoleAPICalled", "Runtime.exceptionThrown"}
	networkMethods := []string{"Network.requestWillBeSent", "Network.responseReceived", "Network.loadingFailed", "Network.loadingFinished"}
	unsubs := make([]func(), 0, len(consoleMethods)+len(networkMethods))
	for _, method := range consoleMethods {
		m := method
		unsubs = append(unsubs, client.SubscribeEventWithSession(m, func(eventSession string, params json.RawMessage) {
			if eventSession != cdpSession {
				return
			}
			d.appendEvent(d.console, key, devToolsEvent{Method: m, SessionID: eventSession, Captured: time.Now().UTC(), Params: cloneRaw(params)})
		}))
	}
	for _, method := range networkMethods {
		m := method
		unsubs = append(unsubs, client.SubscribeEventWithSession(m, func(eventSession string, params json.RawMessage) {
			if eventSession != cdpSession {
				return
			}
			d.appendEvent(d.network, key, devToolsEvent{Method: m, SessionID: eventSession, Captured: time.Now().UTC(), Params: cloneRaw(params)})
		}))
	}
	d.mu.Lock()
	d.unsubscribe[key] = unsubs
	d.mu.Unlock()
	return nil
}

func cloneRaw(in json.RawMessage) json.RawMessage { return append(json.RawMessage(nil), in...) }
func (d *chromiumDevTools) appendEvent(store map[string][]devToolsEvent, key string, evt devToolsEvent) {
	d.mu.Lock()
	defer d.mu.Unlock()
	items := append(store[key], evt)
	if len(items) > devToolsEventLimit {
		items = append([]devToolsEvent(nil), items[len(items)-devToolsEventLimit:]...)
	}
	store[key] = items
}

func (d *chromiumDevTools) consoleMessages(key string, input json.RawMessage) (any, error) {
	return d.readEvents(d.console, key, input)
}
func (d *chromiumDevTools) networkRequests(key string, input json.RawMessage) (any, error) {
	return d.readEvents(d.network, key, input)
}
func (d *chromiumDevTools) readEvents(store map[string][]devToolsEvent, key string, input json.RawMessage) (any, error) {
	var req struct {
		Limit int  `json:"limit"`
		Clear bool `json:"clear"`
	}
	_ = json.Unmarshal(input, &req)
	if req.Limit <= 0 {
		req.Limit = 100
	}
	if req.Limit > devToolsEventLimit {
		req.Limit = devToolsEventLimit
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	items := store[key]
	start := 0
	if len(items) > req.Limit {
		start = len(items) - req.Limit
	}
	out := append([]devToolsEvent(nil), items[start:]...)
	if req.Clear {
		delete(store, key)
	}
	return map[string]any{"events": out, "count": len(out)}, nil
}

func (d *chromiumDevTools) evaluate(ctx context.Context, client *cdpClient, session string, input json.RawMessage, codeMode bool) (any, error) {
	var req struct {
		Expression    string `json:"expression"`
		Code          string `json:"code"`
		AwaitPromise  *bool  `json:"awaitPromise"`
		UserGesture   bool   `json:"userGesture"`
		ReturnByValue *bool  `json:"returnByValue"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	expr := strings.TrimSpace(req.Expression)
	if codeMode {
		code := req.Code
		if strings.TrimSpace(code) == "" {
			code = req.Expression
		}
		if strings.TrimSpace(code) == "" {
			return nil, fmt.Errorf("code is required")
		}
		expr = "(async () => {\n" + code + "\n})()"
	} else if expr == "" {
		return nil, fmt.Errorf("expression is required")
	}
	await := true
	if req.AwaitPromise != nil {
		await = *req.AwaitPromise
	}
	returnByValue := true
	if req.ReturnByValue != nil {
		returnByValue = *req.ReturnByValue
	}
	params := map[string]any{"expression": expr, "awaitPromise": await, "returnByValue": returnByValue, "userGesture": req.UserGesture, "generatePreview": true}
	var result map[string]any
	if err := client.Call(ctx, "Runtime.evaluate", session, params, &result); err != nil {
		return nil, err
	}
	if exception := result["exceptionDetails"]; exception != nil {
		return map[string]any{"ok": false, "exceptionDetails": exception, "result": result["result"]}, nil
	}
	return map[string]any{"ok": true, "result": result["result"]}, nil
}

func (d *chromiumDevTools) handleDialog(ctx context.Context, client *cdpClient, session string, input json.RawMessage) (any, error) {
	var req struct {
		Accept     bool   `json:"accept"`
		PromptText string `json:"promptText"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	if err := client.Call(ctx, "Page.handleJavaScriptDialog", session, map[string]any{"accept": req.Accept, "promptText": req.PromptText}, nil); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "accepted": req.Accept}, nil
}

func (d *chromiumDevTools) resize(ctx context.Context, client *cdpClient, session string, input json.RawMessage) (any, error) {
	var req struct {
		Width             int     `json:"width"`
		Height            int     `json:"height"`
		DeviceScaleFactor float64 `json:"deviceScaleFactor"`
		Mobile            bool    `json:"mobile"`
		Clear             bool    `json:"clear"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	if req.Clear {
		if err := client.Call(ctx, "Emulation.clearDeviceMetricsOverride", session, nil, nil); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "cleared": true}, nil
	}
	if req.Width < 1 || req.Height < 1 || req.Width > 10000 || req.Height > 10000 {
		return nil, fmt.Errorf("width and height must be between 1 and 10000")
	}
	if req.DeviceScaleFactor <= 0 {
		req.DeviceScaleFactor = 1
	}
	if req.DeviceScaleFactor > 10 {
		return nil, fmt.Errorf("deviceScaleFactor is too large")
	}
	if err := client.Call(ctx, "Emulation.setDeviceMetricsOverride", session, map[string]any{"width": req.Width, "height": req.Height, "deviceScaleFactor": req.DeviceScaleFactor, "mobile": req.Mobile}, nil); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "width": req.Width, "height": req.Height, "deviceScaleFactor": req.DeviceScaleFactor, "mobile": req.Mobile}, nil
}

func (d *chromiumDevTools) pressKey(ctx context.Context, client *cdpClient, session string, input json.RawMessage) (any, error) {
	var req struct {
		Key       string `json:"key"`
		Code      string `json:"code"`
		Text      string `json:"text"`
		KeyCode   int    `json:"keyCode"`
		Modifiers int    `json:"modifiers"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	if req.Key == "" && req.Text == "" {
		return nil, fmt.Errorf("key or text is required")
	}
	base := map[string]any{"key": req.Key, "code": req.Code, "modifiers": req.Modifiers}
	if req.KeyCode > 0 {
		base["windowsVirtualKeyCode"] = req.KeyCode
		base["nativeVirtualKeyCode"] = req.KeyCode
	}
	if req.Text != "" {
		base["text"] = req.Text
	}
	down := cloneMap(base)
	down["type"] = "keyDown"
	if err := client.Call(ctx, "Input.dispatchKeyEvent", session, down, nil); err != nil {
		return nil, err
	}
	up := cloneMap(base)
	up["type"] = "keyUp"
	delete(up, "text")
	if err := client.Call(ctx, "Input.dispatchKeyEvent", session, up, nil); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "key": req.Key, "text": req.Text}, nil
}
func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func (d *chromiumDevTools) drag(ctx context.Context, client *cdpClient, session string, input json.RawMessage) (any, error) {
	var req struct {
		FromX      float64 `json:"fromX"`
		FromY      float64 `json:"fromY"`
		ToX        float64 `json:"toX"`
		ToY        float64 `json:"toY"`
		Steps      int     `json:"steps"`
		DurationMS int     `json:"durationMs"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	if req.Steps < 1 {
		req.Steps = 8
	}
	if req.Steps > 100 {
		req.Steps = 100
	}
	if req.DurationMS < 0 {
		req.DurationMS = 0
	}
	if req.DurationMS > 10000 {
		req.DurationMS = 10000
	}
	call := func(typ string, x, y float64, buttons int) error {
		return client.Call(ctx, "Input.dispatchMouseEvent", session, map[string]any{"type": typ, "x": x, "y": y, "button": "left", "buttons": buttons, "clickCount": 1}, nil)
	}
	if err := call("mousePressed", req.FromX, req.FromY, 1); err != nil {
		return nil, err
	}
	delay := time.Duration(0)
	if req.DurationMS > 0 {
		delay = time.Duration(req.DurationMS) * time.Millisecond / time.Duration(req.Steps)
	}
	for i := 1; i <= req.Steps; i++ {
		ratio := float64(i) / float64(req.Steps)
		x := req.FromX + (req.ToX-req.FromX)*ratio
		y := req.FromY + (req.ToY-req.FromY)*ratio
		if err := call("mouseMoved", x, y, 1); err != nil {
			return nil, err
		}
		if delay > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	if err := call("mouseReleased", req.ToX, req.ToY, 0); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "steps": req.Steps}, nil
}

func (d *chromiumDevTools) waitFor(ctx context.Context, client *cdpClient, session string, input json.RawMessage) (any, error) {
	var req struct {
		Selector       string `json:"selector"`
		Text           string `json:"text"`
		Expression     string `json:"expression"`
		TimeoutMS      int    `json:"timeoutMs"`
		PollIntervalMS int    `json:"pollIntervalMs"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	if req.Selector == "" && req.Text == "" && req.Expression == "" {
		return nil, fmt.Errorf("selector, text or expression is required")
	}
	if req.TimeoutMS <= 0 {
		req.TimeoutMS = 30000
	}
	if req.TimeoutMS > 120000 {
		req.TimeoutMS = 120000
	}
	if req.PollIntervalMS <= 0 {
		req.PollIntervalMS = 250
	}
	if req.PollIntervalMS < 50 {
		req.PollIntervalMS = 50
	}
	if req.PollIntervalMS > 5000 {
		req.PollIntervalMS = 5000
	}
	expr := ""
	switch {
	case req.Expression != "":
		expr = "Boolean(" + req.Expression + ")"
	case req.Selector != "":
		expr = "document.querySelector(" + strconv.Quote(req.Selector) + ") !== null"
	default:
		expr = "Boolean(document.body && document.body.innerText.includes(" + strconv.Quote(req.Text) + "))"
	}
	deadline := time.Now().Add(time.Duration(req.TimeoutMS) * time.Millisecond)
	attempts := 0
	for {
		attempts++
		var response struct {
			Result struct {
				Value any `json:"value"`
			} `json:"result"`
			ExceptionDetails any `json:"exceptionDetails"`
		}
		if err := client.Call(ctx, "Runtime.evaluate", session, map[string]any{"expression": expr, "returnByValue": true, "awaitPromise": true}, &response); err == nil && response.ExceptionDetails == nil {
			if ok, _ := response.Result.Value.(bool); ok {
				return map[string]any{"ok": true, "matched": true, "attempts": attempts, "elapsedMs": req.TimeoutMS - int(time.Until(deadline).Milliseconds())}, nil
			}
		}
		if time.Now().After(deadline) {
			return map[string]any{"ok": false, "matched": false, "timedOut": true, "attempts": attempts}, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(req.PollIntervalMS) * time.Millisecond):
		}
	}
}

func (d *chromiumDevTools) cookies(ctx context.Context, client *cdpClient, session string, input json.RawMessage) (any, error) {
	var req struct {
		Action   string   `json:"action"`
		URLs     []string `json:"urls"`
		Name     string   `json:"name"`
		Value    string   `json:"value"`
		URL      string   `json:"url"`
		Domain   string   `json:"domain"`
		Path     string   `json:"path"`
		Secure   bool     `json:"secure"`
		HTTPOnly bool     `json:"httpOnly"`
		SameSite string   `json:"sameSite"`
		Expires  float64  `json:"expires"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action == "" {
		action = "get"
	}
	switch action {
	case "get", "list":
		var out map[string]any
		method := "Network.getAllCookies"
		params := any(nil)
		if len(req.URLs) > 0 {
			method = "Network.getCookies"
			params = map[string]any{"urls": req.URLs}
		}
		if err := client.Call(ctx, method, session, params, &out); err != nil {
			return nil, err
		}
		return out, nil
	case "set":
		if req.Name == "" {
			return nil, fmt.Errorf("name is required")
		}
		params := map[string]any{"name": req.Name, "value": req.Value, "secure": req.Secure, "httpOnly": req.HTTPOnly}
		if req.URL != "" {
			params["url"] = req.URL
		}
		if req.Domain != "" {
			params["domain"] = req.Domain
		}
		if req.Path != "" {
			params["path"] = req.Path
		}
		if req.SameSite != "" {
			params["sameSite"] = req.SameSite
		}
		if req.Expires != 0 {
			params["expires"] = req.Expires
		}
		var out map[string]any
		if err := client.Call(ctx, "Network.setCookie", session, params, &out); err != nil {
			return nil, err
		}
		return out, nil
	case "delete":
		if req.Name == "" {
			return nil, fmt.Errorf("name is required")
		}
		params := map[string]any{"name": req.Name}
		if req.URL != "" {
			params["url"] = req.URL
		}
		if req.Domain != "" {
			params["domain"] = req.Domain
		}
		if req.Path != "" {
			params["path"] = req.Path
		}
		if err := client.Call(ctx, "Network.deleteCookies", session, params, nil); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "deleted": true}, nil
	case "clear":
		if err := client.Call(ctx, "Network.clearBrowserCookies", session, nil, nil); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "cleared": true}, nil
	default:
		return nil, fmt.Errorf("unsupported cookie action %q", req.Action)
	}
}
