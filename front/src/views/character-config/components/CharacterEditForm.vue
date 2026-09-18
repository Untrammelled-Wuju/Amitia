<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-tabs v-model="activeTabModel">
    <el-tab-pane label="编辑角色" name="edit">
      <el-form label-position="top" class="char-form">
        <div class="form-grid">
          <el-form-item label="名称">
            <el-input v-model="nameModel" placeholder="角色名称" />
          </el-form-item>

          <el-form-item label="头像">
            <div class="avatar-upload-row">
              <div
                class="avatar-preview"
                :class="{ 'is-uploading': uploadingAvatar }"
                @click="triggerUpload"
                :title="avatarModel ? '点击更换头像' : '点击上传头像'"
              >
                <el-icon
                  v-if="uploadingAvatar"
                  class="is-loading"
                  :size="24"
                >
                  <Loading />
                </el-icon>
                <img
                  v-else-if="avatarModel && !avatarLoadFailed"
                  :src="avatarModel"
                  class="avatar-img"
                  @error="onAvatarError"
                />
                <el-icon v-else :size="28"><Plus /></el-icon>
              </div>
              <input
                ref="fileInputRef"
                type="file"
                accept="image/*"
                hidden
                @change="onFileChange"
              />
              <el-input
                v-model="avatarModel"
                placeholder="输入头像 URL"
                size="small"
              />
            </div>
          </el-form-item>

          <el-form-item label="身份">
            <el-input
              v-model="identityModel"
              placeholder="例如: AI 虚拟角色"
            />
          </el-form-item>

          <el-form-item label="性格">
            <el-input
              v-model="personalityModel"
              placeholder="例如: 温和、体贴、有耐心"
            />
          </el-form-item>

          <el-form-item label="说话风格">
            <el-input
              v-model="speakingStyleModel"
              placeholder="例如: 简短自然、轻声细语"
            />
          </el-form-item>

          <el-form-item label="关系氛围">
            <el-input
              v-model="relationshipStyleModel"
              placeholder="例如: 亲近但保持边界"
            />
          </el-form-item>

          <el-form-item class="form-item-full" label="角色描述">
            <el-input
              v-model="descriptionModel"
              type="textarea"
              :rows="3"
              placeholder="用于角色卡简介和角色背景描述"
            />
          </el-form-item>

          <el-form-item label="创作者">
            <el-input v-model="creatorModel" placeholder="角色卡作者" />
          </el-form-item>

          <el-form-item label="角色卡版本">
            <el-input v-model="characterVersionModel" placeholder="例如 1.0.0" />
          </el-form-item>

          <el-form-item class="form-item-full" label="标签">
            <el-input
              v-model="tagsTextModel"
              placeholder="使用英文逗号分隔，例如：日常, 治愈, 科幻"
            />
          </el-form-item>
        </div>

        <PersonalitySliders
          v-model="personalityConfigModel"
          style="margin-bottom: 16px"
        />

        <el-form-item label="场景设定">
          <el-input
            v-model="scenarioModel"
            type="textarea"
            :rows="3"
            placeholder="角色所处的世界、地点或初始情境"
          />
        </el-form-item>

        <el-form-item label="系统提示词 (System Prompt)">
          <div class="textarea-toolbar">
            <el-button
              text
              size="small"
              :icon="FullScreen"
              @click="emit('showFullPrompt')"
              >全屏编辑</el-button
            >
            <el-button text size="small" @click="emit('resetPrompt')"
              >恢复默认</el-button
            >
          </div>
          <el-input
            v-model="characterBaseModel"
            type="textarea"
            :rows="8"
            placeholder="编写角色的 System Prompt..."
          />
        </el-form-item>

        <el-form-item label="安全边界规则">
          <div class="textarea-toolbar">
            <el-button
              text
              size="small"
              :icon="FullScreen"
              @click="emit('showFullBounds')"
              >全屏编辑</el-button
            >
            <el-button text size="small" @click="emit('resetBounds')"
              >恢复默认</el-button
            >
          </div>
          <el-input
            v-model="boundaryRulesModel"
            type="textarea"
            :rows="5"
            placeholder="每行一条规则..."
          />
        </el-form-item>

        <el-form-item label="示例对话">
          <el-input
            v-model="exampleMessagesModel"
            type="textarea"
            :rows="5"
            placeholder="{{user}}: ...&#10;{{char}}: ..."
          />
        </el-form-item>

        <el-form-item label="备选问候">
          <el-input
            v-model="alternateGreetingsTextModel"
            type="textarea"
            :rows="5"
            placeholder="每行一条备选问候"
          />
        </el-form-item>

        <el-form-item label="Post-history 指令">
          <el-input
            v-model="postHistoryInstructionsModel"
            type="textarea"
            :rows="4"
            placeholder="放在历史消息之后的附加指令"
          />
        </el-form-item>

        <div class="form-actions">
          <el-checkbox
            v-model="isActiveModel"
            :disabled="isActive && !hasOtherActive"
          >
            设为当前启用角色
          </el-checkbox>
          <el-button type="primary" :loading="saving" @click="emit('save')">
            {{ selectedId ? "保存修改" : "创建角色" }}
          </el-button>
        </div>
      </el-form>
    </el-tab-pane>

    <el-tab-pane label="实时测试" name="test">
      <slot name="test" />
    </el-tab-pane>
  </el-tabs>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { FullScreen, Loading, Plus } from "@element-plus/icons-vue";
