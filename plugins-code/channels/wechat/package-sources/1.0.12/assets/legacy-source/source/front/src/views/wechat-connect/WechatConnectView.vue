<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="wechat-page" :class="{ embedded, compact }">
    <h2 v-if="!embedded" class="page-title">微信连接</h2>

    <template v-if="compact">
      <div class="wechat-compact-form">
        <h3 class="wechat-compact-title">微信连接</h3>
        <div v-if="!pageReady" class="wechat-compact-loading">
          <el-icon class="is-loading" :size="18"><Loading /></el-icon>
          <span>检测中...</span>
        </div>
        <template v-else>
          <div v-if="!isConnected" class="wechat-compact-start">
            <div class="wechat-compact-qr-wrap">
              <img
                v-if="qrCodeUrl"
                :src="qrCodeUrl"
                class="wechat-compact-qr"
                alt="QR"
              />
              <div v-else class="wechat-compact-qr-placeholder">
                <el-icon :size="28"><Picture /></el-icon>
                <span>点击下方按钮获取</span>
              </div>
              <p class="wechat-compact-scan-hint">
                <el-icon class="is-loading" v-if="scanning"
                  ><Loading
                /></el-icon>
                {{
                  scanning ? "等待扫码中..." : qrCodeUrl ? "请用微信扫码" : ""
                }}
              </p>
              <p class="wechat-compact-login-err" v-if="loginError">
                {{ loginError }}
              </p>
            </div>
            <el-button
              type="primary"
              size="small"
              :loading="qrLoading"
              @click="startLogin"
              style="width: 100%"
            >
              获取二维码
            </el-button>
            <div v-if="qrCodeUrl" class="wechat-compact-extra-actions">
              <el-button size="small" :loading="qrLoading" @click="startLogin"
                >刷新二维码</el-button
              >
              <el-button
                size="small"
                type="success"
                :loading="reconnecting"
                @click="reconnectBot"
                >重新连接</el-button
              >
            </div>
          </div>
          <div v-if="isConnected" class="wechat-compact-connected">
            <span class="wechat-compact-dot" /> 已连接
            <button
              class="wechat-compact-disconnect"
              @click="handleRescan"
              :disabled="qrLoading"
            >
              重新扫码
            </button>
          </div>
        </template>
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
        <el-alert
          type="info"
          :closable="false"
          show-icon
          style="margin-bottom: 14px"
        >
          <template #title> 扫码连接你的微信 </template>
        </el-alert>

        <template v-if="!isConnected">
          <el-card shadow="never" class="section-card">
            <template #header
              ><span class="card-header-title">扫码连接</span></template
            >

            <div class="login-layout">
              <div class="login-steps">
                <div class="qr-step-row">
                  <div class="qr-step-num" :class="{ active: qrStep >= 0 }">
                    1
                  </div>
                  <div class="qr-step-body">
                    <span class="qr-step-title">生成二维码</span>
                    <el-button
                      size="small"
                      type="primary"
                      :loading="qrLoading"
                      @click="startLogin"
                      :disabled="qrLoading || isConnected"
                      >获取二维码</el-button
                    >
                  </div>
                </div>
                <div class="qr-step-row">
                  <div class="qr-step-num" :class="{ active: qrStep >= 1 }">
                    2
                  </div>
                  <div class="qr-step-body">
                    <span class="qr-step-title">用微信扫码</span>
                    <span v-if="qrStep >= 1 && !isConnected" class="qr-status">
                      <el-icon class="is-loading" v-if="scanning"
                        ><Loading
                      /></el-icon>
                      {{ scanning ? "等待扫码中..." : "请扫描二维码" }}
                    </span>
                  </div>
                </div>
                <div class="qr-step-row">
                  <div class="qr-step-num" :class="{ active: isConnected }">
                    3
                  </div>
                  <div class="qr-step-body">
                    <span class="qr-step-title">确认连接</span>
                    <span v-if="isConnected" class="qr-done">
                      <el-icon><CircleCheckFilled /></el-icon> 已连接
                    </span>
                  </div>
                </div>
              </div>

              <div class="login-qr">
                <div class="qr-frame" v-if="qrCodeUrl">
                  <img :src="qrCodeUrl" alt="二维码" />
                </div>
                <div class="qr-frame qr-empty" v-else>
                  <el-icon :size="36"><Picture /></el-icon>
                  <span>点击按钮获取二维码</span>
                </div>
              </div>
            </div>

            <div class="qr-tip" v-if="qrCodeUrl">
              打开微信，扫描二维码确认连接。
            </div>
          </el-card>
        </template>

        <el-alert
          v-if="!isConnected && !compact"
          type="warning"
          :closable="false"
          show-icon
          style="margin-bottom: 14px"
        >
          <template #title>主动推送须知</template>
          扫码连接后，添加微信好友必须主动给机器人发一条消息，系统才能记录你的用户ID用于主动推送。用户ID每7天自动刷新，届时需重新发送一条消息。
        </el-alert>

        <template v-if="isConnected">
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
                    @click="reconnectBot"
                    :loading="reconnecting"
                    >重新连接</el-button
                  >
                  <el-button
                    size="small"
                    type="warning"
                    @click="handleRescan"
                    :loading="qrLoading"
                  >
                    重新添加机器人
                  </el-button>
                </div>
              </div>
            </template>
            <div class="status-main">
              <div class="status-row">
                <div class="status-indicator ok"></div>
                <span class="status-label">已连接</span>
              </div>
              <div class="status-detail-grid" v-if="detail">
                <div class="sd-item">
                  <span class="sd-label">消息数</span>
                  <span class="sd-value">{{ detail?.messageCount ?? 0 }}</span>
                </div>
                <div class="sd-item">
                  <span class="sd-label">回复数</span>
                  <span class="sd-value">{{ detail?.replyCount ?? 0 }}</span>
                </div>
                <div class="sd-item" v-if="detail.accountId">
                  <span class="sd-label">账号</span>
                  <span class="sd-value"
                    >{{ detail.accountId.slice(0, 12) }}...</span
                  >
                </div>
                <div class="sd-item">
                  <span class="sd-label">模式</span>
                  <span class="sd-value">OpenClaw</span>
                </div>
                <div class="sd-item" v-if="detail.startedAt">
                  <span class="sd-label">连接时间</span>
                  <span class="sd-value">{{
                    formatStartedAt(detail.startedAt)
                  }}</span>
                </div>
              </div>
            </div>
          </el-card>
        </template>

        <el-alert
          v-if="isConnected && !compact"
          type="warning"
          :closable="false"
          show-icon
          style="margin-bottom: 14px"
        >
          <template #title>主动推送须知</template>
          添加微信好友后，必须主动给机器人发一条消息，系统才能记录你的用户ID用于主动推送。用户ID每7天自动刷新，届时需重新发送一条消息。
        </el-alert>

        <div v-if="!compact" class="ops-grid">
          <el-card shadow="never" class="section-card">
            <template #header>
              <div class="card-header-row">
                <span class="card-header-title">Bridge 恢复与最近事件</span>
                <div class="header-actions">
                  <el-button size="small" :loading="bridgeRecovering" @click="recoverBridge">Bridge Recover</el-button>
                  <el-button size="small" :loading="opsLoading" @click="loadWechatOps">刷新事件</el-button>
                </div>
              </div>
            </template>
            <el-empty v-if="!wechatEvents.length" description="暂无最近事件" :image-size="42" />
            <div v-else class="ops-event-list">
              <div v-for="(event, idx) in wechatEvents.slice(0, 12)" :key="event.id || event.eventId || idx" class="ops-event-item">
                <div class="ops-event-title">{{ event.type || event.eventType || event.name || 'event' }}</div>
                <div class="ops-event-meta">{{ event.status || event.state || '' }} {{ formatOpsTime(event.createdAt || event.timestamp || event.time) }}</div>
                <pre>{{ compactJson(event) }}</pre>
              </div>
            </div>
          </el-card>

          <el-card shadow="never" class="section-card">
            <template #header>
              <div class="card-header-row">
                <span class="card-header-title">Cloud Check 风险摘要</span>
                <div class="header-actions">
                  <el-button size="small" type="primary" :loading="cloudChecking" @click="runCloudCheck">主动检查</el-button>
                  <el-button size="small" :loading="opsLoading" @click="loadWechatOps">刷新摘要</el-button>
                </div>
              </div>
            </template>
            <el-empty v-if="!cloudRiskSummary" description="暂无 Cloud Check 结果" :image-size="42" />
            <pre v-else class="ops-json">{{ prettyJson(cloudRiskSummary) }}</pre>
          </el-card>
        </div>
      </template>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, inject } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  CircleCheckFilled,
  Picture,
  Loading,
  InfoFilled,
  Warning,
} from "@element-plus/icons-vue";
import { useApi } from "../../composables/useApi";

