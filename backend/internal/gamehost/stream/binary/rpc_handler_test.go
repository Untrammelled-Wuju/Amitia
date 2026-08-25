package binary

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/gamehost/channel"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/resource"
	"github.com/u-ai/backend/internal/gamehost/rpc"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type allowBinaryFeatureChecker struct{}

func (allowBinaryFeatureChecker) HasNegotiatedCapability(string, domain.Capability) bool { return true }

type allowBinaryResourceAdmission struct{}

func (allowBinaryResourceAdmission) AcquireBinaryObject(context.Context, resource.RuntimeIdentitySubject, int64) (resource.AdmissionDecision, resource.BinaryRevertFunc) {
	return resource.AdmissionDecision{Allowed: true}, func() {}
}

type denyBinaryResourceAdmission struct{}

func (denyBinaryResourceAdmission) AcquireBinaryObject(context.Context, resource.RuntimeIdentitySubject, int64) (resource.AdmissionDecision, resource.BinaryRevertFunc) {
	return resource.AdmissionDecision{Allowed: false, Reason: resource.DenyBinaryBytesLimit}, func() {}
}

func TestBinaryTransferService_RawFrameWriteRoundTrip(t *testing.T) {
	ctx := context.Background()
	objects := NewObjectRegistry(Options{})
	providers := NewProviderRegistry()
	files, err := NewFileProvider(t.TempDir())
	if err != nil {
		t.Fatalf("new file provider: %v", err)
	}
	providers.Register(files)
	resolver := NewResolver(objects, providers)
	channels := channel.NewMemoryRegistry(channel.Options{})
	declared := channel.RuntimeChannel{
		ID:        channel.NewRuntimeChannelID("runtime-1", "service-1", "frames"),
		PluginID:  "plugin-1",
		RuntimeID: "runtime-1",
		ServiceID: "service-1",
		ChannelID: "frames",
		Kind:      domain.ChannelKindBinary,
		Direction: protocol.ChannelDirectionPluginToHost,
	}
	if err := channels.Register(ctx, declared); err != nil {
		t.Fatalf("register channel: %v", err)
	}
	service, err := NewBinaryTransferService(resolver, objects, channels, allowBinaryFeatureChecker{}, allowBinaryResourceAdmission{})
	if err != nil {
		t.Fatalf("new binary transfer service: %v", err)
	}

	base := rpc.RPCRequest{
		ConnectionID: "conn-1",
		PluginID:     "plugin-1",
		RuntimeID:    "runtime-1",
		ServiceID:    "service-1",
		Generation:   1,
	}
	create := base
	create.ID = "req-create"
	create.Method = MethodBinaryCreate
	create.Payload = json.RawMessage(`{"channelId":"frames","expectedSize":3}`)
	createResp, err := service.Handle(ctx, create)
	if err != nil || createResp.Error != nil {
		t.Fatalf("binary.create failed: resp=%+v err=%v", createResp, err)
	}
	var created struct {
		ID BinaryObjectID `json:"id"`
	}
	if err := json.Unmarshal(createResp.Payload, &created); err != nil || created.ID == "" {
		t.Fatalf("decode binary.create response: id=%q err=%v", created.ID, err)
	}

	write := base
	write.ID = "req-write"
	write.Method = MethodBinaryWrite
	write.BinaryObjectID = string(created.ID)
	write.BinaryOffset = 0
	write.BinaryPayload = []byte("abc")
	writeResp, err := service.Handle(ctx, write)
	if err != nil || writeResp.Error != nil {
		t.Fatalf("raw binary.write failed: resp=%+v err=%v", writeResp, err)
	}

	seal := base
	seal.ID = "req-seal"
	seal.Method = MethodBinarySeal
	seal.Payload, _ = json.Marshal(map[string]any{"id": created.ID})
	sealResp, err := service.Handle(ctx, seal)
	if err != nil || sealResp.Error != nil {
		t.Fatalf("binary.seal failed: resp=%+v err=%v", sealResp, err)
	}
	var sealed struct {
		Reference BinaryReference `json:"reference"`
	}
	if err := json.Unmarshal(sealResp.Payload, &sealed); err != nil {
		t.Fatalf("decode binary.seal response: %v", err)
	}
	if sealed.Reference.Size != 3 || sealed.Reference.Checksum == nil || sealed.Reference.Checksum.Algorithm != "sha256" {
		t.Fatalf("unexpected sealed reference: %+v", sealed.Reference)
	}

	// A caller may present a valid object id with forged descriptive fields. The
	// channel trust boundary must return the registry-authoritative reference,
	// never the caller-controlled checksum/mediaType/metadata.
	forged := sealed.Reference
	forged.MediaType = "application/x-forged"
	forged.Metadata = map[string]json.RawMessage{"spoof": json.RawMessage(`true`)}
	forged.Checksum = &Checksum{Algorithm: "sha256", Value: "0000000000000000000000000000000000000000000000000000000000000000"}
	forgedPayload, _ := json.Marshal(forged)
	canonicalPayload, err := NewChannelBinarySink(resolver).PublishBinary(ctx, declared, channel.BinaryChannelMessage{
		PluginID: "plugin-1", RuntimeID: "runtime-1", ServiceID: "service-1", ChannelID: "frames", Payload: forgedPayload,
	})
	if err != nil {
		t.Fatalf("canonical binary channel validation failed: %v", err)
	}
	var canonical BinaryReference
	if err := json.Unmarshal(canonicalPayload, &canonical); err != nil {
		t.Fatalf("decode canonical binary channel payload: %v", err)
	}
	if canonical.MediaType == forged.MediaType || canonical.Metadata != nil || canonical.Checksum == nil || canonical.Checksum.Value == forged.Checksum.Value {
		t.Fatalf("binary channel preserved caller-controlled reference fields: %+v", canonical)
	}
}

