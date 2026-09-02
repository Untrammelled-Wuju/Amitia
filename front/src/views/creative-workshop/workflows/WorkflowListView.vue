<template>
  <main class="workflow-list-page" v-loading="loading">
    <header class="page-header">
      <div>
        <div class="crumb"><router-link to="/creative-workshop">创意工坊</router-link><span>/</span><span>工作流</span></div>
        <h1>工作流</h1>
        <p>统一管理当前设备、云端以及其他已绑定设备上的 workflow-v2。</p>
      </div>
      <div class="header-actions">
        <input ref="importInput" class="hidden-input" type="file" accept="application/json,.json" @change="handleImport" />
        <el-button :disabled="currentTarget.location === 'device'" @click="importInput?.click()">导入 JSON</el-button>
        <el-button :disabled="currentTarget.location === 'device'" @click="openTemplates">我的模板</el-button>
        <el-button :disabled="currentTarget.location === 'device'" :loading="aiCreating" @click="createWithAI">AI 创建</el-button>
        <el-button type="primary" :disabled="currentTarget.location === 'device' && !deviceId" @click="createNew"><el-icon><Plus /></el-icon>新建</el-button>
      </div>
    </header>

    <section class="management-bar">
      <el-radio-group v-model="location" size="small" @change="handleTargetChange">
        <el-radio-button value="local">当前设备</el-radio-button>
        <el-radio-button value="cloud">云端</el-radio-button>
        <el-radio-button value="device">我的设备</el-radio-button>
      </el-radio-group>
      <el-select v-if="location === 'device'" v-model="deviceId" class="device-filter" placeholder="选择设备" @change="handleTargetChange">
        <el-option v-for="device in devices" :key="device.deviceId" :value="device.deviceId" :label="`${device.label || device.deviceId}${device.online ? ' · 在线' : ' · 离线'}`" />
      </el-select>
      <el-input v-model="search" clearable placeholder="搜索名称或描述" class="search-box" />
      <el-select v-model="statusFilter" class="status-filter">
        <el-option label="全部" value="all" />
        <el-option label="已启用" value="enabled" />
        <el-option label="已停用" value="disabled" />
        <el-option label="允许 AI 调用" value="agent" />
      </el-select>
      <span class="count-text">{{ filteredWorkflows.length }} / {{ workflows.length }}</span>
      <div v-if="selectedIds.size" class="batch-actions">
        <span>已选 {{ selectedIds.size }}</span>
        <el-button size="small" @click="batchSetEnabled(true)">批量启用</el-button>
        <el-button size="small" @click="batchSetEnabled(false)">批量停用</el-button>
        <el-button size="small" type="danger" plain @click="batchDelete">批量删除</el-button>
        <el-button size="small" text @click="clearSelection">取消选择</el-button>
      </div>
    </section>

    <section v-if="filteredWorkflows.length" class="workflow-grid">
      <article v-for="item in filteredWorkflows" :key="item.id" class="workflow-card" :class="{ selected: selectedIds.has(item.id) }">
        <div class="card-top">
          <el-checkbox :model-value="selectedIds.has(item.id)" :disabled="item.offline" @change="(v) => toggleSelected(item.id, Boolean(v))" />
          <div class="workflow-icon"><el-icon><Share /></el-icon></div>
          <div class="title-wrap">
            <h2>{{ item.name }}</h2>
            <p>{{ item.description || "未填写描述" }}</p>
          </div>
          <el-switch :model-value="item.enabled" :disabled="item.offline" @change="(v) => toggle(item, Boolean(v))" />
        </div>
        <div class="stats">
          <span>{{ item.nodes?.length || 0 }} 节点</span>
          <span>{{ item.edges?.length || 0 }} 连线</span>
          <span>{{ item.triggers?.filter(t => t.enabled).length || 0 }} 活动触发器</span>
          <span v-if="item.callableByAgent">Agent Tool</span>
          <span>v{{ item.version || "1.0.0" }}</span>
          <span v-if="item.installation">rev {{ item.installation.revision }}</span>
          <span v-if="item.cached">离线目录缓存</span>
        </div>
        <div class="card-actions">
          <el-button type="primary" plain @click="edit(item)">打开编辑器</el-button>
          <el-button :disabled="!item.enabled || item.offline" @click="openRunDialog(item)"><el-icon><VideoPlay /></el-icon>运行</el-button>
          <el-dropdown trigger="click">
            <el-button><el-icon><MoreFilled /></el-icon></el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item v-if="currentTarget.location !== 'device'" @click="duplicate(item)">复制</el-dropdown-item>
                <el-dropdown-item v-if="currentTarget.location !== 'device'" @click="saveAsTemplate(item)">保存为模板</el-dropdown-item>
                <el-dropdown-item v-if="currentTarget.location !== 'device'" @click="exportItem(item)">导出 JSON</el-dropdown-item>
                <el-dropdown-item divided :disabled="item.offline" @click="remove(item)">删除</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </article>
    </section>

    <section v-else class="empty-card">
      <el-icon><Share /></el-icon>
      <h2>{{ workflows.length ? "没有匹配的工作流" : "还没有工作流" }}</h2>
      <p>{{ workflows.length ? "调整搜索或筛选条件。" : "可以从空白工作流、AI 或我的模板开始。" }}</p>
      <div v-if="!workflows.length" class="empty-actions">
        <el-button v-if="currentTarget.location !== 'device'" @click="openTemplates">我的模板</el-button>
        <el-button v-if="currentTarget.location !== 'device'" :loading="aiCreating" @click="createWithAI">AI 创建</el-button>
        <el-button type="primary" :disabled="currentTarget.location === 'device' && !deviceId" @click="createNew">手动创建</el-button>
      </div>
    </section>

    <el-dialog v-model="runDialogVisible" :title="`运行 · ${runWorkflowItem?.name || '工作流'}`" width="540px" append-to-body destroy-on-close>
      <div class="run-dialog-body">
        <label>执行模式
          <el-select v-model="runMode" style="width:100%">
            <el-option label="Live · 正式执行" value="live" />
            <el-option label="Dry Run · 只验证/规划" value="dry_run" />
            <el-option label="Mocked · 使用显式 Mock" value="mocked" />
            <el-option label="Controlled Live · 副作用前确认" value="controlled_live" />
          </el-select>
        </label>
        <p class="run-mode-help">{{ executionModeDescription(runMode) }}</p>
        <label>Workflow Input (JSON)<el-input v-model="runInputEditor" type="textarea" :rows="6" spellcheck="false" /></label>
        <label v-if="runMode === 'mocked'">Mocks (JSON Array)<el-input v-model="runMocksEditor" type="textarea" :rows="7" spellcheck="false" /></label>
        <p v-if="runMode === 'controlled_live'" class="controlled-warning">运行会在副作用节点前持久化进入 waiting_confirmation；随后会打开运行详情完成确认。</p>
      </div>
      <template #footer><el-button @click="runDialogVisible=false">取消</el-button><el-button type="primary" :loading="runningWorkflow" @click="runNow">开始运行</el-button></template>
    </el-dialog>

    <el-drawer v-model="templateDrawer" title="我的工作流模板" size="min(520px, 92vw)">
      <div class="template-help">模板保存在当前 Amitia 数据库并按用户隔离，不会公开发布。由模板创建的新工作流默认停用自动触发和 Agent 调用，打开编辑器检查后再启用。</div>
      <el-empty v-if="templates.length === 0" description="还没有我的模板" />
      <article v-for="item in templates" :key="item.templateId" class="template-card">
        <div>
          <strong>{{ item.name }}</strong>
          <p>{{ item.description || "未填写描述" }}</p>
          <small>{{ item.nodeCount }} 节点 · {{ item.triggerCount }} 触发器 · {{ formatTime(item.updatedAt) }}</small>
        </div>
        <div class="template-actions">
          <el-button size="small" type="primary" plain @click="useTemplate(item)">使用</el-button>
          <el-button size="small" type="danger" text @click="removeTemplate(item)">删除</el-button>
        </div>
      </article>
    </el-drawer>
  </main>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import { MoreFilled, Plus, Share, VideoPlay } from "@element-plus/icons-vue";
