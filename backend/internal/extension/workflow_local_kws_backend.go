package extension

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	workflowmetrics "github.com/u-ai/backend/internal/extension/kernel/workflow"
	"github.com/u-ai/backend/internal/realtime"
)

const (
	workflowLocalKWSWakeBackend = "workflow_local_kws"
	localKWSManifestName        = "amitia-kws.json"
	localKWSSampleRate          = 16000
	localKWSBytesPerSample      = 2
	localKWSPreRollMS           = 250
	localKWSEndSilenceMS        = 500
	localKWSMinSpeechMS         = 160
	localKWSMaxSpeechMS         = 5000
	localKWSVADEnergy           = 0.0025
)

type localKWSManifest struct {
	KeywordSpotter string `json:"keywordSpotter"`
	Tokens         string `json:"tokens"`
	Encoder        string `json:"encoder"`
	Decoder        string `json:"decoder"`
	Joiner         string `json:"joiner"`
	Provider       string `json:"provider"`
	NumThreads     int    `json:"numThreads"`
}

type workflowLocalKWSWake struct {
	mu sync.Mutex

	cfg      realtime.WakeDetectorConfig
	loaded   bool
	modelDir string
	manifest localKWSManifest

	phraseByNormalized map[string]string
	keywordFile        string
	workDir            string

	preRoll      []byte
	utterance    []byte
	speechMS     int
	silenceMS    int
	inSpeech     bool
	recognizing  bool
	generation   uint64
	pending      []realtime.WakeDetectionResult
	cooldownTill int64

	ctx    context.Context
	cancel context.CancelFunc
}

func newWorkflowLocalKWSWake() realtime.WakeDetector { return &workflowLocalKWSWake{} }

func (w *workflowLocalKWSWake) Capabilities() realtime.WakeCapabilities {
	return realtime.WakeCapabilities{
		Supported:        true,
		LocalOnly:        true,
		MaxPhrases:       16,
		SupportsCustom:   true,
		RequiresModel:    true,
		SupportedLocales: []string{"zh-CN", "en-US"},
		Backend:          workflowLocalKWSWakeBackend,
	}
}

func (w *workflowLocalKWSWake) Load(_ context.Context, cfg realtime.WakeDetectorConfig) error {
	if !cfg.Enabled {
		return errors.New("local wake: config disabled")
	}
	if len(cfg.Phrases) == 0 {
		return errors.New("local wake: no phrases configured")
	}
	modelDir, manifest, err := resolveLocalKWSModel(cfg.ModelResourceURI)
	if err != nil {
		return err
	}
	workDir, err := os.MkdirTemp("", "amitia-local-kws-*")
	if err != nil {
		return fmt.Errorf("local wake: create workspace: %w", err)
	}
	phraseMap := make(map[string]string, len(cfg.Phrases))
	keywordFile := filepath.Join(workDir, "keywords.txt")
	var keywords strings.Builder
	for _, phrase := range cfg.Phrases {
		text := strings.TrimSpace(phrase.DisplayText)
		if text == "" {
			continue
		}
		normalized := normalizeWorkflowWakeText(text)
		if normalized == "" {
			continue
		}
		phraseMap[normalized] = phrase.ID
		keywords.WriteString(text)
		keywords.WriteByte('\n')
	}
	if len(phraseMap) == 0 {
		_ = os.RemoveAll(workDir)
		return errors.New("local wake: phrases are empty after normalization")
	}
	if err := os.WriteFile(keywordFile, []byte(keywords.String()), 0o600); err != nil {
		_ = os.RemoveAll(workDir)
		return fmt.Errorf("local wake: write keyword file: %w", err)
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil {
		w.cancel()
	}
	oldWorkDir := w.workDir
	w.ctx, w.cancel = context.WithCancel(context.Background())
	w.generation++
	w.cfg = cfg
	w.loaded = true
	w.modelDir = modelDir
	w.manifest = manifest
	w.phraseByNormalized = phraseMap
	w.keywordFile = keywordFile
	w.workDir = workDir
	w.recognizing = false
	w.pending = nil
	w.cooldownTill = 0
	w.resetAudioLocked()
	if oldWorkDir != "" && oldWorkDir != workDir {
		_ = os.RemoveAll(oldWorkDir)
	}
	return nil
}

func (w *workflowLocalKWSWake) Reset() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil {
		w.cancel()
	}
	if w.loaded {
		w.ctx, w.cancel = context.WithCancel(context.Background())
	} else {
		w.ctx, w.cancel = nil, nil
	}
	w.generation++
	w.recognizing = false
	w.pending = nil
	w.cooldownTill = 0
	w.resetAudioLocked()
}

