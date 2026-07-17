<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="chat-input-bar">
    <input ref="fileInputRef" type="file" accept="image/*" class="hidden-input" @change="handleImageInput" />
    <input ref="videoInputRef" type="file" accept="video/*" class="hidden-input" @change="handleVideoInput" />

    <div class="composer-stack">
      <div v-if="replyTarget" class="reply-preview-bar">
        <div class="reply-preview-content">
          <span class="reply-preview-label">正在引用{{ replyTarget.role === "assistant" ? "" : "自己" }}：</span>
          <span class="reply-preview-excerpt">{{ replyTargetExcerpt }}</span>
        </div>
        <el-button :icon="CloseBold" circle size="small" class="preview-remove" @click="$emit('cancelReply')" />
      </div>

      <div v-if="hasComposerContext" class="composer-context">
        <div v-if="attachedImagePreview" class="attachment-card image-attachment">
          <span class="attachment-thumb" :style="{ backgroundImage: `url(${attachedImagePreview})` }"></span>
          <span class="attachment-copy">
            <strong>{{ attachedImage?.name || "图片" }}</strong>
            <small>图片</small>
          </span>
          <button type="button" class="context-remove" aria-label="移除图片" @click="clearImage">
            <el-icon><CloseBold /></el-icon>
          </button>
        </div>

        <div v-if="attachedVideo" class="attachment-card video-attachment">
          <span class="attachment-icon"><el-icon><VideoCamera /></el-icon></span>
          <span class="attachment-copy">
            <strong>{{ attachedVideo.name }}</strong>
            <small v-if="uploadingVideo">上传中...</small>
            <small v-else-if="attachedVideoUrl" class="is-ready">视频已就绪</small>
            <small v-else>等待上传</small>
          </span>
          <button type="button" class="context-remove" aria-label="移除视频" :disabled="uploadingVideo" @click="clearVideo">
            <el-icon><CloseBold /></el-icon>
          </button>
        </div>

        <div v-for="name in selectedSkillNames" :key="name" class="skill-context-chip">
          <el-icon><MagicStick /></el-icon>
          <span>${{ name }}</span>
          <button type="button" :aria-label="`移除技能 ${name}`" @click="removeSkill(name)">
            <el-icon><CloseBold /></el-icon>
          </button>
        </div>
      </div>

      <div class="composer-input-row">
        <div ref="inputWrapperRef" class="input-wrapper">
        <div class="input-left-actions">
          <el-popover
            v-model:visible="addMenuOpen"
            placement="top-start"
            :width="skillsPanelOpen ? 340 : 240"
            trigger="click"
            :hide-after="0"
            :teleported="false"
            transition="composer-add-instant"
            popper-class="composer-add-popper"
            @hide="resetAddMenu"
          >
            <template #reference>
              <el-button
                :icon="Plus"
                circle
                size="small"
                class="add-btn"
                :disabled="isInputDisabled"
                title="添加图片、视频或技能"
                aria-label="添加图片、视频或技能"
              />
            </template>

            <div v-if="!skillsPanelOpen" class="add-menu">
              <button type="button" class="add-menu-item" @click="openImagePicker">
                <span class="add-menu-icon"><el-icon><Picture /></el-icon></span>
                <span><strong>上传图片</strong><small>添加一张图片到消息</small></span>
              </button>
              <button type="button" class="add-menu-item" @click="openVideoPicker">
                <span class="add-menu-icon"><el-icon><VideoCamera /></el-icon></span>
                <span><strong>上传视频</strong><small>添加一个视频到消息</small></span>
              </button>
              <div class="add-menu-divider"></div>
              <button type="button" class="add-menu-item" @click="skillsPanelOpen = true">
                <span class="add-menu-icon"><el-icon><MagicStick /></el-icon></span>
                <span><strong>使用技能</strong><small>为本次消息选择 Agent Skill</small></span>
                <el-icon class="menu-chevron"><ArrowRight /></el-icon>
              </button>
            </div>

            <div v-else class="skills-panel">
              <div class="skills-panel-header">
                <button type="button" class="back-btn" aria-label="返回" @click="skillsPanelOpen = false">
                  <el-icon><ArrowLeft /></el-icon>
                </button>
                <div>
                  <strong>使用技能</strong>
                  <small>选择后仅作用于本次输入</small>
                </div>
              </div>
              <el-input v-model="skillSearch" :prefix-icon="Search" clearable placeholder="搜索技能" class="skill-search" />
              <div class="skill-options" role="listbox" aria-label="可用 Agent Skills">
                <button
                  v-for="skill in filteredAgentSkills"
                  :key="skill.extensionId"
                  type="button"
                  class="skill-option"
                  :class="{ 'is-selected': selectedSkillNames.includes(skill.name) }"
                  role="option"
                  :aria-selected="selectedSkillNames.includes(skill.name)"
                  @click="toggleSkill(skill.name)"
                >
                  <span class="skill-option-icon"><el-icon><MagicStick /></el-icon></span>
                  <span class="skill-option-copy">
                    <strong>{{ skill.displayName || skill.name }}</strong>
                    <small>{{ skill.shortDescription || skill.description || "暂无描述" }}</small>
                  </span>
                  <span class="skill-scope">{{ skill.scope === "character" ? "角色" : "全局" }}</span>
                  <el-icon v-if="selectedSkillNames.includes(skill.name)" class="skill-check"><Check /></el-icon>
                </button>
                <div v-if="!filteredAgentSkills.length" class="skill-empty">
                  {{ agentSkills.length ? "没有匹配的技能" : "暂无可用技能" }}
                </div>
              </div>
            </div>
          </el-popover>
        </div>

        <div class="input-body">
          <textarea
            v-show="!voiceMode"
            ref="inputRef"
            v-model="text"
            class="input-field"
            :placeholder="isWechatActive ? '微信消息请在微信端发送...' : isQQActive ? 'QQ消息请在QQ端发送...' : '输入消息...'"
            :disabled="isInputDisabled"
            rows="1"
            :aria-expanded="slashMenuOpen"
            aria-haspopup="listbox"
            :aria-activedescendant="slashMenuOpen && filteredSlashSkills.length ? `slash-skill-${slashActiveIndex}` : undefined"
            @keydown="handleComposerKeydown"
            @input="handleComposerInput"
          />
          <button
            v-show="voiceMode"
            type="button"
            class="hold-voice-btn"
            :class="{ 'is-recording': holding }"
            :disabled="isInputDisabled"
            @pointerdown.prevent="startHold"
            @pointerup.prevent="endHold"
            @pointercancel.prevent="cancelHold"
            @keydown.space.prevent="startHold"
            @keyup.space.prevent="endHold"
            @keydown.enter.prevent="startHold"
            @keyup.enter.prevent="endHold"
            @contextmenu.prevent
          >
            <span v-if="holding" class="hold-voice-label"><span class="hold-voice-dot"></span>松开发送</span>
            <span v-else>按住说话</span>
          </button>
          <div v-if="!voiceMode && slashMenuOpen" class="slash-skill-popover" role="listbox" aria-label="斜杠技能菜单">
            <div class="slash-skill-header">
              <span>使用技能</span>
              <small>/{{ slashQuery }}</small>
            </div>
            <div class="skill-options slash-skill-options">
              <button
                v-for="skill in filteredSlashSkills"
                :key="skill.extensionId"
                :id="`slash-skill-${filteredSlashSkills.indexOf(skill)}`"
                type="button"
                class="skill-option"
                :class="{
                  'is-selected': selectedSkillNames.includes(skill.name),
                  'is-active': filteredSlashSkills.indexOf(skill) === slashActiveIndex,
                }"
                role="option"
                :aria-selected="selectedSkillNames.includes(skill.name)"
                @mousedown.prevent="selectSlashSkill(skill.name)"
                @mouseenter="slashActiveIndex = filteredSlashSkills.indexOf(skill)"
              >
                <span class="skill-option-icon"><el-icon><MagicStick /></el-icon></span>
                <span class="skill-option-copy">
                  <strong>{{ skill.displayName || skill.name }}</strong>
                  <small>{{ skill.shortDescription || skill.description || "暂无描述" }}</small>
                </span>
                <el-icon v-if="selectedSkillNames.includes(skill.name)" class="skill-check"><Check /></el-icon>
              </button>
              <div v-if="!filteredSlashSkills.length" class="skill-empty" aria-live="polite">
                {{ skillsLoading ? "正在加载技能..." : agentSkills.length ? "没有匹配的技能" : "暂无技能" }}
              </div>
            </div>
          </div>
        </div>

          <div class="input-actions">
            <el-button
              :icon="Microphone"
              circle
              size="small"
              class="voice-mode-toggle"
              :class="{ 'is-voice-mode': voiceMode }"
              :disabled="isInputDisabled"
              @click="toggleVoiceMode"
              :title="voiceMode ? '切换到文字输入' : '切换到语音输入'"
            />
            <el-button
              v-if="!voiceMode || generating"
              :type="generating ? 'danger' : 'primary'"
              :icon="generating ? CloseBold : Promotion"
              circle
              size="small"
              :disabled="!generating && (isInputDisabled || uploadingVideo || (!text.trim() && !attachedImagePreview && !attachedVideo && !selectedSkillNames.length) || isSubmitting)"
              @click="generating ? $emit('stop') : handleSendClick()"
              :title="generating ? '停止生成' : '发送 (Enter)'"
            />
          </div>
        </div>

        <el-button
          :type="callActive ? 'danger' : 'default'"
          :icon="Phone"
          circle
          :class="{ 'call-btn-outer': true, 'is-calling': callActive }"
          :disabled="isInputDisabled"
          @click="$emit('toggleCall')"
          title="语音通话"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from "vue"
