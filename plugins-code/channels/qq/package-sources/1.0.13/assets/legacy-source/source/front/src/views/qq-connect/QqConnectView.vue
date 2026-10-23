<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="wechat-page" :class="{ embedded, compact }">
    <h2 v-if="!embedded" class="page-title">QQ 连接</h2>

    <template v-if="compact">
      <div class="qq-compact-form">
        <h3 class="qq-compact-title">QQBot 配置</h3>
        <el-form label-width="70px" @submit.prevent="doConnect">
          <el-form-item label="AppID">
            <el-input v-model="appId" placeholder="输入Bot AppID" />
          </el-form-item>
          <el-form-item label="Secret">
            <el-input
              v-model="token"
              type="password"
              placeholder="输入Bot Secret"
              show-password
            />
          </el-form-item>
          <el-form-item label="沙箱模式">
            <el-switch v-model="sandbox" />
            <span class="sandbox-label">{{
              sandbox ? "沙箱环境" : "正式环境"
            }}</span>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="connecting" @click="doConnect"
              >连接</el-button
            >
            <span v-if="loginStatus === 'connecting'" class="connecting-text">
              <el-icon class="is-loading"><Loading /></el-icon> 连接中...
            </span>
          </el-form-item>
        </el-form>
      </div>
      <div class="qq-compact-help">
        <h4 class="qq-compact-help-title">使用说明</h4>
        <p>
          1. 前往
          <a href="https://q.qq.com/" target="_blank">QQ开放平台</a> 创建机器人
        </p>
        <p>2. 获取 AppID 和 Secret</p>
        <p>3. 填入上方表单，点击"连接"</p>
        <p>4. 连接成功后，在QQ中 @机器人 即可对话</p>
      </div>
      <div v-if="qqOnline" class="qq-compact-connected">
        <span class="qq-compact-dot" /> 已连接
        <button
          class="qq-compact-disconnect"
          @click="doDisconnect"
          :disabled="disconnecting"
        >
          断开
        </button>
      </div>
    </template>

    <template v-if="!compact">
      <div
        v-if="!pageReady"
        style="padding: 40px 0; color: var(--ac-color-text-muted)"
      >
        <el-icon class="is-loading" :size="24"><Loading /></el-icon>
        <p style="margin-top: 8px">检测连接状态...</p>
      </div>
      <template v-if="pageReady">
        <template v-if="qqOnline">
          <el-alert
            type="success"
            :closable="false"
            show-icon
            style="margin-bottom: 14px"
          >
            <template #title>QQ Bot 已成功连接</template>
          </el-alert>
          <el-card shadow="never" class="section-card">
            <template #header>
              <div class="card-header-row">
                <span class="card-header-title">连接状态</span>
                <div class="header-actions">
                  <el-button
                    size="small"
                    @click="refreshStatus"
                    :loading="loading"
                    >刷新</el-button
                  >
                  <el-button
                    size="small"
                    type="success"
                    @click="doReconnect"
                    :loading="reconnecting"
                    >重新连接</el-button
                  >
                  <el-button
                    size="small"
                    type="danger"
                    @click="doDisconnect"
                    :loading="disconnecting"
                    >断开</el-button
                  >
                </div>
              </div>
            </template>
            <div class="status-main">
              <div class="status-row">
                <div class="status-indicator ok"></div>
                <span class="status-label">已连接</span>
              </div>
              <div class="status-detail-grid">
                <div class="sd-item">
                  <span class="sd-label">消息数</span>
                  <span class="sd-value">{{ statusMessageCount }}</span>
                </div>
                <div class="sd-item">
                  <span class="sd-label">回复数</span>
                  <span class="sd-value">{{ statusReplyCount }}</span>
                </div>
                <div class="sd-item" v-if="accountId">
                  <span class="sd-label">Bot ID</span>
                  <span class="sd-value">{{ accountId.slice(0, 12) }}...</span>
                </div>
                <div class="sd-item">
                  <span class="sd-label">协议</span>
                  <span class="sd-value">QQBot (WebSocket)</span>
                </div>
                <div class="sd-item" v-if="startedAt">
                  <span class="sd-label">连接时间</span>
                  <span class="sd-value">{{ formatStartedAt(startedAt) }}</span>
                </div>
              </div>
            </div>
          </el-card>
        </template>

        <template v-if="!qqOnline">
          <el-card shadow="never" class="section-card">
            <template #header
              ><span class="card-header-title">QQBot 配置</span></template
            >
            <div class="pwd-login">
              <el-form label-width="70px" @submit.prevent="doConnect">
                <el-form-item label="AppID">
                  <el-input
                    v-model="appId"
                    placeholder="输入Bot AppID"
                    style="width: 100%; max-width: 400px"
                  />
                </el-form-item>
                <el-form-item label="Secret">
                  <el-input
                    v-model="token"
                    type="password"
                    placeholder="输入Bot Secret"
                    style="width: 100%; max-width: 400px"
                    show-password
                  />
                </el-form-item>
                <el-form-item label="沙箱模式">
                  <el-switch v-model="sandbox" />
                  <span
                    style="
                      margin-left: 8px;
                      font-size: 12px;
                      color: var(--ac-color-text-muted);
                    "
                    >{{ sandbox ? "沙箱环境" : "正式环境" }}</span
                  >
                </el-form-item>
                <el-form-item>
                  <el-button
                    type="primary"
                    :loading="connecting"
                    @click="doConnect"
                    >连接</el-button
                  >
                  <span
                    v-if="loginStatus === 'connecting'"
                    style="
                      margin-left: 10px;
                      font-size: 12px;
                      color: var(--ac-color-warning);
                    "
                  >
                    <el-icon class="is-loading"><Loading /></el-icon> 连接中...
                  </span>
                </el-form-item>
              </el-form>
            </div>
          </el-card>

          <el-card shadow="never" class="section-card" style="margin-top: 12px">
            <template #header
              ><span class="card-header-title">使用说明</span></template
            >
            <div
              style="
                font-size: 13px;
                color: var(--ac-color-text-secondary);
                line-height: 1.8;
              "
            >
              <p>
                1. 前往
                <a
                  href="https://q.qq.com/"
                  target="_blank"
                  style="color: var(--ac-color-primary)"
                  >QQ开放平台</a
                >
                创建机器人
              </p>
              <p>2. 获取 AppID 和 Secret</p>
              <p>3. 填入上方表单，点击"连接"</p>
              <p>4. 连接成功后，在QQ中 @机器人 即可对话</p>
            </div>
          </el-card>
        </template>
      </template>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, inject } from "vue";
