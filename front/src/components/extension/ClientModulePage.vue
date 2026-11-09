<script setup lang="ts">
import { computed } from "vue";
import { pageModuleComponent } from "@/ui-runtime/pageModuleRuntime";

const props = defineProps<{
  extensionId: string;
  pageId: string;
  generation: number;
  entryPath: string;
  entryHash?: string;
  moduleToken?: string;
  context: Record<string, unknown>;
  actions?: Record<string, (input?: unknown) => unknown | Promise<unknown>>;
  navigate?: (target: string, type?: string) => unknown;
}>();

const component = computed(() => pageModuleComponent({
  extensionId: props.extensionId,
  pageId: props.pageId,
  generation: props.generation,
  entryPath: props.entryPath,
  entryHash: props.entryHash,
  moduleToken: props.moduleToken,
}));

const moduleContext = computed(() => ({
  ...props.context,
  extensionId: props.extensionId,
  pageId: props.pageId,
  generation: props.generation,
}));
</script>

<template>
  <component
    :is="component"
    :ui-context="moduleContext"
    :ui-actions="actions"
    :ui-navigate="navigate"
  />
</template>

<style scoped>
:deep(.ui-provider-module) {
  width: 100%;
  min-width: 0;
  min-height: 0;
}
</style>