import PersonalitySliders from "../../../components/PersonalitySliders.vue";
import type { PersonalityConfig } from "../composables/types";

const props = defineProps<{
  activeTab: string;
  name: string;
  avatar: string;
  identity: string;
  personality: string;
  speakingStyle: string;
  relationshipStyle: string;
  characterBase: string;
  boundaryRules: string;
  description: string;
  scenario: string;
  exampleMessages: string;
  alternateGreetingsText: string;
  postHistoryInstructions: string;
  creator: string;
  characterVersion: string;
  tagsText: string;
  personalityConfig: PersonalityConfig;
  isActive: boolean;
  hasOtherActive: boolean;
  saving: boolean;
  selectedId: string;
  uploadingAvatar: boolean;
}>();

const emit = defineEmits<{
  (e: "update:activeTab", v: string): void;
  (e: "update:name", v: string): void;
  (e: "update:avatar", v: string): void;
  (e: "update:identity", v: string): void;
  (e: "update:personality", v: string): void;
  (e: "update:speakingStyle", v: string): void;
  (e: "update:relationshipStyle", v: string): void;
  (e: "update:characterBase", v: string): void;
  (e: "update:boundaryRules", v: string): void;
  (e: "update:description", v: string): void;
  (e: "update:scenario", v: string): void;
  (e: "update:exampleMessages", v: string): void;
  (e: "update:alternateGreetingsText", v: string): void;
  (e: "update:postHistoryInstructions", v: string): void;
  (e: "update:creator", v: string): void;
  (e: "update:characterVersion", v: string): void;
  (e: "update:tagsText", v: string): void;
  (e: "update:personalityConfig", v: PersonalityConfig): void;
  (e: "update:isActive", v: boolean): void;
  (e: "showFullPrompt"): void;
  (e: "showFullBounds"): void;
  (e: "resetPrompt"): void;
  (e: "resetBounds"): void;
  (e: "save"): void;
  (e: "uploadAvatar", v: File): void;
}>();

const fileInputRef = ref<HTMLInputElement>();
const avatarLoadFailed = ref(false);

function triggerUpload() {
  if (props.uploadingAvatar) return;
  fileInputRef.value?.click();
}
function onFileChange(e: Event) {
  const input = e.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;
  if (!file.type.startsWith("image/")) {
    return;
  }
  emit("uploadAvatar", file);
  input.value = "";
}

function onAvatarError() {
  avatarLoadFailed.value = true;
}

watch(
  () => props.avatar,
  () => {
    avatarLoadFailed.value = false;
  },
);

