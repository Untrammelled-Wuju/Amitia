// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"math"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

const cascadeTTSBytesPerMS = 48.0

type cascadeDeliveryRecord struct {
	CallID         string
	TurnID         string
	GenerationID   uint64
	GeneratedText  string
	DeliveredText  string
	Interrupted    bool
	DeliveredRatio float64
}

type cascadePlaybackTracker struct {
	mu            sync.Mutex
	generation    uint64
	turnID        string
	turnIndex     int
	generatedText string
	userText      string
	userAffect    CascadeUserAffect
	userSignals   CascadeUserSignals
	audioBytes    int64
	playedMS      int64
	receivedMS    int64
}

func newCascadePlaybackTracker(gen uint64) *cascadePlaybackTracker {
	return &cascadePlaybackTracker{generation: gen, turnID: strconv.FormatUint(gen, 10), turnIndex: -1}
}

func (p *cascadePlaybackTracker) SetTurnIndex(idx int) {
	p.mu.Lock()
	p.turnIndex = idx
	p.mu.Unlock()
}

func (p *cascadePlaybackTracker) TurnIndex() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.turnIndex
}

func (p *cascadePlaybackTracker) SetGeneratedText(value string) {
	p.mu.Lock()
	p.generatedText = strings.TrimSpace(value)
	p.mu.Unlock()
}

func (p *cascadePlaybackTracker) SetUserText(value string) {
	p.mu.Lock()
	p.userText = strings.TrimSpace(value)
	p.mu.Unlock()
}

func (p *cascadePlaybackTracker) UserText() (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.userText, p.userText != ""
}

func (p *cascadePlaybackTracker) SetUserAffect(value CascadeUserAffect) {
	p.mu.Lock()
	p.userAffect = value
	p.mu.Unlock()
}

func (p *cascadePlaybackTracker) UserAffect() (CascadeUserAffect, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.userAffect, p.userAffect.PrimaryEmotion != ""
}

func (p *cascadePlaybackTracker) SetUserSignals(value CascadeUserSignals) {
	p.mu.Lock()
	p.userSignals = value
	p.mu.Unlock()
}

func (p *cascadePlaybackTracker) UserSignals() (CascadeUserSignals, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.userSignals, p.userSignals.UtteranceDurationMS > 0 || p.userSignals.VolumeRMSMean > 0
}

func (p *cascadePlaybackTracker) AddAudioBytes(count int) {
	if count <= 0 {
		return
	}
	p.mu.Lock()
	p.audioBytes += int64(count)
	p.mu.Unlock()
}

func (p *cascadePlaybackTracker) EstimatedDurationMS() int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return int64(math.Round(float64(p.audioBytes) / cascadeTTSBytesPerMS))
}

func (p *cascadePlaybackTracker) UpdateProgress(playedMS, receivedMS int64) {
	p.mu.Lock()
	if playedMS > p.playedMS {
		p.playedMS = playedMS
	}
	if receivedMS > p.receivedMS {
		p.receivedMS = receivedMS
	}
	p.mu.Unlock()
}

func (p *cascadePlaybackTracker) Delivery(callID string, interrupted bool) (cascadeDeliveryRecord, string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	totalMS := int64(math.Round(float64(p.audioBytes) / cascadeTTSBytesPerMS))
	if p.receivedMS > totalMS {
		totalMS = p.receivedMS
	}
	ratio := 1.0
	if interrupted {
		ratio = 0
		if totalMS > 0 && p.playedMS > 0 {
			ratio = float64(p.playedMS) / float64(totalMS)
		}
		if ratio < 0 {
			ratio = 0
		}
		if ratio > 1 {
			ratio = 1
		}
	}
	delivered := cascadePrefixByRatio(p.generatedText, ratio)
	return cascadeDeliveryRecord{
		CallID:         callID,
		TurnID:         p.turnID,
		GenerationID:   p.generation,
		GeneratedText:  p.generatedText,
		DeliveredText:  delivered,
		Interrupted:    interrupted,
		DeliveredRatio: ratio,
	}, delivered
}

func cascadePrefixByRatio(text string, ratio float64) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 || ratio <= 0 {
		return ""
	}
	if ratio >= 1 {
		return string(runes)
	}
	totalWeight := 0.0
	for _, r := range runes {
		totalWeight += cascadeSpeechRuneWeight(r)
	}
	if totalWeight <= 0 {
		return ""
	}
	target := totalWeight * ratio
	consumed := 0.0
	cut := 0
	for i, r := range runes {
		weight := cascadeSpeechRuneWeight(r)
		if consumed+weight > target {
			break
		}
		consumed += weight
		cut = i + 1
	}
	if cut < 1 {
		return ""
	}
	return strings.TrimSpace(string(runes[:cut]))
}

func cascadeSpeechRuneWeight(r rune) float64 {
	switch {
	case unicode.IsSpace(r):
		return 0.08
	case unicode.IsPunct(r):
		switch r {
		case '。', '！', '？', '!', '?', '；', ';':
			return 0.85
		case '，', ',', '、', '：', ':':
			return 0.50
		default:
			return 0.30
		}
	case unicode.Is(unicode.Han, r), unicode.Is(unicode.Hiragana, r), unicode.Is(unicode.Katakana, r), unicode.Is(unicode.Hangul, r):
		return 1.0
	case unicode.IsLetter(r), unicode.IsDigit(r):
		return 0.45
	default:
		return 0.55
	}
}