import { ElMessage } from "element-plus"
import {
  ArrowLeft,
  ArrowRight,
  Check,
  CloseBold,
  MagicStick,
  Microphone,
  Phone,
  Picture,
  Plus,
  Promotion,
  Search,
  VideoCamera,
} from "@element-plus/icons-vue"
import { useTextInput } from "../composables/useTextInput"
import { useMediaUpload } from "../composables/useMediaUpload"
import { useVoiceInput } from "../composables/useVoiceInput"
import { fetchAgentSkills, resolveCharacterId } from "../views/extensions/api"
import type { AgentSkillDefinition } from "../views/extensions/types"

const props = defineProps<{
  isWechatActive?: boolean
  isQQActive?: boolean
  disabled?: boolean
  sending?: boolean
  generating?: boolean
  isSubmitting?: boolean
  callActive?: boolean
  replyTarget?: any
}>()

const emit = defineEmits<{
  send: [text: string, imageBase64?: string, videoBase64?: string]
  stop: []
  toggleCall: []
  voiceText: [text: string]
  voiceAudio: [blob: Blob, transcript?: string, duration?: number]
  image: [file: File, base64: string]
  removeImage: []
  video: [file: File, videoUrl: string]
  removeVideo: []
  cancelReply: []
}>()

const isDisabled = () => !!props.disabled || !!props.isWechatActive || !!props.isQQActive
const isInputDisabled = computed(isDisabled)
const textInput = useTextInput(emit as any, isDisabled)
const mediaUpload = useMediaUpload(
  (file: File, base64: string) => emit("image", file, base64),
  (file: File, videoUrl: string) => emit("video", file, videoUrl),
  () => emit("removeImage"),
  () => emit("removeVideo"),
)

