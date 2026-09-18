package kernel

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/browser"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func makeBrowserCallFunc(provider browser.BrowserProvider) capability.BrowserCallFunc {
	if provider == nil {
		return nil
	}
	return func(ctx context.Context, handlerName string, invocation capability.ToolInvocationContext, input json.RawMessage) (json.RawMessage, error) {
		return dispatchBrowserCall(ctx, provider, handlerName, input)
	}
}

func makeBrowserHealthFunc(provider browser.BrowserProvider) capability.BrowserHealthFunc {
	if provider == nil {
		return nil
	}
	return func(ctx context.Context) capability.HealthStatus {
		return resolveBrowserHealth(ctx, provider)
	}
}

func resolveBrowserHealth(ctx context.Context, provider browser.BrowserProvider) capability.HealthStatus {
	if provider == nil {
		return capability.HealthUnknown
	}
	health := provider.Runtime().Health(ctx)
	switch health {
	case browser.BrowserHealthHealthy:
		return capability.HealthReady
	case browser.BrowserHealthStarting:
		return capability.HealthDegraded
	case browser.BrowserHealthUnhealthy:
		return capability.HealthUnhealthy
	case browser.BrowserHealthUnavailable:
		return capability.HealthShutdown
	}
	return capability.HealthUnknown
}

func dispatchBrowserCall(ctx context.Context, provider browser.BrowserProvider, handlerName string, input json.RawMessage) (json.RawMessage, error) {
	switch handlerName {
	case "browser.session.create":
		return execSessionCreate(ctx, provider, input)
	case "browser.session.close":
		return execSessionClose(ctx, provider, input)
	case "browser.session.get":
		return execSessionGet(ctx, provider, input)
	case "browser.session.list":
		return execSessionList(ctx, provider, input)
	case "browser.tab.create":
		return execTabCreate(ctx, provider, input)
	case "browser.tab.close":
		return execTabClose(ctx, provider, input)
	case "browser.tab.get":
		return execTabGet(ctx, provider, input)
	case "browser.tab.list":
		return execTabList(ctx, provider, input)
	case "browser.tab.activate":
		return execTabActivate(ctx, provider, input)
	case "browser.navigate":
		return execNavigate(ctx, provider, input)
	case "browser.navigate.reload":
		return execReload(ctx, provider, input)
	case "browser.navigate.back":
		return execGoBack(ctx, provider, input)
	case "browser.navigate.forward":
		return execGoForward(ctx, provider, input)
	case "browser.navigate.stop":
		return execStop(ctx, provider, input)
	case "browser.dom.snapshot":
		return execDOMSnapshot(ctx, provider, input)
	case "browser.dom.find":
		return execFindElement(ctx, provider, input)
	case "browser.dom.scrollToElement":
		return execScrollToElement(ctx, provider, input)
	case "browser.interact.click":
		return execClick(ctx, provider, input)
	case "browser.interact.input":
		return execInput(ctx, provider, input)
	case "browser.interact.select":
		return execSelect(ctx, provider, input)
	case "browser.interact.hover":
		return execHover(ctx, provider, input)
	case "browser.interact.scroll":
		return execScroll(ctx, provider, input)
	case "browser.resource.download":
		return execDownload(ctx, provider, input)
	case "browser.resource.upload":
		return execUpload(ctx, provider, input)
	case "browser.resource.screenshot":
		return execScreenshot(ctx, provider, input)
	case "browser.devtools.console_messages":
		return execDevTools(ctx, provider, "console_messages", input)
	case "browser.devtools.evaluate":
		return execDevTools(ctx, provider, "evaluate", input)
	case "browser.devtools.network_requests":
		return execDevTools(ctx, provider, "network_requests", input)
	case "browser.devtools.handle_dialog":
		return execDevTools(ctx, provider, "handle_dialog", input)
	case "browser.devtools.resize":
		return execDevTools(ctx, provider, "resize", input)
	case "browser.devtools.run_code":
		return execDevTools(ctx, provider, "run_code", input)
	case "browser.devtools.wait_for":
		return execDevTools(ctx, provider, "wait_for", input)
	case "browser.devtools.cookies":
		return execDevTools(ctx, provider, "cookies", input)
	case "browser.devtools.drag":
		return execDevTools(ctx, provider, "drag", input)
	case "browser.devtools.press_key":
		return execDevTools(ctx, provider, "press_key", input)
	}
	return nil, &browser.BrowserError{
		Code:    browser.ErrCodeUnsupportedAction,
		Message: fmt.Sprintf("unsupported browser handler: %s", handlerName),
	}
}

