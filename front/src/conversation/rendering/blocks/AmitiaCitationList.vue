<template>
  <section v-if="sources.length" class="amrp-sources">
    <div class="amrp-sources-title">引用</div>
    <button
      v-for="source in sources"
      :key="source.id"
      type="button"
      class="amrp-source-chip"
      @click="active = source"
    >
      {{ source.id }} · {{ source.title }}
    </button>
    <div v-if="active" class="amrp-citation-card">
      <b>[{{ active.id }}] {{ active.title }}</b>
      <p v-if="active.snippet">{{ active.snippet }}</p>
      <p v-else-if="active.url">{{ active.url }}</p>
      <div>
        <button v-if="active.url" type="button" @click="openSource(active)">打开来源</button>
        <button type="button" @click="copyCitation(active)">复制引用</button>
        <button type="button" @click="active = null">关闭</button>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref, watch } from "vue";
import { ElMessage } from "element-plus";
import type { CitationSource } from "../types";
import { copyText, isSafeLink, openSafeLink } from "../utils";

const props = defineProps<{
  sources: CitationSource[];
  highlightId?: string;
}>();

const emit = defineEmits<{
  "highlight-consumed": [];
}>();

const active = ref<CitationSource | null>(null);

watch(
  () => props.highlightId,
  (id) => {
    if (!id) return;
    active.value = props.sources.find((source) => source.id === id) ?? null;
    emit("highlight-consumed");
  },
  { immediate: true },
);

function openSource(source: CitationSource) {
  if (isSafeLink(source.url)) openSafeLink(source.url);
}

async function copyCitation(source: CitationSource) {
  const copied = await copyText(`[${source.id}] ${source.title}${source.url ? `\n${source.url}` : ""}`);
  copied ? ElMessage.success("已复制引用") : ElMessage.warning("复制失败");
}
</script>

<style scoped>
.amrp-sources {
  max-width: 700px;
  margin: 17px 0 0;
  border-top: 1px solid var(--amrp-line);
  padding-top: 12px;
  color: var(--amrp-muted);
  font-size: 11.5px;
}

.amrp-sources-title {
  margin-bottom: 5px;
}

.amrp-source-chip {
  max-width: 100%;
  margin: 5px 5px 0 0;
  overflow: hidden;
  border: 0;
  border-radius: 6px;
  padding: 4px 7px;
  background: var(--amrp-soft);
  color: var(--amrp-text);
  font: inherit;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: pointer;
}

.amrp-citation-card {
  max-width: 700px;
  margin-top: 9px;
  border: 1px solid var(--amrp-line);
  border-radius: 9px;
  padding: 10px;
  background: var(--amrp-surface);
}

.amrp-citation-card b {
  font-size: 11.5px;
}

.amrp-citation-card p {
  margin: 4px 0 0;
  color: var(--amrp-muted);
  font-size: 10.5px;
}

.amrp-citation-card div {
  display: flex;
  gap: 6px;
  margin-top: 7px;
}

.amrp-citation-card button {
  border: 0;
  border-radius: 6px;
  padding: 4px 7px;
  background: var(--amrp-soft);
  color: var(--amrp-text);
  font: inherit;
  font-size: 10px;
  cursor: pointer;
}
</style>
