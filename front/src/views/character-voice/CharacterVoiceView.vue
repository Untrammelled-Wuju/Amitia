<template>
  <div class="char-voice-page">
    <div class="page-header">
      <div>
        <h2>拟态语音</h2>
        <p class="page-desc">为当前角色配置独立的音色语音系统</p>
      </div>
    </div>

    <div class="cards">
      <div class="card">
        <div class="card-body">
          <div class="mode-row">
            <span class="mode-label">音色模式</span>
            <el-radio-group v-model="voiceMode" @change="onModeChange">
              <el-radio value="preset">预设音色</el-radio>
              <el-radio value="clone">复刻音色</el-radio>
            </el-radio-group>
          </div>

          <div v-if="voiceMode === 'preset'" class="mode-section">
            <div class="form-item">
              <label>选择音色</label>
              <el-select v-model="form.voiceType" placeholder="选择音色" size="default" style="width:100%" @change="onVoiceTypeChange">
                <el-option v-for="v in voicePresets" :key="v.name" :label="v.label" :value="v.name" />
              </el-select>
              <span class="form-hint">从火山引擎预设音色中选择</span>
            </div>
          </div>

          <div v-if="voiceMode === 'clone'" class="mode-section">
            <div class="clone-list" v-if="clonedVoices.length > 0">
              <div class="clone-label-row">
                <span class="form-item-label">已保存的复刻音色</span>
              </div>
              <div
                v-for="v in clonedVoices"
                :key="v.speakerId"
                class="clone-option"
                :class="{ active: form.customVoiceId === v.speakerId }"
                @click="selectCloneVoice(v.speakerId)"
              >
                <div class="clone-info">
                  <span class="clone-name">{{ v.name }}</span>
                  <span class="clone-id">{{ v.speakerId }}</span>
                </div>
                <div class="clone-actions">
                  <el-button size="small" @click.stop="previewClone(v.speakerId)" :loading="previewCloneId === v.speakerId">试听</el-button>
                  <el-button size="small" type="danger" @click.stop="deleteClone(v.speakerId, v.name)">删除</el-button>
                </div>
              </div>
            </div>

            <div class="clone-new-section">
              <div class="form-item-label">训练新音色</div>
              <p class="sub-desc">填入已购买的复刻槽位ID并上传语音样本</p>
              <div class="clone-form-row">
                <el-input v-model="trainSpeakerId" placeholder="槽位ID，如 S_xxxxxxxx" size="default" style="width:220px" />
                <el-input v-model="trainVoiceName" placeholder="备注名称" size="default" style="width:140px" />
              </div>
              <div class="clone-upload-row">
                <el-upload
                  :auto-upload="false"
                  :limit="1"
                  accept=".mp3,.wav,.m4a,.webm,.ogg"
                  :on-change="onCloneFileChange"
                  :file-list="cloneFileList"
                >
                  <el-button type="primary" plain size="small">选择音频文件</el-button>
                  <template #tip>
                    <span class="upload-tip">真实人声，10-30秒</span>
                  </template>
                </el-upload>
              </div>
              <el-button type="success" @click="submitTrain" :loading="trainLoading" :disabled="!trainSpeakerId.trim() || !cloneFile" size="small">
                开始训练
              </el-button>
              <span v-if="trainResult" class="clone-result">{{ trainResult }}</span>
            </div>
          </div>

          <div class="extra-grid">
            <div class="form-item">
              <label>情感</label>
              <el-select v-model="form.emotion" placeholder="默认" size="default" style="width:100%" clearable :disabled="!currentVoiceSupportsEmotion">
                <el-option v-for="e in emotions" :key="e.value" :label="e.label" :value="e.value" />
              </el-select>
              <span class="form-hint">{{ currentVoiceSupportsEmotion ? '设置语音情感色彩' : '当前音色不支持情感参数' }}</span>
            </div>
            <div class="form-item">
              <label>情感强度 {{ form.emotionScale || 4 }}</label>
              <el-slider v-model="form.emotionScale" :min="1" :max="5" :step="1" :disabled="!form.emotion || !currentVoiceSupportsEmotion" />
              <span class="form-hint">仅设置情感后生效，1~5，默认为4</span>
            </div>
            <div class="form-item">
              <label>句尾静音 {{ form.silenceDuration }}ms</label>
              <el-slider v-model="form.silenceDuration" :min="0" :max="5000" :step="100" />
              <span class="form-hint">句尾追加静音时长，0~5000ms</span>
            </div>
          </div>

          <div class="param-grid">
            <div class="param-item">
              <label>
                语速
                <span class="param-value">{{ form.voiceSpeed?.toFixed(1) ?? '1.0' }}x</span>
              </label>
              <div class="sr-body">
                <span class="sr-left">慢</span>
                <div class="sr-slider-wrap">
                  <el-slider v-model="form.voiceSpeed" :min="0.5" :max="2.0" :step="0.1" />
                </div>
                <span class="sr-right">快</span>
              </div>
            </div>
            <div class="param-item">
              <label>
                音调
                <span class="param-value">{{ form.voicePitch > 0 ? '+' + form.voicePitch : form.voicePitch }} 半音</span>
              </label>
              <div class="sr-body">
                <span class="sr-left">-12</span>
                <div class="sr-slider-wrap">
                  <el-slider v-model="form.voicePitch" :min="-12" :max="12" :step="1" />
                </div>
                <span class="sr-right">+12</span>
              </div>
            </div>
            <div class="param-item">
              <label>
                音量
                <span class="param-value">{{ Math.round((form.voiceVolume ?? 1) * 100) }}%</span>
              </label>
              <div class="sr-body">
                <span class="sr-left">小</span>
                <div class="sr-slider-wrap">
                  <el-slider v-model="form.voiceVolume" :min="0.5" :max="2.0" :step="0.1" />
                </div>
                <span class="sr-right">大</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="card-body">
          <h3 class="sub-title">试听测试</h3>
          <p class="sub-desc">使用当前角色的音色进行语音合成试听</p>
          <div class="preview-row">
            <el-input v-model="previewText" placeholder="输入试听文本" size="default" style="flex:1" />
            <el-button type="primary" @click="doPreview" :loading="previewLoading">试听</el-button>
          </div>
          <div v-if="previewAudio" class="audio-preview">
            <audio :src="previewAudio" controls autoplay style="width:100%" />
          </div>
        </div>
      </div>
    </div>

    <div class="save-bar">
      <el-button type="primary" @click="saveVoice" :loading="saving">保存音色配置</el-button>
      <el-button @click="resetForm" :disabled="saving">重置</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, inject, onMounted, computed, type Ref } from "vue"
