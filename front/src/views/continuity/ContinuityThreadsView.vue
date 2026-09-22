<template>
  <div class="continuity-page">
    <div class="page-header">
      <div>
        <h2>持续事项</h2>
        <p>跨会话保存“做到哪、下一步、在等什么”。执行仍由现有 Agent / Workflow / Device Runtime 负责。</p>
      </div>
      <el-button type="primary" @click="createDialog = true">新建事项</el-button>
    </div>

    <div class="toolbar">
      <el-input v-model="query" clearable placeholder="搜索标题、目标或当前状态" @keyup.enter="refresh" />
      <el-select v-model="statusFilter" clearable placeholder="全部状态" @change="refresh">
        <el-option v-for="item in statusOptions" :key="item.value" :label="item.label" :value="item.value" />
      </el-select>
      <el-button :loading="loading" @click="refresh">刷新</el-button>
    </div>

    <el-empty v-if="!loading && threads.length === 0" description="暂无持续事项" />
    <div v-else class="thread-grid">
      <button
        v-for="thread in threads"
        :key="thread.id"
        type="button"
        class="thread-card"
        :class="{ active: selectedId === thread.id }"
        @click="selectThread(thread.id)"
      >
        <div class="thread-card-top">
          <strong>{{ thread.title }}</strong>
          <el-tag size="small" :type="tagType(thread.status)">{{ statusLabel(thread.status) }}</el-tag>
        </div>
        <div class="thread-state">{{ thread.currentState || thread.summary || thread.goal || "暂无状态摘要" }}</div>
        <div v-if="thread.nextAction" class="next-action">下一步：{{ thread.nextAction }}</div>
        <div class="thread-time">最近活动 {{ formatTime(thread.lastActiveAt) }}</div>
      </button>
    </div>

    <el-drawer v-model="detailOpen" size="min(720px, 92vw)" :title="detail?.thread.title || '持续事项'">
      <template v-if="detail">
        <div class="detail-actions">
          <el-button v-if="detail.thread.status !== 'paused' && !terminal(detail.thread.status)" @click="setStatus('paused')">暂停</el-button>
          <el-button v-if="detail.thread.status === 'paused'" type="primary" @click="setStatus('active')">恢复</el-button>
          <el-button v-if="!terminal(detail.thread.status)" type="success" @click="setStatus('completed')">完成</el-button>
          <el-button @click="waitDialog = true" :disabled="terminal(detail.thread.status)">添加等待条件</el-button>
        </div>

        <el-descriptions :column="1" border>
          <el-descriptions-item label="目标">{{ detail.thread.goal || "—" }}</el-descriptions-item>
          <el-descriptions-item label="当前状态">{{ detail.thread.currentState || "—" }}</el-descriptions-item>
          <el-descriptions-item label="下一步">{{ detail.thread.nextAction || "—" }}</el-descriptions-item>
          <el-descriptions-item label="摘要">{{ detail.thread.summary || "—" }}</el-descriptions-item>
        </el-descriptions>

        <section class="detail-section">
          <h3>等待条件</h3>
          <el-empty v-if="detail.waits.length === 0" :image-size="64" description="没有等待条件" />
          <el-table v-else :data="detail.waits" size="small">
            <el-table-column prop="waitType" label="类型" width="110" />
            <el-table-column prop="description" label="说明" min-width="220" />
            <el-table-column label="状态" width="110">
              <template #default="scope"><el-tag size="small">{{ scope.row.status }}</el-tag></template>
            </el-table-column>
            <el-table-column label="唤醒" width="110">
              <template #default="scope">{{ scope.row.wakeState || "—" }}</template>
            </el-table-column>
            <el-table-column label="操作" width="110">
              <template #default="scope">
                <el-button v-if="scope.row.status === 'waiting'" link type="primary" @click="resolveWait(scope.row)">解除</el-button>
                <el-button v-if="scope.row.status === 'waiting'" link type="danger" @click="cancelWait(scope.row)">取消</el-button>
              </template>
            </el-table-column>
          </el-table>
        </section>

        <section class="detail-section">
          <h3>最近事件</h3>
          <el-timeline v-if="detail.events.length">
            <el-timeline-item v-for="event in detail.events" :key="event.id" :timestamp="formatTime(event.occurredAt)">
              <strong>{{ event.eventType }}</strong>
              <div class="event-summary">{{ eventSummary(event.payloadJson) }}</div>
            </el-timeline-item>
          </el-timeline>
          <el-empty v-else :image-size="64" description="暂无事件" />
        </section>
      </template>
    </el-drawer>

    <el-dialog v-model="createDialog" title="新建持续事项" width="520px">
      <el-form label-position="top">
        <el-form-item label="标题"><el-input v-model="createForm.title" maxlength="120" /></el-form-item>
        <el-form-item label="目标"><el-input v-model="createForm.goal" type="textarea" :rows="3" /></el-form-item>
        <el-form-item label="下一步"><el-input v-model="createForm.nextAction" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createDialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="createThread">创建</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="waitDialog" title="添加等待条件" width="560px">
      <el-form label-position="top">
        <el-form-item label="类型">
          <el-select v-model="waitForm.type">
            <el-option label="等待用户" value="user" />
            <el-option label="等待时间" value="time" />
            <el-option label="等待设备" value="device" />
            <el-option label="等待外部事件" value="external" />
            <el-option label="等待审批" value="approval" />
            <el-option label="等待依赖" value="dependency" />
          </el-select>
        </el-form-item>
        <el-form-item label="说明"><el-input v-model="waitForm.description" /></el-form-item>
        <el-form-item v-if="waitForm.type === 'time'" label="到期时间">
          <el-date-picker v-model="waitForm.dueAt" type="datetime" value-format="YYYY-MM-DDTHH:mm:ssZ" />
        </el-form-item>
        <el-form-item v-if="!['user', 'time'].includes(waitForm.type)" label="匹配条件 JSON">
          <el-input v-model="waitForm.conditionJson" type="textarea" :rows="4" placeholder='例如 {"deviceId":"..."}' />
        </el-form-item>
        <el-form-item label="恢复提示"><el-input v-model="waitForm.resumeHint" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="waitDialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="addWait">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  cancelContinuityWait,
  createContinuityThread,
  createContinuityWait,
  getContinuityThread,
  listContinuityThreads,
  resolveContinuityWait,
  updateContinuityThread,
  type ContinuityDetail,
  type ContinuityEvent,
  type ContinuityThread,
  type ContinuityWait,
  type ThreadStatus,
} from "./api";

