<template>
  <SecondaryWorkspace :title="title" :return-to="returnTo" class="account-workspace">
    <template #navigation>
      <div class="account-compact-navigation">
        <label for="space-section">空间分类</label>
        <select id="space-section" :value="activePath" @change="navigateToSection">
          <option v-for="section in sections" :key="section.path" :value="section.path">{{ section.label }}</option>
        </select>
      </div>
      <nav class="account-navigation" :aria-label="`${title}导航`">
        <router-link v-for="section in sections" :key="section.id"
          :to="{ path: section.path, query: route.query }"
          class="account-navigation-item"
          :aria-current="activePath === section.path ? 'page' : undefined">
          {{ section.label }}
        </router-link>
      </nav>
    </template>
    <slot />
  </SecondaryWorkspace>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import SecondaryWorkspace from "./SecondaryWorkspace.vue";
import { resolveSecondaryPage } from "../navigation/secondary-workspace";

defineProps<{ title: string; returnTo: string }>();
const route = useRoute();
const router = useRouter();
const sections = computed(() => resolveSecondaryPage(route.path)?.sections ?? []);
const activePath = computed(() => sections.value.find(section => section.path === route.path)?.path ?? sections.value[0]?.path ?? "");

function navigateToSection(event: Event) {
  const path = (event.target as HTMLSelectElement).value;
  if (sections.value.some(section => section.path === path)) {
    void router.push({ path, query: route.query });
  }
}
</script>

<style scoped>
.account-navigation { display: grid; align-content: start; gap: 4px; overflow-y: auto; padding: 0 16px 24px; }
.account-navigation-item { display: flex; align-items: center; min-height: 34px; padding: 6px 10px; box-sizing: border-box; border-radius: var(--ac-radius-sm); color: var(--text-secondary); font-size: var(--ac-font-size-sm); line-height: 1.4; text-decoration: none; transition: background-color var(--ac-transition-fast), color var(--ac-transition-fast); }
.account-navigation-item:hover { background: var(--control-hover-bg); color: var(--text-primary); }
.account-navigation-item[aria-current="page"] { background: var(--control-active-bg); color: var(--text-primary); box-shadow: inset 2px 0 var(--tp-primary); }
.account-navigation-item:focus-visible, select:focus-visible { outline: 2px solid var(--ac-color-primary); outline-offset: -2px; }
.account-compact-navigation { display: none; }
@media (max-width: 767px) {
  .account-navigation { display: none; }
  .account-compact-navigation { display: flex; align-items: center; gap: 12px; padding: 0 16px 12px; }
  .account-compact-navigation label { flex-shrink: 0; color: var(--text-secondary); font-size: var(--ac-font-size-sm); }
  .account-compact-navigation select { min-width: 0; width: 100%; min-height: 38px; padding: 8px 12px; border: 1px solid var(--surface-border); border-radius: var(--ac-radius-sm); background: var(--surface-bg); color: var(--text-primary); font: inherit; font-size: var(--ac-font-size-base); }
}
</style>
