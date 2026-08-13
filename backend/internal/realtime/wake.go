// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type WakeDetector interface {
	Capabilities() WakeCapabilities
	Load(ctx context.Context, cfg WakeDetectorConfig) error
	Reset()
	Process(frame *VoiceAudioFrame) (WakeDetectionResult, error)
	Unload() error
}

var wakeRegistry sync.Map

func RegisterWakeBackend(name string, factory func(configJSON string) (WakeDetector, error)) {
	wakeRegistry.Store(name, factory)
}

func GetWakeBackend(name string) (func(configJSON string) (WakeDetector, error), bool) {
	val, ok := wakeRegistry.Load(name)
	if !ok {
		return nil, false
	}
	factory, ok := val.(func(configJSON string) (WakeDetector, error))
	return factory, ok
}

type softwareWake struct {
	mu            sync.Mutex
	caps          WakeCapabilities
	config        WakeDetectorConfig
	loaded        bool
	cooldownUntil int64
	energyHistory []float64
}

func NewSoftwareWake() WakeDetector {
	return &softwareWake{
		caps: WakeCapabilities{
			Supported:        true,
			LocalOnly:        true,
			MaxPhrases:       10,
			SupportsCustom:   false,
			RequiresModel:    false,
			SupportedLocales: []string{"zh-CN", "en-US"},
			Backend:          "software",
		},
		energyHistory: make([]float64, 0, 50),
	}
}

func (w *softwareWake) Capabilities() WakeCapabilities {
	return w.caps
}

func (w *softwareWake) Load(ctx context.Context, cfg WakeDetectorConfig) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !cfg.Enabled {
		return fmt.Errorf("wake: config disabled")
	}
	if len(cfg.Phrases) == 0 {
		return fmt.Errorf("wake: no phrases configured")
	}

	w.config = cfg
	w.loaded = true
	w.cooldownUntil = 0
	return nil
}

func (w *softwareWake) Reset() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.cooldownUntil = 0
	w.energyHistory = w.energyHistory[:0]
}

func (w *softwareWake) Process(frame *VoiceAudioFrame) (WakeDetectionResult, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	res := WakeDetectionResult{}

	if !w.loaded || !w.config.Enabled {
		return res, nil
	}

	now := time.Now().UnixNano()
	if w.cooldownUntil > 0 && now < w.cooldownUntil {
		return res, nil
	}

	var energy float64
	samples := len(frame.PCM) / 2
	if samples == 0 {
		return res, nil
	}
	for i := 0; i < len(frame.PCM)-1; i += 2 {
		sample := int16(frame.PCM[i]) | int16(frame.PCM[i+1])<<8
		normalized := float64(sample) / 32768.0
		energy += normalized * normalized
	}
	energy = energy / float64(samples)

	w.energyHistory = append(w.energyHistory, energy)
	if len(w.energyHistory) > 50 {
		w.energyHistory = w.energyHistory[1:]
	}

	threshold := w.config.Threshold
	if threshold <= 0 {
		threshold = 0.05
	}

	if energy > threshold*3 && len(w.energyHistory) >= 3 {
		recent := w.energyHistory[len(w.energyHistory)-3:]
		rising := recent[2] > recent[1] && recent[1] > recent[0]
		if rising {
			res.Detected = true
			res.Confidence = energy / (threshold * 5)
			if res.Confidence > 1.0 {
				res.Confidence = 1.0
			}
			res.DetectedAtNS = now
			if len(w.config.Phrases) > 0 {
				res.PhraseID = w.config.Phrases[0].ID
			}
			if w.config.CooldownMS > 0 {
				w.cooldownUntil = now + w.config.CooldownMS*int64(time.Millisecond)
			} else {
				w.cooldownUntil = now + 2*int64(time.Second)
			}
		}
	}

	return res, nil
}

func (w *softwareWake) Unload() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.loaded = false
	w.energyHistory = w.energyHistory[:0]
	return nil
}

func init() {
	RegisterWakeBackend("software", func(configJSON string) (WakeDetector, error) {
		return NewSoftwareWake(), nil
	})
}

