<template>
  <div class="advanced-page" data-amitia-ui="settings.section">
    <div class="head">
      <el-button :loading="loading" data-amitia-ui="settings.button" @click="loadAll">刷新</el-button>
    </div>

    <el-tabs v-model="tab" data-amitia-ui="settings.tab">
      <el-tab-pane label="Space 与设备安全" name="security">
        <el-card shadow="never" data-amitia-ui="settings.card">
          <template #header>访问安全</template>
          <el-form label-width="130px">
            <el-form-item label="要求认证"><el-switch v-model="accessConfig.requireAuth" data-amitia-ui="component.switch" /></el-form-item>
            <el-form-item label="允许 Origin"><el-input v-model="accessConfig.allowedOrigins" data-amitia-ui="component.input" /></el-form-item>
            <el-form-item label="限流"><el-switch v-model="accessConfig.rateLimit" data-amitia-ui="component.switch" /></el-form-item>
            <el-button type="primary" data-amitia-ui="settings.button" @click="saveAccess">保存</el-button>
          </el-form>
        </el-card>
        <el-card shadow="never" data-amitia-ui="settings.card"><template #header>设备身份状态</template><pre class="json">{{ pretty(identityCheck) }}</pre></el-card>
        <el-card shadow="never" data-amitia-ui="settings.card"><template #header>访问状态</template><pre class="json">{{ pretty(accessStatus) }}</pre></el-card>
      </el-tab-pane>

      <el-tab-pane label="审计" name="audit">
        <el-card shadow="never" data-amitia-ui="settings.card">
          <template #header>
            <div class="card-head">
              <span>系统审计</span>
              <div><el-button size="small" data-amitia-ui="settings.button" @click="saveAuditSettings">保存设置</el-button><el-button size="small" type="danger" plain data-amitia-ui="settings.button" @click="clearAudit">清空日志</el-button></div>
            </div>
          </template>
          <el-form inline>
            <el-form-item label="启用"><el-switch v-model="auditSettings.enabled" data-amitia-ui="component.switch" /></el-form-item>
            <el-form-item label="记录动作"><el-switch v-model="auditSettings.logActions" data-amitia-ui="component.switch" /></el-form-item>
            <el-form-item label="保留天数"><el-input-number v-model="auditSettings.retentionDays" :min="1" :max="3650" /></el-form-item>
          </el-form>
          <p>日志总数：{{ auditStats.total ?? 0 }} · 可审计动作：{{ auditActions.join('、') || '—' }}</p>
          <el-table :data="auditLogs"><el-table-column prop="time" label="时间" min-width="170"/><el-table-column prop="ruleId" label="规则" min-width="150"/><el-table-column prop="action" label="动作" min-width="180"/></el-table>
        </el-card>
      </el-tab-pane>

      <el-tab-pane label="运行观测" name="observability">
        <el-card shadow="never">
          <template #header>情绪历史记录</template>
          <el-space wrap>
            <el-switch v-model="mood.enabled" @change="saveMood" />
            <span>记录阈值</span>
            <el-input-number v-model="mood.threshold" :min="0" :max="1" :step="0.05" :precision="2" @change="saveMood" />
            <span class="muted">仅在情绪变化强度达到阈值时写入 Owner 情绪历史</span>
          </el-space>
        </el-card>
        <el-card shadow="never">
          <template #header><div class="card-head"><span>Shadow Mode</span><div><el-button size="small" type="primary" @click="shadowStart">启动</el-button><el-button size="small" @click="shadowAdvance">推进阶段</el-button><el-button size="small" type="danger" plain @click="shadowStop">停止</el-button></div></div></template>
          <pre class="json">{{ pretty(shadowStatus) }}</pre><h4>阈值</h4><pre class="json">{{ pretty(shadowThresholds) }}</pre><h4>回滚</h4><pre class="json">{{ pretty(shadowRollbacks) }}</pre><el-button size="small" @click="shadowLoadSim">运行短负载模拟</el-button>
        </el-card>
        <el-card shadow="never"><template #header>Runtime 模块健康</template><pre class="json">{{ pretty(runtimeModules) }}</pre></el-card>
        <el-card shadow="never"><template #header>Runtime 健康历史</template><pre class="json">{{ pretty(healthHistory) }}</pre></el-card>
        <el-card shadow="never"><template #header><div class="card-head"><span>模型错误</span><el-button size="small" type="danger" plain @click="clearModelErrors">清空</el-button></div></template><pre class="json">{{ pretty(modelErrors) }}</pre></el-card>
        <el-card shadow="never">
          <template #header>日志文件</template>
          <el-table :data="logFiles"><el-table-column prop="name" label="文件" min-width="220"/><el-table-column prop="size" label="大小" width="120"/><el-table-column label="操作" width="100"><template #default="{row}"><el-button link type="primary" @click="openLog(row.name)">查看</el-button></template></el-table-column></el-table>
          <el-dialog v-model="logOpen" data-amitia-ui="settings.dialog" :title="logName" width="70%"><pre class="json log-content">{{ logContent }}</pre></el-dialog>
        </el-card>
        <el-card shadow="never"><template #header><div class="card-head"><span>Usage 分析</span><el-button size="small" type="danger" plain @click="clearUsage">清空统计</el-button></div></template><h4>按天</h4><pre class="json">{{ pretty(usageDaily) }}</pre><h4>按模型</h4><pre class="json">{{ pretty(usageModels) }}</pre><h4>按来源</h4><pre class="json">{{ pretty(usageSources) }}</pre></el-card>
      </el-tab-pane>

    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { apiClient } from "@/composables/useApi";

