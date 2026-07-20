<template>
  <div
    class="ob-shell"
    :class="`ob-stage-${currentStage}`"
  >
    <div
      class="onboarding-world"
      ref="worldRef"
      :data-level="Math.min(currentStage, 7)"
      :data-stage-current="coreLayoutStage"
      :data-entry-preparing="entryPreparing ? 'true' : null"
      :data-entering="enteringState"
      :data-core-reveal-pending="coreRevealPending ? 'true' : null"
      :data-identity-state="currentStage === 7 ? identityState : null"
      :data-memory-step="currentStage === 8 ? (memoryComplete ? 'complete' : memoryStep) : null"
    >
      <StarfieldBg />
      <CoreZone ref="coreZoneRef" :caption="currentCaption" />

      <button
        class="ob-back"
        :class="{ show: currentStage > 0 }"
        @click="prevStage"
      >返回上一层</button>

      <section class="ob-stage" :class="{ active: stageInitReady && (currentStage === 0 || leavingStage === 0), 'stage-leaving': leavingStage === 0, 'stage-enter-prep': enterPrepStage === 0 }">
        <StageIntro @next="nextStage" />
      </section>

      <section class="ob-stage" :class="{ active: stageInitReady && (currentStage === 1 || leavingStage === 1), 'stage-leaving': leavingStage === 1, 'stage-enter-prep': enterPrepStage === 1 }">
        <StageDeployMode
          :deployMode="deployMode"
          :serverURL="serverURL"
          :detectingRemote="detectingRemote"
          :remoteStatusText="remoteStatusText"
          @update:deployMode="deployMode = $event"
          @update:serverURL="serverURL = $event"
          @next="nextStage"
          @checkRemote="checkRemoteConnection"
        />
      </section>

      <section class="ob-stage ob-setup-stage" :class="{ active: stageInitReady && (currentStage === 2 || leavingStage === 2), 'stage-leaving': leavingStage === 2, 'stage-enter-prep': enterPrepStage === 2 }">
        <StageAdminSetup
          :deployMode="deployMode"
          :step="adminStep"
          :isLogin="isLoginFlow()"
          :accountName="accountName"
          :serverURL="serverURL"
          @update:step="adminStep = $event"
          @healthCheckDone="onHealthCheckDone"
          @submit="handleAdminSubmit"
        />
      </section>

      <section class="ob-stage" :class="{ active: stageInitReady && (currentStage === 3 || leavingStage === 3), 'stage-leaving': leavingStage === 3, 'stage-enter-prep': enterPrepStage === 3 }">
        <StageModelConfig
          :detecting="detectingModels"
          :modelReady="modelReady"
          :modelDetected="modelDetected"
          :detectedModels="detectedModels"
          :statusText="modelStatusText"
          :fieldErrors="modelFieldErrors"
          :baseUrl="modelBaseUrl"
          :apiKey="modelApiKey"
          :modelName="modelName"
          @update:baseUrl="modelBaseUrl = $event"
          @update:apiKey="modelApiKey = $event"
          @update:modelName="modelName = $event"
          @next="nextStage"
          @detect="detectModel"
        />
      </section>

      <section class="ob-stage" :class="{ active: stageInitReady && (currentStage === 4 || leavingStage === 4), 'stage-leaving': leavingStage === 4, 'stage-enter-prep': enterPrepStage === 4 }">
        <StageVisionMode
          :visionMode="visionMode"
          :visionModelKey="visionModelKey"
          :visionModelName="visionModelName"
          :visionModelURL="visionModelURL"
          :visionReady="visionReady"
          :visionDetected="visionDetected"
          :detecting="detectingVision"
          :statusText="visionStatusText"
          @update:visionMode="visionMode = $event"
          @update:visionModelKey="visionModelKey = $event"
          @update:visionModelName="visionModelName = $event"
          @update:visionModelURL="visionModelURL = $event"
          @next="nextStage"
          @detect="detectVision"
        />
      </section>

      <section class="ob-stage" :class="{ active: stageInitReady && (currentStage === 5 || leavingStage === 5), 'stage-leaving': leavingStage === 5, 'stage-enter-prep': enterPrepStage === 5 }">
        <StageVoice
          :voiceModelMode="voiceModelMode"
          :voiceModelKey="voiceModelKey"
          :voiceModelURL="voiceModelURL"
          :voiceModelResource="voiceModelResource"
          :voiceModelVoiceType="voiceModelVoiceType"
          :voiceReady="voiceReady"
          :voiceDetected="voiceDetected"
          :detectingVoice="detectingVoice"
          :voiceStatusText="voiceStatusText"
          @update:voiceModelMode="voiceModelMode = $event"
          @update:voiceModelKey="voiceModelKey = $event"
          @update:voiceModelURL="voiceModelURL = $event"
          @update:voiceModelResource="voiceModelResource = $event"
          @update:voiceModelVoiceType="voiceModelVoiceType = $event"
          @next="nextStage"
          @detectVoice="detectVoice"
        />
      </section>

      <section class="ob-stage" :class="{ active: stageInitReady && (currentStage === 6 || leavingStage === 6), 'stage-leaving': leavingStage === 6, 'stage-enter-prep': enterPrepStage === 6 }">
        <StageVectorModel
          :vectorModelKey="vectorModelKey"
          :vectorModelName="vectorModelName"
          :vectorModelURL="vectorModelURL"
          :vectorReady="vectorReady"
          :vectorDetected="vectorDetected"
          :detecting="detectingVector"
            :vectorModelMode="vectorModelMode"
          :statusText="vectorStatusText"
          @update:vectorModelKey="vectorModelKey = $event"
          @update:vectorModelName="vectorModelName = $event"
          @update:vectorModelURL="vectorModelURL = $event"
            @update:vectorModelMode="vectorModelMode = $event"
          @next="nextStage"
          @detect="detectVector"
        />
      </section>

      <section class="ob-stage" :class="{ active: stageInitReady && (currentStage === 7 || leavingStage === 7), 'stage-leaving': leavingStage === 7, 'stage-enter-prep': enterPrepStage === 7 }"><StageIdentity
          v-if="currentIdentityQuestion"
