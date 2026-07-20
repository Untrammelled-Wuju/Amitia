<template>
  <div class="ob-stage-inner">
    <div class="ob-identity-scene" ref="identitySceneRef" :data-identity-state="state">
      <Transition name="ob-fade" mode="out-in" @enter="onTransitionEnter">
        <div v-if="state !== 'complete'" key="incomplete">
          <div class="ob-identity-prompt" ref="identityPromptRef">
            <Transition name="ob-fade-prompt" mode="out-in">
              <div :key="step" class="ob-prompt-text">
                <div class="ob-character-line">{{ question }}</div>
                <div class="ob-identity-context">{{ context }}</div>
              </div>
            </Transition>
          </div>

          <div class="ob-identity-ledger" ref="identityLedgerRef">
            <div class="ob-identity-ledger-title">已设定的信息</div>
            <div class="ob-identity-ledger-items">
              <div class="ob-identity-ledger-card" :class="{ filled: ledger.name }">
                <span class="ob-identity-ledger-key">名字</span>
                <span class="ob-identity-ledger-value">{{ ledger.name || '等待填写' }}</span>
              </div>
              <div class="ob-identity-ledger-card" :class="{ filled: ledger.role }">
                <span class="ob-identity-ledger-key">身份</span>
                <span class="ob-identity-ledger-value">{{ ledger.role || '等待填写' }}</span>
              </div>
              <div class="ob-identity-ledger-card" :class="{ filled: ledger.personality }">
                <span class="ob-identity-ledger-key">性格</span>
                <span class="ob-identity-ledger-value">{{ ledger.personality || '等待填写' }}</span>
              </div>
            </div>
          </div>

          <div class="ob-identity-answer" ref="identityAnswerRef">
            <textarea
              v-model="inputValue"
              rows="1"
              :maxlength="maxLength"
              :placeholder="placeholder"
              @keydown.enter.prevent="handleSend"
            ></textarea>
            <button @click="handleSend" :disabled="!inputValue.trim()">↑</button>
          </div>

          <div class="ob-identity-quick-choices" ref="identityQuickRef" :class="{ show: quickChoices.length > 0 }">
            <button
              v-for="choice in quickChoices"
              :key="choice"
              class="ob-identity-quick-option"
              @click="selectChoice(choice)"
            >{{ choice }}</button>
          </div>

          <div class="ob-identity-step-markers">
            <span v-for="i in 3" :key="i"
              :class="{ active: step === i - 1, done: step > i - 1 }"
            ></span>
          </div>
        </div>

        <div v-else key="complete">
          <div class="ob-identity-complete-view">
            <div class="ob-identity-complete-card">
              <div class="ob-identity-complete-copy-block">
                <p class="ob-identity-complete-message">
                  <span>我记住了。</span>
                  <span>以后，你可以叫我 <strong>{{ ledger.name }}</strong>。</span>
                  <span>我是你的 <strong>{{ ledger.role }}</strong>。</span>
                  <span>我会以你设定的性格，陪在你身边。</span>
                </p>
                <div class="ob-identity-complete-footer">
                  <span class="ob-identity-complete-kicker">角色设定完成</span>
                  <p class="ob-identity-complete-note">这些设定之后仍可随时修改。</p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </Transition>
    </div>

    <button v-if="state === 'complete'" class="ob-identity-complete-next" @click="$emit('next')">继续</button>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, nextTick } from "vue"

const props = defineProps<{
  step: number
  question: string
  context: string
  placeholder: string
  quickChoices: string[]
  maxLength: number
  ledger: { name: string; role: string; personality: string }
  state: string
}>()

const emit = defineEmits<{
  answer: [value: string]
  next: []
}>()

const inputValue = ref("")

const identitySceneRef = ref<HTMLElement | null>(null)
const identityLedgerRef = ref<HTMLElement | null>(null)
const identityAnswerRef = ref<HTMLElement | null>(null)
const identityQuickRef = ref<HTMLElement | null>(null)
const identityPromptRef = ref<HTMLElement | null>(null)

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
  const scene = identitySceneRef.value
  const ledger = identityLedgerRef.value
  const answer = identityAnswerRef.value
  const quick = identityQuickRef.value
  if (!scene || !ledger || !answer) return

  const sceneRect = scene.getBoundingClientRect()
  const ledgerRect = ledger.getBoundingClientRect()
  const answerRect = answer.getBoundingClientRect()

  if (!sceneRect.height || !ledgerRect.height || !answerRect.height) return

  const answerTop = ledgerRect.top - sceneRect.top + ledgerRect.height - answerRect.height
  answer.style.top = answerTop + "px"
  if (quick) {
    quick.style.top = (answerTop - 56) + "px"
  }

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

function onTransitionEnter(_el: Element) {
  nextTick(() => {
    resizeObserver?.disconnect()
    const elements = [identityLedgerRef.value, identityAnswerRef.value, identityQuickRef.value, identityPromptRef.value]
    elements.filter(Boolean).forEach((e) => resizeObserver!.observe(e!))
    syncBottomAlignment()
  })
}
watch(() => props.step, () => scheduleSync())

let resizeObserver: ResizeObserver | null = null

onMounted(() => {
  scheduleSync()
  window.addEventListener("resize", scheduleSync, { passive: true })

  const elements = [identitySceneRef.value, identityLedgerRef.value, identityAnswerRef.value, identityQuickRef.value, identityPromptRef.value]
  resizeObserver = new ResizeObserver(scheduleSync)
  elements.filter(Boolean).forEach((el) => resizeObserver!.observe(el!))
})

onUnmounted(() => {
  window.removeEventListener("resize", scheduleSync)
  resizeObserver?.disconnect()
})
</script>
