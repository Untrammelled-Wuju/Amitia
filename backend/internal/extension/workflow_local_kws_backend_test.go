package extension

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/realtime"
)

func TestWorkflowLocalKWSCapabilitiesAreLocalAndModelBacked(t *testing.T) {
	detector := &workflowLocalKWSWake{}
	caps := detector.Capabilities()
	if !caps.Supported || !caps.LocalOnly || !caps.RequiresModel || !caps.SupportsCustom {
		t.Fatalf("unexpected local KWS capabilities: %#v", caps)
	}
	if caps.Backend != workflowLocalKWSWakeBackend {
		t.Fatalf("unexpected backend %q", caps.Backend)
	}
}

func TestWorkflowLocalKWSLoadDoesNotRequireCloudCredentials(t *testing.T) {
	detector := &workflowLocalKWSWake{}
	err := detector.Load(t.Context(), realtime.WakeDetectorConfig{Enabled: false})
	if err == nil {
		t.Fatal("disabled config must fail before model loading")
	}
	if got := err.Error(); got != "local wake: config disabled" {
		t.Fatalf("unexpected error %q", got)
	}
}

func TestParseLocalKWSKeywordSupportsJSONAndCLIText(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{name: "json", raw: `{"keyword":"你好 Amitia"}`, want: "你好 Amitia"},
		{name: "cli", raw: "score=0.91 keyword: 打开工作流", want: "打开工作流"},
		{name: "multiline", raw: "debug\n{\"text\":\"start workflow\"}\n", want: "start workflow"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseLocalKWSKeyword([]byte(tc.raw)); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWritePCM16MonoWAVProducesValidHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wake.wav")
	pcm := make([]byte, 320)
	for i := 0; i < len(pcm); i += 2 {
		binary.LittleEndian.PutUint16(pcm[i:i+2], uint16(i))
	}
	if err := writePCM16MonoWAV(path, pcm, localKWSSampleRate); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 44+len(pcm) {
		t.Fatalf("unexpected wav length %d", len(raw))
	}
	if string(raw[:4]) != "RIFF" || string(raw[8:12]) != "WAVE" || string(raw[36:40]) != "data" {
		t.Fatalf("invalid wav header: %q %q %q", raw[:4], raw[8:12], raw[36:40])
	}
	if got := binary.LittleEndian.Uint32(raw[24:28]); got != localKWSSampleRate {
		t.Fatalf("sample rate=%d", got)
	}
	if got := binary.LittleEndian.Uint32(raw[40:44]); got != uint32(len(pcm)) {
		t.Fatalf("data size=%d", got)
	}
}

func TestResolveLocalKWSModelValidatesManifestFilesBeforeRuntime(t *testing.T) {
	dir := t.TempDir()
	manifest := localKWSManifest{
		KeywordSpotter: filepath.Join(dir, "kws-bin"),
		Tokens:         "tokens.txt",
		Encoder:        "encoder.onnx",
		Decoder:        "decoder.onnx",
		Joiner:         "joiner.onnx",
		Provider:       "cpu",
		NumThreads:     1,
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, localKWSManifestName), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{manifest.Tokens, manifest.Encoder, manifest.Decoder, manifest.Joiner, "kws-bin"} {
		mode := os.FileMode(0o600)
		if name == "kws-bin" {
			mode = 0o700
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte("fixture"), mode); err != nil {
			t.Fatal(err)
		}
	}

	resolved, gotManifest, err := resolveLocalKWSModel(dir)
	if err != nil {
		t.Fatal(err)
	}
	if resolved == "" || gotManifest.KeywordSpotter != manifest.KeywordSpotter {
		t.Fatalf("unexpected resolved model: %q %#v", resolved, gotManifest)
	}
}
