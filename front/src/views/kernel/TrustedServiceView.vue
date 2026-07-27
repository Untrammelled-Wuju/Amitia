<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  listServices,
  registerService,
  unregisterService,
  startService,
  stopService,
  healthCheck,
  invokeService,
  listQuarantined,
  releaseQuarantine,
  type TrustedServiceInstance,
  type QuarantineRecord,
  type QuarantineHistoryEntry,
  type ServiceRuntimeDefinition,
} from "./trusted-service-api";

const loading = ref(false);
const services = ref<TrustedServiceInstance[]>([]);
const quarantineActive = ref<QuarantineRecord[]>([]);
const quarantineHistory = ref<QuarantineHistoryEntry[]>([]);
const activeTab = ref("services");

const registerDialogVisible = ref(false);
const invokeDialogVisible = ref(false);
const currentServiceForInvoke = ref<TrustedServiceInstance | null>(null);
const invokeOperation = ref("");
const invokeInput = ref("{}");
const invokeTimeout = ref("30s");
const invokeResult = ref<string>("");

const registerForm = ref({
  service_id: "",
  extension_id: "",
  module_id: "",
  name: "",
  description: "",
  publisher: "",
  trust_level: "trusted",
  protocol: "jsonrpc-stdio",
  executable_path: "",
  executable_platform: "windows/amd64",
  health_check_type: "process",
  auto_start: false,
  loopback_only: true,
});

const stateTagType = (state: string): "" | "success" | "warning" | "danger" | "info" => {
  switch (state) {
    case "ready": return "success";
    case "starting": return "warning";
    case "degraded": return "warning";
    case "crashed": return "danger";
    case "quarantined": return "danger";
    case "failed": return "danger";
    case "stopped": return "info";
    default: return "";
  }
};

const trustTagType = (trust: string): "" | "success" | "warning" | "danger" | "info" => {
  switch (trust) {
    case "official": return "success";
    case "trusted": return "success";
    case "community": return "warning";
    case "unknown": return "danger";
    default: return "info";
  }
};

const reasonLabel = (reason: string): string => {
  const map: Record<string, string> = {
    signature_failure: "签名验证失败",
    binary_hash_changed: "二进制哈希变更",
    publisher_revoked: "发布者已被吊销",
    process_tree_unkillable: "进程树无法终止",
    undeclared_child_process: "未声明子进程",
    undeclared_public_port: "未声明端口监听",
    protocol_identity_mismatch: "协议身份不匹配",
    host_api_violation: "Host API 违规",
    frequent_crash: "频繁崩溃",
    resource_limit_exceeded: "资源超限",
    package_tampered: "包被篡改",
  };
  return map[reason] || reason;
};

const totalServices = computed(() => services.value.length);
const runningServices = computed(() => services.value.filter(s => s.state === "ready").length);
const quarantinedCount = computed(() => quarantineActive.value.length);

async function fetchServices() {
  loading.value = true;
  try {
    const data = await listServices();
    services.value = data.services || [];
  } catch (e: any) {
    ElMessage.error("加载服务列表失败: " + (e.message || e));
  } finally {
    loading.value = false;
  }
}

async function fetchQuarantine() {
  try {
    const data = await listQuarantined();
    quarantineActive.value = data.active || [];
    quarantineHistory.value = data.history || [];
  } catch (e: any) {
    ElMessage.error("加载隔离列表失败: " + (e.message || e));
  }
}

async function refreshAll() {
  await Promise.all([fetchServices(), fetchQuarantine()]);
}

async function handleStart(serviceId: string) {
  try {
    await ElMessageBox.confirm("确认启动该可信服务?", "启动确认", { type: "warning" });
  } catch { return; }
  try {
    const result = await startService(serviceId, {});
    ElMessage.success(`服务已启动 (PID: ${result.pid})`);
    await fetchServices();
  } catch (e: any) {
    ElMessage.error("启动失败: " + (e.message || e));
  }
}

async function handleStop(serviceId: string) {
  try {
    await ElMessageBox.confirm("确认停止该可信服务?", "停止确认", { type: "warning" });
  } catch { return; }
  try {
    await stopService(serviceId, { reason: "manual", force: false });
    ElMessage.success("服务已停止");
    await fetchServices();
  } catch (e: any) {
    ElMessage.error("停止失败: " + (e.message || e));
  }
}

async function handleHealthCheck(serviceId: string) {
  try {
    const result = await healthCheck(serviceId);
    const statusText = result.status === "healthy" ? "健康" : result.status === "unhealthy" ? "不健康" : result.status;
    ElMessage.info(`健康检查: ${statusText}`);
  } catch (e: any) {
    ElMessage.error("健康检查失败: " + (e.message || e));
  }
}