import { ElMessage, ElMessageBox } from "element-plus"
import { apiClient } from "../../ui-index"

const injectedCharacterId = inject<Ref<string | null>>("currentCharacterId", ref(null))

interface VoicePreset {
  name: string
  label: string
  gender: string
}

const voicePresets = ref<VoicePreset[]>([])
const emotions = [
  { value: "", label: "无" },
  { value: "happy", label: "开心" },
  { value: "sad", label: "悲伤" },
  { value: "angry", label: "愤怒" },
  { value: "fearful", label: "恐惧" },
  { value: "surprised", label: "惊讶" },
  { value: "neutral", label: "中性" },
]
const saving = ref(false)
const previewLoading = ref(false)
const previewText = ref("你好，我是你的专属角色")
const previewAudio = ref("")
const voiceMode = ref<"preset" | "clone">("preset")

const form = reactive({
  voiceType: "zh_female_vv_uranus_bigtts",
  voiceSpeed: 1.0,
  voicePitch: 0,
  voiceVolume: 1.0,
  customVoiceId: "",
  voiceConfigId: "",
  emotion: "",
  emotionScale: 4,
  silenceDuration: 0,
})

const originalForm = reactive({ ...form, _mode: "preset" as string, emotion: "", emotionScale: 4, silenceDuration: 0 })