import {
  createWorkflow,
  deleteWorkflow,
  deleteWorkflowTemplate,
  duplicateWorkflow,
  exportWorkflow,
  generateWorkflowWithAI,
  importWorkflow,
  instantiateWorkflowTemplate,
  listWorkflowDevices,
  listWorkflowSyncEvents,
  listWorkflowTemplates,
  listWorkflows,
  runWorkflow,
  saveWorkflowTemplate,
  setWorkflowEnabled,
  type WorkflowDefinition,
  type WorkflowDeviceDescriptor,
  type WorkflowExecutionMode,
  type WorkflowMockBehavior,
  type WorkflowTarget,
  type WorkflowTemplateSummary,
  workflowTargetQuery,
} from "@/api/workflow";

const router = useRouter();
const loading = ref(false);
const aiCreating = ref(false);
const workflows = ref<WorkflowDefinition[]>([]);
const templates = ref<WorkflowTemplateSummary[]>([]);
const devices = ref<WorkflowDeviceDescriptor[]>([]);
const location = ref<"local" | "cloud" | "device">("local");
const deviceId = ref("");
const currentTarget = computed<WorkflowTarget>(() => location.value === "device"
  ? { location: "device", deviceId: deviceId.value }
  : { location: location.value });
const templateDrawer = ref(false);
const importInput = ref<HTMLInputElement | null>(null);
const search = ref("");
const statusFilter = ref<"all" | "enabled" | "disabled" | "agent">("all");
const selectedIds = ref(new Set<string>());
const runDialogVisible = ref(false);
const runningWorkflow = ref(false);
const runWorkflowItem = ref<WorkflowDefinition | null>(null);
const runMode = ref<WorkflowExecutionMode>("live");
const runInputEditor = ref("{}");
const runMocksEditor = ref("[]");
let reconcileTimer: ReturnType<typeof setInterval> | null = null;
let syncCursor: number | null = null;
let syncTargetKey = "";
let syncPolling = false;
let deviceRefreshTicks = 0;

