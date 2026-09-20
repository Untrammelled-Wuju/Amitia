<template>
  <section class="amrp-plugin">
    <header class="amrp-block-head">
      <span class="amrp-title">conversation.renderer · minecraft.server</span>
      <span class="amrp-meta">Extension Block</span>
    </header>
    <div class="amrp-plugin-body">
      <div class="amrp-plugin-hero">
        <div class="amrp-plugin-icon">M</div>
        <div>
          <div class="amrp-plugin-title">{{ title }}</div>
          <div class="amrp-plugin-sub">{{ detail }}</div>
        </div>
        <span class="amrp-grow"></span>
        <button type="button" @click="copyPayload">复制状态</button>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { ElMessage } from "element-plus";
import { copyText, prettyJson } from "../utils";

const props = defineProps<{
  payload: unknown;
  rendererId: string;
  version: number;
}>();

const data = computed(() =>
  props.payload && typeof props.payload === "object"
    ? (props.payload as Record<string, any>)
    : {},
);
const title = computed(() => String(data.value.title ?? "Minecraft 服务器已连接"));
const detail = computed(() =>
  [
    data.value.player,
    data.value.address,
    data.value.tps ? `${data.value.tps} TPS` : "",
  ]
    .filter(Boolean)
    .join(" · "),
);

async function copyPayload() {
  const copied = await copyText(prettyJson(props.payload));
  copied ? ElMessage.success("已复制扩展状态") : ElMessage.warning("复制失败");
}
</script>

<style scoped>
.amrp-plugin {
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
  min-width: 0;
  flex: 1;
  overflow: hidden;
  font-size: 11px;
  font-weight: 680;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.amrp-meta {
  color: var(--amrp-muted);
  font-size: 10px;
}

.amrp-plugin-body {
  padding: 12px;
}

.amrp-plugin-hero {
  min-height: 72px;
  display: flex;
  align-items: center;
  gap: 10px;
  border-radius: 8px;
  padding: 0 14px;
  background: linear-gradient(135deg, var(--amrp-accent-soft), var(--amrp-soft));
}

.amrp-plugin-icon {
  width: 36px;
  height: 36px;
  display: grid;
  flex: 0 0 auto;
  place-items: center;
  border-radius: 10px;
  background: var(--amrp-accent);
  color: white;
  font-size: 16px;
}

.amrp-plugin-title {
  font-weight: 680;
}

.amrp-plugin-sub {
  color: var(--amrp-muted);
  font-size: 10.5px;
}

.amrp-grow {
  flex: 1;
}

.amrp-plugin-hero button {
  border: 0;
  border-radius: 6px;
  padding: 4px 7px;
  background: var(--amrp-surface);
  color: var(--amrp-text);
  font: inherit;
  font-size: 10px;
  cursor: pointer;
}
</style>