const tab = ref("security");
const loading = ref(false);
const auditActions = ref<string[]>([]);
const auditLogs = ref<any[]>([]);
const auditStats = ref<any>({});
const auditSettings = reactive<any>({ enabled: true, logActions: true, retentionDays: 90 });
const mood = reactive<any>({ enabled: false, threshold: 0.5 });
const runtimeModules = ref<any>({});
const healthHistory = ref<any>({});
const modelErrors = ref<any>({});
const logFiles = ref<any[]>([]);
const usageDaily = ref<any>({});
const usageModels = ref<any>({});
const usageSources = ref<any>({});
const shadowStatus = ref<any>({});
const shadowThresholds = ref<any>({});
const shadowRollbacks = ref<any>({});
const accessConfig = reactive<any>({ requireAuth: true, allowedOrigins: "*", rateLimit: true });
const accessStatus = ref<any>({});
const identityCheck = ref<any>({});
const logOpen = ref(false);
const logName = ref("");
const logContent = ref("");

const pretty = (value: any) => JSON.stringify(value ?? {}, null, 2);
const safe = async <T,>(fn: () => Promise<T>, fallback: T) => { try { return await fn(); } catch (error) {
  console.error("[original-error]", error);
return fallback; } };

async function loadAll() {
  loading.value = true;
  try {
    const [aa, al, aset, ast, md, rm, hh, me, lf, ud, um, us, shs, sht, shr, ac, acs, identity] = await Promise.all([
      safe(() => apiClient.get("/api/audit/actions").then(r => r.data), []),
      safe(() => apiClient.get("/api/audit/logs", { params: { limit: 200 } }).then(r => r.data), []),
      safe(() => apiClient.get("/api/audit/settings").then(r => r.data), {}),
      safe(() => apiClient.get("/api/audit/stats").then(r => r.data), {}),
      safe(() => apiClient.get("/api/config/mood-detection").then(r => r.data), {}),
      safe(() => apiClient.get("/api/runtime/modules/health").then(r => r.data), {}),
      safe(() => apiClient.get("/api/runtime/health-history").then(r => r.data), {}),
      safe(() => apiClient.get("/api/logs/model-errors").then(r => r.data), {}),
      safe(() => apiClient.get("/api/logs/files").then(r => r.data), {}),
      safe(() => apiClient.get("/api/usage/daily").then(r => r.data), { error: "使用统计读取失败" }),
      safe(() => apiClient.get("/api/usage/models").then(r => r.data), { error: "使用统计读取失败" }),
      safe(() => apiClient.get("/api/usage/sources").then(r => r.data), { error: "使用统计读取失败" }),
      safe(() => apiClient.get("/api/shadow/status").then(r => r.data), {}),
      safe(() => apiClient.get("/api/shadow/thresholds").then(r => r.data), {}),
      safe(() => apiClient.get("/api/shadow/rollbacks").then(r => r.data), {}),
      safe(() => apiClient.get("/api/security/access-config").then(r => r.data), {}),
      safe(() => apiClient.get("/api/security/access-status").then(r => r.data), {}),
      safe(() => apiClient.get("/api/security/identity-check").then(r => r.data), {}),
    ]);
    auditActions.value = Array.isArray(aa) ? aa : [];
    auditLogs.value = Array.isArray(al) ? al : [];
    Object.assign(auditSettings, aset);
    auditStats.value = ast;
    Object.assign(mood, md);
    runtimeModules.value = rm; healthHistory.value = hh; modelErrors.value = me;
    logFiles.value = Array.isArray((lf as any)?.files) ? (lf as any).files : Array.isArray(lf) ? lf : [];
    usageDaily.value = ud; usageModels.value = um; usageSources.value = us;
    shadowStatus.value = shs; shadowThresholds.value = sht; shadowRollbacks.value = shr;
    Object.assign(accessConfig, ac); accessStatus.value = acs; identityCheck.value = identity;
  } finally { loading.value = false; }
}

