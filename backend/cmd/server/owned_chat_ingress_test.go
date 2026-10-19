package main

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/devicemesh"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/interaction"
)

func TestOwnedIngressMediaCarriesActualBytesAndRejectsUnsafeSources(t *testing.T) {
	data := make([]byte, 46)
	copy(data[:4], "RIFF")
	binary.LittleEndian.PutUint32(data[4:8], 38)
	copy(data[8:16], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:20], 16)
	binary.LittleEndian.PutUint16(data[20:22], 1)
	binary.LittleEndian.PutUint16(data[22:24], 1)
	binary.LittleEndian.PutUint32(data[24:28], 16000)
	binary.LittleEndian.PutUint32(data[28:32], 32000)
	binary.LittleEndian.PutUint16(data[32:34], 2)
	binary.LittleEndian.PutUint16(data[34:36], 16)
	copy(data[36:40], "data")
	binary.LittleEndian.PutUint32(data[40:44], 2)
	encoded := base64.StdEncoding.EncodeToString(data)
	items, err := ownedIngressAttachments(t.Context(), &interaction.UnifiedEntryRequest{AudioUrl: "data:audio/wav;base64," + encoded})
	if err != nil || len(items) != 1 || items[0].Data != encoded || items[0].Kind != "audio" || items[0].Hash == "" {
		t.Fatalf("actual media not forwarded: %+v %v", items, err)
	}
	for _, source := range []string{"/voice/../secret", "/voice/%2e%2e%2fsecret", "file:///secret", "http://127.0.0.1/private", "data:image/png;base64," + strings.Repeat("A", base64.StdEncoding.EncodedLen(1<<20)+4)} {
		if _, _, _, err := ownedIngressMedia(t.Context(), source); err == nil {
			t.Fatalf("unsafe media accepted: %.80s", source)
		}
	}
}

func TestOwnedIngressMessagePlanPreservesTextOutputOrderAndMetadata(t *testing.T) {
	outputs, err := json.Marshal([]chat.MessageOutput{{OutputID: "voice", Part: chat.MessagePart{Type: "audio", URL: "https://assets.example/voice.wav"}}, {OutputID: "picture", Placement: "before_text", Part: chat.MessagePart{Type: "image", URL: "https://assets.example/image.png", FallbackURL: "fallback", MIMEType: "image/png", Width: 32}}})
	if err != nil {
		t.Fatal(err)
	}
	response := &interaction.ProcessResponse{RequestID: "request", Reply: "正文"}
	if err := ownedIngressResponseOutputs(response, outputs); err != nil {
		t.Fatal(err)
	}
	items := response.MessagePlan.Items
	if len(items) != 3 || items[0].MessageID != "picture" || items[1].Content != "正文" || items[2].MessageID != "voice" || items[0].Sequence != 1 || items[2].Sequence != 3 || items[0].Width != 32 || items[0].FallbackAssetReference != "fallback" || len(response.AudioUrls) != 1 {
		t.Fatalf("output plan lost content or ordering: %+v", response)
	}
	if err := ownedIngressResponseOutputs(&interaction.ProcessResponse{RequestID: "plain", Reply: "text"}, nil); err != nil {
		t.Fatal(err)
	}
}

func TestOwnedIngressOwnerRejectsUnboundAndCrossOwnerRequests(t *testing.T) {
	ingress := newOwnedChatIngress(nil, "cloud-core")
	request := &interaction.UnifiedEntryRequest{}
	if _, err := ingress.owner(t.Context(), request); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("unbound source accepted: %v", err)
	}
	ctx := interaction.WithEntryOwner(t.Context(), interaction.EntryOwner{SpaceID: "cloud-core", CoreID: "cloud-core", DeviceID: "phone", TargetDeviceID: "phone"})
	owner, err := ingress.owner(ctx, request)
	if err != nil || owner.DeviceID != "phone" || owner.TargetDeviceID != "phone" {
		t.Fatalf("cloud source silently rebound: %+v %v", owner, err)
	}
	request.SpaceID = "another-core"
	if _, err := ingress.owner(ctx, request); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("payload changed source owner: %v", err)
	}
	ctx = auth.WithActor(ctx, &auth.ActorContext{SpaceID: "cloud-core", DeviceID: "desktop"})
	request.SpaceID = ""
	if _, err := ingress.owner(ctx, request); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("conflicting actor source accepted: %v", err)
	}
}

func TestOwnedIngressChannelBindingRequiresAuthenticatedMatchingRuntime(t *testing.T) {
	registry := capability.NewProviderRegistry()
	services := &AppServices{OwnedBusiness: &business.Engine{}, DeviceMesh: &devicemesh.Runtime{LocalDeviceID: "local"}, KernelContainer: &kernel.Container{CapabilityProviders: registry}}
	ingress := newOwnedChatIngress(services, "cloud-core")
	if _, err := ingress.channelOwnerResolver(t.Context(), "plugin", "module", "telegram"); err == nil {
		t.Fatal("missing source runtime fell back to local device")
	}
	definition := capability.CapabilityProviderDefinition{ID: "telegram-provider", CapabilityID: "channel.provider", Kind: capability.ProviderKindExtension, Placement: capability.ProviderPlacementDevice, ExtensionID: "plugin", ModuleID: "module", Runtime: capability.RuntimeBinding{RuntimeType: capability.RuntimeTypeInternal}, Metadata: map[string]any{"labels": map[string]any{"channelId": "telegram"}}}
	if err := registry.RegisterDefinition(definition); err != nil {
		t.Fatal(err)
	}
	instance := capability.CapabilityProviderInstance{ID: "phone-provider", ProviderID: definition.ID, CapabilityID: definition.CapabilityID, Placement: definition.Placement, ExtensionID: definition.ExtensionID, ModuleID: definition.ModuleID, SpaceID: "cloud-core", DeviceID: "phone", RuntimeID: "phone-runtime", Health: capability.HealthReady, Availability: capability.ProviderAvailabilityAvailable}
	if err := registry.RegisterInstance(instance); err != nil {
		t.Fatal(err)
	}
	ctx, err := ingress.channelOwnerResolver(t.Context(), "plugin", "module", "telegram")
	if err != nil {
		t.Fatal(err)
	}
	owner, ok := interaction.EntryOwnerFromContext(ctx)
	if !ok || owner.DeviceID != "phone" {
		t.Fatalf("source did not retain physical device: %+v", owner)
	}
	if _, err := ingress.channelOwnerResolver(context.Background(), "plugin", "module", "wechat"); err == nil {
		t.Fatal("channel metadata mismatch accepted")
	}
	instance.ID, instance.DeviceID, instance.RuntimeID = "another-provider", "another-phone", "another-runtime"
	if err := registry.RegisterInstance(instance); err != nil {
		t.Fatal(err)
	}
	if _, err := ingress.channelOwnerResolver(t.Context(), "plugin", "module", "telegram"); err == nil {
		t.Fatal("ambiguous physical owner accepted")
	}
}