const activeTabModel = computed({
  get: () => props.activeTab,
  set: (v) => emit("update:activeTab", v),
});
const nameModel = computed({
  get: () => props.name,
  set: (v) => emit("update:name", v),
});
const avatarModel = computed({
  get: () => props.avatar,
  set: (v) => emit("update:avatar", v),
});
const identityModel = computed({
  get: () => props.identity,
  set: (v) => emit("update:identity", v),
});
const personalityModel = computed({
  get: () => props.personality,
  set: (v) => emit("update:personality", v),
});
const speakingStyleModel = computed({
  get: () => props.speakingStyle,
  set: (v) => emit("update:speakingStyle", v),
});
const relationshipStyleModel = computed({
  get: () => props.relationshipStyle,
  set: (v) => emit("update:relationshipStyle", v),
});
const characterBaseModel = computed({
  get: () => props.characterBase,
  set: (v) => emit("update:characterBase", v),
});
const boundaryRulesModel = computed({
  get: () => props.boundaryRules,
  set: (v) => emit("update:boundaryRules", v),
});
const descriptionModel = computed({
  get: () => props.description,
  set: (v) => emit("update:description", v),
});
const scenarioModel = computed({
  get: () => props.scenario,
  set: (v) => emit("update:scenario", v),
});
const exampleMessagesModel = computed({
  get: () => props.exampleMessages,
  set: (v) => emit("update:exampleMessages", v),
});
const alternateGreetingsTextModel = computed({
  get: () => props.alternateGreetingsText,
  set: (v) => emit("update:alternateGreetingsText", v),
});
const postHistoryInstructionsModel = computed({
  get: () => props.postHistoryInstructions,
  set: (v) => emit("update:postHistoryInstructions", v),
});
const creatorModel = computed({
  get: () => props.creator,
  set: (v) => emit("update:creator", v),
});
const characterVersionModel = computed({
  get: () => props.characterVersion,
  set: (v) => emit("update:characterVersion", v),
});
const tagsTextModel = computed({
  get: () => props.tagsText,
  set: (v) => emit("update:tagsText", v),
});
const personalityConfigModel = computed({
  get: () => props.personalityConfig,
  set: (v) => emit("update:personalityConfig", v),
});
const isActiveModel = computed({
  get: () => props.isActive,
  set: (v) => emit("update:isActive", v),
});
</script>

<style scoped>
.char-form {
  width: 100%;
  min-width: 0;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 0 14px;
  min-width: 0;
}

.form-grid :deep(.el-form-item) {
  min-width: 0;
  margin-bottom: 18px;
}

.form-grid :deep(.el-form-item__content),
.char-form :deep(.el-form-item__content) {
  min-width: 0;
}

.char-form :deep(.el-input),
.char-form :deep(.el-textarea),
.char-form :deep(.el-select) {
  width: 100%;
  min-width: 0;
}

.form-item-full {
  grid-column: 1 / -1;
}

.avatar-upload-row {
  display: grid;
  grid-template-columns: 72px minmax(0, 1fr);
  align-items: center;
  gap: 8px;
  width: 100%;
  min-width: 0;
}

.avatar-preview {
  width: 72px;
  height: 72px;
  border-radius: 50%;
  border: 2px dashed var(--ac-color-border);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  overflow: hidden;
  flex-shrink: 0;
  background: var(--ac-color-surface);
  transition: border-color 0.2s;
}

.avatar-preview:hover {
  border-color: var(--ac-color-primary);
}

.avatar-preview.is-uploading {
  cursor: wait;
  opacity: 0.72;
}

.avatar-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.textarea-toolbar {
  display: flex;
  justify-content: flex-end;
  gap: 4px;
  margin-bottom: -4px;
}

.form-actions {
  position: sticky;
  bottom: 0;
  z-index: 3;
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 4px;
  padding: 14px 0 4px;
  border-top: 1px solid var(--ac-color-border-light);
  background: var(--ac-color-surface);
}

@media (max-width: 520px) {
  .form-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .form-item-full {
    grid-column: auto;
  }

  .form-actions {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
