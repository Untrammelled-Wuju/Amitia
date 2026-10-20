package main

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/system"
)

func TestOwnedHostChatAppendAndReadShareActualOwner(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	db := p.services.KernelContainer.DeviceRegistry.Database()
	if _, err := p.services.KernelContainer.DeviceRegistry.EnsureDevice(t.Context(), host_registry.DeviceRecord{SpaceID: "core", DeviceID: "device-a", TrustState: host_registry.DeviceTrustTrusted}); err != nil {
		t.Fatal(err)
	}
	engine := business.NewEngine(coordination.NewService(db), p, nil)
	ctx := auth.WithActor(t.Context(), &auth.ActorContext{PrincipalType: auth.PrincipalTest, SpaceID: "core", DeviceID: "device-a", Permissions: auth.OwnerDevicePermissions()})
	ctx = kernel.WithHostAPIScope(ctx, kernel.HostAPIScopeContext{SpaceID: "core", CharacterID: "one", ConversationID: "owned-host"})
	view, err := engine.Query(ctx, business.Request{SpaceID: "core", DeviceID: "device-a", CoreID: "core", RoleID: "one", ConversationID: "owned-host"}, coordination.DataQuery{Management: true, ResourceKind: "conversation", ConversationID: "owned-host"})
	if err != nil {
		t.Fatal(err)
	}
	scope := view.Scope
	scope.RequestID = "seed-host"
	if _, err := p.store.Apply(ctx, coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{{Kind: "conversation", ID: "owned-host", RoleID: "one", Body: json.RawMessage(`{"id":"owned-host","characterId":"one","channel":"web"}`)}}}); err != nil {
		t.Fatal(err)
	}
	appender := &conversationMessageAppenderAdapter{engine: engine, coreID: "core"}
	request := kernel.ConversationMessageAppendRequest{SpaceID: "core", CharacterID: "one", ConversationID: "owned-host", Channel: "web", Role: "assistant", Source: "extension:test", RequestID: "host-append", Parts: []kernel.ConversationMessagePart{{Type: "text", Content: "Owner message"}}}
	for i := 0; i < 2; i++ {
		result, err := appender.AppendConversationMessages(ctx, request)
		if err != nil || len(result.MessageIDs) != 1 {
			t.Fatalf("append failed: %+v %v", result, err)
		}
	}
	reader := &kernelConversationReader{engine: engine, coreID: "core"}
	items, more, err := reader.ReadConversation(ctx, "owned-host", 50, 0)
	if err != nil || more || len(items) != 1 {
		t.Fatalf("Owner messages missing from plugin reader: %q %v %v", items, more, err)
	}
	var message struct {
		Content string `json:"contentSummary"`
	}
	if json.Unmarshal(items[0], &message) != nil || message.Content != "Owner message" {
		t.Fatalf("reader returned wrong data: %s", items[0])
	}
	var legacy int64
	if err := p.services.DB.Table("messages").Count(&legacy).Error; err != nil || legacy != 0 {
		t.Fatalf("plugin wrote old message table: %d %v", legacy, err)
	}
	request.CharacterID = "two"
	if _, err := appender.AppendConversationMessages(ctx, request); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("cross-role plugin append accepted: %v", err)
	}
	characters := &kernelCharacterReader{engine: engine, catalog: p, coreID: "core"}
	profile, found, err := characters.ReadCharacter(ctx, "one")
	if err != nil || !found || len(profile) == 0 {
		t.Fatalf("Owner character unavailable: %s %v %v", profile, found, err)
	}
	if _, found, err := characters.ReadCharacter(ctx, "two"); err != nil || found {
		t.Fatalf("cross-role character exposed: %v %v", found, err)
	}
	roles, err := characters.ListCharacters(ctx, "core", true)
	if err != nil || len(roles) != 3 {
		t.Fatalf("Owner role catalog lost disabled roles or included foreign role: %q %v", roles, err)
	}
	roles, err = characters.ListCharacters(ctx, "core", false)
	if err != nil || len(roles) != 2 {
		t.Fatalf("enabled role catalog changed: %q %v", roles, err)
	}
	request.CharacterID, request.Role, request.RequestID = "one", "user", "host-image"
	request.Parts = []kernel.ConversationMessagePart{{Type: "image", URL: "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4z8DwHwAFgAI/ScLttAAAAABJRU5ErkJggg=="}}
	result, err := appender.AppendConversationMessages(ctx, request)
	if err != nil || len(result.MessageIDs) != 1 {
		t.Fatalf("plugin image append failed: %+v %v", result, err)
	}
	stored, err := p.store.Get(ctx, "message", result.MessageIDs[0])
	var media struct {
		Attachments []business.Attachment `json:"attachments"`
	}
	if err != nil || stored == nil || json.Unmarshal(stored.Body, &media) != nil || len(media.Attachments) != 1 || business.ValidateAttachments(media.Attachments) != nil {
		t.Fatalf("plugin image was not saved as usable Owner context: %+v %v", stored, err)
	}
	request.RequestID = "host-invalid-image"
	request.Parts[0].URL = "http://127.0.0.1/private-image"
	if _, err := appender.AppendConversationMessages(ctx, request); err == nil {
		t.Fatal("plugin image resolver accepted a private network URL")
	}
	scope.RequestID = "seed-channel-host"
	if _, err := p.store.Apply(ctx, coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{{Kind: "conversation", ID: "channel-host", RoleID: "one", Body: json.RawMessage(`{"id":"channel-host","characterId":"one","channel":"telegram","peerId":"peer"}`)}}}); err != nil {
		t.Fatal(err)
	}
	channelContext := kernel.WithHostAPIScope(ctx, kernel.HostAPIScopeContext{SpaceID: "core", CharacterID: "one", ConversationID: "channel-host"})
	subscriber := system.GetMessageEventBus().Subscribe(t.Name(), []string{"telegram"})
	defer system.GetMessageEventBus().Unsubscribe(t.Name())
	request.ConversationID, request.Role, request.Channel, request.RequestID = "channel-host", "", "", "channel-append"
	request.Parts = []kernel.ConversationMessagePart{{Type: "text", Content: "waiting for actual delivery"}}
	if _, err := appender.AppendConversationMessages(channelContext, request); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-subscriber.Events:
		if event.Status != "sending" || event.Role != "assistant" || event.Channel != "telegram" {
			t.Fatalf("event did not reflect Owner acknowledgement: %+v", event)
		}
	default:
		t.Fatal("Owner channel event was not published")
	}
}
