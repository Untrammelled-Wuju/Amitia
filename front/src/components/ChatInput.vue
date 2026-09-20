<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="chat-input-bar">
    <input
      ref="fileInputRef"
      type="file"
      accept="image/*"
      class="hidden-input"
      @change="handleImageInput"
    />
    <input
      ref="videoInputRef"
      type="file"
      accept="video/*"
      class="hidden-input"
      @change="handleVideoInput"
    />
    <input
      ref="genericFileInputRef"
      type="file"
      class="hidden-input"
      @change="handleGenericFileInput"
    />

        <div class="composer-stack">
      <div ref="inputWrapperRef" class="input-wrapper">
        <ComposerExtensionHost
          v-if="characterId"
          :character-id="characterId"
          :conversation-id="conversationId"
          :channel="channel"
          :platform="env.platform"
          :host="env.host"
          :os="env.os"
          :conversation-state="generating ? 'generating' : 'idle'"
          :capabilities="hostCapabilities"
          :available-skills="agentSkillNames"
          :draft="text"
        />
        <div v-if="replyTarget" class="reply-preview-bar">
            <div class="reply-preview-content">
              <span class="reply-preview-label"
                >正在引用{{
                  replyTarget.role === "assistant" ? "" : "自己"
                }}：</span
              >
              <span class="reply-preview-excerpt">{{ replyTargetExcerpt }}</span>
            </div>
            <el-button
              :icon="CloseBold"
              circle
              size="small"
              class="preview-remove"
              @click="$emit('cancelReply')"
            />
          </div>

          <div v-if="hasComposerContext" class="composer-context">
            <div v-if="attachedImage" class="attachment-card image-attachment">
              <span
                class="attachment-thumb"
                :style="{
                  backgroundImage: attachedImagePreview
                    ? `url(${attachedImagePreview})`
                    : undefined,
                }"
              ></span>
              <span class="attachment-copy">
                <strong>{{ attachedImage?.name || "图片" }}</strong>
                <small v-if="processingImage">图片处理中...</small>
                <small v-else>图片</small>
              </span>
              <button
                type="button"
                class="context-remove"
                aria-label="移除图片"
                @click="clearImage"
              >
                <el-icon><CloseBold /></el-icon>
              </button>
            </div>

            <div v-if="attachedVideo" class="attachment-card video-attachment">
              <span class="attachment-icon"
                ><el-icon><VideoCamera /></el-icon
              ></span>
              <span class="attachment-copy">
                <strong>{{ attachedVideo.name }}</strong>
                <small v-if="uploadingVideo">上传中...</small>
                <small v-else-if="attachedVideoUrl" class="is-ready"
                  >视频已就绪</small
                >
                <small v-else-if="videoUploadError" class="is-error">{{ videoUploadError }}</small>
                <small v-else>等待上传</small>
              </span>
              <button
                type="button"
                class="context-remove"
                aria-label="移除视频"
                :disabled="uploadingVideo"
                @click="clearVideo"
              >
                <el-icon><CloseBold /></el-icon>
              </button>
            </div>

            <div
              v-for="name in selectedSkillNames"
              :key="name"
              class="skill-context-chip"
            >
              <el-icon><MagicStick /></el-icon>
              <span>${{ name }}</span>
              <button
                type="button"
                :aria-label="`移除技能 ${name}`"
                @click="removeSkill(name)"
              >
                <el-icon><CloseBold /></el-icon>
              </button>
            </div>
          </div>

          <div class="input-row">
          <div class="input-left-actions">
            <ComposerActionExtensionHost
              v-if="characterId"
              :character-id="characterId"
              :conversation-id="conversationId"
              :channel="channel"
              :platform="env.platform"
              :host="env.host"
              :os="env.os"
              :conversation-state="generating ? 'generating' : 'idle'"
              :capabilities="hostCapabilities"
              :draft="text"
            />
            <el-popover
              v-model:visible="addMenuOpen"
              placement="top-start"
              :width="skillsPanelOpen ? 340 : 240"
              trigger="click"
              :hide-after="0"
              :teleported="true"
              append-to="#amitia-overlay-root"
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
                <button
                  type="button"
                  class="add-menu-item"
                  @click="openImagePicker"
                >
                  <span class="add-menu-icon"
                    ><el-icon><Picture /></el-icon
                  ></span>
                  <span
                    ><strong>上传图片</strong
                    ><small>添加一张图片到消息</small></span
                  >
                </button>
                <button
                  type="button"
                  class="add-menu-item"
                  @click="openVideoPicker"
                >
                  <span class="add-menu-icon"
                    ><el-icon><VideoCamera /></el-icon
                  ></span>
                  <span
                    ><strong>上传视频</strong
                    ><small>添加一个视频到消息</small></span
                  >
                </button>
                <button
                  type="button"
                  class="add-menu-item"
                  @click="openGenericFilePicker"
                >
                  <span class="add-menu-icon"><el-icon><Document /></el-icon></span>
                  <span><strong>上传文件</strong><small>发送文档、压缩包或其他文件</small></span>
                </button>
                <div class="add-menu-divider"></div>
                <button
                  type="button"
                  class="add-menu-item"
                  @click="skillsPanelOpen = true"
                >
                  <span class="add-menu-icon"
                    ><el-icon><MagicStick /></el-icon
                  ></span>
                  <span
                    ><strong>使用技能</strong
                    ><small>为本次消息选择 Agent Skill</small></span
                  >
                  <el-icon class="menu-chevron"><ArrowRight /></el-icon>
                </button>
              </div>

              <div v-else class="skills-panel">
                <div class="skills-panel-header">
                  <button
                    type="button"
                    class="back-btn"
                    aria-label="返回"
                    @click="skillsPanelOpen = false"
                  >
                    <el-icon><ArrowLeft /></el-icon>
                  </button>
                  <div>
                    <strong>使用技能</strong>
                    <small>选择后仅作用于本次输入</small>
                  </div>
                </div>
                <el-input
                  v-model="skillSearch"
                  :prefix-icon="Search"
                  clearable
                  placeholder="搜索技能"
                  class="skill-search"
                />
                <div
                  class="skill-options"
                  role="listbox"
                  aria-label="可用 Agent Skills"
                >
                  <button
                    v-for="skill in filteredAgentSkills"
                    :key="skill.extensionId"
                    type="button"
                    class="skill-option"
                    :class="{
                      'is-selected': selectedSkillNames.includes(skill.name),
                    }"
                    role="option"
                    :aria-selected="selectedSkillNames.includes(skill.name)"
                    @click="toggleSkill(skill.name)"
                  >
                    <span class="skill-option-icon"
                      ><el-icon><MagicStick /></el-icon
                    ></span>
                    <span class="skill-option-copy">
                      <strong>{{ skill.displayName || skill.name }}</strong>
                      <small>{{
                        skill.shortDescription ||
                        skill.description ||
                        "暂无描述"
                      }}</small>
                    </span>
                    <span class="skill-scope">{{
                      skill.scope === "character" ? "角色" : "全局"
                    }}</span>
                    <el-icon
                      v-if="selectedSkillNames.includes(skill.name)"
                      class="skill-check"
                      ><Check
                    /></el-icon>
                  </button>
                  <div v-if="!filteredAgentSkills.length" class="skill-empty">
                    {{ agentSkills.length ? "没有匹配的技能" : "暂无可用技能" }}
                  </div>
                </div>
              </div>
            </el-popover>
            <el-popover
              v-if="supportsWorkspaceDirectory"
              v-model:visible="workspaceMenuOpen"
              placement="top-start"
              :width="320"
              trigger="click"
              :hide-after="0"
              :teleported="true"
              append-to="#amitia-overlay-root"
              popper-class="workspace-picker-popper"
              @show="refreshRecentWorkspaces"
            >
              <template #reference>
                <button
                  type="button"
                  class="workspace-trigger"
                  :class="{ 'has-workspace': !!currentWorkspace }"
                  :disabled="workspaceLoading || isInputDisabled"
                  title="选择或添加项目文件夹"
                >
                  <el-icon><FolderOpened /></el-icon>
                  <span>{{ workspaceLabel }}</span>
                </button>
              </template>
              <div class="workspace-picker">
                <div class="workspace-picker-header">
                  <div>
                    <strong>项目</strong>
                    <small>当前项目中的所有对话共用此文件夹</small>
                  </div>
                </div>
                <div v-if="recentWorkspaces.length" class="workspace-recent-list">
                  <button
                    v-for="workspace in recentWorkspaces"
                    :key="workspace.id"
                    type="button"
                    class="workspace-option"
                    :class="{
                      'is-selected': currentWorkspace?.workspaceId === workspace.id,
                      'is-unavailable': !workspace.available,
                    }"
                    :disabled="workspaceLoading || !workspace.available"
                    @click="handleWorkspaceSelect(workspace)"
                  >
                    <el-icon class="workspace-option-icon"><FolderOpened /></el-icon>
                    <span class="workspace-option-copy">
                      <strong>{{ workspace.name }}</strong>
                      <small>{{ workspace.available ? '本机目录' : (workspace.statusReason || '目录不可用') }}</small>
                    </span>
                    <el-icon
                      v-if="currentWorkspace?.workspaceId === workspace.id"
                      class="workspace-selected-icon"
                    ><Check /></el-icon>
                  </button>
                </div>
                <div v-else class="workspace-empty">暂无项目</div>
                <div class="workspace-picker-divider"></div>
                <button
                  type="button"
                  class="workspace-action"
                  :disabled="workspaceLoading"
                  @click="handleChooseWorkspaceDirectory"
                >
                  <el-icon><FolderOpened /></el-icon>
                  <span>添加文件夹为项目…</span>
                </button>
                <button
                  v-if="currentWorkspace"
                  type="button"
                  class="workspace-action is-clear"
                  :disabled="workspaceLoading"
                  @click="handleClearWorkspace"
                >
                  <el-icon><CloseBold /></el-icon>
                  <span>移出项目</span>
                </button>
              </div>
            </el-popover>
            <el-popover
              v-model:visible="permissionMenuOpen"
              placement="top-start"
              :width="280"
              trigger="click"
              :hide-after="0"
              :teleported="true"
              append-to="#amitia-overlay-root"
              popper-class="permission-picker-popper"
            >
              <template #reference>
                <button
                  type="button"
                  class="permission-trigger"
                  :class="{ 'is-full-access': permissionMode === 'full_access' }"
                  :disabled="isInputDisabled"
                  :aria-expanded="permissionMenuOpen"
                  aria-haspopup="menu"
                  :title="permissionLabel"
                >
                  <el-icon>
                    <Unlock v-if="permissionMode === 'full_access'" />
                    <Lock v-else />
                  </el-icon>
                  <span>{{ permissionLabel }}</span>
                </button>
              </template>
              <div class="permission-picker" role="menu">
                <div class="permission-picker-header">
                  <strong>工具权限</strong>
                  <small>权限模式仅影响从下一条消息开始的工具执行</small>
                </div>
                <button
                  type="button"
                  class="permission-option"
                  :class="{ 'is-selected': permissionMode !== 'full_access' }"
                  role="menuitemradio"
                  :aria-checked="permissionMode !== 'full_access'"
                  @click="selectPermissionMode('request_approval')"
                >
                  <span class="permission-option-icon"><el-icon><Lock /></el-icon></span>
                  <span class="permission-option-copy">
                    <strong>请求批准</strong>
                    <small>敏感工具执行前需要你批准</small>
                  </span>
                  <el-icon v-if="permissionMode !== 'full_access'" class="permission-check"><Check /></el-icon>
                </button>
                <button
                  type="button"
                  class="permission-option"
                  :class="{ 'is-selected': permissionMode === 'full_access' }"
                  role="menuitemradio"
                  :aria-checked="permissionMode === 'full_access'"
                  @click="selectPermissionMode('full_access')"
                >
                  <span class="permission-option-icon"><el-icon><Unlock /></el-icon></span>
                  <span class="permission-option-copy">
                    <strong>完全访问</strong>
                    <small>自动放行当前会话中可批准的工具操作</small>
                  </span>
                  <el-icon v-if="permissionMode === 'full_access'" class="permission-check"><Check /></el-icon>
                </button>
              </div>
            </el-popover>
          </div>

          <div class="input-body">
            <textarea
              v-show="!voiceMode"
              ref="inputRef"
              v-model="text"
              class="input-field"
              placeholder="输入消息..."
              :disabled="isInputDisabled"
              rows="1"
              :aria-expanded="slashMenuOpen"
              aria-haspopup="listbox"
              :aria-activedescendant="
                slashMenuOpen && filteredSlashSkills.length
                  ? `slash-skill-${slashActiveIndex}`
                  : undefined
              "
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
              <span v-if="holding" class="hold-voice-label"
                ><span class="hold-voice-dot"></span>松开发送</span
              >
              <span v-else>按住说话</span>
            </button>
            <div
              v-if="!voiceMode && slashMenuOpen"
              class="slash-skill-popover"
              role="listbox"
              aria-label="斜杠技能菜单"
            >
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
                    'is-active':
                      filteredSlashSkills.indexOf(skill) === slashActiveIndex,
                  }"
                  role="option"
                  :aria-selected="selectedSkillNames.includes(skill.name)"
                  @mousedown.prevent="selectSlashSkill(skill.name)"
                  @mouseenter="
                    slashActiveIndex = filteredSlashSkills.indexOf(skill)
                  "
                >
                  <span class="skill-option-icon"
                    ><el-icon><MagicStick /></el-icon
                  ></span>
                  <span class="skill-option-copy">
                    <strong>{{ skill.displayName || skill.name }}</strong>
                    <small>{{
                      skill.shortDescription || skill.description || "暂无描述"
                    }}</small>
                  </span>
                  <el-icon
                    v-if="selectedSkillNames.includes(skill.name)"
                    class="skill-check"
                    ><Check
                  /></el-icon>
                </button>
                <div
                  v-if="!filteredSlashSkills.length"
                  class="skill-empty"
                  aria-live="polite"
                >
                  {{
                    skillsLoading
                      ? "正在加载技能..."
                      : agentSkills.length
                        ? "没有匹配的技能"
                        : "暂无技能"
                  }}
                </div>
              </div>
            </div>
          </div>

                    <div class="input-actions">
            <el-popover
              v-model:visible="modelMenuOpen"
              placement="top-end"
              :width="260"
              trigger="click"
              :hide-after="0"
              :teleported="true"
              append-to="#amitia-overlay-root"
              popper-class="composer-model-popper"
              @hide="resetModelMenu"
            >
              <template #reference>
                <button
                  type="button"
                  class="model-effort-trigger"
                  :disabled="isInputDisabled"
                >
                  <span>{{ selectedModelLabel }}</span>
                  <span> · </span>
                  <span>{{ reasoningLabel }}</span>
                </button>
              </template>
              <div class="model-menu-viewport">
              <Transition name="model-menu-fade" mode="out-in">
                <div :key="modelMenuView" class="model-menu-page">
              <div v-if="modelMenuView === 'main'" class="model-effort-menu">
                <div class="model-effort-row">
                  <span>强度</span>
                  <strong>{{ draftReasoningLabel }}</strong>
                </div>
                <button
                  type="button"
                  class="model-effort-row model-effort-row--button"
                  @click="openModelMenuPage('reasoning')"
                >
                  <span>思考</span>
                  <span class="model-effort-model">
                    {{ draftReasoningEnabled ? "支持" : "不支持" }}
                    <el-icon><ArrowRight /></el-icon>
                  </span>
                </button>
                <button
                  type="button"
                  class="model-effort-row model-effort-row--button"
                  @click="openModelMenuPage('models')"
                >
                  <span>模型</span>
                  <span class="model-effort-model">
                    {{ selectedModelLabel }}
                    <el-icon><ArrowRight /></el-icon>
                  </span>
                </button>
                <div class="model-effort-slider">
                  <div
                    class="model-effort-slider-track"
                    :class="{ disabled: !draftReasoningEnabled }"
                  >
                    <div class="model-effort-track-range">
                      <div class="model-effort-track-base"></div>
                      <div
                        class="model-effort-track-active"
                        :class="{ 'is-empty': draftReasoningLevel === 0 }"
                        :style="{ width: reasoningActiveWidth }"
                      ></div>
                    </div>
                    <div class="model-effort-markers" aria-hidden="true">
                      <span
                        v-for="index in 4"
                        :key="index"
                        :class="{ active: draftReasoningValue >= index - 1 }"
                        :style="{
                          left: reasoningMarkerOffsets[index - 1],
                        }"
                      ></span>
                    </div>
                    <el-slider
                      class="model-effort-range"
                      v-model="draftReasoningValue"
                      :min="0"
                      :max="3"
                      :step="1"
                      :show-tooltip="false"
                      :disabled="!draftReasoningEnabled"
                      @input="previewReasoningIndex"
                      @change="applyReasoningIndex"
                    />
                  </div>
                  <div class="model-effort-labels">
                    <span>低</span><span>中</span><span>高</span><span>极高</span>
                  </div>
                  <div v-if="!draftReasoningEnabled" class="model-effort-hint">
                    该模型不支持思考强度
                  </div>
                </div>
              </div>
              <div
                v-else-if="modelMenuView === 'reasoning'"
                class="model-list-panel"
              >
                <div class="model-list-header">
                  <button
                    type="button"
                    aria-label="返回"
                    @click="backModelMenuPage"
                  >
                    <el-icon><ArrowLeft /></el-icon>
                  </button>
                  <strong>选择思考模式</strong>
                </div>
                <button
                  v-for="option in reasoningModeOptions"
                  :key="String(option.value)"
                  type="button"
                  class="model-list-item"
                  :class="{ active: draftReasoningEnabled === option.value }"
                  @click="selectReasoningEnabled(option.value)"
                >
                  <span>
                    <strong>{{ option.label }}</strong>
                    <small>{{ option.description }}</small>
                  </span>
                  <el-icon
                    v-if="draftReasoningEnabled === option.value"
                  >
                    <Check />
                  </el-icon>
                </button>
              </div>
              <div v-else class="model-list-panel">
                <div class="model-list-header">
                  <button type="button" aria-label="返回" @click="backModelMenuPage">
                    <el-icon><ArrowLeft /></el-icon>
                  </button>
                  <strong>选择模型</strong>
                </div>
                <button
                  v-for="model in llmModels"
                  :key="model.id"
                  type="button"
                  class="model-list-item"
                  :class="{ active: model.id === selectedModelId }"
                  @click="selectModel(model)"
                >
                  <span>
                    <strong>{{ model.name || model.modelName }}</strong>
                    <small>{{ model.modelName }} · {{ providerLabel(model) }}</small>
                  </span>
                  <el-icon v-if="model.id === selectedModelId"><Check /></el-icon>
                </button>
              </div>
                </div>
              </Transition>
              </div>
            </el-popover>
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
              :disabled="
                !generating &&
                (isInputDisabled ||
                  uploadingVideo ||
                  !!videoUploadError ||
                  processingImage ||
                  (!text.trim() &&
                    !attachedImagePreview &&
                    !attachedVideo &&
                    !selectedSkillNames.length) ||
                  isSubmitting)
              "
              @click="generating ? $emit('stop') : handleSendClick()"
              :title="generating ? '停止生成' : '发送 (Enter)'"
            />
          </div>
          </div>

      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import {
  ArrowDown,
  ArrowLeft,
  ArrowRight,
  Check,
  CloseBold,
  MagicStick,
  Microphone,
  Picture,
  Plus,
  Promotion,
  Search,
  VideoCamera,
  Document,
  FolderOpened,
  Lock,
  Unlock,
} from "@element-plus/icons-vue";
import { useTextInput } from "../composables/useTextInput";
import { useMediaUpload } from "../composables/useMediaUpload";
import { useVoiceInput } from "../composables/useVoiceInput";
import { fetchAgentSkills } from "../views/extensions/api";
import type { AgentSkillDefinition } from "../views/extensions/types";
import { resolveHostEnvironment } from "@/composables/useHostEnvironment";
import ComposerExtensionHost from "./extension/chat/ComposerExtensionHost.vue";
import ComposerActionExtensionHost from "./extension/chat/ComposerActionExtensionHost.vue";
import {
  useConversationWorkspace,
  type WorkspaceMountSummary,
} from "../composables/useConversationWorkspace";

