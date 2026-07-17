<template>
  <div class="extension-page" v-loading="loading">
    <ExtensionPageHeader :title="skill?.name || '技能详情'">
      <template v-if="skill" #meta>
        <div class="detail-heading-meta">
          <code>{{ skill.id }}</code>
          <div class="tag-list">
            <el-tag :type="skill.enabled ? 'success' : 'info'">{{ skill.enabled ? "已启用" : "已禁用" }}</el-tag>
            <el-tag :type="skill.compatible ? 'success' : 'danger'">{{ skill.compatible ? "兼容" : "不兼容" }}</el-tag>
          </div>
        </div>
      </template>
      <template v-if="skill" #actions>
        <div class="header-actions">
          <el-button v-if="skill.source === 'workflow' || skill.source === 'instructions'" @click="router.push({ path: '/extensions/packages', query: { id: skill.id } })">包与版本</el-button>
          <el-button v-if="skill.source === 'workflow'" :loading="forking" @click="forkRevision">创建新 Revision</el-button>
          <el-button :icon="Lock" @click="permissionVisible = true">管理权限</el-button>
          <el-button :type="skill.enabled ? 'danger' : 'primary'" plain :loading="changing" @click="toggleSkill">{{ skill.enabled ? "禁用" : "启用" }}</el-button>
        </div>
      </template>
    </ExtensionPageHeader>

    <el-alert v-if="loadError" :title="loadError" type="error" show-icon :closable="false">
      <template #default><el-button link type="primary" @click="load">重新加载</el-button></template>
    </el-alert>

    <template v-if="skill">
      <el-card shadow="never">
        <p class="description">{{ skill.description }}</p>
        <el-descriptions :column="3" border class="summary-descriptions">
          <el-descriptions-item label="版本">{{ skill.version }}</el-descriptions-item>
          <el-descriptions-item label="来源">{{ sourceLabel(skill.source) }}</el-descriptions-item>
          <el-descriptions-item label="入口">{{ skill.entry.name }}</el-descriptions-item>
          <el-descriptions-item label="作者">{{ skill.author || "—" }}</el-descriptions-item>
          <el-descriptions-item label="许可证">{{ skill.license || "—" }}</el-descriptions-item>
          <el-descriptions-item label="超时">{{ skill.timeoutMs }} ms</el-descriptions-item>
          <el-descriptions-item label="副作用">{{ skill.hasSideEffects ? "有" : "无" }}</el-descriptions-item>
          <el-descriptions-item label="幂等">{{ skill.idempotent ? "支持" : "不支持" }}</el-descriptions-item>
          <el-descriptions-item label="自动重试">{{ skill.retryable ? "有限重试" : "不重试" }}</el-descriptions-item>
        </el-descriptions>
      </el-card>

      <el-tabs v-model="activeTab" class="detail-tabs">
        <el-tab-pane label="能力与协议" name="protocol">
          <div class="two-column-grid">
            <el-card shadow="never">
              <template #header><span>Capability</span></template>
              <div v-if="skill.capabilities.length" class="capability-list">
                <div v-for="name in skill.capabilities" :key="name" class="capability-item">
                  <div><code>{{ name }}</code><p>{{ capabilityFor(name)?.description || "未知能力" }}</p></div>
                  <el-tag :type="riskType(capabilityFor(name)?.risk)">{{ riskLabel(capabilityFor(name)?.risk) }}</el-tag>
                </div>
              </div>
              <el-empty v-else description="此技能不声明 Capability" :image-size="64" />
            </el-card>
            <el-card shadow="never">
              <template #header><span>触发方式</span></template>
              <div class="tag-list">
                <el-tag v-for="trigger in skill.triggers" :key="trigger" type="info">{{ triggerLabel(trigger) }}</el-tag>
              </div>
              <el-alert v-if="!skill.triggers.includes('manual')" title="该技能不支持手动执行" type="info" :closable="false" show-icon class="inline-alert" />
            </el-card>
          </div>
          <div class="schema-grid">
            <el-card shadow="never"><template #header>Manifest</template><pre>{{ pretty(skill.manifest) }}</pre></el-card>
            <el-card shadow="never"><template #header>输入 Schema</template><pre>{{ pretty(skill.inputSchema) }}</pre></el-card>
            <el-card shadow="never"><template #header>输出 Schema</template><pre>{{ pretty(skill.outputSchema) }}</pre></el-card>
          </div>
        </el-tab-pane>

        <el-tab-pane label="配置" name="config">
          <el-card shadow="never" class="editor-card">
            <el-form label-position="top" @submit.prevent>
              <el-form-item label="全局配置 JSON">
                <el-input v-model="configText" type="textarea" :rows="14" spellcheck="false" aria-label="全局配置 JSON" />
                <div class="field-help">配置会按技能的 Config Schema 校验。包含 Secret 的字段不会在页面或日志中明文显示。</div>
              </el-form-item>
              <div v-if="configError" class="field-error" role="alert">{{ configError }}</div>
              <div class="form-actions">
                <el-button :loading="resetting" @click="resetSkillConfig">恢复默认</el-button>
                <el-button type="primary" :loading="savingConfig" @click="saveConfig">保存配置</el-button>
              </div>
            </el-form>
          </el-card>
        </el-tab-pane>

        <el-tab-pane label="手动测试" name="test">
          <el-card shadow="never" class="editor-card">
            <el-alert v-if="!skill.enabled" title="技能已禁用，手动执行会被拒绝" type="warning" show-icon :closable="false" />
            <el-form label-position="top" @submit.prevent>
              <el-form-item label="输入 JSON">
                <el-input v-model="testInput" type="textarea" :rows="12" spellcheck="false" aria-label="技能测试输入 JSON" />
                <div class="field-help">前端只检查 JSON 格式，最终字段和类型由后端 Schema Validator 校验。</div>
              </el-form-item>
              <div v-if="testError" class="field-error" role="alert">{{ testError }}</div>
              <div class="form-actions">
                <el-input v-model="idempotencyKey" clearable placeholder="可选：幂等键" aria-label="幂等键" />
                <el-button type="primary" :loading="executing" :disabled="!skill.triggers.includes('manual')" @click="runTest">执行技能</el-button>
              </div>
            </el-form>
          </el-card>
          <el-card v-if="testResult" shadow="never" class="result-card">
            <template #header><div class="result-header"><span>执行结果</span><el-tag :type="statusType(testResult.status)">{{ statusLabel(testResult.status) }}</el-tag></div></template>
            <el-descriptions :column="3" border>
              <el-descriptions-item label="runId" :span="2"><code>{{ testResult.runId }}</code></el-descriptions-item>
              <el-descriptions-item label="耗时">{{ testResult.durationMs }} ms</el-descriptions-item>
              <el-descriptions-item v-if="testResult.error" label="错误码">{{ testResult.error.code }}</el-descriptions-item>
              <el-descriptions-item v-if="testResult.error" label="错误详情" :span="2">{{ testResult.error.detail || testResult.error.message }}</el-descriptions-item>
            </el-descriptions>
            <pre v-if="testResult.output">{{ pretty(testResult.output) }}</pre>
            <pre v-if="testResult.sideEffects?.length">{{ pretty(testResult.sideEffects) }}</pre>
          </el-card>
        </el-tab-pane>

        <el-tab-pane label="最近运行" name="runs">
          <el-card shadow="never" class="table-card">
            <el-table :data="skill.recentRuns" empty-text="暂无运行记录" stripe>
              <el-table-column prop="startedAt" label="开始时间" min-width="170"><template #default="{ row }">{{ formatTime(row.startedAt) }}</template></el-table-column>
              <el-table-column prop="trigger" label="触发" width="100"><template #default="{ row }">{{ triggerLabel(row.trigger) }}</template></el-table-column>
              <el-table-column prop="channel" label="渠道" width="100" />
              <el-table-column prop="status" label="状态" width="120"><template #default="{ row }"><el-tag :type="statusType(row.status)" size="small">{{ statusLabel(row.status) }}</el-tag></template></el-table-column>
              <el-table-column prop="durationMs" label="耗时" width="100"><template #default="{ row }">{{ row.durationMs }} ms</template></el-table-column>
              <el-table-column prop="traceId" label="traceId" min-width="180" show-overflow-tooltip />
            </el-table>
            <div class="runs-footer"><el-button @click="router.push('/extensions/runs')">查看全部执行记录</el-button></div>
          </el-card>
        </el-tab-pane>

        <el-tab-pane v-if="skill.source === 'workflow'" label="版本历史" name="versions">
          <el-card shadow="never" class="table-card">
            <el-table :data="skill.versions" empty-text="暂无历史版本" stripe>
              <el-table-column prop="version" label="版本" width="140"><template #default="{ row }"><strong>{{ row.version }}</strong><el-tag v-if="row.version === skill.version" size="small" type="success">当前</el-tag></template></el-table-column>
              <el-table-column prop="checksum" label="Checksum" min-width="220" show-overflow-tooltip />
              <el-table-column prop="createdAt" label="安装时间" min-width="170"><template #default="{ row }">{{ formatTime(row.createdAt) }}</template></el-table-column>
              <el-table-column label="操作" width="190" fixed="right"><template #default="{ row }"><el-button link type="primary" @click="compareVersion(row)">比较</el-button><el-button link type="warning" :disabled="row.version === skill.version" :loading="rollingBack === row.version" @click="rollbackVersion(row.version)">回滚</el-button></template></el-table-column>
            </el-table>
          </el-card>
        </el-tab-pane>
      </el-tabs>
    </template>

    <el-dialog v-model="compareVisible" :title="`版本比较：${compareTarget?.version || ''} ↔ ${skill?.version || ''}`" width="min(1080px, 94vw)" destroy-on-close>
      <div class="version-compare">
        <section><h3>{{ compareTarget?.version }} Manifest</h3><pre>{{ pretty(compareTarget?.manifest) }}</pre></section>
        <section><h3>{{ skill?.version }} Manifest</h3><pre>{{ pretty(skill?.manifest) }}</pre></section>
      </div>
      <template #footer><el-button @click="compareVisible = false">关闭</el-button></template>
    </el-dialog>

    <PermissionDialog
      v-if="skill"
      v-model="permissionVisible"
      :capability-names="skill.capabilities"
      :catalog="capabilityCatalog"
      :grants="skill.permissions"
      :character-id="characterId"
      :saving="savingPermissions"
      @save="savePermissions"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue"