type sessionCloseInput struct {
	SessionID string `json:"sessionId"`
}

type sessionGetInput struct {
	SessionID string `json:"sessionId"`
}

type tabInput struct {
	SessionID string `json:"sessionId"`
	TabID     string `json:"tabId,omitempty"`
}

type navigateInput struct {
	SessionID string `json:"sessionId"`
	TabID     string `json:"tabId"`
	URL       string `json:"url"`
	WaitUntil string `json:"waitUntil,omitempty"`
	TimeoutMS int    `json:"timeoutMs,omitempty"`
	Referer   string `json:"referer,omitempty"`
}

type domSnapshotInput struct {
	SessionID string `json:"sessionId"`
	TabID     string `json:"tabId"`
	MaxDepth  int    `json:"maxDepth,omitempty"`
}

type findElementInput struct {
	SessionID string `json:"sessionId"`
	TabID     string `json:"tabId"`
	Selector  string `json:"selector"`
}

type scrollToElementInput struct {
	SessionID string                    `json:"sessionId"`
	TabID     string                    `json:"tabId"`
	Element   browser.BrowserElementRef `json:"element"`
}

type interactInput struct {
	SessionID string                    `json:"sessionId"`
	TabID     string                    `json:"tabId"`
	Element   browser.BrowserElementRef `json:"element"`
	InputText string                    `json:"inputText,omitempty"`
	Value     string                    `json:"value,omitempty"`
	Direction string                    `json:"direction,omitempty"`
}

type downloadInput struct {
	SessionID     string `json:"sessionId"`
	TabID         string `json:"tabId"`
	ResourceURI   string `json:"resourceURI"`
	Filename      string `json:"filename,omitempty"`
	WaitTimeoutMS int64  `json:"waitTimeoutMs,omitempty"`
	Overwrite     bool   `json:"overwrite,omitempty"`
}

type uploadInput struct {
	SessionID   string `json:"sessionId"`
	TabID       string `json:"tabId"`
	ResourceURI string `json:"resourceURI"`
	TargetInput string `json:"targetInput,omitempty"`
}

type screenshotInput struct {
	SessionID string `json:"sessionId"`
	TabID     string `json:"tabId"`
	Format    string `json:"format,omitempty"`
	Quality   int    `json:"quality,omitempty"`
	FullPage  bool   `json:"fullPage,omitempty"`
}

func execDevTools(ctx context.Context, provider browser.BrowserProvider, operation string, input json.RawMessage) (json.RawMessage, error) {
	var ids struct {
		SessionID string `json:"sessionId"`
		TabID     string `json:"tabId"`
	}
	if err := json.Unmarshal(input, &ids); err != nil {
		return nil, &browser.BrowserError{Code: browser.ErrCodeInvalidRequest, Message: "invalid input: " + err.Error(), Cause: err}
	}
	if ids.SessionID == "" || ids.TabID == "" {
		return nil, &browser.BrowserError{Code: browser.ErrCodeInvalidRequest, Message: "sessionId and tabId are required"}
	}
	devToolsProvider, ok := provider.(browser.BrowserDevToolsProvider)
	if !ok || devToolsProvider.DevTools() == nil {
		return nil, &browser.BrowserError{Code: browser.ErrCodeUnsupportedAction, Message: "browser provider does not expose DevTools support"}
	}
	return devToolsProvider.DevTools().Execute(ctx, operation, browser.BrowserSessionID(ids.SessionID), browser.BrowserTabID(ids.TabID), input)
}

func execSessionCreate(ctx context.Context, provider browser.BrowserProvider, _ json.RawMessage) (json.RawMessage, error) {
	info, err := provider.Sessions().CreateSession(ctx)
	if err != nil {
		return nil, err
	}
	return json.Marshal(info)
}

