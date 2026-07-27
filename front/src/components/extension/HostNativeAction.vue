<script setup lang="ts">
import { computed } from "vue";
import type { UIContributionSummary } from "@/stores/extensionUI";
import { apiClient } from "@/composables/useApi";

const props = defineProps<{
  contribution: UIContributionSummary;
  context?: Record<string, unknown>;
  slotId: string;
}>();

const title = computed(() => props.contribution.title || props.contribution.contributionId);
const icon = computed(() => props.contribution.icon ?? "");

async function invokeAction(actionId: string) {
  try {
    await apiClient.post(`/api/extensions/ui/action/${props.contribution.contributionId}/${actionId}`, {
      context: props.context ?? {},
    });
  } catch (e) {
    console.error("Action invoke failed:", e);
  }
}
</script>

<template>
  <div class="host-native-action" :data-contribution-id="contribution.contributionId">
    <template v-if="contribution.actions && contribution.actions.length > 0">
      <button
        v-for="action in contribution.actions"
        :key="action.actionId"
        class="host-native-action__button"
        :data-action-id="action.actionId"
        :data-risk="action.riskLevel ?? 'low'"
        @click="invokeAction(action.actionId)"
      >
        <span v-if="action.icon" class="host-native-action__icon">{{ action.icon }}</span>
        <span class="host-native-action__label">{{ action.title }}</span>
      </button>
    </template>
    <template v-else>
      <button class="host-native-action__button" @click="invokeAction('default')">
        <span v-if="icon" class="host-native-action__icon">{{ icon }}</span>
        <span class="host-native-action__label">{{ title }}</span>
      </button>
    </template>
  </div>
</template>

<style scoped>
.host-native-action {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.host-native-action__button {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 10px;
  background: transparent;
  border: 1px solid var(--amitia-color-border, rgba(127, 127, 127, 0.3));
  border-radius: 6px;
  color: var(--amitia-color-text, inherit);
  font-size: 12px;
  cursor: pointer;
  transition: background 0.15s ease;
}
.host-native-action__button:hover {
  background: var(--amitia-color-surface-elevated, rgba(127, 127, 127, 0.1));
}
.host-native-action__button[data-risk="high"] {
  border-color: rgb(220, 80, 80);
  color: rgb(200, 60, 60);
}
.host-native-action__icon {
  font-size: 14px;
  line-height: 1;
}
.host-native-action__label {
  white-space: nowrap;
}
</style>
