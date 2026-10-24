<template>
  <aside class="account-sidebar" data-amitia-ui="account.sidebar">
    <header class="account-sidebar-header">
      <h1>我的空间</h1>
    </header>
    <nav class="account-navigation" aria-label="我的空间分类">
      <RouterLink
        v-for="item in accountNavigation"
        :key="item.id"
        :to="item.path"
        class="account-navigation-item"
        :aria-current="currentEntry === item.path ? 'page' : undefined"
      >
        <component :is="accountItemIcon(item.id)" class="account-navigation-icon" />
        <span>{{ item.label }}</span>
      </RouterLink>
    </nav>
  </aside>
</template>

<script setup lang="ts">
import { computed, type Component } from "vue";
import { useRoute } from "vue-router";
import { Connection, Key, Monitor, User } from "@element-plus/icons-vue";
import { accountNavigation, resolveAccountEntry } from "@/navigation/account-navigation";

const route = useRoute();
const currentEntry = computed(() => resolveAccountEntry(route.path));
const icons: Record<string, Component> = {
  profile: User,
  identity: Key,
  runtime: Monitor,
  devices: Connection,
};

function accountItemIcon(id: string) {
  return icons[id] ?? User;
}
</script>

<style scoped>
.account-sidebar {
  width: var(--ac-workspace-sidebar-width);
  flex: 0 0 var(--ac-workspace-sidebar-width);
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: color-mix(in srgb, var(--workbench-bg) 90%, var(--surface-bg-elevated));
  border-right: 1px solid var(--surface-border);
}
.account-sidebar-header { flex: 0 0 auto; padding: 22px 20px 14px; }
.account-sidebar-header h1 { margin: 0; color: var(--text-primary); font-size: var(--ac-font-size-xl); font-weight: 650; }
.account-navigation { min-height: 0; flex: 1; display: grid; align-content: start; gap: 5px; overflow-y: auto; padding: 0 12px 24px; }
.account-navigation-item { display: flex; align-items: center; gap: 10px; min-height: 42px; padding: 8px 12px; box-sizing: border-box; border-radius: 11px; color: var(--text-secondary); font-size: var(--ac-font-size-sm); line-height: 1.4; text-decoration: none; transition: background-color var(--ac-transition-fast), color var(--ac-transition-fast); }
.account-navigation-icon { width: 18px; flex: 0 0 18px; font-size: 18px; }
.account-navigation-item:hover { background: var(--control-hover-bg); color: var(--text-primary); }
.account-navigation-item[aria-current="page"] { background: var(--control-active-bg); color: var(--text-primary); }
.account-navigation-item:focus-visible { outline: 2px solid var(--ac-color-primary); outline-offset: -2px; }
</style>
