<script setup lang="ts">
import { computed, ref, watch } from "vue";
import type { UIContributionSummary } from "@/stores/extensionUI";
import SchemaUIRenderer from "./SchemaUIRenderer.vue";
import ComposerCommandPalette from "@/components/ComposerCommandPalette.vue";

const props = defineProps<{
  contribution: UIContributionSummary;
  context: Record<string, unknown>;
  slotId: string;
  hostActions?: Record<string, (input?: unknown) => unknown | Promise<unknown>>;
}>();
const emit = defineEmits<{ (e: "error", error: string): void }>();
const open = ref(false);
const title = computed(() => props.contribution.title || props.contribution.contributionId);
const instanceId = computed(
  () => `${props.contribution.contributionId}:${props.contribution.generation}`,
);

watch(
  () => [props.context.conversationId, props.context.characterId, props.contribution.generation],
  () => { open.value = false; },
);
</script>

<template>
  <button
    type="button"
    class="schema-composer-action-trigger"
    data-amitia-ui="component.button"
    data-composer-picker-trigger
    :aria-label="title"
    :title="title"
    aria-haspopup="dialog"
    :aria-expanded="open"
    :data-open="open"
    @click="open = !open"
  >
    <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.7" aria-hidden="true">
      <template v-if="contribution.icon === 'emote'">
        <circle cx="12" cy="12" r="9" />
        <path d="M8 14a4.5 4.5 0 0 0 8 0M8 9h.01M16 9h.01" stroke-linecap="round" />
      </template>
      <template v-else>
        <rect x="4" y="4" width="6" height="6" rx="1" />
        <rect x="14" y="4" width="6" height="6" rx="1" />
        <rect x="4" y="14" width="6" height="6" rx="1" />
        <rect x="14" y="14" width="6" height="6" rx="1" />
      </template>
    </svg>
  </button>
  <ComposerCommandPalette
    v-if="open"
    class="schema-composer-action-panel"
    data-amitia-ui="extension.surface"
    mode="custom"
    :title="title"
    :instance-id="instanceId"
    dismissible
    @dismiss="open = false"
  >
    <SchemaUIRenderer
      :contribution="contribution"
      :context="context"
      :slot-id="slotId"
      :host-actions="hostActions"
      @error="emit('error', $event)"
    />
  </ComposerCommandPalette>
</template>

<style scoped>
.schema-composer-action-trigger {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 32px;
  width: 32px;
  height: 32px;
  padding: 0;
  border: 1px solid transparent;
  border-radius: 8px;
  background: transparent;
  color: var(--ac-color-text-secondary);
  cursor: pointer;
}
.schema-composer-action-trigger:hover,
.schema-composer-action-trigger[data-open="true"] {
  background: var(--ac-color-bg-secondary);
  color: var(--ac-color-text);
}
.schema-composer-action-trigger:focus-visible {
  outline: 2px solid var(--surface-border-focus, currentColor);
  outline-offset: 2px;
}
</style>