import { useRoute, useRouter } from "vue-router"
import { ElMessage, ElMessageBox } from "element-plus"
import { Lock } from "@element-plus/icons-vue"
import ExtensionPageHeader from "./components/ExtensionPageHeader.vue"
import PermissionDialog from "./components/PermissionDialog.vue"
import { executeSkill, fetchCapabilities, fetchSkill, forkWorkflowSkill, resetConfig, resolveCharacterId, rollbackWorkflowSkill, setSkillEnabled, updateConfig, updatePermissions } from "./api"
import type { CapabilityDefinition, PermissionGrant, RunStatus, SkillDetail, SkillResult, SkillTrigger } from "./types"

const route = useRoute()
const router = useRouter()
const skillId = computed(() => String(route.params.id || ""))
const skill = ref<SkillDetail>()
const capabilityCatalog = ref<CapabilityDefinition[]>([])
const characterId = ref("")
const loading = ref(false)
const loadError = ref("")
const activeTab = ref("protocol")
const changing = ref(false)
const permissionVisible = ref(false)
const savingPermissions = ref(false)
const configText = ref("{}")
const configError = ref("")
const savingConfig = ref(false)
const resetting = ref(false)
const testInput = ref("{}")
const testError = ref("")
const idempotencyKey = ref("")
const executing = ref(false)
const testResult = ref<SkillResult>()
const forking = ref(false)
const rollingBack = ref("")
const compareVisible = ref(false)
const compareTarget = ref<SkillDetail["versions"][number]>()

