<template>
  <section class="terminal-shell">
    <header class="terminal-header">
      <span>工作区终端 <small v-if="mode">{{ mode === 'conpty' ? 'Windows ConPTY' : mode !== 'pipe' ? 'PTY' : '管道兼容模式（不支持完整交互）' }}</small></span>
      <select v-model="workspaceUri" :disabled="!!terminalId" aria-label="终端工作区">
        <option value="">请选择本机可写工作区</option>
        <option v-for="mount in mounts" :key="mount.id" :value="mount.rootUri">{{ mount.name }}</option>
      </select>
      <button v-if="!terminalId" type="button" :disabled="busy || !workspaceUri" @click="open">启动</button>
      <select v-if="!terminalId && sessions.length" v-model="resumeId" aria-label="恢复当前对话已有终端">
        <option value="">已有终端会话</option>
        <option v-for="session in sessions.filter(entry => (entry.taskId || '') === (props.tab.taskId || ''))" :key="session.id" :value="session.id">{{ session.id.slice(-8) }} · {{ session.running ? '运行中' : '已结束' }}</option>
      </select>
      <button v-if="!terminalId && resumeId" type="button" :disabled="busy" @click="resumeSession">恢复</button>
      <label v-if="terminalId && mode !== 'pipe'" class="terminal-dimensions"><input type="checkbox" v-model="autoFit" aria-label="自适应终端尺寸" />自动适应</label>
      <label v-if="terminalId && mode !== 'pipe' && !autoFit" class="terminal-dimensions">终端尺寸 <select v-model="dimensions" @change="resizeTerminal"><option value="24x80">80 × 24</option><option value="30x100">100 × 30</option><option value="40x120">120 × 40</option></select></label>
      <span v-if="terminalId" class="terminal-transport">{{ streaming ? '实时 WebSocket' : '兼容轮询' }}</span>
      <button v-if="terminalId" type="button" @click="closeTerminal">结束会话</button>
    </header>
    <div v-if="props.tab.taskId" class="terminal-receipts">
      <button type="button" :disabled="receiptsLoading" @click="refreshReceipts">{{ receiptsLoading ? '读取中' : '任务终端历史' }}</button>
      <span v-if="receiptError" role="alert">{{ receiptError }}</span>
      <span v-else-if="receiptsLoaded && receipts.length === 0">没有可查看的历史凭据</span>
      <details v-if="receipts.length"><summary>最近 {{ receiptSummary?.total ?? receipts.length }} 条终端运行记录 · 运行 {{ receiptSummary?.running ?? 0 }} / 中断 {{ receiptSummary?.interrupted ?? 0 }} / 撤销 {{ receiptSummary?.revoked ?? 0 }}</summary><ol><li v-for="receipt in receipts" :key="receipt.sessionId">{{ new Date(receipt.startedAt).toLocaleString() }} · {{ receiptState(receipt.state) }} · {{ receipt.exitCode === undefined ? '退出码未知' : `退出码 ${receipt.exitCode}` }} · {{ receipt.sessionId.slice(-8) }}</li></ol><small>仅含生命周期元数据，不能证明任务或测试通过；未确认结束的旧会话显示为中断。</small></details>
    </div>
    <p v-if="error" class="terminal-error" role="alert">{{ error }}</p>
    <details v-if="terminalId" class="terminal-timeline" @toggle="timelineToggled">
      <summary>终端生命周期 · {{ lifecycle.length }} 条可用事件（仅本机内存记录）</summary>
      <p v-if="lifecycleTruncated" class="terminal-timeline-warning">更早的事件已被环形缓冲覆盖。</p>
      <ol><li v-for="event in lifecycle" :key="event.sequence">{{ event.sequence }} · {{ formatLifecycle(event) }} · {{ new Date(event.at).toLocaleTimeString() }}</li></ol>
    </details>
    <div v-if="terminalId && (props.tab.taskId || mode)" class="terminal-association"><span v-if="props.tab.taskId">任务专属终端 · {{ props.tab.taskId }} · 服务端周期核验绑定</span><span v-else>普通工作区终端 · 未关联 Agent 任务</span><button type="button" :disabled="busy" @click="downloadTranscript">导出有界输出</button></div>
    <WorkbenchTerminalScreen v-if="mode === 'pty' || mode === 'conpty'" ref="terminalScreen" :columns="screenColumns" :lines="screenRows" :enabled="running && !busy" @ready="screenReady" @input="sendScreenInput" @size="applyAutoSize" />
    <div v-else ref="outputEl" class="terminal-output" role="log" aria-label="终端输出" aria-live="off" tabindex="0" @keydown="handleKeyboard" @click="focusTerminal"><pre>{{ output }}</pre></div>
    <p class="terminal-hint">PTY 模式支持 ANSI 色彩、光标定位和交替屏幕；当前是内置 VT 兼容渲染，并非完整 xterm.js。</p>
    <form class="terminal-input" @submit.prevent="sendLine">
      <input v-model="line" :disabled="!terminalId || !running || busy" aria-label="命令输入" autocomplete="off" autocapitalize="off" spellcheck="false" placeholder="输入命令，回车发送" />
      <button type="submit" :disabled="!terminalId || !running || busy">发送</button>
      <button type="button" :disabled="!terminalId || !running || busy || mode === 'pipe'" @click="interrupt">Ctrl+C</button>
    </form>
    <footer class="terminal-footer">{{ terminalId ? (running ? '会话运行中' : `进程已结束（退出码 ${exitCode}）`) : '仅可信本机用户可启动 · 不允许在云端直接执行' }}</footer>
  </section>
