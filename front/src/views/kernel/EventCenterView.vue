<template>
  <div class="event-center">
    <div class="event-header">
      <div class="header-left">
        <h2>事件中心</h2>
        <p class="subtitle">Event System — 第三方事件发布、订阅、投递与死信管理</p>
      </div>
      <div class="header-right">
        <el-button @click="refreshAll" :icon="Refresh" :loading="loading">刷新</el-button>
      </div>
    </div>

    <el-tabs v-model="activeTab" class="event-tabs">
      <el-tab-pane label="概览" name="overview">
        <div class="stats-grid">
          <el-card class="stat-card">
            <div class="stat-value">{{ stats.activeSubscriptions }}</div>
            <div class="stat-label">活跃订阅</div>
          </el-card>
          <el-card class="stat-card">
            <div class="stat-value">{{ stats.pendingOutbox }}</div>
            <div class="stat-label">待处理Outbox</div>
          </el-card>
          <el-card class="stat-card">
            <div class="stat-value warn">{{ stats.retryWaitDeliveries }}</div>
            <div class="stat-label">等待重试</div>
          </el-card>
          <el-card class="stat-card">
            <div class="stat-value danger">{{ stats.deadLetterDeliveries }}</div>
            <div class="stat-label">死信投递</div>
          </el-card>
        </div>

        <el-row :gutter="16" class="stats-row">
          <el-col :span="12">
            <el-card>
              <template #header>Outbox状态分布</template>
              <div class="bar-list">
                <div class="bar-item">
                  <span class="bar-label">待处理</span>
                  <el-progress :percentage="pct(stats.pendingOutbox, outboxTotal)" :stroke-width="16" color="#409eff" />
                  <span class="bar-num">{{ stats.pendingOutbox }}</span>
                </div>
                <div class="bar-item">
                  <span class="bar-label">分发中</span>
                  <el-progress :percentage="pct(stats.dispatchingOutbox, outboxTotal)" :stroke-width="16" color="#e6a23c" />
                  <span class="bar-num">{{ stats.dispatchingOutbox }}</span>
                </div>
                <div class="bar-item">
                  <span class="bar-label">已分发</span>
                  <el-progress :percentage="pct(stats.dispatchedOutbox, outboxTotal)" :stroke-width="16" color="#67c23a" />
                  <span class="bar-num">{{ stats.dispatchedOutbox }}</span>
                </div>
                <div class="bar-item">
                  <span class="bar-label">死信</span>
                  <el-progress :percentage="pct(stats.deadLetterOutbox, outboxTotal)" :stroke-width="16" color="#f56c6c" />
                  <span class="bar-num">{{ stats.deadLetterOutbox }}</span>
                </div>
              </div>
            </el-card>
          </el-col>
          <el-col :span="12">
            <el-card>
              <template #header>投递状态分布</template>
              <div class="bar-list">
                <div class="bar-item">
                  <span class="bar-label">成功</span>
                  <el-progress :percentage="pct(stats.succeededDeliveries, deliveryTotal)" :stroke-width="16" color="#67c23a" />
                  <span class="bar-num">{{ stats.succeededDeliveries }}</span>
                </div>
                <div class="bar-item">
                  <span class="bar-label">等待重试</span>
                  <el-progress :percentage="pct(stats.retryWaitDeliveries, deliveryTotal)" :stroke-width="16" color="#e6a23c" />
                  <span class="bar-num">{{ stats.retryWaitDeliveries }}</span>
                </div>
                <div class="bar-item">
                  <span class="bar-label">失败</span>
                  <el-progress :percentage="pct(stats.failedDeliveries, deliveryTotal)" :stroke-width="16" color="#f56c6c" />
                  <span class="bar-num">{{ stats.failedDeliveries }}</span>
                </div>
                <div class="bar-item">
                  <span class="bar-label">死信</span>
                  <el-progress :percentage="pct(stats.deadLetterDeliveries, deliveryTotal)" :stroke-width="16" color="#909399" />
                  <span class="bar-num">{{ stats.deadLetterDeliveries }}</span>
                </div>
              </div>
            </el-card>
          </el-col>
        </el-row>

        <el-card class="circuit-card">
          <template #header>熔断器状态</template>
          <el-table :data="circuitList" border size="small" v-loading="loading">
            <el-table-column prop="subscriptionId" label="订阅ID" min-width="200" show-overflow-tooltip />
            <el-table-column label="状态" width="100">
              <template #default="{ row }">
                <el-tag :type="circuitTagType(row.State)" size="small">{{ row.State }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="ConsecutiveFails" label="连续失败" width="100" />
            <el-table-column prop="TotalFails" label="总失败" width="80" />
            <el-table-column prop="TotalSuccess" label="总成功" width="80" />
            <el-table-column prop="LastFailCode" label="错误码" width="120" show-overflow-tooltip />
            <el-table-column label="最后失败时间" width="180">
              <template #default="{ row }">{{ formatDate(row.LastFailTime) }}</template>
            </el-table-column>
            <el-table-column label="操作" width="100" fixed="right">
              <template #default="{ row }">
                <el-button size="small" @click="doResetCircuit(row.subscriptionId)">重置</el-button>
              </template>
            </el-table-column>
          </el-table>
          <el-empty v-if="!loading && circuitList.length === 0" description="暂无熔断器记录" />
        </el-card>
      </el-tab-pane>

      <el-tab-pane label="事件类型" name="types">
        <div class="tab-toolbar">
          <el-input v-model="typeFilter" placeholder="搜索事件类型ID..." clearable style="width: 300px" :prefix-icon="Search" />
        </div>
        <el-table :data="filteredTypes" border v-loading="loading" size="small">
          <el-table-column prop="EventTypeID" label="事件类型ID" min-width="200" show-overflow-tooltip />
          <el-table-column prop="Version" label="版本" width="80" />
          <el-table-column prop="RiskLevel" label="风险" width="80">
            <template #default="{ row }">
              <el-tag :type="riskTagType(row.RiskLevel)" size="small">{{ row.RiskLevel }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="OrderingPolicy" label="排序" width="120" />
          <el-table-column label="第三方订阅" width="100">
            <template #default="{ row }">
              <el-tag :type="row.SubscriberPolicy?.AllowThirdParty ? 'success' : 'info'" size="small">
                {{ row.SubscriberPolicy?.AllowThirdParty ? '允许' : '禁止' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="最大载荷" width="100">
            <template #default="{ row }">{{ formatBytes(row.MaxPayloadBytes) }}</template>
          </el-table-column>
          <el-table-column prop="DefinitionHash" label="定义哈希" width="120" show-overflow-tooltip />
          <el-table-column label="操作" width="80" fixed="right">
            <template #default="{ row }">
              <el-button size="small" link @click="showTypeDetail(row)">详情</el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!loading && filteredTypes.length === 0" description="暂无事件类型" />
      </el-tab-pane>

      <el-tab-pane label="投递记录" name="deliveries">
        <div class="tab-toolbar">
          <el-select v-model="deliveryFilter.status" placeholder="状态筛选" clearable style="width: 160px">
            <el-option label="待处理" value="pending" />
            <el-option label="已租约" value="leased" />
            <el-option label="投递中" value="delivering" />
            <el-option label="成功" value="succeeded" />
            <el-option label="等待重试" value="retry_wait" />
            <el-option label="失败" value="failed" />
            <el-option label="死信" value="dead_letter" />
            <el-option label="已取消" value="cancelled" />
            <el-option label="已跳过" value="skipped" />
          </el-select>
          <el-input v-model="deliveryFilter.extensionId" placeholder="扩展ID" clearable style="width: 200px" />
          <el-input v-model="deliveryFilter.subscriptionId" placeholder="订阅ID" clearable style="width: 200px" />
          <el-button type="primary" @click="loadDeliveries">查询</el-button>
        </div>
        <el-table :data="deliveries" border v-loading="loading" size="small">
          <el-table-column prop="DeliveryID" label="投递ID" min-width="200" show-overflow-tooltip />
          <el-table-column prop="EventID" label="事件ID" min-width="200" show-overflow-tooltip />
          <el-table-column prop="SubscriptionID" label="订阅ID" min-width="160" show-overflow-tooltip />
          <el-table-column prop="ExtensionID" label="扩展ID" min-width="140" show-overflow-tooltip />
          <el-table-column label="状态" width="100">
            <template #default="{ row }">
              <el-tag :type="deliveryTagType(row.Status)" size="small">{{ row.Status }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="Attempt" label="尝试" width="60" />
          <el-table-column prop="MaxAttempts" label="最大" width="60" />
          <el-table-column label="可用时间" width="180">
            <template #default="{ row }">{{ formatDate(row.AvailableAt) }}</template>
          </el-table-column>
          <el-table-column prop="ErrorCode" label="错误码" width="120" show-overflow-tooltip />
        </el-table>
        <el-empty v-if="!loading && deliveries.length === 0" description="暂无投递记录" />
      </el-tab-pane>

      <el-tab-pane label="死信队列" name="deadLetters">
        <div class="tab-toolbar">
          <el-select v-model="deadLetterFilter.status" placeholder="状态筛选" clearable style="width: 160px">
            <el-option label="待处理" value="pending" />
            <el-option label="已重放" value="replayed" />
            <el-option label="已丢弃" value="discarded" />
          </el-select>
          <el-select v-model="deadLetterFilter.reason" placeholder="原因筛选" clearable style="width: 200px">
            <el-option label="超过最大尝试次数" value="max_attempts_exceeded" />
            <el-option label="永久错误" value="permanent_error" />
            <el-option label="处理器未找到" value="handler_not_found" />
            <el-option label="订阅无效" value="subscription_invalid" />
            <el-option label="权限被撤销" value="permission_revoked" />
            <el-option label="Scope无效" value="scope_invalid" />
            <el-option label="扩展已禁用" value="extension_disabled" />
            <el-option label="熔断开启" value="circuit_open" />
          </el-select>
          <el-button type="primary" @click="loadDeadLetters">查询</el-button>
        </div>
        <el-table :data="deadLetters" border v-loading="loading" size="small">
          <el-table-column prop="DeadLetterID" label="死信ID" min-width="200" show-overflow-tooltip />
          <el-table-column prop="EventID" label="事件ID" min-width="200" show-overflow-tooltip />
          <el-table-column prop="SubscriptionID" label="订阅ID" min-width="140" show-overflow-tooltip />
          <el-table-column prop="ExtensionID" label="扩展ID" min-width="120" show-overflow-tooltip />
          <el-table-column label="原因" width="180">
            <template #default="{ row }">
              <el-tag type="danger" size="small">{{ row.Reason }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="Attempts" label="尝试次数" width="80" />
          <el-table-column label="状态" width="90">
            <template #default="{ row }">
              <el-tag :type="deadLetterTagType(row.Status)" size="small">{{ row.Status }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="ErrorCode" label="错误码" width="120" show-overflow-tooltip />
          <el-table-column label="创建时间" width="180">
            <template #default="{ row }">{{ formatDate(row.CreatedAt) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="160" fixed="right">
            <template #default="{ row }">
              <el-button size="small" type="primary" @click="doReplay(row)" :disabled="row.Status === 'discarded'">重放</el-button>
              <el-button size="small" type="danger" @click="doDiscard(row)" :disabled="row.Status === 'discarded'">丢弃</el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!loading && deadLetters.length === 0" description="暂无死信记录" />
      </el-tab-pane>

      <el-tab-pane label="Outbox" name="outbox">
        <div class="tab-toolbar">
          <el-select v-model="outboxFilter.status" placeholder="状态筛选" clearable style="width: 160px">
            <el-option label="待处理" value="pending" />
            <el-option label="分发中" value="dispatching" />
            <el-option label="已分发" value="dispatched" />
            <el-option label="失败" value="failed" />
            <el-option label="死信" value="dead_letter" />
            <el-option label="已取消" value="cancelled" />
          </el-select>
          <el-input v-model="outboxFilter.extensionId" placeholder="扩展ID" clearable style="width: 200px" />
          <el-button type="primary" @click="loadOutbox">查询</el-button>
        </div>
        <el-table :data="outboxRecords" border v-loading="loading" size="small">
          <el-table-column prop="OutboxID" label="Outbox ID" min-width="200" show-overflow-tooltip />
          <el-table-column prop="EventID" label="事件ID" min-width="200" show-overflow-tooltip />
          <el-table-column prop="EventTypeID" label="事件类型" min-width="160" show-overflow-tooltip />
          <el-table-column prop="ProducerID" label="生产者" min-width="120" show-overflow-tooltip />
          <el-table-column label="状态" width="100">
            <template #default="{ row }">
              <el-tag :type="outboxTagType(row.Status)" size="small">{{ row.Status }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="Depth" label="深度" width="60" />
          <el-table-column label="发生时间" width="180">
            <template #default="{ row }">{{ formatDate(row.OccurredAt) }}</template>
          </el-table-column>
          <el-table-column prop="ErrorCode" label="错误码" width="120" show-overflow-tooltip />
        </el-table>
        <el-empty v-if="!loading && outboxRecords.length === 0" description="暂无Outbox记录" />
      </el-tab-pane>

      <el-tab-pane label="审计日志" name="audit">
        <div class="tab-toolbar">
          <el-input v-model="auditFilter.eventId" placeholder="事件ID" clearable style="width: 200px" />
          <el-input v-model="auditFilter.deliveryId" placeholder="投递ID" clearable style="width: 200px" />
          <el-input v-model="auditFilter.extensionId" placeholder="扩展ID" clearable style="width: 200px" />
          <el-input v-model="auditFilter.action" placeholder="操作" clearable style="width: 160px" />
          <el-button type="primary" @click="loadAudit">查询</el-button>
        </div>
        <el-table :data="auditEntries" border v-loading="loading" size="small">
          <el-table-column prop="Action" label="操作" width="160" />
          <el-table-column prop="Actor" label="操作者" width="120" />
          <el-table-column prop="ExtensionID" label="扩展ID" min-width="140" show-overflow-tooltip />
          <el-table-column prop="EventID" label="事件ID" min-width="200" show-overflow-tooltip />
          <el-table-column prop="DeliveryID" label="投递ID" min-width="200" show-overflow-tooltip />
          <el-table-column label="成功" width="80">
            <template #default="{ row }">
              <el-tag :type="row.Success ? 'success' : 'danger'" size="small">{{ row.Success ? '是' : '否' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="ErrorCode" label="错误码" width="120" show-overflow-tooltip />
          <el-table-column label="时间" width="180">
            <template #default="{ row }">{{ formatDate(row.Timestamp) }}</template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!loading && auditEntries.length === 0" description="暂无审计日志" />
      </el-tab-pane>
    </el-tabs>

    <el-dialog v-model="typeDetailVisible" title="事件类型详情" width="720px">
      <el-descriptions :column="2" border v-if="typeDetail">
        <el-descriptions-item label="事件类型ID">{{ typeDetail.EventTypeID }}</el-descriptions-item>
        <el-descriptions-item label="版本">{{ typeDetail.Version }}</el-descriptions-item>
        <el-descriptions-item label="描述" :span="2">{{ typeDetail.Description }}</el-descriptions-item>
        <el-descriptions-item label="风险等级">
          <el-tag :type="riskTagType(typeDetail.RiskLevel)" size="small">{{ typeDetail.RiskLevel }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="排序策略">{{ typeDetail.OrderingPolicy }}</el-descriptions-item>
        <el-descriptions-item label="最大载荷">{{ formatBytes(typeDetail.MaxPayloadBytes) }}</el-descriptions-item>
        <el-descriptions-item label="最大元数据">{{ formatBytes(typeDetail.MaxMetadataBytes) }}</el-descriptions-item>
        <el-descriptions-item label="允许第三方订阅">
          <el-tag :type="typeDetail.SubscriberPolicy?.AllowThirdParty ? 'success' : 'info'" size="small">
            {{ typeDetail.SubscriberPolicy?.AllowThirdParty ? '允许' : '禁止' }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="需要系统信任">
          <el-tag :type="typeDetail.ProducerPolicy?.RequireSystemTrust ? 'warning' : 'info'" size="small">
            {{ typeDetail.ProducerPolicy?.RequireSystemTrust ? '是' : '否' }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="投递超时">{{ typeDetail.DeliveryPolicy?.Timeout }}ms</el-descriptions-item>
        <el-descriptions-item label="最大尝试">{{ typeDetail.DeliveryPolicy?.MaxAttempts }}</el-descriptions-item>
        <el-descriptions-item label="最大并发">{{ typeDetail.DeliveryPolicy?.MaxInFlight }}</el-descriptions-item>
        <el-descriptions-item label="保留期限">{{ typeDetail.RetentionPolicy?.MaxAge }}s</el-descriptions-item>
        <el-descriptions-item label="定义哈希" :span="2">{{ typeDetail.DefinitionHash }}</el-descriptions-item>
      </el-descriptions>

      <div v-if="typeDetail?.SensitiveFields?.length" class="detail-section">
        <h4>敏感字段规则</h4>
        <el-table :data="typeDetail.SensitiveFields" size="small" border>
          <el-table-column prop="Path" label="路径" min-width="160" />
          <el-table-column prop="Classification" label="分类" width="120" />
          <el-table-column prop="DefaultAction" label="默认操作" width="120" />
        </el-table>
      </div>

      <div v-if="typeDetail?.ProjectionRules?.length" class="detail-section">
        <h4>投影规则</h4>
        <el-table :data="typeDetail.ProjectionRules" size="small" border>
          <el-table-column prop="SourcePath" label="源路径" min-width="160" />
          <el-table-column prop="TargetPath" label="目标路径" min-width="160" />
          <el-table-column prop="RequiredPermission" label="所需权限" width="160" />
        </el-table>
      </div>
    </el-dialog>

    <el-dialog v-model="replayVisible" title="重放死信" width="480px">
      <el-form label-width="100px">
        <el-form-item label="重放策略">
          <el-select v-model="replayForm.strategy" style="width: 100%">
            <el-option label="同订阅重放" value="replay_same_subscription" />
            <el-option label="修复后重放" value="replay_after_repair" />
            <el-option label="重放到新Generation" value="replay_to_new_generation" />
            <el-option label="丢弃" value="discard" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="replayForm.strategy === 'replay_to_new_generation'" label="新订阅ID">
          <el-input v-model="replayForm.newSubscriptionId" placeholder="输入新订阅ID" />
        </el-form-item>
        <el-form-item label="操作者">
          <el-input v-model="replayForm.requestedBy" placeholder="操作者标识" />
        </el-form-item>
        <el-form-item label="原因">
          <el-input v-model="replayForm.reason" type="textarea" placeholder="重放原因" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="replayVisible = false">取消</el-button>
        <el-button type="primary" @click="confirmReplay">确认重放</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { Refresh, Search } from "@element-plus/icons-vue";
import {
  getEventStats,
  listEventTypes,
  listDeliveries,
  listDeadLetters,
  listOutbox,
  listAudit,
  replayDeadLetter,
  discardDeadLetter,
  resetCircuit,
  type ServiceStats,
  type EventTypeDefinition,
  type Delivery,
  type DeadLetterRecord,
  type OutboxRecord,
  type EventAuditEntry,
  type DeliveryStatus,
  type DeadLetterReason,
  type DeadLetterStatus,
  type OutboxStatus,
  type ReplayStrategy,
} from "./event-api";

const loading = ref(false);
const activeTab = ref("overview");

const stats = ref<ServiceStats>({
  pendingOutbox: 0,
  dispatchingOutbox: 0,
  dispatchedOutbox: 0,
  deadLetterOutbox: 0,
  pendingDeliveries: 0,
  leasedDeliveries: 0,
  succeededDeliveries: 0,
  failedDeliveries: 0,
  retryWaitDeliveries: 0,
  deadLetterDeliveries: 0,
  cancelledDeliveries: 0,
  skippedDeliveries: 0,
  activeSubscriptions: 0,
  circuits: {},
});

const eventTypes = ref<EventTypeDefinition[]>([]);
const deliveries = ref<Delivery[]>([]);
const deadLetters = ref<DeadLetterRecord[]>([]);
const outboxRecords = ref<OutboxRecord[]>([]);
const auditEntries = ref<EventAuditEntry[]>([]);

const typeFilter = ref("");
const deliveryFilter = ref<{ status: DeliveryStatus | ""; extensionId: string; subscriptionId: string }>({
  status: "",
  extensionId: "",
  subscriptionId: "",
});
const deadLetterFilter = ref<{ status: DeadLetterStatus | ""; reason: DeadLetterReason | "" }>({
  status: "",
  reason: "",
});
const outboxFilter = ref<{ status: OutboxStatus | ""; extensionId: string }>({
  status: "",
  extensionId: "",
});
const auditFilter = ref<{ eventId: string; deliveryId: string; extensionId: string; action: string }>({
  eventId: "",
  deliveryId: "",
  extensionId: "",
  action: "",
});

const typeDetailVisible = ref(false);
const typeDetail = ref<EventTypeDefinition | null>(null);

const replayVisible = ref(false);
const replayTarget = ref<DeadLetterRecord | null>(null);
const replayForm = ref<{ strategy: ReplayStrategy; newSubscriptionId: string; requestedBy: string; reason: string }>({
  strategy: "replay_same_subscription",
  newSubscriptionId: "",
  requestedBy: "",
  reason: "",
});

let refreshTimer: ReturnType<typeof setInterval> | null = null;

const outboxTotal = computed(() => stats.value.pendingOutbox + stats.value.dispatchingOutbox + stats.value.dispatchedOutbox + stats.value.deadLetterOutbox);
const deliveryTotal = computed(() => stats.value.succeededDeliveries + stats.value.retryWaitDeliveries + stats.value.failedDeliveries + stats.value.deadLetterDeliveries + stats.value.cancelledDeliveries + stats.value.skippedDeliveries);

const circuitList = computed(() => {
  const list: Array<{ subscriptionId: string; State: string; ConsecutiveFails: number; TotalFails: number; TotalSuccess: number; LastFailCode: string; LastFailTime: string }> = [];
  for (const [id, c] of Object.entries(stats.value.circuits || {})) {
    list.push({ subscriptionId: id, ...c });
  }
  return list;
});

const filteredTypes = computed(() => {
  if (!typeFilter.value) return eventTypes.value;
  const q = typeFilter.value.toLowerCase();
  return eventTypes.value.filter((t) => t.EventTypeID.toLowerCase().includes(q));
});

function pct(val: number, total: number): number {
  if (total === 0) return 0;
  return Math.round((val / total) * 100);
}

function circuitTagType(state: string): "success" | "warning" | "danger" {
  if (state === "open") return "danger";
  if (state === "half_open") return "warning";
  return "success";
}

function riskTagType(risk: string): "success" | "warning" | "danger" {
  if (risk === "critical" || risk === "high") return "danger";
  if (risk === "medium") return "warning";
  return "success";
}

function deliveryTagType(status: string): "success" | "warning" | "danger" | "info" {
  if (status === "succeeded") return "success";
  if (status === "retry_wait" || status === "leased" || status === "delivering") return "warning";
  if (status === "failed" || status === "dead_letter") return "danger";
  return "info";
}

function deadLetterTagType(status: string): "success" | "warning" | "danger" | "info" {
  if (status === "replayed") return "success";
  if (status === "pending") return "warning";
  if (status === "discarded") return "danger";
  return "info";
}

function outboxTagType(status: string): "success" | "warning" | "danger" | "info" {
  if (status === "dispatched") return "success";
  if (status === "pending" || status === "dispatching") return "warning";
  if (status === "failed" || status === "dead_letter") return "danger";
  return "info";
}

function formatDate(s: string): string {
  if (!s) return "";
  try {
    return new Date(s).toLocaleString("zh-CN");
  } catch {
    return s;
  }
}

function formatBytes(bytes: number): string {
  if (!bytes) return "0";
  if (bytes < 1024) return `${bytes}B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)}KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)}MB`;
}

async function loadStats() {
  try {
    stats.value = await getEventStats();
  } catch (e: any) {
    console.error("加载统计失败:", e);
  }
}

async function loadEventTypes() {
  try {
    const res = await listEventTypes();
    eventTypes.value = res.items || [];
  } catch (e: any) {
    console.error("加载事件类型失败:", e);
  }
}

async function loadDeliveries() {
  try {
    const filter: Record<string, unknown> = {};
    if (deliveryFilter.value.status) filter.status = deliveryFilter.value.status;
    if (deliveryFilter.value.extensionId) filter.extensionId = deliveryFilter.value.extensionId;
    if (deliveryFilter.value.subscriptionId) filter.subscriptionId = deliveryFilter.value.subscriptionId;
    const res = await listDeliveries(filter);
    deliveries.value = res.items || [];
  } catch (e: any) {
    ElMessage.error("加载投递记录失败: " + (e?.message || e));
  }
}

async function loadDeadLetters() {
  try {
    const filter: Record<string, unknown> = {};
    if (deadLetterFilter.value.status) filter.status = deadLetterFilter.value.status;
    if (deadLetterFilter.value.reason) filter.reason = deadLetterFilter.value.reason;
    const res = await listDeadLetters(filter);
    deadLetters.value = res.items || [];
  } catch (e: any) {
    ElMessage.error("加载死信记录失败: " + (e?.message || e));
  }
}

async function loadOutbox() {
  try {
    const filter: Record<string, unknown> = {};
    if (outboxFilter.value.status) filter.status = outboxFilter.value.status;
    if (outboxFilter.value.extensionId) filter.extensionId = outboxFilter.value.extensionId;
    const res = await listOutbox(filter);
    outboxRecords.value = res.items || [];
  } catch (e: any) {
    ElMessage.error("加载Outbox记录失败: " + (e?.message || e));
  }
}

async function loadAudit() {
  try {
    const filter: Record<string, unknown> = {};
    if (auditFilter.value.eventId) filter.eventId = auditFilter.value.eventId;
    if (auditFilter.value.deliveryId) filter.deliveryId = auditFilter.value.deliveryId;
    if (auditFilter.value.extensionId) filter.extensionId = auditFilter.value.extensionId;
    if (auditFilter.value.action) filter.action = auditFilter.value.action;
    const res = await listAudit(filter);
    auditEntries.value = res.items || [];
  } catch (e: any) {
    ElMessage.error("加载审计日志失败: " + (e?.message || e));
  }
}

async function refreshAll() {
  loading.value = true;
  try {
    await loadStats();
    if (activeTab.value === "types") await loadEventTypes();
    if (activeTab.value === "deliveries") await loadDeliveries();
    if (activeTab.value === "deadLetters") await loadDeadLetters();
    if (activeTab.value === "outbox") await loadOutbox();
    if (activeTab.value === "audit") await loadAudit();
  } finally {
    loading.value = false;
  }
}

function showTypeDetail(row: EventTypeDefinition) {
  typeDetail.value = row;
  typeDetailVisible.value = true;
}

async function doResetCircuit(subscriptionId: string) {
  try {
    await resetCircuit(subscriptionId);
    ElMessage.success("熔断器已重置");
    await loadStats();
  } catch (e: any) {
    ElMessage.error("重置失败: " + (e?.message || e));
  }
}

function doReplay(row: DeadLetterRecord) {
  replayTarget.value = row;
  replayForm.value = {
    strategy: "replay_same_subscription",
    newSubscriptionId: "",
    requestedBy: "",
    reason: "",
  };
  replayVisible.value = true;
}

async function confirmReplay() {
  if (!replayTarget.value) return;
  try {
    await replayDeadLetter(replayTarget.value.DeadLetterID, replayForm.value);
    ElMessage.success("死信已重放");
    replayVisible.value = false;
    await loadDeadLetters();
  } catch (e: any) {
    ElMessage.error("重放失败: " + (e?.message || e));
  }
}

async function doDiscard(row: DeadLetterRecord) {
  try {
    await ElMessageBox.confirm(`确定丢弃死信 ${row.DeadLetterID} 吗？此操作不可逆。`, "丢弃确认", { type: "warning" });
    await discardDeadLetter(row.DeadLetterID);
    ElMessage.success("死信已丢弃");
    await loadDeadLetters();
  } catch (e: any) {
    if (e !== "cancel" && e?.message !== "cancel") {
      ElMessage.error("丢弃失败: " + (e?.message || e));
    }
  }
}

onMounted(() => {
  refreshAll();
  refreshTimer = setInterval(loadStats, 10000);
});

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer);
});
</script>

<style scoped>
.event-center {
  padding: 24px;
  max-width: 1200px;
  margin: 0 auto;
}

.event-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.event-header h2 {
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

.event-tabs {
  margin-top: 8px;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
  margin-bottom: 20px;
}

.stat-card {
  text-align: center;
}

.stat-value {
  font-size: 32px;
  font-weight: 700;
  color: var(--el-color-primary);
}

.stat-value.warn {
  color: var(--el-color-warning);
}

.stat-value.danger {
  color: var(--el-color-danger);
}

.stat-label {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  margin-top: 4px;
}

.stats-row {
  margin-bottom: 20px;
}

.bar-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.bar-item {
  display: flex;
  align-items: center;
  gap: 12px;
}

.bar-label {
  width: 80px;
  font-size: 13px;
  text-align: right;
  flex-shrink: 0;
}

.bar-item .el-progress {
  flex: 1;
}

.bar-num {
  width: 40px;
  font-size: 13px;
  font-weight: 600;
  flex-shrink: 0;
}

.circuit-card {
  margin-top: 20px;
}

.tab-toolbar {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
  flex-wrap: wrap;
  align-items: center;
}

.detail-section {
  margin-top: 16px;
}

.detail-section h4 {
  margin: 0 0 8px 0;
  font-size: 14px;
}
</style>
