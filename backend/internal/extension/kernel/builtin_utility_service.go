package kernel

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/u-ai/backend/internal/browser"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/workspace"
)

type BuiltinUtilityService struct {
	workspace    *workspace.Service
	precise      workspace.PreciseEditingService
	browser      browser.BrowserProvider
	android      capability.AndroidProvider
	androidLinux interface{}

	sshMu       sync.RWMutex
	sshSessions map[string]RemoteTerminalSession
}

type RemoteTerminalSession struct {
	Host          string
	Port          int
	User          string
	Password      string
	PrivateKey    string
	HostKey       string
	HostKeyPolicy string
	AgentAuth     bool
	TimeoutMS     int
}

type BuiltinUtilityDeps struct {
	Workspace    *workspace.Service
	Browser      browser.BrowserProvider
	Android      capability.AndroidProvider
	AndroidLinux interface{}
}

func NewBuiltinUtilityService(deps BuiltinUtilityDeps) *BuiltinUtilityService {
	var precise workspace.PreciseEditingService
	if deps.Workspace != nil {
		precise = workspace.NewDefaultPreciseEditingService(deps.Workspace)
	}
	return &BuiltinUtilityService{
		workspace:    deps.Workspace,
		precise:      precise,
		browser:      deps.Browser,
		android:      deps.Android,
		androidLinux: deps.AndroidLinux,
		sshSessions:  make(map[string]RemoteTerminalSession),
	}
}

func (s *BuiltinUtilityService) Supports(name string) bool {
	switch name {
	case "read_file_part", "apply_file":
		return s.workspace != nil
	case "visit_web", "browser_close_all", "browser_fill_form":
		return s.browser != nil
	case "bluetooth_send_and_read", "bluetooth_ble_write_and_read_characteristic", "press_key", "combined_operation":
		return s.android != nil
	case "execute_terminal", "execute_in_terminal_session_streaming", "get_terminal_session_screen", "ssh_login", "ssh_exit":
		return s.androidLinux != nil && amitiaLinuxAvailable(s.androidLinux)
	default:
		return false
	}
}

func (s *BuiltinUtilityService) CanHandle(name string) bool { return s.Supports(name) }

func (s *BuiltinUtilityService) Dispatch(ctx context.Context, name string, input json.RawMessage, invocation capability.ToolInvocationContext) (json.RawMessage, error) {
	switch name {
	case "read_file_part":
		return s.readFilePart(ctx, input)
	case "apply_file":
		return s.applyFile(ctx, input)
	case "visit_web":
		return s.visitWeb(ctx, input)
	case "browser_close_all":
		return s.browserCloseAll(ctx)
	case "browser_fill_form":
		return s.browserFillForm(ctx, input)
	case "bluetooth_send_and_read":
		return s.bluetoothSendAndRead(ctx, input, invocation)
	case "bluetooth_ble_write_and_read_characteristic":
		return s.bleWriteAndRead(ctx, input, invocation)
	case "press_key":
		return s.pressKey(ctx, input, invocation)
	case "combined_operation":
		return s.combinedOperation(ctx, input, invocation)
	case "execute_terminal":
		return s.executeTerminal(ctx, input, invocation)
	case "execute_in_terminal_session_streaming":
		return s.executeInTerminalSession(ctx, input, invocation)
	case "get_terminal_session_screen":
		return s.getTerminalSessionScreen(ctx, input, invocation)
	case "ssh_login":
		return s.sshLogin(ctx, input, invocation)
	case "ssh_exit":
		return s.sshExit(invocation)
	default:
		return nil, fmt.Errorf("builtin utility handler %s not found", name)
	}
}

func unmarshalObject(input json.RawMessage) (map[string]any, error) {
	var obj map[string]any
	if len(input) == 0 {
		return map[string]any{}, nil
	}
	if err := json.Unmarshal(input, &obj); err != nil {
		return nil, err
	}
	if obj == nil {
		obj = map[string]any{}
	}
	return obj, nil
}

func stringValue(obj map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := obj[key].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func intValue(obj map[string]any, fallback int, keys ...string) int {
	for _, key := range keys {
		switch value := obj[key].(type) {
		case float64:
			return int(value)
		case int:
			return value
		case json.Number:
			if parsed, err := value.Int64(); err == nil {
				return int(parsed)
			}
		}
	}
	return fallback
}

func boolValue(obj map[string]any, fallback bool, keys ...string) bool {
	for _, key := range keys {
		if value, ok := obj[key].(bool); ok {
			return value
		}
	}
	return fallback
}

func jsonObject(value any) map[string]any {
	if obj, ok := value.(map[string]any); ok && obj != nil {
		return obj
	}
	return map[string]any{}
}

func marshalResult(value any) (json.RawMessage, error) {
	data, err := json.Marshal(value)
	return json.RawMessage(data), err
}

func normalizeJSONLikeMap(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return input
	}
	var out map[string]any
	if err := json.Unmarshal(encoded, &out); err != nil || out == nil {
		return input
	}
	return out
}

func resolveWorkspaceURI(obj map[string]any) (string, error) {
	if uri := stringValue(obj, "uri"); uri != "" {
		return uri, nil
	}
	path := stringValue(obj, "path", "filePath")
	workspaceID := stringValue(obj, "workspaceId")
	if workspaceID == "" {
		if strings.HasPrefix(path, "workspace://") {
			return path, nil
		}
		return "", errors.New("uri or workspaceId + path/filePath is required")
	}
	path = strings.TrimPrefix(strings.TrimSpace(path), "/")
	base := workspace.MountURI(workspace.WorkspaceID(workspaceID))
	if path == "" {
		return base, nil
	}
	return strings.TrimSuffix(base, "/") + "/" + path, nil
}

