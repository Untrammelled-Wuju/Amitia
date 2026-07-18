<template>
  <div class="mcp-page">
    <ExtensionPageHeader title="MCP 服务" description="连接外部 MCP Server，并按角色、工具和权限控制 AI 可使用的能力。">
      <template #actions>
        <el-button :icon="Refresh" :loading="loading" @click="load">刷新</el-button>
        <el-button type="primary" :icon="Plus" @click="openCreate">添加服务</el-button>
      </template>
    </ExtensionPageHeader>

    <section class="summary-grid" aria-label="MCP 服务状态摘要">
      <div class="summary-item"><span>服务总数</span><strong>{{ servers.length }}</strong></div>
      <div class="summary-item"><span>已连接</span><strong>{{ summary.ready }}</strong></div>
      <div class="summary-item"><span>需要授权</span><strong>{{ summary.authorization }}</strong></div>
      <div class="summary-item"><span>异常</span><strong>{{ summary.unhealthy }}</strong></div>
    </section>

    <el-alert v-if="errorText" :title="errorText" type="error" show-icon :closable="false">
      <template #default><el-button link type="primary" @click="load">重新加载</el-button></template>
    </el-alert>

    <el-card shadow="never" class="server-card">
      <div class="table-toolbar">
        <el-input v-model="query" :prefix-icon="Search" clearable placeholder="搜索名称、地址或命令" aria-label="搜索 MCP 服务" />
        <el-select v-model="statusFilter" clearable placeholder="全部状态" aria-label="按状态筛选">
          <el-option label="已连接" value="ready" />
          <el-option label="已断开" value="disconnected" />
          <el-option label="需要授权" value="authorization_required" />
          <el-option label="异常" value="degraded" />
        </el-select>
      </div>
      <el-table v-loading="loading" :data="filteredServers" row-key="id" empty-text="暂无 MCP 服务" stripe>
        <el-table-column label="服务" min-width="240">
          <template #default="{ row }">
            <button class="server-link" type="button" @click="openDetails(row)">
              <span>{{ row.displayName || row.name }}</span>
              <code>{{ row.transport === 'stdio' ? row.command : row.endpoint }}</code>
            </button>
          </template>
        </el-table-column>
        <el-table-column label="传输" width="120"><template #default="{ row }"><el-tag type="info" effect="plain">{{ transportLabel(row.transport) }}</el-tag></template></el-table-column>
        <el-table-column label="认证" width="120"><template #default="{ row }">{{ authLabel(row.authType) }}</template></el-table-column>
        <el-table-column label="协议" width="120"><template #default="{ row }"><span class="muted">{{ row.protocolVersion || "—" }}</span></template></el-table-column>
        <el-table-column label="状态" width="130"><template #default="{ row }"><el-tag :type="statusType(row.status)">{{ statusLabel(row.status) }}</el-tag></template></el-table-column>
        <el-table-column label="最近连接" min-width="150"><template #default="{ row }"><span class="muted">{{ formatTime(row.lastConnectedAt) }}</span></template></el-table-column>
        <el-table-column label="操作" width="280" fixed="right">
          <template #default="{ row }">
            <div class="row-actions">
              <el-button @click="openDetails(row)">详情</el-button>
              <el-button v-if="row.status !== 'ready'" type="primary" plain :loading="actingId === row.id" @click="connect(row)">连接</el-button>
              <el-button v-else :loading="actingId === row.id" @click="disconnect(row)">断开</el-button>
              <el-dropdown trigger="click" @command="(command: string) => handleCommand(command, row)">
                <el-button :icon="MoreFilled" aria-label="更多操作" />
                <template #dropdown><el-dropdown-menu><el-dropdown-item command="refresh">刷新能力</el-dropdown-item><el-dropdown-item command="edit">编辑配置</el-dropdown-item><el-dropdown-item v-if="row.authType === 'oauth'" command="oauth">授权</el-dropdown-item><el-dropdown-item divided command="delete">删除</el-dropdown-item></el-dropdown-menu></template>
              </el-dropdown>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="formVisible" :title="editingId ? '编辑 MCP 服务' : '添加 MCP 服务'" width="min(680px, calc(100vw - 32px))" destroy-on-close @closed="resetForm">
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" @submit.prevent="save">
        <div class="form-grid">
          <el-form-item label="名称" prop="name"><el-input v-model="form.name" maxlength="80" placeholder="例如 github" /></el-form-item>
          <el-form-item label="显示名称"><el-input v-model="form.displayName" maxlength="120" placeholder="例如 GitHub MCP" /></el-form-item>
        </div>
        <el-form-item label="说明"><el-input v-model="form.description" type="textarea" :rows="2" maxlength="500" /></el-form-item>
        <div class="form-grid">
          <el-form-item label="传输方式" prop="transport"><el-segmented v-model="form.transport" :options="transportOptions" /></el-form-item>
          <el-form-item label="认证方式" prop="authType"><el-select v-model="form.authType"><el-option v-for="option in authOptions" :key="option.value" :label="option.label" :value="option.value" /></el-select></el-form-item>
        </div>
        <template v-if="form.transport === 'streamable_http'">
          <el-form-item label="Server URL" prop="endpoint"><el-input v-model="form.endpoint" placeholder="https://example.com/mcp" /><div class="field-help">公网连接必须使用 HTTPS；本机 HTTP 仅允许 loopback 地址。</div></el-form-item>
          <el-alert title="内网地址默认拒绝。只有你确认该地址属于可信私网服务时，才允许连接。" type="info" show-icon :closable="false" />
          <el-form-item class="private-confirm"><el-checkbox v-model="form.privateNetworkConfirmed">我确认允许连接该配置解析到的私网地址</el-checkbox></el-form-item>
        </template>
        <template v-else>
          <el-alert title="stdio 会启动本地进程；Amitia 不会自动下载、安装或通过 Shell 执行程序。" type="warning" show-icon :closable="false" />
          <div class="form-grid local-fields"><el-form-item label="命令" prop="command"><el-input v-model="form.command" placeholder="已安装的可执行程序" /></el-form-item><el-form-item label="工作目录"><el-input v-model="form.workDir" placeholder="可选" /></el-form-item></div>
          <el-form-item label="参数"><el-input v-model="form.argsText" type="textarea" :rows="3" placeholder="每行一个参数" /><div class="field-help">参数按行拆分，不经过 Shell。</div></el-form-item>
        </template>
        <el-form-item v-if="credentialVisible" :label="credentialLabel">
          <el-input v-model="form.credentialText" :type="form.authType === 'bearer_token' ? 'password' : 'textarea'" :rows="3" show-password :placeholder="credentialPlaceholder" autocomplete="off" />
          <div class="field-help">{{ credentialHelp }}</div>
        </el-form-item>
        <el-form-item><el-checkbox v-model="form.enabled">保存后启用并连接</el-checkbox></el-form-item>
      </el-form>
      <template #footer><el-button @click="formVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="save">{{ editingId ? "保存修改" : "添加服务" }}</el-button></template>
    </el-dialog>

    <el-drawer v-model="detailsVisible" size="min(860px, 92vw)" destroy-on-close>
      <template #header><div class="drawer-title"><div><strong>{{ selected?.displayName || selected?.name }}</strong><span>{{ selected?.description || "暂无说明" }}</span></div><el-tag :type="statusType(selected?.status || '')">{{ statusLabel(selected?.status || '') }}</el-tag></div></template>
      <el-tabs v-model="detailTab" @tab-change="loadDetails">
        <el-tab-pane label="概览" name="overview">
          <el-descriptions :column="2" border><el-descriptions-item label="服务 ID">{{ selected?.id }}</el-descriptions-item><el-descriptions-item label="来源">{{ selected?.source }}</el-descriptions-item><el-descriptions-item label="传输">{{ transportLabel(selected?.transport || 'streamable_http') }}</el-descriptions-item><el-descriptions-item label="认证">{{ authLabel(selected?.authType || 'none') }}</el-descriptions-item><el-descriptions-item label="协议">{{ selected?.protocolVersion || "—" }}</el-descriptions-item><el-descriptions-item label="最近连接">{{ formatTime(selected?.lastConnectedAt || '') }}</el-descriptions-item></el-descriptions>
          <el-alert v-if="selected?.lastErrorMessage" :title="selected.lastErrorCode || '连接异常'" :description="selected.lastErrorMessage" type="error" show-icon :closable="false" class="detail-alert" />
          <section class="capability-panel">
            <div class="section-heading"><div><strong>客户端能力</strong><span>能力按服务单独授权，修改后会重新连接并更新协议声明。</span></div></div>
            <div class="capability-list">
              <div v-for="item in capabilityItems" :key="item.name" class="capability-row">
                <div><strong>{{ item.label }}</strong><span>{{ item.description }}</span></div>
                <el-switch :model-value="capabilityEnabled(item.name)" :loading="capabilitySaving === item.name" @change="(value: string | number | boolean) => toggleCapability(item.name, Boolean(value))" />
              </div>
            </div>
            <div v-if="capabilityEnabled('sampling')" class="sampling-limits">
              <el-alert title="Sampling 会允许此服务反向请求模型，每次仍需用户批准；模型凭据不会提供给服务。" type="warning" show-icon :closable="false" />
              <div class="limit-grid"><label>最大 Token<el-input-number v-model="samplingConfig.maxTokens" :min="1" :max="8192" :step="256" /></label><label>超时（秒）<el-input-number v-model="samplingConfig.timeoutSeconds" :min="1" :max="300" /></label><label>最大并发<el-input-number v-model="samplingConfig.maxConcurrent" :min="1" :max="4" /></label></div>
              <el-checkbox v-model="samplingConfig.toolsEnabled" disabled>允许 Sampling 使用工具（尚未授权任何工具）</el-checkbox>
              <el-button :loading="capabilitySaving === 'sampling'" @click="saveSamplingLimits">保存限额</el-button>
            </div>
            <div v-if="capabilityEnabled('roots')" class="roots-panel">
              <el-alert v-if="!canSelectMCPRoot" title="普通 Web 页面不能授权服务器读取本地目录，请在 Amitia 桌面端中配置 Roots。" type="info" show-icon :closable="false" />
              <div v-for="(root, index) in authorizedRoots" :key="root.uri" class="root-item"><div><strong>{{ root.name }}</strong><code>{{ root.uri }}</code></div><el-button type="danger" link :loading="capabilitySaving === 'roots'" @click="removeRoot(index)">移除</el-button></div>
              <el-button v-if="canSelectMCPRoot" :loading="capabilitySaving === 'roots'" @click="addRoot">选择并授权目录</el-button>
              <span v-if="!authorizedRoots.length" class="empty-roots">尚未授权目录，服务请求 Roots 时会得到空列表。</span>
            </div>
            <div v-if="capabilityEnabled('tasks')" class="sampling-limits">
              <el-alert title="Tasks 是实验能力；任务按服务隔离，到期记录会清理，删除服务时会一并移除。" type="info" show-icon :closable="false" />
              <div class="limit-grid"><label>最大并发<el-input-number v-model="tasksConfig.maxConcurrent" :min="1" :max="20" /></label><label>最大 TTL（秒）<el-input-number v-model="tasksConfig.maxTTLSeconds" :min="60" :max="604800" :step="60" /></label></div>
              <el-button :loading="capabilitySaving === 'tasks'" @click="saveTaskLimits">保存限额</el-button>
            </div>
          </section>
          <div class="detail-actions"><el-button :icon="Refresh" :loading="detailLoading" @click="refreshSelected">重新发现能力</el-button><el-button v-if="selected?.authType === 'oauth'" type="primary" plain @click="authorize(selected)">开始授权</el-button></div>
        </el-tab-pane>
        <el-tab-pane :label="`工具 ${tools.length}`" name="tools">
          <el-table v-loading="detailLoading" :data="tools" empty-text="暂无工具"><el-table-column label="工具" min-width="250"><template #default="{ row }"><div class="definition-cell"><strong>{{ row.title || row.remoteName }}</strong><code>{{ row.remoteName }}</code><span>{{ row.description }}</span></div></template></el-table-column><el-table-column label="风险" width="90"><template #default="{ row }"><el-tag :type="riskType(row.riskLevel)" size="small">{{ riskLabel(row.riskLevel) }}</el-tag></template></el-table-column><el-table-column label="启用" width="90"><template #default="{ row }"><el-switch :model-value="row.enabled === 1" :loading="actingId === row.id" @change="(value: string | number | boolean) => toggleTool(row, Boolean(value))" /></template></el-table-column></el-table>
        </el-tab-pane>
        <el-tab-pane :label="`资源 ${resources.length}`" name="resources"><el-table v-loading="detailLoading" :data="resources" empty-text="暂无资源"><el-table-column prop="name" label="名称" min-width="180" /><el-table-column prop="uri" label="URI" min-width="300"><template #default="{ row }"><code>{{ row.uri }}</code></template></el-table-column><el-table-column prop="mimeType" label="类型" width="130" /><el-table-column label="操作" width="150"><template #default="{ row }"><el-button link type="primary" @click="openResource(row)">读取</el-button><el-button link :loading="resourceAction === row.uri" @click="toggleResourceSubscription(row)">{{ subscribedResources.has(row.uri) ? '取消订阅' : '订阅' }}</el-button></template></el-table-column></el-table></el-tab-pane>
        <el-tab-pane :label="`Prompt ${prompts.length}`" name="prompts"><el-table v-loading="detailLoading" :data="prompts" empty-text="暂无 Prompt"><el-table-column label="名称" min-width="180"><template #default="{ row }">{{ row.title || row.remoteName }}</template></el-table-column><el-table-column prop="description" label="说明" min-width="260" /><el-table-column label="操作" width="90"><template #default="{ row }"><el-button link type="primary" @click="openPrompt(row)">使用</el-button></template></el-table-column></el-table></el-tab-pane>
        <el-tab-pane v-if="capabilityEnabled('tasks')" :label="`Tasks ${tasks.length}`" name="tasks"><el-table v-loading="detailLoading" :data="tasks" empty-text="暂无异步任务"><el-table-column prop="remoteTaskId" label="Task ID" min-width="180"><template #default="{ row }"><code>{{ row.remoteTaskId }}</code></template></el-table-column><el-table-column label="状态" width="130"><template #default="{ row }"><el-tag :type="taskStatusType(row.status)">{{ taskStatusLabel(row.status) }}</el-tag></template></el-table-column><el-table-column prop="statusMessage" label="说明" min-width="220" /><el-table-column label="到期" width="170"><template #default="{ row }">{{ formatTime(row.expiresAt) }}</template></el-table-column><el-table-column label="操作" width="90"><template #default="{ row }"><el-button v-if="row.status === 'working' || row.status === 'input_required'" link type="danger" :loading="taskAction === row.remoteTaskId" @click="cancelTask(row)">取消</el-button></template></el-table-column></el-table></el-tab-pane>
        <el-tab-pane label="日志" name="logs"><el-table v-loading="detailLoading" :data="logs" empty-text="暂无日志"><el-table-column prop="operation" label="操作" min-width="150" /><el-table-column prop="toolName" label="工具" min-width="150" /><el-table-column prop="status" label="状态" width="100" /><el-table-column label="时间" width="170"><template #default="{ row }">{{ formatTime(row.createdAt) }}</template></el-table-column></el-table></el-tab-pane>
      </el-tabs>
    </el-drawer>
    <el-dialog v-model="promptVisible" title="使用 MCP Prompt" width="min(640px, calc(100vw - 32px))" destroy-on-close>
      <template v-if="activePrompt">
        <div class="prompt-heading"><strong>{{ activePrompt.title || activePrompt.remoteName }}</strong><span>{{ activePrompt.description }}</span></div>
        <el-alert title="Prompt 内容来自外部 MCP Server，只会在你主动获取后显示，不会自动注入当前对话或覆盖系统提示。" type="info" show-icon :closable="false" />
        <el-form label-position="top" class="prompt-form">
          <el-form-item v-for="argument in activePromptArguments" :key="argument.name" :label="argument.name" :required="argument.required">
            <el-input v-model="promptValues[argument.name]" :placeholder="argument.description" maxlength="2000"><template #append><el-button :loading="completionLoading === argument.name" @click="loadCompletion(argument.name)">获取建议</el-button></template></el-input>
            <div v-if="completionValues[argument.name]?.length" class="completion-list"><button v-for="value in completionValues[argument.name]" :key="value" type="button" @click="promptValues[argument.name] = value">{{ value }}</button></div>
          </el-form-item>
        </el-form>
        <section v-if="promptResult" class="prompt-result"><div><strong>外部 Prompt 结果</strong><el-tag type="warning" size="small">不可信内容</el-tag></div><pre>{{ prettyPromptResult }}</pre></section>
      </template>
      <template #footer><el-button @click="promptVisible = false">关闭</el-button><el-button type="primary" :loading="promptLoading" @click="loadPromptResult">获取 Prompt</el-button></template>
    </el-dialog>
    <el-dialog v-model="resourceVisible" title="MCP Resource" width="min(680px, calc(100vw - 32px))" destroy-on-close>
      <template v-if="activeResource"><div class="prompt-heading"><strong>{{ activeResource.title || activeResource.name || activeResource.uri }}</strong><code>{{ activeResource.uri }}</code></div><el-alert title="资源内容来自外部 MCP Server，按需读取并标记为不可信，不会自动注入对话。" type="warning" show-icon :closable="false" /><section v-loading="resourceAction === activeResource.uri" class="prompt-result resource-result"><div><strong>资源内容</strong><el-tag type="warning" size="small">不可信内容</el-tag></div><pre>{{ prettyResourceResult }}</pre></section></template>
      <template #footer><el-button @click="resourceVisible = false">关闭</el-button><el-button :loading="resourceAction === activeResource?.uri" @click="reloadResource">重新读取</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue"
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from "element-plus"
import { MoreFilled, Plus, Refresh, Search } from "@element-plus/icons-vue"
import ExtensionPageHeader from "@/views/extensions/components/ExtensionPageHeader.vue"
import { cancelMCPTask, completeMCPArgument, connectMCPServer, createMCPServer, deleteMCPServer, disconnectMCPServer, getMCPPrompt, listMCPCapabilities, listMCPLogs, listMCPPrompts, listMCPResources, listMCPServers, listMCPTasks, listMCPTools, readMCPResource, refreshMCPServer, setMCPCapability, setMCPToolEnabled, startMCPOAuth, subscribeMCPResource, updateMCPServer } from "./api"
import type { MCPAuditLog, MCPAuthType, MCPCapabilityName, MCPPrompt, MCPResource, MCPSamplingConfiguration, MCPServer, MCPServerCapability, MCPServerForm, MCPTask, MCPTasksConfiguration, MCPTool, MCPTransport } from "./types"

