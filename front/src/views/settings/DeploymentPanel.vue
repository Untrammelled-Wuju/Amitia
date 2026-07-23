<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="deployment-panel">
    <el-card shadow="never" class="section-card">
      <template #header><span>部署模式</span></template>

      <el-radio-group v-model="formMode" class="mode-radio-group">
        <el-radio value="local" border class="mode-radio-card">
          <div class="mode-label">本地模式</div>
          <div class="mode-desc">Core 在本机运行，数据存储在本地</div>
        </el-radio>
        <el-radio value="cloud" border class="mode-radio-card">
          <div class="mode-label">云端模式</div>
          <div class="mode-desc">连接到你部署的远程 Core 服务器</div>
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

      <div
        style="margin-top: 16px; display: flex; gap: 8px; align-items: center"
      >
        <el-button type="primary" @click="handleSave" :loading="saving"
          >保存配置</el-button
        >
        <el-tag v-if="saveSuccess" type="success" size="small" effect="plain"
          >已保存</el-tag
        >
        <div v-if="saveError" class="save-error">{{ saveError }}</div>
      </div>
    </el-card>

    <el-card shadow="never" class="section-card">
      <template #header><span>当前状态</span></template>
      <el-descriptions :column="2" border size="small">
        <el-descriptions-item label="当前模式">
          <el-tag
            :type="currentMode === 'local' ? 'success' : 'warning'"
            size="small"
          >
            {{ currentMode === "local" ? "本地模式" : "云端模式" }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="API 地址">{{
          currentApiURL || "—"
        }}</el-descriptions-item>
        <el-descriptions-item label="运行状态">
          <el-tag :type="statusType" size="small">{{ statusLabel }}</el-tag>
        </el-descriptions-item>
      </el-descriptions>
    </el-card>
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
import { isDesktopShell } from "../../runtime/runtime-capabilities";

const formMode = ref<"local" | "cloud">("local");
const formData = reactive({ serverURL: "" });
const formRef = ref<FormInstance>();
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

    if (!isDesktopShell()) {
      ElMessage.warning("当前运行在浏览器环境，部署模式仅对桌面端有效");
      saving.value = false;
      return;
    }

    await saveDeploymentConfig(config);
    saveSuccess.value = true;
    ElMessage.success("部署配置已保存，部分更改需要重启应用后生效");

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
  font-size: 15px;
  font-weight: 600;
  color: var(--ac-color-text);
  margin-bottom: 4px;
}

.mode-desc {
  font-size: 12px;
  color: var(--ac-color-text-muted);
  line-height: 1.4;
}

.server-url-section {
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--ac-color-border-light);
}

.form-tip {
  font-size: 12px;
  color: var(--ac-color-text-muted);
  margin-top: 4px;
}

.save-error {
  font-size: 12px;
  color: var(--el-color-danger);
}
</style>
