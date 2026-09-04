<script setup lang="ts">
import { computed } from "vue";
import ExtensionSlot from "@/components/extension/ExtensionSlot.vue";

const props = defineProps<{ messageId: string; messageType: string; direction: "incoming" | "outgoing" | "system"; senderType: "user" | "character" | "system" | "extension"; characterId: string; conversationId: string }>();
const context = computed(() => ({ messageId: props.messageId, messageType: props.messageType, direction: props.direction, senderType: props.senderType, characterId: props.characterId, conversationId: props.conversationId, capabilities: ["text"], scope: "message_action" }));
</script>
<template><ExtensionSlot slot-id="chat.message.action" :context="context" fallback="none" layout="inline" surface-role="message" /></template>
