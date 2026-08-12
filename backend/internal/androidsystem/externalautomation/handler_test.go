package externalautomation

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/androidsystem"
)

type fakeExternalAutomationClient struct {
	statusResult    CapabilityState
	statusErr       error
	resolveAppResult []ResolvedApp
	resolveAppErr   error
	openAppResult   ActionResult
	openAppErr      error
	resolveURIResult ResolvedURI
	resolveURIErr   error
	openURIResult   ActionResult
	openURIErr      error
	openSettingsResult ActionResult
	openSettingsErr error
	invokeIntentResult ActionResult
	invokeIntentErr error
	foregroundResult ForegroundState
	foregroundErr   error
	waitForegroundResult ForegroundState
	waitForegroundErr   error
}

func (f *fakeExternalAutomationClient) Status(ctx context.Context) (CapabilityState, error) {
	return f.statusResult, f.statusErr
}

func (f *fakeExternalAutomationClient) ResolveApp(ctx context.Context, req ResolveAppRequest) ([]ResolvedApp, error) {
	return f.resolveAppResult, f.resolveAppErr
}

func (f *fakeExternalAutomationClient) OpenApp(ctx context.Context, req OpenAppRequest) (ActionResult, error) {
	return f.openAppResult, f.openAppErr
}

func (f *fakeExternalAutomationClient) ResolveURI(ctx context.Context, req ResolveURIRequest) (ResolvedURI, error) {
	return f.resolveURIResult, f.resolveURIErr
}

func (f *fakeExternalAutomationClient) OpenURI(ctx context.Context, req OpenURIRequest) (ActionResult, error) {
	return f.openURIResult, f.openURIErr
}

func (f *fakeExternalAutomationClient) OpenSettings(ctx context.Context, req OpenSettingsRequest) (ActionResult, error) {
	return f.openSettingsResult, f.openSettingsErr
}

func (f *fakeExternalAutomationClient) InvokeIntent(ctx context.Context, spec IntentSpec) (ActionResult, error) {
	return f.invokeIntentResult, f.invokeIntentErr
}

func (f *fakeExternalAutomationClient) Foreground(ctx context.Context) (ForegroundState, error) {
	return f.foregroundResult, f.foregroundErr
}

func (f *fakeExternalAutomationClient) WaitForeground(ctx context.Context, req WaitForegroundRequest) (ForegroundState, error) {
	return f.waitForegroundResult, f.waitForegroundErr
}

func newDefaultFakeClient() *fakeExternalAutomationClient {
	return &fakeExternalAutomationClient{
		statusResult: CapabilityState{
			Supported:           true,
			CanResolveApps:      true,
			CanLaunchApps:       true,
			CanResolveURI:       true,
			CanOpenURI:          true,
			CanOpenSettings:     true,
			CanInvokeIntent:     true,
			CanInspectForeground: true,
			CanWaitForeground:   true,
			State:               StateAvailable,
		},
	}
}

func TestHandlerStatus(t *testing.T) {
	client := newDefaultFakeClient()
	handler := NewExternalAutomationHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationStatus,
	})

	if resp.Status != "success" {
		t.Errorf("expected success, got %s", resp.Status)
	}
	if resp.Error != nil {
		t.Errorf("unexpected error: %v", resp.Error)
	}
	if resp.Result["supported"] != true {
		t.Errorf("expected supported=true, got %v", resp.Result["supported"])
	}
}

func TestHandlerStatusError(t *testing.T) {
	client := newDefaultFakeClient()
	client.statusErr = newAutomationError(AUTOMATION_NATIVE_HOST_UNAVAILABLE, "host not ready")
	handler := NewExternalAutomationHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationStatus,
	})

	if resp.Status != "error" {
		t.Errorf("expected error, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != AUTOMATION_NATIVE_HOST_UNAVAILABLE {
		t.Errorf("expected AUTOMATION_NATIVE_HOST_UNAVAILABLE, got %v", resp.Error)
	}
}

func TestHandlerResolveApp(t *testing.T) {
	client := newDefaultFakeClient()
	client.resolveAppResult = []ResolvedApp{
		{PackageName: "com.chrome", Label: "Chrome", Launchable: true},
	}
	handler := NewExternalAutomationHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationResolveApp,
		Payload: map[string]any{
			"query": "Chrome",
		},
	})

	if resp.Status != "success" {
		t.Errorf("expected success, got %s: %v", resp.Status, resp.Error)
	}
	if resp.Result["count"] != 1 {
		t.Errorf("expected count=1, got %v", resp.Result["count"])
	}
}

func TestHandlerResolveAppEmptyQuery(t *testing.T) {
	client := newDefaultFakeClient()
	handler := NewExternalAutomationHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationResolveApp,
		Payload:   map[string]any{},
	})

	if resp.Status != "error" {
		t.Errorf("expected error, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != AUTOMATION_INVALID_REQUEST {
		t.Errorf("expected AUTOMATION_INVALID_REQUEST, got %v", resp.Error)
	}
}

