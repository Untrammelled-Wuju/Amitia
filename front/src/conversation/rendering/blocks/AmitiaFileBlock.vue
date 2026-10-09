<template>
  <AmitiaArtifactBlock v-if="isHtml" :block="htmlArtifact" />
  <AmitiaImageBlock v-else-if="isImage" :images="fileImages" />
  <div v-else class="amrp-file">
    <div class="amrp-file-icon">{{ extensionLabel }}</div>
    <div class="amrp-file-main">
      <div class="amrp-file-title">{{ block.name }}</div>
      <div class="amrp-file-meta">
        <span v-if="status === 'loading'">加载中</span>
        <span v-else-if="status === 'failed'">{{ error || "加载失败" }}</span>
        <span v-else>{{ [formatBytes(block.size), block.mimeType].filter(Boolean).join(" · ") }}</span>
      </div>
    </div>
    <div class="amrp-file-actions">
      <button v-if="status === 'failed'" type="button" @click="retry">重试</button>
      <button v-if="status === 'ready'" type="button" :disabled="downloading" @click="download">
        {{ downloading ? "下载中" : "下载" }}
      </button>
      <slot name="extension" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import type { FileBlock } from "../types";
import { formatBytes } from "../utils";
import { downloadConversationMedia } from "../media";
import AmitiaArtifactBlock from "./AmitiaArtifactBlock.vue";
import AmitiaImageBlock from "./AmitiaImageBlock.vue";
import type { ArtifactBlock, ImageBlock } from "../types";

const props = defineProps<{
  block: FileBlock;
}>();

const status = ref(props.block.status);
const error = ref(props.block.error ?? "");
const downloading = ref(false);
const isHtml = computed(() => Boolean(props.block.url) && (props.block.mimeType?.includes("html") || /\.html?$/i.test(props.block.name)));
const isImage = computed(() => Boolean(props.block.url) && (props.block.mimeType?.startsWith("image/") || /\.(png|jpe?g|gif|webp|bmp|avif)$/i.test(props.block.name)));
const htmlArtifact = computed<ArtifactBlock>(() => ({ ...props.block, kind: "artifact", artifactKind: "html", title: props.block.name }));
const fileImages = computed<ImageBlock[]>(() => [{ ...props.block, kind: "image", url: props.block.url || "", alt: props.block.name }]);

watch(
  () => props.block,
  (block) => {
    status.value = block.status;
    error.value = block.error ?? "";
  },
  { deep: true },
);

const extensionLabel = computed(() => {
  const match = /\.([a-z0-9]+)$/i.exec(props.block.name);
  return (match?.[1] ?? "FILE").slice(0, 4).toUpperCase();
});

async function download() {
  if ((!props.block.url && !props.block.downloadUrl) || downloading.value) return;
  downloading.value = true;
  try {
    const saved = await downloadConversationMedia(
      props.block.downloadUrl || props.block.url || "",
      props.block.name,
      props.block.mimeType,
    );
    if (saved) ElMessage.success("文件已保存");
  } catch (reason) {
    ElMessage.error(reason instanceof Error ? reason.message : "文件下载失败");
  } finally {
    downloading.value = false;
  }
}

function retry() {
  status.value = props.block.url || props.block.downloadUrl ? "ready" : "failed";
  error.value = "";
}
</script>

<style scoped>
.amrp-file {
  max-width: 560px;
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 10px 0;
  border: 1px solid var(--amrp-line);
  border-radius: 10px;
  padding: 9px 10px;
  background: var(--amrp-surface);
}

.amrp-file-icon {
  width: 32px;
  height: 36px;
  display: grid;
  flex: 0 0 auto;
  place-items: center;
  border-radius: 7px;
  background: var(--amrp-soft);
  color: var(--amrp-muted);
  font-size: 10px;
  font-weight: 750;
}

.amrp-file-main {
  min-width: 0;
  flex: 1;
}

.amrp-file-title {
  overflow: hidden;
  color: var(--amrp-text);
  font-size: 12.5px;
  font-weight: 660;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.amrp-file-meta {
  margin-top: 2px;
  color: var(--amrp-muted);
  font-size: 10.5px;
}

.amrp-file-actions {
  display: flex;
  gap: 5px;
}

button {
  border: 0;
  border-radius: 6px;
  padding: 4px 7px;
  background: var(--amrp-control);
  color: var(--amrp-accent);
  font: inherit;
  font-size: 10px;
  cursor: pointer;
}

button:disabled {
  cursor: wait;
  opacity: 0.65;
}
</style>
