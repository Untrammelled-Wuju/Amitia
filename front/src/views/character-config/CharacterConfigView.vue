<template>
  <div class="char-config-page">
    <!-- Safety notice -->
    <el-alert type="warning" :closable="false" show-icon style="margin-bottom:12px">
      <template #title>
        安全提示：角色不能声称自己是真人、真实恋人，不能诱导依赖、索要隐私、代替回复微信好友，不能输出成人化、操控式、威胁式或危险内容。
      </template>
    </el-alert>

    <div class="char-layout">
      <!-- ===== LEFT: Character List ===== -->
      <aside class="char-sidebar">
        <div class="sidebar-header">
          <h3>角色列表</h3>
          <el-button :icon="Plus" size="small" type="primary" @click="createNew">新建</el-button>
        </div>

        
    <!-- Import Pack Dialog -->
    <el-dialog v-model="showImportDialog" title="导入角色包" width="560px" destroy-on-close>
      <template v-if="!importPreview">
        <el-form label-position="top">
          <el-form-item label="角色包名称">
            <el-input v-model="importPackName" placeholder="输入 data/exports/character-packs/ 下的包名" />
            <div class="form-hint" style="margin-top:4px">角色包位于 data/exports/character-packs/ 目录</div>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="importPreviewing" @click="previewImport" :disabled="!importPackName">
              预览
            </el-button>
          </el-form-item>
        </el-form>

        <!-- Pack History -->
        <div v-if="packHistory.length > 0" style="margin-top:16px">
          <div class="section-label">已有角色包</div>
          <div v-for="p in packHistory" :key="p.name" class="pack-history-item" @click="importPackName = p.name; previewImport()">
            <span class="phi-name">{{ p.name }}</span>
            <span class="phi-time">{{ p.createdAt?.slice(0,10) }}</span>
          </div>
        </div>
      </template>

      <!-- Import Preview -->
      <template v-else>
        <el-alert
          v-if="importPreview.risks.length > 0"
          type="warning"
          title="风险提示"
          :closable="false"
          show-icon
          style="margin-bottom:12px"
        >
          <template #default>
            <ul style="margin:4px 0;padding-left:16px;font-size:13px">
              <li v-for="r in importPreview.risks" :key="r.category" :style="{color: r.level==='high'?'var(--el-color-danger)':'var(--el-color-warning)'}">
                [{{ r.level==='high'?'高':'中' }}] {{ r.message }}
              </li>
            </ul>
          </template>
        </el-alert>

        <div class="import-preview-info">
          <div class="ipi-row"><span class="ipi-label">名称</span><strong>{{ importPreview.name }}</strong></div>
          <div class="ipi-row"><span class="ipi-label">作者</span><span>{{ importPreview.author }}</span></div>
          <div class="ipi-row"><span class="ipi-label">身份</span><span>{{ importPreview.identity || '未设置' }}</span></div>
          <div class="ipi-row"><span class="ipi-label">性格</span><span>{{ importPreview.personality || '未设置' }}</span></div>
          <div class="ipi-row"><span class="ipi-label">说话风格</span><span>{{ importPreview.speakingStyle || '未设置' }}</span></div>
          <div class="ipi-row"><span class="ipi-label">关系氛围</span><span>{{ importPreview.relationshipStyle || '未设置' }}</span></div>
          <div class="ipi-row"><span class="ipi-label">边界规则</span><span class="ipi-value-wrap">{{ importPreview.boundaryRulesSummary }}</span></div>
          <div class="ipi-row"><span class="ipi-label">包含记忆</span><span>{{ importPreview.hasMemories ? importPreview.memoryCount + ' 条' : '无' }}</span></div>
          <div class="ipi-row"><span class="ipi-label">安全等级</span>
            <el-tag :type="importPreview.safetyLevel==='high'?'danger':importPreview.safetyLevel==='medium'?'warning':'success'" size="small">
              {{ importPreview.safetyLevel==='high'?'高风险':importPreview.safetyLevel==='medium'?'中风险':'正常' }}
            </el-tag>
          </div>
        </div>

        <el-divider />
        <div class="confirm-row" style="margin-bottom:8px">
          <span style="font-size:13px">输入 确认导入 以继续：</span>
          <el-input v-model="importConfirmText" placeholder='输入"确认导入"' style="width:160px" size="small" />
        </div>
        <el-row :gutter="8">
          <el-col :span="12">
            <el-button @click="importPreview=null; importConfirmText=''" style="width:100%">返回</el-button>
          </el-col>
          <el-col :span="12">
            <el-button
              type="primary"
              :disabled="importConfirmText !== '确认导入'"
              :loading="importing"
              @click="confirmImport"
              style="width:100%"
            >
              确认导入
            </el-button>
          </el-col>
        </el-row>
      </template>
    </el-dialog>

