<script setup lang="ts">
import { computed, defineAsyncComponent } from "vue";
import { useRoute } from "vue-router";
import { listMainPanels, resolveMainPanel } from "@/ui-runtime/mainPanelRegistry";

const route = useRoute();
const panelId = computed(() => String(route.params.panelId ?? ""));
const panel = computed(() => resolveMainPanel(panelId.value));
const registered = computed(() => listMainPanels());
const renderer = computed(() => {
  const target = panel.value;
  if (!target) return undefined;
  const candidate = target.component as unknown;
  if (typeof candidate === "function" && (candidate as { render?: unknown }).render === undefined) {
    return defineAsyncComponent(candidate as () => Promise<{ default: import("vue").Component }>);
  }
  return candidate as import("vue").Component;
});
</script>

<template>
  <div class="main-panel-host">
    <component :is="renderer" v-if="panel && renderer" :panel-id="panel.id" />
    <div v-else class="main-panel-host__missing">
      <p>主面板 {{ panelId || "<空>" }} 未注册，无法渲染。</p>
      <p>已注册面板：{{ registered.length === 0 ? "无" : registered.map((item) => item.id).join("、") }}</p>
    </div>
  </div>
</template>

<style scoped>
.main-panel-host {
  display: flex;
  flex-direction: column;
  flex: 1 1 auto;
  min-height: 0;
  overflow: auto;
}
.main-panel-host__missing {
  padding: 24px;
  color: var(--text-secondary, #666);
  font-size: 13px;
  line-height: 1.7;
}
</style>
