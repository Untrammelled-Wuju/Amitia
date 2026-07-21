import { ref, reactive, computed } from "vue"
import { useRouter } from "vue-router"
import { ElMessage } from "element-plus"
import { useApi, setToken } from "../../../composables/useApi"
import { getApiBaseURL } from "@/runtime/runtime-adapter"

export function useImmersiveOnboarding() {
  const router = useRouter()
  const { get, post } = useApi()

  const currentStage = ref(0)
  const maxStage = ref(0)
  const stageError = ref("")

  const deployMode = ref("local")
  const serverURL = ref("")
  const remoteChecked = ref(false)

  const adminStep = ref("environment")
  const isAdminLogin = ref(false)
  const hasAdmin = ref(false)
  const accountName = ref("")
  const accountPassword = ref("")
  const accountDone = ref(false)

  const detectingModels = ref(false)
  const modelReady = ref(false)
  const modelDetected = ref(false)
  const modelStatusText = ref("等待检测")
  const detectedModels = ref<Array<{id: string, ownedBy?: string}>>([])

  const modelBaseUrl = ref("https://api.deepseek.com/v1")
  const modelApiKey = ref("")
  const modelName = ref("")

  const modelFieldErrors = ref<{baseUrl?: boolean, apiKey?: boolean, modelName?: boolean}>({})

  const visionMode = ref("dedicated")
  const detectingVision = ref(false)
  const visionReady = ref(false)
  const visionDetected = ref(false)
  const visionStatusText = ref("请选择一种视觉模式")
  const visionModelKey = ref("")
  const visionModelName = ref("doubao-seed-2-0-lite-260428")
  const visionModelURL = ref("https://ark.cn-beijing.volces.com/api/v3")

  const voiceStyle = ref("温和")
  const voiceModelMode = ref("volcengine")
  const detectingVoice = ref(false)
  const voiceReady = ref(false)
  const voiceDetected = ref(false)
  const voiceStatusText = ref("等待测试")
  const voiceModelKey = ref("")
  const voiceModelURL = ref("https://openspeech.bytedance.com/api/v1")
  const voiceModelResource = ref("seed-tts-2.0")
  const voiceModelVoiceType = ref("zh_female_vv_jupiter_bigtts")

  const detectingVector = ref(false)
  const vectorReady = ref(false)
  const vectorDetected = ref(false)
  const vectorStatusText = ref("等待检测")
  const vectorModelMode = ref("volcengine")
  const vectorModelKey = ref("")
  const vectorModelName = ref("doubao-embedding-vision-251215")
  const vectorModelURL = ref("https://ark.cn-beijing.volces.com/api/v3")

  const identityStep = ref(0)
  const identityState = ref<'filling' | 'exiting' | 'spotlight' | 'complete'>('filling')
  const identityName = ref("")
  const identityRole = ref("")
  const identityPersonality = ref("")

  const memoryStep = ref(0)
  const memoryComplete = ref(false)
  const memoryItems = ref(["", "", ""])

  const permissions = reactive({
    autostart: false,
    web: true,
    wechat: false,
    qq: false,
  })

  const entering = ref(false)
  const entryPreparing = ref(false)
  const enteringState = ref<string | null>(null)
  const onboardingComplete = ref(false)

  const stageCaptions = [
    "等待开始",
    "选择运行方式",
    "完成运行准备",
    "连接语言模型",
    "设置图片理解",
    "配置语音输出",
    "配置记忆检索",
    "设定角色",
    "记录初始信息",
    "设置权限与渠道",
  ]

  const identityQuestions = [
    {
      question: "先为我设定一个名字。",
      context: "之后的对话会使用这个名字。",
      placeholder: "例如：Amitia",
      maxLength: 32,
      quickChoices: ["Amitia", "墨染", "晨曦", "月白"],
    },
    {
      question: "我的身份是什么？",
      context: "我是虚拟角色、老友、工作伙伴，还是其他存在？",
      placeholder: "例如：AI 陪伴角色",
      maxLength: 20,
      quickChoices: ["AI 陪伴角色", "虚拟朋友", "工作伙伴", "学习助手"],
    },
    {
      question: "我的性格是怎样的？",
      context: "温和、克制、开朗，还是别的样子？",
      placeholder: "例如：温和、体贴、有耐心",
      maxLength: 30,
      quickChoices: ["温和、克制", "开朗、热情", "冷静、理性", "幽默、随性"],
    },
  ]

  const memoryQuestions = [
    {
      question: "我应该怎么称呼你？",
      context: "告诉我你希望使用的称呼。",
      placeholder: "输入你希望我使用的称呼",
      quickChoices: ["朋友", "主人", "哥哥", "亲爱的"],
    },
    {
      question: "你希望我用什么风格交流？",
      context: "随意、正式，或者其他偏好？",
      placeholder: "描述你偏好的交流风格",
      quickChoices: ["自然随意", "正式礼貌", "简洁直接", "温暖细致"],
    },
    {
      question: "有什么需要我一开始就记住的吗？",
      context: "我会把这条信息放在记忆的起点。",
      placeholder: "任何你觉得重要的事情",
      quickChoices: [],
    },
  ]

  const currentIdentityQuestion = computed(() => identityQuestions[identityStep.value])
  const currentMemoryQuestion = computed(() => memoryQuestions[memoryStep.value])
  const currentCaption = computed(() => stageCaptions[currentStage.value] || "")

  const stageCount = 10

  let stageTransitionToken = 0
  const stageTransitioning = ref(false)
  const coreRevealPending = ref(false)
  const leavingStage = ref(-1)
  const enterPrepStage = ref(-1)

  let pendingNextStageTimer: ReturnType<typeof setTimeout> | null = null

  function goToStage(stage: number) {
    if (stage < 0 || stage >= stageCount) return
    if (stageTransitioning.value && stage !== leavingStage.value) return

    maxStage.value = Math.max(maxStage.value, stage)

    if (stage === 2) {
      if (accountDone.value) {
        hasAdmin.value = true
        adminStep.value = "account"
      } else {
        adminStep.value = "environment"
      }
    }

    const prev = currentStage.value
    if (stage === prev) return

    stageTransitioning.value = true
    const token = ++stageTransitionToken

    leavingStage.value = prev
    coreRevealPending.value = false
    if (pendingNextStageTimer && prev === 7) { clearTimeout(pendingNextStageTimer); pendingNextStageTimer = null }

    setTimeout(() => {
      if (token !== stageTransitionToken) return

      leavingStage.value = -1
      coreRevealPending.value = true
      currentStage.value = stage
      enterPrepStage.value = stage

      if (stage === 7 && identityState.value !== "filling" && identityState.value !== "complete" && identityState.value !== "spotlight") { identityState.value = "filling"; identityStep.value = 0 }

      setTimeout(() => {
        if (token !== stageTransitionToken) return
        enterPrepStage.value = -1
        coreRevealPending.value = false

        setTimeout(() => {
          if (token !== stageTransitionToken) return
          stageTransitioning.value = false
        }, 380)
      }, 700)
    }, 380)
  }

  function nextStage() {
    if (currentStage.value < stageCount - 1) {
      if (currentStage.value === 7 && identityState.value === 'complete') {
        identityState.value = 'spotlight'
        if (pendingNextStageTimer) clearTimeout(pendingNextStageTimer)
        pendingNextStageTimer = setTimeout(() => {
          pendingNextStageTimer = null
          goToStage(currentStage.value + 1)
        }, 1800)
        return
      }
      if (currentStage.value === 3) {
        modelFieldErrors.value = {}
        if (!modelBaseUrl.value.trim()) modelFieldErrors.value.baseUrl = true
        if (!modelApiKey.value.trim()) modelFieldErrors.value.apiKey = true
        if (!modelName.value.trim()) modelFieldErrors.value.modelName = true
        if (Object.keys(modelFieldErrors.value).length > 0) return
      }
      goToStage(currentStage.value + 1)
    }
  }

  function prevStage() {
    if (currentStage.value === 2 && adminStep.value !== "environment") {
      adminStep.value = "environment"
      return
    }

    if (currentStage.value === 7) {
      if (identityState.value !== 'filling') {
        if (pendingNextStageTimer) { clearTimeout(pendingNextStageTimer); pendingNextStageTimer = null }
        startIdentityReverse()
        return
      }
      if (identityStep.value > 0) {
        identityStep.value--
        return
      }
    }

    if (currentStage.value === 8) {
      if (memoryComplete.value) {
        memoryComplete.value = false
        memoryStep.value = 2
        return
      }
      if (memoryStep.value > 0) {
        memoryStep.value--
        return
      }
      memoryStep.value = 0
      identityState.value = "spotlight"
      goToStage(7)
      setTimeout(() => {
        identityState.value = "complete"
      }, 2600)
      return
    }

    if (currentStage.value > 0) {
      goToStage(currentStage.value - 1)
    }
  }


  function isLoginFlow(): boolean {
    return deployMode.value === "remote" || hasAdmin.value
  }

  async function checkAdminExists() {
    try {
      const apiBase = await getApiBaseURL()
      const res = await fetch(`${apiBase}/api/auth/status`)
      if (!res.ok) return
      const json = await res.json()
      hasAdmin.value = !!(json?.data?.hasAdmin || json?.hasAdmin)
    } catch {}
  }

  async function handleAdminSubmit(data: { username: string; password: string; password2: string; isLogin: boolean; deployMode: string }) {
    stageError.value = ""

    try {
      accountName.value = data.username
      accountPassword.value = data.password

      if (!data.isLogin) {
        try {
          await post("/api/auth/setup", { username: data.username, password: data.password })
        } catch (setupErr: any) {
          if (setupErr?.code === 600 || setupErr?.response?.status === 409) {
            hasAdmin.value = true
          } else {
            throw setupErr
          }
        }
      }

      const loginRes = await post<any>("/api/auth/login", { username: data.username, password: data.password })
      if (loginRes?.token) {
        setToken(loginRes.token)
      }

      accountDone.value = true
      accountName.value = data.username
      nextStage()
    } catch (e: any) {
      stageError.value = e?.response?.data?.message || e?.message || (hasAdmin.value ? "登录失败，请检查密码" : "创建账号失败，请重试")
    }
  }

  async function detectModel() {
    detectingModels.value = true
    modelStatusText.value = "正在检测模型连接"

    try {
      await new Promise((resolve) => setTimeout(resolve, 600))

      const res = await post<any>("/api/model/detect-models", {
        baseUrl: modelBaseUrl.value,
        apiKey: modelApiKey.value,
        apiType: "openai-compatible",
      })

      if (res?.models && res.models.length > 0) {
        modelReady.value = true
        modelDetected.value = true
        modelStatusText.value = `已检测到 ${res.models.length} 个模型`
        detectedModels.value = res.models
      } else {
        modelReady.value = false
        modelDetected.value = false
        modelStatusText.value = "未检测到可用模型"
        detectedModels.value = []
      }
    } catch {
      modelReady.value = false
      modelDetected.value = false
      modelStatusText.value = "检测失败，请检查 Base URL 和 API Key"
      detectedModels.value = []
    } finally {
      detectingModels.value = false
    }
  }

  async function detectVision() {
    detectingVision.value = true
    visionStatusText.value = "正在检测视觉模型连接"

    try {
      await new Promise((resolve) => setTimeout(resolve, 600))

      const res = await post<any>("/api/model/detect-models", {
        baseUrl: visionModelURL.value,
        apiKey: visionModelKey.value,
        apiType: "openai-compatible",
      })

      if (res?.models && res.models.length > 0) {
        visionReady.value = true
        visionDetected.value = true
        visionStatusText.value = "视觉模型连接成功，可以继续"
      } else {
        visionReady.value = false
        visionDetected.value = false
        visionStatusText.value = "未检测到可用视觉模型"
      }
    } catch {
      visionReady.value = false
      visionDetected.value = false
      visionStatusText.value = "检测失败，请检查接口地址和 API Key"
    } finally {
      detectingVision.value = false
    }
  }

  async function detectVoice() {
    detectingVoice.value = true
    voiceStatusText.value = "正在测试语音服务连接"

    try {
      await new Promise((resolve) => setTimeout(resolve, 600))

      await post("/api/tts/test-connection", {
        apiKey: voiceModelKey.value,
        baseUrl: voiceModelURL.value,
        resource: voiceModelResource.value,
        voiceType: voiceModelVoiceType.value,
      })

      voiceReady.value = true
      voiceDetected.value = true
      voiceStatusText.value = "语音服务连接成功，可以继续"
    } catch {
      voiceReady.value = false
      voiceDetected.value = false
      voiceStatusText.value = "语音服务测试失败，请检查配置"
    } finally {
      detectingVoice.value = false
    }
  }

  async function detectVector() {
    detectingVector.value = true
    vectorStatusText.value = "正在检测向量模型连接"

    try {
      await new Promise((resolve) => setTimeout(resolve, 600))

      const res = await post<any>("/api/model/detect-models", {
        baseUrl: vectorModelURL.value,
        apiKey: vectorModelKey.value,
        apiType: "openai-compatible",
      })

      if (res?.models && res.models.length > 0) {
        vectorReady.value = true
        vectorDetected.value = true
        vectorStatusText.value = "向量模型连接成功，可以继续"
      } else {
        vectorReady.value = false
        vectorDetected.value = false
        vectorStatusText.value = "未检测到可用向量模型"
      }
    } catch {
      vectorReady.value = false
      vectorDetected.value = false
      vectorStatusText.value = "检测失败，请检查接口地址和 API Key"
    } finally {
      detectingVector.value = false
    }
  }

  function handleIdentityAnswer(value: string) {
    if (identityStep.value === 0) {
      identityName.value = value
    } else if (identityStep.value === 1) {
      identityRole.value = value
    } else if (identityStep.value === 2) {
      identityPersonality.value = value
      startIdentityTransition()
      return
    }
    identityStep.value++
  }

  function startIdentityTransition() {
    identityState.value = 'exiting'
    setTimeout(() => {
      identityState.value = 'spotlight'
      setTimeout(() => {
        identityState.value = 'complete'
      }, 1800)
    }, 800)
  }

  function startIdentityReverse() {
    identityStep.value = 2
    identityState.value = "spotlight"
    setTimeout(() => {
      identityState.value = 'exiting'
      setTimeout(() => {
        identityState.value = 'filling'
      }, 1400)
    }, 1800)
  }

  function handleMemoryAnswer(value: string) {
    memoryItems.value[memoryStep.value] = value
    if (memoryStep.value >= 2) {
      memoryComplete.value = true
      return
    }
    memoryStep.value++
  }

  async function startEntryTransition() {
    if (entering.value || entryPreparing.value) return

    entryPreparing.value = true
    import("@/views/web-chat/WebChatView.vue")
    await new Promise((resolve) => setTimeout(resolve, 1780))
    entryPreparing.value = false
    enteringState.value = "true"

    setTimeout(() => {
      enteringState.value = "complete"
    }, 2200)

    handleEnterAmitia()
  }

  async function handleEnterAmitia() {
    if (entering.value) return
    entering.value = true

    try {
      if (modelApiKey.value && modelBaseUrl.value && modelName.value) {
        await post("/api/model/configs", {
          apiType: "openai-compatible",
          baseUrl: modelBaseUrl.value,
          apiKey: modelApiKey.value,
          modelName: modelName.value,
          isActive: 1,
        })
      }

      if (visionMode.value !== "disabled" && visionModelKey.value && visionModelURL.value) {
        await post("/api/model/configs", {
          apiType: "vision",
          baseUrl: visionModelURL.value,
          apiKey: visionModelKey.value,
          modelName: visionModelName.value,
          visionMode: visionMode.value,
          isActive: 1,
        }).catch(() => {})
      }

      if (voiceModelMode.value !== "disabled" && voiceModelKey.value) {
        await post("/api/model/configs", {
          apiType: "voice",
          provider: voiceModelMode.value,
          apiKey: voiceModelKey.value,
          baseUrl: voiceModelURL.value,
          resource: voiceModelResource.value,
          voiceType: voiceModelVoiceType.value,
          voiceStyle: voiceStyle.value,
          isActive: 1,
        }).catch(() => {})
      }

      if (vectorModelMode.value !== 'disabled' && vectorModelKey.value && vectorModelURL.value) {
        await post("/api/model/configs", {
          apiType: "vector",
          baseUrl: vectorModelURL.value,
          apiKey: vectorModelKey.value,
          modelName: vectorModelName.value,
          isActive: 1,
        }).catch(() => {})
      }

      if (identityName.value) {
        await post("/api/characters", {
          name: identityName.value,
          identity: identityRole.value || "AI 陪伴角色",
          personality: identityPersonality.value || "温和、体贴",
          isActive: 1,
          isDefault: true,
        }).then((charRes: any) => {
          const charId = charRes?.id || charRes?.data?.id
          if (charId) {
            localStorage.setItem("webchat-char-id", charId)
          }
        })
      }

      if (memoryItems.value.some((item) => item)) {
        let userId = ""
        try {
          const me = await get<any>("/api/auth/me")
          userId = me?.id ? String(me.id) : ""
        } catch {}
        for (const item of memoryItems.value.filter(Boolean)) {
          await post("/api/profiles", {
            category: "memory",
            attributeName: "initial_memory",
            attributeValue: item,
            userId: userId,
          }).catch(() => {})
        }
      }

      await post("/api/onboarding/complete", {
        deployMode: deployMode.value === "remote" ? "cloud-web" : "desktop-local",
        serverURL: deployMode.value === "remote" ? serverURL.value.trim().replace(/\/+$/, "") : undefined,
        webChatEnabled: true,
        wechatEnabled: permissions.wechat,
        qqEnabled: permissions.qq,
        modelConfig: modelApiKey.value
          ? { name: "default", apiType: "openai-compatible", baseUrl: modelBaseUrl.value, apiKey: modelApiKey.value, modelName: modelName.value }
          : undefined,
        username: accountName.value,
        password: accountPassword.value || undefined,
      })

      localStorage.removeItem("webchat-last-conv")
      const nextCacheVersion = Date.now()
      localStorage.setItem("char_cache_version", String(nextCacheVersion))

      setTimeout(() => {
        router.push("/chat")
      }, 2700)
    } catch (err: any) {
      stageError.value = err?.message || err?.response?.data?.message || "设置过程中出现错误，请重试"
      entering.value = false
    }
  }

  function playVoiceSample(name: string) {
    voiceStyle.value = name
  }

  return {
    currentStage,
    maxStage,
    stageError,
    deployMode,
    serverURL,
    remoteChecked,
    adminStep,
    isAdminLogin,
    hasAdmin,
    accountName,
    accountDone,
    detectingModels,
    modelReady,
    modelDetected,
    detectedModels,
    modelStatusText,
    modelFieldErrors,
    modelBaseUrl,
    modelApiKey,
    modelName,
    visionMode,
    detectingVision,
    visionReady,
    visionDetected,
    visionStatusText,
    visionModelKey,
    visionModelName,
    visionModelURL,
    voiceStyle,
    voiceModelMode,
    detectingVoice,
    voiceReady,
    voiceDetected,
    voiceStatusText,
    voiceModelKey,
    voiceModelURL,
    voiceModelResource,
    voiceModelVoiceType,
    detectingVector,
    vectorReady,
    vectorDetected,
    vectorStatusText,
    vectorModelMode,
    vectorModelKey,
    vectorModelName,
    vectorModelURL,
    identityStep,
    identityState,
    identityName,
    identityRole,
    identityPersonality,
    identityQuestions,
    memoryStep,
    memoryComplete,
    memoryItems,
    memoryQuestions,
    permissions,
    entering,
    entryPreparing,
    enteringState,
    onboardingComplete,
    currentCaption,
    stageCount,
    currentIdentityQuestion,
    currentMemoryQuestion,
    goToStage,
    nextStage,
    prevStage,
    checkAdminExists,
    isLoginFlow,
    handleAdminSubmit,
    detectModel,
    detectVision,
    detectVoice,
    vectorModelMode,
    detectVector,
    handleIdentityAnswer,
    handleMemoryAnswer,
    handleEnterAmitia,
    startEntryTransition,
    playVoiceSample,
    stageTransitioning,
    coreRevealPending,
    leavingStage,
    enterPrepStage,
  }
}