const {
  text,
  inputRef,
  sendWithImage,
  sendWithVideo,
  autoResize,
  focus,
  setText,
  clear: clearText,
} = textInput

const {
  attachedImage,
  attachedImagePreview,
  fileInputRef,
  videoInputRef,
  attachedVideo,
  attachedVideoUrl,
  uploadingVideo,
  handleImageSelect,
  clearImage,
  handleVideoSelect,
  clearVideo,
  fileToBase64,
} = mediaUpload

const addMenuOpen = ref(false)
const inputWrapperRef = ref<HTMLElement>()
const skillsPanelOpen = ref(false)
const skillSearch = ref("")
const agentSkills = ref<AgentSkillDefinition[]>([])
const selectedSkillNames = ref<string[]>([])
const slashMenuOpen = ref(false)
const slashQuery = ref("")
const slashRange = ref<{ start: number; end: number } | null>(null)
const slashActiveIndex = ref(0)
const skillsLoading = ref(false)
const voiceMode = ref(false)

const {
  holding,
  startHold,
  endHold,
  cancelHold,
} = useVoiceInput(
  (blob: Blob, duration?: number) => emit("voiceAudio", blob, undefined, duration),
  isDisabled,
  () => !!props.generating || !!props.isSubmitting,
  () => ElMessage.warning("无法使用麦克风，请检查录音权限"),
)

