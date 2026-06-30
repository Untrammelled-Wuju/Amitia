<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="char-layout">
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
          <el-tab-pane label="拟态语音" name="voice">
            <CharacterVoiceView v-if="activeTab==='voice'" :key="`voice-${selectedId}`" />
          </el-tab-pane>
          <el-tab-pane label="主动消息" name="proactive">
            <ProactiveRulesView v-if="activeTab==='proactive'" :key="`pro-${selectedId}`" />
          </el-tab-pane>
          <el-tab-pane label="调试" name="debug">
            <CompanionDebugView v-if="activeTab==='debug'" :key="`dbg-${selectedId}`" />
          </el-tab-pane>
        </el-tabs>
      </template>
      <el-empty v-else description="左侧选择一个角色" :image-size="60" style="margin-top:80px" />
    </div>

    <el-dialog v-model="showDialog" :title="editingId ? '编辑角色' : '创建角色'" width="640px">
      <el-form :model="form" label-width="80px">
        <el-form-item label="名称"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="描述"><el-input v-model="form.description" type="textarea" :rows="2" /></el-form-item>
        <el-form-item label="性格"><el-input v-model="form.personality" type="textarea" :rows="3" /></el-form-item>

        <el-divider content-position="left">语音配置</el-divider>

        <el-form-item label="音色">
          <el-select v-model="form.voiceType" style="width:100%" filterable placeholder="选择音色" @change="onVoiceTypeChange">
            <el-option v-for="v in voicePresets" :key="v.name" :label="v.label" :value="v.name" />
          </el-select>
        </el-form-item>
        <el-form-item label="语速">
          <el-slider v-model="form.voiceSpeed" :min="0.5" :max="2.0" :step="0.1" show-input :format-tooltip="(v:number)=>v.toFixed(1)+'x'" style="width:70%" />
        </el-form-item>
        <el-form-item label="音调">
          <el-slider v-model="form.voicePitch" :min="-12" :max="12" :step="1" show-input :format-tooltip="(v:number)=>(v>0?'+':'')+v+'半音'" style="width:70%" />
        </el-form-item>
        <el-form-item label="试听">
          <el-button size="small" @click="testVoice" :loading="testingVoice">试听</el-button>
          <audio v-if="testAudioUrl" :src="testAudioUrl" controls autoplay style="width:260px;margin-left:10px;height:30px" />
        </el-form-item>

        <el-divider content-position="left">声音复刻</el-divider>

        <el-form-item label="复刻音色ID">
          <el-input v-model="form.customVoiceId" placeholder="输入音色ID，如 S_xxxxxxxx" style="width:240px" clearable />
          <span style="font-size:11px;color:var(--ac-color-text-muted);margin-left:8px">在火山控制台训练后填入</span>
        </el-form-item>
        <el-form-item label="试听" v-if="form.customVoiceId">
          <el-button size="small" @click="previewClone" :loading="previewCloneLoading">试听</el-button>
        </el-form-item>
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
import { apiClient } from "@/composables/useApi"
import { AiCharacterSettingsView, CharacterVoiceView, ProactiveRulesView, CompanionDebugView } from "../../ui-index"

const router = useRouter()
const route = useRoute()

const currentCharacterId = computed(() => selectedId.value)
provide('currentCharacterId', currentCharacterId)

const characters = ref<any[]>([])
const showDialog = ref(false)
const editingId = ref<string | null>(null)
const saving = ref(false)
const voicePresets = ref<any[]>([])
const currentVoiceSupportsEmotion = computed(() => {
  const v = voicePresets.value.find((p: any) => p.name === form.voiceType)
  return v?.supportsEmotion ?? false
})
const globalApiKey = ref("")

const form = reactive({
  name: "", description: "", personality: "",
  voiceType: "zh_female_vv_uranus_bigtts",
  voiceSpeed: 1.0, voicePitch: 0, voiceVolume: 1.0,
  customVoiceId: "",
  emotion: "",
  emotionScale: 0,
  silenceDuration: 0,
})

