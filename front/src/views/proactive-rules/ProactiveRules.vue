<template>
  <div class="proactive-page">
    <div class="page-header">
      <h2 class="page-title">主动消息规则</h2>
      <el-tag :type="schedulerRunning ? 'success' : 'danger'" size="small">
        {{ schedulerRunning ? '调度器运行中' : '调度器已停止' }}
      </el-tag>
    </div>

    <el-alert type="warning" :closable="false" show-icon style="margin-bottom:16px">
      <template #title>主动消息默认关闭，需手动开启规则后才会发送。安静时段和每日上限会自动约束发送频率。</template>
    </el-alert>

    <!-- Scheduler Status -->
    <el-card shadow="never" class="section-card">
      <template #header>
        <div class="card-header-row">
          <span class="section-title">调度状态</span>
          <el-button type="primary" size="small" @click="fetchStatus">刷新</el-button>
        </div>
      </template>
      <div class="status-grid">
        <div class="status-item">
          <span class="status-label">调度器</span>
          <el-tag :type="schedulerRunning ? 'success' : 'info'" size="small">{{ schedulerRunning ? '运行中' : '已停止' }}</el-tag>
        </div>
        <div class="status-item">
          <span class="status-label">已启用规则</span>
          <span class="status-value">{{ enabledRuleCount }}</span>
        </div>
        <div class="status-item">
          <span class="status-label">规则总数</span>
          <span class="status-value">{{ totalRuleCount }}</span>
        </div>
      </div>
    </el-card>

    <!-- Rule List -->
    <el-card shadow="never" class="section-card">
      <template #header>
        <div class="card-header-row">
          <span class="section-title">规则列表</span>
          <el-button type="primary" size="small" @click="openCreateDialog">新建规则</el-button>
          <el-button size="small" @click="resetPresetRules" :loading="resettingPresets">恢复预设</el-button>
        </div>
      </template>

      <el-table :data="rules" stripe size="small" v-loading="loading">
        <el-table-column prop="name" label="名称" min-width="120" show-overflow-tooltip />
        <el-table-column label="来源" width="75"><template #default="{ row }"><el-tag v-if="row._isSystem" type="info" size="small">预设</el-tag><el-tag v-else size="small">自定义</el-tag></template></el-table-column><el-table-column label="启用" width="70">
          <template #default="{ row }">
            <el-switch :model-value="!!row.enabled" size="small" @change="(val: boolean) => toggleRule(row, val)" />
          </template>
        </el-table-column>
        <el-table-column label="类型" width="110">
          <template #default="{ row }">{{ typeLabel(row.ruleType) }}</template>
        </el-table-column>
        <el-table-column label="渠道" width="90">
          <template #default="{ row }">
            <el-tag :type="row.channel === 'all' ? 'primary' : row.channel === 'wechat' ? 'success' : 'info'" size="small">
              <span :title="channelLabel(row.channel)">{{ channelLabel(row.channel) }}</span>
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="安静时段" width="110">
          <template #default="{ row }">{{ row.quietStart }} - {{ row.quietEnd }}</template>
        </el-table-column>
        <el-table-column label="今日/上限" width="80">
          <template #default="{ row }">{{ row.sentCountToday }}/{{ row.maxPerDay }}</template>
        </el-table-column>
        <el-table-column label="上次发送" width="120">
          <template #default="{ row }">{{ row.lastSentAt || '从未' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="260" fixed="right">
          <template #default="{ row }">
            <el-button link size="small" @click="openEditDialog(row)">编辑</el-button>
            <el-button link size="small" type="warning" @click="testRule(row)">测试</el-button>
            <el-button link size="small" type="success" @click="triggerRule(row)">立即发送</el-button>
            <el-popconfirm title="确定删除此规则？" @confirm="deleteRule(row)">
              <template #reference>
                <el-button link size="small" type="danger">删除</el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Create/Edit Dialog -->
    <el-dialog
      v-model="dialogVisible"
      :title="isEditing ? '编辑规则' : '新建规则'"
      width="560px"
      destroy-on-close
      :close-on-click-modal="false"
    >
      <el-form :model="form" label-position="top" size="small">
        <el-form-item label="规则名称" required>
          <el-input v-model="form.name" placeholder="例如：早安问候" maxlength="50" />
        </el-form-item>

        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="规则类型" required>
              <el-select v-model="form.ruleType" style="width:100%">
                <el-option v-for="t in ruleTypes" :key="t.value" :label="t.label" :value="t.value" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="发送渠道">
              <el-select v-model="form.channel" style="width:100%">
                <el-option label="全部平台" value="all" />
                <el-option label="Web 端" value="web" />
                <el-option label="微信" value="wechat" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="目标会话">
              <el-select v-model="form.conversationId" placeholder="选择会话" clearable filterable style="width:100%">
                <el-option v-for="c in conversations" :key="c.id" :label="c.title" :value="c.id" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="角色">
              <el-select v-model="form.characterId" placeholder="选择角色" clearable filterable style="width:100%">
                <el-option v-for="ch in characters" :key="ch.id" :label="ch.name" :value="ch.id" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>

        <el-form-item label="Cron 表达式">
          <el-input v-model="form.scheduleCron" placeholder="0 9 * * * （每天9点）">
            <template #append>
              <el-tooltip content="分 时 日 月 周，例如 0 9 * * * 表示每天9:00" placement="top">
                <el-icon><QuestionFilled /></el-icon>
              </el-tooltip>
            </template>
          </el-input>
        </el-form-item>

        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="安静开始">
              <el-time-picker v-model="form.quietStart" format="HH:mm" value-format="HH:mm" placeholder="22:00" style="width:100%" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="安静结束">
              <el-time-picker v-model="form.quietEnd" format="HH:mm" value-format="HH:mm" placeholder="08:00" style="width:100%" />
            </el-form-item>
          </el-col>
        </el-row>

        <el-form-item label="每日上限">
          <el-input-number v-model="form.maxPerDay" :min="1" :max="24" style="width:100%" />
        </el-form-item>

        <el-form-item label="消息模板">
          <el-input
            v-model="form.promptTemplate"
            type="textarea"
            :rows="3"
            placeholder="自定义发送内容，留空使用默认模板"
            maxlength="500"
            show-word-limit
          />
        </el-form-item>

        <el-form-item label="启用">
          <el-switch v-model="form.enabled" />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveRule" :loading="saving">
          {{ isEditing ? '保存' : '创建' }}
        </el-button>
      </template>
    </el-dialog>

    <!-- Test Result Dialog -->
    <el-dialog v-model="testVisible" title="测试结果" width="500px">
      <template v-if="testResult">
        <el-descriptions :column="1" border size="small">
          <el-descriptions-item label="规则">{{ testResult.ruleName }}</el-descriptions-item>
          <el-descriptions-item label="渠道">{{ channelLabel(testResult.channel) }}</el-descriptions-item>
          <el-descriptions-item label="消息内容">
            <div class="msg-preview">{{ testResult.messageContent }}</div>
          </el-descriptions-item>
          <el-descriptions-item label="安全检查">
            <el-tag :type="testResult.safetyCheck?.safe ? 'success' : 'danger'" size="small">
              {{ testResult.safetyCheck?.safe ? '通过' : '未通过' }}
            </el-tag>
            <span v-if="!testResult.safetyCheck?.safe" style="margin-left:8px;color:var(--el-color-danger)">
              {{ testResult.safetyCheck?.reason }}
            </span>
          </el-descriptions-item>
        </el-descriptions>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, inject, type Ref } from "vue"
