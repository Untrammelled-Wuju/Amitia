package feedback

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func feedbackTestRepository(t *testing.T) *ScopedRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "feedback.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&MessageFeedback{}); err != nil {
		t.Fatal(err)
	}
	return NewScopedRepository(db)
}

func TestScopedFeedbackCreateListStatsAndDelete(t *testing.T) {
	repo := feedbackTestRepository(t)
	scope := Scope{SpaceID: "space", OwnerID: "owner", CoreID: "core", RoleID: "role"}
	item := MessageFeedback{ConversationID: "chat", MessageID: "reply", RequestID: "request-1", FeedbackType: "good", Reason: "useful"}
	created, inserted, err := repo.Create(t.Context(), scope, item)
	if err != nil || !inserted || created.ID < 1 {
		t.Fatalf("created=%+v inserted=%v err=%v", created, inserted, err)
	}
	replayed, inserted, err := repo.Create(t.Context(), scope, item)
	if err != nil || inserted || replayed.ID != created.ID {
		t.Fatalf("replayed=%+v inserted=%v err=%v", replayed, inserted, err)
	}
	item.Reason = "changed"
	if _, _, err := repo.Create(t.Context(), scope, item); !errors.Is(err, ErrRequestConflict) {
		t.Fatalf("conflict err=%v", err)
	}
	items, err := repo.ListMessage(t.Context(), scope, "reply")
	if err != nil || len(items) != 1 || items[0].MessageID != "reply" {
		t.Fatalf("items=%+v err=%v", items, err)
	}
	stats, err := repo.Stats(t.Context(), scope, []string{"reply"})
	if err != nil || stats["total"] != 1 {
		t.Fatalf("stats=%+v err=%v", stats, err)
	}
	deleted, err := repo.Delete(t.Context(), scope, created.ID)
	if err != nil || deleted.Revision != 2 {
		t.Fatalf("deleted=%+v err=%v", deleted, err)
	}
	after, err := repo.ListMessage(t.Context(), scope, "reply")
	if err != nil || len(after) != 0 {
		t.Fatalf("after=%+v err=%v", after, err)
	}
}

func TestScopedFeedbackPartitionsByOwnerAndRole(t *testing.T) {
	repo := feedbackTestRepository(t)
	base := Scope{SpaceID: "space", OwnerID: "owner-a", CoreID: "core", RoleID: "role-a"}
	item := MessageFeedback{ConversationID: "chat", MessageID: "reply", RequestID: "request", FeedbackType: "other", Reason: "detail"}
	if _, _, err := repo.Create(t.Context(), base, item); err != nil {
		t.Fatal(err)
	}
	item.RequestID = "request-b"
	if _, _, err := repo.Create(t.Context(), Scope{SpaceID: "space", OwnerID: "owner-b", CoreID: "core", RoleID: "role-b"}, item); err != nil {
		t.Fatal(err)
	}
	stats, err := repo.Stats(t.Context(), base, []string{"reply"})
	if err != nil || stats["total"] != 1 {
		t.Fatalf("stats=%+v err=%v", stats, err)
	}
}