func (s *BuiltinUtilityService) readFilePart(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	obj, err := unmarshalObject(input)
	if err != nil {
		return nil, fmt.Errorf("read_file_part input: %w", err)
	}
	uri, err := resolveWorkspaceURI(obj)
	if err != nil {
		return nil, err
	}
	start := intValue(obj, 1, "line_start", "startLine")
	end := intValue(obj, 0, "line_end", "endLine")
	maxLines := intValue(obj, 400, "maxLines")
	if start < 1 {
		start = 1
	}
	if maxLines < 1 {
		maxLines = 400
	}
	if end <= 0 {
		end = start + maxLines - 1
	}
	if end < start {
		return nil, fmt.Errorf("line end %d must be >= line start %d", end, start)
	}
	if end-start+1 > 20000 {
		end = start + 19999
	}

	const chunkSize int64 = 256 * 1024
	var offset int64
	lineNo := 1
	var pending []byte
	var out strings.Builder
	truncated := false
	finished := false

	consumeLine := func(line []byte, hasNewline bool) {
		if finished {
			return
		}
		if lineNo >= start && lineNo <= end {
			out.Write(line)
			if hasNewline {
				out.WriteByte('\n')
			}
		}
		if lineNo >= end {
			finished = true
		}
		lineNo++
	}

	for !finished {
		result, readErr := s.workspace.Read(ctx, uri, workspace.ReadOptions{Offset: offset, MaxBytes: chunkSize, Encoding: "utf-8"})
		if readErr != nil {
			return nil, fmt.Errorf("read_file_part %s: %w", uri, readErr)
		}
		if len(result.Content) == 0 {
			break
		}
		offset += int64(len(result.Content))
		buf := append(pending, result.Content...)
		pending = pending[:0]
		for {
			idx := bytes.IndexByte(buf, '\n')
			if idx < 0 {
				pending = append(pending, buf...)
				break
			}
			line := buf[:idx]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			consumeLine(line, true)
			buf = buf[idx+1:]
			if finished {
				if len(buf) > 0 {
					truncated = true
				}
				break
			}
		}
		if int64(len(result.Content)) < chunkSize {
			break
		}
	}
	if !finished && len(pending) > 0 {
		if pending[len(pending)-1] == '\r' {
			pending = pending[:len(pending)-1]
		}
		consumeLine(pending, false)
	}
	if finished && !truncated {
		probe, probeErr := s.workspace.Read(ctx, uri, workspace.ReadOptions{Offset: offset, MaxBytes: 1})
		if probeErr != nil {
			return nil, fmt.Errorf("read_file_part truncation probe %s: %w", uri, probeErr)
		}
		truncated = len(probe.Content) > 0
	}
	return marshalResult(map[string]any{
		"uri":       uri,
		"content":   out.String(),
		"startLine": start,
		"endLine":   minInt(end, maxInt(start-1, lineNo-1)),
		"truncated": truncated,
	})
}

func (s *BuiltinUtilityService) applyFile(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	obj, err := unmarshalObject(input)
	if err != nil {
		return nil, fmt.Errorf("apply_file input: %w", err)
	}
	uri, err := resolveWorkspaceURI(obj)
	if err != nil {
		return nil, err
	}
	op := strings.ToLower(strings.TrimSpace(stringValue(obj, "operation")))
	oldText := stringValuePreserveEmpty(obj, "oldText", "old_content")
	newText := stringValuePreserveEmpty(obj, "newText", "new_content")
	content := stringValuePreserveEmpty(obj, "content")
	if op == "" {
		switch {
		case hasAnyKey(obj, "patch"):
			op = "patch"
		case hasAnyKey(obj, "oldText", "old_content"):
			op = "replace"
		case hasAnyKey(obj, "content", "newText", "new_content"):
			op = "write"
		default:
			return nil, errors.New("operation is required when no replacement/content fields are supplied")
		}
	}

	switch op {
	case "delete":
		if err := s.workspace.Delete(ctx, uri, workspace.DeleteOptions{Recursive: boolValue(obj, false, "recursive")}); err != nil {
			return nil, err
		}
		return marshalResult(map[string]any{"applied": true, "operation": "delete", "uri": uri})
	case "create", "write":
		if !hasAnyKey(obj, "content") {
			content = newText
		}
		overwrite := op == "write"
		entry, err := s.workspace.Write(ctx, uri, strings.NewReader(content), workspace.WriteOptions{Overwrite: overwrite, Atomic: true})
		if err != nil {
			return nil, err
		}
		return marshalResult(map[string]any{"applied": true, "operation": op, "uri": uri, "entry": entry})
	case "patch":
		if s.precise == nil {
			return nil, errors.New("workspace precise editing service is unavailable")
		}
		patchText := stringValuePreserveEmpty(obj, "patch")
		if strings.TrimSpace(patchText) == "" {
			return nil, errors.New("patch is required for patch operation")
		}
		workspaceID, filePath, identityErr := resolveWorkspaceIdentity(obj, uri)
		if identityErr != nil {
			return nil, identityErr
		}
		result, patchErr := s.precise.Patch(ctx, workspace.PatchRequest{
			WorkspaceID: workspaceID,
			FilePath:    filePath,
			BaseSHA256:  stringValuePreserveEmpty(obj, "baseSha256"),
			Patch:       patchText,
		})
		if patchErr != nil {
			return nil, patchErr
		}
		return marshalResult(map[string]any{"applied": result.Applied, "operation": "patch", "uri": uri, "result": result})
	case "replace":
		if !hasAnyKey(obj, "oldText", "old_content") {
			return nil, errors.New("oldText/old_content is required for replace")
		}
		result, err := s.workspace.Read(ctx, uri, workspace.ReadOptions{})
		if err != nil {
			return nil, err
		}
		current := string(result.Content)
		expected := intValue(obj, 1, "expectedOccurrences")
		if oldText == "" {
			return nil, errors.New("oldText must not be empty for replacement")
		}
		count := strings.Count(current, oldText)
		if expected >= 0 && count != expected {
			return nil, fmt.Errorf("replacement occurrence mismatch: expected %d, found %d", expected, count)
		}
		updated := strings.ReplaceAll(current, oldText, newText)
		entry, err := s.workspace.Write(ctx, uri, strings.NewReader(updated), workspace.WriteOptions{Overwrite: true, Atomic: true})
		if err != nil {
			return nil, err
		}
		return marshalResult(map[string]any{"applied": true, "operation": "replace", "uri": uri, "occurrences": count, "entry": entry})
	default:
		return nil, fmt.Errorf("unsupported apply_file operation %q", op)
	}
}

