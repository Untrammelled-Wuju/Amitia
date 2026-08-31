<template>
  <main class="builder-page">
    <header class="builder-header">
      <div class="title-area">
        <router-link to="/creative-workshop/workflows" class="back-link">‹ 工作流</router-link>
        <div class="title-row">
          <el-input v-model="workflow.name" class="name-input" maxlength="80" />
          <span v-if="dirty" class="dirty-dot">未保存</span>
          <span v-else class="saved-state">已保存</span>
        </div>
        <el-input v-model="workflow.description" class="description-input" placeholder="工作流描述" maxlength="240" />
      </div>
      <div class="header-actions">
        <el-button :disabled="historyIndex <= 0" @click="undo"><el-icon><RefreshLeft /></el-icon></el-button>
        <el-button :disabled="historyIndex >= history.length - 1" @click="redo"><el-icon><RefreshRight /></el-icon></el-button>
        <el-button :loading="aiWorking" @click="inspectorTab = 'ai'">AI Copilot</el-button>
        <el-button @click="validateCurrent">校验</el-button>
        <el-button :loading="saving" type="primary" @click="save">保存</el-button>
        <el-button :loading="running" type="success" @click="startRun"><el-icon><VideoPlay /></el-icon>运行</el-button>
      </div>
    </header>

    <section class="builder-layout">
      <aside class="palette-panel">
        <div class="panel-title">节点</div>
        <p class="panel-tip">拖到画布或点击添加。节点直接映射到 Kernel Handler。</p>
        <button
          v-for="item in nodePalette"
          :key="item.type"
          type="button"
          class="palette-item"
          draggable="true"
          @dragstart="onPaletteDragStart(item.type, $event)"
          @click="addNode(item.type)"
        >
          <span class="node-type-icon">{{ item.short }}</span>
          <span><strong>{{ item.label }}</strong><small>{{ item.description }}</small></span>
        </button>
        <div class="palette-divider"></div>
        <div class="panel-title small">画布</div>
        <div class="canvas-tools vertical">
          <el-button @click="zoomBy(0.1)">放大</el-button>
          <el-button @click="zoomBy(-0.1)">缩小</el-button>
          <el-button @click="fitView">适配</el-button>
          <el-button @click="autoLayout">自动布局</el-button>
        </div>
      </aside>

      <section
        ref="canvasRef"
        class="workflow-canvas"
        @dragover.prevent
        @drop="onCanvasDrop"
        @pointerdown="onCanvasPointerDown"
        @wheel.prevent="onWheel"
      >
        <div class="canvas-grid" :style="gridStyle"></div>
        <div class="graph-transform" :style="graphTransform">
          <svg class="edge-layer" viewBox="0 0 4000 2600" aria-label="工作流连线">
            <g v-for="edge in workflow.edges" :key="edge.id" class="edge-group" @click.stop="selectEdge(edge.id)">
              <path :d="edgePath(edge)" class="edge-hit" />
              <path :d="edgePath(edge)" class="edge-line" :class="{ selected: selectedEdgeId === edge.id }" />
              <text v-if="edge.label" :x="edgeLabelPoint(edge).x" :y="edgeLabelPoint(edge).y" class="edge-label">{{ edge.label }}</text>
            </g>
            <path v-if="connectingFrom" :d="previewPath" class="edge-line preview" />
          </svg>

          <article
            v-for="node in workflow.nodes"
            :key="node.id"
            class="workflow-node"
            :class="[nodeStatusClass(node.id), { selected: selectedNodeId === node.id }]"
            :style="nodeStyle(node)"
            @pointerdown.stop="selectNode(node.id)"
          >
            <button class="input-handle" type="button" aria-label="连接到此节点" @pointerup.stop="finishConnect(node.id)" @click.stop="finishConnect(node.id)"></button>
            <div class="node-header" @pointerdown.stop="startNodeDrag(node, $event)">
              <span class="node-badge">{{ paletteByType(node.type)?.short || "N" }}</span>
              <div class="node-title"><strong>{{ node.label || paletteByType(node.type)?.label || node.type }}</strong><small>{{ node.type }}</small></div>
              <el-icon class="node-menu" @click.stop="removeNode(node.id)"><Close /></el-icon>
            </div>
            <div class="node-body">
              <span v-if="node.targetId" class="target-text">{{ node.targetId }}</span>
              <span v-else class="target-text muted">未配置目标</span>
              <span v-if="stepStatus(node.id)" class="status-pill">{{ stepStatus(node.id) }}</span>
            </div>
            <button class="output-handle" type="button" aria-label="从此节点连接" @pointerdown.stop="startConnect(node.id, $event)"></button>
          </article>
        </div>

        <div class="zoom-indicator">{{ Math.round(zoom * 100) }}%</div>
        <div class="minimap" aria-label="工作流缩略图">
          <svg viewBox="0 0 4000 2600">
            <rect v-for="node in workflow.nodes" :key="node.id" :x="node.position?.x || 0" :y="node.position?.y || 0" width="180" height="84" rx="8" class="mini-node" />
          </svg>
        </div>
        <div v-if="connectingFrom" class="connect-hint">正在从 {{ connectingFrom }} 连线，松开到目标节点左侧端口</div>
      </section>

      <aside class="inspector-panel">
        <el-tabs v-model="inspectorTab" stretch @tab-change="onInspectorTabChanged">
          <el-tab-pane label="属性" name="properties">
            <div v-if="selectedNode" class="inspector-content">
              <div class="panel-title">节点属性</div>
              <label>名称<el-input v-model="selectedNode.label" @change="markDirty" /></label>
              <label>类型
                <el-select v-model="selectedNode.type" @change="onNodeTypeChanged">
                  <el-option v-for="item in nodePalette" :key="item.type" :label="item.label" :value="item.type" />
                </el-select>
              </label>
              <label v-if="needsTarget(selectedNode.type)">目标 ID<el-input v-model="selectedNode.targetId" placeholder="Tool / MCP / Task / Workflow ID" @change="markDirty" /></label>
              <template v-if="needsRuntime(selectedNode.type)">
                <label>Runtime Type<el-input v-model="selectedNode.runtime.runtimeType" @change="markDirty" /></label>
                <label>Runtime ID<el-input v-model="selectedNode.runtime.runtimeId" placeholder="例如 MCP Server / Task Definition / Service ID" @change="markDirty" /></label>
                <label>Handler Name<el-input v-model="selectedNode.runtime.handlerName" placeholder="例如 MCP Tool / JS Handler / WASM Export" @change="markDirty" /></label>
                <label>Runtime Metadata JSON<el-input v-model="nodeRuntimeMetadataEditor" type="textarea" :rows="5" placeholder='例如 {"extensionId":"...","moduleId":"..."}' @change="applyNodeEditors" /></label>
              </template>
              <label v-if="selectedNode.type === 'wait'">等待毫秒<el-input-number v-model="waitDuration" :min="0" :max="86400000" controls-position="right" @change="applyWaitDuration" /></label>
              <label>输入 JSON<el-input v-model="nodeInputEditor" type="textarea" :rows="7" @change="applyNodeEditors" /></label>
              <label>执行条件 When JSON<el-input v-model="nodeWhenEditor" type="textarea" :rows="5" placeholder='例如 {"op":"eq","left":{"ref":{"source":"input","path":["enabled"]}},"right":{"value":true}}' @change="applyNodeEditors" /></label>
              <label>权限（逗号分隔）<el-input v-model="nodePermissionsEditor" @change="applyNodeEditors" /></label>
              <label>失败策略
                <el-select v-model="selectedNode.step.onError.mode" @change="markDirty">
                  <el-option label="失败即终止" value="fail" />
                  <el-option label="继续执行" value="continue" />
                  <el-option label="使用默认值" value="use_default" />
                </el-select>
              </label>
              <label v-if="selectedNode.step.onError.mode === 'use_default'">失败默认值 JSON<el-input v-model="nodeErrorDefaultEditor" type="textarea" :rows="5" placeholder='例如 {"ok":false}' @change="applyNodeEditors" /></label>
              <el-button type="danger" plain @click="removeNode(selectedNode.id)">删除节点</el-button>
            </div>
            <div v-else-if="selectedEdge" class="inspector-content">
              <div class="panel-title">连线属性</div>
              <label>标签<el-input v-model="selectedEdge.label" @change="markDirty" /></label>
              <div class="edge-summary">{{ selectedEdge.source }} → {{ selectedEdge.target }}</div>
              <label>条件 JSON<el-input v-model="edgeConditionEditor" type="textarea" :rows="7" placeholder="为空表示无条件依赖" @change="applyEdgeEditor" /></label>
              <p class="panel-tip">条件会编译为目标节点的 When 表达式；多个入边条件按 AND 合并。</p>
              <el-button type="danger" plain @click="removeEdge(selectedEdge.id)">删除连线</el-button>
            </div>
            <div v-else class="empty-inspector">选择节点或连线后编辑属性。</div>
          </el-tab-pane>

          <el-tab-pane label="触发器" name="triggers">
            <div class="inspector-content">
              <div class="panel-row"><div class="panel-title">Trigger Center</div><el-button size="small" @click="addTrigger">新增</el-button></div>
              <article v-for="(trigger, index) in workflow.triggers" :key="trigger.id" class="trigger-card">
                <div class="trigger-head"><strong>{{ trigger.id }}</strong><el-switch v-model="trigger.enabled" @change="markDirty" /></div>
                <label>类型
                  <el-select v-model="trigger.type" @change="normalizeTrigger(trigger)">
                    <el-option label="手动" value="manual" />
                    <el-option label="系统 / 设备事件" value="event" />
                    <el-option label="Cron" value="cron" />
                    <el-option label="间隔" value="interval" />
                    <el-option label="单次" value="one_shot" />
                  </el-select>
                </label>
                <label v-if="isEventTrigger(trigger.type)">事件类型<el-input v-model="trigger.eventType" placeholder="例如 message.created / device.wifi.changed" @change="markDirty" /></label>
                <template v-if="trigger.type === 'cron'">
                  <label>Cron 表达式<el-input v-model="trigger.config.cronExpression" placeholder="0 8 * * *" @change="markDirty" /></label>
                  <label>时区<el-input v-model="trigger.config.timezone" placeholder="Asia/Shanghai" @change="markDirty" /></label>
                </template>
                <template v-if="trigger.type === 'interval'">
                  <label>间隔秒数<el-input-number v-model="trigger.config.intervalSeconds" :min="1" :max="31536000" controls-position="right" @change="markDirty" /></label>
                </template>
                <template v-if="trigger.type === 'one_shot'">
                  <label>执行时间（RFC3339）<el-input v-model="trigger.config.runAt" placeholder="2026-09-01T08:00:00+08:00" @change="markDirty" /></label>
                </template>
                <el-button v-if="workflow.triggers.length > 1" text type="danger" @click="removeTrigger(index)">移除</el-button>
              </article>
            </div>
          </el-tab-pane>

          <el-tab-pane label="映射" name="mapping">
            <div v-if="selectedNode" class="inspector-content">
              <div class="panel-title">可视化数据映射</div>
              <p class="panel-tip">把工作流输入或其他节点输出绑定到当前节点输入。跨节点绑定会自动补齐 DAG 依赖，并在保存前校验循环。</p>
              <label>目标输入字段
                <el-select v-model="mappingTargetPath" filterable allow-create default-first-option placeholder="选择或输入字段路径">
                  <el-option v-for="path in mappingTargetFields" :key="path" :label="path" :value="path" />
                </el-select>
              </label>
              <label>数据来源
                <el-select v-model="mappingSourceRef" filterable placeholder="选择上游数据">
                  <el-option-group v-for="group in mappingSourceGroups" :key="group.label" :label="group.label">
                    <el-option v-for="item in group.items" :key="item.ref" :label="item.label" :value="item.ref" />
                  </el-option-group>
                </el-select>
              </label>
              <el-button type="primary" :disabled="!mappingTargetPath || !mappingSourceRef" @click="bindMapping">绑定数据</el-button>
              <div v-if="currentMappings.length" class="mapping-list">
                <div v-for="item in currentMappings" :key="item.path" class="mapping-row">
                  <span><strong>{{ item.path }}</strong><small>{{ item.ref }}</small></span>
                  <el-button text type="danger" @click="removeMapping(item.path)">移除</el-button>
                </div>
              </div>
              <div v-else class="empty-inspector compact">当前节点还没有数据引用。</div>
            </div>
            <div v-else class="empty-inspector">先选择一个节点，再配置数据映射。</div>
          </el-tab-pane>

          <el-tab-pane label="AI" name="ai">
            <div class="inspector-content">
              <div class="panel-title">AI Workflow Copilot</div>
              <p class="panel-tip">AI 直接编辑 workflow-v2 DAG。修改会先通过 Kernel 规范化和编译校验，再作为草稿应用到画布，不会绕过保存校验。</p>
              <label>修改要求<el-input v-model="aiInstruction" type="textarea" :rows="6" placeholder="例如：在天气节点后增加条件，只有下雨才通知；HTTP 失败重试 3 次。" /></label>
              <div class="ai-actions">
                <el-button type="primary" :loading="aiWorking" :disabled="!aiInstruction.trim()" @click="aiEdit">AI 修改</el-button>
                <el-button :loading="aiWorking" @click="aiRepair">自动修复</el-button>
                <el-button :loading="aiWorking" @click="aiExplain">解释工作流</el-button>
              </div>
              <div v-if="aiResult" class="ai-result">
                <strong>{{ aiResult.title }}</strong>
                <p>{{ aiResult.summary }}</p>
                <ul v-if="aiResult.items.length"><li v-for="(item, i) in aiResult.items" :key="i">{{ item }}</li></ul>
              </div>
            </div>
          </el-tab-pane>

          <el-tab-pane label="运行" name="runs">
            <div class="inspector-content">
              <div class="panel-row"><div class="panel-title">Execution Trace</div><el-button size="small" @click="refreshRuns">刷新</el-button></div>
              <div v-if="currentRun" class="current-run-card">
                <div><strong>{{ currentRun.status }}</strong><small>{{ currentRun.executionId }}</small></div>
                <div class="run-actions">
                  <el-button size="small" :disabled="!canPause" @click="pauseCurrent">暂停</el-button>
                  <el-button size="small" :disabled="!canResume" @click="resumeCurrent">恢复</el-button>
                  <el-button size="small" type="danger" plain :disabled="!canCancel" @click="cancelCurrent">取消</el-button>
                </div>
              </div>
              <article v-for="step in stepRuns" :key="step.nodeId" class="trace-step" @click="selectNode(step.nodeId)">
                <span class="trace-status" :class="`s-${step.status}`"></span>
                <div><strong>{{ nodeLabel(step.nodeId) }}</strong><small>{{ step.status }} · attempt {{ step.attempt || 1 }}</small><p v-if="step.error">{{ step.error }}</p></div>
              </article>
              <div class="panel-title run-history-title">历史运行</div>
              <article v-for="run in runs" :key="run.executionId" class="run-history" @click="openRun(run.executionId)">
                <div><strong>{{ run.status }}</strong><small>{{ formatTime(run.startedAt) }}</small></div><span>{{ run.executionId.slice(-8) }}</span>
              </article>
              <div v-if="runs.length === 0" class="empty-inspector">暂无运行记录。</div>
            </div>
          </el-tab-pane>

          <el-tab-pane label="版本" name="versions">
            <div class="inspector-content">
              <div class="panel-row"><div class="panel-title">版本历史</div><el-button size="small" :loading="revisionBusy" @click="manualSnapshot">保存快照</el-button></div>
              <p class="panel-tip">每次保存修改前会自动记录旧版本，最多保留最近 50 个版本快照。回滚前也会先保存当前状态。</p>
              <article v-for="item in revisions" :key="item.revisionId" class="revision-card">
                <div class="revision-main">
                  <strong>#{{ item.revisionNo }} · {{ item.note || "自动快照" }}</strong>
                  <small>{{ formatTime(item.createdAt) }}</small>
                  <span>{{ item.definitionHash.slice(0, 12) }}</span>
                </div>
                <el-button size="small" text type="primary" :loading="revisionBusy" @click="rollbackRevision(item)">回滚</el-button>
              </article>
              <div v-if="revisions.length === 0" class="empty-inspector compact">暂无版本快照。</div>
            </div>
          </el-tab-pane>

          <el-tab-pane label="设置" name="settings">
            <div class="inspector-content">
              <label>启用工作流<el-switch v-model="workflow.enabled" @change="markDirty" /></label>
              <label>允许 AI 调用<el-switch v-model="workflow.callableByAgent" @change="markDirty" /></label>
              <template v-if="workflow.callableByAgent">
                <label>Agent Tool 名称<el-input v-model="workflow.agentTool.name" placeholder="留空则自动生成" maxlength="64" @change="markDirty" /></label>
                <label>Agent Tool 描述<el-input v-model="workflow.agentTool.description" type="textarea" :rows="3" placeholder="告诉模型何时调用这个工作流" maxlength="500" @change="markDirty" /></label>
                <p class="panel-tip">启用并保存后，此工作流会按当前用户隔离注册到 Agent Tool Registry；禁用、关闭或删除时自动撤销。</p>
              </template>
              <label>Input Schema<el-input v-model="inputSchemaEditor" type="textarea" :rows="8" @change="applySchemaEditors" /></label>
              <label>Output Schema<el-input v-model="outputSchemaEditor" type="textarea" :rows="8" @change="applySchemaEditors" /></label>
              <div class="definition-meta">Schema {{ workflow.schemaVersion }}<br />Hash {{ workflow.definitionHash || "保存后生成" }}</div>
            </div>
          </el-tab-pane>
        </el-tabs>
      </aside>
    </section>
  </main>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import { Close, RefreshLeft, RefreshRight, VideoPlay } from "@element-plus/icons-vue";
