<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch, defineAsyncComponent } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useExtensionUIStore } from "@/stores/extensionUI";
import type { UIContributionSummary } from "@/stores/extensionUI";
import { openExtensionPage, pollPageSessionStatus, closePageSession } from "@/api/extension";
import { resolveHostEnvironment } from "@/composables/useHostEnvironment";
import { resolvePluginSurface } from "@/ui-runtime/pluginSurfaceResolver";
import PluginPageFrame from "./PluginPageFrame.vue";
import ExtensionLoadingOverlay from "./ExtensionLoadingOverlay.vue";

const SchemaUIRenderer = defineAsyncComponent({ loader: () => import("./SchemaUIRenderer.vue"), loadingComponent: ExtensionLoadingOverlay, delay: 0 });

type PageState =
  | "resolving"
  | "permission_check"
  | "runtime_starting"
  | "loading"
  | "ready"
  | "degraded"
  | "failed"
  | "disabled"
  | "not_installed"
  | "incompatible"
  | "suspended";

interface PageSpec {
  pageId: string;
  routeKey: string;
  title: { default: string; translations?: Record<string, string> };
  description: { default: string; translations?: Record<string, string> };
  icon?: string;
  entryKind: "schema_page" | "web_page";
  entryPath: string;
  schemaPath?: string;
  scope: "global" | "character" | "conversation";
  permissions?: string[];
  statePolicy: "ephemeral" | "session" | "persistent_preferences";
}

interface OpenPageResult {
  sessionId: string;
  state: PageState;
  definition?: PageSpec;
  missingPermissions?: string[];
  reason?: string;
}

const route = useRoute();
const router = useRouter();
const uiStore = useExtensionUIStore();

const sessionId = ref<string>("");
const pageSessionGeneration = ref<number>(0);
const pageState = ref<PageState>("resolving");
const pageSpec = ref<PageSpec | null>(null);
const missingPermissions = ref<string[]>([]);
const errorReason = ref<string>("");
const diagnostics = ref<Record<string, unknown>>({});

const extensionId = computed(() => String(route.query.extensionId ?? route.params.extensionId ?? ""));
const pageId = computed(() => String(route.params.pageId ?? ""));
const pageTarget = computed(() =>
  `plugin.page.${extensionId.value.trim().toLowerCase().replace(/[^a-z0-9_-]+/g, "-") || "root"}.${pageId.value.trim().toLowerCase().replace(/[^a-z0-9_-]+/g, "-") || "root"}`
);

const title = computed(() => pageSpec.value?.title.default ?? pageId.value);
const description = computed(() => pageSpec.value?.description.default ?? "");

const stateLabel = computed<Record<PageState, string>>(() => ({
  resolving: "解析中",
  permission_check: "权限校验",
  runtime_starting: "运行时启动中",
  loading: "加载中",
  ready: "就绪",
  degraded: "降级",
  failed: "失败",
  disabled: "已禁用",
  not_installed: "未安装",
  incompatible: "不兼容",
  suspended: "已挂起",
}));

const isReady = computed(() => pageState.value === "ready");
const isError = computed(() =>
  ["failed", "disabled", "not_installed", "incompatible"].includes(pageState.value)
);
const pageContribution = computed<UIContributionSummary | null>(() => {
  if (!pageSpec.value) return null;
  return {
    contributionId: pageId.value,
    extensionId: extensionId.value,
    moduleId: "",
    kind: pageSpec.value.entryKind,
    slotId: "extension.page.main",
    contractVersion: 1,
    generation: pageSessionGeneration.value,
    title: pageSpec.value.title.default,
    ordering: 0,
    visible: true,
    effective: true,
    enabled: true,
    runtimeReady: true,
    permissions: pageSpec.value.permissions ?? [],
    sandbox: pageSpec.value.entryKind === "web_page" ? "client_module" : "schema_renderer",
    entryPath: pageSpec.value.entryPath,
    schemaPath: pageSpec.value.schemaPath,
    actions: [],
  };
});
const pageSurfaceKind = computed(() =>
  pageContribution.value ? resolvePluginSurface(pageContribution.value) : "none",
);
const isSchemaPage = computed(() => pageSurfaceKind.value === "schema");

