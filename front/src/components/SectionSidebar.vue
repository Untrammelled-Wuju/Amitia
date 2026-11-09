<template>
  <aside class="section-sidebar" data-amitia-ui="app.section-sidebar">
    <header class="section-sidebar-header">
      <h1>{{ title }}</h1>
    </header>

    <div class="section-sidebar-scroll">
      <nav v-if="sectionEntries.length" class="section-entry-list" :aria-label="`${title}功能`">
        <RouterLink
          v-for="entry in sectionEntries"
          :key="entry.route"
          :to="entry.route"
          class="section-entry"
          :class="{ active: isEntryActive(entry.route) }"
          :aria-current="isEntryActive(entry.route) ? 'page' : undefined"
        >
          <strong>{{ entry.label }}</strong>
        </RouterLink>
      </nav>

      <div v-else-if="sectionId === 'game'" class="game-sidebar">
        <button type="button" class="section-action primary" @click="openGameInstall">
          <el-icon><Plus /></el-icon>
          <span>添加游戏插件</span>
        </button>
        <div class="subsection-heading">
          <span>已安装游戏插件</span>
          <button type="button" :disabled="gameLoading" aria-label="刷新游戏插件" title="刷新游戏插件" @click="loadGamePlugins">
            <el-icon><Refresh /></el-icon>
          </button>
        </div>
        <button
          v-for="plugin in gamePlugins"
          :key="plugin.extensionId"
          type="button"
          class="game-plugin-row"
          @click="openGamePlugin(plugin)"
        >
          <span class="game-plugin-mark">{{ plugin.name.slice(0, 1).toUpperCase() }}</span>
          <span class="game-plugin-copy">
            <strong>{{ plugin.name }}</strong>
            <small>{{ plugin.enabled ? "已启用" : "已禁用" }} · v{{ plugin.version }}</small>
          </span>
        </button>
        <div v-if="!gameLoading && gamePlugins.length === 0" class="section-empty">暂无已安装游戏插件</div>
      </div>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { Plus, Refresh } from "@element-plus/icons-vue";
import { appSection, type AppSectionId } from "@/navigation/appSections";
import { useApi } from "@/composables/useApi";
import { useExtensionUIStore } from "@/stores/extensionUI";
import { collectExtensionSectionEntries } from "@/ui-runtime/providerRoutes";

interface GamePlugin {
  extensionId: string;
  pluginId: string;
  name: string;
  version: string;
  enabled: boolean;
}

const props = defineProps<{ sectionId: AppSectionId }>();
const route = useRoute();
const router = useRouter();
const api = useApi();
const extensionUIStore = useExtensionUIStore();
const section = computed(() => appSection(props.sectionId as Exclude<AppSectionId, "home" | "channels" | "account" | "settings">));
const title = computed(() => section.value?.label ?? sectionTitle(props.sectionId));
const sectionEntries = computed(() => [
  ...(section.value?.entries ?? []),
  ...collectExtensionSectionEntries(extensionUIStore, props.sectionId)
    .map((item) => ({ route: item.route, label: item.label })),
]);
const gamePlugins = ref<GamePlugin[]>([]);
const gameLoading = ref(false);

function sectionTitle(id: AppSectionId) {
  if (id === "game") return "游戏模式";
  return "";
}

function isEntryActive(path: string) {
  return route.path === path || route.path.startsWith(`${path}/`);
}

function openGameInstall() {
  void router.push({ path: "/game-center", query: { install: "1" } });
}

function openGamePlugin(plugin: GamePlugin) {
  void router.push({
    path: "/game-center/plugin",
    query: { pluginId: plugin.pluginId, extensionId: plugin.extensionId },
  });
}

async function loadGamePlugins() {
  gameLoading.value = true;
  try {
    const result = await api.get<{ items?: GamePlugin[] }>("/api/game-center/plugins", { page: 1, pageSize: 100 });
    gamePlugins.value = result?.items ?? [];
  } catch (error) {
  console.error("[original-error]", error);
gamePlugins.value = [];
  } finally {
    gameLoading.value = false;
  }
}

watch(
  () => props.sectionId,
  (sectionId) => {
    if (sectionId === "game") void loadGamePlugins();
  },
  { immediate: true },
);
</script>

<style scoped>
.section-sidebar {
  width: var(--ac-workspace-sidebar-width);
  flex: 0 0 var(--ac-workspace-sidebar-width);
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: var(--workbench-bg);
  border-right: 1px solid var(--surface-border);
}
.section-sidebar-header { flex: 0 0 auto; padding: 18px 18px 12px; }
.section-sidebar-header h1 { margin: 0; color: var(--text-primary); font-size: var(--ac-font-size-xl); font-weight: 650; }
.section-sidebar-scroll { min-height: 0; flex: 1; overflow-y: auto; padding: 0 8px 18px; }
.section-entry-list, .game-sidebar { display: grid; align-content: start; gap: 4px; }
.section-entry, .section-action, .game-plugin-row {
  border: 0;
  border-radius: 9px;
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  font: inherit;
  text-align: left;
}
.section-entry { display: flex; align-items: center; min-height: 38px; padding: 8px 12px; text-decoration: none; }
.section-entry strong { color: inherit; font-size: var(--ac-font-size-sm); font-weight: 560; }
.section-entry:hover, .section-entry.active, .section-action:hover, .game-plugin-row:hover {
  background: var(--workbench-sidebar-hover);
  color: var(--text-primary);
}
.section-action { display: flex; align-items: center; gap: 9px; min-height: 38px; padding: 8px 12px; }
.section-action.primary { color: var(--ac-color-primary); }
.subsection-heading { display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 16px 8px 5px; color: var(--text-muted); font-size: var(--ac-font-size-xs); font-weight: 550; }
.subsection-heading button { display: grid; place-items: center; width: 24px; height: 24px; padding: 0; border: 0; border-radius: 6px; background: transparent; color: var(--text-muted); cursor: pointer; }
.subsection-heading button:hover { background: var(--workbench-sidebar-hover); color: var(--text-primary); }
.game-plugin-row { display: flex; align-items: center; gap: 9px; min-height: 46px; padding: 7px 9px; }
.game-plugin-mark { display: grid; place-items: center; width: 30px; height: 30px; flex: 0 0 30px; border-radius: 8px; background: var(--control-hover-bg); color: var(--ac-color-primary); font-size: 12px; font-weight: 700; }
.game-plugin-copy { min-width: 0; display: grid; gap: 2px; }
.game-plugin-copy strong, .game-plugin-copy small { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.game-plugin-copy strong { color: inherit; font-size: var(--ac-font-size-sm); }
.game-plugin-copy small { color: var(--text-muted); font-size: 11px; }
.section-empty { padding: 12px; color: var(--text-muted); font-size: var(--ac-font-size-xs); text-align: center; }
</style>