func (w *workflowLocalKWSWake) Process(frame *realtime.VoiceAudioFrame) (realtime.WakeDetectionResult, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.loaded || !w.cfg.Enabled {
		return realtime.WakeDetectionResult{}, nil
	}
	if len(w.pending) > 0 {
		result := w.pending[0]
		w.pending = w.pending[1:]
		return result, nil
	}
	if frame == nil || len(frame.PCM) < localKWSBytesPerSample || w.recognizing {
		return realtime.WakeDetectionResult{}, nil
	}
	now := time.Now().UnixNano()
	if w.cooldownTill > now {
		w.resetAudioLocked()
		return realtime.WakeDetectionResult{}, nil
	}
	frameMS := pcmDurationMS(frame.PCM, localKWSSampleRate)
	if frameMS <= 0 {
		return realtime.WakeDetectionResult{}, nil
	}
	speech := pcmRMSEnergy(frame.PCM) >= localKWSVADEnergy
	if !w.inSpeech {
		if !speech {
			w.appendPreRollLocked(frame.PCM)
			return realtime.WakeDetectionResult{}, nil
		}
		w.inSpeech = true
		w.utterance = append(w.utterance[:0], w.preRoll...)
		w.utterance = append(w.utterance, frame.PCM...)
		w.speechMS = frameMS
		w.silenceMS = 0
		return realtime.WakeDetectionResult{}, nil
	}
	w.utterance = append(w.utterance, frame.PCM...)
	if speech {
		w.speechMS += frameMS
		w.silenceMS = 0
	} else {
		w.silenceMS += frameMS
	}
	utteranceMS := pcmDurationMS(w.utterance, localKWSSampleRate)
	shouldRecognize := (w.speechMS >= localKWSMinSpeechMS && w.silenceMS >= localKWSEndSilenceMS) || utteranceMS >= localKWSMaxSpeechMS
	if !shouldRecognize {
		return realtime.WakeDetectionResult{}, nil
	}
	pcm := append([]byte(nil), w.utterance...)
	cfg := w.cfg
	manifest := w.manifest
	modelDir := w.modelDir
	keywordFile := w.keywordFile
	workDir := w.workDir
	phraseMap := cloneStringMap(w.phraseByNormalized)
	parent := w.ctx
	generation := w.generation
	w.recognizing = true
	w.resetAudioLocked()
	go w.recognize(parent, cfg, manifest, modelDir, keywordFile, workDir, phraseMap, pcm, now, generation)
	return realtime.WakeDetectionResult{}, nil
}

func (w *workflowLocalKWSWake) recognize(parent context.Context, cfg realtime.WakeDetectorConfig, manifest localKWSManifest, modelDir, keywordFile, workDir string, phraseMap map[string]string, pcm []byte, detectedAtNS int64, generation uint64) {
	defer clear(pcm)
	ctx, cancel := context.WithTimeout(parent, 8*time.Second)
	defer cancel()
	wavPath := filepath.Join(workDir, fmt.Sprintf("utterance-%d.wav", time.Now().UnixNano()))
	if err := writePCM16MonoWAV(wavPath, pcm, localKWSSampleRate); err != nil {
		w.finishRecognition(generation, realtime.WakeDetectionResult{})
		return
	}
	defer os.Remove(wavPath)

	args := []string{
		"--tokens=" + filepath.Join(modelDir, manifest.Tokens),
		"--encoder=" + filepath.Join(modelDir, manifest.Encoder),
		"--decoder=" + filepath.Join(modelDir, manifest.Decoder),
		"--joiner=" + filepath.Join(modelDir, manifest.Joiner),
		"--keywords-file=" + keywordFile,
		"--provider=" + manifest.Provider,
		"--num-threads=" + strconv.Itoa(manifest.NumThreads),
	}
	threshold := cfg.Threshold
	if threshold > 0 && threshold <= 1 {
		args = append(args, "--keywords-threshold="+strconv.FormatFloat(threshold, 'f', -1, 64))
	}
	args = append(args, wavPath)
	output, err := exec.CommandContext(ctx, manifest.KeywordSpotter, args...).CombinedOutput()
	if err != nil {
		w.finishRecognition(generation, realtime.WakeDetectionResult{})
		return
	}
	keyword := parseLocalKWSKeyword(output)
	normalized := normalizeWorkflowWakeText(keyword)
	phraseID := phraseMap[normalized]
	if phraseID == "" {
		// Some keyword spotter builds append tokens/metadata. Accept only an
		// unambiguous configured phrase contained in the returned keyword.
		for candidate, id := range phraseMap {
			if len([]rune(candidate)) >= 2 && strings.Contains(normalized, candidate) {
				if phraseID != "" && phraseID != id {
					phraseID = ""
					break
				}
				phraseID = id
			}
		}
	}
	if phraseID == "" {
		w.finishRecognition(generation, realtime.WakeDetectionResult{})
		return
	}
	w.finishRecognition(generation, realtime.WakeDetectionResult{Detected: true, PhraseID: phraseID, Confidence: 1, DetectedAtNS: detectedAtNS})
}

