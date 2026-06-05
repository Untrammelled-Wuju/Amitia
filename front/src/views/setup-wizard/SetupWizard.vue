<template>
  <div class="setup-wizard">
    <div class="sw-container">
      <header class="sw-header">
        <h1 class="sw-title">U-Ai System Setup</h1>
        <p class="sw-subtitle">Complete the following steps to configure your AI companion.</p>
      </header>

      <!-- Step indicator -->
      <div class="sw-steps">
        <div
          v-for="(s, i) in wizardSteps"
          :key="s.key"
          class="sw-step-dot"
          :class="{ active: i === currentStepIdx, done: s.done, current: i === currentStepIdx }"
        >
          <div class="sw-dot">
            <span v-if="s.done">&#10003;</span>
            <span v-else>{{ i + 1 }}</span>
          </div>
          <span class="sw-dot-label">{{ s.label }}</span>
        </div>
      </div>

      <!-- Step content -->
      <div class="sw-content" :key="currentStepIdx">
        <template v-if="loading">
          <div class="sw-loading">
            <p>Checking...</p>
          </div>
        </template>

        <template v-else-if="errorMsg">
          <div class="sw-error">
            <p class="sw-error-title">Error</p>
            <p class="sw-error-msg">{{ errorMsg }}</p>
            <p v-if="errorSuggestion" class="sw-error-suggestion">{{ errorSuggestion }}</p>
            <div class="sw-error-actions">
              <button class="sw-btn sw-btn-secondary" @click="retryStep">Retry</button>
              <button v-if="currentStep.key !== 'finish'" class="sw-btn sw-btn-ghost" @click="skipStep">Skip</button>
            </div>
          </div>
        </template>

        <template v-else>
          <div class="sw-step-body">
            <h2 class="sw-step-title">{{ currentStep.title }}</h2>
            <p class="sw-step-desc">{{ currentStep.description }}</p>

            <!-- Step-specific content -->
            <div class="sw-step-form">
              <!-- detect-mode -->
              <div v-if="currentStep.key === 'detect-mode'" class="sw-options">
                <label
                  class="sw-option-card"
                  :class="{ selected: localData.deployMode === 'desktop-local' }"
                >
                  <input v-model="localData.deployMode" type="radio" value="desktop-local" />
                  <div class="sw-option-body">
                    <strong>Desktop Local</strong>
                    <p>Run everything on this computer. Data stays local. Only accessible from this machine.</p>
                  </div>
                </label>
                <label
                  class="sw-option-card"
                  :class="{ selected: localData.deployMode === 'cloud-web' }"
                >
                  <input v-model="localData.deployMode" type="radio" value="cloud-web" />
                  <div class="sw-option-body">
                    <strong>Cloud Web</strong>
                    <p>Deploy on a cloud server. Access from anywhere via web browser. Your computer does not need to stay on.</p>
                  </div>
                </label>
              </div>

              <!-- admin-password -->
              <div v-if="currentStep.key === 'admin-password'" class="sw-form">
                <div v-if="deployMode === 'desktop-local'" class="sw-notice">
                  <strong>Security Notice:</strong> In desktop local mode, anyone with access to this computer can access the app. Setting a password is recommended but optional.
                </div>
                <div v-if="deployMode === 'cloud-web'" class="sw-notice sw-notice-warn">
                  <strong>Required:</strong> Cloud deployment requires an admin password to protect your data from unauthorized access.
                </div>
                <div class="sw-field">
                  <label>Username</label>
                  <input v-model="localData.username" type="text" placeholder="Admin username" maxlength="32" />
                </div>
                <div class="sw-field">
                  <label>Password</label>
                  <input v-model="localData.password" type="password" placeholder="At least 6 characters" maxlength="64" />
                </div>
                <div class="sw-field">
                  <label>Confirm Password</label>
                  <input v-model="localData.confirmPassword" type="password" placeholder="Repeat password" maxlength="64" />
                </div>
                <div v-if="deployMode === 'desktop-local'" class="sw-skip-option">
                  <label>
                    <input v-model="skipAuth" type="checkbox" />
                    Skip password setup (not recommended)
                  </label>
                </div>
              </div>

              <!-- model-config -->
              <div v-if="currentStep.key === 'model-config'" class="sw-form">
                <div class="sw-field">
                  <label>API Type</label>
                  <select v-model="localData.apiType">
                    <option value="openai-compatible">OpenAI Compatible (DeepSeek, etc.)</option>
                    <option value="ollama">Ollama (Local)</option>
                    <option value="custom-http">Custom HTTP</option>
                  </select>
                </div>
                <div class="sw-field">
                  <label>API Base URL</label>
                  <input v-model="localData.baseUrl" type="text" placeholder="https://api.deepseek.com/v1" />
                </div>
                <div class="sw-field">
                  <label>API Key</label>
                  <input v-model="localData.apiKey" type="password" placeholder="sk-..." />
                </div>
                <div class="sw-field">
                  <label>Model Name</label>
                  <input v-model="localData.modelName" type="text" placeholder="deepseek-chat" />
                </div>
              </div>

              <!-- model-test -->
              <div v-if="currentStep.key === 'model-test'" class="sw-form">
                <div class="sw-notice">
                  Click "Test Connection" to verify your model API is reachable and working.
                </div>
                <div v-if="testResult" class="sw-test-result" :class="testResult.passed ? 'success' : 'failed'">
                  <p>{{ testResult.message }}</p>
                  <p v-if="testResult.detail" class="sw-test-detail">{{ testResult.detail }}</p>
                </div>
              </div>

              <!-- character-select -->
              <div v-if="currentStep.key === 'character-select'" class="sw-form">
                <p class="sw-hint">Choose a default personality for your AI companion. You can change this later.</p>
                <div v-if="characters.length === 0" class="sw-notice">
                  No characters found. A default character will be created automatically.
                </div>
                <div v-else class="sw-char-list">
                  <label
                    v-for="c in characters"
                    :key="c.id"
                    class="sw-option-card sw-char-card"
                    :class="{ selected: localData.characterId === c.id }"
                  >
                    <input v-model="localData.characterId" type="radio" :value="c.id" />
                    <div class="sw-option-body">
                      <strong>{{ c.name }}</strong>
                      <p>{{ c.identity || 'No description' }}</p>
                    </div>
                  </label>
                </div>
              </div>

              <!-- wechat-option -->
              <div v-if="currentStep.key === 'wechat-option'" class="sw-options">
                <label class="sw-option-card" :class="{ selected: localData.enableWechat === true }">
                  <input v-model="localData.enableWechat" type="radio" :value="true" />
                  <div class="sw-option-body">
                    <strong>Enable WeChat Access</strong>
                    <p>Your companion can receive and respond to messages via WeChat. You will need to scan a QR code after setup.</p>
                  </div>
                </label>
                <label class="sw-option-card" :class="{ selected: localData.enableWechat === false }">
                  <input v-model="localData.enableWechat" type="radio" :value="false" />
                  <div class="sw-option-body">
                    <strong>Skip for Now</strong>
                    <p>You can enable WeChat access later from the Settings page.</p>
                  </div>
                </label>
              </div>

              <!-- cloud-deploy-info -->
              <div v-if="currentStep.key === 'cloud-deploy-info'" class="sw-info">
                <div v-if="deployMode === 'cloud-web'" class="sw-info-card">
                  <h3>Cloud Deployment</h3>
                  <ul>
                    <li>Your companion runs 24/7 on the cloud server</li>
                    <li>No need to keep your personal computer running</li>
                    <li>Access via web browser or WeChat from anywhere</li>
                    <li>All data is stored on your own server</li>
                  </ul>
                </div>
                <div v-else class="sw-info-card">
                  <h3>Desktop Local</h3>
                  <ul>
                    <li>All services run on this machine</li>
                    <li>Data stays on this computer only</li>
                    <li>This computer needs to be running for the app to work</li>
                  </ul>
                </div>
              </div>

              <!-- privacy-boundary -->
              <div v-if="currentStep.key === 'privacy-boundary'" class="sw-info">
                <div class="sw-info-card">
                  <h3>Privacy &amp; Security Boundaries</h3>
                  <ul>
                    <li><strong>Single User:</strong> This is a single-user system. No multi-user, tenant, or billing features.</li>
                    <li><strong>Data Ownership:</strong> All data is stored locally in your database. No data is sent to third parties.</li>
                    <li><strong>Model Calls:</strong> Messages are sent to your configured AI model provider for processing.</li>
                    <li><strong>No Registration:</strong> There is no public registration, invitation, or platform admin panel.</li>
                  </ul>
                </div>
                <div class="sw-confirm">
                  <label>
                    <input v-model="localData.privacyConfirmed" type="checkbox" />
                    I understand and accept these boundaries
                  </label>
                </div>
              </div>

              <!-- finish -->
              <div v-if="currentStep.key === 'finish'" class="sw-finish">
                <div class="sw-finish-icon">&#10003;</div>
                <h2>Setup Complete!</h2>
                <p>Your AI companion is ready. You can now enter the dashboard and start chatting.</p>
              </div>
            </div>
          </div>
        </template>
      </div>

      <!-- Actions -->
      <div class="sw-actions">
        <button
          v-if="currentStepIdx > 0 && currentStep.key !== 'finish'"
          class="sw-btn sw-btn-secondary"
          :disabled="submitting"
          @click="goBack"
        >
          Back
        </button>
        <button
          v-if="currentStep.key !== 'finish'"
          class="sw-btn sw-btn-primary"
          :disabled="submitting || !canProceed"
          @click="handleNext"
        >
          {{ submitting ? 'Processing...' : currentStep.buttonLabel || 'Next' }}
        </button>
        <button
          v-if="currentStep.key === 'finish'"
          class="sw-btn sw-btn-primary"
          :disabled="submitting"
          @click="handleFinish"
        >
          {{ submitting ? 'Finishing...' : 'Enter Dashboard' }}
        </button>
      </div>

      <!-- Progress bar -->
      <div class="sw-progress">
        <div class="sw-progress-bar">
          <div class="sw-progress-fill" :style="{ width: progressPercent + '%' }"></div>
        </div>
        <span class="sw-progress-text">{{ completedSteps }} / {{ totalSteps }} steps</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from "vue"
