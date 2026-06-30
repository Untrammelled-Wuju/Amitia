<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="voice-config">
    <el-alert
      title="隐私提示"
      type="info"
      description="API Key 与语音配置仅存储在本地服务器，不会上传至第三方。"
      show-icon
      closable
      style="margin-bottom: 16px"
    />

    <el-card class="tts-card" shadow="hover">
      <template #header>
        <div class="tts-header">
          <span>语音识别/合成</span>
          <el-tag size="small" type="success">火山引擎</el-tag>
        </div>
      </template>
            <div class="tts-grid">
        <div class="tts-item">
          <span class="tts-label">API Key</span>
          <el-input v-model="ttsApiKey" size="small" placeholder="火山引擎API Key" type="password" show-password style="width:260px" />
        </div>
        <div class="tts-item">
          <span class="tts-label">语音合成大模型</span>
          <el-input v-model="ttsResourceId" size="small" placeholder="seed-tts-2.0" style="width:260px" />
        </div>
        <div class="tts-item">
          <span class="tts-label">复刻资源ID</span>
          <el-input v-model="cloneResourceId" size="small" placeholder="volc.megatts.timbre" style="width:260px" />
        </div>

        <div class="tts-item">
          <span class="tts-label">APP ID</span>
          <el-input v-model="realtimeAppId" size="small" placeholder="火山引擎APP ID" style="width:260px" />
        </div>
        <div class="tts-item">
          <span class="tts-label">Access Token</span>
          <el-input v-model="realtimeAccessToken" size="small" placeholder="火山引擎Access Token" type="password" show-password style="width:260px" />
        </div>
        <div class="tts-item">
          <span class="tts-label">Secret Key</span>
          <el-input v-model="realtimeSecretKey" size="small" placeholder="火山引擎Secret Key" type="password" show-password style="width:260px" />
        </div>
        <div class="tts-item">
          <span class="tts-label">默认语音</span>
          <el-select v-model="ttsVoiceType" size="small" style="width:240px">
            <el-option v-for="v in voicePresets" :key="v.name" :label="v.label" :value="v.name" />
          </el-select>
        </div>
        <div class="tts-item">
          <span class="tts-label">语速</span>
          <el-slider v-model="ttsSpeed" :min="0.5" :max="2.0" :step="0.1" size="small" show-input style="width:180px" />
        </div>
        <div class="tts-item">
          <span class="tts-label">音调(半音)</span>
          <el-slider v-model="ttsPitch" :min="-12" :max="12" :step="1" size="small" show-input style="width:180px" />
        </div>
        <div class="tts-item">
          <span class="tts-label">音量</span>
          <el-slider v-model="ttsVolume" :min="0.5" :max="2.0" :step="0.1" size="small" show-input style="width:180px" />
        </div>
        <div class="tts-item">
          <span class="tts-label">试听</span>
          <el-button size="small" @click="testTts" :loading="ttsTesting">测试</el-button>
          <audio v-if="ttsAudio" :src="ttsAudio" controls style="width:200px;height:28px;margin-left:8px" />
        </div>
        <div class="tts-item" v-if="ttsTestResult">
          <span class="tts-label">状态</span>
          <span :style="{color: ttsTestResult==='ok'?'#67c23a':'#f56c6c'}">{{ ttsTestResult === 'ok' ? '连接正常' : '连接失败' }}</span>
        </div>
      </div>
      <div style="margin-top:12px;font-size:12px;color:var(--el-text-color-secondary);display:flex;flex-wrap:wrap;gap:12px">
        <span>快捷入口：</span>
        <el-link href="https://console.volcengine.com/speech/new/setting/apikeys?projectName=default" target="_blank" class="quick-link">API Key管理</el-link>
        <el-link href="https://console.volcengine.com/speech/service/10035?AppID=3815252154" target="_blank" class="quick-link">豆包语音合成模型</el-link>
        <el-link href="https://console.volcengine.com/speech/service/10036?AppID=3815252154" target="_blank" class="quick-link">声音复刻模型</el-link>
        <el-link href="https://console.volcengine.com/speech/new/experience/clone?_vtm_=a86845.b103859.0_0.0_0.0.242_7650005566333666822" target="_blank" class="quick-link">声音复刻</el-link>
        <el-link href="https://console.volcengine.com/speech/new/voices?_vtm_=a86845.b103859.0_0.0_0.0.242_7650005566333666822" target="_blank" class="quick-link">音色管理</el-link>
      </div>
      <div style="margin-top:12px">
        <el-button type="primary" size="small" @click="saveAllTts" :loading="ttsSaving">保存配置</el-button>
      </div>
    </el-card>

    <el-empty v-if="!ttsConfig" description="暂无语音模型配置" />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue"
import { ElMessage } from "element-plus"
import { useApi } from "../../composables/useApi"

