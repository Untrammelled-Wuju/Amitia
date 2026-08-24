<template>
  <div class="game-mode-page">
    <header class="page-head">
      <div class="page-copy">
<h1>游戏模式</h1>
        <p>通过游戏插件连接目标游戏。GameHost 只负责插件运行、权限与连接生命周期，具体感知和控制能力由插件提供。</p>
      </div>
      <div class="head-actions">
        <el-button :icon="Refresh" :loading="loading" @click="refresh">刷新</el-button>
        <el-button type="primary" :icon="Plus" @click="openInstallDialog()">添加游戏</el-button>
      </div>
    </header>

    <el-alert v-if="error" :title="error" type="error" show-icon :closable="false" />

    <section class="current-game-card" :class="{ 'is-connected': !!activeRuntime?.connected }">
      <div class="current-game-visual" aria-hidden="true">
        <div class="visual-grid"></div>
        <div class="visual-mark">{{ gameInitial(activePlugin?.name || 'GAME') }}</div>
      </div>

      <div class="current-game-content">
        <div class="current-game-meta">
          <span class="status-pill" :class="activeRuntime?.connected ? 'online' : 'offline'">
            <span class="status-dot"></span>
            {{ activeRuntime?.connected ? "插件运行时已连接" : "等待插件运行时连接" }}
          </span>
          <span v-if="activePlugin" class="version-text">v{{ activePlugin.version }}</span>
        </div>

        <h2>{{ activePlugin?.name || "还没有可用的游戏插件" }}</h2>
        <p v-if="activeRuntime?.connected">
          插件 Runtime 已连接{{ activeRuntime.ready ? "并完成准备" : "，正在准备中" }}。插件向 Agent 注册的能力可在对话中使用；GameHost 不解释这些能力的游戏语义。
        </p>
        <p v-else-if="activePlugin">
          游戏扩展已安装。由插件检测并连接其支持的游戏，GameHost 只承载插件 Runtime 与通信通道。
        </p>
        <p v-else>
          添加一个 `.amitiax` 游戏扩展。安装完成后，Amitia 会自动把它归入游戏模式。
        </p>

        <div class="current-game-stats">
          <div class="stat-item">
            <span>扩展状态</span>
            <strong>{{ activePlugin ? (activePlugin.enabled ? "已启用" : "已禁用") : "未安装" }}</strong>
          </div>
          <div class="stat-item">
            <span>连接状态</span>
            <strong>{{ activeRuntime?.connected ? "在线" : "离线" }}</strong>
          </div>
          <div class="stat-item">
            <span>控制模式</span>
            <strong>{{ controlModeLabel(activeRuntime?.controlMode) }}</strong>
          </div>
        </div>

        <div class="current-game-actions">
          <el-button
            v-if="activeRuntime?.connected"
            type="primary"
            size="large"
            :icon="VideoPlay"
            :disabled="!activeRuntime.ready"
            @click="goToGameChat"
          >
            去对话使用插件能力
          </el-button>
          <el-button
            v-if="activeRuntime?.connected && activeRuntime.controlMode !== 'user_control'"
            size="large"
            :loading="busy === activeRuntime.runtimeId"
            @click="takeManualControl"
          >手动接管</el-button>
          <el-button
            v-if="activeRuntime?.connected && activeRuntime.controlMode === 'user_control'"
            size="large"
            :loading="busy === activeRuntime.runtimeId"
            @click="releaseManualControl"
          >释放手动控制</el-button>
          <el-button v-if="!activePlugin" size="large" @click="openInstallDialog()">选择游戏扩展</el-button>
          <el-button v-else-if="!activeRuntime?.connected" size="large" @click="refresh">检查连接</el-button>
        </div>
      </div>
    </section>

    <section class="content-section">
      <div class="section-heading">
        <div>
          <h2>游戏插件</h2>
          <p>已安装的游戏扩展。安装、更新和权限校验统一由扩展包内核处理。</p>
        </div>
        <span class="section-count">{{ plugins.length }} 个</span>
      </div>

      <div v-if="loading && plugins.length === 0" class="game-grid loading-grid">
        <div v-for="index in 3" :key="index" class="game-card skeleton-card"></div>
      </div>

      <div v-else-if="plugins.length" class="game-grid">
        <article
          v-for="plugin in plugins"
          :key="plugin.extensionId"
          class="game-card"
          :class="{ 'is-active': activePlugin?.extensionId === plugin.extensionId }"
          @click="showPluginDetail(plugin)"
        >
          <div class="game-card-top">
            <div class="game-icon">{{ gameInitial(plugin.name) }}</div>
            <el-dropdown trigger="click" @command="(command) => pluginMenuAction(plugin, String(command))">
              <button class="icon-button" type="button" aria-label="游戏扩展操作" @click.stop>
                <el-icon><MoreFilled /></el-icon>
              </button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="detail">扩展详情</el-dropdown-item>
                  <el-dropdown-item command="update">从本地更新</el-dropdown-item>
                  <el-dropdown-item command="toggle">{{ plugin.enabled ? "禁用" : "启用" }}</el-dropdown-item>
                  <el-dropdown-item command="uninstall" divided>卸载</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>

          <div class="game-card-body">
            <div class="game-card-title-row">
              <h3>{{ plugin.name }}</h3>
              <span class="mini-status" :class="pluginRuntime(plugin)?.connected ? 'online' : 'offline'">
                {{ pluginRuntime(plugin)?.connected ? "已连接" : plugin.enabled ? "待连接" : "已禁用" }}
              </span>
            </div>
            <p>{{ plugin.runtimeCount ? `${plugin.runtimeCount} 个运行连接` : "暂无运行连接" }}</p>
          </div>

          <div class="game-card-footer">
            <span>v{{ plugin.version }}</span>
            <button class="text-action" type="button" @click.stop="showPluginDetail(plugin)">
              查看详情
              <el-icon><ArrowRight /></el-icon>
            </button>
          </div>
        </article>

        <button class="add-game-card" type="button" @click="openInstallDialog()">
          <span class="add-icon"><el-icon><Plus /></el-icon></span>
          <strong>添加游戏</strong>
          <span>安装 .amitiax 游戏扩展</span>
        </button>
      </div>

      <button v-else class="empty-game-state" type="button" @click="openInstallDialog()">
        <span class="empty-game-icon"><el-icon><Plus /></el-icon></span>
        <strong>添加第一个游戏</strong>
        <span>选择或拖入 `.amitiax` 游戏扩展，安装前会进行安全检查和权限预览。</span>
      </button>
    </section>

    <section v-if="runtimes.length" class="content-section runtime-section">
      <div class="section-heading">
        <div>
          <h2>运行连接</h2>
          <p>这里只展示插件 Runtime 的连接和生命周期状态。协议、权限纪元和原始 Service RPC 仅在开发者访问已启用时开放。</p>
        </div>
        <span class="section-count">{{ readyRuntimeCount }}/{{ runtimes.length }} 已就绪</span>
      </div>

      <div class="runtime-list">
        <article v-for="runtime in runtimes" :key="runtime.runtimeId" class="runtime-card">
          <div class="runtime-main">
            <span class="runtime-status-dot" :class="runtime.connected ? 'online' : 'offline'"></span>
            <div class="runtime-copy">
              <strong>{{ runtimeName(runtime) }}</strong>
              <span>{{ runtime.connected ? "已连接" : "离线" }} · {{ stateLabel(runtime.state) }} · {{ healthLabel(runtime.health) }}</span>
            </div>
          </div>

          <div class="runtime-actions">
            <el-button
              v-if="runtime.state !== 'running'"
              size="small"
              :loading="busy === runtime.runtimeId"
              @click="runtimeAction(runtime.runtimeId, 'start')"
            >启动</el-button>
            <el-button
              v-else
              size="small"
              :loading="busy === runtime.runtimeId"
              @click="runtimeAction(runtime.runtimeId, 'restart')"
            >重启</el-button>
            <el-button
              v-if="runtime.state === 'running'"
              size="small"
              :loading="busy === runtime.runtimeId"
              @click="runtimeAction(runtime.runtimeId, 'stop')"
            >停止</el-button>
            <el-button size="small" text @click="showRuntimeDetail(runtime)">{{ developerAccess ? "开发者详情" : "运行详情" }}</el-button>
          </div>
        </article>
      </div>
    </section>

    <el-dialog
      v-model="installDialogVisible"
      :title="installMode === 'update' ? `更新 ${updateTarget?.name || '游戏扩展'}` : '添加游戏'"
      width="680px"
      destroy-on-close
      class="game-install-dialog"
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
          <span>{{ formatBytes(installFile.size) }} · {{ previewLoading ? `正在检查 ${uploadProgress}%` : "已选择" }}</span>
        </template>
        <template v-else>
          <strong>选择或拖入 .amitiax 游戏扩展</strong>
          <span>不会再要求填写后端宿主机文件路径</span>
        </template>
        <el-button :loading="previewLoading" @click="choosePackage">{{ installFile ? "重新选择" : "选择文件" }}</el-button>
        <input ref="packageInput" class="sr-only" type="file" accept=".amitiax" @change="onPackageFile" />
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
          <span class="target-badge" :class="previewIsGame ? 'ok' : 'bad'">
            {{ previewIsGame ? "游戏扩展" : "非游戏扩展" }}
          </span>
        </div>

        <div class="preview-facts">
          <div><span>版本</span><strong>{{ installPreview.version }}</strong></div>
          <div><span>签名</span><strong>{{ signatureLabel(installPreview.signature?.status) }}</strong></div>
          <div><span>兼容性</span><strong>{{ installPreview.compatible ? "通过" : "不兼容" }}</strong></div>
          <div><span>目标</span><strong>{{ installPreview.managementTarget || "未知" }}</strong></div>
        </div>

        <el-alert
          v-if="!previewIsGame"
          title="这个扩展不属于游戏模式，不能从这里安装。请到扩展中心安装普通扩展。"
          type="error"
          show-icon
          :closable="false"
        />
        <el-alert
          v-else-if="installPreview.errors?.length"
          :title="installPreview.errors.join('；')"
          type="error"
          show-icon
          :closable="false"
        />

        <div v-if="installPreview.highRiskCapabilities?.length" class="preview-block">
          <span class="preview-label">高风险项</span>
          <div class="chip-row">
            <span v-for="capability in installPreview.highRiskCapabilities" :key="capability" class="permission-chip risk">
              {{ capability }}
            </span>
          </div>
        </div>

        <div v-else-if="installPreview.capabilities?.length" class="preview-block">
          <span class="preview-label">申请能力</span>
          <div class="chip-row">
            <span v-for="capability in installPreview.capabilities.slice(0, 8)" :key="capability" class="permission-chip">
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

        <el-checkbox v-if="needsInstallAcknowledgement" v-model="installAcknowledged" class="install-confirmation">
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

    <el-dialog v-model="pluginDialogVisible" title="游戏扩展" width="680px">
      <el-skeleton v-if="detailLoading" :rows="7" animated />
      <template v-else-if="pluginDetail">
        <div class="detail-hero">
          <div class="game-icon large">{{ gameInitial(pluginDetail.name || "G") }}</div>
          <div>
            <h3>{{ pluginDetail.name }}</h3>
            <p>{{ pluginDetail.extensionId }} · v{{ pluginDetail.version }}</p>
          </div>
          <span class="mini-status" :class="pluginHealth?.status === 'healthy' ? 'online' : 'offline'">
            {{ healthLabel(pluginHealth?.status || pluginDetail.healthSummary?.status) }}
          </span>
        </div>

        <div class="detail-grid">
          <div><span>管理目标</span><strong>{{ pluginDetail.managementTarget || "game_center" }}</strong></div>
          <div><span>Plugin ID</span><strong>{{ pluginDetail.pluginId || "-" }}</strong></div>
          <div><span>运行连接</span><strong>{{ pluginDetail.runtimeCount ?? "-" }}</strong></div>
          <div><span>状态</span><strong>{{ pluginDetail.enabled === false ? "已禁用" : "已启用" }}</strong></div>
        </div>

        <div class="detail-section">
          <span class="preview-label">能力</span>
          <div class="chip-row">
            <span v-for="capability in pluginDetail.capabilities || []" :key="capability" class="permission-chip">{{ capability }}</span>
            <span v-if="!(pluginDetail.capabilities || []).length" class="muted">未声明</span>
          </div>
        </div>

        <div class="detail-section">
          <span class="preview-label">权限</span>
          <div class="chip-row">
            <span v-for="permission in pluginDetail.permissions || []" :key="permission" class="permission-chip">{{ permission }}</span>
            <span v-if="!(pluginDetail.permissions || []).length" class="muted">未声明</span>
          </div>
        </div>
      </template>
    </el-dialog>

    <el-dialog v-model="runtimeDialogVisible" title="开发者详情" width="860px">
      <el-skeleton v-if="detailLoading" :rows="10" animated />
      <template v-else-if="runtimeDetail">
        <el-alert
          title="以下内容用于 GameHost 调试，普通游戏操作不需要修改这些参数。"
          type="info"
          :closable="false"
          show-icon
        />

        <div class="developer-grid">
          <div><span>Runtime ID</span><code>{{ runtimeDetail.runtimeId }}</code></div>
          <div><span>状态</span><strong>{{ stateLabel(runtimeDetail.runtimeState) }}</strong></div>
          <div><span>健康</span><strong>{{ healthLabel(runtimeHealth?.status || runtimeDetail.healthSummary?.status) }}</strong></div>
          <div><span>连接</span><strong>{{ runtimeDetail.connection?.connected ? "已连接" : "未连接" }}</strong></div>
          <div><span>Handshake</span><strong>{{ runtimeHandshake?.handshakeState || runtimeDetail.handshake?.handshakeState || "-" }}</strong></div>
          <div><span>控制模式</span><strong>{{ controlModeLabel(runtimeAuthority?.mode || runtimeDetail.controlAuthority?.mode) }}</strong></div>
          <div><span>Authority Epoch</span><strong>{{ runtimeAuthority?.epoch ?? runtimeDetail.controlAuthority?.epoch ?? "-" }}</strong></div>
          <div><span>服务</span><strong>{{ runtimeServices.length }}</strong></div>
        </div>

        <div class="developer-actions">
          <el-button size="small" @click="controlAction(selectedRuntimeId, 'takeover')">接管控制</el-button>
          <el-button size="small" @click="controlAction(selectedRuntimeId, 'release')">释放控制</el-button>
          <el-button size="small" type="danger" plain @click="controlAction(selectedRuntimeId, 'emergency-stop')">紧急停止</el-button>
          <el-button size="small" @click="controlAction(selectedRuntimeId, 'rearm')">重新解锁</el-button>
        </div>

        <div class="dialog-section-title">运行时服务</div>
        <el-table :data="runtimeServices" border size="small" empty-text="暂无运行时服务">
          <el-table-column prop="serviceId" label="Service ID" min-width="220" show-overflow-tooltip />
          <el-table-column prop="state" label="状态" width="110" />
          <el-table-column prop="health" label="健康" width="110" />
          <el-table-column label="操作" width="110">
            <template #default="scope"><el-button v-if="developerAccess" size="small" @click="openRpc(scope.row)">RPC</el-button><span v-else>-</span></template>
          </el-table-column>
        </el-table>
      </template>
    </el-dialog>

    <el-dialog v-if="developerAccess" v-model="rpcDialogVisible" title="Service RPC" width="680px">
      <el-form label-position="top">
        <el-form-item label="Service ID"><el-input :model-value="rpcServiceId" disabled /></el-form-item>
        <el-form-item label="Method"><el-input v-model="rpcMethod" placeholder="例如 get_state" /></el-form-item>
        <el-form-item label="Payload (JSON)"><el-input v-model="rpcPayload" type="textarea" :rows="8" placeholder="{}" /></el-form-item>
        <el-form-item label="Timeout (ms)"><el-input-number v-model="rpcTimeout" :min="100" :max="120000" /></el-form-item>
      </el-form>
      <el-alert v-if="rpcResult" title="RPC 返回" type="success" :closable="false"><pre class="json-pre">{{ rpcResult }}</pre></el-alert>
      <template #footer>
        <el-button @click="rpcDialogVisible = false">关闭</el-button>
        <el-button type="primary" :loading="rpcLoading" @click="invokeRpc">调用</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  ArrowRight,
  MoreFilled,
  Plus,
  Refresh,
  UploadFilled,
  VideoPlay,
} from "@element-plus/icons-vue";
import { useApi } from "@/composables/useApi";
import {
  installExtensionPackage,
  previewExtensionPackage,
} from "@/views/extensions/api";
import type { PackageImportPreview } from "@/views/extensions/types";

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
type PackageOperationView = { status?: string; errorCode?: string };