import {
  cancelWorkflowRun, createWorkflowRevision, editWorkflowWithAI, explainWorkflowWithAI, getWorkflow, getWorkflowCatalog, getWorkflowRun, listWorkflowRevisions, listWorkflowRuns, pauseWorkflowRun, repairWorkflowWithAI, resumeWorkflowRun, rollbackWorkflowRevision,
  runWorkflow, updateWorkflow, validateWorkflow,
  type WorkflowAIProposal, type WorkflowCatalogItem, type WorkflowDefinition, type WorkflowEdge, type WorkflowNode, type WorkflowRevisionSummary, type WorkflowRun, type WorkflowStepRun, type WorkflowTrigger,
} from "@/api/workflow";

const route = useRoute();
const workflowId = String(route.params.id || "");
const workflow = reactive<WorkflowDefinition>({
  schemaVersion: "workflow-v2", id: workflowId, name: "工作流", description: "", inputSchema: { type: "object" }, outputSchema: {},
  nodes: [], edges: [], triggers: [{ id: "manual", type: "manual", enabled: true, config: {} }], callableByAgent: false, agentTool: {}, enabled: true,
});
const nodePalette = [
  { type: "tool", label: "Tool", short: "T", description: "调用 Kernel Tool" },
  { type: "mcp", label: "MCP", short: "M", description: "调用 MCP Runtime" },
  { type: "task", label: "Task", short: "K", description: "执行 Task Runtime" },
  { type: "javascript", label: "JavaScript", short: "JS", description: "JavaScript Runtime" },
  { type: "wasm", label: "WASM", short: "W", description: "WASM Runtime" },
  { type: "trusted_service", label: "Trusted Service", short: "S", description: "受信服务 Runtime" },
  { type: "nested_workflow", label: "子工作流", short: "WF", description: "调用另一个工作流" },
  { type: "condition", label: "条件", short: "?", description: "条件 / When 分支" },
  { type: "transform", label: "转换", short: "↔", description: "提取或转换数据" },
  { type: "wait", label: "等待", short: "⏱", description: "延迟执行" },
];

