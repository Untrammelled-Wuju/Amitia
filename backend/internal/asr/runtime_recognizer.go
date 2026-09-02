package asr

import (
	"context"
	"errors"
	"strings"
)

// ActiveRuntimeConfig returns an internal copy of the active ASR configuration,
// including its credential. It is deliberately kept inside the backend package
// boundary and must never be serialized to a client response.
func ActiveRuntimeConfig() (*AsrConfig, error) {
	if asrService == nil {
		return nil, errors.New("ASR service is not initialized")
	}
	cfg, err := asrService.GetActiveConfig()
	if err != nil || cfg == nil {
		if err == nil {
			err = errors.New("active ASR config is unavailable")
		}
		return nil, err
	}
	copy := *cfg
	return &copy, nil
}

// SupportsSegmentPCM reports whether the configured provider can consume a
// private PCM/WAV segment without requiring a publicly reachable audio URL.
// That property is mandatory for an always-on wake runtime.
func SupportsSegmentPCM(cfg *AsrConfig) bool {
	if cfg == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(cfg.ApiType)) {
	case "openai":
		return true
	case "azure":
		_, err := buildAzureShortAudioURL(cfg.BaseURL, "zh-CN")
		return err == nil
	default:
		return false
	}
}

// RecognizePCM transcribes mono PCM16/16 kHz using the existing segment ASR
// adapter. It is shared by the real workflow wake-word backend so loudness is
// only used for speech segmentation, never as proof that a wake phrase matched.
func RecognizePCM(ctx context.Context, cfg *AsrConfig, pcm []byte, language string) (string, error) {
	if !SupportsSegmentPCM(cfg) {
		return "", errors.New("active ASR provider does not support private PCM segment recognition")
	}
	adapter := NewSegmentASRAdapter(cfg)
	adapter.SetLanguage(strings.TrimSpace(language))
	adapter.AppendPCM(pcm)
	return adapter.Recognize(ctx)
}
