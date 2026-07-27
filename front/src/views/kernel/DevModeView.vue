<template>
  <div class="dev-mode-center">
    <div class="dev-header">
      <div class="header-left">
        <h2>开发模式中心</h2>
        <p class="subtitle">Development Mode — 工作区注册、热重载、构建管理与开发信任</p>
      </div>
      <div class="header-right">
        <el-button type="primary" @click="openRegisterDialog">注册工作区</el-button>
        <el-button @click="refreshList" :icon="Refresh">刷新列表</el-button>
      </div>
    </div>

    <el-table :data="workspaces" v-loading="listLoading" border style="width: 100%">
      <el-table-column prop="extensionId" label="扩展ID" min-width="180" show-overflow-tooltip />
      <el-table-column prop="path" label="路径" min-width="200" show-overflow-tooltip />
      <el-table-column label="状态" width="120">
        <template #default="{ row }">
          <el-tag :type="statusTagType(row.status)" size="small">{{ row.status }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="开发信任" width="100">
        <template #default="{ row }">
          <el-tag :type="row.devTrust ? 'success' : 'info'" size="small">
            {{ row.devTrust ? '已信任' : '未信任' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="currentRevision" label="当前版本" min-width="120" show-overflow-tooltip>
        <template #default="{ row }">
          {{ row.currentRevision || '-' }}
        </template>
      </el-table-column>
      <el-table-column label="最后重载时间" width="180">
        <template #default="{ row }">
          {{ formatDate(row.lastReloadAt) }}
        </template>
      </el-table-column>
      <el-table-column label="操作" width="420" fixed="right">
        <template #default="{ row }">
          <el-button
            v-if="!row.devTrust"
            size="small"
            type="success"
            @click="doGrantTrust(row)"
          >信任</el-button>
          <el-button
            v-else
            size="small"
            type="warning"
            @click="doRevokeTrust(row)"
          >撤销信任</el-button>
          <el-button size="small" type="primary" @click="doBuild(row)">构建</el-button>
          <el-button size="small" type="primary" @click="doReload(row)">重载</el-button>
          <el-button size="small" @click="doStartWatch(row)" v-if="!row.watchEnabled">监听</el-button>
          <el-button size="small" type="warning" @click="doStopWatch(row)" v-else>停止监听</el-button>
          <el-button size="small" @click="openRevisions(row)">历史</el-button>
          <el-button size="small" @click="openSessionDialog(row)">会话</el-button>
          <el-button size="small" type="danger" @click="doRemove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="registerVisible" title="注册开发工作区" width="560px">
      <el-form :model="registerForm" label-width="120px">
        <el-form-item label="扩展ID" required>
          <el-input v-model="registerForm.extensionId" placeholder="例如 dev.amitia.example" />
        </el-form-item>
        <el-form-item label="工作区路径" required>
          <el-input v-model="registerForm.path" placeholder="例如 /path/to/workspace" />
        </el-form-item>
        <el-form-item label="Manifest路径" required>
          <el-input v-model="registerForm.manifestPath" placeholder="例如 /path/to/manifest.json" />
        </el-form-item>
        <el-form-item label="启用文件监听">
          <el-switch v-model="registerForm.watchEnabled" />
        </el-form-item>
        <el-form-item label="自动热重载">
          <el-switch v-model="registerForm.autoReload" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="registerVisible = false">取消</el-button>
        <el-button type="primary" :loading="registerLoading" @click="doRegister">确认注册</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="revisionsVisible" title="构建历史" width="800px">
      <el-table :data="revisions" v-loading="revisionsLoading" border size="small">
        <el-table-column prop="revisionId" label="版本ID" min-width="120" show-overflow-tooltip />
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="revisionStatusType(row.status)" size="small">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="构建时间" width="180">
          <template #default="{ row }">
            {{ formatDate(row.builtAt) }}
          </template>
        </el-table-column>
        <el-table-column label="耗时" width="100">
          <template #default="{ row }">
            {{ row.buildDurationMs }}ms
          </template>
        </el-table-column>
        <el-table-column label="错误数" width="80">
          <template #default="{ row }">
            <el-tag :type="row.errorCount > 0 ? 'danger' : 'success'" size="small">
              {{ row.errorCount }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="详情" width="80">
          <template #default="{ row }">
            <el-button size="small" text @click="showRevisionDetail(row)">查看</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>

    <el-dialog v-model="revisionDetailVisible" title="构建版本详情" width="640px">
      <div v-if="revisionDetail">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="版本ID">{{ revisionDetail.revisionId }}</el-descriptions-item>
          <el-descriptions-item label="状态">{{ revisionDetail.status }}</el-descriptions-item>
          <el-descriptions-item label="构建时间">{{ formatDate(revisionDetail.builtAt) }}</el-descriptions-item>
          <el-descriptions-item label="耗时">{{ revisionDetail.buildDurationMs }}ms</el-descriptions-item>
          <el-descriptions-item label="Manifest Hash" :span="2">{{ revisionDetail.manifestHash }}</el-descriptions-item>
          <el-descriptions-item label="Source Hash" :span="2">{{ revisionDetail.sourceHash }}</el-descriptions-item>
          <el-descriptions-item label="产物路径" :span="2">{{ revisionDetail.artifactPath }}</el-descriptions-item>
        </el-descriptions>

        <div v-if="revisionDetail.errors && revisionDetail.errors.length > 0" class="detail-section">
          <h4>构建错误</h4>
          <el-table :data="revisionDetail.errors" size="small" border>
            <el-table-column prop="file" label="文件" min-width="200" show-overflow-tooltip />
            <el-table-column prop="line" label="行" width="60" />
            <el-table-column prop="column" label="列" width="60" />
            <el-table-column prop="code" label="代码" width="100" />
            <el-table-column prop="message" label="消息" min-width="200" show-overflow-tooltip />
          </el-table>
        </div>

        <div v-if="revisionDetail.warnings && revisionDetail.warnings.length > 0" class="detail-section">
          <h4>警告</h4>
          <ul class="warning-list">
            <li v-for="(w, i) in revisionDetail.warnings" :key="i">{{ w }}</li>
          </ul>
        </div>
      </div>
    </el-dialog>

    <el-dialog v-model="sessionVisible" title="开发会话管理" width="600px">
      <div class="session-section">
        <div class="session-actions">
          <el-form :model="sessionForm" label-width="100px" inline>
            <el-form-item label="设备ID">
              <el-input v-model="sessionForm.deviceId" placeholder="例如 dev-local" style="width: 180px" />
            </el-form-item>
            <el-form-item label="UserAgent">
              <el-input v-model="sessionForm.userAgent" placeholder="例如 browser" style="width: 180px" />
            </el-form-item>
          </el-form>
          <div class="session-buttons">
            <el-button type="primary" size="small" :loading="sessionLoading" @click="doOpenSession">打开会话</el-button>
            <el-button type="warning" size="small" @click="doCloseSession">关闭会话</el-button>
          </div>
        </div>

        <div v-if="currentSession" class="session-info">
          <el-descriptions :column="1" border size="small">
            <el-descriptions-item label="会话ID">{{ currentSession.sessionId }}</el-descriptions-item>
            <el-descriptions-item label="工作区ID">{{ currentSession.workspaceId }}</el-descriptions-item>
            <el-descriptions-item label="设备ID">{{ currentSession.deviceId }}</el-descriptions-item>
            <el-descriptions-item label="UserAgent">{{ currentSession.userAgent }}</el-descriptions-item>
            <el-descriptions-item label="开始时间">{{ formatDate(currentSession.startedAt) }}</el-descriptions-item>
            <el-descriptions-item label="过期时间">{{ formatDate(currentSession.expiresAt) }}</el-descriptions-item>
            <el-descriptions-item label="已撤销">{{ currentSession.revoked ? '是' : '否' }}</el-descriptions-item>
            <el-descriptions-item label="Scopes">{{ (currentSession.scopes || []).join(', ') || '-' }}</el-descriptions-item>
          </el-descriptions>
        </div>
        <el-empty v-else description="暂无活跃开发会话" />
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { Refresh } from "@element-plus/icons-vue";
import {
  listWorkspaces,
  registerWorkspace,
  removeWorkspace,
  grantTrust,
  revokeTrust,
  buildWorkspace,
  reloadWorkspace,
  startWatch,
  stopWatch,
  openSession,
  closeSession,
  listRevisions,
  type DevWorkspace,
  type DevRevision,
  type DevSession,
  type RegisterWorkspaceBody,
} from "./dev-mode-api";

const workspaces = ref<DevWorkspace[]>([]);
const listLoading = ref(false);

const registerVisible = ref(false);
const registerLoading = ref(false);
const registerForm = ref<RegisterWorkspaceBody>({
  extensionId: "",
  path: "",
  manifestPath: "",
  watchEnabled: false,
  autoReload: false,
});

const revisionsVisible = ref(false);
const revisionsLoading = ref(false);
const revisions = ref<DevRevision[]>([]);
const revisionDetailVisible = ref(false);
const revisionDetail = ref<DevRevision | null>(null);

const sessionVisible = ref(false);
const sessionLoading = ref(false);
const currentSession = ref<DevSession | null>(null);
const sessionForm = ref({ deviceId: "", userAgent: "" });
const activeWorkspaceId = ref("");

async function refreshList() {
  listLoading.value = true;
  try {
    const data = await listWorkspaces();
    workspaces.value = data.workspaces || [];
  } catch (e: any) {
    ElMessage.error("加载工作区列表失败: " + (e?.message || e));
  } finally {
    listLoading.value = false;
  }
}

function openRegisterDialog() {
  registerForm.value = {
    extensionId: "",
    path: "",
    manifestPath: "",
    watchEnabled: false,
    autoReload: false,
  };
  registerVisible.value = true;
}

async function doRegister() {
  if (!registerForm.value.extensionId || !registerForm.value.path || !registerForm.value.manifestPath) {
    ElMessage.warning("请填写扩展ID、路径和Manifest路径");
    return;
  }
  registerLoading.value = true;
  try {
    await registerWorkspace(registerForm.value);
    ElMessage.success("工作区注册成功");
    registerVisible.value = false;
    await refreshList();
  } catch (e: any) {
    ElMessage.error("注册失败: " + (e?.message || e));
  } finally {
    registerLoading.value = false;
  }
}

async function doGrantTrust(row: DevWorkspace) {
  try {
    await grantTrust(row.workspaceId);
    ElMessage.success("已授予开发信任: " + row.extensionId);
    await refreshList();
  } catch (e: any) {
    ElMessage.error("操作失败: " + (e?.message || e));
  }
}

async function doRevokeTrust(row: DevWorkspace) {
  try {
    await revokeTrust(row.workspaceId);
    ElMessage.success("已撤销开发信任: " + row.extensionId);
    await refreshList();
  } catch (e: any) {
    ElMessage.error("操作失败: " + (e?.message || e));
  }
}

async function doBuild(row: DevWorkspace) {
  try {
    await ElMessageBox.confirm(`确定要构建工作区 ${row.extensionId} 吗？`, "构建确认", { type: "info" });
    ElMessage.info("构建已触发，请稍候...");
    const result = await buildWorkspace(row.workspaceId, { sourceMap: true });
    if (result.revision.errorCount > 0) {
      ElMessage.warning(`构建完成，存在 ${result.revision.errorCount} 个错误`);
    } else {
      ElMessage.success("构建成功: " + result.revision.revisionId);
    }
    await refreshList();
  } catch (e: any) {
    if (e !== "cancel" && e?.message !== "cancel") {
      ElMessage.error("构建失败: " + (e?.message || e));
    }
  }
}

async function doReload(row: DevWorkspace) {
  try {
    await ElMessageBox.confirm(`确定要热重载工作区 ${row.extensionId} 吗？`, "重载确认", { type: "warning" });
    ElMessage.info("热重载已触发...");
    const result = await reloadWorkspace(row.workspaceId, "manual");
    if (result.event.success) {
      ElMessage.success("热重载成功: " + result.event.revisionId);
    } else {
      ElMessage.error("热重载失败: " + result.event.error);
    }
    await refreshList();
  } catch (e: any) {
    if (e !== "cancel" && e?.message !== "cancel") {
      ElMessage.error("重载失败: " + (e?.message || e));
    }
  }
}

async function doStartWatch(row: DevWorkspace) {
  try {
    await startWatch(row.workspaceId);
    ElMessage.success("文件监听已启动: " + row.extensionId);
    await refreshList();
  } catch (e: any) {
    ElMessage.error("启动监听失败: " + (e?.message || e));
  }
}

async function doStopWatch(row: DevWorkspace) {
  try {
    await stopWatch(row.workspaceId);
    ElMessage.success("文件监听已停止: " + row.extensionId);
    await refreshList();
  } catch (e: any) {
    ElMessage.error("停止监听失败: " + (e?.message || e));
  }
}

async function doRemove(row: DevWorkspace) {
  try {
    await ElMessageBox.confirm(`确定要删除工作区 ${row.extensionId} 吗？此操作不可恢复。`, "删除确认", { type: "warning" });
    await removeWorkspace(row.workspaceId);
    ElMessage.success("已删除工作区: " + row.extensionId);
    await refreshList();
  } catch (e: any) {
    if (e !== "cancel" && e?.message !== "cancel") {
      ElMessage.error("删除失败: " + (e?.message || e));
    }
  }
}

async function openRevisions(row: DevWorkspace) {
  activeWorkspaceId.value = row.workspaceId;
  revisionsVisible.value = true;
  revisionsLoading.value = true;
  try {
    const data = await listRevisions(row.workspaceId);
    revisions.value = data.revisions || [];
  } catch (e: any) {
    ElMessage.error("加载构建历史失败: " + (e?.message || e));
    revisions.value = [];
  } finally {
    revisionsLoading.value = false;
  }
}

function showRevisionDetail(row: DevRevision) {
  revisionDetail.value = row;
  revisionDetailVisible.value = true;
}

function openSessionDialog(row: DevWorkspace) {
  activeWorkspaceId.value = row.workspaceId;
  currentSession.value = null;
  sessionForm.value = { deviceId: "", userAgent: "" };
  sessionVisible.value = true;
}

async function doOpenSession() {
  if (!sessionForm.value.deviceId) {
    ElMessage.warning("请填写设备ID");
    return;
  }
  sessionLoading.value = true;
  try {
    const sess = await openSession(activeWorkspaceId.value, {
      deviceId: sessionForm.value.deviceId,
      userAgent: sessionForm.value.userAgent || "browser",
    });
    currentSession.value = sess;
    ElMessage.success("开发会话已打开: " + sess.sessionId);
  } catch (e: any) {
    ElMessage.error("打开会话失败: " + (e?.message || e));
  } finally {
    sessionLoading.value = false;
  }
}

async function doCloseSession() {
  try {
    await closeSession(activeWorkspaceId.value);
    ElMessage.success("开发会话已关闭");
    currentSession.value = null;
  } catch (e: any) {
    ElMessage.error("关闭会话失败: " + (e?.message || e));
  }
}

function statusTagType(status: string): "primary" | "success" | "warning" | "danger" | "info" {
  const map: Record<string, "primary" | "success" | "warning" | "danger" | "info"> = {
    registered: "info",
    building: "warning",
    ready: "success",
    reloading: "warning",
    failed: "danger",
    disabled: "info",
  };
  return map[status] || "info";
}

function revisionStatusType(status: string): "primary" | "success" | "warning" | "danger" | "info" {
  const map: Record<string, "primary" | "success" | "warning" | "danger" | "info"> = {
    building: "warning",
    succeeded: "success",
    failed: "danger",
    stale: "info",
  };
  return map[status] || "info";
}

function formatDate(s: string): string {
  if (!s) return "-";
  try {
    const d = new Date(s);
    if (d.getTime() === 0 || isNaN(d.getTime())) return "-";
    return d.toLocaleString("zh-CN");
  } catch {
    return s;
  }
}

onMounted(async () => {
  await refreshList();
});
</script>

<style scoped>
.dev-mode-center {
  padding: 20px;
}

.dev-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.dev-header h2 {
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

.detail-section {
  margin-top: 16px;
}

.detail-section h4 {
  margin: 0 0 8px 0;
  font-size: 14px;
}

.warning-list {
  margin: 0;
  padding-left: 20px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.session-section {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.session-actions {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.session-buttons {
  display: flex;
  gap: 8px;
}

.session-info {
  margin-top: 8px;
}
</style>
