package business

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func projectInput(title, deviceID string) CreateProjectInput {
	workspaceID := "workspace-" + title
	return CreateProjectInput{
		Title: title,
		Workspaces: []ProjectWorkspace{{
			ID:       workspaceID,
			DeviceID: deviceID,
			Name:     title,
			Kind:     "local",
			RootURI:  "amitia://workspace/@" + workspaceID + "/",
		}},
		PrimaryWorkspaceID: workspaceID,
	}
}

func TestOwnedProjectCreationRenameDeleteAndRetryKeepOriginalOwner(t *testing.T) {
	for _, coordinated := range []bool{false, true} {
		t.Run(map[bool]string{false: "device", true: "core"}[coordinated], func(t *testing.T) {
			engine, db, service, _ := engineHarness(t)
			if coordinated {
				if _, err := service.InitializeCoordinated(t.Context(), "core", "a", 1, "role"); err != nil {
					t.Fatal(err)
				}
			}
			request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "new-project"}
			query, err := engine.Query(t.Context(), request, coordination.DataQuery{ResourceKind: "project", Management: true})
			if err != nil {
				t.Fatal(err)
			}
			request.ExpectedScope = &query.Scope
			created, err := engine.CreateProject(t.Context(), request, projectInput("我的项目", query.Scope.TargetDeviceID))
			if err != nil || !created.Saved || created.Acknowledgement.OwnerID != query.Scope.ResourceOwnerID {
				t.Fatalf("creation=%+v err=%v", created, err)
			}
			replayed, err := engine.CreateProject(t.Context(), request, projectInput("我的项目", query.Scope.TargetDeviceID))
			if err != nil || !reflect.DeepEqual(replayed.Project, created.Project) {
				t.Fatalf("retry=%+v err=%v", replayed, err)
			}
			if _, err := engine.CreateProject(t.Context(), request, projectInput("另一个项目", query.Scope.TargetDeviceID)); !errors.Is(err, coordination.ErrRequestConflict) {
				t.Fatalf("changed retry: %v", err)
			}
			owner := query.Scope.ResourceOwnerID
			other := "core"
			if coordinated {
				other = "a"
			}
			rows, err := coordination.NewOwnershipStore(db, other).List(t.Context(), "project", "role", false)
			if err != nil || len(rows) != 0 {
				t.Fatal("project copied to other owner")
			}
			rename := EditRequest{ExpectedScope: &query.Scope, RoleID: "role", RequestID: "rename-project", Kind: "project", ID: created.Project.ID, ExpectedRevision: 1, Changes: map[string]json.RawMessage{"title": json.RawMessage(`"新标题"`)}}
			ack, err := engine.Edit(t.Context(), request, rename)
			if err != nil || ack.OwnerID != owner || ack.Versions["project/"+created.Project.ID] != 2 {
				t.Fatalf("rename=%+v %v", ack, err)
			}
			if _, err := engine.Edit(t.Context(), request, EditRequest{ExpectedScope: &query.Scope, RoleID: "role", RequestID: "foreign-field", Kind: "project", ID: created.Project.ID, ExpectedRevision: 2, Changes: map[string]json.RawMessage{"path": json.RawMessage(`"C:/private"`)}}); err == nil {
				t.Fatal("project granted filesystem access")
			}
			ack, err = engine.Edit(t.Context(), request, EditRequest{ExpectedScope: &query.Scope, RoleID: "role", RequestID: "delete-project", Kind: "project", ID: created.Project.ID, ExpectedRevision: 2, Deleted: true})
			if err != nil || ack.Versions["project/"+created.Project.ID] != 3 {
				t.Fatalf("delete=%+v %v", ack, err)
			}
			rows, err = coordination.NewOwnershipStore(db, owner).List(t.Context(), "project", "role", false)
			if err != nil || len(rows) != 0 {
				t.Fatal("deleted project remained visible")
			}
		})
	}
}

func TestConversationProjectAssignmentIsRejected(t *testing.T) {
	engine, db, _, _ := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "project-create"}
	query, err := engine.Query(t.Context(), request, coordination.DataQuery{ResourceKind: "project", Management: true})
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedScope = &query.Scope
	project, err := engine.CreateProject(t.Context(), request, projectInput("分组", query.Scope.TargetDeviceID))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Run(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "conversation-create", ConversationID: "conversation", Message: "hello"}); err != nil {
		t.Fatal(err)
	}
	edit := EditRequest{ExpectedScope: &query.Scope, RoleID: "role", RequestID: "move-project", Kind: "conversation", ID: "conversation", ExpectedRevision: 1, Changes: map[string]json.RawMessage{"projectId": body(project.Project.ID)}}
	if _, err := engine.Edit(t.Context(), request, edit); err != nil {
		t.Fatal(err)
	}
	row, err := coordination.NewOwnershipStore(db, "a").Get(t.Context(), "conversation", "conversation")
	if err != nil || row.Revision != 2 {
		t.Fatal("project assignment did not change conversation")
	}
	var document struct {
		ProjectID string `json:"projectId"`
	}
	if json.Unmarshal(row.Body, &document) != nil || document.ProjectID != project.Project.ID {
		t.Fatal("conversation project assignment missing")
	}
}

