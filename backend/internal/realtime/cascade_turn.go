// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"math"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type cascadeTurnDecisionType string

const (
	cascadeTurnNone            cascadeTurnDecisionType = "NONE"
	cascadeTurnHold            cascadeTurnDecisionType = "HOLD"
	cascadeTurnBackchannel     cascadeTurnDecisionType = "BACKCHANNEL"
	cascadeTurnCompletionCheck cascadeTurnDecisionType = "COMPLETION_CHECK"
	cascadeTurnCommit          cascadeTurnDecisionType = "COMMIT"
)

type cascadeTurnDecision struct {
	Type    cascadeTurnDecisionType
	Text    string
	Partial bool
}

type cascadeTurnThresholds struct {
	VeryShortPauseMS    int
	PartialStableMS     int
	NormalTurnGapMS     int
	HoldWindowMS        int
	BackchannelWindowMS int
	CompletionCheckMS   int
	LongSilenceMS       int
}

func defaultCascadeTurnThresholds() cascadeTurnThresholds {
	return cascadeTurnThresholds{
		VeryShortPauseMS:    280,
		PartialStableMS:     200,
		NormalTurnGapMS:     420,
		HoldWindowMS:        800,
		BackchannelWindowMS: 1250,
		CompletionCheckMS:   2200,
		LongSilenceMS:       3200,
	}
}

type cascadeTurnController struct {
	mu sync.Mutex

	thresholds cascadeTurnThresholds

	pendingText         string
	latestPartial       string
	partialStableAt     time.Time
	finalAt             time.Time
	finalReceived       bool
	awaitingFinal       bool
	committedPartial    string
	speechEndAt         time.Time
	speechActive        bool
	backchannelSent     bool
	completionCheckSent bool
	backchannelCursor   uint64
	completionCursor    uint64
}

func newCascadeTurnController(thresholds cascadeTurnThresholds) *cascadeTurnController {
	return &cascadeTurnController{thresholds: thresholds}
}

func (t *cascadeTurnController) OnSpeechStart() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.speechActive = true
	t.speechEndAt = time.Time{}
	t.partialStableAt = time.Time{}
	t.finalAt = time.Time{}
	t.finalReceived = false
	t.awaitingFinal = false
	t.committedPartial = ""
	t.backchannelSent = false
	t.completionCheckSent = false
}

func (t *cascadeTurnController) OnSpeechEnd(now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.speechActive = false
	t.speechEndAt = now
}

func (t *cascadeTurnController) OnPartial(text string, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if t.awaitingFinal {
		return
	}
	if text != t.latestPartial || t.partialStableAt.IsZero() {
		t.partialStableAt = now
	}
	t.latestPartial = text
	t.finalAt = time.Time{}
	t.finalReceived = false
	t.backchannelSent = false
	t.completionCheckSent = false
}

func (t *cascadeTurnController) OnFinal(text string, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if t.awaitingFinal {
		t.awaitingFinal = false
		t.committedPartial = ""
		return
	}
	t.pendingText = mergeCascadeFinalText(t.pendingText, text)
	t.latestPartial = ""
	t.finalAt = now
	t.finalReceived = true
	t.backchannelSent = false
	t.completionCheckSent = false
}

func (t *cascadeTurnController) HasPending() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return strings.TrimSpace(t.pendingText) != ""
}

func (t *cascadeTurnController) HasActivity() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.speechActive || strings.TrimSpace(t.latestPartial) != "" || strings.TrimSpace(t.pendingText) != ""
}

func (t *cascadeTurnController) PendingText() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if text := strings.TrimSpace(t.pendingText); text != "" {
		return text
	}
	return strings.TrimSpace(t.latestPartial)
}

func (t *cascadeTurnController) MarkInterruptedTurn() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.backchannelSent = false
	t.completionCheckSent = false
}