const replyTargetExcerpt = computed(() => {
  const target = props.replyTarget
  if (!target) return ""
  const excerpt = target.replyToExcerpt || target.content || ""
  return excerpt.length > 60 ? `${excerpt.slice(0, 60)}...` : excerpt
})

const hasComposerContext = computed(() =>
  !!attachedImagePreview.value || !!attachedVideo.value || selectedSkillNames.value.length > 0,
)

const filteredAgentSkills = computed(() => {
  const query = skillSearch.value.trim().toLowerCase()
  return agentSkills.value.filter((skill) => {
    if (!query) return true
    return [skill.name, skill.displayName, skill.description, skill.shortDescription]
      .filter(Boolean)
      .some((value) => String(value).toLowerCase().includes(query))
  }).slice(0, 30)
})

const filteredSlashSkills = computed(() => {
  const query = slashQuery.value.trim().toLowerCase()
  return agentSkills.value.filter((skill) => {
    if (!query) return true
    return [skill.name, skill.displayName, skill.description, skill.shortDescription]
      .filter(Boolean)
      .some((value) => String(value).toLowerCase().includes(query))
  }).slice(0, 12)
})

async function loadAgentSkills() {
  skillsLoading.value = true
  try {
    const characterId = await resolveCharacterId()
    if (!characterId) return
    const page = await fetchAgentSkills(characterId, { pageSize: 100 })
    agentSkills.value = (page.items || []).filter((skill) => skill.enabled && skill.compatibilityStatus !== "blocked")
  } catch {
  } finally {
    skillsLoading.value = false
  }
}

function resetAddMenu() {
  skillsPanelOpen.value = false
  skillSearch.value = ""
}

function openImagePicker() {
  addMenuOpen.value = false
  fileInputRef.value?.click()
}

function openVideoPicker() {
  addMenuOpen.value = false
  videoInputRef.value?.click()
}

function handleImageInput(event: Event) {
  if (attachedVideo.value) clearVideo()
  handleImageSelect(event)
}

function handleVideoInput(event: Event) {
  if (attachedImage.value) clearImage()
  handleVideoSelect(event)
}

function toggleSkill(name: string) {
  if (selectedSkillNames.value.includes(name)) {
    removeSkill(name)
  } else {
    selectedSkillNames.value.push(name)
  }
}

function removeSkill(name: string) {
  selectedSkillNames.value = selectedSkillNames.value.filter((item) => item !== name)
}

function closeSlashMenu() {
  slashMenuOpen.value = false
  slashQuery.value = ""
  slashRange.value = null
  slashActiveIndex.value = 0
}

