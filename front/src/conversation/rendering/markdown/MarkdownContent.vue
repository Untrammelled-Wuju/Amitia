<template>
  <div ref="rootEl" class="amrp-markdown" :class="{ 'is-streaming': streaming }" @click="handleClick">
    <template v-for="segment in segments" :key="segment.id">
      <div
        v-if="segment.type === 'markdown'"
        class="amrp-markdown-segment"
        v-html="renderMarkdownSegment(segment.content, citationIds)"
      ></div>
      <AmitiaCodeBlock
        v-else-if="segment.type === 'code'"
        :code="segment.content"
        :language="segment.language"
        :filename="segment.filename"
        :streaming="segment.streaming"
      />
      <AmitiaDiffBlock
        v-else-if="segment.type === 'diff'"
        :diff="segment.content"
        :filename="segment.filename"
        :streaming="segment.streaming"
      />
      <AmitiaTerminalBlock
        v-else-if="segment.type === 'terminal'"
        :command="segment.content"
        :language="segment.language"
        :filename="segment.filename"
        :streaming="segment.streaming"
      />
      <AmitiaLatex
        v-else-if="segment.type === 'latex'"
        :source="segment.content"
        :streaming="segment.streaming"
        display
      />
      <AmitiaMermaid
        v-else-if="segment.type === 'mermaid'"
        :source="segment.content"
        :streaming="segment.streaming"
      />
      <AmitiaHtmlPreview
        v-else-if="segment.type === 'html-preview'"
        :source="segment.content"
        :filename="segment.filename"
        :streaming="segment.streaming"
      />
    </template>
    <span v-if="streaming" class="amrp-stream-caret" aria-hidden="true"></span>
  </div>

  <Teleport to="body">
    <div v-if="imagePreview" class="amrp-image-modal" @click.self="imagePreview = ''">
      <button type="button" @click="imagePreview = ''">关闭</button>
      <img :src="imagePreview" alt="" />
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, ref, toRef } from "vue";
import { renderMarkdownSegment, splitMarkdownSegments } from "./markdownEngine";
import { useStreamRenderScheduler } from "../stream/streamRenderScheduler";
import { isSafeLink } from "../utils";
import AmitiaCodeBlock from "../code/AmitiaCodeBlock.vue";
import AmitiaDiffBlock from "../code/AmitiaDiffBlock.vue";
import AmitiaTerminalBlock from "../code/AmitiaTerminalBlock.vue";
import AmitiaLatex from "../math/AmitiaLatex.vue";
import AmitiaMermaid from "../mermaid/AmitiaMermaid.vue";
import AmitiaHtmlPreview from "../preview/AmitiaHtmlPreview.vue";

const props = withDefaults(
  defineProps<{
    source: string;
    streaming?: boolean;
    citationIds?: string[];
  }>(),
  {
    streaming: false,
    citationIds: () => [],
  },
);

const emit = defineEmits<{
  citation: [id: string];
}>();

const rootEl = ref<HTMLElement>();
const imagePreview = ref("");
const sourceRef = toRef(props, "source");
const streamingRef = computed(() => props.streaming);
const renderedSource = useStreamRenderScheduler(sourceRef, streamingRef);
const segments = computed(() => splitMarkdownSegments(renderedSource.value));

function handleClick(event: MouseEvent) {
  const target = event.target as HTMLElement | null;
  const citation = target?.closest<HTMLElement>("[data-citation-id]");
  if (citation?.dataset.citationId) {
    emit("citation", citation.dataset.citationId);
    return;
  }
  const image = target?.closest<HTMLImageElement>("img[data-amitia-image]");
  if (image?.src) {
    imagePreview.value = image.src;
    return;
  }
  const anchor = target?.closest<HTMLAnchorElement>("a[href]");
  if (!anchor) return;
  const href = anchor.getAttribute("href") ?? "";
  if (!isSafeLink(href)) {
    event.preventDefault();
    return;
  }
  anchor.target = "_blank";
  anchor.rel = "noopener noreferrer";
}
</script>

<style scoped>
.amrp-markdown {
  color: var(--amrp-text);
  font-size: 14px;
  line-height: 1.72;
  word-break: break-word;
  overflow-wrap: anywhere;
}

.amrp-markdown-segment {
  min-width: 0;
}

.amrp-markdown-segment :deep(p) {
  max-width: 700px;
  margin: 0 0 11px;
}

.amrp-markdown-segment :deep(h1),
.amrp-markdown-segment :deep(h2),
.amrp-markdown-segment :deep(h3),
.amrp-markdown-segment :deep(h4),
.amrp-markdown-segment :deep(h5),
.amrp-markdown-segment :deep(h6) {
  max-width: 700px;
  margin: 22px 0 8px;
  line-height: 1.4;
  font-weight: 720;
}

