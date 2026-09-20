<template>
  <component
    :is="registeredRenderer"
    v-if="registeredRenderer"
    :payload="block.payload"
    :renderer-id="block.rendererId"
    :version="block.version"
  />
  <AmitiaRendererFallback v-else :block="block" />
</template>

<script setup lang="ts">
import { computed } from "vue";
import type { ExtensionBlock } from "../types";
import { resolveExtensionRenderer } from "../rendererRegistry";
import AmitiaRendererFallback from "./AmitiaRendererFallback.vue";

const props = defineProps<{
  block: ExtensionBlock;
}>();

const registeredRenderer = computed(() => resolveExtensionRenderer(props.block.rendererId));
</script>