func TestHandlerOpenApp(t *testing.T) {
	client := newDefaultFakeClient()
	client.openAppResult = ActionResult{
		Success:       true,
		Operation:     "open_app",
		TargetPackage: "com.example.app",
	}
	handler := NewExternalAutomationHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationOpenApp,
		Payload: map[string]any{
			"packageName": "com.example.app",
		},
	})

	if resp.Status != "success" {
		t.Errorf("expected success, got %s: %v", resp.Status, resp.Error)
	}
	if resp.Result["success"] != true {
		t.Errorf("expected success=true, got %v", resp.Result["success"])
	}
}

func TestHandlerOpenAppEmptyPackage(t *testing.T) {
	client := newDefaultFakeClient()
	handler := NewExternalAutomationHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationOpenApp,
		Payload:   map[string]any{},
	})

	if resp.Status != "error" {
		t.Errorf("expected error, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != AUTOMATION_INVALID_REQUEST {
		t.Errorf("expected AUTOMATION_INVALID_REQUEST, got %v", resp.Error)
	}
}

func TestHandlerResolveURI(t *testing.T) {
	client := newDefaultFakeClient()
	client.resolveURIResult = ResolvedURI{
		URI:      "https://example.com",
		Scheme:   "https",
		Resolved: true,
	}
	handler := NewExternalAutomationHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationResolveURI,
		Payload: map[string]any{
			"uri": "https://example.com",
		},
	})

	if resp.Status != "success" {
		t.Errorf("expected success, got %s: %v", resp.Status, resp.Error)
	}
	if resp.Result["scheme"] != "https" {
		t.Errorf("expected scheme=https, got %v", resp.Result["scheme"])
	}
}

func TestHandlerResolveURIEmptyURI(t *testing.T) {
	client := newDefaultFakeClient()
	handler := NewExternalAutomationHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationResolveURI,
		Payload:   map[string]any{},
	})

	if resp.Status != "error" {
		t.Errorf("expected error, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != AUTOMATION_URI_INVALID {
		t.Errorf("expected AUTOMATION_URI_INVALID, got %v", resp.Error)
	}
}

func TestHandlerOpenURI(t *testing.T) {
	client := newDefaultFakeClient()
	client.openURIResult = ActionResult{
		Success: true,
	}
	handler := NewExternalAutomationHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationOpenURI,
		Payload: map[string]any{
			"uri": "https://example.com",
		},
	})

	if resp.Status != "success" {
		t.Errorf("expected success, got %s: %v", resp.Status, resp.Error)
	}
}

func TestHandlerOpenURIBlockedScheme(t *testing.T) {
	client := newDefaultFakeClient()
	handler := NewExternalAutomationHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationOpenURI,
		Payload: map[string]any{
			"uri": "javascript:alert(1)",
		},
	})

	if resp.Status != "error" {
		t.Errorf("expected error, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != AUTOMATION_URI_SCHEME_BLOCKED {
		t.Errorf("expected AUTOMATION_URI_SCHEME_BLOCKED, got %v", resp.Error)
	}
}

func TestHandlerOpenSettings(t *testing.T) {
	client := newDefaultFakeClient()
	client.openSettingsResult = ActionResult{
		Success: true,
	}
	handler := NewExternalAutomationHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationOpenSettings,
		Payload: map[string]any{
			"page": "accessibility",
		},
	})

	if resp.Status != "success" {
		t.Errorf("expected success, got %s: %v", resp.Status, resp.Error)
	}
}

func TestHandlerOpenSettingsUnsupported(t *testing.T) {
	client := newDefaultFakeClient()
	handler := NewExternalAutomationHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationOpenSettings,
		Payload: map[string]any{
			"page": "invalid_page",
		},
	})

	if resp.Status != "error" {
		t.Errorf("expected error, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != AUTOMATION_SETTINGS_UNSUPPORTED {
		t.Errorf("expected AUTOMATION_SETTINGS_UNSUPPORTED, got %v", resp.Error)
	}
}

func TestHandlerInvokeIntent(t *testing.T) {
	client := newDefaultFakeClient()
	client.invokeIntentResult = ActionResult{
		Success: true,
	}
	handler := NewExternalAutomationHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationInvokeIntent,
		Payload: map[string]any{
			"action": "android.intent.action.VIEW",
		},
	})

	if resp.Status != "success" {
		t.Errorf("expected success, got %s: %v", resp.Status, resp.Error)
	}
}

func TestHandlerInvokeIntentBlockedAction(t *testing.T) {
	client := newDefaultFakeClient()
	handler := NewExternalAutomationHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationInvokeIntent,
		Payload: map[string]any{
			"action": "com.vendor.custom.ACTION",
		},
	})

	if resp.Status != "error" {
		t.Errorf("expected error, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != AUTOMATION_INTENT_ACTION_BLOCKED {
		t.Errorf("expected AUTOMATION_INTENT_ACTION_BLOCKED, got %v", resp.Error)
	}
}

