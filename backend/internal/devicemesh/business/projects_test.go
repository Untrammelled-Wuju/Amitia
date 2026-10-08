package business

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestOwnedProjectCreationRenameDeleteAndRetryKeepOriginalOwner(t *testing.T) {
	for _, coordinated := range []bool{false, true} {
		t.Run(map[bool]string{false: "device", true: "core"}[coordinated], func(t *testing.T) {
			engine, db, service, _ := engineHarness(t)
			if coordinated {
				if _, err := service.ChangeMode(t.Context(), "core", "a", 1, true, "role"); err != nil {
					t.Fatal(err)
				}
			}
			request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "new-project"}
			query, err := engine.Query(t.Context(), request, coordination.DataQuery{ResourceKind: "project", Management: true})
			if err != nil {
				t.Fatal(err)
			}
			request.ExpectedScope = &query.Scope
			created, err := engine.CreateProject(t.Context(), request, "我的项目")
			if err != nil || !created.Saved || created.Acknowledgement.OwnerID != query.Scope.ResourceOwnerID {
				t.Fatalf("creation=%+v err=%v", created, err)
			}
			replayed, err := engine.CreateProject(t.Context(), request, "我的项目")
			if err != nil || replayed.Project != created.Project {
				t.Fatalf("retry=%+v err=%v", replayed, err)
			}
			if _, err := engine.CreateProject(t.Context(), request, "另一个项目"); !errors.Is(err, coordination.ErrRequestConflict) {
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

type changingProjectPort struct {
	testDataPort
	projectID string
	changed   bool
}

func (p *changingProjectPort) Commit(ctx context.Context, commit coordination.Commit) (coordination.Acknowledgement, error) {
	if !p.changed && len(commit.Dependencies) > 0 {
		p.changed = true
		scope := commit.Scope
		scope.RequestID = "concurrent-project-change"
		if _, err := p.testDataPort.Commit(ctx, coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{{Kind: "project", ID: p.projectID, RoleID: scope.RoleID, ExpectedRevision: 1, Body: body(Project{ID: p.projectID, Title: "同时改名"})}}}); err != nil {
			return coordination.Acknowledgement{}, err
		}
	}
	return p.testDataPort.Commit(ctx, commit)
}

func TestConversationProjectAssignmentChecksOwnerAndProjectRevision(t *testing.T) {
	engine, db, _, _ := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "project-create"}
	query, err := engine.Query(t.Context(), request, coordination.DataQuery{ResourceKind: "project", Management: true})
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedScope = &query.Scope
	project, err := engine.CreateProject(t.Context(), request, "分组")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Run(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "conversation-create", ConversationID: "conversation", Message: "hello"}); err != nil {
		t.Fatal(err)
	}
	edit := EditRequest{ExpectedScope: &query.Scope, RoleID: "role", RequestID: "move-missing-project", Kind: "conversation", ID: "conversation", ExpectedRevision: 1, Changes: map[string]json.RawMessage{"projectId": body("missing")}}
	if _, err := engine.Edit(t.Context(), request, edit); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("missing project accepted: %v", err)
	}
	edit.RequestID = "move-project"
	edit.Changes["projectId"] = body(project.Project.ID)
	engine.data = &changingProjectPort{testDataPort: engine.data.(testDataPort), projectID: project.Project.ID}
	if _, err := engine.Edit(t.Context(), request, edit); !errors.Is(err, coordination.ErrResourceVersion) {
		t.Fatalf("changed project accepted: %v", err)
	}
	row, err := coordination.NewOwnershipStore(db, "a").Get(t.Context(), "conversation", "conversation")
	if err != nil || row.Revision != 1 {
		t.Fatal("failed assignment changed conversation")
	}
	edit.RequestID = "move-refreshed-project"
	ack, err := engine.Edit(t.Context(), request, edit)
	if err != nil || ack.Versions["conversation/conversation"] != 2 {
		t.Fatalf("fresh assignment rejected: %+v %v", ack, err)
	}
}

func TestOwnedProjectRejectsMissingOrChangedScopeAndInvalidTitle(t *testing.T) {
	engine, _, service, _ := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "create"}
	if _, err := engine.CreateProject(t.Context(), request, "valid"); err == nil {
		t.Fatal("missing original scope accepted")
	}
	query, err := engine.Query(t.Context(), request, coordination.DataQuery{ResourceKind: "project", Management: true})
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedScope = &query.Scope
	for _, title := range []string{"", "bad\nname", "bad\x00name", string([]byte{255})} {
		if _, err := engine.CreateProject(t.Context(), request, title); err == nil {
			t.Fatal("invalid title accepted")
		}
	}
	if _, err := service.ChangeMode(t.Context(), "core", "a", 1, true, "role"); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.CreateProject(t.Context(), request, "valid"); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("old owner accepted: %v", err)
	}
}
