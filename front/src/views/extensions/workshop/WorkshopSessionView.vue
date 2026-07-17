<template>
  <main class="session-page" v-loading="loading">
    <ExtensionPageHeader :title="draft?.metadata.name || '扩展工坊会话'" :description="session?.requirement">
      <template #actions><div class="header-status"><el-tag v-if="session" :type="statusType(session.status)">{{ statusLabel(session.status) }}</el-tag><span v-if="session?.currentRevision">Revision {{ session.currentRevision }}</span></div></template>
    </ExtensionPageHeader>

    <el-alert title="本工坊只创建声明式 Skill" type="info" :closable="false" show-icon>不能生成 Plugin、源码、JavaScript、Shell 或 SQL；所有安装结果默认禁用。</el-alert>

    <el-steps :active="activeStep" finish-status="success" align-center class="progress" aria-label="工坊进度">
      <el-step v-for="item in steps" :key="item" :title="item" />
    </el-steps>

    <el-card v-if="!session?.currentRevision" shadow="never" class="stage-card">
      <h2>1. 描述需求</h2>
      <p class="stage-help">确认需求后，模型只会返回受严格 Schema 限制的结构化 Draft。</p>
      <el-input v-model="requirement" type="textarea" :rows="8" maxlength="20000" show-word-limit aria-label="Skill 需求描述" />
      <div class="actions"><el-button type="primary" :loading="working === 'generate'" :disabled="!requirement.trim()" @click="generate">生成结构化草案</el-button></div>
    </el-card>

    <template v-else-if="draft">
      <el-card shadow="never" class="stage-card">
        <div class="card-heading"><div><h2>2. 查看生成草案</h2><p class="stage-help">模型建议仅供审阅，后端分析器会重新计算权限、幂等性和副作用。</p></div><el-button @click="editorOpen = true">结构化编辑</el-button></div>
        <el-descriptions :column="2" border>
          <el-descriptions-item label="名称">{{ draft.metadata.name }}</el-descriptions-item><el-descriptions-item label="版本">{{ draft.metadata.version }}</el-descriptions-item>
          <el-descriptions-item label="Skill ID" :span="2"><code>{{ draft.metadata.id }}</code></el-descriptions-item><el-descriptions-item label="描述" :span="2">{{ draft.metadata.description }}</el-descriptions-item>
          <el-descriptions-item label="触发方式"><el-tag v-for="trigger in draft.intent.triggers" :key="trigger" size="small">{{ trigger }}</el-tag></el-descriptions-item><el-descriptions-item label="依赖">{{ draft.dependencies.length || "无" }}</el-descriptions-item>
        </el-descriptions>
        <section v-if="plan?.goal" class="plan-summary">
          <h3>结构化需求计划</h3>
          <p>{{ plan.goal }}</p>
          <div class="plan-grid">
            <div><strong>步骤</strong><el-tag v-for="step in plan.steps" :key="step.id" size="small">{{ step.id }} · {{ step.type }}</el-tag></div>
            <div><strong>风险</strong><span v-if="!plan.risks.length">无</span><span v-for="risk in plan.risks" :key="risk">{{ risk }}</span></div>
            <div><strong>待确认</strong><span v-if="!plan.missingDetails.length">无</span><span v-for="detail in plan.missingDetails" :key="detail">{{ detail }}</span></div>
            <div><strong>假设</strong><span v-if="!plan.assumptions.length">无</span><span v-for="assumption in plan.assumptions" :key="assumption">{{ assumption }}</span></div>
          </div>
        </section>
        <el-alert v-for="warning in draft.warnings" :key="`${warning.code}-${warning.path}`" :title="warning.message" type="warning" :closable="false" show-icon />
      </el-card>

      <el-card shadow="never" class="stage-card">
        <h2>3. 检查输入、输出与配置</h2>
        <p class="stage-help">编辑器只解析 JSON，不执行其中任何文本。保存会创建新 Revision，并使旧权限和测试失效。</p>
        <el-tabs v-model="schemaTab">
          <el-tab-pane label="Input Schema" name="input"><pre>{{ pretty(draft.inputSchema) }}</pre></el-tab-pane>
          <el-tab-pane label="Output Schema" name="output"><pre>{{ pretty(draft.outputSchema) }}</pre></el-tab-pane>
          <el-tab-pane label="Config Schema" name="config"><pre>{{ pretty(draft.configSchema) }}</pre></el-tab-pane>
          <el-tab-pane label="默认配置" name="defaults"><pre>{{ pretty(draft.defaultConfig) }}</pre></el-tab-pane>
        </el-tabs>
      </el-card>

      <el-card shadow="never" class="stage-card">
        <h2>4. 检查声明式工作流</h2>
        <p class="stage-help">按顺序执行，仅允许九种白名单步骤，不支持循环、任意跳转或代码表达式。</p>
        <ol class="workflow-list">
          <li v-for="step in draft.workflow.steps" :key="step.id"><span class="step-index">{{ step.id }}</span><el-tag>{{ step.type }}</el-tag><span>失败策略：{{ step.onError?.mode || "fail" }}</span><pre>{{ pretty(step.input) }}</pre></li>
        </ol>
        <h3>最终输出</h3><pre>{{ pretty(draft.workflow.output) }}</pre>
      </el-card>

      <el-card shadow="never" class="stage-card">
        <h2>5. 查看权限和风险</h2>
        <CapabilityRiskList :capabilities="capabilityAnalysis.required" :high-risk="capabilityAnalysis.highRisk" :by-step="capabilityAnalysis.byStep" :confirmed="confirmedCapabilities" :confirmable="Boolean(validation?.valid)" @toggle="toggleCapability" />
        <div v-if="validation?.valid" class="actions"><el-button type="warning" :loading="working === 'permissions'" :disabled="!allCapabilitiesConfirmed || testPermissionConfirmed" @click="confirmPermissions(false)">{{ testPermissionConfirmed ? "测试权限已确认" : "确认测试权限" }}</el-button></div>
        <p v-else class="stage-help">先执行校验，校验通过后才能确认权限。</p>
      </el-card>

      <el-card shadow="never" class="stage-card">
        <h2>6. 执行校验</h2>
        <div class="actions start"><el-button type="primary" :loading="working === 'validate'" @click="validate">重新校验当前 Revision</el-button><span v-if="validation">Checksum：<code>{{ validation.workflowChecksum.slice(0, 16) }}</code></span></div>
        <div v-if="validation" class="issues" aria-live="polite">
          <el-alert :title="validation.valid ? '校验通过' : '校验未通过'" :type="validation.valid ? 'success' : 'error'" show-icon :closable="false" />
          <el-collapse>
            <el-collapse-item v-for="group in issueGroups" :key="group.level" :name="group.level" :title="`${levelLabel(group.level)}（${group.items.length}）`">
              <div v-for="issue in group.items" :key="`${issue.code}-${issue.path}-${issue.stepId}`" class="issue-row"><el-tag :type="issue.level === 'error' ? 'danger' : issue.level === 'warning' ? 'warning' : 'info'" size="small">{{ issue.code }}</el-tag><span>{{ issue.message }}</span><code v-if="issue.path">{{ issue.path }}</code><code v-if="issue.stepId">步骤 {{ issue.stepId }}</code></div>
            </el-collapse-item>
          </el-collapse>
        </div>
      </el-card>

      <el-card shadow="never" class="stage-card">
        <h2>7. 配置测试</h2>
        <el-form label-position="top">
          <el-form-item label="测试模式">
            <el-radio-group v-model="testMode">
              <el-radio-button value="dry_run">Dry Run</el-radio-button><el-radio-button value="mocked">Mocked</el-radio-button><el-radio-button value="controlled_live">Controlled Live</el-radio-button>
            </el-radio-group>
          </el-form-item>
          <el-alert v-if="testMode === 'dry_run'" title="不会产生任何真实副作用" type="info" :closable="false" />
          <el-alert v-else-if="testMode === 'mocked'" title="HTTP 与 Skill 调用必须命中当前测试用例的 Mock" type="info" :closable="false" />
          <el-alert v-else title="高级高风险模式：仅允许白名单网络，禁止真实渠道消息、正式记忆和长期任务" type="warning" :closable="false" show-icon />
          <el-checkbox v-if="testMode === 'controlled_live'" v-model="controlledConfirmed">我确认使用受限 Controlled Live 测试</el-checkbox>
        </el-form>
      </el-card>

      <el-card shadow="never" class="stage-card">
        <h2>8. 运行测试</h2>
        <p class="stage-help">测试结果绑定 Revision 与 Workflow Checksum，草案变化后自动失效。</p>
        <div class="actions start"><el-button type="primary" :loading="working === 'test'" :disabled="!testPermissionConfirmed || (testMode === 'controlled_live' && !controlledConfirmed)" @click="runTest">运行 {{ testMode }}</el-button></div>
      </el-card>

      <el-card shadow="never" class="stage-card">
        <h2>9. 查看测试结果</h2>
        <TestResultViewer :report="latestReport" />
        <el-empty v-if="!latestReport" description="还没有当前 Revision 的测试结果" :image-size="70" />
      </el-card>

      <el-card shadow="never" class="stage-card install-card">
        <h2>10. 确认安装</h2>
        <el-descriptions :column="2" border><el-descriptions-item label="Skill ID">{{ draft.metadata.id }}</el-descriptions-item><el-descriptions-item label="版本">{{ draft.metadata.version }}</el-descriptions-item><el-descriptions-item label="权限">{{ draft.capabilities.length }} 项</el-descriptions-item><el-descriptions-item label="安装状态">安装后禁用</el-descriptions-item></el-descriptions>
        <el-alert v-if="session?.status === 'installed'" title="已安装、未启用" type="success" show-icon :closable="false"><template #default><el-button link type="primary" @click="router.push(`/extensions/skills/${encodeURIComponent(session.installedSkillId || '')}`)">前往 Skill 详情</el-button></template></el-alert>
        <div v-else class="actions"><el-button type="warning" :loading="working === 'production-permissions'" :disabled="latestReport?.status !== 'passed' || !allCapabilitiesConfirmed || productionPermissionConfirmed" @click="confirmPermissions(true)">{{ productionPermissionConfirmed ? "生产权限已确认" : "单独确认生产权限" }}</el-button><el-button type="primary" :loading="working === 'install'" :disabled="latestReport?.status !== 'passed' || !productionPermissionConfirmed" @click="install">安装当前版本</el-button></div>
      </el-card>
    </template>

    <el-dialog v-model="editorOpen" title="结构化 Draft 编辑器" width="min(1100px, 96vw)" :close-on-click-modal="false">
      <StructuredDraftEditor v-if="draft" :key="session?.currentRevision" :draft="draft" :saving="working === 'save'" @cancel="editorOpen = false" @save="saveDraft" />
    </el-dialog>
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue"
import { useRoute, useRouter } from "vue-router"
import { ElMessage, ElMessageBox } from "element-plus"
import ExtensionPageHeader from "../components/ExtensionPageHeader.vue"
import { confirmWorkshopPermissions, fetchWorkshopSession, generateWorkshopDraft, installWorkshopDraft, resolveCharacterId, testWorkshopDraft, validateWorkshopDraft } from "../api"
import type { AnalysisIssue, CapabilityAnalysis, ExtensionDraft, WorkshopSessionDetail, WorkshopStatus, WorkshopTestReport, WorkshopValidation } from "../types"
import CapabilityRiskList from "./components/CapabilityRiskList.vue"
import TestResultViewer from "./components/TestResultViewer.vue"
import StructuredDraftEditor from "./components/StructuredDraftEditor.vue"

