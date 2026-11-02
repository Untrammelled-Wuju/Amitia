<template>
  <div class="extension-center">
    <ExtensionPageHeader
      title="扩展中心"
      description="统一查看扩展安装、更新、权限风险和运行状态"
    >
      <template #actions>
        <el-button :loading="loading" @click="load">
          <el-icon><Refresh /></el-icon>
          刷新
        </el-button>
        <el-button type="primary" @click="router.push('/extensions/packages')">
          <el-icon><Box /></el-icon>
          扩展包
        </el-button>
      </template>
    </ExtensionPageHeader>

    <div class="summary-grid">
      <div class="summary-item">
        <span>已安装</span>
        <strong>{{ installedCount }}</strong>
      </div>
      <div class="summary-item">
        <span>可更新</span>
        <strong>{{ view?.updates.length ?? 0 }}</strong>
      </div>
      <div class="summary-item">
        <span>需要处理</span>
        <strong>{{ view?.needsAction.length ?? 0 }}</strong>
      </div>
      <div class="summary-item">
        <span>开发与本地导入</span>
        <strong>{{ specialCount }}</strong>
      </div>
    </div>

    <el-alert
      v-if="view?.partial"
      type="warning"
      show-icon
      :closable="false"
      title="扩展状态未完全加载"
      class="partial-alert"
    >
      <div class="diagnostic-list">
        <div v-for="item in view.errors || []" :key="`${item.code}-${item.message}`">
          <strong>{{ item.code }}</strong>
          <span>{{ item.message }}</span>
        </div>
      </div>
    </el-alert>

    <div class="entry-grid">
      <router-link
        v-for="entry in entries"
        :key="entry.to"
        :to="entry.to"
        class="entry-card"
      >
        <el-icon><component :is="entry.icon" /></el-icon>
        <div>
          <h2>{{ entry.title }}</h2>
          <p>{{ entry.description }}</p>
        </div>
        <el-icon><ArrowRight /></el-icon>
      </router-link>
    </div>

    <div class="toolbar">
      <el-input
        v-model="search"
        clearable
        placeholder="搜索名称、扩展 ID、发布者或来源"
        class="search-input"
      >
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>
      <span class="updated-at">数据时间：{{ formatDate(view?.generatedAt) }}</span>
    </div>

    <el-alert
      v-if="loadError"
      type="error"
      show-icon
      :closable="false"
      :title="loadError"
      class="load-error"
    />

    <el-skeleton v-if="loading && !view" :rows="8" animated />

    <template v-else-if="view">
      <ExtensionCenterGroup
        title="可更新"
        :cards="filtered(view.updates)"
        empty-text="暂无可用更新"
        @detail="openDetail"
        @toggle="toggleExtension"
        @action="handleAction"
      />
      <ExtensionCenterGroup
        title="需要处理"
        :cards="filtered(view.needsAction)"
        empty-text="没有需要处理的扩展"
        @detail="openDetail"
        @toggle="toggleExtension"
        @action="handleAction"
      />
      <ExtensionCenterGroup
        title="已安装"
        :cards="filtered(view.installed)"
        empty-text="暂无已安装扩展"
        @detail="openDetail"
        @toggle="toggleExtension"
        @action="handleAction"
      />
      <ExtensionCenterGroup
        title="本地导入"
        :cards="filtered(view.localImports)"
        empty-text="暂无本地导入扩展"
        @detail="openDetail"
        @toggle="toggleExtension"
        @action="handleAction"
      />
      <ExtensionCenterGroup
        title="开发模式"
        :cards="filtered(view.dev)"
        empty-text="暂无开发模式扩展"
        @detail="openDetail"
        @toggle="toggleExtension"
        @action="handleAction"
      />
      <ExtensionCenterGroup
        title="发现"
        :cards="filtered(view.discover)"
        empty-text="暂无可发现扩展"
        :show-toggle="false"
        @detail="openDetail"
        @action="handleAction"
      />
    </template>

    <el-dialog v-model="detailVisible" title="扩展详情" width="900px" top="5vh">
      <el-skeleton v-if="detailLoading" :rows="8" animated />
      <div v-else-if="detailError" class="detail-error">
        <el-alert type="error" :closable="false" show-icon :title="detailError" />
      </div>
      <div v-else-if="detail" class="detail-content">
        <div class="detail-heading">
          <div>
            <h2>{{ detail.displayName }}</h2>
            <p>{{ detail.extensionId }} · v{{ detail.version }}</p>
          </div>
          <el-tag :type="detail.enabled ? 'success' : 'info'">
            {{ detail.enabled ? "已启用" : "已停用" }}
          </el-tag>
        </div>
        <el-descriptions :column="2" border>
          <el-descriptions-item label="发布者">{{ detail.publisher || "-" }}</el-descriptions-item>
          <el-descriptions-item label="信任级别">{{ detail.trust || "-" }}</el-descriptions-item>
          <el-descriptions-item label="签名状态">{{ detail.signatureStatus || "-" }}</el-descriptions-item>
          <el-descriptions-item label="权限风险">{{ detail.permissionRisk || "-" }}</el-descriptions-item>
          <el-descriptions-item label="来源">{{ detail.source || "-" }}</el-descriptions-item>
          <el-descriptions-item label="运行状态">{{ detail.effectiveState || "-" }}</el-descriptions-item>
          <el-descriptions-item label="最新版本">{{ detail.latestVersion || detail.version }}</el-descriptions-item>
          <el-descriptions-item label="更新时间">{{ formatDate(detail.updatedAt) }}</el-descriptions-item>
        </el-descriptions>
        <el-alert
          v-if="detail.updateError"
          type="warning"
          show-icon
          :closable="false"
          :title="detail.updateError"
          class="detail-update-error"
        />
        <section class="detail-section">
          <h3>模块</h3>
          <el-table :data="detail.modules" size="small" border>
            <el-table-column prop="moduleId" label="模块 ID" min-width="180" />
            <el-table-column prop="kind" label="类型" width="120" />
            <el-table-column prop="runtime" label="运行时" width="130" />
            <el-table-column prop="status" label="状态" width="120" />
          </el-table>
        </section>
        <section class="detail-section">
          <h3>权限</h3>
          <el-table :data="detail.permissions" size="small" border>
            <el-table-column prop="permission" label="权限" min-width="220" />
            <el-table-column prop="risk" label="风险" width="100" />
            <el-table-column label="状态" width="100">
              <template #default="{ row }">
                <el-tag :type="row.granted ? 'success' : 'warning'" size="small">
                  {{ row.granted ? "已授权" : "未授权" }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="reason" label="说明" min-width="220" />
          </el-table>
        </section>
        <section class="detail-section">
          <h3>版本</h3>
          <el-table :data="detail.versions" size="small" border>
            <el-table-column prop="version" label="版本" width="140" />
            <el-table-column label="状态" width="120">
              <template #default="{ row }">
                <el-tag :type="row.current ? 'success' : 'info'" size="small">
                  {{ row.current ? "当前" : "历史" }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="安装时间" min-width="180">
              <template #default="{ row }">{{ formatDate(row.releasedAt) }}</template>
            </el-table-column>
          </el-table>
        </section>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, ref, type PropType } from "vue";
import { useRouter } from "vue-router";
import {
  ArrowRight,
  Box,
  Connection,
  DocumentChecked,
  Refresh,
  Search,
  Share,
} from "@element-plus/icons-vue";
import { ElButton, ElMessage, ElSwitch } from "element-plus";
import ExtensionPageHeader from "./components/ExtensionPageHeader.vue";
import { fetchExtensionCenterDetail, fetchExtensionCenterView } from "./api";
import type {
  ExtensionCenterCard,
  ExtensionCenterDetail,
  ExtensionCenterView,
} from "./types";
import { disableExtension, enableExtension } from "@/views/kernel/api";
import { useExtensionUIStore } from "@/stores/extensionUI";

const router = useRouter();
const extensionUIStore = useExtensionUIStore();
const loading = ref(false);
const detailLoading = ref(false);
const view = ref<ExtensionCenterView | null>(null);
const detail = ref<ExtensionCenterDetail | null>(null);
const detailVisible = ref(false);
const detailError = ref("");
const loadError = ref("");
const search = ref("");

const entries = [
  { to: "/extensions/packages", title: "扩展包", description: "安装、更新、回滚和卸载扩展", icon: Box },
  { to: "/extensions/workflows", title: "工作流", description: "创建、运行和管理自动化工作流", icon: Share },
  { to: "/extensions/mcp", title: "MCP 服务", description: "连接外部 MCP 服务", icon: Connection },
  { to: "/extensions/agent-skills", title: "Agent Skills", description: "管理 SKILL.md 指令包", icon: DocumentChecked },
];

const installedCount = computed(() =>
  new Set([
    ...(view.value?.installed || []),
    ...(view.value?.updates || []),
    ...(view.value?.needsAction || []),
    ...(view.value?.localImports || []),
    ...(view.value?.dev || []),
  ].map((card) => card.extensionId)).size,
);

const specialCount = computed(() => {
  const ids = new Set<string>();
  for (const card of [...(view.value?.localImports || []), ...(view.value?.dev || [])]) {
    ids.add(card.extensionId);
  }
  return ids.size;
});

function matches(card: ExtensionCenterCard) {
  const query = search.value.trim().toLowerCase();
  if (!query) return true;
  return [
    card.displayName,
    card.extensionId,
    card.publisher,
    card.description,
    card.source,
    card.trust,
    card.status,
  ].some((value) => String(value || "").toLowerCase().includes(query));
}

function filtered(cards: ExtensionCenterCard[]) {
  return cards.filter(matches);
}

function formatDate(value?: string) {
  if (!value) return "-";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString("zh-CN");
}

async function load() {
  loading.value = true;
  loadError.value = "";
  try {
    view.value = await fetchExtensionCenterView();
  } catch (error: any) {
    loadError.value = error?.message || String(error);
  } finally {
    loading.value = false;
  }
}

async function openDetail(card: ExtensionCenterCard) {
  detailVisible.value = true;
  detailLoading.value = true;
  detailError.value = "";
  detail.value = null;
  try {
    detail.value = await fetchExtensionCenterDetail(card.extensionId);
  } catch (error: any) {
    detailError.value = error?.message || String(error);
  } finally {
    detailLoading.value = false;
  }
}

async function toggleExtension(card: ExtensionCenterCard, enabled: boolean) {
  try {
    if (enabled) await enableExtension(card.extensionId);
    else await disableExtension(card.extensionId);
    extensionUIStore.dispatchExtensionChanged(enabled ? "enable" : "disable");
    await load();
  } catch (error: any) {
    ElMessage.error(error?.message || String(error));
  }
}

function handleAction(card: ExtensionCenterCard, actionID: string) {
  if (actionID === "detail") {
    void openDetail(card);
    return;
  }
  if (actionID === "permissions" || actionID === "update" || actionID === "rollback" || actionID === "uninstall") {
    void router.push({
      path: "/extensions/packages",
      query: { extensionId: card.extensionId, action: actionID },
    });
    return;
  }
  if (actionID === "diagnose") {
    void router.push({
      path: "/kernel/dev-console",
      query: { extensionId: card.extensionId },
    });
  }
}

const ExtensionCenterGroup = defineComponent({
  name: "ExtensionCenterGroup",
  props: {
    title: { type: String, required: true },
    cards: { type: Array as PropType<ExtensionCenterCard[]>, required: true },
    emptyText: { type: String, required: true },
    showToggle: { type: Boolean, default: true },
  },
  emits: ["detail", "toggle", "action"],
  setup(props, { emit }) {
    return () => h("section", { class: "group-section" }, [
      h("div", { class: "group-heading" }, [
        h("h2", props.title),
        h("span", String(props.cards.length)),
      ]),
      props.cards.length === 0
        ? h("div", { class: "group-empty" }, props.emptyText)
        : h("div", { class: "card-grid" }, props.cards.map((card) => h("article", {
          class: "extension-card",
          key: card.extensionId,
        }, [
          h("div", { class: "card-main" }, [
            h("div", { class: "card-title" }, [
              h("h3", card.displayName || card.extensionId),
              h("span", `v${card.version || "-"}`),
            ]),
            h("p", { class: "card-id" }, card.extensionId),
            h("p", { class: "card-description" }, card.description || "暂无描述"),
            h("div", { class: "card-tags" }, [
              h("span", { class: "tag" }, card.status),
              h("span", { class: "tag" }, card.trust || "untrusted"),
              card.permissionRisk ? h("span", { class: `tag risk-${card.permissionRisk}` }, `风险 ${card.permissionRisk}`) : null,
              card.source ? h("span", { class: "tag" }, card.source) : null,
            ]),
            card.diagnostics?.length
              ? h("div", { class: "card-diagnostics" }, card.diagnostics.map((item) => h("p", {
                key: `${item.code}-${item.message}`,
              }, `${item.code}: ${item.message}`)))
              : null,
          ]),
          h("div", { class: "card-actions" }, [
            props.showToggle
              ? h(ElSwitch, {
                modelValue: card.enabled,
                disabled: card.status === "installing" || card.status === "updating",
                "onUpdate:modelValue": (value: string | number | boolean) => emit("toggle", card, Boolean(value)),
              })
              : null,
            h(ElButton, { size: "small", onClick: () => emit("detail", card) }, { default: () => "详情" }),
            h(ElButton, {
              size: "small",
              type: card.updateAvailable ? "primary" : "default",
              onClick: () => emit("action", card, card.updateAvailable ? "update" : "permissions"),
            }, { default: () => card.updateAvailable ? `更新到 ${card.latestVersion || "新版本"}` : "权限" }),
          ]),
        ]))),
    ]);
  },
});

onMounted(load);
</script>

<style scoped>
.extension-center {
  display: grid;
  gap: 18px;
  min-height: 100%;
  padding: 4px;
}
.summary-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}
.summary-item {
  display: grid;
  gap: 4px;
  padding: 15px 16px;
  border: 1px solid var(--console-border);
  border-radius: 12px;
  background: var(--ac-color-surface);
}
.summary-item span { color: var(--console-text-muted); font-size: 12px; }
.summary-item strong { color: var(--console-text); font-size: 24px; }
.partial-alert :deep(.el-alert__content) { width: 100%; }
.diagnostic-list { display: grid; gap: 6px; margin-top: 8px; }
.diagnostic-list div { display: grid; grid-template-columns: minmax(160px, auto) 1fr; gap: 10px; font-size: 12px; }
.entry-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 12px;
}
.entry-card {
  display: grid;
  grid-template-columns: auto 1fr auto;
  align-items: center;
  gap: 14px;
  min-height: 86px;
  padding: 16px;
  border: 1px solid var(--console-border);
  border-radius: 12px;
  background: var(--ac-color-surface);
  color: var(--console-text);
  text-decoration: none;
}
.entry-card:hover,
.entry-card:focus-visible {
  border-color: var(--el-color-primary);
  background: var(--ac-color-surface-soft);
  outline: none;
}
.entry-card > .el-icon:first-child { font-size: 24px; color: var(--el-color-primary); }
.entry-card h2 { margin: 0 0 5px; font-size: 16px; }
.entry-card p { margin: 0; color: var(--console-text-muted); font-size: 12px; line-height: 1.5; }
.toolbar { display: flex; align-items: center; justify-content: space-between; gap: 14px; }
.search-input { width: min(440px, 100%); }
.updated-at { color: var(--console-text-muted); font-size: 12px; }
.load-error { margin: 0; }
.group-section { display: grid; gap: 10px; }
.group-heading { display: flex; align-items: center; gap: 8px; }
.group-heading h2 { margin: 0; color: var(--console-text); font-size: 18px; }
.group-heading span {
  min-width: 22px;
  padding: 1px 7px;
  border-radius: 999px;
  background: var(--ac-color-surface-soft);
  color: var(--console-text-muted);
  text-align: center;
  font-size: 12px;
}
.group-empty {
  padding: 22px;
  border: 1px dashed var(--console-border);
  border-radius: 12px;
  color: var(--console-text-muted);
  text-align: center;
  font-size: 13px;
}
.card-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(360px, 1fr)); gap: 12px; }
.extension-card {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 16px;
  padding: 16px;
  border: 1px solid var(--console-border);
  border-radius: 12px;
  background: var(--ac-color-surface);
}
.card-main { min-width: 0; }
.card-title { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.card-title h3 { margin: 0; color: var(--console-text); font-size: 16px; }
.card-title span { color: var(--console-text-muted); font-size: 12px; }
.card-id { margin: 4px 0 8px; color: var(--console-text-muted); font-size: 11px; word-break: break-all; }
.card-description { margin: 0; color: var(--console-text-muted); font-size: 12px; line-height: 1.55; }
.card-tags { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 10px; }
.tag {
  padding: 2px 7px;
  border-radius: 999px;
  background: var(--ac-color-surface-soft);
  color: var(--console-text-muted);
  font-size: 11px;
}
.risk-high,
.risk-critical { color: var(--el-color-danger); }
.risk-medium { color: var(--el-color-warning); }
.card-diagnostics {
  margin-top: 10px;
  padding: 8px 10px;
  border-left: 3px solid var(--el-color-warning);
  background: color-mix(in srgb, var(--el-color-warning) 8%, transparent);
}
.card-diagnostics p { margin: 0; color: var(--el-color-warning); font-size: 11px; line-height: 1.5; word-break: break-word; }
.card-actions { display: flex; flex-direction: column; align-items: flex-end; justify-content: center; gap: 8px; }
.detail-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; margin-bottom: 16px; }
.detail-heading h2 { margin: 0 0 4px; }
.detail-heading p { margin: 0; color: var(--console-text-muted); font-size: 12px; }
.detail-update-error { margin-top: 12px; }
.detail-section { display: grid; gap: 8px; margin-top: 18px; }
.detail-section h3 { margin: 0; color: var(--console-text); font-size: 15px; }
.detail-error { min-height: 120px; }
@media (max-width: 900px) {
  .summary-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .card-grid { grid-template-columns: 1fr; }
}
</style>
