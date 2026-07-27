<template>
  <main class="package-page">
    <ExtensionPageHeader
      title="扩展包"
      description="安装和管理 .amitiax 扩展包。"
      parent-title="扩展中心"
      parent-path="/extensions"
    >
      <template #actions>
        <el-tag :type="statusReady ? 'success' : 'danger'" size="large">
          {{ statusReady ? '内核在线' : '内核离线' }}
        </el-tag>
        <el-tag v-if="statusCount !== undefined" type="info" size="large">
          已安装 {{ statusCount }}
        </el-tag>
      </template>
    </ExtensionPageHeader>

    <el-alert
      v-if="loadError"
      :title="loadError"
      type="error"
      show-icon
      :closable="false"
    >
      <el-button link type="primary" @click="refreshList">重新加载</el-button>
    </el-alert>

    <div class="tab-bar">
      <h3
        v-for="item in allTabs"
        :key="item.key"
        class="tab-item"
        :class="{ active: tab === item.key }"
        @click="switchTab(item.key)"
      >{{ item.label }}</h3>
    </div>

    <section v-show="tab === 'install'" class="panel">
      <div class="section-heading">
        <div>
          <h2>安装扩展包</h2>
          <p>仅支持 .amitiax 扩展包，安装前将执行安全检查与预览。</p>
        </div>
        <el-tag :type="statusReady ? 'success' : 'danger'">
          {{ statusReady ? "服务可用" : "服务不可用" }}
        </el-tag>
      </div>
      <div
        class="drop-zone"
        @dragover.prevent
        @drop.prevent="onPackageDrop"
      >
        <el-icon><UploadFilled /></el-icon>
        <strong>选择或拖入 .amitiax 扩展包</strong>
        <span>安装前将执行格式与 Manifest 校验</span>
        <el-button
          type="primary"
          :disabled="!statusReady"
          @click="choosePackage"
        >
          选择扩展包
        </el-button>
        <input
          ref="packageInput"
          class="sr-only"
          type="file"
          accept=".amitiax"
          @change="onPackageFile"
        />
      </div>
      <div v-if="selectedFile" class="install-actions">
        <el-tag type="info" closable @close="clearSelectedFile">
          {{ selectedFile.name }}
        </el-tag>
        <el-button
          type="success"
          :loading="previewLoading"
          @click="doPreview"
        >
          预览安装
        </el-button>
        <el-button
          v-if="previewResult && previewResult.installable"
          type="warning"
          :loading="installLoading"
          @click="doInstall"
        >
          确认安装
        </el-button>
      </div>
    </section>

    <section v-show="tab === 'installed'" class="panel">
      <div class="section-heading">
        <div>
          <h2>已安装扩展包</h2>
          <p>共 {{ extensions.length }} 个扩展包</p>
        </div>
        <el-button :loading="listLoading" @click="refreshList">刷新</el-button>
      </div>
      <el-table
        :data="extensions"
        v-loading="listLoading"
        border
        style="width: 100%"
        empty-text="暂无已安装扩展包"
      >
        <el-table-column prop="extensionId" label="扩展ID" min-width="200" show-overflow-tooltip />
        <el-table-column prop="version" label="版本" width="100" />
        <el-table-column label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="row.state === 'installed' ? 'success' : 'warning'" size="small">
              {{ row.state }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="启用" width="100">
          <template #default="{ row }">
            <el-tag :type="row.enablement === 'enabled' ? 'success' : 'info'" size="small">
              {{ row.enablement === 'enabled' ? '已启用' : '已禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="installedAt" label="安装时间" width="180">
          <template #default="{ row }">
            {{ formatDate(row.installedAt) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="520" fixed="right">
          <template #default="{ row }">
            <el-button
              v-if="row.enablement !== 'enabled'"
              size="small"
              type="success"
              @click="toggleEnable(row, true)"
            >启用</el-button>
            <el-button
              v-else
              size="small"
              type="warning"
              @click="toggleEnable(row, false)"
            >禁用</el-button>
            <el-button
              v-if="row.enablement === 'enabled'"
              size="small"
              type="info"
              @click="doPause(row)"
            >暂停</el-button>
            <el-button
              size="small"
              type="primary"
              :loading="updateChecking[row.extensionId]"
              @click="checkRowUpdate(row)"
            >更新</el-button>
            <el-button
              size="small"
              @click="doRollback(row)"
            >回滚</el-button>
            <el-button
              size="small"
              @click="goDiagnose"
            >诊断</el-button>
            <el-button
              size="small"
              type="primary"
              @click="openDetail(row)"
            >详情</el-button>
            <el-button
              size="small"
              type="danger"
              @click="doUninstall(row)"
            >卸载</el-button>
          </template>
        </el-table-column>
      </el-table>
    </section>

    <section v-show="!['install', 'installed'].includes(tab)" class="panel kernel-view-panel">
      <Suspense>
        <component :is="currentKernelComponent" />
        <template #fallback>
          <div class="kernel-loading">
            <el-icon class="loading-icon" :size="24"><Loading /></el-icon>
            <span>加载中...</span>
          </div>
        </template>
      </Suspense>
    </section>

    <el-dialog v-model="previewVisible" title="安装预览" width="640px">
      <div v-if="previewResult" class="preview-dialog">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="扩展ID">{{ previewResult.extensionId }}</el-descriptions-item>
          <el-descriptions-item label="名称">{{ previewResult.name }}</el-descriptions-item>
          <el-descriptions-item label="版本">{{ previewResult.version }}</el-descriptions-item>
          <el-descriptions-item label="发布者">{{ previewResult.publisher }}</el-descriptions-item>
          <el-descriptions-item label="安全检查">
            <el-tag :type="previewResult.securityPassed ? 'success' : 'danger'">
              {{ previewResult.securityPassed ? '通过' : '未通过' }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="可安装">
            <el-tag :type="previewResult.installable ? 'success' : 'warning'">
              {{ previewResult.installable ? '可安装' : '不可安装' }}
            </el-tag>
          </el-descriptions-item>
        </el-descriptions>

        <div v-if="previewResult.issues && previewResult.issues.length > 0" class="preview-section">
          <h4>问题列表</h4>
          <el-table :data="previewResult.issues" size="small" border>
            <el-table-column prop="category" label="类别" width="140" />
            <el-table-column prop="code" label="代码" width="160" />
            <el-table-column prop="message" label="描述" />
          </el-table>
        </div>

        <div v-if="previewResult.modules && previewResult.modules.length > 0" class="preview-section">
          <h4>模块列表</h4>
          <el-table :data="previewResult.modules" size="small" border>
            <el-table-column prop="id" label="ID" width="160" />
            <el-table-column prop="name" label="名称" />
            <el-table-column prop="type" label="类型" width="100" />
            <el-table-column prop="runtime" label="运行时" width="100" />
            <el-table-column label="支持" width="80">
              <template #default="{ row }">
                <el-tag :type="row.supported ? 'success' : 'danger'" size="small">
                  {{ row.supported ? '是' : '否' }}
                </el-tag>
              </template>
            </el-table-column>
          </el-table>
        </div>
      </div>
    </el-dialog>

    <el-dialog v-model="detailVisible" title="扩展详情" width="820px">
      <div v-if="detail">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="扩展ID">{{ detail.extensionId }}</el-descriptions-item>
          <el-descriptions-item label="版本">{{ detail.version }}</el-descriptions-item>
          <el-descriptions-item label="安装ID">{{ detail.installationId }}</el-descriptions-item>
          <el-descriptions-item label="状态">{{ detail.state }}</el-descriptions-item>
          <el-descriptions-item label="启用">{{ detail.enablement }}</el-descriptions-item>
          <el-descriptions-item label="Generation">{{ detail.generation }}</el-descriptions-item>
          <el-descriptions-item label="发布者">{{ detail.publisher || '-' }}</el-descriptions-item>
          <el-descriptions-item label="签名状态">
            <el-tag v-if="detail.signatureStatus" :type="detail.signatureStatus === 'verified' ? 'success' : 'warning'" size="small">
              {{ detail.signatureStatus }}
            </el-tag>
            <span v-else>-</span>
          </el-descriptions-item>
          <el-descriptions-item label="Trust 级别">
            <el-tag v-if="detail.trustLevel" :type="detail.trustLevel === 'trusted' ? 'success' : 'info'" size="small">
              {{ detail.trustLevel }}
            </el-tag>
            <span v-else>-</span>
          </el-descriptions-item>
          <el-descriptions-item label="运行时状态">
            <el-tag v-if="detail.runtimeStatus" :type="detail.runtimeStatus === 'running' ? 'success' : 'info'" size="small">
              {{ detail.runtimeStatus }}
            </el-tag>
            <span v-else>-</span>
          </el-descriptions-item>
          <el-descriptions-item label="Circuit 状态">
            <el-tag v-if="detail.circuitState" :type="detail.circuitState === 'closed' ? 'success' : 'danger'" size="small">
              {{ detail.circuitState }}
            </el-tag>
            <span v-else>-</span>
          </el-descriptions-item>
          <el-descriptions-item label="Quarantine 状态">
            <el-tag v-if="detail.quarantineState" :type="detail.quarantineState === 'none' ? 'success' : 'warning'" size="small">
              {{ detail.quarantineState }}
            </el-tag>
            <span v-else>-</span>
          </el-descriptions-item>
        </el-descriptions>

        <div v-if="detail.permissions && detail.permissions.length > 0" class="detail-section">
          <h4>权限列表</h4>
          <el-table :data="detail.permissions" size="small" border>
            <el-table-column prop="name" label="名称" width="180" />
            <el-table-column prop="scope" label="Scope" width="120" />
            <el-table-column prop="reason" label="原因" show-overflow-tooltip />
            <el-table-column label="必需" width="60">
              <template #default="{ row }">
                <el-tag :type="row.required ? 'danger' : 'info'" size="small">
                  {{ row.required ? '是' : '否' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="已授予" width="80">
              <template #default="{ row }">
                <el-tag :type="row.granted ? 'success' : 'warning'" size="small">
                  {{ row.granted ? '是' : '否' }}
                </el-tag>
              </template>
            </el-table-column>
          </el-table>
        </div>

        <div v-if="detail.scopes && detail.scopes.length > 0" class="detail-section">
          <h4>Scope 列表</h4>
          <el-table :data="detail.scopes" size="small" border>
            <el-table-column prop="name" label="名称" />
            <el-table-column label="已授予" width="80">
              <template #default="{ row }">
                <el-tag :type="row.granted ? 'success' : 'warning'" size="small">
                  {{ row.granted ? '是' : '否' }}
                </el-tag>
              </template>
            </el-table-column>
          </el-table>
        </div>

        <div v-if="detail.dependencies && detail.dependencies.length > 0" class="detail-section">
          <h4>依赖列表</h4>
          <el-table :data="detail.dependencies" size="small" border>
            <el-table-column prop="type" label="类型" width="100" />
            <el-table-column prop="id" label="ID" width="200" />
            <el-table-column prop="version" label="版本" width="100" />
            <el-table-column label="可选" width="60">
              <template #default="{ row }">
                <el-tag :type="row.optional ? 'info' : 'warning'" size="small">
                  {{ row.optional ? '是' : '否' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="已满足" width="80">
              <template #default="{ row }">
                <el-tag :type="row.satisfied ? 'success' : 'danger'" size="small">
                  {{ row.satisfied ? '是' : '否' }}
                </el-tag>
              </template>
            </el-table-column>
          </el-table>
        </div>

        <div v-if="detail.modules && detail.modules.length > 0" class="detail-section">
          <h4>模块</h4>
          <el-table :data="detail.modules" size="small" border>
            <el-table-column prop="id" label="ID" width="140" />
            <el-table-column prop="type" label="类型" width="100" />
            <el-table-column prop="runtime" label="运行时" width="100" />
            <el-table-column prop="entryPoint" label="入口" show-overflow-tooltip />
            <el-table-column prop="contributionCount" label="贡献数" width="80" />
          </el-table>
        </div>

        <div v-if="detail.contributions && detail.contributions.length > 0" class="detail-section">
          <h4>贡献点</h4>
          <el-table :data="detail.contributions" size="small" border>
            <el-table-column prop="id" label="ID" width="180" />
            <el-table-column prop="kind" label="类型" width="120" />
            <el-table-column prop="moduleId" label="模块ID" width="140" />
            <el-table-column prop="name" label="名称" />
            <el-table-column label="操作" width="100">
              <template #default="{ row }">
                <el-button
                  v-if="row.kind === 'ui_page'"
                  size="small"
                  type="primary"
                  @click="openExtPage(row.id)"
                >打开页面</el-button>
              </template>
            </el-table-column>
          </el-table>
        </div>
      </div>
    </el-dialog>

    <el-dialog v-model="updateDialogVisible" title="扩展更新" width="680px">
      <div v-if="updateTarget">
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item label="扩展ID">{{ updateTarget.extensionId }}</el-descriptions-item>
          <el-descriptions-item label="当前版本">{{ updateTarget.version }}</el-descriptions-item>
        </el-descriptions>

        <div v-if="updateMeta" class="update-meta-section">
          <h4>可用更新</h4>
          <el-descriptions :column="2" border size="small">
            <el-descriptions-item label="新版本">{{ updateMeta.version }}</el-descriptions-item>
            <el-descriptions-item label="发布通道">{{ updateMeta.releaseChannel || '-' }}</el-descriptions-item>
            <el-descriptions-item label="发布者">{{ updateMeta.publisherId }}</el-descriptions-item>
            <el-descriptions-item label="包大小">{{ formatSize(updateMeta.packageSize) }}</el-descriptions-item>
            <el-descriptions-item label="发布时间">{{ formatDate(updateMeta.publishedAt) }}</el-descriptions-item>
            <el-descriptions-item label="Manifest版本">{{ updateMeta.manifestVersion }}</el-descriptions-item>
          </el-descriptions>
          <div class="update-actions">
            <el-button type="primary" :loading="updateActionLoading" @click="doDownloadUpdate">下载</el-button>
            <el-button type="success" :loading="updateActionLoading" :disabled="!updateOperationId" @click="doInstallUpdate">安装</el-button>
            <el-button type="warning" :loading="updateActionLoading" :disabled="!updateOperationId" @click="doCancelUpdate">取消</el-button>
            <el-button :loading="updateActionLoading" :disabled="!updateOperationId" @click="doRetryUpdate">重试</el-button>
            <el-button type="danger" :loading="updateActionLoading" :disabled="!updateOperationId" @click="doRollbackUpdate">回滚</el-button>
          </div>
        </div>

        <div v-if="updateOperation" class="update-op-section">
          <h4>当前操作</h4>
          <el-descriptions :column="2" border size="small">
            <el-descriptions-item label="操作ID">{{ updateOperation.operationId }}</el-descriptions-item>
            <el-descriptions-item label="状态">
              <el-tag :type="opStatusTagType(updateOperation.status)" size="small">{{ updateOperation.status }}</el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="版本">{{ updateOperation.version || '-' }}</el-descriptions-item>
            <el-descriptions-item label="创建时间">{{ formatDate(updateOperation.createdAt) }}</el-descriptions-item>
            <el-descriptions-item v-if="updateOperation.error" label="错误" :span="2">{{ updateOperation.error }}</el-descriptions-item>
          </el-descriptions>
        </div>
      </div>
    </el-dialog>
  </main>
</template>

<script setup lang="ts">
import { defineAsyncComponent, onMounted, ref, shallowRef, watch } from "vue";
import { useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import { UploadFilled, Loading } from "@element-plus/icons-vue";
import ExtensionPageHeader from "../components/ExtensionPageHeader.vue";
import {
  getKernelStatus,
  listExtensions,
  getExtension,
  previewInstall,
  installExtension,
  enableExtension,
  disableExtension,
  uninstallExtension,
  pauseExtension,
  rollbackExtension,
  type KernelStatus,
  type KernelExtension,
  type KernelExtensionDetail,
  type InstallPreview,
} from "@/views/kernel/api";
import {
  checkUpdates,
  downloadUpdate,
  installUpdate,
  cancelUpdate,
  retryUpdate,
  rollbackUpdate,
} from "@/api/desktop";
import type {
  ExtensionUpdateMeta,
  UpdateOperationInfo,
} from "@/api/desktop";

const router = useRouter();

const tab = ref("install");
const loadError = ref("");

const statusReady = ref(false);
const statusCount = ref<number | undefined>(undefined);
const extensions = ref<KernelExtension[]>([]);
const listLoading = ref(false);

const selectedFile = ref<File | null>(null);
const previewLoading = ref(false);
const previewResult = ref<InstallPreview | null>(null);
const previewVisible = ref(false);
const installLoading = ref(false);

const detailVisible = ref(false);
const detail = ref<KernelExtensionDetail | null>(null);

const packageInput = ref<HTMLInputElement>();

const allTabs = [
  { key: "install", label: "安装扩展包" },
  { key: "installed", label: "已安装扩展包" },
  { key: "trusted-services", label: "可信服务运行时" },
  { key: "wasm", label: "WASM 运行时" },
  { key: "hooks", label: "Hook 中心" },
  { key: "tasks", label: "任务运行时" },
  { key: "events", label: "事件中心" },
  { key: "schedules", label: "调度中心" },
  { key: "desktop", label: "桌面贡献中心" },
  { key: "dev-console", label: "开发者诊断控制台" },
  { key: "migrations", label: "迁移与灰度中心" },
  { key: "dev-mode", label: "开发模式中心" },
];

const kernelComponentMap: Record<string, () => Promise<any>> = {
  "trusted-services": () => import("@/views/kernel/TrustedServiceView.vue"),
  "wasm": () => import("@/views/kernel/WasmRuntimeDetailView.vue"),
  "hooks": () => import("@/views/kernel/HookCenterView.vue"),
  "tasks": () => import("@/views/kernel/tasks/TaskCenterView.vue"),
  "events": () => import("@/views/kernel/EventCenterView.vue"),
  "schedules": () => import("@/views/kernel/ScheduleCenterView.vue"),
  "desktop": () => import("@/views/kernel/DesktopCenterView.vue"),
  "dev-console": () => import("@/views/kernel/DeveloperConsoleView.vue"),
  "migrations": () => import("@/views/kernel/MigrationCenterView.vue"),
  "dev-mode": () => import("@/views/kernel/DevModeView.vue"),
};

const currentKernelComponent = shallowRef<any>(null);

watch(tab, (newTab) => {
  if (newTab !== "install" && newTab !== "installed" && kernelComponentMap[newTab]) {
    currentKernelComponent.value = defineAsyncComponent(kernelComponentMap[newTab]);
  }
}, { immediate: true });

async function loadStatus() {
  try {
    const s: KernelStatus = await getKernelStatus();
    statusReady.value = s.ready;
    statusCount.value = s.count;
  } catch {
    statusReady.value = false;
  }
}

async function refreshList() {
  listLoading.value = true;
  loadError.value = "";
  try {
    await loadStatus();
    const data = await listExtensions();
    extensions.value = data.extensions || [];
    statusCount.value = data.total;
  } catch (e: any) {
    loadError.value = "扩展包服务加载失败: " + (e?.message || e);
    statusReady.value = false;
    extensions.value = [];
  } finally {
    listLoading.value = false;
  }
}

function clearSelectedFile() {
  selectedFile.value = null;
  previewResult.value = null;
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
  selectedFile.value = new File([bytes], selected.name, { type: "application/zip" });
  previewResult.value = null;
  ElMessage.info("已选择文件: " + selected.name);
}

function onPackageFile(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  if (file) {
    if (!file.name.toLowerCase().endsWith(".amitiax")) {
      ElMessage.warning("请选择 .amitiax 扩展包");
      input.value = "";
      return;
    }
    selectedFile.value = file;
    previewResult.value = null;
    ElMessage.info("已选择文件: " + file.name);
  }
  input.value = "";
}

function onPackageDrop(event: DragEvent) {
  const file = event.dataTransfer?.files?.[0];
  if (!file) return;
  if (!file.name.toLowerCase().endsWith(".amitiax")) {
    ElMessage.warning("请选择 .amitiax 扩展包");
    return;
  }
  selectedFile.value = file;
  previewResult.value = null;
  ElMessage.info("已选择文件: " + file.name);
}

async function doPreview() {
  if (!selectedFile.value) {
    ElMessage.warning("请先选择扩展包");
    return;
  }
  previewLoading.value = true;
  try {
    const result = await previewInstall(selectedFile.value);
    previewResult.value = result;
    previewVisible.value = true;
  } catch (e: any) {
    ElMessage.error("预览失败: " + (e?.message || e));
  } finally {
    previewLoading.value = false;
  }
}

async function doInstall() {
  if (!selectedFile.value) {
    ElMessage.warning("请先选择扩展包");
    return;
  }
  installLoading.value = true;
  try {
    const result = await installExtension(selectedFile.value);
    ElMessage.success(`安装成功: ${result.extensionId} v${result.version}`);
    previewVisible.value = false;
    selectedFile.value = null;
    previewResult.value = null;
    await refreshList();
    tab.value = "installed";
  } catch (e: any) {
    ElMessage.error("安装失败: " + (e?.message || e));
  } finally {
    installLoading.value = false;
  }
}

async function toggleEnable(row: KernelExtension, enable: boolean) {
  try {
    if (enable) {
      await enableExtension(row.extensionId);
      ElMessage.success("已启用: " + row.extensionId);
    } else {
      await disableExtension(row.extensionId);
      ElMessage.success("已禁用: " + row.extensionId);
    }
    await refreshList();
  } catch (e: any) {
    ElMessage.error("操作失败: " + (e?.message || e));
  }
}

async function doUninstall(row: KernelExtension) {
  try {
    await ElMessageBox.confirm(
      `确定要卸载扩展 ${row.extensionId} v${row.version} 吗？`,
      "卸载确认",
      { type: "warning" }
    );
    await uninstallExtension(row.extensionId);
    ElMessage.success("已卸载: " + row.extensionId);
    await refreshList();
  } catch (e: any) {
    if (e !== "cancel" && e?.message !== "cancel") {
      ElMessage.error("卸载失败: " + (e?.message || e));
    }
  }
}

async function doPause(row: KernelExtension) {
  try {
    await ElMessageBox.confirm(
      `确定要暂停扩展 ${row.extensionId} 吗？`,
      "暂停确认",
      { type: "warning" }
    );
    await pauseExtension(row.extensionId);
    ElMessage.success("已暂停: " + row.extensionId);
    await refreshList();
  } catch (e: any) {
    if (e !== "cancel" && e?.message !== "cancel") {
      ElMessage.error("暂停失败: " + (e?.message || e));
    }
  }
}

async function doRollback(row: KernelExtension) {
  try {
    await ElMessageBox.confirm(
      `确定要回滚扩展 ${row.extensionId} 吗？`,
      "回滚确认",
      { type: "warning" }
    );
    await rollbackExtension(row.extensionId);
    ElMessage.success("已回滚: " + row.extensionId);
    await refreshList();
  } catch (e: any) {
    if (e !== "cancel" && e?.message !== "cancel") {
      ElMessage.error("回滚失败: " + (e?.message || e));
    }
  }
}

async function openDetail(row: KernelExtension) {
  try {
    const data = await getExtension(row.extensionId);
    detail.value = data;
    detailVisible.value = true;
  } catch (e: any) {
    ElMessage.error("加载详情失败: " + (e?.message || e));
  }
}

const updateChecking = ref<Record<string, boolean>>({});
const updateDialogVisible = ref(false);
const updateTarget = ref<KernelExtension | null>(null);
const updateMeta = ref<ExtensionUpdateMeta | null>(null);
const updateOperation = ref<UpdateOperationInfo | null>(null);
const updateOperationId = ref("");
const updateActionLoading = ref(false);

function opStatusTagType(status: string): "success" | "info" | "warning" | "danger" {
  if (status === "completed" || status === "installed" || status === "success") return "success";
  if (status === "pending" || status === "downloading" || status === "installing" || status === "queued") return "warning";
  if (status === "failed" || status === "cancelled") return "danger";
  return "info";
}

function formatSize(bytes: number): string {
  if (!bytes && bytes !== 0) return "-";
  if (bytes < 1024) return bytes + " B";
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(2) + " KB";
  if (bytes < 1024 * 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(2) + " MB";
  return (bytes / (1024 * 1024 * 1024)).toFixed(2) + " GB";
}

async function checkRowUpdate(row: KernelExtension) {
  updateChecking.value[row.extensionId] = true;
  try {
    const result = await checkUpdates(row.extensionId);
    updateTarget.value = row;
    updateOperation.value = null;
    updateOperationId.value = "";
    if (result.available && result.update) {
      updateMeta.value = result.update;
      ElMessage.success(`发现可用更新: ${result.update.version}`);
    } else {
      updateMeta.value = null;
      ElMessage.info("当前已是最新版本");
    }
    updateDialogVisible.value = true;
  } catch (e: any) {
    ElMessage.error("检查更新失败: " + (e?.message || e));
  } finally {
    updateChecking.value[row.extensionId] = false;
  }
}

async function doDownloadUpdate() {
  if (!updateTarget.value || !updateMeta.value) {
    ElMessage.warning("无可用更新");
    return;
  }
  updateActionLoading.value = true;
  try {
    const op = await downloadUpdate(updateTarget.value.extensionId, updateMeta.value.version);
    updateOperation.value = op;
    updateOperationId.value = op.operationId;
    ElMessage.success("下载已开始: " + op.operationId);
  } catch (e: any) {
    ElMessage.error("下载失败: " + (e?.message || e));
  } finally {
    updateActionLoading.value = false;
  }
}

async function doInstallUpdate() {
  if (!updateTarget.value || !updateOperationId.value) {
    ElMessage.warning("无可用操作");
    return;
  }
  updateActionLoading.value = true;
  try {
    const op = await installUpdate(updateTarget.value.extensionId, updateOperationId.value);
    updateOperation.value = op;
    ElMessage.success("安装已触发");
    await refreshList();
  } catch (e: any) {
    ElMessage.error("安装失败: " + (e?.message || e));
  } finally {
    updateActionLoading.value = false;
  }
}

async function doCancelUpdate() {
  if (!updateTarget.value || !updateOperationId.value) return;
  updateActionLoading.value = true;
  try {
    const op = await cancelUpdate(updateTarget.value.extensionId, updateOperationId.value);
    updateOperation.value = op;
    ElMessage.success("操作已取消");
  } catch (e: any) {
    ElMessage.error("取消失败: " + (e?.message || e));
  } finally {
    updateActionLoading.value = false;
  }
}

async function doRetryUpdate() {
  if (!updateTarget.value || !updateOperationId.value) return;
  updateActionLoading.value = true;
  try {
    const op = await retryUpdate(updateTarget.value.extensionId, updateOperationId.value);
    updateOperation.value = op;
    ElMessage.success("已重试");
  } catch (e: any) {
    ElMessage.error("重试失败: " + (e?.message || e));
  } finally {
    updateActionLoading.value = false;
  }
}

async function doRollbackUpdate() {
  if (!updateTarget.value || !updateOperationId.value) return;
  try {
    await ElMessageBox.confirm("确定要回滚此更新操作吗？", "回滚确认", { type: "warning" });
  } catch {
    return;
  }
  updateActionLoading.value = true;
  try {
    const op = await rollbackUpdate(updateTarget.value.extensionId, updateOperationId.value);
    updateOperation.value = op;
    ElMessage.success("已回滚");
    await refreshList();
  } catch (e: any) {
    ElMessage.error("回滚失败: " + (e?.message || e));
  } finally {
    updateActionLoading.value = false;
  }
}

function goDiagnose() {
  switchTab("dev-console");
}

function openExtPage(pageId: string) {
  if (!detail.value) return;
  router.push({
    name: "extensionPage",
    params: { pageId },
    query: { extensionId: detail.value.extensionId },
  });
}

function formatDate(s: string): string {
  if (!s) return "";
  try {
    return new Date(s).toLocaleString("zh-CN");
  } catch {
    return s;
  }
}

function switchTab(name: string) {
  tab.value = name;
  if (name === "installed") void refreshList();
}

onMounted(async () => {
  await loadStatus();
  await refreshList();
});
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
  margin-top: 20px;
  border: 1px solid var(--console-border);
  border-radius: 14px;
  background: var(--ac-color-surface);
  box-shadow: none;
}
.tab-bar {
  display: flex;
  flex-wrap: nowrap;
  overflow-x: auto;
  align-items: baseline;
  gap: 0;
  margin-top: 20px;
  border-bottom: 1px solid var(--console-border);
  padding-bottom: 0;
}
.tab-item {
  margin: 0;
  padding: 8px 16px;
  font-size: 15px;
  font-weight: 600;
  color: var(--console-text-muted);
  cursor: pointer;
  white-space: nowrap;
  border-bottom: 2px solid transparent;
  transition: color 0.2s, border-color 0.2s;
  position: relative;
  bottom: -1px;
  flex-shrink: 0;
}
.tab-item:hover {
  color: var(--el-color-primary);
}
.tab-item.active {
  color: var(--el-color-primary);
  border-bottom-color: var(--el-color-primary);
}
.kernel-view-panel {
  padding: 0;
  overflow: hidden;
}
.kernel-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  height: 200px;
  color: var(--console-text-muted);
}
.loading-icon {
  animation: spin 1.5s linear infinite;
}
@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
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
.drop-zone span {
  color: var(--console-text-muted);
}
.install-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 16px;
  flex-wrap: wrap;
}
.preview-section,
.detail-section,
.update-meta-section,
.update-op-section {
  margin-top: 16px;
}
.preview-section h4,
.detail-section h4,
.update-meta-section h4,
.update-op-section h4 {
  margin: 0 0 8px 0;
  font-size: 14px;
}
.update-actions {
  display: flex;
  gap: 12px;
  margin-top: 12px;
  flex-wrap: wrap;
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