const canvasRef = ref<HTMLElement | null>(null);
const selectedNodeId = ref(""); const selectedEdgeId = ref(""); const inspectorTab = ref("properties");
const dirty = ref(false); const saving = ref(false); const running = ref(false);
const zoom = ref(1); const pan = reactive({ x: 80, y: 70 }); const pointerGraph = reactive({ x: 0, y: 0 });
const connectingFrom = ref("");
const nodeInputEditor = ref("{}"); const nodeWhenEditor = ref(""); const nodePermissionsEditor = ref(""); const nodeRuntimeMetadataEditor = ref("{}"); const nodeErrorDefaultEditor = ref(""); const edgeConditionEditor = ref("");
const inputSchemaEditor = ref("{}"); const outputSchemaEditor = ref("{}");
const catalog = ref<WorkflowCatalogItem[]>([]); const mappingTargetPath = ref(""); const mappingSourceRef = ref("");
const aiInstruction = ref(""); const aiWorking = ref(false); const aiResult = ref<{title:string;summary:string;items:string[]}|null>(null);
const history = ref<string[]>([]); const historyIndex = ref(-1); let restoringHistory = false;
const runs = ref<WorkflowRun[]>([]); const currentRun = ref<WorkflowRun | null>(null); const stepRuns = ref<WorkflowStepRun[]>([]); let pollTimer: number | undefined;
const revisions = ref<WorkflowRevisionSummary[]>([]); const revisionBusy = ref(false);
let dragState: { nodeId: string; startClientX: number; startClientY: number; startX: number; startY: number } | null = null;
let panState: { startClientX: number; startClientY: number; startX: number; startY: number } | null = null;
let draggedPaletteType = "";