const loading = ref(false)
const saving = ref(false)
const detailLoading = ref(false)
const errorText = ref("")
const actingId = ref("")
const query = ref("")
const statusFilter = ref("")
const servers = ref<MCPServer[]>([])
const formVisible = ref(false)
const detailsVisible = ref(false)
const editingId = ref("")
const selected = ref<MCPServer>()
const detailTab = ref("overview")
const tools = ref<MCPTool[]>([])
const resources = ref<MCPResource[]>([])
const prompts = ref<MCPPrompt[]>([])
const logs = ref<MCPAuditLog[]>([])
const capabilities = ref<MCPServerCapability[]>([])
const capabilitySaving = ref<MCPCapabilityName | "">("")
const samplingConfig = reactive<MCPSamplingConfiguration>({ maxTokens: 2048, timeoutSeconds: 60, maxConcurrent: 1, toolsEnabled: false })
const tasksConfig = reactive<MCPTasksConfiguration>({ maxConcurrent: 5, maxTTLSeconds: 86400 })
const authorizedRoots = ref<Array<{ uri: string; name: string }>>([])
const promptVisible = ref(false)
const promptLoading = ref(false)
const completionLoading = ref("")
const activePrompt = ref<MCPPrompt>()
const promptValues = reactive<Record<string, string>>({})
const completionValues = reactive<Record<string, string[]>>({})
const promptResult = ref<any>()
const resourceVisible = ref(false)
const resourceAction = ref("")
const activeResource = ref<MCPResource>()
const resourceResult = ref<any>()
const subscribedResources = reactive(new Set<string>())
const tasks = ref<MCPTask[]>([])
const taskAction = ref("")
const formRef = ref<FormInstance>()
const form = reactive<MCPServerForm>(emptyForm())