const loading = ref(false);
const saving = ref(false);
const threads = ref<ContinuityThread[]>([]);
const selectedId = ref("");
const detail = ref<ContinuityDetail | null>(null);
const detailOpen = ref(false);
const query = ref("");
const statusFilter = ref<ThreadStatus | "">("");
const createDialog = ref(false);
const waitDialog = ref(false);
const createForm = reactive({ title: "", goal: "", nextAction: "" });
const waitForm = reactive({ type: "user", description: "", dueAt: "", conditionJson: "{}", resumeHint: "" });

const statusOptions: Array<{ label: string; value: ThreadStatus }> = [
  { label: "进行中", value: "active" }, { label: "等待中", value: "waiting" }, { label: "已阻塞", value: "blocked" },
  { label: "已暂停", value: "paused" }, { label: "已完成", value: "completed" }, { label: "已取消", value: "cancelled" },
];

function statusLabel(status: ThreadStatus) { return statusOptions.find((item) => item.value === status)?.label || status; }
function tagType(status: ThreadStatus) { return status === "completed" ? "success" : status === "blocked" ? "danger" : status === "waiting" ? "warning" : status === "paused" ? "info" : "primary"; }
function terminal(status: ThreadStatus) { return status === "completed" || status === "cancelled"; }
function formatTime(value?: string) { if (!value) return "—"; return new Date(value).toLocaleString(); }
function eventSummary(raw: string) { try { const data = JSON.parse(raw || "{}"); return String(data.summary || data.error || ""); } catch { return ""; } }