import { useRouter } from "vue-router"
import { apiClient, setToken } from "../../ui-index"

const router = useRouter()

// ============================================================
// Wizard Step Definitions
// ============================================================
interface WizardStep {
  key: string
  label: string
  title: string
  description: string
  buttonLabel?: string
  done: boolean
  requiresAction: boolean
}

const wizardSteps = ref<WizardStep[]>([
  { key: "detect-mode", label: "Mode", title: "Select Deployment Mode", description: "Choose how you want to run your AI companion.", buttonLabel: "Confirm Mode", done: false, requiresAction: true },
  { key: "core-check", label: "Core", title: "Core Service Check", description: "Verifying the core service is running.", done: false, requiresAction: false },
  { key: "web-check", label: "Web", title: "Web Interface Check", description: "Verifying the web interface is accessible.", done: false, requiresAction: false },
  { key: "bridge-check", label: "Bridge", title: "WeChat Bridge Check", description: "Checking WeChat Bridge availability.", done: false, requiresAction: false },
  { key: "db-check", label: "Database", title: "Database Check", description: "Verifying the database is writable.", done: false, requiresAction: false },
  { key: "admin-password", label: "Password", title: "Set Admin Password", description: "Create an admin account to protect your data.", buttonLabel: "Save Password", done: false, requiresAction: true },
  { key: "model-config", label: "Model", title: "Configure AI Model", description: "Connect your AI model API for chat functionality.", buttonLabel: "Save & Test", done: false, requiresAction: true },
  { key: "model-test", label: "Test", title: "Test Model Connection", description: "Verify the model API is reachable and working.", buttonLabel: "Test Connection", done: false, requiresAction: true },
  { key: "character-select", label: "Character", title: "Choose Default Character", description: "Select the personality for your AI companion.", buttonLabel: "Confirm Character", done: false, requiresAction: false },
  { key: "wechat-option", label: "WeChat", title: "WeChat Access", description: "Decide if you want to use the companion via WeChat.", buttonLabel: "Confirm", done: false, requiresAction: true },
  { key: "cloud-deploy-info", label: "Deploy", title: "Deployment Overview", description: "Review how your system will run.", buttonLabel: "Continue", done: false, requiresAction: false },
  { key: "privacy-boundary", label: "Privacy", title: "Privacy & Security", description: "Understand your privacy and security boundaries.", buttonLabel: "Confirm", done: false, requiresAction: true },
  { key: "finish", label: "Done", title: "Setup Complete", description: "You are all set!", buttonLabel: "Enter Dashboard", done: false, requiresAction: true },
])

