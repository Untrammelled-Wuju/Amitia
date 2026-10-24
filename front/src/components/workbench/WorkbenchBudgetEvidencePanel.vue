<template>
  <section class="evidence" aria-label="预算未确认消费核对">
    <div class="actions">
      <button type="button" :disabled="stale || busy" @click="open('model')">核对模型用量</button>
      <button type="button" :disabled="stale || busy" @click="open('tool')">核对工具调用</button>
      <button v-if="kind" type="button" @click="close">关闭证据</button>
    </div>
    <div v-if="kind" class="details">
      <p class="note">持久预算账本：未确认消费不会自动返还额度，工具调用记录不是外部操作完成或撤销的证明。</p>
      <p v-if="busy" role="status">正在从 Owner 账本读取证据…</p>
      <p v-if="error" role="alert" class="warning">{{ error }}</p>
      <template v-if="!stale">
        <p v-if="!busy && !records.length" class="note">当前范围没有待核对记录。</p>
        <div v-for="entry in records" :key="entry.id" class="entry">
          <span>{{ kind === 'model' ? '模型调用' : entry.toolName }} · {{ entry.status }}</span>
          <small>{{ entry.id }}</small>
          <small v-if="kind === 'model'">保留 Token {{ entry.reservedTokens }} · 保留金额 {{ entry.reservedCostMicroUSD }} 微美元 · {{ entry.budgetOverrun ? '超出预留上界，需人工核对（额度不返还）' : entry.usageReported ? '有可信用量' : '用量未确认' }}</small>
          <small>{{ formatDate(entry.startedAt) }}</small>
          <small v-if="entry.reviewEvidenceRef">人工登记的证据引用（未核实，不代表核销）: {{ entry.reviewEvidenceRef }} · 登记设备 {{ entry.reviewActorId }} · {{ formatDate(entry.reviewRecordedAt) }}</small>
          <button v-if="canReview && !entry.reviewEvidenceRef && editingId !== entry.id" type="button" :disabled="busy" @click="beginReview(entry.id)">登记核对证据</button>
          <form v-if="canReview && editingId === entry.id" @submit.prevent="saveReview(entry.id)">
            <label>证据引用<input v-model="evidenceRef" aria-label="证据引用" :disabled="busy" maxlength="512" required /></label>
            <small>登记会保留原额度和执行状态，已登记内容不可覆盖。</small>
            <button type="submit" :disabled="busy || stale">{{ busy ? '正在确认登记' : '确认登记' }}</button>
            <button type="button" :disabled="busy" @click="editingId = ''">取消</button>
          </form>
        </div>
        <button v-if="next && !busy" type="button" :disabled="stale" @click="fetchPage(next)">继续读取下一页</button>
      </template>
    </div>
  </section>
</template>
<script setup lang="ts">
import { ref, watch, onBeforeUnmount } from 'vue';
import { ElMessage } from 'element-plus';
import { loadBudgetEvidence, type BudgetEvidencePage, type BudgetEvidenceRecord } from './workbench-task-reconciliation';
import { registerWorkbenchBudgetEvidence, WorkbenchTaskReadError } from './workbench-task-api';

