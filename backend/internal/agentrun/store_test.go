package agentrun

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

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

func TestListChildrenReturnsOnlyDirectChildren(t *testing.T) {
	service := testService(t)
	seedCoordination(t, service)
	children, err := service.ListChildren(context.Background(), "space-a", RootRunID("coordination-a"))
	if err != nil {
		t.Fatal(err)
	}
	if len(children) != 1 || children[0].ID != ChildRunID("assignment-a") ||
		children[0].ParentRunID != RootRunID("coordination-a") {
		t.Fatalf("unexpected direct children: %+v", children)
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

func TestClaimLeaseAndAcknowledgePreventsRedelivery(t *testing.T) {
	service := testService(t)
	seedCoordination(t, service)
	rootID := RootRunID("coordination-a")
	childID := ChildRunID("assignment-a")
	message, err := service.Send(context.Background(), SendInput{
		SpaceID: "space-a", SenderRunID: rootID, RecipientID: childID,
		Kind: MessageInstruction, Content: "inspect the change",
	})
	if err != nil {
		t.Fatal(err)
	}

	claim, err := service.Claim(context.Background(), "space-a", childID, "worker-a", time.Minute, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(claim.Messages) != 1 || claim.Messages[0].ID != message.ID || claim.LeaseToken == "" {
		t.Fatalf("unexpected claim: %+v", claim)
	}
	if claim.Messages[0].DeliveryAttempts != 1 || claim.Messages[0].DeliveryState != DeliveryLeased {
		t.Fatalf("unexpected leased message: %+v", claim.Messages[0])
	}
	empty, err := service.Claim(context.Background(), "space-a", childID, "worker-b", time.Minute, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(empty.Messages) != 0 {
		t.Fatalf("active lease was claimed twice: %+v", empty.Messages)
	}
	if _, err := service.Acknowledge(context.Background(), "space-a", childID, "stale-token", []string{message.ID}); !errors.Is(err, ErrLeaseNotHeld) {
		t.Fatalf("expected lease rejection, got %v", err)
	}
	count, err := service.Acknowledge(context.Background(), "space-a", childID, claim.LeaseToken, []string{message.ID})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("acknowledged=%d, want 1", count)
	}
	afterAck, err := service.Claim(context.Background(), "space-a", childID, "worker-c", time.Minute, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(afterAck.Messages) != 0 {
		t.Fatalf("acknowledged message was redelivered: %+v", afterAck.Messages)
	}
}

func TestExpiredLeaseRedeliversAndRejectsStaleAcknowledgement(t *testing.T) {
	service := testService(t)
	seedCoordination(t, service)
	rootID := RootRunID("coordination-a")
	childID := ChildRunID("assignment-a")
	message, err := service.Send(context.Background(), SendInput{
		SpaceID: "space-a", SenderRunID: rootID, RecipientID: childID,
		Kind: MessageInstruction, Content: "retry safely",
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.Claim(context.Background(), "space-a", childID, "worker-a", time.Second, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Messages) != 1 {
		t.Fatalf("first claim=%+v", first.Messages)
	}
	expired := time.Now().UTC().Add(-time.Second)
	if err := service.db.Model(&Message{}).Where("id = ?", message.ID).Update("lease_expires_at", expired).Error; err != nil {
		t.Fatal(err)
	}
	second, err := service.Claim(context.Background(), "space-a", childID, "worker-b", time.Minute, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Messages) != 1 || second.LeaseToken == first.LeaseToken {
		t.Fatalf("expired lease was not redelivered: %+v", second)
	}
	if second.Messages[0].DeliveryAttempts != 2 {
		t.Fatalf("delivery attempts=%d, want 2", second.Messages[0].DeliveryAttempts)
	}
	if _, err := service.Acknowledge(context.Background(), "space-a", childID, first.LeaseToken, []string{message.ID}); !errors.Is(err, ErrLeaseNotHeld) {
		t.Fatalf("expected stale lease rejection, got %v", err)
	}
	if _, err := service.Acknowledge(context.Background(), "space-a", childID, second.LeaseToken, []string{message.ID}); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentClaimsReturnMessageAtMostOnce(t *testing.T) {
	service := testService(t)
	seedCoordination(t, service)
	rootID := RootRunID("coordination-a")
	childID := ChildRunID("assignment-a")
	if _, err := service.Send(context.Background(), SendInput{
		SpaceID: "space-a", SenderRunID: rootID, RecipientID: childID,
		Kind: MessageInstruction, Content: "single consumer",
	}); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := service.db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	var wg sync.WaitGroup
	var mu sync.Mutex
	total := 0
	errs := make([]error, 0)
	for _, owner := range []string{"worker-a", "worker-b"} {
		wg.Add(1)
		go func(owner string) {
			defer wg.Done()
			claim, claimErr := service.Claim(context.Background(), "space-a", childID, owner, time.Minute, 1)
			mu.Lock()
			defer mu.Unlock()
			total += len(claim.Messages)
			if claimErr != nil {
				errs = append(errs, claimErr)
			}
		}(owner)
	}
	wg.Wait()
	if len(errs) != 0 {
		t.Fatalf("concurrent claim errors: %v", errs)
	}
	if total != 1 {
		t.Fatalf("concurrent claims delivered %d messages, want 1", total)
	}
}
