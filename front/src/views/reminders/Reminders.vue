<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="reminders-page">
    <div class="page-header">
      <h2 class="page-title">日程提醒</h2>
      <el-tag :type="schedulerRunning ? 'success' : 'danger'" size="small">
        {{ schedulerRunning ? '调度器运行中' : '调度器已停止' }}
      </el-tag>
    </div>

    <el-alert type="info" :closable="false" show-icon style="margin-bottom:16px">
      <template #title>到达提醒时间后会自动发送消息。一次性提醒发送后自动关闭，重复提醒会按规则循环。</template>
    </el-alert>

    <!-- Status bar -->
    <el-card shadow="never" class="section-card">
      <template #header>
        <div class="card-header-row">
          <span class="section-title">状态</span>
          <span style="margin-left:16px;font-size:13px;color:var(--ac-color-text-muted)">自动清理</span>
          <el-select v-model="cleanupDays" size="small" style="width:90px;margin-left:6px" @change="setCleanupConfig">
            <el-option label="从不" value="0" />
            <el-option label="1天" value="1" />
            <el-option label="3天" value="3" />
            <el-option label="7天" value="7" />
            <el-option label="14天" value="14" />
            <el-option label="30天" value="30" />
          </el-select>
          <el-button type="primary" size="small" @click="fetchStatus">刷新</el-button>
        </div>
      </template>
      <div class="status-grid">
        <div class="status-item"><span class="status-label">调度器</span><el-tag :type="schedulerRunning ? 'success' : 'info'" size="small">{{ schedulerRunning ? '运行中' : '已停止' }}</el-tag></div>
        <div class="status-item"><span class="status-label">提醒总数</span><span class="status-value">{{ totalCount }}</span></div>
        <div class="status-item"><span class="status-label">待触发</span><span class="status-value" style="color:var(--el-color-warning)">{{ dueCount }}</span></div>
      </div>
    </el-card>

    <!-- Reminder list -->
    <el-card shadow="never" class="section-card">
      <template #header>
        <div class="card-header-row">
          <span class="section-title">提醒列表</span>
          <el-button type="primary" size="small" @click="openCreate">新建提醒</el-button>
        </div>
      </template>

      <el-table :data="reminders" stripe size="small" v-loading="loading" empty-text="暂无提醒">
        <el-table-column prop="title" label="标题" min-width="120" show-overflow-tooltip />
        <el-table-column label="启用" width="70">
          <template #default="{ row }"><el-switch :model-value="row.enabled" size="small" @change="(val: boolean) => toggleReminder(row, val)" /></template>
        </el-table-column>
        <el-table-column label="渠道" width="70">
          <template #default="{ row }"><el-tag :type="row.channel === 'wechat' ? 'success' : row.channel === 'qq' ? 'primary' : 'info'" size="small">{{ row.channel === 'wechat' ? '微信' : row.channel === 'qq' ? 'QQ' : 'Web' }}</el-tag></template>
        </el-table-column>
        <el-table-column label="提醒时间" width="155">
          <template #default="{ row }">{{ row.remindAt }}</template>
        </el-table-column>
        <el-table-column label="重复" width="80">
          <template #default="{ row }">{{ repeatLabel(row.repeatRule) }}</template>
        </el-table-column>
        <el-table-column label="角色" width="100">
          <template #default="{ row }">
            <span v-if="row.characterName" style="font-size:12px">{{ row.characterName }}</span>
            <span v-else style="color:var(--el-text-color-placeholder);font-size:12px">—</span>
          </template>
        </el-table-column>
        <el-table-column label="关联对话" width="120" show-overflow-tooltip>
          <template #default="{ row }">
            <span v-if="row.conversationTitle" style="font-size:12px">{{ row.conversationTitle }}</span>
            <span v-else style="color:var(--el-text-color-placeholder);font-size:12px">—</span>
          </template>
        </el-table-column>
        <el-table-column label="上次触发" width="155">
          <template #default="{ row }">{{ row.lastTriggeredAt || '从未' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="260" fixed="right">
          <template #default="{ row }">
            <el-button link size="small" @click="openEdit(row)">编辑</el-button>
            <el-button link size="small" type="warning" @click="testReminder(row)">测试</el-button>
            <el-button link size="small" type="success" @click="triggerNow(row)">立即发送</el-button>
            <el-popconfirm title="确定删除？" @confirm="deleteReminder(row)">
              <template #reference><el-button link size="small" type="danger">删除</el-button></template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Create/Edit Dialog -->
    <el-dialog v-model="dialogVisible" :title="isEditing ? '编辑提醒' : '新建提醒'" width="520px" destroy-on-close :close-on-click-modal="false">
      <el-form :model="form" label-position="top" size="small">
        <el-form-item label="标题" required><el-input v-model="form.title" placeholder="例如：生日快乐提醒" maxlength="50" /></el-form-item>
        <el-form-item label="内容">
          <el-input v-model="form.content" type="textarea" :rows="3" placeholder="自定义提醒内容（可选）" maxlength="300" show-word-limit />
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="发送渠道">
              <el-select v-model="form.channel" style="width:100%"><el-option label="Web 端" value="web" /><el-option label="微信" value="wechat" /><el-option label="QQ" value="qq" /></el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="重复规则">
              <el-select v-model="form.repeatRule" style="width:100%"><el-option label="不重复" value="none" /><el-option label="每天" value="daily" /><el-option label="每周" value="weekly" /></el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="关联对话">
          <el-select v-if="form.channel !== 'wechat'" v-model="form.conversationId" placeholder="不关联" clearable filterable style="width:100%" :loading="loadingConvs">
            <el-option v-for="c in conversationOptions" :key="c.id" :label="c.title" :value="c.id" />
          </el-select>
          <el-input v-else :model-value="'微信对话（自动）'" disabled />
        </el-form-item>
        <el-form-item label="提醒时间" required>
          <el-date-picker v-model="form.remindAt" type="datetime" format="YYYY-MM-DD HH:mm:ss" value-format="YYYY-MM-DD HH:mm:ss" placeholder="选择日期时间" style="width:100%" />
        </el-form-item>
        <el-form-item label="启用"><el-switch v-model="form.enabled" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveReminder" :loading="saving">{{ isEditing ? '保存' : '创建' }}</el-button>
      </template>
    </el-dialog>

    <!-- Test Result Dialog -->
    <el-dialog v-model="testVisible" title="测试结果" width="480px">
      <template v-if="testResult">
        <el-descriptions :column="1" border size="small">
          <el-descriptions-item label="标题">{{ testResult.title }}</el-descriptions-item>
          <el-descriptions-item label="渠道">{{ testResult.channel === 'wechat' ? '微信' : testResult.channel === 'qq' ? 'QQ' : 'Web' }}</el-descriptions-item>
          <el-descriptions-item label="消息内容"><div class="msg-preview">{{ testResult.messageContent }}</div></el-descriptions-item>
          <el-descriptions-item label="安全检查">
            <el-tag :type="testResult.safetyCheck?.safe ? 'success' : 'danger'" size="small">{{ testResult.safetyCheck?.safe ? '通过' : '未通过' }}</el-tag>
            <span v-if="!testResult.safetyCheck?.safe" style="margin-left:8px;color:var(--el-color-danger)">{{ testResult.safetyCheck?.reason }}</span>
          </el-descriptions-item>
        </el-descriptions>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from "vue"
