<script setup lang="ts">
import { computed } from "vue";
import { useRoute } from "vue-router";
import UIProviderHost from "./UIProviderHost.vue";
import ChannelPluginWorkspace from "./ChannelPluginWorkspace.vue";
import { buildProviderRouteContext } from "@/ui-runtime/routeContext";

const route = useRoute();
const providerId = computed(() => String(route.meta.uiProviderId ?? ""));
const capability = computed(() => (String(route.meta.uiCapability ?? "page.provider") as any));
const context = computed(() => buildProviderRouteContext(
  route.fullPath,
  route.params,
  route.query,
));
const isChannelPlugin = computed(() => route.path.startsWith("/channels/"));
const EmptyFallback = { template: '<div class="ui-provider-route-unavailable">UI provider unavailable</div>' };
</script>

<template>
  <ChannelPluginWorkspace
    v-if="isChannelPlugin"
    :provider-id="providerId"
    :capability="capability"
    :context="context"
  />
  <UIProviderHost
    v-else
    :capability="capability"
    :provider-id="providerId || undefined"
    :fallback="EmptyFallback"
    :context="context"
  />
</template>

<style scoped>
.ui-provider-route-unavailable { padding: 24px; color: var(--amitia-text-secondary); }
</style>
