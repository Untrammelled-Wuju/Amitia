<script setup lang="ts">
import ChatStatusExtensionHost from "@/components/extension/chat/ChatStatusExtensionHost.vue";
import RealtimeCallWidget from "@/components/RealtimeCallWidget.vue";

defineProps<{
  uiContextValue: Record<string, unknown>;
  callActive: boolean;
  hasStatusExtensions: boolean;
  apiKey: string;
  voiceType: string;
  resourceId: string;
  conversationId: string;
}>();

const emit = defineEmits<{ (e: "state-change", state: string): void }>();
</script>

<template>
  <template v-if="callActive || hasStatusExtensions">
    <ChatStatusExtensionHost :context="uiContextValue" />
    <RealtimeCallWidget
      v-if="callActive"
      :visible="callActive"
      :api-key="apiKey"
      :voice-type="voiceType"
      :resource-id="resourceId"
      :conversation-id="conversationId"
      @state-change="(state) => emit('state-change', state)"
    />
  </template>
</template>