const env = resolveHostEnvironment();
const props = withDefaults(defineProps<{
  disabled?: boolean;
  sending?: boolean;
  generating?: boolean;
  isSubmitting?: boolean;
  callActive?: boolean;
  replyTarget?: any;
  characterId?: string;
  conversationId?: string;
  channel?: string;
  models?: any[];
  selectedModelId?: number;
  reasoningEffort?: string;
  supportsReasoning?: boolean;
  reasoningEnabled?: boolean;
  permissionMode?: string;
  modelPreviewChange?: (
    modelId: number,
    reasoningEffort: string,
    reasoningEnabled: boolean,
  ) => void;
  modelCommitChange?: (
    modelId: number,
    reasoningEffort: string,
    reasoningEnabled: boolean,
  ) => void;
}>(), {
  characterId: "",
  conversationId: "",
  channel: "web",
  models: () => [],
  selectedModelId: 0,
  reasoningEffort: "high",
  supportsReasoning: false,
  reasoningEnabled: true,
  permissionMode: "request_approval",
});

const emit = defineEmits<{
  send: [text: string, imageBase64?: string, videoBase64?: string];
  stop: [];
  toggleCall: [];
  voiceText: [text: string];
  voiceAudio: [blob: Blob, transcript?: string, duration?: number];
  image: [file: File, base64: string];
  removeImage: [];
  video: [file: File, videoUrl: string];
  removeVideo: [];
  cancelReply: [];
  file: [file: File];
  "preview-model": [
    modelId: number,
    reasoningEffort: string,
    reasoningEnabled: boolean,
  ];
  "update:model": [
    modelId: number,
    reasoningEffort: string,
    reasoningEnabled: boolean,
  ];
  "update:permission": [mode: string];
}>();