func (w *workflowLocalKWSWake) finishRecognition(generation uint64, result realtime.WakeDetectionResult) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if generation != w.generation {
		return
	}
	w.recognizing = false
	if !w.loaded {
		return
	}
	if !result.Detected {
		workflowmetrics.DefaultWorkflowReliabilityMetrics.Inc(workflowmetrics.MetricWakeFalseOrRejectedTotal)
		return
	}
	workflowmetrics.DefaultWorkflowReliabilityMetrics.Inc(workflowmetrics.MetricWakeDetectionTotal)
	if result.DetectedAtNS > 0 {
		workflowmetrics.DefaultWorkflowReliabilityMetrics.Observe(workflowmetrics.MetricWakeDetectionLatencyMS, float64(time.Now().UnixNano()-result.DetectedAtNS)/float64(time.Millisecond))
	}
	w.pending = append(w.pending, result)
	cooldown := w.cfg.CooldownMS
	if cooldown <= 0 {
		cooldown = 2000
	}
	w.cooldownTill = time.Now().Add(time.Duration(cooldown) * time.Millisecond).UnixNano()
}

func (w *workflowLocalKWSWake) Unload() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil {
		w.cancel()
	}
	workDir := w.workDir
	w.cancel, w.ctx = nil, nil
	w.generation++
	w.loaded = false
	w.recognizing = false
	w.pending = nil
	w.modelDir = ""
	w.keywordFile = ""
	w.workDir = ""
	w.phraseByNormalized = nil
	w.resetAudioLocked()
	if workDir != "" {
		return os.RemoveAll(workDir)
	}
	return nil
}

func (w *workflowLocalKWSWake) resetAudioLocked() {
	if len(w.utterance) > 0 {
		clear(w.utterance)
	}
	if len(w.preRoll) > 0 {
		clear(w.preRoll)
	}
	w.utterance = w.utterance[:0]
	w.preRoll = w.preRoll[:0]
	w.speechMS, w.silenceMS = 0, 0
	w.inSpeech = false
}

func (w *workflowLocalKWSWake) appendPreRollLocked(pcm []byte) {
	w.preRoll = append(w.preRoll, pcm...)
	maxBytes := localKWSSampleRate * localKWSBytesPerSample * localKWSPreRollMS / 1000
	if len(w.preRoll) > maxBytes {
		copy(w.preRoll, w.preRoll[len(w.preRoll)-maxBytes:])
		w.preRoll = w.preRoll[:maxBytes]
	}
}

