<template>
  <section class="amrp-artifact">
    <div class="amrp-artifact-body">
      <div class="amrp-artifact-icon">A</div>
      <div>
        <div class="amrp-artifact-title">{{ block.title }}</div>
        <div class="amrp-artifact-meta">
          {{ block.artifactKind }}<template v-if="formatBytes(block.size)"> · {{ formatBytes(block.size) }}</template>
        </div>
      </div>
      <div class="amrp-artifact-actions">
        <button type="button" @click="preview = !preview">{{ preview ? "收起" : "预览" }}</button>
        <button v-if="block.url" type="button" :disabled="downloading" @click="download">
          {{ downloading ? "下载中" : "下载" }}
        </button>
        <button type="button" @click="fullscreen = true">全屏</button>
        <button type="button" @click="copyContent">复制</button>
      </div>
    </div>
    <div v-if="preview" class="amrp-artifact-preview">
      <p v-if="loading">加载中</p>
      <button v-else-if="error" type="button" @click="loadContent">{{ error }}，点击重试</button>
      <AmitiaHtmlPreview
        v-else-if="isHtml"
        :source="content"
        :base-url="baseUrl"
        :filename="block.title"
      />
      <pre v-else><code>{{ content }}</code></pre>
    </div>
  </section>

  <Teleport to="body">
    <div v-if="fullscreen" class="amrp-artifact-fullscreen" @click.self="fullscreen = false">
      <div class="amrp-artifact-fullscreen-panel">
        <header>
          <b>{{ block.title }}</b>
          <button type="button" @click="copyContent">复制</button>
          <button type="button" @click="fullscreen = false">关闭</button>
        </header>
        <p v-if="loading">加载中</p>
        <button v-else-if="error" type="button" @click="loadContent">{{ error }}，点击重试</button>
        <AmitiaHtmlPreview v-else-if="isHtml" :source="content" :base-url="baseUrl" :filename="block.title" />
        <pre v-else><code>{{ content || prettyJson(block) }}</code></pre>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import type { ArtifactBlock } from "../types";
import { copyText, formatBytes, prettyJson } from "../utils";
import AmitiaHtmlPreview from "../preview/AmitiaHtmlPreview.vue";
import { downloadConversationMedia, loadConversationText } from "../media";

const props = defineProps<{
  block: ArtifactBlock;
}>();

const preview = ref(false);
const fullscreen = ref(false);
const downloading = ref(false);
const content = ref("");
const baseUrl = ref("");
const loading = ref(false);
const error = ref("");
let generation = 0;
async function loadContent() {
  const current = ++generation;
  content.value = props.block.content || "";
  baseUrl.value = "";
  error.value = "";
  loading.value = false;
  if (content.value || !props.block.url || (!preview.value && !fullscreen.value)) return;
  loading.value = true;
  try {
    const result = await loadConversationText(props.block.url);
    if (current !== generation) return;
    content.value = result.source;
    baseUrl.value = result.url;
  } catch (reason) {
    if (current === generation) error.value = reason instanceof Error ? reason.message : "文件加载失败";
  } finally {
    if (current === generation) loading.value = false;
  }
}
watch(() => [props.block.url, props.block.content, preview.value, fullscreen.value], loadContent, { immediate: true });
const isHtml = computed(() => {
  const kind = props.block.artifactKind.toLowerCase();
  const mime = String(props.block.mimeType ?? "").toLowerCase();
  return kind.includes("html") || mime.includes("html") || /\.html?$/i.test(props.block.title);
});

async function copyContent() {
  if (!content.value && props.block.url) { preview.value = true; await loadContent(); }
  const copied = await copyText(content.value || prettyJson(props.block));
  copied ? ElMessage.success("已复制 Artifact") : ElMessage.warning("复制失败");
}

async function download() {
  if (!props.block.url || downloading.value) return;
  downloading.value = true;
  try {
    const saved = await downloadConversationMedia(
      props.block.downloadUrl || props.block.url || "",
      props.block.title,
      props.block.mimeType,
    );
    if (saved) ElMessage.success("Artifact 已保存");
  } catch (reason) {
    ElMessage.error(reason instanceof Error ? reason.message : "Artifact 保存失败");
  } finally {
    downloading.value = false;
  }
}
</script>

<style scoped>
.amrp-artifact {
  width: 100%;
  max-width: 700px;
  margin: 12px 0 16px;
  overflow: hidden;
  border: 1px solid var(--amrp-line);
  border-radius: 10px;
  background: var(--amrp-surface);
}

.amrp-artifact-body {
  display: flex;
  align-items: center;
  gap: 11px;
  padding: 12px;
}

.amrp-artifact-icon {
  width: 38px;
  height: 38px;
  display: grid;
  flex: 0 0 auto;
  place-items: center;
  border-radius: 10px;
  background: var(--amrp-accent-soft);
  color: var(--amrp-accent);
  font-weight: 800;
}

.amrp-artifact-title {
  font-size: 12.5px;
  font-weight: 680;
}

.amrp-artifact-meta {
  margin-top: 2px;
  color: var(--amrp-muted);
  font-size: 10.5px;
}

.amrp-artifact-actions {
  display: flex;
  gap: 5px;
  margin-left: auto;
}

.amrp-artifact-actions button,
.amrp-artifact-fullscreen button {
  border: 0;
  border-radius: 6px;
  padding: 4px 7px;
  background: var(--amrp-control);
  color: var(--amrp-text);
  font: inherit;
  font-size: 10px;
  cursor: pointer;
}

.amrp-artifact-actions button:disabled {
  cursor: wait;
  opacity: 0.65;
}

.amrp-artifact-preview {
  border-top: 1px solid var(--amrp-line);
  padding: 10px;
}

.amrp-artifact-preview pre,
.amrp-artifact-fullscreen pre {
  max-height: 420px;
  margin: 0;
  overflow: auto;
  padding: 12px;
  border-radius: 7px;
  background: var(--amrp-code-bg);
  color: #d5d7dd;
  font: 12px/1.7 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}

.amrp-artifact-fullscreen {
  position: fixed;
  z-index: 4000;
  inset: 0;
  display: grid;
  place-items: center;
  padding: 24px;
  background: rgba(7, 8, 10, 0.72);
}

.amrp-artifact-fullscreen-panel {
  width: min(1080px, 96vw);
  height: min(760px, 90vh);
  overflow: auto;
  border-radius: 12px;
  background: var(--amrp-surface);
}

.amrp-artifact-fullscreen-panel > header {
  min-height: 40px;
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 0 12px;
  border-bottom: 1px solid var(--amrp-line);
}

.amrp-artifact-fullscreen-panel > header b {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>

