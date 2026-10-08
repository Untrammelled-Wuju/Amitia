package business

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestRealtimePCMRejectsInvalidAndKeepsExactSamples(t *testing.T) {
	for _, pcm := range [][]byte{nil, {1}, make([]byte, 1<<20)} {
		if _, err := RealtimePCM(pcm); err == nil {
			t.Fatal("invalid PCM accepted")
		}
	}
	item, err := RealtimePCM([]byte{0, 1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := base64.StdEncoding.DecodeString(item.Data)
	if string(data[44:]) != string([]byte{0, 1, 2, 3}) || binary.LittleEndian.Uint32(data[24:]) != 16000 || ValidateAttachments([]Attachment{item}) != nil {
		t.Fatal("PCM samples or WAV metadata changed")
	}
}

func TestRealtimeTurnPreservesVisualAndOwnerConfirmedTranscription(t *testing.T) {
	engine, _, _, model := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: uuid.NewString()}
	_, scope, finish, err := engine.continuityAuthority(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	finish()
	request.ExpectedScope = &scope
	frame := imageAttachment(t, 1)
	engine.model = audioTestModel{testModel: model, transcribe: func(context.Context, Inference, Attachment) (string, error) {
		return "识别后的当前画面问题", nil
	}}
	model.generate = func(_ context.Context, input Inference) (Generation, error) {
		if input.Message != "识别后的当前画面问题" || len(input.Attachments) != 2 || input.Attachments[1] != frame {
			t.Fatal("visual or confirmed transcription missing")
		}
		return Generation{Text: "当前画面回答"}, nil
	}
	result, err := engine.RealtimeTurn(t.Context(), request, []byte{0, 1}, &frame, nil)
	if err != nil || !result.Saved || result.UserRevision != 2 || result.Transcription != "识别后的当前画面问题" {
		t.Fatalf("owner confirmed realtime failed: %+v %v", result, err)
	}
	stale := scope
	stale.RoleRevision++
	request.ExpectedScope = &stale
	request.RequestID = uuid.NewString()
	if _, err = engine.RealtimeTurn(t.Context(), request, []byte{0, 1}, nil, nil); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("old role allowed: %v", err)
	}
}