func resolveWorkspaceIdentity(obj map[string]any, uri string) (string, string, error) {
	workspaceID := strings.TrimSpace(stringValue(obj, "workspaceId"))
	filePath := strings.TrimPrefix(strings.TrimSpace(stringValue(obj, "path", "filePath")), "/")
	if workspaceID != "" && filePath != "" {
		return workspaceID, filePath, nil
	}

	const mountedPrefix = "amitia://workspace/@"
	if strings.HasPrefix(uri, mountedPrefix) {
		remainder := strings.TrimPrefix(uri, mountedPrefix)
		parts := strings.SplitN(remainder, "/", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) != "" && strings.TrimSpace(parts[1]) != "" {
			return strings.TrimSpace(parts[0]), strings.TrimPrefix(parts[1], "/"), nil
		}
	}
	return "", "", errors.New("patch operation requires workspaceId + path/filePath or an amitia://workspace/@<id>/<path> URI")
}

func stringValuePreserveEmpty(obj map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := obj[key].(string); ok {
			return value
		}
	}
	return ""
}

func hasAnyKey(obj map[string]any, keys ...string) bool {
	for _, key := range keys {
		if _, ok := obj[key]; ok {
			return true
		}
	}
	return false
}

func (s *BuiltinUtilityService) visitWeb(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	obj, err := unmarshalObject(input)
	if err != nil {
		return nil, err
	}
	url := stringValue(obj, "url")
	if url == "" {
		return nil, errors.New("url is required")
	}
	session, browserErr := s.browser.Sessions().CreateSession(ctx)
	if browserErr != nil {
		return nil, browserError("create browser session", browserErr)
	}
	closeOnError := true
	defer func() {
		if closeOnError || boolValue(obj, false, "closeAfter") {
			_ = s.browser.Sessions().CloseSession(context.Background(), session.SessionID)
		}
	}()
	tab, browserErr := s.browser.Tabs().CreateTab(ctx, session.SessionID)
	if browserErr != nil {
		return nil, browserError("create browser tab", browserErr)
	}
	nav, browserErr := s.browser.Navigate().Navigate(ctx, session.SessionID, tab.TabID, browser.NavigateRequest{
		URL:       url,
		WaitUntil: stringValue(obj, "waitUntil"),
		TimeoutMS: intValue(obj, 30000, "timeoutMs"),
	})
	if browserErr != nil {
		return nil, browserError("navigate browser", browserErr)
	}
	depth := intValue(obj, 24, "maxDepth")
	snapshot, browserErr := s.browser.Observe().GetDOMSnapshot(ctx, session.SessionID, tab.TabID, depth)
	if browserErr != nil {
		return nil, browserError("read browser DOM", browserErr)
	}
	closeOnError = false
	return marshalResult(map[string]any{
		"sessionId":  session.SessionID,
		"tabId":      tab.TabID,
		"navigation": nav,
		"url":        snapshot.URL,
		"title":      snapshot.Title,
		"content":    snapshot.Content,
		"truncated":  snapshot.Truncated,
		"nodeCount":  snapshot.NodeCount,
	})
}

func (s *BuiltinUtilityService) browserCloseAll(ctx context.Context) (json.RawMessage, error) {
	sessions, browserErr := s.browser.Sessions().ListSessions(ctx)
	if browserErr != nil {
		return nil, browserError("list browser sessions", browserErr)
	}
	results := make([]map[string]any, 0, len(sessions))
	failed := 0
	for _, session := range sessions {
		entry := map[string]any{"sessionId": session.SessionID}
		if closeErr := s.browser.Sessions().CloseSession(ctx, session.SessionID); closeErr != nil {
			entry["closed"] = false
			entry["error"] = closeErr.Message
			failed++
		} else {
			entry["closed"] = true
		}
		results = append(results, entry)
	}
	return marshalResult(map[string]any{"closed": len(sessions) - failed, "failed": failed, "sessions": results})
}

func (s *BuiltinUtilityService) browserFillForm(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	obj, err := unmarshalObject(input)
	if err != nil {
		return nil, err
	}
	sessionID := browser.BrowserSessionID(stringValue(obj, "sessionId"))
	tabID := browser.BrowserTabID(stringValue(obj, "tabId"))
	fields, ok := obj["fields"].([]any)
	if !ok || len(fields) == 0 {
		return nil, errors.New("fields is required")
	}
	results := make([]map[string]any, 0, len(fields))
	for index, raw := range fields {
		field := jsonObject(raw)
		var element browser.BrowserElementRef
		if rawElement, ok := field["element"]; ok {
			encoded, _ := json.Marshal(rawElement)
			if err := json.Unmarshal(encoded, &element); err != nil {
				return nil, fmt.Errorf("field %d element: %w", index, err)
			}
		}
		if selector := stringValue(field, "selector"); selector != "" {
			found, findErr := s.browser.Observe().FindElement(ctx, sessionID, tabID, selector)
			if findErr != nil {
				return nil, browserError(fmt.Sprintf("find field %d", index), findErr)
			}
			element = *found
		}
		if element.SessionID == "" {
			element.SessionID = sessionID
		}
		if element.TabID == "" {
			element.TabID = tabID
		}
		if element.StableID == "" && element.Selector == "" {
			return nil, fmt.Errorf("field %d requires selector or element", index)
		}
		action := strings.ToLower(stringValue(field, "action", "type"))
		if action == "" {
			action = "input"
		}
		value := stringValuePreserveEmpty(field, "value", "text")
		var interaction *browser.BrowserInteractionResult
		var interactErr *browser.BrowserError
		switch action {
		case "select":
			interaction, interactErr = s.browser.Interact().Select(ctx, sessionID, tabID, element, value)
		case "input":
			interaction, interactErr = s.browser.Interact().Input(ctx, sessionID, tabID, element, value)
		case "click", "checkbox", "radio":
			interaction, interactErr = s.browser.Interact().Click(ctx, sessionID, tabID, element)
		default:
			return nil, fmt.Errorf("field %d has unsupported action %q", index, action)
		}
		if interactErr != nil {
			return nil, browserError(fmt.Sprintf("fill field %d", index), interactErr)
		}
		results = append(results, map[string]any{"index": index, "action": action, "result": interaction})
	}
	return marshalResult(map[string]any{"success": true, "count": len(results), "fields": results})
}

func browserError(prefix string, err *browser.BrowserError) error {
	if err == nil {
		return nil
	}
	if err.Message != "" {
		return fmt.Errorf("%s: %s", prefix, err.Message)
	}
	return fmt.Errorf("%s: %s", prefix, err.Code)
}