const transportOptions = [{ label: "HTTP", value: "streamable_http" }, { label: "本地 stdio", value: "stdio" }]
const authOptions: Array<{ label: string; value: MCPAuthType }> = [{ label: "无需认证", value: "none" }, { label: "OAuth", value: "oauth" }, { label: "Bearer Token", value: "bearer_token" }, { label: "自定义 Header", value: "custom_headers" }, { label: "stdio 环境变量", value: "stdio_env" }]
const capabilityItems: Array<{ name: MCPCapabilityName; label: string; description: string }> = [{ name: "roots", label: "Roots", description: "仅向该服务提供经桌面端明确授权的目录。" }, { name: "sampling", label: "Sampling", description: "允许服务在限额内请求 Amitia 模型，默认关闭。" }, { name: "elicitation", label: "Elicitation", description: "允许服务请求受限表单或外部 URL 确认。" }, { name: "tasks", label: "Tasks", description: "实验性异步任务、查询与取消能力。" }]
const rules: FormRules = { name: [{ required: true, message: "请输入名称", trigger: "blur" }], transport: [{ required: true }], endpoint: [{ validator: (_rule, value, callback) => form.transport !== "streamable_http" || value ? callback() : callback(new Error("请输入 Server URL")), trigger: "blur" }], command: [{ validator: (_rule, value, callback) => form.transport !== "stdio" || value ? callback() : callback(new Error("请输入命令")), trigger: "blur" }] }