const api = useApi();
const router = useRouter();
const loading = ref(false);
const detailLoading = ref(false);
const busy = ref("");
const error = ref("");
const plugins = ref<Plugin[]>([]);
const runtimes = ref<Runtime[]>([]);
const developerAccess = ref(false);

const readyRuntimeCount = computed(() => runtimes.value.filter((item) => item.ready).length);
const activeRuntime = computed<Runtime | null>(() =>
  runtimes.value.find((item) => item.connected && item.ready)
  || runtimes.value.find((item) => item.connected)
  || runtimes.value.find((item) => item.ready)
  || null,
);
const activePlugin = computed<Plugin | null>(() => {
  const runtime = activeRuntime.value;
  if (runtime?.pluginId) {
    const matched = plugins.value.find((item) => item.pluginId === runtime.pluginId);
    if (matched) return matched;
  }
  return plugins.value.find((item) => item.enabled) || plugins.value[0] || null;
});

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

const installDialogVisible = ref(false);
const installMode = ref<"install" | "update">("install");
const updateTarget = ref<Plugin | null>(null);
const packageInput = ref<HTMLInputElement>();
const installFile = ref<File | null>(null);
const installPreview = ref<PackageImportPreview | null>(null);
const previewLoading = ref(false);
const installLoading = ref(false);
const uploadProgress = ref(0);
const installAcknowledged = ref(false);