func (s *BuiltinUtilityService) androidCall(ctx context.Context, invocation capability.ToolInvocationContext, suffix, operation string, payload map[string]any) (map[string]any, error) {
	if s.android == nil {
		return nil, errors.New("android native provider unavailable")
	}
	requestID := invocation.InvocationID
	if requestID == "" {
		requestID = fmt.Sprintf("amitia-%d", time.Now().UnixNano())
	}
	payload = normalizeJSONLikeMap(payload)
	resp := s.android.Execute(ctx, capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       requestID + ":" + suffix,
		Operation:       operation,
		Payload:         payload,
	})
	if strings.EqualFold(resp.Status, "success") || strings.EqualFold(resp.Status, "ok") {
		if resp.Result == nil {
			return map[string]any{}, nil
		}
		return resp.Result, nil
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("%s failed [%s]: %s", operation, resp.Error.Code, resp.Error.Message)
	}
	return nil, fmt.Errorf("%s failed with status %q", operation, resp.Status)
}

func (s *BuiltinUtilityService) bluetoothSendAndRead(ctx context.Context, input json.RawMessage, invocation capability.ToolInvocationContext) (json.RawMessage, error) {
	obj, err := unmarshalObject(input)
	if err != nil {
		return nil, err
	}
	if stringValue(obj, "sessionId") == "" {
		return nil, errors.New("sessionId is required")
	}
	if !hasAnyKey(obj, "valueBase64", "valueText") {
		return nil, errors.New("valueBase64 or valueText is required")
	}
	writePayload := pickKeys(obj, "sessionId", "valueBase64", "valueText")
	writeResult, err := s.androidCall(ctx, invocation, "classic-write", "device.bluetooth_classic_write", writePayload)
	if err != nil {
		return nil, err
	}
	readPayload := pickKeys(obj, "sessionId", "maxBytes", "timeoutMs", "decodeUtf8")
	readResult, err := s.androidCall(ctx, invocation, "classic-read", "device.bluetooth_classic_read", readPayload)
	if err != nil {
		return nil, err
	}
	return marshalResult(map[string]any{"write": writeResult, "read": readResult})
}

func (s *BuiltinUtilityService) bleWriteAndRead(ctx context.Context, input json.RawMessage, invocation capability.ToolInvocationContext) (json.RawMessage, error) {
	obj, err := unmarshalObject(input)
	if err != nil {
		return nil, err
	}
	for _, required := range []string{"sessionId", "serviceUuid", "characteristicUuid"} {
		if stringValue(obj, required) == "" {
			return nil, fmt.Errorf("%s is required", required)
		}
	}
	if !hasAnyKey(obj, "valueBase64", "valueText") {
		return nil, errors.New("valueBase64 or valueText is required")
	}
	writePayload := pickKeys(obj, "sessionId", "serviceUuid", "characteristicUuid", "valueBase64", "valueText", "withoutResponse", "timeoutMs")
	writeResult, err := s.androidCall(ctx, invocation, "ble-write", "device.ble_write", writePayload)
	if err != nil {
		return nil, err
	}
	readPayload := pickKeys(obj, "sessionId", "serviceUuid", "characteristicUuid", "timeoutMs")
	readResult, err := s.androidCall(ctx, invocation, "ble-read", "device.ble_read", readPayload)
	if err != nil {
		return nil, err
	}
	return marshalResult(map[string]any{"write": writeResult, "read": readResult})
}

func pickKeys(obj map[string]any, keys ...string) map[string]any {
	out := make(map[string]any, len(keys))
	for _, key := range keys {
		if value, ok := obj[key]; ok {
			out[key] = value
		}
	}
	return out
}

func (s *BuiltinUtilityService) pressKey(ctx context.Context, input json.RawMessage, invocation capability.ToolInvocationContext) (json.RawMessage, error) {
	obj, err := unmarshalObject(input)
	if err != nil {
		return nil, err
	}
	return s.pressKeyObject(ctx, obj, invocation)
}

func (s *BuiltinUtilityService) pressKeyObject(ctx context.Context, obj map[string]any, invocation capability.ToolInvocationContext) (json.RawMessage, error) {
	key := strings.TrimSpace(stringValue(obj, "key"))
	keyCode := intValue(obj, -1, "keyCode", "key_code")

	if action := normalizeGlobalAction(key); action != "" {
		result, err := s.androidCall(ctx, invocation, "press-global", "device.global_action", map[string]any{"action": action})
		if err == nil {
			return marshalResult(map[string]any{"success": true, "strategy": "global_action", "key": action, "result": result})
		}
	}
	if nativeKey := normalizeNativeKey(key); nativeKey != "" {
		result, err := s.androidCall(ctx, invocation, "press-native", "device.press_key", map[string]any{"key": nativeKey})
		if err == nil {
			return marshalResult(map[string]any{"success": true, "strategy": "native", "key": nativeKey, "result": result})
		}
	}
	if keyCode < 0 {
		keyCode = androidKeyCode(key)
	}
	if keyCode < 0 {
		return nil, fmt.Errorf("unknown Android key %q; provide keyCode for arbitrary key events", key)
	}

	args := []any{"keyevent", strconv.Itoa(keyCode)}
	if result, err := s.androidCall(ctx, invocation, "press-shizuku", "shizuku.execute", map[string]any{"executable": "input", "args": args}); err == nil {
		return marshalResult(map[string]any{"success": true, "strategy": "shizuku", "keyCode": keyCode, "result": result})
	}
	result, rootErr := s.androidCall(ctx, invocation, "press-root", "root.execute", map[string]any{"executable": "input", "args": args, "mode": "structured", "timeoutMs": 5000})
	if rootErr != nil {
		return nil, fmt.Errorf("arbitrary key injection unavailable through both Shizuku and Root: %w", rootErr)
	}
	return marshalResult(map[string]any{"success": true, "strategy": "root", "keyCode": keyCode, "result": result})
}

func normalizeGlobalAction(key string) string {
	normalized := strings.ToLower(strings.TrimSpace(key))
	normalized = strings.TrimPrefix(normalized, "keycode_")
	switch normalized {
	case "back", "home", "recents", "notifications", "quick_settings", "power_dialog", "lock_screen", "take_screenshot":
		return normalized
	default:
		return ""
	}
}

func normalizeNativeKey(key string) string {
	normalized := strings.ToLower(strings.TrimSpace(key))
	normalized = strings.TrimPrefix(normalized, "keycode_")
	switch normalized {
	case "volume_up", "volume_down", "mute", "media_play_pause", "media_next", "media_previous":
		return normalized
	default:
		return ""
	}
}

