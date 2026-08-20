<script setup lang="ts">
import ChatSidebarExtensionHost from "@/components/extension/chat/ChatSidebarExtensionHost.vue";
defineProps<{ uiContextValue: Record<string, unknown> }>();
</script>

<template>
  <ChatSidebarExtensionHost :context="uiContextValue" />
</template>
