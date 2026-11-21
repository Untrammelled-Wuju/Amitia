<template>
  <div class="workbench-chat" data-amitia-ui="workbench.chat">
    <BuiltinWebChatView
      embedded
      :initial-conversation-id="conversationId"
      :initial-character-id="seed?.roleId || ''"
      :initial-draft-text="seed?.message || ''"
      @conversation-created="handleCreated"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import BuiltinWebChatView from "@/views/web-chat/BuiltinWebChatView.vue";
import { useWorkbenchStore } from "@/stores/workbench";

const props = defineProps<{ tabId: string }>();
const store = useWorkbenchStore();
const seed = computed(() => store.sideChatSeed(props.tabId));
const conversationId = ref(seed.value?.conversationId || "");

function handleCreated(id: string) {
  conversationId.value = id;
  store.rememberSideChatConversation(props.tabId, id);
}
</script>

<style scoped>
.workbench-chat { display: flex; height: 100%; min-height: 0; }
.workbench-chat :deep(.webchat-page),
.workbench-chat :deep(.chat-surface) { flex: 1 1 auto; height: 100%; min-height: 0; }
</style>
