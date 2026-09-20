<template>
  <section class="amrp-diff">
    <header class="amrp-block-head">
      <span class="amrp-filename">{{ filename || "Diff" }}</span>
      <span class="amrp-language">diff</span>
      <div class="amrp-tools">
        <button type="button" @click="expanded = !expanded">{{ expanded ? "收起" : "展开" }}</button>
        <button type="button" @click="fullscreen = true">全屏</button>
        <button type="button" @click="copyDiff">复制 Diff</button>
      </div>
    </header>
    <div class="amrp-diff-scroll" :class="{ expanded }">
      <div v-for="(line, index) in lines" :key="index" class="amrp-diff-line" :class="line.kind">
        <span class="amrp-line-number">{{ index + 1 }}</span>
        <span class="amrp-line-marker">{{ line.marker }}</span>
        <code>{{ line.text }}</code>
      </div>
      <div v-if="!expanded && lines.length > 18" class="amrp-diff-fade"></div>
    </div>
  </section>

  <Teleport to="body">
    <div v-if="fullscreen" class="amrp-fullscreen" @click.self="fullscreen = false">
      <div class="amrp-fullscreen-panel">
        <header class="amrp-block-head">
          <span class="amrp-filename">{{ filename || "Diff" }}</span>
          <div class="amrp-tools">
            <button type="button" @click="copyDiff">复制 Diff</button>
            <button type="button" @click="fullscreen = false">关闭</button>
          </div>
        </header>
        <div class="amrp-diff-scroll expanded">
          <div v-for="(line, index) in lines" :key="index" class="amrp-diff-line" :class="line.kind">
            <span class="amrp-line-number">{{ index + 1 }}</span>
            <span class="amrp-line-marker">{{ line.marker }}</span>
            <code>{{ line.text }}</code>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { ElMessage } from "element-plus";
import { copyText } from "../utils";

const props = withDefaults(
  defineProps<{
    diff: string;
    filename?: string;
    streaming?: boolean;
  }>(),
  {
    filename: "",
    streaming: false,
  },
);

const expanded = ref(false);
const fullscreen = ref(false);

const lines = computed(() =>
  String(props.diff ?? "")
    .split("\n")
    .map((line) => {
      if (line.startsWith("@@")) return { kind: "hunk" as const, marker: "@@", text: line.slice(2) };
      if (line.startsWith("+") && !line.startsWith("+++")) {
        return { kind: "add" as const, marker: "+", text: line.slice(1) };
      }
      if (line.startsWith("-") && !line.startsWith("---")) {
        return { kind: "del" as const, marker: "-", text: line.slice(1) };
      }
      if (line.startsWith("+++") || line.startsWith("---")) {
        return { kind: "meta" as const, marker: "", text: line };
      }
      return { kind: "context" as const, marker: " ", text: line };
    }),
);

async function copyDiff() {
  const copied = await copyText(props.diff);
  copied ? ElMessage.success("已复制 Diff") : ElMessage.warning("复制失败");
}
</script>

<style scoped>
.amrp-diff {
  width: 100%;
  margin: 14px 0 17px;
  overflow: hidden;
  border-radius: 10px;
  background: var(--amrp-code-bg);
}

.amrp-block-head {
  min-height: 36px;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 11px;
  background: var(--amrp-code-head);
}

.amrp-filename {
  min-width: 0;
  overflow: hidden;
  color: #d2d3d6;
  font-size: 11.5px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.amrp-language {
  color: #81838a;
  font-size: 10px;
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
  background: #303137;
  color: #c9cbd0;
  font: inherit;
  font-size: 10px;
  cursor: pointer;
}

.amrp-diff-scroll {
  position: relative;
  max-height: 420px;
  overflow: auto;
  padding: 8px 0;
}

.amrp-diff-scroll.expanded {
  max-height: none;
}

.amrp-diff-line {
  display: grid;
  grid-template-columns: 44px 18px minmax(max-content, 1fr);
  min-height: 24px;
  align-items: start;
  color: #c9cbd0;
  font: 12px/1.7 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}

.amrp-diff-line.add {
  background: color-mix(in srgb, #3e8d5d 22%, transparent);
  color: #9adcaa;
}

.amrp-diff-line.del {
  background: color-mix(in srgb, #c85353 20%, transparent);
  color: #f0a0a0;
}

.amrp-diff-line.hunk {
  background: rgba(112, 96, 232, 0.16);
  color: #bcb3ff;
}

.amrp-diff-line.meta {
  color: #8c8e95;
}

.amrp-line-number,
.amrp-line-marker {
  padding: 0 6px;
  user-select: none;
  color: #6e7077;
  text-align: right;
}

.amrp-diff-line code {
  padding: 0 12px 0 0;
  white-space: pre;
}

.amrp-diff-fade {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 70px;
  pointer-events: none;
  background: linear-gradient(transparent, var(--amrp-code-bg));
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
}

@media (max-width: 700px) {
  .amrp-tools button:first-child {
    display: none;
  }
}
</style>

