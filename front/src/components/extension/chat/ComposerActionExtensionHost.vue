<script setup lang="ts">
import { computed } from "vue";
import ExtensionSlot from "@/components/extension/ExtensionSlot.vue";

const props = defineProps<{
  characterId: string;
  conversationId: string;
  channel: string;
  platform: string;
  host: string;
  os: string;
  conversationState: "idle" | "generating" | "offline";
  capabilities: string[];
  draft: string;
}>();

const composerContext = computed(() => ({
  characterId: props.characterId,
  conversationId: props.conversationId,
  channel: props.channel,
  platform: props.platform,
  host: props.host,
  os: props.os,
  conversationState: props.conversationState,
  capabilities: props.capabilities,
  draft: props.draft,
  scope: "composer",
}));
</script>

<template>
  <div class="composer-action-extension-host">
    <ExtensionSlot
      slot-id="chat.composer.action"
      :context="composerContext"
      fallback="none"
      layout="inline"
      surface-role="composer"
    />
  </div>
</template>

<style scoped>
.composer-action-extension-host {
  display: flex;
  align-items: center;
  justify-content: center;
}
</style>
