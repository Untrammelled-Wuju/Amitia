<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="deployment-panel" data-amitia-ui="settings.section">
    <el-card shadow="never" class="section-card" data-amitia-ui="settings.card">
      <template #header><span>选择运行模式</span></template>
      <p class="connection-explanation">本地模式在当前设备运行全部项目服务；云端统筹模式连接 Cloud Core 并统一管理多设备数据。</p>

      <el-radio-group v-model="formMode" class="mode-radio-group">
        <el-radio value="local" border class="mode-radio-card" :disabled="!desktopShell">
          <div class="mode-label">本地模式</div>
          <div class="mode-desc">项目服务在本地运行，不限制模型或扩展能力</div>
        </el-radio>
        <el-radio value="cloud" border class="mode-radio-card">
          <div class="mode-label">云端统筹模式</div>
          <div class="mode-desc">连接远程 Core，统一管理角色、数据和跨设备任务</div>
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
              data-amitia-ui="settings.input"
              placeholder="例如 http://192.168.1.100:端口"
              clearable
            />
            <div class="form-tip">
              输入你部署的 Core 服务器完整地址，包含协议和端口
            </div>
          </el-form-item>
        </el-form>
        <ol class="cloud-requirements">
          <li>验证 Cloud Core 健康状态、空间身份和运行时兼容性</li>
          <li>当前设备完成首设备设置码或一次性 Offer 配对</li>
          <li>保存 DeviceCredential、切换本机 Device Agent 并验证会话</li>
        </ol>
        <DevicePairingPanel :serverURL="formData.serverURL" @paired="handleDevicePaired" />
        <BoundDevicesPanel :key="deviceManagementGeneration" />
      </div>

      <div class="mode-save-row">
        <span class="mode-save-hint">
          {{ formMode === "cloud" ? "填写地址不会直接切换，必须完成配对和凭证验证。" : "保存后业务连接会重新建立，不迁移当前运行数据。" }}
        </span>
        <el-tag v-if="saveSuccess" type="success" size="small" effect="plain">已保存</el-tag>
        <div v-if="saveError" class="save-error">{{ saveError }}</div>
        <el-button type="primary" data-amitia-ui="settings.button" @click="handleSave" :loading="saving">
          {{ formMode === "cloud" ? "检查、配对并切换" : "保存本地模式" }}
        </el-button>
      </div>
    </el-card>

    <el-card v-if="formMode === 'cloud' && cloudSetupURL" shadow="never" class="section-card">
      <template #header><span>云端统筹设备配对</span></template>
      <StageRuntimeSetup
        deploy-mode="remote"
        :serverURL="cloudSetupURL"
        :before-verify="prepareCloudConnection"
        :after-verification-failed="rollbackCloudSetup"
        @health-check-done="completeCloudSetup"
      />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, watch } from "vue";
import { ElMessage } from "element-plus";
import type { FormInstance, FormRules } from "element-plus";
import {
  getDeploymentConfig,
  saveDeploymentConfig,
} from "../../runtime/runtime-adapter";
import type { DeploymentModeConfig } from "../../runtime/runtime-types";
import { initializeRuntimeCapabilities, isDesktopShell } from "../../runtime/runtime-capabilities";
import StageRuntimeSetup from "@/views/onboarding/components/StageRuntimeSetup.vue";
import DevicePairingPanel from "@/components/DevicePairingPanel.vue";
import BoundDevicesPanel from "@/components/BoundDevicesPanel.vue";

const formMode = ref<"local" | "cloud">("local");
const formData = reactive({ serverURL: "" });
const formRef = ref<FormInstance>();
const desktopShell = isDesktopShell();
const saving = ref(false);
const saveSuccess = ref(false);
const saveError = ref("");

const cloudSetupURL = ref("");
const cloudSetupSaving = ref(false);
const cloudConfigPrepared = ref(false);
const cloudPreviousConfig = ref<DeploymentModeConfig | null>(null);
const deviceManagementGeneration = ref(0);

watch(formMode, (value) => {
  if (value === "local") cloudSetupURL.value = "";
});

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

async function loadConfig() {
  try {
    const config = await getDeploymentConfig();
    formMode.value = config.mode;
    if (config.mode === "cloud" && config.serverURL) {
      formData.serverURL = config.serverURL;
    }
  } catch (err) {
    console.error("加载部署配置失败:", err);
  }
}

