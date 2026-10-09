<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="runtime-mode-page">
    <h2 class="page-title">
      <el-icon><Monitor /></el-icon>
      Core 配置与校验
    </h2>

    <p class="page-description">仅配置当前 Core 服务端的部署形态，与上方当前设备连接哪个 Core 无关。</p>
    <ModeConfigPanel
      :mode="mode"
      :validating="validating"
      :validation-result="validationResult"
      :cloud-checklist="cloudChecklist"
      @update:cloud-checklist="cloudChecklist = $event"
      @validate="runValidate"
    />
    <ModeSwitchPanel
      :mode-deploy-mode="mode.deployMode"
      :switching="switching"
      @confirm-switch="confirmSwitch"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from "vue";
import { ElMessage } from "element-plus";
import { Monitor } from "@element-plus/icons-vue";
import ModeConfigPanel from "./components/ModeConfigPanel.vue";
import ModeSwitchPanel from "./components/ModeSwitchPanel.vue";
import { fetchModeApi, switchModeApi, validateModeApi } from "./api";
import type {
  RuntimeModeResponse,
  RuntimeModeValidationResult,
  DeployMode,
} from "@/types";

const mode = reactive<RuntimeModeResponse>({
  deployMode: "desktop-local",
  host: "127.0.0.1",
  port: 18080,
  web: { enabled: true, publicBaseUrl: "", requireAuth: true },
  bridge: { enabled: true, mode: "cloud", host: "127.0.0.1", port: 8898 },
  storage: { dataDir: "./data" },
});

const switching = ref(false);
const validating = ref(false);
const validationResult = ref<RuntimeModeValidationResult | null>(null);
const cloudChecklist = ref<string[]>([]);

onMounted(async () => {
  await fetchMode();
});

async function fetchMode() {
  const data = await fetchModeApi();
  if (!data) return;
  if (typeof data.deployMode === "string") mode.deployMode = data.deployMode;
  if (typeof data.host === "string") mode.host = data.host;
  if (Number.isFinite(data.port)) mode.port = data.port;
  if (data.web && typeof data.web === "object") Object.assign(mode.web, data.web);
  if (data.bridge && typeof data.bridge === "object") Object.assign(mode.bridge, data.bridge);
  if (data.storage && typeof data.storage === "object") Object.assign(mode.storage, data.storage);
}

async function confirmSwitch(targetMode: DeployMode) {
  switching.value = true;
  try {
    await switchModeApi(targetMode);
    await fetchMode();
    ElMessage.success(
      `已更新当前 Core 的${mode.deployMode === "cloud-web" ? "私有云部署" : "本机部署"}配置。可能需要重启 Core 生效；这不会自动迁移服务或更改设备连接目标。`,
    );
  } catch (err: any) {
    ElMessage.error(
      "切换失败: " + (err.response?.data?.message || err.message),
    );
  } finally {
    switching.value = false;
  }
}

async function runValidate() {
  validating.value = true;
  try {
    const data = await validateModeApi();
    validationResult.value = data;

    if (data.valid) {
      ElMessage.success("配置校验通过");
    } else if ((data as any).errors?.length) {
      ElMessage.warning(`发现 ${(data as any).errors.length} 个错误`);
    } else if ((data as any).warnings?.length) {
      ElMessage.info(`发现 ${(data as any).warnings.length} 个警告`);
    }
  } catch (err: any) {
    ElMessage.error(
      "校验失败: " + (err.response?.data?.message || err.message),
    );
  } finally {
    validating.value = false;
  }
}
</script>

<style scoped>
.runtime-mode-page {
  padding: 20px;
  max-width: 780px;
}
.page-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--ac-color-text);
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}
.page-description {
  margin: 0 0 20px;
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.6;
}
</style>