function handleComposerInput(event: Event) {
  autoResize()
  const target = event.target as HTMLTextAreaElement
  const caret = target.selectionStart ?? text.value.length
  const beforeCaret = text.value.slice(0, caret)
  const match = beforeCaret.match(/(?:^|\s)\/([^\s/]*)$/)
  if (!match) {
    closeSlashMenu()
    return
  }
  slashQuery.value = match[1] || ""
  slashRange.value = { start: caret - slashQuery.value.length - 1, end: caret }
  slashActiveIndex.value = 0
  slashMenuOpen.value = true
}

function scrollActiveSlashSkillIntoView() {
  nextTick(() => {
    inputWrapperRef.value?.querySelector(".slash-skill-popover .skill-option.is-active")?.scrollIntoView({ block: "nearest" })
  })
}

function handleComposerKeydown(event: KeyboardEvent) {
  if (slashMenuOpen.value) {
    const count = filteredSlashSkills.value.length
    if (event.key === "ArrowDown" && count) {
      event.preventDefault()
      slashActiveIndex.value = (slashActiveIndex.value + 1) % count
      scrollActiveSlashSkillIntoView()
      return
    }
    if (event.key === "ArrowUp" && count) {
      event.preventDefault()
      slashActiveIndex.value = (slashActiveIndex.value - 1 + count) % count
      scrollActiveSlashSkillIntoView()
      return
    }
    if ((event.key === "Enter" || event.key === "Tab") && !event.shiftKey && !event.isComposing) {
      event.preventDefault()
      const skill = filteredSlashSkills.value[slashActiveIndex.value]
      if (skill) selectSlashSkill(skill.name)
      return
    }
    if (event.key === "Escape") {
      event.preventDefault()
      closeSlashMenu()
      return
    }
  }
  if (event.key === "Enter" && !event.shiftKey && !event.ctrlKey && !event.altKey && !event.metaKey && !event.isComposing) {
    handleEnterSend(event)
  }
}

function selectSlashSkill(name: string) {
  const range = slashRange.value
  if (!range) return
  if (!selectedSkillNames.value.includes(name)) selectedSkillNames.value.push(name)
  const before = text.value.slice(0, range.start)
  const after = text.value.slice(range.end)
  setText(`${before}${after}`)
  closeSlashMenu()
  nextTick(() => inputRef.value?.focus())
}

function handleComposerOutsidePointer(event: PointerEvent) {
  if (!inputWrapperRef.value?.contains(event.target as Node)) closeSlashMenu()
}

function buildOutgoingText(content: string) {
  const skillPrefix = selectedSkillNames.value.map((name) => `$${name}`).join(" ")
  return [skillPrefix, content.trim()].filter(Boolean).join(" ")
}

function finishSubmit() {
  selectedSkillNames.value = []
  closeSlashMenu()
}

async function submitComposer(event?: KeyboardEvent) {
  if (event) event.preventDefault()
  if (isInputDisabled.value || props.generating || props.isSubmitting || uploadingVideo.value) return
  const outgoingText = buildOutgoingText(text.value)

  if (attachedVideo.value) {
    if (!attachedVideoUrl.value) return
    sendWithVideo(outgoingText || "[视频]", attachedVideoUrl.value)
    clearVideo()
    finishSubmit()
    return
  }

  if (attachedImage.value) {
    if (attachedImagePreview.value) {
      sendWithImage(outgoingText, attachedImagePreview.value)
    } else {
      sendWithImage(outgoingText, await fileToBase64(attachedImage.value))
    }
    clearImage()
    finishSubmit()
    return
  }

  if (!outgoingText) return
  emit("send", outgoingText)
  clearText()
  finishSubmit()
  nextTick(autoResize)
}

function handleEnterSend(event: KeyboardEvent) {
  submitComposer(event)
}

function handleSendClick() {
  submitComposer()
}

function toggleVoiceMode() {
  if (holding.value) cancelHold()
  voiceMode.value = !voiceMode.value
  closeSlashMenu()
  if (!voiceMode.value) nextTick(() => inputRef.value?.focus())
}

