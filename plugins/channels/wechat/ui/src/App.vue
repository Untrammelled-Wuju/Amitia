<script setup>
import { computed, onMounted, onUnmounted, ref } from "vue";
import QRCode from "qrcode";
import { createChannelClient, loginCode, statusLabel } from "./session.mjs";

const detail = ref({ status: "idle" });
const ready = ref(false);
const busy = ref("");
const error = ref("");
const qrImage = ref("");
const qrExpired = ref(false);
const confirmation = ref("");
const connected = computed(() => detail.value.status === "connected");
const label = computed(() => qrExpired.value && !connected.value ? "二维码已过期，请重新获取" : statusLabel(detail.value.status));
let invoke;
let disposed = false;
let pollTimer;
let refreshing = false;
let revision = 0;
let qrCreatedAt = 0;

function failure(reason) {
  return reason instanceof Error ? reason.message : String(reason || "操作失败，请重试");
}

function applyStatus(next) {
  detail.value = next;
  if (next.status === "connected") {
    qrImage.value = "";
    qrCreatedAt = 0;
    qrExpired.value = false;
  } else if (qrCreatedAt && (Date.now() - qrCreatedAt > 130000 || ["expired", "error", "idle", "disconnected"].includes(next.status))) {
    qrExpired.value = true;
  }
}

async function refresh(manual = false) {
  if (!invoke || disposed || refreshing || busy.value) return;
  refreshing = true;
  const current = revision;
  if (manual) busy.value = "refresh";
  try {
    const next = await invoke("status");
    if (!disposed && current === revision) {
      applyStatus(next);
      error.value = "";
    }
  } catch (reason) {
    if (!disposed && current === revision) error.value = failure(reason);
  } finally {
    refreshing = false;
    if (manual && !disposed) busy.value = "";
  }
}

function schedulePoll() {
  clearTimeout(pollTimer);
  if (disposed || document.hidden) return;
  pollTimer = setTimeout(async () => {
    await refresh();
    schedulePoll();
  }, invoke?.nextPollDelay(!!qrImage.value && !qrExpired.value) ?? 15000);
}

function onVisibilityChange() {
  clearTimeout(pollTimer);
  if (!document.hidden && !disposed) void refresh().finally(schedulePoll);
}

async function connect(mode = "login") {
  if (!invoke || busy.value) return;
  const current = ++revision;
  busy.value = mode;
  error.value = "";
  confirmation.value = "";
  qrImage.value = "";
  qrExpired.value = false;
  try {
    const config = mode === "rescan" ? { force: true } : mode === "reconnect" ? { reconnect: true } : {};
    const next = await invoke("connect", { config });
    if (disposed || current !== revision) return;
    applyStatus(next);
    if (next.status !== "connected") {
      const code = loginCode(next);
      const svg = code.image ? "" : await QRCode.toString(code.text, { type: "svg", errorCorrectionLevel: "M", margin: 4, width: 280 });
      if (disposed || current !== revision) return;
      qrImage.value = code.image || `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`;
      qrCreatedAt = Date.now();
      detail.value = { ...next, status: next.status === "idle" || !next.status ? "waiting_scan" : next.status };
    }
  } catch (reason) {
    if (!disposed && current === revision) error.value = failure(reason);
  } finally {
    if (!disposed && current === revision) {
      busy.value = "";
      schedulePoll();
    }
  }
}

async function disconnect() {
  if (!invoke || busy.value) return;
  const current = ++revision;
  busy.value = "disconnect";
  confirmation.value = "";
  error.value = "";
  try {
    await invoke("disconnect");
    if (!disposed && current === revision) {
      detail.value = { status: "disconnected" };
      qrImage.value = "";
      qrCreatedAt = 0;
    }
  } catch (reason) {
    if (!disposed && current === revision) error.value = failure(reason);
  } finally {
    if (!disposed && current === revision) busy.value = "";
  }
}

async function initialize() {
  if (busy.value) return;
  error.value = "";
  try {
    const bridge = window.amitiaUI;
    if (typeof bridge?.ready === "function") await bridge.ready();
    if (disposed) return;
    invoke = createChannelClient(bridge);
    ready.value = true;
    await refresh();
    schedulePoll();
  } catch (reason) {
    if (!disposed) error.value = failure(reason);
  }
}

onMounted(() => {
  document.addEventListener("visibilitychange", onVisibilityChange);
  void initialize();
});
onUnmounted(() => {
  disposed = true;
  revision++;
  clearTimeout(pollTimer);
  document.removeEventListener("visibilitychange", onVisibilityChange);
});
</script>

