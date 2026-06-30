// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, watch, onUnmounted } from "vue"
import { splitSentences } from "@/utils/typing"

const CHAR_DELAY_MS = 20
const SENTENCE_FLOAT_MIN = 200
const SENTENCE_FLOAT_MAX = 500

function randomFloat(min: number, max: number): number {
  return Math.random() * (max - min) + min
}

export function useTyping(source: () => string, enabled: () => boolean) {
  const displayText = ref("")
  const isTyping = ref(false)
  let timer: ReturnType<typeof setTimeout> | null = null
  let cancelled = false

  function cancel() {
    cancelled = true
    if (timer) {
      clearTimeout(timer)
      timer = null
    }
  }

  function typeOut(target: string, fromIndex: number) {
    if (cancelled) return

    const sentences = splitSentences(target)
    let globalIdx = 0
    let currentDisplay = ""

    function typeSentence(sentenceIdx: number, charIdx: number) {
      if (cancelled) return

      if (sentenceIdx >= sentences.length) {
        isTyping.value = false
        return
      }

      const sentence = sentences[sentenceIdx]

      if (charIdx < sentence.length) {
        currentDisplay += sentence[charIdx]
        displayText.value = currentDisplay
        globalIdx++
        timer = setTimeout(() => typeSentence(sentenceIdx, charIdx + 1), CHAR_DELAY_MS)
      } else {
        const sentenceFloat = randomFloat(SENTENCE_FLOAT_MIN, SENTENCE_FLOAT_MAX)
        timer = setTimeout(() => typeSentence(sentenceIdx + 1, 0), sentenceFloat)
      }
    }

    isTyping.value = true
    displayText.value = ""
    currentDisplay = ""
    typeSentence(0, 0)
  }

  watch(
    source,
    (newVal, oldVal) => {
      if (!enabled()) {
        displayText.value = newVal
        return
      }

      cancel()
      cancelled = false

      if (newVal === oldVal) return

      if (!newVal || newVal.length === 0) {
        displayText.value = ""
        isTyping.value = false
        return
      }

      if (oldVal && newVal.startsWith(oldVal)) {
        const newPart = newVal.slice(oldVal.length)
        const sentences = splitSentences(newPart)
        let currentExtra = ""
        let sentenceIdx = 0
        let charIdx = 0

        function typeExtra() {
          if (cancelled) return
          if (sentenceIdx >= sentences.length) {
            isTyping.value = false
            return
          }
          const sentence = sentences[sentenceIdx]
          if (charIdx < sentence.length) {
            currentExtra += sentence[charIdx]
            displayText.value = oldVal + currentExtra
            charIdx++
            timer = setTimeout(typeExtra, CHAR_DELAY_MS)
          } else {
            const sentenceFloat = randomFloat(SENTENCE_FLOAT_MIN, SENTENCE_FLOAT_MAX)
            timer = setTimeout(() => {
              sentenceIdx++
              charIdx = 0
              typeExtra()
            }, sentenceFloat)
          }
        }

        isTyping.value = true
        displayText.value = oldVal
        typeExtra()
      } else {
        typeOut(newVal, 0)
      }
    },
    { immediate: true }
  )

  onUnmounted(() => {
    cancel()
  })

  return {
    displayText,
    isTyping,
    cancel,
  }
}
