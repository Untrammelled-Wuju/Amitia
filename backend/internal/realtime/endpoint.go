// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"sync"
	"time"
)

type EndpointDetector struct {
	config       EndpointConfig
	mu           sync.Mutex
	state        endpointState
	speechStart  int64
	silenceStart int64
	utteranceMS  int
	preRollBuf   []byte
}

type endpointState int

const (
	endpointStateSilent endpointState = iota
	endpointStateSpeech
	endpointStatePostSpeech
)

func NewEndpointDetector(config EndpointConfig) *EndpointDetector {
	return &EndpointDetector{
		config:     config,
		state:      endpointStateSilent,
		preRollBuf: make([]byte, 0, config.PreRollMS*CanonicalSampleRate*2/1000),
	}
}

func (e *EndpointDetector) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state = endpointStateSilent
	e.speechStart = 0
	e.silenceStart = 0
	e.utteranceMS = 0
	e.preRollBuf = e.preRollBuf[:0]
}

func (e *EndpointDetector) Process(result VADResult, frame *VoiceAudioFrame) EndpointResult {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := result.TimestampNS
	if now == 0 {
		now = time.Now().UnixNano()
	}

	res := EndpointResult{TimestampNS: now}

	switch e.state {
	case endpointStateSilent:
		if result.Speech {
			if e.speechStart == 0 {
				e.speechStart = now
			}
			e.state = endpointStateSpeech
			res.TurnStart = true
			res.DurationMS = 0
		} else {
			e.preRollBuf = append(e.preRollBuf, frame.PCM...)
			maxPreRoll := e.config.PreRollMS * CanonicalSampleRate * 2 / 1000
			if len(e.preRollBuf) > maxPreRoll {
				e.preRollBuf = e.preRollBuf[len(e.preRollBuf)-maxPreRoll:]
			}
		}

	case endpointStateSpeech:
		if result.Speech {
			e.silenceStart = 0
			e.utteranceMS = int((now - e.speechStart) / int64(time.Millisecond))
			if e.utteranceMS >= e.config.MaxUtteranceMS {
				res.TurnEnd = true
				e.state = endpointStateSilent
				e.speechStart = 0
				e.utteranceMS = 0
			}
		} else {
			if e.silenceStart == 0 {
				e.silenceStart = now
			}
			silenceMS := int((now - e.silenceStart) / int64(time.Millisecond))
			speechMS := int((e.silenceStart - e.speechStart) / int64(time.Millisecond))
			if silenceMS >= e.config.EndSilenceMS && speechMS >= e.config.MinSpeechMS {
				res.TurnEnd = true
				e.state = endpointStateSilent
				e.speechStart = 0
				e.silenceStart = 0
				e.utteranceMS = 0
			} else if silenceMS < e.config.EndSilenceMS {
				e.utteranceMS = int((now - e.speechStart) / int64(time.Millisecond))
			}
		}

	case endpointStatePostSpeech:
		if result.Speech {
			e.state = endpointStateSpeech
		} else {
			res.TurnEnd = true
			e.state = endpointStateSilent
		}
	}

	return res
}

func (e *EndpointDetector) GetPreRollAudio() []byte {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.preRollBuf) == 0 {
		return nil
	}
	out := make([]byte, len(e.preRollBuf))
	copy(out, e.preRollBuf)
	return out
}

func (e *EndpointDetector) ReleasePreRollAudio() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.preRollBuf = e.preRollBuf[:0]
}