const selectedNode = computed(() => workflow.nodes.find(n => n.id === selectedNodeId.value));
const selectedEdge = computed(() => workflow.edges.find(e => e.id === selectedEdgeId.value));
const graphTransform = computed(() => ({ transform: `translate(${pan.x}px, ${pan.y}px) scale(${zoom.value})` }));
const gridStyle = computed(() => ({ backgroundPosition: `${pan.x}px ${pan.y}px`, backgroundSize: `${24 * zoom.value}px ${24 * zoom.value}px` }));
const waitDuration = computed({ get: () => Number(selectedNode.value?.runtime?.metadata?.durationMs || 0), set: v => { if (selectedNode.value) { ensureRuntime(selectedNode.value).metadata!.durationMs = Number(v || 0); markDirty(); } } });
const canPause = computed(() => currentRun.value?.status === "running" || currentRun.value?.status === "resuming");
const canResume = computed(() => currentRun.value?.status === "paused");
const canCancel = computed(() => ["running","pausing","paused","resuming","compensating"].includes(currentRun.value?.status || ""));
const previewPath = computed(() => { const n = workflow.nodes.find(x => x.id === connectingFrom.value); if (!n) return ""; const s = outputPoint(n); return bezier(s.x, s.y, pointerGraph.x, pointerGraph.y); });
const selectedTargetCatalog = computed(() => {
  const node = selectedNode.value; if (!node) return undefined;
  return catalog.value.find(item => item.id === node.targetId || item.modelName === node.targetId || item.runtime?.runtimeId === node.runtime?.runtimeId);
});
const mappingTargetFields = computed(() => {
  const paths = schemaLeafPaths(selectedTargetCatalog.value?.inputSchema);
  if (paths.length) return paths;
  const input = selectedNode.value?.step?.input;
  return input && typeof input === "object" && !Array.isArray(input) ? Object.keys(input as Record<string, unknown>) : [];
});
const mappingSourceGroups = computed(() => {
  const groups: Array<{label:string;items:Array<{label:string;ref:string}>}> = [];
  const inputItems = schemaLeafPaths(workflow.inputSchema).map(path => ({ label: `工作流输入 · ${path}`, ref: `input.${path}` }));
  if (inputItems.length) groups.push({ label: "工作流输入", items: inputItems });
  groups.push({
    label: "运行时上下文",
    items: [
      { label: "当前用户 · userId", ref: "runtime.userId" },
      { label: "当前会话 · conversationId", ref: "runtime.conversationId" },
      { label: "当前角色 · characterId", ref: "runtime.characterId" },
      { label: "根任务 · rootId", ref: "runtime.rootId" },
      { label: "调度任务 · scheduleId", ref: "runtime.scheduleId" },
      { label: "执行追踪 · traceId", ref: "runtime.traceId" },
    ],
  });
  const target = selectedNode.value;
  if (!target) return groups;
  for (const node of workflow.nodes) {
    if (node.id === target.id || wouldCreateCycle(node.id, target.id)) continue;
    const item = catalog.value.find(x => x.id === node.targetId || x.modelName === node.targetId || x.runtime?.runtimeId === node.runtime?.runtimeId);
    let paths = schemaLeafPaths(item?.outputSchema);
    if (!paths.length) {
      const observed = stepRuns.value.find(step => step.nodeId === node.id)?.output;
      paths = valueLeafPaths(observed);
    }
    if (!paths.length) continue;
    groups.push({ label: node.label || node.id, items: paths.map(path => ({ label: `${node.label || node.id} · ${path}`, ref: `steps.${node.id}.${path}` })) });
  }
  return groups;
});
const currentMappings = computed(() => collectMappings(selectedNode.value?.step?.input));

function paletteByType(type: string) { return nodePalette.find(x => x.type === type); }
function needsTarget(type: string) { return ["tool","mcp","task","javascript","wasm","trusted_service","nested_workflow"].includes(type); }
function needsRuntime(type: string) { return ["mcp","task","javascript","wasm","trusted_service"].includes(type); }
function defaultRuntimeType(type: string) { return ({ mcp:"mcp", task:"task", javascript:"javascript", wasm:"wasm", trusted_service:"trusted_service" } as Record<string,string>)[type] || ""; }
function ensureRuntime(node: WorkflowNode) { node.runtime ||= {}; node.runtime.metadata ||= {}; return node.runtime; }
function markDirty() { if (!restoringHistory) dirty.value = true; }
function pretty(value: unknown) { return value == null ? "" : JSON.stringify(value, null, 2); }
function parseEditor(text: string, fallback: unknown) { if (!text.trim()) return fallback; return JSON.parse(text); }
function modelSnapshot() { return JSON.stringify({ name: workflow.name, description: workflow.description, inputSchema: workflow.inputSchema, outputSchema: workflow.outputSchema, nodes: workflow.nodes, edges: workflow.edges, triggers: workflow.triggers, callableByAgent: workflow.callableByAgent, agentTool: workflow.agentTool, enabled: workflow.enabled }); }
function pushHistory() { const snap = modelSnapshot(); if (history.value[historyIndex.value] === snap) return; history.value = history.value.slice(0, historyIndex.value + 1); history.value.push(snap); if (history.value.length > 60) history.value.shift(); historyIndex.value = history.value.length - 1; }
function restoreSnapshot(snap: string) { restoringHistory = true; const data = JSON.parse(snap); Object.assign(workflow, data); selectedNodeId.value = ""; selectedEdgeId.value = ""; restoringHistory = false; dirty.value = true; }
function undo() { if (historyIndex.value <= 0) return; historyIndex.value--; restoreSnapshot(history.value[historyIndex.value]); }
function redo() { if (historyIndex.value >= history.value.length - 1) return; historyIndex.value++; restoreSnapshot(history.value[historyIndex.value]); }

function schemaLeafPaths(schema: unknown, prefix = ""): string[] {
  if (!schema || typeof schema !== "object" || Array.isArray(schema)) return [];
  const obj = schema as Record<string, any>; const props = obj.properties;
  if (!props || typeof props !== "object") return prefix ? [prefix] : [];
  const out: string[] = [];
  for (const [key, child] of Object.entries(props as Record<string, unknown>)) {
    const path = prefix ? `${prefix}.${key}` : key; const nested = schemaLeafPaths(child, path); out.push(...(nested.length ? nested : [path]));
  }
  return out;
}
function valueLeafPaths(value: unknown, prefix = "", depth = 0): string[] {
  if (depth > 6) return prefix ? [prefix] : [];
  if (value == null || typeof value !== "object") return prefix ? [prefix] : [];
  if (Array.isArray(value)) {
    if (!value.length) return prefix ? [prefix] : [];
    return valueLeafPaths(value[0], prefix ? `${prefix}.0` : "0", depth + 1);
  }
  const entries = Object.entries(value as Record<string, unknown>);
  if (!entries.length) return prefix ? [prefix] : [];
  const out: string[] = [];
  for (const [key, child] of entries) {
    const path = prefix ? `${prefix}.${key}` : key;
    const nested = valueLeafPaths(child, path, depth + 1);
    out.push(...(nested.length ? nested : [path]));
  }
  return [...new Set(out)].slice(0, 100);
}
function setPath(root: Record<string, any>, path: string, value: unknown) {
  const parts = path.split(".").map(x=>x.trim()).filter(Boolean); if (!parts.length) return; let cur=root;
  for (let i=0;i<parts.length-1;i++){const key=parts[i];if(!cur[key]||typeof cur[key]!=="object"||Array.isArray(cur[key]))cur[key]={};cur=cur[key];}
  cur[parts[parts.length-1]]=value;
}
function deletePath(root: Record<string, any>, path: string) {
  const parts=path.split(".").filter(Boolean); if(!parts.length)return; let cur:any=root;
  for(let i=0;i<parts.length-1;i++){cur=cur?.[parts[i]];if(!cur||typeof cur!=="object")return;} delete cur[parts[parts.length-1]];
}
function collectMappings(value: unknown, prefix=""): Array<{path:string;ref:string}> {
  if (typeof value === "string" && /^(input|steps|runtime)\./.test(value)) return prefix ? [{path:prefix,ref:value}] : [];
  if (!value || typeof value !== "object" || Array.isArray(value)) return [];
  return Object.entries(value as Record<string, unknown>).flatMap(([key, child]) => collectMappings(child, prefix ? `${prefix}.${key}` : key));
}
function isAncestor(source:string,target:string){if(source===target)return false;const stack=[source],seen=new Set<string>();while(stack.length){const id=stack.pop()!;if(id===target)return true;if(seen.has(id))continue;seen.add(id);for(const e of workflow.edges)if(e.source===id)stack.push(e.target);}return false;}
function bindMapping(){
  const node=selectedNode.value,targetPath=mappingTargetPath.value.trim(),refValue=mappingSourceRef.value.trim();if(!node||!targetPath||!refValue)return;
  const match=/^steps\.([^.]+)\./.exec(refValue); const sourceId=match?.[1];
  if(sourceId&&!isAncestor(sourceId,node.id)){if(wouldCreateCycle(sourceId,node.id)){ElMessage.error("该数据来源会形成循环依赖");return;}workflow.edges.push({id:`edge-map-${Date.now().toString(36)}`,source:sourceId,target:node.id,label:"data"});}
  const input=(node.step.input&&typeof node.step.input==="object"&&!Array.isArray(node.step.input)?node.step.input:{} ) as Record<string,any>; setPath(input,targetPath,refValue); node.step.input=input; nodeInputEditor.value=pretty(input); markDirty();pushHistory();ElMessage.success("数据映射已绑定");
}
function removeMapping(path:string){const node=selectedNode.value;if(!node||!node.step.input||typeof node.step.input!=="object"||Array.isArray(node.step.input))return;deletePath(node.step.input as Record<string,any>,path);nodeInputEditor.value=pretty(node.step.input);markDirty();pushHistory();}
function normalizeLoadedDefinition(){
  workflow.edges ||= []; workflow.triggers ||= [{id:"manual",type:"manual",enabled:true,config:{}}]; workflow.agentTool ||= {};
  for(const trigger of workflow.triggers){trigger.config ||= {};} for(const n of workflow.nodes){n.position ||= {x:100,y:100};n.step ||= {input:{},onError:{mode:"fail"}};n.step.onError ||= {mode:"fail"};ensureRuntime(n);}
  inputSchemaEditor.value=pretty(workflow.inputSchema);outputSchemaEditor.value=pretty(workflow.outputSchema);
}
function applyAIProposal(proposal: WorkflowAIProposal){
  const def=JSON.parse(JSON.stringify(proposal.definition)) as WorkflowDefinition;def.id=workflow.id;Object.assign(workflow,def);normalizeLoadedDefinition();selectedNodeId.value="";selectedEdgeId.value="";dirty.value=true;pushHistory();autoLayout();aiResult.value={title:"AI 修改已应用到草稿",summary:proposal.summary||"工作流已更新",items:[...(proposal.changes||[]),...(proposal.warnings||[]).map(x=>`警告：${x}`)]};
}
async function ensureSavedForAI(){if(!dirty.value)return true;await save();return !dirty.value;}
async function aiEdit(){if(!aiInstruction.value.trim()||aiWorking.value)return;if(!(await ensureSavedForAI()))return;aiWorking.value=true;try{applyAIProposal(await editWorkflowWithAI(workflow.id,aiInstruction.value.trim()));ElMessage.success("AI 修改已应用，请确认后保存");}catch(e:any){ElMessage.error(e?.response?.data?.error||e?.message||"AI 修改失败");}finally{aiWorking.value=false;}}
async function aiRepair(){if(aiWorking.value)return;if(!(await ensureSavedForAI()))return;aiWorking.value=true;try{applyAIProposal(await repairWorkflowWithAI(workflow.id,aiInstruction.value.trim()));ElMessage.success("AI 修复建议已应用，请确认后保存");}catch(e:any){ElMessage.error(e?.response?.data?.error||e?.message||"AI 修复失败");}finally{aiWorking.value=false;}}
async function aiExplain(){if(aiWorking.value)return;if(!(await ensureSavedForAI()))return;aiWorking.value=true;try{const result=await explainWorkflowWithAI(workflow.id,aiInstruction.value.trim());aiResult.value={title:"AI 工作流解释",summary:result.summary,items:[...(result.flow||[]).map(x=>`流程：${x}`),...(result.issues||[]).map(x=>`问题：${x}`),...(result.suggestions||[]).map(x=>`建议：${x}`)]};}catch(e:any){ElMessage.error(e?.response?.data?.error||e?.message||"AI 解释失败");}finally{aiWorking.value=false;}}

