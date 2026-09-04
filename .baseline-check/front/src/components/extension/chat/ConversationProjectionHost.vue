<script setup lang="ts">
import ExtensionSlot from "@/components/extension/ExtensionSlot.vue";
import type { ConversationNode } from "@/ui-runtime/conversationProjection";

defineProps<{
  node: ConversationNode;
  context?: Record<string, unknown>;
}>();
</script>

<template>
  <div
    class="conversation-projection-host__node"
    :data-conversation-node-id="node.nodeId"
    :data-anchor-seq="node.anchorSeq"
  >
    <ExtensionSlot
      slot-id="chat.conversation.node"
      :contribution-id="node.contributionId"
      :context="{ ...(context || {}), conversationId: node.conversationId, eventType: node.eventType, conversationNode: node }"
      fallback="none"
      layout="stack"
      surface-role="message"
    />
  </div>
</template>

<style scoped>
.conversation-projection-host__node {
  display: flex;
  flex-direction: column;
  width: 100%;
  min-width: 0;
}
</style>
