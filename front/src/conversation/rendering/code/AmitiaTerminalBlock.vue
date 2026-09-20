<template>
  <section class="amrp-terminal">
    <header class="amrp-block-head">
      <span class="amrp-filename">{{ filename || "Terminal" }}</span>
      <span class="amrp-language">{{ language }}<template v-if="exitCode !== null"> · exit {{ exitCode }}</template></span>
      <div class="amrp-tools">
        <button type="button" @click="expanded = !expanded">{{ expanded ? "收起" : "展开" }}</button>
        <button type="button" @click="copyTerminal">复制</button>
      </div>
    </header>
    <pre :class="{ expanded }"><code><span v-for="(line, index) in lines" :key="index" :class="line.kind">{{ line.text }}
</span></code></pre>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { ElMessage } from "element-plus";
import { copyText } from "../utils";

const props = withDefaults(
  defineProps<{
    command: string;
    language?: string;
    filename?: string;
    streaming?: boolean;
  }>(),
  {
    language: "shell",
    filename: "",
    streaming: false,
  },
);

const expanded = ref(false);
const lines = computed(() =>
  String(props.command ?? "")
    .split("\n")
    .map((line) => {
      const trimmed = line.trim();
      if (/^(?:\$|>|PS>|❯)\s?/.test(trimmed) || /^(?:pnpm|npm|yarn|flutter|go|git|dart)\b/.test(trimmed)) {
        return { kind: "command", text: line.startsWith("$") ? line : `$ ${line}` };
      }
      if (/^(?:\[stderr\]|stderr:|error\b|ERR!|✗)/i.test(trimmed)) {
        return { kind: "stderr", text: line };
      }
      return { kind: "stdout", text: line };
    }),
);
const exitCode = computed(() => {
  const match = /exit(?:\s+code)?[:\s]+(-?\d+)/i.exec(props.command);
  return match ? Number(match[1]) : null;
});

async function copyTerminal() {
  const copied = await copyText(props.command);
  copied ? ElMessage.success("已复制终端输出") : ElMessage.warning("复制失败");
}
</script>

<style scoped>
.amrp-terminal {
  width: 100%;
  margin: 14px 0 17px;
  overflow: hidden;
  border-radius: 10px;
  background: #111216;
}

.amrp-block-head {
  min-height: 36px;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 11px;
  background: #1c1d22;
}

.amrp-filename {
  color: #d2d3d6;
  font-size: 11.5px;
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

pre {
  max-height: 220px;
  margin: 0;
  overflow: auto;
  padding: 14px 16px 16px;
  color: #d5d7dd;
  font: 12.5px/1.7 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

pre.expanded {
  max-height: none;
}

.command {
  color: #86d39b;
}

.stderr {
  color: #f19494;
}

.stdout {
  color: #d5d7dd;
}
</style>
