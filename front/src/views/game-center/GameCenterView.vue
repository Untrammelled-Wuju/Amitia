<template>
  <div class="game-center-page">
    <div class="page-head">
      <div>
        <h2>游戏中心</h2>
        <p>管理游戏插件、运行时、服务调用与控制权限。数据直接来自 Game Center Runtime。</p>
      </div>
      <div class="head-actions">
        <el-button @click="installPlugin">安装插件</el-button>
        <el-button :loading="loading" @click="refresh">刷新</el-button>
      </div>
    </div>

    <el-alert v-if="error" :title="error" type="error" show-icon :closable="false" />

    <div class="summary-grid">
      <el-card shadow="never"><div class="metric"><strong>{{ health?.pluginCount ?? plugins.length }}</strong><span>插件</span></div></el-card>
      <el-card shadow="never"><div class="metric"><strong>{{ runtimes.length }}</strong><span>运行时</span></div></el-card>
      <el-card shadow="never"><div class="metric"><strong>{{ readyRuntimeCount }}</strong><span>已就绪</span></div></el-card>
      <el-card shadow="never"><div class="metric"><strong>{{ health?.status || "unknown" }}</strong><span>中心状态</span></div></el-card>
    </div>

    <el-card shadow="never" class="section-card">
      <template #header>
        <div class="section-head"><span>游戏插件</span><span class="muted">{{ plugins.length }} 项</span></div>
      </template>
      <el-empty v-if="!loading && plugins.length === 0" description="尚未安装游戏插件" />
      <el-table v-else :data="plugins" v-loading="loading" style="width: 100%">
        <el-table-column prop="name" label="名称" min-width="160" />
        <el-table-column prop="version" label="版本" width="110" />
        <el-table-column label="状态" width="120">
          <template #default="scope"><el-tag :type="scope.row.enabled ? 'success' : 'info'">{{ scope.row.enabled ? "已启用" : "已禁用" }}</el-tag></template>
        </el-table-column>
        <el-table-column prop="health" label="健康" width="120" />
        <el-table-column prop="runtimeCount" label="运行时" width="90" />
        <el-table-column label="操作" width="360" fixed="right">
          <template #default="scope">
            <el-button size="small" @click="showPluginDetail(scope.row)">详情</el-button>
            <el-button size="small" :loading="busy === scope.row.extensionId" @click="togglePlugin(scope.row)">{{ scope.row.enabled ? "禁用" : "启用" }}</el-button>
            <el-button size="small" :loading="busy === scope.row.extensionId" @click="updatePlugin(scope.row)">更新</el-button>
            <el-button size="small" type="danger" plain :loading="busy === scope.row.extensionId" @click="uninstall(scope.row)">卸载</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-card shadow="never" class="section-card">
      <template #header>
        <div class="section-head"><span>运行时</span><span class="muted">{{ runtimes.length }} 项</span></div>
      </template>
      <el-empty v-if="!loading && runtimes.length === 0" description="暂无游戏运行时" />
      <el-table v-else :data="runtimes" v-loading="loading" style="width: 100%">
        <el-table-column prop="runtimeId" label="Runtime ID" min-width="210" show-overflow-tooltip />
        <el-table-column prop="state" label="状态" width="120" />
        <el-table-column prop="health" label="健康" width="120" />
        <el-table-column label="连接" width="100"><template #default="scope">{{ scope.row.connected ? "已连接" : "离线" }}</template></el-table-column>
        <el-table-column label="控制" width="120"><template #default="scope">{{ scope.row.controlMode || "-" }}</template></el-table-column>
        <el-table-column label="操作" width="390" fixed="right">
          <template #default="scope">
            <el-button size="small" @click="showRuntimeDetail(scope.row)">详情</el-button>
            <el-button size="small" :loading="busy === scope.row.runtimeId" @click="runtimeAction(scope.row.runtimeId, 'start')">启动</el-button>
            <el-button size="small" :loading="busy === scope.row.runtimeId" @click="runtimeAction(scope.row.runtimeId, 'restart')">重启</el-button>
            <el-button size="small" :loading="busy === scope.row.runtimeId" @click="runtimeAction(scope.row.runtimeId, 'stop')">停止</el-button>
            <el-dropdown @command="(command) => controlAction(scope.row.runtimeId, String(command))">
              <el-button size="small">控制</el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="takeover">接管</el-dropdown-item>
                  <el-dropdown-item command="release">释放</el-dropdown-item>
                  <el-dropdown-item command="emergency-stop" divided>紧急停止</el-dropdown-item>
                  <el-dropdown-item command="rearm">重新解锁</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="pluginDialogVisible" title="游戏插件详情" width="720px">
      <el-skeleton v-if="detailLoading" :rows="8" animated />
      <template v-else-if="pluginDetail">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="名称">{{ pluginDetail.name }}</el-descriptions-item>
          <el-descriptions-item label="版本">{{ pluginDetail.version }}</el-descriptions-item>
          <el-descriptions-item label="Extension ID">{{ pluginDetail.extensionId }}</el-descriptions-item>
          <el-descriptions-item label="Plugin ID">{{ pluginDetail.pluginId }}</el-descriptions-item>
          <el-descriptions-item label="管理目标">{{ pluginDetail.managementTarget || '-' }}</el-descriptions-item>
          <el-descriptions-item label="健康">{{ pluginHealth?.status || pluginDetail.healthSummary?.status || '-' }}</el-descriptions-item>
          <el-descriptions-item label="能力" :span="2">{{ (pluginDetail.capabilities || []).join(', ') || '-' }}</el-descriptions-item>
          <el-descriptions-item label="权限" :span="2">{{ (pluginDetail.permissions || []).join(', ') || '-' }}</el-descriptions-item>
        </el-descriptions>
      </template>
    </el-dialog>

    <el-dialog v-model="runtimeDialogVisible" title="游戏运行时详情" width="860px">
      <el-skeleton v-if="detailLoading" :rows="10" animated />
      <template v-else-if="runtimeDetail">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="Runtime ID">{{ runtimeDetail.runtimeId }}</el-descriptions-item>
          <el-descriptions-item label="状态">{{ runtimeDetail.runtimeState }}</el-descriptions-item>
          <el-descriptions-item label="健康">{{ runtimeHealth?.status || runtimeDetail.healthSummary?.status || '-' }}</el-descriptions-item>
          <el-descriptions-item label="连接">{{ runtimeDetail.connection?.connected ? '已连接' : '未连接' }}</el-descriptions-item>
          <el-descriptions-item label="握手">{{ runtimeHandshake?.handshakeState || runtimeDetail.handshake?.handshakeState || '-' }}</el-descriptions-item>
          <el-descriptions-item label="控制模式">{{ runtimeAuthority?.mode || runtimeDetail.controlAuthority?.mode || '-' }}</el-descriptions-item>
          <el-descriptions-item label="Authority Epoch">{{ runtimeAuthority?.epoch ?? runtimeDetail.controlAuthority?.epoch ?? '-' }}</el-descriptions-item>
          <el-descriptions-item label="服务数">{{ runtimeServices.length }}</el-descriptions-item>
        </el-descriptions>

        <div class="dialog-section-title">运行时服务</div>
        <el-table :data="runtimeServices" border size="small">
          <el-table-column prop="serviceId" label="Service ID" min-width="200" show-overflow-tooltip />
          <el-table-column prop="state" label="状态" width="100" />
          <el-table-column prop="health" label="健康" width="100" />
          <el-table-column label="操作" width="110">
            <template #default="scope"><el-button size="small" @click="openRpc(scope.row)">RPC</el-button></template>
          </el-table-column>
        </el-table>
      </template>
    </el-dialog>

    <el-dialog v-model="rpcDialogVisible" title="Service RPC" width="680px">
      <el-form label-position="top">
        <el-form-item label="Service ID"><el-input :model-value="rpcServiceId" disabled /></el-form-item>
        <el-form-item label="Method"><el-input v-model="rpcMethod" placeholder="例如 get_state" /></el-form-item>
        <el-form-item label="Payload (JSON)"><el-input v-model="rpcPayload" type="textarea" :rows="8" placeholder="{}" /></el-form-item>
        <el-form-item label="Timeout (ms)"><el-input-number v-model="rpcTimeout" :min="100" :max="120000" /></el-form-item>
      </el-form>
      <el-alert v-if="rpcResult" :title="'RPC 返回'" type="success" :closable="false"><pre class="json-pre">{{ rpcResult }}</pre></el-alert>
      <template #footer>
        <el-button @click="rpcDialogVisible = false">关闭</el-button>
        <el-button type="primary" :loading="rpcLoading" @click="invokeRpc">调用</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useApi } from "@/composables/useApi";

