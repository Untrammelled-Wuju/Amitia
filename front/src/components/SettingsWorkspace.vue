<template>
  <SecondaryWorkspace :title="title" :return-to="returnTo">
    <template #navigation>
      <div class="settings-compact-navigation">
        <label for="settings-section">设置分类</label>
        <select id="settings-section" :value="currentEntry" @change="navigateToSetting">
          <option value="" disabled>选择设置项目</option>
          <optgroup v-for="group in settingsGroups" :key="group.title" :label="group.title">
            <option v-for="item in group.items" :key="item.path" :value="item.path">{{ item.label }}</option>
          </optgroup>
        </select>
      </div>
      <nav class="settings-navigation" aria-label="设置分类">
        <section v-for="(group, index) in settingsGroups" :key="group.title" class="settings-navigation-group" :aria-labelledby="`settings-group-${index}`">
          <h2 :id="`settings-group-${index}`">{{ group.title }}</h2>
          <router-link v-for="item in group.items" :key="item.path" :to="item.path"
            class="settings-navigation-item" :class="{ 'settings-navigation-item-active': currentEntry === item.path }"
            :aria-current="currentEntry === item.path ? 'page' : undefined">
            {{ item.label }}
          </router-link>
        </section>
      </nav>
    </template>
    <slot />
  </SecondaryWorkspace>
</template>

<script setup lang="ts">
import { computed } from "vue";
import SecondaryWorkspace from "./SecondaryWorkspace.vue";
import { useRoute, useRouter } from "vue-router";
import { resolveSettingsEntry, settingsGroups } from "../navigation/settings-navigation";

defineProps<{ returnTo: string; title: string }>();
const route = useRoute();
const router = useRouter();
const currentEntry = computed(() => resolveSettingsEntry(route.path));

function navigateToSetting(event: Event) {
  const path = (event.target as HTMLSelectElement).value;
  if (settingsGroups.some(group => group.items.some(item => item.path === path))) {
    void router.push(path);
  }
}
</script>

<style scoped>
.settings-navigation { display: grid; align-content: start; gap: 18px; overflow-y: auto; padding: 0 16px 24px; }
.settings-navigation-group { display: grid; gap: 2px; }
.settings-navigation-group h2 { margin: 0 10px 4px; color: var(--text-muted); font-size: var(--ac-font-size-xs); font-weight: 500; line-height: 1.4; }
.settings-navigation-item { display: flex; align-items: center; min-height: 34px; padding: 6px 10px; box-sizing: border-box; border-radius: var(--ac-radius-sm); color: var(--text-secondary); font-size: var(--ac-font-size-sm); line-height: 1.4; text-decoration: none; transition: background-color var(--ac-transition-fast), color var(--ac-transition-fast); }
.settings-navigation-item:hover { background: var(--control-hover-bg); color: var(--text-primary); }
.settings-navigation-item-active { background: var(--control-active-bg); color: var(--text-primary); font-weight: 600; }
.settings-navigation-item:active { background: var(--control-active-bg); transition: none; }
.settings-navigation-item:focus-visible, select:focus-visible { outline: 2px solid var(--ac-color-primary); outline-offset: -2px; }
.settings-compact-navigation { display: none; }
@media (max-width: 767px) {
  .settings-navigation { display: none; }
  .settings-compact-navigation { display: flex; align-items: center; gap: 12px; padding: 0 16px 12px; }
  .settings-compact-navigation label { flex-shrink: 0; color: var(--text-secondary); font-size: var(--ac-font-size-sm); }
  .settings-compact-navigation select { min-width: 0; width: 100%; min-height: 38px; padding: 8px 12px; border: 1px solid var(--surface-border); border-radius: var(--ac-radius-sm); background: var(--surface-bg); color: var(--text-primary); font: inherit; font-size: var(--ac-font-size-base); }
}
</style>