func (t *cascadeTurnController) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pendingText = ""
	t.latestPartial = ""
	t.partialStableAt = time.Time{}
	t.finalAt = time.Time{}
	t.finalReceived = false
	t.awaitingFinal = false
	t.committedPartial = ""
	t.speechEndAt = time.Time{}
	t.speechActive = false
	t.backchannelSent = false
	t.completionCheckSent = false
}

func (t *cascadeTurnController) Decide(now time.Time) cascadeTurnDecision {
	t.mu.Lock()
	defer t.mu.Unlock()

	finalText := strings.TrimSpace(t.pendingText)
	partialText := strings.TrimSpace(t.latestPartial)
	partialOnly := false
	text := finalText
	if text == "" && partialText != "" && !t.partialStableAt.IsZero() {
		text = partialText
		partialOnly = true
	}
	if text == "" || t.speechActive || (!t.finalReceived && !partialOnly) {
		return cascadeTurnDecision{Type: cascadeTurnNone}
	}

	anchor := t.finalAt
	if !t.speechEndAt.IsZero() {
		anchor = t.speechEndAt
	}
	silence := now.Sub(anchor)
	if silence < 0 {
		silence = 0
	}

	veryShortPause, normalGap, holdWindow, backchannelWindow, completionWindow, longSilence := t.thresholdsLocked()
	incomplete := cascadeSemanticTurnIncomplete(text)

	if silence < veryShortPause {
		return cascadeTurnDecision{Type: cascadeTurnHold}
	}

	partialStable := !partialOnly || now.Sub(t.partialStableAt) >= t.partialStableWindowLocked()
	if !incomplete && silence >= normalGap && partialStable {
		committed := text
		t.resetCommittedLocked(committed, partialOnly)
		return cascadeTurnDecision{Type: cascadeTurnCommit, Text: committed, Partial: partialOnly}
	}

	if incomplete {
		if silence < holdWindow {
			return cascadeTurnDecision{Type: cascadeTurnHold}
		}
		if !t.backchannelSent && silence >= backchannelWindow {
			t.backchannelSent = true
			text := chooseCascadeBackchannel(text, t.backchannelCursor)
			t.backchannelCursor++
			return cascadeTurnDecision{Type: cascadeTurnBackchannel, Text: text}
		}
		if !t.completionCheckSent && silence >= completionWindow {
			t.completionCheckSent = true
			checks := []string{"还有吗？", "你说完啦？"}
			text := checks[t.completionCursor%uint64(len(checks))]
			t.completionCursor++
			return cascadeTurnDecision{Type: cascadeTurnCompletionCheck, Text: text}
		}
		if silence >= longSilence {
			committed := text
			t.resetCommittedLocked(committed, partialOnly)
			return cascadeTurnDecision{Type: cascadeTurnCommit, Text: committed, Partial: partialOnly}
		}
		return cascadeTurnDecision{Type: cascadeTurnHold}
	}

	return cascadeTurnDecision{Type: cascadeTurnNone}
}

func (t *cascadeTurnController) partialStableWindowLocked() time.Duration {
	value := t.thresholds.PartialStableMS
	if value <= 0 {
		value = 200
	}
	return time.Duration(value) * time.Millisecond
}

func (t *cascadeTurnController) thresholdsLocked() (veryShortPause, normalGap, holdWindow, backchannelWindow, completionWindow, longSilence time.Duration) {
	ms := func(value int, fallback int) float64 {
		if value <= 0 {
			return float64(fallback)
		}
		return float64(value)
	}
	veryShort := ms(t.thresholds.VeryShortPauseMS, 280)
	normal := ms(t.thresholds.NormalTurnGapMS, 420)
	hold := ms(t.thresholds.HoldWindowMS, 800)
	back := ms(t.thresholds.BackchannelWindowMS, 1250)
	completion := ms(t.thresholds.CompletionCheckMS, 2200)
	long := ms(t.thresholds.LongSilenceMS, 3200)
	if normal < veryShort {
		normal = veryShort
	}
	if hold < normal {
		hold = normal
	}
	if back < hold {
		back = hold
	}
	if completion < back {
		completion = back
	}
	if long < completion {
		long = completion
	}
	return time.Duration(veryShort) * time.Millisecond,
		time.Duration(normal) * time.Millisecond,
		time.Duration(hold) * time.Millisecond,
		time.Duration(back) * time.Millisecond,
		time.Duration(completion) * time.Millisecond,
		time.Duration(long) * time.Millisecond
}

