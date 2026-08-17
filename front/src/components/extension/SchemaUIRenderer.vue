<script setup lang="ts">
import { ref, reactive, computed, watch, onMounted, onBeforeUnmount, onErrorCaptured, provide } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import type { UIContributionSummary } from "@/stores/extensionUI";
import { useExtensionUIStore } from "@/stores/extensionUI";
import { apiClient } from "@/composables/useApi";
import { fetchSchemaDocument } from "@/api/extension";
import SchemaUINode from "./SchemaUINode.vue";
import ExtensionRenderState from "./ExtensionRenderState.vue";
import {
  validateDocument,
  countNodes,
  type SchemaUIDocument,
  type SchemaUINode as SchemaUINodeType,
  type SchemaUIActionBinding,
  type UITheme,
} from "./schema-ui-utils";

const props = defineProps<{
  contribution: UIContributionSummary;
  context?: Record<string, unknown>;
  slotId: string;
}>();

const uiStore = useExtensionUIStore();

const schema = ref<SchemaUIDocument | null>(null);
const loading = ref(true);
const error = ref<string | null>(null);
const validationErrors = ref<string[]>([]);
const sessionId = ref<string>("");
const sessionReady = ref(false);
const sessionOrigin = ref("");
const sessionContractVersion = ref(0);
const actionLoading = reactive<Record<string, boolean>>({});
const capturedError = ref<string | null>(null);

const formState = reactive<Record<string, unknown>>({});
const localContextOverride = reactive<Record<string, unknown>>({});

const sessionScopeKey = computed(() =>
  `${props.contribution.contributionId}:${props.contribution.generation}:${props.context?.characterId || ""}:${props.context?.conversationId || ""}`
);

const mergedContext = computed<Record<string, unknown>>(() => ({
  ...(props.context ?? {}),
  ...localContextOverride,
  extensionId: props.contribution.extensionId,
  contributionId: props.contribution.contributionId,
  moduleId: props.contribution.moduleId,
  slotId: props.slotId,
  permissions: props.contribution.permissions ?? [],
  runtimeReady: props.contribution.runtimeReady,
  enabled: props.contribution.enabled,
  effective: props.contribution.effective,
  sandbox: props.contribution.sandbox,
  form_state: formState,
  formState: formState,
}));

const rootNode = computed<SchemaUINodeType | null>(() => schema.value?.root ?? null);
const rootList = computed<SchemaUINodeType[]>(() => schema.value?.children ?? []);
const hasRoot = computed(() => rootNode.value !== null || rootList.value.length > 0);

const themeConfig = computed(() => schema.value?.theme ?? null);
const localeConfig = computed(() => schema.value?.locale ?? null);
const accessibilityConfig = computed(() => schema.value?.accessibility ?? null);
const performanceBudget = computed(() => schema.value?.performanceBudget ?? null);

const themeMode = computed<UITheme>(() => themeConfig.value?.mode ?? "auto");
const effectiveTheme = computed<"light" | "dark">(() => {
  if (themeMode.value === "auto") {
    const hostTheme = (props.context?.theme as { mode?: "light" | "dark" } | undefined)?.mode;
    if (hostTheme === "light" || hostTheme === "dark") return hostTheme;
    if (typeof window !== "undefined" && window.matchMedia("(prefers-color-scheme: dark)").matches) {
      return "dark";
    }
    return "light";
  }
  return themeMode.value;
});

const themeOverridesStyle = computed(() => {
  const overrides = themeConfig.value?.overrides;
  if (!overrides) return undefined;
  const allowedPrefix = "--amitia-";
  const forbiddenKeys = new Set(["position", "z-index", "zIndex", "display", "top", "left", "right", "bottom", "width", "height", "max-width", "max-height", "min-width", "min-height", "overflow", "pointer-events", "visibility", "opacity", "transform", "transition", "animation"]);
  const entries: string[] = [];
  for (const [k, v] of Object.entries(overrides)) {
    if (!k.startsWith(allowedPrefix)) {
      if (import.meta.env.DEV) console.warn(`[SchemaUI] theme override ignored (not in whitelist): ${k}`);
      continue;
    }
    const propName = k.slice(allowedPrefix.length);
    if (forbiddenKeys.has(propName)) {
      if (import.meta.env.DEV) console.warn(`[SchemaUI] theme override ignored (forbidden): ${k}`);
      continue;
    }
    entries.push(`${k}: ${v}`);
  }
  return entries.length > 0 ? entries.join("; ") : undefined;
});