const previewIsGame = computed(() => {
  const preview = installPreview.value;
  if (!preview) return false;
  return preview.managementTarget === "game_center" || preview.contributionKinds?.includes("game_plugin") === true;
});

const needsInstallAcknowledgement = computed(() => {
  const preview = installPreview.value;
  if (!preview) return false;
  return preview.signature?.status === "unsigned"
    || preview.scripts > 0
    || (preview.highRiskCapabilities?.length || 0) > 0
    || (preview.capabilityConfirmations?.length || 0) > 0
    || (preview.warnings?.length || 0) > 0;
});

const canInstallPreview = computed(() => {
  const preview = installPreview.value;
  if (!preview || previewLoading.value || installLoading.value) return false;
  if (!previewIsGame.value || !preview.compatible || (preview.errors?.length || 0) > 0) return false;
  if (installMode.value === "update" && updateTarget.value && preview.id !== updateTarget.value.extensionId) return false;
  return !needsInstallAcknowledgement.value || installAcknowledged.value;
});

async function refresh() {
  loading.value = true;
  error.value = "";
  try {
    const [pluginResult, runtimeResult, developerResult] = await Promise.all([
      api.get<{ items?: Plugin[] }>("/api/game-center/plugins", { page: 1, pageSize: 100 }),
      api.get<{ items?: Runtime[] }>("/api/game-center/runtimes", { page: 1, pageSize: 100 }),
      api.get<{ enabled?: boolean }>("/api/game-center/developer-access").catch(() => ({ enabled: false })),
    ]);
    plugins.value = pluginResult?.items ?? [];
    runtimes.value = runtimeResult?.items ?? [];
    developerAccess.value = developerResult?.enabled === true;
  } catch (err: any) {
    error.value = err?.message || "游戏模式加载失败";
  } finally {
    loading.value = false;
  }
}

