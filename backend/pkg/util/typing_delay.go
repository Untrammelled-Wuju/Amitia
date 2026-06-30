// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package util

import (
	"math"
	"time"
	"unicode/utf8"
)

// CalculateTypingDelay 根据文本长度计算模拟真人打字的发送间隔
// 公式: 基准 300ms + 每字 80ms，上限 3000ms，下限 200ms
// 短消息(1-2字): ~300-400ms
// 中等消息(5-10字): ~700-1100ms
// 长消息(20字+): ~1900-3000ms
func CalculateTypingDelay(text string) time.Duration {
	charCount := utf8.RuneCountInString(text)
	ms := 300 + charCount*80
	ms = int(math.Min(float64(ms), 3000))
	ms = int(math.Max(float64(ms), 200))
	return time.Duration(ms) * time.Millisecond
}
