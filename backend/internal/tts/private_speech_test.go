package tts

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func privateSpeechTestDB(t *testing.T, url string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "speech.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.Exec(`CREATE TABLE tts_configs(id INTEGER PRIMARY KEY,api_type TEXT,api_key TEXT,base_url TEXT,voice_type TEXT,is_active INTEGER,speed REAL,pitch REAL,volume REAL,is_custom INTEGER DEFAULT 0,custom_voice_id TEXT DEFAULT '')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO tts_configs(id,api_type,api_key,base_url,voice_type,is_active,speed,pitch,volume) VALUES(7,'openai','test-only',?,'echo',1,1,1,1),(999,'openai','wrong-config','http://invalid.invalid','wrong',0,1,1,1)`, url).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE tts_cloned_voices(speaker_id TEXT PRIMARY KEY,space_id TEXT,tts_config_id INTEGER,name TEXT,status TEXT,language INTEGER,created_at TEXT,updated_at TEXT)`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func TestPrivateSpeechRejectsSourceInheritedOrDisguisedCoreClone(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte("ID3unexpected-clone"))
	}))
	defer server.Close()
	db := privateSpeechTestDB(t, server.URL)
	if err := db.Exec(`INSERT INTO tts_cloned_voices(speaker_id,space_id,tts_config_id,name,status) VALUES('core-only-clone','core',7,'Core私有复刻','ready')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`UPDATE tts_configs SET voice_type='core-only-clone' WHERE id=7`).Error; err != nil {
		t.Fatal(err)
	}
	for _, profile := range []json.RawMessage{json.RawMessage(`{"voice":{"voiceMode":"preset"}}`), json.RawMessage(`{"voice":{"voiceType":"core-only-clone","voiceMode":"preset"}}`)} {
		if _, err := SynthesizePrivateRole(t.Context(), db, "core", profile, false, "设备不能借用Core私有音色"); err == nil || calls.Load() != 0 {
			t.Fatal("Source preset inherited or disguised a registered Core clone")
		}
	}
	if err := db.Exec(`UPDATE tts_configs SET voice_type='unregistered-clone',is_custom=1,custom_voice_id='unregistered-clone' WHERE id=7`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := SynthesizePrivateRole(t.Context(), db, "core", json.RawMessage(`{"voice":{"voiceMode":"preset"}}`), false, "缺少明确音色"); err == nil || calls.Load() != 0 {
		t.Fatal("Source inherited active custom clone metadata")
	}
}

func TestPrivateSpeechAllConfigurableProvidersUseRequestCancellation(t *testing.T) {
	for _, provider := range []string{"openai", "azure", "edge", "elevenlabs", "minimax", "aliyun", "cosyvoice"} {
		t.Run(provider, func(t *testing.T) {
			started := make(chan struct{})
			cancelled := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				_ = r.Body.Close()
				close(started)
				<-r.Context().Done()
				close(cancelled)
			}))
			defer server.Close()
			db := privateSpeechTestDB(t, server.URL)
			if err := db.Exec(`UPDATE tts_configs SET api_type=? WHERE id=7`, provider).Error; err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, err := SynthesizePrivateRole(ctx, db, "core", json.RawMessage(`{"voice":{"voiceMode":"preset"}}`), false, "所有供方均须取消")
				done <- err
			}()
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("provider request did not start")
			}
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("provider request ignored context: %v", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("provider request did not stop")
			}
			select {
			case <-cancelled:
			case <-time.After(5 * time.Second):
				t.Fatal("provider server did not observe cancellation")
			}
		})
	}
}

func TestPrivateSpeechUsesCoreActiveConfigAndSourceAcousticSnapshot(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var input map[string]any
		if r.URL.Path != "/audio/speech" || r.Header.Get("Authorization") != "Bearer test-only" || json.NewDecoder(r.Body).Decode(&input) != nil || input["voice"] != "alloy" || input["input"] != "设备角色朗读" {
			t.Error("Source provider ID selected or acoustic snapshot lost")
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("ID3private-provider-audio"))
	}))
	defer server.Close()
	db := privateSpeechTestDB(t, server.URL)
	profile := json.RawMessage(`{"voice":{"voiceConfigId":"999","voiceType":"alloy","voiceMode":"preset","voiceSpeed":1.25,"voicePitch":1,"voiceVolume":1}}`)
	data, err := SynthesizePrivateRole(t.Context(), db, "core", profile, false, "设备角色朗读")
	if err != nil || string(data) != "ID3private-provider-audio" || calls.Load() != 1 {
		t.Fatalf("private synthesis: %v", err)
	}
	profile = json.RawMessage(`{"voice":{"voiceType":"foreign-clone","voiceMode":"clone","customVoiceId":"foreign-clone"}}`)
	if _, err := SynthesizePrivateRole(t.Context(), db, "core", profile, false, "设备复刻音色"); err == nil || calls.Load() != 1 {
		t.Fatal("Source cloned voice borrowed Core provider slot")
	}
}

func TestPrivateSpeechUsesExplicitOwnedCloneAndRejectsForeignClone(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var input struct {
			ReqParams struct {
				Speaker string `json:"speaker"`
			} `json:"req_params"`
		}
		if r.URL.Path != "/api/v3/tts/unidirectional/sse" || json.NewDecoder(r.Body).Decode(&input) != nil || input.ReqParams.Speaker != "owned-clone" {
			t.Error("explicit clone selection was not sent to the active TTS service")
		}
		payload, _ := json.Marshal(map[string]any{"code": 20000000, "data": base64.StdEncoding.EncodeToString([]byte("ID3owned-clone"))})
		_, _ = w.Write([]byte("data: " + string(payload) + "\n\n"))
	}))
	defer server.Close()
	db := privateSpeechTestDB(t, server.URL)
	if err := db.Exec(`UPDATE tts_configs SET api_type='volcengine',base_url=? WHERE id=7`, server.URL).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO tts_cloned_voices(speaker_id,space_id,tts_config_id,name,status) VALUES('foreign-clone','other-core',7,'Other','ready')`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := SynthesizePrivateRole(t.Context(), db, "core", json.RawMessage(`{"voice":{"voiceMode":"preset"}}`), false, "复刻音色试听", PrivateSpeechOptions{SpeakerID: "foreign-clone"}); err == nil || calls.Load() != 0 {
		t.Fatal("foreign clone reached provider")
	}
	if err := db.Exec(`INSERT INTO tts_cloned_voices(speaker_id,space_id,tts_config_id,name,status) VALUES('owned-clone','core',7,'Owned','ready')`).Error; err != nil {
		t.Fatal(err)
	}
	data, err := SynthesizePrivateRole(t.Context(), db, "core", json.RawMessage(`{"voice":{"voiceMode":"preset","voiceSpeed":2}}`), false, "复刻音色试听", PrivateSpeechOptions{SpeakerID: "owned-clone"})
	if err != nil || string(data) != "ID3owned-clone" || calls.Load() != 1 {
		t.Fatalf("owned clone preview failed: %v", err)
	}
}

