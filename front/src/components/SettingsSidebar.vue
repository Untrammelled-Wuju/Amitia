<template>
  <aside class="settings-sidebar" data-amitia-ui="settings.sidebar">
    <header class="settings-sidebar-header">
      <h1>设置</h1>
    </header>
    <nav class="settings-navigation" aria-label="设置分类">
      <section
        v-for="(group, index) in settingsGroups"
        :key="group.title"
        class="settings-navigation-group"
        :aria-labelledby="`settings-group-${index}`"
      >
        <h2 :id="`settings-group-${index}`">{{ group.title }}</h2>
        <RouterLink
          v-for="item in group.items"
          :key="item.path"
          :to="item.path"
          class="settings-navigation-item"
          data-amitia-ui="settings.tab"
          :aria-current="currentEntry === item.path ? 'page' : undefined"
        >
          {{ item.label }}
        </RouterLink>
      </section>
    </nav>
  </aside>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useRoute } from "vue-router";
import { resolveSettingsEntry, settingsGroups } from "@/navigation/settings-navigation";

const route = useRoute();
const currentEntry = computed(() => resolveSettingsEntry(route.path));
</script>

<style scoped>
.settings-sidebar {
  width: var(--ac-workspace-sidebar-width);
  flex: 0 0 var(--ac-workspace-sidebar-width);
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: var(--workbench-bg);
  border-right: 1px solid var(--surface-border);
}
.settings-sidebar-header { flex: 0 0 auto; padding: 18px 18px 12px; }
.settings-sidebar-header h1 { margin: 0; color: var(--text-primary); font-size: var(--ac-font-size-xl); font-weight: 650; }
.settings-navigation { min-height: 0; flex: 1; display: grid; align-content: start; gap: 18px; overflow-y: auto; padding: 0 12px 24px; }
.settings-navigation-group { display: grid; gap: 2px; }
.settings-navigation-group h2 { margin: 0 8px 4px; color: var(--text-muted); font-size: var(--ac-font-size-xs); font-weight: 500; line-height: 1.4; }
.settings-navigation-item { display: flex; align-items: center; min-height: 34px; padding: 6px 10px; box-sizing: border-box; border-radius: var(--ac-radius-sm); color: var(--text-secondary); font-size: var(--ac-font-size-sm); line-height: 1.4; text-decoration: none; transition: background-color var(--ac-transition-fast), color var(--ac-transition-fast); }
.settings-navigation-item:hover { background: var(--control-hover-bg); color: var(--text-primary); }
.settings-navigation-item[aria-current="page"] { background: var(--control-active-bg); color: var(--text-primary); }
.settings-navigation-item:focus-visible { outline: 2px solid var(--ac-color-primary); outline-offset: -2px; }
</style>
