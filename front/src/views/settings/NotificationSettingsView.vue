<template>
  <div class="panel">
    <div class="head">
      <el-button :loading="loading" data-amitia-ui="settings.button" @click="load">刷新</el-button>
    </div>

    <el-alert v-if="error" :title="error" type="error" :closable="false" show-icon>
      <el-button link type="primary" @click="load">重新加载</el-button>
    </el-alert>

    <el-card v-loading="loading" shadow="never" data-amitia-ui="settings.card">
      <el-form label-position="left" label-width="150px">
        <el-form-item label="允许通知">
          <el-switch
            v-model="preferences.pushEnabled"
            data-amitia-ui="component.switch"
            :loading="saving"
            @change="(value: string | number | boolean) => updatePush(Boolean(value))"
          />
        </el-form-item>
        <el-form-item label="AI 消息">
          <el-switch
            v-model="preferences.messagePushEnabled"
            data-amitia-ui="component.switch"
            :disabled="!preferences.pushEnabled || saving"
            @change="(value: string | number | boolean) => updatePreferences({ messagePushEnabled: Boolean(value) })"
          />
        </el-form-item>
        <el-form-item label="Agent 执行状态">
          <el-switch
            v-model="preferences.executionActivityEnabled"
            data-amitia-ui="component.switch"
            :disabled="!preferences.pushEnabled || saving"
            @change="(value: string | number | boolean) => updatePreferences({ executionActivityEnabled: Boolean(value) })"
          />
        </el-form-item>
        <el-form-item label="通话">
          <el-switch
            v-model="preferences.callPushEnabled"
            data-amitia-ui="component.switch"
            :disabled="!preferences.pushEnabled || saving"
            @change="(value: string | number | boolean) => updatePreferences({ callPushEnabled: Boolean(value) })"
          />
        </el-form-item>
        <el-form-item label="提醒与主动任务">
          <el-switch
            v-model="preferences.reminderPushEnabled"
            data-amitia-ui="component.switch"
            :disabled="!preferences.pushEnabled || saving"
            @change="(value: string | number | boolean) => updatePreferences({ reminderPushEnabled: Boolean(value) })"
          />
        </el-form-item>
        <el-form-item label="声音">
          <el-switch
            v-model="preferences.soundEnabled"
            data-amitia-ui="component.switch"
            :disabled="!preferences.pushEnabled || saving"
            @change="(value: string | number | boolean) => updatePreferences({ soundEnabled: Boolean(value) })"
          />
        </el-form-item>
        <el-form-item label="隐私预览">
          <el-radio-group
            v-model="preferences.previewMode"
            :disabled="!preferences.pushEnabled || saving"
            @change="updatePreview"
          >
            <el-radio-button value="full">完整</el-radio-button>
            <el-radio-button value="sender_only">仅角色</el-radio-button>
            <el-radio-button value="hidden">隐藏内容</el-radio-button>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <div class="actions">
        <el-button type="primary" data-amitia-ui="settings.button" :loading="saving" :disabled="!preferences.pushEnabled" @click="test">发送正式测试通知</el-button>
      </div>
    </el-card>

    <el-card shadow="never" data-amitia-ui="settings.card">
      <template #header>设备投递状态</template>
      <el-descriptions :column="1" border>
        <el-descriptions-item label="设备">{{ deviceId || '等待设备身份' }}</el-descriptions-item>
        <el-descriptions-item label="设备名称">{{ endpoint?.deviceName || '未提供' }}</el-descriptions-item>
        <el-descriptions-item label="平台">{{ platformLabel }}</el-descriptions-item>
        <el-descriptions-item label="系统版本">{{ endpoint?.osVersion || '未提供' }}</el-descriptions-item>
        <el-descriptions-item label="Provider">{{ endpoint?.preferredProvider || 'desktop-sse' }}</el-descriptions-item>
        <el-descriptions-item label="投递模式">{{ deliveryMode }}</el-descriptions-item>
        <el-descriptions-item label="Provider 就绪">{{ providerReadiness }}</el-descriptions-item>
      </el-descriptions>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessage } from "element-plus";
import { apiClient } from "@/composables/useApi";
import { resolveUIHostDeviceId } from "@/ui-runtime/deviceIdentity";
import {
  ensureNotificationDevice,
  notificationPlatform,
  testNotificationDevice,
  updateNotificationPreferences,
  type DeviceNotificationEndpoint,
  type DeviceNotificationPreferences,
} from "@/runtime/device-notifications";

