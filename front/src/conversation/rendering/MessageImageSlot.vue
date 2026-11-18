<template>
  <ExtensionSlot
    :slot-id="slotId"
    :context="slotContext"
    fallback="default"
    layout="stack"
    surface-role="message"
  >
    <AmitiaImageBlock :images="images" />
  </ExtensionSlot>
</template>

<script setup lang="ts">
import { computed } from "vue";
import ExtensionSlot from "@/components/extension/ExtensionSlot.vue";
import AmitiaImageBlock from "./blocks/AmitiaImageBlock.vue";
import type { AIMessageData, ImageBlock } from "./types";

const props = withDefaults(
  defineProps<{
    images: ImageBlock[];
    message: AIMessageData;
    extensionContext?: Record<string, unknown>;
  }>(),
  {
    extensionContext: () => ({}),
  },
);

const slotId = computed(() =>
  props.message.role === "user"
    ? "chat.message.user.images"
    : "chat.message.images",
);

const slotContext = computed(() => ({
  ...props.extensionContext,
  messageId: props.message.id,
  conversationId: props.message.conversationId,
  sessionId: props.message.conversationId,
  role: props.message.role,
  direction: props.message.role === "user" ? "outgoing" : "incoming",
  senderType: props.message.role === "user" ? "user" : "character",
  scope: "message-images",
  images: props.images.map((image) => ({
    id: image.id,
    alt: image.alt || "",
    mimeType: image.mimeType || "",
    width: image.width || 0,
    height: image.height || 0,
    animated: image.animated === true,
    status: image.status || "ready",
  })),
  message: {
    id: props.message.id,
    conversationId: props.message.conversationId,
    role: props.message.role,
    state: props.message.state,
    imageIds: props.images.map((image) => image.id),
    raw: props.message.raw,
  },
}));
</script>