const filteredServers = computed(() => {
  const needle = query.value.trim().toLowerCase()
  return servers.value.filter(server => (!statusFilter.value || server.status === statusFilter.value) && (!needle || [server.name, server.displayName, server.endpoint, server.command].some(value => value?.toLowerCase().includes(needle))))
})
const summary = computed(() => ({ ready: servers.value.filter(item => item.status === "ready").length, authorization: servers.value.filter(item => item.status === "authorization_required" || item.authType === "oauth" && item.status !== "ready").length, unhealthy: servers.value.filter(item => item.status === "degraded" || item.lastErrorCode).length }))
const credentialVisible = computed(() => form.authType === "bearer_token" || form.authType === "custom_headers" || form.authType === "stdio_env")
const credentialLabel = computed(() => form.authType === "bearer_token" ? "Token" : form.authType === "custom_headers" ? "Headers JSON" : "环境变量 JSON")
const credentialPlaceholder = computed(() => form.authType === "bearer_token" ? "输入 Token" : form.authType === "custom_headers" ? '{"X-API-Key":"..."}' : '{"API_KEY":"..."}')
const credentialHelp = computed(() => editingId.value ? "留空表示保留现有凭据。内容会加密保存，不会显示在页面或日志中。" : "内容会加密保存，数据库只记录 Secret Reference。")
const canSelectMCPRoot = computed(() => Boolean(window.amitiaDesktop?.selectMCPRoot))
const activePromptArguments = computed<Array<{ name: string; description: string; required: boolean }>>(() => { try { const value = JSON.parse(activePrompt.value?.arguments || "[]"); return Array.isArray(value) ? value.filter(item => typeof item?.name === "string").map(item => ({ name: item.name, description: String(item.description || ""), required: Boolean(item.required) })) : [] } catch { return [] } })
const prettyPromptResult = computed(() => JSON.stringify(promptResult.value, null, 2))
const prettyResourceResult = computed(() => JSON.stringify(resourceResult.value, null, 2))