const trainSpeakerId = ref("")
const trainVoiceName = ref("")
const cloneFile = ref<File | null>(null)
const cloneFileList = ref<any[]>([])
const trainLoading = ref(false)
const trainResult = ref("")
const clonedVoices = ref<any[]>([])
const previewCloneId = ref("")

function loadClonedVoices() {
  const saved = localStorage.getItem("uai-cloned-voices")
  if (saved) {
    try { clonedVoices.value = JSON.parse(saved) } catch {}
  }
}

function saveClonedVoices() {
  localStorage.setItem("uai-cloned-voices", JSON.stringify(clonedVoices.value))
}

function onModeChange(_mode: string) {}

function selectCloneVoice(speakerId: string) {
  form.customVoiceId = speakerId
}

async function loadGlobalApiKey() {
  try {
    const configs = await apiClient.get("/api/tts/configs").then((r: any) => r.data?.data || [])
    const active = configs.find((c: any) => c.isActive)
    if (active) globalApiKey.value = active.apiKey || ""
  } catch {}
}

async function loadVoicePresets() {
  try {
    const r = await apiClient.get("/api/tts/voices")
    const data = r.data?.data || r.data
    if (Array.isArray(data)) voicePresets.value = data
  } catch {}
}

async function loadCharacterVoice() {
  const cid = injectedCharacterId.value
  if (!cid) return
  try {
    const r = await apiClient.get(`/api/characters/${cid}`)
    const data = r.data?.data || r.data
    if (data) {
      form.voiceType = data.voiceType || "zh_female_vv_uranus_bigtts"
      form.voiceSpeed = data.voiceSpeed ?? 1.0
      form.voicePitch = data.voicePitch ?? 0
      form.voiceVolume = data.voiceVolume ?? 1.0
      form.customVoiceId = data.customVoiceId || ""
      form.voiceConfigId = data.voiceConfigId || ""
      form.emotion = data.emotion || ""
      form.emotionScale = data.emotionScale || 4
      if (!currentVoiceSupportsEmotion.value) {
        form.emotion = ""
        form.emotionScale = 4
      }
      form.silenceDuration = data.silenceDuration || 0

      if (data.voiceMode) {
        voiceMode.value = data.voiceMode as "preset" | "clone"
      } else if (data.customVoiceId) {
        voiceMode.value = "clone"
      } else {
        voiceMode.value = "preset"
      }

      Object.assign(originalForm, { ...form, _mode: voiceMode.value })
    }
  } catch {}
}

async function submitClone() {
  if (!cloneFile.value || !cloneForm.name.trim()) return
  cloneLoading.value = true
  cloneResult.value = ""
  try {
    const formData = new FormData()
    formData.append("audio", cloneFile.value)
    formData.append("name", cloneForm.name.trim())
    formData.append("language", "cn")
    if (cloneForm.refText.trim()) formData.append("refText", cloneForm.refText.trim())

    const token = localStorage.getItem("ai-companion-token")
    if (!globalApiKey.value) { ElMessage.warning("请先在音色配置中设置API Key"); cloneLoading.value = false; return }
    const url = "/api/tts/voice-clone?apiKey=" + encodeURIComponent(globalApiKey.value)
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
    const newVoice = {
      speakerId: data.speakerId,
      name: data.name || cloneForm.name,
      createdAt: new Date().toISOString(),
    }
    clonedVoices.value.unshift(newVoice)
    saveClonedVoices()
    form.customVoiceId = data.speakerId
    cloneResult.value = "复刻成功: " + data.speakerId
    ElMessage.success("音色复刻成功，已自动选中")
    cloneForm.name = ""
    cloneForm.refText = ""
    cloneFile.value = null
    cloneFileList.value = []
  } catch (err: any) {
    ElMessage.error(err?.message || "复刻失败")
  } finally {
    cloneLoading.value = false
  }
}