func androidKeyCode(key string) int {
	normalized := strings.ToUpper(strings.TrimSpace(key))
	normalized = strings.TrimPrefix(normalized, "KEYCODE_")
	codes := map[string]int{
		"HOME": 3, "BACK": 4, "CALL": 5, "ENDCALL": 6, "DPAD_UP": 19, "DPAD_DOWN": 20,
		"DPAD_LEFT": 21, "DPAD_RIGHT": 22, "DPAD_CENTER": 23, "VOLUME_UP": 24, "VOLUME_DOWN": 25,
		"POWER": 26, "CAMERA": 27, "CLEAR": 28, "TAB": 61, "SPACE": 62, "ENTER": 66,
		"DEL": 67, "BACKSPACE": 67, "ESCAPE": 111, "FORWARD_DEL": 112, "CTRL_LEFT": 113,
		"CTRL_RIGHT": 114, "CAPS_LOCK": 115, "SCROLL_LOCK": 116, "META_LEFT": 117, "META_RIGHT": 118,
		"PAGE_UP": 92, "PAGE_DOWN": 93, "MOVE_HOME": 122, "MOVE_END": 123, "INSERT": 124,
		"MENU": 82, "SEARCH": 84, "MEDIA_PLAY_PAUSE": 85, "MEDIA_STOP": 86, "MEDIA_NEXT": 87,
		"MEDIA_PREVIOUS": 88, "MEDIA_REWIND": 89, "MEDIA_FAST_FORWARD": 90, "MUTE": 91,
	}
	if code, ok := codes[normalized]; ok {
		return code
	}
	runes := []rune(normalized)
	if len(runes) == 1 {
		r := runes[0]
		if r >= 'A' && r <= 'Z' {
			return 29 + int(r-'A')
		}
		if r >= '0' && r <= '9' {
			return 7 + int(r-'0')
		}
		if unicode.IsSpace(r) {
			return 62
		}
	}
	return -1
}

func (s *BuiltinUtilityService) combinedOperation(ctx context.Context, input json.RawMessage, invocation capability.ToolInvocationContext) (json.RawMessage, error) {
	obj, err := unmarshalObject(input)
	if err != nil {
		return nil, err
	}
	operations, ok := obj["operations"].([]any)
	if !ok || len(operations) == 0 {
		return nil, errors.New("operations is required")
	}
	stopOnError := boolValue(obj, true, "stopOnError")
	results := make([]map[string]any, 0, len(operations))
	for index, raw := range operations {
		op := jsonObject(raw)
		action := strings.ToLower(stringValue(op, "action", "type", "operation"))
		entry := map[string]any{"index": index, "action": action}
		result, execErr := s.executeCombinedStep(ctx, action, op, invocation)
		if execErr != nil {
			entry["success"] = false
			entry["error"] = execErr.Error()
			results = append(results, entry)
			if stopOnError {
				return marshalResult(map[string]any{"success": false, "failedIndex": index, "results": results})
			}
			continue
		}
		entry["success"] = true
		if result != nil {
			var decoded any
			if json.Unmarshal(result, &decoded) == nil {
				entry["result"] = decoded
			}
		}
		results = append(results, entry)
	}
	return marshalResult(map[string]any{"success": true, "results": results})
}

func (s *BuiltinUtilityService) executeCombinedStep(ctx context.Context, action string, op map[string]any, invocation capability.ToolInvocationContext) (json.RawMessage, error) {
	switch action {
	case "wait", "sleep", "delay":
		ms := intValue(op, 0, "durationMs", "ms", "waitMs")
		if ms < 0 || ms > 30000 {
			return nil, errors.New("wait duration must be between 0 and 30000 ms")
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(ms) * time.Millisecond):
			return marshalResult(map[string]any{"waitedMs": ms})
		}
	case "press_key", "key", "press":
		return s.pressKeyObject(ctx, op, invocation)
	case "tap", "click", "click_element":
		payload := map[string]any{"allowCoordinateFallback": true, "allowShizukuFallback": true, "allowRootFallback": true, "allowAdbFallback": true}
		if target, ok := op["target"].(map[string]any); ok {
			payload["target"] = target
		} else {
			payload["target"] = pickKeys(op, "x", "y", "resourceId", "text", "description", "bounds")
		}
		return s.androidCallRaw(ctx, invocation, fmt.Sprintf("combined-%d", time.Now().UnixNano()), "interaction.click", payload)
	case "input", "input_text", "set_input_text", "type":
		payload := map[string]any{"text": stringValuePreserveEmpty(op, "text", "value"), "allowAdbFallback": true}
		if target, ok := op["target"].(map[string]any); ok {
			payload["target"] = target
		} else {
			target := pickKeys(op, "x", "y", "resourceId", "description", "bounds", "role", "index")
			if locatorText := stringValue(op, "targetText", "matchText"); locatorText != "" {
				target["text"] = locatorText
			}
			payload["target"] = target
		}
		return s.androidCallRaw(ctx, invocation, fmt.Sprintf("combined-%d", time.Now().UnixNano()), "interaction.input_text", payload)
	case "long_press", "long_click":
		payload := map[string]any{"allowCoordinateFallback": true, "allowShizukuFallback": true, "allowRootFallback": true, "allowAdbFallback": true}
		if target, ok := op["target"].(map[string]any); ok {
			payload["target"] = target
		} else {
			payload["target"] = pickKeys(op, "x", "y", "resourceId", "text", "description", "bounds")
		}
		if duration := intValue(op, 0, "durationMs"); duration > 0 {
			payload["durationMs"] = duration
		}
		return s.androidCallRaw(ctx, invocation, fmt.Sprintf("combined-%d", time.Now().UnixNano()), "interaction.long_click", payload)
	case "scroll":
		payload := pickKeys(op, "direction", "amount", "displayId", "x", "y", "resourceId", "text", "description", "bounds")
		if target, ok := op["target"].(map[string]any); ok {
			payload["target"] = target
		}
		return s.androidCallRaw(ctx, invocation, fmt.Sprintf("combined-%d", time.Now().UnixNano()), "interaction.scroll", payload)
	case "swipe":
		payload := pickKeys(op, "displayId", "startX", "startY", "endX", "endY", "durationMs")
		return s.androidCallRaw(ctx, invocation, fmt.Sprintf("combined-%d", time.Now().UnixNano()), "interaction.swipe", payload)
	default:
		allowed := map[string]bool{
			"interaction.click": true, "interaction.long_click": true,
			"interaction.input_text": true, "interaction.clear_text": true,
			"interaction.scroll": true, "interaction.swipe": true,
		}
		if allowed[action] {
			payload := jsonObject(op["payload"])
			if len(payload) == 0 {
				payload = make(map[string]any, len(op))
				for key, value := range op {
					if key != "action" && key != "type" && key != "operation" {
						payload[key] = value
					}
				}
			}
			return s.androidCallRaw(ctx, invocation, fmt.Sprintf("combined-%d", time.Now().UnixNano()), action, payload)
		}
		return nil, fmt.Errorf("unsupported combined UI operation %q", action)
	}
}