func TestBinaryTransferService_RejectsMixedRawAndJSONWrite(t *testing.T) {
	request := rpc.RPCRequest{
		ID:             "req-mixed",
		BinaryObjectID: "bin_deadbeef",
		BinaryPayload:  []byte("x"),
		Payload:        json.RawMessage(`{"id":"bin_deadbeef","offset":0,"data":"eA=="}`),
	}
	service := &BinaryTransferService{}
	response, err := service.handleWrite(context.Background(), request)
	if err != nil {
		t.Fatalf("mixed write should produce protocol response, got transport error: %v", err)
	}
	if response.Error == nil || response.Error.Code != string(domain.ErrInvalidArgument) {
		t.Fatalf("expected invalid_argument for mixed write, got %+v", response.Error)
	}
}

func TestBinaryTransferService_CreateHonorsResourceAdmission(t *testing.T) {
	ctx := context.Background()
	objects := NewObjectRegistry(Options{})
	providers := NewProviderRegistry()
	files, err := NewFileProvider(t.TempDir())
	if err != nil {
		t.Fatalf("new file provider: %v", err)
	}
	providers.Register(files)
	channels := channel.NewMemoryRegistry(channel.Options{})
	declared := channel.RuntimeChannel{
		ID:        channel.NewRuntimeChannelID("runtime-1", "service-1", "frames"),
		PluginID:  "plugin-1",
		RuntimeID: "runtime-1",
		ServiceID: "service-1",
		ChannelID: "frames",
		Kind:      domain.ChannelKindBinary,
		Direction: protocol.ChannelDirectionPluginToHost,
	}
	if err := channels.Register(ctx, declared); err != nil {
		t.Fatalf("register channel: %v", err)
	}
	service, err := NewBinaryTransferService(NewResolver(objects, providers), objects, channels, allowBinaryFeatureChecker{}, denyBinaryResourceAdmission{})
	if err != nil {
		t.Fatalf("new binary transfer service: %v", err)
	}
	request := rpc.RPCRequest{
		ID: "req-create", Method: MethodBinaryCreate, ConnectionID: "conn-1",
		PluginID: "plugin-1", RuntimeID: "runtime-1", ServiceID: "service-1", Generation: 1,
		Payload: json.RawMessage(`{"channelId":"frames","expectedSize":3}`),
	}
	response, err := service.Handle(ctx, request)
	if err != nil {
		t.Fatalf("resource denial should be a protocol response: %v", err)
	}
	if response.Error == nil || response.Error.Code != string(domain.ErrResourceExhausted) {
		t.Fatalf("expected resource_exhausted, got %+v", response.Error)
	}
	if objects.CountActive() != 0 {
		t.Fatalf("denied create must not allocate binary objects, active=%d", objects.CountActive())
	}
}