const isDisabled = () => !!props.disabled;
const isInputDisabled = computed(isDisabled);
const mediaUpload = useMediaUpload(
  (file: File, base64: string) => emit("image", file, base64),
  (file: File, videoUrl: string) => emit("video", file, videoUrl),
  () => emit("removeImage"),
  () => emit("removeVideo"),
);

const {
  attachedImage,
  attachedImagePreview,
  fileInputRef,
  videoInputRef,
  attachedVideo,
  attachedVideoUrl,
  uploadingVideo,
  videoUploadError,
  processingImage,
  handleImageSelect,
  clearImage,
  handleVideoSelect,
  clearVideo,
  fileToBase64,
} = mediaUpload;

const addMenuOpen = ref(false);
const genericFileInputRef = ref<HTMLInputElement | null>(null);
const inputWrapperRef = ref<HTMLElement>();
const skillsPanelOpen = ref(false);
const skillSearch = ref("");
const agentSkills = ref<AgentSkillDefinition[]>([]);
const selectedSkillNames = ref<string[]>([]);
const slashMenuOpen = ref(false);
const slashQuery = ref("");
const slashRange = ref<{ start: number; end: number } | null>(null);
const slashActiveIndex = ref(0);
const skillsLoading = ref(false);
const voiceMode = ref(false);
const modelMenuOpen = ref(false);
const modelMenuView = ref<"main" | "models" | "reasoning">("main");
const draftReasoningValue = ref(1);
const draftReasoningEnabled = ref(false);
const modelSliderDragging = ref(false);
const workspaceMenuOpen = ref(false);
const permissionMenuOpen = ref(false);
const supportsWorkspaceDirectory = computed(
  () => typeof window !== "undefined" && !!window.amitiaDesktop?.selectWorkspaceDirectory,
);
const permissionLabel = computed(() =>
  props.permissionMode === "full_access" ? "完全访问" : "请求批准",
);