import { ElMessage, ElMessageBox } from "element-plus"
import { QuestionFilled } from "@element-plus/icons-vue"
import { request } from "../../composables/request"

interface ProactiveRule {
  id: number
  name: string
  enabled: number
  channel: string
  conversationId: string | null
  characterId: string | null
  ruleType: string
  scheduleCron: string
  quietStart: string
  quietEnd: string
  maxPerDay: number
  lastSentAt: string | null
  sentCountToday: number
  promptTemplate: string
  createdAt: string
  updatedAt: string
}

const rules = ref<ProactiveRule[]>([])
const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const isEditing = ref(false)
const editingId = ref<number | null>(null)
const testVisible = ref(false)
const testResult = ref<any>(null)
const schedulerRunning = ref(false)

const CHANNEL_LABELS: Record<string, string> = { all: "全部平台", web: "Web 端", wechat: "微信", "web,wechat": "Web + 微信" }
function channelLabel(ch: string) { return CHANNEL_LABELS[ch] || ch }
const enabledRuleCount = ref(0)
const totalRuleCount = ref(0)

const ruleTypes = [
  { value: "daily_greeting", label: "每日问候" },
  { value: "sleep_reminder", label: "休息提醒" },
  { value: "study_checkin", label: "学习提醒" },
  { value: "work_break", label: "休息提示" },
  { value: "custom", label: "自定义" },
]

