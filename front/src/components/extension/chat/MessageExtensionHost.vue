<script setup lang="ts">
import { computed } from "vue";
import ExtensionSlot from "@/components/extension/ExtensionSlot.vue";

interface MessageSummary {
  messageId: string;
  type: string;
  direction: "incoming" | "outgoing" | "system";
  senderType: "user" | "character" | "system" | "extension";
  createdAt: string;
  status: string;
  hasText: boolean;
  attachmentTypes: string[];
  extensionType?: string;
}

const props = defineProps<{
  message: MessageSummary;
  characterId: string;
  conversationId: string;
}>();

const messageContext = computed(() => ({
  messageId: props.message.messageId,
  messageType: props.message.type,
  direction: props.message.direction,
  senderType: props.message.senderType,
  characterId: props.characterId,
  conversationId: props.conversationId,
  capabilities: ["text"],
  extensionType: props.message.extensionType,
}));

const hasAttachments = computed(() => props.message.attachmentTypes.length > 0);
const isCustomMessage = computed(() => !!props.message.extensionType);
</script>

<template>
  <div class="message-extension-host" :data-message-id="message.messageId">
    <div class="message-extension-host__badges">
      <ExtensionSlot
        slot-id="chat.message.badge"
        :context="messageContext"
        fallback="none"
        layout="inline"
      />
    </div>

    <div class="message-extension-host__content">
      <template v-if="isCustomMessage">
        <ExtensionSlot
          slot-id="chat.message.custom_renderer"
          :context="messageContext"
          fallback="default"
          layout="stack"
        />
      </template>
      <template v-if="hasAttachments">
        <ExtensionSlot
          slot-id="chat.message.attachment_renderer"
          :context="messageContext"
          fallback="none"
          layout="stack"
        />
      </template>
    </div>

    <div class="message-extension-host__actions">
      <ExtensionSlot
        slot-id="chat.message.action"
        :context="messageContext"
        fallback="none"
        layout="inline"
      />
    </div>
  </div>
</template>

<style scoped>
.message-extension-host {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}
.message-extension-host__badges {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
}
.message-extension-host__content {
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.message-extension-host__actions {
  display: flex;
  gap: 4px;
  opacity: 0;
  transition: opacity 0.15s;
}
.message-extension-host:hover .message-extension-host__actions {
  opacity: 1;
}
</style>