import { Loading } from "@element-plus/icons-vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { getQQApiBaseURL } from "../../runtime/runtime-adapter";
import { createAuthenticatedFetchInit } from "../../runtime/request-auth";


const props = withDefaults(
  defineProps<{ embedded?: boolean; compact?: boolean }>(),
  {
    embedded: false,
    compact: false,
  },
);

const emit = defineEmits<{
  connectionChanged: [connected: boolean];
}>();

const refreshHealth = inject<() => Promise<void>>("refreshHealth");

const qqApiBaseUrl = ref("");

const pageReady = ref(false);
const loading = ref(false);
const qqOnline = ref(false);
const accountId = ref<string | null>(null);
const loginStatus = ref("");
const connecting = ref(false);
const disconnecting = ref(false);
const reconnecting = ref(false);
const statusMessageCount = ref(0);
const statusReplyCount = ref(0);
const startedAt = ref("");

const appId = ref("");
const token = ref("");
const sandbox = ref(false);

let pollTimer: ReturnType<typeof setInterval> | null = null;

let connectPollTimer: ReturnType<typeof setInterval> | null = null;

let lastShownError: string = "";

function stopConnectPoll() {
  if (connectPollTimer) {
    clearInterval(connectPollTimer);
    connectPollTimer = null;
  }
}

async function ensureApiBaseUrl() {
  if (!qqApiBaseUrl.value) {
    qqApiBaseUrl.value = await getQQApiBaseURL();
  }
}

