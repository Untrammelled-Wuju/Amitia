package chat

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/system/dataportability"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

func newProjectTestService(t *testing.T) *service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "app.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&Project{}, &Conversation{}, &Message{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE workspace_mounts (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		kind TEXT NOT NULL,
		local_root TEXT,
		native_grant_id TEXT,
		backend_config_json TEXT,
		credential_ref TEXT,
		read_only INTEGER NOT NULL DEFAULT 0,
		enabled INTEGER NOT NULL DEFAULT 1,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		last_used_at DATETIME
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO workspace_mounts (id, name, kind, enabled, created_at, updated_at)
		VALUES ('workspace-1', 'Amitia', 'local', 1, datetime('now'), datetime('now'))`).Error; err != nil {
		t.Fatal(err)
	}
	ctx := app.NewAppContext(db, nil)
	return &service{repo: NewRepository(ctx), db: db}
}

func TestProjectConversationLifecycle(t *testing.T) {
	svc := newProjectTestService(t)
	project, err := svc.CreateProjectForSpace(&CreateProjectRequest{
		Name:        "Amitia",
		WorkspaceID: "workspace-1",
		RootURI:     "amitia://workspace/@workspace-1/",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	conversation, err := svc.CreateProjectConversationForSpace(project.ID, &CreateConversationRequest{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if conversation.ProjectID != project.ID {
		t.Fatalf("project id = %q, want %q", conversation.ProjectID, project.ID)
	}
	reused, err := svc.CreateProjectConversationForSpace(project.ID, &CreateConversationRequest{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if reused.ID != conversation.ID {
		t.Fatalf("empty project conversation was not reused: %q != %q", reused.ID, conversation.ID)
	}
	sidebar, err := svc.ListConversationSidebarForSpace("", 20, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(sidebar.Projects) != 1 || len(sidebar.Projects[0].Conversations) != 1 {
		t.Fatalf("unexpected sidebar: %#v", sidebar)
	}
	if _, err := svc.MoveConversationToProjectForSpace(conversation.ID, "", ""); err != nil {
		t.Fatal(err)
	}
	sidebar, err = svc.ListConversationSidebarForSpace("", 20, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(sidebar.Recent) != 1 || len(sidebar.Projects[0].Conversations) != 0 {
		t.Fatalf("conversation did not move to recent: %#v", sidebar)
	}
	if err := svc.DeleteProjectForSpace(project.ID, ""); err != nil {
		t.Fatal(err)
	}
}

func TestRecentEmptyConversationIsReused(t *testing.T) {
	svc := newProjectTestService(t)
	first, err := svc.CreateConversationForSpace(&CreateConversationRequest{Channel: "web"}, "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.CreateConversationForSpace(&CreateConversationRequest{Channel: "web"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("empty recent conversation was not reused: %q != %q", first.ID, second.ID)
	}
}

func TestBackupPlanExcludesProjectConversations(t *testing.T) {
	svc := newProjectTestService(t)
	project, err := svc.CreateProjectForSpace(&CreateProjectRequest{
		Name:        "Amitia",
		WorkspaceID: "workspace-1",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	projectConversation, err := svc.CreateProjectConversationForSpace(project.ID, &CreateConversationRequest{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.repo.CreateMessage(&Message{ID: "project-message", ConversationID: projectConversation.ID, Role: "user", Content: "project"}); err != nil {
		t.Fatal(err)
	}
	recentConversation, err := svc.CreateConversationForSpace(&CreateConversationRequest{Channel: "web"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.repo.CreateMessage(&Message{ID: "recent-message", ConversationID: recentConversation.ID, Role: "user", Content: "recent"}); err != nil {
		t.Fatal(err)
	}
	plans, err := NewChatBackupContributor(svc.db).Plan(context.Background(), dataportability.BackupRequest{})
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int64{}
	for _, plan := range plans {
		counts[plan.ID] = plan.ItemCount
	}
	if counts[ComponentIDChatConversations] != 1 {
		t.Fatalf("conversation backup count = %d, want 1", counts[ComponentIDChatConversations])
	}
	if counts[ComponentIDChatMessages] != 1 {
		t.Fatalf("message backup count = %d, want 1", counts[ComponentIDChatMessages])
	}
}

func TestChannelConversationsStayOutOfRecentSidebar(t *testing.T) {
	svc := newProjectTestService(t)
	if err := svc.db.Create(&Conversation{
		ID:        "channel-conversation",
		SpaceID:   normalizeConversationOwner(""),
		Title:     "个人微信",
		Channel:   "wechat_personal",
		Source:    "channel",
		ProjectID: "",
	}).Error; err != nil {
		t.Fatal(err)
	}
	sidebar, err := svc.ListConversationSidebarForSpace("", 5, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(sidebar.Recent) != 0 {
		t.Fatalf("channel conversation must stay out of recent sidebar: %#v", sidebar.Recent)
	}
	channels, err := svc.ListChannelConversationsForSpace("", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) != 1 || channels[0].ID != "channel-conversation" {
		t.Fatalf("channel conversation missing from channel entry: %#v", channels)
	}
}

func TestSidebarConversationPinArchiveAndProjectPin(t *testing.T) {
	svc := newProjectTestService(t)
	conversation, err := svc.CreateConversationForSpace(&CreateConversationRequest{Channel: "web"}, "")
	if err != nil {
		t.Fatal(err)
	}
	pinned := true
	if _, err := svc.UpdateConversationSidebarStateForSpace(conversation.ID, &pinned, nil, ""); err != nil {
		t.Fatal(err)
	}
	sidebar, err := svc.ListConversationSidebarForSpace("", 20, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(sidebar.Pinned) != 1 || sidebar.Pinned[0].ID != conversation.ID {
		t.Fatalf("conversation was not pinned: %#v", sidebar)
	}
	if len(sidebar.Recent) != 0 {
		t.Fatalf("pinned conversation must leave recent: %#v", sidebar.Recent)
	}
	archived := true
	if _, err := svc.UpdateConversationSidebarStateForSpace(conversation.ID, nil, &archived, ""); err != nil {
		t.Fatal(err)
	}
	sidebar, err = svc.ListConversationSidebarForSpace("", 20, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(sidebar.Pinned) != 0 || len(sidebar.Recent) != 0 {
		t.Fatalf("archived conversation must be hidden: %#v", sidebar)
	}

	project, err := svc.CreateProjectForSpace(&CreateProjectRequest{
		Name:        "Amitia",
		WorkspaceID: "workspace-1",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	projectPinned := true
	project, err = svc.UpdateProjectForSpace(project.ID, &UpdateProjectRequest{Pinned: &projectPinned}, "")
	if err != nil {
		t.Fatal(err)
	}
	if project.PinnedAt == nil || *project.PinnedAt == "" {
		t.Fatalf("project was not pinned: %#v", project)
	}
}

func TestProjectOpenTargetUsesLocalWorkspaceRoot(t *testing.T) {
	svc := newProjectTestService(t)
	root := t.TempDir()
	if err := svc.db.Exec("UPDATE workspace_mounts SET local_root = ? WHERE id = 'workspace-1'", root).Error; err != nil {
		t.Fatal(err)
	}
	project, err := svc.CreateProjectForSpace(&CreateProjectRequest{
		Name:        "Amitia",
		WorkspaceID: "workspace-1",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	target, err := svc.GetProjectOpenTargetForSpace(project.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if target.Kind != "local" || target.Path != root {
		t.Fatalf("unexpected open target: %#v", target)
	}
}

func TestArchivedConversationListOnlyReturnsArchived(t *testing.T) {
	svc := newProjectTestService(t)
	archivedConversation, err := svc.CreateConversationForSpace(&CreateConversationRequest{Channel: "web"}, "")
	if err != nil {
		t.Fatal(err)
	}
	archived := true
	if _, err := svc.UpdateConversationSidebarStateForSpace(archivedConversation.ID, nil, &archived, ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.repo.CreateMessage(&Message{
		ID:             "archived-search-message",
		ConversationID: archivedConversation.ID,
		Role:           "user",
		Content:        "needle in archived content",
	}); err != nil {
		t.Fatal(err)
	}
	recentConversation, err := svc.CreateConversationForSpace(&CreateConversationRequest{Channel: "web", Title: "普通对话"}, "")
	if err != nil {
		t.Fatal(err)
	}
	response, err := svc.ListConversationsForSpace(ConversationQuery{
		ArchivedOnly: true,
		Page:         1,
		PageSize:     20,
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Items) != 1 || response.Items[0].ID != archivedConversation.ID {
		t.Fatalf("archived list mismatch: %#v", response.Items)
	}
	if response.Items[0].ID == recentConversation.ID {
		t.Fatal("ordinary conversation appeared in archived list")
	}
	searchResponse, err := svc.ListConversationsForSpace(ConversationQuery{
		ArchivedOnly: true,
		Keyword:      "needle",
		Page:         1,
		PageSize:     20,
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(searchResponse.Items) != 1 || searchResponse.Items[0].ID != archivedConversation.ID {
		t.Fatalf("archived content search mismatch: %#v", searchResponse.Items)
	}
}