async function previewClone(speakerId: string) {
  previewCloneId.value = speakerId
  try {
    const res: any = await apiClient.post("/api/tts/synthesize", {
      speakerId: speakerId,
      text: "测试",
    })
    const data = res.data?.data || res.data
    if (data?.audioUrl) {
      previewAudio.value = data.audioUrl
    } else {
      ElMessage.warning("未能获取音频")
    }
  } catch {
    ElMessage.error("试听失败，请检查全局音色配置")
  } finally {
    previewCloneId.value = ""
  }
}

async function deleteClone(speakerId: string, name: string) {
  try {
    await ElMessageBox.confirm('确定删除音色"' + name + '"吗？', "确认", { type: "warning", confirmButtonText: "删除" })
    clonedVoices.value = clonedVoices.value.filter((v: any) => v.speakerId !== speakerId)
    if (form.customVoiceId === speakerId) {
      form.customVoiceId = ""
    }
    saveClonedVoices()
    ElMessage.success("已删除")
  } catch {}
}

async function doPreview() {
  if (!previewText.value.trim()) {
    ElMessage.warning("请输入试听文本")
    return
  }
  previewLoading.value = true
  previewAudio.value = ""
  try {
    const res = await apiClient.post("/api/tts/synthesize", {
      characterId: injectedCharacterId.value,
      text: previewText.value,
    })
    const data = res.data?.data || res.data
    if (data?.audioUrl) {
      previewAudio.value = data.audioUrl
    } else {
      ElMessage.warning("未能获取音频，请检查全局音色配置")
    }
  } catch {
    ElMessage.warning("试听失败，请检查全局音色配置")
  } finally {
    previewLoading.value = false
  }
}

async function saveVoice() {
  const cid = injectedCharacterId.value
  if (!cid) {
    ElMessage.warning("未找到角色 ID")
    return
  }
  saving.value = true
  try {
    await apiClient.put(`/api/characters/${cid}`, {
      voiceType: form.voiceType,
      voiceSpeed: form.voiceSpeed,
      voicePitch: form.voicePitch,
      voiceVolume: form.voiceVolume,
      customVoiceId: form.customVoiceId,
      voiceConfigId: form.voiceConfigId || "",
      voiceMode: voiceMode.value,
      emotion: form.emotion || "",
      emotionScale: form.emotionScale || 0,
      silenceDuration: form.silenceDuration || 0,
    })
    ElMessage.success("音色配置已保存")
    Object.assign(originalForm, { ...form, _mode: voiceMode.value })
  } catch (e: any) {
    ElMessage.error(e?.message || "保存失败")
  } finally {
    saving.value = false
  }
}

function resetForm() {
  Object.assign(form, { 
    voiceType: (originalForm as any).voiceType,
    voiceSpeed: (originalForm as any).voiceSpeed,
    voicePitch: Number((originalForm as any).voicePitch),
    voiceVolume: (originalForm as any).voiceVolume,
    customVoiceId: (originalForm as any).customVoiceId,
    voiceConfigId: (originalForm as any).voiceConfigId,
    emotion: (originalForm as any).emotion || "",
    emotionScale: (originalForm as any).emotionScale || 4,
    silenceDuration: (originalForm as any).silenceDuration || 0,
  })
  voiceMode.value = (originalForm as any)._mode || "preset"
  ElMessage.info("已重置为上次保存的值")
}

onMounted(() => {
  loadVoicePresets()
  loadCharacterVoice()
  loadClonedVoices()
})
</script>

<style scoped>
.char-voice-page {
  padding: 20px 24px;
  max-width: 780px;
  margin: 0 auto;
}

.page-header {
  margin-bottom: 16px;
}

.page-header h2 {
  font-size: 18px;
  font-weight: 600;
  margin: 0;
}