watch(() => props.disabled, (value) => {
  if (!value) loadAgentSkills()
})

onMounted(() => {
  loadAgentSkills()
  document.addEventListener("pointerdown", handleComposerOutsidePointer)
})
onUnmounted(() => {
  document.removeEventListener("pointerdown", handleComposerOutsidePointer)
})

defineExpose({ focus, setText, clear: clearText })
</script>

<style scoped>
.chat-input-bar {
  display: flex;
  align-items: flex-end;
  gap: 8px;
  padding: 10px 12px;
  border-top: 1px solid var(--tp-glass-border);
  backdrop-filter: blur(var(--tp-glass-blur)) saturate(var(--tp-glass-saturate));
  -webkit-backdrop-filter: blur(var(--tp-glass-blur)) saturate(var(--tp-glass-saturate));
}

.hidden-input {
  display: none;
}

.composer-stack {
  min-width: 0;
  flex: 1;
}

.composer-input-row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.input-wrapper {
  display: flex;
  align-items: flex-end;
  min-width: 0;
  flex: 1;
  gap: 6px;
  min-height: 52px;
  padding: 8px 9px;
  border: 1px solid var(--ac-color-border);
  border-radius: 18px;
  background: var(--ac-color-surface);
  transition: border-color 0.18s ease, box-shadow 0.18s ease;
}

.input-wrapper:focus-within {
  border-color: var(--ac-color-border-strong);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--ac-color-primary) 10%, transparent);
}

.input-left-actions,
.input-actions {
  display: flex;
  align-items: center;
  flex-shrink: 0;
  min-height: 34px;
}

.input-actions {
  gap: 4px;
}

.add-btn {
  width: 32px;
  height: 32px;
  border-color: transparent;
  background: transparent;
  color: var(--ac-color-text-secondary);
  font-size: 16px;
}

.add-btn:hover,
.add-btn:focus-visible {
  border-color: var(--ac-color-border);
  background: var(--ac-color-bg-secondary);
  color: var(--ac-color-text);
}

.input-body {
  display: flex;
  position: relative;
  align-items: center;
  min-width: 0;
  min-height: 34px;
  flex: 1;
}

.slash-skill-popover {
  position: absolute;
  z-index: 40;
  bottom: calc(100% + 14px);
  left: 0;
  width: min(340px, calc(100vw - 88px));
  padding: 8px;
  border: 1px solid var(--ac-color-border);
  border-radius: 12px;
  background: var(--ac-color-surface);
  box-shadow: var(--tp-shadow-float);
  color: var(--ac-color-text);
}

.slash-skill-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 4px 6px 8px;
  color: var(--ac-color-text-secondary);
  font-size: 12px;
  font-weight: 550;
}

.slash-skill-header small {
  overflow: hidden;
  color: var(--ac-color-text-muted);
  font-size: 11px;
  font-weight: 400;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.slash-skill-options {
  max-height: 248px;
}

.input-field {
  display: block;
  box-sizing: border-box;
  width: 100%;
  min-height: 30px;
  max-height: 144px;
  padding: 5px 2px;
  border: 0;
  outline: 0;
  resize: none;
  overflow-y: auto;
  background: transparent;
  color: var(--ac-color-text);
  font-family: var(--ac-font-family);
  font-size: var(--ac-font-size-sm);
  line-height: 1.5;
}

.input-field::placeholder {
  color: var(--ac-color-text-placeholder);
}

.input-field:disabled {
  opacity: 0.7;
}

.voice-mode-toggle {
  transition: border-color 0.18s ease, background 0.18s ease, color 0.18s ease;
}

.voice-mode-toggle.is-voice-mode {
  border-color: var(--ac-color-border-strong);
  background: var(--ac-color-bg-secondary);
  color: var(--ac-color-text);
}

.hold-voice-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  box-sizing: border-box;
  width: 100%;
  min-height: 34px;
  padding: 0 14px;
  border: 1px solid var(--ac-color-border);
  border-radius: 10px;
  background: var(--ac-color-bg-primary);
  color: var(--ac-color-text-secondary);
  font-family: var(--ac-font-family);
  font-size: var(--ac-font-size-sm);
  cursor: pointer;
  touch-action: none;
  user-select: none;
  -webkit-user-select: none;
  transition: border-color 0.18s ease, background 0.18s ease, color 0.18s ease;
}

