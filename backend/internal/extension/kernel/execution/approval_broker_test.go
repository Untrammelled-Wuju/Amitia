package execution

import (
	"context"
	"testing"
	"time"
)

func TestApprovalBrokerResolveAndList(t *testing.T) {
	broker := NewApprovalBroker()
	var resolved ApprovalRequest
	broker.SetObservers(nil, func(request ApprovalRequest) error {
		resolved = request
		return nil
	})
	result := make(chan bool, 1)
	go func() {
		approved, err := broker.Await(context.Background(), ApprovalRequest{
			SpaceID:        "space-1",
			ConversationID: "conv-1",
			ToolName:       "write_file",
		}, time.Minute)
		if err != nil {
			t.Errorf("Await returned error: %v", err)
		}
		result <- approved
	}()

	var pending []ApprovalRequest
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		pending = broker.List("space-1", "conv-1")
		if len(pending) == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if len(pending) != 1 {
		t.Fatalf("expected one pending approval, got %d", len(pending))
	}
	if err := broker.ResolveWithActor(pending[0].ID, true, ApprovalActor{ID: "device-a", PrincipalType: "local_ui", AuthMethod: "desktop_session", SessionID: "session-a"}); err != nil {
		t.Fatal(err)
	}
	if resolved.ResolvedBy != "device-a" || resolved.ResolvedByType != "local_ui" || resolved.AuthMethod != "desktop_session" || resolved.SessionID != "session-a" || resolved.ResolvedAt == "" {
		t.Fatalf("unexpected resolver: %+v", resolved)
	}
	if approved := <-result; !approved {
		t.Fatal("expected approval to continue")
	}
	if remaining := broker.List("space-1", "conv-1"); len(remaining) != 0 {
		t.Fatalf("expected pending approval to be removed, got %d", len(remaining))
	}
}
