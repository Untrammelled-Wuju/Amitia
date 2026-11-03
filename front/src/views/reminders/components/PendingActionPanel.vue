<template>
  <section class="attention-panel">
    <div class="attention-heading">
      <div>
        <h2>现在需要你处理</h2>
        <p>审批、等待、失败恢复和可查看结果集中在这里。</p>
      </div>
      <el-button :icon="Refresh" :loading="loading" @click="refresh">刷新</el-button>
    </div>
    <el-alert v-if="error" :title="error" type="error" :closable="false" show-icon />
    <div v-loading="loading" class="attention-list">
      <article v-for="item in items" :key="item.id" class="attention-item" :class="`priority-${item.priority}`">
        <div class="attention-item-main">
          <div class="attention-item-title">
            <el-tag :type="tagType(item)" effect="light" round>{{ kindLabel(item.kind) }}</el-tag>
            <strong>{{ item.title }}</strong>
            <span>{{ formatTime(item.createdAt) }}</span>
          </div>
          <p v-if="item.summary">{{ item.summary }}</p>
          <small>{{ item.reason }}</small>
        </div>
        <div class="attention-actions">
          <template v-if="item.kind === 'agent_approval'">
            <el-button size="small" @click="resolveAgentApproval(item, false)">拒绝</el-button>
            <el-button size="small" type="primary" @click="resolveAgentApproval(item, true)">本次允许</el-button>
          </template>
          <template v-else-if="item.kind === 'task_approval'">
            <el-button size="small" @click="resolveTaskApproval(item, false)">拒绝</el-button>
            <el-button size="small" type="primary" @click="resolveTaskApproval(item, true)">批准本次执行</el-button>
          </template>
          <el-button size="small" :disabled="!hasTimeline(item)" @click="openTimeline(item)">决策时间线</el-button>
          <el-button size="small" :disabled="!item.route" @click="item.route && openRoute(item.route)">打开</el-button>
        </div>
      </article>
      <el-empty v-if="!loading && !items.length" description="现在没有需要你处理的事项" />
    </div>

    <el-collapse v-model="expandedSettings" class="attention-settings">
      <el-collapse-item title="注意力预算" name="budget">
        <div class="budget-summary">
          <span>今日打断 <strong>{{ budget.stats.todayInterruptions }}</strong></span>
          <span>已抑制 <strong>{{ budget.stats.todaySuppressed }}</strong></span>
          <span>影子观察 <strong>{{ budget.stats.todayObserved }}</strong></span>
        </div>
        <el-form label-position="top" class="settings-form">
          <div class="settings-grid">
            <el-form-item label="启用预算"><el-switch v-model="budget.policy.enabled" /></el-form-item>
            <el-form-item label="影子模式">
              <el-switch v-model="budget.policy.shadowMode" />
              <span class="hint">开启时只记录本应抑制的提醒，不改变实际发送。</span>
            </el-form-item>
            <el-form-item label="每日打断上限">
              <el-input-number v-model="budget.policy.maxInterruptionsPerDay" :min="1" :max="1000" />
            </el-form-item>
            <el-form-item label="重复确认上限">
              <el-input-number v-model="budget.policy.maxRepeatedConfirmations" :min="1" :max="100" />
            </el-form-item>
            <el-form-item label="连续追问上限">
              <el-input-number v-model="budget.policy.maxConsecutiveFollowups" :min="1" :max="100" />
            </el-form-item>
            <el-form-item label="无变化通知上限">
              <el-input-number v-model="budget.policy.maxUnchangedNotifications" :min="1" :max="100" />
            </el-form-item>
            <el-form-item label="去重窗口（分钟）">
              <el-input-number v-model="budget.policy.dedupeWindowMinutes" :min="1" :max="1440" />
            </el-form-item>
          </div>
          <el-button type="primary" :loading="budgetSaving" @click="saveBudget">保存预算</el-button>
        </el-form>
      </el-collapse-item>
      <el-collapse-item title="来源授权策略" name="policies">
        <div class="policy-toolbar">
          <span>按空间、渠道、账号、会话和发送者逐级覆盖。</span>
          <el-button size="small" @click="addPolicy">新增策略</el-button>
        </div>
        <el-table :data="policies" size="small" empty-text="尚未添加更具体策略">
          <el-table-column prop="scopeType" label="范围" width="100" />
          <el-table-column prop="channelId" label="渠道" min-width="120" />
          <el-table-column prop="conversationId" label="会话" min-width="160" />
          <el-table-column prop="senderId" label="发送者" min-width="120" />
          <el-table-column label="明确请求" width="100"><template #default="{ row }">{{ modeLabel(row.explicitMode) }}</template></el-table-column>
          <el-table-column label="推断请求" width="100"><template #default="{ row }">{{ modeLabel(row.inferredMode) }}</template></el-table-column>
          <el-table-column label="权限上限" width="120"><template #default="{ row }">{{ permissionLabel(row.permissionCeiling) }}</template></el-table-column>
          <el-table-column label="操作" width="110" align="right">
            <template #default="{ row }">
              <el-button link type="primary" @click="editPolicy(row)">编辑</el-button>
              <el-button link type="danger" @click="removePolicy(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-collapse-item>
    </el-collapse>

    <el-drawer v-model="timelineVisible" title="决策时间线" size="min(620px, 100vw)">
      <el-timeline v-if="timeline.length">
        <el-timeline-item v-for="event in timeline" :key="event.id" :timestamp="formatTime(event.occurredAt)" :type="timelineType(event.status)">
          <strong>{{ event.title }}</strong>
          <p v-if="event.summary" class="timeline-summary">{{ event.summary }}</p>
          <small>{{ event.source }} · {{ event.status || "记录" }}</small>
          <pre v-if="timelineDetails(event.details)" class="timeline-details">{{ timelineDetails(event.details) }}</pre>
        </el-timeline-item>
      </el-timeline>
      <el-empty v-else description="没有可展示的决策记录" />
    </el-drawer>

    <el-dialog v-model="policyDialog" :title="policyForm.id ? '编辑来源策略' : '新增来源策略'" width="min(620px, 94vw)">
      <el-form label-position="top">
        <div class="settings-grid">
          <el-form-item label="范围">
            <el-select v-model="policyForm.scopeType">
              <el-option label="空间" value="space" />
              <el-option label="渠道" value="channel" />
              <el-option label="账号" value="account" />
              <el-option label="会话" value="conversation" />
              <el-option label="发送者" value="sender" />
            </el-select>
          </el-form-item>
          <el-form-item label="渠道"><el-input v-model="policyForm.channelId" /></el-form-item>
          <el-form-item label="账号"><el-input v-model="policyForm.accountId" /></el-form-item>
          <el-form-item label="会话"><el-input v-model="policyForm.conversationId" /></el-form-item>
          <el-form-item label="发送者"><el-input v-model="policyForm.senderId" /></el-form-item>
          <el-form-item label="触发方式">
            <el-select v-model="policyForm.triggerMode">
              <el-option label="私聊或明确提及" value="direct_or_mention" />
              <el-option label="仅私聊" value="direct" />
              <el-option label="仅明确提及" value="mention" />
              <el-option label="全部消息" value="all" />
              <el-option label="仅推断请求" value="inferred_only" />
            </el-select>
          </el-form-item>
          <el-form-item label="明确请求">
            <el-select v-model="policyForm.explicitMode"><el-option label="允许执行" value="allow" /><el-option label="仅记录" value="observe" /><el-option label="拒绝" value="deny" /></el-select>
          </el-form-item>
          <el-form-item label="推断请求">
            <el-select v-model="policyForm.inferredMode"><el-option label="允许执行" value="allow" /><el-option label="仅记录" value="observe" /><el-option label="拒绝" value="deny" /></el-select>
          </el-form-item>
          <el-form-item label="权限上限">
            <el-select v-model="policyForm.permissionCeiling"><el-option label="请求批准" value="request_approval" /><el-option label="完全权限" value="full_access" /></el-select>
          </el-form-item>
          <el-form-item label="启用"><el-switch v-model="policyForm.enabled" /></el-form-item>
        </div>
      </el-form>
      <template #footer><el-button @click="policyDialog = false">取消</el-button><el-button type="primary" :loading="policySaving" @click="savePolicy">保存</el-button></template>
    </el-dialog>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import { Refresh } from "@element-plus/icons-vue";