async function qqRequest<T>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  await ensureApiBaseUrl();
  const requestPath = `/api/qq${path.startsWith("/") ? path : `/${path}`}`;
  const authenticated = await createAuthenticatedFetchInit(requestPath, init);
  const headers = new Headers(authenticated.headers ?? undefined);
  if (typeof authenticated.body === "string" && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  const response = await fetch(`${qqApiBaseUrl.value}${path}`, {
    ...authenticated,
    headers,
  });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(body?.error || body?.message || `HTTP ${response.status}`);
  }
  return (body?.data ?? body) as T;
}

function formatStartedAt(iso: string): string {
  if (!iso) return "";
  try {
    const d = new Date(iso);
    const pad = (n: number) => String(n).padStart(2, "0");
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
  } catch {
    return iso;
  }
}

async function doConnect() {
  if (!appId.value || !token.value) return;
  await ensureApiBaseUrl();
  connecting.value = true;
  loginStatus.value = "";
  try {
    await qqRequest("/connect", {
      method: "POST",
      body: JSON.stringify({
        appId: appId.value,
        token: token.value,
        sandbox: sandbox.value,
      }),
    });
    loginStatus.value = "connecting";
    stopConnectPoll();
    const startTime = Date.now();
    connectPollTimer = setInterval(async () => {
      await refreshStatus();
      if (qqOnline.value) {
        stopConnectPoll();
        connecting.value = false;
        loginStatus.value = "";
        ElMessage.success("QQ Bot 连接成功");
        refreshHealth?.();
        return;
      }
      if (Date.now() - startTime > 30000) {
        stopConnectPoll();
        connecting.value = false;
        loginStatus.value = "";
        ElMessage.error("连接超时，请检查AppID和Secret是否有效");
      }
    }, 2000);
  } catch (e: any) {
    const msg = e?.message || "连接失败，请检查AppID和Secret";
    ElMessage.error(msg);
    connecting.value = false;
  }
}

async function doReconnect() {
  await ensureApiBaseUrl();
  reconnecting.value = true;
  try {
    const cfg = await qqRequest<any>("/config");
    if (!cfg?.appId || !cfg?.token) {
      ElMessage.warning("未找到已保存的凭证，请手动重新连接");
      reconnecting.value = false;
      return;
    }
    await qqRequest("/disconnect", { method: "POST" });
    await new Promise((r) => setTimeout(r, 1000));
    await qqRequest("/connect", {
      method: "POST",
      body: JSON.stringify({
        appId: cfg.appId,
        token: cfg.token,
        sandbox: cfg.sandbox || false,
      }),
    });
    loginStatus.value = "connecting";
    stopConnectPoll();
    const startTime = Date.now();
    connectPollTimer = setInterval(async () => {
      await refreshStatus();
      if (qqOnline.value) {
        stopConnectPoll();
        reconnecting.value = false;
        loginStatus.value = "";
        ElMessage.success("QQ Bot 重新连接成功");
        refreshHealth?.();
        return;
      }
      if (Date.now() - startTime > 30000) {
        stopConnectPoll();
        reconnecting.value = false;
        loginStatus.value = "";
        ElMessage.error("重新连接超时");
      }
    }, 2000);
  } catch (e: any) {
    ElMessage.error("重新连接失败");
    reconnecting.value = false;
  }
}

async function doDisconnect() {
  await ensureApiBaseUrl();
  disconnecting.value = true;
  try {
    await qqRequest("/disconnect", { method: "POST" });
    qqOnline.value = false;
    accountId.value = null;
    startedAt.value = "";
    loginStatus.value = "";
    emit("connectionChanged", false);
    refreshHealth?.();
  } catch (e: any) {}
  disconnecting.value = false;
}

