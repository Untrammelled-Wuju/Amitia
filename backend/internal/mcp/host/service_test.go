package host

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/mcp"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type samplingCapture struct {
	request map[string]any
}

func (s *samplingCapture) CreateMessage(_ context.Context, _ string, raw json.RawMessage) (any, error) {
	_ = json.Unmarshal(raw, &s.request)
	return map[string]any{"role": "assistant", "content": "ok"}, nil
}

func hostTestService(t *testing.T, sampling SamplingProvider) (*Service, *mcp.Repository, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&mcp.Server{}, &mcp.ServerCapability{}, &mcp.Task{}, &mcp.AuditLog{}); err != nil {
		t.Fatal(err)
	}
	repository := mcp.NewRepository(db)
	server, err := repository.CreateServer(context.Background(), mcp.ServerInput{Name: "host-test", Transport: "streamable_http", Endpoint: "https://example.com/mcp"})
	if err != nil {
		t.Fatal(err)
	}
	return New(repository, nil, nil, sampling, nil), repository, server.ID
}

func TestSamplingDisabledByDefault(t *testing.T) {
	service, _, serverID := hostTestService(t, &samplingCapture{})
	_, rpcErr := service.createMessage(serverID)(context.Background(), json.RawMessage(`{"maxTokens":100}`))
	if rpcErr == nil || rpcErr.Message != "Sampling is not enabled for this server" {
		t.Fatalf("unexpected error: %#v", rpcErr)
	}
}

func TestSamplingClampsTokenLimit(t *testing.T) {
	capture := &samplingCapture{}
	service, repository, serverID := hostTestService(t, capture)
	_, err := repository.SetServerCapability(context.Background(), serverID, "sampling", true, json.RawMessage(`{"maxTokens":256,"timeoutSeconds":10,"maxConcurrent":1}`))
	if err != nil {
		t.Fatal(err)
	}
	result, rpcErr := service.createMessage(serverID)(context.Background(), json.RawMessage(`{"maxTokens":4096,"messages":[]}`))
	if rpcErr != nil || result == nil || capture.request["maxTokens"] != float64(256) {
		t.Fatalf("unexpected result=%#v request=%#v error=%#v", result, capture.request, rpcErr)
	}
}

func TestElicitationRejectsSensitiveFieldsAndUnsafeURL(t *testing.T) {
	service, repository, serverID := hostTestService(t, nil)
	_, err := repository.SetServerCapability(context.Background(), serverID, "elicitation", true, json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{`{"mode":"form","requestedSchema":{"properties":{"api_key":{"type":"string"}}}}`, `{"mode":"url","url":"http://example.com/authorize"}`} {
		result, rpcErr := service.elicit(serverID)(context.Background(), json.RawMessage(raw))
		if rpcErr == nil || result.(map[string]any)["action"] != "decline" {
			t.Fatalf("expected decline for %s, got %#v %#v", raw, result, rpcErr)
		}
	}
}

func TestTasksDisabledByDefault(t *testing.T) {
	service, _, serverID := hostTestService(t, nil)
	_, rpcErr := service.getTask(serverID)(context.Background(), json.RawMessage(`{"taskId":"task-1"}`))
	if rpcErr == nil || rpcErr.Message != "Tasks are not enabled for this server" {
		t.Fatalf("unexpected error: %#v", rpcErr)
	}
}

func TestTaskStatusEnforcesConcurrencyTTLAndResultLimit(t *testing.T) {
	service, repository, serverID := hostTestService(t, nil)
	_, err := repository.SetServerCapability(context.Background(), serverID, "tasks", true, json.RawMessage(`{"maxConcurrent":1,"maxTTLSeconds":60}`))
	if err != nil {
		t.Fatal(err)
	}
	service.taskStatus(serverID)(context.Background(), json.RawMessage(`{"taskId":"one","status":"working","result":{},"expiresAt":"2099-01-01T00:00:00Z"}`))
	service.taskStatus(serverID)(context.Background(), json.RawMessage(`{"taskId":"two","status":"working","result":{}}`))
	tasks, err := repository.ListTasks(context.Background(), serverID, 10)
	if err != nil || len(tasks) != 1 || tasks[0].RemoteTaskID != "one" {
		t.Fatalf("unexpected tasks=%#v err=%v", tasks, err)
	}
	expires, err := time.Parse(time.RFC3339Nano, tasks[0].ExpiresAt)
	if err != nil || expires.After(time.Now().Add(61*time.Second)) {
		t.Fatalf("TTL was not clamped: %s", tasks[0].ExpiresAt)
	}
	large := `{"taskId":"one","status":"completed","result":"` + strings.Repeat("x", (2<<20)+1) + `"}`
	service.taskStatus(serverID)(context.Background(), json.RawMessage(large))
	task, err := repository.GetTask(context.Background(), serverID, "one")
	if err != nil || task.Status != "completed" || task.ResultJSON != `{"truncated":true}` {
		t.Fatalf("unexpected task=%#v err=%v", task, err)
	}
}

type staticRoots []Root

func (r staticRoots) Roots(context.Context, string) ([]Root, error) { return r, nil }

func TestRootsRequireCapabilityAndValidateFileURI(t *testing.T) {
	service, repository, serverID := hostTestService(t, nil)
	service.roots = staticRoots{{URI: "file:///safe/project", Name: "project"}}
	result, rpcErr := service.rootsList(serverID)(context.Background(), nil)
	if rpcErr != nil || len(result.(map[string]any)["roots"].([]Root)) != 0 {
		t.Fatalf("roots should be hidden: %#v %#v", result, rpcErr)
	}
	if _, err := repository.SetServerCapability(context.Background(), serverID, "roots", true, json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	result, rpcErr = service.rootsList(serverID)(context.Background(), nil)
	if rpcErr != nil || len(result.(map[string]any)["roots"].([]Root)) != 1 {
		t.Fatalf("unexpected roots: %#v %#v", result, rpcErr)
	}
	service.roots = staticRoots{{URI: "https://example.com/not-file"}}
	if _, rpcErr := service.rootsList(serverID)(context.Background(), nil); rpcErr == nil {
		t.Fatal("expected invalid root rejection")
	}
}

func TestServerLoggingIsRateLimitedTruncatedAndRedacted(t *testing.T) {
	service, repository, serverID := hostTestService(t, nil)
	handler := service.serverLog(serverID)
	for index := 0; index < 61; index++ {
		handler(context.Background(), json.RawMessage(`{"level":"info","data":"password=secretvalue"}`))
	}
	logs, err := repository.ListAuditLogs(context.Background(), serverID, 100)
	if err != nil || len(logs) != 60 {
		t.Fatalf("unexpected logs=%d err=%v", len(logs), err)
	}
	if strings.Contains(logs[0].SummaryJSON, "secretvalue") || !strings.Contains(logs[0].SummaryJSON, "[REDACTED]") {
		t.Fatalf("secret not redacted: %s", logs[0].SummaryJSON)
	}
}
