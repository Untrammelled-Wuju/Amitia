<template>
  <section class="amrp-preview">
    <header class="amrp-block-head">
      <span class="amrp-title">{{ filename || "index.html" }}</span>
      <span class="amrp-meta">sandbox preview</span>
      <div class="amrp-tools">
        <button type="button" @click="showSource = !showSource">{{ showSource ? "预览" : "源码" }}</button>
        <button type="button" @click="copySource">复制</button>
        <button type="button" @click="fullscreen = true">全屏</button>
      </div>
    </header>
    <pre v-if="showSource || streaming" class="amrp-source"><code>{{ source }}</code></pre>
    <button v-else-if="previewError" type="button" @click="retryVersion++">{{ previewError }}，点击重试</button>
    <div v-else-if="!sandboxDocument">加载中</div>
    <div v-else class="amrp-browser">
      <div class="amrp-browser-bar">
        <i></i><i></i><i></i>
        <span>sandbox://preview/index.html</span>
      </div>
      <iframe
        title="Sandboxed HTML Preview"
        :src="previewUrl"
        sandbox="allow-scripts"
        referrerpolicy="no-referrer"
      ></iframe>
    </div>
  </section>

  <Teleport to="body">
    <div v-if="fullscreen" class="amrp-fullscreen" @click.self="fullscreen = false">
      <div class="amrp-fullscreen-panel">
        <header class="amrp-block-head">
          <span class="amrp-title">Sandboxed HTML Preview</span>
          <div class="amrp-tools">
            <button type="button" @click="copySource">复制</button>
            <button type="button" @click="fullscreen = false">关闭</button>
          </div>
        </header>
        <iframe
          class="amrp-fullscreen-frame"
          title="Sandboxed HTML Preview Fullscreen"
          :src="previewUrl"
          sandbox="allow-scripts"
          referrerpolicy="no-referrer"
        ></iframe>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, watch, onBeforeUnmount } from "vue";
import { buildHtmlPreviewDocument } from "./htmlDocument";
import { apiClient } from "@/composables/useApi";
import { getApiBaseURLForPath } from "@/runtime/runtime-adapter";
import { ElMessage } from "element-plus";
import { copyText } from "../utils";

const props = withDefaults(
  defineProps<{
    source: string;
    filename?: string;
    streaming?: boolean;
    baseUrl?: string;
  }>(),
  {
    filename: "",
    streaming: false,
  },
);

const showSource = ref(false);
const fullscreen = ref(false);

const sandboxDocument = ref("");
const previewError = ref("");
const previewUrl = ref("");
const retryVersion = ref(0);
let previewId = "";
function releasePreview(id: string) {
  if (id) void apiClient.delete(`/api/artifacts/v1/previews/${encodeURIComponent(id)}`).catch(() => {});
}
let generation = 0;
onBeforeUnmount(() => { generation++; releasePreview(previewId); });
watch(() => [props.source, props.baseUrl, props.streaming, retryVersion.value], async () => {
  const current = ++generation;
  if (props.streaming) return;
  sandboxDocument.value = "";
  previewError.value = "";
  try {
    const document = await buildHtmlPreviewDocument(props.source, props.baseUrl);
    if (current !== generation) return;
    const response = await apiClient.post<{ url: string; previewId: string }>("/api/artifacts/v1/previews", { html: document });
    if (current !== generation) { releasePreview(response.data.previewId); return; }
    const base = await getApiBaseURLForPath("/api/artifacts/v1/previews");
    const url = response.data?.url;
    if (!url) throw new Error("后端未返回 HTML 预览地址");
    if (current === generation) {
      releasePreview(previewId);
      previewId = response.data.previewId;
      previewUrl.value = `${base.replace(/\/+$/, "")}/${url.replace(/^\/+/, "")}`;
      sandboxDocument.value = document;
    }
  } catch (reason) {
    if (current === generation) previewError.value = reason instanceof Error ? reason.message : "HTML 加载失败";
  }
}, { immediate: true });

async function copySource() {
  const copied = await copyText(props.source);
  copied ? ElMessage.success("已复制 HTML 源码") : ElMessage.warning("复制失败");
}
</script>

<style scoped>
.amrp-preview {
  width: 100%;
  margin: 12px 0 16px;
  overflow: hidden;
  border: 1px solid var(--amrp-line);
  border-radius: 10px;
  background: var(--amrp-surface);
}

.amrp-block-head {
  min-height: 36px;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 10px;
  border-bottom: 1px solid var(--amrp-line);
  background: var(--amrp-soft);
}

.amrp-title {
  min-width: 0;
  overflow: hidden;
  font-size: 11.5px;
  font-weight: 680;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.amrp-meta {
  color: var(--amrp-muted);
  font-size: 10.5px;
}

.amrp-tools {
  display: flex;
  gap: 5px;
  margin-left: auto;
}

.amrp-tools button {
  border: 0;
  border-radius: 6px;
  padding: 4px 7px;
  background: var(--amrp-control);
  color: var(--amrp-text);
  font: inherit;
  font-size: 10px;
  cursor: pointer;
}

.amrp-source {
  margin: 0;
  overflow: auto;
  padding: 14px 16px;
  background: var(--amrp-code-bg);
  color: #d5d7dd;
  font: 12px/1.7 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}

.amrp-browser-bar {
  height: 32px;
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 0 9px;
  background: var(--amrp-soft);
}

.amrp-browser-bar i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #c6c7cc;
}

.amrp-browser-bar span {
  flex: 1;
  margin-left: 6px;
  border-radius: 5px;
  padding: 3px 7px;
  background: var(--amrp-surface);
  color: var(--amrp-muted);
  font-size: 9px;
}

iframe {
  display: block;
  width: 100%;
  height: 190px;
  border: 0;
  background: white;
}

.amrp-fullscreen {
  position: fixed;
  z-index: 4000;
  inset: 0;
  display: grid;
  place-items: center;
  padding: 24px;
  background: rgba(7, 8, 10, 0.72);
}

.amrp-fullscreen-panel {
  width: min(1180px, 96vw);
  height: min(820px, 92vh);
  overflow: hidden;
  border-radius: 12px;
  background: var(--amrp-surface);
}

.amrp-fullscreen-frame {
  height: calc(100% - 36px);
}
</style>

