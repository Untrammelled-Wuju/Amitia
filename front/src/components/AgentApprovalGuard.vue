<template>
  <el-dialog
    v-model="visible"
    :close-on-click-modal="false"
    :close-on-press-escape="false"
    width="min(620px, calc(100vw - 32px))"
    append-to-body
  >
    <template #header>
      <div class="approval-header">
        <div>
          <strong>{{ current?.title || "需要你的处理" }}</strong>
          <span>{{ kindLabel(current?.kind) }}</span>
        </div>
        <el-tag :type="tagType(current)">{{ tagLabel(current) }}</el-tag>
      </div>
    </template>
    <el-alert
      v-if="current?.kind === 'agent_approval'"
      title="本次批准仅允许当前工具调用继续执行，不会更改会话的权限模式。"
      type="warning"
      show-icon
      :closable="false"
    />
    <p v-if="current?.summary" class="attention-summary">{{ current.summary }}</p>
    <p v-if="current?.reason" class="attention-reason">{{ current.reason }}</p>
    <dl v-if="current" class="approval-facts">
      <div>
        <dt>来源</dt>
        <dd>{{ current.kind === "agent_approval" ? "Agent 工具" : current.kind === "task_approval" ? "设备任务" : current.kind.startsWith("automation_") ? "自动化任务" : "持续事项" }}</dd>
      </div>
      <div v-if="current.kind === 'agent_approval'">
        <dt>风险等级</dt>
        <dd>{{ current.payload?.riskLevel || "未知" }}</dd>
      </div>
      <div>
        <dt>到期时间</dt>
        <dd>{{ formatTime(current.expiresAt || current.createdAt) }}</dd>
      </div>
    </dl>
    <section v-if="current?.kind === 'agent_approval'" class="approval-arguments">
      <strong>调用参数</strong>
      <pre>{{ prettyArguments }}</pre>
    </section>
    <section v-if="timeline.length" class="approval-timeline">
      <strong>决策时间线</strong>
      <el-timeline>
        <el-timeline-item v-for="event in timeline" :key="event.id" :timestamp="formatTime(event.occurredAt)">
          <span>{{ event.title }}</span>
          <p v-if="event.summary">{{ event.summary }}</p>
          <pre v-if="timelineDetails(event.details)" class="timeline-details">{{ timelineDetails(event.details) }}</pre>
        </el-timeline-item>
      </el-timeline>
    </section>
    <template #footer>
      <div class="approval-footer">
        <el-button :loading="resolving" @click="visible = false">稍后</el-button>
        <el-button v-if="current && hasTimeline(current)" :loading="timelineLoading" @click="loadTimeline(current)">查看时间线</el-button>
        <el-button v-if="current?.route" @click="openRoute(current.route)">打开任务</el-button>
        <template v-if="current?.kind === 'agent_approval'">
          <el-button :loading="resolving" @click="resolveAgent(current, false)">拒绝</el-button>
          <el-button type="primary" :loading="resolving" @click="resolveAgent(current, true)">本次允许</el-button>
        </template>
        <template v-else-if="current?.kind === 'task_approval'">
          <el-button :loading="resolving" @click="resolveTask(current, false)">拒绝</el-button>
          <el-button type="primary" :loading="resolving" @click="resolveTask(current, true)">批准本次执行</el-button>
        </template>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage } from "element-plus";
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
  expiresAt?: string;
  payload?: Record<string, any>;
}

interface TimelineEvent {
  id: string;
  title: string;
  summary?: string;
  occurredAt: string;
  details?: Record<string, unknown>;
}

const route = useRoute();
const router = useRouter();
const items = ref<AttentionItem[]>([]);
const dismissed = ref(new Set<string>());
const resolving = ref(false);
const timelineLoading = ref(false);
const timeline = ref<TimelineEvent[]>([]);
let pollTimer: ReturnType<typeof setInterval> | null = null;
let refreshEpoch = 0;

