<template>
  <div class="char-layout">
    <!-- 左侧角色列表 -->
    <div class="char-sidebar">
      <div class="sidebar-header">
        <h3>角色</h3>
        <el-button size="small" type="primary" @click="openCreate">+</el-button>
      </div>
      <div class="char-list">
        <div
          v-for="c in characters"
          :key="c.id"
          class="char-item"
          :class="{ active: selectedId === String(c.id) }"
          @click="selectChar(c)"
        >
          <span class="char-name">{{ c.name }}</span>
          <span class="char-desc">{{ c.description?.slice(0,15) || '' }}</span>
        </div>
        <el-empty v-if="!characters.length" description="暂无角色" :image-size="40" />
      </div>
    </div>

    <!-- 右侧详情 -->
    <div class="char-main">
      <template v-if="selectedId">
        <div class="detail-top">
          <h2>{{ selectedChar?.name }}</h2>
          <el-button size="small" @click="editCurrent">编辑</el-button>
          <el-button size="small" type="danger" @click="deleteCurrent">删除</el-button>
          
        </div>
        <el-tabs :model-value="activeTab" @tab-change="onTabChange" type="border-card">
          <el-tab-pane label="生活规则" name="life-rules">
            <AiCharacterSettingsView v-if="activeTab==='life-rules'" :key="`life-${selectedId}`" />
          </el-tab-pane>
          <el-tab-pane label="记忆管理" name="memory">
            <MemoryManagerView v-if="activeTab==='memory'" :key="`mem-${selectedId}`" />
          </el-tab-pane>
          <el-tab-pane label="记忆时间线" name="timeline">
            <MemoryTimelineView v-if="activeTab==='timeline'" :key="`tl-${selectedId}`" />
          </el-tab-pane>
          <el-tab-pane label="主动消息" name="proactive">
            <ProactiveRulesView v-if="activeTab==='proactive'" :key="`pro-${selectedId}`" />
          </el-tab-pane>
        </el-tabs>
      </template>
      <el-empty v-else description="左侧选择一个角色" :image-size="60" style="margin-top:80px" />
    </div>

    <!-- 编辑/创建弹窗 -->
    <el-dialog v-model="showDialog" :title="editingId ? '编辑角色' : '创建角色'" width="560px">
<el-form :model="form" label-width="80px">
        <el-form-item label="名称"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="描述"><el-input v-model="form.description" type="textarea" :rows="2" /></el-form-item>
        <el-form-item label="性格"><el-input v-model="form.personality" type="textarea" :rows="3" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showDialog=false">取消</el-button>
        <el-button type="primary" @click="saveCharacter" :loading="saving">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, watch, provide } from "vue"
import { useRouter, useRoute } from "vue-router"
import { ElMessage, ElMessageBox } from "element-plus"
import { apiClient } from "../../ui-index"
import { AiCharacterSettingsView, MemoryManagerView, MemoryTimelineView, ProactiveRulesView } from "../../ui-index"

const router = useRouter()
const route = useRoute()

// Provide selected character ID to sub-components
const currentCharacterId = computed(() => selectedId.value)
provide('currentCharacterId', currentCharacterId)



const characters = ref<any[]>([])
const showDialog = ref(false)
const editingId = ref<number | null>(null)
const saving = ref(false)
const form = reactive({ name:"", description:"", personality:"" })

const selectedId = ref<string | null>(null)
const selectedChar = ref<any>(null)

const activeTab = computed(() => {
  const p = route.path
  if (p.endsWith("/memory")) return "memory"
  if (p.endsWith("/timeline")) return "timeline"
  if (p.endsWith("/proactive")) return "proactive"
  return "life-rules"
})

onMounted(async () => {
  await loadCharacters()
  // 从路由恢复选中
  const id = route.params.id as string
  if (id) {
    selectedId.value = id
    const c = characters.value.find((x:any) => String(x.id) === id)
    if (c) selectedChar.value = c
  }
})

watch(() => characters.value, () => {
  const id = route.params.id as string
  if (id) {
    const c = characters.value.find((x:any) => String(x.id) === id)
    if (c) selectedChar.value = c
  }
})

async function loadCharacters() {
  try { const r = await apiClient.get("/api/characters"); characters.value = r.data?.data || r.data || [] } catch {}
}

function selectChar(c: any) {
  selectedId.value = String(c.id)
  selectedChar.value = c
  router.push(`/character/${c.id}/life-rules`)
}

function onTabChange(tab: string) {
  if (selectedId.value) router.push(`/character/${selectedId.value}/${tab}`)
}

function openCreate() { editingId.value=null;form.name="";form.description="";form.personality="";showDialog.value=true }
function editCurrent() { if(selectedChar.value) { editingId.value=selectedChar.value.id;form.name=selectedChar.value.name;form.description=selectedChar.value.description;form.personality=selectedChar.value.personality;showDialog.value=true } }


async function saveCharacter() {
  saving.value=true
  try {
    if(editingId.value){ await apiClient.put(`/api/characters/${editingId.value}`,form) }
    else { const r=await apiClient.post("/api/characters",form); const created=r.data?.data||r.data; if(created){ selectedId.value=String(created.id); selectedChar.value=created; router.push(`/character/${created.id}/life-rules`) } }
    ElMessage.success("已保存"); showDialog.value=false; await loadCharacters()
  } catch { ElMessage.error("保存失败") }
  finally { saving.value=false }
}

async function deleteCurrent() {
  if(!selectedChar.value) return
  try {
    await ElMessageBox.confirm(`确定删除「${selectedChar.value.name}」？`,"确认",{type:"warning"})
    await apiClient.delete(`/api/characters/${selectedChar.value.id}`)
    ElMessage.success("已删除")
    selectedId.value=null; selectedChar.value=null
    router.push("/character")
    await loadCharacters()
  } catch {}
}
</script>

<style scoped>
.char-layout { display:flex; height:calc(100vh - 80px); gap:0; }
.char-sidebar { width:200px; flex-shrink:0; border-right:1px solid var(--el-border-color-light); background:var(--el-bg-color); display:flex; flex-direction:column; }
.sidebar-header { display:flex; align-items:center; justify-content:space-between; padding:12px; border-bottom:1px solid var(--el-border-color-lighter); }
.sidebar-header h3 { font-size:15px; font-weight:600; margin:0; }
.char-list { flex:1; overflow-y:auto; padding:4px; }
.char-item { padding:10px 12px; cursor:pointer; border-radius:6px; margin:2px 0; display:flex; flex-direction:column; gap:2px; transition:background .15s; }
.char-item:hover { background:var(--el-fill-color-light); }
.char-item.active { background:var(--el-color-primary-light-9); }
.char-name { font-size:14px; font-weight:500; }
.char-desc { font-size:11px; color:var(--el-text-color-secondary); }
.char-main { flex:1; overflow-y:auto; padding:16px 20px; }
.detail-top { display:flex; align-items:center; gap:10px; margin-bottom:12px; }
.detail-top h2 { font-size:18px; font-weight:600; margin:0; flex:1; }
</style>