type Plugin = {
  extensionId: string;
  pluginId: string;
  name: string;
  version: string;
  enabled: boolean;
  health: string;
  runtimeCount: number;
};

type Runtime = {
  runtimeId: string;
  pluginId?: string;
  state: string;
  health: string;
  connected: boolean;
  ready: boolean;
  controlMode: string;
};

type GameService = { serviceId: string; state?: string; health?: string };

const api = useApi();
const loading = ref(false);
const detailLoading = ref(false);
const busy = ref("");
const error = ref("");
const health = ref<Record<string, any> | null>(null);
const plugins = ref<Plugin[]>([]);
const runtimes = ref<Runtime[]>([]);
const readyRuntimeCount = computed(() => runtimes.value.filter((item) => item.ready).length);

const pluginDialogVisible = ref(false);
const pluginDetail = ref<Record<string, any> | null>(null);
const pluginHealth = ref<Record<string, any> | null>(null);
const runtimeDialogVisible = ref(false);
const runtimeDetail = ref<Record<string, any> | null>(null);
const runtimeHealth = ref<Record<string, any> | null>(null);
const runtimeHandshake = ref<Record<string, any> | null>(null);
const runtimeAuthority = ref<Record<string, any> | null>(null);
const runtimeServices = ref<GameService[]>([]);
const selectedRuntimeId = ref("");