async function load() {
  loading.value = true
  loadError.value = ""
  try {
    if (!characterId.value) characterId.value = await resolveCharacterId()
    if (!characterId.value) throw new Error("请先创建或选择角色")
    const [detail, catalog] = await Promise.all([fetchSkill(characterId.value, skillId.value), fetchCapabilities()])
    skill.value = detail
    capabilityCatalog.value = catalog
    configText.value = pretty(detail.config || {})
    testInput.value = buildDefaultInput(detail.inputSchema)
  } catch (error: any) {
    loadError.value = problemDetail(error) || "技能详情加载失败"
  } finally {
    loading.value = false
  }
}

async function toggleSkill() {
  if (!skill.value) return
  if (skill.value.enabled) await ElMessageBox.confirm("禁用后模型和手动入口都不能执行该技能。", "确认禁用", { type: "warning" })
  changing.value = true
  try {
    await setSkillEnabled(skill.value.id, !skill.value.enabled)
    ElMessage.success(skill.value.enabled ? "技能已禁用" : "技能已启用")
    await load()
  } finally {
    changing.value = false
  }
}

async function forkRevision() {
  if (!skill.value) return
  forking.value = true
  try {
    const session = await forkWorkflowSkill(skill.value.id, characterId.value)
    ElMessage.success(`已创建 ${session.revision?.normalizedDraft.metadata.version || "新版本"} Revision`)
    await router.push(`/extensions/workshop/${session.id}`)
  } finally {
    forking.value = false
  }
}