async function refresh() {
  loading.value = true;
  try {
    threads.value = await listContinuityThreads({ q: query.value || undefined, status: statusFilter.value || undefined, limit: 100 });
  } finally { loading.value = false; }
}

async function selectThread(id: string) {
  selectedId.value = id;
  detail.value = await getContinuityThread(id);
  detailOpen.value = true;
}

async function reloadDetail() {
  if (!selectedId.value) return;
  detail.value = await getContinuityThread(selectedId.value);
  await refresh();
}

async function setStatus(status: ThreadStatus) {
  if (!detail.value) return;
  saving.value = true;
  try { await updateContinuityThread(detail.value.thread.id, { status }); await reloadDetail(); }
  finally { saving.value = false; }
}

async function resolveWait(wait: ContinuityWait) {
  if (!detail.value) return;
  await resolveContinuityWait(detail.value.thread.id, wait.id, wait.autoResume);
  await reloadDetail();
}

async function cancelWait(wait: ContinuityWait) {
  if (!detail.value) return;
  try {
    await ElMessageBox.confirm("取消后不会触发自动恢复。", "取消等待条件", { type: "warning" });
  } catch {
    return;
  }
  await cancelContinuityWait(detail.value.thread.id, wait.id);
  await reloadDetail();
}

async function createThread() {
  if (!createForm.title.trim()) { ElMessage.warning("请输入标题"); return; }
  saving.value = true;
  try {
    const item = await createContinuityThread({ ...createForm });
    createDialog.value = false;
    createForm.title = ""; createForm.goal = ""; createForm.nextAction = "";
    await refresh();
    await selectThread(item.id);
  } finally { saving.value = false; }
}

async function addWait() {
  if (!detail.value) return;
  let condition: Record<string, unknown> = {};
  try { condition = JSON.parse(waitForm.conditionJson || "{}"); }
  catch { ElMessage.warning("匹配条件不是有效 JSON"); return; }
  saving.value = true;
  try {
    await createContinuityWait(detail.value.thread.id, {
      type: waitForm.type, description: waitForm.description, condition,
      dueAt: waitForm.type === "time" ? waitForm.dueAt || undefined : undefined,
      resumeHint: waitForm.resumeHint,
    });
    waitDialog.value = false;
    waitForm.description = ""; waitForm.dueAt = ""; waitForm.conditionJson = "{}"; waitForm.resumeHint = "";
    await reloadDetail();
  } finally { saving.value = false; }
}

onMounted(refresh);
</script>

<style scoped>
.continuity-page { max-width: 1280px; margin: 0 auto; }
.page-header { display: flex; justify-content: space-between; gap: 16px; align-items: flex-start; margin-bottom: 18px; }
.page-header h2 { margin: 0 0 6px; }
.page-header p { margin: 0; color: var(--el-text-color-secondary); }
.toolbar { display: grid; grid-template-columns: minmax(260px, 1fr) 180px auto; gap: 10px; margin-bottom: 16px; }
.thread-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 12px; }
.thread-card { appearance: none; text-align: left; border: 1px solid var(--el-border-color); background: var(--el-bg-color); border-radius: 10px; padding: 16px; cursor: pointer; color: inherit; }
.thread-card:hover, .thread-card.active { border-color: var(--el-color-primary); }
.thread-card-top { display: flex; justify-content: space-between; gap: 12px; align-items: center; }
.thread-state { margin-top: 12px; line-height: 1.55; min-height: 44px; color: var(--el-text-color-regular); }
.next-action { margin-top: 10px; font-size: 13px; color: var(--el-text-color-secondary); }
.thread-time { margin-top: 12px; font-size: 12px; color: var(--el-text-color-placeholder); }
.detail-actions { display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 16px; }
.detail-section { margin-top: 24px; }
.detail-section h3 { margin: 0 0 12px; }
.event-summary { margin-top: 4px; color: var(--el-text-color-secondary); line-height: 1.5; }
@media (max-width: 720px) { .toolbar { grid-template-columns: 1fr; } .page-header { align-items: stretch; flex-direction: column; } }
</style>