func TestOwnedProjectRejectsMissingOrChangedScopeAndInvalidTitle(t *testing.T) {
	engine, _, service, _ := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "create"}
	if _, err := engine.CreateProject(t.Context(), request, projectInput("valid", "a")); err == nil {
		t.Fatal("missing original scope accepted")
	}
	query, err := engine.Query(t.Context(), request, coordination.DataQuery{ResourceKind: "project", Management: true})
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedScope = &query.Scope
	for _, title := range []string{"", "bad\nname", "bad\x00name", string([]byte{255})} {
		if _, err := engine.CreateProject(t.Context(), request, projectInput(title, query.Scope.TargetDeviceID)); err == nil {
			t.Fatal("invalid title accepted")
		}
	}
	if _, err := service.InitializeCoordinated(t.Context(), "core", "a", 1, "role"); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.CreateProject(t.Context(), request, projectInput("valid", query.Scope.TargetDeviceID)); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("old owner accepted: %v", err)
	}
}

func TestOwnedProjectCanExistWithoutFolders(t *testing.T) {
	engine, _, _, _ := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "empty-project"}
	query, err := engine.Query(t.Context(), request, coordination.DataQuery{ResourceKind: "project", Management: true})
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedScope = &query.Scope
	created, err := engine.CreateProject(t.Context(), request, CreateProjectInput{Title: "无文件夹项目"})
	if err != nil {
		t.Fatal(err)
	}
	if len(created.Project.Workspaces) != 0 || created.Project.PrimaryWorkspaceID != "" {
		t.Fatalf("unexpected folders: %+v", created.Project)
	}
	run := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "empty-project-chat", RequestID: "empty-project-message", ProjectID: created.Project.ID, Message: "hello"}
	response, err := engine.Run(t.Context(), run)
	if err != nil || !response.Saved {
		t.Fatalf("response=%+v err=%v", response, err)
	}
}

func TestOwnedProjectsCanReuseTheSameWorkspace(t *testing.T) {
	engine, _, _, _ := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "shared-workspace-project-one"}
	query, err := engine.Query(t.Context(), request, coordination.DataQuery{ResourceKind: "project", Management: true})
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedScope = &query.Scope
	workspace := ProjectWorkspace{
		ID:       "workspace-downloads",
		DeviceID: query.Scope.TargetDeviceID,
		Name:     "Downloads",
		Kind:     "local",
		RootURI:  "amitia://workspace/@workspace-downloads/",
	}
	first, err := engine.CreateProject(t.Context(), request, CreateProjectInput{Title: "项目一", Workspaces: []ProjectWorkspace{workspace}, PrimaryWorkspaceID: workspace.ID})
	if err != nil {
		t.Fatal(err)
	}
	request.RequestID = "shared-workspace-project-two"
	second, err := engine.CreateProject(t.Context(), request, CreateProjectInput{Title: "项目二", Workspaces: []ProjectWorkspace{workspace}, PrimaryWorkspaceID: workspace.ID})
	if err != nil {
		t.Fatal(err)
	}
	if first.Project.ID == second.Project.ID || !reflect.DeepEqual(first.Project.Workspaces, second.Project.Workspaces) {
		t.Fatalf("first=%+v second=%+v", first.Project, second.Project)
	}
}