const localeValue = computed(() => localeConfig.value?.current ?? "zh-CN");
provide("schema-ui-locale", localeValue);

const accessibilityAttrs = computed<Record<string, string>>(() => {
  const cfg = accessibilityConfig.value;
  if (!cfg || !cfg.enabled) return {};
  const attrs: Record<string, string> = {};
  attrs["role"] = "region";
  attrs["aria-label"] = schema.value?.title || "Schema UI";
  if (cfg.highContrast) attrs["data-high-contrast"] = "true";
  if (cfg.reducedMotion) attrs["data-reduced-motion"] = "true";
  if (cfg.screenReader) attrs["aria-live"] = "polite";
  if (cfg.keyboardNav) attrs["tabindex"] = "0";
  return attrs;
});

const nodeCountExceeded = computed(() => {
  const budget = performanceBudget.value;
  if (!budget || budget.maxNodeCount <= 0) return false;
  const roots: SchemaUINodeType[] = schema.value?.root ? [schema.value.root] : schema.value?.children ?? [];
  let total = 0;
  for (const r of roots) total += countNodes(r);
  return total > budget.maxNodeCount;
});

onErrorCaptured((err) => {
  const message = err instanceof Error ? err.message : String(err);
  capturedError.value = message;
  try {
    uiStore.recordError({
      contributionId: props.contribution.contributionId,
      slotId: props.slotId,
      message,
      timestamp: Date.now(),
      recoverable: true,
    });
  } catch {
  }
  return false;
});

async function loadSchema() {
  if (!props.contribution.schemaPath && !props.contribution.entryPath) {
    loading.value = false;
    error.value = "该贡献未提供 schema 路径";
    return;
  }
  loading.value = true;
  error.value = null;
  capturedError.value = null;
  validationErrors.value = [];
  schema.value = null;
  try {
    const data = (await fetchSchemaDocument(
      props.contribution.extensionId,
      props.contribution.contributionId,
    )) as SchemaUIDocument;
    const result = validateDocument(data);
    if (!result.valid) {
      validationErrors.value = result.errors;
      error.value = `schema 校验未通过: ${result.errors.join("; ")}`;
      schema.value = data;
    } else {
      schema.value = data;
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    loading.value = false;
  }
}

async function ensureSession(): Promise<string> {
  if (sessionId.value && sessionReady.value) return sessionId.value;
  const key = `${props.contribution.extensionId}/${props.contribution.contributionId}`;
  const surfaceData = (props.context?.surface as Record<string, unknown> | undefined) ?? {};
  const res = await apiClient.post<{
    sessionId?: string;
    session_id?: string;
    origin?: string;
    contractVersion?: number;
  }>("/api/extensions/ui/sessions", {
    contributionId: props.contribution.contributionId,
    surface: String(surfaceData.role ?? "main"),
    characterId: (props.context?.characterId as string) || "",
    conversationId: (props.context?.conversationId as string) || "",
  });
  const data = res.data;
  const sid = data.sessionId ?? data.session_id ?? "";
  if (!sid) throw new Error("session 创建响应缺少 sessionId");
  sessionId.value = sid;
  sessionOrigin.value = data.origin ?? "";
  sessionContractVersion.value = data.contractVersion ?? props.contribution.contractVersion;
  sessionReady.value = true;
  uiStore.registerSession({
    contributionId: key,
    sessionId: sid,
    createdAt: Date.now(),
    lastActivity: Date.now(),
  });
  return sid;
}

async function invokeAction(payload: { action: SchemaUIActionBinding; node: SchemaUINodeType }) {
  const { action, node } = payload;
  if (!action?.action_id) {
    ElMessage.warning("操作缺少 action_id");
    return;
  }
  if (action.confirmation) {
    try {
      await ElMessageBox.confirm(action.confirmation, "确认操作", {
        type: "warning",
        confirmButtonText: "确定",
        cancelButtonText: "取消",
      });
    } catch {
      return;
    }
  }
  actionLoading[action.action_id] = true;
  try {
    const sid = await ensureSession();
    const res = await fetch(`/api/extensions/ui/sessions/${sid}/bridge`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        method: "ui.action.invoke",
        contributionId: props.contribution.contributionId,
        origin: sessionOrigin.value,
        contractVersion: sessionContractVersion.value,
        payload: {
          action_id: action.action_id,
          input: {
            ...(action.input ?? {}),
            node_id: node.id,
            form_state: { ...formState },
          },
        },
      }),
    });
    if (!res.ok) {
      const text = await res.text().catch(() => "");
      throw new Error(`操作调用失败: ${res.status} ${text}`);
    }
    const envelope = (await res.json().catch(() => ({}))) as {
      ok?: boolean;
      result?: Record<string, unknown>;
      error?: { message?: string } | string;
    };
    if (envelope.ok === false) {
      const detail = typeof envelope.error === "string" ? envelope.error : envelope.error?.message;
      throw new Error(detail || "操作执行失败");
    }
    const data = envelope.result ?? {};
    if (data && typeof data === "object") {
      if (data.clientExecute === true && typeof data.text === "string") {
        try {
          if (window.amitiaDesktop?.writeClipboardText) {
            await window.amitiaDesktop.writeClipboardText(data.text as string);
          } else if (navigator.clipboard) {
            await navigator.clipboard.writeText(data.text as string);
          } else {
            throw new Error("CLIPBOARD_HOST_UNAVAILABLE");
          }
          ElMessage.success("已复制到剪贴板");
        } catch (clipErr) {
          const clipMsg = clipErr instanceof Error ? clipErr.message : String(clipErr);
          throw new Error(`复制失败: ${clipMsg}`);
        }
      }
      if (data.form_state && typeof data.form_state === "object") {
        const incoming = data.form_state as Record<string, unknown>;
        for (const k of Object.keys(incoming)) formState[k] = incoming[k];
      }
      if (data.context_update && typeof data.context_update === "object") {
        const update = data.context_update as Record<string, unknown>;
        for (const k of Object.keys(update)) localContextOverride[k] = update[k];
      }
      if (data.reload_schema === true) {
        await loadSchema();
      }
      if (typeof data.message === "string" && data.message) {
        ElMessage.success(data.message);
      }
    }
  } catch (e) {
    const message = e instanceof Error ? e.message : String(e);
    ElMessage.error(message);
    try {
      uiStore.recordError({
        contributionId: props.contribution.contributionId,
        slotId: props.slotId,
        message,
        timestamp: Date.now(),
        recoverable: true,
      });
    } catch {
    }
  } finally {
    actionLoading[action.action_id] = false;
  }
}

