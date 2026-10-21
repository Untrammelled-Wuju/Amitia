package wake

import (
	"context"
	"errors"
	"sync"
	"time"
)

type softwareDetector struct {
	mu            sync.Mutex
	config        WakeDetectorConfig
	loaded        bool
	cooldownUntil int64
	energyHistory []float64
}

func NewSoftwareDetector() WakeDetector {
	return &softwareDetector{energyHistory: make([]float64, 0, 50)}
}

func (d *softwareDetector) Capabilities() WakeCapabilities {
	return WakeCapabilities{Supported: true, LocalOnly: true, MaxPhrases: 10, SupportedLocales: []string{"zh-CN", "en-US"}, Backend: "software"}
}

func (d *softwareDetector) Load(_ context.Context, config WakeDetectorConfig) error {
	if !config.Enabled {
		return errors.New("wake: config disabled")
	}
	if len(config.Phrases) == 0 {
		return errors.New("wake: no phrases configured")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.config = config
	d.loaded = true
	d.cooldownUntil = 0
	d.energyHistory = d.energyHistory[:0]
	return nil
}

func (d *softwareDetector) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cooldownUntil = 0
	d.energyHistory = d.energyHistory[:0]
}

func (d *softwareDetector) Process(frame *VoiceAudioFrame) (WakeDetectionResult, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.loaded || !d.config.Enabled || frame == nil {
		return WakeDetectionResult{}, nil
	}
	now := time.Now().UnixNano()
	if d.cooldownUntil > 0 && now < d.cooldownUntil {
		return WakeDetectionResult{}, nil
	}
	samples := len(frame.PCM) / 2
	if samples == 0 {
		return WakeDetectionResult{}, nil
	}
	var energy float64
	for index := 0; index+1 < len(frame.PCM); index += 2 {
		sample := (int(frame.PCM[index+1])<<8 | int(frame.PCM[index])) << 16 >> 16
		normalized := float64(sample) / 32768
		energy += normalized * normalized
	}
	energy /= float64(samples)
	d.energyHistory = append(d.energyHistory, energy)
	if len(d.energyHistory) > 50 {
		d.energyHistory = d.energyHistory[1:]
	}
	threshold := d.config.Threshold
	if threshold <= 0 {
		threshold = 0.05
	}
	if energy <= threshold*3 || len(d.energyHistory) < 3 {
		return WakeDetectionResult{}, nil
	}
	recent := d.energyHistory[len(d.energyHistory)-3:]
	if recent[2] <= recent[1] || recent[1] <= recent[0] {
		return WakeDetectionResult{}, nil
	}
	confidence := energy / (threshold * 5)
	if confidence > 1 {
		confidence = 1
	}
	cooldown := d.config.CooldownMS
	if cooldown <= 0 {
		cooldown = 2000
	}
	d.cooldownUntil = now + cooldown*int64(time.Millisecond)
	phraseID := ""
	if len(d.config.Phrases) > 0 {
		phraseID = d.config.Phrases[0].ID
	}
	return WakeDetectionResult{Detected: true, PhraseID: phraseID, Confidence: confidence, DetectedAtNS: now}, nil
}

func (d *softwareDetector) Unload() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.loaded = false
	d.energyHistory = d.energyHistory[:0]
	return nil
}

func init() {
	RegisterWakeBackend("software", func(string) (WakeDetector, error) {
		return NewSoftwareDetector(), nil
	})
}
