package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
)

type recordingProjectLister struct {
	limit  int
	cursor string
	output any
	err    error
	called bool
}

func (r *recordingProjectLister) ListProjects(_ context.Context, limit int, cursor string) (any, error) {
	r.called = true
	r.limit, r.cursor = limit, cursor
	return r.output, r.err
}

type recordingProjectCreator struct {
	request ProjectCreateRequest
	output  json.RawMessage
	err     error
	called  bool
}

func (r *recordingProjectCreator) CreateProject(_ context.Context, request ProjectCreateRequest) (json.RawMessage, error) {
	r.called = true
	r.request = request
	return r.output, r.err
}

type recordingProjectUpdater struct {
	request ProjectUpdateRequest
	output  json.RawMessage
	err     error
	called  bool
}

func (r *recordingProjectUpdater) UpdateProject(_ context.Context, request ProjectUpdateRequest) (json.RawMessage, error) {
	r.called = true
	r.request = request
	return r.output, r.err
}

type recordingProjectDeleter struct {
	request ProjectDeleteRequest
	output  json.RawMessage
	err     error
	called  bool
}

func (r *recordingProjectDeleter) DeleteProject(_ context.Context, request ProjectDeleteRequest) (json.RawMessage, error) {
	r.called = true
	r.request = request
	return r.output, r.err
}

type recordingConversationArchiver struct {
	request ConversationArchiveRequest
	output  json.RawMessage
	err     error
	called  bool
}

func (r *recordingConversationArchiver) ArchiveConversation(_ context.Context, request ConversationArchiveRequest) (json.RawMessage, error) {
	r.called = true
	r.request = request
	return r.output, r.err
}

func projectRouteGateway(t *testing.T, deps HostAPIRouteDeps) *host_api.DefaultGateway {
	t.Helper()
	if deps.ScopeSnapshotStore == nil {
		deps.ScopeSnapshotStore = &fixedConversationScopeStore{snapshot: &scope.ScopeSnapshot{
			SnapshotID:     "scope-project",
			SpaceID:        "space-1",
			CharacterID:    "character-1",
			ConversationID: "conversation-1",
		}}
	}
	gateway := newConversationMessageTestGateway()
	if err := setupDefaultHostAPIRoutes(gateway, deps); err != nil {
		t.Fatalf("setup routes: %v", err)
	}
	return gateway
}

func projectRouteCall(method host_api.Method, input string) host_api.CallRequest {
	return host_api.CallRequest{
		CallID:          "call-" + string(method),
		RuntimeIdentity: runtime_supervisor.RuntimeIdentity{InstanceID: "runtime-1", ExtensionID: domain.ExtensionID("com.amitia/dsh-compatible-host"), ModuleID: "dsh-compatible-runtime"},
		Method:          method,
		Version:         1,
		Input:           json.RawMessage(input),
		ScopeSnapshotID: "scope-project",
	}
}

func TestProjectRoutePermissionsAndPolicies(t *testing.T) {
	for _, method := range []host_api.Method{host_api.MethodProjectCreate, host_api.MethodProjectUpdate, host_api.MethodProjectDelete} {
		permissions := host_api.RoutePermissionForMethod(method)
		if len(permissions) != 1 || permissions[0].Name != "project.manage" || permissions[0].Resource != "project" {
			t.Fatalf("unexpected permissions for %s: %+v", method, permissions)
		}
		if risk := host_api.RouteRiskForMethod(method); risk != host_api.RiskHigh {
			t.Fatalf("unexpected risk for %s: %s", method, risk)
		}
		if effect := host_api.RouteSideEffectForMethod(method); effect != host_api.SideEffectWrite {
			t.Fatalf("unexpected side effect for %s: %s", method, effect)
		}
		if !host_api.IsDataRouteMethod(method) || !host_api.RouteScopeForMethod(method).Namespaced {
			t.Fatalf("expected data route with namespaced scope for %s", method)
		}
	}
	permissions := host_api.RoutePermissionForMethod(host_api.MethodProjectList)
	if len(permissions) != 1 || permissions[0].Name != "project.read" || permissions[0].Resource != "project" {
		t.Fatalf("unexpected permissions for project list: %+v", permissions)
	}
	if risk := host_api.RouteRiskForMethod(host_api.MethodProjectList); risk != host_api.RiskLow {
		t.Fatalf("unexpected risk for project list: %s", risk)
	}
	if effect := host_api.RouteSideEffectForMethod(host_api.MethodProjectList); effect != host_api.SideEffectReadOnly {
		t.Fatalf("unexpected side effect for project list: %s", effect)
	}
	archive := host_api.RoutePermissionForMethod(host_api.MethodConversationArchive)
	if len(archive) != 1 || archive[0].Name != "conversation.manage" || archive[0].Resource != "conversation" {
		t.Fatalf("unexpected permissions for conversation archive: %+v", archive)
	}
	if risk := host_api.RouteRiskForMethod(host_api.MethodConversationArchive); risk != host_api.RiskHigh {
		t.Fatalf("unexpected risk for conversation archive: %s", risk)
	}
}

