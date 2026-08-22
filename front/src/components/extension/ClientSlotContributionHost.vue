<script setup lang="ts">
import { computed, inject, provide, shallowReadonly } from "vue";
import type { ClientSlotContribution } from "@/ui-runtime/clientPluginRuntime";
import { CLIENT_SLOT_RUNTIME_CONTEXT } from "@/ui-runtime/clientSlotContext";

const props = defineProps<{
  contribution: ClientSlotContribution;
  context: Record<string, unknown>;
}>();

const parent = inject(CLIENT_SLOT_RUNTIME_CONTEXT, null);
const injectedFace = shallowReadonly(props.contribution.injected ?? {});

provide(CLIENT_SLOT_RUNTIME_CONTEXT, {
  pluginId: props.contribution.pluginId,
  slotId: props.contribution.slotId,
  contributionId: props.contribution.contributionId,
  store: props.contribution.store,
  injected: injectedFace,
  parent,
});

const componentProps = computed(() => ({
  ...(props.contribution.props ?? {}),
  ...injectedFace,
}));
</script>

<template>
  <component
    :is="contribution.component"
    v-bind="componentProps"
    :context="context"
    :slot-id="contribution.slotId"
    :plugin-id="contribution.pluginId"
    :contribution-id="contribution.contributionId"
    :slot-store="contribution.store"
    :slot-inject="injectedFace"
  />
</template>