const pageContext = computed<Record<string, unknown>>(() => {
  const env = resolveHostEnvironment();
  return {
    extensionId: extensionId.value,
    pageId: pageId.value,
    sessionId: sessionId.value,
    scope: pageSpec.value?.scope ?? "global",
    params: route.query,
    platform: env.platform,
    host: env.host,
    os: env.os,
    locale: navigator.language ?? "en",
  };
});

async function openPage() {
  const token = ++pageRequestToken;
  if (!extensionId.value || !pageId.value) return;
  pageState.value = "resolving";
  errorReason.value = "";
  try {
    const result = await openExtensionPage(extensionId.value, pageId.value, {
      params: route.query as Record<string, unknown>,
      scopeSnapshot: buildScopeSnapshot(),
    });
    if (disposed || token !== pageRequestToken) {
      if (result.sessionId) {
        void closePageSession(result.sessionId).catch((error) => {
  console.error("[original-error]", error);
});
      }
      return;
    }
    sessionId.value = result.sessionId;
    pageSessionGeneration.value = result.generation ?? 0;
    pageState.value = result.state as PageState;
    pageSpec.value = (result.definition as PageSpec) ?? null;
    missingPermissions.value = result.missingPermissions ?? [];
    errorReason.value = result.reason ?? "";
    if (result.state === "loading" || result.state === "runtime_starting") {
      void pollPageStatus(token);
    }
    if (result.sessionId) {
      const registryKey = `${extensionId.value}/${pageId.value}`;
      activePageKey = registryKey;
      uiStore.registerSession({
        contributionId: registryKey,
        sessionId: result.sessionId,
        createdAt: Date.now(),
        lastActivity: Date.now(),
      });
    }
  } catch (e) {
    if (disposed || token !== pageRequestToken) return;
    pageState.value = "failed";
    errorReason.value = e instanceof Error ? e.message : String(e);
  }
}

let pollTimer: ReturnType<typeof setTimeout> | null = null;
let pageRequestToken = 0;
let disposed = false;
let activePageKey = "";

async function pollPageStatus(token: number) {
  if (disposed || token !== pageRequestToken || !sessionId.value) return;
  const currentSessionId = sessionId.value;
  try {
    const result = await pollPageSessionStatus(currentSessionId);
    if (disposed || token !== pageRequestToken || currentSessionId !== sessionId.value) return;
    pageSessionGeneration.value = result.generation ?? pageSessionGeneration.value;
    pageState.value = result.state as PageState;
    pageSpec.value = (result.definition as PageSpec) ?? pageSpec.value;
    missingPermissions.value = result.missingPermissions ?? [];
    errorReason.value = result.reason ?? "";
    if (result.state === "loading" || result.state === "runtime_starting") {
      pollTimer = setTimeout(() => {
        void pollPageStatus(token);
      }, 1000);
    }
  } catch (e) {
    if (disposed || token !== pageRequestToken) return;
    pageState.value = "failed";
    errorReason.value = e instanceof Error ? e.message : String(e);
  }
}

async function closePage() {
  pageRequestToken += 1;
  if (pollTimer) {
    clearTimeout(pollTimer);
    pollTimer = null;
  }
  const currentSessionId = sessionId.value;
  const registryKey = activePageKey;
  activePageKey = "";
  if (!currentSessionId) return;
  sessionId.value = "";
  pageSessionGeneration.value = 0;
  if (registryKey) {
    uiStore.unregisterSession(registryKey);
  }
  try {
    await closePageSession(currentSessionId);
  } catch (error) {
  console.error("[original-error]", error);
}
}

function buildScopeSnapshot(): string {
  const env = resolveHostEnvironment();
  return JSON.stringify({
    platform: env.platform,
    host: env.host,
    os: env.os,
    locale: navigator.language ?? "en",
    timestamp: Date.now(),
  });
}

async function retry() {
  await closePage();
  await openPage();
}

async function goBack() {
  await closePage();
  router.push({ name: "extensionCenter" });
}

onMounted(() => {
  disposed = false;
  void openPage();
});
onBeforeUnmount(() => {
  disposed = true;
  pageRequestToken += 1;
  if (pollTimer) {
    clearTimeout(pollTimer);
    pollTimer = null;
  }
  void closePage();
});

