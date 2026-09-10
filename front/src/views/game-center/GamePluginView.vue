<template>
  <div class="game-plugin-page" v-loading="loading">
    <ExtensionPageHeader
      :title="detail?.name || '游戏控制台'"
      :description="detail?.description || '由游戏扩展提供的动态控制界面。'"
      parent-title="游戏模式"
      parent-path="/game-center"
    >
      <template v-if="detail" #meta>
        <div class="game-meta">
          <el-tag :type="detail.enabled ? 'success' : 'info'">
            {{ detail.enabled ? '已启用' : '已禁用' }}
          </el-tag>
          <el-tag v-if="detail.healthSummary?.status" :type="healthTagType(detail.healthSummary.status)">
            {{ healthLabel(detail.healthSummary.status) }}
          </el-tag>
          <span>v{{ detail.version }}</span>
        </div>
      </template>
      <template #actions>
        <el-button :icon="Refresh" :loading="refreshing" @click="refresh">
          刷新
        </el-button>
      </template>
    </ExtensionPageHeader>

    <el-alert
      v-if="error"
      class="page-alert"
      :title="error"
      type="error"
      show-icon
      :closable="false"
    >
      <template #default>
        <el-button link type="primary" @click="refresh">重新加载</el-button>
      </template>
    </el-alert>

    <section v-if="extensionId && pluginId" class="game-surface-shell">
      <ExtensionSlot
        slot-id="extension.detail.tab"
        :extension-id="extensionId"
        :context="gameSurfaceContext"
        fallback="default"
        layout="stack"
        surface-role="main"
        bare
      >
        <template #default>
          <div class="surface-fallback">
            <el-empty description="该游戏扩展暂未提供专属控制界面">
              <el-button type="primary" @click="goBack">返回游戏模式</el-button>
            </el-empty>
          </div>
        </template>
      </ExtensionSlot>
    </section>

    <el-empty
      v-else-if="!loading"
      class="invalid-route"
      description="缺少游戏扩展标识，无法打开专属页面"
    >
      <el-button type="primary" @click="goBack">返回游戏模式</el-button>
    </el-empty>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { Refresh } from "@element-plus/icons-vue";
import ExtensionSlot from "@/components/extension/ExtensionSlot.vue";
import { useApi } from "@/composables/useApi";
import { useExtensionUIStore } from "@/stores/extensionUI";
import ExtensionPageHeader from "@/views/extensions/components/ExtensionPageHeader.vue";

type GamePluginDetail = {
  extensionId: string;
  pluginId: string;
  name: string;
  version: string;
  description?: string;
  enabled: boolean;
  healthSummary?: { status?: string };
  installState?: string;
  capabilities?: string[];
  runtimes?: Array<Record<string, unknown>>;
  [key: string]: unknown;
};

const api = useApi();
const route = useRoute();
const router = useRouter();
const uiStore = useExtensionUIStore();
const loading = ref(false);
const refreshing = ref(false);
const error = ref("");
const detail = ref<GamePluginDetail | null>(null);

function queryText(value: unknown): string {
  if (Array.isArray(value)) return String(value[0] ?? "").trim();
  return String(value ?? "").trim();
}

const extensionId = computed(() => queryText(route.query.extensionId));
const pluginId = computed(() => queryText(route.query.pluginId));

const gameSurfaceContext = computed<Record<string, unknown>>(() => ({
  extensionId: extensionId.value,
  pluginId: pluginId.value,
  extension: detail.value,
  gamePlugin: detail.value,
  managementTarget: "game-center",
  surface: "game-detail",
  surfaceRole: "main",
  capabilities: detail.value?.capabilities ?? [],
}));

function healthTagType(health: string): "success" | "warning" | "danger" | "info" {
  switch (health.toLowerCase()) {
    case "healthy":
      return "success";
    case "degraded":
      return "warning";
    case "unhealthy":
    case "failed":
      return "danger";
    default:
      return "info";
  }
}

function healthLabel(health: string): string {
  switch (health.toLowerCase()) {
    case "healthy":
      return "正常";
    case "degraded":
      return "异常";
    case "unhealthy":
      return "故障";
    default:
      return health || "未知";
  }
}

async function load(forceUI = false) {
  if (!extensionId.value || !pluginId.value) {
    detail.value = null;
    error.value = "";
    return;
  }
  loading.value = true;
  error.value = "";
  try {
    const [pluginDetail] = await Promise.all([
      api.get<GamePluginDetail>(
        `/api/game-center/plugins/detail?pluginId=${encodeURIComponent(pluginId.value)}&extensionId=${encodeURIComponent(extensionId.value)}`,
      ),
      uiStore.refreshSnapshot(forceUI),
    ]);
    detail.value = pluginDetail;
  } catch (err: any) {
    error.value = err?.message || "加载游戏专属页面失败";
  } finally {
    loading.value = false;
  }
}

async function refresh() {
  refreshing.value = true;
  try {
    await load(true);
  } finally {
    refreshing.value = false;
  }
}

function goBack() {
  void router.push("/game-center");
}

watch([extensionId, pluginId], () => {
  void load(false);
});

onMounted(() => {
  void load(false);
});
</script>

<style scoped>
.game-plugin-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
  min-width: 0;
}

.game-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  color: var(--ac-color-text-muted);
}

.page-alert {
  margin: 0;
}

.game-surface-shell {
  min-width: 0;
}

.game-surface-shell :deep(.extension-slot),
.game-surface-shell :deep(.extension-slot__contribution) {
  min-width: 0;
  width: 100%;
}

.surface-fallback {
  width: 100%;
  padding: 48px 16px;
  border: 1px solid var(--ac-color-border);
  border-radius: 12px;
  background: var(--ac-color-surface);
}

.invalid-route {
  padding: 56px 16px;
}

@media (max-width: 720px) {
  .game-plugin-page {
    padding: 16px;
  }
}
</style>