func TestProjectListRouteResolvesScopeAndClampsLimit(t *testing.T) {
	lister := &recordingProjectLister{output: map[string]any{"projects": []any{}}}
	gateway := projectRouteGateway(t, HostAPIRouteDeps{ProjectLister: lister})
	result := gateway.Call(context.Background(), projectRouteCall(host_api.MethodProjectList, `{"limit":900,"cursor":"cursor-1"}`))
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success, got %s: %+v", result.Status, result.Error)
	}
	if !lister.called || lister.limit != 128 || lister.cursor != "cursor-1" {
		t.Fatalf("unexpected list call: limit=%d cursor=%q", lister.limit, lister.cursor)
	}
	result = gateway.Call(context.Background(), projectRouteCall(host_api.MethodProjectList, `{}`))
	if result.Status != host_api.StatusSuccess || lister.limit != 50 {
		t.Fatalf("unexpected default limit: status=%s limit=%d", result.Status, lister.limit)
	}
}

func TestProjectCreateRouteUsesResolvedScope(t *testing.T) {
	creator := &recordingProjectCreator{output: json.RawMessage(`{"project":{"id":"project-1","title":"Docs"}}`)}
	gateway := projectRouteGateway(t, HostAPIRouteDeps{ProjectCreator: creator})
	result := gateway.Call(context.Background(), projectRouteCall(host_api.MethodProjectCreate, `{"title":"Docs","workspaces":[{"id":"workspace-1","deviceId":"device-1","name":"Docs","kind":"local","rootUri":"amitia://workspace/@workspace-1/vault"}],"primaryWorkspaceId":"workspace-1","requestId":"project-create-1"}`))
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success, got %s: %+v", result.Status, result.Error)
	}
	if !creator.called {
		t.Fatal("creator was not called")
	}
	if creator.request.SpaceID != "space-1" || creator.request.CharacterID != "character-1" || creator.request.Title != "Docs" || creator.request.PrimaryWorkspaceID != "workspace-1" || creator.request.RequestID != "project-create-1" {
		t.Fatalf("unexpected create request: %+v", creator.request)
	}
	if len(creator.request.Workspaces) != 1 || creator.request.Workspaces[0].RootURI != "amitia://workspace/@workspace-1/vault" {
		t.Fatalf("unexpected workspaces: %+v", creator.request.Workspaces)
	}
}

func TestProjectCreateRouteForwardsDirectoryPath(t *testing.T) {
	creator := &recordingProjectCreator{output: json.RawMessage(`{"project":{"id":"project-1"}}`)}
	gateway := projectRouteGateway(t, HostAPIRouteDeps{ProjectCreator: creator})
	result := gateway.Call(context.Background(), projectRouteCall(host_api.MethodProjectCreate, `{"title":"Docs","directoryPath":"/Users/example/vault","requestId":"project-directory-1"}`))
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success, got %s: %+v", result.Status, result.Error)
	}
	if creator.request.DirectoryPath != "/Users/example/vault" {
		t.Fatalf("expected directory path to be forwarded, got %+v", creator.request)
	}
	if len(creator.request.Workspaces) != 0 || creator.request.PrimaryWorkspaceID != "" {
		t.Fatalf("expected no explicit workspaces, got %+v", creator.request)
	}
}

