// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, reactive, computed } from "vue"
import { useRouter } from "vue-router"
import { apiClient, setToken } from "../../../ui-index"

export interface WizardStep {
  key: string
  label: string
  title: string
  description: string
  buttonLabel?: string
  done: boolean
  requiresAction: boolean
}

export const DEFAULT_STEPS: WizardStep[] = [
  { key: "detect-mode", label: "Mode", title: "Select Deployment Mode", description: "Choose how you want to run your AI-Amitia companion.", buttonLabel: "Confirm Mode", done: false, requiresAction: true },
  { key: "core-check", label: "Core", title: "Core Service Check", description: "Verifying the core service is running.", done: false, requiresAction: false },
  { key: "web-check", label: "Web", title: "Web Interface Check", description: "Verifying the web interface is accessible.", done: false, requiresAction: false },
  { key: "bridge-check", label: "Bridge", title: "WeChat Bridge Check", description: "Checking WeChat Bridge availability.", done: false, requiresAction: false },
  { key: "db-check", label: "Database", title: "Database Check", description: "Verifying the database is writable.", done: false, requiresAction: false },
  { key: "admin-password", label: "Password", title: "Set Admin Password", description: "Create an admin account to protect your data.", buttonLabel: "Save Password", done: false, requiresAction: true },
  { key: "model-config", label: "Model", title: "Configure AI Model", description: "Connect your AI model API for chat functionality.", buttonLabel: "Save & Test", done: false, requiresAction: true },
  { key: "model-test", label: "Test", title: "Test Model Connection", description: "Verify the model API is reachable and working.", buttonLabel: "Test Connection", done: false, requiresAction: true },
  { key: "character-select", label: "Character", title: "Choose Default Character", description: "Select the personality for your AI-Amitia companion.", buttonLabel: "Confirm Character", done: false, requiresAction: false },
  { key: "wechat-option", label: "WeChat", title: "WeChat Access", description: "Decide if you want to use the companion via WeChat.", buttonLabel: "Confirm", done: false, requiresAction: true },
  { key: "cloud-deploy-info", label: "Deploy", title: "Deployment Overview", description: "Review how your system will run.", buttonLabel: "Continue", done: false, requiresAction: false },
  { key: "privacy-boundary", label: "Privacy", title: "Privacy & Security", description: "Understand your privacy and security boundaries.", buttonLabel: "Confirm", done: false, requiresAction: true },
  { key: "finish", label: "Done", title: "Setup Complete", description: "You are all set!", buttonLabel: "Enter Dashboard", done: false, requiresAction: true },
]

export const STEP_KEYS = DEFAULT_STEPS.map(s => s.key)