func TestOwnedProjectEditCanAddWorkspaceUsedByAnotherProject(t *testing.T) {
	engine, db, _, _ := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "workspace-edit-project-one"}
	query, err := engine.Query(t.Context(), request, coordination.DataQuery{ResourceKind: "project", Management: true})
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedScope = &query.Scope
	shared := ProjectWorkspace{ID: "workspace-shared", DeviceID: query.Scope.TargetDeviceID, Name: "Shared", Kind: "local", RootURI: "amitia://workspace/@workspace-shared/"}
	other := ProjectWorkspace{ID: "workspace-other", DeviceID: query.Scope.TargetDeviceID, Name: "Other", Kind: "local", RootURI: "amitia://workspace/@workspace-other/"}
	if _, err := engine.CreateProject(t.Context(), request, CreateProjectInput{Title: "项目一", Workspaces: []ProjectWorkspace{shared}, PrimaryWorkspaceID: shared.ID}); err != nil {
		t.Fatal(err)
	}
	request.RequestID = "workspace-edit-project-two"
	second, err := engine.CreateProject(t.Context(), request, CreateProjectInput{Title: "项目二", Workspaces: []ProjectWorkspace{other}, PrimaryWorkspaceID: other.ID})
	if err != nil {
		t.Fatal(err)
	}
	workspaces := []ProjectWorkspace{other, shared}
	ack, err := engine.Edit(t.Context(), request, EditRequest{
		ExpectedScope:    &query.Scope,
		RoleID:           "role",
		RequestID:        "add-shared-workspace",
		Kind:             "project",
		ID:               second.Project.ID,
		ExpectedRevision: 1,
		Changes:          map[string]json.RawMessage{"workspaces": body(workspaces)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ack.Versions["project/"+second.Project.ID] != 2 {
		t.Fatalf("ack=%+v", ack)
	}
	row, err := coordination.NewOwnershipStore(db, query.Scope.ResourceOwnerID).Get(t.Context(), "project", second.Project.ID)
	if err != nil {
		t.Fatal(err)
	}
	var saved Project
	if json.Unmarshal(row.Body, &saved) != nil || !reflect.DeepEqual(saved.Workspaces, workspaces) || saved.PrimaryWorkspaceID != other.ID {
		t.Fatalf("project=%+v", saved)
	}
}

func TestConversationRunUsesProjectMultiWorkspaceSnapshot(t *testing.T) {
	engine, db, _, model := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "list-project"}
	query, err := engine.Query(t.Context(), request, coordination.DataQuery{ResourceKind: "project", Management: true})
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedScope = &query.Scope
	created, err := engine.CreateProject(t.Context(), request, CreateProjectInput{
		Title: "多根项目",
		Workspaces: []ProjectWorkspace{
			{ID: "workspace-a", DeviceID: query.Scope.TargetDeviceID, Name: "A", Kind: "local", RootURI: "amitia://workspace/@workspace-a/"},
			{ID: "workspace-b", DeviceID: query.Scope.TargetDeviceID, Name: "B", Kind: "local", RootURI: "amitia://workspace/@workspace-b/"},
		},
		PrimaryWorkspaceID: "workspace-b",
	})
	if err != nil {
		t.Fatal(err)
	}
	var captured Inference
	model.generate = func(_ context.Context, inference Inference) (Generation, error) {
		captured = inference
		return Generation{Text: "ok"}, nil
	}
	if _, err := engine.Run(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "project-chat", ConversationID: "project-conversation", ProjectID: created.Project.ID, Message: "继续"}); err != nil {
		t.Fatal(err)
	}
	if captured.ProjectID != created.Project.ID || captured.PrimaryWorkspaceID != "workspace-b" || len(captured.Workspaces) != 2 {
		t.Fatalf("project workspace snapshot=%+v", captured)
	}
	row, err := coordination.NewOwnershipStore(db, query.Scope.ResourceOwnerID).Get(t.Context(), "conversation", "project-conversation")
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		ProjectID string `json:"projectId"`
	}
	if json.Unmarshal(row.Body, &document) != nil || document.ProjectID != created.Project.ID {
		t.Fatal("conversation was not persisted into the project")
	}
	if _, err := engine.Edit(t.Context(), request, EditRequest{ExpectedScope: &query.Scope, RoleID: "role", RequestID: "hide-project", Kind: "project", ID: created.Project.ID, ExpectedRevision: 1, Changes: map[string]json.RawMessage{"hidden": body(true)}}); err != nil {
		t.Fatal(err)
	}
	projectRow, err := coordination.NewOwnershipStore(db, query.Scope.ResourceOwnerID).Get(t.Context(), "project", created.Project.ID)
	if err != nil || projectRow.Deleted {
		t.Fatal("hidden project was deleted")
	}
	var hiddenProject Project
	if json.Unmarshal(projectRow.Body, &hiddenProject) != nil || hiddenProject.HiddenAt == nil {
		t.Fatal("hidden project state was not retained")
	}
	if _, err := engine.Run(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "hidden-chat", ConversationID: "hidden-conversation", ProjectID: created.Project.ID, Message: "不能继续"}); err == nil {
		t.Fatal("hidden project accepted a new conversation")
	}
}