<!-- From Template button -->
        <div class="templates-section">
          <el-button type="primary" :icon="Plus" size="small" @click="showTemplateDialog = true; fetchTemplates()" style="width:100%">
            从模板创建
          </el-button>
        </div>

        <div class="divider"></div>

        <!-- User characters -->
        <div class="char-list">
          <div
            v-for="c in characters"
            :key="c.id"
            class="char-list-item"
            :class="{ active: selectedId === c.id, 'is-active': c.isActive }"
            @click="selectChar(c)"
          >
            <div class="cli-main">
              <el-avatar :size="28">{{ c.name?.charAt(0) }}</el-avatar>
              <span class="cli-name">{{ c.name }}</span>
              <el-tag v-if="c.isActive" type="success" size="small" effect="dark">当前</el-tag>
            </div>
            <div class="cli-actions" v-if="selectedId === c.id">
              <el-button text size="small" @click.stop="copyChar(c)" title="复制">
                <el-icon><CopyDocument /></el-icon>
              </el-button>
              <el-button text size="small" type="danger" @click.stop="delChar(c)" title="删除">
                <el-icon><Delete /></el-icon>
              </el-button>
            </div>
          </div>

          <el-empty v-if="characters.length === 0" description="还没有角色" :image-size="50" />
        </div>
      </aside>

      <!-- ===== RIGHT: Edit Form + Test ===== -->
      <main class="char-main" v-if="selected">
        <!-- Tab: Edit / Test -->
        <el-tabs v-model="activeTab">
          <el-tab-pane label="编辑角色" name="edit">
            <el-form :model="form" label-position="top" class="char-form">
              <el-row :gutter="12">
                <el-col :span="16">
                  <el-form-item label="名称">
                    <el-input v-model="form.name" placeholder="角色名称" />
                  </el-form-item>
                </el-col>
                <el-col :span="8">
                  <el-form-item label="头像链接">
                    <el-input v-model="form.avatar" placeholder="URL（可选）" />
                  </el-form-item>
                </el-col>
              </el-row>

              <el-row :gutter="12">
                <el-col :span="12">
                  <el-form-item label="身份">
                    <el-input v-model="form.identity" placeholder="例如: AI 虚拟陪伴角色" />
                  </el-form-item>
                </el-col>
                <el-col :span="12">
                  <el-form-item label="性格">
                    <el-input v-model="form.personality" placeholder="例如: 温和、体贴、有耐心" />
                  </el-form-item>
                </el-col>
              </el-row>

              <el-row :gutter="12">
                <el-col :span="12">
                  <el-form-item label="说话风格">
                    <el-input v-model="form.speakingStyle" placeholder="例如: 简短自然、轻声细语" />
                  </el-form-item>
                </el-col>
                <el-col :span="12">
                  <el-form-item label="关系氛围">
                    <el-input v-model="form.relationshipStyle" placeholder="例如: 亲近但保持边界" />
                  </el-form-item>
                </el-col>
              </el-row>

              <PersonalitySliders v-model="form.personalityConfig" style="margin-bottom:16px" />

              <!-- System Prompt (full-screen capable) -->
              <el-form-item label="系统提示词 (System Prompt)">
                <div class="textarea-toolbar">
                  <el-button text size="small" :icon="FullScreen" @click="showFullPrompt = true">
                    全屏编辑
                  </el-button>
                  <el-button text size="small" @click="resetPrompt">
                    恢复默认
                  </el-button>
                </div>
                <el-input
                  v-model="form.systemPrompt"
                  type="textarea"
                  :rows="8"
                  placeholder="编写角色的 System Prompt..."
                />
              </el-form-item>

              <!-- Boundary Rules (full-screen capable) -->
              <el-form-item label="安全边界规则">
                <div class="textarea-toolbar">
                  <el-button text size="small" :icon="FullScreen" @click="showFullBounds = true">
                    全屏编辑
                  </el-button>
                  <el-button text size="small" @click="resetBounds">
                    恢复默认
                  </el-button>
                </div>
                <el-input
                  v-model="form.boundaryRules"
                  type="textarea"
                  :rows="5"
                  placeholder="每行一条规则..."
                />
              </el-form-item>

              <div class="form-actions">
                <el-checkbox v-model="form.isActive" :disabled="form.isActive && !hasOtherActive">
                  设为当前启用角色
                </el-checkbox>
                <el-button type="primary" :loading="saving" @click="saveChar">
                  {{ selected.id ? "保存修改" : "创建角色" }}
                </el-button>
              </div>
            </el-form>
          </el-tab-pane>

          <!-- Test tab -->
          <el-tab-pane label="实时测试" name="test">
            <div class="test-area">
              <div class="test-chat" ref="testChatRef">
                <div v-if="testMessages.length === 0 && !testLoading" class="test-empty">
                  <p>在下方输入测试消息，预览角色回复</p>
                  <p class="test-hint">测试不会写入正式会话</p>
                </div>
                <div v-for="(m, i) in testMessages" :key="i" class="test-msg" :class="m.role">
                  <span class="tm-role">{{ m.role === "user" ? "你" : selected.name }}</span>
                  <div class="tm-content">{{ m.content }}</div>
                </div>
                <div v-if="testLoading" class="test-msg assistant">
                  <span class="tm-role">{{ selected.name }}</span>
                  <div class="tm-content typing">回复中...</div>
                </div>
              </div>
              <div class="test-input">
                <el-input
                  v-model="testMsg"
                  placeholder="输入测试消息..."
                  @keyup.enter="sendTest"
                  :disabled="testLoading"
                >
                  <template #append>
                    <el-button :icon="Promotion" @click="sendTest" :disabled="testLoading || !testMsg.trim()" />
                  </template>
                </el-input>
              </div>
            </div>
          </el-tab-pane>
        </el-tabs>
      </main>

      <!-- No character selected -->
      <main class="char-main empty" v-else>
        <el-empty description="选择左侧角色或新建一个" :image-size="80" />
      </main>
    </div>

    <!-- Full-screen Prompt dialog -->
    <el-dialog v-model="showFullPrompt" title="全屏编辑 - System Prompt" fullscreen destroy-on-close>
      <el-input v-model="form.systemPrompt" type="textarea" :rows="25" placeholder="编写 System Prompt..." />
      <template #footer>
        <el-button @click="showFullPrompt = false">完成</el-button>
      </template>
    </el-dialog>

    <!-- Full-screen Boundary Rules dialog -->
    <el-dialog v-model="showFullBounds" title="全屏编辑 - 安全边界" fullscreen destroy-on-close>
      <el-input v-model="form.boundaryRules" type="textarea" :rows="25" placeholder="编写安全边界规则..." />
      <template #footer>
        <el-button @click="showFullBounds = false">完成</el-button>
      </template>
    </el-dialog>
  </div>

    <!-- Template picker dialog -->
    <el-dialog v-model="showTemplateDialog" title="从模板创建角色" width="720px" top="5vh">
      <div v-if="templateLoading" style="text-align:center;padding:40px">
        <el-icon class="is-loading" :size="32"><Loading /></el-icon>
        <p style="margin-top:12px;color:var(--ac-color-text-muted)">加载模板中...</p>
      </div>
      <div v-else-if="templates.length === 0" style="text-align:center;padding:40px">
        <el-empty description="暂无可用模板" :image-size="60" />
      </div>
      <div v-else class="template-grid">
        <div
          v-for="tpl in templates"
          :key="tpl.id"
          class="template-card"
          @click="createFromTemplate(tpl)"
        >
          <div class="tpl-card-header">
            <span class="tpl-card-name">{{ tpl.name }}</span>
            <el-tag v-if="tpl.hasSafeBoundaries" type="success" size="small" effect="plain">已审查</el-tag>
          </div>
          <div class="tpl-card-scenario">{{ tpl.scenario }}</div>
          <div class="tpl-card-details">
            <div class="tpl-detail-row">
              <span class="tpl-detail-label">说话风格</span>
              <span class="tpl-detail-value">{{ tpl.speakingStyle }}</span>
            </div>
            <div class="tpl-detail-row">
              <span class="tpl-detail-label">关系氛围</span>
              <span class="tpl-detail-value">{{ tpl.relationshipStyle }}</span>
            </div>
          </div>
        </div>
      </div>
    </el-dialog>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, nextTick, inject } from "vue"