const route = useRoute()
const router = useRouter()
const id = String(route.params.id)
const steps = ["需求", "草案", "Schema", "工作流", "权限", "校验", "测试配置", "运行测试", "测试结果", "安装"]
const loading = ref(false)
const working = ref("")
const characterId = ref("")
const session = ref<WorkshopSessionDetail>()
const requirement = ref("")
const validation = ref<WorkshopValidation>()
const latestReport = ref<WorkshopTestReport>()
const schemaTab = ref("input")
const testMode = ref<"dry_run" | "mocked" | "controlled_live">("dry_run")
const controlledConfirmed = ref(false)
const testPermissionConfirmed = ref(false)
const productionPermissionConfirmed = ref(false)
const confirmedCapabilities = ref<string[]>([])
const editorOpen = ref(false)
const draft = computed(() => session.value?.revision?.normalizedDraft)
const plan = computed(() => session.value?.revision?.plan)
const capabilityAnalysis = computed<CapabilityAnalysis>(() => validation.value?.capabilities || { required: draft.value?.capabilities || [], declared: draft.value?.capabilities || [], missing: [], excess: [], byStep: {}, highRisk: [] })
const allCapabilitiesConfirmed = computed(() => capabilityAnalysis.value.required.every((capability) => confirmedCapabilities.value.includes(capability)))
const activeStep = computed(() => { if (!session.value?.currentRevision) return 0; if (session.value.status === "installed") return 10; if (latestReport.value?.status === "passed") return 9; if (latestReport.value) return 8; if (testPermissionConfirmed.value) return 7; if (validation.value?.valid) return 5; return 1 })
const issueGroups = computed(() => (["error", "warning", "info"] as const).map((level) => ({ level, items: (validation.value?.issues || []).filter((issue) => issue.level === level) })).filter((group) => group.items.length))

