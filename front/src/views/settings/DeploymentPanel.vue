<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="deployment-panel">
    <section class="mode-summary" aria-label="当前设备连接状态">
      <div class="mode-summary-main">
        <span class="mode-summary-label">当前设备连接到</span>
        <strong>{{ currentMode === "local" ? "本机 Core" : "远程 Core" }}</strong>
        <span class="mode-summary-address">{{ currentApiURL || "连接地址读取中" }}</span>
      </div>
      <el-tag :type="statusType" effect="plain">{{ statusLabel }}</el-tag>
    </section>

    <el-card shadow="never" class="section-card">
      <template #header><span>当前设备的 Core 连接目标</span></template>
      <p class="connection-explanation">只决定这台设备使用哪个 Core，不会改变 Core 服务器自身的部署形态。</p>

      <el-radio-group v-model="formMode" class="mode-radio-group">
        <el-radio value="local" border class="mode-radio-card" :disabled="!desktopShell">
          <div class="mode-label">连接本机 Core</div>
          <div class="mode-desc">当前设备直接使用本机启动的 Core 服务</div>
        </el-radio>
        <el-radio value="cloud" border class="mode-radio-card">
          <div class="mode-label">连接远程 Core</div>
          <div class="mode-desc">当前设备连接一台已经部署好的远程 Core</div>
        </el-radio>
      </el-radio-group>

      <div v-if="formMode === 'cloud'" class="server-url-section">
        <el-form
          label-position="top"
          :model="formData"
          :rules="rules"
          ref="formRef"
        >
          <el-form-item label="服务器地址" prop="serverURL">
            <el-input
              v-model="formData.serverURL"
              placeholder="例如 http://192.168.1.100:18899"
              clearable
            />
            <div class="form-tip">
              输入你部署的 Core 服务器完整地址，包含协议和端口
            </div>
          </el-form-item>
        </el-form>
      </div>

      <div class="mode-save-row">
        <span class="mode-save-hint">仅更改当前设备的连接目标，保存后可能需要重新连接。</span>
        <el-tag v-if="saveSuccess" type="success" size="small" effect="plain">已保存</el-tag>
        <div v-if="saveError" class="save-error">{{ saveError }}</div>
        <el-button type="primary" @click="handleSave" :loading="saving">保存连接设置</el-button>
      </div>
    </el-card>

    <details class="deployment-advanced">
      <summary>当前设备连接详情</summary>
      <el-descriptions :column="1" border size="small">
        <el-descriptions-item label="当前连接目标">
          <el-tag
            :type="currentMode === 'local' ? 'success' : 'warning'"
            size="small"
          >
            {{ currentMode === "local" ? "连接本机 Core" : "连接远程 Core" }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="API 地址">{{
          currentApiURL || "—"
        }}</el-descriptions-item>
        <el-descriptions-item label="运行状态">
          <el-tag :type="statusType" size="small">{{ statusLabel }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item v-if="runtimeStatus?.businessCore" label="业务 Core">
          <div style="display: flex; align-items: center; gap: 8px;">
            <span style="font-size: var(--ac-font-size-xs); color: var(--ac-color-text-muted);">{{ runtimeStatus.businessCore.baseURL }}</span>
            <el-tag :type="endpointStatusType(runtimeStatus.businessCore.state)" size="small">{{ endpointStatusLabel(runtimeStatus.businessCore.state) }}</el-tag>
          </div>
        </el-descriptions-item>
        <el-descriptions-item v-if="runtimeStatus?.localRuntime" label="本机执行节点">
          <div style="display: flex; align-items: center; gap: 8px;">
            <span style="font-size: var(--ac-font-size-xs); color: var(--ac-color-text-muted);">{{ runtimeStatus.localRuntime.baseURL }}</span>
            <el-tag :type="endpointStatusType(runtimeStatus.localRuntime.state)" size="small">{{ endpointStatusLabel(runtimeStatus.localRuntime.state) }}</el-tag>
            <span v-if="runtimeStatus.localRuntime.profile" style="font-size: var(--ac-font-size-xs); color: var(--ac-color-text-muted);">({{ runtimeStatus.localRuntime.profile }})</span>
          </div>
        </el-descriptions-item>
      </el-descriptions>
    </details>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted, computed } from "vue";
import { ElMessage } from "element-plus";
import type { FormInstance, FormRules } from "element-plus";
import {
  getDeploymentConfig,
  saveDeploymentConfig,
  getApiBaseURL,
} from "../../runtime/runtime-adapter";
import type {
  DeploymentModeConfig,
  RuntimeStatus,
} from "../../runtime/runtime-types";
import { initializeRuntimeCapabilities, isDesktopShell } from "../../runtime/runtime-capabilities";

const formMode = ref<"local" | "cloud">("local");
const formData = reactive({ serverURL: "" });
const formRef = ref<FormInstance>();
const desktopShell = isDesktopShell();
const saving = ref(false);
const saveSuccess = ref(false);
const saveError = ref("");

const currentMode = ref<"local" | "cloud">("local");
const currentApiURL = ref("");
const runtimeStatus = ref<RuntimeStatus | null>(null);

const rules: FormRules = {
  serverURL: [
    { required: true, message: "请输入服务器地址", trigger: "blur" },
    {
      validator: (_rule, value, callback) => {
        if (!value || typeof value !== "string" || value.trim().length === 0) {
          callback(new Error("请输入服务器地址"));
          return;
        }
        const v = value.trim();
        if (!/^https?:\/\/.+/.test(v)) {
          callback(new Error("地址需以 http:// 或 https:// 开头"));
          return;
        }
        callback();
      },
      trigger: "blur",
    },
  ],
};

const statusType = computed(() => {
  if (!runtimeStatus.value) return "info";
  const map: Record<string, string> = {
    ready: "success",
    starting: "warning",
    "not-ready": "warning",
    "not-installed": "info",
    failed: "danger",
  };
  return map[runtimeStatus.value.state] || "info";
});

const statusLabel = computed(() => {
  if (!runtimeStatus.value) return "未知";
  const map: Record<string, string> = {
    ready: "就绪",
    starting: "启动中",
    "not-ready": "未就绪",
    "not-installed": "未安装",
    failed: "失败",
  };
  return map[runtimeStatus.value.state] || runtimeStatus.value.state;
});

function endpointStatusType(state: string): string {
  const map: Record<string, string> = {
    ready: "success",
    starting: "warning",
    "not-ready": "warning",
    "not-installed": "info",
    failed: "danger",
  };
  return map[state] || "info";
}

function endpointStatusLabel(state: string): string {
  const map: Record<string, string> = {
    ready: "就绪",
    starting: "启动中",
    "not-ready": "未就绪",
    "not-installed": "未安装",
    failed: "失败",
  };
  return map[state] || state;
}

async function loadConfig() {
  try {
    const config = await getDeploymentConfig();
    currentMode.value = config.mode;
    formMode.value = config.mode;
    if (config.mode === "cloud" && config.serverURL) {
      formData.serverURL = config.serverURL;
    }
    currentApiURL.value = await getApiBaseURL();
  } catch (err) {
    console.error("加载部署配置失败:", err);
  }
}

async function handleSave() {
  saveError.value = "";
  saveSuccess.value = false;

  if (formMode.value === "cloud") {
    if (!formRef.value) return;
    try {
      await formRef.value.validate();
    } catch {
      return;
    }
  }

  saving.value = true;
  try {
    const config: DeploymentModeConfig =
      formMode.value === "cloud"
        ? {
            mode: "cloud",
            serverURL: formData.serverURL.trim().replace(/\/+$/, ""),
          }
        : { mode: "local" };

    if (!desktopShell && config.mode === "local") {
      ElMessage.warning("浏览器作为 Cloud Web 设备运行，不支持切换为本地 Core 模式");
      return;
    }

    await saveDeploymentConfig(config);
    await initializeRuntimeCapabilities(true);
    saveSuccess.value = true;
    ElMessage.success(desktopShell ? "设备连接设置已保存，可能需要重新连接或重启应用生效；Core 服务端配置未改变" : "远程 Core 连接地址已保存；如服务已变更，请在设备配对页完成配对");

    currentMode.value = config.mode;
    currentApiURL.value = await getApiBaseURL();
  } catch (err: any) {
    saveError.value = err?.message || "保存失败";
    ElMessage.error("保存失败: " + saveError.value);
  } finally {
    saving.value = false;
  }
}

let unsubscribeStatus: (() => void) | null = null;

onMounted(async () => {
  await loadConfig();

  const api = window.amitiaDesktop;
  if (api) {
    try {
      runtimeStatus.value = await api.getRuntimeStatus();
    } catch {}
    unsubscribeStatus = api.onRuntimeStatusChanged((status) => {
      runtimeStatus.value = status;
    });
  }
});

onUnmounted(() => {
  unsubscribeStatus?.();
});
</script>

<style scoped>
.deployment-panel {
}
.section-card {
  margin-bottom: 12px;
  border: 1px solid var(--ac-color-border-light);
}

.mode-radio-group {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.mode-radio-card {
  flex: 1;
  min-width: 200px;
  padding: 14px 16px !important;
  margin-right: 0 !important;
  height: auto !important;
  border-radius: var(--ac-radius-md) !important;
}

.mode-label {
  font-size: calc(var(--ac-font-size-base) * 15 / 14);
  font-weight: 600;
  color: var(--ac-color-text);
  margin-bottom: 4px;
}

.mode-desc {
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-muted);
  line-height: 1.4;
}

.server-url-section {
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--ac-color-border-light);
}