import { ElMessage, ElMessageBox } from "element-plus"
import { Plus, CopyDocument, Delete, FullScreen, Promotion, Loading } from "@element-plus/icons-vue"
import { useApi } from "../../composables/useApi"
import PersonalitySliders from "../../components/PersonalitySliders.vue"

const { get, post, put, del } = useApi()
const refreshHealth = inject<() => void>("refreshHealth", () => {})

// ==========================================================
// Built-in Templates
// ==========================================================
const templates = ref<any[]>([])
const showTemplateDialog = ref(false)
const templateLoading = ref(false)

type TemplateItem = { id: string; name: string; scenario: string; identity: string; personality: string; speakingStyle: string; relationshipStyle: string; hasSafeBoundaries: boolean }

async function fetchTemplates() {
  templateLoading.value = true
  try {
    templates.value = await get<any[]>("/api/character-templates") || []
  } catch {
    templates.value = []
  } finally {
    templateLoading.value = false
  }
}

const DEFAULT_BOUNDARY = [
  "不能声称自己是真人。",
  "不能声称自己是真实恋人。",
  "不能诱导用户依赖。",
  "不能索要验证码、密码、银行卡、身份证号等敏感信息。",
  "不能代替用户回复微信好友。",
  "不能输出成人化、操控式、威胁式或危险内容。",
].join("\n")