async function rollbackVersion(version: string) {
  if (!skill.value) return
  await ElMessageBox.confirm(`将 ${skill.value.id} 回滚到 ${version}，现有授权不会自动恢复。`, "确认回滚", { type: "warning", confirmButtonText: "回滚", cancelButtonText: "取消" })
  rollingBack.value = version
  try {
    await rollbackWorkflowSkill(skill.value.id, version, characterId.value)
    ElMessage.success(`已回滚到 ${version}`)
    await load()
  } finally {
    rollingBack.value = ""
  }
}

function compareVersion(version: SkillDetail["versions"][number]) {
  compareTarget.value = version
  compareVisible.value = true
}

async function savePermissions(grants: PermissionGrant[]) {
  if (!skill.value) return
  savingPermissions.value = true
  try {
    await updatePermissions(skill.value.id, characterId.value, grants)
    ElMessage.success("权限已更新并立即生效")
    permissionVisible.value = false
    await load()
  } finally {
    savingPermissions.value = false
  }
}

async function saveConfig() {
  if (!skill.value) return
  configError.value = ""
  let value: unknown
  try {
    value = JSON.parse(configText.value)
  } catch (error: any) {
    configError.value = `JSON 格式错误：${error.message}`
    return
  }
  savingConfig.value = true
  try {
    await updateConfig(skill.value.id, value)
    ElMessage.success("配置已保存")
    await load()
  } catch (error: any) {
    configError.value = problemDetail(error) || "配置保存失败"
  } finally {
    savingConfig.value = false
  }
}

async function resetSkillConfig() {
  if (!skill.value) return
  resetting.value = true
  try {
    await resetConfig(skill.value.id)
    ElMessage.success("已恢复默认配置")
    await load()
  } finally {
    resetting.value = false
  }
}

async function runTest() {
  if (!skill.value) return
  testError.value = ""
  let input: unknown
  try {
    input = JSON.parse(testInput.value)
  } catch (error: any) {
    testError.value = `JSON 格式错误：${error.message}`
    return
  }
  executing.value = true
  try {
    testResult.value = await executeSkill(skill.value.id, { characterId: characterId.value, channel: "web", idempotencyKey: idempotencyKey.value || undefined, input })
    await load()
    activeTab.value = "test"
  } catch (error: any) {
    const responseResult = error?.raw?.result || error?.response?.data?.result
    if (responseResult) testResult.value = responseResult
    testError.value = problemDetail(error) || "技能执行失败"
  } finally {
    executing.value = false
  }
}

function buildDefaultInput(schema: any) {
  const value: Record<string, unknown> = {}
  for (const name of schema?.required || []) {
    const property = schema?.properties?.[name]
    if (property?.type === "string") value[name] = ""
    else if (property?.type === "integer" || property?.type === "number") value[name] = 0
    else if (property?.type === "boolean") value[name] = false
    else if (property?.type === "array") value[name] = []
    else value[name] = {}
  }
  return pretty(value)
}

function capabilityFor(name: string) {
  return capabilityCatalog.value.find((item) => item.name === name)
}

function pretty(value: unknown) {
  return JSON.stringify(value ?? {}, null, 2)
}

function problemDetail(error: any) {
  return error?.response?.data?.detail || error?.detail || error?.message
}

