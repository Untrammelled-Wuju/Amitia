<template>
  <div class="runtime-debug-page">
    <div class="page-header">
      <div>
        <h2 class="page-title">运行时协作调试</h2>
        <p class="page-subtitle">
          查看交互状态、预算、队列、投递、工具结果与一致性巡检快照
        </p>
      </div>
      <div class="page-actions">
        <el-tag
          :type="snapshot.meta.degraded ? 'warning' : 'success'"
          effect="plain"
        >
          {{ snapshot.meta.degraded ? "降级中" : "运行正常" }}
        </el-tag>
        <el-button :loading="loading" type="primary" @click="loadSnapshot"
          >刷新</el-button
        >
      </div>
    </div>

    <el-alert
      v-if="loadError"
      type="warning"
      :closable="false"
      show-icon
      :title="loadError"
      class="top-alert"
    />

    <section class="summary-grid">
      <el-card shadow="hover">
        <div class="metric-label">快照时间</div>
        <div class="metric-value">
          {{ formatDateTime(snapshot.meta.generatedAt) }}
        </div>
        <div class="metric-note">最近一次运行时状态汇总</div>
      </el-card>
      <el-card shadow="hover">
        <div class="metric-label">活跃交互</div>
        <div class="metric-value">
          {{ snapshot.summary.activeInteractions }}
        </div>
        <div class="metric-note">正在执行或等待提交的交互</div>
      </el-card>
      <el-card shadow="hover">
        <div class="metric-label">排队任务</div>
        <div class="metric-value">{{ snapshot.summary.queuedTasks }}</div>
        <div class="metric-note">包含用户输入与后台任务</div>
      </el-card>
      <el-card shadow="hover">
        <div class="metric-label">一致性异常</div>
        <div class="metric-value">
          {{ snapshot.summary.reconciliationIssues }}
        </div>
        <div class="metric-note">高优先级漂移与孤儿任务</div>
      </el-card>
    </section>

    <div class="content-grid">
      <el-card shadow="hover" class="panel-card">
        <template #header>交互状态</template>
        <el-table
          :data="snapshot.interactions"
          size="small"
          stripe
          empty-text="暂无交互快照"
        >
          <el-table-column
            prop="scope"
            label="Scope"
            min-width="160"
            show-overflow-tooltip
          />
          <el-table-column prop="status" label="状态" width="110">
            <template #default="{ row }">
              <el-tag :type="interactionStatusType(row.status)" size="small">{{
                row.status
              }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="priority" label="优先级" width="80" />
          <el-table-column prop="path" label="路径" width="90" />
          <el-table-column prop="stateVersion" label="版本" width="90" />
          <el-table-column prop="deadlineAt" label="截止时间" min-width="170">
            <template #default="{ row }">{{
              formatDateTime(row.deadlineAt)
            }}</template>
          </el-table-column>
          <el-table-column
            prop="cancelReason"
            label="取消原因"
            min-width="160"
            show-overflow-tooltip
          />
        </el-table>
      </el-card>

      <el-card shadow="hover" class="panel-card">
        <template #header>预算与队列</template>
        <div class="split-panel">
          <div>
            <div class="section-title">预算消耗</div>
            <el-table
              :data="snapshot.budgets"
              size="small"
              stripe
              empty-text="暂无预算数据"
            >
              <el-table-column
                prop="scope"
                label="Scope"
                min-width="130"
                show-overflow-tooltip
              />
              <el-table-column prop="path" label="路径" width="88" />
              <el-table-column label="调用" width="130">
                <template #default="{ row }"
                  >{{ row.callsUsed }}/{{ row.callsLimit }}</template
                >
              </el-table-column>
              <el-table-column label="Token" width="130">
                <template #default="{ row }"
                  >{{ row.tokensUsed }}/{{ row.tokensLimit }}</template
                >
              </el-table-column>
              <el-table-column prop="queueMs" label="排队ms" width="100" />
            </el-table>
          </div>
          <div>
            <div class="section-title">队列深度</div>
            <el-table
              :data="snapshot.queues"
              size="small"
              stripe
              empty-text="暂无队列数据"
            >
              <el-table-column prop="name" label="队列" min-width="120" />
              <el-table-column prop="priority" label="级别" width="80" />
              <el-table-column prop="depth" label="深度" width="80" />
              <el-table-column prop="oldestAgeMs" label="最老ms" width="100" />
              <el-table-column prop="status" label="状态" width="90">
                <template #default="{ row }">
                  <el-tag :type="queueStatusType(row.status)" size="small">{{
                    row.status
                  }}</el-tag>
                </template>
              </el-table-column>
            </el-table>
          </div>
        </div>
      </el-card>

      <el-card shadow="hover" class="panel-card">
        <template #header>行为与表达计划</template>
        <template #default>
          <el-empty
            v-if="!snapshot.behaviorPlan && !snapshot.expressionPlan"
            description="暂无行为与表达计划数据"
            :image-size="40"
          />
          <div v-else class="behavior-plan-grid">
            <div v-if="snapshot.behaviorPlan" class="plan-block">
              <div class="section-title">BehaviorPlan</div>
              <el-descriptions :column="2" border size="small">
                <el-descriptions-item label="意图" :span="2">{{
                  snapshot.behaviorPlan.intention
                }}</el-descriptions-item>
                <el-descriptions-item label="策略" :span="2">{{
                  snapshot.behaviorPlan.strategy
                }}</el-descriptions-item>
                <el-descriptions-item label="必须包含">{{
                  snapshot.behaviorPlan.mustInclude.join(", ") || "—"
                }}</el-descriptions-item>
                <el-descriptions-item label="可以包含">{{
                  snapshot.behaviorPlan.mayInclude.join(", ") || "—"
                }}</el-descriptions-item>
                <el-descriptions-item label="禁止">{{
                  snapshot.behaviorPlan.mustAvoid.join(", ") || "—"
                }}</el-descriptions-item>
                <el-descriptions-item label="提问策略">{{
                  snapshot.behaviorPlan.questionPolicy
                }}</el-descriptions-item>
                <el-descriptions-item label="建议策略">{{
                  snapshot.behaviorPlan.advicePolicy
                }}</el-descriptions-item>
                <el-descriptions-item label="投递策略">{{
                  snapshot.behaviorPlan.deliveryPolicy
                }}</el-descriptions-item>
                <el-descriptions-item label="状态版本">{{
                  snapshot.behaviorPlan.stateVersion
                }}</el-descriptions-item>
                <el-descriptions-item label="选中候选">{{
                  snapshot.behaviorPlan.winnerCandidate
                }}</el-descriptions-item>
              </el-descriptions>
            </div>
            <div v-if="snapshot.expressionPlan" class="plan-block">
              <div class="section-title">ExpressionPlan</div>
              <el-descriptions :column="2" border size="small">
                <el-descriptions-item label="句数">{{
                  snapshot.expressionPlan.sentenceCount
                }}</el-descriptions-item>
                <el-descriptions-item label="最大长度">{{
                  snapshot.expressionPlan.maxLength
                }}</el-descriptions-item>
                <el-descriptions-item label="直接度">{{
                  snapshot.expressionPlan.directness
                }}</el-descriptions-item>
                <el-descriptions-item label="温暖度">{{
                  snapshot.expressionPlan.warmth
                }}</el-descriptions-item>
                <el-descriptions-item label="情绪显露">{{
                  snapshot.expressionPlan.emotionDisplay
                }}</el-descriptions-item>
                <el-descriptions-item label="使用提问">{{
                  snapshot.expressionPlan.useQuestion ? "是" : "否"
                }}</el-descriptions-item>
                <el-descriptions-item label="语音参数">{{
                  snapshot.expressionPlan.voiceParams || "—"
                }}</el-descriptions-item>
                <el-descriptions-item label="回避话题">{{
                  snapshot.expressionPlan.avoidTopics.join(", ") || "—"
                }}</el-descriptions-item>
              </el-descriptions>
              <div
                v-if="snapshot.behaviorPlan?.rejectedCandidates?.length"
                class="rejected-section"
                style="margin-top: 12px"
              >
                <div class="section-title">被拒绝候选</div>
                <el-table
                  :data="
                    snapshot.behaviorPlan.rejectedCandidates.map((r, i) => ({
                      idx: i + 1,
                      candidate: r,
                    }))
                  "
                  size="small"
                  stripe
                >
                  <el-table-column label="#" width="50" prop="idx" />
                  <el-table-column label="候选" prop="candidate" />
                </el-table>
              </div>
            </div>
          </div>
        </template>
      </el-card>

      <el-card shadow="hover" class="panel-card">
        <template #header>投递与工具结果</template>
        <div class="split-panel">
          <div>
            <div class="section-title">投递状态</div>
            <el-table
              :data="snapshot.deliveries"
              size="small"
              stripe
              empty-text="暂无投递数据"
            >
              <el-table-column prop="channel" label="渠道" width="100" />
              <el-table-column prop="leaseState" label="租约" width="90">
                <template #default="{ row }">
                  <el-tag
                    :type="deliveryLeaseType(row.leaseState)"
                    size="small"
                    >{{ row.leaseState }}</el-tag
                  >
                </template>
              </el-table-column>
              <el-table-column prop="deliveryState" label="投递" width="100">
                <template #default="{ row }">
                  <el-tag
                    :type="deliveryStateType(row.deliveryState)"
                    size="small"
                    >{{ row.deliveryState }}</el-tag
                  >
                </template>
              </el-table-column>
              <el-table-column prop="attempt" label="重试" width="70" />
              <el-table-column
                prop="updatedAt"
                label="更新时间"
                min-width="170"
              >
                <template #default="{ row }">{{
                  formatDateTime(row.updatedAt)
                }}</template>
              </el-table-column>
            </el-table>
          </div>
          <div>
            <div class="section-title">工具执行</div>
            <el-table
              :data="snapshot.tools"
              size="small"
              stripe
              empty-text="暂无工具结果"
            >
              <el-table-column prop="tool" label="工具" min-width="120" />
              <el-table-column prop="status" label="状态" width="100">
                <template #default="{ row }">
                  <el-tag :type="toolStatusType(row.status)" size="small">{{
                    row.status
                  }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column
                prop="idempotencyKey"
                label="幂等键"
                min-width="150"
                show-overflow-tooltip
              />
              <el-table-column
                prop="updatedAt"
                label="更新时间"
                min-width="170"
              >
                <template #default="{ row }">{{
                  formatDateTime(row.updatedAt)
                }}</template>
              </el-table-column>
            </el-table>
          </div>
        </div>
      </el-card>

      <el-card shadow="hover" class="panel-card">
        <template #header>熔断与一致性巡检</template>
        <div class="split-panel">
          <div>
            <div class="section-title">依赖熔断</div>
            <el-table
              :data="snapshot.circuits"
              size="small"
              stripe
              empty-text="暂无熔断状态"
            >
              <el-table-column prop="dependency" label="依赖" min-width="120" />
              <el-table-column prop="state" label="状态" width="110">
                <template #default="{ row }">
                  <el-tag :type="circuitStateType(row.state)" size="small">{{
                    row.state
                  }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="failures" label="失败数" width="90" />
              <el-table-column prop="openedAt" label="打开时间" min-width="170">
                <template #default="{ row }">{{
                  formatDateTime(row.openedAt)
                }}</template>
              </el-table-column>
            </el-table>
          </div>
          <div>
            <div class="section-title">巡检结果</div>
            <el-table
              :data="snapshot.reconciliation"
              size="small"
              stripe
              empty-text="暂无巡检结果"
            >
              <el-table-column prop="category" label="类别" min-width="140" />
              <el-table-column prop="severity" label="级别" width="90">
                <template #default="{ row }">
                  <el-tag
                    :type="reconciliationSeverityType(row.severity)"
                    size="small"
                    >{{ row.severity }}</el-tag
                  >
                </template>
              </el-table-column>
              <el-table-column prop="count" label="数量" width="70" />
              <el-table-column
                prop="strategy"
                label="收敛策略"
                min-width="150"
                show-overflow-tooltip
              />
              <el-table-column
                prop="updatedAt"
                label="更新时间"
                min-width="170"
              >
                <template #default="{ row }">{{
                  formatDateTime(row.updatedAt)
                }}</template>
              </el-table-column>
            </el-table>
          </div>
        </div>
      </el-card>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import type { RuntimeDebugSnapshot, RuntimeMetrics } from "@/types";
