<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="page">
    <h2 class="page-title">音色配置</h2>

    <el-alert type="warning" :closable="false" show-icon style="margin-bottom:14px">
      <template #title>
        语音合成按字符计费。试听每次合成2-4字，费用极低。相同文本+参数仅首次调用API，后续命中本地缓存不收费。
      </template>
    </el-alert>

    <div class="toolbar">
      <el-button type="primary" :icon="Plus" @click="showDialog(null)">新增配置</el-button>
    </div>

    <div class="config-cards" v-if="configs.length > 0">
      <div v-for="cfg in configs" :key="cfg.id" class="config-card" :class="{ 'is-active': cfg.isActive }">
        <div class="card-top">
          <div class="card-header">
            <span class="card-name">{{ cfg.name }}</span>
            <el-tag v-if="cfg.isActive" type="success" size="small" effect="dark">默认</el-tag>
            <el-tag v-if="cfg.emotion" size="small" type="warning">{{ emotionLabel(cfg.emotion) }}</el-tag>
          </div>
          <div class="card-type">
            <el-tag size="small" type="primary">火山引擎</el-tag>
            <span class="card-model">{{ voiceLabel(cfg.voiceType) }}</span>
          </div>
        </div>
        <div class="card-details">
          <div class="detail-row"><span class="dl">资源</span><span class="dv">{{ cfg.resourceId || 'seed-tts-2.0' }}</span></div>
          <div class="detail-row"><span class="dl">API Key</span><span class="dv">{{ cfg.hasApiKey ? '已设置' : '未设置' }}</span></div>
          <div class="detail-row" v-if="cfg.lastTestResult"><span class="dl">状态</span><span class="dv" :style="{color: cfg.lastTestResult === 'success' ? '#67c23a' : '#f56c6c'}">{{ cfg.lastTestResult === 'success' ? '连接正常' : '连接失败' }}</span></div>
          <div class="detail-row"><span class="dl">语速</span><span class="dv">{{ cfg.speed?.toFixed(1) ?? '1.0' }}x</span></div>
          <div class="detail-row"><span class="dl">音调</span><span class="dv">{{ cfg.pitch?.toFixed(1) ?? '1.0' }}x</span></div>
          <div class="detail-row"><span class="dl">音量</span><span class="dv">{{ Math.round((cfg.volume ?? 1) * 100) }}%</span></div>
        </div>
        <div class="card-actions">
          <el-popover placement="top" :width="280" trigger="click">
            <template #reference>
              <el-button size="small" :loading="testingId === cfg.id">试听</el-button>
            </template>
            <div>
              <el-input v-model="previewText" size="small" placeholder="试听文本" style="margin-bottom:8px" />
              <el-button size="small" type="primary" @click="doPreview(cfg.id)" :loading="previewLoading">播放试听</el-button>
              <audio v-if="previewAudio" :src="previewAudio" controls autoplay style="width:100%;margin-top:8px" />
            </div>
          </el-popover>
          <el-button size="small" @click="showDialog(cfg)">编辑</el-button>
          <el-button v-if="!cfg.isActive" size="small" type="primary" @click="setActive(cfg.id)">设为默认</el-button>
          <el-button size="small" type="danger" :disabled="cfg.isActive && configs.length <= 1" @click="delConfig(cfg.id)">删除</el-button>
        </div>
      </div>
    </div>


    <div class="clone-section" v-if="configs.length > 0">
      <h3 class="section-title">声音复刻</h3>
      <p class="section-desc">上传一段10-30秒的清晰语音样本，创建你自己的专属音色。创建后可像预设音色一样使用。<br><strong style="color:var(--el-color-warning)">⚠️ 音色首次合成即转正收费，请务必试听满意后再正式使用。</strong></p>
      <div class="clone-cards">
        <div v-for="v in clonedVoices" :key="v.speakerId" class="clone-card">
          <div class="clone-card-info">
            <span class="clone-name">{{ v.name }}</span>
            <span class="clone-id">{{ v.speakerId }}</span>
            <span class="clone-time">{{ v.createdAt?.slice(0, 10) }}</span>
          </div>
          <div class="clone-card-actions">
            <el-button size="small" @click="previewClone(v.speakerId)" :loading="previewCloneId === v.speakerId">试听</el-button>
            <el-button size="small" type="danger" @click="deleteClone(v.speakerId, v.name)">删除</el-button>
          </div>
        </div>
      </div>
      <el-button type="primary" size="small" style="margin-top:10px" @click="showCloneDialog = true" :icon="Plus">复刻新音色</el-button>
    </div>

    <el-dialog v-model="showCloneDialog" title="声音复刻" width="480px" destroy-on-close>
      <el-form :model="cloneForm" label-position="top">
        <el-form-item label="音色名称（英文）">
          <el-input v-model="cloneForm.name" placeholder="例如: my_voice_01" />
          <div class="form-hint">8-256位，首字符为英文字母，允许数字、字母、-、_</div>
        </el-form-item>
        <el-form-item label="语言">
          <el-select v-model="cloneForm.language" style="width:100%">
            <el-option value="cn" label="中文" />
            <el-option value="en" label="英文" />
            <el-option value="ja" label="日语" />
          </el-select>
        </el-form-item>
        <el-form-item label="参考文本（可选）">
          <el-input v-model="cloneForm.refText" type="textarea" :rows="2" placeholder="音频中说的话语文本，用于提升复刻质量" />
        </el-form-item>
        <el-form-item label="上传音频">
          <el-upload
            :auto-upload="false"
            :limit="1"
            accept=".mp3,.wav,.ogg,.m4a,.aac,.pcm"
            :on-change="onCloneFileChange"
            :on-remove="() => cloneForm.audioFile = null"
          >
            <el-button size="small">选择音频文件</el-button>
            <template #tip>
              <div class="form-hint">支持 mp3/wav/ogg/m4a/aac/pcm，最大10MB。10-30秒清晰语音效果最佳</div>
            </template>
          </el-upload>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCloneDialog = false">取消</el-button>
        <el-button type="primary" @click="submitClone" :loading="cloneLoading" :disabled="!cloneForm.audioFile || !cloneForm.name.trim()">开始复刻</el-button>
      </template>
    </el-dialog>

    <el-empty v-if="configs.length === 0" description="还没有音色配置" :image-size="80">
      <el-button type="primary" @click="showDialog(null)">新增配置</el-button>
    </el-empty>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑音色配置' : '新增音色配置'" width="520px" destroy-on-close>
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" @submit.prevent>
        <el-form-item label="名称" prop="name">
          <el-input v-model="form.name" placeholder="例如: 默认女声" />
        </el-form-item>

        <el-form-item label="API Key" prop="apiKey">
          <el-input v-model="form.apiKey" type="password" show-password :placeholder="editingId ? '留空则保留现有 Key' : '火山引擎控制台 API Key'" />
        </el-form-item>

        <el-form-item label="资源ID">
          <el-select v-model="form.resourceId" style="width:100%">
            <el-option value="seed-tts-2.0" label="语音合成2.0 (推荐)" />
            <el-option value="seed-tts-1.0" label="语音合成1.0" />
            <el-option value="seed-icl-2.0" label="声音复刻2.0" />
            <el-option value="seed-icl-1.0" label="声音复刻1.0" />
          </el-select>
        </el-form-item>

        <el-form-item label="音色" prop="voiceType">
          <el-select v-model="form.voiceType" style="width:100%" filterable>
            <el-option v-for="v in availableVoices" :key="v.name" :label="v.label" :value="v.name">
              <span>{{ v.label }}</span>
              <span style="float:right;font-size:11px;color:var(--el-text-color-secondary)">{{ v.gender === 'male' ? '♂' : '♀' }}</span>
            </el-option>
          </el-select>
        </el-form-item>

        <el-form-item label="情感风格">
          <el-select v-model="form.emotion" style="width:100%" clearable placeholder="不指定">
            <el-option v-for="e in emotions" :key="e.value" :label="e.label" :value="e.value" />
          </el-select>
        </el-form-item>

        <el-row :gutter="12">
          <el-col :span="8">
            <el-form-item label="语速"><el-input-number v-model="form.speed" :min="0.5" :max="2.0" :step="0.1" :precision="1" controls-position="right" style="width:100%" /></el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="音调"><el-input-number v-model="form.pitch" :min="0.5" :max="2.0" :step="0.1" :precision="1" controls-position="right" style="width:100%" /></el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="音量"><el-input-number v-model="form.volume" :min="0.1" :max="2.0" :step="0.1" :precision="1" controls-position="right" style="width:100%" /></el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveConfig">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from "vue"
