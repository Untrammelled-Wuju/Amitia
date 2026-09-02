package asr

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildAzureShortAudioURL(t *testing.T) {
	got, err := buildAzureShortAudioURL("https://eastus.stt.speech.microsoft.com", "en-US")
	if err != nil {
		t.Fatalf("build endpoint: %v", err)
	}
	if !strings.Contains(got, "/speech/recognition/conversation/cognitiveservices/v1") {
		t.Fatalf("missing short-audio v1 path: %s", got)
	}
	if !strings.Contains(got, "language=en-US") {
		t.Fatalf("missing language query: %s", got)
	}
}

func TestBuildAzureShortAudioURLRejectsRegionlessDefault(t *testing.T) {
	if _, err := buildAzureShortAudioURL("https://stt.speech.microsoft.com", "zh-CN"); err == nil {
		t.Fatal("expected regionless Azure Speech endpoint to be rejected")
	}
}

func TestSupportsSegmentPCMAzureRequiresUsableEndpoint(t *testing.T) {
	if SupportsSegmentPCM(&AsrConfig{ApiType: "azure", BaseURL: "https://stt.speech.microsoft.com"}) {
		t.Fatal("regionless Azure endpoint must not be advertised for private segment recognition")
	}
	if !SupportsSegmentPCM(&AsrConfig{ApiType: "azure", BaseURL: "https://westus2.stt.speech.microsoft.com"}) {
		t.Fatal("regional Azure endpoint should support private segment recognition")
	}
}

func TestFetchAudioDataRestrictsLocalFilesToSegmentTemp(t *testing.T) {
	arbitrary := filepath.Join(t.TempDir(), "secret.wav")
	if err := os.WriteFile(arbitrary, []byte("not-a-real-wave"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := fetchAudioData("file://" + arbitrary); err == nil {
		t.Fatal("arbitrary local file must not be readable through ASR submit")
	}

	segment := filepath.Join(os.TempDir(), "amitia_segment_test_guard.wav")
	defer os.Remove(segment)
	if err := os.WriteFile(segment, []byte("wave"), 0o600); err != nil {
		t.Fatal(err)
	}
	data, name, err := fetchAudioData("file://" + segment)
	if err != nil {
		t.Fatalf("segment temp should be readable: %v", err)
	}
	if string(data) != "wave" || name != filepath.Base(segment) {
		t.Fatalf("unexpected segment result: name=%q data=%q", name, string(data))
	}

	symlink := filepath.Join(os.TempDir(), "amitia_segment_symlink_guard.wav")
	defer os.Remove(symlink)
	if err := os.Symlink(arbitrary, symlink); err == nil {
		if _, _, err := fetchAudioData("file://" + symlink); err == nil {
			t.Fatal("segment temp symlink must not be readable")
		}
	}
}