function onNodeError(payload: { nodeId: string; message: string }) {
  capturedError.value = `节点 ${payload.nodeId}: ${payload.message}`;
}

function retry() {
  loadSchema();
}

onMounted(() => {
  loadSchema();
  ensureSession().catch(() => {});
});

watch(
  () => props.contribution.contributionId,
  async () => {
    for (const k of Object.keys(formState)) delete formState[k];
    for (const k of Object.keys(localContextOverride)) delete localContextOverride[k];
    if (sessionId.value) {
      await apiClient.delete(`/api/extensions/ui/sessions/${sessionId.value}`).catch(() => {});
    }
    sessionId.value = "";
    sessionReady.value = false;
    capturedError.value = null;
    loadSchema();
    ensureSession().catch(() => {});
  }
);

watch(sessionScopeKey, async () => {
  if (sessionId.value) {
    await apiClient.delete(`/api/extensions/ui/sessions/${sessionId.value}`).catch(() => {});
  }
  sessionId.value = "";
  sessionReady.value = false;
  ensureSession().catch(() => {});
});

watch(
  () => props.contribution.schemaPath,
  () => {
    loadSchema();
  }
);

watch(
  () => props.contribution.generation,
  async () => {
    if (sessionId.value) {
      await apiClient.delete(`/api/extensions/ui/sessions/${sessionId.value}`).catch(() => {});
    }
    sessionId.value = "";
    sessionReady.value = false;
    ensureSession().catch(() => {});
  }
);

onBeforeUnmount(() => {
  if (sessionId.value) {
    apiClient.delete(`/api/extensions/ui/sessions/${sessionId.value}`).catch(() => {});
    sessionId.value = "";
    sessionReady.value = false;
  }
});
</script>