func (s *BuiltinUtilityService) androidCallRaw(ctx context.Context, invocation capability.ToolInvocationContext, suffix, operation string, payload map[string]any) (json.RawMessage, error) {
	result, err := s.androidCall(ctx, invocation, suffix, operation, payload)
	if err != nil {
		return nil, err
	}
	return marshalResult(result)
}

func (s *BuiltinUtilityService) executeTerminal(ctx context.Context, input json.RawMessage, invocation capability.ToolInvocationContext) (json.RawMessage, error) {
	obj, err := unmarshalObject(input)
	if err != nil {
		return nil, err
	}
	command := stringValue(obj, "command")
	if command == "" {
		return nil, errors.New("command is required")
	}
	environment := strings.ToLower(stringValue(obj, "environment"))
	if environment == "" {
		environment = "linux"
	}
	if environment == "linux" || environment == "ssh" {
		if sshConfig, ok := s.getSSH(invocation); ok {
			payload := sshConfig.payload()
			payload["command"] = command
			payload["workingDir"] = stringValue(obj, "workingDir", "cwd")
			if timeout := intValue(obj, sshConfig.TimeoutMS, "timeoutMs"); timeout > 0 {
				payload["timeoutMs"] = timeout
			}
			if maxOut := intValue(obj, 1024*1024, "maxOutputBytes"); maxOut > 0 {
				payload["maxOutputBytes"] = maxOut
			}
			return amitiaLinuxCall(ctx, s.androidLinux, invocation, "ssh.exec", payload)
		}
		if environment == "ssh" {
			return nil, errors.New("no active SSH session; call ssh_login first")
		}
	}
	payload := map[string]any{"command": command, "mode": "shell"}
	if cwd := stringValue(obj, "cwd", "workingDir"); cwd != "" {
		payload["workingDir"] = cwd
	}
	if timeout := intValue(obj, 30000, "timeoutMs"); timeout > 0 {
		payload["timeoutMs"] = timeout
	}
	if maxOut := intValue(obj, 1024*1024, "maxOutputBytes"); maxOut > 0 {
		payload["maxOutputBytes"] = maxOut
	}
	return amitiaLinuxCall(ctx, s.androidLinux, invocation, "shell.exec", payload)
}

func (s *BuiltinUtilityService) executeInTerminalSession(ctx context.Context, input json.RawMessage, invocation capability.ToolInvocationContext) (json.RawMessage, error) {
	obj, err := unmarshalObject(input)
	if err != nil {
		return nil, err
	}
	command := stringValue(obj, "command")
	if command == "" {
		return nil, errors.New("command is required")
	}
	sessionID := stringValue(obj, "sessionId")
	opened := false
	if sessionID == "" {
		openPayload := pickKeys(obj, "shell", "cwd")
		openResult, err := amitiaLinuxCall(ctx, s.androidLinux, invocation, "terminal.open", openPayload)
		if err != nil {
			return nil, err
		}
		var decoded map[string]any
		if err := json.Unmarshal(openResult, &decoded); err != nil {
			return nil, fmt.Errorf("decode terminal.open result: %w", err)
		}
		sessionID = stringValue(decoded, "sessionId")
		if sessionID == "" {
			return nil, errors.New("terminal.open did not return sessionId")
		}
		opened = true
	}

	after := intValue(obj, 0, "afterSequence")
	// Returns output produced by this command, not the entire historical terminal buffer.
	if !opened && !hasAnyKey(obj, "afterSequence") {
		baselineResult, baselineErr := amitiaLinuxCall(ctx, s.androidLinux, invocation, "terminal.read", map[string]any{
			"sessionId": sessionID, "afterSequence": 0, "maxBytes": 1, "waitMs": 0,
		})
		if baselineErr != nil {
			return nil, baselineErr
		}
		var baseline map[string]any
		if err := json.Unmarshal(baselineResult, &baseline); err != nil {
			return nil, fmt.Errorf("decode terminal.read baseline: %w", err)
		}
		after = intValue(baseline, 0, "nextSequence")
	}

	_, err = amitiaLinuxCall(ctx, s.androidLinux, invocation, "terminal.write", map[string]any{"sessionId": sessionID, "text": command + "\n"})
	if err != nil {
		return nil, err
	}

	waitMS := intValue(obj, 250, "waitMs")
	maxBytes := intValue(obj, 256*1024, "maxBytes")
	maxReads := intValue(obj, 8, "maxReads")
	allChunks := make([]any, 0)
	state := ""
	truncated := false
	for i := 0; i < maxReads; i++ {
		readResult, readErr := amitiaLinuxCall(ctx, s.androidLinux, invocation, "terminal.read", map[string]any{
			"sessionId": sessionID, "afterSequence": after, "maxBytes": maxBytes, "waitMs": waitMS,
		})
		if readErr != nil {
			return nil, readErr
		}
		var decoded map[string]any
		if err := json.Unmarshal(readResult, &decoded); err != nil {
			return nil, err
		}
		chunks, _ := decoded["chunks"].([]any)
		allChunks = append(allChunks, chunks...)
		next := intValue(decoded, after, "nextSequence")
		state = stringValue(decoded, "state")
		truncated = truncated || boolValue(decoded, false, "truncated")
		if next <= after || len(chunks) == 0 {
			break
		}
		after = next
		if state != "" && state != "running" && state != "ready" {
			break
		}
	}
	return marshalResult(map[string]any{
		"sessionId":     sessionID,
		"openedSession": opened,
		"chunks":        allChunks,
		"nextSequence":  after,
		"truncated":     truncated,
		"state":         state,
		"screen":        terminalChunkText(allChunks),
	})
}

