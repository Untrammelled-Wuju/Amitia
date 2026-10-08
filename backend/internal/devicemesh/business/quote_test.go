package business

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestOwnedQuoteReadsActualOwnerAndStoresReviewedSnapshot(t *testing.T) {
	for _, historical := range []bool{false, true} {
		t.Run(map[bool]string{false: "current", true: "historical"}[historical], func(t *testing.T) {
			e, db, service, model := engineHarness(t)
			e.data = originDataPort{testDataPort{db: db, role: coordination.Role{ID: "role", Revision: 1, Profile: json.RawMessage(`{}`)}}}
			model.extract = func(context.Context, Inference, Generation) ([]DerivedMemory, error) { return nil, nil }
			req := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "chat", RequestID: "original", Message: "我喜欢茶"}
			first, err := e.Run(t.Context(), req)
			if err != nil {
				t.Fatal(err)
			}
			if historical {
				if _, err = service.ChangeMode(t.Context(), "core", "a", 1, true, "role"); err != nil {
					t.Fatal(err)
				}
				req.ConversationOrigin = &ConversationOrigin{OwnerID: "a", ID: "chat"}
			}
			current, err := e.Query(t.Context(), req, coordination.DataQuery{ConversationID: "chat", Management: true})
			if err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256([]byte("我喜欢茶"))
			req.RequestID, req.Message, req.ExpectedScope = "quoted", "为什么？", &current.Scope
			req.Quote = &QuoteReference{OwnerID: first.Scope.ResourceOwnerID, RoleID: "role", ConversationID: "chat", MessageID: "original/user", ExpectedRevision: 1, ContentHash: hex.EncodeToString(digest[:]), ExpectedScope: &current.Scope}
			model.generate = func(_ context.Context, in Inference) (Generation, error) {
				if in.Quote == nil || in.Quote.Content != "我喜欢茶" || in.Quote.OwnerID != "a" {
					t.Fatalf("unreviewed quote: %+v", in.Quote)
				}
				return Generation{Text: "基于原文回复"}, nil
			}
			response, err := e.Run(t.Context(), req)
			if err != nil || !response.Saved {
				t.Fatalf("response=%+v err=%v", response, err)
			}
			stored, err := coordination.NewOwnershipStore(db, response.Scope.ResourceOwnerID).Get(t.Context(), "message", "quoted/user")
			if err != nil || stored == nil {
				t.Fatal(err)
			}
			var document struct {
				Quote *ReviewedQuote `json:"quote"`
			}
			if json.Unmarshal(stored.Body, &document) != nil || document.Quote == nil || document.Quote.OwnerID != "a" || document.Quote.Revision != 1 {
				t.Fatal("reviewed source quote was not saved with new owner input")
			}
		})
	}
}

func TestOwnedQuoteRejectsForeignOwnerRoleVersionAndOriginalAuthority(t *testing.T) {
	for _, change := range []string{"owner", "role", "conversation", "revision", "hash", "scope"} {
		t.Run(change, func(t *testing.T) {
			e, _, _, model := engineHarness(t)
			model.extract = func(context.Context, Inference, Generation) ([]DerivedMemory, error) { return nil, nil }
			req := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "chat", RequestID: "first", Message: "真实消息"}
			response, err := e.Run(t.Context(), req)
			if err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256([]byte("真实消息"))
			original := response.Scope
			quote := &QuoteReference{OwnerID: "a", RoleID: "role", ConversationID: "chat", MessageID: "first/user", ExpectedRevision: 1, ContentHash: hex.EncodeToString(digest[:]), ExpectedScope: &original}
			switch change {
			case "owner":
				quote.OwnerID = "other"
			case "role":
				quote.RoleID = "other"
			case "conversation":
				quote.ConversationID = "other"
			case "revision":
				quote.ExpectedRevision = 2
			case "hash":
				quote.ContentHash = string(make([]byte, 64))
			case "scope":
				original.AuthorizationRealm = "other"
			}
			req.RequestID, req.Message, req.Quote = "bad", "继续", quote
			_, err = e.Run(t.Context(), req)
			if err == nil || model.calls.Load() != 1 {
				t.Fatalf("bad quote reached model: %v", err)
			}
			if change == "revision" && !errors.Is(err, coordination.ErrResourceVersion) {
				t.Fatal(err)
			}
		})
	}
}
