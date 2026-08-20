<script setup lang="ts">
import { computed } from "vue";
import { useRoute } from "vue-router";
import UIProviderHost from "./UIProviderHost.vue";

const route = useRoute();
const providerId = computed(() => String(route.meta.uiProviderId ?? ""));
const capability = computed(() => (String(route.meta.uiCapability ?? "page.provider") as any));
const EmptyFallback = { template: '<div class="ui-provider-route-unavailable">UI provider unavailable</div>' };
</script>

<template>
  <UIProviderHost
    :capability="capability"
    :provider-id="providerId || undefined"
    :fallback="EmptyFallback"
    :context="{ routeParams: route.params, routeQuery: route.query }"
  />
</template>

<style scoped>
.ui-provider-route-unavailable { padding: 24px; color: var(--amitia-text-secondary); }
</style>