import { apiClient } from "@/composables/useApi";
import { ownedChatRuntimeRequest } from "@/runtime/owned-chat-management";
import { decideSourceTaskApproval } from "@/views/kernel/tasks/api";

interface AttentionItem {
  id: string;
  kind: string;
  decision: string;
  priority: number;
  requiresAction: boolean;
  title: string;
  summary: string;
  reason: string;
  roleId?: string;
  conversationId?: string;
  threadId?: string;
  requestId?: string;
  coordinationId?: string;
  approvalId?: string;
  turnId?: string;
  workflowId?: string;
  workflowRunId?: string;
  route?: string;
  createdAt: string;
  payload?: Record<string, any>;
}

interface TimelineEvent {
  id: string;
  type: string;
  title: string;
  summary?: string;
  status?: string;
  source: string;
  occurredAt: string;
  details?: Record<string, unknown>;
}

interface BudgetPolicy {
  spaceId: string;
  enabled: boolean;
  shadowMode: boolean;
  maxInterruptionsPerDay: number;
  maxRepeatedConfirmations: number;
  maxConsecutiveFollowups: number;
  maxUnchangedNotifications: number;
  dedupeWindowMinutes: number;
  revision: number;
}

interface BudgetStats {
  todayInterruptions: number;
  todaySuppressed: number;
  todayObserved: number;
  repeatedConfirmations: number;
  consecutiveFollowups: number;
  unchangedNotifications: number;
}

