// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"fmt"
	"sync"
)

type VoiceActivityDetector interface {
	Capabilities() VADCapabilities
	Reset()
	Process(frame *VoiceAudioFrame) (VADResult, error)
}

var vadRegistry sync.Map

func RegisterVADBackend(name string, factory func(configJSON string) (VoiceActivityDetector, error)) {
	vadRegistry.Store(name, factory)
}

func GetVADBackend(name string) (func(configJSON string) (VoiceActivityDetector, error), bool) {
	val, ok := vadRegistry.Load(name)
	if !ok {
		return nil, false
	}
	factory, ok := val.(func(configJSON string) (VoiceActivityDetector, error))
	return factory, ok
}

type softwareVAD struct {
	caps      VADCapabilities
	threshold float64
	speech    bool
}

func NewSoftwareVAD(threshold float64) VoiceActivityDetector {
	return &softwareVAD{
		caps: VADCapabilities{
			SupportsStreaming:    true,
			SupportedSampleRates: []int{8000, 16000, 32000, 48000},
			MinFrameMS:           10,
			MaxFrameMS:           100,
			ProvidesProbability:  true,
			Backend:              "software",
		},
		threshold: threshold,
	}
}

func (v *softwareVAD) Capabilities() VADCapabilities {
	return v.caps
}

func (v *softwareVAD) Reset() {
	v.speech = false
}

func (v *softwareVAD) Process(frame *VoiceAudioFrame) (VADResult, error) {
	if frame == nil || len(frame.PCM) == 0 {
		return VADResult{}, fmt.Errorf("vad: empty frame")
	}

	var sumSquares float64
	samples := len(frame.PCM) / 2
	if samples == 0 {
		return VADResult{}, fmt.Errorf("vad: no samples")
	}

	for i := 0; i < len(frame.PCM)-1; i += 2 {
		sample := int16(frame.PCM[i]) | int16(frame.PCM[i+1])<<8
		normalized := float64(sample) / 32768.0
		sumSquares += normalized * normalized
	}

	rms := sqrt(sumSquares / float64(samples))
	probability := rms / v.threshold
	if probability > 1.0 {
		probability = 1.0
	}

	speechNow := rms > v.threshold
	result := VADResult{
		Speech:      speechNow,
		Probability: probability,
		TimestampNS: frame.TimestampNS,
	}

	if speechNow && !v.speech {
		result.SpeechStarted = true
	} else if !speechNow && v.speech {
		result.SpeechEnded = true
	}
	v.speech = speechNow

	return result, nil
}

func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x
	for i := 0; i < 20; i++ {
		z = (z + x/z) / 2
	}
	return z
}

func init() {
	RegisterVADBackend("software", func(configJSON string) (VoiceActivityDetector, error) {
		return NewSoftwareVAD(0.02), nil
	})
}

