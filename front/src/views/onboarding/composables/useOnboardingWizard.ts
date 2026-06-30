// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, reactive, watch, onUnmounted } from "vue"
import { useRouter } from "vue-router"
import { ElMessage } from "element-plus"
import { useApi, setToken } from "../../../ui-index"
import axios from "axios"

export const steps = [
  { label: "欢迎", key: "welcome" },
  { label: "部署", key: "deploy" },
  { label: "账号", key: "admin" },
  { label: "模型", key: "model" },
  { label: "角色", key: "character" },
  { label: "画像", key: "profile" },
  { label: "Web", key: "web" },
  { label: "微信", key: "wechat" },
  { label: "QQ", key: "qq" },
  { label: "隐私", key: "privacy" },
]

export function useOnboardingWizard() {
  const router = useRouter()
  const { get, post } = useApi()

  const current = ref(0)
  const stepError = ref("")
  const detectingModels = ref(false)
  const detectedModels = ref<{ id: string; owned_by?: string }[]>([])
  const detectError = ref("")
  const hasAdmin = ref(false)

  const form = reactive({
    deployMode: "desktop",
    username: "",
    password: "",
    password2: "",
    apiType: "openai-compatible",
    baseUrl: "https://api.deepseek.com/v1",
    apiKey: "",
    modelName: "",
    charName: "小暖",
    charIdentity: "AI 虚拟陪伴角色",
    charPersonality: "温和、体贴、有耐心",
    webChatEnabled: true,
    wechatEnabled: false,
    qqEnabled: false,
    qqAppId: "",
    qqToken: "",
  })

  const profileList = reactive<{ category: string; attributeName: string; attributeValue: string }[]>([])

  const wxQrLoading = ref(false)
  const wxQrCodeUrl = ref("")
  const wxQrStep = ref(0)
  const wxScanning = ref(false)
  const wxConnected = ref(false)
  const wxAccountId = ref("")
  const wxMessageCount = ref(0)
  const wxError = ref("")
  let wxPollTimer: ReturnType<typeof setInterval> | null = null

  const QQ_API = "http://127.0.0.1:8899/api/qq"
  const qqConnected = ref(false)
  const qqConnecting = ref(false)
  const qqAccountId = ref("")
  const qqMessageCount = ref(0)
  const qqError = ref("")
  let qqPollTimer: ReturnType<typeof setInterval> | null = null

  watch(current, async (val) => {
    if (val === 7 && form.wechatEnabled) { await refreshWxStatus() }
    if (val === 8 && form.qqEnabled) { await refreshQQStatus() }
  })

  watch(() => form.wechatEnabled, async (enabled) => {
    if (enabled && current.value === 7) { await refreshWxStatus() }
  })

  watch(() => form.qqEnabled, async (enabled) => {
    if (enabled && current.value === 8) { await refreshQQStatus() }
  })

  async function detectModels() {
    detectError.value = ""
    detectedModels.value = []
    detectingModels.value = true
    try {
      const res = await post<any>("/api/model/detect-models", {
        baseUrl: form.baseUrl,
        apiKey: form.apiKey,
        apiType: form.apiType,
      })
      detectedModels.value = res?.models || []
      if (detectedModels.value.length === 0) {
        detectError.value = "未检测到可用模型"
      }
    } catch (err: any) {
      detectError.value = err?.message || "检测失败，请检查 Base URL 和 API Key"
    } finally {
      detectingModels.value = false
    }
  }

  function pickModel(id: string) {
    form.modelName = id
    detectError.value = ""
    detectedModels.value = []
  }

  function stopWxPolling() {
    if (wxPollTimer) { clearInterval(wxPollTimer); wxPollTimer = null }
  }

  function stopQQPoll() {
    if (qqPollTimer) { clearInterval(qqPollTimer); qqPollTimer = null }
  }

  async function refreshWxStatus() {
    try {
      const res = await get<any>("/api/wechat/status")
      const data = res?.data || res
      if (data?.status === "connected") {
        wxConnected.value = true
        wxScanning.value = false
        wxQrStep.value = 3
        wxAccountId.value = data?.accountId || ""
        wxMessageCount.value = data?.messageCount || 0
        stopWxPolling()
      }
    } catch { }
  }

  async function startWxLogin() {
    wxError.value = ""
    wxQrLoading.value = true
    stopWxPolling()
    wxConnected.value = false
    wxQrCodeUrl.value = ""
    wxQrStep.value = 0
    wxScanning.value = false
    try {
      const res = await post<any>("/api/wechat/login/rescan")
      const imgUrl = res?.data?.qrImageUrl || res?.qrImageUrl || res?.data?.qrCodeUrl || res?.qrCodeUrl
      if (imgUrl) {
        wxQrCodeUrl.value = imgUrl
        wxQrStep.value = 1
        wxScanning.value = true
        startWxPolling()
      } else {
        wxError.value = "获取二维码失败"
      }
    } catch (err: any) {
      wxError.value = err?.message || "获取二维码失败"
    } finally {
      wxQrLoading.value = false
    }
  }

  function startWxPolling() {
    stopWxPolling()
    const startTime = Date.now()
    wxPollTimer = setInterval(async () => {
      if (Date.now() - startTime > 130000) {
        stopWxPolling()
        wxScanning.value = false
        wxQrStep.value = 0
        ElMessage.warning("扫码超时，请重新获取二维码")
        return
      }
      await refreshWxStatus()
    }, 2000)
  }

  async function resetQQConnection() {
    try { await axios.post(QQ_API + "/disconnect") } catch {}
    qqConnected.value = false
    form.qqAppId = ""
    form.qqToken = ""
    qqError.value = ""
  }

  async function connectQQ() {
    qqError.value = ""
    qqConnecting.value = true
    try {
      if (form.qqAppId && form.qqToken) {
        await axios.post(QQ_API + "/connect", {
          appId: form.qqAppId,
          token: form.qqToken,
          sandbox: false,
        })
      } else {
        await axios.post(QQ_API + "/connect", {})
      }
      stopQQPoll()
      const startTime = Date.now()
      qqPollTimer = setInterval(async () => {
        await refreshQQStatus()
        if (qqConnected.value) {
          stopQQPoll()
          qqConnecting.value = false
          return
        }
        if (Date.now() - startTime > 30000) {
          stopQQPoll()
          qqConnecting.value = false
          qqError.value = "连接超时，请检查AppID和Token是否有效"
        }
      }, 2000)
    } catch (e: any) {
      qqError.value = e?.response?.data?.error || "连接失败，请检查AppID和Token"
      qqConnecting.value = false
    }
  }

  async function refreshQQStatus() {
    try {
      const res = await axios.get(QQ_API + "/status")
      const data = res.data?.data || res.data
      qqConnected.value = !!data?.qqOnline
      qqAccountId.value = data?.accountId || ""
      qqMessageCount.value = data?.messageCount || 0
    } catch { }
  }

  async function handleNext() {
    stepError.value = ""
    if (current.value === 2) {
      if (!form.username || !form.password) {
        stepError.value = "请填写用户名和密码"
        return
      }
      if (!hasAdmin.value && form.password !== form.password2) {
        stepError.value = "两次密码不一致"
        return
      }
      if (form.password.length < 6) {
        stepError.value = "密码至少 6 位"
        return
      }
      try {
        if (!hasAdmin.value) {
          try {
            await post("/api/auth/setup", { username: form.username, password: form.password })
          } catch (setupErr: any) {
            if (setupErr?.response?.status === 409 || setupErr?.response?.data?.code === 20006) {
            } else {
              throw setupErr
            }
          }
        }
        const loginRes = await post<any>("/api/auth/login", { username: form.username, password: form.password })
        if (loginRes?.token) {
          setToken(loginRes.token)
        }
        current.value++
        hasAdmin.value = true
      } catch (e: any) {
        stepError.value = e?.response?.data?.message || e?.message || (hasAdmin.value ? "登录失败，请检查密码" : "创建账号失败，请重试")
      }
      return
    }
    if (current.value === 4) {
      if (!form.charName) {
        stepError.value = "请输入角色名称"
        return
      }
    }
    if (current.value === 3) {
      if (!form.modelName.trim()) {
        stepError.value = "请输入模型名称"
        return
      }
    }
    current.value++
  }

  async function handleFinish() {
    stepError.value = ""
    try {
      if (form.apiKey && form.baseUrl && form.modelName) {
        await post("/api/model/configs", {
          apiType: form.apiType,
          baseUrl: form.baseUrl,
          apiKey: form.apiKey,
          modelName: form.modelName,
          isActive: 1,
        })
      }
      if (form.charName) {
        await post("/api/characters", {
          name: form.charName,
          identity: form.charIdentity,
          personality: form.charPersonality,
          isActive: 1,
          isDefault: true,
        })
      }
      const validProfiles = profileList.filter(p => p.attributeName && p.attributeValue)
      for (const p of validProfiles) {
        await post("/api/profiles", {
          category: p.category,
          attributeName: p.attributeName,
          attributeValue: p.attributeValue,
        }).catch(() => {})
      }
      await post("/api/onboarding/complete", {
        deployMode: form.deployMode === "cloud" ? "cloud-web" : "desktop-local",
        webChatEnabled: form.webChatEnabled,
        wechatEnabled: form.wechatEnabled,
        qqEnabled: form.qqEnabled,
        modelConfig: form.apiKey ? { name: "default", apiType: form.apiType, baseUrl: form.baseUrl, apiKey: form.apiKey, modelName: form.modelName } : undefined,
        username: form.username,
        password: form.password || undefined,
      })
      ElMessage.success("设置完成！即将跳转到聊天页面")
      setTimeout(() => router.push("/chat"), 1500)
    } catch (err: any) {
      stepError.value = err?.message || err?.response?.data?.message || "设置过程中出现错误，请重试"
    }
  }

  function cleanup() {
    stopWxPolling()
    stopQQPoll()
  }

  return {
    current,
    stepError,
    detectingModels,
    detectedModels,
    detectError,
    hasAdmin,
    form,
    profileList,
    wxQrLoading,
    wxQrCodeUrl,
    wxQrStep,
    wxScanning,
    wxConnected,
    wxAccountId,
    wxMessageCount,
    wxError,
    qqConnected,
    qqConnecting,
    qqAccountId,
    qqMessageCount,
    qqError,
    detectModels,
    pickModel,
    startWxLogin,
    refreshWxStatus,
    startWxPolling,
    stopWxPolling,
    stopQQPoll,
    resetQQConnection,
    connectQQ,
    refreshQQStatus,
    handleNext,
    handleFinish,
    cleanup,
  }
}
