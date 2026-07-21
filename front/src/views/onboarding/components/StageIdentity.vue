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
              <div class="ob-identity-ledger-card" :class="{ filled: avatarUploaded }">
                <span class="ob-identity-ledger-key">头像</span>
                <span class="ob-identity-ledger-value">{{ avatarUploaded ? '已上传' : '等待上传' }}</span>
              </div>
            </div>
          </div>

          <div v-if="!isAvatarStep" class="ob-identity-answer" ref="identityAnswerRef">
            <textarea
              v-model="inputValue"
              rows="1"
              :maxlength="maxLength"
              :placeholder="placeholder"
              @keydown.enter.prevent="handleSend"
            ></textarea>
            <button @click="handleSend" :disabled="!inputValue.trim()">↑</button>
          </div>

          <div v-if="!isAvatarStep" class="ob-identity-quick-choices" ref="identityQuickRef" :class="{ show: quickChoices.length > 0 }">
            <button
              v-for="choice in quickChoices"
              :key="choice"
              class="ob-identity-quick-option"
              @click="selectChoice(choice)"
            >{{ choice }}</button>
          </div>

          <div v-if="isAvatarStep" class="ob-identity-avatar-area" ref="identityAvatarRef">
            <div class="ob-identity-avatar-circle" :class="{ 'has-image': avatarPreviewUrl }" @click="triggerFileInput">
              <img v-if="avatarPreviewUrl" :src="avatarPreviewUrl" alt="角色头像预览" />
              <span v-else class="ob-identity-avatar-placeholder">+</span>
            </div>
            <p class="ob-identity-avatar-hint">{{ avatarPreviewUrl ? '点击更换头像' : '点击上传头像' }}</p>
            <input ref="fileInputRef" type="file" accept="image/*" class="ob-avatar-file-input" @change="onFileChange" />
            <div class="ob-identity-avatar-actions">
              <button class="ob-skip-btn" @click="$emit('avatarSkip')">跳过</button>
              <button class="ob-primary-ghost ob-avatar-continue-btn" :disabled="!avatarPreviewUrl" @click="$emit('avatarContinue')">继续</button>
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
  avatarPreviewUrl: string
  avatarUploaded: boolean
  isAvatarStep: boolean
}>()

const emit = defineEmits<{
  answer: [value: string]
  next: []
  avatarSkip: []
  avatarContinue: []
  avatarFileSelected: [file: File]
}>()

const inputValue = ref("")
const fileInputRef = ref<HTMLInputElement | null>(null)
const identityAvatarRef = ref<HTMLElement | null>(null)

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

function triggerFileInput() {
  fileInputRef.value?.click()
}

function onFileChange(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  if (!file.type.startsWith("image/")) return
  emit("avatarFileSelected", file)
}

let alignFrame = 0

function syncBottomAlignment() {
  if (window.matchMedia("(max-width: 540px)").matches) return
  const scene = identitySceneRef.value
  const ledger = identityLedgerRef.value

  if (props.isAvatarStep) {
    if (!scene || !ledger) return
    const sceneRect = scene.getBoundingClientRect()
    const ledgerRect = ledger.getBoundingClientRect()
    if (!sceneRect.height || !ledgerRect.height) return
    const avatarEl = identityAvatarRef.value
    if (!avatarEl) return
    const avatarRect = avatarEl.getBoundingClientRect()
    if (!avatarRect.height) return
    const avatarTop = ledgerRect.top - sceneRect.top + ledgerRect.height - avatarRect.height
    avatarEl.style.top = avatarTop + "px"
    return
  }

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
    if (!props.isAvatarStep) {
      const elements = [identityLedgerRef.value, identityAnswerRef.value, identityQuickRef.value, identityPromptRef.value]
      elements.filter(Boolean).forEach((e) => resizeObserver!.observe(e!))
    }
    if (props.isAvatarStep) {
      const elements = [identityLedgerRef.value, identityPromptRef.value, identityAvatarRef.value]
      elements.filter(Boolean).forEach((e) => resizeObserver!.observe(e!))
    }
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
