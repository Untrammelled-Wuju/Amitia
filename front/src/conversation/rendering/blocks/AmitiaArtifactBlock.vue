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
        <button type="button" @click="fullscreen = true">全屏</button>
        <button type="button" @click="copyContent">复制</button>
      </div>
    </div>
    <div v-if="preview && block.content" class="amrp-artifact-preview">
      <AmitiaHtmlPreview
        v-if="isHtml"
        :source="block.content"
        :filename="block.title"
      />
      <pre v-else><code>{{ block.content }}</code></pre>
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
        <AmitiaHtmlPreview v-if="isHtml && block.content" :source="block.content" :filename="block.title" />
        <pre v-else><code>{{ block.content || prettyJson(block) }}</code></pre>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { ElMessage } from "element-plus";
import type { ArtifactBlock } from "../types";
import { copyText, formatBytes, prettyJson } from "../utils";
import AmitiaHtmlPreview from "../preview/AmitiaHtmlPreview.vue";

const props = defineProps<{
  block: ArtifactBlock;
}>();

const preview = ref(false);
const fullscreen = ref(false);
const isHtml = computed(() => {
  const kind = props.block.artifactKind.toLowerCase();
  const mime = String(props.block.mimeType ?? "").toLowerCase();
  return kind.includes("html") || mime.includes("html");
});

async function copyContent() {
  const copied = await copyText(props.block.content ?? prettyJson(props.block));
  copied ? ElMessage.success("已复制 Artifact") : ElMessage.warning("复制失败");
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