function gameInitial(name: string) {
  const cleaned = String(name || "GAME").trim();
  return cleaned.slice(0, 4).toUpperCase();
}

function pluginRuntime(plugin: Plugin) {
  return runtimes.value.find((item) => item.pluginId === plugin.pluginId && item.connected)
    || runtimes.value.find((item) => item.pluginId === plugin.pluginId)
    || null;
}

function runtimeName(runtime: Runtime) {
  const plugin = plugins.value.find((item) => item.pluginId === runtime.pluginId);
  return plugin?.name || runtime.pluginId || "Game Runtime";
}

function stateLabel(value?: string) {
  const labels: Record<string, string> = {
    created: "已创建",
    running: "运行中",
    degraded: "受限运行",
    suspended: "已暂停",
    restarting: "重启中",
    stopped: "已停止",
    starting: "启动中",
    stopping: "停止中",
    failed: "异常",
    ready: "已就绪",
  };
  return labels[String(value || "").toLowerCase()] || value || "未知";
}

function healthLabel(value?: string) {
  const labels: Record<string, string> = {
    healthy: "正常",
    ok: "正常",
    ready: "正常",
    degraded: "受限",
    unhealthy: "异常",
    failed: "异常",
    unknown: "未知",
  };
  return labels[String(value || "").toLowerCase()] || value || "未知";
}

function controlModeLabel(value?: string) {
  const labels: Record<string, string> = {
    observe_only: "观察",
    observe: "观察",
    assist: "辅助控制",
    shared_control: "共享控制",
    plugin_control: "插件控制",
    user_control: "手动接管",
    suspended: "已暂停",
  };
  return labels[String(value || "").toLowerCase()] || value || "未接管";
}

function signatureLabel(value?: string) {
  const labels: Record<string, string> = {
    "valid-trusted": "可信签名",
    "valid-untrusted": "有效 / 未信任",
    unsigned: "未签名",
    invalid: "签名无效",
  };
  return labels[String(value || "").toLowerCase()] || value || "未知";
}

function formatBytes(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  const value = bytes / 1024 ** index;
  return `${value >= 10 || index === 0 ? value.toFixed(0) : value.toFixed(1)} ${units[index]}`;
}

function goToGameChat() {
  void router.push("/chat");
}

async function takeManualControl() {
  const runtime = activeRuntime.value;
  if (!runtime?.connected) {
    ElMessage.warning("当前没有已连接的游戏运行时");
    return;
  }
  if (!runtime.ready) {
    ElMessage.warning("游戏运行时尚未准备完成");
    return;
  }
  await controlAction(runtime.runtimeId, "takeover", "已切换为手动接管");
}

async function releaseManualControl() {
  const runtime = activeRuntime.value;
  if (!runtime) return;
  await controlAction(runtime.runtimeId, "release", "已释放手动控制");
}

function openInstallDialog(plugin?: Plugin) {
  resetInstallState();
  installMode.value = plugin ? "update" : "install";
  updateTarget.value = plugin || null;
  installDialogVisible.value = true;
}

