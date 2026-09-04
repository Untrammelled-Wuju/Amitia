<script setup lang="ts">
import { computed } from "vue";
import { MoreFilled } from "@element-plus/icons-vue";
import ExtensionSlot from "@/components/extension/ExtensionSlot.vue";
import { useExtensionUIStore } from "@/stores/extensionUI";
import { browserClientPluginRuntime } from "@/ui-runtime/clientPluginRuntime";
import { buildUnifiedSlotItems, type UnifiedSlotItem } from "@/ui-runtime/slotLedger";

const props = withDefaults(defineProps<{ context: Record<string, unknown>; maxItems?: number }>(), { maxItems: 4 });
const store = useExtensionUIStore();

const resolvedItems = computed(() => {
  browserClientPluginRuntime.slots.revision.value;
  const contract = store.slotsById.get("chat.header.action") ?? browserClientPluginRuntime.slots.getDefinition("chat.header.action");
  return buildUnifiedSlotItems(
    contract,
    store.getVisibleContributions("chat.header.action", props.context),
    browserClientPluginRuntime.slots.listContributions("chat.header.action"),
  );
});

function actionCount(item: UnifiedSlotItem): number {
  if (item.source === "server") return item.server.actions?.length ?? 1;
  return 1;
}

const totalActionCount = computed(() => resolvedItems.value.reduce((sum, item) => sum + actionCount(item), 0));
const hasOverflow = computed(() => totalActionCount.value > props.maxItems);

const visibleItems = computed(() => {
  const result: UnifiedSlotItem[] = [];
  let count = 0;
  for (const item of resolvedItems.value) {
    const next = actionCount(item);
    if (count + next > props.maxItems) break;
    result.push(item);
    count += next;
  }
  return result;
});

const overflowItems = computed(() => {
  const result: UnifiedSlotItem[] = [];
  let count = 0;
  let overflowing = false;
  for (const item of resolvedItems.value) {
    const next = actionCount(item);
    if (!overflowing && count + next <= props.maxItems) {
      count += next;
      continue;
    }
    overflowing = true;
    result.push(item);
  }
  return result;
});
</script>

<template>
  <div class="chat-header-extension-host">
    <ExtensionSlot
      v-for="item in visibleItems"
      :key="item.key"
      slot-id="chat.header.action"
      :context="context"
      :contribution-id="item.contributionId"
      fallback="none"
      layout="inline"
      surface-role="header"
    />
    <el-dropdown v-if="hasOverflow" trigger="click" placement="bottom-end">
      <button type="button" class="chat-header-extension-host__overflow" aria-label="更多扩展操作"><el-icon><MoreFilled /></el-icon></button>
      <template #dropdown>
        <div class="chat-header-extension-host__menu">
          <ExtensionSlot
            v-for="item in overflowItems"
            :key="item.key"
            slot-id="chat.header.action"
            :context="context"
            :contribution-id="item.contributionId"
            fallback="none"
            layout="stack"
            surface-role="overlay"
          />
        </div>
      </template>
    </el-dropdown>
  </div>
</template>

<style scoped>
.chat-header-extension-host { display: inline-flex; align-items: center; gap: 4px; }
.chat-header-extension-host__overflow { display: grid; place-items: center; width: 32px; height: 32px; border: 1px solid transparent; border-radius: var(--radius-sm); background: transparent; color: var(--text-secondary); cursor: pointer; }
.chat-header-extension-host__overflow:hover, .chat-header-extension-host__overflow:focus-visible { border-color: var(--surface-border); background: var(--control-hover-bg); outline: none; }
.chat-header-extension-host__menu { width: 220px; padding: 6px; }
</style>
