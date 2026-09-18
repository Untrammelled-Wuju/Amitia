// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"strings"
	"sync"
)

type cascadeContextCompiler struct {
	mu sync.Mutex

	recent          []cascadeVoiceTurn
	rolling         []string
	semanticSummary string
	recentLimit     int
	rollingLimit    int
}

func newCascadeContextCompiler() *cascadeContextCompiler {
	return &cascadeContextCompiler{recentLimit: 12, rollingLimit: 24}
}

func (c *cascadeContextCompiler) Snapshot() ([]cascadeVoiceTurn, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	recent := append([]cascadeVoiceTurn(nil), c.recent...)
	return recent, c.renderSummaryLocked()
}

func (c *cascadeContextCompiler) AddUser(text string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recent = append(c.recent, cascadeVoiceTurn{UserText: strings.TrimSpace(text)})
	c.compactLocked()
	return len(c.recent) - 1
}

func (c *cascadeContextCompiler) SetAssistantAt(idx int, text string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if idx >= 0 && idx < len(c.recent) {
		c.recent[idx].SpeechText = text
	}
}

func (c *cascadeContextCompiler) SummaryCandidate(minLines int) (existing, raw string, count int, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if minLines <= 0 {
		minLines = 6
	}
	if len(c.rolling) < minLines {
		return "", "", 0, false
	}
	count = len(c.rolling)
	return c.semanticSummary, strings.Join(append([]string(nil), c.rolling...), "\n"), count, true
}

func (c *cascadeContextCompiler) ApplySemanticSummary(summary, candidateRaw string, consumed int) {
	summary = strings.TrimSpace(summary)
	if summary == "" || consumed <= 0 {
		return
	}
	candidateLines := strings.Split(strings.TrimSpace(candidateRaw), "\n")
	if len(candidateLines) == 0 {
		return
	}
	if consumed > len(candidateLines) {
		consumed = len(candidateLines)
	}
	candidateLines = candidateLines[:consumed]

	c.mu.Lock()
	defer c.mu.Unlock()
	c.semanticSummary = cascadeTruncateRunes(summary, 1600)

	remove := 0
	for dropped := 0; dropped < len(candidateLines); dropped++ {
		remaining := candidateLines[dropped:]
		if len(c.rolling) < len(remaining) {
			continue
		}
		matched := true
		for i := range remaining {
			if c.rolling[i] != remaining[i] {
				matched = false
				break
			}
		}
		if matched {
			remove = len(remaining)
			break
		}
	}
	if remove > 0 {
		c.rolling = append([]string(nil), c.rolling[remove:]...)
	}
}

func (c *cascadeContextCompiler) compactLocked() {
	for len(c.recent) > c.recentLimit {
		turn := c.recent[0]
		c.recent = c.recent[1:]
		parts := make([]string, 0, 2)
		if turn.UserText != "" {
			parts = append(parts, "用户："+cascadeTruncateRunes(cascadeSingleLine(turn.UserText), 220))
		}
		if turn.SpeechText != "" {
			parts = append(parts, "AI："+cascadeTruncateRunes(cascadeSingleLine(turn.SpeechText), 220))
		}
		if len(parts) > 0 {
			c.rolling = append(c.rolling, strings.Join(parts, " / "))
		}
	}
	if len(c.rolling) > c.rollingLimit {
		c.rolling = append([]string(nil), c.rolling[len(c.rolling)-c.rollingLimit:]...)
	}
}

func (c *cascadeContextCompiler) renderSummaryLocked() string {
	parts := make([]string, 0, 2)
	if strings.TrimSpace(c.semanticSummary) != "" {
		parts = append(parts, strings.TrimSpace(c.semanticSummary))
	}
	if len(c.rolling) > 0 {
		parts = append(parts, "【尚未语义压缩的较早片段】\n"+strings.Join(c.rolling, "\n"))
	}
	return strings.Join(parts, "\n\n")
}

func cascadeSingleLine(value string) string {
	value = strings.ReplaceAll(value, "\r\n", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	return strings.TrimSpace(value)
}

func cascadeTruncateRunes(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit]) + "…"
}
