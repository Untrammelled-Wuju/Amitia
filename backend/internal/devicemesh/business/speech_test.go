package business

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type testSpeechModel struct {
	*testModel
	calls          atomic.Int32
	generateSpeech func(context.Context, SpeechInference) ([]byte, error)
}

type forgedSpeechACKPort struct {
	testDataPort
	final  bool
	change string
}

func (p forgedSpeechACKPort) Commit(ctx context.Context, commit coordination.Commit) (coordination.Acknowledgement, error) {
	ack, err := p.testDataPort.Commit(ctx, commit)
	if err != nil {
		return ack, err
	}
	final := false
	for _, mutation := range commit.Mutations {
		final = final || mutation.Kind == "tool-result"
	}
	if final != p.final {
		return ack, nil
	}
	switch p.change {
	case "owner":
		ack.OwnerID = "wrong-owner"
	case "request":
		ack.RequestID = "wrong-request"
	case "version":
		for key := range ack.Versions {
			ack.Versions[key]++
		}
	}
	return ack, nil
}

func TestOwnedSpeechRejectsForgedStartAndFinalAcknowledgements(t *testing.T) {
	engine, _, _, base := engineHarness(t)
	model := &testSpeechModel{testModel: base}
	engine.model = model
	port := engine.data.(testDataPort)
	for _, final := range []bool{false, true} {
		for _, change := range []string{"owner", "request", "version"} {
			engine.data = port
			id := "start-" + change
			if final {
				id = "final-" + change
			}
			request := speechTestRequest(t, engine, id)
			engine.data = forgedSpeechACKPort{testDataPort: port, final: final, change: change}
			before := model.calls.Load()
			response, err := engine.Speech(t.Context(), request, "不可伪造确认")
			if err == nil || response.Saved || response.Audio.Data != "" {
				t.Fatalf("forged %s ACK released audio (final=%v)", change, final)
			}
			expected := before
			if final {
				expected++
			}
			if model.calls.Load() != expected {
				t.Fatal("unconfirmed start acknowledgement reached provider")
			}
		}
	}
}

func TestOwnedSpeechRejectsPollutedStoredAudioScopeAndHashOnReplay(t *testing.T) {
	engine, db, _, base := engineHarness(t)
	model := &testSpeechModel{testModel: base}
	engine.model = model
	for _, change := range []string{"core", "role", "request", "hash", "data", "mime"} {
		request := speechTestRequest(t, engine, "polluted-"+change)
		response, err := engine.Speech(t.Context(), request, "回放原音频")
		if err != nil {
			t.Fatal(err)
		}
		rows, err := coordination.NewOwnershipStore(db, "a").List(t.Context(), "tool-result", "role", false)
		if err != nil {
			t.Fatal(err)
		}
		id := ""
		for _, row := range rows {
			var stored SpeechResponse
			if json.Unmarshal(row.Body, &stored) == nil && stored.RequestID == request.RequestID {
				id = row.ID
			}
		}
		if id == "" {
			t.Fatal("speech resource missing")
		}
		switch change {
		case "core":
			response.Scope.CoreID = "wrong-core"
		case "role":
			response.Scope.RoleRevision++
		case "request":
			response.RequestID = "wrong-request"
		case "hash":
			response.Audio.Hash = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
		case "data":
			response.Audio.Data = "invalid-base64"
		case "mime":
			response.Audio.MIME = "text/html"
		}
		if _, err := db.ExecContext(t.Context(), `UPDATE kernel_device_owned_resources SET body=? WHERE owner_id='a' AND kind='tool-result' AND resource_id=?`, body(response), id); err != nil {
			t.Fatal(err)
		}
		before := model.calls.Load()
		replayed, err := engine.Speech(t.Context(), request, "回放原音频")
		if err == nil || replayed.Saved || replayed.Audio.Data != "" || model.calls.Load() != before {
			t.Fatalf("polluted %s audio replay accepted: %v", change, err)
		}
	}
}

func (m *testSpeechModel) GenerateOwnedSpeech(ctx context.Context, input SpeechInference) ([]byte, error) {
	m.calls.Add(1)
	if m.generateSpeech != nil {
		return m.generateSpeech(ctx, input)
	}
	return []byte("ID3private-mp3"), nil
}

type rejectedSpeechOwnerPort struct{ testDataPort }

func (p rejectedSpeechOwnerPort) Commit(ctx context.Context, commit coordination.Commit) (coordination.Acknowledgement, error) {
	for _, mutation := range commit.Mutations {
		if mutation.Kind == "tool-result" {
			return coordination.Acknowledgement{}, coordination.ErrResourceVersion
		}
	}
	return p.testDataPort.Commit(ctx, commit)
}