function emptyForm(): MCPServerForm { return { name: "", displayName: "", description: "", transport: "streamable_http", endpoint: "", command: "", argsText: "", workDir: "", authType: "none", enabled: false, credentialText: "", privateNetworkConfirmed: false } }
function resetForm() { Object.assign(form, emptyForm()); editingId.value = ""; formRef.value?.clearValidate() }

async function load() { loading.value = true; errorText.value = ""; try { servers.value = await listMCPServers() } catch (error: any) { errorText.value = errorMessage(error, "MCP 服务加载失败") } finally { loading.value = false } }
function openCreate() { resetForm(); formVisible.value = true }
function openEdit(server: MCPServer) { editingId.value = server.id; Object.assign(form, { name: server.name, displayName: server.displayName, description: server.description, transport: server.transport, endpoint: server.endpoint, command: server.command, argsText: parseArgs(server.args).join("\n"), workDir: server.workDir, authType: server.authType, enabled: server.enabled === 1, credentialText: "", privateNetworkConfirmed: server.privateNetworkConfirmed }); formVisible.value = true }
async function save() { if (!await formRef.value?.validate().catch(() => false)) return; saving.value = true; try { if (editingId.value) await updateMCPServer(editingId.value, form); else await createMCPServer(form); ElMessage.success(editingId.value ? "配置已保存" : "服务已添加"); formVisible.value = false; await load() } catch (error: any) { ElMessage.error(errorMessage(error, "保存失败")) } finally { saving.value = false } }

async function connect(server: MCPServer) { actingId.value = server.id; try { await connectMCPServer(server.id); ElMessage.success("连接成功"); await load() } catch (error: any) { ElMessage.error(errorMessage(error, "连接失败")) } finally { actingId.value = "" } }
async function disconnect(server: MCPServer) { actingId.value = server.id; try { await disconnectMCPServer(server.id); ElMessage.success("已断开"); await load() } finally { actingId.value = "" } }
async function remove(server: MCPServer) { await ElMessageBox.confirm(`删除“${server.displayName || server.name}”后，其连接配置、能力缓存和专属凭据将被移除。共享依赖仍在使用时会阻止删除。`, "确认删除 MCP 服务", { type: "warning", confirmButtonText: "删除", cancelButtonText: "取消" }); await deleteMCPServer(server.id); ElMessage.success("服务已删除"); await load() }
async function handleCommand(command: string, server: MCPServer) { if (command === "edit") openEdit(server); if (command === "delete") await remove(server); if (command === "refresh") { selected.value = server; await refreshSelected() }; if (command === "oauth") await authorize(server) }