import { ElMessage, ElMessageBox } from "element-plus"
import { request } from "../../composables/request"

interface Reminder {
  id: number; title: string; content: string; channel: string
  conversationId: string | null; characterId: string | null
  remindAt: string; repeatRule: string; enabled: boolean
  lastTriggeredAt: string | null; createdAt: string; updatedAt: string
}

const reminders = ref<Reminder[]>([])
const loading = ref(false); const saving = ref(false)
const dialogVisible = ref(false); const isEditing = ref(false); const editingId = ref<number | null>(null)
const testVisible = ref(false); const testResult = ref<any>(null)
const schedulerRunning = ref(false); const totalCount = ref(0); const dueCount = ref(0)
const conversationOptions = ref<{id:string;title:string}[]>([])
const characterOptions = ref<{id:string;name:string}[]>([])
const loadingConvs = ref(false); const loadingChars = ref(false); const cleanupDays = ref("0")

const repeatLabels: Record<string, string> = { none: '不重复', daily: '每天', weekly: '每周' }
function repeatLabel(r: string): string { return repeatLabels[r] || r }

const form = ref({ title: '', content: '', channel: 'web', conversationId: '', characterId: '', remindAt: null as any, repeatRule: 'none', enabled: true })
function resetForm() { form.value = { title: '', content: '', channel: 'web', conversationId: '', characterId: '', remindAt: null as any, repeatRule: 'none', enabled: true } }