const { get, post } = useApi();

withDefaults(defineProps<{ embedded?: boolean; compact?: boolean }>(), {
  embedded: false,
  compact: false,
});

const emit = defineEmits<{
  connectionChanged: [connected: boolean];
}>();

const refreshHealth = inject<() => Promise<void>>("refreshHealth");

const detail = ref<any>(null);
const loading = ref(false);
const qrLoading = ref(false);
const qrCodeUrl = ref("");
const qrStep = ref(0);
const scanning = ref(false);
const loginError = ref("");

const pageReady = ref(false);

const reconnecting = ref(false);
const bridgeRecovering = ref(false);
const cloudChecking = ref(false);
const opsLoading = ref(false);
const wechatEvents = ref<any[]>([]);
const cloudRiskSummary = ref<any>(null);

const isConnected = computed(() => detail.value?.status === "connected");

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

async function refreshStatus() {
  loading.value = true;
  try {
    const resp = await get<any>("/api/wechat/status");
    detail.value = resp?.data || resp;
    emit("connectionChanged", detail.value?.status === "connected");
  } catch (err: any) {
    if (err?.message && !err.message.includes("404")) {
      console.warn("WeChat status fetch failed:", err.message);
    }
  } finally {
    loading.value = false;
  }

  pageReady.value = true;
}

