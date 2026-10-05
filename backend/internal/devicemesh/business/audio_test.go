package business

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type audioTestModel struct {
	*testModel
	transcribe func(context.Context, Inference, Attachment) (string, error)
}

type rejectedTranscriptionPort struct {
	coordination.DataPort
}

func (p rejectedTranscriptionPort) Commit(ctx context.Context, commit coordination.Commit) (coordination.Acknowledgement, error) {
	if len(commit.Scope.RequestID) >= 14 && commit.Scope.RequestID[len(commit.Scope.RequestID)-14:] == "|transcription" {
		return coordination.Acknowledgement{}, coordination.ErrResourceVersion
	}
	return p.DataPort.Commit(ctx, commit)
}

func TestOwnedAudioDoesNotGenerateUntilTranscriptionOwnerAcknowledges(t *testing.T) {
	engine, _, _, model := engineHarness(t)
	engine.data = rejectedTranscriptionPort{DataPort: engine.data}
	engine.model = audioTestModel{testModel: model, transcribe: func(context.Context, Inference, Attachment) (string, error) { return "unconfirmed text", nil }}
	_, err := engine.Run(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RequestID: "unconfirmed-audio", Message: "语音消息", Attachments: []Attachment{audioAttachment()}})
	if !errors.Is(err, coordination.ErrResourceVersion) || model.calls.Load() != 0 || model.extractions.Load() != 0 {
		t.Fatalf("unconfirmed transcription reached model: %v", err)
	}
}

func (m audioTestModel) TranscribeOwnedAudio(ctx context.Context, input Inference, attachment Attachment) (string, error) {
	return m.transcribe(ctx, input, attachment)
}

func audioAttachment() Attachment {
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
	digest := sha256.Sum256(data)
	return Attachment{Kind: "audio", Name: "voice.wav", MIME: "audio/wav", Data: base64.StdEncoding.EncodeToString(data), Hash: hex.EncodeToString(digest[:])}
}

func TestOwnedAudioConfirmsOriginalAndTranscriptionBeforeReplyAndMemory(t *testing.T) {
	for _, coordinated := range []bool{false, true} {
		t.Run(map[bool]string{false: "device", true: "core"}[coordinated], func(t *testing.T) {
			engine, db, service, model := engineHarness(t)
			owner := "a"
			if coordinated {
				if _, err := service.ChangeMode(t.Context(), "core", "a", 1, true, "role"); err != nil {
					t.Fatal(err)
				}
				owner = "core"
			}
			attachment := audioAttachment()
			transcriptions := 0
			engine.model = audioTestModel{testModel: model, transcribe: func(_ context.Context, _ Inference, item Attachment) (string, error) {
				transcriptions++
				row, err := coordination.NewOwnershipStore(db, owner).Get(t.Context(), "message", "audio/user")
				if err != nil || row == nil || row.Revision != 1 || item != attachment {
					t.Fatal("ASR started before original owner confirmation")
				}
				return "我喜欢喝茶", nil
			}}
			model.generate = func(_ context.Context, inference Inference) (Generation, error) {
				row, err := coordination.NewOwnershipStore(db, owner).Get(t.Context(), "message", "audio/user")
				if err != nil || row == nil || row.Revision != 2 || inference.Message != "我喜欢喝茶" {
					t.Fatal("reply started before transcription owner confirmation")
				}
				var saved map[string]any
				if json.Unmarshal(row.Body, &saved) != nil || saved["content"] != "语音消息" || saved["transcription"] != "我喜欢喝茶" || saved["transcriptionSourceContent"] != "语音消息" {
					t.Fatal("original audio message or transcription was lost")
				}
				return Generation{Text: "记住了"}, nil
			}
			model.extract = func(_ context.Context, inference Inference, _ Generation) ([]DerivedMemory, error) {
				if inference.Message != "我喜欢喝茶" {
					t.Fatal("memory used placeholder instead of transcription")
				}
				return nil, nil
			}
			request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RequestID: "audio", Message: "语音消息", Attachments: []Attachment{attachment}}
			response, err := engine.Run(t.Context(), request)
			if err != nil || !response.Saved || response.Transcription != "我喜欢喝茶" || response.UserRevision != 2 || response.MemoryStatus != "saved" {
				t.Fatalf("audio response failed: %+v %v", response, err)
			}
			if _, err := engine.Run(t.Context(), request); err != nil || transcriptions != 1 || model.calls.Load() != 1 {
				t.Fatalf("audio was transcribed or generated twice: %v", err)
			}
			other := "core"
			if coordinated {
				other = "a"
			}
			if mirrored, err := coordination.NewOwnershipStore(db, other).Get(t.Context(), "message", "audio/user"); err != nil || mirrored != nil {
				t.Fatal("audio was duplicated at the other owner")
			}
		})
	}
}

func TestOwnedAudioRejectsReplyAfterASRFailureOrCoreCutover(t *testing.T) {
	for _, change := range []bool{false, true} {
		t.Run(map[bool]string{false: "recognition-failure", true: "mode-cutover"}[change], func(t *testing.T) {
			engine, _, service, model := engineHarness(t)
			engine.model = audioTestModel{testModel: model, transcribe: func(ctx context.Context, _ Inference, _ Attachment) (string, error) {
				if !change {
					return "", errors.New("provider unavailable")
				}
				if _, err := service.ChangeMode(t.Context(), "core", "a", 1, true, "role"); err != nil {
					t.Fatal(err)
				}
				return "late transcription", nil
			}}
			_, err := engine.Run(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RequestID: "audio-failure", Message: "语音消息", Attachments: []Attachment{audioAttachment()}})
			if err == nil || model.calls.Load() != 0 || model.extractions.Load() != 0 {
				t.Fatalf("failed or interrupted audio reached reply: %v", err)
			}
			if change && !errors.Is(err, coordination.ErrScopeExpired) {
				t.Fatalf("cutover cause was lost: %v", err)
			}
		})
	}
}
