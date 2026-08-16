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
const surfaceRole = computed(() => String((props.context?.surface as Record<string, unknown> | undefined)?.role ?? "main"));
const host = computed(() => String(props.context?.host ?? "web"));
const isAvailable = computed(() => props.contribution.kind !== "desktop_command" || host.value === "desktop");

async function invokeAction(actionId: string) {
  try {
    const res = await apiClient.post(`/api/extensions/ui/action/${props.contribution.contributionId}/${actionId}`, {
      context: props.context ?? {},
    });
    const result = res.data?.result ?? res.data;
    if (result?.clientExecute === true && typeof result.text === "string") {
      try {
        if (window.amitiaDesktop?.writeClipboardText) {
          await window.amitiaDesktop.writeClipboardText(result.text as string);
        } else if (navigator.clipboard) {
          await navigator.clipboard.writeText(result.text as string);
        } else {
          console.error("Clipboard write failed: CLIPBOARD_HOST_UNAVAILABLE");
        }
      } catch (clipErr) {
        console.error("Clipboard write failed:", clipErr);
      }
    }
  } catch (e) {
    console.error("Action invoke failed:", e);
  }
}
</script>

<template>
  <div v-if="isAvailable" class="host-native-action" :class="`host-native-action--${surfaceRole}`" :data-contribution-id="contribution.contributionId">
    <template v-if="contribution.actions && contribution.actions.length > 0">
      <button
        v-for="action in contribution.actions"
        :key="action.actionId"
        class="host-native-action__button"
        :data-action-id="action.actionId"
        :data-risk="action.riskLevel ?? 'low'"
        :aria-label="action.title || title"
        @click="invokeAction(action.actionId)"
      >
        <span v-if="action.icon" class="host-native-action__icon">{{ action.icon }}</span>
        <span class="host-native-action__label">{{ action.title }}</span>
      </button>
    </template>
    <template v-else>
      <button class="host-native-action__button" :aria-label="title" @click="invokeAction('default')">
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
  border: 1px solid var(--amitia-border, var(--amitia-color-border, rgba(127, 127, 127, 0.3)));
  border-radius: var(--amitia-radius-sm, 6px);
  color: var(--amitia-color-text, inherit);
  font-size: 12px;
  cursor: pointer;
  transition: background 0.15s ease;
}
.host-native-action--header .host-native-action__button, .host-native-action--composer .host-native-action__button { width: 32px; height: 32px; justify-content: center; padding: 0; }
.host-native-action--header .host-native-action__label, .host-native-action--composer .host-native-action__label { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }
.host-native-action--status .host-native-action__button { min-height: 24px; padding: 2px 6px; border-color: transparent; color: var(--text-muted); font-size: 11px; }
.host-native-action--message .host-native-action__button { min-height: 28px; padding: 3px 7px; }
.host-native-action__button:focus-visible { outline: 2px solid var(--surface-border-focus); outline-offset: 2px; }
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