async function startLogin() {
  stopPolling();
  qrStep.value = 0;
  scanning.value = false;
  qrCodeUrl.value = "";
  qrLoading.value = true;
  loginError.value = "";
  try {
    const resp = await get<any>("/api/wechat/login/start");
    if (resp?.data?.status === "connected" || resp?.status === "connected") {
      await refreshStatus();
      ElMessage.success("已连接微信");
      refreshHealth?.();
      return;
    }
    const imgUrl =
      resp?.data?.qrImageUrl ||
      resp?.qrImageUrl ||
      resp?.data?.qrCodeUrl ||
      resp?.qrCodeUrl;
    if (imgUrl) {
      qrCodeUrl.value = imgUrl;
      qrStep.value = 1;
      scanning.value = true;
      ElMessage.success("二维码已生成，请用微信扫码");
      startPolling();
    } else {
      const msg = resp?.message || resp?.data?.message || "获取二维码失败";
      loginError.value = msg;
      ElMessage.warning(msg);
    }
  } catch (err: any) {
    loginError.value = err.message || "获取二维码失败";
    ElMessage.error(loginError.value);
  } finally {
    qrLoading.value = false;
  }
}

async function handleRescan() {
  try {
    await ElMessageBox.confirm(
      "重新添加将断开当前连接并生成新的二维码，确定要继续吗？",
      "确认操作",
      {
        confirmButtonText: "确定",
        cancelButtonText: "取消",
        type: "warning",
      },
    );
    await startRescan();
  } catch {
    // 用户取消
  }
}

async function startRescan() {
  stopPolling();
  qrLoading.value = true;
  loginError.value = "";
  qrCodeUrl.value = "";
  qrStep.value = 0;
  scanning.value = false;
  try {
    const resp = await post<any>("/api/wechat/login/rescan");
    const imgUrl =
      resp?.data?.qrImageUrl ||
      resp?.qrImageUrl ||
      resp?.data?.qrCodeUrl ||
      resp?.qrCodeUrl;
    if (imgUrl) {
      detail.value = { status: "waiting_scan" };
      qrCodeUrl.value = imgUrl;
      qrStep.value = 1;
      scanning.value = true;
      ElMessage.success("已生成新机器人的二维码，请用微信扫码添加");
      startPolling();
    } else {
      const msg = resp?.message || resp?.data?.message || "获取二维码失败";
      loginError.value = msg;
      ElMessage.warning(msg);
      await refreshStatus();
    }
  } catch (err: any) {
    loginError.value = err.message || "获取二维码失败";
    ElMessage.error(loginError.value);
    await refreshStatus();
  } finally {
    qrLoading.value = false;
  }
}

async function reconnectBot() {
  reconnecting.value = true;
  try {
    await post<any>("/api/wechat/login/reconnect");
    await refreshStatus();
    ElMessage.success("已重新连接");
    refreshHealth?.();
  } catch (err: any) {
    ElMessage.error(err?.message || "重新连接失败");
  } finally {
    reconnecting.value = false;
  }
}

function unwrap<T = any>(resp: any): T {
  return (resp?.data ?? resp) as T;
}

