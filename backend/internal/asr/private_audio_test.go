package asr

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestPrivateAudioCancellationStopsUpstreamRequest(t *testing.T) {
	for _, provider := range []string{"openai", "azure"} {
		t.Run(provider, func(t *testing.T) {
			started, stopped, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				close(started)
				select {
				case <-r.Context().Done():
					close(stopped)
				case <-release:
				}
			}))
			defer server.Close()
			defer close(release)
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			result := make(chan error, 1)
			go func() {
				_, err := RecognizePrivateAudio(ctx, &AsrConfig{ApiType: provider, ApiKey: "test-only", BaseURL: server.URL}, encodeWAV([]byte{1, 2}, 16000, 1), "audio/wav", "zh-CN")
				result <- err
			}()
			select {
			case <-started:
			case <-time.After(2 * time.Second):
				t.Fatal("ASR request did not start")
			}
			cancel(coordination.ErrScopeExpired)
			select {
			case err := <-result:
				if !errors.Is(err, coordination.ErrScopeExpired) {
					t.Fatalf("cutover cause was lost: %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("ASR did not stop after core cutover")
			}
			select {
			case <-stopped:
			case <-time.After(2 * time.Second):
				t.Fatal("upstream ASR request was left running")
			}
		})
	}
}

func TestPrivateAudioUsesInMemoryOwnerInputWithoutAsyncResultCache(t *testing.T) {
	wave := encodeWAV([]byte{1, 2, 3, 4}, 16000, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/audio/transcriptions" || r.Header.Get("Authorization") != "Bearer test-only" {
			t.Error("private audio did not use core provider configuration")
			w.WriteHeader(400)
			return
		}
		if err := r.ParseMultipartForm(4 << 20); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		defer r.MultipartForm.RemoveAll()
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		defer file.Close()
		data, _ := io.ReadAll(file)
		if !bytes.Equal(data, wave) || header.Filename != "audio.wav" || r.FormValue("model") != "configured-model" || r.FormValue("language") != "zh" {
			t.Error("audio or model configuration changed")
		}
		_, _ = w.Write([]byte(`{"text":" 已识别的文字 "}`))
	}))
	defer server.Close()
	before, after := 0, 0
	syncResults.Range(func(_, _ any) bool { before++; return true })
	text, err := RecognizePrivateAudio(t.Context(), &AsrConfig{ApiType: "openai", ApiKey: "test-only", BaseURL: server.URL, ResourceId: "configured-model"}, wave, "audio/wav", "zh")
	if err != nil || text != "已识别的文字" {
		t.Fatalf("private recognition failed: %q %v", text, err)
	}
	syncResults.Range(func(_, _ any) bool { after++; return true })
	if after != before {
		t.Fatal("private audio created a second asynchronous result copy")
	}
}

func TestPrivateAudioRejectsMalformedAndUnboundedDataBeforeProvider(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { requests++; w.WriteHeader(500) }))
	defer server.Close()
	cfg := &AsrConfig{ApiType: "openai", ApiKey: "test-only", BaseURL: server.URL}
	valid := encodeWAV([]byte{1, 2}, 16000, 1)
	for _, invalid := range [][]byte{nil, []byte("fake-wave"), valid[:len(valid)-1], encodeWAV([]byte{1, 2}, 44100, 1), encodeWAV([]byte{1, 2}, 16000, 2), encodeWAV([]byte{1}, 16000, 1), make([]byte, maxPrivateAudioBytes+1)} {
		if _, err := RecognizePrivateAudio(t.Context(), cfg, invalid, "audio/wav", ""); err == nil {
			t.Fatal("invalid private audio accepted")
		}
	}
	if requests != 0 {
		t.Fatal("invalid private audio reached provider")
	}
	if _, err := readPrivateASRResponse(strings.NewReader(strings.Repeat("x", maxPrivateASRResponseBytes+1))); err == nil {
		t.Fatal("unbounded ASR response accepted")
	}
	adapter := NewSegmentASRAdapter(cfg)
	adapter.AppendPCM(make([]byte, maxPrivateAudioBytes))
	if _, err := adapter.Recognize(t.Context()); err == nil || len(adapter.audioBuf) != 0 {
		t.Fatal("oversized PCM segment was retained or recognized")
	}
	adapter.Reset()
	adapter.AppendPCM([]byte{1, 2})
	if adapter.bufferErr != nil || len(adapter.audioBuf) != 2 {
		t.Fatal("PCM buffer did not recover after explicit reset")
	}
}