async function fetchReminders() {
  loading.value = true
  try {
    const res: any = await request.get("/api/reminders")
    const raw = Array.isArray(res) ? res : (res?.items || res?.data || res || [])
    reminders.value = raw.map((item: any) => ({ ...item, enabled: !!item.enabled }))
  } catch { reminders.value = [] }
  finally { loading.value = false }
}

async function fetchStatus() {
  try { const res: any = await request.get("/api/reminders/status"); const d = res; schedulerRunning.value = d?.schedulerRunning ?? false; totalCount.value = d?.total ?? 0; dueCount.value = d?.dueNow ?? 0 } catch {}
}

async function fetchConversationOptions() {
  loadingConvs.value = true
  try { const r: any = await request.get("/api/web-chat/conversations"); conversationOptions.value = r?.items || r?.data || r?.conversations || r || [] }
  catch { conversationOptions.value = [] }
  finally { loadingConvs.value = false }
}
async function fetchCharacterOptions() {
  loadingChars.value = true
  try { const r = await request.get("/api/characters"); characterOptions.value = Array.isArray(r) ? r : (r?.data || []) }
  catch { characterOptions.value = [] }
  finally { loadingChars.value = false }
}
function openCreate() { isEditing.value = false; editingId.value = null; resetForm(); dialogVisible.value = true; fetchConversationOptions(); fetchCharacterOptions() }
function openEdit(row: Reminder) {
  isEditing.value = true; editingId.value = row.id
  form.value = { title: row.title, content: row.content, channel: row.channel, conversationId: row.conversationId || '', characterId: row.characterId || '', remindAt: row.remindAt, repeatRule: row.repeatRule, enabled: !!row.enabled }
  dialogVisible.value = true; fetchConversationOptions(); fetchCharacterOptions()
}

async function fetchCleanupConfig() {
  try { const r = await request.get("/api/reminders/cleanup-config"); const rd = (r as any)?.data; cleanupDays.value = (rd?.data?.cleanupDays ?? rd?.cleanupDays) || "0" }
  catch { cleanupDays.value = "0" }
}

async function setCleanupConfig(val: string) {
  try { await request.put("/api/reminders/cleanup-config", { cleanupDays: val }); ElMessage.success("已更新") }
  catch (err: any) { ElMessage.error(err?.message || "设置失败") }
}