function handleDevicePaired(baseURL: string) {
  deviceManagementGeneration.value++;
  formMode.value = "cloud";
  formData.serverURL = baseURL;
  saveSuccess.value = true;
  saveError.value = "";
  cloudSetupURL.value = "";
  cloudConfigPrepared.value = false;
  cloudPreviousConfig.value = null;
}

async function handleSave() {
  saveError.value = "";
  saveSuccess.value = false;

  if (formMode.value === "cloud") {
    if (!formRef.value) return;
    try {
      await formRef.value.validate();
    } catch (error) {
  console.error("[original-error]", error);
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

    if (config.mode === "cloud") {
      cloudSetupURL.value = config.serverURL || "";
      cloudConfigPrepared.value = false;
      cloudPreviousConfig.value = await getDeploymentConfig();
      return;
    }
    await saveDeploymentConfig(config);
    await initializeRuntimeCapabilities(true);
    saveSuccess.value = true;
    ElMessage.success("本地模式已保存，业务连接会重新建立");
  } catch (err: any) {
    saveError.value = err?.message || "保存失败";
    ElMessage.error(String(err?.message || err));
  } finally {
    saving.value = false;
  }
}

async function prepareCloudConnection(baseURL: string) {
  if (cloudConfigPrepared.value) return;
  await saveDeploymentConfig({ mode: "cloud", serverURL: baseURL });
  await initializeRuntimeCapabilities(true);
  cloudConfigPrepared.value = true;
}

async function rollbackCloudSetup(_baseURL: string, error: unknown) {
  const previous = cloudPreviousConfig.value;
  cloudConfigPrepared.value = false;
  cloudSetupURL.value = "";
  saveSuccess.value = false;
  saveError.value = error instanceof Error ? error.message : "云端统筹凭证验证失败";
  if (!previous) return;
  try {
    await saveDeploymentConfig(previous);
    await initializeRuntimeCapabilities(true);
    formMode.value = previous.mode;
  } catch (error) {
  console.error("[original-error]", error);
saveError.value = `${saveError.value}；${error instanceof Error ? error.message : String(error)}`;
  }
  cloudPreviousConfig.value = null;
}

async function completeCloudSetup() {
  if (!cloudSetupURL.value || cloudSetupSaving.value) return;
  cloudSetupSaving.value = true;
  try {
    if (!cloudConfigPrepared.value) {
      await prepareCloudConnection(cloudSetupURL.value);
    }
    saveSuccess.value = true;
    ElMessage.success("设备已完成云端统筹配对、凭证验证并切换");
    formMode.value = "cloud";
    cloudSetupURL.value = "";
    cloudConfigPrepared.value = false;
    cloudPreviousConfig.value = null;
    deviceManagementGeneration.value++;
  } catch (err: any) {
    saveError.value = err?.message || "保存云端统筹设置失败";
    ElMessage.error(String(err?.message || err));
  } finally {
    cloudSetupSaving.value = false;
  }
}

onMounted(loadConfig);
</script>

<style scoped>
.deployment-panel {
}
.section-card {
  margin-bottom: 12px;
  border: 1px solid var(--ac-color-border-light);
}
.cloud-requirements {
  margin: 0 0 14px;
  padding-left: 20px;
  color: var(--ac-color-text-muted);
  font-size: 12px;
  line-height: 1.7;
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
.deployment-panel :deep(.section-card) { border: 1px solid var(--surface-border); border-radius: 14px; background: var(--surface-bg); margin-bottom: 0; }
.deployment-panel :deep(.section-card .el-card__header) { padding: 20px 24px 8px; border-bottom: 0; font-weight: 600; }
.deployment-panel :deep(.section-card .el-card__body) { padding: 16px 24px 24px; }
.connection-explanation { margin: 0 0 16px; color: var(--text-secondary); font-size: 13px; line-height: 1.6; }
.mode-radio-group { width: 100%; }
.mode-radio-card { min-width: min(210px, 100%); }
.mode-save-row { display: flex; align-items: center; flex-wrap: wrap; gap: 12px; margin-top: 22px; padding-top: 18px; border-top: 1px solid var(--surface-border); }
.mode-save-hint { flex: 1; font-size: 12px; color: var(--text-secondary); }
@media (max-width: 720px) { .deployment-panel :deep(.section-card .el-card__body) { padding: 14px 18px 20px; } .mode-radio-card { min-width: 100%; } }
</style>
