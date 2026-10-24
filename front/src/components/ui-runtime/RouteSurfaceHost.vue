<script setup lang="ts">
import { computed, onMounted, type Component } from "vue";
import { useRoute } from "vue-router";
import UIProviderHost from "./UIProviderHost.vue";
import type { UIProviderCapability } from "@/ui-runtime/types";
import { useExtensionUIStore } from "@/stores/extensionUI";
import { resolvePageProvider } from "@/ui-runtime/pageProviderRegistry";
import { canonicalUISurfaceId, uiRouteAliases } from "@/ui-runtime/uiSurfaceCatalog";
import { buildProviderRouteContext } from "@/ui-runtime/routeContext";

const props = defineProps<{ fallback: Component }>();
const route = useRoute();
const store = useExtensionUIStore();

const capability = computed<UIProviderCapability | null>(() => {
  const path = route.path;
  if (route.meta.extensionRoute === true) return null;
  if (path === "/settings/ui-providers") return null;
  if (path === "/chat") return null; // WebChatView owns the conversation boundary.
  if (path === "/character") return "character.shell";
  if (path.startsWith("/character/")) return "character.detail";
  if (path === "/memory" || path === "/memory-manager") return "memory.shell";
  if (path.startsWith("/memory-") || path.startsWith("/memory/")) return "memory.detail";
  if (path === "/settings") return "settings.shell";
  if (path.startsWith("/settings/")) return "settings.section";
  if (path === "/extensions") return "extension.center";
  if (path.startsWith("/extensions/") || path.startsWith("/extension/page/")) return "extension.page";
  return "page.provider";
});

const providerId = computed(() => {
  if (!capability.value) return undefined;
  const selected = resolvePageProvider(
    store.getProviders(capability.value),
    store.getResolvedProvider(capability.value),
    capability.value,
    route.path,
    store.snapshot?.providerContext,
  );
  return selected?.providerId;
});

const providerContext = computed(() => buildProviderRouteContext(
  route.fullPath,
  route.params,
  route.query,
  {
    surfaceId: canonicalUISurfaceId(route.path),
    routeAliases: uiRouteAliases(route.path),
  },
));

onMounted(() => { if (!store.snapshot) void store.refreshSnapshot(); });
</script>

<template>
  <UIProviderHost
    v-if="capability"
    :capability="capability"
    :provider-id="providerId"
    :fallback="fallback"
    :context="providerContext"
  />
  <component :is="fallback" v-else />
</template>