const { get, post, put } = useApi()

const ttsConfig = ref<any>(null)
const ttsApiKey = ref("")
const ttsVoiceType = ref("zh_female_vv_uranus_bigtts")
const ttsResourceId = ref("seed-tts-2.0")
const cloneResourceId = ref("volc.megatts.timbre")
const realtimeAppId = ref("")
const realtimeAccessToken = ref("")
const realtimeSecretKey = ref("")
const ttsSpeed = ref(1.0)
const ttsPitch = ref(0)
const ttsVolume = ref(1.0)
const ttsAudio = ref("")
const ttsTesting = ref(false)
const ttsTestResult = ref("")
const voicePresets = ref<any[]>([])

const ttsSaving = ref(false)


onMounted(async () => {
  fetchTtsConfig()
  fetchVoices()
})

async function fetchTtsConfig() {
  try {
    const r: any = await get("/api/tts/configs")
    const configs = r || []
    const cfg = configs.find((c: any) => c.isActive) || configs[0]
    if (cfg) {
      ttsConfig.value = cfg
      ttsApiKey.value = cfg.apiKey || ""
      ttsResourceId.value = cfg.resourceId || "seed-tts-2.0"
        cloneResourceId.value = cfg.cloneResourceId || "volc.megatts.timbre"
      ttsVoiceType.value = cfg.voiceType || "zh_female_vv_uranus_bigtts"
      ttsSpeed.value = cfg.speed || 1.0
      ttsPitch.value = cfg.pitch || 0
      ttsVolume.value = cfg.volume || 1.0
      realtimeAppId.value = cfg.realtimeAppId || ""
      realtimeAccessToken.value = cfg.realtimeAccessToken || ""
      realtimeSecretKey.value = cfg.realtimeSecretKey || ""
    }
  } catch (e: any) { console.error("fetchTtsConfig failed", e); ttsConfig.value = null }
}


async function fetchVoices() {
  try { voicePresets.value = await get("/api/tts/voices") } catch { voicePresets.value = [] }
}

async function saveAllTts() {
  ttsSaving.value = true
  try {
    const payload: any = { apiKey: ttsApiKey.value, resourceId: ttsResourceId.value, cloneResourceId: cloneResourceId.value, voiceType: ttsVoiceType.value, speed: ttsSpeed.value, pitch: ttsPitch.value, volume: ttsVolume.value, realtimeAppId: realtimeAppId.value, realtimeAccessToken: realtimeAccessToken.value, realtimeSecretKey: realtimeSecretKey.value }
    if (ttsConfig.value?.id) {
      await put("/api/tts/configs/" + ttsConfig.value.id, payload)
    } else {
      const r = await post("/api/tts/configs", { ...payload, name: "默认配置", isActive: 1 })
      ttsConfig.value = r
    }
    ttsTestResult.value = ""
    ElMessage.success("语音配置保存成功")
  } catch (err: any) { ElMessage.error(err?.message || "保存失败") }
  finally { ttsSaving.value = false }
}

async function testTts() {
  ttsTesting.value = true; ttsAudio.value = ""; ttsTestResult.value = ""
  try {
    if (!ttsConfig.value?.id) {
      const payload: any = { apiKey: ttsApiKey.value, resourceId: ttsResourceId.value, cloneResourceId: cloneResourceId.value, voiceType: ttsVoiceType.value, speed: ttsSpeed.value, pitch: ttsPitch.value, volume: ttsVolume.value, realtimeAppId: realtimeAppId.value, realtimeAccessToken: realtimeAccessToken.value, realtimeSecretKey: realtimeSecretKey.value }
      const r = await post("/api/tts/configs", { ...payload, name: "默认配置", isActive: 1 })
      ttsConfig.value = r
      if (!ttsConfig.value?.id) { return }
    }
    const res: any = await post("/api/tts/synthesize", { voiceId: ttsConfig.value.id, text: "测试" })
    ttsAudio.value = res?.audioUrl || res?.data?.audioUrl || ""
    ttsTestResult.value = "ok"
  } catch {
    ttsTestResult.value = "fail"
  } finally { ttsTesting.value = false }
}

</script>

<style scoped>
.voice-config { padding: 0; }
.tts-card { margin-bottom: 16px; }
.tts-header { display: flex; align-items: center; gap: 8px; font-weight: 600; font-size: 15px; }
.tts-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 12px; }
.tts-item { display: flex; align-items: center; gap: 8px; }
.tts-label { font-size: 13px; color: var(--el-text-color-secondary); min-width: 60px; flex-shrink: 0; }
.tts-val { font-size: 13px; color: var(--el-text-color-regular); }
.quick-link { color: var(--el-color-primary) !important; text-decoration: underline !important; }
.quick-link:hover { color: var(--el-color-primary) !important; text-decoration: underline !important; }
</style>