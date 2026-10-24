<template>
  <aside class="channel-sidebar" data-amitia-ui="channels.sidebar">
    <header class="channel-sidebar-header">
      <h1>渠道接入</h1>
    </header>
    <nav class="channel-navigation" aria-label="渠道接入功能">
      <RouterLink
        v-for="item in pluginItems"
        :key="item.id"
        :to="item.route"
        class="channel-navigation-item"
        :aria-current="isActive(item.route) ? 'page' : undefined"
      >
        {{ item.label }}
      </RouterLink>
      <div v-if="pluginItems.length === 0" class="channel-navigation-empty">暂无渠道插件页面</div>
    </nav>
  </aside>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useRoute } from "vue-router";
import { useExtensionUIStore } from "@/stores/extensionUI";
import { collectChannelPluginNavigationItems } from "@/ui-runtime/navigationRegistry";

const route = useRoute();
const store = useExtensionUIStore();
const pluginItems = computed(() => collectChannelPluginNavigationItems(store));

function isActive(path: string) {
  return route.path === path || route.path.startsWith(`${path}/`);
}
</script>

<style scoped>
.channel-sidebar {
  width: var(--ac-workspace-sidebar-width);
  flex: 0 0 var(--ac-workspace-sidebar-width);
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: var(--workbench-bg);
  border-right: 1px solid var(--surface-border);
}
.channel-sidebar-header { flex: 0 0 auto; padding: 18px 18px 12px; }
.channel-sidebar-header h1 { margin: 0; color: var(--text-primary); font-size: var(--ac-font-size-xl); font-weight: 650; }
.channel-navigation { min-height: 0; flex: 1; display: grid; align-content: start; gap: 2px; overflow-y: auto; padding: 0 12px 24px; }
.channel-navigation-item { display: flex; align-items: center; min-height: 34px; padding: 6px 10px; box-sizing: border-box; border-radius: var(--ac-radius-sm); color: var(--text-secondary); font-size: var(--ac-font-size-sm); line-height: 1.4; text-decoration: none; transition: background-color var(--ac-transition-fast), color var(--ac-transition-fast); }
.channel-navigation-item:hover { background: var(--control-hover-bg); color: var(--text-primary); }
.channel-navigation-item[aria-current="page"] { background: var(--control-active-bg); color: var(--text-primary); }
.channel-navigation-item:focus-visible { outline: 2px solid var(--ac-color-primary); outline-offset: -2px; }
.channel-navigation-empty { padding: 12px 10px; color: var(--text-muted); font-size: var(--ac-font-size-xs); }
</style>