func TestProjectCreateRouteRejectsDirectoryPathWithWorkspaces(t *testing.T) {
	creator := &recordingProjectCreator{output: json.RawMessage(`{}`)}
	gateway := projectRouteGateway(t, HostAPIRouteDeps{ProjectCreator: creator})
	result := gateway.Call(context.Background(), projectRouteCall(host_api.MethodProjectCreate, `{"title":"Docs","directoryPath":"/Users/example/vault","workspaces":[{"id":"workspace-1","deviceId":"device-1","name":"Docs","kind":"local","rootUri":"amitia://workspace/@workspace-1/"}],"requestId":"project-directory-2"}`))
	if result.Status != host_api.StatusFailed || result.Error == nil || result.Error.Code != host_api.ErrorCodeInputInvalid {
		t.Fatalf("expected input rejection, got %s: %+v", result.Status, result.Error)
	}
	if creator.called {
		t.Fatal("creator must not be called when directoryPath and workspaces are both provided")
	}
}

func TestProjectCreateRoutePreservesCreatorError(t *testing.T) {
	original := errors.New("project create fixture failed: 项目工作区不属于当前执行设备")
	creator := &recordingProjectCreator{err: original}
	gateway := projectRouteGateway(t, HostAPIRouteDeps{ProjectCreator: creator})
	result := gateway.Call(context.Background(), projectRouteCall(host_api.MethodProjectCreate, `{"title":"Docs","requestId":"project-create-error"}`))
	if result.Status != host_api.StatusFailed || result.Error == nil {
		t.Fatalf("expected failure, got %s", result.Status)
	}
	if result.Error.Message != original.Error() {
		t.Fatalf("expected original error message, got %q", result.Error.Message)
	}
}

func TestProjectCreateRouteRejectsScopeMismatch(t *testing.T) {
	creator := &recordingProjectCreator{output: json.RawMessage(`{}`)}
	gateway := projectRouteGateway(t, HostAPIRouteDeps{ProjectCreator: creator})
	result := gateway.Call(context.Background(), projectRouteCall(host_api.MethodProjectCreate, `{"spaceId":"space-2","title":"Docs"}`))
	if result.Status != host_api.StatusRejected || result.Error == nil || result.Error.Code != host_api.ErrorCodeScopeDenied {
		t.Fatalf("expected scope rejection, got %s: %+v", result.Status, result.Error)
	}
	if creator.called {
		t.Fatal("creator must not be called on scope mismatch")
	}
}

func TestProjectUpdateRouteRequiresChangeAndForwardsFields(t *testing.T) {
	updater := &recordingProjectUpdater{output: json.RawMessage(`{"projectId":"project-1","revision":2}`)}
	gateway := projectRouteGateway(t, HostAPIRouteDeps{ProjectUpdater: updater})
	result := gateway.Call(context.Background(), projectRouteCall(host_api.MethodProjectUpdate, `{"projectId":"project-1"}`))
	if result.Status != host_api.StatusFailed || result.Error == nil || result.Error.Code != host_api.ErrorCodeInputInvalid {
		t.Fatalf("expected input rejection, got %s: %+v", result.Status, result.Error)
	}
	if updater.called {
		t.Fatal("updater must not be called without changes")
	}
	result = gateway.Call(context.Background(), projectRouteCall(host_api.MethodProjectUpdate, `{"projectId":"project-1","title":"Renamed","pinned":true,"requestId":"project-update-1"}`))
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success, got %s: %+v", result.Status, result.Error)
	}
	if !updater.called || updater.request.ProjectID != "project-1" || updater.request.Title == nil || *updater.request.Title != "Renamed" || updater.request.Pinned == nil || !*updater.request.Pinned || updater.request.Hidden != nil || updater.request.RequestID != "project-update-1" {
		t.Fatalf("unexpected update request: %+v", updater.request)
	}
}

