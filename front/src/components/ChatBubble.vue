<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <AIMessageRenderer
    :message="message"
    :char-name="charName"
    :char-avatar="charAvatar"
    :character-id="characterId"
    :read-only="readOnly"
    @retry="$emit('retry', $event)"
    @reply="$emit('reply', $event)"
    @edit="$emit('edit', $event)"
    @scroll-to-message="$emit('scroll-to-message', $event)"
  >
    <template #badges="slotProps">
      <slot name="badges" v-bind="slotProps" />
    </template>
    <template #extension-content="slotProps">
      <slot name="extension-content" v-bind="slotProps" />
    </template>
    <template #actions="slotProps">
      <slot name="actions" v-bind="slotProps" />
    </template>
  </AIMessageRenderer>
</template>

<script setup lang="ts">
import AIMessageRenderer from "@/conversation/rendering/AIMessageRenderer.vue";

withDefaults(
  defineProps<{
    message: Record<string, any>;
    charName?: string;
    charAvatar?: string;
    isStreaming?: boolean;
    status?: string;
    characterId?: string;
    readOnly?: boolean;
    reasoningOpen?: boolean;
  }>(),
  {
    charName: "Amitia",
    charAvatar: "",
    isStreaming: false,
    status: "",
    characterId: "",
    readOnly: false,
    reasoningOpen: false,
  },
);

defineEmits<{
  retry: [message: Record<string, any>];
  reply: [message: Record<string, any>];
  edit: [message: Record<string, any>];
  "scroll-to-message": [id: string];
}>();

</script>