const filteredWorkflows = computed(() => {
  const q = search.value.trim().toLowerCase();
  return workflows.value.filter((item) => {
    if (statusFilter.value === "enabled" && !item.enabled) return false;
    if (statusFilter.value === "disabled" && item.enabled) return false;
    if (statusFilter.value === "agent" && !item.callableByAgent) return false;
    return !q || item.name.toLowerCase().includes(q) || (item.description || "").toLowerCase().includes(q);
  });
});

async function loadDevices() {
  try {
    devices.value = await listWorkflowDevices();
    if (!devices.value.some((item) => item.deviceId === deviceId.value)) deviceId.value = devices.value[0]?.deviceId || "";
  } catch {
    devices.value = [];
  }
}
async function load() {
  if (location.value === "device" && !deviceId.value) { workflows.value = []; return; }
  loading.value = true;
  try { workflows.value = await listWorkflows(currentTarget.value); }
  finally { loading.value = false; }
}
function syncKey(target: WorkflowTarget) {
  return target.location === "device" ? `device:${String(target.deviceId || "")}` : target.location;
}
function snapshotTarget(): WorkflowTarget {
  return currentTarget.value.location === "device"
    ? { location: "device", deviceId: String(currentTarget.value.deviceId || "") }
    : { location: currentTarget.value.location };
}
async function primeSyncCursor(target = snapshotTarget()) {
  const key = syncKey(target);
  try {
    const page = await listWorkflowSyncEvents(target);
    if (syncKey(snapshotTarget()) !== key) return;
    syncCursor = page.cursor;
    syncTargetKey = key;
  } catch {
    if (syncKey(snapshotTarget()) === key) { syncCursor = null; syncTargetKey = ""; }
  }
}
async function pollSyncEvents() {
  if (document.visibilityState !== "visible" || loading.value || syncPolling) return;
  const target = snapshotTarget();
  if (target.location === "device" && !target.deviceId) return;
  const key = syncKey(target);
  syncPolling = true;
  try {
    if (syncCursor === null || syncTargetKey !== key) {
      await primeSyncCursor(target);
      return;
    }
    const page = await listWorkflowSyncEvents(target, syncCursor);
    if (syncKey(snapshotTarget()) !== key) return;
    syncCursor = page.cursor;
    if (page.items.length > 0) await load();
    deviceRefreshTicks += 1;
    if (target.location !== "local" && deviceRefreshTicks % 5 === 0) await loadDevices();
    // Keep a low-frequency catalog reconciliation only as a recovery path for
    // missed/outbox-corruption scenarios; normal synchronization is durable push/pull.
    if (target.location === "device" && deviceRefreshTicks % 8 === 0 && page.items.length === 0) await load();
  } catch {
    // Keep the current list usable during transient sync failures. The cursor is
    // preserved so durable events are retried on the next poll.
  } finally {
    syncPolling = false;
  }
}
async function handleTargetChange() {
  clearSelection();
  syncCursor = null; syncTargetKey = ""; deviceRefreshTicks = 0;
  if (location.value === "device" && !deviceId.value) await loadDevices();
  await load();
  await primeSyncCursor();
}
async function loadTemplates() { templates.value = await listWorkflowTemplates(currentTarget.value); }
async function openTemplates() { if (currentTarget.value.location === "device") return; await loadTemplates(); templateDrawer.value = true; }

