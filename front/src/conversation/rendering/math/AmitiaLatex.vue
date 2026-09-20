<template>
  <section class="amrp-latex" :class="{ block: display, streaming }">
    <div v-if="streaming" class="amrp-latex-source">{{ source }}</div>
    <div v-else-if="rendered" class="amrp-latex-rendered" v-html="rendered"></div>
    <div v-else class="amrp-latex-error">
      <span>{{ source }}</span>
      <button type="button" @click="copySource">复制公式</button>
    </div>
    <button v-if="display && !streaming" type="button" class="amrp-latex-copy" @click="copySource">复制</button>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import katex from "katex";
import { ElMessage } from "element-plus";
import { copyText } from "../utils";

const props = withDefaults(
  defineProps<{
    source: string;
    display?: boolean;
    streaming?: boolean;
  }>(),
  {
    display: false,
    streaming: false,
  },
);

const rendered = ref("");
const renderError = ref("");

const formula = computed(() => String(props.source ?? "").trim());
function renderFormula() {
  if (props.streaming || !formula.value) {
    rendered.value = "";
    renderError.value = "";
    return;
  }
  try {
    const html = katex.renderToString(formula.value, {
      displayMode: props.display,
      throwOnError: true,
      strict: "ignore",
      trust: false,
      output: "htmlAndMathml",
    });
    rendered.value = html;
    renderError.value = "";
  } catch (error) {
    rendered.value = "";
    renderError.value = error instanceof Error ? error.message : String(error);
  }
}

watch(
  () => [formula.value, props.display, props.streaming],
  renderFormula,
  { immediate: true },
);

async function copySource() {
  const copied = await copyText(formula.value);
  copied ? ElMessage.success("已复制公式源码") : ElMessage.warning("复制失败");
}
</script>

<style scoped>
.amrp-latex {
  position: relative;
  max-width: 100%;
  margin: 3px 0;
}

.amrp-latex.block {
  width: 100%;
  margin: 12px 0 16px;
  overflow: auto hidden;
  border: 1px solid var(--amrp-line);
  border-radius: 10px;
  padding: 14px 42px 14px 16px;
  background: var(--amrp-surface);
  text-align: center;
  white-space: nowrap;
}

.amrp-latex-rendered {
  display: inline-block;
}

.amrp-latex-source,
.amrp-latex-error {
  display: inline-flex;
  gap: 8px;
  align-items: center;
  padding: 2px 6px;
  border-radius: 5px;
  background: var(--amrp-soft);
  color: var(--amrp-muted);
  font: 12.5px/1.55 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}

.amrp-latex-error button,
.amrp-latex-copy {
  border: 0;
  border-radius: 6px;
  padding: 4px 7px;
  background: var(--amrp-soft);
  color: var(--amrp-accent);
  font: inherit;
  font-size: 10px;
  cursor: pointer;
}

.amrp-latex-copy {
  position: absolute;
  right: 9px;
  top: 9px;
}
</style>