function addNode(type: string, position?: {x:number;y:number}) {
  pushHistory(); const index = workflow.nodes.length;
  const node: WorkflowNode = { id: `${type}-${Date.now().toString(36)}-${index+1}`, type, label: paletteByType(type)?.label || type, position: position || { x: 120 + (index%4)*230, y: 120 + Math.floor(index/4)*150 }, step: { input: {}, onError: { mode: "fail" } }, runtime: { runtimeType: defaultRuntimeType(type), metadata: {} } };
  if (type === "wait") node.runtime.metadata = { durationMs: 1000 };
  workflow.nodes.push(node); selectNode(node.id); markDirty(); pushHistory();
}
function removeNode(id: string) { pushHistory(); workflow.nodes = workflow.nodes.filter(n => n.id !== id); workflow.edges = workflow.edges.filter(e => e.source !== id && e.target !== id); if (selectedNodeId.value === id) selectedNodeId.value=""; markDirty(); pushHistory(); }
function removeEdge(id: string) { pushHistory(); workflow.edges = workflow.edges.filter(e => e.id !== id); selectedEdgeId.value=""; markDirty(); pushHistory(); }
function selectNode(id: string) { selectedNodeId.value=id; selectedEdgeId.value=""; inspectorTab.value="properties"; syncNodeEditors(); }
function selectEdge(id: string) { selectedEdgeId.value=id; selectedNodeId.value=""; inspectorTab.value="properties"; edgeConditionEditor.value = pretty(selectedEdge.value?.condition); }
function onNodeTypeChanged() { const n=selectedNode.value; if(!n)return; const rt=defaultRuntimeType(n.type); if(rt) ensureRuntime(n).runtimeType=rt; markDirty(); }
function syncNodeEditors() { const n=selectedNode.value; if(!n)return; ensureRuntime(n); nodeInputEditor.value=pretty(n.step?.input ?? {}); nodeWhenEditor.value=pretty(n.step?.when); nodePermissionsEditor.value=(n.permissions||[]).join(", "); nodeRuntimeMetadataEditor.value=pretty(n.runtime?.metadata ?? {}); nodeErrorDefaultEditor.value=pretty(n.step?.onError?.default); }
function applyNodeEditors(): boolean { const n=selectedNode.value; if(!n)return true; try { n.step ||= { input:{}, onError:{mode:"fail"} }; n.step.onError ||= { mode:"fail" }; n.step.input=parseEditor(nodeInputEditor.value, {}); n.step.when=nodeWhenEditor.value.trim()?parseEditor(nodeWhenEditor.value, undefined):undefined; n.step.onError.default=n.step.onError.mode==="use_default"?(nodeErrorDefaultEditor.value.trim()?parseEditor(nodeErrorDefaultEditor.value, undefined):null):undefined; n.permissions=nodePermissionsEditor.value.split(",").map(x=>x.trim()).filter(Boolean); ensureRuntime(n).metadata=parseEditor(nodeRuntimeMetadataEditor.value, {}) as Record<string, unknown>; markDirty(); return true; } catch(e:any){ ElMessage.error(`节点 JSON 无效：${e.message}`); return false; } }
function applyEdgeEditor(): boolean { const e=selectedEdge.value;if(!e)return true; try{e.condition=edgeConditionEditor.value.trim()?parseEditor(edgeConditionEditor.value, undefined):undefined;markDirty();return true;}catch(err:any){ElMessage.error(`连线条件 JSON 无效：${err.message}`);return false;} }
function applySchemaEditors(): boolean { try { workflow.inputSchema=parseEditor(inputSchemaEditor.value,{}); workflow.outputSchema=parseEditor(outputSchemaEditor.value,{}); markDirty(); return true; } catch(e:any){ElMessage.error(`Schema JSON 无效：${e.message}`);return false;} }
function applyWaitDuration(v: number | undefined) { waitDuration.value = Number(v || 0); }