const rpcDialogVisible = ref(false);
const rpcLoading = ref(false);
const rpcServiceId = ref("");
const rpcMethod = ref("");
const rpcPayload = ref("{}");
const rpcTimeout = ref(30000);
const rpcResult = ref("");

async function refresh() {
  loading.value = true;
  error.value = "";
  try {
    const [healthResult, pluginResult, runtimeResult] = await Promise.all([
      api.get<Record<string, any>>("/api/game-center/health"),
      api.get<{ items?: Plugin[] }>("/api/game-center/plugins", { page: 1, pageSize: 100 }),
      api.get<{ items?: Runtime[] }>("/api/game-center/runtimes", { page: 1, pageSize: 100 }),
    ]);
    health.value = healthResult;
    plugins.value = pluginResult?.items ?? [];
    runtimes.value = runtimeResult?.items ?? [];
  } catch (err: any) {
    error.value = err?.message || "游戏中心加载失败";
  } finally {
    loading.value = false;
  }
}

async function installPlugin() {
  try {
    const { value } = await ElMessageBox.prompt("请输入游戏插件安装包在后端宿主上的完整路径", "安装游戏插件", {
      confirmButtonText: "安装",
      cancelButtonText: "取消",
      inputPlaceholder: "/path/to/plugin.amitiax",
    });
    if (!value?.trim()) return;
    await api.post("/api/game-center/plugins/install", { archivePath: value.trim() });
    ElMessage.success("游戏插件已安装");
    await refresh();
  } catch (err: any) {
    if (err === "cancel" || err === "close") return;
    ElMessage.error(err?.message || "安装失败");
  }
}

async function updatePlugin(plugin: Plugin) {
  try {
    const { value } = await ElMessageBox.prompt("请输入更新包在后端宿主上的完整路径", `更新 ${plugin.name}`, {
      confirmButtonText: "更新",
      cancelButtonText: "取消",
    });
    if (!value?.trim()) return;
    busy.value = plugin.extensionId;
    await api.post(`/api/game-center/plugins/${encodeURIComponent(plugin.extensionId)}/update`, { archivePath: value.trim() });
    ElMessage.success("插件已更新");
    await refresh();
  } catch (err: any) {
    if (err === "cancel" || err === "close") return;
    ElMessage.error(err?.message || "更新失败");
  } finally {
    busy.value = "";
  }
}

async function showPluginDetail(plugin: Plugin) {
  pluginDialogVisible.value = true;
  detailLoading.value = true;
  pluginDetail.value = null;
  pluginHealth.value = null;
  try {
    const [detail, healthResult] = await Promise.all([
      api.get<Record<string, any>>(`/api/game-center/plugins/${encodeURIComponent(plugin.pluginId)}`, { extensionId: plugin.extensionId }),
      api.get<Record<string, any>>(`/api/game-center/plugins/${encodeURIComponent(plugin.pluginId)}/health`),
    ]);
    pluginDetail.value = detail;
    pluginHealth.value = healthResult;
  } catch (err: any) {
    ElMessage.error(err?.message || "加载插件详情失败");
  } finally {
    detailLoading.value = false;
  }
}

async function showRuntimeDetail(runtime: Runtime) {
  selectedRuntimeId.value = runtime.runtimeId;
  runtimeDialogVisible.value = true;
  detailLoading.value = true;
  runtimeDetail.value = null;
  runtimeHealth.value = null;
  runtimeHandshake.value = null;
  runtimeAuthority.value = null;
  runtimeServices.value = [];
  try {
    const id = encodeURIComponent(runtime.runtimeId);
    const [detail, servicesResult, healthResult, handshakeResult, authorityResult] = await Promise.all([
      api.get<Record<string, any>>(`/api/game-center/runtimes/${id}`, runtime.pluginId ? { pluginId: runtime.pluginId } : undefined),
      api.get<{ items?: GameService[] }>(`/api/game-center/runtimes/${id}/services`),
      api.get<Record<string, any>>(`/api/game-center/runtimes/${id}/health`),
      api.get<Record<string, any>>(`/api/game-center/runtimes/${id}/handshake`),
      api.get<Record<string, any>>(`/api/game-center/runtimes/${id}/authority`),
    ]);
    runtimeDetail.value = detail;
    runtimeServices.value = servicesResult?.items ?? detail?.services ?? [];
    runtimeHealth.value = healthResult;
    runtimeHandshake.value = handshakeResult;
    runtimeAuthority.value = authorityResult;
  } catch (err: any) {
    ElMessage.error(err?.message || "加载运行时详情失败");
  } finally {
    detailLoading.value = false;
  }
}