.hold-voice-btn.is-recording {
  border-color: var(--ac-color-danger);
  background: var(--ac-color-danger-bg);
  color: var(--ac-color-danger);
  animation: voiceRecordPulse 1.4s ease-in-out infinite;
}

.hold-voice-btn:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.hold-voice-label {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.hold-voice-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: currentColor;
}

@keyframes voiceRecordPulse {
  0%, 100% { box-shadow: 0 0 0 0 color-mix(in srgb, var(--ac-color-danger) 24%, transparent); }
  50% { box-shadow: 0 0 0 5px transparent; }
}

.composer-context {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 7px;
  padding: 0 4px 8px;
}

.attachment-card {
  display: flex;
  align-items: center;
  gap: 9px;
  min-width: 0;
  max-width: 280px;
  height: 48px;
  padding: 5px 7px;
  border: 1px solid var(--ac-color-border-light);
  border-radius: 11px;
  background: var(--ac-color-surface);
}

.attachment-thumb,
.attachment-icon {
  width: 36px;
  height: 36px;
  flex: 0 0 auto;
  border-radius: 8px;
}

.attachment-thumb {
  border: 1px solid var(--ac-color-border-light);
  background-position: center;
  background-size: cover;
}

.attachment-icon {
  display: grid;
  place-items: center;
  background: var(--ac-color-bg-secondary);
  color: var(--ac-color-primary);
  font-size: 17px;
}

.attachment-copy {
  min-width: 0;
  flex: 1;
}

.attachment-copy strong,
.attachment-copy small {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.attachment-copy strong {
  color: var(--ac-color-text);
  font-size: 12px;
  font-weight: 550;
}

.attachment-copy small {
  margin-top: 3px;
  color: var(--ac-color-text-muted);
  font-size: 10px;
}

.attachment-copy small.is-ready {
  color: var(--ac-color-success);
}

.context-remove,
.skill-context-chip button,
.back-btn {
  display: grid;
  place-items: center;
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--ac-color-text-muted);
  cursor: pointer;
}

.context-remove {
  width: 26px;
  height: 26px;
  flex: 0 0 auto;
  border-radius: 7px;
}

.context-remove:hover,
.skill-context-chip button:hover,
.back-btn:hover {
  background: var(--ac-color-bg-secondary);
  color: var(--ac-color-text);
}

.context-remove:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}

.skill-context-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 34px;
  padding: 0 7px 0 10px;
  border: 1px solid var(--ac-color-border-light);
  border-radius: 9px;
  background: var(--ac-color-surface);
  color: var(--ac-color-text-secondary);
  font-size: 12px;
}

.skill-context-chip > .el-icon {
  color: var(--ac-color-primary);
}

.skill-context-chip button {
  width: 22px;
  height: 22px;
  border-radius: 6px;
}

.reply-preview-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 4px 8px;
  padding: 7px 9px 7px 11px;
  border-left: 3px solid var(--ac-color-primary);
  border-radius: 8px;
  background: var(--ac-color-primary-bg);
  font-size: 12px;
}

.reply-preview-content {
  display: flex;
  min-width: 0;
  flex: 1;
  flex-direction: column;
  gap: 2px;
}

.reply-preview-label {
  color: var(--ac-color-primary);
  font-weight: 500;
}