async function handleUnregister(serviceId: string) {
  try {
    await ElMessageBox.confirm("确认注销该服务定义? 此操作不可恢复.", "注销确认", { type: "error" });
  } catch { return; }
  try {
    await unregisterService(serviceId);
    ElMessage.success("服务已注销");
    await fetchServices();
  } catch (e: any) {
    ElMessage.error("注销失败: " + (e.message || e));
  }
}

function openInvokeDialog(service: TrustedServiceInstance) {
  currentServiceForInvoke.value = service;
  invokeOperation.value = "";
  invokeInput.value = "{}";
  invokeTimeout.value = "30s";
  invokeResult.value = "";
  invokeDialogVisible.value = true;
}

async function executeInvoke() {
  if (!currentServiceForInvoke.value || !invokeOperation.value) {
    ElMessage.warning("请输入操作名称");
    return;
  }
  let parsedInput: unknown = null;
  try {
    parsedInput = JSON.parse(invokeInput.value);
  } catch {
    ElMessage.warning("输入 JSON 格式无效");
    return;
  }
  try {
    const result = await invokeService(currentServiceForInvoke.value.service_id, {
      operation: invokeOperation.value,
      input: parsedInput,
      timeout: invokeTimeout.value,
    });
    invokeResult.value = JSON.stringify(result.output, null, 2);
    ElMessage.success("调用成功");
  } catch (e: any) {
    invokeResult.value = "Error: " + (e.message || String(e));
    ElMessage.error("调用失败");
  }
}

async function handleReleaseQuarantine(serviceId: string) {
  try {
    await ElMessageBox.confirm("确认解除该服务的隔离状态?", "解除隔离", { type: "warning" });
  } catch { return; }
  try {
    await releaseQuarantine(serviceId);
    ElMessage.success("隔离已解除");
    await fetchQuarantine();
  } catch (e: any) {
    ElMessage.error("解除失败: " + (e.message || e));
  }
}

async function handleRegister() {
  if (!registerForm.value.service_id || !registerForm.value.name) {
    ElMessage.warning("服务 ID 和名称为必填项");
    return;
  }
  const def: Partial<ServiceRuntimeDefinition> = {
    service_id: registerForm.value.service_id,
    extension_id: registerForm.value.extension_id || "standalone",
    module_id: registerForm.value.module_id || "default",
    name: registerForm.value.name,
    description: registerForm.value.description,
    publisher: registerForm.value.publisher || "local",
    trust_level: registerForm.value.trust_level,
    protocol: registerForm.value.protocol,
    auto_start: registerForm.value.auto_start,
    executables: [{
      platform: registerForm.value.executable_platform,
      path: registerForm.value.executable_path,
      sha256: "",
      signature: { algorithm: "none", value: "", trusted: false },
    }],
    health_check: {
      type: registerForm.value.health_check_type,
      interval: 30000000000,
      timeout: 5000000000,
      grace_period: 10000000000,
      max_consecutive_fails: 3,
    },
    recovery: {
      max_restarts: 3,
      restart_delay: 2000000000,
      backoff_multiplier: 2,
      max_restart_delay: 60000000000,
      quarantine_on_fail: true,
    },
    shutdown: {
      grace_period: 5000000000,
      kill_timeout: 10000000000,
      cleanup_children: true,
      remove_temp_dir: true,
    },
    limits: {
      max_memory_mb: 512,
      max_cpu_percent: 50,
      max_file_descriptors: 1024,
      max_disk_mb: 1024,
      max_subprocesses: 10,
    },
    network: {
      allow_inbound: false,
      allow_outbound: true,
      loopback_only: registerForm.value.loopback_only,
      require_proxy: false,
      audit_all: true,
    },
    allowed_namespaces: [],
    manifest_hash: "",
    definition_version: 1,
  };
  try {
    await registerService(def);
    ElMessage.success("服务定义已注册");
    registerDialogVisible.value = false;
    await fetchServices();
  } catch (e: any) {
    ElMessage.error("注册失败: " + (e.message || e));
  }
}

onMounted(() => {
  refreshAll();
});
</script>