:step="identityStep"
          :question="currentIdentityQuestion.question"
          :context="currentIdentityQuestion.context"
          :placeholder="currentIdentityQuestion.placeholder"
          :quickChoices="currentIdentityQuestion.quickChoices"
          :maxLength="currentIdentityQuestion.maxLength"
          :ledger="{ name: identityName, role: identityRole, personality: identityPersonality }"
          :state="identityState"
          @answer="handleIdentityAnswer"
          @next="nextStage"
        />
</section>

      <section class="ob-stage" :class="{ active: stageInitReady && (currentStage === 8 || leavingStage === 8), 'stage-leaving': leavingStage === 8, 'stage-enter-prep': enterPrepStage === 8 }">
        <StageMemory
          v-if="currentMemoryQuestion"
          :key="memoryStep"
          :step="memoryStep"
          :question="currentMemoryQuestion.question"
          :context="currentMemoryQuestion.context"
          :placeholder="currentMemoryQuestion.placeholder"
          :quickChoices="currentMemoryQuestion.quickChoices"
          :items="memoryItems"
          :complete="memoryComplete"
          @answer="handleMemoryAnswer"
          @next="nextStage"
        />
      </section>

      <section class="ob-stage" :class="{ active: stageInitReady && (currentStage === 9 || leavingStage === 9), 'stage-leaving': leavingStage === 9, 'stage-enter-prep': enterPrepStage === 9 }">
        <StageBoundary
          :permissions="permissions"
          :entering="entering"
          @enter="startEntryTransition"
        />
      </section>

      <AmitiaEntryTransition />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from "vue"
