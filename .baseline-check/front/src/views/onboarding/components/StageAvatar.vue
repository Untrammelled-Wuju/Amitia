<template>
  <div class="ob-stage-inner">
    <div class="ob-avatar-scene">
      <div class="ob-avatar-prompt">
        <div class="ob-character-line">为角色选一张头像</div>
        <div class="ob-identity-context">
          可以是照片、插画或任何你觉得合适的图片
        </div>
      </div>

      <div class="ob-avatar-preview-area">
        <div
          class="ob-avatar-circle"
          :class="{ 'has-image': previewUrl }"
          @click="triggerFileInput"
        >
          <img v-if="previewUrl" :src="previewUrl" alt="角色头像预览" />
          <span v-else class="ob-avatar-placeholder">+</span>
        </div>
        <p class="ob-avatar-hint" v-if="!previewUrl">点击上传头像</p>
        <p class="ob-avatar-hint" v-else>点击更换头像</p>
      </div>

      <input
        ref="fileInputRef"
        type="file"
        accept="image/*"
        class="ob-avatar-file-input"
        @change="onFileChange"
      />

      <div class="ob-avatar-actions">
        <button class="ob-skip-btn" @click="$emit('skip')">跳过</button>
        <button class="ob-primary-ghost" @click="$emit('next', avatarFile)">
          继续
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from "vue";

defineEmits<{
  next: [file: File | null];
  skip: [];
}>();

const avatarFile = ref<File | null>(null);
const previewUrl = ref("");
const fileInputRef = ref<HTMLInputElement | null>(null);

function triggerFileInput() {
  fileInputRef.value?.click();
}

function onFileChange(e: Event) {
  const input = e.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;

  if (!file.type.startsWith("image/")) return;

  avatarFile.value = file;
  const url = URL.createObjectURL(file);
  if (previewUrl.value) URL.revokeObjectURL(previewUrl.value);
  previewUrl.value = url;
}
</script>

<style scoped>
.ob-avatar-scene {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 28px;
  height: 100%;
  padding: 40px 24px;
}

.ob-avatar-prompt {
  text-align: center;
}

.ob-avatar-preview-area {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
}

.ob-avatar-circle {
  width: 140px;
  height: 140px;
  border-radius: 50%;
  border: 2px dashed rgba(255, 255, 255, 0.18);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  overflow: hidden;
  transition: border-color 0.2s;
  background: rgba(255, 255, 255, 0.04);
}

.ob-avatar-circle:hover {
  border-color: rgba(255, 255, 255, 0.35);
}

.ob-avatar-circle.has-image {
  border-style: solid;
  border-color: rgba(255, 255, 255, 0.12);
}

.ob-avatar-circle img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.ob-avatar-placeholder {
  font-size: 40px;
  color: rgba(255, 255, 255, 0.25);
  line-height: 1;
  user-select: none;
}

.ob-avatar-hint {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.35);
  margin: 0;
}

.ob-avatar-file-input {
  display: none;
}

.ob-avatar-actions {
  display: flex;
  gap: 16px;
  align-items: center;
}

.ob-skip-btn {
  padding: 10px 22px;
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 12px;
  background: transparent;
  color: rgba(255, 255, 255, 0.55);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
}

.ob-skip-btn:hover {
  border-color: rgba(255, 255, 255, 0.3);
  color: rgba(255, 255, 255, 0.8);
}
</style>
