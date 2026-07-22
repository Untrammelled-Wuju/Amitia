<template>
  <div class="ob-stage-inner">
    <div class="ob-identity-scene" ref="memorySceneRef" :data-identity-state="complete ? 'complete' : 'filling'">
      <Transition name="ob-fade" mode="out-in" @enter="onTransitionEnter">
        <div v-if="!complete" key="incomplete">
          <div class="ob-identity-prompt" ref="memoryPromptRef">
            <Transition name="ob-fade-prompt" mode="out-in">
              <div :key="step" class="ob-prompt-text">
                <div class="ob-character-line">{{ question }}</div>
                <div class="ob-identity-context">{{ context }}</div>
              </div>
            </Transition>
          </div>

          <div class="ob-identity-ledger" ref="memoryLedgerRef">
            <div class="ob-identity-ledger-title">已记录的信息</div>
            <div class="ob-identity-ledger-items">
              <div class="ob-identity-ledger-card" :class="{ filled: items[0] }">
                <span class="ob-identity-ledger-key">称呼</span>
                <span class="ob-identity-ledger-value">{{ items[0] || '等待回答' }}</span>
              </div>
              <div class="ob-identity-ledger-card" :class="{ filled: items[1] }">
                <span class="ob-identity-ledger-key">交流偏好</span>
                <span class="ob-identity-ledger-value">{{ items[1] || '等待回答' }}</span>
              </div>
              <div class="ob-identity-ledger-card" :class="{ filled: items[2] }">
                <span class="ob-identity-ledger-key">重要信息</span>
                <span class="ob-identity-ledger-value">{{ items[2] || '等待回答' }}</span>
              </div>
              <div class="ob-identity-ledger-card" :class="{ filled: memoryAvatarUploaded }">
                <span class="ob-identity-ledger-key">头像</span>
                <span class="ob-identity-ledger-value">{{ memoryAvatarUploaded ? '已上传' : '等待上传' }}</span>
              </div>
            </div>
          </div>

          <div v-show="!isMemoryAvatarStep" class="ob-identity-answer" ref="memoryAnswerRef">
            <textarea
              v-model="inputValue"
              rows="1"
              :placeholder="placeholder"
              @keydown.enter.prevent="handleSend"
            ></textarea>
            <button @click="handleSend" :disabled="!inputValue.trim()">↑</button>
          </div>

          <div v-show="!isMemoryAvatarStep" class="ob-identity-quick-choices" ref="memoryQuickRef" :class="{ show: quickChoices.length > 0 }">
            <button
              v-for="choice in quickChoices"
              :key="choice"
              class="ob-identity-quick-option"
              @click="selectChoice(choice)"
            >{{ choice }}</button>
          </div>

          <div v-show="isMemoryAvatarStep" class="ob-identity-avatar-area ob-memory-avatar-area" ref="memoryAvatarRef">
            <div class="ob-identity-avatar-circle" :class="{ 'has-image': memoryAvatarPreviewUrl }" @click="triggerFileInput">
              <img v-if="memoryAvatarPreviewUrl" :src="memoryAvatarPreviewUrl" alt="用户头像预览" />
              <span v-else class="ob-identity-avatar-placeholder">+</span>
            </div>
            <p class="ob-identity-avatar-hint">{{ memoryAvatarPreviewUrl ? '点击更换头像' : '点击上传头像' }}</p>
            <input ref="fileInputRef" type="file" accept="image/*" class="ob-avatar-file-input" @change="onFileChange" />
            <div class="ob-identity-avatar-actions">
              <button class="ob-skip-btn" @click="$emit('memoryAvatarSkip')">跳过</button>
              <button class="ob-primary-ghost ob-avatar-continue-btn" :disabled="!memoryAvatarPreviewUrl" @click="$emit('memoryAvatarContinue')">继续</button>
            </div>
          </div>

          <div class="ob-identity-step-markers">
            <span v-for="i in 4" :key="i"
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
                  <span>以后，用 <strong>{{ items[0] || '未记录' }}</strong> 称呼你。</span>
                  <span>交流时保持 <strong>{{ items[1] || '未记录' }}</strong> 的风格。</span>
                  <span v-if="items[2]">你会希望我记得：<strong>{{ items[2] }}</strong>。</span>
                </p>
                <div class="ob-identity-complete-footer">
                  <span class="ob-identity-complete-kicker">初始信息已保存</span>
                  <p class="ob-identity-complete-note">这些设定之后仍可随时修改。</p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </Transition>
    </div>

    <button v-if="complete" class="ob-identity-complete-next" @click="$emit('next')">继续</button>
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
  items: string[]
  complete: boolean
  memoryAvatarPreviewUrl: string
  memoryAvatarUploaded: boolean
  isMemoryAvatarStep: boolean
}>()

const emit = defineEmits<{
  answer: [value: string]
  next: []
  memoryAvatarSkip: []
  memoryAvatarContinue: []
  memoryAvatarFileSelected: [file: File]
}>()

const inputValue = ref("")
const fileInputRef = ref<HTMLInputElement | null>(null)
const memoryAvatarRef = ref<HTMLElement | null>(null)

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

function triggerFileInput() {
  fileInputRef.value?.click()
}

function onFileChange(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  if (!file.type.startsWith("image/")) return
  emit("memoryAvatarFileSelected", file)
}

let alignFrame = 0

function syncBottomAlignment() {
  if (window.matchMedia("(max-width: 540px)").matches) return
  const scene = memorySceneRef.value
  const ledger = memoryLedgerRef.value

  if (props.isMemoryAvatarStep) {
    if (!scene || !ledger) return
    const sceneRect = scene.getBoundingClientRect()
    const ledgerRect = ledger.getBoundingClientRect()
    if (!sceneRect.height || !ledgerRect.height) return
    const avatarEl = memoryAvatarRef.value
    if (!avatarEl) return
    const avatarRect = avatarEl.getBoundingClientRect()
    if (!avatarRect.height) return
    const avatarTop = ledgerRect.top - sceneRect.top + ledgerRect.height - avatarRect.height
    avatarEl.style.top = avatarTop + "px"
    return
  }

  const answer = memoryAnswerRef.value
  const quick = memoryQuickRef.value
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
    const elements = [memoryLedgerRef.value, memoryAnswerRef.value, memoryQuickRef.value, memoryPromptRef.value, memoryAvatarRef.value]
    elements.filter(Boolean).forEach((e) => resizeObserver!.observe(e!))
    syncBottomAlignment()
  })
}

watch(() => props.step, () => scheduleSync())

let resizeObserver: ResizeObserver | null = null

onMounted(() => {
  scheduleSync()
  window.addEventListener("resize", scheduleSync, { passive: true })

  const elements = [memorySceneRef.value, memoryLedgerRef.value, memoryAnswerRef.value, memoryQuickRef.value, memoryPromptRef.value, memoryAvatarRef.value]
  resizeObserver = new ResizeObserver(scheduleSync)
  elements.filter(Boolean).forEach((el) => resizeObserver!.observe(el!))
})

onUnmounted(() => {
  window.removeEventListener("resize", scheduleSync)
  resizeObserver?.disconnect()
})
</script>
