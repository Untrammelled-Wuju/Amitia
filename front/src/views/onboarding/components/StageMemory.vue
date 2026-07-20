<template>
  <div class="ob-stage-inner">
    <div class="ob-memory-scene" ref="memorySceneRef" :data-memory-step="complete ? 'complete' : step">
      <div class="ob-memory-prompt" ref="memoryPromptRef">
        <div class="ob-character-line">{{ question }}</div>
        <div class="ob-memory-context">{{ context }}</div>
      </div>

      <div class="ob-memory-ledger" ref="memoryLedgerRef">
        <div class="ob-memory-ledger-title">已记录的信息</div>
        <div class="ob-memory-items">
          <div class="ob-memory-card" :class="{ filled: items[0] }">
            <span class="ob-memory-key">称呼</span>
            <span class="ob-memory-value">{{ items[0] || '等待回答' }}</span>
          </div>
          <div class="ob-memory-card" :class="{ filled: items[1] }">
            <span class="ob-memory-key">交流偏好</span>
            <span class="ob-memory-value">{{ items[1] || '等待回答' }}</span>
          </div>
          <div class="ob-memory-card" :class="{ filled: items[2] }">
            <span class="ob-memory-key">重要信息</span>
            <span class="ob-memory-value">{{ items[2] || '等待回答' }}</span>
          </div>
        </div>
      </div>

      <div class="ob-memory-answer" ref="memoryAnswerRef" :class="{ complete }">
        <textarea
          v-model="inputValue"
          rows="1"
          :placeholder="placeholder"
          @keydown.enter.prevent="handleSend"
        ></textarea>
        <button @click="handleSend" :disabled="!inputValue.trim()">↑</button>
      </div>

      <div class="ob-memory-quick-choices" ref="memoryQuickRef" :class="{ show: quickChoices.length > 0 && !complete }">
        <button
          v-for="choice in quickChoices"
          :key="choice"
          class="ob-memory-quick-option"
          @click="selectChoice(choice)"
        >{{ choice }}</button>
      </div>

      <div class="ob-memory-step-markers" v-if="!complete">
        <span v-for="i in 3" :key="i"
          :class="{ active: step === i - 1, done: step > i - 1 }"
        ></span>
      </div>

      <div v-if="complete" class="ob-memory-complete-view">
        <div class="ob-memory-complete-copy">
          <div class="kicker">初始信息已保存</div>
          <h2>基础信息已经准备好了。</h2>
          <p>你的称呼、交流偏好和第一条重要信息已经记录。更多了解，会在之后的相处中自然形成。</p>
        </div>
        <div class="ob-memory-complete-summary">
          <div class="ob-memory-summary-row" v-for="(label, i) in ['称呼', '交流偏好', '重要信息']" :key="label">
            <span class="ob-memory-summary-index">{{ String(i + 1).padStart(2, '0') }}</span>
            <div>
              <div class="ob-memory-summary-label">{{ label }}</div>
              <div class="ob-memory-summary-value">{{ items[i] || '尚未记录' }}</div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <button v-if="complete" class="ob-memory-complete-next" @click="$emit('next')">继续</button>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from "vue"

const props = defineProps<{
  step: number
  question: string
  context: string
  placeholder: string
  quickChoices: string[]
  items: string[]
  complete: boolean
}>()

const emit = defineEmits<{
  answer: [value: string]
  next: []
}>()

const inputValue = ref("")

const memorySceneRef = ref<HTMLElement | null>(null)
const memoryLedgerRef = ref<HTMLElement | null>(null)
const memoryAnswerRef = ref<HTMLElement | null>(null)
const memoryQuickRef = ref<HTMLElement | null>(null)
const memoryPromptRef = ref<HTMLElement | null>(null)

function handleSend() {
  const val = inputValue.value.trim()
  if (!val) return
  emit("answer", val)
  inputValue.value = ""
}

function selectChoice(choice: string) {
  emit("answer", choice)
}

let alignFrame = 0

function syncBottomAlignment() {
  if (window.matchMedia("(max-width: 540px)").matches) return
  const scene = memorySceneRef.value
  const ledger = memoryLedgerRef.value
  const answer = memoryAnswerRef.value
  const quick = memoryQuickRef.value
  if (!scene || !ledger || !answer) return

  const sceneRect = scene.getBoundingClientRect()
  const ledgerRect = ledger.getBoundingClientRect()
  const answerRect = answer.getBoundingClientRect()

  if (!sceneRect.height || !ledgerRect.height || !answerRect.height) return

  const bottomOffset = Math.max(0, sceneRect.bottom - ledgerRect.bottom)
  answer.style.setProperty("--ledger-bottom-offset", `${bottomOffset.toFixed(2)}px`)
  answer.style.setProperty("--answer-block-height", `${answerRect.height.toFixed(2)}px`)
  if (quick) {
    quick.style.setProperty("--ledger-bottom-offset", `${bottomOffset.toFixed(2)}px`)
    quick.style.setProperty("--answer-block-height", `${answerRect.height.toFixed(2)}px`)
  }
}

function scheduleSync() {
  cancelAnimationFrame(alignFrame)
  alignFrame = requestAnimationFrame(() => {
    requestAnimationFrame(syncBottomAlignment)
  })
}

watch(() => props.complete, () => scheduleSync())
watch(() => props.step, () => scheduleSync())

let resizeObserver: ResizeObserver | null = null

onMounted(() => {
  scheduleSync()
  window.addEventListener("resize", scheduleSync, { passive: true })

  const elements = [memorySceneRef.value, memoryLedgerRef.value, memoryAnswerRef.value, memoryQuickRef.value, memoryPromptRef.value]
  resizeObserver = new ResizeObserver(scheduleSync)
  elements.filter(Boolean).forEach((el) => resizeObserver!.observe(el!))
})

onUnmounted(() => {
  window.removeEventListener("resize", scheduleSync)
  resizeObserver?.disconnect()
})
</script>