interface SourcePolicy {
  id: string;
  spaceId: string;
  scopeType: string;
  channelId: string;
  accountId: string;
  conversationId: string;
  senderId: string;
  explicitMode: string;
  inferredMode: string;
  triggerMode: string;
  permissionCeiling: string;
  enabled: boolean;
  revision: number;
}

const router = useRouter();
const loading = ref(false);
const error = ref("");
const items = ref<AttentionItem[]>([]);
const timeline = ref<TimelineEvent[]>([]);
const timelineVisible = ref(false);
const expandedSettings = ref<string[]>([]);
const policies = ref<SourcePolicy[]>([]);
const policyDialog = ref(false);
const policySaving = ref(false);
const budgetSaving = ref(false);
const budget = reactive({
  policy: defaultBudgetPolicy(),
  stats: { todayInterruptions: 0, todaySuppressed: 0, todayObserved: 0, repeatedConfirmations: 0, consecutiveFollowups: 0, unchangedNotifications: 0 } as BudgetStats,
});
const policyForm = reactive(defaultPolicy());

function defaultBudgetPolicy(): BudgetPolicy {
  return { spaceId: "", enabled: true, shadowMode: true, maxInterruptionsPerDay: 24, maxRepeatedConfirmations: 3, maxConsecutiveFollowups: 2, maxUnchangedNotifications: 1, dedupeWindowMinutes: 30, revision: 0 };
}

function defaultPolicy(): SourcePolicy {
  return { id: "", spaceId: "", scopeType: "channel", channelId: "", accountId: "", conversationId: "", senderId: "", explicitMode: "allow", inferredMode: "observe", triggerMode: "direct_or_mention", permissionCeiling: "request_approval", enabled: true, revision: 0 };
}

