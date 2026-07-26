<template>
  <main class="package-page">
    <ExtensionPageHeader
      title="本地扩展包"
      description="安装和管理本地 .amitiax 扩展包。"
      parent-title="扩展中心"
      parent-path="/extensions"
    />

    <el-alert
      v-if="loadError"
      :title="loadError"
      type="error"
      show-icon
      :closable="false"
    >
      <el-button link type="primary" @click="loadPackages">重新加载</el-button>
    </el-alert>

    <el-tabs v-model="tab" class="package-tabs" @tab-change="onTabChange">
      <el-tab-pane label="安装扩展包" name="install">
        <section class="panel">
          <div class="section-heading">
            <div>
              <h2>安装扩展包</h2>
              <p>仅支持 .amitiax 扩展包。</p>
            </div>
            <el-tag :type="serviceReady ? 'success' : 'danger'">
              {{ serviceReady ? "服务可用" : "服务不可用" }}
            </el-tag>
          </div>
          <div
            class="drop-zone"
            @dragover.prevent
            @drop.prevent="onPackageDrop"
          >
            <el-icon><UploadFilled /></el-icon>
            <strong>选择 .amitiax 扩展包</strong>
            <span>安装前将执行格式与 Manifest 校验</span>
            <el-button
              type="primary"
              :disabled="!serviceReady"
              :loading="installing"
              @click="choosePackage"
            >
              选择扩展包
            </el-button>
            <el-progress
              v-if="installing"
              class="upload-progress"
              :percentage="uploadProgress"
              :status="uploadProgress === 100 ? 'success' : undefined"
            />
            <input
              ref="packageInput"
              class="sr-only"
              type="file"
              accept=".amitiax"
              @change="onPackageFile"
            />
          </div>
        </section>
      </el-tab-pane>

      <el-tab-pane label="已安装扩展包" name="installed">
        <section class="panel">
          <div class="section-heading">
            <div>
              <h2>已安装扩展包</h2>
              <p>共 {{ packages.length }} 个本地扩展包</p>
            </div>
            <el-button :loading="loading" @click="loadPackages">刷新</el-button>
          </div>
          <el-table
            v-loading="loading"
            :data="packages"
            row-key="id"
            empty-text="暂无已安装扩展包"
            stripe
          >
            <el-table-column label="扩展包" min-width="260">
              <template #default="{ row }">
                <strong>{{ row.name }}</strong>
                <div class="package-id">{{ row.id }}</div>
              </template>
            </el-table-column>
            <el-table-column prop="version" label="版本" width="100" />
            <el-table-column prop="publisher" label="发布者" min-width="160" />
            <el-table-column prop="moduleCount" label="模块" width="90" />
            <el-table-column label="安装时间" min-width="180">
              <template #default="{ row }">
                {{ formatTime(row.installedAt) }}
              </template>
            </el-table-column>
          </el-table>
        </section>
      </el-tab-pane>
    </el-tabs>
  </main>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { ElMessage } from "element-plus";
import { UploadFilled } from "@element-plus/icons-vue";
import ExtensionPageHeader from "../components/ExtensionPageHeader.vue";
import type { LocalExtensionPackage } from "../types";
import {
  fetchLocalExtensionPackages,
  fetchLocalExtensionPackageStatus,
  installLocalExtensionPackage,
} from "../api";

const loading = ref(false);
const tab = ref("install");
const installing = ref(false);
const serviceReady = ref(false);
const loadError = ref("");
const uploadProgress = ref(0);
const packages = ref<LocalExtensionPackage[]>([]);
const packageInput = ref<HTMLInputElement>();

async function loadPackages() {
  loading.value = true;
  loadError.value = "";
  try {
    const [status, installed] = await Promise.all([
      fetchLocalExtensionPackageStatus(),
      fetchLocalExtensionPackages(),
    ]);
    serviceReady.value = status.ready;
    packages.value = installed;
  } catch (error: any) {
    serviceReady.value = false;
    packages.value = [];
    loadError.value =
      error?.response?.data?.error || error?.message || "扩展包服务加载失败";
  } finally {
    loading.value = false;
  }
}

async function installPackage(file: File) {
  if (!file.name.toLowerCase().endsWith(".amitiax")) {
    ElMessage.warning("请选择 .amitiax 扩展包");
    return;
  }
  installing.value = true;
  uploadProgress.value = 0;
  try {
    const installed = await installLocalExtensionPackage(file, (value) => {
      uploadProgress.value = value;
    });
    ElMessage.success(`扩展包 ${installed.name || installed.id} 已安装`);
    await loadPackages();
    tab.value = "installed";
  } catch (error: any) {
    ElMessage.error(
      error?.response?.data?.error || error?.message || "扩展包安装失败",
    );
  } finally {
    installing.value = false;
    uploadProgress.value = 0;
  }
}

async function choosePackage() {
  const desktop = window.amitiaDesktop;
  if (!desktop?.selectExtensionPackage) {
    packageInput.value?.click();
    return;
  }
  const selected = await desktop.selectExtensionPackage();
  if (!selected) return;
  const bytes = Uint8Array.from(atob(selected.base64), (character) =>
    character.charCodeAt(0),
  );
  await installPackage(
    new File([bytes], selected.name, { type: "application/zip" }),
  );
}

function onPackageFile(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  if (file) void installPackage(file);
  input.value = "";
}

function onPackageDrop(event: DragEvent) {
  const file = event.dataTransfer?.files?.[0];
  if (file) void installPackage(file);
}

function formatTime(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

function onTabChange(name: string | number) {
  if (name === "installed") void loadPackages();
}

onMounted(loadPackages);
</script>

<style scoped>
.package-page {
  height: 100%;
  overflow: auto;
  color: var(--console-text);
  background: transparent;
}
.panel {
  padding: 20px;
  border: 1px solid var(--console-border);
  border-radius: 14px;
  background: var(--ac-color-surface);
  box-shadow: none;
}
.package-tabs {
  margin-top: 20px;
}
.section-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}
h2,
p {
  margin: 0;
}
.section-heading p {
  margin-top: 6px;
  color: var(--console-text-muted);
}
.drop-zone {
  min-height: 250px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  margin-top: 18px;
  border: 1px dashed var(--el-border-color);
  border-radius: 12px;
  background: var(--el-fill-color-lighter);
  text-align: center;
}
.drop-zone > .el-icon {
  font-size: 44px;
  color: var(--el-color-primary);
}
.drop-zone span,
.package-id {
  color: var(--console-text-muted);
}
.upload-progress {
  width: min(420px, 80%);
}
.package-id {
  margin-top: 4px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  overflow-wrap: anywhere;
}
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}
@media (max-width: 760px) {
  .section-heading {
    flex-direction: column;
  }
}
</style>