func execSessionClose(ctx context.Context, provider browser.BrowserProvider, input json.RawMessage) (json.RawMessage, error) {
	var in sessionCloseInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, &browser.BrowserError{Code: browser.ErrCodeInvalidRequest, Message: "invalid input: " + err.Error(), Cause: err}
	}
	if err := provider.Sessions().CloseSession(ctx, browser.BrowserSessionID(in.SessionID)); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]bool{"ok": true})
}

func execSessionGet(ctx context.Context, provider browser.BrowserProvider, input json.RawMessage) (json.RawMessage, error) {
	var in sessionGetInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, &browser.BrowserError{Code: browser.ErrCodeInvalidRequest, Message: "invalid input: " + err.Error(), Cause: err}
	}
	info, err := provider.Sessions().GetSession(ctx, browser.BrowserSessionID(in.SessionID))
	if err != nil {
		return nil, err
	}
	return json.Marshal(info)
}

func execSessionList(ctx context.Context, provider browser.BrowserProvider, _ json.RawMessage) (json.RawMessage, error) {
	list, err := provider.Sessions().ListSessions(ctx)
	if err != nil {
		return nil, err
	}
	return json.Marshal(list)
}

func execTabCreate(ctx context.Context, provider browser.BrowserProvider, input json.RawMessage) (json.RawMessage, error) {
	var in tabInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, &browser.BrowserError{Code: browser.ErrCodeInvalidRequest, Message: "invalid input: " + err.Error(), Cause: err}
	}
	info, err := provider.Tabs().CreateTab(ctx, browser.BrowserSessionID(in.SessionID))
	if err != nil {
		return nil, err
	}
	return json.Marshal(info)
}

func execTabClose(ctx context.Context, provider browser.BrowserProvider, input json.RawMessage) (json.RawMessage, error) {
	var in tabInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, &browser.BrowserError{Code: browser.ErrCodeInvalidRequest, Message: "invalid input: " + err.Error(), Cause: err}
	}
	if err := provider.Tabs().CloseTab(ctx, browser.BrowserSessionID(in.SessionID), browser.BrowserTabID(in.TabID)); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]bool{"ok": true})
}

func execTabGet(ctx context.Context, provider browser.BrowserProvider, input json.RawMessage) (json.RawMessage, error) {
	var in tabInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, &browser.BrowserError{Code: browser.ErrCodeInvalidRequest, Message: "invalid input: " + err.Error(), Cause: err}
	}
	info, err := provider.Tabs().GetTab(ctx, browser.BrowserSessionID(in.SessionID), browser.BrowserTabID(in.TabID))
	if err != nil {
		return nil, err
	}
	return json.Marshal(info)
}

func execTabList(ctx context.Context, provider browser.BrowserProvider, input json.RawMessage) (json.RawMessage, error) {
	var in tabInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, &browser.BrowserError{Code: browser.ErrCodeInvalidRequest, Message: "invalid input: " + err.Error(), Cause: err}
	}
	list, err := provider.Tabs().ListTabs(ctx, browser.BrowserSessionID(in.SessionID))
	if err != nil {
		return nil, err
	}
	return json.Marshal(list)
}

func execTabActivate(ctx context.Context, provider browser.BrowserProvider, input json.RawMessage) (json.RawMessage, error) {
	var in tabInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, &browser.BrowserError{Code: browser.ErrCodeInvalidRequest, Message: "invalid input: " + err.Error(), Cause: err}
	}
	if err := provider.Tabs().ActivateTab(ctx, browser.BrowserSessionID(in.SessionID), browser.BrowserTabID(in.TabID)); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]bool{"ok": true})
}

func execNavigate(ctx context.Context, provider browser.BrowserProvider, input json.RawMessage) (json.RawMessage, error) {
	var in navigateInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, &browser.BrowserError{Code: browser.ErrCodeInvalidRequest, Message: "invalid input: " + err.Error(), Cause: err}
	}
	req := browser.NavigateRequest{
		URL:       in.URL,
		WaitUntil: in.WaitUntil,
		TimeoutMS: in.TimeoutMS,
		Referer:   in.Referer,
	}
	result, err := provider.Navigate().Navigate(ctx, browser.BrowserSessionID(in.SessionID), browser.BrowserTabID(in.TabID), req)
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