func TestHandlerInvokeIntentSendRejected(t *testing.T) {
	client := newDefaultFakeClient()
	handler := NewExternalAutomationHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationInvokeIntent,
		Payload: map[string]any{
			"action": "android.intent.action.SEND",
		},
	})

	if resp.Status != "error" {
		t.Errorf("expected error, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != AUTOMATION_INTENT_ACTION_BLOCKED {
		t.Errorf("expected AUTOMATION_INTENT_ACTION_BLOCKED, got %v", resp.Error)
	}
}

func TestHandlerForeground(t *testing.T) {
	client := newDefaultFakeClient()
	client.foregroundResult = ForegroundState{
		PackageName: "com.example.app",
		Source:      SourceAccessibility,
		Confidence:  ConfidenceHigh,
	}
	handler := NewExternalAutomationHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationForeground,
	})

	if resp.Status != "success" {
		t.Errorf("expected success, got %s: %v", resp.Status, resp.Error)
	}
	if resp.Result["packageName"] != "com.example.app" {
		t.Errorf("expected packageName=com.example.app, got %v", resp.Result["packageName"])
	}
}

func TestHandlerWaitForeground(t *testing.T) {
	client := newDefaultFakeClient()
	client.waitForegroundResult = ForegroundState{
		PackageName: "com.example.app",
		Source:      SourceAccessibility,
	}
	handler := NewExternalAutomationHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationWaitForeground,
		Payload: map[string]any{
			"packageName": "com.example.app",
			"timeoutMs":   5000,
		},
	})

	if resp.Status != "success" {
		t.Errorf("expected success, got %s: %v", resp.Status, resp.Error)
	}
}

func TestHandlerWaitForegroundEmptyPackage(t *testing.T) {
	client := newDefaultFakeClient()
	handler := NewExternalAutomationHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationWaitForeground,
		Payload:   map[string]any{},
	})

	if resp.Status != "error" {
		t.Errorf("expected error, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != AUTOMATION_INVALID_REQUEST {
		t.Errorf("expected AUTOMATION_INVALID_REQUEST, got %v", resp.Error)
	}
}

func TestHandlerUnknownOperation(t *testing.T) {
	client := newDefaultFakeClient()
	handler := NewExternalAutomationHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: "unknown.op",
	})

	if resp.Status != "error" {
		t.Errorf("expected error, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != AUTOMATION_UNSUPPORTED {
		t.Errorf("expected AUTOMATION_UNSUPPORTED, got %v", resp.Error)
	}
}

func TestHandlerGenericError(t *testing.T) {
	client := newDefaultFakeClient()
	client.statusErr = errors.New("generic error")
	handler := NewExternalAutomationHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationStatus,
	})

	if resp.Status != "error" {
		t.Errorf("expected error, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != AUTOMATION_NATIVE_HOST_UNAVAILABLE {
		t.Errorf("expected AUTOMATION_NATIVE_HOST_UNAVAILABLE, got %v", resp.Error)
	}
}

func TestBlockedClient(t *testing.T) {
	client := NewBlockedExternalAutomationClient()
	handler := NewExternalAutomationHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationStatus,
	})

	if resp.Status != "error" {
		t.Errorf("expected error, got %s", resp.Status)
	}
}

func TestHandlerWithNilClient(t *testing.T) {
	handler := NewExternalAutomationHandler(nil)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationStatus,
	})

	if resp.Status != "error" {
		t.Errorf("expected error, got %s", resp.Status)
	}
}

func TestHandlerOpenAppWithExtras(t *testing.T) {
	client := newDefaultFakeClient()
	client.openAppResult = ActionResult{
		Success:       true,
		TargetPackage: "com.example.app",
	}
	handler := NewExternalAutomationHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationOpenApp,
		Payload: map[string]any{
			"packageName": "com.example.app",
			"extras": map[string]any{
				"key1": "value1",
				"key2": 123,
			},
			"newTask": true,
		},
	})

	if resp.Status != "success" {
		t.Errorf("expected success, got %s: %v", resp.Status, resp.Error)
	}
}

func TestHandlerInvokeIntentWithCategories(t *testing.T) {
	client := newDefaultFakeClient()
	client.invokeIntentResult = ActionResult{
		Success: true,
	}
	handler := NewExternalAutomationHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationInvokeIntent,
		Payload: map[string]any{
			"action":     "android.intent.action.VIEW",
			"categories": []any{"android.intent.category.DEFAULT"},
		},
	})

	if resp.Status != "success" {
		t.Errorf("expected success, got %s: %v", resp.Status, resp.Error)
	}
}

func TestAutomationError(t *testing.T) {
	err := newAutomationError(TEST_ERROR_CODE, "test message")
	if err.Error() != "test message" {
		t.Errorf("expected 'test message', got '%s'", err.Error())
	}
	if err.Code() != TEST_ERROR_CODE {
		t.Errorf("expected '%s', got '%s'", TEST_ERROR_CODE, err.Code())
	}
}

const TEST_ERROR_CODE = "TEST_ERROR_CODE"
