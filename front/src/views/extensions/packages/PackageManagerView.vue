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
        <span>选择后将打开安装弹窗，在弹窗内完成安全检查与安装</span>
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
        <el-table-column label="名称" min-width="160" show-overflow-tooltip>
          <template #default="{ row }">{{ row.name || '-' }}</template>
        </el-table-column>
        <el-table-column prop="extensionId" label="扩展ID" min-width="200" show-overflow-tooltip />
        <el-table-column prop="version" label="版本" width="100" />
        <el-table-column label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="row.state === 'installed' ? 'success' : 'warning'" size="small">
              {{ row.state }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="installedAt" label="安装时间" width="180">
          <template #default="{ row }">
            {{ formatDate(row.installedAt) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="240" fixed="right">
          <template #default="{ row }">
            <div class="operation-cell">
              <el-switch
                :model-value="row.enablement === 'enabled'"
                :loading="toggleUpdating[row.extensionId]"
                :disabled="listLoading"
                :aria-label="row.enablement === 'enabled' ? '停用扩展' : '启用扩展'"
                @change="(value: string | number | boolean) => toggleEnable(row, Boolean(value))"
              />
              <el-button
                size="small"
                type="primary"
                @click="openDetail(row)"
              >详情</el-button>
              <el-dropdown
                trigger="click"
                @command="(command: string | number | object) => handleRowCommand(row, String(command))"
              >
                <el-button size="small" :icon="MoreFilled" aria-label="更多操作" />
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="permissions" :disabled="row.systemManaged">权限管理</el-dropdown-item>
                    <el-dropdown-item
                      command="pause"
                      :disabled="row.enablement !== 'enabled'"
                    >暂停</el-dropdown-item>
                    <el-dropdown-item
                      command="update"
                      :disabled="updateChecking[row.extensionId]"
                    >更新</el-dropdown-item>
                    <el-dropdown-item command="rollback">回滚</el-dropdown-item>
                    <el-dropdown-item command="diagnose">诊断</el-dropdown-item>
                    <el-dropdown-item
                      command="uninstall"
                      divided
                      :disabled="row.systemManaged"
                    >卸载</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
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

    <el-dialog
      v-model="installDialogVisible"
      :title="installMode === 'update' ? `更新 ${installPreview?.name || '扩展包'}` : '安装扩展包'"
      width="680px"
      destroy-on-close
      class="extension-install-dialog"
      @closed="resetInstallState"
    >
      <div
        class="package-drop-zone"
        :class="{ 'has-file': !!installFile }"
        @dragover.prevent
        @drop.prevent="onPackageDrop"
      >
        <el-icon class="upload-icon"><UploadFilled /></el-icon>
        <template v-if="installFile">
          <strong>{{ installFile.name }}</strong>
          <span>
            {{ formatSize(installFile.size) }} ·
            {{ previewLoading ? `正在检查 ${uploadProgress}%` : "已选择" }}
          </span>
        </template>
        <template v-else>
          <strong>选择或拖入 .amitiax 扩展包</strong>
          <span>安装前将执行格式、签名、权限与兼容性检查</span>
        </template>
        <el-button :loading="previewLoading" @click="choosePackage">
          {{ installFile ? "重新选择" : "选择文件" }}
        </el-button>
      </div>

      <el-progress
        v-if="previewLoading"
        class="upload-progress"
        :percentage="uploadProgress"
        :show-text="false"
        :stroke-width="4"
      />

      <div v-if="installPreview" class="package-preview">
        <div class="preview-head">
          <div>
            <span class="preview-kicker">安装预览</span>
            <h3>{{ installPreview.name }}</h3>
            <p>{{ installPreview.description || installPreview.id }}</p>
          </div>
          <span class="target-badge">扩展包</span>
        </div>

        <div class="preview-facts">
          <div><span>版本</span><strong>{{ installPreview.version }}</strong></div>
          <div><span>签名</span><strong>{{ signatureLabel(installPreview.signature?.status) }}</strong></div>
          <div><span>兼容性</span><strong>{{ installPreview.compatible ? "通过" : "不兼容" }}</strong></div>
          <div><span>目标</span><strong>{{ installPreview.managementTarget || "扩展中心" }}</strong></div>
        </div>

        <el-alert
          v-if="installPreview.errors?.length"
          :title="installPreview.errors.join('；')"
          type="error"
          show-icon
          :closable="false"
        />

        <el-alert
          v-else-if="previewMatchesInstalledVersion"
          :title="`版本 ${installPreview.version} 已安装，无需重复安装。`"
          type="info"
          show-icon
          :closable="false"
        />

        <div v-if="installPreview.highRiskCapabilities?.length" class="preview-block">
          <span class="preview-label">高风险项</span>
          <div class="chip-row">
            <span
              v-for="capability in installPreview.highRiskCapabilities"
              :key="capability"
              class="permission-chip risk"
            >
              {{ capability }}
            </span>
          </div>
        </div>

        <div v-else-if="installPreview.capabilities?.length" class="preview-block">
          <span class="preview-label">申请能力</span>
          <div class="chip-row">
            <span
              v-for="capability in installPreview.capabilities.slice(0, 8)"
              :key="capability"
              class="permission-chip"
            >
              {{ capability }}
            </span>
          </div>
        </div>

        <div v-if="installPreview.dependencies?.length" class="preview-block">
          <span class="preview-label">依赖</span>
          <div class="dependency-list">
            <span v-for="dependency in installPreview.dependencies" :key="dependency.id">
              {{ dependency.id }}
              <small>{{ dependency.installed ? "已满足" : dependency.required ? "缺失" : "可选" }}</small>
            </span>
          </div>
        </div>

        <el-alert
          v-for="warning in installPreview.warnings || []"
          :key="warning"
          :title="warning"
          type="warning"
          show-icon
          :closable="false"
          class="preview-warning"
        />

        <el-checkbox
          v-if="needsInstallAcknowledgement"
          v-model="installAcknowledged"
          class="install-confirmation"
        >
          我已查看此扩展的签名、权限和风险信息，并确认继续{{ installMode === "update" ? "更新" : "安装" }}。
        </el-checkbox>
      </div>

      <template #footer>
        <el-button @click="installDialogVisible = false">取消</el-button>
        <el-button
          type="primary"
          :loading="installLoading"
          :disabled="!canInstallPreview"
          @click="commitPackageInstall"
        >
          {{ installMode === "update" ? "确认更新" : "确认安装" }}
        </el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="detailVisible" title="扩展详情" width="820px">
      <div v-if="detail">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="名称">{{ detail.name || '-' }}</el-descriptions-item>
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

    <el-dialog v-model="permissionDialogVisible" title="权限管理" width="760px">
      <div v-if="permissionTarget" class="permission-manager">
        <div class="permission-manager-head">
          <div>
            <strong>{{ permissionTarget.name || permissionTarget.extensionId }}</strong>
            <span>{{ permissionTarget.extensionId }}</span>
          </div>
          <el-tag type="info" size="small">{{ permissionRows.length }} 项</el-tag>
        </div>
        <el-table
          v-loading="permissionLoading"
          :data="permissionRows"
          size="small"
          border
          empty-text="该插件没有声明权限"
        >
          <el-table-column prop="name" label="权限" min-width="210" show-overflow-tooltip />
          <el-table-column prop="scope" label="Scope" width="100" />
          <el-table-column prop="reason" label="用途" show-overflow-tooltip />
          <el-table-column label="必需" width="70">
            <template #default="{ row }">
              <el-tag :type="row.required ? 'warning' : 'info'" size="small">
                {{ row.required ? "是" : "否" }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="授权" width="104" align="center">
            <template #default="{ row }">
              <el-switch
                class="permission-pill-switch"
                :model-value="row.granted"
                :loading="permissionUpdating[row.name]"
                :disabled="permissionLoading"
                :aria-label="`${row.granted ? '关闭' : '开启'}权限 ${row.name}`"
                @change="(value: string | number | boolean) => togglePermission(row, Boolean(value))"
              />
            </template>
          </el-table-column>
        </el-table>
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
            <el-descriptions-item label="包格式版本">{{ updateMeta.manifestVersion }}</el-descriptions-item>
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
import { computed, defineAsyncComponent, onMounted, ref, shallowRef, watch } from "vue";
import { useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import { UploadFilled, Loading, MoreFilled } from "@element-plus/icons-vue";
import { apiClient } from "@/composables/useApi";
import ExtensionPageHeader from "../components/ExtensionPageHeader.vue";
import {
  getKernelStatus,
  listExtensions,
  getExtension,
  enableExtension,
  disableExtension,
  uninstallExtension,
  pauseExtension,
  rollbackExtension,
  setExtensionPermission,
  type KernelStatus,
  type KernelExtension,
  type KernelExtensionDetail,
  type KernelPermission,
} from "@/views/kernel/api";
import {
  installExtensionPackage,
  previewExtensionPackage,
} from "@/views/extensions/api";
import type { PackageImportPreview } from "@/views/extensions/types";
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
import { useExtensionUIStore } from "@/stores/extensionUI";

const router = useRouter();
const extensionUIStore = useExtensionUIStore();

const tab = ref("install");
const loadError = ref("");

const statusReady = ref(false);
const statusCount = ref<number | undefined>(undefined);
const extensions = ref<KernelExtension[]>([]);
const listLoading = ref(false);

const installDialogVisible = ref(false);
const installMode = ref<"install" | "update">("install");
const installFile = ref<File | null>(null);
const installPreview = ref<PackageImportPreview | null>(null);
const previewLoading = ref(false);
const installLoading = ref(false);
const uploadProgress = ref(0);
const installAcknowledged = ref(false);

const needsInstallAcknowledgement = computed(() => {
  const preview = installPreview.value;
  if (!preview) return false;
  return preview.signature?.status === "unsigned"
    || preview.scripts > 0
    || (preview.highRiskCapabilities?.length || 0) > 0
    || (preview.capabilityConfirmations?.length || 0) > 0
    || (preview.warnings?.length || 0) > 0;
});

const previewMatchesInstalledVersion = computed(() => {
  const preview = installPreview.value;
  if (!preview?.currentVersion) return false;
  return preview.conflict === "same-version-same-content"
    || preview.currentVersion === preview.version;
});

const canInstallPreview = computed(() => {
  const preview = installPreview.value;
  if (!preview || previewLoading.value || installLoading.value) return false;
  if (!preview.compatible || (preview.errors?.length || 0) > 0) return false;
  if (previewMatchesInstalledVersion.value) return false;
  return !needsInstallAcknowledgement.value || installAcknowledged.value;
});

const detailVisible = ref(false);
const detail = ref<KernelExtensionDetail | null>(null);
const permissionDialogVisible = ref(false);
const permissionTarget = ref<KernelExtension | null>(null);
const permissionRows = ref<KernelPermission[]>([]);
const permissionLoading = ref(false);
const permissionUpdating = ref<Record<string, boolean>>({});
const toggleUpdating = ref<Record<string, boolean>>({});

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
  await setPackageFile(new File([bytes], selected.name, { type: "application/zip" }));
}

async function onPackageFile(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = "";
  if (file) await setPackageFile(file);
}

async function onPackageDrop(event: DragEvent) {
  const file = event.dataTransfer?.files?.[0];
  if (file) await setPackageFile(file);
}

async function setPackageFile(file: File) {
  if (!file.name.toLowerCase().endsWith(".amitiax")) {
    ElMessage.warning("请选择 .amitiax 扩展包");
    return;
  }
  installFile.value = file;
  installPreview.value = null;
  installAcknowledged.value = false;
  installMode.value = "install";
  installDialogVisible.value = true;
  await buildPackagePreview();
}

async function buildPackagePreview() {
  if (!installFile.value) return;
  previewLoading.value = true;
  uploadProgress.value = 0;
  try {
    const preview = await previewExtensionPackage(
      installFile.value,
      "global",
      "",
      "",
      (percent) => {
        uploadProgress.value = percent;
      },
    );
    installPreview.value = preview;
    installMode.value = preview.currentVersion ? "update" : "install";
  } catch (e: any) {
    installPreview.value = null;
    ElMessage.error("预览失败: " + (e?.message || e));
  } finally {
    previewLoading.value = false;
    uploadProgress.value = installPreview.value ? 100 : uploadProgress.value;
  }
}

async function commitPackageInstall() {
  const preview = installPreview.value;
  if (!preview || !canInstallPreview.value) return;
  installLoading.value = true;
  try {
    const acknowledged = installAcknowledged.value || !needsInstallAcknowledgement.value;
    const result = await installExtensionPackage(
      preview,
      {
        unsigned: acknowledged,
        scripts: acknowledged,
        capabilities: acknowledged ? [...(preview.highRiskCapabilities || [])] : [],
        versionChange: acknowledged,
        signerChange: acknowledged,
        configMigration: acknowledged,
      },
      installMode.value === "update" ? preview.id : "",
    );
    await waitForPackageOperation(result.operationId);
    ElMessage.success(installMode.value === "update" ? "扩展包已更新" : "扩展包已安装");
    extensionUIStore.dispatchExtensionChanged(installMode.value === "update" ? "update" : "install");
    installDialogVisible.value = false;
    await refreshList();
    tab.value = "installed";
  } catch (e: any) {
    ElMessage.error("安装失败: " + (e?.message || e));
  } finally {
    installLoading.value = false;
  }
}

async function waitForPackageOperation(operationId?: string) {
  if (!operationId) return;
  for (let attempt = 0; attempt < 120; attempt += 1) {
    const response = await apiClient.get<{ status?: string; errorCode?: string }>(
      `/api/extensions/packages/operations/${encodeURIComponent(operationId)}`,
    );
    const status = String(response.data?.status || "").toLowerCase();
    if (status === "completed") return;
    if (status === "failed" || status === "requires_recovery") {
      throw new Error(response.data?.errorCode || "扩展包操作失败");
    }
    await new Promise((resolve) => window.setTimeout(resolve, 500));
  }
  throw new Error("扩展包操作等待超时，请刷新页面检查最终状态");
}

function resetInstallState() {
  installMode.value = "install";
  installFile.value = null;
  installPreview.value = null;
  previewLoading.value = false;
  installLoading.value = false;
  uploadProgress.value = 0;
  installAcknowledged.value = false;
}

function signatureLabel(status?: string) {
  const labels: Record<string, string> = {
    "valid-trusted": "可信签名",
    "valid-untrusted": "有效但不受信任",
    unsigned: "未签名",
    invalid: "签名无效",
  };
  return status ? labels[status] || status : "未提供";
}

async function toggleEnable(row: KernelExtension, enable: boolean) {
  if (toggleUpdating.value[row.extensionId]) return;
  toggleUpdating.value[row.extensionId] = true;
  try {
    if (enable) {
      await enableExtension(row.extensionId);
      ElMessage.success("已启用: " + row.extensionId);
      extensionUIStore.dispatchExtensionChanged("enable");
    } else {
      await disableExtension(row.extensionId);
      ElMessage.success("已禁用: " + row.extensionId);
      extensionUIStore.dispatchExtensionChanged("disable");
    }
    await refreshList();
  } catch (e: any) {
    ElMessage.error("操作失败: " + (e?.message || e));
  } finally {
    toggleUpdating.value[row.extensionId] = false;
  }
}

async function doUninstall(row: KernelExtension) {
  if (row.systemManaged) return;
  try {
    await ElMessageBox.confirm(
      `确定要卸载扩展 ${row.extensionId} v${row.version} 吗？`,
      "卸载确认",
      { type: "warning" }
    );
    await uninstallExtension(row.extensionId);
    ElMessage.success("已卸载: " + row.extensionId);
    extensionUIStore.dispatchExtensionChanged("uninstall");
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
    extensionUIStore.dispatchExtensionChanged("disable");
    await refreshList();
  } catch (e: any) {
    if (e !== "cancel" && e?.message !== "cancel") {
      ElMessage.error("暂停失败: " + (e?.message || e));
    }
  }
}

async function handleRowCommand(row: KernelExtension, command: string) {
  switch (command) {
    case "pause":
      await doPause(row);
      break;
    case "update":
      await checkRowUpdate(row);
      break;
    case "permissions":
      await openPermissionManager(row);
      break;
    case "rollback":
      await doRollback(row);
      break;
    case "diagnose":
      goDiagnose();
      break;
    case "uninstall":
      if (row.systemManaged) return;
      await doUninstall(row);
      break;
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
    extensionUIStore.dispatchExtensionChanged("update");
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

async function openPermissionManager(row: KernelExtension) {
  permissionTarget.value = row;
  permissionRows.value = [];
  permissionDialogVisible.value = true;
  permissionLoading.value = true;
  try {
    const data = await getExtension(row.extensionId);
    permissionRows.value = data.permissions ?? [];
  } catch (e: any) {
    ElMessage.error("加载权限失败: " + (e?.message || e));
  } finally {
    permissionLoading.value = false;
  }
}

async function togglePermission(row: KernelPermission, granted: boolean) {
  if (!permissionTarget.value || permissionUpdating.value[row.name]) return;
  permissionUpdating.value[row.name] = true;
  try {
    const updated = await setExtensionPermission(
      permissionTarget.value.extensionId,
      row.name,
      granted,
    );
    row.granted = updated.granted;
    ElMessage.success(granted ? "权限已开启" : "权限已关闭");
  } catch (e: any) {
    ElMessage.error("权限更新失败: " + (e?.message || e));
  } finally {
    permissionUpdating.value[row.name] = false;
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
    extensionUIStore.dispatchExtensionChanged("update");
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
    extensionUIStore.dispatchExtensionChanged("update");
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
.operation-cell {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  white-space: nowrap;
}
.permission-manager {
  display: grid;
  gap: 14px;
}
.permission-manager-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 14px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 10px;
  background: var(--el-fill-color-lighter);
}
.permission-manager-head > div {
  display: grid;
  min-width: 0;
  gap: 4px;
}
.permission-manager-head strong {
  overflow: hidden;
  color: var(--console-text);
  text-overflow: ellipsis;
  white-space: nowrap;
}
.permission-manager-head span {
  overflow: hidden;
  color: var(--console-text-muted);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.permission-pill-switch {
  --el-switch-on-color: var(--el-color-success);
}
.permission-pill-switch :deep(.el-switch__core) {
  min-width: 46px;
  border-radius: 999px;
}
.package-drop-zone {
  min-height: 188px;
  padding: 24px;
  border: 1px dashed var(--el-border-color);
  border-radius: 16px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  text-align: center;
  background: var(--el-fill-color-lighter);
  transition: border-color 160ms ease, background 160ms ease;
}
.package-drop-zone:hover,
.package-drop-zone.has-file {
  border-color: var(--el-color-primary-light-7);
  background: var(--el-color-primary-light-9);
}
.upload-icon {
  margin-bottom: 4px;
  font-size: 32px;
  color: var(--el-color-primary);
}
.package-drop-zone strong {
  font-size: 14px;
}
.package-drop-zone span {
  margin-bottom: 6px;
  color: var(--console-text-muted);
  font-size: 12px;
}
.upload-progress {
  margin-top: 10px;
}
.package-preview {
  margin-top: 16px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.preview-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}
.preview-head h3 {
  margin-top: 4px;
  font-size: 18px;
}
.preview-head p {
  max-width: 470px;
  margin-top: 4px;
  font-size: 12px;
}
.preview-kicker {
  display: block;
  color: var(--el-color-primary);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}
.target-badge {
  flex-shrink: 0;
  padding: 4px 8px;
  border: 1px solid var(--el-color-primary-light-7);
  border-radius: 999px;
  color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
  font-size: 12px;
}
.preview-facts {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}
.preview-facts > div {
  min-width: 0;
  padding: 10px 12px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 10px;
  background: var(--el-fill-color-lighter);
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.preview-facts span {
  color: var(--console-text-muted);
  font-size: 11px;
}
.preview-facts strong {
  word-break: break-word;
}
.preview-block {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.preview-label {
  display: block;
  color: var(--el-color-primary);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}
.chip-row {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.permission-chip {
  max-width: 100%;
  padding: 5px 8px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  color: var(--console-text);
  background: var(--el-fill-color-lighter);
  font-size: 11px;
  word-break: break-all;
}
.permission-chip.risk {
  color: var(--el-color-danger);
  background: var(--el-color-danger-light-9);
  border-color: var(--el-color-danger-light-7);
}
.dependency-list {
  display: grid;
  gap: 6px;
}
.dependency-list > span {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 7px 9px;
  border-radius: 8px;
  background: var(--el-fill-color-lighter);
  font-size: 11px;
}
.dependency-list small {
  color: var(--console-text-muted);
}
.preview-warning {
  margin-top: -2px;
}
.install-confirmation {
  height: auto;
  align-items: flex-start;
  white-space: normal;
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
  .preview-facts {
    grid-template-columns: 1fr;
  }
}
</style>