function openRpc(service: GameService) {
  rpcServiceId.value = service.serviceId;
  rpcMethod.value = "";
  rpcPayload.value = "{}";
  rpcResult.value = "";
  rpcDialogVisible.value = true;
}

async function invokeRpc() {
  if (!selectedRuntimeId.value || !rpcServiceId.value || !rpcMethod.value.trim()) {
    ElMessage.warning("请填写 RPC Method");
    return;
  }
  let payload: any = undefined;
  try {
    const text = rpcPayload.value.trim();
    payload = text ? JSON.parse(text) : undefined;
  } catch {
    ElMessage.error("Payload 必须是合法 JSON");
    return;
  }
  rpcLoading.value = true;
  try {
    const result = await api.post(
      `/api/game-center/runtimes/${encodeURIComponent(selectedRuntimeId.value)}/services/${encodeURIComponent(rpcServiceId.value)}/rpc`,
      { method: rpcMethod.value.trim(), payload, timeoutMs: rpcTimeout.value },
    );
    rpcResult.value = JSON.stringify(result ?? null, null, 2);
  } catch (err: any) {
    ElMessage.error(err?.message || "RPC 调用失败");
  } finally {
    rpcLoading.value = false;
  }
}

async function togglePlugin(plugin: Plugin) {
  busy.value = plugin.extensionId;
  try {
    await api.post(`/api/game-center/plugins/${encodeURIComponent(plugin.extensionId)}/${plugin.enabled ? "disable" : "enable"}`);
    ElMessage.success(plugin.enabled ? "插件已禁用" : "插件已启用");
    await refresh();
  } catch (err: any) {
    ElMessage.error(err?.message || "插件状态更新失败");
  } finally {
    busy.value = "";
  }
}

async function uninstall(plugin: Plugin) {
  await ElMessageBox.confirm(`确定卸载「${plugin.name}」吗？`, "卸载游戏插件", { type: "warning", confirmButtonText: "卸载", cancelButtonText: "取消" });
  busy.value = plugin.extensionId;
  try {
    await api.del(`/api/game-center/plugins/${encodeURIComponent(plugin.extensionId)}`);
    ElMessage.success("插件已卸载");
    await refresh();
  } catch (err: any) {
    ElMessage.error(err?.message || "卸载失败");
  } finally {
    busy.value = "";
  }
}

async function runtimeAction(runtimeId: string, action: "start" | "stop" | "restart") {
  busy.value = runtimeId;
  try {
    await api.post(`/api/game-center/runtimes/${encodeURIComponent(runtimeId)}/${action}`);
    ElMessage.success("运行时操作已提交");
    await refresh();
  } catch (err: any) {
    ElMessage.error(err?.message || "运行时操作失败");
  } finally {
    busy.value = "";
  }
}

async function controlAction(runtimeId: string, action: string) {
  busy.value = runtimeId;
  try {
    if (action === "release") {
      const authority = await api.get<Record<string, any>>(`/api/game-center/runtimes/${encodeURIComponent(runtimeId)}/authority`);
      await api.post(`/api/game-center/runtimes/${encodeURIComponent(runtimeId)}/release`, {
        targetMode: "observe",
        expectedEpoch: authority?.epoch ?? 0,
      });
    } else {
      await api.post(`/api/game-center/runtimes/${encodeURIComponent(runtimeId)}/${action}`);
    }
    ElMessage.success("控制状态已更新");
    await refresh();
  } catch (err: any) {
    ElMessage.error(err?.message || "控制操作失败");
  } finally {
    busy.value = "";
  }
}

onMounted(refresh);
</script>

<style scoped>
.game-center-page { padding: 24px; display: flex; flex-direction: column; gap: 18px; }
.page-head, .section-head { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.page-head h2 { margin: 0 0 6px; font-size: 22px; }
.page-head p { margin: 0; color: var(--el-text-color-secondary); }
.head-actions { display: flex; gap: 8px; }
.summary-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
.metric { display: flex; flex-direction: column; gap: 6px; }
.metric strong { font-size: 24px; line-height: 1; }
.metric span, .muted { color: var(--el-text-color-secondary); font-size: 13px; }
.section-card { min-width: 0; }
.dialog-section-title { margin: 18px 0 10px; font-weight: 600; }
.json-pre { margin: 8px 0 0; max-height: 260px; overflow: auto; white-space: pre-wrap; word-break: break-word; }
@media (max-width: 900px) { .summary-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } .game-center-page { padding: 16px; } .page-head { align-items: flex-start; flex-direction: column; } }
</style>