func speechTestRequest(t *testing.T, engine *Engine, id string) Request {
	t.Helper()
	request := Request{SpaceID: "core", CoreID: "core", DeviceID: "a", RoleID: "role", RequestID: id}
	query, err := engine.Query(t.Context(), request, coordination.DataQuery{ResourceKind: "memory", Management: true})
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedScope = &query.Scope
	return request
}

func TestOwnedSpeechRequiresOwnerAcknowledgementAndDeduplicatesModelCalls(t *testing.T) {
	for _, coordinated := range []bool{false, true} {
		t.Run(map[bool]string{false: "source", true: "core"}[coordinated], func(t *testing.T) {
			engine, db, policies, base := engineHarness(t)
			if coordinated {
				if _, err := policies.ChangeMode(t.Context(), "core", "a", 1, true, "role"); err != nil {
					t.Fatal(err)
				}
			}
			model := &testSpeechModel{testModel: base}
			engine.model = model
			request := speechTestRequest(t, engine, "speech-request")
			response, err := engine.Speech(t.Context(), request, "私有朗读内容")
			if err != nil || !response.Saved || response.Acknowledgement.OwnerID != request.ExpectedScope.ResourceOwnerID {
				t.Fatalf("speech owner acknowledgement: %v %+v", err, response.Acknowledgement)
			}
			data, err := base64.StdEncoding.DecodeString(response.Audio.Data)
			if err != nil || string(data) != "ID3private-mp3" {
				t.Fatal("speech audio changed")
			}
			rows, err := coordination.NewOwnershipStore(db, response.Scope.ResourceOwnerID).List(t.Context(), "tool-result", "role", false)
			if err != nil || len(rows) != 1 {
				t.Fatal("audio not saved by original owner")
			}
			other := "core"
			if coordinated {
				other = "a"
			}
			copies, err := coordination.NewOwnershipStore(db, other).List(t.Context(), "tool-result", "role", false)
			if err != nil || len(copies) != 0 {
				t.Fatal("private speech mirrored on another owner")
			}
			replayed, err := engine.Speech(t.Context(), request, "私有朗读内容")
			if err != nil || !replayed.Saved || replayed.Audio != response.Audio || model.calls.Load() != 1 {
				t.Fatalf("speech retry repeated paid synthesis: %v", err)
			}
			if _, err := engine.Speech(t.Context(), request, "改变后的文本"); !errors.Is(err, coordination.ErrRequestConflict) {
				t.Fatalf("changed replay accepted: %v", err)
			}
		})
	}
}

func TestOwnedSpeechDropsAudioOnOwnerRejectionAndDoesNotReplayUncertainCall(t *testing.T) {
	engine, db, _, base := engineHarness(t)
	model := &testSpeechModel{testModel: base}
	engine.model = model
	request := speechTestRequest(t, engine, "rejected-speech")
	engine.data = rejectedSpeechOwnerPort{testDataPort: engine.data.(testDataPort)}
	response, err := engine.Speech(t.Context(), request, "未确认音频")
	if !errors.Is(err, coordination.ErrResourceVersion) || response.Saved || response.Audio.Data != "" {
		t.Fatalf("unconfirmed speech escaped: %v", err)
	}
	if _, err := engine.Speech(t.Context(), request, "未确认音频"); !errors.Is(err, ErrUncertainExecution) || model.calls.Load() != 1 {
		t.Fatalf("uncertain paid call replayed: %v", err)
	}
	rows, err := coordination.NewOwnershipStore(db, "a").List(t.Context(), "tool-result", "role", false)
	if err != nil || len(rows) != 0 {
		t.Fatal("rejected speech was stored")
	}
}

func TestOwnedSpeechCancelsProviderAndRejectsOldScopeAfterModeChanges(t *testing.T) {
	engine, db, policies, base := engineHarness(t)
	model := &testSpeechModel{testModel: base}
	engine.model = model
	request := speechTestRequest(t, engine, "cancelled-speech")
	model.generateSpeech = func(ctx context.Context, _ SpeechInference) ([]byte, error) {
		if _, err := policies.ChangeMode(t.Context(), "core", "a", 1, true, "role"); err != nil {
			return nil, err
		}
		<-ctx.Done()
		return []byte("ID3late-audio"), nil
	}
	response, err := engine.Speech(t.Context(), request, "切换时朗读")
	if err == nil || response.Audio.Data != "" || response.Saved {
		t.Fatal("late audio escaped changed mode")
	}
	if _, err := engine.Speech(t.Context(), request, "切换时朗读"); !errors.Is(err, coordination.ErrScopeExpired) || model.calls.Load() != 1 {
		t.Fatalf("old displayed scope accepted: %v", err)
	}
	for _, owner := range []string{"core", "a"} {
		rows, err := coordination.NewOwnershipStore(db, owner).List(t.Context(), "tool-result", "role", false)
		if err != nil || len(rows) != 0 {
			t.Fatal("cancelled audio saved")
		}
	}
}