function selectPermissionMode(mode: string) {
  const next = mode === "full_access" ? "full_access" : "request_approval";
  emit("update:permission", next);
  permissionMenuOpen.value = false;
}
const {
  currentWorkspace,
  recentWorkspaces,
  workspaceLoading,
  refreshRecentWorkspaces,
  loadConversationWorkspace,
  chooseWorkspaceDirectory,
  selectWorkspaceMount,
  clearWorkspace,
} = useConversationWorkspace();
const workspaceLabel = computed(
  () => currentWorkspace.value?.workspaceName || "选择项目",
);

const reasoningOptions = ["low", "medium", "high", "xhigh"];
const reasoningMarkerOffsets = [
  "32px",
  "calc(33.333% + 10.667px)",
  "calc(66.667% - 10.667px)",
  "calc(100% - 32px)",
];
const reasoningTrackWidths = [
  "16px",
  "calc(16px + (100% - 32px) * 0.333333)",
  "calc(16px + (100% - 32px) * 0.666667)",
  "calc(100% - 16px)",
];
const reasoningLabels: Record<string, string> = {
  low: "低",
  medium: "中",
  high: "高",
  xhigh: "极高",
};
const reasoningModeOptions = [
  {
    value: true,
    label: "支持",
    description: "允许模型按所选强度进行思考",
  },
  {
    value: false,
    label: "不支持",
    description: "关闭模型的思考过程",
  },
];
const llmModels = computed(() =>
  (props.models || []).filter((model: any) => {
    const type = String(model.apiType || model.provider || "").toLowerCase();
    return !["voice", "asr", "embedding", "vector", "vision", "imagegen"].includes(type);
  }),
);
const selectedModel = computed(() =>
  llmModels.value.find((model: any) => Number(model.id) === Number(props.selectedModelId)),
);
const selectedModelLabel = computed(
  () => selectedModel.value?.name || selectedModel.value?.modelName || "选择模型",
);
const reasoningLabel = computed(
  () => reasoningLabels[props.reasoningEffort] || "中",
);
const draftReasoningLevel = computed(() =>
  Math.min(Math.max(Math.round(draftReasoningValue.value), 0), 3),
);
const draftReasoningLabel = computed(
  () => reasoningLabels[reasoningOptions[draftReasoningLevel.value]] || "中",
);
const reasoningActiveWidth = computed(
  () => reasoningTrackWidths[draftReasoningLevel.value],
);
watch(
  () => props.reasoningEffort,
  (value) => {
    if (modelSliderDragging.value) return;
    draftReasoningValue.value = Math.max(
      0,
      reasoningOptions.indexOf(String(value || "high")),
    );
  },
  { immediate: true },
);

watch(
  () => props.reasoningEnabled,
  (value) => {
    draftReasoningEnabled.value = value === true;
  },
  { immediate: true },
);

function reasoningValueFromInput(value: number | number[]) {
  const index = Array.isArray(value) ? Number(value[0]) : Number(value);
  return Math.min(Math.max(Number.isFinite(index) ? index : 1, 0), 3);
}

function reasoningEffortFromIndex(index: number) {
  return reasoningOptions[index] || "high";
}

function notifyModelPreview(
  modelId: number,
  reasoningEffort: string,
  reasoningEnabled: boolean,
) {
  props.modelPreviewChange?.(
    modelId,
    reasoningEffort,
    reasoningEnabled,
  );
  emit(
    "preview-model",
    modelId,
    reasoningEffort,
    reasoningEnabled,
  );
}

function notifyModelCommit(
  modelId: number,
  reasoningEffort: string,
  reasoningEnabled: boolean,
) {
  props.modelCommitChange?.(
    modelId,
    reasoningEffort,
    reasoningEnabled,
  );
  emit(
    "update:model",
    modelId,
    reasoningEffort,
    reasoningEnabled,
  );
}

function previewReasoningIndex(value: number | number[]) {
  const index = Math.round(reasoningValueFromInput(value));
  modelSliderDragging.value = true;
  draftReasoningValue.value = index;
  notifyModelPreview(
    Number(props.selectedModelId),
    reasoningEffortFromIndex(index),
    draftReasoningEnabled.value,
  );
}