async function refresh() {
  loading.value = true;
  error.value = "";
  try {
    const [nextItems, nextBudget, nextPolicies] = await Promise.all([
      apiClient.get("/api/attention/items"),
      apiClient.get("/api/attention/budget"),
      apiClient.get("/api/agent-access/policies"),
    ]);
    items.value = Array.isArray(nextItems.data) ? nextItems.data : [];
    budget.policy = { ...defaultBudgetPolicy(), ...(nextBudget.data?.policy || {}) };
    budget.stats = { ...budget.stats, ...(nextBudget.data || {}) };
    policies.value = Array.isArray(nextPolicies.data) ? nextPolicies.data : [];
  } catch (reason: any) {
    error.value = reason?.response?.data?.msg || reason?.message || String(reason);
  } finally {
    loading.value = false;
  }
}

function kindLabel(kind: string) {
  return ({ agent_approval: "工具审批", task_approval: "设备审批", continuity: "事项等待", task_failed: "任务失败", task_blocked: "任务等待", task_result: "任务结果", automation_failed: "自动化失败", automation_approval: "自动化确认", source_error: "读取异常" } as Record<string, string>)[kind] || kind;
}

function tagType(item: AttentionItem): "danger" | "warning" | "success" | "info" | "primary" {
  if (item.kind === "task_failed" || item.kind === "automation_failed") return "danger";
  if (item.kind === "automation_approval") return "warning";
  if (item.kind === "task_result") return "success";
  if (item.priority >= 90) return "warning";
  return "info";
}