.form-tip {
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-muted);
  margin-top: 4px;
}

.save-error {
  font-size: var(--ac-font-size-xs);
  color: var(--el-color-danger);
}
.mode-summary { display: flex; justify-content: space-between; align-items: flex-start; gap: 18px; margin-bottom: 20px; padding: 24px; background: var(--surface-bg); border: 1px solid var(--surface-border); border-radius: 14px; }
.mode-summary-main { display: grid; gap: 7px; min-width: 0; }
.mode-summary-label { font-size: 12px; color: var(--text-secondary); }
.mode-summary-main strong { font-size: 22px; line-height: 1.3; letter-spacing: -0.02em; }
.mode-summary-address { color: var(--text-secondary); font-size: 13px; overflow-wrap: anywhere; }
.deployment-panel :deep(.section-card) { border: 1px solid var(--surface-border); border-radius: 14px; background: var(--surface-bg); margin-bottom: 0; }
.deployment-panel :deep(.section-card .el-card__header) { padding: 20px 24px 8px; border-bottom: 0; font-weight: 600; }
.deployment-panel :deep(.section-card .el-card__body) { padding: 16px 24px 24px; }
.connection-explanation { margin: 0 0 16px; color: var(--text-secondary); font-size: 13px; line-height: 1.6; }
.mode-radio-group { width: 100%; }
.mode-radio-card { min-width: min(210px, 100%); }
.mode-save-row { display: flex; align-items: center; flex-wrap: wrap; gap: 12px; margin-top: 22px; padding-top: 18px; border-top: 1px solid var(--surface-border); }
.mode-save-hint { flex: 1; font-size: 12px; color: var(--text-secondary); }
.deployment-advanced { padding: 16px 20px; border: 1px solid var(--surface-border); border-radius: 12px; margin-top: 18px; }
.deployment-advanced summary { cursor: pointer; color: var(--text-secondary); font-size: 13px; }
.deployment-advanced .el-descriptions { margin-top: 16px; }
@media (max-width: 720px) { .mode-summary { padding: 18px; } .deployment-panel :deep(.section-card .el-card__body) { padding: 14px 18px 20px; } .mode-radio-card { min-width: 100%; } }
</style>