async function saveReminder() {
  if (!form.value.title.trim()) { ElMessage.warning('请输入标题'); return }
  if (!form.value.remindAt) { ElMessage.warning('请选择提醒时间'); return }
  saving.value = true
  try {
    const payload = { title: form.value.title.trim(), content: form.value.content, channel: form.value.channel, conversationId: form.value.conversationId || null, characterId: form.value.characterId || null, remindAt: form.value.remindAt, repeatRule: form.value.repeatRule, enabled: form.value.enabled }
    if (isEditing.value && editingId.value) { await request.put(`/api/reminders/${editingId.value}`, payload); ElMessage.success('提醒已更新') }
    else { await request.post('/api/reminders', payload); ElMessage.success('提醒已创建') }
    dialogVisible.value = false; await fetchReminders(); await fetchStatus()
  } catch (err: any) { ElMessage.error(err?.message || '操作失败') }
  finally { saving.value = false }
}

async function toggleReminder(row: Reminder, nextValue?: boolean) {
  const prev = row.enabled
  const id = row.id
  const updateLocal = (v: boolean) => {
    const idx = reminders.value.findIndex(r => r.id === id)
    if (idx >= 0) reminders.value[idx].enabled = v
  }
  updateLocal(typeof nextValue === "boolean" ? nextValue : !row.enabled)
  try {
    const res: any = await request.post(`/api/reminders/${id}/toggle`)
    const newVal = res?.enabled != null ? !!res.enabled : !prev
    updateLocal(newVal)
    ElMessage.success(newVal ? '已启用' : '已停用')
    await fetchStatus()
  } catch (err: any) {
    updateLocal(prev)
    ElMessage.error(err?.message || '操作失败')
  }
}

async function deleteReminder(row: Reminder) {
  try { await request.delete(`/api/reminders/${row.id}`); ElMessage.success('已删除'); await fetchReminders(); await fetchStatus() }
  catch (err: any) { ElMessage.error(err?.message || '删除失败') }
}

async function testReminder(row: Reminder) {
  try { const res: any = await request.post(`/api/reminders/${row.id}/test`); testResult.value = res; testVisible.value = true }
  catch (err: any) { ElMessage.error(err?.message || '测试失败') }
}

async function triggerNow(row: Reminder) {
  try { await request.post(`/api/reminders/${row.id}/trigger`); ElMessage.success('已发送'); await fetchReminders(); await fetchStatus() }
  catch (err: any) { ElMessage.error(err?.message || '发送失败') }
}

let reminderSSE: EventSource | null = null

function connectReminderSSE() {
  try {
    const baseUrl = import.meta.env.VITE_API_BASE || ""
    reminderSSE = new EventSource(baseUrl + "/api/reminders/stream")
    reminderSSE.onmessage = (e) => {
      if (e.data) { fetchReminders(); fetchStatus() }
    }
    reminderSSE.addEventListener("changed", () => {
      fetchReminders(); fetchStatus()
    })
    reminderSSE.onerror = () => {
      reminderSSE?.close()
      setTimeout(connectReminderSSE, 5000)
    }
  } catch {}
}

onMounted(async () => {
  await fetchReminders()
  await fetchStatus()
  connectReminderSSE()
  await fetchCleanupConfig()
})

onUnmounted(() => {
  if (reminderSSE) reminderSSE.close()
})
</script>
<style scoped>
.reminders-page { padding: 20px; }
.page-header { display: flex; align-items: center; gap: 12px; margin-bottom: 16px; }
.page-title { margin: 0; font-size: var(--ac-font-size-xl); color: var(--ac-color-text); }
.section-card { margin-bottom: 16px; }
.section-title { font-weight: 600; font-size: var(--ac-font-size-base); }
.card-header-row { display: flex; justify-content: space-between; align-items: center; }
.status-grid { display: flex; gap: 32px; }
.status-item { display: flex; align-items: center; gap: 8px; }
.status-label { color: var(--ac-color-text-secondary); font-size: var(--ac-font-size-sm); }
.status-value { font-weight: 600; font-size: var(--ac-font-size-lg); color: var(--ac-color-primary); }
.msg-preview { white-space: pre-wrap; padding: 8px; background: var(--ac-color-surface-hover); border-radius: var(--ac-radius-sm); font-size: var(--ac-font-size-sm); }
</style>