import { Plus } from "@element-plus/icons-vue"
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from "element-plus"
import { useApi } from "../../composables/useApi"
import type { TtsConfig, VoicePreset } from "@/types"

const { get, post, put, del } = useApi()

const configs = ref<TtsConfig[]>([])
const availableVoices = ref<VoicePreset[]>([])
const dialogVisible = ref(false)
const editingId = ref<number | null>(null)
const saving = ref(false)
const testingId = ref<number | null>(null)
const previewText = ref("测试")
const previewAudio = ref("")
const previewLoading = ref(false)

const emotions = [
  { value: "", label: "无" },
  { value: "happy", label: "开心" }, { value: "sad", label: "悲伤" },
  { value: "angry", label: "生气" }, { value: "fearful", label: "恐惧" },
  { value: "disgusted", label: "厌恶" }, { value: "surprised", label: "惊讶" },
  { value: "neutral", label: "中性" },
]

const form = reactive({
  name: "", apiKey: "", resourceId: "seed-tts-2.0",
  voiceType: "zh_female_cancan_mars_bigtts",
  emotion: "", speed: 1.0, pitch: 1.0, volume: 1.0,
})

const rules: FormRules = {
  name: [{ required: true, message: "请输入名称", trigger: "blur" }],
  apiKey: [{ required: true, message: "请输入 API Key", trigger: "blur" }],
  voiceType: [{ required: true, message: "请选择音色", trigger: "change" }],
}