.page-desc {
  font-size: 13px;
  color: var(--ac-color-text-muted);
  margin: 4px 0 0;
}

.cards {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.card {
  background: var(--ac-color-surface);
  border: 1px solid var(--ac-color-border-light);
  border-radius: var(--ac-radius-md);
  overflow: hidden;
}

.card-body {
  padding: 16px;
}

.sub-title {
  font-size: 14px;
  font-weight: 600;
  margin: 0 0 4px;
}

.sub-desc {
  font-size: 12px;
  color: var(--ac-color-text-muted);
  margin: 0 0 12px;
}

.mode-row {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--ac-color-border-light);
}

.mode-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--ac-color-text);
}

.mode-section {
  margin-bottom: 16px;
}

.form-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.form-item label {
  font-size: 12px;
  font-weight: 500;
  color: var(--ac-color-text);
}

.form-item-label {
  font-size: 12px;
  font-weight: 500;
  color: var(--ac-color-text);
  margin-bottom: 8px;
}

.form-hint {
  font-size: 11px;
  color: var(--ac-color-text-placeholder);
  line-height: 1.3;
}

.param-grid {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 14px 20px;
  padding-top: 12px;
  border-top: 1px solid var(--ac-color-border-light);
}

.extra-grid {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 14px 20px;
  margin-top: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--ac-color-border-light);
}

.param-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.param-item label {
  font-size: 12px;
  font-weight: 500;
  color: var(--ac-color-text);
  display: flex;
  align-items: center;
  gap: 8px;
}

.param-value {
  font-size: 11px;
  font-weight: 700;
  color: var(--ac-color-primary);
}

.sr-body {
  display: flex;
  align-items: center;
  gap: 6px;
}

.sr-left,
.sr-right {
  font-size: 10px;
  color: var(--ac-color-text-placeholder);
  min-width: 20px;
}

.sr-left {
  text-align: right;
}

.sr-slider-wrap {
  flex: 1;
}

.clone-list {
  margin-bottom: 16px;
}

.clone-label-row {
  margin-bottom: 6px;
}

.clone-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  border: 1px solid var(--ac-color-border-light);
  border-radius: 6px;
  margin-bottom: 6px;
  cursor: pointer;
  transition: border-color 0.2s, background 0.2s;
}

.clone-option:hover {
  border-color: var(--ac-color-primary);
  background: var(--ac-color-primary-bg);
}

.clone-option.active {
  border-color: var(--ac-color-primary);
  background: var(--ac-color-primary-light-9);
}

.clone-info {
  display: flex;
  gap: 12px;
  align-items: center;
}

.clone-name {
  font-size: 13px;
  font-weight: 500;
}

.clone-id {
  font-size: 11px;
  color: var(--ac-color-text-muted);
  font-family: monospace;
}

.clone-actions {
  display: flex;
  gap: 6px;
}

.clone-new-section {
  padding-top: 12px;
  border-top: 1px solid var(--ac-color-border-light);
}

.clone-form-row {
  display: flex;
  gap: 10px;
  margin-bottom: 8px;
}

.clone-upload-row {
  margin-bottom: 10px;
}

.upload-tip {
  font-size: 11px;
  color: var(--ac-color-text-placeholder);
}

.clone-result {
  margin-left: 10px;
  font-size: 13px;
  color: var(--el-color-success);
}

.preview-row {
  display: flex;
  gap: 10px;
  margin-bottom: 12px;
}

.audio-preview {
  margin-top: 12px;
}

.save-bar {
  display: flex;
  gap: 10px;
  margin-top: 20px;
  padding-top: 16px;
  border-top: 1px solid var(--ac-color-border-light);
}

@media (max-width: 700px) {
  .param-grid, .extra-grid {
    grid-template-columns: 1fr;
  }
  .preview-row {
    flex-direction: column;
  }
  .clone-form-row {
    flex-direction: column;
  }
  .clone-option {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }
}
</style>