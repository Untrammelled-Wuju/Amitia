<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="bubble-image" v-if="imageUrl" @click="showImagePreview = true">
    <img :src="imageUrl" alt="用户上传图片" style="width:150px;height:120px;object-fit:cover;display:block;border-radius:6px;max-width:100%" />
    <div class="image-overlay">
      <el-icon><ZoomIn /></el-icon>
    </div>
  </div>
  <div class="bubble-video" v-if="videoUrl" @click="handleVideoClick(videoUrl)">
    <video :src="videoUrl" preload="metadata" />
    <div class="video-overlay">
      <el-icon size="28"><VideoPlay /></el-icon>
    </div>
  </div>
  <el-dialog v-model="showImagePreview" title="图片预览" width="90%" :close-on-click-modal="true" class="image-preview-dialog">
    <img :src="imageUrl" style="width:100%;max-height:70vh;object-fit:contain" />
  </el-dialog>
  <el-dialog v-model="showVideoPreview" title="视频预览" width="90%" :close-on-click-modal="true" class="video-preview-dialog" @closed="stopPreviewVideo">
    <video :src="previewVideoUrl" controls autoplay style="width:100%;max-height:70vh;border-radius:6px" />
  </el-dialog>
</template>

<script setup lang="ts">
import { ref } from "vue"
import { ZoomIn, VideoPlay } from "@element-plus/icons-vue"

defineProps<{
  imageUrl?: string
  videoUrl?: string
}>()

const showImagePreview = ref(false)
const showVideoPreview = ref(false)
const previewVideoUrl = ref('')

function handleVideoClick(url: string) {
  previewVideoUrl.value = url
  showVideoPreview.value = true
}

function stopPreviewVideo() {
  previewVideoUrl.value = ''
}
</script>

<style scoped>
.bubble-image {
  position: relative;
  display: inline-block;
  max-width: 150px;
  margin-top: 6px;
  border-radius: 8px;
  overflow: hidden;
  cursor: pointer;
  border: 1px solid var(--ac-color-border-light);
  transition: border-color 0.2s;
}
.bubble-image:hover {
  border-color: var(--ac-color-primary);
}
.bubble-image img {
  display: block;
  border-radius: 6px;
}
.image-overlay {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.05);
  display: flex;
  align-items: center;
  justify-content: center;
  opacity: 0;
  transition: opacity 0.2s;
  color: #fff;
  font-size: 24px;
}
.bubble-image:hover .image-overlay {
  opacity: 1;
}
.bubble-video {
  position: relative;
  border-radius: 6px;
  display: inline-block;
  max-width: 260px;
  min-width: 120px;
  cursor: pointer;
  overflow: hidden;
  flex-shrink: 0;
  margin-top: 6px;
}
.bubble-video video {
  display: block;
  width: 100%;
  max-width: 260px;
  height: 160px;
  object-fit: cover;
  border-radius: 6px;
}
.video-overlay {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  color: white;
  opacity: 0.7;
  transition: opacity 0.2s;
  pointer-events: none;
  text-shadow: 0 2px 8px rgba(0,0,0,0.5);
}
.bubble-video:hover .video-overlay {
  opacity: 1;
}
.image-preview-dialog .el-dialog__body {
  padding: 0;
}
.video-preview-dialog .el-dialog__body {
  padding: 0;
}
</style>
