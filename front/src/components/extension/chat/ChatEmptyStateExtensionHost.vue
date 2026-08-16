<script setup lang="ts">
import ExtensionSlot from "@/components/extension/ExtensionSlot.vue";
defineProps<{ context: Record<string, unknown> }>();
</script>
<template><ExtensionSlot slot-id="chat.empty_state.card" :context="context" fallback="none" layout="stack" surface-role="main" /></template>
