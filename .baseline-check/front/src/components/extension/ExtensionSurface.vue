<script setup lang="ts">
import type { ExtensionSurfaceRole } from "@/composables/useExtensionSlot";

withDefaults(defineProps<{
  role: ExtensionSurfaceRole;
  bordered?: boolean;
  scrollMode?: "none" | "auto";
}>(), { bordered: true, scrollMode: "none" });
</script>

<template>
  <section class="extension-surface" :class="[`extension-surface--${role}`, { 'is-bordered': bordered, 'is-scrollable': scrollMode === 'auto' }]">
    <slot />
  </section>
</template>

<style scoped>
.extension-surface { min-width: 0; min-height: 0; max-width: 100%; background: var(--plugin-surface-bg); border-radius: var(--radius-sm); }
.extension-surface.is-bordered { border: 1px solid var(--plugin-surface-border); }
.extension-surface.is-scrollable { overflow: auto; }
.extension-surface--header, .extension-surface--composer, .extension-surface--status { border: 0; background: transparent; }
.extension-surface--message { overflow: hidden; }
.extension-surface--sidebar, .extension-surface--main { height: 100%; flex: 1; min-height: 0; }
</style>
