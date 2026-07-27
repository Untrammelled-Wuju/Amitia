<template>
  <div class="kernel-center">
    <div class="kernel-header">
      <div class="header-left">
        <h2>扩展内核中心</h2>
        <p class="subtitle">Extension Kernel — 安装、启用、卸载扩展包</p>
      </div>
      <div class="header-right">
        <el-tag :type="statusReady ? 'success' : 'danger'" size="large">
          {{ statusReady ? '内核在线' : '内核离线' }}
        </el-tag>
        <el-tag v-if="statusCount !== undefined" type="info" size="large">
          已安装 {{ statusCount }}
        </el-tag>
      </div>
    </div>

    <div class="kernel-toolbar">
      <el-upload
        ref="uploadRef"
        :auto-upload="false"
        :show-file-list="false"
        accept=".amitiax"
        :on-change="onFileChange"
      >
        <el-button type="primary" :icon="Upload">选择扩展包</el-button>
      </el-upload>
      <el-button
        v-if="selectedFile"
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
      <el-button @click="refreshList" :icon="Refresh">刷新列表</el-button>
    </div>

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

        <div v-if="previewResult.issues && previewResult.issues.length > 0" class="preview-issues">
          <h4>问题列表</h4>
          <el-table :data="previewResult.issues" size="small" border>
            <el-table-column prop="category" label="类别" width="140" />
            <el-table-column prop="code" label="代码" width="160" />
            <el-table-column prop="message" label="描述" />
          </el-table>
        </div>

        <div v-if="previewResult.modules && previewResult.modules.length > 0" class="preview-modules">
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

    <el-table
      :data="extensions"
      v-loading="listLoading"
      border
      style="width: 100%"
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
      <el-table-column label="操作" width="280" fixed="right">
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
            size="small"
            type="danger"
            @click="doUninstall(row)"
          >卸载</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="detailVisible" title="扩展详情" width="720px">
      <div v-if="detail">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="扩展ID">{{ detail.extensionId }}</el-descriptions-item>
          <el-descriptions-item label="版本">{{ detail.version }}</el-descriptions-item>
          <el-descriptions-item label="安装ID">{{ detail.installationId }}</el-descriptions-item>
          <el-descriptions-item label="状态">{{ detail.state }}</el-descriptions-item>
          <el-descriptions-item label="启用">{{ detail.enablement }}</el-descriptions-item>
          <el-descriptions-item label="Generation">{{ detail.generation }}</el-descriptions-item>
        </el-descriptions>

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
          </el-table>
        </div>
      </div>
    </el-dialog>

    <div class="kernel-nav-cards">
      <router-link to="/kernel/trusted-services" class="nav-card">
        <div class="nav-card-title">可信服务运行时</div>
        <div class="nav-card-desc">Trusted Service Runtime — 原生服务进程管理与隔离</div>
      </router-link>
      <router-link to="/kernel/wasm" class="nav-card">
        <div class="nav-card-title">WASM 运行时</div>
        <div class="nav-card-desc">WebAssembly 模块管理与实例化</div>
      </router-link>
      <router-link to="/kernel/hooks" class="nav-card">
        <div class="nav-card-title">Hook 中心</div>
        <div class="nav-card-desc">扩展 Hook 贡献点与触发记录</div>
      </router-link>
      <router-link to="/kernel/tasks" class="nav-card">
        <div class="nav-card-title">任务运行时</div>
        <div class="nav-card-desc">Task Runtime — 后台任务队列与子进程执行</div>
      </router-link>
      <router-link to="/kernel/events" class="nav-card">
        <div class="nav-card-title">事件中心</div>
        <div class="nav-card-desc">Event System — 第三方事件发布、订阅、投递与死信管理</div>
      </router-link>
      <router-link to="/kernel/schedules" class="nav-card">
        <div class="nav-card-title">调度中心</div>
        <div class="nav-card-desc">Schedule System — 第三方定时触发、误火处理、熔断保护</div>
      </router-link>
      <router-link to="/kernel/desktop" class="nav-card">
        <div class="nav-card-title">桌面贡献中心</div>
        <div class="nav-card-desc">Desktop Contribution — 菜单、托盘、快捷键贡献管理与冲突解决</div>
      </router-link>
      <router-link to="/kernel/updates" class="nav-card">
        <div class="nav-card-title">扩展更新中心</div>
        <div class="nav-card-desc">Extension Update Center — 检查、下载、安装、回滚扩展更新</div>
      </router-link>
      <router-link to="/kernel/dev-console" class="nav-card">
        <div class="nav-card-title">开发者诊断控制台</div>
        <div class="nav-card-desc">Developer Console — 运行时诊断、调用追踪、事件监控与导出</div>
      </router-link>
      <router-link to="/kernel/migrations" class="nav-card">
        <div class="nav-card-title">迁移与灰度中心</div>
        <div class="nav-card-desc">Migration &amp; Canary Center — 数据迁移、灰度发布、回滚管理与崩溃恢复</div>
      </router-link>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { Upload, Refresh } from "@element-plus/icons-vue";
import type { UploadFile } from "element-plus";
import {
  getKernelStatus,
  listExtensions,
  getExtension,
  previewInstall,
  installExtension,
  enableExtension,
  disableExtension,
  uninstallExtension,
  type KernelStatus,
  type KernelExtension,
  type KernelExtensionDetail,
  type InstallPreview,
} from "./api";

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
  try {
    const data = await listExtensions();
    extensions.value = data.extensions || [];
    statusCount.value = data.total;
  } catch (e: any) {
    ElMessage.error("加载列表失败: " + (e?.message || e));
  } finally {
    listLoading.value = false;
  }
}

function onFileChange(file: UploadFile) {
  if (file.raw) {
    selectedFile.value = file.raw;
    previewResult.value = null;
    ElMessage.info("已选择文件: " + file.name);
  }
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

function formatDate(s: string): string {
  if (!s) return "";
  try {
    return new Date(s).toLocaleString("zh-CN");
  } catch {
    return s;
  }
}

onMounted(async () => {
  await loadStatus();
  await refreshList();
});
</script>

<style scoped>
.kernel-center {
  padding: 24px;
  max-width: 1200px;
  margin: 0 auto;
}

.kernel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.kernel-header h2 {
  margin: 0 0 4px 0;
  font-size: 22px;
}

.subtitle {
  margin: 0;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.header-right {
  display: flex;
  gap: 8px;
}

.kernel-toolbar {
  display: flex;
  gap: 12px;
  margin-bottom: 20px;
  align-items: center;
}

.preview-issues,
.preview-modules,
.detail-section {
  margin-top: 16px;
}

.preview-issues h4,
.preview-modules h4,
.detail-section h4 {
  margin: 0 0 8px 0;
  font-size: 14px;
}

.kernel-nav-cards {
  display: flex;
  gap: 16px;
  margin-top: 24px;
}

.nav-card {
  flex: 1;
  display: block;
  padding: 16px 20px;
  border: 1px solid var(--el-border-color);
  border-radius: 8px;
  text-decoration: none;
  color: var(--el-text-color-primary);
  transition: border-color 0.2s, box-shadow 0.2s;
}

.nav-card:hover {
  border-color: var(--el-color-primary);
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.08);
}

.nav-card-title {
  font-size: 15px;
  font-weight: 600;
  margin-bottom: 4px;
}

.nav-card-desc {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
</style>