func resolveLocalKWSModel(uri string) (string, localKWSManifest, error) {
	uri = strings.TrimSpace(uri)
	if strings.HasPrefix(uri, "file://") {
		uri = strings.TrimPrefix(uri, "file://")
	}
	if uri == "" || strings.HasPrefix(uri, "builtin://") {
		uri = strings.TrimSpace(os.Getenv("AMITIA_LOCAL_KWS_MODEL_DIR"))
	}
	if uri == "" {
		return "", localKWSManifest{}, errors.New("local wake model is not installed; set modelResourceUri or AMITIA_LOCAL_KWS_MODEL_DIR")
	}
	abs, err := filepath.Abs(uri)
	if err != nil {
		return "", localKWSManifest{}, fmt.Errorf("local wake: resolve model path: %w", err)
	}
	raw, err := os.ReadFile(filepath.Join(abs, localKWSManifestName))
	if err != nil {
		return "", localKWSManifest{}, fmt.Errorf("local wake: read %s: %w", localKWSManifestName, err)
	}
	var manifest localKWSManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return "", localKWSManifest{}, fmt.Errorf("local wake: decode model manifest: %w", err)
	}
	if env := strings.TrimSpace(os.Getenv("AMITIA_SHERPA_KWS_BIN")); env != "" {
		manifest.KeywordSpotter = env
	}
	if strings.TrimSpace(manifest.KeywordSpotter) == "" {
		manifest.KeywordSpotter = "sherpa-onnx-keyword-spotter"
	}
	if manifest.Provider == "" {
		manifest.Provider = "cpu"
	}
	if manifest.NumThreads <= 0 {
		manifest.NumThreads = 1
	}
	for field, value := range map[string]string{"tokens": manifest.Tokens, "encoder": manifest.Encoder, "decoder": manifest.Decoder, "joiner": manifest.Joiner} {
		value = strings.TrimSpace(value)
		if value == "" {
			return "", localKWSManifest{}, fmt.Errorf("local wake model manifest missing %s", field)
		}
		if _, err := os.Stat(filepath.Join(abs, value)); err != nil {
			return "", localKWSManifest{}, fmt.Errorf("local wake model %s unavailable: %w", field, err)
		}
	}
	if strings.ContainsAny(manifest.KeywordSpotter, `/\\`) {
		if _, err := os.Stat(manifest.KeywordSpotter); err != nil {
			return "", localKWSManifest{}, fmt.Errorf("local wake keyword spotter unavailable: %w", err)
		}
	} else if _, err := exec.LookPath(manifest.KeywordSpotter); err != nil {
		return "", localKWSManifest{}, fmt.Errorf("local wake keyword spotter unavailable: %w", err)
	}
	return abs, manifest, nil
}

func parseLocalKWSKeyword(output []byte) string {
	text := strings.TrimSpace(string(output))
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var obj map[string]any
		if json.Unmarshal([]byte(line), &obj) == nil {
			for _, key := range []string{"keyword", "text", "result"} {
				if value, ok := obj[key].(string); ok && strings.TrimSpace(value) != "" {
					return value
				}
			}
		}
		lower := strings.ToLower(line)
		if idx := strings.LastIndex(lower, "keyword:"); idx >= 0 {
			return strings.TrimSpace(line[idx+len("keyword:"):])
		}
	}
	return text
}

func writePCM16MonoWAV(path string, pcm []byte, sampleRate int) error {
	if sampleRate <= 0 || len(pcm)%2 != 0 {
		return errors.New("local wake: invalid PCM16 audio")
	}
	var out bytes.Buffer
	dataSize := uint32(len(pcm))
	_ = binary.Write(&out, binary.LittleEndian, [4]byte{'R', 'I', 'F', 'F'})
	_ = binary.Write(&out, binary.LittleEndian, uint32(36)+dataSize)
	_, _ = io.WriteString(&out, "WAVEfmt ")
	_ = binary.Write(&out, binary.LittleEndian, uint32(16))
	_ = binary.Write(&out, binary.LittleEndian, uint16(1))
	_ = binary.Write(&out, binary.LittleEndian, uint16(1))
	_ = binary.Write(&out, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(&out, binary.LittleEndian, uint32(sampleRate*2))
	_ = binary.Write(&out, binary.LittleEndian, uint16(2))
	_ = binary.Write(&out, binary.LittleEndian, uint16(16))
	_, _ = io.WriteString(&out, "data")
	_ = binary.Write(&out, binary.LittleEndian, dataSize)
	_, _ = out.Write(pcm)
	return os.WriteFile(path, out.Bytes(), 0o600)
}

func cloneStringMap(input map[string]string) map[string]string {
	result := make(map[string]string, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func init() {
	realtime.RegisterWakeBackend(workflowLocalKWSWakeBackend, func(string) (realtime.WakeDetector, error) {
		return newWorkflowLocalKWSWake(), nil
	})
}
