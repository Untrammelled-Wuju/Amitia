<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<script setup lang="ts">
import ChatStatusExtensionHost from "@/components/extension/chat/ChatStatusExtensionHost.vue";

defineProps<{
  uiContextValue: Record<string, unknown>;
  callActive: boolean;
  hasStatusExtensions: boolean;
}>();
</script>

<template>
  <template v-if="callActive || hasStatusExtensions">
    <ChatStatusExtensionHost :context="uiContextValue" />
  </template>
</template>