<template>
  <div
    class="schema-ui-renderer"
    :data-contribution-id="contribution.contributionId"
    :data-theme="effectiveTheme"
    :style="themeOverridesStyle"
    v-bind="accessibilityAttrs"
  >
    <template v-if="loading">
      <ExtensionRenderState state="loading" />
    </template>
    <template v-else-if="capturedError">
      <ExtensionRenderState state="error" :detail="capturedError" @retry="retry" />
    </template>
    <template v-else-if="error && !schema">
      <ExtensionRenderState state="error" :detail="error" @retry="retry" />
    </template>
    <template v-else-if="schema">
      <div
        v-if="validationErrors.length > 0"
        class="schema-ui-renderer__warning"
      >
        <span>schema 校验警告: {{ validationErrors.join("; ") }}</span>
      </div>
      <div
        v-if="nodeCountExceeded"
        class="schema-ui-renderer__warning"
      >
        <span>节点数量超出性能预算限制</span>
      </div>
      <div v-if="!nodeCountExceeded" class="schema-ui-renderer__content">
        <template v-if="hasRoot">
          <SchemaUINode
            v-if="rootNode"
            :node="rootNode"
            :depth="1"
            :form-state="formState"
            :context="mergedContext"
            :session-id="sessionId"
            :extension-id="contribution.extensionId"
            :contribution-id="contribution.contributionId"
            @action="invokeAction"
            @error="onNodeError"
          />
          <template v-else>
            <SchemaUINode
              v-for="r in rootList"
              :key="r.id"
              :node="r"
              :depth="1"
              :form-state="formState"
              :context="mergedContext"
              :session-id="sessionId"
              :extension-id="contribution.extensionId"
              :contribution-id="contribution.contributionId"
              @action="invokeAction"
              @error="onNodeError"
            />
          </template>
        </template>
        <ExtensionRenderState v-else state="empty" detail="该贡献未提供可渲染的 schema 内容" />
      </div>
    </template>
    <template v-else>
      <ExtensionRenderState state="empty" detail="暂无 schema 数据" />
    </template>
  </div>
</template>

<style scoped>
.schema-ui-renderer {
  width: 100%;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-width: 100%;
  color: var(--amitia-text-primary, var(--amitia-color-text, inherit));
  background: var(--amitia-bg-surface, var(--amitia-color-surface, transparent));
}
.schema-ui-renderer__loading {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  font-size: 12px;
  color: var(--amitia-color-text-secondary, rgba(127, 127, 127, 0.8));
}
.schema-ui-renderer__spinner {
  width: 14px;
  height: 14px;
  border: 2px solid rgba(127, 127, 127, 0.2);
  border-top-color: var(--amitia-color-accent, rgba(127, 127, 127, 0.8));
  border-radius: 50%;
  animation: schema-ui-spin 0.9s linear infinite;
}
@keyframes schema-ui-spin {
  to {
    transform: rotate(360deg);
  }
}
.schema-ui-renderer__error {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 10px 12px;
  border-radius: 6px;
  background: var(--ac-color-danger-bg);
  border: 1px solid color-mix(in srgb, var(--ac-color-danger) 32%, var(--plugin-surface-border));
  color: var(--ac-color-danger);
  font-size: 12px;
}
.schema-ui-renderer__error-title {
  font-weight: 600;
  font-size: 13px;
}
.schema-ui-renderer__error-detail {
  opacity: 0.9;
  word-break: break-word;
}
.schema-ui-renderer__retry {
  align-self: flex-start;
  padding: 3px 10px;
  border: 1px solid rgba(220, 60, 60, 0.4);
  border-radius: 4px;
  background: transparent;
  color: rgb(180, 40, 40);
  font-size: 12px;
  cursor: pointer;
}
.schema-ui-renderer__retry:hover {
  background: rgba(220, 60, 60, 0.1);
}
.schema-ui-renderer__warning {
  padding: 6px 10px;
  border-radius: 4px;
  background: var(--ac-color-warning-bg);
  border: 1px solid color-mix(in srgb, var(--ac-color-warning) 30%, var(--plugin-surface-border));
  color: var(--ac-color-warning);
  font-size: 11px;
  word-break: break-word;
}
.schema-ui-renderer__content {
  width: 100%;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.schema-ui-renderer__empty {
  padding: 12px;
  font-size: 12px;
  color: var(--amitia-color-text-secondary, rgba(127, 127, 127, 0.6));
  text-align: center;
  border: 1px dashed var(--amitia-color-border, rgba(127, 127, 127, 0.25));
  border-radius: 6px;
}
@media (max-width: 480px) {
  .schema-ui-renderer__content {
    gap: 8px;
  }
}
</style>