function nodeStyle(node: WorkflowNode) { return { left:`${node.position?.x || 0}px`, top:`${node.position?.y || 0}px` }; }
function inputPoint(node: WorkflowNode) { return { x:(node.position?.x||0), y:(node.position?.y||0)+42 }; }
function outputPoint(node: WorkflowNode) { return { x:(node.position?.x||0)+180, y:(node.position?.y||0)+42 }; }
function bezier(x1:number,y1:number,x2:number,y2:number) { const d=Math.max(70,Math.abs(x2-x1)*0.45); return `M ${x1} ${y1} C ${x1+d} ${y1}, ${x2-d} ${y2}, ${x2} ${y2}`; }
function edgePath(edge: WorkflowEdge) { const s=workflow.nodes.find(n=>n.id===edge.source), t=workflow.nodes.find(n=>n.id===edge.target); if(!s||!t)return ""; const a=outputPoint(s),b=inputPoint(t); return bezier(a.x,a.y,b.x,b.y); }
function edgeLabelPoint(edge: WorkflowEdge) { const s=workflow.nodes.find(n=>n.id===edge.source), t=workflow.nodes.find(n=>n.id===edge.target); if(!s||!t)return{x:0,y:0}; const a=outputPoint(s),b=inputPoint(t); return{x:(a.x+b.x)/2,y:(a.y+b.y)/2-8}; }
function canvasCoordinates(clientX:number,clientY:number){const r=canvasRef.value!.getBoundingClientRect();return{x:(clientX-r.left-pan.x)/zoom.value,y:(clientY-r.top-pan.y)/zoom.value};}
function startNodeDrag(node:WorkflowNode,event:PointerEvent){ if(event.button!==0)return; pushHistory(); dragState={nodeId:node.id,startClientX:event.clientX,startClientY:event.clientY,startX:node.position?.x||0,startY:node.position?.y||0}; window.addEventListener("pointermove",onGlobalPointerMove);window.addEventListener("pointerup",onGlobalPointerUp,{once:true}); }
function startConnect(nodeId:string,event:PointerEvent){ connectingFrom.value=nodeId; const p=canvasCoordinates(event.clientX,event.clientY);pointerGraph.x=p.x;pointerGraph.y=p.y;window.addEventListener("pointermove",onGlobalPointerMove);window.addEventListener("pointerup",onConnectPointerUp,{once:true}); }
function onConnectPointerUp(){window.removeEventListener("pointermove",onGlobalPointerMove);setTimeout(()=>{connectingFrom.value=""},50);}
function finishConnect(targetId:string){ const source=connectingFrom.value;if(!source||source===targetId)return; if(workflow.edges.some(e=>e.source===source&&e.target===targetId)){connectingFrom.value="";return;} if(wouldCreateCycle(source,targetId)){ElMessage.error("该连线会形成环，DAG 不允许循环依赖");connectingFrom.value="";return;} pushHistory(); workflow.edges.push({id:`edge-${Date.now().toString(36)}`,source,target:targetId});connectingFrom.value="";markDirty();pushHistory(); }
function wouldCreateCycle(source:string,target:string){const adj=new Map<string,string[]>();for(const e of workflow.edges){const a=adj.get(e.source)||[];a.push(e.target);adj.set(e.source,a);}const stack=[target],seen=new Set<string>();while(stack.length){const n=stack.pop()!;if(n===source)return true;if(seen.has(n))continue;seen.add(n);stack.push(...(adj.get(n)||[]));}return false;}
function onGlobalPointerMove(event:PointerEvent){if(dragState){const n=workflow.nodes.find(x=>x.id===dragState!.nodeId);if(n){n.position={x:Math.max(0,dragState.startX+(event.clientX-dragState.startClientX)/zoom.value),y:Math.max(0,dragState.startY+(event.clientY-dragState.startClientY)/zoom.value)};markDirty();}}if(connectingFrom.value&&canvasRef.value){const p=canvasCoordinates(event.clientX,event.clientY);pointerGraph.x=p.x;pointerGraph.y=p.y;}}
function onGlobalPointerUp(){window.removeEventListener("pointermove",onGlobalPointerMove);dragState=null;pushHistory();}
function onCanvasPointerDown(event:PointerEvent){if(event.target!==canvasRef.value&&!(event.target as HTMLElement).classList.contains("canvas-grid"))return;if(event.button!==0)return;selectedNodeId.value="";selectedEdgeId.value="";panState={startClientX:event.clientX,startClientY:event.clientY,startX:pan.x,startY:pan.y};const move=(e:PointerEvent)=>{if(!panState)return;pan.x=panState.startX+(e.clientX-panState.startClientX);pan.y=panState.startY+(e.clientY-panState.startClientY);};const up=()=>{window.removeEventListener("pointermove",move);panState=null;};window.addEventListener("pointermove",move);window.addEventListener("pointerup",up,{once:true});}
function onWheel(e:WheelEvent){const delta=e.deltaY>0?-0.08:0.08;zoomBy(delta);}
function zoomBy(delta:number){zoom.value=Math.min(1.8,Math.max(0.35,Number((zoom.value+delta).toFixed(2))));}
function fitView(){if(workflow.nodes.length===0){zoom.value=1;pan.x=80;pan.y=70;return;}const xs=workflow.nodes.map(n=>n.position?.x||0),ys=workflow.nodes.map(n=>n.position?.y||0);const minX=Math.min(...xs),maxX=Math.max(...xs)+180,minY=Math.min(...ys),maxY=Math.max(...ys)+84;const r=canvasRef.value?.getBoundingClientRect();if(!r)return;const z=Math.min(1.2,Math.max(0.35,Math.min((r.width-100)/(maxX-minX+1),(r.height-100)/(maxY-minY+1))));zoom.value=z;pan.x=50-minX*z;pan.y=50-minY*z;}
function autoLayout(){pushHistory();const indeg=new Map<string,number>(),adj=new Map<string,string[]>();workflow.nodes.forEach(n=>indeg.set(n.id,0));workflow.edges.forEach(e=>{indeg.set(e.target,(indeg.get(e.target)||0)+1);const a=adj.get(e.source)||[];a.push(e.target);adj.set(e.source,a);});let current=workflow.nodes.filter(n=>(indeg.get(n.id)||0)===0).map(n=>n.id),level=0,seen=0;while(current.length){const next:string[]=[];current.forEach((id,i)=>{const n=workflow.nodes.find(x=>x.id===id);if(n)n.position={x:100+level*260,y:90+i*140};seen++;for(const t of adj.get(id)||[]){indeg.set(t,(indeg.get(t)||0)-1);if(indeg.get(t)===0)next.push(t);}});current=next;level++;}if(seen!==workflow.nodes.length){ElMessage.error("存在循环依赖，无法自动布局");return;}markDirty();pushHistory();fitView();}
function onPaletteDragStart(type:string,e:DragEvent){draggedPaletteType=type;if(e.dataTransfer)e.dataTransfer.effectAllowed="copy";}
function onCanvasDrop(e:DragEvent){if(!draggedPaletteType||!canvasRef.value)return;const p=canvasCoordinates(e.clientX,e.clientY);addNode(draggedPaletteType,{x:Math.max(0,p.x-90),y:Math.max(0,p.y-42)});draggedPaletteType="";}

function addTrigger(){pushHistory();workflow.triggers.push({id:`trigger-${Date.now().toString(36)}`,type:"event",eventType:"",enabled:true,config:{}});markDirty();pushHistory();}
function removeTrigger(index:number){pushHistory();workflow.triggers.splice(index,1);markDirty();pushHistory();}
function isEventTrigger(type:string){return type === "event";}
function normalizeTrigger(trigger:WorkflowTrigger){trigger.config ||= {};if(trigger.type==="cron"){trigger.config.type="cron";trigger.config.cronExpression ||= "0 8 * * *";trigger.config.timezone ||= Intl.DateTimeFormat().resolvedOptions().timeZone||"UTC";}if(trigger.type==="interval"){trigger.config.type="interval";trigger.config.intervalSeconds ||= 3600;}if(trigger.type==="one_shot"){trigger.config.type="one_shot";trigger.config.runAt ||= new Date(Date.now()+3600000).toISOString();}markDirty();}