const stepKeys = [
  "detect-mode", "core-check", "web-check", "bridge-check", "db-check",
  "admin-password", "model-config", "model-test", "character-select",
  "wechat-option", "cloud-deploy-info", "privacy-boundary", "finish",
]

// ============================================================
// State
// ============================================================
const loading = ref(true)
const submitting = ref(false)
const errorMsg = ref("")
const errorSuggestion = ref("")
const testResult = ref<{ passed: boolean; message: string; detail?: string } | null>(null)
const skipAuth = ref(false)
const characters = ref<{ id: string; name: string; identity: string }[]>([])
const deployMode = ref<"desktop-local" | "cloud-web">("desktop-local")
const initializing = ref(true)

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

const currentStepIdx = ref(0)
const completedSteps = ref(0)
const totalSteps = stepKeys.length

const currentStep = computed(() => wizardSteps.value[currentStepIdx.value])

const progressPercent = computed(() => {
  return Math.round((completedSteps.value / totalSteps) * 100)
})

const canProceed = computed(() => {
  const step = currentStep.value
  if (submitting.value) return false
  if (!step) return false
  if (!step.requiresAction) return true

  switch (step.key) {
    case "detect-mode":
      return !!localData.deployMode
    case "admin-password":
      if (deployMode.value === "cloud-web") {
        return !!localData.username && localData.password.length >= 6 && localData.password === localData.confirmPassword
      }
      return skipAuth.value || (!!localData.username && localData.password.length >= 6 && localData.password === localData.confirmPassword)
    case "model-config":
      return !!localData.apiType && !!localData.baseUrl && !!localData.apiKey && !!localData.modelName
    case "model-test":
      return true
    case "wechat-option":
      return localData.enableWechat !== null
    case "privacy-boundary":
      return localData.privacyConfirmed
    case "finish":
      return true
    default:
      return true
  }
})

