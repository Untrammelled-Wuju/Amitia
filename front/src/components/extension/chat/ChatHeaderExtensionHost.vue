<script setup lang="ts">
import { computed } from "vue";
import { MoreFilled } from "@element-plus/icons-vue";
import ExtensionSlot from "@/components/extension/ExtensionSlot.vue";
import { useExtensionUIStore } from "@/stores/extensionUI";
const props = withDefaults(defineProps<{ context: Record<string, unknown>; maxItems?: number }>(), { maxItems: 4 });
const store = useExtensionUIStore();
const hasOverflow = computed(() => store.getVisibleContributions("chat.header.action").length > props.maxItems);
</script>
<template>
  <div class="chat-header-extension-host">
    <ExtensionSlot slot-id="chat.header.action" :context="context" :max-items="maxItems" fallback="none" layout="inline" surface-role="header" />
    <el-dropdown v-if="hasOverflow" trigger="click" placement="bottom-end">
      <button type="button" class="chat-header-extension-host__overflow" aria-label="更多扩展操作"><el-icon><MoreFilled /></el-icon></button>
      <template #dropdown><div class="chat-header-extension-host__menu"><ExtensionSlot slot-id="chat.header.action" :context="context" :start-index="maxItems" fallback="none" layout="stack" surface-role="overlay" /></div></template>
    </el-dropdown>
  </div>
</template>
<style scoped>
.chat-header-extension-host { display: inline-flex; align-items: center; gap: 4px; }
.chat-header-extension-host__overflow { display: grid; place-items: center; width: 32px; height: 32px; border: 1px solid transparent; border-radius: var(--radius-sm); background: transparent; color: var(--text-secondary); cursor: pointer; }
.chat-header-extension-host__overflow:hover, .chat-header-extension-host__overflow:focus-visible { border-color: var(--surface-border); background: var(--control-hover-bg); outline: none; }
.chat-header-extension-host__menu { width: 220px; padding: 6px; }
</style>