function normalizeEventList(raw: any): any[] {
  const value = unwrap<any>(raw);
  if (Array.isArray(value)) return value;
  if (Array.isArray(value?.items)) return value.items;
  if (Array.isArray(value?.events)) return value.events;
  if (Array.isArray(value?.data)) return value.data;
  return [];
}

function prettyJson(value: any): string {
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value ?? "");
  }
}

function compactJson(value: any): string {
  const text = prettyJson(value);
  return text.length > 900 ? text.slice(0, 900) + "\n…" : text;
}

function formatOpsTime(value: any): string {
  if (!value) return "";
  try {
    return new Date(value).toLocaleString();
  } catch {
    return String(value);
  }
}

async function loadWechatOps() {
  opsLoading.value = true;
  try {
    const [eventsResp, bridgeEventsResp, riskResp] = await Promise.allSettled([
      get<any>("/api/wechat/events"),
      get<any>("/api/wechat/bridge/events"),
      get<any>("/api/wechat/cloud-check/risk-summary"),
    ]);
    const merged: any[] = [];
    if (eventsResp.status === "fulfilled") merged.push(...normalizeEventList(eventsResp.value));
    if (bridgeEventsResp.status === "fulfilled") merged.push(...normalizeEventList(bridgeEventsResp.value));
    wechatEvents.value = merged
      .filter((item, index, all) => {
        const key = item?.id || item?.eventId;
        return !key || all.findIndex((other) => (other?.id || other?.eventId) === key) === index;
      })
      .slice(0, 30);
    if (riskResp.status === "fulfilled") {
      cloudRiskSummary.value = unwrap(riskResp.value);
    }
  } finally {
    opsLoading.value = false;
  }
}

async function recoverBridge() {
  bridgeRecovering.value = true;
  try {
    const result = unwrap<any>(await post<any>("/api/wechat/bridge/recover"));
    ElMessage.success(result?.message || "Bridge 恢复操作已执行");
    await Promise.all([refreshStatus(), loadWechatOps()]);
  } catch (err: any) {
    ElMessage.error(err?.message || "Bridge Recover 失败");
  } finally {
    bridgeRecovering.value = false;
  }
}

async function runCloudCheck() {
  cloudChecking.value = true;
  try {
    const result = unwrap<any>(await post<any>("/api/wechat/cloud-check/run"));
    ElMessage.success(result?.message || "Cloud Check 已完成");
    await loadWechatOps();
  } catch (err: any) {
    ElMessage.error(err?.message || "Cloud Check 失败");
  } finally {
    cloudChecking.value = false;
  }
}

let pollTimer: ReturnType<typeof setInterval> | null = null;

function startPolling() {
  stopPolling();
  const startTime = Date.now();
  const maxWait = 130000;
  pollTimer = setInterval(async () => {
    if (Date.now() - startTime > maxWait) {
      stopPolling();
      scanning.value = false;
      qrStep.value = 0;
      ElMessage.warning("扫码超时，请重新获取二维码");
      return;
    }
    try {
      await refreshStatus();
      if (detail.value?.status === "connected") {
        stopPolling();
        qrStep.value = 3;
        scanning.value = false;
        ElMessage.success("已连接微信！");
        refreshHealth?.();
      }
    } catch {
      /* keep polling */
    }
  }, 2000);
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer);
    pollTimer = null;
  }
}

let statusTimer: ReturnType<typeof setInterval> | null = null;

onMounted(async () => {
  await Promise.allSettled([refreshStatus(), loadWechatOps()]);
  statusTimer = setInterval(() => {
    refreshStatus();
  }, 10000);
});

onUnmounted(() => {
  if (statusTimer) {
    clearInterval(statusTimer);
    statusTimer = null;
  }
  stopPolling();
});
</script>

<style scoped>
.wechat-page {
  margin: 0;
}
.page-title {
  font-size: 24px;
  font-weight: 600;
  margin-bottom: 14px;
  color: var(--ac-color-text);
}
.section-card {
  margin-bottom: 12px;
}
.card-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
.card-header-title {
  font-weight: 600;
  font-size: var(--ac-font-size-sm);
}

.login-layout {
  display: flex;
  gap: 28px;
  align-items: flex-start;
}
@media (max-width: 560px) {
  .login-layout {
    flex-direction: column-reverse;
    align-items: center;
  }
}

.login-steps {
  max-width: 300px;
  flex-shrink: 0;
}
.login-qr {
  flex-shrink: 0;
}