async function createWithAI() {
  try {
    const { value } = await ElMessageBox.prompt("描述你想创建的工作流。AI 会直接生成可编辑的 workflow-v2 DAG。", "AI 创建工作流", {
      inputType: "textarea", inputPlaceholder: "例如：每天早上 8 点获取天气，如果下雨就通知我带伞", confirmButtonText: "生成", cancelButtonText: "取消",
    });
    const instruction = String(value || "").trim();
    if (!instruction) return;
    aiCreating.value = true;
    const proposal = await generateWorkflowWithAI(instruction, currentTarget.value);
    const created = await createWorkflow({ ...proposal.definition, definitionHash: undefined }, currentTarget.value);
    if (proposal.warnings?.length) ElMessage.warning(proposal.warnings.join("；"));
    else ElMessage.success(proposal.summary || "AI 工作流已生成");
    await router.push({ path: `/creative-workshop/workflows/${encodeURIComponent(created.id)}`, query: workflowTargetQuery(currentTarget.value) });
  } catch (e: any) {
    if (e === "cancel" || e === "close") return;
    ElMessage.error(e?.response?.data?.error || e?.message || "AI 工作流生成失败");
  } finally { aiCreating.value = false; }
}

async function createNew() {
  if (currentTarget.value.location === "device" && !deviceId.value) { ElMessage.warning("请先选择目标设备"); return; }
  try {
    const def = await createWorkflow({
      name: "未命名工作流", description: "", schemaVersion: "workflow-v2",
      inputSchema: { type: "object" }, outputSchema: {}, nodes: [], edges: [],
      triggers: [{ id: "manual", type: "manual", enabled: true, config: {} }], callableByAgent: false, enabled: true,
    }, currentTarget.value);
    await router.push({ path: `/creative-workshop/workflows/${encodeURIComponent(def.id)}`, query: workflowTargetQuery(currentTarget.value) });
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.error || e?.message || "创建失败");
  }
}
function edit(item: WorkflowDefinition) {
  if (item.offline) { ElMessage.warning("设备离线：当前只能查看最后已知目录，完整 Definition 需要设备上线后读取。"); return; }
  router.push({ path: `/creative-workshop/workflows/${encodeURIComponent(item.id)}`, query: workflowTargetQuery(currentTarget.value) });
}
async function toggle(item: WorkflowDefinition, enabled: boolean) {
  if (item.offline) { ElMessage.warning("设备离线：不能修改本地工作流"); return; }
  try {
    const installation = await setWorkflowEnabled(item.id, enabled, currentTarget.value, item.installation?.revision);
    item.enabled = enabled;
    if (installation && typeof installation === "object" && "revision" in installation) item.installation = installation as any;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.error || e?.message || "更新失败");
    await load();
  }
}
function executionModeDescription(mode: WorkflowExecutionMode) {
  if (mode === "dry_run") return "只执行校验、路由、依赖和条件规划，不调用真实节点 Handler。";
  if (mode === "mocked") return "按显式 Mock 输出运行；副作用节点没有 Mock 时直接阻断。";
  if (mode === "controlled_live") return "真实执行，但副作用节点必须逐 Run 确认后才会继续。";
  return "按正式运行语义执行节点和副作用。";
}
function openRunDialog(item: WorkflowDefinition) {
  if (!item.enabled || item.offline) { if (item.offline) ElMessage.warning("设备离线：不能运行本地工作流"); return; }
  runWorkflowItem.value = item; runMode.value = "live"; runInputEditor.value = "{}"; runMocksEditor.value = "[]"; runDialogVisible.value = true;
}
function parseRunJSON<T>(text: string, label: string): T { try { return JSON.parse(text || "null") as T; } catch (e: any) { throw new Error(`${label} JSON 无效：${e?.message || e}`); } }
async function runNow() {
  const item = runWorkflowItem.value; if (!item || runningWorkflow.value) return;
  runningWorkflow.value = true;
  try {
    const input = parseRunJSON<unknown>(runInputEditor.value, "Input");
    let mocks: WorkflowMockBehavior[] | undefined;
    if (runMode.value === "mocked") { const parsed = parseRunJSON<unknown>(runMocksEditor.value, "Mocks"); if (!Array.isArray(parsed)) throw new Error("Mocks 必须是 JSON Array"); mocks = parsed as WorkflowMockBehavior[]; }
    const result = await runWorkflow(item.id, input, false, currentTarget.value, { mode: runMode.value, mocks });
    runDialogVisible.value = false;
    if (result.status === "waiting_confirmation") {
      ElMessage.warning(`运行等待副作用确认：${result.executionId}`);
      await router.push({ path: `/creative-workshop/workflows/${encodeURIComponent(item.id)}`, query: { ...workflowTargetQuery(currentTarget.value), runId: result.executionId } });
      return;
    }
    ElMessage.success(`${runMode.value === "dry_run" ? "Dry Run 已完成" : "已开始运行"}：${result.executionId}`);
  } catch (e: any) { ElMessage.error(e?.response?.data?.error || e?.message || "运行失败"); }
  finally { runningWorkflow.value = false; }
}
async function duplicate(item: WorkflowDefinition) {
  const created = await duplicateWorkflow(item.id, currentTarget.value);
  await router.push({ path: `/creative-workshop/workflows/${encodeURIComponent(created.id)}`, query: workflowTargetQuery(currentTarget.value) });
}
async function remove(item: WorkflowDefinition) {
  if (item.offline) { ElMessage.warning("设备离线：不能删除本地工作流"); return; }
  await ElMessageBox.confirm(`确定删除“${item.name}”吗？运行历史和版本快照会一起删除。`, "删除工作流", { type: "warning" });
  await deleteWorkflow(item.id, currentTarget.value); selectedIds.value.delete(item.id); selectedIds.value = new Set(selectedIds.value); await load();
}