const conversations = ref<any[]>([])
const characters = ref<any[]>([])


function typeLabel(type: string): string {
  return ruleTypes.find(t => t.value === type)?.label || type
}

const form = ref({
  name: "",
  enabled: true,
  channel: "all",
  conversationId: "",
  characterId: "",
  ruleType: "daily_greeting",
  scheduleCron: "0 9 * * *",
  quietStart: "22:00",
  quietEnd: "08:00",
  maxPerDay: 1,
  promptTemplate: "",
})

const injectedCharacterId = inject<Ref<string>>("currentCharacterId", ref(""))

function resetForm() {
  form.value = {
    name: "",
    enabled: true,
    channel: "all",
    conversationId: "",
    characterId: "",
    ruleType: "daily_greeting",
    scheduleCron: "0 9 * * *",
    quietStart: "22:00",
    quietEnd: "08:00",
    maxPerDay: 1,
    promptTemplate: "",
  }
}

async function fetchRules() {
  loading.value = true
  try {
    const params: any = {}; if (injectedCharacterId?.value) params.characterId = injectedCharacterId.value; const res: any = await request.get("/api/proactive/rules", params)
    const rawRules = Array.isArray(res) ? res : (res?.items || res?.data || [])
    rules.value = rawRules.map((r: any) => ({ ...r, _isSystem: typeof r._isSystem === "boolean" ? r._isSystem : ["早安问候","晚安提醒","学习打卡","工作间歇","午饭时间","晚间闲聊"].includes(r.name) }))
  } catch {
    rules.value = []
  } finally {
    loading.value = false
  }
}

async function fetchStatus() {
  try {
    const sParams: any = {}; if (injectedCharacterId?.value) sParams.characterId = injectedCharacterId.value; const res: any = await request.get("/api/proactive/status", sParams)
    const data = res
    schedulerRunning.value = res?.schedulerRunning ?? false
    enabledRuleCount.value = res?.enabledRuleCount ?? 0
    totalRuleCount.value = res?.totalRuleCount ?? 0
  } catch {
    // ignore
  }
}

function openCreateDialog() {
  isEditing.value = false
  editingId.value = null
  resetForm()
  if (injectedCharacterId?.value) form.value.characterId = injectedCharacterId.value
  dialogVisible.value = true
}

function openEditDialog(row: ProactiveRule) {
  isEditing.value = true
  editingId.value = row.id
  form.value = {
    name: row.name,
    enabled: row.enabled,
    channel: row.channel,
    conversationId: row.conversationId || "",
    characterId: row.characterId || "",
    ruleType: row.ruleType,
    scheduleCron: row.scheduleCron,
    quietStart: row.quietStart,
    quietEnd: row.quietEnd,
    maxPerDay: row.maxPerDay,
    promptTemplate: row.promptTemplate || "",
  }
  dialogVisible.value = true
}

async function saveRule() {
  if (!form.value.name.trim()) {
    ElMessage.warning("请输入规则名称")
    return
  }
  saving.value = true
  try {
    const payload: any = {
      name: form.value.name.trim(),
      enabled: form.value.enabled,
      channel: form.value.channel,
      conversationId: form.value.conversationId || null,
      characterId: form.value.characterId || null,
      ruleType: form.value.ruleType,
      scheduleCron: form.value.scheduleCron,
      quietStart: form.value.quietStart,
      quietEnd: form.value.quietEnd,
      maxPerDay: form.value.maxPerDay,
      promptTemplate: form.value.promptTemplate,
    }

    if (isEditing.value && editingId.value) {
      await request.put(`/api/proactive/rules/${editingId.value}`, payload)
      ElMessage.success("规则已更新")
    } else {
      await request.post("/api/proactive/rules", payload)
      ElMessage.success("规则已创建")
    }
    dialogVisible.value = false
    await fetchRules()
    await fetchStatus()
  } catch (err: any) {
    ElMessage.error(err?.message || "操作失败")
  } finally {
    saving.value = false
  }
}