function applyReasoningIndex(value: number | number[]) {
  const index = Math.round(reasoningValueFromInput(value));
  const effort = reasoningEffortFromIndex(index);
  modelSliderDragging.value = false;
  draftReasoningValue.value = index;
  notifyModelCommit(
    Number(props.selectedModelId),
    effort,
    draftReasoningEnabled.value,
  );
}

function openModelMenuPage(view: "models" | "reasoning") {
  modelMenuView.value = view;
}

function backModelMenuPage() {
  modelMenuView.value = "main";
}

function resetModelMenu() {
  modelMenuView.value = "main";
}

function selectModel(model: any) {
  modelSliderDragging.value = false;
  draftReasoningEnabled.value = model.supportsReasoning === true;
  const effort =
    model.defaultReasoningEffort || props.reasoningEffort || "high";
  draftReasoningValue.value = Math.max(0, reasoningOptions.indexOf(effort));
  notifyModelCommit(
    Number(model.id),
    effort,
    draftReasoningEnabled.value,
  );
  backModelMenuPage();
}

function selectReasoningEnabled(value: boolean) {
  modelSliderDragging.value = false;
  draftReasoningEnabled.value = value;
  notifyModelCommit(
    Number(props.selectedModelId),
    props.reasoningEffort || "high",
    value,
  );
  backModelMenuPage();
}

function providerLabel(model: any) {
  return String(model.apiType || model.provider || "");
}
const draftKey = computed(() => {
  const conversationId = String(props.conversationId || "").trim();
  if (conversationId) return `conversation:${conversationId}`;
  const projectId = String(currentWorkspace.value?.projectId || "").trim();
  return projectId ? `new:project:${projectId}` : "new:recent";
});
const textInput = useTextInput(emit as any, isDisabled, draftKey);
const {
  text,
  inputRef,
  sendWithImage,
  sendWithVideo,
  autoResize,
  focus,
  setText,
  clear: clearText,
} = textInput;
const agentSkillNames = computed(() =>
  agentSkills.value.map((s) => s.name).filter(Boolean),
);

const hostCapabilities = computed(() => {
  const caps = ["text", "browser"];
  if (env.host === "desktop") {
    caps.push("desktop", "clipboard-host");
  }
  return caps;
});

const { holding, startHold, endHold, cancelHold } = useVoiceInput(
  (blob: Blob, duration?: number) =>
    emit("voiceAudio", blob, undefined, duration),
  isDisabled,
  () => !!props.generating || !!props.isSubmitting,
  () => ElMessage.warning("无法使用麦克风，请检查录音权限"),
);

const replyTargetExcerpt = computed(() => {
  const target = props.replyTarget;
  if (!target) return "";
  const excerpt = target.replyToExcerpt || target.content || "";
  return excerpt.length > 60 ? `${excerpt.slice(0, 60)}...` : excerpt;
});

const hasComposerContext = computed(
  () =>
    !!attachedImage.value ||
    !!attachedVideo.value ||
    selectedSkillNames.value.length > 0,
);

const filteredAgentSkills = computed(() => {
  const query = skillSearch.value.trim().toLowerCase();
  return agentSkills.value
    .filter((skill) => {
      if (!query) return true;
      return [
        skill.name,
        skill.displayName,
        skill.description,
        skill.shortDescription,
      ]
        .filter(Boolean)
        .some((value) => String(value).toLowerCase().includes(query));
    })
    .slice(0, 30);
});

const filteredSlashSkills = computed(() => {
  const query = slashQuery.value.trim().toLowerCase();
  return agentSkills.value
    .filter((skill) => {
      if (!query) return true;
      return [
        skill.name,
        skill.displayName,
        skill.description,
        skill.shortDescription,
      ]
        .filter(Boolean)
        .some((value) => String(value).toLowerCase().includes(query));
    })
    .slice(0, 12);
});

async function loadAgentSkills() {
  skillsLoading.value = true;
  try {
    const page = await fetchAgentSkills({ pageSize: 100 });
    agentSkills.value = (page.items || []).filter(
      (skill) => skill.enabled && skill.compatibilityStatus !== "blocked",
    );
  } catch {
  } finally {
    skillsLoading.value = false;
  }
}

async function handleWorkspaceSelect(workspace: WorkspaceMountSummary) {
  try {
    await selectWorkspaceMount(workspace);
    workspaceMenuOpen.value = false;
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "切换工作目录失败");
  }
}

async function handleChooseWorkspaceDirectory() {
  try {
    await chooseWorkspaceDirectory();
    workspaceMenuOpen.value = false;
  } catch {
    // chooseWorkspaceDirectory already reports a user-facing error.
  }
}

async function handleClearWorkspace() {
  try {
    await clearWorkspace();
    workspaceMenuOpen.value = false;
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "清除工作目录失败");
  }
}

function resetAddMenu() {
  skillsPanelOpen.value = false;
  skillSearch.value = "";
}

function openImagePicker() {
  addMenuOpen.value = false;
  fileInputRef.value?.click();
}

function openVideoPicker() {
  addMenuOpen.value = false;
  videoInputRef.value?.click();
}

function openGenericFilePicker() {
  addMenuOpen.value = false;
  genericFileInputRef.value?.click();
}

function handleGenericFileInput(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = "";
  if (file) emit("file", file);
}

async function handleImageInput(event: Event) {
  if (attachedVideo.value) clearVideo();
  try {
    await handleImageSelect(event);
  } catch (error) {
    ElMessage.error(
      error instanceof Error ? error.message : "图片转换为 PNG 失败",
    );
  }
}

function handleVideoInput(event: Event) {
  if (attachedImage.value) clearImage();
  handleVideoSelect(event);
}

function toggleSkill(name: string) {
  if (selectedSkillNames.value.includes(name)) {
    removeSkill(name);
  } else {
    selectedSkillNames.value.push(name);
  }
}

function removeSkill(name: string) {
  selectedSkillNames.value = selectedSkillNames.value.filter(
    (item) => item !== name,
  );
}

function closeSlashMenu() {
  slashMenuOpen.value = false;
  slashQuery.value = "";
  slashRange.value = null;
  slashActiveIndex.value = 0;
}

function handleComposerInput(event: Event) {
  autoResize();
  const target = event.target as HTMLTextAreaElement;
  const caret = target.selectionStart ?? text.value.length;
  const beforeCaret = text.value.slice(0, caret);
  const match = beforeCaret.match(/(?:^|\s)\/([^\s/]*)$/);
  if (!match) {
    closeSlashMenu();
    return;
  }
  slashQuery.value = match[1] || "";
  slashRange.value = { start: caret - slashQuery.value.length - 1, end: caret };
  slashActiveIndex.value = 0;
  slashMenuOpen.value = true;
}

function scrollActiveSlashSkillIntoView() {
  nextTick(() => {
    inputWrapperRef.value
      ?.querySelector(".slash-skill-popover .skill-option.is-active")
      ?.scrollIntoView({ block: "nearest" });
  });
}

