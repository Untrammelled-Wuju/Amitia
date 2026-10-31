<template>
  <div class="browser-view">
    <div class="browser-toolbar">
      <button type="button" :disabled="remote || busy || !running || (observed && !canControl)" title="后退" @click="action('back')">←</button>
      <button type="button" :disabled="remote || busy || !running || (observed && !canControl)" title="前进" @click="action('forward')">→</button>
      <input v-model="address" :readonly="remote || (observed && !canControl)" aria-label="浏览器地址" placeholder="https://example.com" @keydown.enter="navigate" />
      <button type="button" :disabled="remote || busy || (observed && !canControl)" @click="navigate">{{ previewMode ? '预览' : running ? '访问' : '启动浏览器' }}</button>
      <button type="button" :disabled="remote || busy || !running || (observed && !canControl)" title="刷新" @click="action('reload')">↻</button>
    </div>
    <div class="browser-status" role="status">
      <span>{{ remote ? '跨设备 Agent Chromium · 只读观察' : observed ? (canControl ? 'Agent Chromium · 手动控制中（临时）' : 'Agent Chromium · 同页只读观察') : running ? '已连接后端 Chromium · CDP 截图' : '浏览器实例未启动' }}</span>
      <button v-if="running && observed && !remote" type="button" :disabled="busy" @click="control(canControl ? 'release' : 'acquire')">{{ canControl ? '释放控制' : '申请控制' }}</button>
      <button v-if="running" type="button" @click="loadDOM">{{ showDOM ? '关闭 DOM' : '查看 DOM' }}</button>
      <button v-if="!observed && !remote" type="button" @click="togglePreview">{{ previewMode ? '切到 Chromium' : '轻量网页预览' }}</button>
      <button v-if="running" type="button" @click="stop">{{ observed ? '退出观察' : '结束会话' }}</button>
      <button v-if="explicitlyClosed" type="button" @click="reconnect">重新连接</button>
    </div>
    <div v-if="error" class="browser-error" role="alert">{{ error }}</div>
    <div v-if="previewMode" class="browser-viewport">
      <div class="browser-placeholder">独立 iframe 预览（沙箱模式），部分网站不允许被嵌入；此模式不提供 Agent 操作能力。</div>
      <iframe v-if="previewURL" :src="previewURL" title="轻量网页预览" sandbox="allow-forms allow-scripts" referrerpolicy="no-referrer" />
      <button type="button" class="browser-external" @click="openSystemBrowser">使用系统浏览器打开</button>
    </div>
    <div v-else-if="!running" class="browser-placeholder">启动后可通过真实 Browser Runtime 访问网页。当前为可观察的独立 Chromium 会话，不会自动接管 Agent 已有的浏览器会话。</div>
    <div v-if="running && !previewMode && !remote && (!observed || canControl)" class="browser-input">
      <input v-model="inputText" aria-label="输入到浏览器当前焦点" placeholder="向网页当前焦点输入文字" @keydown.enter="sendText" />
      <button type="button" :disabled="busy || !inputText" @click="sendText">输入</button>
      <button type="button" :disabled="busy" @click="sendScroll(-480)">向上</button>
      <button type="button" :disabled="busy" @click="sendScroll(480)">向下</button>
    </div>
    <div v-if="!previewMode && running && showDOM" class="browser-dom"><pre>{{ domContent || '正在读取页面 DOM…' }}</pre></div>
    <div v-else-if="!previewMode && running" class="browser-viewport">
      <img v-if="frameURL" :src="frameURL" alt="Chromium 浏览器当前页面截图，点击可操作" draggable="false" @click="clickFrame" @wheel.prevent="wheelFrame" />
      <div v-else class="browser-placeholder">正在等待浏览器截图…</div>
    </div>
  </div>
</template>
<script setup lang="ts">
import { ref, computed, onActivated, onDeactivated, onBeforeUnmount, watch } from 'vue';
import { apiClient } from '@/composables/useApi';
import { useWorkbenchStore, type WorkbenchTab } from '@/stores/workbench';
import { classifyRemoteBrowserFailure } from './remoteBrowserFailure';