async function toggleRule(row: ProactiveRule, val: boolean) {
  try {
    const result: any = await request.post(`/api/proactive/rules/${row.id}/toggle`)
    row.enabled = result?.enabled ?? (val ? 1 : 0)
    ElMessage.success(row.enabled ? "规则已启用" : "规则已停用")
    await fetchStatus()
  } catch (err: any) {
    ElMessage.error(err?.message || "操作失败")
  }
}
async function deleteRule(row: ProactiveRule) {
  try {
    await request.delete(`/api/proactive/rules/${row.id}`)
    ElMessage.success("规则已删除")
    await fetchRules()
    await fetchStatus()
  } catch (err: any) {
    ElMessage.error(err?.message || "删除失败")
  }
}

async function testRule(row: ProactiveRule) {
  try {
    const res: any = await request.post(`/api/proactive/rules/${row.id}/test`)
    testResult.value = res
    testVisible.value = true
  } catch (err: any) {
    ElMessage.error(err?.message || "测试失败")
  }
}

async function triggerRule(row: ProactiveRule) {
  try {
    await request.post(`/api/proactive/rules/${row.id}/trigger`)
    ElMessage.success("消息已发送")
    await fetchRules()
    await fetchStatus()
  } catch (err: any) {
    ElMessage.error(err?.message || "发送失败")
  }
}

const PRESET_RULE_NAMES = ["早安问候", "晚安提醒", "工作间歇", "午饭时间", "晚间闲聊", "早安心情", "午间日常", "傍晚时光", "睡前分享"]
const resettingPresets = ref(false)

function isPresetRule(row: any): boolean {
  return PRESET_RULE_NAMES.includes(row.name)
}

async function resetPresetRules() {
  try {
    await ElMessageBox.confirm(
      "将删除所有现有规则并恢复为系统预设的6条规则。确定继续？",
      "恢复预设",
      { confirmButtonText: "确定", cancelButtonText: "取消", type: "warning" }
    )
    resettingPresets.value = true
    const resetPayload: any = {}; if (injectedCharacterId?.value) resetPayload.characterId = injectedCharacterId.value; await request.post("/api/proactive/rules/reset-presets", resetPayload)
    ElMessage.success("已恢复为系统预设规则")
    await fetchRules()
    await fetchStatus()
  } catch (err: any) {
    if (err !== "cancel") {
      ElMessage.error(err?.message || "恢复失败")
    }
  } finally {
    resettingPresets.value = false
  }
}

async function fetchConversations() {
  try {
    const res: any = await request.get("/api/chats/conversations", { pageSize: 100 })
    conversations.value = res?.items || res?.data || []
  } catch {}
}

async function fetchCharacters() {
  try {
    const res: any = await request.get("/api/characters")
    characters.value = Array.isArray(res) ? res : (res?.items || res?.data || [])
  } catch {}
}

onMounted(() => {
  fetchRules()
  fetchStatus()
  fetchConversations()
  fetchCharacters()
})
</script>

<style scoped>
.proactive-page {
  padding: 20px;
  }

.page-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}

.page-title {
  margin: 0;
  font-size: var(--ac-font-size-xl);
  color: var(--ac-color-text);
}

.section-card {
  margin-bottom: 16px;
}

.section-title {
  font-weight: 600;
  font-size: var(--ac-font-size-base);
}

.card-header-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.status-grid {
  display: flex;
  gap: 32px;
}

.status-item {
  display: flex;
  align-items: center;
  gap: 8px;
}

.status-label {
  color: var(--ac-color-text-secondary);
  font-size: var(--ac-font-size-sm);
}

.status-value {
  font-weight: 600;
  font-size: var(--ac-font-size-lg);
  color: var(--ac-color-primary);
}

.msg-preview {
  white-space: pre-wrap;
  padding: 8px;
  background: var(--ac-color-surface-hover);
  border-radius: var(--ac-radius-sm);
  font-size: var(--ac-font-size-sm);
}
</style>