function resetInstallState() {
  installFile.value = null;
  installPreview.value = null;
  previewLoading.value = false;
  installLoading.value = false;
  uploadProgress.value = 0;
  installAcknowledged.value = false;
  updateTarget.value = null;
  installMode.value = "install";
}

async function choosePackage() {
  const desktop = window.amitiaDesktop;
  if (!desktop?.selectExtensionPackage) {
    packageInput.value?.click();
    return;
  }
  try {
    const selected = await desktop.selectExtensionPackage();
    if (!selected) return;
    const bytes = Uint8Array.from(atob(selected.base64), (character) => character.charCodeAt(0));
    await setPackageFile(new File([bytes], selected.name, { type: "application/zip" }));
  } catch (err: any) {
    ElMessage.error(err?.message || "选择扩展包失败");
  }
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
    ElMessage.warning("请选择 .amitiax 游戏扩展包");
    return;
  }
  installFile.value = file;
  installPreview.value = null;
  installAcknowledged.value = false;
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
      updateTarget.value?.extensionId || "",
      (percent) => { uploadProgress.value = percent; },
    );
    installPreview.value = preview;
    if (preview.managementTarget !== "game_center" && !preview.contributionKinds?.includes("game_plugin")) {
      ElMessage.error("该 .amitiax 包不是游戏扩展，已阻止从游戏模式安装");
      return;
    }
    if (updateTarget.value && preview.id !== updateTarget.value.extensionId) {
      ElMessage.error(`更新包 ID 不匹配：需要 ${updateTarget.value.extensionId}，实际为 ${preview.id}`);
    }
  } catch (err: any) {
    installPreview.value = null;
    ElMessage.error(err?.message || "扩展包预览失败");
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
      installMode.value === "update" ? updateTarget.value?.extensionId || "" : "",
    );
    await waitForPackageOperation(result.operationId);
    ElMessage.success(installMode.value === "update" ? "游戏扩展已更新" : "游戏扩展已安装");
    installDialogVisible.value = false;
    await refresh();
  } catch (err: any) {
    ElMessage.error(err?.message || (installMode.value === "update" ? "更新失败" : "安装失败"));
  } finally {
    installLoading.value = false;
  }
}

async function waitForPackageOperation(operationId?: string) {
  if (!operationId) return;
  for (let attempt = 0; attempt < 120; attempt += 1) {
    const operation = await api.get<PackageOperationView>(`/api/extensions/packages/operations/${encodeURIComponent(operationId)}`);
    const status = String(operation?.status || "").toLowerCase();
    if (status === "completed") return;
    if (status === "failed" || status === "requires_recovery") {
      throw new Error(operation?.errorCode || "扩展包操作失败");
    }
    await new Promise((resolve) => window.setTimeout(resolve, 500));
  }
  throw new Error("扩展包操作等待超时，请刷新游戏中心检查最终状态");
}

async function pluginMenuAction(plugin: Plugin, command: string) {
  if (command === "detail") {
    await showPluginDetail(plugin);
    return;
  }
  if (command === "update") {
    openInstallDialog(plugin);
    return;
  }
  if (command === "toggle") {
    await togglePlugin(plugin);
    return;
  }
  if (command === "uninstall") {
    await uninstall(plugin);
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
    ElMessage.error(err?.message || "加载游戏扩展详情失败");
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
    ElMessage.error(err?.message || "加载运行时开发者详情失败");
  } finally {
    detailLoading.value = false;
  }
}

function openRpc(service: GameService) {
  if (!developerAccess.value) {
    ElMessage.warning("Service RPC 仅在开发者访问已启用时开放");
    return;
  }
  rpcServiceId.value = service.serviceId;
  rpcMethod.value = "";
  rpcPayload.value = "{}";
  rpcResult.value = "";
  rpcDialogVisible.value = true;
}

async function invokeRpc() {
  if (!developerAccess.value) {
    ElMessage.error("当前账号没有 GameHost 开发者访问权限");
    rpcDialogVisible.value = false;
    return;
  }
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
    await api.post(`/api/game-center/extensions/${encodeURIComponent(plugin.extensionId)}/${plugin.enabled ? "disable" : "enable"}`);
    ElMessage.success(plugin.enabled ? "游戏扩展已禁用" : "游戏扩展已启用");
    await refresh();
  } catch (err: any) {
    ElMessage.error(err?.message || "游戏扩展状态更新失败");
  } finally {
    busy.value = "";
  }
}

async function uninstall(plugin: Plugin) {
  busy.value = plugin.extensionId;
  try {
    const preview = await api.post<Record<string, any>>(
      "/api/extensions/kernel/extensions/uninstall/preview",
      { extensionId: plugin.extensionId, scopeType: "global", scopeId: "" },
    );
    if (preview?.uninstallable === false) {
      throw new Error("后端判定当前游戏扩展不可卸载");
    }
    const dependents = Array.isArray(preview?.dependents) ? preview.dependents.map(String) : [];
    const required = Array.isArray(preview?.requiredConfirmations)
      ? preview.requiredConfirmations.map(String).filter(Boolean)
      : [];
    const detail = [
      `确定卸载「${plugin.name}」吗？对应游戏能力将立即从游戏模式移除。`,
      dependents.length ? `\n依赖此扩展：${dependents.join("、")}` : "",
      preview?.artifactPolicy ? `\n制品策略：${String(preview.artifactPolicy)}` : "",
    ].join("");
    await ElMessageBox.confirm(detail, "卸载游戏扩展", {
      type: "warning",
      confirmButtonText: "卸载",
      cancelButtonText: "取消",
    });

    const confirmation = await api.post<Record<string, any>>(
      "/api/extensions/kernel/extensions/uninstall/confirm",
      {
        extensionId: plugin.extensionId,
        scopeType: "global",
        scopeId: "",
        confirmations: Object.fromEntries(required.map((key) => [key, true])),
      },
    );
    const confirmationToken = String(confirmation?.confirmationToken || "");
    if (!confirmationToken) throw new Error("卸载确认令牌缺失");

    const result = await api.post<{ operationId?: string }>("/api/extensions/kernel/extensions/uninstall", {
      extensionId: plugin.extensionId,
      scopeType: "global",
      scopeId: "",
      confirmationToken,
    });
    await waitForPackageOperation(result?.operationId);
    ElMessage.success("游戏扩展已卸载");
    await refresh();
  } catch (err: any) {
    if (err === "cancel" || err === "close" || err?.toString?.().includes("cancel")) return;
    ElMessage.error(err?.message || "卸载失败");
  } finally {
    busy.value = "";
  }
}

