<template>
  <div class="schedule-center">
    <div class="schedule-header">
      <div class="header-left">
        <h2>调度中心</h2>
        <p class="subtitle">第三方Schedule系统 — 定时触发、误火处理、熔断保护</p>
      </div>
      <div class="header-right">
        <el-button @click="refreshAll" :icon="Refresh" :loading="loading">刷新</el-button>
      </div>
    </div>

    <div class="schedule-toolbar">
      <el-input
        v-model="extensionFilter"
        placeholder="按扩展ID过滤..."
        clearable
        style="width: 280px"
        :prefix-icon="Search"
        @keyup.enter="loadSchedules"
        @clear="loadSchedules"
      />
      <el-button type="primary" @click="loadSchedules">查询</el-button>
    </div>

    <el-table :data="schedules" border v-loading="loading" style="width: 100%">
      <el-table-column label="调度名称" min-width="160" show-overflow-tooltip>
        <template #default="{ row }">{{ row.definition.name }}</template>
      </el-table-column>
      <el-table-column label="扩展ID" min-width="160" show-overflow-tooltip>
        <template #default="{ row }">{{ row.definition.extensionId }}</template>
      </el-table-column>
      <el-table-column label="触发类型" width="110">
        <template #default="{ row }">
          <el-tag size="small">{{ row.definition.trigger.type }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="目标类型" width="110">
        <template #default="{ row }">
          <el-tag size="small" type="info">{{ row.definition.target.type }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="110">
        <template #default="{ row }">
          <el-tag :type="statusTagType(row.state.status)" size="small">{{ row.state.status }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="下次执行时间" width="180">
        <template #default="{ row }">{{ formatDate(row.state.nextScheduledAt) }}</template>
      </el-table-column>
      <el-table-column label="Generation" width="100" prop="state.generation" />
      <el-table-column label="操作" width="340" fixed="right">
        <template #default="{ row }">
          <el-button
            v-if="row.state.status !== 'enabled'"
            size="small"
            type="success"
            @click="doEnable(row)"
          >启用</el-button>
          <el-button
            v-else
            size="small"
            type="warning"
            @click="doDisable(row)"
          >禁用</el-button>
          <el-button
            v-if="row.state.status === 'enabled' && !row.state.paused"
            size="small"
            type="warning"
            @click="doPause(row)"
          >暂停</el-button>
          <el-button
            v-if="row.state.paused"
            size="small"
            type="success"
            @click="doResume(row)"
          >恢复</el-button>
          <el-button size="small" type="primary" @click="doRunNow(row)">立即执行</el-button>
          <el-button size="small" @click="doSkipNext(row)">跳过下次</el-button>
          <el-button size="small" link @click="showDetail(row)">详情</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="detailVisible" title="调度详情" width="900px" top="5vh">
      <el-tabs v-if="detail" v-model="detailTab">
        <el-tab-pane label="定义与状态" name="overview">
          <el-descriptions title="基本信息" :column="2" border>
            <el-descriptions-item label="调度名称">{{ detail.definition.name }}</el-descriptions-item>
            <el-descriptions-item label="调度ID">{{ detail.definition.scheduleId }}</el-descriptions-item>
            <el-descriptions-item label="贡献ID">{{ detail.definition.contributionId }}</el-descriptions-item>
            <el-descriptions-item label="扩展ID">{{ detail.definition.extensionId }}</el-descriptions-item>
            <el-descriptions-item label="模块ID">{{ detail.definition.moduleId }}</el-descriptions-item>
            <el-descriptions-item label="版本">{{ detail.definition.version }}</el-descriptions-item>
            <el-descriptions-item label="描述" :span="2">{{ detail.definition.description }}</el-descriptions-item>
            <el-descriptions-item label="时区">{{ detail.definition.timezone }}</el-descriptions-item>
            <el-descriptions-item label="默认启用">{{ detail.definition.enabledByDefault ? '是' : '否' }}</el-descriptions-item>
            <el-descriptions-item label="开始时间">{{ formatDate(detail.definition.startAt) }}</el-descriptions-item>
            <el-descriptions-item label="结束时间">{{ formatDate(detail.definition.endAt) }}</el-descriptions-item>
            <el-descriptions-item label="定义哈希" :span="2">{{ detail.definition.definitionHash }}</el-descriptions-item>
          </el-descriptions>

          <el-descriptions title="触发配置" :column="1" border class="detail-section">
            <el-descriptions-item label="触发类型">{{ detail.definition.trigger.type }}</el-descriptions-item>
            <el-descriptions-item v-if="detail.definition.trigger.cron" label="Cron表达式">
              {{ detail.definition.trigger.cron.expression }} (seconds={{ detail.definition.trigger.cron.seconds }})
            </el-descriptions-item>
            <el-descriptions-item v-if="detail.definition.trigger.interval" label="间隔">
              {{ detail.definition.trigger.interval.interval }}ms / anchor={{ detail.definition.trigger.interval.anchorAt }}
            </el-descriptions-item>
            <el-descriptions-item v-if="detail.definition.trigger.oneShot" label="执行时间">
              {{ formatDate(detail.definition.trigger.oneShot.runAt) }}
            </el-descriptions-item>
          </el-descriptions>

          <el-descriptions title="目标配置" :column="2" border class="detail-section">
            <el-descriptions-item label="目标类型">{{ detail.definition.target.type }}</el-descriptions-item>
            <el-descriptions-item label="目标ID">{{ detail.definition.target.targetId }}</el-descriptions-item>
            <el-descriptions-item label="幂等模式">{{ detail.definition.target.idempotencyMode }}</el-descriptions-item>
            <el-descriptions-item label="输入模板" :span="2">
              <pre class="json-preview">{{ JSON.stringify(detail.definition.target.inputTemplate, null, 2) }}</pre>
            </el-descriptions-item>
          </el-descriptions>

          <el-descriptions title="策略配置" :column="2" border class="detail-section">
            <el-descriptions-item label="误火策略">{{ detail.definition.misfirePolicy.policy }}</el-descriptions-item>
            <el-descriptions-item label="最大追赶">{{ detail.definition.misfirePolicy.maxCatchUp }}</el-descriptions-item>
            <el-descriptions-item label="重叠策略">{{ detail.definition.overlapPolicy.policy }}</el-descriptions-item>
            <el-descriptions-item label="DST春季策略">{{ detail.definition.dstSpringPolicy || '默认' }}</el-descriptions-item>
            <el-descriptions-item label="DST秋季策略">{{ detail.definition.dstFallPolicy || '默认' }}</el-descriptions-item>
            <el-descriptions-item label="抖动启用">{{ detail.definition.jitterPolicy.enabled ? '是' : '否' }}</el-descriptions-item>
            <el-descriptions-item label="抖动最大延迟">{{ detail.definition.jitterPolicy.maxDelay }}ms</el-descriptions-item>
            <el-descriptions-item label="抖动种子模式">{{ detail.definition.jitterPolicy.seedMode }}</el-descriptions-item>
          </el-descriptions>

          <el-descriptions title="重试策略" :column="2" border class="detail-section">
            <el-descriptions-item label="最大尝试次数">{{ detail.definition.retryPolicy.maxAttempts }}</el-descriptions-item>
            <el-descriptions-item label="初始退避">{{ detail.definition.retryPolicy.initialBackoff }}ms</el-descriptions-item>
            <el-descriptions-item label="最大退避">{{ detail.definition.retryPolicy.maxBackoff }}ms</el-descriptions-item>
            <el-descriptions-item label="退避乘数">{{ detail.definition.retryPolicy.multiplier }}</el-descriptions-item>
            <el-descriptions-item label="抖动因子">{{ detail.definition.retryPolicy.jitter }}</el-descriptions-item>
            <el-descriptions-item label="可重试错误码" :span="2">
              {{ detail.definition.retryPolicy.retryableErrorCodes?.join(', ') || '无' }}
            </el-descriptions-item>
          </el-descriptions>

          <el-descriptions title="并发策略" :column="3" border class="detail-section">
            <el-descriptions-item label="最大并发运行">{{ detail.definition.concurrencyPolicy.maxConcurrentRuns }}</el-descriptions-item>
            <el-descriptions-item label="每扩展限制">{{ detail.definition.concurrencyPolicy.perExtensionLimit }}</el-descriptions-item>
            <el-descriptions-item label="每目标限制">{{ detail.definition.concurrencyPolicy.perTargetLimit }}</el-descriptions-item>
          </el-descriptions>

          <el-descriptions title="Scope规则" :column="2" border class="detail-section">
            <el-descriptions-item label="Scope类型">{{ detail.definition.scopeRule.scopeType }}</el-descriptions-item>
            <el-descriptions-item label="Scope IDs">{{ detail.definition.scopeRule.scopeIds?.join(', ') || '无' }}</el-descriptions-item>
            <el-descriptions-item label="命名空间" :span="2">{{ detail.definition.scopeRule.namespaces?.join(', ') || '无' }}</el-descriptions-item>
          </el-descriptions>

          <div v-if="detail.definition.permissionRequirements?.length" class="detail-section">
            <h4>权限要求</h4>
            <el-table :data="detail.definition.permissionRequirements" size="small" border>
              <el-table-column prop="permission" label="权限" min-width="160" />
              <el-table-column label="必需" width="80">
                <template #default="{ row }">
                  <el-tag :type="row.required ? 'danger' : 'info'" size="small">{{ row.required ? '是' : '否' }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="reason" label="原因" show-overflow-tooltip />
            </el-table>
          </div>

          <div v-if="detail.definition.dependencyRequirements?.length" class="detail-section">
            <h4>依赖要求</h4>
            <el-table :data="detail.definition.dependencyRequirements" size="small" border>
              <el-table-column prop="type" label="类型" width="120" />
              <el-table-column prop="id" label="ID" min-width="160" />
              <el-table-column label="可选" width="80">
                <template #default="{ row }">
                  <el-tag :type="row.optional ? 'info' : 'warning'" size="small">{{ row.optional ? '是' : '否' }}</el-tag>
                </template>
              </el-table-column>
            </el-table>
          </div>

          <el-descriptions title="运行状态" :column="2" border class="detail-section">
            <el-descriptions-item label="状态">
              <el-tag :type="statusTagType(detail.state.status)" size="small">{{ detail.state.status }}</el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="已启用">{{ detail.state.enabled ? '是' : '否' }}</el-descriptions-item>
            <el-descriptions-item label="已暂停">{{ detail.state.paused ? '是' : '否' }}</el-descriptions-item>
            <el-descriptions-item label="Generation">{{ detail.state.generation }}</el-descriptions-item>
            <el-descriptions-item label="失败次数">{{ detail.state.failureCount }}</el-descriptions-item>
            <el-descriptions-item label="最后结果">{{ detail.state.lastResult || '无' }}</el-descriptions-item>
            <el-descriptions-item label="上次调度时间">{{ formatDate(detail.state.lastScheduledAt) }}</el-descriptions-item>
            <el-descriptions-item label="上次触发时间">{{ formatDate(detail.state.lastTriggeredAt) }}</el-descriptions-item>
            <el-descriptions-item label="上次完成时间">{{ formatDate(detail.state.lastFinishedAt) }}</el-descriptions-item>
            <el-descriptions-item label="下次调度时间">{{ formatDate(detail.state.nextScheduledAt) }}</el-descriptions-item>
            <el-descriptions-item label="下次生效时间">{{ formatDate(detail.state.nextEffectiveAt) }}</el-descriptions-item>
            <el-descriptions-item label="更新时间">{{ formatDate(detail.state.updatedAt) }}</el-descriptions-item>
          </el-descriptions>
        </el-tab-pane>

        <el-tab-pane label="触发记录" name="triggers">
          <div class="tab-toolbar">
            <el-button size="small" @click="loadTriggers(detail.definition.scheduleId)">刷新</el-button>
          </div>
          <el-table :data="triggers" border size="small" v-loading="subLoading">
            <el-table-column prop="triggerId" label="触发ID" min-width="180" show-overflow-tooltip />
            <el-table-column label="状态" width="110">
              <template #default="{ row }">
                <el-tag :type="runStatusTagType(row.status)" size="small">{{ row.status }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="手动" width="60">
              <template #default="{ row }">{{ row.manual ? '是' : '否' }}</template>
            </el-table-column>
            <el-table-column prop="attempt" label="尝试" width="60" />
            <el-table-column prop="generation" label="Gen" width="60" />
            <el-table-column label="调度时间" width="180">
              <template #default="{ row }">{{ formatDate(row.scheduledAt) }}</template>
            </el-table-column>
            <el-table-column label="生效时间" width="180">
              <template #default="{ row }">{{ formatDate(row.effectiveAt) }}</template>
            </el-table-column>
            <el-table-column label="触发时间" width="180">
              <template #default="{ row }">{{ formatDate(row.triggeredAt) }}</template>
            </el-table-column>
            <el-table-column prop="leaseOwner" label="租约持有" width="120" show-overflow-tooltip />
            <el-table-column prop="errorCode" label="错误码" width="120" show-overflow-tooltip />
            <el-table-column prop="misfireDecision" label="误火决策" width="120" show-overflow-tooltip />
            <el-table-column prop="overlapDecision" label="重叠决策" width="120" show-overflow-tooltip />
          </el-table>
          <el-empty v-if="!subLoading && triggers.length === 0" description="暂无触发记录" />
        </el-tab-pane>

        <el-tab-pane label="运行记录" name="runs">
          <div class="tab-toolbar">
            <el-button size="small" @click="loadRuns(detail.definition.scheduleId)">刷新</el-button>
          </div>
          <el-table :data="runs" border size="small" v-loading="subLoading">
            <el-table-column prop="runId" label="运行ID" min-width="180" show-overflow-tooltip />
            <el-table-column prop="triggerId" label="触发ID" min-width="180" show-overflow-tooltip />
            <el-table-column label="状态" width="110">
              <template #default="{ row }">
                <el-tag :type="runStatusTagType(row.status)" size="small">{{ row.status }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="attempt" label="尝试" width="60" />
            <el-table-column prop="targetType" label="目标类型" width="100" />
            <el-table-column prop="targetId" label="目标ID" min-width="140" show-overflow-tooltip />
            <el-table-column prop="operationId" label="操作ID" min-width="140" show-overflow-tooltip />
            <el-table-column label="开始时间" width="180">
              <template #default="{ row }">{{ formatDate(row.startedAt) }}</template>
            </el-table-column>
            <el-table-column label="完成时间" width="180">
              <template #default="{ row }">{{ formatDate(row.finishedAt) }}</template>
            </el-table-column>
            <el-table-column prop="errorCode" label="错误码" width="120" show-overflow-tooltip />
            <el-table-column prop="errorMessage" label="错误信息" min-width="160" show-overflow-tooltip />
          </el-table>
          <el-empty v-if="!subLoading && runs.length === 0" description="暂无运行记录" />
        </el-tab-pane>

        <el-tab-pane label="误火记录" name="misfires">
          <div class="tab-toolbar">
            <el-button size="small" @click="loadMisfires(detail.definition.scheduleId)">刷新</el-button>
          </div>
          <el-table :data="misfires" border size="small" v-loading="subLoading">
            <el-table-column prop="misfireId" label="误火ID" min-width="180" show-overflow-tooltip />
            <el-table-column prop="policy" label="策略" width="140" />
            <el-table-column prop="action" label="动作" width="140" />
            <el-table-column prop="skippedCount" label="跳过数" width="80" />
            <el-table-column label="调度时间" width="180">
              <template #default="{ row }">{{ formatDate(row.scheduledAt) }}</template>
            </el-table-column>
            <el-table-column label="检测时间" width="180">
              <template #default="{ row }">{{ formatDate(row.detectedAt) }}</template>
            </el-table-column>
            <el-table-column prop="detail" label="详情" min-width="200" show-overflow-tooltip />
          </el-table>
          <el-empty v-if="!subLoading && misfires.length === 0" description="暂无误火记录" />
        </el-tab-pane>

        <el-tab-pane label="熔断器" name="circuit">
          <div class="tab-toolbar">
            <el-button size="small" @click="loadCircuit(detail.definition.scheduleId)">刷新</el-button>
            <el-button size="small" type="warning" @click="doResetCircuit(detail.definition.scheduleId)">重置熔断器</el-button>
          </div>
          <el-descriptions :column="2" border v-if="circuit" v-loading="subLoading">
            <el-descriptions-item label="状态">
              <el-tag :type="circuitTagType(circuit.state)" size="small">{{ circuit.state }}</el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="连续失败">{{ circuit.consecutiveFails }}</el-descriptions-item>
            <el-descriptions-item label="总失败">{{ circuit.totalFails }}</el-descriptions-item>
            <el-descriptions-item label="总成功">{{ circuit.totalSuccess }}</el-descriptions-item>
            <el-descriptions-item label="最后错误码">{{ circuit.lastFailCode || '无' }}</el-descriptions-item>
            <el-descriptions-item label="最后失败时间">{{ formatDate(circuit.lastFailTime) }}</el-descriptions-item>
            <el-descriptions-item label="开启时间">{{ formatDate(circuit.openedAt) }}</el-descriptions-item>
            <el-descriptions-item label="更新时间">{{ formatDate(circuit.updatedAt) }}</el-descriptions-item>
          </el-descriptions>
          <el-empty v-else description="暂无熔断器记录" />
        </el-tab-pane>
      </el-tabs>
    </el-dialog>

    <el-card class="quarantine-card">
      <template #header>
        <div class="card-header">
          <span>隔离记录</span>
          <el-button size="small" @click="loadQuarantines" :icon="Refresh">刷新</el-button>
        </div>
      </template>
      <el-table :data="quarantines" border size="small" v-loading="quarantineLoading">
        <el-table-column prop="quarantineId" label="隔离ID" min-width="180" show-overflow-tooltip />
        <el-table-column prop="scheduleId" label="调度ID" min-width="160" show-overflow-tooltip />
        <el-table-column prop="reason" label="原因" width="160" />
        <el-table-column prop="detail" label="详情" min-width="200" show-overflow-tooltip />
        <el-table-column label="隔离时间" width="180">
          <template #default="{ row }">{{ formatDate(row.quarantinedAt) }}</template>
        </el-table-column>
        <el-table-column label="释放时间" width="180">
          <template #default="{ row }">{{ formatDate(row.releasedAt) }}</template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!quarantineLoading && quarantines.length === 0" description="暂无隔离记录" />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { Refresh, Search } from "@element-plus/icons-vue";
import {
  listSchedules,
  getSchedule,
  enableSchedule,
  disableSchedule,
  pauseSchedule,
  resumeSchedule,
  runScheduleNow,
  skipNextRun,
  getTriggers,
  getRuns,
  getMisfires,
  getCircuit,
  resetCircuit,
  getQuarantines,
} from "./schedule-api";
import type {
  ScheduleDetail,
  ScheduleTriggerRecord,
  ScheduleRunRecord,
  ScheduleMisfireRecord,
  ScheduleCircuitRecord,
  ScheduleQuarantineRecord,
  ScheduleDefinitionStatus,
  ScheduleRunStatus,
  CircuitState,
} from "./schedule-types";

const loading = ref(false);
const subLoading = ref(false);
const quarantineLoading = ref(false);

const extensionFilter = ref("");
const schedules = ref<ScheduleDetail[]>([]);

const detailVisible = ref(false);
const detail = ref<ScheduleDetail | null>(null);
const detailTab = ref("overview");

const triggers = ref<ScheduleTriggerRecord[]>([]);
const runs = ref<ScheduleRunRecord[]>([]);
const misfires = ref<ScheduleMisfireRecord[]>([]);
const circuit = ref<ScheduleCircuitRecord | null>(null);
const quarantines = ref<ScheduleQuarantineRecord[]>([]);

function statusTagType(status: ScheduleDefinitionStatus): "success" | "info" | "warning" | "danger" {
  if (status === "enabled") return "success";
  if (status === "disabled" || status === "created") return "info";
  if (status === "paused") return "warning";
  return "danger";
}

function runStatusTagType(status: ScheduleRunStatus): "success" | "info" | "warning" | "danger" {
  if (status === "completed") return "success";
  if (status === "waiting") return "info";
  if (status === "due" || status === "leased" || status === "triggering" || status === "running" || status === "retry_wait") return "warning";
  return "danger";
}

function circuitTagType(state: CircuitState): "success" | "warning" | "danger" {
  if (state === "open") return "danger";
  if (state === "half_open") return "warning";
  return "success";
}

function formatDate(s?: string): string {
  if (!s) return "";
  try {
    return new Date(s).toLocaleString("zh-CN");
  } catch {
    return s;
  }
}

async function loadSchedules() {
  loading.value = true;
  try {
    const data = await listSchedules(extensionFilter.value || undefined);
    schedules.value = data.items || [];
  } catch (e: any) {
    ElMessage.error("加载调度列表失败: " + (e?.message || e));
  } finally {
    loading.value = false;
  }
}

async function refreshAll() {
  await loadSchedules();
  await loadQuarantines();
}

async function showDetail(row: ScheduleDetail) {
  detailTab.value = "overview";
  triggers.value = [];
  runs.value = [];
  misfires.value = [];
  circuit.value = null;
  try {
    detail.value = await getSchedule(row.definition.scheduleId);
    detailVisible.value = true;
  } catch (e: any) {
    ElMessage.error("加载详情失败: " + (e?.message || e));
  }
}

async function loadTriggers(scheduleId: string) {
  subLoading.value = true;
  try {
    const data = await getTriggers(scheduleId, 50);
    triggers.value = data.items || [];
  } catch (e: any) {
    ElMessage.error("加载触发记录失败: " + (e?.message || e));
  } finally {
    subLoading.value = false;
  }
}

async function loadRuns(scheduleId: string) {
  subLoading.value = true;
  try {
    const data = await getRuns(scheduleId, 50);
    runs.value = data.items || [];
  } catch (e: any) {
    ElMessage.error("加载运行记录失败: " + (e?.message || e));
  } finally {
    subLoading.value = false;
  }
}

async function loadMisfires(scheduleId: string) {
  subLoading.value = true;
  try {
    const data = await getMisfires(scheduleId, 50);
    misfires.value = data.items || [];
  } catch (e: any) {
    ElMessage.error("加载误火记录失败: " + (e?.message || e));
  } finally {
    subLoading.value = false;
  }
}

async function loadCircuit(scheduleId: string) {
  subLoading.value = true;
  try {
    circuit.value = await getCircuit(scheduleId);
  } catch (e: any) {
    circuit.value = null;
  } finally {
    subLoading.value = false;
  }
}

async function loadQuarantines() {
  quarantineLoading.value = true;
  try {
    const data = await getQuarantines();
    quarantines.value = data.items || [];
  } catch (e: any) {
    ElMessage.error("加载隔离记录失败: " + (e?.message || e));
  } finally {
    quarantineLoading.value = false;
  }
}

async function doEnable(row: ScheduleDetail) {
  try {
    await enableSchedule(row.definition.scheduleId);
    ElMessage.success("已启用: " + row.definition.name);
    await loadSchedules();
  } catch (e: any) {
    ElMessage.error("启用失败: " + (e?.message || e));
  }
}

async function doDisable(row: ScheduleDetail) {
  try {
    await disableSchedule(row.definition.scheduleId);
    ElMessage.success("已禁用: " + row.definition.name);
    await loadSchedules();
  } catch (e: any) {
    ElMessage.error("禁用失败: " + (e?.message || e));
  }
}

async function doPause(row: ScheduleDetail) {
  try {
    await pauseSchedule(row.definition.scheduleId);
    ElMessage.success("已暂停: " + row.definition.name);
    await loadSchedules();
  } catch (e: any) {
    ElMessage.error("暂停失败: " + (e?.message || e));
  }
}

async function doResume(row: ScheduleDetail) {
  try {
    await resumeSchedule(row.definition.scheduleId);
    ElMessage.success("已恢复: " + row.definition.name);
    await loadSchedules();
  } catch (e: any) {
    ElMessage.error("恢复失败: " + (e?.message || e));
  }
}

async function doRunNow(row: ScheduleDetail) {
  try {
    await ElMessageBox.confirm(`确定立即执行调度 "${row.definition.name}" 吗？`, "执行确认", { type: "warning" });
    const result = await runScheduleNow(row.definition.scheduleId);
    ElMessage.success("已触发执行: " + (result.triggerId || ""));
  } catch (e: any) {
    if (e !== "cancel" && e?.message !== "cancel") {
      ElMessage.error("执行失败: " + (e?.message || e));
    }
  }
}

async function doSkipNext(row: ScheduleDetail) {
  try {
    await ElMessageBox.confirm(`确定跳过调度 "${row.definition.name}" 的下次执行吗？`, "跳过确认", { type: "warning" });
    await skipNextRun(row.definition.scheduleId);
    ElMessage.success("已跳过下次执行");
    await loadSchedules();
  } catch (e: any) {
    if (e !== "cancel" && e?.message !== "cancel") {
      ElMessage.error("跳过失败: " + (e?.message || e));
    }
  }
}

async function doResetCircuit(scheduleId: string) {
  try {
    await resetCircuit(scheduleId);
    ElMessage.success("熔断器已重置");
    await loadCircuit(scheduleId);
  } catch (e: any) {
    ElMessage.error("重置失败: " + (e?.message || e));
  }
}

onMounted(async () => {
  await loadSchedules();
  await loadQuarantines();
});
</script>

<style scoped>
.schedule-center {
  padding: 24px;
  max-width: 1200px;
  margin: 0 auto;
}

.schedule-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.schedule-header h2 {
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

.schedule-toolbar {
  display: flex;
  gap: 12px;
  margin-bottom: 20px;
  align-items: center;
}

.detail-section {
  margin-top: 16px;
}

.detail-section h4 {
  margin: 0 0 8px 0;
  font-size: 14px;
}

.json-preview {
  margin: 0;
  max-height: 200px;
  overflow: auto;
  font-size: 12px;
  background: var(--el-fill-color-light);
  padding: 8px;
  border-radius: 4px;
}

.tab-toolbar {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
  align-items: center;
}

.quarantine-card {
  margin-top: 24px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
</style>
