<template>
  <section class="amrp-fallback">
    <header class="amrp-block-head">
      <span class="amrp-title">暂不支持的扩展内容</span>
      <span class="amrp-meta">Fallback Renderer</span>
    </header>
    <div class="amrp-fallback-body">
      <p v-if="block.kind === 'extension'">
        当前客户端没有注册 <code>{{ block.rendererId }}</code> Renderer，因此显示安全降级内容。
      </p>
      <p v-else>未知 Block 类型：<code>{{ block.type }}</code></p>
      <pre>{{ payloadText }}</pre>
      <button type="button" @click="copyPayload">复制 JSON</button>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { ElMessage } from "element-plus";
import type { ExtensionBlock, UnknownRichBlock } from "../types";
import { copyText, prettyJson } from "../utils";

const props = defineProps<{
  block: ExtensionBlock | UnknownRichBlock;
}>();

const payloadText = computed(() =>
  prettyJson(props.block.kind === "extension" ? props.block.payload : props.block.payload),
);

async function copyPayload() {
  const copied = await copyText(payloadText.value);
  copied ? ElMessage.success("已复制降级内容") : ElMessage.warning("复制失败");
}
</script>

<style scoped>
.amrp-fallback {
  width: 100%;
  max-width: 700px;
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

.amrp-fallback-body {
  padding: 12px;
  color: var(--amrp-muted);
  font-size: 11.5px;
}

code {
  color: var(--amrp-accent);
}

pre {
  max-height: 260px;
  margin: 8px 0 0;
  overflow: auto;
  padding: 9px;
  border-radius: 7px;
  background: var(--amrp-soft);
  color: var(--amrp-text);
  font: 10.5px/1.55 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  white-space: pre-wrap;
}

button {
  margin-top: 8px;
  border: 0;
  border-radius: 6px;
  padding: 4px 7px;
  background: var(--amrp-control);
  color: var(--amrp-accent);
  font: inherit;
  font-size: 10px;
  cursor: pointer;
}
</style>