const conversationId = computed(() =>
  route.path.startsWith("/chat") ? String(route.query.conversationId || "").trim() : "",
);
const attentionId = computed(() => String(route.query.attentionId || "").trim());
const candidates = computed(() =>
  items.value
    .filter((item) => item.requiresAction && item.conversationId === conversationId.value && (!dismissed.value.has(item.id) || item.id === attentionId.value))
    .sort((left, right) => right.priority - left.priority || Date.parse(left.createdAt) - Date.parse(right.createdAt)),
);
const current = computed(() => {
  if (!conversationId.value) return undefined;
  if (attentionId.value) {
    const exact = candidates.value.find((item) => item.id === attentionId.value);
    if (exact) return exact;
  }
  return candidates.value[0];
});
const visible = computed({
  get: () => Boolean(current.value),
  set: (value: boolean) => {
    if (!value) dismissCurrent();
  },
});
const prettyArguments = computed(() => {
  const raw = String(current.value?.payload?.arguments || "").trim();
  if (!raw) return "无参数";
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch (error) {
    console.error("[original-error]", error);
    return raw.slice(0, 8000);
  }
});

function kindLabel(kind?: string) {
  return ({
    agent_approval: "工具执行审批",
    task_approval: "设备任务审批",
    continuity: "持续事项等待",
    task_failed: "任务失败恢复",
    task_blocked: "任务等待处理",
    automation_failed: "自动化失败",
    automation_approval: "自动化确认",
  } as Record<string, string>)[kind || ""] || "需要处理";
}

function tagType(item?: AttentionItem): "danger" | "warning" | "primary" {
  if (item?.kind === "task_failed" || item?.kind === "automation_failed") return "danger";
  if (item?.kind === "automation_approval") return "warning";
  if (item?.kind === "continuity" || item?.kind === "task_blocked") return "warning";
  return "primary";
}

function tagLabel(item?: AttentionItem) {
  if (item?.kind === "task_failed" || item?.kind === "automation_failed") return "等待恢复";
  if (item?.kind === "automation_approval") return "等待确认";
  if (item?.kind === "task_approval") return "等待批准";
  if (item?.kind === "continuity") return "等待确认";
  return "请求批准";
}

function formatTime(value?: string) {
  if (!value) return "未知";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

function timelineDetails(details?: Record<string, unknown>) {
  if (!details || Object.keys(details).length === 0) return "";
  return JSON.stringify(details, null, 2);
}

function hasTimeline(item: AttentionItem) {
  return Boolean(item.requestId || item.threadId || item.coordinationId || item.approvalId || item.workflowRunId);
}

async function loadItems() {
  const epoch = ++refreshEpoch;
  if (!conversationId.value) {
    items.value = [];
    return;
  }
  try {
    const response = await apiClient.get("/api/attention/items");
    if (epoch !== refreshEpoch) return;
    items.value = Array.isArray(response.data) ? response.data : [];
  } catch (error: any) {
    if (epoch === refreshEpoch) {
      console.error("[original-error]", error?.response?.data?.msg || error?.message || error);
    }
  }
}

function dismissCurrent() {
  const item = current.value;
  if (!item) return;
  dismissed.value.add(item.id);
  if (attentionId.value !== item.id) return;
  const query = { ...route.query };
  delete query.attentionId;
  void router.replace({ path: route.path, query });
}

async function resolveAgent(item: AttentionItem, approved: boolean) {
  if (!item.conversationId || !item.turnId || !item.approvalId) {
    throw new Error("审批缺少会话或回合绑定");
  }
  resolving.value = true;
  try {
    const result = await ownedChatRuntimeRequest<{ id: string; turnId: string; approved: boolean }>(
      "approval",
      { conversationId: item.conversationId, turnId: item.turnId, approvalId: item.approvalId, approved },
      item.roleId || "",
    );
    if (result.id !== item.approvalId || result.turnId !== item.turnId || result.approved !== approved) {
      throw new Error("审批结果与当前请求不匹配");
    }
    dismissed.value.add(item.id);
    ElMessage.success(approved ? "本次工具调用已允许" : "本次工具调用已拒绝");
    await loadItems();
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.msg || error?.message || String(error));
  } finally {
    resolving.value = false;
  }
}

