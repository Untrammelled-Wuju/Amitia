<template>
  <RendererErrorBoundary :label="blockName">
    <AmitiaToolBlock v-if="block.kind === 'tool'" :block="block" />
    <AmitiaFileBlock v-else-if="block.kind === 'file'" :block="block" />
    <AmitiaImageBlock v-else-if="block.kind === 'image'" :images="[block]" />
    <AmitiaAudioBlock v-else-if="block.kind === 'audio'" :block="block" />
    <AmitiaVideoBlock v-else-if="block.kind === 'video'" :block="block" />
    <AmitiaAgentTaskBlock v-else-if="block.kind === 'agent-task'" :block="block" />
    <AmitiaArtifactBlock v-else-if="block.kind === 'artifact'" :block="block" />
    <AmitiaExtensionBlock v-else-if="block.kind === 'extension'" :block="block" />
    <AmitiaRendererFallback v-else :block="block" />
  </RendererErrorBoundary>
</template>

<script setup lang="ts">
import { computed } from "vue";
import type { RichBlock } from "../types";
import AmitiaToolBlock from "./AmitiaToolBlock.vue";
import AmitiaFileBlock from "./AmitiaFileBlock.vue";
import AmitiaImageBlock from "./AmitiaImageBlock.vue";
import AmitiaAudioBlock from "./AmitiaAudioBlock.vue";
import AmitiaVideoBlock from "./AmitiaVideoBlock.vue";
import AmitiaAgentTaskBlock from "./AmitiaAgentTaskBlock.vue";
import AmitiaArtifactBlock from "./AmitiaArtifactBlock.vue";
import AmitiaExtensionBlock from "./AmitiaExtensionBlock.vue";
import AmitiaRendererFallback from "./AmitiaRendererFallback.vue";
import RendererErrorBoundary from "./RendererErrorBoundary.vue";

const props = defineProps<{
  block: RichBlock;
}>();

const labels: Record<RichBlock["kind"], string> = {
  tool: "Tool",
  file: "File",
  image: "Image",
  audio: "Audio",
  video: "Video",
  "agent-task": "Agent",
  artifact: "Artifact",
  extension: "Extension",
  unknown: "Fallback",
};

const blockName = computed(() => `${labels[props.block.kind]} Renderer`);
</script>