export function useSetupWizard() {
  const router = useRouter()

  const wizardSteps = ref<WizardStep[]>(DEFAULT_STEPS.map(s => ({ ...s })))
  const currentStepIdx = ref(0)
  const completedSteps = ref(0)
  const totalSteps = STEP_KEYS.length

  const loading = ref(true)
  const submitting = ref(false)
  const errorMsg = ref("")
  const errorSuggestion = ref("")
  const testResult = ref<{ passed: boolean; message: string; detail?: string } | null>(null)
  const skipAuth = ref(false)
  const deployMode = ref<"desktop-local" | "cloud-web">("desktop-local")
  const initializing = ref(true)
  const characters = ref<{ id: string; name: string; identity: string }[]>([])

  const localData = reactive({
    deployMode: "desktop-local" as string,
    username: "admin",
    password: "",
    confirmPassword: "",
    apiType: "openai-compatible" as string,
    baseUrl: "",
    apiKey: "",
    modelName: "gpt-4o-mini",
    characterId: "" as string,
    enableWechat: false as boolean | null,
    privacyConfirmed: false,
  })

  const currentStep = computed(() => wizardSteps.value[currentStepIdx.value])
  const progressPercent = computed(() => Math.round((completedSteps.value / totalSteps) * 100))
  const canProceed = computed(() => {
    if (!currentStep.value || currentStep.value.key === "finish") return false
    if (!currentStep.value.requiresAction) return true
    switch (currentStep.value.key) {
      case "detect-mode": return !!localData.deployMode
      case "admin-password": return skipAuth.value || (localData.username.trim() && localData.password.length >= 6 && localData.password === localData.confirmPassword)
      case "model-config": return localData.baseUrl.trim() && localData.apiKey.trim() && localData.modelName.trim()
      case "model-test": return true
      case "character-select": return true
      case "wechat-option": return localData.enableWechat !== null
      case "privacy-boundary": return localData.privacyConfirmed
      default: return true
    }
  })

  async function fetchStatus(): Promise<any> {
    try {
      const res = await apiClient.get("/api/setup/status")
      return res.data?.data || res.data || {}
    } catch { return {} }
  }

  async function getChecks(): Promise<any> {
    try {
      const res = await apiClient.get("/api/setup/checks")
      return res.data?.data || res.data || {}
    } catch { return {} }
  }

  async function submitStep(step: string, data: any): Promise<any> {
    const res = await apiClient.post("/api/setup/step", { step, ...data })
    return res.data?.data || res.data || {}
  }

  async function submitFinish(): Promise<any> {
    const res = await apiClient.post("/api/setup/finish")
    return res.data?.data || res.data
  }

  async function fetchCharacters(): Promise<any[]> {
    try {
      const res = await apiClient.get("/api/characters")
      const data = res.data?.data || res.data
      return data?.characters || data || []
    } catch { return [] }
  }

  function markStepDone(key: string) {
    const s = wizardSteps.value.find(s => s.key === key)
    if (s) s.done = true
    completedSteps.value = wizardSteps.value.filter(s => s.done).length
  }

  async function runAutoStep(step: string): Promise<boolean> {
    try {
      const result = await submitStep(step, {})
      if (result.done) {
        markStepDone(step)
        return true
      }
      return false
    } catch (err: any) {
      const msg = err?.response?.data?.message || err?.message || "Step failed"
      const detail = err?.response?.data?.detail || err?.response?.data?.suggestion || ""
      errorMsg.value = msg
      errorSuggestion.value = detail
      return false
    }
  }

  function goBack() {
    if (currentStepIdx.value > 0) {
      currentStepIdx.value--
      errorMsg.value = ""
      errorSuggestion.value = ""
    }
  }

  function skipStep() {
    markStepDone(currentStep.value.key)
    currentStepIdx.value++
    errorMsg.value = ""
    errorSuggestion.value = ""
  }

  function retryStep() {
    errorMsg.value = ""
    errorSuggestion.value = ""
  }

  async function handleNext() {
    if (submitting.value) return
    const step = currentStep.value
    if (!step) return

    if (step.requiresAction) {
      submitting.value = true
      errorMsg.value = ""
      errorSuggestion.value = ""

      try {
        let stepData: any = {}
        switch (step.key) {
          case "detect-mode":
            stepData = { deployMode: localData.deployMode }
            break
          case "admin-password":
            if (skipAuth.value && deployMode.value === "desktop-local") {
              stepData = { skip: true }
            } else {
              stepData = { username: localData.username, password: localData.password, confirmPassword: localData.confirmPassword }
            }
            break
          case "model-config":
            stepData = { apiType: localData.apiType, baseUrl: localData.baseUrl, apiKey: localData.apiKey, modelName: localData.modelName }
            break
          case "model-test":
            stepData = {}
            break
          case "character-select":
            stepData = { characterId: localData.characterId || undefined }
            break
          case "wechat-option":
            stepData = { enableWechat: localData.enableWechat }
            break
          case "privacy-boundary":
            stepData = { confirmed: localData.privacyConfirmed }
            break
        }

        const result = await submitStep(step.key, stepData)
        if (result.done === false) {
          errorMsg.value = result.message || "Step failed"
          errorSuggestion.value = result.suggestion || ""
          submitting.value = false
          return
        }

        if (step.key === "model-test") {
          testResult.value = {
            passed: result.done !== false,
            message: result.message || "Test completed",
            detail: result.testDetail ? JSON.stringify(result.testDetail) : undefined,
          }
        }

        markStepDone(step.key)
        submitting.value = false
        currentStepIdx.value++
        errorMsg.value = ""
        errorSuggestion.value = ""
      } catch (err: any) {
        const msg = err?.response?.data?.message || err?.message || "An error occurred"
        const suggestion = err?.response?.data?.suggestion || err?.response?.data?.detail || ""
        errorMsg.value = msg
        errorSuggestion.value = suggestion
        submitting.value = false
        return
      }
    } else {
      markStepDone(step.key)
      currentStepIdx.value++
      errorMsg.value = ""
      errorSuggestion.value = ""
    }
  }

  async function handleFinish() {
    submitting.value = true
    try {
      const result = await submitFinish()
      if (result.token) {
        setToken(result.token)
      }
      markStepDone("finish")
      router.push("/dashboard")
    } catch (err: any) {
      const msg = err?.response?.data?.message || err?.message || "Failed to complete setup"
      errorMsg.value = msg
      submitting.value = false
    }
  }

  async function runCurrentAutoStep() {
    const step = wizardSteps.value[currentStepIdx.value]
    if (step && !step.requiresAction) {
      if (["core-check", "web-check", "bridge-check", "db-check", "cloud-deploy-info"].includes(step.key)) {
        loading.value = true
        const success = await runAutoStep(step.key)
        loading.value = false
        if (success) {
          currentStepIdx.value++
          runCurrentAutoStep()
        }
      }
    }
  }

  async function init() {
    try {
      const status = await fetchStatus()
      if (status.completed) {
        router.replace("/dashboard")
        return false
      }
      if (status.deployMode) {
        deployMode.value = status.deployMode
        localData.deployMode = status.deployMode
      }
      if (status.steps) {
        for (const s of status.steps) {
          const ws = wizardSteps.value.find(w => w.key === s.step)
          if (ws && s.done) ws.done = true
        }
        completedSteps.value = wizardSteps.value.filter(s => s.done).length
      }
      const curStep = status.currentStep || "detect-mode"
      const idx = STEP_KEYS.indexOf(curStep)
      currentStepIdx.value = idx >= 0 ? idx : 0

      fetchCharacters().then(chars => { characters.value = chars })

      initializing.value = false
      loading.value = false

      if (!currentStep.value?.requiresAction) {
        runCurrentAutoStep()
      }
      return true
    } catch {
      loading.value = false
      initializing.value = false
      return true
    }
  }

  return {
    wizardSteps,
    currentStepIdx,
    completedSteps,
    totalSteps,
    loading,
    submitting,
    errorMsg,
    errorSuggestion,
    testResult,
    skipAuth,
    deployMode,
    initializing,
    characters,
    localData,
    currentStep,
    progressPercent,
    canProceed,
    markStepDone,
    goBack,
    skipStep,
    retryStep,
    handleNext,
    handleFinish,
    init,
    runCurrentAutoStep,
    fetchCharacters,
  }
}
