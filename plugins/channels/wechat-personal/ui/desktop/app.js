const bridge = window.amitiaUI;
const channelId = "wechat_personal";
const $ = (id) => document.getElementById(id);
let currentConversation = "";
let loginWatchToken = 0;

function unwrap(value) {
  if (value && typeof value === "object" && "data" in value && Object.keys(value).length <= 3) return value.data ?? value;
  return value;
}
async function action(name, input = {}) {
  if (!bridge?.invokeAction) throw new Error("Amitia UI Bridge 不可用");
  return unwrap(await bridge.invokeAction(name, { channelId, ...input }));
}
function text(id, value) { $(id).textContent = value == null || value === "" ? "—" : String(value); }
function show(id, visible) { $(id).classList.toggle("hidden", !visible); }
function setError(value) { text("error", value); show("error", Boolean(value)); }
function setBusy(busy) {
  const button = $("connect");
  button.disabled = busy;
  button.setAttribute("aria-busy", String(busy));
  button.textContent = busy ? "正在获取..." : "获取登录二维码";
  $("refresh").disabled = busy;
}
function capability(id, enabled) {
  const node = $(id);
  node.classList.toggle("enabled", Boolean(enabled));
  node.classList.toggle("disabled", !enabled);
  node.setAttribute("aria-label", `${node.textContent}${enabled ? "可用" : "不可用"}`);
}
function statusLabel(s) {
  if (s.status === "connected_limited") return ["已登录 · 能力受限","warn"];
  if (s.connected || ["connected","online"].includes(s.status)) return ["已连接","ok"];
  if (s.status === "scanned") return ["已扫码","warn"];
  if (s.status === "verify_required") return ["需要验证码","warn"];
  if (s.status === "qr_ready") return ["等待扫码","warn"];
  if (s.status === "login_expired") return ["二维码已过期","warn"];
  if (s.status === "login_error" || s.status === "error") return ["连接异常","warn"];
  if (s.status === "driver_required") return ["连接不可用","warn"];
  if (s.status === "waiting_login") return ["等待登录","warn"];
  return ["未连接",""];
}
function renderStatus(raw) {
  const s = raw?.data || raw || {};
  const [label, cls] = statusLabel(s);
  text("badge", label); $("badge").className = `badge ${cls}`;
  text("status-message", s.message || s.lastError || "等待连接");
  text("nickname", s.nickname || (s.connected ? "个人微信" : "未登录"));
  text("account-id", s.alias || s.accountId || "—");
  text("driver", s.protocol === "ilink" ? `腾讯 iLink${s.driverVersion ? ` · ${s.driverVersion}` : ""}` : "未连接");
  text("platform", `${s.platform || "—"}${s.architecture ? `/${s.architecture}` : ""}`);
  text("native", s.localWechatRequired === false ? "插件自有账号 · 不读取本机微信" : "状态未知");
  capability("cap-qr", true);
  capability("cap-login", true);
  capability("cap-recv", true);
  capability("cap-send", true);
  text("received", s.messageCount || 0); text("sent", s.replyCount || 0);
  if (s.avatar) { $("avatar").style.backgroundImage = `url(${JSON.stringify(s.avatar).slice(1,-1)})`; $("avatar").textContent = ""; }
  const qrSource = s.qrImageUrl || s.qrCodeUrl;
  if (qrSource) { $("qr").src = qrSource; show("qr", true); show("qr-placeholder", false); }
  else if (s.connected) { show("qr", false); show("qr-placeholder", true); text("qr-placeholder", "微信已连接，可关闭此页面保持后台运行"); }
  show("verify-row", s.status === "verify_required");
  setError(s.lastError || "");
}
async function refresh() {
  try { renderStatus(await action("status")); } catch (e) { setError(e?.message || String(e)); }
}
async function watchLogin() {
  const token = ++loginWatchToken;
  for (let attempt = 0; attempt < 150; attempt += 1) {
    await new Promise((resolve) => window.setTimeout(resolve, 2000));
    if (token !== loginWatchToken) return;
    let raw;
    try { raw = await action("status"); } catch (e) { setError(e?.message || String(e)); continue; }
    if (token !== loginWatchToken) return;
    renderStatus(raw);
    const state = raw?.data || raw || {};
    if (state.connected || ["connected", "connected_limited", "login_error", "login_expired", "error"].includes(state.status)) return;
  }
}
async function connect() {
  setBusy(true); setError("");
  try {
    const result = await action("connect", { config: { force: true } });
    const data = result?.data || result || {};
    const qrSource = data.qrImageUrl || data.qrCodeUrl;
    if (qrSource) { $("qr").src = qrSource; show("qr", true); show("qr-placeholder", false); }
    await refresh();
    void watchLogin();
  } catch (e) { setError(e?.message || String(e)); }
  finally { setBusy(false); }
}
async function verify() {
  const code = String($("verify-code").value || "").trim();
  if (!code) { setError("请输入手机微信显示的数字"); return; }
  try { await action("connect", { config: { verifyCode: code } }); $("verify-code").value = ""; await refresh(); }
  catch (e) { setError(e?.message || String(e)); }
}
async function disconnect() {
  loginWatchToken += 1;
  try { await action("disconnect"); await refresh(); } catch (e) { setError(e?.message || String(e)); }
}
function renderMessages(result) {
  const data = result?.data || result || {};
  const bindings = Array.isArray(data.bindings) ? data.bindings : [];
  const messages = Array.isArray(data.messages) ? data.messages : [];
  $("bindings").innerHTML = "";
  for (const b of bindings) {
    const btn = document.createElement("button"); btn.className = `binding ${b.conversationId === currentConversation ? "active" : ""}`;
    btn.textContent = b.displayName || b.externalUserId || b.externalConversationId || b.conversationId || "会话";
    btn.onclick = async () => { currentConversation = b.conversationId || ""; await loadMessages(); };
    $("bindings").appendChild(btn);
  }
  $("messages").innerHTML = "";
  for (const m of messages) {
    const row = document.createElement("div"); row.className = "message";
    const meta = document.createElement("small"); meta.textContent = `${m.role || "message"} · ${m.createdAt || ""}`;
    const p = document.createElement("p"); p.textContent = m.content || m.text || "";
    row.append(meta,p); $("messages").appendChild(row);
  }
}
async function loadMessages() {
  try { renderMessages(await action("messages", { conversationId: currentConversation, limit: 200, offset: 0 })); } catch (e) { setError(e?.message || String(e)); }
}

async function start() {
  if (bridge?.ready) await bridge.ready();
  let ctx = {}; try { ctx = bridge?.getContext ? await bridge.getContext() : {}; } catch {}
  const route = String(ctx?.route || ctx?.uiContext?.route || "");
  const isMessages = route.includes("/messages");
  show("messages-view", isMessages); show("connect-view", !isMessages);
  $("refresh").onclick = refresh; $("connect").onclick = connect; $("verify").onclick = verify; $("disconnect").onclick = disconnect; $("refresh-messages").onclick = loadMessages;
  if (isMessages) await loadMessages(); else await refresh();
}
void start();
