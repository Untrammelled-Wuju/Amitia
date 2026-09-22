package continuity

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func testRepository(t *testing.T) *Repository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "continuity.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	repo := NewRepository(db)
	if err := repo.InitSchema(); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	return repo
}

func TestResolverCreatesAndResumesAcrossConversations(t *testing.T) {
	repo := testRepository(t)
	resolver := NewResolver(repo)

	created, err := resolver.Resolve(context.Background(), ResolveInput{
		SpaceID:        "space-1",
		CharacterID:    "char-1",
		ConversationID: "conv-a",
		Message:        "我要把服务器迁移到新机器，并且完成部署和 DNS 切换",
	})
	if err != nil {
		t.Fatalf("create resolve: %v", err)
	}
	if created.Thread == nil || !created.Created {
		t.Fatalf("expected created thread, got %#v", created)
	}

	resumed, err := resolver.Resolve(context.Background(), ResolveInput{
		SpaceID:        "space-1",
		CharacterID:    "char-1",
		ConversationID: "conv-b",
		Message:        "继续服务器迁移那个",
	})
	if err != nil {
		t.Fatalf("resume resolve: %v", err)
	}
	if resumed.Thread == nil || resumed.Thread.ID != created.Thread.ID {
		t.Fatalf("expected same thread %q, got %#v", created.Thread.ID, resumed.Thread)
	}
}

func TestResolverDoesNotCreateForSmallTalk(t *testing.T) {
	repo := testRepository(t)
	resolver := NewResolver(repo)

	resolution, err := resolver.Resolve(context.Background(), ResolveInput{
		SpaceID:        "space-1",
		ConversationID: "conv-a",
		Message:        "早上好",
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolution.Thread != nil || resolution.Created {
		t.Fatalf("small talk must not create a persistent thread: %#v", resolution)
	}
}

func TestResolverCanSwitchThreadInsideSameConversation(t *testing.T) {
	repo := testRepository(t)
	resolver := NewResolver(repo)

	server, err := resolver.Resolve(context.Background(), ResolveInput{
		SpaceID:        "space-1",
		ConversationID: "conv-a",
		Message:        "我要迁移服务器并完成后端部署",
	})
	if err != nil || server.Thread == nil {
		t.Fatalf("create server thread: %v %#v", err, server)
	}

	study, err := resolver.Resolve(context.Background(), ResolveInput{
		SpaceID:        "space-1",
		ConversationID: "conv-a",
		Message:        "我要制定专升本学习计划并持续跟进每天进度",
	})
	if err != nil || study.Thread == nil {
		t.Fatalf("create study thread: %v %#v", err, study)
	}
	if study.Thread.ID == server.Thread.ID {
		t.Fatalf("expected a distinct thread when the conversation changes to an unrelated persistent goal")
	}

	back, err := resolver.Resolve(context.Background(), ResolveInput{
		SpaceID:        "space-1",
		ConversationID: "conv-a",
		Message:        "继续服务器迁移",
	})
	if err != nil {
		t.Fatalf("switch back: %v", err)
	}
	if back.Thread == nil || back.Thread.ID != server.Thread.ID {
		t.Fatalf("expected server thread %q, got %#v", server.Thread.ID, back.Thread)
	}
}

func TestResolverDoesNotGuessBareContinuationWithMultipleUnboundThreads(t *testing.T) {
	repo := testRepository(t)
	resolver := NewResolver(repo)
	for _, thread := range []*Thread{
		{SpaceID: "space-1", Title: "服务器迁移", Goal: "迁移服务器", Status: ThreadStatusActive},
		{SpaceID: "space-1", Title: "专升本学习", Goal: "准备专升本", Status: ThreadStatusActive},
	} {
		if err := repo.CreateThread(thread); err != nil {
			t.Fatalf("create seed thread: %v", err)
		}
	}

	resolution, err := resolver.Resolve(context.Background(), ResolveInput{
		SpaceID:        "space-1",
		ConversationID: "brand-new-conversation",
		Message:        "继续",
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolution.Thread != nil {
		t.Fatalf("ambiguous bare continuation must not be guessed, got %#v", resolution.Thread)
	}
}

func TestResolverSuppressesCreationForInternalOrProactiveInput(t *testing.T) {
	repo := testRepository(t)
	resolver := NewResolver(repo)

	resolution, err := resolver.Resolve(context.Background(), ResolveInput{
		SpaceID:        "space-1",
		ConversationID: "conv-proactive",
		Message:        "需要完成服务器迁移项目并等待 DNS 生效",
		SuppressCreate: true,
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolution.Thread != nil || resolution.Created {
		t.Fatalf("suppressed input must not create a persistent thread: %#v", resolution)
	}
}

func TestResolverResumesRecentWorkspaceThreadAcrossConversations(t *testing.T) {
	repo := testRepository(t)
	resolver := NewResolver(repo)

	created, err := resolver.Resolve(context.Background(), ResolveInput{
		SpaceID:        "space-1",
		ConversationID: "conv-a",
		WorkspaceID:    "workspace-1",
		Message:        "我要开发微信插件并持续完成扫码登录和消息收发",
	})
	if err != nil || created.Thread == nil {
		t.Fatalf("create workspace thread: %v %#v", err, created)
	}

	resumed, err := resolver.Resolve(context.Background(), ResolveInput{
		SpaceID:        "space-1",
		ConversationID: "conv-b",
		WorkspaceID:    "workspace-1",
		Message:        "继续",
	})
	if err != nil {
		t.Fatalf("resume workspace thread: %v", err)
	}
	if resumed.Thread == nil || resumed.Thread.ID != created.Thread.ID {
		t.Fatalf("expected workspace thread %q, got %#v", created.Thread.ID, resumed.Thread)
	}
}

func TestResolverDoesNotCreateThreadForInformationalProjectQuestion(t *testing.T) {
	repo := testRepository(t)
	resolver := NewResolver(repo)

	for _, message := range []string{
		"介绍一下这个项目的整体架构",
		"服务器迁移通常应该怎么做",
		"对比一下部署和迁移有什么区别",
	} {
		resolution, err := resolver.Resolve(context.Background(), ResolveInput{
			SpaceID:        "space-1",
			ConversationID: "conv-info",
			Message:        message,
		})
		if err != nil {
			t.Fatalf("resolve %q: %v", message, err)
		}
		if resolution.Thread != nil || resolution.Created {
			t.Fatalf("informational query %q must not create a persistent thread: %#v", message, resolution)
		}
	}
}