async function runtimeAction(runtimeId: string, action: "start" | "stop" | "restart") {
  busy.value = runtimeId;
  try {
    await api.post(`/api/game-center/runtimes/${encodeURIComponent(runtimeId)}/${action}`);
    ElMessage.success(action === "start" ? "启动请求已提交" : action === "stop" ? "停止请求已提交" : "重启请求已提交");
    await refresh();
  } catch (err: any) {
    ElMessage.error(err?.message || "运行时操作失败");
  } finally {
    busy.value = "";
  }
}

async function controlAction(runtimeId: string, action: string, successMessage = "控制状态已更新") {
  if (!runtimeId) return;
  busy.value = runtimeId;
  try {
    if (action === "release") {
      const authority = await api.get<Record<string, any>>(`/api/game-center/runtimes/${encodeURIComponent(runtimeId)}/authority`);
      await api.post(`/api/game-center/runtimes/${encodeURIComponent(runtimeId)}/release`, {
        targetMode: "observe_only",
        expectedEpoch: authority?.epoch ?? 0,
      });
    } else {
      await api.post(`/api/game-center/runtimes/${encodeURIComponent(runtimeId)}/${action}`);
    }
    ElMessage.success(successMessage);
    await refresh();
    if (runtimeDialogVisible.value) {
      const runtime = runtimes.value.find((item) => item.runtimeId === runtimeId);
      if (runtime) await showRuntimeDetail(runtime);
    }
  } catch (err: any) {
    ElMessage.error(err?.message || "控制操作失败");
  } finally {
    busy.value = "";
  }
}

onMounted(refresh);
</script>