async function openDetails(server: MCPServer) { selected.value = server; detailTab.value = "overview"; tools.value = []; resources.value = []; prompts.value = []; logs.value = []; tasks.value = []; capabilities.value = []; detailsVisible.value = true; await loadCapabilities() }
async function loadDetails() { if (!selected.value || detailTab.value === "overview") return; detailLoading.value = true; try { if (detailTab.value === "tools") tools.value = await listMCPTools(selected.value.id); if (detailTab.value === "resources") resources.value = (await listMCPResources(selected.value.id)).resources || []; if (detailTab.value === "prompts") prompts.value = await listMCPPrompts(selected.value.id); if (detailTab.value === "tasks") tasks.value = await listMCPTasks(selected.value.id); if (detailTab.value === "logs") logs.value = await listMCPLogs(selected.value.id) } catch (error: any) { ElMessage.error(errorMessage(error, "详情加载失败")) } finally { detailLoading.value = false } }
async function refreshSelected() { if (!selected.value) return; detailLoading.value = true; try { await refreshMCPServer(selected.value.id); ElMessage.success("能力已刷新"); await load(); await loadDetails() } catch (error: any) { ElMessage.error(errorMessage(error, "刷新失败")) } finally { detailLoading.value = false } }
async function toggleTool(tool: MCPTool, enabled: boolean) { if (!selected.value) return; if (enabled) await ElMessageBox.confirm("启用只会让工具进入 Skill Runtime，实际调用仍需配置权限授权。", "确认启用工具", { type: "warning", confirmButtonText: "启用", cancelButtonText: "取消" }); actingId.value = tool.id; try { await setMCPToolEnabled(selected.value.id, tool.id, enabled); await loadDetails() } finally { actingId.value = "" } }
function capabilityEnabled(name: MCPCapabilityName) { return capabilities.value.some(item => item.capability === name && item.enabled === 1) }
async function loadCapabilities() { if (!selected.value) return; capabilities.value = await listMCPCapabilities(selected.value.id); const sampling = capabilities.value.find(item => item.capability === "sampling"); if (sampling) { try { Object.assign(samplingConfig, JSON.parse(sampling.configuration || "{}")) } catch { Object.assign(samplingConfig, { maxTokens: 2048, timeoutSeconds: 60, maxConcurrent: 1, toolsEnabled: false }) } } const tasks = capabilities.value.find(item => item.capability === "tasks"); if (tasks) { try { Object.assign(tasksConfig, JSON.parse(tasks.configuration || "{}")) } catch { Object.assign(tasksConfig, { maxConcurrent: 5, maxTTLSeconds: 86400 }) } } const roots = capabilities.value.find(item => item.capability === "roots"); authorizedRoots.value = []; if (roots) { try { const parsed = JSON.parse(roots.configuration || "{}"); authorizedRoots.value = Array.isArray(parsed.roots) ? parsed.roots.filter((item: any) => typeof item?.uri === "string" && item.uri.startsWith("file://")).slice(0, 20) : [] } catch {} } }
async function toggleCapability(name: MCPCapabilityName, enabled: boolean) { if (!selected.value) return; if (enabled) { const message = name === "sampling" ? "Sampling 是高风险递归能力。启用后，该服务可以发起模型请求，但每次请求仍必须经过用户批准并受限额约束。" : name === "roots" ? "Roots 只应包含你明确授权给此服务的目录，当前没有授权目录时会返回空列表。" : name === "elicitation" ? "Elicitation 可能请求你填写非敏感信息或打开外部链接，敏感字段会被拒绝。" : "Tasks 是实验能力，异步任务会绑定当前服务并受状态和结果大小限制。"; await ElMessageBox.confirm(message, `启用 ${name}`, { type: "warning", confirmButtonText: "确认启用", cancelButtonText: "取消" }) } capabilitySaving.value = name; try { const configuration = name === "sampling" ? { ...samplingConfig } : name === "roots" ? { roots: authorizedRoots.value } : name === "tasks" ? { ...tasksConfig } : {}; await setMCPCapability(selected.value.id, name, enabled, configuration); await loadCapabilities(); await load(); ElMessage.success(enabled ? "能力已启用并重新连接" : "能力已关闭并重新连接") } catch (error: any) { ElMessage.error(errorMessage(error, "能力更新失败")) } finally { capabilitySaving.value = "" } }
async function saveSamplingLimits() { if (!selected.value) return; capabilitySaving.value = "sampling"; try { await setMCPCapability(selected.value.id, "sampling", true, { ...samplingConfig }); await loadCapabilities(); ElMessage.success("Sampling 限额已保存") } catch (error: any) { ElMessage.error(errorMessage(error, "限额保存失败")) } finally { capabilitySaving.value = "" } }
async function saveTaskLimits() { if (!selected.value) return; capabilitySaving.value = "tasks"; try { await setMCPCapability(selected.value.id, "tasks", true, { ...tasksConfig }); await loadCapabilities(); ElMessage.success("Tasks 限额已保存") } catch (error: any) { ElMessage.error(errorMessage(error, "限额保存失败")) } finally { capabilitySaving.value = "" } }
function pathToFileURI(value: string) { const normalized = value.replace(/\\/g, "/"); return normalized.startsWith("//") ? `file:${encodeURI(normalized)}` : `file:///${encodeURI(normalized)}` }
async function saveRoots(next: Array<{ uri: string; name: string }>) { if (!selected.value) return; capabilitySaving.value = "roots"; try { await setMCPCapability(selected.value.id, "roots", true, { roots: next }); authorizedRoots.value = next; await loadCapabilities(); ElMessage.success("Roots 授权已更新") } catch (error: any) { ElMessage.error(errorMessage(error, "Roots 更新失败")) } finally { capabilitySaving.value = "" } }
async function addRoot() { const selectedRoot = await window.amitiaDesktop?.selectMCPRoot?.(); if (!selectedRoot) return; const root = { uri: pathToFileURI(selectedRoot.path), name: selectedRoot.name }; if (authorizedRoots.value.some(item => item.uri === root.uri)) return ElMessage.info("该目录已经授权"); if (authorizedRoots.value.length >= 20) return ElMessage.warning("每个服务最多授权 20 个目录"); await saveRoots([...authorizedRoots.value, root]) }
async function removeRoot(index: number) { await saveRoots(authorizedRoots.value.filter((_, itemIndex) => itemIndex !== index)) }
function openPrompt(prompt: MCPPrompt) { activePrompt.value = prompt; promptResult.value = undefined; for (const key of Object.keys(promptValues)) delete promptValues[key]; for (const key of Object.keys(completionValues)) delete completionValues[key]; for (const argument of (() => { try { return JSON.parse(prompt.arguments || "[]") } catch { return [] } })()) promptValues[argument.name] = ""; promptVisible.value = true }
async function loadCompletion(name: string) { if (!selected.value || !activePrompt.value) return; completionLoading.value = name; try { const result = await completeMCPArgument(selected.value.id, activePrompt.value.remoteName, name, promptValues[name] || "", { ...promptValues }); completionValues[name] = result.values || []; if (!completionValues[name].length) ElMessage.info("没有可用建议") } catch (error: any) { ElMessage.error(errorMessage(error, "补全失败")) } finally { completionLoading.value = "" } }
async function loadPromptResult() { if (!selected.value || !activePrompt.value) return; for (const argument of activePromptArguments.value) { if (argument.required && !promptValues[argument.name]?.trim()) return ElMessage.warning(`请填写“${argument.name}”`) } promptLoading.value = true; try { promptResult.value = await getMCPPrompt(selected.value.id, activePrompt.value.remoteName, { ...promptValues }) } catch (error: any) { ElMessage.error(errorMessage(error, "Prompt 获取失败")) } finally { promptLoading.value = false } }
async function openResource(resource: MCPResource) { activeResource.value = resource; resourceResult.value = undefined; resourceVisible.value = true; await reloadResource() }
async function reloadResource() { if (!selected.value || !activeResource.value) return; resourceAction.value = activeResource.value.uri; try { resourceResult.value = await readMCPResource(selected.value.id, activeResource.value.uri) } catch (error: any) { ElMessage.error(errorMessage(error, "资源读取失败")) } finally { resourceAction.value = "" } }
async function toggleResourceSubscription(resource: MCPResource) { if (!selected.value) return; const next = !subscribedResources.has(resource.uri); resourceAction.value = resource.uri; try { await subscribeMCPResource(selected.value.id, resource.uri, next); if (next) subscribedResources.add(resource.uri); else subscribedResources.delete(resource.uri); ElMessage.success(next ? "已订阅资源更新" : "已取消订阅") } catch (error: any) { ElMessage.error(errorMessage(error, "订阅更新失败")) } finally { resourceAction.value = "" } }
async function cancelTask(task: MCPTask) { if (!selected.value) return; await ElMessageBox.confirm(`取消任务 ${task.remoteTaskId}？`, "确认取消", { type: "warning", confirmButtonText: "取消任务", cancelButtonText: "返回" }); taskAction.value = task.remoteTaskId; try { await cancelMCPTask(selected.value.id, task.remoteTaskId); ElMessage.success("任务已取消"); await loadDetails() } catch (error: any) { ElMessage.error(errorMessage(error, "任务取消失败")) } finally { taskAction.value = "" } }
async function authorize(server: MCPServer) { try { const result = await startMCPOAuth(server.id, server.endpoint); window.open(result.authorizationUrl, "_blank", "noopener,noreferrer"); ElMessage.info("已打开授权页面，完成后返回此页面刷新连接") } catch (error: any) { ElMessage.error(errorMessage(error, "无法开始授权")) } }