async function saveAuditSettings(){ await apiClient.put("/api/audit/settings", { ...auditSettings }); ElMessage.success("审计设置已保存"); }
async function clearAudit(){ await ElMessageBox.confirm("确定清空系统审计日志？", "确认"); await apiClient.delete("/api/audit/logs"); await loadAll(); }
async function saveMood(){ await apiClient.put("/api/config/mood-detection", { enabled: mood.enabled, threshold: mood.threshold }); ElMessage.success("情绪检测设置已保存"); }
async function clearModelErrors(){ await ElMessageBox.confirm("确定清空模型错误日志？", "确认"); await apiClient.delete("/api/logs/model-errors"); await loadAll(); }
async function clearUsage(){ await ElMessageBox.confirm("重置当前范围的统计起点？历史调用和聊天记录将保留。", "确认"); const scope = (usageDaily.value as any)?.scope;
 if(!scope?.coreId) throw new Error("请先重新加载统计范围");
 await apiClient.delete("/api/usage/clear", { params: { scope: scope.deviceId ? "device" : "core", expectedCoreId: scope.coreId, expectedDeviceId: scope.deviceId || "" } }); await loadAll(); }
async function openLog(name: string){ const r = await apiClient.get(`/api/logs/files/${encodeURIComponent(name)}`); logName.value = name; logContent.value = typeof r.data === "string" ? r.data : pretty(r.data); logOpen.value = true; }
async function saveAccess(){ await apiClient.put("/api/security/access-config", { ...accessConfig }); ElMessage.success("访问安全设置已保存"); await loadAll(); }
async function shadowStart(){ await apiClient.post("/api/shadow/start", { phase: "interaction" }); ElMessage.success("Shadow Mode 已启动"); await loadAll(); }
async function shadowStop(){ await apiClient.post("/api/shadow/stop", {}); ElMessage.success("Shadow Mode 已停止"); await loadAll(); }
async function shadowAdvance(){ await apiClient.post("/api/shadow/phase/advance", {}); await loadAll(); }
async function shadowLoadSim(){ await apiClient.post("/api/shadow/load-sim", { profile: "burst", durationSeconds: 10, burstRate: 50, sustainedRps: 20 }); ElMessage.success("负载模拟完成"); await loadAll(); }

onMounted(loadAll);
</script>

<style scoped>
.advanced-page{display:grid;gap:16px}.head,.card-head{display:flex;justify-content:space-between;align-items:flex-start;gap:12px}.head{justify-content:flex-end}.el-card{margin-bottom:14px}.json{max-height:360px;overflow:auto;background:var(--el-fill-color-light);padding:12px;border-radius:8px;white-space:pre-wrap;word-break:break-word}.log-content{max-height:65vh}.muted{color:var(--el-text-color-secondary);margin-left:10px}
</style>
