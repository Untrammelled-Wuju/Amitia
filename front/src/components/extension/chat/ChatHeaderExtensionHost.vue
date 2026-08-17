<script setup lang="ts">
import { computed } from "vue";
import { MoreFilled } from "@element-plus/icons-vue";
import ExtensionSlot from "@/components/extension/ExtensionSlot.vue";
import { useExtensionUIStore } from "@/stores/extensionUI";
const props = withDefaults(defineProps<{ context: Record<string, unknown>; maxItems?: number }>(), { maxItems: 4 });
const store = useExtensionUIStore();
const contributions = computed(() => store.getVisibleContributions("chat.header.action"));
const totalActionCount = computed(() => {
  let count = 0;
  for (const c of contributions.value) {
    if (c.actions && Array.isArray(c.actions)) {
      count += c.actions.length;
    } else {
      count += 1;
    }
  }
  return count;
});
const hasOverflow = computed(() => totalActionCount.value > props.maxItems);
const visibleContributions = computed(() => {
  const result: typeof contributions.value = [];
  let actionCount = 0;
  for (const c of contributions.value) {
    const cActionCount = c.actions?.length ?? 1;
    if (actionCount + cActionCount <= props.maxItems) {
      result.push(c);
      actionCount += cActionCount;
    } else {
      break;
    }
  }
  return result;
});
const overflowContributions = computed(() => {
  const result: typeof contributions.value = [];
  let actionCount = 0;
  let started = false;
  for (const c of contributions.value) {
    const cActionCount = c.actions?.length ?? 1;
    if (!started && actionCount + cActionCount <= props.maxItems) {
      actionCount += cActionCount;
      continue;
    }
    started = true;
    result.push(c);
  }
  return result;
});
</script>
<template>
  <div class="chat-header-extension-host">
    <ExtensionSlot v-for="contribution in visibleContributions" :key="contribution.contributionId" slot-id="chat.header.action" :context="context" :contribution-id="contribution.contributionId" fallback="none" layout="inline" surface-role="header" />
    <el-dropdown v-if="hasOverflow" trigger="click" placement="bottom-end">
      <button type="button" class="chat-header-extension-host__overflow" aria-label="更多扩展操作"><el-icon><MoreFilled /></el-icon></button>
      <template #dropdown><div class="chat-header-extension-host__menu"><ExtensionSlot v-for="contribution in overflowContributions" :key="contribution.contributionId" slot-id="chat.header.action" :context="context" :contribution-id="contribution.contributionId" fallback="none" layout="stack" surface-role="overlay" /></div></template>
    </el-dropdown>
  </div>
</template>
<style scoped>
.chat-header-extension-host { display: inline-flex; align-items: center; gap: 4px; }
.chat-header-extension-host__overflow { display: grid; place-items: center; width: 32px; height: 32px; border: 1px solid transparent; border-radius: var(--radius-sm); background: transparent; color: var(--text-secondary); cursor: pointer; }
.chat-header-extension-host__overflow:hover, .chat-header-extension-host__overflow:focus-visible { border-color: var(--surface-border); background: var(--control-hover-bg); outline: none; }
.chat-header-extension-host__menu { width: 220px; padding: 6px; }
</style>