const testingVoice = ref(false)
const testAudioUrl = ref("")
const cloneFile = ref<File | null>(null)
const cloneName = ref("")
const cloneLoading = ref(false)
const cloneResult = ref("")
const previewCloneLoading = ref(false)

const selectedId = ref<string | null>(null)
const selectedChar = ref<any>(null)

const activeTab = computed(() => {
  const p = route.path
  if (p.endsWith("/voice")) return "voice"
  if (p.endsWith("/proactive")) return "proactive"
  if (p.endsWith("/debug")) return "debug"
  return "life-rules"
})

onMounted(async () => {
  await loadVoices()
  await loadGlobalApiKey()
  await loadCharacters()
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

async function loadVoices() {
  try { voicePresets.value = await apiClient.get("/api/tts/voices").then(r => r.data?.data || []) } catch { voicePresets.value = [] }
}

async function loadGlobalApiKey() {
  try {
    const configs = await apiClient.get("/api/tts/configs").then(r => r.data?.data || [])
    const active = configs.find((c: any) => c.isActive)
    if (active) globalApiKey.value = active.apiKey || ""
  } catch {}
}

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

function openCreate() {
  editingId.value = null
  form.name = ""; form.description = ""; form.personality = ""
  form.voiceType = "zh_female_vv_uranus_bigtts"
  form.voiceSpeed = 1.0; form.voicePitch = 0; form.voiceVolume = 1.0
  form.customVoiceId = ""
  form.emotion = ""; form.emotionScale = 0; form.silenceDuration = 0
  cloneFile.value = null; cloneName.value = ""; cloneResult.value = ""
  showDialog.value = true
}

function editCurrent() {
  if (!selectedChar.value) return
  editingId.value = selectedChar.value.id
  form.name = selectedChar.value.name || ""
  form.description = selectedChar.value.description || ""
  form.personality = selectedChar.value.personality || ""
  form.voiceType = selectedChar.value.voiceType || "zh_female_vv_uranus_bigtts"
  form.voiceSpeed = selectedChar.value.voiceSpeed ?? 1.0
  form.voicePitch = selectedChar.value.voicePitch ?? 0
  form.voiceVolume = selectedChar.value.voiceVolume ?? 1.0
  form.customVoiceId = selectedChar.value.customVoiceId || ""
  form.emotion = selectedChar.value.emotion || ""
  form.emotionScale = selectedChar.value.emotionScale ?? 0
  form.silenceDuration = selectedChar.value.silenceDuration ?? 0
  cloneFile.value = null; cloneName.value = ""; cloneResult.value = ""
  showDialog.value = true
}

function onVoiceTypeChange() {
  const v = voicePresets.value.find((p: any) => p.name === form.voiceType)
  if (v) {
    if (v.supportsEmotion) {
      if (!form.emotion) form.emotion = "happy"
    } else {
      form.emotion = ""
    }
  }
}

async function testVoice() {
  testingVoice.value = true; testAudioUrl.value = ""
  try {
    const token = localStorage.getItem("ai-companion-token")
    const res = await fetch("/api/tts/synthesize", {
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: token ? "Bearer " + token : "" },
      body: JSON.stringify({
        voiceType: form.voiceType,
        text: "你好，我是你的AI伙伴",
        speedRatio: form.voiceSpeed,
        pitchRatio: form.voicePitch,
        volumeRatio: form.voiceVolume,
        emotion: form.emotion || undefined,
        emotionScale: form.emotionScale || undefined,
        silenceDuration: form.silenceDuration || undefined,
      }),
    })
    const json = await res.json()
    testAudioUrl.value = json?.data?.audioUrl || json?.audioUrl || ""
  } catch {}
  finally { testingVoice.value = false }
}

async function ensureTtsConfig() {
  if (!globalApiKey.value) return
  const configs = await apiClient.get("/api/tts/configs").then(r => r.data?.data || [])
  const existing = configs.find((c: any) => c.isActive)
  if (existing) {
    if (!existing.hasApiKey) await apiClient.put(`/api/tts/configs/${existing.id}`, { apiKey: globalApiKey.value })
  } else {
    await apiClient.post("/api/tts/configs", { name: "默认配置", apiKey: globalApiKey.value, voiceType: form.voiceType, isActive: 1 })
  }
}

async function submitClone() {
  if (!cloneFile.value || !cloneName.value.trim()) return
  if (!globalApiKey.value) { ElMessage.warning("请先设置API Key"); return }
  cloneLoading.value = true; cloneResult.value = ""
  try {
    const fd = new FormData()
    fd.append("audio", cloneFile.value)
    fd.append("name", cloneName.value.trim())
    fd.append("language", "cn")

    const url = "/api/tts/voice-clone?apiKey=" + encodeURIComponent(globalApiKey.value)
    const token = localStorage.getItem("ai-companion-token")
    const resp = await fetch(url, { method: "POST", headers: token ? { Authorization: "Bearer " + token } : {}, body: fd })
    const json = await resp.json()
    if (json.code !== 200) { ElMessage.error(json.message || "复刻失败"); return }
    const speakerId = json.data?.speakerId || ""
    form.customVoiceId = speakerId
    cloneResult.value = "复刻成功: " + speakerId
    ElMessage.success("声音复刻成功")
  } catch (err: any) { ElMessage.error(err?.message || "复刻失败") }
  finally { cloneLoading.value = false }
}

async function previewClone() {
  if (!form.customVoiceId) return
  previewCloneLoading.value = true; testAudioUrl.value = ""
  try {
    const configs = await apiClient.get("/api/tts/configs").then(r => r.data?.data || [])
    const cfg = configs.find((c: any) => c.isActive) || configs[0]
    if (!cfg) { ElMessage.warning("未找到音色配置"); return }
    await apiClient.put(`/api/tts/configs/${cfg.id}`, { voiceType: form.customVoiceId })
    const res = await apiClient.post("/api/tts/synthesize", { speakerId: form.customVoiceId, text: "复刻音色试听" })
    testAudioUrl.value = (res as any)?.data?.audioUrl || (res as any)?.audioUrl || ""
  } catch (err: any) { ElMessage.error(err?.message || "试听失败") }
  finally { previewCloneLoading.value = false }
}

async function saveCharacter() {
  saving.value = true
  try {
    const payload: any = {
      name: form.name,
      description: form.description,
      personality: form.personality,
      voiceType: form.voiceType,
      voiceSpeed: form.voiceSpeed,
      voicePitch: form.voicePitch,
      voiceVolume: form.voiceVolume,
      customVoiceId: form.customVoiceId,
      emotion: form.emotion || "",
      emotionScale: form.emotionScale || 0,
      silenceDuration: form.silenceDuration || 0,
    }

    if (editingId.value) {
      await apiClient.put(`/api/characters/${editingId.value}`, payload)
    } else {
      const r = await apiClient.post("/api/characters", payload)
      const created = r.data?.data || r.data
      if (created) {
        selectedId.value = String(created.id)
        selectedChar.value = created
        router.push(`/character/${created.id}/life-rules`)
      }
    }
    ElMessage.success("已保存")
    showDialog.value = false
    await loadCharacters()
  } catch { ElMessage.error("保存失败") }
  finally { saving.value = false }
}

async function deleteCurrent() {
  if (!selectedChar.value) return
  try {
    await ElMessageBox.confirm("确定删除「" + selectedChar.value.name + "」？", "确认", { type: "warning" })
    await apiClient.delete(`/api/characters/${selectedChar.value.id}`)
    ElMessage.success("已删除")
    selectedId.value = null; selectedChar.value = null
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