function handleComposerKeydown(event: KeyboardEvent) {
  if (slashMenuOpen.value) {
    const count = filteredSlashSkills.value.length;
    if (event.key === "ArrowDown" && count) {
      event.preventDefault();
      slashActiveIndex.value = (slashActiveIndex.value + 1) % count;
      scrollActiveSlashSkillIntoView();
      return;
    }
    if (event.key === "ArrowUp" && count) {
      event.preventDefault();
      slashActiveIndex.value = (slashActiveIndex.value - 1 + count) % count;
      scrollActiveSlashSkillIntoView();
      return;
    }
    if (
      (event.key === "Enter" || event.key === "Tab") &&
      !event.shiftKey &&
      !event.isComposing
    ) {
      event.preventDefault();
      const skill = filteredSlashSkills.value[slashActiveIndex.value];
      if (skill) selectSlashSkill(skill.name);
      return;
    }
    if (event.key === "Escape") {
      event.preventDefault();
      closeSlashMenu();
      return;
    }
  }
  if (
    event.key === "Enter" &&
    !event.shiftKey &&
    !event.ctrlKey &&
    !event.altKey &&
    !event.metaKey &&
    !event.isComposing
  ) {
    handleEnterSend(event);
  }
}

function isEditableTarget(target: Element | null): boolean {
  if (!target) return false;
  if (target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement) return true;
  return target instanceof HTMLElement && target.isContentEditable;
}

function isModalInteractionTarget(target: Element | null): boolean {
  return Boolean(target?.closest('.el-overlay, [role="dialog"], [aria-modal="true"]'));
}

async function readClipboardText(): Promise<string> {
  if (window.amitiaDesktop?.readClipboardText) {
    return window.amitiaDesktop.readClipboardText();
  }
  if (navigator.clipboard?.readText) {
    return navigator.clipboard.readText();
  }
  return "";
}

async function insertComposerText(content: string) {
  const input = inputRef.value;
  if (!input || !content) return;
  input.focus();
  const start = input.selectionStart ?? text.value.length;
  const end = input.selectionEnd ?? start;
  setText(`${text.value.slice(0, start)}${content}${text.value.slice(end)}`);
  await nextTick();
  input.focus();
  const caret = start + content.length;
  input.setSelectionRange(caret, caret);
}

async function handleGlobalChatPaste(event: KeyboardEvent) {
  if (event.defaultPrevented || (!event.ctrlKey && !event.metaKey) || event.altKey || event.key.toLowerCase() !== "v") return;
  const active = document.activeElement;
  if (active === inputRef.value || isEditableTarget(active) || isModalInteractionTarget(active)) return;
  if (isInputDisabled.value || voiceMode.value) return;
  event.preventDefault();
  const pasted = await readClipboardText();
  await insertComposerText(pasted);
}

function handleGlobalComposerKey(event: KeyboardEvent) {
  if (
    event.defaultPrevented ||
    event.isComposing ||
    event.ctrlKey ||
    event.metaKey ||
    event.altKey ||
    event.key.length !== 1
  ) {
    return;
  }
  const active = document.activeElement;
  if (active === inputRef.value || isEditableTarget(active) || isModalInteractionTarget(active)) return;
  if (isInputDisabled.value || voiceMode.value) return;
  event.preventDefault();
  void insertComposerText(event.key);
}

function selectSlashSkill(name: string) {
  const range = slashRange.value;
  if (!range) return;
  if (!selectedSkillNames.value.includes(name))
    selectedSkillNames.value.push(name);
  const before = text.value.slice(0, range.start);
  const after = text.value.slice(range.end);
  setText(`${before}${after}`);
  closeSlashMenu();
  nextTick(() => inputRef.value?.focus());
}

function handleComposerOutsidePointer(event: PointerEvent) {
  if (!inputWrapperRef.value?.contains(event.target as Node)) closeSlashMenu();
}

function buildOutgoingText(content: string) {
  const skillPrefix = selectedSkillNames.value
    .map((name) => `$${name}`)
    .join(" ");
  return [skillPrefix, content.trim()].filter(Boolean).join(" ");
}

function finishSubmit() {
  selectedSkillNames.value = [];
  closeSlashMenu();
}

async function submitComposer(event?: KeyboardEvent) {
  if (event) event.preventDefault();
  if (
    isInputDisabled.value ||
    props.generating ||
    props.isSubmitting ||
    processingImage.value ||
    uploadingVideo.value
  )
    return;
  const outgoingText = buildOutgoingText(text.value);

  if (attachedVideo.value) {
    if (!attachedVideoUrl.value) {
      ElMessage.error(videoUploadError.value || "视频尚未上传完成");
      return;
    }
    sendWithVideo(outgoingText || "[视频]", attachedVideoUrl.value);
    clearVideo();
    finishSubmit();
    return;
  }

  if (attachedImage.value) {
    if (attachedImagePreview.value) {
      sendWithImage(outgoingText, attachedImagePreview.value);
    } else {
      sendWithImage(outgoingText, await fileToBase64(attachedImage.value));
    }
    clearImage();
    finishSubmit();
    return;
  }

  if (!outgoingText) return;
  emit("send", outgoingText);
  clearText();
  finishSubmit();
  nextTick(autoResize);
}

function handleEnterSend(event: KeyboardEvent) {
  submitComposer(event);
}

function handleSendClick() {
  submitComposer();
}

function toggleVoiceMode() {
  if (holding.value) cancelHold();
  voiceMode.value = !voiceMode.value;
  closeSlashMenu();
  if (!voiceMode.value) nextTick(() => inputRef.value?.focus());
}

watch(
  () => props.conversationId,
  (conversationId) => {
    loadConversationWorkspace(conversationId || "");
  },
  { immediate: true },
);

watch(
  () => props.disabled,
  (value) => {
    if (!value) loadAgentSkills();
  },
);

watch(
  () => props.characterId,
  () => { loadAgentSkills(); },
);

onMounted(() => {
  loadAgentSkills();
  refreshRecentWorkspaces();
  document.addEventListener("pointerdown", handleComposerOutsidePointer);
  window.addEventListener("keydown", handleGlobalChatPaste, true);
  window.addEventListener("keydown", handleGlobalComposerKey, true);
});
onUnmounted(() => {
  document.removeEventListener("pointerdown", handleComposerOutsidePointer);
  window.removeEventListener("keydown", handleGlobalChatPaste, true);
  window.removeEventListener("keydown", handleGlobalComposerKey, true);
});

defineExpose({ focus, setText, clear: clearText });
</script>

<style scoped>
.chat-input-bar {
  display: flex;
  align-items: flex-end;
  justify-content: center;
  padding: 12px 24px 16px;
  border-top: 1px solid var(--surface-border);
  background: var(--chat-surface-bg);
}

.hidden-input {
  display: none;
}

.composer-stack {
  min-width: 0;
  flex: 0 1 820px;
  width: min(100%, 820px);
}

.input-wrapper {
  display: flex;
  flex-direction: column;
  min-width: 0;
  gap: 2px;
  padding: 8px 9px;
  border: 1px solid var(--composer-border);
  border-radius: var(--radius-composer);
  background: var(--workbench-sidebar-bg);
  box-shadow: var(--composer-shadow);
  transition:
    border-color 0.18s ease,
    box-shadow 0.18s ease;
}