</template>
<script setup lang="ts">
import { nextTick, onActivated, onBeforeUnmount, onDeactivated, onMounted, ref } from 'vue';
import type { WorkbenchTab } from '@/stores/workbench';
import { useWorkbenchStore } from '@/stores/workbench';
import { terminalApi, type TerminalMount, type TerminalSnapshot, type TerminalSessionInfo, type TerminalLifecycleEvent, type TerminalTaskReceipt, type TerminalTaskReceiptSummary } from './useWorkbenchTerminal';
import { connectTerminalStream, type TerminalStream } from './useWorkbenchTerminalStream';
import { TerminalTextBuffer, keyToTerminalData } from './terminalTextBuffer';
import WorkbenchTerminalScreen from './WorkbenchTerminalScreen.vue';
import { appendLifecycleEvents } from './terminalLifecycle';
import { reconcileTerminalChunk, reconcileTerminalStatus } from './terminalSnapshotReconcile';

const props = defineProps<{ tab: WorkbenchTab }>();
const store = useWorkbenchStore();
const conversation = store.conversationId;
const mounts = ref<TerminalMount[]>([]);
const sessions = ref<TerminalSessionInfo[]>([]);
const lifecycle = ref<TerminalLifecycleEvent[]>([]);
const receipts = ref<TerminalTaskReceipt[]>([]);
const receiptSummary = ref<TerminalTaskReceiptSummary | null>(null);
const receiptsLoading = ref(false);
const receiptsLoaded = ref(false);
const receiptError = ref('');
function receiptState(value: string) { return ({ running:'运行中',exited:'进程已退出',closed:'已关闭',revoked:'授权已撤销',interrupted:'上次服务中断，运行结果未知' } as Record<string,string>)[value] || '未知'; }
async function refreshReceipts() {
  if (!props.tab.taskId || receiptsLoading.value || disposed) return;
  receiptsLoading.value = true;
  receiptError.value = '';
  try { const data = await terminalApi.taskReceiptDiagnostics(conversation, props.tab.taskId); if (!disposed) { receipts.value = data.receipts; receiptSummary.value = data.summary; receiptsLoaded.value = true; } }
  catch (cause) { if (!disposed) receiptError.value = failure(cause); }
  finally { receiptsLoading.value = false; }
}
const lifecycleTruncated = ref(false);
let lifecycleCursor = 0;
let lastLifecycleRefresh = 0;
let timelineOpen = false;
let timelineFetching = false;
function clearLifecycle() { lifecycle.value = []; lifecycleTruncated.value = false; lifecycleCursor = 0; lastLifecycleRefresh = 0; }
function formatLifecycle(event: TerminalLifecycleEvent) {
  if (event.kind === 'session.started') return `进程启动 · ${event.cols ?? '?'}×${event.rows ?? '?'}`;
  if (event.kind === 'session.resized') return `窗口调整 · ${event.cols ?? '?'}×${event.rows ?? '?'}`;
  if (event.kind === 'process.exited') return `进程退出 · 退出码 ${event.exitCode ?? '未知'}`;
  return '终端事件';
}
async function refreshLifecycle() {
  if (!timelineOpen || timelineFetching || disposed || !active || !terminalId.value) return;
  timelineFetching = true;
  const id = terminalId.value;
  try {
    lastLifecycleRefresh = Date.now();
    const page = await terminalApi.lifecycle(id, conversation, lifecycleCursor);
    if (disposed || id !== terminalId.value) return;
    lifecycleTruncated.value ||= page.truncated;
    lifecycle.value = appendLifecycleEvents(lifecycle.value, page);
    lifecycleCursor = Math.max(lifecycleCursor, page.latestSequence);
  } catch (cause) { if (id === terminalId.value) error.value = failure(cause); }
  finally { timelineFetching = false; }
}
function timelineToggled(event: Event) {
  timelineOpen = (event.target as HTMLDetailsElement).open;
  if (timelineOpen) void refreshLifecycle();
}
const resumeId = ref('');
const workspaceUri = ref(props.tab.workspaceUri || '');
const terminalId = ref('');
const mode = ref<'pty' | 'pipe' | 'conpty' | ''>('');
const error = ref('');
const output = ref('');
const line = ref('');
const busy = ref(false);
const running = ref(false);
const exitCode = ref(-1);
const streaming = ref(false);
const dimensions = ref("30x100");
const autoFit = ref(true);
let autoSizeTimer: ReturnType<typeof setTimeout> | undefined;
const outputEl = ref<HTMLDivElement>();
const terminalScreen = ref<{ write: (value: string) => void; reset: () => void; truncated: () => void; resize: (cols: number, rows: number) => void; focus: () => void }>();
const screenRows = ref(30);
const screenColumns = ref(100);
let screenPending = '';
let inputQueue: Promise<void> = Promise.resolve();
let cursor = 0;
let terminalExited = false;
let pollTimer: ReturnType<typeof setInterval> | undefined;
let polling = false;
let stream: TerminalStream | undefined;
let reconnectTimer: ReturnType<typeof setTimeout> | undefined;
let reconnectAttempts = 0;
let transportVersion = 0;
let active = true;
let connecting: AbortController | undefined;
let disposed = false;
let decoder = new TextDecoder();
const textBuffer = new TerminalTextBuffer();
function failure(value: unknown) { return (value as any)?.response?.data?.error || (value as Error)?.message || String(value); }
function consume(snap: TerminalSnapshot) {
  const page = reconcileTerminalChunk(snap, cursor);
  if (page.kind === 'ignore') return;
  mode.value = snap.mode;
  const status = reconcileTerminalStatus(terminalExited, exitCode.value, snap);
  terminalExited = status.ended;
  running.value = status.running;
  exitCode.value = status.exitCode;
  if (page.truncated) {
    textBuffer.truncated(); decoder = new TextDecoder();
    if (terminalScreen.value) terminalScreen.value.truncated();
    else screenPending += '\r\n[较早的终端输出已被截断]\r\n';
  }
  const chunk = decoder.decode(page.bytes, { stream: running.value });
  if (mode.value === 'pipe') { textBuffer.write(chunk); output.value = textBuffer.text; }
  if (mode.value !== 'pipe') {
    if (terminalScreen.value) terminalScreen.value.write(chunk);
    else {
      screenPending += chunk;
      if (screenPending.length > 160_000) screenPending = '\r\n[输出已截断]\r\n' + screenPending.slice(-150_000);
    }
  }
  cursor = page.cursor;
  if (timelineOpen && Date.now() - lastLifecycleRefresh >= 1500) void refreshLifecycle();
  void nextTick(() => { if (outputEl.value) outputEl.value.scrollTop = outputEl.value.scrollHeight; });
}
function pausePolling() { if (pollTimer) clearInterval(pollTimer); pollTimer = undefined; }
function pauseTransport() {
  transportVersion++;
  connecting?.abort();
  connecting = undefined;
  if (reconnectTimer) clearTimeout(reconnectTimer);
  reconnectTimer = undefined;
  stream?.close();
  stream = undefined;
  streaming.value = false;
  pausePolling();
}
function scheduleReconnect() {
  if (disposed || !active || !terminalId.value || !running.value || reconnectTimer) return;
  const delay = Math.min(20_000, 1000 * 2 ** Math.min(reconnectAttempts++, 5));
  reconnectTimer = setTimeout(() => { reconnectTimer = undefined; void startStream(); }, delay);
}
async function startStream() {
  if (disposed || !active || !terminalId.value || stream || connecting || !running.value) return;
  const connection = new AbortController();
  connecting = connection;
  const id = terminalId.value;
  const version = ++transportVersion;
  try {
    const connected = await connectTerminalStream(id, conversation, cursor, (snap) => {
      if (disposed || !active || id !== terminalId.value || version !== transportVersion) return;
      consume(snap);
      error.value = '';
      if (!snap.running) pauseTransport();
    }, () => {
      if (disposed || version !== transportVersion || id !== terminalId.value) return;
      stream = undefined;
      streaming.value = false;
      resumePolling();
      scheduleReconnect();
    }, connection.signal);
    if (disposed || !active || id !== terminalId.value || version !== transportVersion) { connected.close(); return; }
    stream = connected;
    streaming.value = true;
    reconnectAttempts = 0;
    pausePolling();
  } catch (error) {
  console.error("[original-error]", error);
if (disposed || !active || id !== terminalId.value || version !== transportVersion) return;
    streaming.value = false;
    resumePolling();
    scheduleReconnect();
  } finally {
    if (connecting === connection) connecting = undefined;
    if (!disposed && active && id === terminalId.value && version !== transportVersion && !stream && !reconnectTimer && !connecting) scheduleReconnect();
  }
}
function resumeTransport() {
  if (disposed || !active || !terminalId.value || !running.value) return;
  resumePolling();
  void startStream();
}
function screenReady() {
  if (screenPending && terminalScreen.value) { terminalScreen.value.write(screenPending); screenPending = ''; }
}
function applyAutoSize(rows: number, cols: number) {
  if (!autoFit.value || !terminalId.value || mode.value === 'pipe' || disposed) return;
  if (rows === screenRows.value && cols === screenColumns.value) return;
  screenRows.value = rows;
  screenColumns.value = cols;
  terminalScreen.value?.resize(cols, rows);
  if (autoSizeTimer) clearTimeout(autoSizeTimer);
  autoSizeTimer = setTimeout(() => {
    autoSizeTimer = undefined;
    const id = terminalId.value;
    if (!id || !active || disposed) return;
    if (stream?.resize(rows, cols)) return;
    void terminalApi.resize(id, conversation, rows, cols).catch(cause => { if (id === terminalId.value) error.value = failure(cause); });
  }, 100);
}
function sendScreenInput(value: string) { send(value); }
function resetScreen() { screenPending = ''; terminalScreen.value?.reset(); }
async function resizeTerminal() {
  if (!terminalId.value || mode.value === 'pipe') return;
  const [rows, cols] = dimensions.value.split('x').map(Number);
  screenRows.value = rows; screenColumns.value = cols;
  terminalScreen.value?.resize(cols, rows);
  if (stream?.resize(rows, cols)) return;
  try { await terminalApi.resize(terminalId.value, conversation, rows, cols); }
  catch (cause) { error.value = failure(cause); }
}
function resumePolling() { if (disposed || !active || streaming.value || pollTimer || !terminalId.value) return; pollTimer = setInterval(() => void poll(), 600); void poll(); }
async function poll() {
  if (polling || !terminalId.value || disposed) return;
  polling = true;
  const current = terminalId.value;
  try {
    const snap = await terminalApi.read(current, conversation, cursor);
    if (current !== terminalId.value || disposed) return;
    consume(snap);
    error.value = '';
    if (!snap.running) pausePolling();
    if (timelineOpen) void refreshLifecycle();
  } catch (cause) {
    if (current !== terminalId.value || disposed) return;
    error.value = failure(cause);
    const status = (cause as any)?.response?.status;
    if (status === 403 || status === 404) { terminalId.value = ''; pausePolling(); }
  } finally { polling = false; }
}
async function refreshSessions() {
  try { sessions.value = await terminalApi.listSessions(conversation); }
  catch (error) {
  console.error("[original-error]", error);
sessions.value = []; }
}
async function resumeSession() {
  if (!resumeId.value || terminalId.value || busy.value) return;
  busy.value = true;
  error.value = '';
  try {
    const snap = await terminalApi.read(resumeId.value, conversation, 0);
    if (disposed) return;
    if (props.tab.taskId && snap.taskId !== props.tab.taskId) throw new Error("终端任务绑定身份不一致");
    terminalId.value = snap.id;
    output.value = ''; cursor = 0; terminalExited = false; exitCode.value = -1; clearLifecycle(); decoder = new TextDecoder(); textBuffer.reset(); resetScreen();
    consume(snap); resumeTransport();
  } catch (cause) { error.value = failure(cause); await refreshSessions(); }
  finally { busy.value = false; }
}
async function open() {
  if (!workspaceUri.value || busy.value || terminalId.value) return;
  busy.value = true; error.value = '';
  try {
    const snap = await terminalApi.open(conversation, workspaceUri.value, props.tab.taskId || "");
    if (disposed) return;
    terminalId.value = snap.id;
    output.value = ''; cursor = 0; terminalExited = false; exitCode.value = -1; clearLifecycle(); decoder = new TextDecoder(); textBuffer.reset(); resetScreen();
    consume(snap); resumeTransport();
  } catch (cause) { error.value = failure(cause); }
  finally { busy.value = false; }
}
function send(value: string) {
  if (!terminalId.value || !running.value || !value) return;
  if (stream?.input(value)) return;
  const id = terminalId.value;
  inputQueue = inputQueue.catch((error) => {
  console.error("[original-error]", error);
  return undefined;
}).then(async () => {
    if (id !== terminalId.value || disposed) return;
    try { await terminalApi.input(id, conversation, value); }
    catch (cause) { if (id === terminalId.value) error.value = failure(cause); }
  });
}
function sendLine() { const value = line.value; if (!value.trim()) return; const newline = mode.value === 'pipe' ? '\n' : '\r'; if (new TextEncoder().encode(value + newline).length > 4096) { error.value = '单次输入不得超过 4096 字节'; return; } line.value = ''; send(value + newline); }
function interrupt() { send('\x03'); }
function focusTerminal(event: MouseEvent) { (event.currentTarget as HTMLElement)?.focus(); }
function handleKeyboard(event: KeyboardEvent) {
  if (!terminalId.value || !running.value || busy.value || event.isComposing) return;
  const data = keyToTerminalData(event);
  if (data === null) return;
  event.preventDefault();
  send(data);
}
async function downloadTranscript() {
  if (!terminalId.value || !window.confirm("终端输出可能包含访问令牌、密码或其他敏感信息。确定导出当前缓冲区内容？")) return;
  const id = terminalId.value;
  try {
    const transcript = await terminalApi.exportTranscript(id, conversation);
    if (id !== terminalId.value) return;
    const blob = new Blob([JSON.stringify({ format: "amitia.terminal.transcript.v1", warning: "仅包含被保留的终端输出字节。摘要不是完整会话审计凭据，也不是自动化测试通过证明。", ...transcript }, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    try {
      const link = document.createElement("a");
      link.href = url; link.download = `terminal-${id.slice(0, 12)}.json`;
      link.click();
    } finally { setTimeout(() => URL.revokeObjectURL(url), 1000); }
  } catch (cause) { error.value = failure(cause); }
}
async function closeTerminal() {
  pauseTransport();
  const id = terminalId.value;
  terminalId.value = ''; running.value = false; terminalExited = true; clearLifecycle();
  if (id) { try { await terminalApi.close(id, conversation); } catch (cause) { error.value = failure(cause); } }
  await refreshSessions();
}
onMounted(async () => {
  try { mounts.value = await terminalApi.listMounts(); if (!workspaceUri.value && mounts.value.length === 1) workspaceUri.value = mounts.value[0].rootUri; }
  catch (cause) { error.value = failure(cause); }
  await refreshSessions();
});
onActivated(() => { active = true; resumeTransport(); });
onDeactivated(() => { active = false; pauseTransport(); });
onBeforeUnmount(() => {
  disposed = true; pauseTransport();
  if (autoSizeTimer) clearTimeout(autoSizeTimer);
});
</script>
<style scoped>
.terminal-shell{display:flex;flex-direction:column;height:100%;min-height:0;background:#14171b;color:#e1e6ee;font-size:12px}.terminal-header{display:flex;align-items:center;gap:8px;padding:9px;border-bottom:1px solid #40454c;flex-wrap:wrap}.terminal-header span{font-weight:600}.terminal-header small{font-weight:400;color:#a2aeba}.terminal-header select{min-width:110px;flex:1;max-width:280px}.terminal-shell button,.terminal-shell select,.terminal-shell input{border:1px solid #505865;background:#222831;color:#e1e6ee;border-radius:5px;padding:6px}.terminal-shell button{cursor:pointer}.terminal-shell button:disabled{opacity:.45;cursor:not-allowed}.terminal-output{flex:1;min-height:0;overflow:auto;padding:10px;user-select:text}.terminal-output pre{white-space:pre-wrap;overflow-wrap:anywhere;margin:0;font-family:ui-monospace,Consolas,monospace;font-size:12px;line-height:1.55}.terminal-input{display:flex;gap:6px;padding:8px;border-top:1px solid #40454c}.terminal-input input{flex:1;min-width:0;font-family:ui-monospace,Consolas,monospace}.terminal-footer{padding:5px 10px;font-size:11px;color:#9ba8b8}.terminal-error{padding:6px 10px;color:#ffc2b9}.terminal-dimensions{display:flex;gap:5px;align-items:center}.terminal-dimensions select{min-width:95px}.terminal-transport{font-size:11px;color:#a1c8ab}
.terminal-association{display:flex;gap:10px;align-items:center;justify-content:space-between;padding:6px 10px;border-bottom:1px solid #40454c;color:#a4c8c9;overflow-wrap:anywhere}.terminal-association button{white-space:nowrap}.terminal-hint{margin:0;padding:4px 10px;color:#9aa6b5;font-size:11px}
.terminal-output:focus-visible{outline:2px solid #789ac5;outline-offset:-2px}
.terminal-receipts{padding:6px 10px;border-bottom:1px solid #40454c;display:flex;gap:8px;align-items:center;flex-wrap:wrap}.terminal-receipts details{width:100%;max-height:160px;overflow:auto}.terminal-receipts summary{cursor:pointer}.terminal-receipts ol{padding-left:20px}.terminal-receipts small{color:#d1ae7f}
.terminal-timeline{border-bottom:1px solid #40454c;padding:7px 12px;max-height:130px;overflow:auto}.terminal-timeline summary{cursor:pointer;font-size:11px;color:#adb8c7}.terminal-timeline ol{margin:7px 0;padding-left:20px;font-size:11px;color:#cbd5e1}.terminal-timeline-warning{color:#ffd48b;font-size:11px}
</style>