function parseArgs(raw: string) { try { const value = JSON.parse(raw || "[]"); return Array.isArray(value) ? value.map(String) : [] } catch { return [] } }
function transportLabel(value: MCPTransport) { return value === "stdio" ? "本地 stdio" : "HTTP" }
function authLabel(value: MCPAuthType) { return { none: "无", oauth: "OAuth", bearer_token: "Bearer Token", custom_headers: "自定义 Header", stdio_env: "环境变量" }[value] || value }
function statusLabel(value: string) { return { ready: "已连接", disconnected: "已断开", connecting: "连接中", initializing: "初始化", reconnecting: "重连中", degraded: "异常", authorization_required: "需要授权", draft: "草稿" }[value] || value || "未知" }
function statusType(value: string) { if (value === "ready") return "success"; if (value === "connecting" || value === "initializing" || value === "reconnecting" || value === "authorization_required") return "warning"; if (value === "degraded") return "danger"; return "info" }
function taskStatusLabel(value: string) { return { working: "执行中", input_required: "等待输入", completed: "已完成", failed: "失败", cancelled: "已取消" }[value] || value }
function taskStatusType(value: string) { if (value === "completed") return "success"; if (value === "working" || value === "input_required") return "warning"; if (value === "failed") return "danger"; return "info" }
function riskLabel(value: string) { return { low: "低", medium: "中", high: "高" }[value] || value }
function riskType(value: string) { return value === "high" ? "danger" : value === "medium" ? "warning" : "success" }
function formatTime(value: string) { if (!value) return "—"; const date = new Date(value); return Number.isNaN(date.getTime()) ? value : date.toLocaleString() }
function errorMessage(error: any, fallback: string) { return error?.response?.data?.detail || error?.message || fallback }

onMounted(load)
</script>

