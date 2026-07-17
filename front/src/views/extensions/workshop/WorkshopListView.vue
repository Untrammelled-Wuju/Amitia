<template>
  <main class="workshop-page">
    <ExtensionPageHeader title="扩展工坊" description="用自然语言创建声明式工作流或纯指令 Agent Skill。工坊不会生成脚本。">
      <template #actions><el-button type="primary" :icon="Plus" @click="createOpen = true">创建 Skill</el-button></template>
    </ExtensionPageHeader>

    <el-alert title="安全边界" type="info" :closable="false" show-icon>
      <template #default>所有草案都要经过结构校验、权限确认和沙箱测试，安装后默认保持禁用。</template>
    </el-alert>

    <el-card shadow="never" class="session-card">
      <el-table v-loading="loading" :data="sessions" row-key="id" empty-text="还没有工坊会话">
        <el-table-column label="Skill 需求" min-width="320">
          <template #default="{ row }">
            <button type="button" class="session-link" @click="openSession(row.id)">
              <span>{{ row.requirement }}</span>
              <code>{{ row.id }}</code>
            </button>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="150"><template #default="{ row }"><el-tag :type="statusType(row.status)">{{ statusLabel(row.status) }}</el-tag></template></el-table-column>
        <el-table-column prop="currentRevision" label="修订" width="80" />
        <el-table-column label="更新时间" width="180"><template #default="{ row }">{{ formatTime(row.updatedAt) }}</template></el-table-column>
        <el-table-column label="操作" width="170" fixed="right">
          <template #default="{ row }">
            <el-button @click="openSession(row.id)">继续</el-button>
            <el-button type="danger" link :disabled="row.status === 'archived'" @click="archive(row)">归档</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-pagination v-if="total > pageSize" v-model:current-page="page" :page-size="pageSize" :total="total" layout="prev, pager, next" @current-change="load" />
    </el-card>

    <el-dialog v-model="createOpen" title="描述你要创建的 Skill" width="min(620px, 92vw)" :close-on-click-modal="false">
      <el-form label-position="top" @submit.prevent="create">
        <el-form-item label="产物类型" required><el-radio-group v-model="productType"><el-radio-button value="workflow">执行型工作流</el-radio-button><el-radio-button value="instructions">指令型 Agent Skill</el-radio-button></el-radio-group><p class="helper">知识、审查、写作和操作规范请选择指令型；API 与宿主操作请选择工作流。</p></el-form-item>
        <el-form-item label="需求描述" required :error="requirementError">
          <el-input v-model="requirement" type="textarea" :rows="7" maxlength="20000" show-word-limit :placeholder="productType === 'instructions' ? '例如：创建一个代码审查流程，用于用户要求检查正确性和安全性时。' : '例如：输入城市后，通过 HTTPS 天气接口获取数据并生成简洁摘要。'" @blur="validateRequirement" />
          <p class="helper">不要粘贴 API Key。需要凭证时只写 Secret 引用名称。</p>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createOpen = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="create">创建会话</el-button>
      </template>
    </el-dialog>
  </main>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue"
import { useRouter } from "vue-router"
import { ElMessage, ElMessageBox } from "element-plus"
import { Plus } from "@element-plus/icons-vue"
import ExtensionPageHeader from "../components/ExtensionPageHeader.vue"
import { archiveWorkshopSession, createWorkshopSession, fetchWorkshopSessions, generateWorkshopInstruction, installAgentSkill, resolveCharacterId } from "../api"
import type { WorkshopSession, WorkshopStatus } from "../types"

const router = useRouter()
const loading = ref(false)
const creating = ref(false)
const createOpen = ref(false)
const requirement = ref("")
const productType = ref<"workflow" | "instructions">("workflow")
const requirementError = ref("")
const characterId = ref("")
const sessions = ref<WorkshopSession[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20

async function load() {
  loading.value = true
  try {
    if (!characterId.value) characterId.value = await resolveCharacterId()
    if (!characterId.value) throw new Error("请先创建或选择角色")
    const result = await fetchWorkshopSessions(characterId.value, page.value, pageSize)
    sessions.value = result.items
    total.value = result.total
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.detail || error?.message || "工坊会话加载失败")
  } finally {
    loading.value = false
  }
}

