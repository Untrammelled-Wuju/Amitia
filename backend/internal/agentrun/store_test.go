package agentrun

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/migration"
	"gorm.io/gorm"
)

func testService(t *testing.T) *Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := migration.ApplyBaseline(db); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(db)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func seedCoordination(t *testing.T, service *Service) {
	t.Helper()
	if err := service.SyncCoordination(context.Background(), Coordination{
		SpaceID: "space-a", ConversationID: "conversation-a", CoordinationID: "coordination-a",
		ParentInteractionID: "parent-a", ParentCharacterID: "role-a", Status: StatusRunning,
		Assignments: []Assignment{{
			AssignmentID: "assignment-a", ChildInteraction: "child-a", CharacterID: "role-a",
			WorkerRole: "coding_worker", Objective: "inspect", Status: StatusRunning,
		}},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRunsAreUpsertedAndListedByConversation(t *testing.T) {
	service := testService(t)
	seedCoordination(t, service)
	seedCoordination(t, service)
	runs, err := service.List(context.Background(), "space-a", "conversation-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 {
		t.Fatalf("expected parent and child runs, got %d", len(runs))
	}
	if runs[0].ID != RootRunID("coordination-a") || runs[1].ParentRunID != runs[0].ID {
		t.Fatalf("unexpected run tree: %+v", runs)
	}
}

func TestParentAndChildExchangeDurableMessages(t *testing.T) {
	service := testService(t)
	seedCoordination(t, service)
	rootID := RootRunID("coordination-a")
	childID := ChildRunID("assignment-a")
	if _, err := service.Send(context.Background(), SendInput{
		SpaceID: "space-a", SenderRunID: rootID, RecipientID: childID,
		Kind: MessageInstruction, Content: "check the tests first",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Send(context.Background(), SendInput{
		SpaceID: "space-a", SenderRunID: childID, RecipientID: rootID,
		Kind: MessageResult, Content: "tests passed",
	}); err != nil {
		t.Fatal(err)
	}
	inbox, err := service.Inbox(context.Background(), "space-a", rootID, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(inbox) != 1 || inbox[0].Content != "tests passed" || inbox[0].Status != MessageRead {
		t.Fatalf("unexpected root inbox: %+v", inbox)
	}
	all, err := service.Messages(context.Background(), "space-a", rootID)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("expected both directions, got %d", len(all))
	}
}

func TestUserMessageTargetsOnlyRunInSameSpace(t *testing.T) {
	service := testService(t)
	seedCoordination(t, service)
	childID := ChildRunID("assignment-a")
	message, err := service.SendFromUser(context.Background(), "space-a", childID, "continue with the API")
	if err != nil {
		t.Fatal(err)
	}
	if message.SenderRunID != UserRunID(childID) || message.Kind != MessageUser {
		t.Fatalf("unexpected user message: %+v", message)
	}
	if _, err := service.SendFromUser(context.Background(), "space-b", childID, "cross-space"); err == nil {
		t.Fatal("expected cross-space message rejection")
	}
}