const loading = ref(false);
const saving = ref(false);
const error = ref("");
const deviceId = ref("");
const endpoint = ref<DeviceNotificationEndpoint>();
const capabilities = ref<Record<string, any>>({});
const preferences = reactive<DeviceNotificationPreferences>({
  pushEnabled: false,
  systemNotificationsEnabled: false,
  messagePushEnabled: true,
  executionActivityEnabled: true,
  callPushEnabled: true,
  reminderPushEnabled: true,
  previewMode: "full",
  soundEnabled: true,
});

const platform = notificationPlatform();
const platformLabel = computed(() => platform === "desktop" ? "电脑端" : "Web");
const deliveryMode = computed(() => {
  if (capabilities.value.deliveryMode === "native") return "本机原生通道";
  if (capabilities.value.deliveryMode === "remote") return "Core 远程持久化投递";
  return "由当前 Core 管理";
});
const providerReadiness = computed(() => {
  const providers = capabilities.value.pushProviders;
  const available = providers && typeof providers === "object"
    ? Object.entries(providers).filter(([, value]) => value === true).map(([name]) => name)
    : [];
  return available.length ? available.join("、") : compactProvider();
});

function compactProvider() {
  return platform === "desktop" ? "桌面 SSE 可用性以测试结果为准" : "浏览器 SSE 可用性以测试结果为准";
}

function applyEndpoint(value: DeviceNotificationEndpoint) {
  endpoint.value = value;
  preferences.pushEnabled = value.pushEnabled;
  preferences.systemNotificationsEnabled = value.systemNotificationsEnabled;
  preferences.messagePushEnabled = value.messagePushEnabled;
  preferences.executionActivityEnabled = value.executionActivityEnabled;
  preferences.callPushEnabled = value.callPushEnabled;
  preferences.reminderPushEnabled = value.reminderPushEnabled;
  preferences.previewMode = value.previewMode;
  preferences.soundEnabled = value.soundEnabled;
}

async function requestPermission() {
  if (typeof Notification === "undefined") throw new Error("当前运行环境不支持系统通知");
  if (Notification.permission === "granted") return;
  if (await Notification.requestPermission() !== "granted") throw new Error("系统通知权限未授予");
}

async function load() {
  loading.value = true;
  error.value = "";
  try {
    deviceId.value = await resolveUIHostDeviceId();
    applyEndpoint(await ensureNotificationDevice(deviceId.value));
    const result = await apiClient.get("/api/notifications/capabilities");
    capabilities.value = result.data?.data ?? result.data ?? {};
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : String(cause);
  } finally {
    loading.value = false;
  }
}

async function updatePush(enabled: boolean) {
  try {
    if (enabled) await requestPermission();
    await updatePreferences({ pushEnabled: enabled, systemNotificationsEnabled: enabled });
  } catch (cause) {
    preferences.pushEnabled = endpoint.value?.pushEnabled ?? false;
    ElMessage.error(cause instanceof Error ? cause.message : String(cause));
  }
}

async function updatePreview(value: string | number | boolean | undefined) {
  await updatePreferences({ previewMode: String(value) as DeviceNotificationPreferences["previewMode"] });
}

async function updatePreferences(changes: Partial<DeviceNotificationPreferences>) {
  if (!deviceId.value || saving.value) return;
  saving.value = true;
  try {
    applyEndpoint(await updateNotificationPreferences(deviceId.value, changes));
    ElMessage.success("通知偏好已保存");
  } catch (cause) {
    await load();
    ElMessage.error(cause instanceof Error ? cause.message : String(cause));
  } finally {
    saving.value = false;
  }
}

async function test() {
  if (!deviceId.value || saving.value) return;
  saving.value = true;
  try {
    const result = await testNotificationDevice(deviceId.value);
    if (result.accepted !== true) throw new Error(result.errorMessage || result.errorCode || "通知 Provider 未接受测试请求");
    ElMessage.success(`测试通知已提交到 ${result.provider || endpoint.value?.preferredProvider || "通知通道"}`);
  } catch (cause) {
    ElMessage.error(cause instanceof Error ? cause.message : String(cause));
  } finally {
    saving.value = false;
  }
}

onMounted(load);
</script>

<style scoped>
.panel { display: grid; gap: 16px; }
.head { display: flex; align-items: flex-start; justify-content: flex-end; gap: 16px; }
.actions { display: flex; gap: 10px; flex-wrap: wrap; }
</style>