import StarfieldBg from "./components/StarfieldBg.vue"
import CoreZone from "./components/CoreZone.vue"
import StageIntro from "./components/StageIntro.vue"
import StageDeployMode from "./components/StageDeployMode.vue"
import StageAdminSetup from "./components/StageAdminSetup.vue"
import StageModelConfig from "./components/StageModelConfig.vue"
import StageVisionMode from "./components/StageVisionMode.vue"
import StageVoice from "./components/StageVoice.vue"
import StageVectorModel from "./components/StageVectorModel.vue"
import StageIdentity from "./components/StageIdentity.vue"
import StageMemory from "./components/StageMemory.vue"
import StageBoundary from "./components/StageBoundary.vue"
import AmitiaEntryTransition from "./components/AmitiaEntryTransition.vue"
import { useImmersiveOnboarding } from "./composables/useImmersiveOnboarding"

import "./styles/onboarding.css"
import "./styles/stages.css"
import "./styles/typography.css"
import "./styles/stage-specific.css"
import "./styles/identity-memory.css"
import "./styles/boundary.css"
import "./styles/core-layout.css"
import "./styles/entry-transition.css"
import "./styles/bottom-alignment.css"
import "./styles/completion-buttons.css"

const {
  currentStage,
  deployMode,
  serverURL,
  adminStep,
  accountName,
  detectingModels,
  modelReady,
  modelDetected,
  detectedModels,
  modelFieldErrors,
  modelStatusText,
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
  vectorModelKey,
  vectorModelName,
  vectorModelMode,
  vectorModelURL,
  identityStep,
  identityState,
  identityName,
  identityRole,
  identityPersonality,
  memoryStep,
  memoryComplete,
  memoryItems,
  permissions,
  entering,
  entryPreparing,
  enteringState,
  currentCaption,
  currentIdentityQuestion,
  currentMemoryQuestion,
  nextStage,
  prevStage,
  checkAdminExists,
  isLoginFlow,
  handleAdminSubmit,
  detectModel,
  detectVision,
  detectVoice,
  detectVector,
  handleIdentityAnswer,
  handleMemoryAnswer,
  handleEnterAmitia,
  startEntryTransition,
  playVoiceSample,
  coreRevealPending,
  leavingStage,
  enterPrepStage,
} = useImmersiveOnboarding()

const coreZoneRef = ref<InstanceType<typeof CoreZone> | null>(null)
const coreLayoutStage = ref<number | null>(null)
const stageInitReady = ref(true)

const detectingRemote = ref(false)
const remoteStatusText = ref("")

const worldRef = ref<HTMLDivElement | null>(null)

watch(currentStage, (val) => {
  if (stageInitReady.value && coreLayoutStage.value !== val) {
    coreLayoutStage.value = val
  }
})

watch(serverURL, () => {
  remoteStatusText.value = ""
})

async function onHealthCheckDone() {
  await checkAdminExists()
  adminStep.value = "account"
}

function checkRemoteConnection() {
  const url = serverURL.value.trim()
  if (!url) return
  serverURL.value = url

  detectingRemote.value = true
  remoteStatusText.value = "正在检测"
  coreZoneRef.value?.ping()

  const healthUrl = url.replace(/\/+$/, "") + "/api/health"
  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), 10000)

  fetch(healthUrl, { signal: controller.signal })
    .then((res) => {
      if (res.ok) {
        remoteStatusText.value = "连接成功"
      } else {
        remoteStatusText.value = `服务器返回 ${res.status}`
      }
    })
    .catch(() => {
      remoteStatusText.value = "无法连接，请检查地址"
    })
    .finally(() => {
      clearTimeout(timeout)
      detectingRemote.value = false
    })
}
onMounted(() => {
  requestAnimationFrame(() => {
    coreLayoutStage.value = 0
  })
})

</script>

<style scoped>
.ob-shell {
  width: 100%;
  height: 100%;
  overflow: hidden;
  background: #070708;
  display: grid;
  place-items: center;
}
</style>
