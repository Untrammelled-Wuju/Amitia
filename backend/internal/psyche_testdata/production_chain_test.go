package psyche_testdata

import (
	"encoding/json"
	"fmt"
	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
	"testing"
)

func TestAllCasesEnterOwnedRuntimeAssembly(t *testing.T) {
	cases, err := LoadCases(DefaultCasesPath())
	if err != nil {
		t.Fatal(err)
	}
	tracker := interaction.NewInMemoryTracker()
	runtimeService := interaction.NewRuntimeContextService(tracker, 16)
	for i, c := range cases {
		request := productionUnifiedEntryRequest(c, fmt.Sprintf("%04d-%s", i, c.ID))
		scope := coordination.ExecutionScope{SpaceID: request.SpaceID, CoreID: request.SpaceID, InitiatorDeviceID: "device", TargetDeviceID: "device", RoleOwnerID: "device", ResourceOwnerID: "device", RoleID: request.CharacterID, RoleRevision: 1, RequestID: request.RequestID}
		profile, err := json.Marshal(character.RoleRuntimeProfile{CharacterID: request.CharacterID, Name: "测试角色"})
		if err != nil {
			t.Fatal(err)
		}
		ctx := coordination.WithScope(t.Context(), scope)
		ctx, err = interaction.WithOwnedContext(ctx, scope, coordination.DataSnapshot{OwnerID: "device", Role: coordination.Role{ID: scope.RoleID, Revision: 1, Profile: profile}}, request.ConversationID, request.Channel)
		if err != nil {
			t.Fatal(err)
		}
		record := interaction.NewInteractionRecord(interaction.InteractionScope{SpaceID: request.SpaceID, CharacterID: request.CharacterID, ConversationID: request.ConversationID, Channel: request.Channel, PeerID: request.PeerID, Source: request.Source, RequestID: request.RequestID})
		if err := tracker.Create(ctx, record); err != nil {
			t.Fatal(err)
		}
		record, err = tracker.TransitionCAS(ctx, record.ID, record.StatusVersion, interaction.InteractionStatusProcessing)
		if err != nil {
			t.Fatal(err)
		}
		input := &interaction.ProcessRequest{CharacterID: request.CharacterID, ConversationID: request.ConversationID, SpaceID: request.SpaceID, Channel: request.Channel, Source: request.Source, RequestID: request.RequestID, Message: request.Message}
		_, runtime, finish, err := runtimeService.PrepareOwnedRuntime(ctx, record, input)
		if err != nil {
			t.Fatalf("%s: %v", c.ID, err)
		}
		if runtime == nil || runtime.ExecutorID != "owned" || runtime.Delivery.Channel == "" || runtime.Transaction.Name == "" || input.Runtime != runtime {
			t.Fatalf("%s incomplete Owner runtime: %+v", c.ID, runtime)
		}
		if err := finish(ctx, true); err != nil {
			t.Fatal(err)
		}
	}
}
func productionUnifiedEntryRequest(c Case, requestID string) *interaction.UnifiedEntryRequest {
	channel := stringFromCaseInput(c, "channel", "web")
	message := stringFromCaseInput(c, "content", c.ID)
	eventType := stringFromCaseInput(c, "type", "text")
	return &interaction.UnifiedEntryRequest{
		Channel:        channel,
		Source:         channel,
		SpaceID:        "user-" + c.ID,
		CharacterID:    "char-" + c.Category,
		ConversationID: "conv-" + c.ID,
		PeerID:         "peer-" + c.ID,
		SessionID:      "session-" + c.Category,
		RequestID:      requestID,
		Message:        message,
		VoiceMessage:   eventType == "voice" || eventType == "audio",
	}
}

func stringFromCaseInput(c Case, key, fallback string) string {
	if value, ok := c.InputEvent[key]; ok && value != nil {
		text := fmt.Sprint(value)
		if text != "" {
			return text
		}
	}
	return fallback
}