const props = defineProps<{
  conversationId: string;
  characterId: string;
  coordinationId: string;
  parentInteractionId: string;
  updatedAt: string;
  authority: string;
  stale: boolean;
}>();
const emit = defineEmits<{ invalidated: [] }>();
const kind = ref<'model' | 'tool' | ''>('');
const records = ref<BudgetEvidenceRecord[]>([]);
const next = ref<BudgetEvidencePage['next']>();
const error = ref('');
const busy = ref(false);
const canReview = ref(false);
const editingId = ref('');
const evidenceRef = ref('');
let generation = 0;
let controller: AbortController | undefined;
function close() {
  generation++;
  controller?.abort();
  controller = undefined;
  kind.value = '';
  records.value = [];
  next.value = undefined;
  error.value = '';
  busy.value = false;
  canReview.value = false;
  editingId.value = '';
  evidenceRef.value = '';
}
function open(selected: 'model' | 'tool') {
  if (props.stale || !props.authority) return;
  close();
  kind.value = selected;
  void fetchPage();
}
async function fetchPage(cursor?: BudgetEvidencePage['next']) {
  if (props.stale || !props.authority || !kind.value || busy.value) return;
  const token = ++generation;
  const selected = kind.value;
  const authority = props.authority;
  const controllerForRequest = new AbortController();
  controller = controllerForRequest;
  busy.value = true;
  error.value = '';
  try {
    const response = await loadBudgetEvidence(props.conversationId, props.characterId, props.coordinationId, props.parentInteractionId, selected, controllerForRequest.signal, cursor);
    if (controllerForRequest.signal.aborted || token !== generation || selected !== kind.value || props.stale || authority !== props.authority) return;
    records.value = cursor ? [...records.value, ...response.records] : response.records;
    canReview.value = response.canReview;
    next.value = response.next;
  } catch (cause) {
    if (token !== generation || controllerForRequest.signal.aborted) return;
    if (cause instanceof WorkbenchTaskReadError && cause.invalidatesSnapshot) {
      close();
      emit('invalidated');
      return;
    }
    close();
    error.value = (cause as Error)?.message || '预算证据读取失败';
    kind.value = selected;
  } finally {
    if (token === generation) { busy.value = false; controller = undefined; }
  }
}
function beginReview(id: string) {
  if (props.stale || busy.value || !canReview.value) return;
  editingId.value = id;
  evidenceRef.value = '';
}
async function saveReview(id: string) {
  if (props.stale || busy.value || !canReview.value || !kind.value || editingId.value !== id) return;
  const reference = evidenceRef.value.trim();
  const length = new TextEncoder().encode(reference).length;
  if (length < 8 || length > 512 || /[\x00\r\n]/.test(reference)) {
    error.value = '证据引用须为 8–512 字节的单行内容';
    return;
  }
  const token = ++generation;
  const authority = props.authority;
  const selected = kind.value;
  const active = new AbortController();
  controller = active;
  busy.value = true;
  error.value = '';
  try {
    const result = await registerWorkbenchBudgetEvidence(props.conversationId, props.characterId, props.coordinationId, props.parentInteractionId, { kind: selected, recordId: id, evidenceRef: reference }, active.signal);
    if (active.signal.aborted || token !== generation || props.stale || authority !== props.authority) return;
    if (result.recorded !== true || result.effect !== 'evidence-only' || result.budgetReleased !== false || result.executionConfirmed !== false || result.coordinationId !== props.coordinationId || result.parentInteractionId !== props.parentInteractionId || result.recordId !== id || result.kind !== selected || result.evidenceRef !== reference) throw new Error('登记回执无法核实，请重新读取证据确认');
    close();
    open(selected);
    ElMessage.success('证据已登记，额度和执行状态保持原样');
  } catch (cause) {
    if (token !== generation || active.signal.aborted) return;
    if (cause instanceof WorkbenchTaskReadError && cause.invalidatesSnapshot) {
      close();
      emit('invalidated');
      return;
    }
    error.value = (cause as Error)?.message || '登记结果未确认，请重新读取证据核对';
  } finally {
    if (token === generation) { busy.value = false; controller = undefined; }
  }
}
function formatDate(raw: string) {
  const value = Date.parse(raw);
  return Number.isFinite(value) ? new Date(value).toLocaleString() : '时间未知';
}
watch(() => [props.conversationId, props.characterId, props.coordinationId, props.parentInteractionId, props.updatedAt, props.authority, props.stale], close);
onBeforeUnmount(close);
</script>
<style scoped>
.evidence{border:1px solid var(--surface-border);border-radius:7px;padding:8px}.actions{display:flex;gap:8px;flex-wrap:wrap}.actions button,.details button{border:1px solid var(--surface-border);border-radius:6px;padding:5px 8px;background:transparent;color:inherit;cursor:pointer}.actions button:disabled{opacity:.45;cursor:not-allowed}.details{display:flex;flex-direction:column;gap:7px;padding-top:8px}.entry{display:flex;flex-direction:column;gap:3px;border-bottom:1px solid var(--surface-border);padding-bottom:8px;overflow-wrap:anywhere}.entry small,.note{font-size:12px;color:var(--el-text-color-secondary)}.warning{color:var(--el-color-danger)}
</style>
