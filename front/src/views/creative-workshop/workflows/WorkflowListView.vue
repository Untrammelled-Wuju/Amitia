<template>
  <main class="workflow-list-page">
    <header class="page-header">
      <div>
        <div class="crumb"><router-link to="/creative-workshop">创意工坊</router-link><span>/</span><span>工作流</span></div>
        <h1>工作流</h1>
        <p>使用可视化 DAG 编排 Kernel Tool、MCP、Task 与运行时节点。</p>
      </div>
      <div class="header-actions">
        <el-button :loading="loading" @click="load">刷新</el-button>
        <el-button :loading="aiCreating" @click="createWithAI">AI 创建</el-button>
        <el-button type="primary" @click="createNew"><el-icon><Plus /></el-icon>新建工作流</el-button>
      </div>
    </header>

    <section v-if="!loading && workflows.length === 0" class="empty-card">
      <el-icon><Share /></el-icon>
      <h2>还没有工作流</h2>
      <p>创建第一个可视化工作流，节点与连线会直接保存到 Extension Kernel。</p>
      <div class="empty-actions"><el-button :loading="aiCreating" @click="createWithAI">AI 创建</el-button><el-button type="primary" @click="createNew">创建工作流</el-button></div>
    </section>

    <section v-else class="workflow-grid">
      <article v-for="item in workflows" :key="item.id" class="workflow-card">
        <div class="card-top">
          <div class="workflow-icon"><el-icon><Share /></el-icon></div>
          <div class="title-wrap">
            <h2>{{ item.name }}</h2>
            <p>{{ item.description || "未填写描述" }}</p>
          </div>
          <el-switch :model-value="item.enabled" @change="(v) => toggle(item, Boolean(v))" />
        </div>
        <div class="stats">
          <span>{{ item.nodes?.length || 0 }} 节点</span>
          <span>{{ item.edges?.length || 0 }} 连线</span>
          <span>{{ item.triggers?.filter(t => t.enabled).length || 0 }} 触发器</span>
        </div>
        <div class="card-actions">
          <el-button type="primary" plain @click="edit(item)">打开编辑器</el-button>
          <el-button @click="runNow(item)"><el-icon><VideoPlay /></el-icon>运行</el-button>
          <el-dropdown trigger="click">
            <el-button><el-icon><MoreFilled /></el-icon></el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item @click="duplicate(item)">复制</el-dropdown-item>
                <el-dropdown-item divided @click="remove(item)">删除</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </article>
    </section>
  </main>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import { MoreFilled, Plus, Share, VideoPlay } from "@element-plus/icons-vue";
import { createWorkflow, deleteWorkflow, duplicateWorkflow, generateWorkflowWithAI, listWorkflows, runWorkflow, setWorkflowEnabled, type WorkflowDefinition } from "@/api/workflow";

const router = useRouter();
const loading = ref(false);
const aiCreating = ref(false);
const workflows = ref<WorkflowDefinition[]>([]);

async function load() {
  loading.value = true;
  try { workflows.value = await listWorkflows(); }
  finally { loading.value = false; }
}
async function createWithAI() {
  try {
    const { value } = await ElMessageBox.prompt("描述你想创建的工作流。AI 会直接生成可编辑的 workflow-v2 DAG。", "AI 创建工作流", {
      inputType: "textarea", inputPlaceholder: "例如：每天早上 8 点获取天气，如果下雨就通知我带伞", confirmButtonText: "生成", cancelButtonText: "取消",
    });
    const instruction = String(value || "").trim();
    if (!instruction) return;
    aiCreating.value = true;
    const proposal = await generateWorkflowWithAI(instruction);
    const created = await createWorkflow({ ...proposal.definition, definitionHash: undefined });
    if (proposal.warnings?.length) ElMessage.warning(proposal.warnings.join("；"));
    else ElMessage.success(proposal.summary || "AI 工作流已生成");
    await router.push(`/creative-workshop/workflows/${encodeURIComponent(created.id)}`);
  } catch (e: any) {
    if (e === "cancel" || e === "close") return;
    ElMessage.error(e?.response?.data?.error || e?.message || "AI 工作流生成失败");
  } finally { aiCreating.value = false; }
}

