<script setup lang="ts">
import { computed, defineAsyncComponent } from "vue";
import type { UIContributionSummary } from "@/stores/extensionUI";
import ExtensionSurface from "./ExtensionSurface.vue";
import PluginPageFrame from "./PluginPageFrame.vue";
import ExtensionLoadingOverlay from "./ExtensionLoadingOverlay.vue";
import { resolvePluginSurface } from "@/ui-runtime/pluginSurfaceResolver";

const props = defineProps<{
  contribution: UIContributionSummary;
  context: Record<string, unknown>;
  slotId: string;
  hostActions?: Record<string, (input?: unknown) => unknown | Promise<unknown>>;
  frameless?: boolean;
}>();
const emit = defineEmits<{ (e: "error", error: string): void }>();

const isHostRuntime = computed(() => props.contribution.runtimeId?.trim().startsWith("host.") ?? false);
const surfaceKind = computed(() => resolvePluginSurface(props.contribution));

const renderer = computed(() => {
  switch (surfaceKind.value) {
    case "host_runtime":
      return defineAsyncComponent(() => import("./HostRuntimeContribution.vue"));
    case "web_composer":
      return defineAsyncComponent(() => import("./WebComposerActionProxy.vue"));
    case "schema":
      if (props.contribution.kind === "composer_action" && props.slotId === "chat.composer.action") {
        return defineAsyncComponent(() => import("./SchemaComposerActionProxy.vue"));
      }
      return defineAsyncComponent({ loader: () => import("./SchemaUIRenderer.vue"), loadingComponent: ExtensionLoadingOverlay, delay: 0 });
    case "web":
      return defineAsyncComponent({ loader: () => import("./SandboxWebUIFrame.vue"), loadingComponent: ExtensionLoadingOverlay, delay: 0 });
    case "native":
      return defineAsyncComponent(() => import("./HostNativeAction.vue"));
    case "none":
      return null;
  }
});
const surfaceRole = computed(() => String((props.context.surface as Record<string, unknown> | undefined)?.role ?? "main"));
const needsSurface = computed(() => !isHostRuntime.value && !(props.contribution.kind === "composer_action" && props.slotId === "chat.composer.action") && (props.contribution.sandbox !== "host_native" || ["schema_page", "settings_section", "panel", "card", "web_page", "message_renderer"].includes(props.contribution.kind)));
const target = computed(() => props.contribution.kind === "card" ? "extension.card" : "extension.surface");
const isPage = computed(() => ["schema_page", "web_page"].includes(props.contribution.kind));
</script>

<template>
  <component
    :is="renderer"
    v-if="renderer && isPage && frameless"
    :contribution="contribution"
    :context="context"
    :slot-id="slotId"
    :host-actions="hostActions"
    @error="(error: string) => emit('error', error)"
  />
  <PluginPageFrame v-else-if="renderer && isPage" :title="contribution.title">
    <component :is="renderer" :contribution="contribution" :context="context" :slot-id="slotId" :host-actions="hostActions" @error="(error: string) => emit('error', error)" />
  </PluginPageFrame>
  <ExtensionSurface v-else-if="renderer && needsSurface" :role="surfaceRole as any" :bordered="surfaceRole !== 'message' && surfaceRole !== 'composer'" :target="target">
    <component :is="renderer" :contribution="contribution" :context="context" :slot-id="slotId" :host-actions="hostActions" @error="(error: string) => emit('error', error)" />
  </ExtensionSurface>
  <component :is="renderer" v-else-if="renderer" :contribution="contribution" :context="context" :slot-id="slotId" :host-actions="hostActions" @error="(error: string) => emit('error', error)" />
</template>