function formatTime(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

function hasTimeline(item: AttentionItem) {
  return Boolean(item.requestId || item.threadId || item.coordinationId || item.approvalId || item.workflowRunId);
}

async function openTimeline(item: AttentionItem) {
  timeline.value = [];
  timelineVisible.value = true;
  try {
    const response = await apiClient.get("/api/attention/timeline", { params: { requestId: item.requestId, threadId: item.threadId, coordinationId: item.coordinationId, approvalId: item.approvalId, workflowRunId: item.workflowRunId } });
    timeline.value = Array.isArray(response.data) ? response.data : [];
  } catch (reason: any) {
    ElMessage.error(reason?.response?.data?.msg || reason?.message || String(reason));
  }
}

function timelineType(status?: string): "primary" | "success" | "warning" | "danger" | "info" {
  if (status === "failed" || status === "denied") return "danger";
  if (status === "succeeded" || status === "completed" || status === "approved") return "success";
  if (status === "waiting" || status === "paused" || status === "observe") return "warning";
  return "primary";
}

function timelineDetails(details?: Record<string, unknown>) {
  if (!details || Object.keys(details).length === 0) return "";
  return JSON.stringify(details, null, 2);
}

function openRoute(route: string) {
  void router.push(route);
}

async function resolveAgentApproval(item: AttentionItem, approved: boolean) {
  if (!item.conversationId || !item.turnId || !item.approvalId) throw new Error("审批缺少会话或回合绑定");
  await ownedChatRuntimeRequest("approval", { conversationId: item.conversationId, turnId: item.turnId, approvalId: item.approvalId, approved }, item.roleId || "");
  ElMessage.success(approved ? "本次工具调用已允许" : "本次工具调用已拒绝");
  await refresh();
}

async function resolveTaskApproval(item: AttentionItem, approved: boolean) {
  const binding = item.payload?.binding;
  const revision = Number(item.payload?.revision);
  if (!binding) throw new Error("设备审批缺少授权绑定");
  await decideSourceTaskApproval({ id: item.approvalId, binding, revision, status: "pending" } as any, approved);
  ElMessage.success(approved ? "设备任务已批准" : "设备任务已拒绝");
  await refresh();
}

async function saveBudget() {
  budgetSaving.value = true;
  try {
    const response = await apiClient.put("/api/attention/budget", budget.policy);
    budget.policy = { ...budget.policy, ...(response.data || {}) };
    ElMessage.success("注意力预算已保存");
    await refresh();
  } catch (reason: any) {
    ElMessage.error(reason?.response?.data?.msg || reason?.message || String(reason));
  } finally {
    budgetSaving.value = false;
  }
}

function modeLabel(mode: string) {
  return ({ allow: "允许", observe: "仅记录", deny: "拒绝" } as Record<string, string>)[mode] || mode;
}

function permissionLabel(mode: string) {
  return mode === "full_access" ? "完全权限" : "请求批准";
}

function addPolicy() {
  Object.assign(policyForm, defaultPolicy());
  policyDialog.value = true;
}

function editPolicy(policy: SourcePolicy) {
  Object.assign(policyForm, policy);
  policyDialog.value = true;
}

async function savePolicy() {
  policySaving.value = true;
  try {
    const response = await apiClient.put("/api/agent-access/policies", { ...policyForm });
    Object.assign(policyForm, response.data || {});
    policyDialog.value = false;
    ElMessage.success("来源策略已保存");
    await refresh();
  } catch (reason: any) {
    ElMessage.error(reason?.response?.data?.msg || reason?.message || String(reason));
  } finally {
    policySaving.value = false;
  }
}

async function removePolicy(policy: SourcePolicy) {
  try {
    await ElMessageBox.confirm("删除后该范围会回退到上一级策略。", "删除来源策略？", { type: "warning" });
  } catch (reason) {
    console.error("[original-error]", reason);
    return;
  }
  try {
    await apiClient.delete(`/api/agent-access/policies/${encodeURIComponent(policy.id)}`, { params: { revision: policy.revision } });
    ElMessage.success("来源策略已删除");
    await refresh();
  } catch (reason: any) {
    ElMessage.error(reason?.response?.data?.msg || reason?.message || String(reason));
  }
}

onMounted(refresh);
</script>

<style scoped>
.attention-panel { display: grid; gap: 16px; }
.attention-heading, .attention-item, .attention-item-title, .attention-actions, .policy-toolbar, .budget-summary { display: flex; align-items: center; gap: 10px; }
.attention-heading { justify-content: space-between; }
.attention-heading h2 { margin: 0 0 4px; font-size: 20px; }
.attention-heading p, .attention-item p, .attention-item small { margin: 0; color: var(--text-secondary); }
.attention-list { display: grid; gap: 10px; min-height: 120px; }
.attention-item { justify-content: space-between; padding: 14px 16px; border: 1px solid var(--border-color); border-radius: 12px; background: var(--surface-color); }
.attention-item.priority-100, .attention-item.priority-98 { border-color: color-mix(in srgb, var(--el-color-warning) 55%, var(--border-color)); }
.attention-item-main { min-width: 0; display: grid; gap: 6px; }
.attention-item-title strong { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.attention-item-title span { margin-left: auto; color: var(--text-secondary); font-size: 12px; }
.attention-actions { flex-wrap: wrap; justify-content: flex-end; }
.attention-settings { border-top: 1px solid var(--border-color); }
.budget-summary { flex-wrap: wrap; margin-bottom: 14px; }
.settings-form { display: grid; gap: 10px; }
.settings-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 0 14px; }
.hint { margin-left: 8px; color: var(--text-secondary); font-size: 12px; }
.policy-toolbar { justify-content: space-between; margin-bottom: 10px; }
.timeline-summary { white-space: pre-wrap; word-break: break-word; }
.timeline-details { margin: 8px 0 0; padding: 8px 10px; overflow: auto; border-radius: 8px; background: var(--el-fill-color-light); color: var(--el-text-color-regular); font: 11px/1.55 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; white-space: pre-wrap; overflow-wrap: anywhere; }
@media (max-width: 760px) {
  .attention-item { align-items: stretch; flex-direction: column; }
  .attention-actions { justify-content: flex-start; }
}
</style>