import { fetchRuntimeDebugSnapshotApi } from "./api";

const loading = ref(false);
const loadError = ref("");
const snapshot = reactive<RuntimeDebugSnapshot>({
  meta: { generatedAt: "", degraded: false },
  summary: { activeInteractions: 0, queuedTasks: 0, reconciliationIssues: 0 },
  interactions: [],
  budgets: [],
  queues: [],
  deliveries: [],
  tools: [],
  circuits: [],
  reconciliation: [],
  traces: [],
  metrics: undefined,
  behaviorPlan: undefined,
  expressionPlan: undefined,
});

async function loadSnapshot() {
  loading.value = true;
  loadError.value = "";
  try {
    const data = await fetchRuntimeDebugSnapshotApi();
    Object.assign(snapshot, {
      meta: data.meta,
      summary: data.summary,
      interactions: data.interactions,
      budgets: data.budgets,
      queues: data.queues,
      deliveries: data.deliveries,
      tools: data.tools,
      circuits: data.circuits,
      reconciliation: data.reconciliation,
      traces: data.traces,
      metrics: data.metrics,
      behaviorPlan: data.behaviorPlan,
      expressionPlan: data.expressionPlan,
    });
  } catch (error: any) {
    loadError.value = error?.message || "运行时调试接口暂不可用";
  } finally {
    loading.value = false;
  }
}