// ============================================================
// API helpers
// ============================================================

async function fetchStatus() {
  const res = await apiClient.get("/api/setup/status")
  const data = res.data?.data || res.data
  return data
}

async function fetchChecks() {
  const res = await apiClient.get("/api/setup/checks")
  const data = res.data?.data || res.data
  return data.checks || []
}

async function submitStep(step: string, stepData?: any) {
  const res = await apiClient.post("/api/setup/step", { step, data: stepData })
  const data = res.data?.data || res.data
  return data
}

async function submitFinish() {
  const res = await apiClient.post("/api/setup/finish")
  const data = res.data?.data || res.data
  return data
}

async function fetchCharacters() {
  try {
    const res = await apiClient.get("/api/characters")
    const data = res.data?.data || res.data
    return data?.characters || data || []
  } catch {
    return []
  }
}

// ============================================================
// Auto-run steps (no user action needed)
// ============================================================
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

function markStepDone(key: string) {
  const s = wizardSteps.value.find((s) => s.key === key)
  if (s) s.done = true
  completedSteps.value = wizardSteps.value.filter((s) => s.done).length
}

// ============================================================
// Navigation
// ============================================================

function goBack() {
  if (currentStepIdx.value > 0) {
    currentStepIdx.value--
    errorMsg.value = ""
    errorSuggestion.value = ""
    loadStepData()
  }
}

function skipStep() {
  markStepDone(currentStep.value.key)
  currentStepIdx.value++
  errorMsg.value = ""
  errorSuggestion.value = ""
  loadStepData()
  runCurrentAutoStep()
}

function retryStep() {
  errorMsg.value = ""
  errorSuggestion.value = ""
  handleNext()
}