async function fetchConfigs() { configs.value = await get<TtsConfig[]>("/api/tts/configs") || [] }
async function fetchVoices() { availableVoices.value = await get<VoicePreset[]>("/api/tts/voices") || [] }

function voiceLabel(name: string): string {
  const v = availableVoices.value.find((v: VoicePreset) => v.name === name)
  return v?.label || name
}
function emotionLabel(e: string): string {
  const found = emotions.find((x) => x.value === e)
  return found?.label || e
}

function showDialog(cfg: TtsConfig | null) {
  previewAudio.value = ""
  if (cfg) {
    editingId.value = cfg.id
    form.name = cfg.name; form.apiKey = "";
    form.resourceId = cfg.resourceId || "seed-tts-2.0";
    form.voiceType = cfg.voiceType; form.emotion = cfg.emotion || "";
    form.speed = cfg.speed; form.pitch = cfg.pitch; form.volume = cfg.volume;
  } else {
    editingId.value = null
    form.name = ""; form.apiKey = ""; form.voiceType = "zh_female_cancan_mars_bigtts";
    form.emotion = ""; form.speed = 1.0; form.pitch = 1.0; form.volume = 1.0;
  }
  dialogVisible.value = true
}

async function saveConfig() {
  saving.value = true
  try {
    const payload: Record<string, any> = {
      name: form.name, resourceId: form.resourceId, voiceType: form.voiceType,
      emotion: form.emotion, speed: form.speed, pitch: form.pitch, volume: form.volume,
    }
    if (editingId.value) {
      if (form.apiKey) payload.apiKey = form.apiKey
      await put('/api/tts/configs/' + editingId.value, payload)
      ElMessage.success("已更新")
    } else {
      payload.apiKey = form.apiKey; payload.resourceId = form.resourceId
      await post("/api/tts/configs", payload)
      ElMessage.success("已创建")
    }
    dialogVisible.value = false
    fetchConfigs()
  } catch (err: any) { ElMessage.error(err?.message || "保存失败") }
  finally { saving.value = false }
}

async function setActive(id: number) {
  try { await post('/api/tts/configs/' + id + '/activate'); ElMessage.success("已设为默认"); fetchConfigs() }
  catch (err: any) { ElMessage.error(err?.message || "操作失败") }
}

async function delConfig(id: number) {
  try {
    await ElMessageBox.confirm("确定删除？", "确认", { type: "warning", confirmButtonText: "删除" })
    await del('/api/tts/configs/' + id); ElMessage.success("已删除"); fetchConfigs()
  } catch {}
}

async function testConnection(id: number) {
  testingId.value = id
  try {
    await post('/api/tts/configs/' + id + '/test')
    ElMessage.success("连接测试通过")
    fetchConfigs()
  } catch (err: any) { ElMessage.error(err?.message || "连接失败") }
  finally { testingId.value = null }
}

async function doPreview(voiceId: number) {
  previewLoading.value = true; previewAudio.value = ""
  try {
    const res = await post<any>("/api/tts/synthesize", { voiceId, text: previewText.value || "测试" })
    previewAudio.value = (res as any)?.audioUrl || ""
    if (!previewAudio.value) ElMessage.warning("未能获取音频")
  } catch (err: any) { ElMessage.error(err?.message || "试听失败") }
  finally { previewLoading.value = false }
}