// ==========================================================
// State
// ==========================================================
const characters = ref<any[]>([])
const selected = ref<any>(null)
const selectedId = ref("")
const activeTab = ref("edit")
const saving = ref(false)
const showFullPrompt = ref(false)
const showFullBounds = ref(false)
const exportingPack = ref(false)
const showImportDialog = ref(false)
const importPackName = ref("")
const importPreview = ref<any | null>(null)
const importPreviewing = ref(false)
const importConfirmText = ref("")
const importing = ref(false)
const packHistory = ref<any[]>([])

const form = reactive({
  name: "", avatar: "", identity: "", personality: "",
  speakingStyle: "", relationshipStyle: "",
  systemPrompt: "", boundaryRules: DEFAULT_BOUNDARY,
  isActive: true,
  description: "",
  basePrompt: "",
  isDefault: false,
  status: "enabled",
  personalityConfig: {
    familiarity: 78, formality: 22, customerServiceAvoidance: 92,
    directness: 75, verbosity: 32, structureLevel: 40, shortSentence: 85, toneWords: 45,
    warmth: 58, emotionalExpression: 45, comfortLevel: 55, preachingAvoidance: 88,
    rationality: 62, humor: 35, teasing: 30, initiative: 50, patience: 60,
    companionship: 55, boundary: 85, dependencyAvoidance: 85,
    execution: 75, explanationDepth: 55, judgment: 75, clarification: 35,
    intimacyExpression: 25, flirtiness: 0, romanticTone: 0,
    suggestivenessAvoidance: 100, intimacyBoundary: 90,
  },
  chatStyleConfig: null as any,
  sceneRules: null as any,
})

const hasOtherActive = computed(() =>
  characters.value.some(c => c.isActive && c.id !== selectedId.value)
)

// Test chat
const testMessages = ref<{ role: string; content: string }[]>([])
const testMsg = ref("")
const testLoading = ref(false)
const testChatRef = ref<HTMLElement>()

// ==========================================================
// Fetch
// ==========================================================
async function fetchChars() {
  try { characters.value = await get<any[]>("/api/characters") || [] } catch {}
}

// ==========================================================
// Select / Create
// ==========================================================
function selectChar(c: any) {
  selected.value = c
  selectedId.value = c.id
  activeTab.value = "edit"
  form.name = c.name || ""
  form.avatar = c.avatar || ""
  form.identity = c.identity || ""
  form.personality = c.personality || ""
  form.speakingStyle = c.speakingStyle || ""
  form.relationshipStyle = c.relationshipStyle || ""
  form.systemPrompt = c.systemPrompt || ""
  form.boundaryRules = c.boundaryRules ?? DEFAULT_BOUNDARY
  form.description = c.description || ""
  form.basePrompt = c.basePrompt || ""
  form.isDefault = !!c.isDefault
  form.status = c.status || "enabled"
  form.personalityConfig = typeof c.personalityConfig === 'string' ? JSON.parse(c.personalityConfig) : (c.personalityConfig || { ...form.personalityConfig })
  form.chatStyleConfig = c.chatStyleConfig || null
  form.sceneRules = c.sceneRules || null
  form.isActive = !!c.isActive
  testMessages.value = []
}