func TestProjectDeleteRoutePreservesDeleterError(t *testing.T) {
	original := errors.New("project delete fixture failed: 修改页面的数据范围已变化")
	deleter := &recordingProjectDeleter{err: original}
	gateway := projectRouteGateway(t, HostAPIRouteDeps{ProjectDeleter: deleter})
	result := gateway.Call(context.Background(), projectRouteCall(host_api.MethodProjectDelete, `{"projectId":"project-1","requestId":"project-delete-1"}`))
	if result.Status != host_api.StatusFailed || result.Error == nil || result.Error.Message != original.Error() {
		t.Fatalf("expected original delete error, got %s: %+v", result.Status, result.Error)
	}
	deleter.err = nil
	deleter.output = json.RawMessage(`{"projectId":"project-1","revision":3}`)
	result = gateway.Call(context.Background(), projectRouteCall(host_api.MethodProjectDelete, `{"projectId":"project-1","requestId":"project-delete-2"}`))
	if result.Status != host_api.StatusSuccess || deleter.request.ProjectID != "project-1" || deleter.request.RequestID != "project-delete-2" {
		t.Fatalf("unexpected delete result: %s %+v", result.Status, deleter.request)
	}
}

func TestConversationArchiveRouteInterruptsThenArchives(t *testing.T) {
	archiver := &recordingConversationArchiver{output: json.RawMessage(`{"conversationId":"conversation-1","archived":true}`)}
	controller := &recordingConversationTurnController{result: ConversationTurnInterruptResult{Interrupted: true}}
	gateway := projectRouteGateway(t, HostAPIRouteDeps{ConversationArchiver: archiver, ConversationTurnController: controller})
	result := gateway.Call(context.Background(), projectRouteCall(host_api.MethodConversationArchive, `{"conversationId":"conversation-1","archived":true,"stopActivity":true,"requestId":"archive-1"}`))
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success, got %s: %+v", result.Status, result.Error)
	}
	if !controller.called || controller.request.ConversationID != "conversation-1" {
		t.Fatalf("expected interrupt before archive, got %+v", controller.request)
	}
	if !archiver.called || archiver.request.ConversationID != "conversation-1" || !archiver.request.Archived || archiver.request.SpaceID != "space-1" || archiver.request.CharacterID != "character-1" || archiver.request.RequestID != "archive-1" {
		t.Fatalf("unexpected archive request: %+v", archiver.request)
	}
}

func TestConversationArchiveRouteSkipsInterruptWhenActivityNotStopped(t *testing.T) {
	archiver := &recordingConversationArchiver{output: json.RawMessage(`{"conversationId":"conversation-1","archived":false}`)}
	controller := &recordingConversationTurnController{result: ConversationTurnInterruptResult{Interrupted: true}}
	gateway := projectRouteGateway(t, HostAPIRouteDeps{ConversationArchiver: archiver, ConversationTurnController: controller})
	result := gateway.Call(context.Background(), projectRouteCall(host_api.MethodConversationArchive, `{"conversationId":"conversation-1","archived":false}`))
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success, got %s: %+v", result.Status, result.Error)
	}
	if controller.called {
		t.Fatal("interrupt must not run when archiving is false")
	}
	if !archiver.called || archiver.request.Archived {
		t.Fatalf("unexpected archive request: %+v", archiver.request)
	}
}

func TestConversationArchiveRouteRequiresArchivedFlag(t *testing.T) {
	archiver := &recordingConversationArchiver{output: json.RawMessage(`{}`)}
	gateway := projectRouteGateway(t, HostAPIRouteDeps{ConversationArchiver: archiver})
	result := gateway.Call(context.Background(), projectRouteCall(host_api.MethodConversationArchive, `{"conversationId":"conversation-1"}`))
	if result.Status != host_api.StatusFailed || result.Error == nil || result.Error.Code != host_api.ErrorCodeInputInvalid {
		t.Fatalf("expected input rejection, got %s: %+v", result.Status, result.Error)
	}
	if archiver.called {
		t.Fatal("archiver must not be called without archived flag")
	}
}

