<template>
  <section class="amrp-code" :class="{ 'is-expanded': expanded, 'is-wrap': wrap }">
    <header class="amrp-block-head">
      <span v-if="filename" class="amrp-filename">{{ filename }}</span>
      <span class="amrp-language">{{ displayLanguage }}</span>
      <div class="amrp-tools">
        <button type="button" @click="wrap = !wrap">{{ wrap ? "不换行" : "换行" }}</button>
        <button type="button" @click="expanded = !expanded">{{ expanded ? "收起" : "展开" }}</button>
        <button type="button" @click="openFullscreen">全屏</button>
        <button type="button" @click="copyCode">复制</button>
      </div>
    </header>
    <div class="amrp-code-scroll" :style="scrollStyle">
      <div v-if="highlightedHtml && !streaming" class="amrp-highlighted" v-html="highlightedHtml"></div>
      <pre v-else><code>{{ code }}</code></pre>
      <div v-if="canCollapse && !expanded" class="amrp-code-fade"></div>
    </div>
    <button v-if="canCollapse" type="button" class="amrp-expand-bar" @click="expanded = !expanded">
      {{ expanded ? "收起代码" : "显示更多代码" }}
    </button>
  </section>

  <Teleport to="body">
    <div v-if="fullscreen" class="amrp-fullscreen" @click.self="fullscreen = false">
      <div class="amrp-fullscreen-panel">
        <header class="amrp-block-head">
          <span class="amrp-filename">{{ filename || displayLanguage }}</span>
          <span class="amrp-language">{{ displayLanguage }}</span>
          <div class="amrp-tools">
            <button type="button" @click="wrap = !wrap">{{ wrap ? "不换行" : "换行" }}</button>
            <button type="button" @click="copyCode">复制</button>
            <button type="button" @click="fullscreen = false">关闭</button>
          </div>
        </header>
        <div class="amrp-fullscreen-code">
          <div v-if="highlightedHtml && !streaming" class="amrp-highlighted" v-html="highlightedHtml"></div>
          <pre v-else><code>{{ code }}</code></pre>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import { useTheme } from "@/composables/useTheme";
import { highlightCode } from "./shikiService";
import { copyText, normalizedLanguage } from "../utils";

const props = withDefaults(
  defineProps<{
    code: string;
    language?: string;
    filename?: string;
    streaming?: boolean;
    maxHeight?: number;
  }>(),
  {
    language: "text",
    filename: "",
    streaming: false,
    maxHeight: 420,
  },
);

const { resolvedMode } = useTheme();
const wrap = ref(false);
const expanded = ref(false);
const fullscreen = ref(false);
const highlightedHtml = ref("");

const displayLanguage = computed(() => normalizedLanguage(props.language));
const lineCount = computed(() => props.code.split("\n").length);
const canCollapse = computed(() => !expanded.value && (lineCount.value > 24 || props.code.length > 1800));
const scrollStyle = computed(() => ({
  maxHeight: expanded.value ? "none" : `${props.maxHeight}px`,
}));

async function refreshHighlight() {
  if (props.streaming || lineCount.value > 4000 || props.code.length > 240_000) {
    highlightedHtml.value = "";
    return;
  }
  highlightedHtml.value = await highlightCode(
    props.code,
    displayLanguage.value,
    resolvedMode.value === "dark",
  );
}

async function copyCode() {
  const copied = await copyText(props.code);
  copied ? ElMessage.success("已复制代码") : ElMessage.warning("复制失败");
}

function openFullscreen() {
  fullscreen.value = true;
}

onMounted(refreshHighlight);
watch(
  () => [props.code, displayLanguage.value, resolvedMode.value, props.streaming],
  refreshHighlight,
);
</script>

<style scoped>
.amrp-code {
  position: relative;
  width: 100%;
  margin: 14px 0 17px;
  overflow: hidden;
  border-radius: 10px;
  background: var(--amrp-code-bg);
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.035);
}

.amrp-block-head {
  min-height: 36px;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 11px;
  background: var(--amrp-code-head);
  color: #aaa;
}

.amrp-filename {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: #d2d3d6;
  font-size: 11.5px;
}

.amrp-language {
  color: #81838a;
  font-size: 10px;
}

.amrp-tools {
  display: flex;
  align-items: center;
  gap: 5px;
  margin-left: auto;
}

.amrp-tools button {
  border: 0;
  border-radius: 6px;
  padding: 4px 7px;
  background: #303137;
  color: #c9cbd0;
  font: inherit;
  font-size: 10px;
  cursor: pointer;
}

.amrp-tools button:hover {
  background: #3a3b43;
}

.amrp-code-scroll {
  position: relative;
  overflow: auto;
}

.amrp-code :deep(pre),
.amrp-fullscreen-code :deep(pre) {
  margin: 0;
  padding: 14px 16px 16px;
  overflow: visible;
  color: #e8e8eb;
  font: 12.5px/1.7 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}

.amrp-code.is-wrap :deep(pre),
.amrp-fullscreen-code.is-wrap pre {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.amrp-highlighted :deep(.shiki),
.amrp-highlighted :deep(.amrp-shiki) {
  margin: 0;
  padding: 14px 16px 16px;
  overflow: visible;
  background: transparent !important;
  font: 12.5px/1.7 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}

.amrp-code.is-wrap .amrp-highlighted :deep(code) {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.amrp-code-fade {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 58px;
  pointer-events: none;
  background: linear-gradient(transparent, var(--amrp-code-bg));
}

.amrp-expand-bar {
  width: 100%;
  border: 0;
  padding: 7px;
  background: var(--amrp-code-head);
  color: #aaa;
  font: inherit;
  font-size: 10px;
  cursor: pointer;
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
  width: min(1120px, 96vw);
  height: min(760px, 90vh);
  overflow: hidden;
  border-radius: 12px;
  background: var(--amrp-code-bg);
  box-shadow: 0 24px 90px rgba(0, 0, 0, 0.45);
}

.amrp-fullscreen-code {
  height: calc(100% - 36px);
  overflow: auto;
}

@media (max-width: 700px) {
  .amrp-tools button:nth-child(2),
  .amrp-tools button:nth-child(3) {
    display: none;
  }

  .amrp-fullscreen {
    padding: 10px;
  }

  .amrp-fullscreen-panel {
    width: 100%;
    height: 100%;
  }
}
</style>

