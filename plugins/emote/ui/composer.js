const state = { groups: [], emotes: [], groupId: "", query: "", open: false, context: {} };
const $ = (id) => document.getElementById(id);

async function call(action, payload = {}) {
  const result = await window.amitiaUI.invokeAction("command", { action, payload });
  return result && Object.prototype.hasOwnProperty.call(result, "result") ? result.result : result;
}

function escapeHTML(value) {
  return String(value == null ? "" : value).replace(/[&<>"']/g, (char) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[char]));
}

async function load() {
  if (!state.groups.length) state.groups = await call("groups.list");
  $("group").innerHTML = `<option value="">全部表情</option><option value="recent">最近使用</option>` + state.groups.map((group) => `<option value="${escapeHTML(group.id)}">${escapeHTML(group.name)}</option>`).join("");
  await loadEmotes();
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
  window.amitiaUI.requestResize(380, 360);
  load().catch(() => {});
}

function close() {
  state.open = false;
  $("panel").classList.add("hidden");
  window.amitiaUI.requestResize(38, 44);
}

$("toggle").onclick = () => state.open ? close() : open();
$("group").onchange = () => { state.groupId = $("group").value; loadEmotes(); };
let timer;
$("search").oninput = () => { state.query = $("search").value; clearTimeout(timer); timer = setTimeout(loadEmotes, 200); };

window.amitiaUI.ready().then(async () => {
  state.context = await window.amitiaUI.getContext();
  window.amitiaUI.requestResize(38, 44);
}).catch(() => {});