const props = defineProps<{ tab: WorkbenchTab }>();
const store = useWorkbenchStore();
const conversationId = store.conversationId;
const address = ref(props.tab.resourceUri);
const running = ref(false);
const busy = ref(false);
const error = ref('');
const frameURL = ref('');
const showDOM = ref(false);
const domContent = ref('');
const inputText = ref('');
const previewMode = ref(false);
const remote = Boolean(props.tab.targetDeviceId);
const observed = ref(Boolean(props.tab.browserRef));
const canControl = ref(false);
const leaseId = ref('');
const explicitlyClosed = ref(false);
let consecutiveFailures = 0;
let nextRetryAt = 0;
const retryDelay = () => Math.min(30000, 1500 * 2 ** Math.min(5, consecutiveFailures));
function recordFailure(cause: unknown) {
  const failure = remote || (observed.value && running.value) ? classifyRemoteBrowserFailure(cause) : { terminal: false, message: explanation(cause) };
  error.value = failure.terminal ? `${failure.message}；请重新获取浏览器观察授权` : failure.message;
  running.value = false;
  if (remote || failure.terminal) { replaceFrame(''); domContent.value = ''; showDOM.value = false; canControl.value = false; leaseId.value = ''; }
  if (failure.terminal) { explicitlyClosed.value = true; if (timer) { clearInterval(timer); timer = null; } return; }
  consecutiveFailures = Math.min(7, consecutiveFailures + 1);
  nextRetryAt = Date.now() + retryDelay();
}
function reconnect() {
  explicitlyClosed.value = false;
  consecutiveFailures = 0;
  nextRetryAt = 0;
  error.value = '';
  resume();
}
const previewURL = computed(() => { try { const url = new URL(address.value); return ['http:', 'https:'].includes(url.protocol) ? url.toString() : ''; } catch (error) {
  console.error("[original-error]", error);
return ''; } });
function togglePreview() { previewMode.value = !previewMode.value; if (!previewMode.value) void refreshFrame(); }
function openSystemBrowser() { if (previewURL.value) window.open(previewURL.value, '_blank', 'noopener,noreferrer'); }
let active = true;
let timer: ReturnType<typeof setInterval> | null = null;
let pendingFrameGeneration = -1;
let frameTick = 0;
let revision = 0;
let lifecycleAbort = new AbortController();
async function remoteRead(operation: 'state' | 'screenshot' | 'dom') {
  if (!remote || !props.tab.browserRef) throw new Error('远程浏览器缺少观察引用');
  const response = await apiClient.post('/api/device-mesh/v1/workbench/browser/read', {
    targetDeviceId: props.tab.targetDeviceId, conversationId, browserRef: props.tab.browserRef, operation,
  }, { timeout: 16000, signal: lifecycleAbort.signal });
  if (response.data?.ready !== true || response.data?.observed !== true) throw new Error('远程观察许可已失效');
  return response.data;
}
const baseURL = () => `/api/workbench/browser/${encodeURIComponent(conversationId)}/${encodeURIComponent(props.tab.id)}`;
const explanation = (cause: unknown) => String((cause as any)?.response?.data?.error || (cause as Error)?.message || cause);
function replaceFrame(next: string) { const previous = frameURL.value; frameURL.value = next; if (previous) URL.revokeObjectURL(previous); }
async function refreshFrame() {
  if (!active || store.conversationId !== conversationId || !running.value || pendingFrameGeneration === revision || showDOM.value || previewMode.value) return;
  const current = revision;
  pendingFrameGeneration = current;
  try {
    if (remote) {
      const response = await remoteRead('screenshot');
      const encoded = String(response.imageBase64 || '');
      if (!encoded || encoded.length > 3 * 1024 * 1024 || !/^[A-Za-z0-9+/=]+$/.test(encoded)) throw new Error('远程截图数据无效');
      const raw = atob(encoded);
      if (raw.length > 2 * 1024 * 1024) throw new Error('远程截图超过上限');
      const bytes = Uint8Array.from(raw, c => c.charCodeAt(0));
      if (current === revision && active) replaceFrame(URL.createObjectURL(new Blob([bytes], { type: 'image/jpeg' })));
    } else {
      const response = await apiClient.get(`${baseURL()}/screenshot`, { responseType: 'blob', timeout: 15000 });
      if (current === revision && active && response.data instanceof Blob && response.data.type.startsWith('image/')) {
        replaceFrame(URL.createObjectURL(response.data));
      }
    }
  } catch (cause) {
    if (current === revision && active) {
      if (remote) recordFailure(cause);
      else error.value = explanation(cause);
    }
  }
  finally { if (pendingFrameGeneration === current) pendingFrameGeneration = -1; }
}
async function start() {
  if (busy.value || explicitlyClosed.value || !active) return;
  const current = revision;
  busy.value = true; error.value = '';
  try {
    const response = remote
      ? { data: await remoteRead('state') }
      : props.tab.browserRef
      ? await apiClient.post('/api/workbench/browser/observe', { conversationId, viewId: props.tab.id, browserRef: props.tab.browserRef }, { signal: lifecycleAbort.signal })
      : await apiClient.post('/api/workbench/browser/open', { conversationId, viewId: props.tab.id, url: address.value.trim() }, { signal: lifecycleAbort.signal });
    if (current !== revision || !active || explicitlyClosed.value) return;
    running.value = response.data?.ready === true;
    observed.value = response.data?.observed === true || Boolean(props.tab.browserRef);
    canControl.value = response.data?.canControl === true; leaseId.value = canControl.value ? String(response.data?.leaseId || '') : '';
    if (response.data?.url) store.updateTab(props.tab.id, { resourceUri: response.data.url, title: response.data.title || new URL(response.data.url).hostname });
    if (running.value) { consecutiveFailures = 0; nextRetryAt = 0; await refreshFrame(); }
  } catch (cause) {
    if (current === revision && active) {
      recordFailure(cause);
      if (!props.tab.browserRef && !remote) previewMode.value = true;
    }
  } finally { busy.value = false; }
}
async function navigate() {
  if (remote) return;
  if (observed.value && !canControl.value) { if (!running.value) await start(); return; }
  if (previewMode.value) { if (!previewURL.value) error.value = '请输入有效的 HTTP(S) 网页地址'; else store.updateTab(props.tab.id, { resourceUri: previewURL.value, title: new URL(previewURL.value).hostname }); return; }
  if (!running.value) { await start(); return; }
  if (busy.value) return;
  busy.value = true; error.value = '';
  const requestRevision = revision;
  const requestToken = observed.value ? leaseId.value : '';
  try {
    const response = await apiClient.post(`${baseURL()}/navigate`, { url: address.value.trim(), ...(observed.value ? { leaseId: requestToken } : {}) }, { signal: lifecycleAbort.signal });
    if (!active || revision !== requestRevision || (observed.value && leaseId.value !== requestToken)) return;
    const url = response.data?.url || address.value;
    store.updateTab(props.tab.id, { resourceUri: url, title: response.data?.title || new URL(url).hostname });
    address.value = url;
    await refreshFrame();
  } catch (cause) { if (active && revision === requestRevision && (!observed.value || leaseId.value === requestToken)) error.value = explanation(cause); }
  finally { busy.value = false; }
}
async function action(name: 'back' | 'forward' | 'reload') {
  if (remote) return;
  if (busy.value || !running.value || (observed.value && !canControl.value)) return;
  busy.value = true; error.value = '';
  const requestRevision = revision;
  const requestToken = observed.value ? leaseId.value : '';
  try {
    const response = await apiClient.post(`${baseURL()}/action`, { action: name, ...(observed.value ? { leaseId: requestToken } : {}) }, { signal: lifecycleAbort.signal });
    if (!active || revision !== requestRevision || (observed.value && leaseId.value !== requestToken)) return;
    if (response.data?.url) { address.value = response.data.url; store.updateTab(props.tab.id, { resourceUri: address.value, title: response.data.title || new URL(address.value).hostname }); }
    await refreshFrame();
  } catch (cause) { if (active && revision === requestRevision && (!observed.value || leaseId.value === requestToken)) error.value = explanation(cause); }
  finally { busy.value = false; }
}
async function loadDOM() {
  showDOM.value = !showDOM.value;
  if (!showDOM.value) { void refreshFrame(); return; }
  const current = revision;
  try {
    const data = remote ? await remoteRead('dom') : (await apiClient.get(`${baseURL()}/dom`, { signal: lifecycleAbort.signal })).data;
    if (current === revision && active && showDOM.value) domContent.value = String(data?.content || '');
  } catch (cause) {
    if (current === revision && active) {
      if (remote) recordFailure(cause); else error.value = explanation(cause);
    }
  }
}
async function browserInput(body: Record<string, unknown>) {
  if (remote) return false;
  if (!running.value || busy.value || (observed.value && !canControl.value)) return;
  busy.value = true; error.value = '';
  const requestRevision = revision;
  const requestToken = observed.value ? leaseId.value : '';
  try {
    await apiClient.post(`${baseURL()}/input`, { ...body, ...(observed.value ? { leaseId: requestToken } : {}) }, { signal: lifecycleAbort.signal });
    if (!active || revision !== requestRevision || (observed.value && leaseId.value !== requestToken)) return false;
    await refreshFrame(); return true;
  }
  catch (cause) { if (active && revision === requestRevision && (!observed.value || leaseId.value === requestToken)) error.value = explanation(cause); return false; }
  finally { busy.value = false; }
}
function clickFrame(event: MouseEvent) {
  if (remote) return;
  if (observed.value && !canControl.value) return;
  const img = event.currentTarget as HTMLImageElement;
  const bounds = img.getBoundingClientRect();
  if (bounds.width < 1 || bounds.height < 1) return;
  const x = (event.clientX - bounds.left) * img.naturalWidth / bounds.width;
  const y = (event.clientY - bounds.top) * img.naturalHeight / bounds.height;
  void browserInput({ action: 'click', x, y });
}
function wheelFrame(event: WheelEvent) { if (remote) return; if (observed.value && !canControl.value) return; void browserInput({ action: 'scroll', amount: Math.max(-1200, Math.min(1200, event.deltaY)) }); }
function sendText() { const text = inputText.value; if (!text) return; void browserInput({ action: 'text', text }).then(ok => { if (ok) inputText.value = ''; }); }
function sendScroll(amount: number) { void browserInput({ action: 'scroll', amount }); }
async function control(mode: 'acquire' | 'renew' | 'release') {
  if (remote || !running.value || !observed.value || busy.value) return;
  const requestLeaseId = leaseId.value;
  if (mode !== 'acquire' && !requestLeaseId) { canControl.value = false; return; }
  const current = revision;
  if (mode !== 'renew') { busy.value = true; error.value = ''; }
  try {
    const response = await apiClient.post(`${baseURL()}/control`, { action: mode, ...(mode !== 'acquire' ? { leaseId: requestLeaseId } : {}) }, { signal: lifecycleAbort.signal });
    if (!active || current !== revision || (mode !== 'acquire' && leaseId.value !== requestLeaseId)) {
      if (mode === 'acquire' && response.data?.leaseId) void apiClient.post(`${baseURL()}/control`, { action: 'release', leaseId: response.data.leaseId }).catch((error) => {
  console.error("[original-error]", error);
});
      return;
    }
    canControl.value = response.data?.canControl === true;
    leaseId.value = canControl.value ? String(response.data?.leaseId || '') : '';
  } catch (cause) {
    if (active && current === revision && (mode === 'acquire' || leaseId.value === requestLeaseId)) {
      canControl.value = false; leaseId.value = ''; error.value = explanation(cause);
    }
  } finally { if (mode !== 'renew') busy.value = false; }
}
async function stop() {
  revision++;
  lifecycleAbort.abort();
  lifecycleAbort = new AbortController();
  explicitlyClosed.value = true;
  nextRetryAt = 0;
  if (timer) { clearInterval(timer); timer = null; }
  running.value = false;
  canControl.value = false;
  leaseId.value = '';
  replaceFrame('');
  if (remote) return;
  try { await apiClient.delete(baseURL()); }
  catch (cause) { error.value = explanation(cause); }
}
function pause() {
  revision++;
  active = false;
  lifecycleAbort.abort();
  lifecycleAbort = new AbortController();
  const token = leaseId.value;
  canControl.value = false;
  leaseId.value = '';
  if (token && !remote) void apiClient.post(`${baseURL()}/control`, { action: 'release', leaseId: token }).catch((error) => {
  console.error("[original-error]", error);
});
  if (timer) { clearInterval(timer); timer = null; }
}
async function syncObservedState() {
  if (!active || !running.value || !observed.value) return;
  const current = revision;
  try {
    const response = remote ? { data: await remoteRead('state') } : await apiClient.get(baseURL(), { signal: lifecycleAbort.signal });
    if (!active || current !== revision || !response.data?.observed) return;
    canControl.value = response.data?.canControl === true;
    leaseId.value = canControl.value ? String(response.data?.leaseId || '') : '';
    const nextURL = String(response.data?.url || '');
    if (nextURL.startsWith('http://') || nextURL.startsWith('https://')) {
      address.value = nextURL;
      store.updateTab(props.tab.id, { resourceUri: nextURL, title: response.data.title || new URL(nextURL).hostname });
    }
  } catch (cause) {
    if (!active || current !== revision) return;
    recordFailure(cause);
  }
}
function resume() {
  active = store.conversationId === conversationId;
  if (!active || explicitlyClosed.value) return;
  if (!timer) timer = setInterval(() => {
    if (running.value) void refreshFrame();
    else if (!busy.value && Date.now() >= nextRetryAt && (remote || props.tab.browserRef)) void start();
    frameTick++;
    if (running.value && frameTick % 4 === 0) void syncObservedState();
    if (canControl.value && frameTick % 7 === 0) void control('renew');
  }, remote ? 4200 : 2600);
  if (!running.value && !busy.value && Date.now() >= nextRetryAt) {
    const current = revision;
    void (remote ? remoteRead('state').then(data => ({ data })) : apiClient.get(baseURL(), { signal: lifecycleAbort.signal })).then(response => {
      if (!active || current !== revision || explicitlyClosed.value) return;
      if (props.tab.browserRef && response.data?.observed !== true) { void start(); return; }
      consecutiveFailures = 0; nextRetryAt = 0;
      running.value = response.data?.ready === true;
      observed.value = response.data?.observed === true || Boolean(props.tab.browserRef);
      canControl.value = response.data?.canControl === true;
      leaseId.value = canControl.value ? String(response.data?.leaseId || '') : '';
      if (running.value) void refreshFrame();
    }).catch(cause => { if (active && current === revision && !explicitlyClosed.value) { recordFailure(cause); } });
  } else void refreshFrame();
}
watch(() => props.tab.resourceUri, value => { if (value && value !== address.value) address.value = value; });
onActivated(resume);
onDeactivated(pause);
onBeforeUnmount(() => { pause(); replaceFrame(''); });
resume();
</script>
<style scoped>
.browser-view{display:flex;flex-direction:column;height:100%;min-height:0;gap:8px;padding:12px;box-sizing:border-box;color:var(--el-text-color-primary)}.browser-toolbar{display:flex;gap:5px}.browser-toolbar input{flex:1;min-width:0;padding:8px;background:var(--workbench-bg);color:inherit;border:1px solid var(--surface-border);border-radius:6px}.browser-toolbar button,.browser-status button{background:transparent;border:1px solid var(--surface-border);border-radius:6px;color:inherit;padding:6px 9px;cursor:pointer}.browser-toolbar button:disabled{opacity:.4;cursor:not-allowed}.browser-status{display:flex;align-items:center;gap:8px;font-size:11px;color:var(--el-text-color-secondary)}.browser-status span{flex:1}.browser-error{color:var(--el-color-danger);font-size:12px}.browser-placeholder{padding:24px;font-size:12px;color:var(--el-text-color-secondary);text-align:center}.browser-viewport{flex:1;min-height:0;overflow:auto;background:var(--el-fill-color-light);border-radius:7px;text-align:center}.browser-viewport img{max-width:100%;height:auto;display:block;margin:auto;cursor:crosshair}.browser-viewport iframe{width:100%;height:75%;min-height:320px;border:none}.browser-external{margin:8px;padding:8px}.browser-input{display:flex;gap:6px}.browser-input input{flex:1;min-width:0;padding:7px;border:1px solid var(--surface-border);border-radius:6px;background:var(--workbench-bg);color:inherit}.browser-input button{border:1px solid var(--surface-border);border-radius:6px;background:transparent;color:inherit;cursor:pointer}.browser-dom{flex:1;min-height:0;overflow:auto;background:var(--el-fill-color-light);padding:12px}.browser-dom pre{white-space:pre-wrap;overflow-wrap:anywhere;font-size:12px}
</style>