async function refreshStatus() {
  await ensureApiBaseUrl();
  try {
    const data = await qqRequest<any>("/status");
    qqOnline.value = !!data?.qqOnline;
    emit("connectionChanged", qqOnline.value);
    accountId.value = data?.accountId || null;
    loginStatus.value = data?.status || "";
    startedAt.value = data?.startedAt || "";
    statusMessageCount.value = data?.messageCount ?? 0;
    statusReplyCount.value = data?.replyCount ?? 0;

    if (qqOnline.value && loginStatus.value !== "connecting") {
      if (loginStatus.value === "connecting") {
        ElMessage.success("QQ Bot 连接成功");
      }
      loginStatus.value = "";
    }
    const err = data?.error || "";
    if (
      err &&
      loginStatus.value !== "connecting" &&
      loginStatus.value !== "online"
    ) {
      if (err !== lastShownError) {
        lastShownError = err;
        ElMessage.error(err);
      }
    }
    if (!err) {
      lastShownError = "";
    }
    if (!qqOnline.value) {
      try {
        const cfg = await qqRequest<any>("/config");
        if (cfg?.appId) {
          appId.value = cfg.appId;
          token.value = cfg.token || "";
          sandbox.value = cfg.sandbox || false;
        }
      } catch {}
    }
  } catch {
    qqOnline.value = false;
    emit("connectionChanged", false);
  } finally {
    pageReady.value = true;
  }
}

onMounted(async () => {
  if (!props.compact) {
    qqApiBaseUrl.value = await getQQApiBaseURL();
    await refreshStatus();
    pollTimer = setInterval(refreshStatus, 3000);
  }
});

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer);
  stopConnectPoll();
});
</script>

<style scoped>
.wechat-page {
  margin: 0;
}
.page-title {
  font-size: 24px;
  font-weight: 700;
  color: var(--ac-color-text);
  margin-bottom: 16px;
}
.section-card {
  margin-bottom: 16px;
}
.card-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.header-actions {
  display: flex;
  gap: 8px;
}
.card-header-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--ac-color-text);
}
.status-main {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.status-row {
  display: flex;
  align-items: center;
  gap: 10px;
}
.status-indicator {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  flex-shrink: 0;
}
.status-indicator.ok {
  background: var(--ac-color-success);
}
.status-label {
  font-size: 16px;
  font-weight: 600;
  color: var(--ac-color-text);
}
.status-detail-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
  gap: 8px;
}
.sd-item {
  padding: 8px 12px;
  background: var(--ac-color-bg-secondary);
  border-radius: 4px;
}
.sd-label {
  font-size: 11px;
  color: var(--ac-color-text-muted);
  display: block;
}
.sd-value {
  font-size: 14px;
  font-weight: 600;
  color: var(--ac-color-text);
}
.pwd-login {
  padding: 8px 0;
}

.qq-compact-form {
  padding: 0;
}
.qq-compact-title {
  font-size: 14px;
  font-weight: 650;
  color: var(--ac-color-text);
  margin: 0 0 12px;
}
.qq-compact-form :deep(.el-form-item) {
  margin-bottom: 7px;
}
.qq-compact-form :deep(.el-form-item__label) {
  font-size: 12px;
  color: var(--ac-color-text-secondary);
}
.qq-compact-form :deep(.el-input__inner) {
  height: 30px;
  font-size: 13px;
}
.sandbox-label {
  margin-left: 8px;
  font-size: 11px;
  color: var(--ac-color-text-muted);
}
.connecting-text {
  margin-left: 8px;
  font-size: 11px;
  color: var(--ac-color-warning);
}

.qq-compact-help {
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px solid var(--ac-color-border);
}
.qq-compact-help-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--ac-color-text);
  margin: 0 0 4px;
}
.qq-compact-help p {
  font-size: 11px;
  color: var(--ac-color-text-secondary);
  line-height: 1.6;
  margin: 0;
}
.qq-compact-help a {
  color: var(--ac-color-primary);
}

.qq-compact-connected {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px solid var(--ac-color-border);
  font-size: 12px;
  color: var(--ac-color-success);
}
.qq-compact-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--ac-color-success);
  flex-shrink: 0;
}
.qq-compact-disconnect {
  margin-left: auto;
  font-size: 10px;
  padding: 2px 8px;
  border: 1px solid var(--ac-color-border);
  border-radius: 4px;
  background: transparent;
  color: var(--ac-color-text-muted);
  cursor: pointer;
}
.qq-compact-disconnect:hover {
  color: var(--ac-color-danger);
  border-color: var(--ac-color-danger);
}
</style>
