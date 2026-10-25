<template>
  <aside class="plugin-workspace-sidebar" data-amitia-ui="app.workspace-sidebar">
    <header class="plugin-workspace-sidebar__header">
      <h1>{{ title || "插件" }}</h1>
    </header>
    <nav class="plugin-workspace-sidebar__nav" :aria-label="`${title || '插件'}功能`">
      <RouterLink
        v-for="item in items"
        :key="item.route + item.label"
        :to="item.route"
        class="plugin-workspace-sidebar__item"
        :aria-current="isActive(item.route) ? 'page' : undefined"
      >
        {{ item.label }}
      </RouterLink>
      <div v-if="items.length === 0" class="plugin-workspace-sidebar__empty">暂无页面</div>
    </nav>
  </aside>
</template>

<script setup lang="ts">
import { useRoute, useRouter } from "vue-router";
import type { UINavigationSidebarItem } from "@/ui-runtime/navigationRegistry";

withDefaults(defineProps<{
  title?: string;
  items?: UINavigationSidebarItem[];
}>(), {
  title: "",
  items: () => [],
});

const route = useRoute();
const router = useRouter();

function isActive(target: string) {
  return route.fullPath === router.resolve(target).fullPath;
}
</script>

<style scoped>
.plugin-workspace-sidebar {
  width: var(--ac-workspace-sidebar-width);
  flex: 0 0 var(--ac-workspace-sidebar-width);
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: var(--workbench-bg);
  border-right: 1px solid var(--surface-border);
}

.plugin-workspace-sidebar__header {
  flex: 0 0 auto;
  padding: 18px 18px 12px;
}

.plugin-workspace-sidebar__header h1 {
  margin: 0;
  color: var(--text-primary);
  font-size: var(--ac-font-size-xl);
  font-weight: 650;
}

.plugin-workspace-sidebar__nav {
  min-height: 0;
  flex: 1;
  display: grid;
  align-content: start;
  gap: 4px;
  overflow-y: auto;
  padding: 0 8px 18px;
}

.plugin-workspace-sidebar__item {
  display: flex;
  align-items: center;
  min-height: 38px;
  padding: 8px 12px;
  border-radius: 9px;
  color: var(--text-secondary);
  font-size: var(--ac-font-size-sm);
  font-weight: 560;
  text-decoration: none;
}

.plugin-workspace-sidebar__item:hover,
.plugin-workspace-sidebar__item[aria-current="page"] {
  background: var(--workbench-sidebar-hover);
  color: var(--text-primary);
}

.plugin-workspace-sidebar__empty {
  padding: 12px;
  color: var(--text-muted);
  font-size: var(--ac-font-size-xs);
  text-align: center;
}
</style>