.reply-preview-excerpt {
  overflow: hidden;
  color: var(--ac-color-text-muted);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.add-menu,
.skills-panel {
  color: var(--ac-color-text);
}

.add-menu {
  display: grid;
  gap: 3px;
}

.add-menu-item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  min-height: 52px;
  padding: 7px 8px;
  border: 0;
  border-radius: 9px;
  background: transparent;
  color: var(--ac-color-text);
  cursor: pointer;
  text-align: left;
}

.add-menu-item:hover,
.add-menu-item:focus-visible,
.skill-option:hover,
.skill-option:focus-visible,
.skill-option.is-active {
  outline: 0;
  background: var(--ac-color-bg-secondary);
}

.add-menu-icon,
.skill-option-icon {
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  flex: 0 0 auto;
  border-radius: 9px;
  background: var(--ac-color-bg-secondary);
  color: var(--ac-color-text-secondary);
  font-size: 15px;
}

.add-menu-item > span:nth-child(2) {
  min-width: 0;
  flex: 1;
}

.add-menu-item strong,
.add-menu-item small {
  display: block;
}

.add-menu-item strong {
  font-size: 13px;
  font-weight: 550;
}

.add-menu-item small {
  margin-top: 3px;
  color: var(--ac-color-text-muted);
  font-size: 10px;
}

.menu-chevron {
  flex: 0 0 auto;
  color: var(--ac-color-text-muted);
}

.add-menu-divider {
  height: 1px;
  margin: 3px 6px;
  background: var(--ac-color-border-light);
}

.skills-panel-header {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 2px 2px 10px;
}

.back-btn {
  width: 30px;
  height: 30px;
  flex: 0 0 auto;
  border-radius: 8px;
}

.skills-panel-header strong,
.skills-panel-header small {
  display: block;
}

.skills-panel-header strong {
  font-size: 13px;
}

.skills-panel-header small {
  margin-top: 2px;
  color: var(--ac-color-text-muted);
  font-size: 10px;
}

.skill-search {
  margin-bottom: 8px;
}

.skill-options {
  display: grid;
  gap: 3px;
  max-height: 280px;
  overflow-y: auto;
}

.skill-option {
  display: flex;
  align-items: center;
  gap: 9px;
  width: 100%;
  min-height: 54px;
  padding: 7px;
  border: 0;
  border-radius: 9px;
  background: transparent;
  color: var(--ac-color-text);
  cursor: pointer;
  text-align: left;
}

.skill-option.is-selected {
  background: var(--ac-color-primary-bg);
}

.skill-option-copy {
  min-width: 0;
  flex: 1;
}

.skill-option-copy strong,
.skill-option-copy small {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.skill-option-copy strong {
  font-size: 12px;
  font-weight: 550;
}

.skill-option-copy small {
  margin-top: 3px;
  color: var(--ac-color-text-muted);
  font-size: 10px;
}

.skill-scope {
  flex: 0 0 auto;
  color: var(--ac-color-text-muted);
  font-size: 10px;
}

.skill-check {
  flex: 0 0 auto;
  color: var(--ac-color-primary);
}

.skill-empty {
  padding: 24px 12px;
  color: var(--ac-color-text-muted);
  font-size: 12px;
  text-align: center;
}

.call-btn-outer {
  width: 40px;
  height: 40px;
  flex-shrink: 0;
  font-size: 18px;
}

.is-calling {
  border-color: var(--el-color-success) !important;
  background: var(--el-color-success-light-9) !important;
  color: var(--el-color-success) !important;
}

@media (max-width: 768px) {
  .chat-input-bar {
    padding: 8px 10px;
    padding-bottom: calc(8px + env(safe-area-inset-bottom, 0px));
  }

  .input-wrapper {
    min-height: 50px;
    padding: 7px;
    border-radius: 17px;
  }

  .input-field {
    font-size: 16px;
  }

  .attachment-card {
    max-width: 100%;
  }
}

@media (prefers-reduced-motion: reduce) {
  .hold-voice-btn.is-recording {
    animation: none;
  }
}
</style>
