package business

import "testing"

func TestForwardedContextSeparatesSameMessageIDByOwner(t *testing.T) {
	request := Request{ConversationID: "conversation", Context: &ForwardedContext{PreviousCoreID: "previous", ConversationID: "conversation", Messages: []ContextMessage{{ID: "same", OwnerID: "device", Role: "user", Content: "device history"}, {ID: "same", OwnerID: "previous-core", Role: "assistant", Content: "cloud history"}}}}
	if err := validateForwardedContext(request); err != nil {
		t.Fatalf("distinct owned context lost: %v", err)
	}
	request.Context.Messages[1].OwnerID = "device"
	if err := validateForwardedContext(request); err == nil {
		t.Fatal("duplicate owner/message accepted")
	}
	request.Context.Messages[1].OwnerID = "bad\x00owner"
	if err := validateForwardedContext(request); err == nil {
		t.Fatal("ambiguous owner key accepted")
	}
}
