const state = { groups: [], emotes: [], groupId: "", query: "", open: false, context: {}, cacheKey: "" };
const $ = (id) => document.getElementById(id);
let loadPromise = null;
let hostOpen = false;

async function call(action, payload = {}) {
  const result = await window.amitiaUI.invokeAction("command", { action, payload });
  return result && Object.prototype.hasOwnProperty.call(result, "result") ? result.result : result;
}

function escapeHTML(value) {
  return String(value == null ? "" : value).replace(/[&<>"']/g, (char) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[char]));
}

function resolveCacheKey() {
  return `amitia.emote.composer.v1:${state.context.characterId || "default"}:${state.context.conversationId || "default"}`;
}

function restoreCache() {
  state.cacheKey = resolveCacheKey();
  try {
    const cached = JSON.parse(sessionStorage.getItem(state.cacheKey) || "null");
    if (!cached || Date.now() - Number(cached.savedAt || 0) > 10 * 60 * 1000) return false;
    state.groups = Array.isArray(cached.groups) ? cached.groups : [];
    state.emotes = Array.isArray(cached.emotes) ? cached.emotes : [];
    return true;
  } catch {
    return false;
  }
}

function persistCache() {
  if (!state.cacheKey) state.cacheKey = resolveCacheKey();
  try {
    sessionStorage.setItem(state.cacheKey, JSON.stringify({
      savedAt: Date.now(),
      groups: state.groups,
      emotes: state.emotes,
    }));
  } catch {}
}

async function loadGroups() {
  state.groups = await call("groups.list");
  if (!Array.isArray(state.groups)) state.groups = [];
  $("group").innerHTML = `<option value="">全部表情</option><option value="recent">最近使用</option>` + state.groups.map((group) => `<option value="${escapeHTML(group.id)}">${escapeHTML(group.name)}</option>`).join("");
  persistCache();
}

async function loadEmotes() {
  const recent = state.groupId === "recent";
  const data = await call("emotes.list", {
    groupId: recent ? undefined : state.groupId || undefined,
    view: recent ? "recent" : undefined,
    q: state.query,
    pageSize: 100,
  });
  state.emotes = data.items || [];
  $("grid").innerHTML = state.emotes.length ? state.emotes.map((item) =>
    `<button class="item" data-id="${escapeHTML(item.id)}"><img src="${escapeHTML(item.thumbnailUrl || item.assetUrl || "")}" alt=""><span>${escapeHTML(item.name)}</span></button>`
  ).join("") : `<div class="empty">没有可用表情</div>`;
  $("grid").querySelectorAll("[data-id]").forEach((button) => button.onclick = () => send(button.dataset.id));
  persistCache();
}

function load() {
  if (loadPromise) return loadPromise;
  loadPromise = Promise.all([loadGroups(), loadEmotes()]).finally(() => {
    loadPromise = null;
  });
  return loadPromise;
}

async function send(emoteId) {
  try {
    await call("emotes.send", {
      emoteId,
      conversationId: state.context.conversationId,
      characterId: state.context.characterId,
      channel: state.context.channel || "web",
    });
    close();
  } catch (error) {
    alert(error instanceof Error ? error.message : String(error));
  }
}

function open() {
  state.open = true;
  $("panel").classList.remove("hidden");
  window.amitiaUI.requestResize(360, 480);
  load().catch(() => {});
}

function close() {
  state.open = false;
  $("panel").classList.add("hidden");
  window.amitiaUI.requestResize(32, 32);
}

function applyHostSurfaceState(context) {
  const panelBottom = Number(context?.surfaceMetrics?.panelBottom ?? context?.uiContext?.surfaceMetrics?.panelBottom);
  if (Number.isFinite(panelBottom) && panelBottom > 0) {
    $("panel").style.bottom = `${Math.round(panelBottom)}px`;
  }
  const next = Boolean(context?.surfaceState?.open ?? context?.uiContext?.surfaceState?.open);
  if (next && !hostOpen) open();
  hostOpen = next;
}

$("toggle").onclick = () => state.open ? close() : open();
$("group").onchange = () => { state.groupId = $("group").value; loadEmotes(); };
let timer;
$("search").oninput = () => { state.query = $("search").value; clearTimeout(timer); timer = setTimeout(loadEmotes, 200); };

window.amitiaUI.ready().then(async () => {
  state.context = await window.amitiaUI.getContext();
  applyHostSurfaceState(state.context);
  window.amitiaUI.onHostContextChange?.((context) => applyHostSurfaceState(context));
  if (restoreCache()) {
    $("group").innerHTML = `<option value="">全部表情</option><option value="recent">最近使用</option>` + state.groups.map((group) => `<option value="${escapeHTML(group.id)}">${escapeHTML(group.name)}</option>`).join("");
    $("grid").innerHTML = state.emotes.length ? state.emotes.map((item) =>
      `<button class="item" data-id="${escapeHTML(item.id)}"><img src="${escapeHTML(item.thumbnailUrl || item.assetUrl || "")}" alt=""><span>${escapeHTML(item.name)}</span></button>`
    ).join("") : `<div class="empty">没有可用表情</div>`;
    $("grid").querySelectorAll("[data-id]").forEach((button) => button.onclick = () => send(button.dataset.id));
  }
  void load().catch(() => {});
  window.amitiaUI.requestResize(32, 32);
}).catch(() => {});
