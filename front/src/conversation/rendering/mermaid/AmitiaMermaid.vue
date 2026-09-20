<template>
  <section class="amrp-mermaid">
    <header class="amrp-block-head">
      <span class="amrp-title">flowchart</span>
      <span class="amrp-meta">Mermaid</span>
      <div class="amrp-tools">
        <button type="button" @click="showSource = !showSource">{{ showSource ? "预览" : "源码" }}</button>
        <button type="button" @click="copySource">复制源码</button>
        <button type="button" @click="fullscreen = true">全屏</button>
      </div>
    </header>
    <div v-if="showSource || streaming" class="amrp-mermaid-source">
      <pre><code>{{ source }}</code></pre>
      <span v-if="streaming" class="amrp-stream-label">等待代码块闭合后渲染</span>
    </div>
    <div v-else-if="error" class="amrp-render-error">
      <b>Mermaid Renderer 发生异常</b>
      <p>{{ error }}</p>
      <button type="button" @click="renderDiagram">重试</button>
      <button type="button" @click="showSource = true">查看源码</button>
    </div>
    <div v-else-if="svg" class="amrp-mermaid-preview">
      <div v-html="svg"></div>
    </div>
    <div v-else class="amrp-mermaid-loading">
      <span class="amrp-spinner"></span>
      <span>正在渲染图表</span>
    </div>
  </section>

  <Teleport to="body">
    <div v-if="fullscreen" class="amrp-fullscreen" @click.self="fullscreen = false">
      <div class="amrp-fullscreen-panel">
        <header class="amrp-block-head">
          <span class="amrp-title">Mermaid</span>
          <div class="amrp-tools">
            <button type="button" @click="copySource">复制源码</button>
            <button type="button" @click="fullscreen = false">关闭</button>
          </div>
        </header>
        <div class="amrp-fullscreen-diagram">
          <div v-if="svg" v-html="svg"></div>
          <pre v-else><code>{{ source }}</code></pre>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import DOMPurify from "dompurify";
import { ElMessage } from "element-plus";
import { useTheme } from "@/composables/useTheme";
import { copyText } from "../utils";

const props = withDefaults(
  defineProps<{
    source: string;
    streaming?: boolean;
  }>(),
  {
    streaming: false,
  },
);

const { resolvedMode } = useTheme();
const svg = ref("");
const error = ref("");
const showSource = ref(false);
const fullscreen = ref(false);
const cache = new Map<string, string>();
let renderSequence = 0;
let renderTimer: ReturnType<typeof setTimeout> | null = null;

async function renderDiagram() {
  if (props.streaming || !String(props.source ?? "").trim()) {
    svg.value = "";
    error.value = "";
    return;
  }
  const dark = resolvedMode.value === "dark";
  const key = `${dark ? "dark" : "light"}:${props.source}`;
  const cached = cache.get(key);
  if (cached) {
    svg.value = cached;
    error.value = "";
    return;
  }
  const sequence = ++renderSequence;
  try {
    const mermaid = (await import("mermaid")).default;
    mermaid.initialize({
      startOnLoad: false,
      securityLevel: "strict",
      theme: dark ? "dark" : "default",
      fontFamily: "Inter, ui-sans-serif, system-ui, sans-serif",
      suppressErrorRendering: true,
    });
    const id = `amrp-mermaid-${Date.now()}-${sequence}`;
    const result = await mermaid.render(id, props.source);
    const rendered = typeof result === "string" ? result : result.svg;
    const sanitized = DOMPurify.sanitize(rendered, {
      USE_PROFILES: { svg: true, svgFilters: true },
    });
    if (sequence !== renderSequence) return;
    cache.set(key, sanitized);
    svg.value = sanitized;
    error.value = "";
  } catch (reason) {
    if (sequence !== renderSequence) return;
    svg.value = "";
    error.value = reason instanceof Error ? reason.message : String(reason);
  }
}

async function copySource() {
  const copied = await copyText(props.source);
  copied ? ElMessage.success("已复制 Mermaid 源码") : ElMessage.warning("复制失败");
}

onMounted(renderDiagram);
watch(
  () => [props.source, props.streaming, resolvedMode.value],
  () => {
    if (renderTimer) clearTimeout(renderTimer);
    renderTimer = setTimeout(renderDiagram, 180);
  },
);

onBeforeUnmount(() => {
  if (renderTimer) clearTimeout(renderTimer);
});
</script>

<style scoped>
.amrp-mermaid {
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
  font-size: 11.5px;
  font-weight: 680;
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

.amrp-tools button,
.amrp-render-error button {
  border: 0;
  border-radius: 6px;
  padding: 4px 7px;
  background: var(--amrp-control);
  color: var(--amrp-text);
  font: inherit;
  font-size: 10px;
  cursor: pointer;
}

.amrp-mermaid-preview {
  min-height: 210px;
  display: grid;
  place-items: center;
  padding: 18px;
  overflow: auto;
  background: linear-gradient(var(--amrp-surface), var(--amrp-soft));
}

.amrp-mermaid-preview :deep(svg) {
  max-width: 100%;
  height: auto;
}

.amrp-mermaid-source {
  position: relative;
}

.amrp-mermaid-source pre {
  margin: 0;
  overflow: auto;
  padding: 14px 16px;
  background: var(--amrp-code-bg);
  color: #d5d7dd;
  font: 12px/1.7 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}

.amrp-stream-label {
  position: absolute;
  right: 10px;
  bottom: 8px;
  padding: 3px 6px;
  border-radius: 5px;
  background: var(--amrp-soft);
  color: var(--amrp-muted);
  font-size: 10px;
}

.amrp-render-error {
  padding: 12px;
  color: var(--amrp-danger);
}

.amrp-render-error p {
  margin: 5px 0 9px;
  color: var(--amrp-muted);
  font-size: 11px;
}

.amrp-render-error button + button {
  margin-left: 5px;
}

.amrp-mermaid-loading {
  min-height: 160px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: var(--amrp-muted);
  font-size: 11px;
}

.amrp-spinner {
  width: 17px;
  height: 17px;
  border: 2px solid var(--amrp-line);
  border-top-color: var(--amrp-accent);
  border-radius: 50%;
  animation: amrp-spin 1s linear infinite;
}

@keyframes amrp-spin {
  to { transform: rotate(360deg); }
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
  width: min(1080px, 96vw);
  height: min(760px, 90vh);
  overflow: hidden;
  border-radius: 12px;
  background: var(--amrp-surface);
}

.amrp-fullscreen-diagram {
  height: calc(100% - 36px);
  overflow: auto;
  padding: 20px;
  background: var(--amrp-surface);
}

.amrp-fullscreen-diagram :deep(svg) {
  display: block;
  max-width: none;
  margin: auto;
}
</style>