function toggleSelected(id: string, selected: boolean) {
  const next = new Set(selectedIds.value); selected ? next.add(id) : next.delete(id); selectedIds.value = next;
}
function clearSelection() { selectedIds.value = new Set(); }
async function batchSetEnabled(enabled: boolean) {
  const ids = [...selectedIds.value];
  await Promise.all(ids.map((id) => { const item = workflows.value.find((w) => w.id === id); return setWorkflowEnabled(id, enabled, currentTarget.value, item?.installation?.revision); }));
  clearSelection(); await load(); ElMessage.success(enabled ? "已批量启用" : "已批量停用");
}
async function batchDelete() {
  const ids = [...selectedIds.value];
  await ElMessageBox.confirm(`确定删除选中的 ${ids.length} 个工作流吗？`, "批量删除", { type: "warning" });
  for (const id of ids) await deleteWorkflow(id, currentTarget.value);
  clearSelection(); await load(); ElMessage.success("已批量删除");
}

async function saveAsTemplate(item: WorkflowDefinition) {
  const { value } = await ElMessageBox.prompt("模板保存在当前 Amitia 数据库并按用户隔离，不会公开发布。", "保存为模板", { inputValue: item.name, confirmButtonText: "保存", cancelButtonText: "取消" });
  const name = String(value || "").trim(); if (!name) return;
  await saveWorkflowTemplate(item.id, name, item.description || "", currentTarget.value);
  ElMessage.success("已保存为模板");
}
async function useTemplate(item: WorkflowTemplateSummary) {
  const created = await instantiateWorkflowTemplate(item.templateId, "", currentTarget.value);
  templateDrawer.value = false;
  await router.push({ path: `/creative-workshop/workflows/${encodeURIComponent(created.id)}`, query: workflowTargetQuery(currentTarget.value) });
}
async function removeTemplate(item: WorkflowTemplateSummary) {
  await ElMessageBox.confirm(`删除我的模板“${item.name}”？`, "删除模板", { type: "warning" });
  await deleteWorkflowTemplate(item.templateId, currentTarget.value); await loadTemplates();
}

