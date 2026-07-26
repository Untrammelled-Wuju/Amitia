<script setup lang="ts">
import { computed } from "vue";
import ExtensionSlot from "@/components/extension/ExtensionSlot.vue";

interface ChatUIContext {
  characterId: string;
  conversationId: string;
  channel: string;
  platform: string;
  conversationState: "idle" | "generating" | "offline";
  capabilities: string[];
}

const props = defineProps<{
  context: ChatUIContext;
  maxHeaderActions?: number;
}>();

const headerContext = computed(() => ({
  characterId: props.context.characterId,
  conversationId: props.context.conversationId,
  channel: props.context.channel,
  platform: props.context.platform,
  conversationState: props.context.conversationState,
  capabilities: props.context.capabilities,
}));

const sidebarContext = computed(() => ({ ...headerContext.value, scope: "sidebar" }));
const statusContext = computed(() => ({ ...headerContext.value, scope: "status" }));
const emptyStateContext = computed(() => ({ ...headerContext.value, scope: "empty_state" }));

const maxHeader = computed(() => props.maxHeaderActions ?? 4);
</script>

<template>
  <div class="chat-extension-host">
    <div class="chat-extension-host__header-actions">
      <ExtensionSlot
        slot-id="chat.header.action"
        :context="headerContext"
        :max-items="maxHeader"
        fallback="none"
        layout="inline"
      />
    </div>

    <div class="chat-extension-host__status-bar">
      <ExtensionSlot
        slot-id="chat.status.item"
        :context="statusContext"
        fallback="none"
        layout="inline"
      />
    </div>

    <aside class="chat-extension-host__sidebar">
      <ExtensionSlot
        slot-id="chat.sidebar.panel"
        :context="sidebarContext"
        fallback="empty"
        layout="stack"
      />
    </aside>

    <div class="chat-extension-host__empty-state">
      <ExtensionSlot
        slot-id="chat.empty_state.card"
        :context="emptyStateContext"
        fallback="empty"
        layout="stack"
      />
    </div>
  </div>
</template>

<style scoped>
.chat-extension-host {
  display: contents;
}
.chat-extension-host__header-actions {
  display: inline-flex;
  align-items: center;
}
.chat-extension-host__status-bar {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 2px 0;
}
.chat-extension-host__sidebar {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
}
.chat-extension-host__empty-state {
  display: flex;
  flex-direction: column;
}
</style>