async function createNew() {
  const def = await createWorkflow({
    name: "未命名工作流",
    description: "",
    schemaVersion: "workflow-v2",
    inputSchema: { type: "object" },
    outputSchema: {},
    nodes: [], edges: [], triggers: [{ id: "manual", type: "manual", enabled: true, config: {} }],
    callableByAgent: false, enabled: true,
  });
  await router.push(`/creative-workshop/workflows/${encodeURIComponent(def.id)}`);
}
function edit(item: WorkflowDefinition) { router.push(`/creative-workshop/workflows/${encodeURIComponent(item.id)}`); }
async function toggle(item: WorkflowDefinition, enabled: boolean) {
  await setWorkflowEnabled(item.id, enabled); item.enabled = enabled;
}
async function runNow(item: WorkflowDefinition) {
  const result = await runWorkflow(item.id, {}, false);
  ElMessage.success(`已开始运行：${result.executionId}`);
}
async function duplicate(item: WorkflowDefinition) {
  const created = await duplicateWorkflow(item.id);
  await router.push(`/creative-workshop/workflows/${encodeURIComponent(created.id)}`);
}
async function remove(item: WorkflowDefinition) {
  await ElMessageBox.confirm(`确定删除“${item.name}”吗？`, "删除工作流", { type: "warning" });
  await deleteWorkflow(item.id); await load();
}
onMounted(load);
</script>

<style scoped>
.workflow-list-page { height: 100%; overflow: auto; padding: 4px 2px 32px; color: var(--console-text); }
.page-header { display:flex; align-items:flex-start; justify-content:space-between; gap:20px; margin-bottom:22px; }
.crumb { display:flex; gap:8px; align-items:center; color:var(--console-text-muted); font-size:12px; margin-bottom:8px; }
.crumb a { color:var(--el-color-primary); text-decoration:none; }
h1 { margin:0; font-size:24px; } .page-header p { margin:6px 0 0; color:var(--console-text-muted); font-size:13px; }
.header-actions { display:flex; gap:8px; }.empty-actions{display:flex;gap:8px;flex-wrap:wrap;justify-content:center}
.workflow-grid { display:grid; grid-template-columns:repeat(auto-fill,minmax(330px,1fr)); gap:14px; }
.workflow-card,.empty-card { border:1px solid var(--console-border); background:var(--ac-color-surface); border-radius:14px; }
.workflow-card { padding:18px; display:flex; flex-direction:column; gap:16px; }
.card-top { display:grid; grid-template-columns:auto 1fr auto; gap:12px; align-items:start; }
.workflow-icon { width:42px; height:42px; border-radius:11px; display:grid; place-items:center; background:var(--ac-color-surface-soft); color:var(--el-color-primary); font-size:20px; }
.title-wrap { min-width:0; } .title-wrap h2 { margin:1px 0 4px; font-size:16px; } .title-wrap p { margin:0; color:var(--console-text-muted); font-size:12px; line-height:1.5; min-height:36px; }
.stats { display:flex; gap:8px; flex-wrap:wrap; } .stats span { padding:5px 8px; border-radius:7px; background:var(--ac-color-surface-soft); color:var(--console-text-muted); font-size:11px; }
.card-actions { display:flex; gap:8px; align-items:center; }
.empty-card { min-height:310px; display:flex; flex-direction:column; align-items:center; justify-content:center; text-align:center; gap:10px; }
.empty-card>.el-icon { font-size:42px; color:var(--el-color-primary); } .empty-card h2 { margin:0; font-size:18px; } .empty-card p { margin:0 0 8px; color:var(--console-text-muted); max-width:420px; }
@media (max-width:720px){ .page-header{flex-direction:column}.header-actions{width:100%}.workflow-grid{grid-template-columns:1fr}.card-actions{flex-wrap:wrap} }
</style>