.amrp-markdown-segment :deep(h1) { font-size: 22px; }
.amrp-markdown-segment :deep(h2) { font-size: 19px; }
.amrp-markdown-segment :deep(h3) { font-size: 16px; }
.amrp-markdown-segment :deep(h4) { font-size: 14.5px; }
.amrp-markdown-segment :deep(h5),
.amrp-markdown-segment :deep(h6) { font-size: 13.5px; }

.amrp-markdown-segment :deep(ul),
.amrp-markdown-segment :deep(ol) {
  max-width: 700px;
  margin: 6px 0 13px;
  padding-left: 20px;
}

.amrp-markdown-segment :deep(li) {
  margin: 3px 0;
}

.amrp-markdown-segment :deep(li.task-list-item) {
  display: flex;
  align-items: flex-start;
  gap: 7px;
  list-style: none;
}

.amrp-markdown-segment :deep(input[type="checkbox"]) {
  width: 14px;
  height: 14px;
  margin: 4px 0 0;
  accent-color: var(--amrp-accent);
}

.amrp-markdown-segment :deep(strong) {
  font-weight: 720;
}

.amrp-markdown-segment :deep(a) {
  color: var(--amrp-accent);
  text-decoration: none;
}

.amrp-markdown-segment :deep(code) {
  padding: 2px 5px;
  border-radius: 5px;
  background: var(--amrp-inline-code);
  font: 12.5px ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}

.amrp-markdown-segment :deep(blockquote) {
  max-width: 700px;
  margin: 12px 0;
  border-left: 2px solid color-mix(in srgb, var(--amrp-accent) 45%, var(--amrp-line));
  padding: 5px 0 5px 12px;
  color: var(--amrp-muted);
}

.amrp-markdown-segment :deep(hr) {
  max-width: 700px;
  height: 1px;
  margin: 16px 0;
  border: 0;
  background: var(--amrp-line);
}

.amrp-markdown-segment :deep(.amrp-table-scroll) {
  width: 100%;
  margin: 14px 0 17px;
  overflow: auto;
  border: 1px solid var(--amrp-line);
  border-radius: 9px;
  background: var(--amrp-surface);
}

.amrp-markdown-segment :deep(table) {
  width: 100%;
  min-width: 600px;
  border-collapse: collapse;
}

.amrp-markdown-segment :deep(th),
.amrp-markdown-segment :deep(td) {
  padding: 9px 11px;
  border-bottom: 1px solid var(--amrp-line);
  text-align: left;
}

.amrp-markdown-segment :deep(th) {
  background: var(--amrp-soft);
  color: var(--amrp-muted);
  font-size: 11px;
}

.amrp-markdown-segment :deep(td) {
  font-size: 12.5px;
}

.amrp-markdown-segment :deep(img[data-amitia-image]) {
  display: block;
  max-width: min(560px, 100%);
  max-height: 420px;
  margin: 11px 0;
  border-radius: 10px;
  object-fit: contain;
  cursor: zoom-in;
  background: var(--amrp-soft);
}

.amrp-markdown-segment :deep(.amrp-citation-ref) {
  border: 0;
  padding: 0 2px;
  background: transparent;
  color: var(--amrp-accent);
  font: inherit;
  cursor: pointer;
}

.amrp-markdown-segment :deep(.amrp-katex .katex-display) {
  max-width: 100%;
  margin: 12px 0 16px;
  overflow: auto hidden;
  padding: 14px 16px;
  border: 1px solid var(--amrp-line);
  border-radius: 10px;
  background: var(--amrp-surface);
}

.amrp-stream-caret {
  display: inline-block;
  width: 7px;
  height: 16px;
  margin-left: 3px;
  border-radius: 2px;
  background: var(--amrp-muted);
  vertical-align: -3px;
  animation: amrp-blink 0.9s infinite;
}

@keyframes amrp-blink {
  50% { opacity: 0.15; }
}

.amrp-image-modal {
  position: fixed;
  z-index: 4000;
  inset: 0;
  display: grid;
  place-items: center;
  padding: 34px;
  background: rgba(7, 8, 10, 0.82);
}

.amrp-image-modal img {
  max-width: 100%;
  max-height: 92vh;
  border-radius: 10px;
}

.amrp-image-modal button {
  position: fixed;
  top: 16px;
  right: 16px;
  border: 0;
  border-radius: 8px;
  padding: 7px 10px;
  background: rgba(255, 255, 255, 0.12);
  color: white;
  cursor: pointer;
}

@media (max-width: 700px) {
  .amrp-markdown {
    font-size: 14px;
  }

  .amrp-markdown-segment :deep(.amrp-table-scroll),
  .amrp-markdown-segment :deep(img[data-amitia-image]) {
    max-width: 100%;
  }
}
</style>