function validateRequirement() {
  requirementError.value = requirement.value.trim() ? "" : "请描述要创建的 Skill"
  return !requirementError.value
}

async function create() {
  if (!validateRequirement()) return
  creating.value = true
  try {
    if (productType.value === "instructions") {
      const preview = await generateWorkshopInstruction(requirement.value.trim(), characterId.value)
      await ElMessageBox.confirm(`将安装指令型 Agent Skill “${preview.definition.name}”。兼容状态：${preview.compatibilityReport.status}。安装后默认禁用，且不会生成或执行 scripts。`, "确认工坊产物", { type: preview.compatibilityReport.status === "blocked" ? "error" : "warning", confirmButtonText: "安装为当前角色 Skill", cancelButtonText: "取消" })
      await installAgentSkill(preview.previewId, "character", characterId.value)
      createOpen.value = false
      requirement.value = ""
      ElMessage.success("Agent Skill 已安装，默认禁用")
      await router.push("/extensions/agent-skills")
      return
    }
    const session = await createWorkshopSession(requirement.value.trim(), characterId.value)
    createOpen.value = false
    requirement.value = ""
    await router.push(`/extensions/workshop/${session.id}`)
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.detail || error?.message || "创建失败")
  } finally {
    creating.value = false
  }
}

function openSession(id: string) { router.push(`/extensions/workshop/${encodeURIComponent(id)}`) }
async function archive(session: WorkshopSession) { await ElMessageBox.confirm("归档只关闭工坊会话，不会删除已安装的 Skill。", "确认归档", { type: "warning" }); await archiveWorkshopSession(session.id, characterId.value); ElMessage.success("会话已归档"); await load() }
function formatTime(value: string) { return new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium", timeStyle: "short" }).format(new Date(value)) }
function statusLabel(status: WorkshopStatus) { return ({ draft: "待生成", generating: "生成中", generated: "草案已生成", validating: "校验中", validation_failed: "校验失败", validated: "校验通过", awaiting_permission_confirmation: "权限已确认", testing: "测试中", test_failed: "测试失败", test_passed: "测试通过", installing: "安装中", installed: "已安装·未启用", enabled: "已启用", disabled: "已禁用", archived: "已归档", error: "异常" } as Record<string, string>)[status] || status }
function statusType(status: WorkshopStatus) { if (["installed", "enabled", "test_passed", "validated"].includes(status)) return "success"; if (["validation_failed", "test_failed", "error"].includes(status)) return "danger"; if (["generating", "validating", "testing", "installing"].includes(status)) return "warning"; return "info" }
onMounted(load)
</script>

<style scoped>
.workshop-page { display: flex; flex-direction: column; gap: 16px; height: 100%; overflow: auto; }
.page-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 24px; }
.page-header h1 { margin: 0 0 8px; color: var(--console-text); }
.page-header p, .helper { margin: 0; color: var(--console-text-muted); line-height: 1.6; }
.session-card { min-height: 240px; }
.session-link { display: flex; flex-direction: column; gap: 5px; width: 100%; padding: 6px 0; border: 0; background: transparent; color: var(--console-text); text-align: left; cursor: pointer; }
.session-link span { max-width: 70ch; line-height: 1.5; }
.session-link code { color: var(--console-text-muted); font-size: 12px; }
.session-link:focus-visible { outline: 2px solid var(--el-color-primary); outline-offset: 3px; border-radius: 4px; }
.helper { margin-top: 8px; font-size: 13px; }
@media (max-width: 720px) { .page-header { flex-direction: column; } .page-header .el-button { width: 100%; min-height: 44px; } }
</style>
