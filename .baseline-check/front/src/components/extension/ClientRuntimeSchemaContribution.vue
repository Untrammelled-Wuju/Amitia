<script setup lang="ts">
import { computed } from "vue";
import SchemaUIRenderer from "./SchemaUIRenderer.vue";
import type { UIContributionSummary } from "@/stores/extensionUI";

const props = withDefaults(defineProps<{
  sourceExtensionId: string;
  sourceContributionId: string;
  contributionId?: string;
  pluginId?: string;
  slotId: string;
  context?: Record<string, unknown>;
  title?: string;
  kind?: string;
}>(), {
  contributionId: "",
  pluginId: "",
  context: () => ({}),
  title: "Dynamic UI",
  kind: "panel",
});

const rendererContext = computed(() => ({
  ...(props.context ?? {}),
  ...(props.pluginId ? { clientRuntimePluginId: props.pluginId } : {}),
}));

const contribution = computed<UIContributionSummary>(() => ({
  // SchemaUIRenderer resolves the canonical schema using the source extension
  // and contribution IDs. The client-runtime contribution ID remains available
  // to the surrounding ExtensionSlot for lifecycle/ordering identity.
  contributionId: props.sourceContributionId,
  extensionId: props.sourceExtensionId,
  moduleId: "client-runtime",
  kind: props.kind,
  slotId: props.slotId,
  contractVersion: 1,
  generation: 1,
  title: props.title,
  ordering: 0,
  visible: true,
  effective: true,
  enabled: true,
  runtimeReady: true,
  sandbox: "schema_renderer",
  schemaPath: `runtime:${props.sourceExtensionId}/${props.sourceContributionId}`,
  dataContract: {
    clientRuntimeContributionId: props.contributionId,
    clientRuntimePluginId: props.pluginId,
  },
}));
</script>

<template>
  <SchemaUIRenderer
    :contribution="contribution"
    :context="rendererContext"
    :slot-id="slotId"
  />
</template>
