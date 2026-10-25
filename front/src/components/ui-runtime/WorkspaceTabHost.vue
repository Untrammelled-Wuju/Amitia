<script setup lang="ts">
import type { WorkspaceTabDefinition } from "@/ui-runtime/workspaceTabs";
import ExternalWebUIView from "./ExternalWebUIView.vue";
import UIProviderHost from "./UIProviderHost.vue";

defineProps<{
  tab: WorkspaceTabDefinition;
  context: Record<string, unknown>;
}>();

const UnsupportedFallback = { template: '<div class="workspace-tab-unavailable">此工作区 Tab 暂不可用</div>' };
</script>

<template>
  <ExternalWebUIView
    v-if="tab.entry.type === 'external_web_ui'"
    :provider-id="tab.provider.providerId"
    :extension-id="tab.provider.extensionId"
    :title="tab.label"
    :url="tab.entry.url || ''"
    :allowed-origins="tab.entry.allowedOrigins || []"
    :storage-key="`${tab.provider.extensionId}/${tab.provider.providerId}/${tab.tabId}`"
    :persistent="tab.storage === 'persistent'"
  />
  <UIProviderHost
    v-else
    :capability="'workspace.tab'"
    :provider-id="tab.provider.providerId"
    :fallback="UnsupportedFallback"
    :context="context"
    frameless
  />
</template>

<style scoped>
.workspace-tab-unavailable {
  padding: 24px;
  color: var(--text-secondary);
}
</style>