func TestConversationArchiveRoutePreservesArchiverError(t *testing.T) {
	original := errors.New("conversation archive fixture failed: 数据修改参数无效")
	archiver := &recordingConversationArchiver{err: original}
	gateway := projectRouteGateway(t, HostAPIRouteDeps{ConversationArchiver: archiver})
	result := gateway.Call(context.Background(), projectRouteCall(host_api.MethodConversationArchive, `{"conversationId":"conversation-1","archived":true}`))
	if result.Status != host_api.StatusFailed || result.Error == nil || result.Error.Message != original.Error() {
		t.Fatalf("expected original archive error, got %s: %+v", result.Status, result.Error)
	}
}

func TestProjectRoutesAcceptMatchingInputScopeAndRejectMismatch(t *testing.T) {
	lister := &recordingProjectLister{output: map[string]any{"projects": []any{}}}
	deleter := &recordingProjectDeleter{output: json.RawMessage(`{"projectId":"project-1","deleted":true}`)}
	gateway := projectRouteGateway(t, HostAPIRouteDeps{ProjectLister: lister, ProjectDeleter: deleter})
	result := gateway.Call(context.Background(), projectRouteCall(host_api.MethodProjectList, `{"spaceId":"space-1","characterId":"character-1","limit":5}`))
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success with matching input scope, got %s: %+v", result.Status, result.Error)
	}
	result = gateway.Call(context.Background(), projectRouteCall(host_api.MethodProjectDelete, `{"projectId":"project-1","spaceId":"space-1","characterId":"character-1"}`))
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected delete success with matching input scope, got %s: %+v", result.Status, result.Error)
	}
	lister.called = false
	deleter.called = false
	for _, testCase := range []struct {
		method host_api.Method
		input  string
	}{
		{host_api.MethodProjectList, `{"spaceId":"space-2"}`},
		{host_api.MethodProjectList, `{"characterId":"character-2"}`},
		{host_api.MethodProjectDelete, `{"projectId":"project-1","spaceId":"space-2"}`},
		{host_api.MethodProjectDelete, `{"projectId":"project-1","characterId":"character-2"}`},
	} {
		result := gateway.Call(context.Background(), projectRouteCall(testCase.method, testCase.input))
		if result.Status != host_api.StatusRejected || result.Error == nil || result.Error.Code != host_api.ErrorCodeScopeDenied {
			t.Fatalf("expected scope denial for %s %s, got %s: %+v", testCase.method, testCase.input, result.Status, result.Error)
		}
	}
	if lister.called || deleter.called {
		t.Fatal("route handler called the port despite a mismatched input scope")
	}
}

type recordingProjectReorderer struct {
	request ProjectReorderRequest
	output  json.RawMessage
	err     error
	called  bool
}

func (r *recordingProjectReorderer) ReorderProject(_ context.Context, request ProjectReorderRequest) (json.RawMessage, error) {
	r.called = true
	r.request = request
	return r.output, r.err
}

type recordingConversationReorderer struct {
	request ConversationReorderRequest
	output  json.RawMessage
	err     error
	called  bool
}

func (r *recordingConversationReorderer) ReorderConversation(_ context.Context, request ConversationReorderRequest) (json.RawMessage, error) {
	r.called = true
	r.request = request
	return r.output, r.err
}

func TestReorderRoutesUseProjectAndConversationPolicies(t *testing.T) {
	for method, permission := range map[host_api.Method]string{host_api.MethodProjectReorder: "project.manage", host_api.MethodConversationReorder: "conversation.manage"} {
		permissions := host_api.RoutePermissionForMethod(method)
		if len(permissions) != 1 || permissions[0].Name != permission {
			t.Fatalf("unexpected permissions for %s: %+v", method, permissions)
		}
		if risk := host_api.RouteRiskForMethod(method); risk != host_api.RiskMedium {
			t.Fatalf("unexpected risk for %s: %s", method, risk)
		}
		if effect := host_api.RouteSideEffectForMethod(method); effect != host_api.SideEffectWrite {
			t.Fatalf("unexpected side effect for %s: %s", method, effect)
		}
		if !host_api.IsDataRouteMethod(method) || !host_api.RouteScopeForMethod(method).Namespaced {
			t.Fatalf("expected data route with namespaced scope for %s", method)
		}
	}
}

