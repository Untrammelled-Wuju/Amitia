<!-- Root Slot boundary: every replaceable provider surface hangs from this tree. -->
<script setup lang="ts">
import BuiltinAppShell from "./BuiltinAppShell.vue";
import ExtensionSlot from "./extension/ExtensionSlot.vue";
import UIPortalHost from "./extension/UIPortalHost.vue";
import UIRightPaneHost from "./extension/UIRightPaneHost.vue";
import UIProviderHost from "./ui-runtime/UIProviderHost.vue";
import { computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAppClientModules } from "@/composables/useAppClientModules";

const { modules: appClientModules } = useAppClientModules();
const { modules: dshShellModules } = useAppClientModules({
  enabled: true,
  contributionIds: ["dsh-desktop-shell-host"],
});

const route = useRoute();
const router = useRouter();

const dshShellContext = computed(() => {
  const conversationId = String(route.query.conversationId ?? "").trim();
  return conversationId
    ? { scope: "root", sessionId: conversationId, conversationId, conversation: { id: conversationId } }
    : { scope: "root" };
});

const dshShellNavigate = (target: string) => router.push(target);
</script>

<template>
  <div class="app-shell-target" data-amitia-ui="app.shell">
    <UIPortalHost />
    <UIRightPaneHost />
    <ExtensionSlot
      slot-id="app.shell.quota_notice"
      fallback="none"
      layout="stack"
      surface-role="status"
    />
    <ExtensionSlot slot-id="root" fallback="default" layout="stack" surface-role="main" bare>
      <template #default>
        <UIProviderHost capability="app.shell" :fallback="BuiltinAppShell">
          <slot />
        </UIProviderHost>
      </template>
    </ExtensionSlot>
    <ExtensionSlot
      slot-id="app.shell.bottom"
      fallback="none"
      layout="stack"
      surface-role="main"
    />
    <component
      :is="item.component"
      v-for="item in appClientModules"
      :key="`${item.extensionId}:${item.contributionId}`"
      :ui-actions="item.actions"
      :ui-context="{ extensionId: item.extensionId, contributionId: item.contributionId }"
    />
    <component
      :is="item.component"
      v-for="item in dshShellModules"
      :key="`dsh:${item.extensionId}:${item.contributionId}`"
      :actions="item.actions"
      :ui-actions="item.actions"
      :ui-context="dshShellContext"
      :ui-navigate="dshShellNavigate"
    />
  </div>
</template>

<style scoped>
.app-shell-target {
  height: 100%;
}
</style>