async function resolveTask(item: AttentionItem, approved: boolean) {
  const binding = item.payload?.binding;
  const revision = Number(item.payload?.revision);
  if (!binding || !item.approvalId) throw new Error("设备审批缺少授权绑定");
  resolving.value = true;
  try {
    await decideSourceTaskApproval({ id: item.approvalId, binding, revision, status: "pending" } as any, approved);
    dismissed.value.add(item.id);
    ElMessage.success(approved ? "设备任务已批准" : "设备任务已拒绝");
    await loadItems();
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.msg || error?.message || String(error));
  } finally {
    resolving.value = false;
  }
}

async function loadTimeline(item: AttentionItem) {
  timelineLoading.value = true;
  try {
    const response = await apiClient.get("/api/attention/timeline", {
      params: {
        requestId: item.requestId,
        threadId: item.threadId,
        coordinationId: item.coordinationId,
        approvalId: item.approvalId,
        workflowRunId: item.workflowRunId,
      },
    });
    timeline.value = Array.isArray(response.data) ? response.data : [];
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.msg || error?.message || String(error));
  } finally {
    timelineLoading.value = false;
  }
}

function openRoute(target: string) {
  void router.push(target);
}

function onApprovalEvent() {
  void loadItems();
}

watch(
  () => route.fullPath,
  () => {
    timeline.value = [];
    void loadItems();
  },
);

watch(current, (next, previous) => {
  if (next?.id !== previous?.id) timeline.value = [];
});

onMounted(() => {
  window.addEventListener("amitia:agent-approval", onApprovalEvent);
  void loadItems();
  pollTimer = setInterval(() => {
    if (document.visibilityState === "visible") void loadItems();
  }, 3000);
});

onBeforeUnmount(() => {
  window.removeEventListener("amitia:agent-approval", onApprovalEvent);
  if (pollTimer) clearInterval(pollTimer);
});
</script>

<style scoped>
.approval-header,
.approval-footer {
  display: flex;
  align-items: center;
  gap: 10px;
}

.approval-header {
  justify-content: space-between;
}

.approval-header strong,
.approval-header span {
  display: block;
}

.approval-header strong {
  font-size: 15px;
}

.approval-header span {
  margin-top: 3px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.attention-summary {
  margin: 14px 0 6px;
  color: var(--el-text-color-primary);
  font-size: 14px;
}

.attention-reason {
  margin: 0;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.approval-facts {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  margin: 14px 0;
}

.approval-facts div {
  border-radius: 8px;
  padding: 9px 10px;
  background: var(--el-fill-color-light);
}

.approval-facts dt {
  color: var(--el-text-color-secondary);
  font-size: 11px;
}

.approval-facts dd {
  margin: 4px 0 0;
  color: var(--el-text-color-primary);
  font-size: 13px;
}

.approval-arguments pre {
  max-height: 240px;
  margin: 7px 0 0;
  overflow: auto;
  border-radius: 8px;
  padding: 10px;
  background: var(--el-fill-color-light);
  color: var(--el-text-color-regular);
  font: 11px/1.55 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.approval-timeline {
  max-height: 280px;
  overflow: auto;
  margin-top: 16px;
}

.timeline-details {
  margin: 7px 0 0;
  overflow: auto;
  border-radius: 8px;
  padding: 8px 10px;
  background: var(--el-fill-color-light);
  color: var(--el-text-color-regular);
  font: 11px/1.55 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.approval-timeline p {
  margin: 3px 0 0;
  color: var(--el-text-color-secondary);
  font-size: 11px;
}

.approval-footer {
  justify-content: flex-end;
  flex-wrap: wrap;
}

@media (max-width: 560px) {
  .approval-facts {
    grid-template-columns: 1fr;
  }
}
</style>