<style scoped>
.game-mode-page {
  --game-primary: var(--tp-primary, #8a5728);
  --game-primary-hover: var(--tp-primary-hover, #74451e);
  --game-primary-soft: var(--tp-primary-soft, #f1e4d4);
  --game-panel: var(--tp-panel, #fcfbf8);
  --game-panel-soft: var(--tp-panel-soft, #f6f3ed);
  --game-text: var(--tp-text, #24221f);
  --game-text-secondary: var(--tp-text-secondary, #5f5b54);
  --game-text-muted: var(--tp-text-muted, #706b63);
  --game-border: var(--tp-border, #d8d2c7);
  --game-border-light: var(--tp-border-light, #e6e1d8);
  --game-success: var(--tp-success, #3f7653);
  --game-success-soft: var(--tp-success-soft, #e4efe7);
  --game-danger: var(--tp-danger, #a83f3f);
  --game-danger-soft: var(--tp-danger-soft, #f5e3e1);
  color: var(--game-text);
  display: flex;
  flex-direction: column;
  gap: 24px;
  max-width: 1440px;
  margin: 0 auto;
}

.page-head,
.section-heading,
.game-card-top,
.game-card-title-row,
.game-card-footer,
.runtime-card,
.runtime-main,
.runtime-actions,
.current-game-meta,
.current-game-actions,
.detail-hero {
  display: flex;
  align-items: center;
}

.page-head,
.section-heading,
.game-card-top,
.game-card-footer,
.runtime-card {
  justify-content: space-between;
}

.page-head {
  gap: 20px;
  align-items: flex-start;
}

.page-copy {
  min-width: 0;
}

.eyebrow,
.preview-kicker,
.preview-label {
  display: block;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--game-primary);
}

.page-head h1 {
  margin: 0 0 6px;
  color: var(--console-text);
  font-size: 24px;
}

.current-game-content p,
.game-card-body p,
.preview-head p,
.detail-hero p {
  margin: 0;
  color: var(--game-text-secondary);
}

.page-head p,
.section-heading p {
  margin: 0;
  color: var(--console-text-muted);
  line-height: 1.6;
  font-size: 13px;
}

.head-actions {
  display: flex;
  gap: 10px;
  flex: 0 0 auto;
}

.current-game-card {
  position: relative;
  min-height: 310px;
  display: grid;
  grid-template-columns: minmax(260px, 0.85fr) minmax(420px, 1.45fr);
  overflow: hidden;
  border: 1px solid var(--game-border);
  border-radius: 22px;
  background: var(--game-panel);
  box-shadow: var(--tp-shadow-card, 0 1px 2px rgba(48, 40, 31, 0.05));
}

.current-game-card.is-connected {
  border-color: var(--tp-primary-border, rgba(138, 87, 40, 0.28));
}

.current-game-visual {
  position: relative;
  min-height: 310px;
  overflow: hidden;
  background:
    radial-gradient(circle at 25% 20%, color-mix(in srgb, var(--game-primary) 24%, transparent), transparent 38%),
    linear-gradient(135deg, var(--game-panel-soft), color-mix(in srgb, var(--game-primary) 10%, var(--game-panel)));
  border-right: 1px solid var(--game-border-light);
}

.visual-grid {
  position: absolute;
  inset: 0;
  opacity: 0.32;
  background-image:
    linear-gradient(var(--game-border-light) 1px, transparent 1px),
    linear-gradient(90deg, var(--game-border-light) 1px, transparent 1px);
  background-size: 32px 32px;
  mask-image: linear-gradient(135deg, #000, transparent 78%);
}

.visual-mark {
  position: absolute;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  width: 118px;
  height: 118px;
  border-radius: 30px;
  display: grid;
  place-items: center;
  font-size: 38px;
  font-weight: 700;
  color: var(--game-primary);
  background: color-mix(in srgb, var(--game-panel) 84%, transparent);
  border: 1px solid var(--tp-primary-border, rgba(138, 87, 40, 0.28));
  box-shadow: var(--tp-shadow-float, 0 18px 48px rgba(36, 32, 27, 0.15));
  backdrop-filter: blur(12px);
}

.current-game-content {
  padding: 34px 38px;
  display: flex;
  flex-direction: column;
  justify-content: center;
}

.current-game-meta {
  gap: 10px;
}

.status-pill,
.mini-status,
.target-badge {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
}

.status-pill {
  padding: 6px 10px;
  background: var(--game-panel-soft);
  color: var(--game-text-secondary);
}

.status-pill.online,
.mini-status.online,
.target-badge.ok {
  color: var(--game-success);
  background: var(--game-success-soft);
}

.status-pill.offline,
.mini-status.offline {
  color: var(--game-text-muted);
  background: var(--game-panel-soft);
}

.target-badge.bad {
  color: var(--game-danger);
  background: var(--game-danger-soft);
}

.status-dot,
.runtime-status-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: currentColor;
  flex: 0 0 auto;
}

.version-text {
  font-size: 12px;
  color: var(--game-text-muted);
}

.current-game-content h2 {
  margin: 15px 0 8px;
  font-size: clamp(26px, 3vw, 38px);
  line-height: 1.08;
  letter-spacing: -0.03em;
}

.current-game-content > p {
  max-width: 680px;
  line-height: 1.65;
  font-size: 14px;
}

.current-game-stats {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
  margin: 24px 0;
}

.stat-item {
  padding: 12px 14px;
  border-radius: 12px;
  background: var(--game-panel-soft);
  border: 1px solid var(--game-border-light);
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.stat-item span,
.detail-grid span,
.developer-grid span,
.preview-facts span {
  color: var(--game-text-muted);
  font-size: 11px;
}

.stat-item strong,
.detail-grid strong,
.developer-grid strong,
.preview-facts strong {
  font-size: 13px;
  font-weight: 600;
}

.current-game-actions {
  gap: 10px;
  flex-wrap: wrap;
}

.content-section {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.section-heading {
  gap: 16px;
}

.section-heading h2 {
  margin: 0 0 5px;
  font-size: 18px;
}

.section-count {
  font-size: 12px;
  color: var(--game-text-muted);
  background: var(--game-panel-soft);
  border: 1px solid var(--game-border-light);
  border-radius: 999px;
  padding: 5px 9px;
}

.game-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
  gap: 12px;
}

.game-card,
.add-game-card,
.empty-game-state,
.runtime-card {
  border: 1px solid var(--game-border);
  background: var(--game-panel);
  border-radius: 16px;
}

.game-card {
  min-width: 0;
  padding: 16px;
  cursor: pointer;
  transition: border-color 160ms ease, transform 160ms ease, background 160ms ease;
}

.game-card:hover {
  border-color: var(--tp-primary-border, rgba(138, 87, 40, 0.28));
  transform: translateY(-1px);
}

.game-card.is-active {
  border-color: var(--tp-primary-border, rgba(138, 87, 40, 0.28));
  box-shadow: inset 0 0 0 1px var(--tp-primary-border, rgba(138, 87, 40, 0.14));
}

.game-icon {
  width: 42px;
  height: 42px;
  border-radius: 12px;
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  color: var(--game-primary);
  background: var(--game-primary-soft);
  font-size: 13px;
  font-weight: 700;
}

.game-icon.large {
  width: 54px;
  height: 54px;
  border-radius: 15px;
  font-size: 16px;
}

.icon-button,
.text-action,
.add-game-card,
.empty-game-state {
  appearance: none;
  border: 0;
  font: inherit;
  color: inherit;
}

.icon-button {
  width: 32px;
  height: 32px;
  border-radius: 9px;
  display: grid;
  place-items: center;
  cursor: pointer;
  color: var(--game-text-muted);
  background: transparent;
}

.icon-button:hover {
  background: var(--game-panel-soft);
  color: var(--game-text);
}

.game-card-body {
  padding: 18px 0 16px;
}

.game-card-title-row {
  justify-content: space-between;
  gap: 10px;
}

.game-card-body h3,
.preview-head h3,
.detail-hero h3 {
  margin: 0;
}

.game-card-body h3 {
  font-size: 16px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.mini-status,
.target-badge {
  padding: 4px 8px;
  white-space: nowrap;
}

.game-card-body p {
  margin-top: 7px;
  font-size: 12px;
}

.game-card-footer {
  padding-top: 12px;
  border-top: 1px solid var(--game-border-light);
  color: var(--game-text-muted);
  font-size: 12px;
}

.text-action {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 0;
  background: transparent;
  cursor: pointer;
  color: var(--game-text-secondary);
}

.text-action:hover {
  color: var(--game-primary);
}

.add-game-card,
.empty-game-state {
  cursor: pointer;
  border-style: dashed;
  color: var(--game-text-secondary);
  background: color-mix(in srgb, var(--game-panel) 72%, transparent);
}

.add-game-card {
  min-height: 178px;
  padding: 20px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 7px;
}

.add-game-card:hover,
.empty-game-state:hover {
  color: var(--game-primary);
  border-color: var(--tp-primary-border, rgba(138, 87, 40, 0.28));
  background: var(--game-primary-soft);
}

.add-icon,
.empty-game-icon {
  display: grid;
  place-items: center;
  border-radius: 50%;
  background: var(--game-primary-soft);
  color: var(--game-primary);
}

.add-icon {
  width: 38px;
  height: 38px;
}

.add-game-card strong {
  font-size: 14px;
}

.add-game-card > span:last-child {
  font-size: 11px;
  color: var(--game-text-muted);
}

.empty-game-state {
  min-height: 180px;
  padding: 32px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  gap: 9px;
}

.empty-game-icon {
  width: 44px;
  height: 44px;
  font-size: 20px;
}

.empty-game-state strong {
  font-size: 15px;
}

.empty-game-state > span:last-child {
  max-width: 520px;
  font-size: 12px;
  line-height: 1.6;
}

.loading-grid .skeleton-card {
  min-height: 178px;
  cursor: default;
  animation: pulse 1.3s ease-in-out infinite alternate;
}

@keyframes pulse {
  from { opacity: 0.45; }
  to { opacity: 0.82; }
}

.runtime-section {
  margin-top: 4px;
}

.runtime-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.runtime-card {
  min-height: 70px;
  padding: 12px 14px;
  gap: 16px;
}

.runtime-main {
  gap: 12px;
  min-width: 0;
}

.runtime-status-dot {
  width: 9px;
  height: 9px;
}

.runtime-status-dot.online {
  color: var(--game-success);
}

.runtime-status-dot.offline {
  color: var(--game-text-muted);
}

.runtime-copy {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.runtime-copy strong {
  font-size: 13px;
}

.runtime-copy span {
  font-size: 11px;
  color: var(--game-text-muted);
}

.runtime-actions {
  gap: 6px;
  flex: 0 0 auto;
}

.package-drop-zone {
  min-height: 188px;
  border: 1px dashed var(--game-border);
  border-radius: 16px;
  padding: 24px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  text-align: center;
  background: var(--game-panel-soft);
  transition: border-color 160ms ease, background 160ms ease;
}

.package-drop-zone:hover,
.package-drop-zone.has-file {
  border-color: var(--tp-primary-border, rgba(138, 87, 40, 0.28));
  background: color-mix(in srgb, var(--game-primary-soft) 55%, var(--game-panel));
}

.upload-icon {
  font-size: 32px;
  color: var(--game-primary);
  margin-bottom: 4px;
}

.package-drop-zone strong {
  font-size: 14px;
}

.package-drop-zone span {
  font-size: 12px;
  color: var(--game-text-muted);
  margin-bottom: 6px;
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
  margin-top: 4px;
  font-size: 12px;
  max-width: 470px;
}

.preview-facts,
.detail-grid,
.developer-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.preview-facts > div,
.detail-grid > div,
.developer-grid > div {
  min-width: 0;
  padding: 10px 12px;
  border-radius: 10px;
  background: var(--game-panel-soft);
  border: 1px solid var(--game-border-light);
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.preview-block,
.detail-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.chip-row {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.permission-chip {
  max-width: 100%;
  padding: 5px 8px;
  border-radius: 8px;
  background: var(--game-panel-soft);
  color: var(--game-text-secondary);
  border: 1px solid var(--game-border-light);
  font-size: 11px;
  word-break: break-all;
}

.permission-chip.risk {
  color: var(--game-danger);
  background: var(--game-danger-soft);
  border-color: color-mix(in srgb, var(--game-danger) 22%, transparent);
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
  background: var(--game-panel-soft);
  font-size: 11px;
}

.dependency-list small {
  color: var(--game-text-muted);
}

.preview-warning {
  margin-top: -2px;
}

.install-confirmation {
  height: auto;
  white-space: normal;
  align-items: flex-start;
}

.detail-hero {
  gap: 12px;
  margin-bottom: 16px;
}

.detail-hero > div:nth-child(2) {
  min-width: 0;
  flex: 1;
}

.detail-hero p {
  margin-top: 4px;
  font-size: 11px;
  word-break: break-all;
}

.detail-grid,
.developer-grid {
  margin-bottom: 16px;
}

.detail-section + .detail-section {
  margin-top: 14px;
}

.developer-grid {
  margin-top: 14px;
}

.developer-grid code {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 11px;
}

.developer-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 16px;
}

.dialog-section-title {
  margin: 18px 0 10px;
  font-weight: 600;
  font-size: 13px;
}

.json-pre {
  margin: 8px 0 0;
  max-height: 260px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 11px;
}

.muted {
  color: var(--game-text-muted);
  font-size: 12px;
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

@media (max-width: 1000px) {
  .game-mode-page {
    padding: 22px 20px 34px;
  }

  .current-game-card {
    grid-template-columns: 0.75fr 1.25fr;
  }

  .current-game-content {
    padding: 28px;
  }
}

@media (max-width: 760px) {
  .game-mode-page {
    padding: 18px 14px 28px;
    gap: 20px;
  }

  .page-head,
  .section-heading,
  .runtime-card {
    align-items: flex-start;
    flex-direction: column;
  }

  .head-actions,
  .runtime-actions {
    width: 100%;
  }

  .head-actions :deep(.el-button),
  .runtime-actions :deep(.el-button) {
    flex: 1;
  }

  .current-game-card {
    grid-template-columns: 1fr;
  }

  .current-game-visual {
    min-height: 180px;
    border-right: 0;
    border-bottom: 1px solid var(--game-border-light);
  }

  .visual-mark {
    width: 88px;
    height: 88px;
    border-radius: 24px;
    font-size: 28px;
  }

  .current-game-content {
    padding: 24px 20px;
  }

  .current-game-stats,
  .preview-facts,
  .detail-grid,
  .developer-grid {
    grid-template-columns: 1fr;
  }

  .game-grid {
    grid-template-columns: 1fr;
  }

  .runtime-actions {
    flex-wrap: wrap;
  }
}
</style>
