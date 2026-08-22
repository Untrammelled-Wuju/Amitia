<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import ExtensionSlot from "@/components/extension/ExtensionSlot.vue";
import { useExtensionUIStore } from "@/stores/extensionUI";
import {
  projectConversationEvent,
  subscribeConversationRuntimeEvents,
  type ConversationNode,
  type RuntimeConversationEvent,
} from "@/ui-runtime/conversationProjection";

const props = defineProps<{
  conversationId: string;
  messages?: any[];
  context?: Record<string, unknown>;
}>();

const store = useExtensionUIStore();
const nodes = ref<Record<string, ConversationNode>>({});
let unsubscribe: (() => void) | null = null;

const contributions = computed(() => store.getVisibleContributions("chat.conversation.node", {
  ...(props.context ?? {}),
  conversationId: props.conversationId,
}));
const orderedNodes = computed(() => Object.values(nodes.value)
  .filter((node) => node.conversationId === props.conversationId)
  .sort((a, b) => Date.parse(a.createdAt) - Date.parse(b.createdAt)));

function consume(event: RuntimeConversationEvent) {
  if (!event.conversationId || event.conversationId !== props.conversationId) return;
  for (const contribution of contributions.value) {
    const provisionalKeyPrefix = `${contribution.contributionId}:`;
    let current: ConversationNode | undefined;
    for (const node of Object.values(nodes.value)) {
      if (node.nodeId.startsWith(provisionalKeyPrefix)) {
        const projected = projectConversationEvent(event, contribution, node);
        if (projected && projected.nodeId === node.nodeId) {
          current = node;
          break;
        }
      }
    }
    const projected = projectConversationEvent(event, contribution, current);
    if (projected) nodes.value = { ...nodes.value, [projected.nodeId]: projected };
  }
}

function replayMessages() {
  const next: Record<string, ConversationNode> = {};
  nodes.value = next;
  for (const message of props.messages ?? []) {
    const conversationId = String(message?.conversationId ?? props.conversationId ?? "");
    if (!conversationId) continue;
    consume({
      id: String(message?.id ?? `message-${message?.createdAt ?? Date.now()}`),
      eventType: "message_created",
      conversationId,
      timestamp: String(message?.createdAt ?? new Date().toISOString()),
      source: "history",
      payload: message && typeof message === "object" ? { ...message } : { value: message },
    });
  }
}

watch(() => [props.conversationId, props.messages] as const, replayMessages, { deep: true });
watch(contributions, replayMessages, { deep: true });

onMounted(() => {
  unsubscribe = subscribeConversationRuntimeEvents(consume);
  replayMessages();
});
onBeforeUnmount(() => unsubscribe?.());
</script>

<template>
  <div v-if="orderedNodes.length" class="conversation-projection-host">
    <div
      v-for="node in orderedNodes"
      :key="node.nodeId"
      class="conversation-projection-host__node"
      :data-conversation-node-id="node.nodeId"
    >
      <ExtensionSlot
        slot-id="chat.conversation.node"
        :contribution-id="node.contributionId"
        :context="{ ...(context || {}), conversationId, eventType: node.eventType, conversationNode: node }"
        fallback="none"
        layout="stack"
        surface-role="message"
      />
    </div>
  </div>
</template>

<style scoped>
.conversation-projection-host,
.conversation-projection-host__node {
  display: flex;
  flex-direction: column;
  width: 100%;
  min-width: 0;
}
.conversation-projection-host { gap: 8px; }
</style>
