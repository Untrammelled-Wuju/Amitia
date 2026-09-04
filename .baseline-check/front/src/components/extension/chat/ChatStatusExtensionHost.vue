<script setup lang="ts">
import ExtensionSlot from "@/components/extension/ExtensionSlot.vue";
defineProps<{ context: Record<string, unknown> }>();
</script>
<template><ExtensionSlot slot-id="chat.status.item" :context="context" fallback="none" layout="inline" surface-role="status" /></template>
