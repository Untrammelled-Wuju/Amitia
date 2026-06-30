// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, onMounted, inject, type Ref } from "vue"
import { ElMessage, ElMessageBox } from "element-plus"
import { request } from "../../../composables/request"

export interface ProactiveRule {
  id: number
  name: string
  enabled: boolean
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

const CHANNEL_LABELS: Record<string, string> = { all: "全部平台", web: "Web 端", wechat: "微信", "web,wechat": "Web + 微信" }

const PRESET_RULE_NAMES = ["早安问候", "晚安提醒", "工作间歇", "午饭时间", "晚间闲聊", "早安心情", "午间日常", "傍晚时光", "睡前分享"]

const RULE_TYPES = [
  { value: "daily_greeting", label: "每日问候" },
  { value: "sleep_reminder", label: "休息提醒" },
  { value: "study_checkin", label: "学习提醒" },
  { value: "work_break", label: "休息提示" },
  { value: "custom", label: "自定义" },
]

export function useProactiveRules() {
  const injectedCharacterId = inject<Ref<string>>("currentCharacterId", ref(""))

  const rules = ref<ProactiveRule[]>([])
  const loading = ref(false)
  const saving = ref(false)
  const dialogVisible = ref(false)
  const isEditing = ref(false)
  const editingId = ref<number | null>(null)
  const testVisible = ref(false)
  const testResult = ref<any>(null)
  const schedulerRunning = ref(false)

  const activeMsgSettings = ref({
    enabled: true,
    activeLevel: 40,
    quietStart: "23:00",
    quietEnd: "07:00",
    minInterval: 60,
    maxPerDay: 6,
    maxDailyCalls: 10,
    channel: "all",
  })
  const savingSettings = ref(false)

  const enabledRuleCount = ref(0)
  const totalRuleCount = ref(0)

  const conversations = ref<any[]>([])
  const characters = ref<any[]>([])
  const resettingPresets = ref(false)

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

  function channelLabel(ch: string) { return CHANNEL_LABELS[ch] || ch }
  function typeLabel(type: string): string { return RULE_TYPES.find(t => t.value === type)?.label || type }
  function isPresetRule(row: any): boolean { return PRESET_RULE_NAMES.includes(row.name) }

  function resetForm() {
    form.value = {
      name: "", enabled: true, channel: "all", conversationId: "", characterId: "",
      ruleType: "daily_greeting", scheduleCron: "0 9 * * *", quietStart: "22:00",
      quietEnd: "08:00", maxPerDay: 1, promptTemplate: "",
    }
  }

  async function fetchRules() {
    loading.value = true
    try {
      const params: any = {}
      if (injectedCharacterId?.value) params.characterId = injectedCharacterId.value
      const res: any = await request.get("/api/proactive/rules", params)
      const rawRules = Array.isArray(res) ? res : (res?.items || res?.data || [])
      rules.value = rawRules.map((r: any) => ({
        ...r,
        enabled: !!r.enabled,
        _isSystem: typeof r._isSystem === "boolean" ? r._isSystem : ["早安问候","晚安提醒","学习打卡","工作间歇","午饭时间","晚间闲聊"].includes(r.name),
      }))
    } catch {
      rules.value = []
    } finally {
      loading.value = false
    }
  }

  async function fetchStatus() {
    try {
      const sParams: any = {}
      if (injectedCharacterId?.value) sParams.characterId = injectedCharacterId.value
      const res: any = await request.get("/api/proactive/status", sParams)
      schedulerRunning.value = res?.schedulerRunning ?? false
      enabledRuleCount.value = res?.enabledRuleCount ?? 0
      totalRuleCount.value = res?.totalRuleCount ?? 0
    } catch {}
  }

  async function fetchActiveMsgSettings() {
    try {
      const params: any = {}
      if (injectedCharacterId?.value) params.characterId = injectedCharacterId.value
      const res: any = await request.get("/api/companion/active-message/setting", params)
      if (res) Object.assign(activeMsgSettings.value, res)
    } catch {}
  }

  async function saveActiveMsgSettings() {
    savingSettings.value = true
    try {
      let url = "/api/companion/active-message/setting"
      if (injectedCharacterId?.value) url += "?characterId=" + encodeURIComponent(injectedCharacterId.value)
      await request.put(url, activeMsgSettings.value)
      ElMessage.success("设置已保存")
    } catch (err: any) {
      ElMessage.error(err?.message || "保存失败")
    } finally {
      savingSettings.value = false
    }
  }

  function openCreateDialog() {
    isEditing.value = false; editingId.value = null; resetForm()
    if (injectedCharacterId?.value) form.value.characterId = injectedCharacterId.value
    dialogVisible.value = true
  }

  function openEditDialog(row: ProactiveRule) {
    isEditing.value = true; editingId.value = row.id
      form.value = {
      name: row.name, enabled: !!row.enabled, channel: row.channel,
      conversationId: row.conversationId || "", characterId: row.characterId || "",
      ruleType: row.ruleType, scheduleCron: row.scheduleCron,
      quietStart: row.quietStart, quietEnd: row.quietEnd,
      maxPerDay: row.maxPerDay, promptTemplate: row.promptTemplate || "",
    }
    dialogVisible.value = true
  }

  async function saveRule() {
    if (!form.value.name.trim()) { ElMessage.warning("请输入规则名称"); return }
    saving.value = true
    try {
      const payload: any = {
        name: form.value.name.trim(), enabled: form.value.enabled,
        channel: form.value.channel, conversationId: form.value.conversationId || null,
        characterId: form.value.characterId || null, ruleType: form.value.ruleType,
        scheduleCron: form.value.scheduleCron, quietStart: form.value.quietStart,
        quietEnd: form.value.quietEnd, maxPerDay: form.value.maxPerDay,
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
      await fetchRules(); await fetchStatus()
    } catch (err: any) {
      ElMessage.error(err?.message || "操作失败")
    } finally { saving.value = false }
  }

  async function toggleRule(row: ProactiveRule, val: boolean) {
    try {
      const result: any = await request.post(`/api/proactive/rules/${row.id}/toggle`)
      row.enabled = result?.enabled != null ? !!result.enabled : !!val
      ElMessage.success(row.enabled ? "规则已启用" : "规则已停用")
      await fetchStatus()
    } catch (err: any) { ElMessage.error(err?.message || "操作失败") }
  }

  async function deleteRule(row: ProactiveRule) {
    try {
      await request.delete(`/api/proactive/rules/${row.id}`)
      ElMessage.success("规则已删除")
      await fetchRules(); await fetchStatus()
    } catch (err: any) { ElMessage.error(err?.message || "删除失败") }
  }

  async function testRule(row: ProactiveRule) {
    try {
      const res: any = await request.post(`/api/proactive/rules/${row.id}/test`)
      testResult.value = res
      testVisible.value = true
    } catch (err: any) { ElMessage.error(err?.message || "测试失败") }
  }

  async function triggerRule(row: ProactiveRule) {
    try {
      await request.post(`/api/proactive/rules/${row.id}/trigger`)
      ElMessage.success("消息已发送")
      await fetchRules(); await fetchStatus()
    } catch (err: any) { ElMessage.error(err?.message || "发送失败") }
  }

  async function resetPresetRules() {
    try {
      await ElMessageBox.confirm(
        "将删除所有现有规则并恢复为系统预设的6条规则。确定继续？",
        "恢复预设", { confirmButtonText: "确定", cancelButtonText: "取消", type: "warning" }
      )
      resettingPresets.value = true
      const resetPayload: any = {}
      if (injectedCharacterId?.value) resetPayload.characterId = injectedCharacterId.value
      await request.post("/api/proactive/rules/reset-presets", resetPayload)
      ElMessage.success("已恢复为系统预设规则")
      await fetchRules(); await fetchStatus()
    } catch (err: any) {
      if (err !== "cancel") ElMessage.error(err?.message || "恢复失败")
    } finally { resettingPresets.value = false }
  }

  async function fetchConversations() {
    try {
      const res: any = await request.get("/api/chats/conversations", { params: { pageSize: 100 } })
      conversations.value = res?.items || res?.data || res || []
    } catch {}
  }

  async function fetchCharacters() {
    try {
      const res: any = await request.get("/api/characters")
      characters.value = Array.isArray(res) ? res : (res?.items || res?.data || [])
    } catch {}
  }

  onMounted(() => {
    fetchRules(); fetchStatus(); fetchConversations(); fetchCharacters(); fetchActiveMsgSettings()
  })

  return {
    rules, loading, saving, dialogVisible, isEditing, editingId,
    testVisible, testResult, schedulerRunning,
    activeMsgSettings, savingSettings,
    enabledRuleCount, totalRuleCount,
    conversations, characters, resettingPresets, form,
    RULE_TYPES,
    channelLabel, typeLabel, isPresetRule,
    openCreateDialog, openEditDialog, saveRule,
    toggleRule, deleteRule, testRule, triggerRule,
    resetPresetRules, saveActiveMsgSettings, fetchStatus,
  }
}