func (s *BuiltinUtilityService) getTerminalSessionScreen(ctx context.Context, input json.RawMessage, invocation capability.ToolInvocationContext) (json.RawMessage, error) {
	obj, err := unmarshalObject(input)
	if err != nil {
		return nil, err
	}
	sessionID := stringValue(obj, "sessionId")
	if sessionID == "" {
		return nil, errors.New("sessionId is required")
	}
	readResult, err := amitiaLinuxCall(ctx, s.androidLinux, invocation, "terminal.read", map[string]any{
		"sessionId":     sessionID,
		"afterSequence": intValue(obj, 0, "afterSequence"),
		"maxBytes":      intValue(obj, 1024*1024, "maxBytes"),
		"waitMs":        intValue(obj, 0, "waitMs"),
	})
	if err != nil {
		return nil, err
	}
	statusResult, statusErr := amitiaLinuxCall(ctx, s.androidLinux, invocation, "terminal.status", map[string]any{"sessionId": sessionID})
	if statusErr != nil {
		return nil, statusErr
	}
	var read map[string]any
	var status map[string]any
	_ = json.Unmarshal(readResult, &read)
	_ = json.Unmarshal(statusResult, &status)
	chunks, _ := read["chunks"].([]any)
	rows := intValue(status, 24, "rows")
	cols := intValue(status, 80, "cols")
	if rows < 1 {
		rows = 24
	}
	if cols < 1 {
		cols = 80
	}
	rawOutput := terminalChunkText(chunks)
	truncated := boolValue(read, false, "truncated")
	return marshalResult(map[string]any{
		"sessionId":      sessionID,
		"screen":         renderTerminalScreen(rawOutput, rows, cols),
		"screenComplete": !truncated,
		"rows":           rows,
		"cols":           cols,
		"chunks":         chunks,
		"nextSequence":   read["nextSequence"],
		"truncated":      truncated,
		"status":         status,
	})
}

func terminalChunkText(chunks []any) string {
	var out strings.Builder
	for _, raw := range chunks {
		chunk := jsonObject(raw)
		if value, ok := chunk["text"].(string); ok {
			out.WriteString(value)
			continue
		}
		if value, ok := chunk["data"].(string); ok {
			if decoded, err := base64.StdEncoding.DecodeString(value); err == nil {
				out.Write(decoded)
			} else {
				out.WriteString(value)
			}
		}
	}
	return out.String()
}

func renderTerminalScreen(raw string, rows, cols int) string {
	if rows < 1 || cols < 1 {
		return raw
	}
	grid := make([][]rune, rows)
	for row := range grid {
		grid[row] = make([]rune, cols)
		for col := range grid[row] {
			grid[row][col] = ' '
		}
	}

	row, col := 0, 0
	savedRow, savedCol := 0, 0
	scroll := func() {
		copy(grid[0:], grid[1:])
		grid[rows-1] = make([]rune, cols)
		for i := range grid[rows-1] {
			grid[rows-1][i] = ' '
		}
		row = rows - 1
	}
	lineFeed := func() {
		row++
		if row >= rows {
			scroll()
		}
	}
	putRune := func(r rune) {
		if col >= cols {
			col = 0
			lineFeed()
		}
		grid[row][col] = r
		col++
	}
	clearRange := func(r, start, end int) {
		if r < 0 || r >= rows {
			return
		}
		if start < 0 {
			start = 0
		}
		if end >= cols {
			end = cols - 1
		}
		for c := start; c <= end && c < cols; c++ {
			grid[r][c] = ' '
		}
	}

	runes := []rune(raw)
	for i := 0; i < len(runes); {
		r := runes[i]
		switch r {
		case '\x1b':
			if i+1 >= len(runes) {
				i++
				continue
			}
			next := runes[i+1]
			if next == '[' {
				j := i + 2
				for j < len(runes) && !(runes[j] >= '@' && runes[j] <= '~') {
					j++
				}
				if j >= len(runes) {
					return terminalGridText(grid)
				}
				paramsText := string(runes[i+2 : j])
				params := parseCSIParams(paramsText)
				param := func(index, fallback int) int {
					if index < len(params) && params[index] > 0 {
						return params[index]
					}
					return fallback
				}
				switch runes[j] {
				case 'A':
					row = maxInt(0, row-param(0, 1))
				case 'B':
					row = minInt(rows-1, row+param(0, 1))
				case 'C':
					col = minInt(cols-1, col+param(0, 1))
				case 'D':
					col = maxInt(0, col-param(0, 1))
				case 'E':
					row = minInt(rows-1, row+param(0, 1))
					col = 0
				case 'F':
					row = maxInt(0, row-param(0, 1))
					col = 0
				case 'G':
					col = minInt(cols-1, maxInt(0, param(0, 1)-1))
				case 'H', 'f':
					row = minInt(rows-1, maxInt(0, param(0, 1)-1))
					col = minInt(cols-1, maxInt(0, param(1, 1)-1))
				case 'J':
					mode := 0
					if len(params) > 0 {
						mode = params[0]
					}
					switch mode {
					case 1:
						for rr := 0; rr < row; rr++ {
							clearRange(rr, 0, cols-1)
						}
						clearRange(row, 0, col)
					case 2, 3:
						for rr := range grid {
							clearRange(rr, 0, cols-1)
						}
					default:
						clearRange(row, col, cols-1)
						for rr := row + 1; rr < rows; rr++ {
							clearRange(rr, 0, cols-1)
						}
					}
				case 'K':
					mode := 0
					if len(params) > 0 {
						mode = params[0]
					}
					switch mode {
					case 1:
						clearRange(row, 0, col)
					case 2:
						clearRange(row, 0, cols-1)
					default:
						clearRange(row, col, cols-1)
					}
				case 'P':
					count := param(0, 1)
					if count > cols-col {
						count = cols - col
					}
					copy(grid[row][col:], grid[row][col+count:])
					clearRange(row, cols-count, cols-1)
				case '@':
					count := param(0, 1)
					if count > cols-col {
						count = cols - col
					}
					copy(grid[row][col+count:], grid[row][col:cols-count])
					clearRange(row, col, col+count-1)
				case 'X':
					clearRange(row, col, col+param(0, 1)-1)
				case 's':
					savedRow, savedCol = row, col
				case 'u':
					row, col = savedRow, savedCol
				}
				i = j + 1
				continue
			}
			if next == ']' {
				j := i + 2
				for j < len(runes) {
					if runes[j] == '\a' {
						j++
						break
					}
					if runes[j] == '\x1b' && j+1 < len(runes) && runes[j+1] == '\\' {
						j += 2
						break
					}
					j++
				}
				i = j
				continue
			}
			if next == '7' {
				savedRow, savedCol = row, col
			} else if next == '8' {
				row, col = savedRow, savedCol
			} else if next == 'c' {
				for rr := range grid {
					clearRange(rr, 0, cols-1)
				}
				row, col = 0, 0
			}
			i += 2
			continue
		case '\r':
			col = 0
		case '\n':
			lineFeed()
		case '\b':
			if col > 0 {
				col--
			}
		case '\t':
			nextTab := ((col / 8) + 1) * 8
			if nextTab >= cols {
				col = 0
				lineFeed()
			} else {
				col = nextTab
			}
		default:
			if r >= 0x20 && r != 0x7f {
				putRune(r)
			}
		}
		i++
	}
	return terminalGridText(grid)
}