async function load() {
  loading.value = true
  try {
    if (!characterId.value) characterId.value = await resolveCharacterId()
    session.value = await fetchWorkshopSession(id, characterId.value)
    requirement.value = session.value.requirement
    validation.value = session.value.revision?.validation
    latestReport.value = session.value.testReports.find((report) => report.revision === session.value?.currentRevision && report.workflowChecksum === session.value?.revision?.workflowChecksum)
    testPermissionConfirmed.value = session.value.testPermissionConfirmed
    productionPermissionConfirmed.value = session.value.productionPermissionConfirmed
    if (testPermissionConfirmed.value || productionPermissionConfirmed.value) confirmedCapabilities.value = [...(validation.value?.capabilities.required || draft.value?.capabilities || [])]
  } catch (error: any) { ElMessage.error(error?.response?.data?.detail || error?.message || "会话加载失败") } finally { loading.value = false }
}

async function perform(name: string, action: () => Promise<void>) { working.value = name; try { await action() } catch (error: any) { ElMessage.error(error?.response?.data?.detail || error?.message || "操作失败") } finally { working.value = "" } }
async function generate() { await perform("generate", async () => { await generateWorkshopDraft(id, characterId.value, { requirement: requirement.value.trim() }); ElMessage.success("结构化草案已生成"); await load() }) }
async function validate() { await perform("validate", async () => { validation.value = await validateWorkshopDraft(id, session.value!.currentRevision, characterId.value); testPermissionConfirmed.value = false; productionPermissionConfirmed.value = false; confirmedCapabilities.value = []; ElMessage[validation.value.valid ? "success" : "warning"](validation.value.valid ? "校验通过" : "校验发现需要修复的问题"); await load() }) }
function toggleCapability(capability: string, value: boolean) { if (value) { if (!confirmedCapabilities.value.includes(capability)) confirmedCapabilities.value.push(capability) } else { confirmedCapabilities.value = confirmedCapabilities.value.filter((item) => item !== capability) } }
async function confirmPermissions(production: boolean) { await perform(production ? "production-permissions" : "permissions", async () => { await confirmWorkshopPermissions(id, session.value!.currentRevision, characterId.value, { workflowChecksum: validation.value!.workflowChecksum, capabilities: capabilityAnalysis.value.required, confirmedHighRisk: capabilityAnalysis.value.highRisk.filter((item) => confirmedCapabilities.value.includes(item)), production }); if (production) productionPermissionConfirmed.value = true; else testPermissionConfirmed.value = true; ElMessage.success(production ? "生产权限已独立绑定当前 Revision" : "测试权限已绑定当前 Revision"); await load() }) }
async function runTest() { await perform("test", async () => { latestReport.value = await testWorkshopDraft(id, session.value!.currentRevision, characterId.value, { mode: testMode.value, controlledLiveConfirmed: controlledConfirmed.value }); ElMessage[latestReport.value.status === "passed" ? "success" : "warning"](latestReport.value.status === "passed" ? "测试通过" : "测试未通过"); await load() }) }
async function install() { await ElMessageBox.confirm(`将安装 ${draft.value!.metadata.id}@${draft.value!.metadata.version}，安装后保持禁用。`, "确认安装", { type: "warning", confirmButtonText: "安装", cancelButtonText: "取消" }); await perform("install", async () => { await installWorkshopDraft(id, session.value!.currentRevision, characterId.value); ElMessage.success("Skill 已安装，当前未启用"); await load() }) }
async function saveDraft(parsed: ExtensionDraft) { await perform("save", async () => { await generateWorkshopDraft(id, characterId.value, { draft: parsed }); editorOpen.value = false; testPermissionConfirmed.value = false; productionPermissionConfirmed.value = false; confirmedCapabilities.value = []; latestReport.value = undefined; ElMessage.success("已创建新 Revision，旧权限和测试结果已失效"); await load() }) }
function pretty(value: unknown) { return JSON.stringify(value, null, 2) }
function levelLabel(level: AnalysisIssue["level"]) { return { error: "错误", warning: "警告", info: "信息" }[level] }
function statusLabel(status: WorkshopStatus) { return ({ draft: "待生成", generating: "生成中", generated: "草案已生成", validating: "校验中", validation_failed: "校验失败", validated: "校验通过", awaiting_permission_confirmation: "权限已确认", testing: "测试中", test_failed: "测试失败", test_passed: "测试通过", installing: "安装中", installed: "已安装·未启用", enabled: "已启用", disabled: "已禁用", archived: "已归档", error: "异常" } as Record<string, string>)[status] || status }
function statusType(status: WorkshopStatus) { if (["installed", "enabled", "test_passed", "validated"].includes(status)) return "success"; if (["validation_failed", "test_failed", "error"].includes(status)) return "danger"; if (["generating", "validating", "testing", "installing"].includes(status)) return "warning"; return "info" }
onMounted(load)
</script>

