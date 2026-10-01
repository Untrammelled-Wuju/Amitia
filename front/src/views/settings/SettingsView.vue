<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="settings-page">
    <ExtensionSlot
      slot-id="system.status.item"
      :context="settingsContext"
      fallback="none"
      layout="inline"
      surface-role="status"
      class="settings-status-slot"
    />
    <router-view />
    <ExtensionSlot
      v-for="slotId in settingsSlots"
      :key="slotId"
      :slot-id="slotId"
      :context="settingsContext"
      fallback="none"
      layout="stack"
      surface-role="main"
    />
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useRoute } from "vue-router";
import ExtensionSlot from "@/components/extension/ExtensionSlot.vue";

const route = useRoute();
const settingsSlots = [
  "system.settings.section",
  "extension.settings.section",
  "extension.settings.page",
];
const settingsContext = computed(() => ({
  route: route.fullPath,
  routeName: String(route.name ?? ""),
  section: String(route.path.split("/").filter(Boolean).at(-1) ?? "settings"),
}));
</script>

<style scoped>
.settings-page { min-width: 0; }
.settings-status-slot { margin-bottom: 12px; }
</style>
