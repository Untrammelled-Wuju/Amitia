// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
const CHAR_DELAY_MS = 20
const SENTENCE_FLOAT_MIN = 200
const SENTENCE_FLOAT_MAX = 500

export function calcTypingDelay(text: string): number {
  const len = [...text].length
  let ms = 300 + len * 80
  if (ms > 3000) ms = 3000
  if (ms < 200) ms = 200
  return ms
}

export function calcCharRevealDelay(text: string): number {
  const len = [...text].length
  return len * CHAR_DELAY_MS + randomFloat(SENTENCE_FLOAT_MIN, SENTENCE_FLOAT_MAX)
}

export function calcTotalTypingDuration(text: string): number {
  const sentences = splitSentences(text)
  let total = 0
  for (const s of sentences) {
    const len = [...s].length
    if (len === 0) continue
    total += len * CHAR_DELAY_MS + randomFloat(SENTENCE_FLOAT_MIN, SENTENCE_FLOAT_MAX)
  }
  return total
}

export function splitSentences(text: string): string[] {
  const result: string[] = []
  let current = ''
  for (const ch of text) {
    current += ch
    if (/[。！？.!?\n]/.test(ch)) {
      if (current.trim()) result.push(current)
      current = ''
    }
  }
  if (current.trim()) result.push(current)
  if (result.length === 0 && text.trim()) result.push(text)
  return result
}

function randomFloat(min: number, max: number): number {
  return Math.random() * (max - min) + min
}