// ============================================================
// Handle next step
// ============================================================

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
            stepData = {
              username: localData.username,
              password: localData.password,
              confirmPassword: localData.confirmPassword,
            }
          }
          break
        case "model-config":
          stepData = {
            apiType: localData.apiType,
            baseUrl: localData.baseUrl,
            apiKey: localData.apiKey,
            modelName: localData.modelName,
          }
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

      // Handle model test result display
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
      loadStepData()
      runCurrentAutoStep()
    } catch (err: any) {
      const msg = err?.response?.data?.message || err?.message || "An error occurred"
      const suggestion = err?.response?.data?.suggestion || err?.response?.data?.detail || ""
      errorMsg.value = msg
      errorSuggestion.value = suggestion
      submitting.value = false
    }
  } else {
    // Auto-step: mark done and move on
    markStepDone(step.key)
    currentStepIdx.value++
    errorMsg.value = ""
    errorSuggestion.value = ""
    loadStepData()
    runCurrentAutoStep()
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

function loadStepData() {
  // Called before rendering each step to load any needed data
  const step = currentStep.value
  if (!step) return
  if (step.key === "character-select" && characters.value.length === 0) {
    fetchCharacters().then((chars) => {
      characters.value = chars
    })
  }
}

async function runCurrentAutoStep() {
  // If current step requires no action, auto-run it
  const step = wizardSteps.value[currentStepIdx.value]
  if (step && !step.requiresAction) {
    // core-check, web-check, bridge-check, db-check, character-select, cloud-deploy-info
    if (["core-check", "web-check", "bridge-check", "db-check", "cloud-deploy-info"].includes(step.key)) {
      loading.value = true
      const success = await runAutoStep(step.key)
      loading.value = false
      if (success) {
        currentStepIdx.value++
        // Check next step too (chained auto-steps)
        runCurrentAutoStep()
      }
    }
  }
}

// ============================================================
// Initialization
// ============================================================

onMounted(async () => {
  try {
    const status = await fetchStatus()

    // If already completed, redirect to dashboard
    if (status.completed) {
      router.replace("/dashboard")
      return
    }

    // Restore deploy mode
    if (status.deployMode) {
      deployMode.value = status.deployMode
      localData.deployMode = status.deployMode
    }

    // Restore done steps
    if (status.steps) {
      for (const s of status.steps) {
        const ws = wizardSteps.value.find((w) => w.key === s.step)
        if (ws && s.done) ws.done = true
      }
      completedSteps.value = wizardSteps.value.filter((s) => s.done).length
    }

    // Find current step index
    const curStep = status.currentStep || "detect-mode"
    const idx = stepKeys.indexOf(curStep)
    currentStepIdx.value = idx >= 0 ? idx : 0

    // Load characters for character step
    fetchCharacters().then((chars) => {
      characters.value = chars
    })

    initializing.value = false
    loading.value = false

    // Auto-run non-interactive steps if at one
    if (!currentStep.value?.requiresAction) {
      runCurrentAutoStep()
    }
  } catch (err) {
    // Core may not be running; proceed with wizard anyway
    loading.value = false
    initializing.value = false
  }
})
</script>

<style scoped>
.setup-wizard {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--ac-color-bg, #f5f5f5);
  padding: 20px;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
}

.sw-container {
  width: 100%;
  max-width: 560px;
  background: var(--ac-color-surface, #fff);
  border: 1px solid var(--ac-color-border-light, #e5e5e5);
  border-radius: 8px;
  padding: 32px 28px 24px;
  box-shadow: 0 1px 3px rgba(0,0,0,0.06);
}

.sw-header {
  text-align: center;
  margin-bottom: 24px;
}

.sw-title {
  font-size: 22px;
  font-weight: 600;
  color: var(--ac-color-text, #1a1a1a);
  margin: 0 0 6px;
}

.sw-subtitle {
  font-size: 13px;
  color: var(--ac-color-text-muted, #888);
  margin: 0;
}

/* Step dots */
.sw-steps {
  display: flex;
  justify-content: center;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 28px;
  flex-wrap: wrap;
  padding: 0 8px;
}

.sw-step-dot {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  opacity: 0.35;
  transition: opacity 0.2s;
  cursor: default;
  max-width: 60px;
}

.sw-step-dot.active,
.sw-step-dot.done {
  opacity: 1;
}

.sw-dot {
  width: 26px;
  height: 26px;
  border-radius: 50%;
  background: var(--ac-color-bg-secondary, #e8e8e8);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 11px;
  font-weight: 600;
  color: var(--ac-color-text-secondary, #666);
}

.sw-step-dot.active .sw-dot {
  background: var(--ac-color-primary, #4a7c59);
  color: #fff;
}

.sw-step-dot.done .sw-dot {
  background: var(--ac-color-success, #3a8f4a);
  color: #fff;
}

.sw-dot-label {
  font-size: 10px;
  color: var(--ac-color-text-muted, #999);
  text-align: center;
  white-space: nowrap;
}

/* Content */
.sw-content {
  min-height: 180px;
  margin-bottom: 20px;
}

.sw-loading {
  display: flex;
  justify-content: center;
  padding: 40px 0;
  color: var(--ac-color-text-muted, #888);
}

.sw-step-body {
  animation: swFadeIn 0.2s ease;
}

@keyframes swFadeIn {
  from { opacity: 0; transform: translateY(6px); }
  to { opacity: 1; transform: translateY(0); }
}

.sw-step-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--ac-color-text, #1a1a1a);
  margin: 0 0 6px;
}

.sw-step-desc {
  font-size: 13px;
  color: var(--ac-color-text-secondary, #666);
  margin: 0 0 16px;
  line-height: 1.5;
}

/* Error display */
.sw-error {
  padding: 16px;
  border: 1px solid var(--ac-color-error-light, #f5c6c6);
  background: var(--ac-color-error-bg, #fef2f2);
  border-radius: 6px;
}

.sw-error-title {
  font-weight: 600;
  color: var(--ac-color-error, #dc2626);
  margin: 0 0 4px;
}

.sw-error-msg {
  font-size: 13px;
  color: var(--ac-color-error, #dc2626);
  margin: 0 0 4px;
}

.sw-error-suggestion {
  font-size: 12px;
  color: var(--ac-color-text-muted, #888);
  margin: 4px 0 12px;
  font-style: italic;
}

.sw-error-actions {
  display: flex;
  gap: 8px;
}

/* Form elements */
.sw-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.sw-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.sw-field label {
  font-size: 13px;
  font-weight: 500;
  color: var(--ac-color-text, #1a1a1a);
}

.sw-field input,
.sw-field select {
  padding: 8px 12px;
  border: 1px solid var(--ac-color-border-light, #d4d4d4);
  border-radius: 4px;
  font-size: 14px;
  color: var(--ac-color-text, #1a1a1a);
  background: var(--ac-color-surface, #fff);
  outline: none;
  transition: border-color 0.15s;
}

.sw-field input:focus,
.sw-field select:focus {
  border-color: var(--ac-color-primary, #4a7c59);
}

/* Notice boxes */
.sw-notice {
  padding: 10px 12px;
  background: var(--ac-color-bg-secondary, #f0f0f0);
  border-radius: 4px;
  font-size: 12px;
  color: var(--ac-color-text-secondary, #666);
  line-height: 1.5;
}

.sw-notice-warn {
  background: #fff7ed;
  border: 1px solid #fed7aa;
  color: var(--ac-color-text, #1a1a1a);
}

.sw-skip-option {
  font-size: 13px;
  color: var(--ac-color-text-muted, #888);
}

.sw-skip-option label {
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
}

/* Option cards */
.sw-options {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.sw-option-card {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px 14px;
  border: 1px solid var(--ac-color-border-light, #d4d4d4);
  border-radius: 6px;
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s;
}

.sw-option-card:hover {
  background: var(--ac-color-bg-secondary, #fafafa);
}

.sw-option-card.selected {
  border-color: var(--ac-color-primary, #4a7c59);
  background: var(--ac-color-primary-bg, #f0f7f0);
}

.sw-option-card input[type="radio"] {
  margin-top: 2px;
  accent-color: var(--ac-color-primary, #4a7c59);
}

.sw-option-body strong {
  font-size: 14px;
  display: block;
  margin-bottom: 2px;
  color: var(--ac-color-text, #1a1a1a);
}

.sw-option-body p {
  font-size: 12px;
  color: var(--ac-color-text-secondary, #777);
  margin: 0;
  line-height: 1.4;
}

/* Character cards */
.sw-char-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.sw-char-card {
  padding: 10px 12px;
}

/* Info cards */
.sw-info {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.sw-info-card {
  padding: 16px;
  background: var(--ac-color-bg-secondary, #fafafa);
  border: 1px solid var(--ac-color-border-light, #e5e5e5);
  border-radius: 6px;
}

.sw-info-card h3 {
  font-size: 15px;
  font-weight: 600;
  margin: 0 0 10px;
  color: var(--ac-color-text, #1a1a1a);
}

.sw-info-card ul {
  margin: 0;
  padding: 0;
  list-style: none;
}

.sw-info-card li {
  font-size: 13px;
  color: var(--ac-color-text-secondary, #555);
  padding: 4px 0;
  padding-left: 16px;
  position: relative;
  line-height: 1.5;
}

.sw-info-card li::before {
  content: "";
  position: absolute;
  left: 0;
  top: 11px;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--ac-color-primary, #4a7c59);
}

.sw-confirm {
  font-size: 13px;
  color: var(--ac-color-text, #1a1a1a);
}

.sw-confirm label {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
}

/* Test result */
.sw-test-result {
  padding: 10px 12px;
  border-radius: 4px;
  font-size: 13px;
}

.sw-test-result.success {
  background: #f0fdf4;
  border: 1px solid #bbf7d0;
  color: #166534;
}

.sw-test-result.failed {
  background: #fef2f2;
  border: 1px solid #fecaca;
  color: #991b1b;
}

.sw-test-detail {
  font-size: 11px;
  white-space: pre-wrap;
  word-break: break-all;
}

.sw-hint {
  font-size: 13px;
  color: var(--ac-color-text-secondary, #666);
  margin: 0 0 8px;
}

/* Finish screen */
.sw-finish {
  text-align: center;
  padding: 20px 0;
}

.sw-finish-icon {
  width: 48px;
  height: 48px;
  border-radius: 50%;
  background: var(--ac-color-success, #3a8f4a);
  color: #fff;
  font-size: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin: 0 auto 16px;
}

.sw-finish h2 {
  font-size: 20px;
  margin: 0 0 8px;
  color: var(--ac-color-text, #1a1a1a);
}

.sw-finish p {
  font-size: 14px;
  color: var(--ac-color-text-secondary, #666);
}

/* Actions */
.sw-actions {
  display: flex;
  justify-content: center;
  gap: 10px;
  margin-bottom: 16px;
}

.sw-btn {
  padding: 8px 24px;
  border-radius: 4px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  border: 1px solid transparent;
  transition: background 0.15s, opacity 0.15s;
}

.sw-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.sw-btn-primary {
  background: var(--ac-color-primary, #4a7c59);
  color: #fff;
  border-color: var(--ac-color-primary, #4a7c59);
}

.sw-btn-primary:hover:not(:disabled) {
  background: var(--ac-color-primary-dark, #3d6649);
}

.sw-btn-secondary {
  background: var(--ac-color-surface, #fff);
  color: var(--ac-color-text, #1a1a1a);
  border-color: var(--ac-color-border-light, #d4d4d4);
}

.sw-btn-secondary:hover:not(:disabled) {
  background: var(--ac-color-bg-secondary, #f5f5f5);
}

.sw-btn-ghost {
  background: transparent;
  color: var(--ac-color-text-secondary, #666);
  border-color: transparent;
  font-size: 13px;
}

.sw-btn-ghost:hover:not(:disabled) {
  background: var(--ac-color-bg-secondary, #f5f5f5);
}

/* Progress bar */
.sw-progress {
  display: flex;
  align-items: center;
  gap: 10px;
}

.sw-progress-bar {
  flex: 1;
  height: 4px;
  background: var(--ac-color-bg-secondary, #e5e5e5);
  border-radius: 2px;
  overflow: hidden;
}

.sw-progress-fill {
  height: 100%;
  background: var(--ac-color-primary, #4a7c59);
  border-radius: 2px;
  transition: width 0.3s ease;
}

.sw-progress-text {
  font-size: 11px;
  color: var(--ac-color-text-muted, #999);
  white-space: nowrap;
}

/* Mobile */
@media (max-width: 640px) {
  .setup-wizard {
    padding: 10px;
    align-items: flex-start;
    padding-top: 40px;
  }

  .sw-container {
    border: none;
    box-shadow: none;
    padding: 20px 16px 16px;
    max-width: 100%;
  }

  .sw-steps {
    gap: 8px;
  }

  .sw-dot-label {
    display: none;
  }

  .sw-step-dot {
    max-width: 28px;
  }
}
</style>