function formatDateTime(value?: string | null) {
  if (!value) {
    return "—";
  }
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return value;
  }
  return parsed.toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  });
}

function interactionStatusType(status: string) {
  if (status === "COMMITTED" || status === "DELIVERED") return "success";
  if (status === "SUPERSEDED" || status === "CANCELLED") return "danger";
  if (status === "RUNNING" || status === "DECIDED") return "warning";
  return "info";
}

function queueStatusType(status: string) {
  if (status === "HEALTHY") return "success";
  if (status === "BACKPRESSURE" || status === "THROTTLED") return "warning";
  if (status === "DROPPING" || status === "STARVED") return "danger";
  return "info";
}

function deliveryLeaseType(state: string) {
  if (state === "HELD") return "warning";
  if (state === "RELEASED") return "success";
  if (state === "EXPIRED") return "danger";
  return "info";
}

function deliveryStateType(state: string) {
  if (state === "DELIVERED") return "success";
  if (state === "UNKNOWN" || state === "PENDING") return "warning";
  if (state === "FAILED" || state === "CANCELLED") return "danger";
  return "info";
}

function toolStatusType(status: string) {
  if (status === "SUCCESS") return "success";
  if (status === "UNKNOWN" || status === "PARTIAL") return "warning";
  if (status === "FAILED" || status === "CANCELLED") return "danger";
  return "info";
}