func TestProjectReorderRouteForwardsAnchor(t *testing.T) {
	reorderer := &recordingProjectReorderer{output: json.RawMessage(`{"projectIds":["project-2","project-1"]}`)}
	gateway := projectRouteGateway(t, HostAPIRouteDeps{ProjectReorderer: reorderer})
	result := gateway.Call(context.Background(), projectRouteCall(host_api.MethodProjectReorder, `{"projectId":"project-2","beforeProjectId":"project-1","requestId":"project-reorder-1"}`))
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success, got %s: %+v", result.Status, result.Error)
	}
	if !reorderer.called || reorderer.request.ProjectID != "project-2" || reorderer.request.BeforeProjectID != "project-1" || reorderer.request.RequestID != "project-reorder-1" {
		t.Fatalf("unexpected reorder request: %+v", reorderer.request)
	}
}

func TestProjectReorderRouteRequiresProjectID(t *testing.T) {
	reorderer := &recordingProjectReorderer{output: json.RawMessage(`{"projectIds":[]}`)}
	gateway := projectRouteGateway(t, HostAPIRouteDeps{ProjectReorderer: reorderer})
	result := gateway.Call(context.Background(), projectRouteCall(host_api.MethodProjectReorder, `{"beforeProjectId":"project-1"}`))
	if result.Status == host_api.StatusSuccess || reorderer.called {
		t.Fatalf("expected input rejection, got %s", result.Status)
	}
}

func TestConversationReorderRouteForwardsWorkspaceScope(t *testing.T) {
	reorderer := &recordingConversationReorderer{output: json.RawMessage(`{"conversationIds":["conversation-1"]}`)}
	gateway := projectRouteGateway(t, HostAPIRouteDeps{ConversationReorderer: reorderer})
	result := gateway.Call(context.Background(), projectRouteCall(host_api.MethodConversationReorder, `{"conversationId":"conversation-1","beforeConversationId":"conversation-0","projectId":"project-1","requestId":"conversation-reorder-1"}`))
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success, got %s: %+v", result.Status, result.Error)
	}
	if !reorderer.called || reorderer.request.ConversationID != "conversation-1" || reorderer.request.BeforeConversationID != "conversation-0" || reorderer.request.ProjectID != "project-1" {
		t.Fatalf("unexpected reorder request: %+v", reorderer.request)
	}
}

func TestProjectUpdateRouteForwardsOrder(t *testing.T) {
	updater := &recordingProjectUpdater{output: json.RawMessage(`{"projectId":"project-1"}`)}
	gateway := projectRouteGateway(t, HostAPIRouteDeps{ProjectUpdater: updater})
	result := gateway.Call(context.Background(), projectRouteCall(host_api.MethodProjectUpdate, `{"projectId":"project-1","order":2048,"requestId":"project-update-order"}`))
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success, got %s: %+v", result.Status, result.Error)
	}
	if updater.request.Order == nil || *updater.request.Order != 2048 {
		t.Fatalf("order not forwarded: %+v", updater.request)
	}
}

func TestProjectUpdateRouteRejectsNegativeOrder(t *testing.T) {
	updater := &recordingProjectUpdater{output: json.RawMessage(`{"projectId":"project-1"}`)}
	gateway := projectRouteGateway(t, HostAPIRouteDeps{ProjectUpdater: updater})
	result := gateway.Call(context.Background(), projectRouteCall(host_api.MethodProjectUpdate, `{"projectId":"project-1","order":-1,"requestId":"project-update-order"}`))
	if result.Status == host_api.StatusSuccess || updater.called {
		t.Fatalf("expected input rejection, got %s", result.Status)
	}
}