func execReload(ctx context.Context, provider browser.BrowserProvider, input json.RawMessage) (json.RawMessage, error) {
	var in navigateInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, &browser.BrowserError{Code: browser.ErrCodeInvalidRequest, Message: "invalid input: " + err.Error(), Cause: err}
	}
	req := browser.NavigateRequest{
		WaitUntil: in.WaitUntil,
		TimeoutMS: in.TimeoutMS,
		Referer:   in.Referer,
	}
	result, err := provider.Navigate().Reload(ctx, browser.BrowserSessionID(in.SessionID), browser.BrowserTabID(in.TabID), req)
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

func execGoBack(ctx context.Context, provider browser.BrowserProvider, input json.RawMessage) (json.RawMessage, error) {
	var in tabInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, &browser.BrowserError{Code: browser.ErrCodeInvalidRequest, Message: "invalid input: " + err.Error(), Cause: err}
	}
	result, err := provider.Navigate().GoBack(ctx, browser.BrowserSessionID(in.SessionID), browser.BrowserTabID(in.TabID))
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

func execGoForward(ctx context.Context, provider browser.BrowserProvider, input json.RawMessage) (json.RawMessage, error) {
	var in tabInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, &browser.BrowserError{Code: browser.ErrCodeInvalidRequest, Message: "invalid input: " + err.Error(), Cause: err}
	}
	result, err := provider.Navigate().GoForward(ctx, browser.BrowserSessionID(in.SessionID), browser.BrowserTabID(in.TabID))
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

func execStop(ctx context.Context, provider browser.BrowserProvider, input json.RawMessage) (json.RawMessage, error) {
	var in tabInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, &browser.BrowserError{Code: browser.ErrCodeInvalidRequest, Message: "invalid input: " + err.Error(), Cause: err}
	}
	if err := provider.Navigate().Stop(ctx, browser.BrowserSessionID(in.SessionID), browser.BrowserTabID(in.TabID)); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]bool{"ok": true})
}

func execDOMSnapshot(ctx context.Context, provider browser.BrowserProvider, input json.RawMessage) (json.RawMessage, error) {
	var in domSnapshotInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, &browser.BrowserError{Code: browser.ErrCodeInvalidRequest, Message: "invalid input: " + err.Error(), Cause: err}
	}
	snapshot, err := provider.Observe().GetDOMSnapshot(ctx, browser.BrowserSessionID(in.SessionID), browser.BrowserTabID(in.TabID), in.MaxDepth)
	if err != nil {
		return nil, err
	}
	return json.Marshal(snapshot)
}

func execFindElement(ctx context.Context, provider browser.BrowserProvider, input json.RawMessage) (json.RawMessage, error) {
	var in findElementInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, &browser.BrowserError{Code: browser.ErrCodeInvalidRequest, Message: "invalid input: " + err.Error(), Cause: err}
	}
	ref, err := provider.Observe().FindElement(ctx, browser.BrowserSessionID(in.SessionID), browser.BrowserTabID(in.TabID), in.Selector)
	if err != nil {
		return nil, err
	}
	return json.Marshal(ref)
}

func execScrollToElement(ctx context.Context, provider browser.BrowserProvider, input json.RawMessage) (json.RawMessage, error) {
	var in scrollToElementInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, &browser.BrowserError{Code: browser.ErrCodeInvalidRequest, Message: "invalid input: " + err.Error(), Cause: err}
	}
	if err := provider.Observe().ScrollToElement(ctx, browser.BrowserSessionID(in.SessionID), browser.BrowserTabID(in.TabID), in.Element); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]bool{"ok": true})
}