function createNew() {
  selected.value = { id: "", name: "", isActive: false }
  selectedId.value = ""
  activeTab.value = "edit"
  form.name = ""
  form.avatar = ""
  form.identity = ""
  form.personality = ""
  form.speakingStyle = ""
  form.relationshipStyle = ""
  form.systemPrompt = ""
  form.boundaryRules = ""
  form.isActive = true
  testMessages.value = []
}

async function createFromTemplate(tpl: TemplateItem) {
  try {
    const result = await post<any>(`/api/character-templates/${tpl.id}/create-character`, {
      name: tpl.name,
    })
    if (result) {
      showTemplateDialog.value = false
      await fetchChars()
      selectChar(result)
    }
  } catch (err: any) {
    console.error("Failed to create from template:", err)
  }
}

async function exportPack() {
  if (!selectedId.value) return
  exportingPack.value = true
  try {
    const d = await post<any>("/api/characters/" + selectedId.value + "/export-pack", {
      includeMemories: false,
    })
    ElMessage.success("角色包已导出: " + d.packName)
  } catch (err: any) {
    ElMessage.error("导出失败: " + (err.response?.data?.message || err.message))
  } finally { exportingPack.value = false }
}

async function previewImport() {
  if (!importPackName.value) return
  importPreviewing.value = true
  importPreview.value = null
  try {
    importPreview.value = await post<any>("/api/characters/import-pack/preview", {
      packName: importPackName.value,
    })
  } catch (err: any) {
    ElMessage.error("预览失败: " + (err.response?.data?.message || err.message))
  } finally { importPreviewing.value = false }
}

async function confirmImport() {
  if (importConfirmText.value !== "确认导入" || !importPackName.value) return
  importing.value = true
  try {
    const d = await post<any>("/api/characters/import-pack/confirm", {
      packName: importPackName.value,
      confirmText: "确认导入",
      setActive: true,
    })
    ElMessage.success("角色包导入成功")
    showImportDialog.value = false
    importPreview.value = null
    importConfirmText.value = ""
    importPackName.value = ""
    await fetchChars()
    loadPackHistory()
    if (d.characterId) selectCharById(d.characterId)
  } catch (err: any) {
    ElMessage.error("导入失败: " + (err.response?.data?.message || err.message))
  } finally { importing.value = false }
}

async function loadPackHistory() {
  try {
    packHistory.value = await get<any[]>("/api/characters/packs/history") || []
  } catch { packHistory.value = [] }
}

function selectCharById(id: string) {
  const found = characters.value.find(c => c.id === id)
  if (found) selectChar(found)
}

function copyChar(c: any) {
  createNew()
  form.name = (c.name || "") + " (副本)"
  form.avatar = c.avatar || ""
  form.identity = c.identity || ""
  form.personality = c.personality || ""
  form.speakingStyle = c.speakingStyle || ""
  form.relationshipStyle = c.relationshipStyle || ""
  form.systemPrompt = c.systemPrompt || ""
  form.boundaryRules = c.boundaryRules ?? DEFAULT_BOUNDARY
  form.description = c.description || ""
  form.basePrompt = c.basePrompt || ""
  form.isDefault = false
  form.status = "enabled"
  form.personalityConfig = typeof c.personalityConfig === 'string' ? JSON.parse(c.personalityConfig) : (c.personalityConfig || { ...form.personalityConfig })
  form.chatStyleConfig = c.chatStyleConfig || null
  form.sceneRules = c.sceneRules || null
  form.isActive = false
  ElMessage.success("已复制角色，请修改后保存")
}

// ==========================================================
// Save
// ==========================================================
async function saveChar() {
  if (!form.name.trim()) {
    ElMessage.warning("请输入角色名称")
    return
  }

  saving.value = true
  try {
    const payload = { ...form }

    if (selected.value?.id) {
      await put(`/api/characters/${selected.value.id}`, payload)
      ElMessage.success("保存成功")
    } else {
      const created = await post<any>("/api/characters", payload)
      ElMessage.success("创建成功")
      if (created?.id) {
        selected.value = { ...payload, id: created.id }
        selectedId.value = created.id
      }
    }
    await fetchChars()
    // 保存后刷新表单显示服务端实际值（可能被安全裁剪）
    if (selectedId.value) {
      const refreshed = characters.value.find((c: any) => c.id === selectedId.value)
      if (refreshed) selectChar(refreshed)
    }
    refreshHealth()
  } catch {
    // handled by interceptor
  } finally {
    saving.value = false
  }
}