func (t *cascadeTurnController) resetCommittedLocked(committed string, partialOnly bool) {
	t.pendingText = ""
	t.latestPartial = ""
	t.partialStableAt = time.Time{}
	t.finalAt = time.Time{}
	t.finalReceived = false
	t.speechEndAt = time.Time{}
	t.awaitingFinal = partialOnly
	t.committedPartial = ""
	if partialOnly {
		t.committedPartial = committed
	}
	t.backchannelSent = false
	t.completionCheckSent = false
}

func mergeCascadeFinalText(existing, next string) string {
	existing = strings.TrimSpace(existing)
	next = strings.TrimSpace(next)
	if existing == "" {
		return next
	}
	if existing == next || strings.HasSuffix(existing, next) {
		return existing
	}
	if strings.HasPrefix(next, existing) {
		return next
	}
	if strings.HasSuffix(existing, "，") || strings.HasSuffix(existing, ",") || strings.HasSuffix(existing, "。") || strings.HasSuffix(existing, "！") || strings.HasSuffix(existing, "？") || strings.HasSuffix(existing, "!") || strings.HasSuffix(existing, "?") {
		return existing + next
	}
	return existing + "，" + next
}

func cascadeSemanticTurnIncomplete(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return true
	}
	if strings.HasSuffix(text, "……") || strings.HasSuffix(text, "...") || strings.HasSuffix(text, "…") {
		return true
	}
	normalized := strings.TrimRight(text, "，,、：:；;。！？!?~～ \t\r\n")
	trailing := []string{
		"其实", "然后", "但是", "可是", "不过", "所以", "因为", "如果", "虽然", "而且", "还有", "就是", "就是我", "我就是", "我想", "我想说", "怎么说", "怎么讲", "那个", "就是那个", "后来", "结果", "接着", "和", "跟", "以及", "或者", "还是", "但",
	}
	for _, suffix := range trailing {
		if strings.HasSuffix(normalized, suffix) {
			return true
		}
	}
	if utf8.RuneCountInString(normalized) <= 2 && (strings.Contains(normalized, "嗯") || strings.Contains(normalized, "呃") || strings.Contains(normalized, "额")) {
		return true
	}
	return false
}

func chooseCascadeBackchannel(text string, cursor uint64) string {
	text = strings.TrimSpace(text)
	var options []string
	switch {
	case strings.Contains(text, "不知道怎么") || strings.Contains(text, "不知道该") || strings.Contains(text, "怎么说") || strings.Contains(text, "怎么讲"):
		options = []string{"慢慢说。", "我听着。", "没事，你慢慢说。"}
	case strings.Contains(text, "其实") || strings.Contains(text, "但是") || strings.Contains(text, "然后"):
		options = []string{"嗯？", "你说。", "我听着。"}
	default:
		options = []string{"我听着。", "你说。", "嗯。"}
	}
	return options[cursor%uint64(len(options))]
}

func cascadeAudioEnergy(data []byte) float64 {
	if len(data) == 0 {
		return 0
	}
	var sum float64
	step := 1
	if len(data) > 1024 {
		step = len(data) / 512
	}
	count := 0
	for i := 0; i+1 < len(data); i += step {
		val := float64(int16(data[i])<<8|int16(data[i+1])) / 32768.0
		sum += val * val
		count++
	}
	if count == 0 {
		return 0
	}
	energy := sum / float64(count)
	if energy > 1.0 {
		energy = 1.0
	}
	if energy < 0 {
		energy = 0
	}
	if math.IsNaN(energy) {
		return 0
	}
	return energy
}