async function validateCurrent(showSuccess=true){if(!applyNodeEditors()||!applyEdgeEditor()||!applySchemaEditors())return false;try{const result=await validateWorkflow(JSON.parse(JSON.stringify(workflow)));if(showSuccess)ElMessage.success(`DAG 校验通过，拓扑节点 ${result.topologicalOrder?.length||0} 个`);return true;}catch(e:any){if(showSuccess)ElMessage.error(e?.response?.data?.error || e?.message || "工作流校验失败");return false;}}
async function save(){if(saving.value)return;if(!(await validateCurrent(false)))return;saving.value=true;try{const updated=await updateWorkflow(workflow.id,JSON.parse(JSON.stringify(workflow)));Object.assign(workflow,updated);dirty.value=false;pushHistory();await refreshRevisions();ElMessage.success("工作流已保存");}finally{saving.value=false;}}
async function startRun(){if(dirty.value){await save();if(dirty.value)return;}running.value=true;try{const res=await runWorkflow(workflow.id,{},false);currentRun.value={executionId:res.executionId,workflowId:workflow.id,status:"running"};inspectorTab.value="runs";startPolling(res.executionId);}finally{running.value=false;}}
function startPolling(runId:string){if(pollTimer)clearInterval(pollTimer);const tick=async()=>{try{const data=await getWorkflowRun(runId);currentRun.value=data.run;stepRuns.value=data.stepRuns||[];if(["succeeded","failed","cancelled","compensated"].includes(data.run.status)){if(pollTimer)clearInterval(pollTimer);pollTimer=undefined;await refreshRuns();}}catch{/* run can take a moment to be persisted */}};void tick();pollTimer=window.setInterval(tick,800);}
async function refreshRevisions(){try{revisions.value=await listWorkflowRevisions(workflowId,50);}catch{revisions.value=[];}}
async function manualSnapshot(){if(dirty.value){ElMessage.warning("请先保存当前修改，再创建版本快照");return;}revisionBusy.value=true;try{const {value}=await ElMessageBox.prompt("可选：给这个快照加一个备注。","保存版本快照",{inputPlaceholder:"例如：调整天气分支前",confirmButtonText:"保存",cancelButtonText:"取消"});await createWorkflowRevision(workflowId,String(value||"").trim());await refreshRevisions();ElMessage.success("版本快照已保存");}catch(e:any){if(e!=="cancel"&&e!=="close")ElMessage.error(e?.response?.data?.error||e?.message||"保存快照失败");}finally{revisionBusy.value=false;}}
async function rollbackRevision(item:WorkflowRevisionSummary){try{await ElMessageBox.confirm(`回滚到版本 #${item.revisionNo}？当前状态会先自动保存，回滚会恢复当时的 DAG、触发器和设置。`,"版本回滚",{type:"warning",confirmButtonText:"回滚",cancelButtonText:"取消"});revisionBusy.value=true;const restored=await rollbackWorkflowRevision(workflowId,item.revisionId);Object.assign(workflow,restored);normalizeLoadedDefinition();dirty.value=false;history.value=[modelSnapshot()];historyIndex.value=0;await Promise.all([refreshRevisions(),refreshRuns()]);ElMessage.success(`已回滚到版本 #${item.revisionNo}`);}catch(e:any){if(e!=="cancel"&&e!=="close")ElMessage.error(e?.response?.data?.error||e?.message||"回滚失败");}finally{revisionBusy.value=false;}}
async function refreshRuns(){const data=await listWorkflowRuns(workflow.id,40);runs.value=data.items||[];}
async function openRun(runId:string){const data=await getWorkflowRun(runId);currentRun.value=data.run;stepRuns.value=data.stepRuns||[];if(!["succeeded","failed","cancelled","compensated"].includes(data.run.status))startPolling(runId);}
async function pauseCurrent(){if(!currentRun.value)return;await pauseWorkflowRun(currentRun.value.executionId);startPolling(currentRun.value.executionId);}
async function resumeCurrent(){if(!currentRun.value)return;await resumeWorkflowRun(currentRun.value.executionId);startPolling(currentRun.value.executionId);}
async function cancelCurrent(){if(!currentRun.value)return;await cancelWorkflowRun(currentRun.value.executionId);startPolling(currentRun.value.executionId);}
function stepStatus(nodeId:string){return stepRuns.value.find(s=>s.nodeId===nodeId)?.status||"";}
function nodeStatusClass(nodeId:string){const s=stepStatus(nodeId);return s?`run-${s}`:"";}
function nodeLabel(id:string){return workflow.nodes.find(n=>n.id===id)?.label||id;}
function formatTime(v?:string){return v?new Date(v).toLocaleString():"";}
function onInspectorTabChanged(name:string|number){if(String(name)==="runs")void refreshRuns();}

watch(()=>workflow.name,markDirty);watch(()=>workflow.description,markDirty);
watch(selectedNodeId,()=>{syncNodeEditors();mappingTargetPath.value="";mappingSourceRef.value="";});
watch(selectedEdgeId,()=>{edgeConditionEditor.value=pretty(selectedEdge.value?.condition);});

onMounted(async()=>{const [loaded,items]=await Promise.all([getWorkflow(workflowId),getWorkflowCatalog().catch(()=>[] as WorkflowCatalogItem[])]);Object.assign(workflow,loaded);catalog.value=items;normalizeLoadedDefinition();dirty.value=false;history.value=[modelSnapshot()];historyIndex.value=0;await Promise.all([refreshRuns(),refreshRevisions()]);setTimeout(fitView,0);});
onBeforeUnmount(()=>{if(pollTimer)clearInterval(pollTimer);window.removeEventListener("pointermove",onGlobalPointerMove);});
</script>