function circuitStateType(state: string) {
  if (state === "CLOSED") return "success";
  if (state === "HALF_OPEN") return "warning";
  if (state === "OPEN") return "danger";
  return "info";
}

function traceStatusType(status: string) {
  if (status === "completed" || status === "success") return "success";
  if (status === "running" || status === "pending") return "warning";
  if (status === "failed" || status === "cancelled") return "danger";
  return "info";
}

function reconciliationSeverityType(severity: string) {
  if (severity === "high") return "danger";
  if (severity === "medium") return "warning";
  if (severity === "low") return "info";
  return "success";
}

onMounted(loadSnapshot);
</script>

<style scoped>
.runtime-debug-page {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  flex-wrap: wrap;
}

.page-title {
  margin: 0;
  font-size: 20px;
  line-height: 28px;
  color: var(--ac-color-text);
}

.page-subtitle {
  margin: 6px 0 0;
  color: var(--ac-color-text-secondary);
  font-size: 13px;
  line-height: 20px;
}

.page-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.top-alert {
  margin-bottom: 4px;
}

.summary-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.metric-label {
  color: var(--ac-color-text-muted);
  font-size: 12px;
  line-height: 18px;
}

.metric-value {
  margin-top: 8px;
  color: var(--ac-color-text);
  font-size: 22px;
  line-height: 30px;
  font-weight: 600;
}

.metric-note {
  margin-top: 6px;
  color: var(--ac-color-text-secondary);
  font-size: 12px;
  line-height: 18px;
}

.content-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 16px;
}

.panel-card :deep(.el-card__header) {
  font-weight: 600;
}

.split-panel {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.section-title {
  margin-bottom: 10px;
  color: var(--ac-color-text);
  font-size: 14px;
  line-height: 20px;
  font-weight: 500;
}

@media (max-width: 1120px) {
  .summary-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .split-panel {
    grid-template-columns: minmax(0, 1fr);
  }
}

@media (max-width: 720px) {
  .runtime-debug-page {
    padding: 16px;
  }

  .summary-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .page-actions {
    width: 100%;
    justify-content: space-between;
  }
}
</style>