.qr-frame {
  width: 200px;
  height: 200px;
  border: 1px solid var(--ac-color-border);
  border-radius: 6px;
  overflow: hidden;
  background: var(--ac-color-surface);
}
.qr-frame img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  display: block;
}
.qr-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: var(--ac-color-text-muted);
  font-size: 12px;
}

.qr-tip {
  margin-top: 14px;
  font-size: 12px;
  color: var(--ac-color-text-muted);
  text-align: left;
}

.qr-step-row {
  display: flex;
  gap: 10px;
  align-items: center;
  padding: 12px 0;
  border-bottom: 1px solid var(--ac-color-border-light);
}
.qr-step-row:last-child {
  border-bottom: none;
}
.qr-step-num {
  color: var(--ac-color-text-secondary);
  width: 24px;
  height: 24px;
  border-radius: 50%;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  font-weight: 600;
  background: var(--ac-color-bg-secondary);
  color: var(--ac-color-text-muted);
  border: 2px solid var(--ac-color-border);
}
.qr-step-num.active {
  background: var(--ac-color-primary);
  color: var(--ac-color-text-on-primary);
  border-color: var(--ac-color-primary);
}
.qr-step-body {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 10px;
}
.qr-step-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--ac-color-text);
}
.qr-status {
  font-size: 12px;
  color: var(--ac-color-text-secondary);
  display: flex;
  align-items: center;
  gap: 4px;
}
.qr-done {
  font-size: 12px;
  color: var(--ac-color-success);
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 4px;
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

.wechat-compact-form {
  padding: 0;
}
.wechat-compact-title {
  font-size: 14px;
  font-weight: 650;
  color: var(--ac-color-text);
  margin: 0 0 12px;
}
.wechat-compact-loading {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--ac-color-text-muted);
  font-size: 12px;
  padding: 20px 0;
  justify-content: center;
}
.wechat-compact-start {
  padding: 4px 0 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.wechat-compact-qr-wrap {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}
.wechat-compact-qr {
  width: 160px;
  height: 160px;
  border: 1px solid rgba(200, 121, 91, 0.18);
  border-radius: 6px;
  object-fit: contain;
  background: #fff;
}
.wechat-compact-qr-placeholder {
  width: 160px;
  height: 160px;
  border: 1px dashed rgba(200, 121, 91, 0.25);
  border-radius: 6px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
  color: var(--ac-color-text-muted);
  font-size: 10px;
}
.wechat-compact-scan-hint {
  font-size: 11px;
  color: var(--ac-color-text-secondary);
  display: flex;
  align-items: center;
  gap: 4px;
  margin: 0;
  min-height: 16px;
}
.wechat-compact-login-err {
  font-size: 11px;
  color: var(--ac-color-danger);
  margin: 0;
}
.wechat-compact-extra-actions {
  display: flex;
  gap: 8px;
  padding-top: 4px;
}
.wechat-compact-connected {
  display: flex;
  align-items: center;
  gap: 6px;
  padding-top: 8px;
  border-top: 1px solid var(--ac-color-border);
  font-size: 12px;
  color: var(--ac-color-success);
}
.wechat-compact-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--ac-color-success);
  flex-shrink: 0;
}
.wechat-compact-disconnect {
  margin-left: auto;
  font-size: 10px;
  padding: 2px 8px;
  border: 1px solid var(--ac-color-border);
  border-radius: 4px;
  background: transparent;
  color: var(--ac-color-text-muted);
  cursor: pointer;
}
.wechat-compact-disconnect:hover {
  color: var(--ac-color-danger);
  border-color: var(--ac-color-danger);
}

.ops-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 12px;
}
.ops-event-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 420px;
  overflow: auto;
}
.ops-event-item {
  padding: 9px 10px;
  border: 1px solid var(--ac-color-border);
  border-radius: 6px;
  background: var(--ac-color-bg-secondary);
}
.ops-event-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--ac-color-text);
}
.ops-event-meta {
  margin-top: 2px;
  font-size: 11px;
  color: var(--ac-color-text-muted);
}
.ops-event-item pre,
.ops-json {
  margin: 7px 0 0;
  white-space: pre-wrap;
  word-break: break-word;
  font: 11px/1.45 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  color: var(--ac-color-text-secondary);
}
.ops-json {
  max-height: 420px;
  overflow: auto;
  padding: 10px;
  border-radius: 6px;
  background: var(--ac-color-bg-secondary);
}
</style>