<style scoped>
.builder-page{height:100%;min-height:0;display:flex;flex-direction:column;color:var(--console-text);overflow:hidden}.builder-header{display:flex;align-items:center;justify-content:space-between;gap:18px;padding:2px 2px 14px;border-bottom:1px solid var(--console-border)}.title-area{min-width:0;flex:1}.back-link{display:inline-block;margin-bottom:5px;color:var(--console-text-muted);font-size:12px;text-decoration:none}.title-row{display:flex;align-items:center;gap:10px}.name-input{max-width:420px}.name-input :deep(.el-input__wrapper),.description-input :deep(.el-input__wrapper){box-shadow:none;background:transparent;padding-left:0}.name-input :deep(.el-input__inner){font-size:20px;font-weight:650}.description-input{max-width:560px}.description-input :deep(.el-input__inner){font-size:12px;color:var(--console-text-muted)}.dirty-dot,.saved-state{font-size:11px;white-space:nowrap}.dirty-dot{color:var(--el-color-warning)}.saved-state{color:var(--console-text-muted)}.header-actions{display:flex;gap:7px;align-items:center}.builder-layout{flex:1;min-height:0;display:grid;grid-template-columns:210px minmax(0,1fr) 310px}.palette-panel,.inspector-panel{min-height:0;overflow:auto;background:var(--ac-color-surface);padding:14px}.palette-panel{border-right:1px solid var(--console-border)}.inspector-panel{border-left:1px solid var(--console-border);padding:0 12px 16px}.panel-title{font-size:13px;font-weight:650;margin-bottom:9px}.panel-title.small{font-size:12px}.panel-tip{margin:0 0 12px;color:var(--console-text-muted);font-size:11px;line-height:1.5}.palette-item{width:100%;display:grid;grid-template-columns:34px 1fr;align-items:center;text-align:left;gap:9px;border:1px solid transparent;border-radius:9px;background:transparent;color:var(--console-text);padding:8px;cursor:grab}.palette-item:hover{background:var(--ac-color-surface-soft);border-color:var(--console-border)}.palette-item strong,.palette-item small{display:block}.palette-item strong{font-size:12px}.palette-item small{font-size:10px;color:var(--console-text-muted);margin-top:2px}.node-type-icon{width:32px;height:32px;border-radius:8px;background:var(--ac-color-surface-soft);display:grid;place-items:center;color:var(--el-color-primary);font-size:11px;font-weight:700}.palette-divider{height:1px;background:var(--console-border);margin:13px 0}.canvas-tools.vertical{display:grid;grid-template-columns:1fr 1fr;gap:6px}.canvas-tools :deep(.el-button){margin-left:0}.workflow-canvas{position:relative;min-width:0;min-height:0;overflow:hidden;background:var(--ac-color-page,#0f0f10);touch-action:none}.canvas-grid{position:absolute;inset:0;background-image:radial-gradient(circle,var(--console-border) 1px,transparent 1px);opacity:.55}.graph-transform{position:absolute;left:0;top:0;width:4000px;height:2600px;transform-origin:0 0}.edge-layer{position:absolute;inset:0;width:4000px;height:2600px;overflow:visible}.edge-line{fill:none;stroke:var(--console-text-muted);stroke-width:1.5;opacity:.72}.edge-line.selected{stroke:var(--el-color-primary);stroke-width:2.5;opacity:1}.edge-line.preview{stroke:var(--el-color-primary);stroke-dasharray:6 4}.edge-hit{fill:none;stroke:transparent;stroke-width:14;cursor:pointer}.edge-label{font-size:11px;fill:var(--console-text-muted);text-anchor:middle}.workflow-node{position:absolute;width:180px;height:84px;border:1px solid var(--console-border);border-radius:11px;background:var(--ac-color-surface);box-shadow:0 5px 18px rgba(0,0,0,.12);user-select:none}.workflow-node.selected{border-color:var(--el-color-primary);box-shadow:0 0 0 1px var(--el-color-primary)}.workflow-node.run-running{border-color:var(--el-color-warning)}.workflow-node.run-succeeded,.workflow-node.run-defaulted{border-color:var(--el-color-success)}.workflow-node.run-failed{border-color:var(--el-color-danger)}.workflow-node.run-skipped{opacity:.58}.node-header{height:48px;display:grid;grid-template-columns:32px 1fr 18px;gap:8px;align-items:center;padding:7px 9px;cursor:grab}.node-badge{width:30px;height:30px;border-radius:8px;background:var(--ac-color-surface-soft);display:grid;place-items:center;color:var(--el-color-primary);font-size:10px;font-weight:700}.node-title{min-width:0}.node-title strong,.node-title small{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.node-title strong{font-size:12px}.node-title small{font-size:9px;color:var(--console-text-muted);margin-top:2px}.node-menu{font-size:14px;color:var(--console-text-muted);cursor:pointer}.node-body{height:35px;border-top:1px solid var(--console-border);display:flex;align-items:center;justify-content:space-between;gap:6px;padding:0 9px;font-size:9px}.target-text{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.muted{color:var(--console-text-muted)}.status-pill{font-size:8px;text-transform:uppercase;color:var(--el-color-primary)}.input-handle,.output-handle{position:absolute;top:34px;width:14px;height:14px;border-radius:50%;border:2px solid var(--ac-color-surface);background:var(--console-text-muted);padding:0;cursor:crosshair;z-index:3}.input-handle{left:-8px}.output-handle{right:-8px;background:var(--el-color-primary)}.zoom-indicator{position:absolute;left:12px;bottom:12px;padding:5px 8px;border:1px solid var(--console-border);border-radius:7px;background:var(--ac-color-surface);font-size:10px;color:var(--console-text-muted)}.minimap{position:absolute;right:12px;bottom:12px;width:150px;height:94px;border:1px solid var(--console-border);border-radius:8px;background:var(--ac-color-surface);overflow:hidden;opacity:.88}.minimap svg{width:100%;height:100%}.mini-node{fill:var(--console-text-muted);opacity:.55}.connect-hint{position:absolute;left:50%;bottom:14px;transform:translateX(-50%);padding:7px 11px;border-radius:8px;background:var(--ac-color-surface);border:1px solid var(--el-color-primary);font-size:10px}.inspector-content{display:flex;flex-direction:column;gap:11px;padding:4px 3px}.inspector-content label{display:flex;flex-direction:column;gap:5px;font-size:11px;color:var(--console-text-muted)}.inspector-content :deep(.el-select),.inspector-content :deep(.el-input-number){width:100%}.empty-inspector{padding:30px 4px;text-align:center;color:var(--console-text-muted);font-size:12px}.edge-summary,.definition-meta{padding:8px;border-radius:8px;background:var(--ac-color-surface-soft);font-size:10px;color:var(--console-text-muted);word-break:break-all}.panel-row,.trigger-head,.current-run-card,.run-history{display:flex;align-items:center;justify-content:space-between;gap:8px}.trigger-card{display:flex;flex-direction:column;gap:9px;padding:10px;border:1px solid var(--console-border);border-radius:10px}.trigger-head strong{font-size:11px}.current-run-card{align-items:flex-start;padding:10px;border-radius:9px;background:var(--ac-color-surface-soft)}.current-run-card strong,.current-run-card small{display:block}.current-run-card small{font-size:9px;color:var(--console-text-muted);word-break:break-all;margin-top:3px}.run-actions{display:flex;gap:4px;flex-wrap:wrap;justify-content:flex-end}.trace-step{display:grid;grid-template-columns:10px 1fr;gap:8px;padding:8px;border:1px solid var(--console-border);border-radius:8px;cursor:pointer}.trace-status{width:8px;height:8px;border-radius:50%;margin-top:4px;background:var(--console-text-muted)}.trace-status.s-succeeded,.trace-status.s-defaulted{background:var(--el-color-success)}.trace-status.s-failed{background:var(--el-color-danger)}.trace-status.s-running{background:var(--el-color-warning)}.trace-step strong,.trace-step small{display:block}.trace-step strong{font-size:11px}.trace-step small{font-size:9px;color:var(--console-text-muted);margin-top:2px}.trace-step p{margin:4px 0 0;color:var(--el-color-danger);font-size:9px;word-break:break-word}.run-history-title{margin-top:8px}.run-history{padding:8px;border-bottom:1px solid var(--console-border);cursor:pointer}.run-history strong,.run-history small{display:block;font-size:10px}.run-history small,.run-history>span{color:var(--console-text-muted);font-size:9px}.mapping-list{display:flex;flex-direction:column;gap:6px}.mapping-row{display:flex;align-items:center;justify-content:space-between;gap:6px;padding:8px;border:1px solid var(--console-border);border-radius:8px}.mapping-row span{min-width:0}.mapping-row strong,.mapping-row small{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.mapping-row strong{font-size:10px}.mapping-row small{font-size:9px;color:var(--console-text-muted);margin-top:2px}.empty-inspector.compact{padding:12px 4px}.ai-actions{display:flex;gap:6px;flex-wrap:wrap}.ai-result{padding:10px;border:1px solid var(--console-border);border-radius:9px;background:var(--ac-color-surface-soft);font-size:10px}.ai-result strong{font-size:11px}.ai-result p{margin:6px 0;color:var(--console-text-muted);line-height:1.5}.ai-result ul{margin:6px 0 0;padding-left:18px;line-height:1.5}.revision-card{display:flex;align-items:center;justify-content:space-between;gap:8px;padding:9px;border:1px solid var(--console-border);border-radius:8px}.revision-main{min-width:0}.revision-main strong,.revision-main small,.revision-main span{display:block}.revision-main strong{font-size:10px}.revision-main small,.revision-main span{font-size:9px;color:var(--console-text-muted);margin-top:2px}.revision-main span{font-family:monospace}
@media(max-width:1050px){.builder-layout{grid-template-columns:170px minmax(0,1fr) 270px}}@media(max-width:760px){.builder-header{align-items:flex-start;flex-direction:column}.header-actions{flex-wrap:wrap}.builder-layout{grid-template-columns:1fr}.palette-panel{display:none}.inspector-panel{position:absolute;right:0;top:106px;bottom:0;width:min(86vw,320px);z-index:8;box-shadow:-8px 0 24px rgba(0,0,0,.18)}.workflow-canvas{min-height:600px}}
</style>