async function exportItem(item: WorkflowDefinition) {
  const envelope = await exportWorkflow(item.id, currentTarget.value);
  const blob = new Blob([JSON.stringify(envelope, null, 2)], { type: "application/json;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a"); a.href = url; a.download = `${safeFileName(item.name)}.workflow.json`; a.click(); URL.revokeObjectURL(url);
}
async function handleImport(event: Event) {
  const input = event.target as HTMLInputElement; const file = input.files?.[0]; input.value = ""; if (!file) return;
  try {
    const payload = JSON.parse(await file.text());
    const created = await importWorkflow(payload, currentTarget.value);
    ElMessage.success("已导入。为安全起见，自动触发和 Agent 调用默认停用。");
    await router.push({ path: `/creative-workshop/workflows/${encodeURIComponent(created.id)}`, query: workflowTargetQuery(currentTarget.value) });
  } catch (e: any) { ElMessage.error(e?.response?.data?.error || e?.message || "导入失败"); }
}
function safeFileName(name: string) { return (name || "workflow").replace(/[\\/:*?"<>|\r\n]+/g, "-").slice(0, 80); }
function formatTime(value?: string) { if (!value) return ""; const d = new Date(value); return Number.isNaN(d.getTime()) ? value : d.toLocaleString(); }

onMounted(async () => {
  await loadDevices();
  await load();
  await primeSyncCursor();
  reconcileTimer = setInterval(() => { void pollSyncEvents(); }, 2000);
});
onBeforeUnmount(() => { if (reconcileTimer) clearInterval(reconcileTimer); reconcileTimer = null; });
</script>

<style scoped>
.workflow-list-page { height:100%; overflow:auto; padding:4px 2px 32px; color:var(--console-text); }
.page-header { display:flex; align-items:flex-start; justify-content:space-between; gap:20px; margin-bottom:16px; }
.crumb { display:flex; gap:8px; align-items:center; color:var(--console-text-muted); font-size:12px; margin-bottom:8px; }
.crumb a { color:var(--el-color-primary); text-decoration:none; } h1{margin:0;font-size:24px}.page-header p{margin:6px 0 0;color:var(--console-text-muted);font-size:13px}
.header-actions{display:flex;gap:8px;flex-wrap:wrap;justify-content:flex-end}.hidden-input{display:none}
.management-bar{display:flex;gap:10px;align-items:center;flex-wrap:wrap;margin-bottom:16px;padding:10px;border:1px solid var(--console-border);border-radius:12px;background:var(--ac-color-surface)}
.search-box{width:min(340px,100%)}.status-filter{width:140px}.device-filter{width:220px}.count-text{font-size:12px;color:var(--console-text-muted)}.batch-actions{margin-left:auto;display:flex;gap:7px;align-items:center;flex-wrap:wrap;font-size:12px}
.workflow-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(340px,1fr));gap:14px}.workflow-card,.empty-card{border:1px solid var(--console-border);background:var(--ac-color-surface);border-radius:14px}.workflow-card{padding:18px;display:flex;flex-direction:column;gap:16px}.workflow-card.selected{border-color:var(--el-color-primary)}
.card-top{display:grid;grid-template-columns:auto auto 1fr auto;gap:10px;align-items:start}.workflow-icon{width:42px;height:42px;border-radius:11px;display:grid;place-items:center;background:var(--ac-color-surface-soft);color:var(--el-color-primary);font-size:20px}.title-wrap{min-width:0}.title-wrap h2{margin:1px 0 4px;font-size:16px}.title-wrap p{margin:0;color:var(--console-text-muted);font-size:12px;line-height:1.5;min-height:36px}.stats{display:flex;gap:8px;flex-wrap:wrap}.stats span{padding:5px 8px;border-radius:7px;background:var(--ac-color-surface-soft);color:var(--console-text-muted);font-size:11px}.card-actions{display:flex;gap:8px;align-items:center}
.empty-card{min-height:310px;display:flex;flex-direction:column;align-items:center;justify-content:center;text-align:center;gap:10px}.empty-card>.el-icon{font-size:42px;color:var(--el-color-primary)}.empty-card h2{margin:0;font-size:18px}.empty-card p{margin:0 0 8px;color:var(--console-text-muted);max-width:420px}.empty-actions{display:flex;gap:8px;flex-wrap:wrap;justify-content:center}
.template-help{font-size:12px;line-height:1.6;color:var(--console-text-muted);padding:10px 12px;background:var(--ac-color-surface-soft);border-radius:10px;margin-bottom:12px}.template-card{display:flex;gap:12px;align-items:flex-start;justify-content:space-between;padding:14px 0;border-bottom:1px solid var(--console-border)}.template-card p{margin:5px 0;color:var(--console-text-muted);font-size:12px}.template-card small{color:var(--console-text-muted);font-size:11px}.template-actions{display:flex;gap:5px;flex-shrink:0}
@media(max-width:720px){.page-header{flex-direction:column}.header-actions{width:100%;justify-content:flex-start}.workflow-grid{grid-template-columns:1fr}.card-actions{flex-wrap:wrap}.management-bar{align-items:stretch}.search-box,.status-filter{width:100%}.batch-actions{margin-left:0}.card-top{grid-template-columns:auto auto 1fr}.card-top>.el-switch{grid-column:3;justify-self:start}}
.run-dialog-body{display:flex;flex-direction:column;gap:12px}.run-dialog-body label{display:flex;flex-direction:column;gap:6px;font-size:12px;color:var(--console-text-muted)}.run-mode-help{margin:0;color:var(--console-text-muted);font-size:11px;line-height:1.5}.controlled-warning{margin:0;padding:10px;border:1px solid var(--el-color-warning-light-5);border-radius:8px;background:var(--el-color-warning-light-9);font-size:11px;line-height:1.55;color:var(--console-text-muted)}
</style>