func TestPrivateSpeechCancelsActualProviderHTTPRequest(t *testing.T) {
	started := make(chan struct{})
	cancelled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
		close(started)
		<-r.Context().Done()
		close(cancelled)
	}))
	defer server.Close()
	db := privateSpeechTestDB(t, server.URL)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := SynthesizePrivateRole(ctx, db, "core", json.RawMessage(`{"voice":{"voiceMode":"preset"}}`), false, "取消私有朗读")
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("provider did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("request cancellation lost: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("provider request did not stop")
	}
	select {
	case <-cancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("provider connection was not cancelled")
	}
}

func TestPrivateSpeechBoundsProviderAudioAndRejectsMissingSnapshot(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(strings.Repeat("x", privateSpeechAudioLimit+1)))
	}))
	defer server.Close()
	db := privateSpeechTestDB(t, server.URL)
	if _, err := SynthesizePrivateRole(t.Context(), db, "core", json.RawMessage(`{}`), false, "朗读"); err == nil || calls.Load() != 0 {
		t.Fatal("missing snapshot reached provider")
	}
	if _, err := SynthesizePrivateRole(t.Context(), db, "core", json.RawMessage(`{"voice":{"voiceMode":"preset"}}`), false, "朗读"); err == nil || calls.Load() != 1 {
		t.Fatal("oversized provider audio escaped")
	}
}
