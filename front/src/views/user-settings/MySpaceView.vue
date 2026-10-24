<template>
  <div class="my-space-page">
    <div class="my-space-compact-navigation">
      <label for="space-section-mobile">空间分类</label>
      <select id="space-section-mobile" :value="currentEntry" @change="navigateToSection">
        <option value="" disabled>选择空间功能</option>
        <option v-for="item in accountNavigation" :key="item.path" :value="item.path">{{ item.label }}</option>
      </select>
    </div>
    <div class="my-space-route-content">
      <router-view />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import { accountNavigation, resolveAccountEntry } from "@/navigation/account-navigation";

const route = useRoute();
const router = useRouter();
const currentEntry = computed(() => resolveAccountEntry(route.path));

function navigateToSection(event: Event) {
  const path = (event.target as HTMLSelectElement).value;
  if (accountNavigation.some(item => item.path === path)) void router.push(path);
}
</script>

<style scoped>
.my-space-page { min-width: 0; width: 100%; }
.my-space-route-content { min-width: 0; width: 100%; }
.my-space-compact-navigation { display: none; }
@media (max-width: 767px) {
  .my-space-compact-navigation { display: flex; align-items: center; gap: 12px; margin-bottom: 16px; }
  .my-space-compact-navigation label { flex-shrink: 0; color: var(--text-secondary); font-size: var(--ac-font-size-sm); }
  .my-space-compact-navigation select { min-width: 0; width: 100%; min-height: 40px; padding: 8px 12px; border: 0; border-radius: 11px; background: var(--surface-bg-elevated); color: var(--text-primary); font: inherit; font-size: var(--ac-font-size-base); }
}
</style>