watch([extensionId, pageId], async () => {
  await closePage();
  await openPage();
});
</script>

<template>
  <PluginPageFrame
    class="extension-page-host"
    :title="title"
    :description="description"
    data-amitia-ui="extension.page"
    :data-amitia-node-target="pageTarget"
    data-amitia-surface="plugin.page.main"
    :data-extension-id="extensionId"
    :data-page-id="pageId"
  >
    <template #actions>
        <span class="extension-page-host__state-badge" :data-state="pageState">
          {{ stateLabel[pageState] }}
        </span>
        <button class="extension-page-host__action" @click="retry">重试</button>
        <button class="extension-page-host__action" @click="goBack">返回</button>
    </template>

    <main class="extension-page-host__content">
      <template v-if="isError">
        <div class="extension-page-host__error">
          <p>{{ stateLabel[pageState] }}</p>
          <p v-if="errorReason" class="extension-page-host__error-reason">{{ errorReason }}</p>
          <p v-if="missingPermissions.length > 0" class="extension-page-host__missing">
            缺少权限: {{ missingPermissions.join(", ") }}
          </p>
        </div>
      </template>
      <template v-else-if="!isReady">
        <ExtensionLoadingOverlay />
      </template>
      <template v-else-if="pageContribution">
        <div class="extension-page-host__page-content">
          <SchemaUIRenderer
            v-if="isSchemaPage"
            :contribution="pageContribution"
            :slot-id="`extension.page.main`"
            :context="pageContext"
          />
          <div v-else class="extension-page-host__placeholder">
            未知页面类型: {{ pageSpec?.entryKind }}
          </div>
        </div>
      </template>
    </main>

    <footer v-if="diagnostics && Object.keys(diagnostics).length > 0" class="extension-page-host__diag">
      <pre>{{ JSON.stringify(diagnostics, null, 2) }}</pre>
    </footer>
  </PluginPageFrame>
</template>

<style scoped>
.extension-page-host {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}
.extension-page-host :deep(.plugin-page-frame__content) {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}
.extension-page-host__state-badge {
  padding: 2px 8px;
  border-radius: 12px;
  background: rgba(127, 127, 127, 0.15);
  font-size: 11px;
}
.extension-page-host__state-badge[data-state="ready"] {
  background: rgba(80, 200, 80, 0.15);
  color: rgb(40, 160, 40);
}
.extension-page-host__state-badge[data-state="failed"],
.extension-page-host__state-badge[data-state="disabled"],
.extension-page-host__state-badge[data-state="not_installed"],
.extension-page-host__state-badge[data-state="incompatible"] {
  background: rgba(220, 60, 60, 0.15);
  color: rgb(180, 40, 40);
}
.extension-page-host__actions {
  display: flex;
  gap: 8px;
}
.extension-page-host__action {
  padding: 4px 10px;
  background: transparent;
  border: 1px solid var(--amitia-color-border, rgba(127, 127, 127, 0.3));
  border-radius: 6px;
  cursor: pointer;
  font-size: 12px;
}
.extension-page-host__action:hover {
  background: rgba(127, 127, 127, 0.1);
}
.extension-page-host__content {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: auto;
  padding: 0;
}
.extension-page-host__page-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}
.extension-page-host__loading {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  gap: 12px;
}
.extension-page-host__spinner {
  width: 32px;
  height: 32px;
  border: 2px solid rgba(127, 127, 127, 0.2);
  border-top-color: rgba(127, 127, 127, 0.8);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}
@keyframes spin {
  to { transform: rotate(360deg); }
}
.extension-page-host__error {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  text-align: center;
  color: rgb(180, 40, 40);
  gap: 6px;
}
.extension-page-host__error-reason,
.extension-page-host__missing {
  font-size: 12px;
  opacity: 0.8;
}
.extension-page-host__placeholder {
  padding: 24px;
  text-align: center;
  opacity: 0.6;
  border: 1px dashed rgba(127, 127, 127, 0.3);
  border-radius: 8px;
}
.extension-page-host__diag {
  padding: 8px 16px;
  background: rgba(127, 127, 127, 0.05);
  font-size: 11px;
  font-family: monospace;
}
</style>