func parseCSIParams(value string) []int {
	value = strings.TrimLeft(value, "?<>=!")
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ";")
	out := make([]int, len(parts))
	for i, part := range parts {
		if part == "" {
			continue
		}
		if parsed, err := strconv.Atoi(part); err == nil {
			out[i] = parsed
		}
	}
	return out
}

func terminalGridText(grid [][]rune) string {
	lines := make([]string, len(grid))
	lastNonEmpty := -1
	for i, row := range grid {
		line := strings.TrimRight(string(row), " ")
		lines[i] = line
		if line != "" {
			lastNonEmpty = i
		}
	}
	if lastNonEmpty < 0 {
		return ""
	}
	return strings.Join(lines[:lastNonEmpty+1], "\n")
}

func (s *BuiltinUtilityService) sshLogin(ctx context.Context, input json.RawMessage, invocation capability.ToolInvocationContext) (json.RawMessage, error) {
	obj, err := unmarshalObject(input)
	if err != nil {
		return nil, err
	}
	config := RemoteTerminalSession{
		Host:          stringValue(obj, "host"),
		Port:          intValue(obj, 22, "port"),
		User:          stringValue(obj, "user"),
		Password:      stringValuePreserveEmpty(obj, "password"),
		PrivateKey:    stringValuePreserveEmpty(obj, "privateKey"),
		HostKey:       stringValuePreserveEmpty(obj, "hostKey"),
		HostKeyPolicy: stringValue(obj, "hostKeyPolicy"),
		AgentAuth:     boolValue(obj, false, "agentAuth"),
		TimeoutMS:     intValue(obj, 30000, "timeoutMs"),
	}
	if config.Host == "" || config.User == "" {
		return nil, errors.New("host and user are required")
	}
	if config.HostKeyPolicy == "" {
		config.HostKeyPolicy = "reject"
	}
	payload := config.payload()
	payload["command"] = "printf __AMITIA_SSH_READY__"
	payload["maxOutputBytes"] = 4096
	probe, err := amitiaLinuxCall(ctx, s.androidLinux, invocation, "ssh.exec", payload)
	if err != nil {
		return nil, fmt.Errorf("ssh_login validation failed: %w", err)
	}
	var probeResult map[string]any
	if err := json.Unmarshal(probe, &probeResult); err != nil {
		return nil, fmt.Errorf("ssh_login validation returned invalid result: %w", err)
	}
	if exitCode := intValue(probeResult, -1, "exitCode"); exitCode != 0 {
		return nil, fmt.Errorf("ssh_login validation command exited with code %d: %s", exitCode, stringValuePreserveEmpty(probeResult, "stderr"))
	}
	if !strings.Contains(stringValuePreserveEmpty(probeResult, "stdout"), "__AMITIA_SSH_READY__") {
		return nil, errors.New("ssh_login validation did not receive readiness marker")
	}
	key, ok := sshScopeKey(invocation)
	if !ok {
		return nil, errors.New("ssh_login requires a user, conversation, character, or session scope")
	}
	s.sshMu.Lock()
	s.sshSessions[key] = config
	s.sshMu.Unlock()
	return marshalResult(map[string]any{"success": true, "connected": true, "host": config.Host, "port": config.Port, "user": config.User, "scope": key, "probe": json.RawMessage(probe)})
}

func (c RemoteTerminalSession) payload() map[string]any {
	payload := map[string]any{
		"host": c.Host, "port": c.Port, "user": c.User, "timeoutMs": c.TimeoutMS,
		"hostKeyPolicy": c.HostKeyPolicy, "agentAuth": c.AgentAuth,
	}
	if c.Password != "" {
		payload["password"] = c.Password
	}
	if c.PrivateKey != "" {
		payload["privateKey"] = c.PrivateKey
	}
	if c.HostKey != "" {
		payload["hostKey"] = c.HostKey
	}
	return payload
}

func (s *BuiltinUtilityService) sshExit(invocation capability.ToolInvocationContext) (json.RawMessage, error) {
	key, ok := sshScopeKey(invocation)
	if !ok {
		return nil, errors.New("ssh_exit requires a user, conversation, character, or session scope")
	}
	s.sshMu.Lock()
	_, existed := s.sshSessions[key]
	delete(s.sshSessions, key)
	s.sshMu.Unlock()
	return marshalResult(map[string]any{"success": true, "disconnected": existed, "scope": key})
}

func (s *BuiltinUtilityService) getSSH(invocation capability.ToolInvocationContext) (RemoteTerminalSession, bool) {
	key, ok := sshScopeKey(invocation)
	if !ok {
		return RemoteTerminalSession{}, false
	}
	s.sshMu.RLock()
	config, ok := s.sshSessions[key]
	s.sshMu.RUnlock()
	return config, ok
}

func sshScopeKey(invocation capability.ToolInvocationContext) (string, bool) {
	parts := []string{invocation.UserID, invocation.ConversationID, invocation.CharacterID, invocation.SessionID}
	nonEmpty := false
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
		nonEmpty = nonEmpty || parts[i] != ""
	}
	if !nonEmpty {
		return "", false
	}
	return strings.Join(parts, "|"), true
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