<style scoped>
.mcp-page { display: flex; flex-direction: column; gap: 16px; min-width: 0; }
.summary-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
.summary-item { display: flex; align-items: center; justify-content: space-between; min-height: 68px; padding: 12px 16px; border: 1px solid var(--console-border); border-radius: 10px; background: var(--ac-color-surface); }
.summary-item span { color: var(--ac-color-text-secondary); font-size: var(--ac-font-size-sm); }
.summary-item strong { color: var(--ac-color-text); font-size: 22px; font-variant-numeric: tabular-nums; }
.server-card :deep(.el-card__body) { padding: 0; }
.table-toolbar { display: flex; gap: 12px; padding: 12px; border-bottom: 1px solid var(--console-border); }
.table-toolbar .el-input { max-width: 360px; }
.table-toolbar .el-select { width: 160px; }
.server-link { display: flex; flex-direction: column; align-items: flex-start; justify-content: center; gap: 4px; min-height: 44px; padding: 2px 0; border: 0; background: transparent; color: var(--ac-color-text); cursor: pointer; text-align: left; }
.server-link span { font-weight: 600; }
.server-link code, .definition-cell code, .muted { color: var(--ac-color-text-muted); font-size: var(--ac-font-size-xs); }
.server-link code { max-width: 420px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.server-link:focus-visible { outline: 2px solid var(--ac-color-primary); outline-offset: 3px; }
.row-actions, .detail-actions { display: flex; align-items: center; gap: 8px; }
.form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
.field-help { margin-top: 6px; color: var(--ac-color-text-muted); font-size: var(--ac-font-size-xs); line-height: 1.5; }
.local-fields { margin-top: 16px; }
.private-confirm { margin-top: 12px; }
.drawer-title { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; width: 100%; padding-right: 12px; }
.drawer-title > div { display: flex; flex-direction: column; gap: 4px; min-width: 0; }
.drawer-title strong { color: var(--ac-color-text); font-size: 18px; }
.drawer-title span { color: var(--ac-color-text-secondary); font-size: var(--ac-font-size-sm); }
.detail-alert, .detail-actions { margin-top: 16px; }
.capability-panel { margin-top: 16px; border: 1px solid var(--console-border); border-radius: 10px; overflow: hidden; }
.section-heading { padding: 14px 16px; border-bottom: 1px solid var(--console-border); background: var(--ac-color-surface); }
.section-heading > div, .capability-row > div { display: flex; flex-direction: column; gap: 4px; }
.section-heading span, .capability-row span { color: var(--ac-color-text-secondary); font-size: var(--ac-font-size-xs); line-height: 1.5; }
.capability-list { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); }
.capability-row { display: flex; align-items: center; justify-content: space-between; gap: 16px; min-height: 72px; padding: 12px 16px; border-bottom: 1px solid var(--console-border); }
.capability-row:nth-child(odd) { border-right: 1px solid var(--console-border); }
.sampling-limits { display: flex; flex-direction: column; align-items: flex-start; gap: 12px; padding: 16px; background: var(--ac-color-surface); }
.roots-panel { display: flex; flex-direction: column; align-items: flex-start; gap: 10px; padding: 16px; border-top: 1px solid var(--console-border); background: var(--ac-color-surface); }
.root-item { display: flex; align-items: center; justify-content: space-between; gap: 16px; width: 100%; padding: 10px 12px; border: 1px solid var(--console-border); border-radius: 8px; }
.root-item > div { display: flex; flex-direction: column; gap: 4px; min-width: 0; }
.root-item code, .empty-roots { color: var(--ac-color-text-secondary); font-size: var(--ac-font-size-xs); overflow-wrap: anywhere; }
.limit-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; width: 100%; }
.limit-grid label { display: flex; flex-direction: column; gap: 6px; color: var(--ac-color-text-secondary); font-size: var(--ac-font-size-xs); }
.limit-grid .el-input-number { width: 100%; }
.definition-cell { display: flex; flex-direction: column; gap: 4px; padding: 6px 0; }
.definition-cell span { color: var(--ac-color-text-secondary); line-height: 1.5; }
.prompt-heading { display: flex; flex-direction: column; gap: 4px; margin-bottom: 14px; }
.prompt-heading span { color: var(--ac-color-text-secondary); font-size: var(--ac-font-size-sm); }
.prompt-form { margin-top: 16px; }
.completion-list { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 8px; }
.completion-list button { padding: 4px 8px; border: 1px solid var(--console-border); border-radius: 999px; background: var(--ac-color-surface); color: var(--ac-color-text-secondary); cursor: pointer; }
.completion-list button:hover, .completion-list button:focus-visible { border-color: var(--ac-color-primary); color: var(--ac-color-primary); }
.prompt-result { margin-top: 14px; padding: 12px; border: 1px solid var(--console-border); border-radius: 8px; }
.prompt-result > div { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.prompt-result pre { max-height: 300px; overflow: auto; margin: 10px 0 0; color: var(--ac-color-text-secondary); font-size: var(--ac-font-size-xs); white-space: pre-wrap; overflow-wrap: anywhere; }
.resource-result { min-height: 180px; }
code { overflow-wrap: anywhere; }
@media (max-width: 900px) { .summary-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 640px) { .summary-grid, .form-grid, .capability-list, .limit-grid { grid-template-columns: 1fr; } .capability-row:nth-child(odd) { border-right: 0; } .table-toolbar { align-items: stretch; flex-direction: column; } .table-toolbar .el-input, .table-toolbar .el-select { width: 100%; max-width: none; } }
@media (prefers-reduced-motion: reduce) { .server-link { scroll-behavior: auto; } }
</style>