// ==========================================================
// Reset defaults
// ==========================================================
function resetPrompt() {
  ElMessageBox.confirm("恢复默认提示词？当前内容将丢失。", "提示", { type: "warning" })
    .then(() => {
      form.systemPrompt = ""
      ElMessage.success("已恢复")
    })
    .catch(() => {})
}

function resetBounds() {
  ElMessageBox.confirm("恢复默认边界规则？", "提示", { type: "warning" })
    .then(() => {
      form.boundaryRules = ""
      ElMessage.success("已恢复")
    })
    .catch(() => {})
}

// ==========================================================
// Delete
// ==========================================================
async function delChar(c: any) {
  if (c.isActive) {
    const others = characters.value.filter(x => x.id !== c.id)
    if (others.length === 0) {
      ElMessage.warning("不能删除唯一的角色")
      return
    }
  }

  await ElMessageBox.confirm(
    `确定删除角色「${c.name}」？此操作不可撤销。`,
    "确认删除",
    { type: "warning", confirmButtonText: "删除", confirmButtonClass: "el-button--danger" }
  )

  try {
    await del(`/api/characters/${c.id}`)
    ElMessage.success("已删除")
    if (selectedId.value === c.id) {
      selected.value = null
      selectedId.value = ""
    }
    await fetchChars()
    // 保存后刷新表单显示服务端实际值（可能被安全裁剪）
    if (selectedId.value) {
      const refreshed = characters.value.find((c: any) => c.id === selectedId.value)
      if (refreshed) selectChar(refreshed)
    }
    refreshHealth()
  } catch {}
}

// ==========================================================
// Test Chat
// ==========================================================
async function sendTest() {
  const msg = testMsg.value.trim()
  if (!msg || testLoading.value || !selected.value?.id) return

  testMessages.value.push({ role: "user", content: msg })
  testMsg.value = ""
  testLoading.value = true

  try {
    const result = await post<any>(`/api/characters/${selected.value.id}/test`, { message: msg })
    testMessages.value.push({ role: "assistant", content: result?.reply || "(无回复)" })
  } catch {
    testMessages.value.push({ role: "assistant", content: "测试失败，请检查模型配置" })
  } finally {
    testLoading.value = false
    nextTick(() => {
      if (testChatRef.value) testChatRef.value.scrollTop = testChatRef.value.scrollHeight
    })
  }
}

onMounted(() => {
  fetchChars()
  loadPackHistory()
})
</script>

<style scoped>
.char-config-page {
  
  height: 100%;
  display: flex;
  flex-direction: column;
}

/* ---- Layout ---- */
.char-layout {
  display: flex;
  gap: 16px;
  flex: 1;
  overflow: hidden;
  min-height: 0;
}

