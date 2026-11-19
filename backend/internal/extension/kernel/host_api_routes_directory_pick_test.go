package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

type recordingDirectoryPickHost struct {
	path      string
	err       error
	called    bool
	title     string
	startPath string
	extension string
}

func (h *recordingDirectoryPickHost) Notify(ctx context.Context, extensionID string, title string, body string, severity string) error {
	return nil
}

func (h *recordingDirectoryPickHost) Dialog(ctx context.Context, extensionID string, dialogID string, message string, buttons []string) (string, error) {
	return "", nil
}

func (h *recordingDirectoryPickHost) PickDirectory(ctx context.Context, extensionID string, requestID string, title string, startPath string) (string, error) {
	h.called = true
	h.extension = extensionID
	h.title = title
	h.startPath = startPath
	return h.path, h.err
}

func (h *recordingDirectoryPickHost) Navigate(ctx context.Context, extensionID string, target string) error {
	return nil
}

func callDirectoryPickRoute(t *testing.T, deps HostAPIRouteDeps, input string) host_api.CallResult {
	t.Helper()
	gateway := newConversationMessageTestGateway()
	if err := setupDefaultHostAPIRoutes(gateway, deps); err != nil {
		t.Fatal(err)
	}
	return gateway.Call(context.Background(), host_api.CallRequest{
		CallID:          "call-directory-pick",
		RuntimeIdentity: runtime_supervisor.RuntimeIdentity{InstanceID: "runtime-1", ExtensionID: domain.ExtensionID("com.amitia/dsh-compatible-host"), ModuleID: "dsh-compatible-runtime"},
		Method:          host_api.MethodDirectoryPick,
		Version:         1,
		Input:           json.RawMessage(input),
	})
}

func TestDirectoryPickRouteReturnsHostSelection(t *testing.T) {
	permissions := host_api.RoutePermissionForMethod(host_api.MethodDirectoryPick)
	if len(permissions) != 1 || permissions[0].Name != "workspace.read" {
		t.Fatalf("unexpected permissions: %+v", permissions)
	}
	host := &recordingDirectoryPickHost{path: "/Users/untrammelled/Desktop/Project/U-AI"}
	result := callDirectoryPickRoute(t, HostAPIRouteDeps{UIHostNotifier: host}, `{"title":"选择目录","startPath":"/Users/untrammelled"}`)
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success, got %s: %+v", result.Status, result.Error)
	}
	if !host.called || host.extension != "com.amitia/dsh-compatible-host" || host.title != "选择目录" || host.startPath != "/Users/untrammelled" {
		t.Fatalf("unexpected host call: %+v", host)
	}
	var output struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatal(err)
	}
	if output.Path != "/Users/untrammelled/Desktop/Project/U-AI" {
		t.Fatalf("unexpected path: %s", result.Output)
	}
}

func TestDirectoryPickRouteReportsUnavailableAndPreservesError(t *testing.T) {
	result := callDirectoryPickRoute(t, HostAPIRouteDeps{}, `{}`)
	if result.Status != host_api.StatusFailed || result.Error == nil || result.Error.Code != host_api.ErrorCodeHostUnavailable {
		t.Fatalf("expected host unavailable, got %+v", result)
	}
	original := errors.New("ui_host: directory pick host unavailable")
	result = callDirectoryPickRoute(t, HostAPIRouteDeps{UIHostNotifier: &recordingDirectoryPickHost{err: original}}, `{}`)
	if result.Status != host_api.StatusFailed || result.Error == nil || !strings.Contains(result.Error.Message, original.Error()) {
		t.Fatalf("expected original error, got %+v", result)
	}
}