.permission-trigger {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  height: 30px;
  padding: 0 8px;
  border: 1px solid transparent;
  border-radius: 8px;
  background: transparent;
  color: var(--ac-color-text-muted);
  font: inherit;
  font-size: 11px;
  cursor: pointer;
  transition:
    border-color 0.18s ease,
    background-color 0.18s ease,
    color 0.18s ease;
}

.permission-trigger:hover,
.permission-trigger:focus-visible {
  border-color: var(--ac-color-border);
  background: var(--ac-color-bg-secondary);
  color: var(--ac-color-text);
}

.permission-trigger.is-full-access {
  color: var(--ac-color-warning, #b7791f);
}

.permission-trigger:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.permission-trigger > span {
  max-width: 72px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.permission-picker-header {
  padding: 4px 6px 9px;
}

.permission-picker-header strong,
.permission-picker-header small {
  display: block;
}

.permission-picker-header strong {
  font-size: 13px;
  font-weight: 600;
}

.permission-picker-header small {
  margin-top: 3px;
  color: var(--ac-color-text-muted);
  font-size: 10px;
  line-height: 1.4;
}

.permission-option {
  display: flex;
  width: 100%;
  min-height: 50px;
  align-items: center;
  gap: 9px;
  border: 0;
  border-radius: 8px;
  padding: 7px 8px;
  background: transparent;
  color: var(--ac-color-text);
  text-align: left;
  cursor: pointer;
}

.permission-option:hover,
.permission-option.is-selected {
  background: var(--ac-color-bg-secondary);
}

.permission-option-icon {
  display: grid;
  width: 28px;
  height: 28px;
  flex: 0 0 auto;
  place-items: center;
  border-radius: 8px;
  background: var(--ac-color-bg-secondary);
  color: var(--ac-color-text-secondary);
}

.permission-option-copy {
  min-width: 0;
  flex: 1;
}

.permission-option-copy strong,
.permission-option-copy small {
  display: block;
}

.permission-option-copy strong {
  font-size: 12px;
  font-weight: 600;
}

.permission-option-copy small {
  margin-top: 2px;
  color: var(--ac-color-text-muted);
  font-size: 10px;
  line-height: 1.35;
}

.permission-check {
  flex: 0 0 auto;
  color: var(--ac-color-primary);
}

.workspace-trigger {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  max-width: min(180px, 42vw);
  height: 30px;
  padding: 0 8px;
  border: 1px solid transparent;
  border-radius: 8px;
  background: transparent;
  color: var(--ac-color-text-muted);
  font: inherit;
  font-size: 11px;
  cursor: pointer;
}

.workspace-trigger:hover,
.workspace-trigger:focus-visible,
.workspace-trigger.has-workspace {
  border-color: var(--ac-color-border);
  background: var(--ac-color-bg-secondary);
  color: var(--ac-color-text);
}

.workspace-trigger:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.workspace-trigger > span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.workspace-picker {
  color: var(--ac-color-text);
}

.workspace-picker-header {
  padding: 4px 6px 9px;
}

.workspace-picker-header strong,
.workspace-picker-header small {
  display: block;
}

.workspace-picker-header strong {
  font-size: 13px;
  font-weight: 600;
}

.workspace-picker-header small {
  margin-top: 3px;
  color: var(--ac-color-text-muted);
  font-size: 10px;
  line-height: 1.4;
}

.workspace-recent-list {
  display: grid;
  gap: 2px;
  max-height: 250px;
  overflow-y: auto;
}

.workspace-option,
.workspace-action {
  display: flex;
  align-items: center;
  width: 100%;
  border: 0;
  background: transparent;
  color: var(--ac-color-text);
  cursor: pointer;
}

.workspace-option {
  gap: 9px;
  min-height: 46px;
  padding: 6px 8px;
  border-radius: 8px;
  text-align: left;
}

.workspace-option:hover,
.workspace-option.is-selected,
.workspace-action:hover {
  background: var(--ac-color-bg-secondary);
}

.workspace-option:disabled,
.workspace-action:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.workspace-option-icon {
  flex: 0 0 auto;
  color: var(--ac-color-text-secondary);
}

.workspace-option-copy {
  min-width: 0;
  flex: 1;
}

.workspace-option-copy strong,
.workspace-option-copy small {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.workspace-option-copy strong {
  font-size: 12px;
  font-weight: 550;
}

.workspace-option-copy small {
  margin-top: 2px;
  color: var(--ac-color-text-muted);
  font-size: 10px;
}

.workspace-selected-icon {
  flex: 0 0 auto;
  color: var(--ac-color-primary);
}

.workspace-empty {
  padding: 14px 8px;
  color: var(--ac-color-text-muted);
  font-size: 11px;
  text-align: center;
}

.workspace-picker-divider {
  height: 1px;
  margin: 6px 2px;
  background: var(--ac-color-border-light);
}

.workspace-action {
  gap: 9px;
  min-height: 36px;
  padding: 6px 8px;
  border-radius: 8px;
  font-size: 12px;
}

.workspace-action.is-clear {
  color: var(--ac-color-text-muted);
}

.input-row {
  display: flex;
  align-items: flex-end;
  min-width: 0;
  flex: 1;
  gap: 6px;
}

.input-left-actions,
.input-actions {
  display: flex;
  align-items: center;
  flex-shrink: 0;
  min-height: 34px;
}

.input-left-actions { gap: 4px; }
.input-actions { gap: 4px; }

.input-left-actions :deep(.el-button + .el-button) {
  margin-left: 0;
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
  transition:
    border-color 0.18s ease,
    background 0.18s ease,
    color 0.18s ease;
}

.voice-mode-toggle.is-voice-mode {
  border-color: var(--ac-color-border-strong);
  background: var(--ac-color-bg-secondary);
  color: var(--ac-color-text);
}

.model-effort-trigger {
  max-width: 200px;
  height: 28px;
  overflow: hidden;
  border: 1px solid transparent;
  border-radius: 8px;
  padding: 0 9px;
  background: transparent;
  color: var(--ac-color-text-muted);
  font: inherit;
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: pointer;
  transition:
    border-color 160ms ease,
    background-color 160ms ease;
}

.model-effort-trigger:hover,
.model-effort-trigger:focus-visible {
  border-color: var(--composer-border);
  background: var(--ac-color-bg-secondary);
}

.model-effort-menu,
.model-list-panel {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.model-menu-viewport {
  position: relative;
  min-width: 0;
  overflow: hidden;
}

.model-menu-page {
  min-width: 0;
}

.model-menu-fade-enter-active {
  transition: opacity 220ms cubic-bezier(0.22, 1, 0.36, 1);
  will-change: opacity;
}

.model-menu-fade-leave-active {
  transition: opacity 150ms ease-in;
  pointer-events: none;
}

.model-menu-fade-enter-from,
.model-menu-fade-leave-to {
  opacity: 0;
}

:global(.composer-model-popper.el-popover.el-popper) {
  --el-popper-border-radius: 16px;
  border-radius: 16px;
}

.model-effort-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 34px;
  padding: 0 8px;
  color: var(--ac-color-text-muted);
  font-size: 12px;
}

.model-effort-row--button {
  width: 100%;
  border: 0;
  border-radius: 7px;
  background: transparent;
  font: inherit;
  cursor: pointer;
}

.model-effort-row--button:hover {
  background: var(--ac-color-primary-bg);
}

.model-effort-model {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  max-width: 170px;
  overflow: hidden;
  color: var(--ac-color-text);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.model-effort-slider {
  padding: 7px 8px 4px;
}

.model-effort-slider-track {
  --model-effort-cap-inset: 16px;
  position: relative;
  display: flex;
  align-items: center;
  height: 48px;
}

.model-effort-track-range {
  position: absolute;
  top: 9px;
  right: var(--model-effort-cap-inset);
  left: var(--model-effort-cap-inset);
  height: 30px;
}

.model-effort-track-base,
.model-effort-track-active {
  position: absolute;
  top: 0;
  left: 0;
  height: 100%;
  border-radius: 999px;
}

.model-effort-track-base {
  right: 0;
  border: 1px solid var(--ac-color-border);
  background: var(--ac-color-bg-secondary);
}

.model-effort-track-active {
  background: var(--ac-color-primary);
  transition:
    width 180ms cubic-bezier(0.2, 0.8, 0.2, 1),
    background-color 180ms ease;
}

.model-effort-track-active.is-empty {
  background: transparent;
}

.model-effort-markers {
  position: absolute;
  top: 50%;
  right: 0;
  left: 0;
  z-index: 1;
  height: 6px;
  transform: translateY(-50%);
  pointer-events: none;
}

.model-effort-markers span {
  position: absolute;
  top: 0;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--ac-color-text-muted);
  opacity: 0.62;
  transform: translateX(-50%);
  transition:
    background-color 180ms ease,
    opacity 180ms ease;
}

.model-effort-markers span.active {
  background: #fff;
  opacity: 0.78;
}

.model-effort-slider :deep(.el-slider) {
  position: relative;
  z-index: 2;
  --el-slider-height: 30px;
  --el-slider-button-size: 32px;
  --el-slider-button-wrapper-size: 48px;
  --el-slider-button-wrapper-offset: -9px;
  height: 48px;
}

.model-effort-slider :deep(.el-slider__button-wrapper) {
  transition: left 180ms cubic-bezier(0.2, 0.8, 0.2, 1);
}

.model-effort-slider :deep(.el-slider__runway) {
  background: transparent;
  margin: 0 32px;
}

.model-effort-slider :deep(.el-slider__bar) {
  background: transparent;
}

.model-effort-slider :deep(.el-slider__button) {
  border: 0;
  background: #fff;
  box-shadow:
    0 2px 7px rgba(0, 0, 0, 0.24),
    0 0 0 1px rgba(0, 0, 0, 0.05);
}

.model-effort-slider :deep(.el-slider__button:hover),
.model-effort-slider :deep(.el-slider__button.hover),
.model-effort-slider :deep(.el-slider__button.dragging) {
  transform: scale(1.04);
}

.model-effort-slider :deep(.el-slider.is-disabled .el-slider__button) {
  opacity: 0.72;
}

.model-effort-slider-track.disabled .model-effort-track-base,
.model-effort-slider-track.disabled .model-effort-track-active {
  opacity: 0.45;
}

.model-effort-slider-track.disabled .model-effort-markers {
  opacity: 0.5;
}

.model-effort-labels {
  display: flex;
  justify-content: space-between;
  margin-top: -3px;
  color: var(--ac-color-text-muted);
  font-size: 10px;
}

.model-effort-hint {
  margin-top: 6px;
  color: var(--ac-color-danger);
  font-size: 11px;
}

.model-list-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 2px 4px 8px;
}

.model-list-header button {
  display: grid;
  width: 26px;
  height: 26px;
  place-items: center;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--ac-color-text-muted);
  cursor: pointer;
}

.model-list-header button:hover {
  background: var(--ac-color-primary-bg);
}

.model-list-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  min-height: 48px;
  border: 0;
  border-radius: 8px;
  padding: 6px 8px;
  background: transparent;
  color: var(--ac-color-text);
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.model-list-item:hover,
.model-list-item.active {
  background: var(--ac-color-primary-bg);
}

.model-list-item span {
  min-width: 0;
}

.model-list-item strong,
.model-list-item small {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.model-list-item small {
  margin-top: 2px;
  color: var(--ac-color-text-muted);
  font-size: 10px;
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
  transition:
    border-color 0.18s ease,
    background 0.18s ease,
    color 0.18s ease;
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
  0%,
  100% {
    box-shadow: 0 0 0 0
      color-mix(in srgb, var(--ac-color-danger) 24%, transparent);
  }
  50% {
    box-shadow: 0 0 0 5px transparent;
  }
}

.composer-context {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 7px;
  padding: 0 2px 6px;
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
  margin: 0 0 4px;
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

/* Codex-like composer shell: floating, two-level, while retaining Amitia actions. */
.chat-input-bar {
  padding: 8px 24px 22px;
  border-top: 0;
  background: transparent;
}
.composer-stack {
  flex-basis: 760px;
  width: min(100%, 760px);
}
.input-wrapper {
  gap: 4px;
  padding: 8px 10px 9px;
  border-color: color-mix(in srgb, var(--composer-border) 88%, transparent);
  border-radius: 14px;
  background: var(--workbench-sidebar-bg);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
}
.input-row {
  display: grid;
  grid-template-columns: max-content 1fr max-content;
  grid-template-rows: auto 34px;
  align-items: center;
  gap: 2px 6px;
}
.input-body {
  grid-column: 1 / -1;
  grid-row: 1;
  min-height: 44px;
  align-items: flex-start;
}
.input-left-actions {
  grid-column: 1;
  grid-row: 2;
  min-height: 32px;
  align-self: end;
}
.input-actions {
  grid-column: 3;
  grid-row: 2;
  min-height: 32px;
  align-self: end;
}
.input-field {
  min-height: 44px;
  max-height: 180px;
  padding: 8px 3px 7px;
  font-size: 13px;
  line-height: 1.5;
}
.add-btn, .voice-mode-toggle {
  width: 30px;
  height: 30px;
}
:deep(.el-button--small) {
  min-width: 32px !important;
  width: 32px !important;
}
.input-left-actions :deep(.el-button),
.input-actions :deep(.el-button) {
  min-width: 32px !important;
  width: 32px !important;
}
.composer-context { padding-bottom: 4px; }
.reply-preview-bar { margin-bottom: 2px; }
@media (max-width: 768px) {
  .chat-input-bar { padding: 6px 8px calc(10px + var(--ac-safe-area-bottom)); }
  .composer-stack { flex-basis: 100%; width: 100%; }
  .input-wrapper { border-radius: 13px; }
  .input-actions { position: relative; right: -3px; }
  .input-field { max-height: 132px; }
}


.attachment-card .is-error {
  color: var(--el-color-danger);
}
</style>