onMounted(() => { fetchConfigs(); fetchVoices(); fetchClonedVoices() })
const showCloneDialog = ref(false)
const cloneLoading = ref(false)
const clonedVoices = ref<any[]>([])
const previewCloneId = ref("")
const cloneForm = reactive({
  name: "",
  language: "cn",
  refText: "",
  audioFile: null as File | null,
})

function onCloneFileChange(file: any) {
  cloneForm.audioFile = file.raw || file
}

async function fetchClonedVoices() {
  const saved = localStorage.getItem("uai-cloned-voices")
  if (saved) {
    try { clonedVoices.value = JSON.parse(saved) } catch {}
  }
}

function saveClonedVoices() {
  localStorage.setItem("uai-cloned-voices", JSON.stringify(clonedVoices.value))
}

async function submitClone() {
  if (!cloneForm.audioFile || !cloneForm.name.trim()) return
  cloneLoading.value = true
  try {
    const formData = new FormData()
    formData.append("audio", cloneForm.audioFile)
    formData.append("name", cloneForm.name.trim())
    formData.append("language", cloneForm.language)
    if (cloneForm.refText.trim()) formData.append("refText", cloneForm.refText.trim())

    const apiKey = configs.value.find((c: any) => c.hasApiKey)?.apiKey || ""
    const url = '/api/tts/voice-clone' + (apiKey ? '?apiKey=' + encodeURIComponent(apiKey) : '')
    
    const token = localStorage.getItem("ai-companion-token")
    const resp = await fetch(url, {
      method: "POST",
      headers: token ? { Authorization: 'Bearer ' + token } : {},
      body: formData,
    })
    const json = await resp.json()
    if (json.code !== 200) {
      ElMessage.error(json.message || "复刻失败")
      return
    }
    const data = json.data
    clonedVoices.value.unshift({
      speakerId: data.speakerId,
      name: data.name || cloneForm.name,
      createdAt: new Date().toISOString(),
    })
    saveClonedVoices()
    ElMessage.success("音色复刻成功！可用于语音合成")
    showCloneDialog.value = false
  } catch (err: any) {
    ElMessage.error(err?.message || "复刻失败")
  } finally {
    cloneLoading.value = false
  }
}

async function previewClone(speakerId: string) {
  previewCloneId.value = speakerId
  try {
    const apiKey = configs.value.find((c: any) => c.hasApiKey)?.apiKey || ""
    const res = await post<any>('/api/tts/synthesize', {
      voiceId: 0,
      text: "测试",
    })
    previewAudio.value = (res as any)?.audioUrl || ""
  } catch (err: any) {
    ElMessage.error(err?.message || "试听失败")
  } finally {
    previewCloneId.value = ""
  }
}

async function deleteClone(speakerId: string, name: string) {
  try {
    await ElMessageBox.confirm('确定删除音色"' + name + '"吗？', "确认", { type: "warning", confirmButtonText: "删除" })
    clonedVoices.value = clonedVoices.value.filter((v: any) => v.speakerId !== speakerId)
    saveClonedVoices()
    ElMessage.success("已删除")
  } catch {}
}

</script>

<style scoped>
.page { max-width: 720px; margin: 0 auto; padding: 20px 16px; }
.page-title { font-size: 20px; font-weight: 600; margin: 0 0 16px; }
.toolbar { margin-bottom: 14px; display: flex; gap: 8px; }
.config-cards { display: flex; flex-direction: column; gap: 10px; }
.config-card { background: var(--el-bg-color); border: 1px solid var(--el-border-color-light); border-radius: 8px; padding: 14px; transition: border-color 0.2s; }
.config-card.is-active { border-color: var(--el-color-primary); }
.card-top { display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; }
.card-header { display: flex; align-items: center; gap: 8px; }
.card-name { font-weight: 600; font-size: 15px; }
.card-type { display: flex; align-items: center; gap: 6px; }
.card-model { font-size: 12px; color: var(--el-text-color-secondary); }
.card-details { display: grid; grid-template-columns: 1fr 1fr; gap: 4px 12px; margin-bottom: 10px; font-size: 13px; }
.detail-row { display: flex; justify-content: space-between; }
.dl { color: var(--el-text-color-secondary); }
.dv { color: var(--el-text-color-regular); }
.card-actions { display: flex; gap: 8px; flex-wrap: wrap; }
</style>
