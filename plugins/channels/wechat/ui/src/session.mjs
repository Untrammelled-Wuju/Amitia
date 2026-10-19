export function unwrapResult(result) {
  let value = result;
  for (let depth = 0; depth < 6; depth++) {
    if (!value || typeof value !== "object") break;
    if (value.success === false || value.ok === false || (typeof value.code === "number" && value.code !== 0 && value.code !== 200)) {
      throw new Error(value.error?.message || value.error || value.message || value.msg || "微信服务请求失败");
    }
    if (!("data" in value) || value.data == null) break;
    value = value.data;
  }
  return value ?? {};
}

export function loginCode(status) {
  const value = unwrapResult(status);
  const image = value.qrImageUrl || value.qrCodeUrl || "";
  if (/^data:image\/(?:png|jpeg|webp|svg\+xml)[;,]/i.test(image)) return { image, text: "" };
  const text = String(value.qrCodeUrl || value.qrContent || value.qrcode || image).trim();
  if (!text) throw new Error(value.message || value.lastError || "服务未返回二维码，请重新获取");
  return { image: "", text };
}

export function statusLabel(status) {
  return {
    connected: "已连接",
    waiting_scan: "等待微信扫码",
    qr_ready: "等待微信扫码",
    waiting_confirm: "请在微信中确认",
    scanned: "请在微信中确认",
    connecting: "连接中",
    disconnected: "未连接",
    expired: "二维码已过期",
    error: "连接异常",
    idle: "未连接",
  }[status] || "未连接";
}

export function createChannelClient(bridge, { now = Date.now } = {}) {
  if (!bridge || typeof bridge.invokeAction !== "function") throw new Error("插件连接尚未就绪，请关闭页面后重新打开");
  const calls = [];
  let blockedUntil = 0;
  let lastStatusAt = -Infinity;
  let lastStatus;
  let pendingStatus;
  function prune() {
    while (calls.length && calls[0] <= now() - 60000) calls.shift();
  }
  function limited() {
    const error = new Error("插件操作较频繁，请稍后再试，状态将自动恢复刷新");
    error.retryAfterMs = Math.max(1000, blockedUntil - now());
    return error;
  }
  async function request(action, input) {
    prune();
    if (now() < blockedUntil) throw limited();
    if (calls.length >= 24) {
      blockedUntil = calls[0] + 60000;
      throw limited();
    }
    calls.push(now());
    try {
      const result = unwrapResult(await bridge.invokeAction(action, { channelId: "wechat", ...input }));
      if (action === "status") {
        lastStatus = result;
        lastStatusAt = now();
      } else {
        lastStatusAt = -Infinity;
      }
      return result;
    } catch (error) {
      if (/rate.?limit|429/i.test(String(error?.message || error))) {
        blockedUntil = now() + 60000;
        throw limited();
      }
      throw error;
    }
  }
  const client = (action, input = {}) => {
    if (action !== "status") return request(action, input);
    if (pendingStatus) return pendingStatus;
    prune();
    if (calls.length >= 18 && lastStatus && now() >= blockedUntil) return Promise.resolve(lastStatus);
    if (lastStatus && now() - lastStatusAt < 5000) return Promise.resolve(lastStatus);
    pendingStatus = request(action, input).finally(() => { pendingStatus = undefined; });
    return pendingStatus;
  };
  client.nextPollDelay = (active) => {
    prune();
    return Math.max(active ? 5000 : 15000, blockedUntil - now(), calls.length >= 18 ? calls[0] + 60000 - now() : 0);
  };
  return client;
}