func execClick(ctx context.Context, provider browser.BrowserProvider, input json.RawMessage) (json.RawMessage, error) {
	var in interactInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, &browser.BrowserError{Code: browser.ErrCodeInvalidRequest, Message: "invalid input: " + err.Error(), Cause: err}
	}
	result, err := provider.Interact().Click(ctx, browser.BrowserSessionID(in.SessionID), browser.BrowserTabID(in.TabID), in.Element)
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

func execInput(ctx context.Context, provider browser.BrowserProvider, input json.RawMessage) (json.RawMessage, error) {
	var in interactInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, &browser.BrowserError{Code: browser.ErrCodeInvalidRequest, Message: "invalid input: " + err.Error(), Cause: err}
	}
	result, err := provider.Interact().Input(ctx, browser.BrowserSessionID(in.SessionID), browser.BrowserTabID(in.TabID), in.Element, in.InputText)
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

func execSelect(ctx context.Context, provider browser.BrowserProvider, input json.RawMessage) (json.RawMessage, error) {
	var in interactInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, &browser.BrowserError{Code: browser.ErrCodeInvalidRequest, Message: "invalid input: " + err.Error(), Cause: err}
	}
	result, err := provider.Interact().Select(ctx, browser.BrowserSessionID(in.SessionID), browser.BrowserTabID(in.TabID), in.Element, in.Value)
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

func execHover(ctx context.Context, provider browser.BrowserProvider, input json.RawMessage) (json.RawMessage, error) {
	var in interactInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, &browser.BrowserError{Code: browser.ErrCodeInvalidRequest, Message: "invalid input: " + err.Error(), Cause: err}
	}
	result, err := provider.Interact().Hover(ctx, browser.BrowserSessionID(in.SessionID), browser.BrowserTabID(in.TabID), in.Element)
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

func execScroll(ctx context.Context, provider browser.BrowserProvider, input json.RawMessage) (json.RawMessage, error) {
	var in interactInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, &browser.BrowserError{Code: browser.ErrCodeInvalidRequest, Message: "invalid input: " + err.Error(), Cause: err}
	}
	result, err := provider.Interact().Scroll(ctx, browser.BrowserSessionID(in.SessionID), browser.BrowserTabID(in.TabID), in.Direction)
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

func execDownload(ctx context.Context, provider browser.BrowserProvider, input json.RawMessage) (json.RawMessage, error) {
	var in downloadInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, &browser.BrowserError{Code: browser.ErrCodeInvalidRequest, Message: "invalid input: " + err.Error(), Cause: err}
	}
	req := browser.BrowserDownloadRequest{
		SessionID:     browser.BrowserSessionID(in.SessionID),
		TabID:         browser.BrowserTabID(in.TabID),
		ResourceURI:   in.ResourceURI,
		Filename:      in.Filename,
		WaitTimeoutMS: in.WaitTimeoutMS,
		Overwrite:     in.Overwrite,
	}
	result, err := provider.Resources().Download(ctx, req)
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

func execUpload(ctx context.Context, provider browser.BrowserProvider, input json.RawMessage) (json.RawMessage, error) {
	var in uploadInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, &browser.BrowserError{Code: browser.ErrCodeInvalidRequest, Message: "invalid input: " + err.Error(), Cause: err}
	}
	req := browser.BrowserUploadRequest{
		SessionID:   browser.BrowserSessionID(in.SessionID),
		TabID:       browser.BrowserTabID(in.TabID),
		ResourceURI: in.ResourceURI,
		TargetInput: in.TargetInput,
	}
	result, err := provider.Resources().Upload(ctx, req)
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

func execScreenshot(ctx context.Context, provider browser.BrowserProvider, input json.RawMessage) (json.RawMessage, error) {
	var in screenshotInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, &browser.BrowserError{Code: browser.ErrCodeInvalidRequest, Message: "invalid input: " + err.Error(), Cause: err}
	}
	req := browser.BrowserScreenshotRequest{
		SessionID: browser.BrowserSessionID(in.SessionID),
		TabID:     browser.BrowserTabID(in.TabID),
		Format:    in.Format,
		Quality:   in.Quality,
		FullPage:  in.FullPage,
	}
	result, err := provider.Resources().Screenshot(ctx, req)
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}
