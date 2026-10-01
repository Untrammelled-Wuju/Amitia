<template>
  <div class="amrp-image-gallery" :class="{ multiple: images.length > 1 }">
    <figure v-for="(image, index) in images" :key="image.id" class="amrp-image-card">
      <div class="amrp-image-stage">
        <span v-if="!resolvedUrls[index] || image.status === 'loading'" class="amrp-image-state">
          <span class="amrp-spinner"></span>
          加载中
        </span>
        <button v-else-if="image.status === 'failed'" type="button" class="amrp-image-state failed" @click="retry(image)">
          <span>×</span>
          加载失败，点击重试
        </button>
        <img
          v-show="resolvedUrls[index]"
          :class="{ hidden: image.status === 'loading' }"
          :src="cacheBustedUrl(image, index)"
          :alt="image.alt || '图片'"
          loading="lazy"
          decoding="async"
          @load="image.status = 'ready'"
          @error="image.status = 'failed'"
          @click="openPreview(index)"
        />
        <span v-if="image.animated || isGif(image)" class="amrp-gif-badge">GIF</span>
      </div>
      <figcaption>
        <span>{{ image.alt || `图片 ${index + 1}` }}</span>
        <button type="button" @click="openPreview(index)">预览</button>
        <button type="button" @click="downloadImage(index)">保存</button>
      </figcaption>
    </figure>
  </div>

  <Teleport to="body">
    <div v-if="previewIndex >= 0" class="amrp-lightbox" @click.self="previewIndex = -1">
      <button type="button" class="amrp-lightbox-close" @click="previewIndex = -1">关闭</button>
      <button v-if="images.length > 1" type="button" class="amrp-lightbox-prev" @click="step(-1)">‹</button>
      <img :src="resolvedUrls[previewIndex]" :alt="images[previewIndex]?.alt || ''" />
      <button v-if="images.length > 1" type="button" class="amrp-lightbox-next" @click="step(1)">›</button>
      <button type="button" class="amrp-lightbox-save" @click="downloadCurrent">保存</button>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, watch } from "vue";
import { ElMessage } from "element-plus";
import type { ImageBlock } from "../types";
import {
  downloadConversationMedia,
  resolveConversationMediaUrl,
} from "../media";

const props = defineProps<{
  images: ImageBlock[];
}>();

const previewIndex = ref(-1);
const retryToken = ref(0);
const resolvedUrls = ref<string[]>([]);

function isGif(image: ImageBlock): boolean {
  return /\.gif(?:$|\?)/i.test(image.url) || image.mimeType === "image/gif";
}

async function resolveImages() {
  resolvedUrls.value = await Promise.all(
    props.images.map(async (image) => {
      try {
        return await resolveConversationMediaUrl(image.url);
      } catch {
        return "";
      }
    }),
  );
}

function cacheBustedUrl(image: ImageBlock, index: number): string {
  const value = resolvedUrls.value[index] || "";
  if (image.status !== "failed" || retryToken.value === 0) return value;
  return `${value}${value.includes("?") ? "&" : "?"}amrpRetry=${retryToken.value}`;
}

function retry(image: ImageBlock) {
  image.status = "loading";
  retryToken.value += 1;
  void resolveImages();
}

function openPreview(index: number) {
  previewIndex.value = index;
}

function step(delta: number) {
  const length = props.images.length;
  if (!length) return;
  previewIndex.value = (previewIndex.value + delta + length) % length;
}

async function downloadCurrent() {
  await downloadImage(previewIndex.value);
}

async function downloadImage(index: number) {
  const image = props.images[index];
  if (!image) return;
  try {
    const saved = await downloadConversationMedia(
      image.downloadUrl || image.url,
      image.alt || `image-${index + 1}.png`,
      image.mimeType,
    );
    if (saved) ElMessage.success("图片已保存");
  } catch (reason) {
    ElMessage.error(reason instanceof Error ? reason.message : "图片保存失败");
  }
}

watch(
  () => props.images.map((image) => image.url).join("\n"),
  () => {
    void resolveImages();
  },
  { immediate: true },
);
</script>

<style scoped>
.amrp-image-gallery {
  width: 100%;
  max-width: 700px;
  display: grid;
  gap: 10px;
  margin: 11px 0 16px;
}

.amrp-image-gallery.multiple {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

figure {
  min-width: 0;
  margin: 0;
  overflow: hidden;
  border: 1px solid var(--amrp-line);
  border-radius: 10px;
  background: var(--amrp-surface);
}

.amrp-image-stage {
  position: relative;
  min-height: 180px;
  display: grid;
  place-items: center;
  background: linear-gradient(135deg, var(--amrp-soft), var(--amrp-surface));
}

.amrp-image-stage img {
  display: block;
  width: 100%;
  height: 220px;
  object-fit: contain;
  cursor: zoom-in;
}

.amrp-image-stage img.hidden {
  position: absolute;
  width: 1px;
  height: 1px;
  opacity: 0;
  pointer-events: none;
}

.amrp-image-state {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  color: var(--amrp-muted);
  font-size: 11px;
}

.amrp-image-state.failed {
  border: 0;
  background: transparent;
  color: var(--amrp-danger);
  cursor: pointer;
}

.amrp-image-state.failed span {
  font-size: 22px;
}

.amrp-spinner {
  width: 18px;
  height: 18px;
  border: 2px solid var(--amrp-line);
  border-top-color: var(--amrp-accent);
  border-radius: 50%;
  animation: amrp-spin 1s linear infinite;
}

@keyframes amrp-spin {
  to { transform: rotate(360deg); }
}

.amrp-gif-badge {
  position: absolute;
  left: 7px;
  top: 7px;
  border-radius: 4px;
  padding: 2px 5px;
  background: rgba(0, 0, 0, 0.68);
  color: white;
  font-size: 9px;
}

figcaption {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 7px 9px;
  color: var(--amrp-muted);
  font-size: 10.5px;
}

figcaption span {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

figcaption button {
  border: 0;
  background: transparent;
  color: var(--amrp-accent);
  font: inherit;
  cursor: pointer;
}

.amrp-lightbox {
  position: fixed;
  z-index: 4000;
  inset: 0;
  display: grid;
  place-items: center;
  background: rgba(7, 8, 10, 0.88);
}

.amrp-lightbox img {
  max-width: 88vw;
  max-height: 88vh;
  object-fit: contain;
}

.amrp-lightbox button,
.amrp-lightbox a {
  position: fixed;
  top: 18px;
  border: 0;
  border-radius: 8px;
  padding: 7px 10px;
  background: rgba(255, 255, 255, 0.14);
  color: white;
  font: inherit;
  text-decoration: none;
  cursor: pointer;
}

.amrp-lightbox-close { right: 18px; }
.amrp-lightbox a,
.amrp-lightbox-save { right: 82px; }
.amrp-lightbox-prev { left: 18px; top: 50%; font-size: 34px; }
.amrp-lightbox-next { right: 18px; top: 50%; font-size: 34px; }

@media (max-width: 700px) {
  .amrp-image-gallery.multiple {
    grid-template-columns: 1fr;
  }
}
</style>
