<template>
  <div class="game-center-page">
    <div class="page-head">
      <div>
        <h2>游戏中心</h2>
        <p>管理游戏插件、运行时和控制权限。数据直接来自 Game Center Runtime。</p>
      </div>
      <el-button :loading="loading" @click="refresh">刷新</el-button>
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
        <el-table-column label="操作" width="240" fixed="right">
          <template #default="scope">
            <el-button size="small" :loading="busy === scope.row.extensionId" @click="togglePlugin(scope.row)">{{ scope.row.enabled ? "禁用" : "启用" }}</el-button>
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
        <el-table-column label="操作" width="320" fixed="right">
          <template #default="scope">
            <el-button size="small" :loading="busy === scope.row.runtimeId" @click="runtimeAction(scope.row.runtimeId, 'start')">启动</el-button>
            <el-button size="small" :loading="busy === scope.row.runtimeId" @click="runtimeAction(scope.row.runtimeId, 'restart')">重启</el-button>
            <el-button size="small" :loading="busy === scope.row.runtimeId" @click="runtimeAction(scope.row.runtimeId, 'stop')">停止</el-button>
            <el-dropdown @command="(command) => controlAction(scope.row.runtimeId, command)">
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
  state: string;
  health: string;
  connected: boolean;
  ready: boolean;
  controlMode: string;
};

const api = useApi();
const loading = ref(false);
const busy = ref("");
const error = ref("");
const health = ref<Record<string, any> | null>(null);
const plugins = ref<Plugin[]>([]);
const runtimes = ref<Runtime[]>([]);
const readyRuntimeCount = computed(() => runtimes.value.filter((item) => item.ready).length);

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

async function togglePlugin(plugin: Plugin) {
  busy.value = plugin.extensionId;
  try {
    await api.post(`/api/game-center/plugins/${encodeURIComponent(plugin.extensionId)}/${plugin.enabled ? "disable" : "enable"}`);
    ElMessage.success(plugin.enabled ? "插件已禁用" : "插件已启用");
    await refresh();
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
  } finally {
    busy.value = "";
  }
}

async function controlAction(runtimeId: string, action: string) {
  busy.value = runtimeId;
  try {
    await api.post(`/api/game-center/runtimes/${encodeURIComponent(runtimeId)}/${action}`);
    ElMessage.success("控制状态已更新");
    await refresh();
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
.summary-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
.metric { display: flex; flex-direction: column; gap: 6px; }
.metric strong { font-size: 24px; line-height: 1; }
.metric span, .muted { color: var(--el-text-color-secondary); font-size: 13px; }
.section-card { min-width: 0; }
@media (max-width: 900px) { .summary-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } .game-center-page { padding: 16px; } }
</style>