<template>
  <div class="trusted-service-view">
    <div class="page-header">
      <div class="header-left">
        <h2>可信服务运行时</h2>
        <p class="subtitle">Trusted Service Runtime - 高权限原生服务进程管理</p>
      </div>
      <div class="header-right">
        <el-button @click="refreshAll" :loading="loading">刷新</el-button>
        <el-button type="primary" @click="registerDialogVisible = true">注册服务</el-button>
      </div>
    </div>

    <div class="stats-row">
      <el-card class="stat-card" shadow="hover">
        <div class="stat-value">{{ totalServices }}</div>
        <div class="stat-label">总服务数</div>
      </el-card>
      <el-card class="stat-card" shadow="hover">
        <div class="stat-value stat-running">{{ runningServices }}</div>
        <div class="stat-label">运行中</div>
      </el-card>
      <el-card class="stat-card" shadow="hover">
        <div class="stat-value stat-quarantine">{{ quarantinedCount }}</div>
        <div class="stat-label">已隔离</div>
      </el-card>
    </div>

    <el-tabs v-model="activeTab" class="content-tabs">
      <el-tab-pane label="服务列表" name="services">
        <el-table :data="services" v-loading="loading" stripe style="width: 100%">
          <el-table-column prop="service_id" label="服务 ID" min-width="180" show-overflow-tooltip />
          <el-table-column prop="name" label="名称" min-width="120" show-overflow-tooltip />
          <el-table-column prop="state" label="状态" width="110" align="center">
            <template #default="{ row }">
              <el-tag :type="stateTagType(row.state)" size="small">{{ row.state }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="trust_level" label="信任级别" width="100" align="center">
            <template #default="{ row }">
              <el-tag :type="trustTagType(row.trust_level)" size="small" effect="plain">{{ row.trust_level }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="protocol" label="协议" width="130" />
          <el-table-column prop="pid" label="PID" width="80" align="center" />
          <el-table-column prop="generation" label="版本" width="70" align="center" />
          <el-table-column prop="restart_count" label="重启次数" width="90" align="center" />
          <el-table-column prop="health_fails" label="健康失败" width="90" align="center" />
          <el-table-column label="操作" width="320" fixed="right">
            <template #default="{ row }">
              <el-button size="small" type="success" @click="handleStart(row.service_id)" :disabled="row.state === 'ready'">启动</el-button>
              <el-button size="small" type="warning" @click="handleStop(row.service_id)" :disabled="row.state !== 'ready' && row.state !== 'degraded'">停止</el-button>
              <el-button size="small" @click="handleHealthCheck(row.service_id)">健康</el-button>
              <el-button size="small" type="info" @click="openInvokeDialog(row)">调用</el-button>
              <el-button size="small" type="danger" @click="handleUnregister(row.service_id)">注销</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <el-tab-pane label="隔离管理" name="quarantine">
        <div class="quarantine-section">
          <h3>当前隔离</h3>
          <el-table :data="quarantineActive" stripe style="width: 100%" empty-text="无隔离服务">
            <el-table-column prop="service_id" label="服务 ID" min-width="180" show-overflow-tooltip />
            <el-table-column prop="reason" label="原因" width="200">
              <template #default="{ row }">
                <el-tag type="danger" size="small">{{ reasonLabel(row.reason) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="detail" label="详情" min-width="200" show-overflow-tooltip />
            <el-table-column prop="quarantined_at" label="隔离时间" width="180" />
            <el-table-column label="操作" width="120" fixed="right">
              <template #default="{ row }">
                <el-button size="small" type="primary" @click="handleReleaseQuarantine(row.service_id)">解除</el-button>
              </template>
            </el-table-column>
          </el-table>
        </div>
        <div class="quarantine-section">
          <h3>历史记录</h3>
          <el-table :data="quarantineHistory" stripe style="width: 100%" empty-text="无历史记录">
            <el-table-column prop="service_id" label="服务 ID" min-width="180" show-overflow-tooltip />
            <el-table-column prop="reason" label="原因" width="200">
              <template #default="{ row }">
                <el-tag type="danger" size="small" effect="plain">{{ reasonLabel(row.reason) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="quarantined_at" label="隔离时间" width="180" />
            <el-table-column prop="released_at" label="解除时间" width="180" />
            <el-table-column prop="release_reason" label="解除原因" min-width="150" show-overflow-tooltip />
          </el-table>
        </div>
      </el-tab-pane>
    </el-tabs>

    <el-dialog v-model="registerDialogVisible" title="注册可信服务" width="640px">
      <el-form :model="registerForm" label-width="120px" label-position="right">
        <el-form-item label="服务 ID" required>
          <el-input v-model="registerForm.service_id" placeholder="如: com.example.myservice" />
        </el-form-item>
        <el-form-item label="名称" required>
          <el-input v-model="registerForm.name" placeholder="服务显示名称" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="registerForm.description" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item label="扩展 ID">
          <el-input v-model="registerForm.extension_id" placeholder="如: com.example.ext (默认 standalone)" />
        </el-form-item>
        <el-form-item label="模块 ID">
          <el-input v-model="registerForm.module_id" placeholder="默认 default" />
        </el-form-item>
        <el-form-item label="发布者">
          <el-input v-model="registerForm.publisher" placeholder="发布者标识" />
        </el-form-item>
        <el-form-item label="信任级别">
          <el-select v-model="registerForm.trust_level" style="width: 100%">
            <el-option label="Official (官方)" value="official" />
            <el-option label="Trusted (可信)" value="trusted" />
            <el-option label="Community (社区)" value="community" />
          </el-select>
        </el-form-item>
        <el-form-item label="协议">
          <el-select v-model="registerForm.protocol" style="width: 100%">
            <el-option label="JSON-RPC over stdio" value="jsonrpc-stdio" />
            <el-option label="HTTP (loopback)" value="http-loopback" />
            <el-option label="Plain stdio" value="plain-stdio" />
          </el-select>
        </el-form-item>
        <el-form-item label="可执行文件路径">
          <el-input v-model="registerForm.executable_path" placeholder="如: ./bin/myservice.exe" />
        </el-form-item>
        <el-form-item label="平台">
          <el-select v-model="registerForm.executable_platform" style="width: 100%">
            <el-option label="Windows AMD64" value="windows/amd64" />
            <el-option label="Windows ARM64" value="windows/arm64" />
            <el-option label="Linux AMD64" value="linux/amd64" />
            <el-option label="macOS ARM64" value="darwin/arm64" />
          </el-select>
        </el-form-item>
        <el-form-item label="健康检查">
          <el-select v-model="registerForm.health_check_type" style="width: 100%">
            <el-option label="进程存活" value="process" />
            <el-option label="RPC 检查" value="rpc" />
            <el-option label="无" value="none" />
          </el-select>
        </el-form-item>
        <el-form-item label="仅回环网络">
          <el-switch v-model="registerForm.loopback_only" />
        </el-form-item>
        <el-form-item label="自动启动">
          <el-switch v-model="registerForm.auto_start" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="registerDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleRegister">注册</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="invokeDialogVisible" title="调用服务" width="720px">
      <div v-if="currentServiceForInvoke" class="invoke-info">
        <span>服务: <strong>{{ currentServiceForInvoke.name || currentServiceForInvoke.service_id }}</strong></span>
        <span>状态: <el-tag :type="stateTagType(currentServiceForInvoke.state)" size="small">{{ currentServiceForInvoke.state }}</el-tag></span>
      </div>
      <el-form label-width="80px" style="margin-top: 16px">
        <el-form-item label="操作">
          <el-input v-model="invokeOperation" placeholder="操作名称, 如: query / execute" />
        </el-form-item>
        <el-form-item label="超时">
          <el-input v-model="invokeTimeout" placeholder="如: 30s / 1m" style="width: 200px" />
        </el-form-item>
        <el-form-item label="输入">
          <el-input v-model="invokeInput" type="textarea" :rows="6" placeholder='JSON 格式输入, 如: {"key": "value"}' />
        </el-form-item>
        <el-form-item label="结果">
          <el-input v-model="invokeResult" type="textarea" :rows="6" readonly placeholder="调用结果将显示在此" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="invokeDialogVisible = false">关闭</el-button>
        <el-button type="primary" @click="executeInvoke">执行</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.trusted-service-view {
  padding: 24px;
  max-width: 1400px;
  margin: 0 auto;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 24px;
}

.page-header h2 {
  margin: 0 0 4px 0;
  font-size: 22px;
}

.subtitle {
  margin: 0;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.stats-row {
  display: flex;
  gap: 16px;
  margin-bottom: 24px;
}

.stat-card {
  flex: 1;
  text-align: center;
}

.stat-value {
  font-size: 28px;
  font-weight: 700;
  color: var(--el-text-color-primary);
}

.stat-running { color: var(--el-color-success); }
.stat-quarantine { color: var(--el-color-danger); }

.stat-label {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  margin-top: 4px;
}

.content-tabs {
  min-height: 400px;
}

.quarantine-section {
  margin-bottom: 24px;
}

.quarantine-section h3 {
  margin: 0 0 12px 0;
  font-size: 16px;
}

.invoke-info {
  display: flex;
  gap: 24px;
  align-items: center;
  padding: 12px 16px;
  background: var(--el-fill-color-light);
  border-radius: 8px;
}
</style>