<template>
  <main class="wechat-page" aria-labelledby="page-title">
    <header class="page-heading">
      <div>
        <p class="eyebrow">OPENCLAW · 微信渠道</p>
        <h1 id="page-title">微信连接</h1>
        <p class="subtitle">使用微信扫码，将账号连接到 Amitia。</p>
      </div>
      <span class="connection-badge" :class="{ connected }" role="status">{{ ready ? label : '正在连接插件…' }}</span>
    </header>

    <div v-if="error" class="error-message" role="alert">
      <span>{{ error }}</span>
      <button v-if="!ready" type="button" @click="initialize">重试</button>
      <button v-else type="button" :disabled="!!busy" @click="refresh(true)">重新检查</button>
    </div>

    <section v-if="!ready && !error" class="loading-panel" aria-busy="true">正在检测微信连接状态…</section>

    <section v-if="ready" class="connection-panel" :aria-busy="!!busy">
      <div class="connection-info">
        <h2>{{ connected ? '微信已连接' : '扫码连接微信' }}</h2>
        <p class="section-description">{{ connected ? '微信渠道服务正在运行，可以接收消息并发送角色回复。' : '获取二维码后，使用微信扫一扫，并在手机上确认连接。' }}</p>
        <ol v-if="!connected" class="login-steps">
          <li :class="{ complete: !!qrImage }"><span class="step-number">1</span><div><strong>获取二维码</strong><p>点击下方按钮生成本次登录二维码。</p></div></li>
          <li :class="{ complete: detail.status === 'scanned' || detail.status === 'waiting_confirm' }"><span class="step-number">2</span><div><strong>微信扫码</strong><p>打开微信，扫描右侧二维码。</p></div></li>
          <li><span class="step-number">3</span><div><strong>确认连接</strong><p>在手机上确认后，此页面会自动更新。</p></div></li>
        </ol>
        <dl v-else class="account-details">
          <div v-if="detail.accountId"><dt>微信账号</dt><dd>{{ detail.accountId }}</dd></div>
          <div><dt>接收消息</dt><dd>{{ detail.messageCount ?? 0 }}</dd></div>
          <div><dt>发送回复</dt><dd>{{ detail.replyCount ?? 0 }}</dd></div>
        </dl>
        <div class="actions">
          <button v-if="!connected" class="primary-button" type="button" :disabled="!!busy" @click="connect()">{{ busy === 'login' ? '正在获取…' : qrImage ? '刷新二维码' : '获取二维码' }}</button>
          <button type="button" :disabled="!!busy" @click="refresh(true)">{{ busy === 'refresh' ? '刷新中…' : '刷新状态' }}</button>
          <button v-if="connected" type="button" :disabled="!!busy" @click="connect('reconnect')">{{ busy === 'reconnect' ? '重连中…' : '重新连接' }}</button>
          <button v-if="connected" type="button" :disabled="!!busy" @click="confirmation = 'rescan'">重新扫码</button>
          <button v-if="connected" class="danger-button" type="button" :disabled="!!busy" @click="confirmation = 'disconnect'">断开连接</button>
        </div>
        <div v-if="confirmation" class="confirmation" role="alert">
          <p>{{ confirmation === 'rescan' ? '重新扫码会替换当前微信连接，是否继续？' : '断开后将停止接收和回复微信消息，是否继续？' }}</p>
          <div class="actions"><button class="primary-button" type="button" :disabled="!!busy" @click="confirmation === 'rescan' ? connect('rescan') : disconnect()">确认</button><button type="button" @click="confirmation = ''">取消</button></div>
        </div>
      </div>
      <div v-if="!connected" class="qr-section">
        <div class="qr-frame" :class="{ expired: qrExpired }">
          <img v-if="qrImage" :src="qrImage" alt="微信登录二维码" width="280" height="280" @error="error = '二维码显示失败，请重新获取'" />
          <div v-else class="qr-placeholder"><span class="qr-symbol" aria-hidden="true">▦</span><p>{{ busy === 'login' ? '正在生成二维码…' : '点击「获取二维码」开始连接' }}</p></div>
        </div>
        <p class="qr-caption" role="status">{{ qrImage ? label : '二维码将在这里显示' }}</p>
        <p v-if="qrExpired" class="expiry-hint">请刷新二维码后重新扫描。</p>
      </div>
    </section>

    <aside v-if="ready" class="usage-note">
      <h2>使用提示</h2>
      <p>连接后，先在微信中向机器人发送一条消息，Amitia 才能记录联系人并进行回复或主动推送。</p>
      <p>微信用户标识可能定期更新；主动推送不可用时，请先重新发送一条消息。</p>
    </aside>
  </main>
</template>
