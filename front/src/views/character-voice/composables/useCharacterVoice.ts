// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, reactive, inject, onMounted, computed, type Ref } from "vue"
import { ElMessage, ElMessageBox } from "element-plus"
import { apiClient } from "../../../ui-index"

interface VoicePreset {
  name: string
  label: string
  gender: string
}

export function useCharacterVoice() {
  const injectedCharacterId = inject<Ref<string | null>>("currentCharacterId", ref(null))

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

  const globalApiKey = ref("")
  const cloneForm = reactive({ name: "", refText: "" })
  const cloneLoading = ref(false)
  const cloneResult = ref("")

  const currentVoiceSupportsEmotion = computed(() => {
    const v = voicePresets.value.find((p) => p.name === form.voiceType)
    return !!v
  })

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

  function onVoiceTypeChange() {}

  function onCloneFileChange(file: any) {
    cloneFile.value = file?.raw || file
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
        headers: token ? { Authorization: "Bearer " + token } : {},
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

  async function submitTrain() {
    await submitClone()
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

  return {
    voicePresets, emotions, saving, previewLoading, previewText, previewAudio,
    voiceMode, form, originalForm,
    trainSpeakerId, trainVoiceName, cloneFile, cloneFileList, trainLoading, trainResult,
    clonedVoices, previewCloneId,
    globalApiKey, cloneForm, cloneLoading, cloneResult,
    currentVoiceSupportsEmotion,
    onModeChange, selectCloneVoice, onVoiceTypeChange, onCloneFileChange,
    loadVoicePresets, loadCharacterVoice, loadClonedVoices,
    submitClone, submitTrain, previewClone, deleteClone,
    doPreview, saveVoice, resetForm,
  }
}
