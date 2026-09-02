<template>
  <div class="ob-shell" :class="`ob-stage-${currentStage}`">
    <div
      class="onboarding-world"
      :data-level="Math.min(currentStage, 7)"
      :data-stage-current="currentStage"
      :data-entry-preparing="entryPreparing ? 'true' : null"
      :data-entering="enteringState"
    >
      <StarfieldBg />

      <button
        class="ob-back"
        :class="{ show: currentStage > 0 }"
        type="button"
        @click="prevStage"
      >
        上一步
      </button>

      <section class="ob-stage active">
        <StageDeployMode
          v-if="currentStage === 0"
          ref="deployStageRef"
          :deployMode="deployMode"
          :serverURL="serverURL"
          :detectingRemote="detectingRemote"
          :remoteStatusText="remoteStatusText"
          @update:deployMode="deployMode = $event"
          @update:serverURL="serverURL = $event"
          @next="nextStage"
          @checkRemote="checkRemoteConnection"
        />

        <StageAdminSetup
          v-else-if="currentStage === 1"
          ref="adminStageRef"
          :deployMode="deployMode"
          :step="adminStep"
          :isLogin="isLoginFlow()"
          :accountName="accountName"
          :serverURL="serverURL"
          @healthCheckDone="onHealthCheckDone"
          @submit="handleAdminSubmit"
        />

        <StageModelConfig
          v-else
          :detecting="detectingModels"
          :modelReady="modelReady"
          :modelDetected="modelDetected"
          :detectedModels="detectedModels"
          :statusText="modelStatusText"
          :fieldErrors="modelFieldErrors"
          :baseUrl="modelBaseUrl"
          :apiKey="modelApiKey"
          :modelName="modelName"
          :modelType="modelType"
          @update:baseUrl="modelBaseUrl = $event"
          @update:apiKey="modelApiKey = $event"
          @update:modelName="modelName = $event"
          @update:modelType="modelType = $event"
          @detect="detectModel"
        />
      </section>

      <div class="ob-guide-actions">
        <button
          v-if="currentStage === 0"
          class="ob-stage-action"
          type="button"
          @click="deployStageRef?.continueFlow()"
        >
          下一步
        </button>
        <button
          v-else-if="currentStage === 1 && adminStep === 'account'"
          class="ob-stage-action ob-setup-inline-action"
          type="button"
          @click="adminStageRef?.submit()"
        >
          下一步
        </button>
        <button
          v-else-if="currentStage === 2"
          class="ob-model-setup-next"
          type="button"
          :disabled="!isModelFormValid"
          @click="handleStartUsing"
        >
          开始使用
        </button>
      </div>

      <AmitiaEntryTransition />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import StarfieldBg from "./components/StarfieldBg.vue";
import StageDeployMode from "./components/StageDeployMode.vue";
import StageAdminSetup from "./components/StageAdminSetup.vue";
import StageModelConfig from "./components/StageModelConfig.vue";
import AmitiaEntryTransition from "./components/AmitiaEntryTransition.vue";
import { useImmersiveOnboarding } from "./composables/useImmersiveOnboarding";

import "./styles/onboarding.css";
import "./styles/stages.css";
import "./styles/typography.css";
import "./styles/stage-specific.css";
import "./styles/entry-transition.css";

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
  modelType,
  entryPreparing,
  enteringState,
  nextStage,
  prevStage,
  checkAdminExists,
  isLoginFlow,
  handleAdminSubmit,
  detectModel,
  startEntryTransition,
} = useImmersiveOnboarding();

const deployStageRef = ref<{ continueFlow: () => void } | null>(null);
const adminStageRef = ref<{ submit: () => void } | null>(null);
const detectingRemote = ref(false);
const remoteStatusText = ref("");

const isModelFormValid = computed(() => {
  if (!modelBaseUrl.value.trim()) return false;
  if (modelType.value !== "local" && !modelApiKey.value.trim()) return false;
  return modelName.value.trim().length > 0;
});

watch(serverURL, () => {
  remoteStatusText.value = "";
});

async function onHealthCheckDone() {
  await checkAdminExists();
  adminStep.value = "account";
}

function handleStartUsing() {
  modelFieldErrors.value = {};
  if (!modelBaseUrl.value.trim()) modelFieldErrors.value.baseUrl = true;
  if (modelType.value !== "local" && !modelApiKey.value.trim()) {
    modelFieldErrors.value.apiKey = true;
  }
  if (!modelName.value.trim()) modelFieldErrors.value.modelName = true;
  if (Object.keys(modelFieldErrors.value).length > 0) return;
  startEntryTransition();
}

function checkRemoteConnection() {
  const url = serverURL.value.trim();
  if (!url) return;
  serverURL.value = url;
  detectingRemote.value = true;
  remoteStatusText.value = "正在检测";

  const healthUrl = url.replace(/\/+$/, "") + "/api/health";
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 10000);

  fetch(healthUrl, { signal: controller.signal })
    .then((res) => {
      remoteStatusText.value = res.ok ? "连接成功" : `服务器返回 ${res.status}`;
    })
    .catch(() => {
      remoteStatusText.value = "无法连接，请检查地址";
    })
    .finally(() => {
      clearTimeout(timeout);
      detectingRemote.value = false;
    });
}
</script>

<style scoped>
.ob-shell {
  width: 100%;
  height: 100%;
  overflow: hidden;
  background: #070708;
}
</style>