/* ---- Sidebar ---- */
.char-sidebar {
  width: 250px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.sidebar-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.sidebar-header h3 {
  font-size: var(--ac-font-size-base);
  font-weight: 600;
}

/* Templates */
.templates-section {
  margin-bottom: 8px;
}

.section-label {
  font-size: var(--ac-font-size-xs);
  font-weight: 600;
  color: var(--ac-color-text-muted);
  margin-bottom: 6px;
}

.template-item {
  padding: 8px 10px;
  border-radius: var(--ac-radius-sm);
  cursor: pointer;
  transition: background var(--ac-transition-fast);
  margin-bottom: 2px;
}

.template-item:hover {
  background: var(--ac-color-primary-bg);
}

.tpl-name {
  display: block;
  font-size: var(--ac-font-size-sm);
  font-weight: 500;
  color: var(--ac-color-primary);
}

.tpl-desc {
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.divider {
  height: 1px;
  background: var(--ac-color-border-light);
  margin: 8px 0;
}

/* Character list */
.char-list {
  flex: 1;
  overflow-y: auto;
}

.char-list-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 10px;
  border-radius: var(--ac-radius-sm);
  cursor: pointer;
  transition: background var(--ac-transition-fast);
  margin-bottom: 2px;
}

.char-list-item:hover {
  background: var(--ac-color-surface-hover);
}

.char-list-item.active {
  background: var(--ac-color-primary-bg);
}

.char-list-item.is-active {
  border-left: 3px solid var(--ac-color-success);
}

.cli-main {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 0;
}

.cli-name {
  font-size: var(--ac-font-size-sm);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cli-actions {
  display: flex;
  gap: 2px;
}

/* ---- Main ---- */
.char-main {
  flex: 1;
  overflow-y: auto;
  min-width: 0;
}

.char-main.empty {
  display: flex;
  align-items: center;
  justify-content: center;
}

.char-form {
  
}

/* Textarea toolbar */
.textarea-toolbar {
  display: flex;
  gap: 8px;
  margin-bottom: 6px;
}

.form-actions {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--ac-color-border-light);
}

/* Test area */
.test-area {
  display: flex;
  flex-direction: column;
  height: 400px;
  max-height: calc(100vh - 340px);
}

.test-chat {
  flex: 1;
  overflow-y: auto;
  padding: 8px 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.test-empty {
  text-align: center;
  color: var(--ac-color-text-muted);
  padding: 40px 20px;
}

.test-hint {
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-placeholder);
  margin-top: 4px;
}

.test-msg {
  max-width: 85%;
}

.test-msg.user {
  align-self: flex-end;
}

.test-msg.user .tm-role {
  text-align: right;
  display: block;
}
.tm-role {
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-muted);
  margin-bottom: 2px;
}

.tm-content {
  padding: 8px 12px;
  border-radius: var(--ac-radius-sm);
  font-size: var(--ac-font-size-sm);
  line-height: 1.5;
}

.test-msg.user .tm-content {
  background: var(--ac-color-primary-bg);
}

.test-msg.assistant .tm-content {
  background: var(--ac-color-surface);
  border: 1px solid var(--ac-color-border);
}

.typing {
  font-style: italic;
  color: var(--ac-color-text-muted);
}

.test-input {
  flex-shrink: 0;
  padding-top: 8px;
}

.template-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
  max-height: 60vh;
  overflow-y: auto;
}

.template-card {
  border: 1px solid var(--ac-color-border-light);
  border-radius: var(--ac-radius-md);
  padding: 14px;
  cursor: pointer;
  transition: all var(--ac-transition-fast);
  background: var(--ac-color-surface);
}

.template-card:hover {
  border-color: var(--ac-color-primary);
  background: var(--ac-color-primary-bg);
  transform: translateY(-1px);
  box-shadow: var(--ac-shadow-sm);
}

.tpl-card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.tpl-card-name {
  font-size: var(--ac-font-size-base);
  font-weight: 600;
  color: var(--ac-color-text);
  flex: 1;
}

.tpl-card-scenario {
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-secondary);
  line-height: 1.5;
  margin-bottom: 10px;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--ac-color-border-light);
}

.tpl-card-details {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.tpl-detail-row {
  display: flex;
  align-items: baseline;
  gap: 8px;
  font-size: var(--ac-font-size-xs);
}

.tpl-detail-label {
  color: var(--ac-color-text-muted);
  flex-shrink: 0;
  min-width: 60px;
}

.tpl-detail-value {
  color: var(--ac-color-text-secondary);
}

@media (max-width: 768px) {
  .char-layout {
    flex-direction: column;
  }

  .char-sidebar {
    width: 100%;
    max-height: 200px;
    flex-shrink: 0;
  }

  .test-area {
    height: 300px;
  }
}

/* Pack Import */
.pack-history-item {
  display: flex;
  justify-content: space-between;
  padding: 8px 10px;
  border-radius: var(--ac-radius-sm);
  cursor: pointer;
  font-size: var(--ac-font-size-xs);
  border: 1px solid var(--ac-color-border-light);
  margin-bottom: 4px;
}
.pack-history-item:hover { background: var(--ac-color-surface-hover); }
.phi-name { color: var(--ac-color-text); font-weight: 500; }
.phi-time { color: var(--ac-color-text-muted); }

.import-preview-info { font-size: 13px; }
.ipi-row {
  display: flex;
  padding: 5px 0;
  border-bottom: 1px solid var(--ac-color-border-light);
}
.ipi-label {
  color: var(--ac-color-text-muted);
  min-width: 80px;
  flex-shrink: 0;
}
.ipi-value-wrap {
  color: var(--ac-color-text-secondary);
  font-size: 12px;
  line-height: 1.4;
}
</style>