<style scoped>
.session-page { display: flex; flex-direction: column; gap: 16px; height: 100%; overflow: auto; }
.page-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 20px; }
.page-header h1 { margin: 8px 0; color: var(--console-text); }
.page-header p, .stage-help { margin: 0; color: var(--console-text-muted); line-height: 1.6; }
.header-status { display: flex; align-items: center; gap: 10px; white-space: nowrap; color: var(--console-text-muted); font-variant-numeric: tabular-nums; }
.progress { padding: 18px 6px; overflow-x: auto; }
.stage-card h2 { margin: 0 0 8px; color: var(--console-text); font-size: 20px; }
.stage-card h3, .stage-card h4 { color: var(--console-text); }
.stage-card { flex-shrink: 0; }
.card-heading, .actions, .actions.start { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.actions { justify-content: flex-end; margin-top: 18px; }
.actions.start { justify-content: flex-start; align-items: center; }
pre { max-height: 360px; padding: 14px; overflow: auto; border: 1px solid var(--console-border); border-radius: 8px; background: var(--console-code-bg, rgba(127, 127, 127, 0.08)); color: var(--console-text); font-size: 12px; line-height: 1.6; white-space: pre-wrap; overflow-wrap: anywhere; }
.workflow-list { display: grid; gap: 12px; padding: 0; list-style: none; }
.workflow-list li { display: grid; grid-template-columns: minmax(100px, auto) auto 1fr; align-items: center; gap: 10px; padding: 14px; border: 1px solid var(--console-border); border-radius: 10px; }
.workflow-list pre { grid-column: 1 / -1; width: 100%; margin: 0; }
.step-index { font-weight: 600; }
.plan-summary { margin-top: 16px; padding: 14px; border: 1px solid var(--console-border); border-radius: 10px; }
.plan-summary p { color: var(--console-text-muted); line-height: 1.6; }
.plan-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; }
.plan-grid > div { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
.plan-grid strong { width: 100%; color: var(--console-text); }
.issues { display: grid; gap: 12px; margin-top: 16px; }
.issue-row { display: grid; grid-template-columns: auto minmax(220px, 1fr) auto auto; align-items: center; gap: 10px; padding: 8px 0; border-bottom: 1px solid var(--console-border-soft); }
.install-card { margin-bottom: 24px; }
code { overflow-wrap: anywhere; }
@media (max-width: 900px) { .progress { align-items: flex-start; } .workflow-list li { grid-template-columns: 1fr auto; } .issue-row { grid-template-columns: auto 1fr; } .issue-row code { grid-column: 2; } }
@media (max-width: 720px) { .page-header, .card-heading { flex-direction: column; } .header-status { white-space: normal; } .actions .el-button, .actions.start .el-button { min-height: 44px; } .stage-card :deep(.el-descriptions__body) { overflow-x: auto; } .plan-grid { grid-template-columns: minmax(0, 1fr); } }
@media (prefers-reduced-motion: reduce) { .session-page * { scroll-behavior: auto !important; transition-duration: 0.01ms !important; animation-duration: 0.01ms !important; } }
</style>