function sourceLabel(source: string) {
  return source === "legacy_tool" ? "旧工具适配" : source === "builtin" ? "内置技能" : source
}

function triggerLabel(trigger: SkillTrigger) {
  return { llm: "模型调用", manual: "手动执行", schedule: "定时协议", system_event: "系统事件协议" }[trigger]
}

function riskType(risk?: string) {
  if (risk === "high") return "danger"
  if (risk === "medium") return "warning"
  return "success"
}

function riskLabel(risk?: string) {
  if (risk === "high") return "高风险"
  if (risk === "medium") return "中风险"
  return "低风险"
}

function statusLabel(status: RunStatus) {
  return { pending: "等待中", running: "运行中", succeeded: "成功", failed: "失败", cancelled: "已取消", timed_out: "超时", partially_succeeded: "部分成功" }[status]
}

function statusType(status: RunStatus) {
  if (status === "succeeded") return "success"
  if (status === "running" || status === "pending" || status === "partially_succeeded") return "warning"
  if (status === "failed" || status === "timed_out") return "danger"
  return "info"
}

function formatTime(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString("zh-CN", { hour12: false })
}

onMounted(load)
</script>

<style scoped>
.extension-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 0;
}

.page-header,
.title-row,
.detail-heading-meta,
.header-actions,
.tag-list,
.result-header,
.form-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.page-header,
.title-row,
.result-header {
  justify-content: space-between;
}

.page-header {
  align-items: flex-end;
}

.title-block {
  min-width: 0;
  flex: 1;
}

.title-row {
  margin-top: 8px;
}

h1 {
  color: var(--ac-color-text);
  font-size: 24px;
  line-height: 32px;
}

.detail-heading-meta code,
.capability-item code {
  color: var(--ac-color-text-secondary);
  overflow-wrap: anywhere;
}

.description {
  margin-bottom: 16px;
  color: var(--ac-color-text-secondary);
  line-height: 1.7;
}

.two-column-grid,
.schema-grid {
  display: grid;
  gap: 16px;
}

.two-column-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.schema-grid {
  grid-template-columns: repeat(3, minmax(0, 1fr));
  margin-top: 16px;
}

.capability-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.capability-item {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--ac-color-border-light);
}

.capability-item:last-child {
  padding-bottom: 0;
  border-bottom: 0;
}

.capability-item p,
.field-help {
  margin-top: 6px;
  color: var(--ac-color-text-muted);
  font-size: var(--ac-font-size-sm);
  line-height: 1.6;
}

pre {
  max-height: 420px;
  overflow: auto;
  padding: 14px;
  border-radius: var(--ac-radius-sm);
  background: var(--ac-color-bg-secondary);
  color: var(--ac-color-text);
  font-family: "SFMono-Regular", Consolas, monospace;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.inline-alert,
.result-card {
  margin-top: 16px;
}

.editor-card {
  max-width: 900px;
}

.editor-card :deep(textarea) {
  font-family: "SFMono-Regular", Consolas, monospace;
}

.form-actions {
  justify-content: flex-end;
}

.form-actions .el-input {
  max-width: 360px;
  margin-right: auto;
}

.field-error {
  margin: -8px 0 14px;
  color: var(--ac-color-danger);
  line-height: 1.5;
}

.result-card pre {
  margin-top: 16px;
}

.table-card :deep(.el-card__body) {
  padding: 0;
  overflow-x: auto;
}

.runs-footer {
  display: flex;
  justify-content: flex-end;
  padding: 16px;
}

.version-compare {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.version-compare section {
  min-width: 0;
}

.version-compare h3 {
  margin-bottom: 8px;
  color: var(--ac-color-text);
}

@media (max-width: 1120px) {
  .schema-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}

@media (max-width: 720px) {
  .page-header,
  .title-row,
  .two-column-grid {
    align-items: stretch;
    grid-template-columns: minmax(0, 1fr);
    flex-direction: column;
  }

  .header-actions,
  .form-actions {
    align-items: stretch;
    flex-direction: column;
  }

  .form-actions .el-input {
    max-width: none;
    margin-right: 0;
  }

  .summary-descriptions :deep(.el-descriptions__body) {
    overflow-x: auto;
  }

  .version-compare {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
